package cli

import (
	"fmt"
	"time"

	configyaml "github.com/directedbits/recur/src/infra/yaml/config"
	processos "github.com/directedbits/recur/src/infra/os/process"
	statejsonfile "github.com/directedbits/recur/src/infra/jsonfile/state"
	"github.com/spf13/cobra"
)

func newRestartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Stop and restart the daemon with its original launch args",
		RunE: func(cmd *cobra.Command, args []string) error {
			quiet, _ := cmd.Flags().GetBool("quiet")

			// --- Stop ---
			pidPath, err := processos.PIDPath()
			if err != nil {
				return err
			}

			running, pid, err := processos.IsRunning(pidPath)
			if err != nil {
				return fmt.Errorf("could not check daemon status: %w", err)
			}
			if !running {
				return fmt.Errorf("daemon is not running")
			}

			// Read launch args from state before stopping (daemon writes state on shutdown too,
			// but read now to fail fast if no args are persisted).
			statePath, err := statejsonfile.DefaultPath()
			if err != nil {
				return fmt.Errorf("could not determine state path: %w", err)
			}

			launchArgs, err := statejsonfile.LoadLaunchArgs(statePath)
			if err != nil {
				return fmt.Errorf("could not read launch args: %w", err)
			}
			if launchArgs == nil {
				return fmt.Errorf("no previous launch args found \u2014 use recur start")
			}
			if launchArgs.Foreground {
				return fmt.Errorf("cannot restart a foreground daemon \u2014 use recur start --foreground")
			}

			if err := processos.SendTermSignal(pid); err != nil {
				return fmt.Errorf("could not send shutdown signal: %w", err)
			}

			store, _, _ := configyaml.InitStore(nil, nil)
			timeout := 30 * time.Second
			if store != nil {
				cfg := store.Get()
				if cfg.ShutdownTimeout != nil {
					if d, err := time.ParseDuration(*cfg.ShutdownTimeout); err == nil {
						timeout = d
					}
				}
			}

			deadline := time.Now().Add(timeout)
			for time.Now().Before(deadline) {
				running, _, _ = processos.IsRunning(pidPath)
				if !running {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			if running {
				return fmt.Errorf("daemon did not stop within %s", timeout)
			}

			if !quiet {
				fmt.Println("Daemon stopped")
			}

			// --- Start with persisted args ---
			return runBackground(launchArgs.ConfigPath, launchArgs.SocketAddress, launchArgs.LogLevel, quiet)
		},
	}

	return cmd
}
