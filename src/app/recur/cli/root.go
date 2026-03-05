// Package cli provides the CLI command definitions and Cobra setup.
package cli

import (
	"github.com/spf13/cobra"
)

// Version is the CLI version. Bump alongside releases.
var Version = "0.1.0-alpha"

// NewRootCmd creates the root `recur` command.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "recur",
		Short: "Recur — declarative triggered actions via YAML",
		Long:  "Recur is a daemon + CLI tool for declaratively configuring triggered actions driven by local YAML files (Recurfiles).",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Suppress usage output for runtime errors (e.g. gRPC failures).
			// This runs after arg/flag validation, so bad input still shows usage.
			cmd.SilenceUsage = true
		},
	}

	// Global flags
	root.PersistentFlags().BoolP("json", "j", false, "Output in JSON format")
	root.PersistentFlags().BoolP("quiet", "q", false, "Suppress non-essential output")
	root.PersistentFlags().BoolP("verbose", "v", false, "Show debug-level detail")
	root.PersistentFlags().StringP("socket", "s", "", "Override daemon socket path")
	root.PersistentFlags().BoolP("yes", "y", false, "Skip confirmation prompts")

	// Subcommands
	root.AddCommand(newVersionCmd())
	root.AddCommand(newStartCmd())
	root.AddCommand(newStopCmd())
	root.AddCommand(newRestartCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newRegisterCmd())
	root.AddCommand(newVerifyCmd())
	root.AddCommand(newDeregisterCmd())
	root.AddCommand(newListCmd())
	root.AddCommand(newInspectCmd())
	root.AddCommand(newSuspendCmd())
	root.AddCommand(newResumeCmd())
	root.AddCommand(newTestCmd())
	root.AddCommand(newAddCmd())
	root.AddCommand(newInstallCmd())
	root.AddCommand(newUninstallCmd())

	return root
}
