// fileevents is an external trigger plugin that provides file system event
// monitoring powered by fsnotify/fsbroker.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	sdk "github.com/directedbits/recur/pkg/plugin-sdk"
)

// pluginInput is the JSON payload read from stdin.
type pluginInput struct {
	TriggerType string         `json:"trigger_type"`
	Options     map[string]any `json:"options"`
	Config      map[string]any `json:"config"`
}

// parseInput reads stdin JSON and validates the file events trigger configuration.
func parseInput(r io.Reader) (*pluginInput, *parsedOptions, error) {
	var input pluginInput
	if err := json.NewDecoder(r).Decode(&input); err != nil {
		return nil, nil, fmt.Errorf("reading stdin: %w", err)
	}

	if !isFileEventType(input.TriggerType) {
		return nil, nil, fmt.Errorf("unsupported trigger_type: %s", input.TriggerType)
	}

	opts, err := parseOptions(input.Options, input.Config)
	if err != nil {
		return nil, nil, err
	}

	// If no path specified, use the working directory (the daemon sets this
	// to the recurfile's parent directory per the plugin protocol).
	if opts.WatchPath == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, nil, fmt.Errorf("no watch path and cannot determine working directory: %w", err)
		}
		opts.WatchPath = wd
	}

	return &input, opts, nil
}

func main() {
	log.SetPrefix("fileevents: ")
	log.SetFlags(0)

	input, opts, err := parseInput(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	socketPath := os.Getenv("RECUR_SOCKET")
	triggerID := os.Getenv("RECUR_TRIGGER_ID")
	if socketPath == "" || triggerID == "" {
		log.Fatal("RECUR_SOCKET and RECUR_TRIGGER_ID must be set")
	}

	broker, err := createBroker(opts)
	if err != nil {
		log.Fatal(err)
	}

	client, err := sdk.Connect(socketPath)
	if err != nil {
		log.Fatalf("connecting to daemon: %v", err)
	}
	defer func() { _ = client.Close() }()

	broker.Start()
	defer broker.Stop()

	log.Printf("started: %s path=%q recursive=%v entity_type=%s filters=%v",
		input.TriggerType, opts.WatchPath, opts.Recursive, opts.EntityType, opts.Filters)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

	for {
		select {
		case action, ok := <-broker.Next():
			if !ok {
				log.Print("watcher closed, exiting")
				return
			}

			if !matchesTriggerType(input.TriggerType, action.Type) {
				continue
			}

			filePath := ""
			isDir := false
			if action.Subject != nil {
				filePath = action.Subject.Path
				isDir = action.Subject.IsDir()
			}

			if !matchesFilter(opts.Filters, filePath) {
				continue
			}

			if matchesExclude(opts.ExcludePaths, filePath) {
				continue
			}

			if opts.DefaultsActive && isRecurfileName(filepath.Base(filePath)) {
				continue
			}

			if !matchesEntityType(opts.EntityType, isDir) {
				continue
			}

			ctxVars := buildContext(input.TriggerType, action)
			ctxVars["TriggeredOn"] = time.Now().UTC().Format(time.RFC3339)

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

			log.Printf("event reported: %s path=%q", input.TriggerType, filePath)

		case err, ok := <-broker.Error():
			if !ok {
				return
			}
			log.Printf("watcher error: %v", err)

		case sig := <-sigCh:
			fmt.Fprintf(os.Stderr, "received %v, shutting down\n", sig)
			return
		}
	}
}
