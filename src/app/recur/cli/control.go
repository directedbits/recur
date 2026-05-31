package cli

import (
	"context"
	"fmt"
	"strings"

	displayterminal "github.com/directedbits/recur/src/infra/terminal/display"
	recurv1 "github.com/directedbits/recur/src/infra/grpc/v1"
	"github.com/spf13/cobra"
)

func newSuspendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "suspend [entity] <id>",
		Short: "Suspend a trigger or action",
		Long:  "Suspend a trigger or action. If no entity type is given, the identifier is resolved automatically.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := requireDaemon(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = client.Close() }()

			quiet, _ := cmd.Flags().GetBool("quiet")
			identifier := args[0]

			resp, err := client.Service.SuspendEntity(context.Background(), &recurv1.SuspendEntityRequest{
				Identifier: identifier,
			})
			if err != nil {
				if ambErr := displayterminal.HandleAmbiguousError(err, false); ambErr != nil {
					return ambErr
				}
				return fmt.Errorf("failed to suspend: %v", err)
			}

			if !quiet {
				label := capitalizeFirst(resp.EntityType)
				fmt.Printf("%s %s suspended.\n", label, identifier)
			}
			return nil
		},
	}

	cmd.AddCommand(newSuspendTriggerCmd())
	cmd.AddCommand(newSuspendActionCmd())

	return cmd
}

func newResumeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resume [entity] <id>",
		Short: "Resume a suspended trigger or action",
		Long:  "Resume a suspended trigger or action. If no entity type is given, the identifier is resolved automatically.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := requireDaemon(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = client.Close() }()

			quiet, _ := cmd.Flags().GetBool("quiet")
			identifier := args[0]

			resp, err := client.Service.ResumeEntity(context.Background(), &recurv1.ResumeEntityRequest{
				Identifier: identifier,
			})
			if err != nil {
				if ambErr := displayterminal.HandleAmbiguousError(err, false); ambErr != nil {
					return ambErr
				}
				return fmt.Errorf("failed to resume: %v", err)
			}

			if !quiet {
				label := capitalizeFirst(resp.EntityType)
				fmt.Printf("%s %s resumed.\n", label, identifier)
			}
			return nil
		},
	}

	cmd.AddCommand(newResumeTriggerCmd())
	cmd.AddCommand(newResumeActionCmd())

	return cmd
}

func newTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "test [entity] <id>",
		Short: "Manually fire a trigger or execute an action",
		Long:  "Manually fire a trigger or execute an action. If no entity type is given, the identifier is resolved automatically.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := requireDaemon(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = client.Close() }()

			identifier := args[0]
			jsonFlag, _ := cmd.Flags().GetBool("json")

			sets, _ := cmd.Flags().GetStringArray("set")
			ctx := make(map[string]string)
			for _, s := range sets {
				k, v := splitKeyValue(s)
				ctx[k] = v
			}

			resp, err := client.Service.TestEntity(context.Background(), &recurv1.TestEntityRequest{
				Identifier: identifier,
				Context:    ctx,
			})
			if err != nil {
				if ambErr := displayterminal.HandleAmbiguousError(err, false); ambErr != nil {
					return ambErr
				}
				return fmt.Errorf("failed to test: %v", err)
			}

			if jsonFlag {
				return displayterminal.PrintJSON(resp)
			}

			switch resp.EntityType {
			case "trigger":
				fmt.Printf("Trigger %s fired.\n", identifier)
				for _, r := range resp.Results {
					printTestActionResult(r, "  ")
				}
			case "action":
				if resp.Result != nil {
					printTestActionResult(resp.Result, "")
				}
			}
			return nil
		},
	}

	cmd.Flags().StringArray("set", nil, "Set a context variable (repeatable, format: key=value)")

	cmd.AddCommand(newTestTriggerCmd())
	cmd.AddCommand(newTestActionCmd())

	return cmd
}

func newSuspendTriggerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "trigger <id>",
		Short: "Suspend a trigger",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := requireDaemon(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = client.Close() }()

			_, err = client.Service.SuspendEntity(context.Background(), &recurv1.SuspendEntityRequest{
				Identifier: args[0],
				EntityType: "trigger",
			})
			if err != nil {
				return fmt.Errorf("failed to suspend trigger: %v", err)
			}

			quiet, _ := cmd.Flags().GetBool("quiet")
			if !quiet {
				fmt.Printf("Trigger %s suspended.\n", args[0])
			}
			return nil
		},
	}
}

func newSuspendActionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "action <id>",
		Short: "Suspend an action",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := requireDaemon(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = client.Close() }()

			_, err = client.Service.SuspendEntity(context.Background(), &recurv1.SuspendEntityRequest{
				Identifier: args[0],
				EntityType: "action",
			})
			if err != nil {
				return fmt.Errorf("failed to suspend action: %v", err)
			}

			quiet, _ := cmd.Flags().GetBool("quiet")
			if !quiet {
				fmt.Printf("Action %s suspended.\n", args[0])
			}
			return nil
		},
	}
}

func newResumeTriggerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "trigger <id>",
		Short: "Resume a suspended trigger",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := requireDaemon(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = client.Close() }()

			_, err = client.Service.ResumeEntity(context.Background(), &recurv1.ResumeEntityRequest{
				Identifier: args[0],
				EntityType: "trigger",
			})
			if err != nil {
				return fmt.Errorf("failed to resume trigger: %v", err)
			}

			quiet, _ := cmd.Flags().GetBool("quiet")
			if !quiet {
				fmt.Printf("Trigger %s resumed.\n", args[0])
			}
			return nil
		},
	}
}

func newResumeActionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "action <id>",
		Short: "Resume a suspended action",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := requireDaemon(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = client.Close() }()

			_, err = client.Service.ResumeEntity(context.Background(), &recurv1.ResumeEntityRequest{
				Identifier: args[0],
				EntityType: "action",
			})
			if err != nil {
				return fmt.Errorf("failed to resume action: %v", err)
			}

			quiet, _ := cmd.Flags().GetBool("quiet")
			if !quiet {
				fmt.Printf("Action %s resumed.\n", args[0])
			}
			return nil
		},
	}
}

func newTestTriggerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trigger <id>",
		Short: "Manually fire a trigger",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := requireDaemon(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = client.Close() }()

			sets, _ := cmd.Flags().GetStringArray("set")
			ctx := make(map[string]string)
			for _, s := range sets {
				k, v := splitKeyValue(s)
				ctx[k] = v
			}

			resp, err := client.Service.TestEntity(context.Background(), &recurv1.TestEntityRequest{
				Identifier: args[0],
				EntityType: "trigger",
				Context:    ctx,
			})
			if err != nil {
				return fmt.Errorf("failed to test trigger: %v", err)
			}

			jsonFlag, _ := cmd.Flags().GetBool("json")
			if jsonFlag {
				return displayterminal.PrintJSON(resp)
			}

			fmt.Printf("Trigger %s fired.\n", args[0])
			for _, r := range resp.Results {
				printTestActionResult(r, "  ")
			}
			return nil
		},
	}

	cmd.Flags().StringArray("set", nil, "Set a context variable (repeatable, format: key=value)")

	return cmd
}

func newTestActionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "action <id>",
		Short: "Manually execute an action",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := requireDaemon(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = client.Close() }()

			sets, _ := cmd.Flags().GetStringArray("set")
			ctx := make(map[string]string)
			for _, s := range sets {
				k, v := splitKeyValue(s)
				ctx[k] = v
			}

			resp, err := client.Service.TestEntity(context.Background(), &recurv1.TestEntityRequest{
				Identifier: args[0],
				EntityType: "action",
				Context:    ctx,
			})
			if err != nil {
				return fmt.Errorf("failed to test action: %v", err)
			}

			jsonFlag, _ := cmd.Flags().GetBool("json")
			if jsonFlag {
				return displayterminal.PrintJSON(resp)
			}

			if resp.Result != nil {
				printTestActionResult(resp.Result, "")
			}
			return nil
		},
	}

	cmd.Flags().StringArray("set", nil, "Set a context variable (repeatable, format: key=value)")

	return cmd
}

func splitKeyValue(s string) (string, string) {
	for i, c := range s {
		if c == '=' {
			return s[:i], s[i+1:]
		}
	}
	return s, ""
}

// capitalizeFirst returns the string with the first letter uppercased.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// printTestActionResult displays a test action result with full detail.
func printTestActionResult(r *recurv1.TestActionResult, indent string) {
	status := "success"
	if !r.Success {
		status = "failed"
	}

	detail := status
	if r.ExitCode != 0 {
		detail = fmt.Sprintf("%s (exit code %d)", status, r.ExitCode)
	}
	if r.Duration != "" {
		detail = fmt.Sprintf("%s, %s", detail, r.Duration)
	}

	fmt.Printf("%sAction %s: %s\n", indent, r.ActionType, detail)

	if r.Error != "" {
		fmt.Printf("%s  Error: %s\n", indent, r.Error)
	}
	if r.Output != "" {
		fmt.Printf("%s  Output: %s\n", indent, r.Output)
	}
}
