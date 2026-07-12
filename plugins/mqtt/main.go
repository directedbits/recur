// mqtt is an external plugin that provides MQTT triggers (subscribe) and
// actions (publish). The binary mode is determined by the stdin JSON:
// trigger_type present → trigger mode, action_type present → action mode.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	sdk "github.com/directedbits/recur/pkg/plugin-sdk"
)

// pluginInput is the JSON payload read from stdin.
type pluginInput struct {
	TriggerType string         `json:"trigger_type"`
	ActionType  string         `json:"action_type"`
	Options     map[string]any `json:"options"`
	Config      map[string]any `json:"config"`
	Test        bool           `json:"test"`
}

// actionOutput is the JSON response written to stdout for action mode.
type actionOutput struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error"`
}

// parseInput reads stdin JSON and validates the basic structure.
func parseInput(r io.Reader) (*pluginInput, error) {
	var input pluginInput
	if err := json.NewDecoder(r).Decode(&input); err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}

	if input.TriggerType == "" && input.ActionType == "" {
		return nil, fmt.Errorf("stdin must contain trigger_type or action_type")
	}

	if input.TriggerType != "" && input.TriggerType != "MessageReceived" {
		return nil, fmt.Errorf("unsupported trigger_type: %s", input.TriggerType)
	}

	if input.ActionType != "" && input.ActionType != "Publish" {
		return nil, fmt.Errorf("unsupported action_type: %s", input.ActionType)
	}

	return &input, nil
}

func main() {
	log.SetPrefix("mqtt: ")
	log.SetFlags(0)

	input, err := parseInput(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	switch {
	case input.TriggerType != "":
		runTrigger(input)
	case input.ActionType != "":
		runAction(input)
	}
}

func runTrigger(input *pluginInput) {
	socketPath := os.Getenv("RECUR_SOCKET")
	triggerID := os.Getenv("RECUR_TRIGGER_ID")
	if socketPath == "" || triggerID == "" {
		log.Fatal("RECUR_SOCKET and RECUR_TRIGGER_ID must be set")
	}

	cfg, err := BuildClientConfig(input.Options, input.Config)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	if cfg.Topic == "" {
		log.Fatal("topic option is required")
	}

	messages, cleanup, err := Subscribe(cfg)
	if err != nil {
		log.Fatalf("subscribe: %v", err)
	}
	defer cleanup()

	log.Printf("subscribed: broker=%s topic=%s qos=%d", cfg.Broker, cfg.Topic, cfg.QoS)

	client, err := sdk.Connect(socketPath)
	if err != nil {
		log.Fatalf("connecting to daemon: %v", err)
	}
	defer func() { _ = client.Close() }()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	for {
		select {
		case msg, ok := <-messages:
			if !ok {
				log.Print("message channel closed, exiting")
				return
			}

			ctxVars := map[string]string{
				"Topic":     msg.Topic,
				"Payload":   msg.Payload,
				"QoS":       strconv.Itoa(int(msg.QoS)),
				"Retained":  strconv.FormatBool(msg.Retained),
				"MessageID": strconv.FormatUint(uint64(msg.MessageID), 10),
			}

			resp, err := client.Service.ReportTriggerEvent(context.Background(), &sdk.ReportTriggerEventRequest{
				TriggerId: triggerID,
				Context:   ctxVars,
			})
			if err != nil {
				log.Printf("reporting event: %v", err)
				continue
			}
			if !resp.Accepted {
				log.Printf("event rejected: %s", resp.Error)
				continue
			}

			log.Printf("event reported: topic=%s len=%d", msg.Topic, len(msg.Payload))

		case sig := <-sigCh:
			fmt.Fprintf(os.Stderr, "received %v, shutting down\n", sig)
			return
		}
	}
}

func runAction(input *pluginInput) {
	cfg, err := BuildClientConfig(input.Options, input.Config)
	if err != nil {
		writeActionOutput(false, "", fmt.Sprintf("config: %v", err))
		return
	}

	if cfg.Topic == "" {
		writeActionOutput(false, "", "topic option is required")
		return
	}

	payload := optStr(input.Options, "payload", "")
	retainStr := optStr(input.Options, "retain", "false")
	retain := retainStr == "true"

	// Test mode: return success without connecting
	if input.Test {
		writeActionOutput(true, fmt.Sprintf("would publish to %s on %s", cfg.Topic, cfg.Broker), "")
		return
	}

	if err := Publish(cfg, payload, retain); err != nil {
		writeActionOutput(false, "", err.Error())
		return
	}

	writeActionOutput(true, fmt.Sprintf("published to %s", cfg.Topic), "")
}

func writeActionOutput(success bool, output, errMsg string) {
	writeActionOutputTo(os.Stdout, success, output, errMsg)
}

func writeActionOutputTo(w io.Writer, success bool, output, errMsg string) {
	out := actionOutput{
		Success: success,
		Output:  output,
		Error:   errMsg,
	}
	_ = json.NewEncoder(w).Encode(out)
}
