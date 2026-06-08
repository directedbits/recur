package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	displayterminal "github.com/directedbits/recur/src/infra/terminal/display"
	recurv1 "github.com/directedbits/recur/src/infra/grpc/recur/v1"
	"github.com/spf13/cobra"
)

func newDeregisterCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "deregister [id|path]",
		Short: "Deregister a recurfile and all its triggers/actions",
		Long:  "Deregister a recurfile. If no argument is given, searches the current directory for a recurfile (recur.yaml, recur.yml, etc.).",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := requireDaemon(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = client.Close() }()

			var identifier string
			if len(args) > 0 {
				identifier = resolvePathIdentifier(args[0])
			} else {
				path, err := resolveRecurfilePath(nil)
				if err != nil {
					return err
				}
				identifier = resolvePathIdentifier(path)
			}

			jsonFlag, _ := cmd.Flags().GetBool("json")

			resp, err := client.Service.DeregisterRecurfile(context.Background(), &recurv1.DeregisterRecurfileRequest{
				Identifier: identifier,
			})
			if err != nil {
				if ambErr := displayterminal.HandleAmbiguousError(err, jsonFlag); ambErr != nil {
					return ambErr
				}
				return fmt.Errorf("deregistration failed: %v", err)
			}

			quiet, _ := cmd.Flags().GetBool("quiet")
			if quiet {
				return nil
			}
			if jsonFlag {
				data, _ := json.MarshalIndent(map[string]any{
					"id":               resp.Id,
					"path":             resp.Path,
					"triggers_removed": resp.TriggersRemoved,
					"actions_removed":  resp.ActionsRemoved,
				}, "", "  ")
				fmt.Println(string(data))
			} else {
				fmt.Printf("Deregistered: %s (id: %s)\n", resp.Path, resp.Id)
				fmt.Printf("  Triggers removed: %d\n", resp.TriggersRemoved)
				fmt.Printf("  Actions removed:  %d\n", resp.ActionsRemoved)
			}
			return nil
		},
	}
}

// resolvePathIdentifier resolves a path-like identifier to an absolute path.
// If the input is an absolute path, it is cleaned and returned.
// If it matches a file in the current directory, it is resolved to an absolute path.
// Otherwise it is returned as-is (treated as an ID or name).
func resolvePathIdentifier(input string) string {
	if filepath.IsAbs(input) {
		return filepath.Clean(input)
	}

	// Check if it's a file in the current directory
	if _, err := os.Stat(input); err == nil {
		if abs, err := filepath.Abs(input); err == nil {
			return abs
		}
	}

	return input
}
