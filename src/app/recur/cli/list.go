package cli

import (
	"context"
	"fmt"
	"strings"

	displayterminal "github.com/directedbits/recur/src/infra/terminal/display"
	clientgrpc "github.com/directedbits/recur/src/infra/grpc/client"
	recurv1 "github.com/directedbits/recur/src/infra/grpc/recur/v1"
	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <entity>",
		Short: "List registered entities",
		Long:  "List registered entities. Supported: triggers, actions, groups, plugins, recurfiles.",
	}

	cmd.AddCommand(newListTriggersCmd())
	cmd.AddCommand(newListActionsCmd())
	cmd.AddCommand(newListGroupsCmd())
	cmd.AddCommand(newListPluginsCmd())
	cmd.AddCommand(newListRecurfilesCmd())

	return cmd
}

func requireDaemon(cmd *cobra.Command) (*clientgrpc.Client, error) {
	socketPath, _ := resolveSocketPath(cmd)
	client, err := connectFunc(socketPath)
	if err != nil {
		return nil, fmt.Errorf("daemon is not running (start it with 'recur start'): %w", err)
	}
	return client, nil
}

func newListTriggersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "triggers",
		Short: "List registered triggers",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := requireDaemon(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = client.Close() }()

			resp, err := client.Service.ListTriggers(context.Background(), &recurv1.ListTriggersRequest{})
			if err != nil {
				return fmt.Errorf("failed to list triggers: %v", err)
			}

			triggers := resp.Triggers
			if all, _ := cmd.Flags().GetBool("all"); !all {
				triggers = excludeTriggersByStatus(triggers, recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED)
			}

			jsonFlag, _ := cmd.Flags().GetBool("json")
			if jsonFlag {
				return displayterminal.PrintJSON(triggers)
			}

			if len(triggers) == 0 {
				fmt.Println("No triggers found.")
				return nil
			}

			for _, t := range triggers {
				status := displayterminal.EntityStatus(t.Status)
				fmt.Printf("%-12s %-20s %-25s %-10s %s\n", t.Id[:8], t.Name, t.Group, status, t.Plugin)
			}
			return nil
		},
	}
	cmd.Flags().BoolP("all", "a", false, "Include suspended triggers (default: hide suspended; active and error always shown)")
	return cmd
}

func newListActionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "actions",
		Short: "List registered actions",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := requireDaemon(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = client.Close() }()

			resp, err := client.Service.ListActions(context.Background(), &recurv1.ListActionsRequest{})
			if err != nil {
				return fmt.Errorf("failed to list actions: %v", err)
			}

			actions := resp.Actions
			if all, _ := cmd.Flags().GetBool("all"); !all {
				actions = excludeActionsByStatus(actions, recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED)
			}

			jsonFlag, _ := cmd.Flags().GetBool("json")
			if jsonFlag {
				return displayterminal.PrintJSON(actions)
			}

			if len(actions) == 0 {
				fmt.Println("No actions found.")
				return nil
			}

			for _, a := range actions {
				status := displayterminal.EntityStatus(a.Status)
				fmt.Printf("%-12s %-20s %-25s %-10s %s\n", a.Id[:8], a.Name, a.Group, status, a.Plugin)
			}
			return nil
		},
	}
	cmd.Flags().BoolP("all", "a", false, "Include suspended actions (default: hide suspended; active and error always shown)")
	return cmd
}

func newListGroupsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "groups",
		Short: "List registered groups",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := requireDaemon(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = client.Close() }()

			resp, err := client.Service.ListGroups(context.Background(), &recurv1.ListGroupsRequest{})
			if err != nil {
				return fmt.Errorf("failed to list groups: %v", err)
			}

			jsonFlag, _ := cmd.Flags().GetBool("json")
			if jsonFlag {
				return displayterminal.PrintJSON(resp.Groups)
			}

			if len(resp.Groups) == 0 {
				fmt.Println("No groups found.")
				return nil
			}

			for _, g := range resp.Groups {
				fmt.Printf("%-12s %-25s triggers=%d actions=%d\n", g.Id[:8], g.Name, g.TriggerCount, g.ActionCount)
			}
			return nil
		},
	}
}

func newListPluginsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "plugins",
		Short: "List loaded plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := requireDaemon(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = client.Close() }()

			resp, err := client.Service.ListPlugins(context.Background(), &recurv1.ListPluginsRequest{})
			if err != nil {
				return fmt.Errorf("failed to list plugins: %v", err)
			}

			jsonFlag, _ := cmd.Flags().GetBool("json")
			if jsonFlag {
				return displayterminal.PrintJSON(resp.Plugins)
			}

			if len(resp.Plugins) == 0 {
				fmt.Println("No plugins found.")
				return nil
			}

			for _, p := range resp.Plugins {
				status := displayterminal.EntityStatus(p.Status)
				fmt.Printf("%-12s %-20s %-30s %-8s %-10s triggers=%d actions=%d\n",
					p.Id[:minLen(len(p.Id), 8)], p.Name, p.Namespace, p.Version, status, p.TriggerCount, p.ActionCount)
			}
			return nil
		},
	}
}

func newListRecurfilesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "recurfiles",
		Short: "List registered recurfiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := requireDaemon(cmd)
			if err != nil {
				return err
			}
			defer func() { _ = client.Close() }()

			resp, err := client.Service.ListRecurfiles(context.Background(), &recurv1.ListRecurfilesRequest{})
			if err != nil {
				return fmt.Errorf("failed to list recurfiles: %v", err)
			}

			jsonFlag, _ := cmd.Flags().GetBool("json")
			if jsonFlag {
				return displayterminal.PrintJSON(resp.Recurfiles)
			}

			if len(resp.Recurfiles) == 0 {
				fmt.Println("No recurfiles found.")
				return nil
			}

			for _, w := range resp.Recurfiles {
				fmt.Printf("%-12s %-40s groups=%d triggers=%d actions=%d\n",
					w.Id[:minLen(len(w.Id), 8)], w.Path, w.GroupCount, w.TriggerCount, w.ActionCount)
			}
			return nil
		},
	}
}

func minLen(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// padRight pads a string to a minimum width.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// filterTriggersByStatus returns only triggers matching the given status.
func filterTriggersByStatus(triggers []*recurv1.TriggerSummary, status recurv1.EntityStatus) []*recurv1.TriggerSummary {
	var result []*recurv1.TriggerSummary
	for _, t := range triggers {
		if t.Status == status {
			result = append(result, t)
		}
	}
	return result
}

// filterActionsByStatus returns only actions matching the given status.
func filterActionsByStatus(actions []*recurv1.ActionSummary, status recurv1.EntityStatus) []*recurv1.ActionSummary {
	var result []*recurv1.ActionSummary
	for _, a := range actions {
		if a.Status == status {
			result = append(result, a)
		}
	}
	return result
}

// excludeTriggersByStatus returns triggers whose status differs from the given one.
func excludeTriggersByStatus(triggers []*recurv1.TriggerSummary, status recurv1.EntityStatus) []*recurv1.TriggerSummary {
	result := make([]*recurv1.TriggerSummary, 0, len(triggers))
	for _, t := range triggers {
		if t.Status != status {
			result = append(result, t)
		}
	}
	return result
}

// excludeActionsByStatus returns actions whose status differs from the given one.
func excludeActionsByStatus(actions []*recurv1.ActionSummary, status recurv1.EntityStatus) []*recurv1.ActionSummary {
	result := make([]*recurv1.ActionSummary, 0, len(actions))
	for _, a := range actions {
		if a.Status != status {
			result = append(result, a)
		}
	}
	return result
}
