// docker is an external plugin that provides Docker container lifecycle
// triggers and management actions. The binary mode is determined by the
// stdin JSON: trigger_type present -> trigger mode, action_type -> action mode.
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
	"strings"
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

var validTriggers = map[string]bool{
	"ContainerStarted": true,
	"ContainerStopped": true,
	"HealthChanged":    true,
}

var validActions = map[string]bool{
	"ContainerStart":   true,
	"ContainerStop":    true,
	"ContainerRestart": true,
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

	if input.TriggerType != "" && !validTriggers[input.TriggerType] {
		return nil, fmt.Errorf("unsupported trigger_type: %s", input.TriggerType)
	}

	if input.ActionType != "" && !validActions[input.ActionType] {
		return nil, fmt.Errorf("unsupported action_type: %s", input.ActionType)
	}

	return &input, nil
}

func main() {
	log.SetPrefix("docker: ")
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

	host := optStr(input.Options, "host", "unix:///var/run/docker.sock")
	filterName := optStr(input.Options, "filter_name", "")
	filterImage := optStr(input.Options, "filter_image", "")
	filterLabel := optStr(input.Options, "filter_label", "")

	api, err := apiFactory(host)
	if err != nil {
		log.Fatalf("docker client: %v", err)
	}

	// Build Docker API-level event filters.
	filters := map[string][]string{}
	switch input.TriggerType {
	case "ContainerStarted":
		filters["event"] = []string{"start"}
	case "ContainerStopped":
		filters["event"] = []string{"stop", "die"}
	case "HealthChanged":
		filters["event"] = []string{"health_status"}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, errCh := api.Events(ctx, filters)

	log.Printf("watching: host=%s trigger=%s", host, input.TriggerType)

	grpcClient, err := sdk.Connect(socketPath)
	if err != nil {
		log.Fatalf("connecting to daemon: %v", err)
	}
	defer func() { _ = grpcClient.Close() }()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	for {
		select {
		case evt, ok := <-events:
			if !ok {
				log.Print("event channel closed, exiting")
				return
			}

			// Apply plugin-level filters.
			if !matchesFilter(evt, filterName, filterImage, filterLabel, evt.Labels) {
				continue
			}

			ctxVars := buildContextVars(input.TriggerType, evt)

			resp, err := grpcClient.Service.ReportTriggerEvent(context.Background(), &sdk.ReportTriggerEventRequest{
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

			log.Printf("event reported: container=%s status=%s", evt.ContainerName, evt.Status)

		case err, ok := <-errCh:
			if ok && err != nil {
				log.Fatalf("event stream error: %v", err)
			}

		case sig := <-sigCh:
			fmt.Fprintf(os.Stderr, "received %v, shutting down\n", sig)
			return
		}
	}
}

// buildContextVars creates the context map for a trigger event report.
func buildContextVars(triggerType string, evt ContainerEvent) map[string]string {
	vars := map[string]string{
		"ContainerID":   evt.ContainerID,
		"ContainerName": evt.ContainerName,
		"Image":         evt.Image,
	}

	switch triggerType {
	case "ContainerStarted":
		vars["Status"] = evt.Status
	case "ContainerStopped":
		vars["Status"] = evt.Status
		vars["ExitCode"] = evt.ExitCode
	case "HealthChanged":
		vars["HealthStatus"] = evt.HealthStatus
	}

	return vars
}

func runAction(input *pluginInput) {
	host := optStr(input.Options, "host", "unix:///var/run/docker.sock")
	container := optStr(input.Options, "container", "")
	if container == "" {
		writeActionOutput(false, "", "container option is required")
		return
	}

	// Test mode: return success without connecting.
	if input.Test {
		writeActionOutput(true, fmt.Sprintf("would %s container %s on %s",
			strings.ToLower(strings.TrimPrefix(input.ActionType, "Container")),
			container, host), "")
		return
	}

	api, err := apiFactory(host)
	if err != nil {
		writeActionOutput(false, "", fmt.Sprintf("docker client: %v", err))
		return
	}

	ctx := context.Background()

	switch input.ActionType {
	case "ContainerStart":
		if err := api.ContainerStart(ctx, container); err != nil {
			writeActionOutput(false, "", err.Error())
			return
		}
		writeActionOutput(true, fmt.Sprintf("started container %s", container), "")

	case "ContainerStop":
		timeout := parseTimeout(input.Options)
		if err := api.ContainerStop(ctx, container, timeout); err != nil {
			writeActionOutput(false, "", err.Error())
			return
		}
		writeActionOutput(true, fmt.Sprintf("stopped container %s", container), "")

	case "ContainerRestart":
		timeout := parseTimeout(input.Options)
		if err := api.ContainerRestart(ctx, container, timeout); err != nil {
			writeActionOutput(false, "", err.Error())
			return
		}
		writeActionOutput(true, fmt.Sprintf("restarted container %s", container), "")
	}
}

// parseTimeout extracts the timeout option, defaulting to 10 seconds.
func parseTimeout(options map[string]any) int {
	s := optStr(options, "timeout", "10")
	t, err := strconv.Atoi(s)
	if err != nil || t < 0 {
		return 10
	}
	return t
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
	json.NewEncoder(w).Encode(out)
}
