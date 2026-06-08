package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	displayterminal "github.com/directedbits/recur/src/infra/terminal/display"
	recurv1 "github.com/directedbits/recur/src/infra/grpc/recur/v1"
	recurfileyaml "github.com/directedbits/recur/src/infra/yaml/recurfile"
	"github.com/spf13/cobra"
)

// resolveRecurfilePathOrID resolves the argument as a file path first, then as
// a recurfile ID via the daemon, falling back to default recurfile discovery.
func resolveRecurfilePathOrID(cmd *cobra.Command, args []string) (string, error) {
	if len(args) == 0 {
		return resolveRecurfilePath(args)
	}

	arg := args[0]

	// If it exists as a file, use it directly
	if _, err := os.Stat(arg); err == nil {
		abs, err := filepath.Abs(arg)
		if err != nil {
			return arg, nil
		}
		return abs, nil
	}

	// Try as a recurfile ID via the daemon
	socketPath, _ := resolveSocketPath(cmd)
	client := connectOrNilFunc(socketPath)
	if client != nil {
		defer func() { _ = client.Close() }()
		resp, err := client.Service.InspectEntity(context.Background(), &recurv1.InspectEntityRequest{
			Identifier: arg,
			EntityType: "recurfile",
		})
		if err == nil && resp.Recurfile != nil && resp.Recurfile.Path != "" {
			return resp.Recurfile.Path, nil
		}
	}

	return "", fmt.Errorf("recurfile not found: %s (not a file path or known recurfile ID)", arg)
}

// ErrValidationFailed is returned when recurfile validation fails.
var ErrValidationFailed = errors.New("recurfile validation failed")

// recurfileNamePattern is the human-readable description of accepted Recurfile
// names. Use recurfileyaml.IsRecurfileName for actual matching.
const recurfileNamePattern = "recurfile (case-insensitive) with optional .yaml or .yml extension"

func newRegisterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register [file|id]",
		Short: "Register a recurfile with the daemon",
		Long: `Register a recurfile with the daemon. If the recurfile is already registered,
it will be deregistered and re-registered atomically (reload).

The argument can be a file path or a recurfile ID. If no argument is specified,
searches the current directory for a file named recurfile (case-insensitive)
with an optional .yaml or .yml extension.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			verifyOnly, _ := cmd.Flags().GetBool("verify")
			jsonFlag, _ := cmd.Flags().GetBool("json")

			path, err := resolveRecurfilePathOrID(cmd, args)
			if err != nil {
				return err
			}

			// Parse and validate the recurfile locally first
			f, err := recurfileyaml.Load(path)
			if err != nil {
				return err
			}

			if verifyOnly {
				err := runVerify(cmd, f, path, jsonFlag)
				if errors.Is(err, ErrValidationFailed) {
					cmd.SilenceErrors = true
				}
				return err
			}

			return runRegister(cmd, f, path, jsonFlag)
		},
	}

	cmd.Flags().Bool("verify", false, "Dry-run mode: validate without registering")

	return cmd
}

func newVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify [file]",
		Short: "Validate a recurfile without registering (alias for register --verify)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonFlag, _ := cmd.Flags().GetBool("json")

			path, err := resolveRecurfilePath(args)
			if err != nil {
				return err
			}

			f, err := recurfileyaml.Load(path)
			if err != nil {
				return err
			}

			err = runVerify(cmd, f, path, jsonFlag)
			if errors.Is(err, ErrValidationFailed) {
				cmd.SilenceErrors = true
			}
			return err
		},
	}
}

func runVerify(cmd *cobra.Command, f *recurfileyaml.RawFile, path string, jsonFlag bool) error {
	// Count triggers and actions
	triggerCount, actionCount := countEntities(f)
	var warnings []string

	// Check for triggers with no actions at any level
	for _, g := range f.Groups {
		for _, t := range g.Triggers {
			if len(t.Actions) == 0 && len(g.Actions) == 0 {
				warnings = append(warnings, fmt.Sprintf("group %q, trigger %q: no actions defined", g.Name, t.Type))
			}
		}
	}

	// Try daemon for deeper validation if available
	socketPath, _ := resolveSocketPath(cmd)
	client := connectOrNilFunc(socketPath)
	if client != nil {
		defer func() { _ = client.Close() }()
		absPath, _ := filepath.Abs(path)
		resp, err := client.Service.VerifyRecurfile(context.Background(), &recurv1.VerifyRecurfileRequest{
			Path: absPath,
		})
		if err == nil {
			// Use daemon's richer validation results
			return printVerifyResult(resp, jsonFlag)
		}
		// Daemon verify failed — fall back to local-only validation
	}

	// Local-only verify result
	if jsonFlag {
		data, _ := json.MarshalIndent(map[string]any{
			"valid":         true,
			"path":          path,
			"trigger_count": triggerCount,
			"action_count":  actionCount,
			"warnings":      warnings,
		}, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Printf("Valid: %s\n", path)
		fmt.Printf("  Groups:   %d\n", len(f.Groups))
		fmt.Printf("  Triggers: %d\n", triggerCount)
		fmt.Printf("  Actions:  %d\n", actionCount)
		for _, w := range warnings {
			fmt.Printf("  Warning: %s\n", w)
		}
	}
	return nil
}

func runRegister(cmd *cobra.Command, f *recurfileyaml.RawFile, path string, jsonFlag bool) error {
	socketPath, _ := resolveSocketPath(cmd)
	client, err := connectFunc(socketPath)
	if err != nil {
		return fmt.Errorf("daemon is not running (start it with 'recur start'): %w", err)
	}
	defer func() { _ = client.Close() }()

	absPath, _ := filepath.Abs(path)
	resp, err := client.Service.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{
		Path: absPath,
	})
	if err != nil {
		return fmt.Errorf("registration failed: %v", err)
	}

	quiet, _ := cmd.Flags().GetBool("quiet")
	if quiet {
		return nil
	}

	if jsonFlag {
		data, _ := json.MarshalIndent(map[string]any{
			"id":            resp.Id,
			"path":          resp.Path,
			"trigger_count": resp.TriggerCount,
			"action_count":  resp.ActionCount,
			"warnings":      resp.Warnings,
			"reloaded":      resp.Reloaded,
		}, "", "  ")
		fmt.Println(string(data))
	} else {
		verb := "Registered"
		if resp.Reloaded {
			verb = "Reloaded"
		}
		fmt.Printf("%s: %s (id: %s)\n", verb, resp.Path, resp.Id)
		fmt.Printf("  Triggers: %d\n", resp.TriggerCount)
		fmt.Printf("  Actions:  %d\n", resp.ActionCount)
		for _, w := range resp.Warnings {
			fmt.Printf("  Warning: %s\n", w)
		}
	}
	return nil
}

func printVerifyResult(resp *recurv1.VerifyRecurfileResponse, jsonFlag bool) error {
	if jsonFlag {
		data, _ := json.MarshalIndent(map[string]any{
			"valid":         resp.Valid,
			"errors":        resp.Errors,
			"warnings":      resp.Warnings,
			"trigger_count": resp.TriggerCount,
			"action_count":  resp.ActionCount,
		}, "", "  ")
		fmt.Println(string(data))
	} else {
		if resp.Valid {
			fmt.Println("Valid")
		} else {
			fmt.Println("Invalid")
		}
		fmt.Printf("  Triggers: %d\n", resp.TriggerCount)
		fmt.Printf("  Actions:  %d\n", resp.ActionCount)
		for _, e := range resp.Errors {
			fmt.Printf("  Error: %s\n", e)
		}
		for _, w := range resp.Warnings {
			fmt.Printf("  Warning: %s\n", w)
		}
	}

	if !resp.Valid {
		return ErrValidationFailed
	}
	return nil
}

func resolveRecurfilePath(args []string) (string, error) {
	if len(args) > 0 {
		path := args[0]
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("recurfile not found: %s", path)
		}
		return path, nil
	}

	// Search CWD for files matching the recurfile naming convention.
	entries, err := os.ReadDir(".")
	if err != nil {
		return "", fmt.Errorf("could not read current directory: %w", err)
	}
	var found []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if recurfileyaml.IsRecurfileName(entry.Name()) {
			found = append(found, entry.Name())
		}
	}

	switch len(found) {
	case 0:
		return "", fmt.Errorf("no recurfile found in current directory (expected a file named %s)",
			recurfileNamePattern)
	case 1:
		return found[0], nil
	default:
		return "", fmt.Errorf("multiple recurfiles found: %s (specify one explicitly)",
			displayterminal.JoinNames(found))
	}
}

func countEntities(f *recurfileyaml.RawFile) (triggers, actions int) {
	for _, g := range f.Groups {
		triggers += len(g.Triggers)
		for _, t := range g.Triggers {
			if len(t.Actions) > 0 {
				actions += len(t.Actions)
			} else {
				actions += len(g.Actions)
			}
		}
	}
	return
}

