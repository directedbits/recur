package cli

import (
	"context"
	"fmt"
	"strings"

	pkgconfig "github.com/directedbits/recur/pkg/config"
	
	configyaml "github.com/directedbits/recur/src/infra/yaml/config"
	recurv1 "github.com/directedbits/recur/src/infra/grpc/v1"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage daemon configuration",
	}

	cmd.AddCommand(newConfigGetCmd())
	cmd.AddCommand(newConfigSetCmd())
	cmd.AddCommand(newConfigDeleteCmd())

	return cmd
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get [key]",
		Short: "Show configuration values",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonFlag, _ := cmd.Flags().GetBool("json")
			verbose, _ := cmd.Flags().GetBool("verbose")

			store, _, err := configyaml.InitStore(nil, nil)
			if err != nil {
				return err
			}

			populateCLIArgsFromDaemon(cmd, store)

			if len(args) == 0 {
				return printAllConfig(store, jsonFlag, verbose)
			}

			val, err := configyaml.GetByKey(store, args[0])
			if err != nil {
				return err
			}

			if jsonFlag {
				s, err := configyaml.ToJSON(val)
				if err != nil {
					return err
				}
				fmt.Println(s)
			} else {
				suffix := ""
				if verbose {
					suffix = storeSourceAnnotation(args[0], store)
				}
				fmt.Printf("%s = %s%s\n", args[0], formatValue(val), suffix)
			}
			return nil
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			quiet, _ := cmd.Flags().GetBool("quiet")

			// If daemon is running, go through gRPC so its Store stays in sync
			socketPath, _ := resolveSocketPath(cmd)
			if client := connectOrNilFunc(socketPath); client != nil {
				defer func() { _ = client.Close() }()
				_, err := client.Service.SetConfig(context.Background(), &recurv1.SetConfigRequest{
					Key:   args[0],
					Value: args[1],
				})
				if err != nil {
					return fmt.Errorf("config set failed: %v", err)
				}
				if !quiet {
					fmt.Printf("%s = %s\n", args[0], args[1])
				}
				return nil
			}

			// Daemon not running — write directly to file
			store, cfgPath, err := configyaml.InitStore(nil, nil)
			if err != nil {
				return err
			}

			if err := configyaml.SetByKey(store, "file", args[0], args[1]); err != nil {
				return err
			}

			fileLayer, _ := store.GetLayer("file")
			if err := configyaml.Save(&fileLayer, cfgPath); err != nil {
				return err
			}

			if !quiet {
				fmt.Printf("%s = %s\n", args[0], args[1])
			}
			return nil
		},
	}
}

func newConfigDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <key>",
		Short: "Remove a config key (plugin keys are deleted; daemon keys revert to their default)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			quiet, _ := cmd.Flags().GetBool("quiet")

			// If daemon is running, go through gRPC so its Store stays in sync
			socketPath, _ := resolveSocketPath(cmd)
			if client := connectOrNilFunc(socketPath); client != nil {
				defer func() { _ = client.Close() }()
				_, err := client.Service.DeleteConfig(context.Background(), &recurv1.DeleteConfigRequest{
					Key: args[0],
				})
				if err != nil {
					return fmt.Errorf("config delete failed: %v", err)
				}
				if !quiet {
					fmt.Println(configDeleteMessage(args[0], nil))
				}
				return nil
			}

			// Daemon not running — write directly to file
			store, cfgPath, err := configyaml.InitStore(nil, nil)
			if err != nil {
				return err
			}

			if err := configyaml.DeleteByKey(store, "file", args[0]); err != nil {
				return err
			}

			fileLayer, _ := store.GetLayer("file")
			if err := configyaml.Save(&fileLayer, cfgPath); err != nil {
				return err
			}

			if !quiet {
				val, _ := configyaml.GetByKey(store, args[0])
				fmt.Println(configDeleteMessage(args[0], val))
			}
			return nil
		},
	}
}

// configDeleteMessage renders the success message for a config delete.
// Plugin keys (plugins.<ns>.<field>) have no daemon default to revert to —
// they're user-set per-namespace — so we report them as "deleted". Daemon
// keys revert to their DefaultConfig() value, so we report them as
// "reverted to default" (with the effective value when known).
func configDeleteMessage(key string, effective any) string {
	if strings.HasPrefix(key, "plugins.") {
		return fmt.Sprintf("%s deleted", key)
	}
	if effective != nil {
		return fmt.Sprintf("%s reverted to default (%v)", key, effective)
	}
	return fmt.Sprintf("%s reverted to default", key)
}

// populateCLIArgsFromDaemon tries to connect to the daemon and, if running,
// populates the "cli args" layer in the store from the daemon's LaunchArgs.
func populateCLIArgsFromDaemon(cmd *cobra.Command, store *pkgconfig.Store[configyaml.Config]) {
	socketPath, _ := resolveSocketPath(cmd)
	client := connectOrNilFunc(socketPath)
	if client == nil {
		return
	}
	defer func() { _ = client.Close() }()

	resp, err := client.Service.GetStatus(context.Background(), &recurv1.GetStatusRequest{})
	if err != nil || resp.LaunchArgs == nil {
		return
	}

	la := resp.LaunchArgs
	cliArgs := configyaml.Config{}
	hasOverrides := false

	if la.LogLevel != "" {
		ll := la.LogLevel
		cliArgs.LogLevel = &ll
		hasOverrides = true
	}
	if la.SocketAddress != "" {
		sa := la.SocketAddress
		cliArgs.SocketAddress = &sa
		hasOverrides = true
	}

	if hasOverrides {
		_ = store.Set("cli args", cliArgs)
	}
}

// storeSourceAnnotation returns a source suffix using the config store's Inspect.
func storeSourceAnnotation(key string, store *pkgconfig.Store[configyaml.Config]) string {
	if store == nil {
		return ""
	}

	if strings.HasPrefix(key, "plugins.") {
		return ""
	}

	def, err := configyaml.LookupKey(key)
	if err != nil || def.Field == "" {
		return ""
	}

	layers := store.Inspect(def.Field)
	for _, entry := range layers {
		if entry.Defined {
			switch entry.Layer {
			case "cli args":
				return " (set via CLI flag)"
			case "file":
				return " (set in configyaml.yaml)"
			case "default":
				return " (inherited from default)"
			default:
				return fmt.Sprintf(" (from %s)", entry.Layer)
			}
		}
	}

	if def.Fallback != "" {
		return fmt.Sprintf(" (inherited from %s)", def.Fallback)
	}
	return " (inherited from default)"
}

// printAllConfig prints all config using the store for annotations.
func printAllConfig(store *pkgconfig.Store[configyaml.Config], jsonFlag, verbose bool) error {
	all := configyaml.AllKeys(store)

	if jsonFlag {
		m := make(map[string]any, len(all))
		for _, kv := range all {
			m[kv.Key] = kv.Value
		}
		s, err := configyaml.ToJSON(m)
		if err != nil {
			return err
		}
		fmt.Println(s)
		return nil
	}

	maxLen := 0
	for _, kv := range all {
		if len(kv.Key) > maxLen {
			maxLen = len(kv.Key)
		}
	}

	for _, kv := range all {
		padding := strings.Repeat(" ", maxLen-len(kv.Key))
		suffix := ""
		if verbose {
			suffix = storeSourceAnnotation(kv.Key, store)
		}
		fmt.Printf("%s%s = %s%s\n", kv.Key, padding, formatValue(kv.Value), suffix)
	}
	return nil
}

func formatValue(v any) string {
	if v == nil {
		return "(not set)"
	}
	return fmt.Sprintf("%v", v)
}
