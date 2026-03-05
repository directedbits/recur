package cli

import (
	"context"
	"fmt"

	displayterminal "github.com/directedbits/recur/src/infra/terminal/display"
	recurv1 "github.com/directedbits/recur/src/infra/grpc/v1"
	"github.com/spf13/cobra"
)

func newInspectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect [entity] <id>",
		Short: "Show full details for an entity",
		Long: `Show full details for an entity. Supported entity types: trigger, action, group, plugin, recurfile.

When the entity type is omitted, the identifier is searched across all entity types.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := requireDaemon(cmd)
			if err != nil {
				return err
			}
			defer client.Close()

			identifier := resolvePathIdentifier(args[0])

			jsonFlag, _ := cmd.Flags().GetBool("json")
			verbose, _ := cmd.Flags().GetBool("verbose")

			resp, err := client.Service.InspectEntity(context.Background(), &recurv1.InspectEntityRequest{
				Identifier: identifier,
			})
			if err != nil {
				if ambErr := displayterminal.HandleAmbiguousError(err, jsonFlag); ambErr != nil {
					return ambErr
				}
				return fmt.Errorf("failed to inspect: %v", err)
			}

			return displayterminal.InspectEntityResponse(resp, jsonFlag, verbose)
		},
	}

	cmd.AddCommand(newInspectTriggerCmd())
	cmd.AddCommand(newInspectActionCmd())
	cmd.AddCommand(newInspectGroupCmd())
	cmd.AddCommand(newInspectPluginCmd())
	cmd.AddCommand(newInspectRecurfileCmd())

	return cmd
}

func inspectFlags(cmd *cobra.Command) (jsonFlag, verbose bool) {
	jsonFlag, _ = cmd.Flags().GetBool("json")
	verbose, _ = cmd.Flags().GetBool("verbose")
	return
}

func newInspectTriggerCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "trigger <id>",
		Short: "Show full trigger details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := requireDaemon(cmd)
			if err != nil {
				return err
			}
			defer client.Close()

			jsonFlag, verbose := inspectFlags(cmd)
			resp, err := client.Service.InspectEntity(context.Background(), &recurv1.InspectEntityRequest{
				Identifier: args[0],
				EntityType: "trigger",
			})
			if err != nil {
				if ambErr := displayterminal.HandleAmbiguousError(err, jsonFlag); ambErr != nil {
					return ambErr
				}
				return fmt.Errorf("failed to inspect trigger: %v", err)
			}
			return displayterminal.Trigger(resp.Trigger, jsonFlag, verbose)
		},
	}
}

func newInspectActionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "action <id>",
		Short: "Show full action details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := requireDaemon(cmd)
			if err != nil {
				return err
			}
			defer client.Close()

			jsonFlag, verbose := inspectFlags(cmd)
			resp, err := client.Service.InspectEntity(context.Background(), &recurv1.InspectEntityRequest{
				Identifier: args[0],
				EntityType: "action",
			})
			if err != nil {
				if ambErr := displayterminal.HandleAmbiguousError(err, jsonFlag); ambErr != nil {
					return ambErr
				}
				return fmt.Errorf("failed to inspect action: %v", err)
			}
			return displayterminal.Action(resp.Action, jsonFlag, verbose)
		},
	}
}

func newInspectGroupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "group <id>",
		Short: "Show full merged group configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := requireDaemon(cmd)
			if err != nil {
				return err
			}
			defer client.Close()

			jsonFlag, _ := inspectFlags(cmd)
			resp, err := client.Service.InspectEntity(context.Background(), &recurv1.InspectEntityRequest{
				Identifier: args[0],
				EntityType: "group",
			})
			if err != nil {
				if ambErr := displayterminal.HandleAmbiguousError(err, jsonFlag); ambErr != nil {
					return ambErr
				}
				return fmt.Errorf("failed to inspect group: %v", err)
			}
			return displayterminal.Group(resp.Group, jsonFlag)
		},
	}
}

func newInspectPluginCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "plugin <id>",
		Short: "Show plugin manifest and status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := requireDaemon(cmd)
			if err != nil {
				return err
			}
			defer client.Close()

			jsonFlag, verbose := inspectFlags(cmd)
			resp, err := client.Service.InspectEntity(context.Background(), &recurv1.InspectEntityRequest{
				Identifier: args[0],
				EntityType: "plugin",
			})
			if err != nil {
				if ambErr := displayterminal.HandleAmbiguousError(err, jsonFlag); ambErr != nil {
					return ambErr
				}
				return fmt.Errorf("failed to inspect plugin: %v", err)
			}
			return displayterminal.Plugin(resp.Plugin, jsonFlag, verbose)
		},
	}
}

func newInspectRecurfileCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "recurfile <id|path>",
		Short: "Show recurfile details and associated entities",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := requireDaemon(cmd)
			if err != nil {
				return err
			}
			defer client.Close()

			jsonFlag, _ := inspectFlags(cmd)
			identifier := resolvePathIdentifier(args[0])

			resp, err := client.Service.InspectEntity(context.Background(), &recurv1.InspectEntityRequest{
				Identifier: identifier,
				EntityType: "recurfile",
			})
			if err != nil {
				if ambErr := displayterminal.HandleAmbiguousError(err, jsonFlag); ambErr != nil {
					return ambErr
				}
				return fmt.Errorf("failed to inspect recurfile: %v", err)
			}
			return displayterminal.Recurfile(resp.Recurfile, jsonFlag)
		},
	}
}
