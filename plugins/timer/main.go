// timer is an external trigger plugin that provides time-driven triggers
// via cron schedules and fixed intervals.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	clientgrpc "github.com/directedbits/recur/src/infra/grpc/client"
	recurv1 "github.com/directedbits/recur/src/infra/grpc/v1"
)

// pluginInput is the JSON payload read from stdin.
type pluginInput struct {
	TriggerType string         `json:"trigger_type"`
	Options     map[string]any `json:"options"`
	Config      map[string]any `json:"config"`
}

// parsedTimerInput holds the validated fields extracted from a pluginInput.
type parsedTimerInput struct {
	Input       *pluginInput
	FireOnStart bool
	// Cron fields
	Expression string
	Timezone   string
	// Interval fields
	Every string
}

// parseInput reads stdin JSON and validates the timer trigger configuration.
func parseInput(r io.Reader) (*parsedTimerInput, error) {
	var input pluginInput
	if err := json.NewDecoder(r).Decode(&input); err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}

	fireOnStart := optString(input.Options, "fire_on_start", "false") == "true"

	result := &parsedTimerInput{
		Input:       &input,
		FireOnStart: fireOnStart,
	}

	switch input.TriggerType {
	case "Cron":
		result.Expression = optString(input.Options, "expression", "")
		if result.Expression == "" {
			return nil, fmt.Errorf("cron trigger requires 'expression' option")
		}
		result.Timezone = optString(input.Options, "timezone", "Local")

	case "Interval":
		result.Every = optString(input.Options, "every", "")
		if result.Every == "" {
			return nil, fmt.Errorf("interval trigger requires 'every' option")
		}

	default:
		return nil, fmt.Errorf("unsupported trigger_type: %s", input.TriggerType)
	}

	return result, nil
}

func main() {
	log.SetPrefix("timer: ")
	log.SetFlags(0)

	parsed, err := parseInput(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	// Read required env vars
	socketPath := os.Getenv("RECUR_SOCKET")
	triggerID := os.Getenv("RECUR_TRIGGER_ID")
	if socketPath == "" || triggerID == "" {
		log.Fatal("RECUR_SOCKET and RECUR_TRIGGER_ID must be set")
	}

	// Set up the event source based on trigger type
	var events <-chan TickEvent
	var stop func()

	switch parsed.Input.TriggerType {
	case "Cron":
		events, stop, err = StartCron(parsed.Expression, parsed.Timezone, parsed.FireOnStart)
		if err != nil {
			log.Fatalf("starting cron: %v", err)
		}
		log.Printf("started: Cron expression=%q timezone=%s fire_on_start=%v", parsed.Expression, parsed.Timezone, parsed.FireOnStart)

	case "Interval":
		events, stop, err = StartInterval(parsed.Every, parsed.FireOnStart)
		if err != nil {
			log.Fatalf("starting interval: %v", err)
		}
		log.Printf("started: Interval every=%s fire_on_start=%v", parsed.Every, parsed.FireOnStart)
	}

	defer stop()

	// Connect to daemon gRPC socket
	client, err := clientgrpc.Connect(socketPath)
	if err != nil {
		log.Fatalf("connecting to daemon: %v", err)
	}
	defer client.Close()

	// Set up signal handler for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	// Event loop
	for {
		select {
		case tick, ok := <-events:
			if !ok {
				log.Print("event channel closed, exiting")
				return
			}

			ctxVars := map[string]string{
				"TickCount":        tick.TickCount,
				"TimeSinceStarted": tick.TimeSinceStarted,
			}

			resp, err := client.Service.ReportTriggerEvent(context.Background(), &recurv1.ReportTriggerEventRequest{
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

			log.Printf("tick #%s reported (uptime: %s)", tick.TickCount, tick.TimeSinceStarted)

		case sig := <-sigCh:
			fmt.Fprintf(os.Stderr, "received %v, shutting down\n", sig)
			return
		}
	}
}

// optString extracts a string option with a default fallback.
func optString(opts map[string]any, key, fallback string) string {
	if v, ok := opts[key].(string); ok && v != "" {
		return v
	}
	return fallback
}
