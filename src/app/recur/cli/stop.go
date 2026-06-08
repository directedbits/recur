package cli

import (
	"fmt"
	"time"

	configyaml "github.com/directedbits/recur/src/infra/yaml/config"
	processos "github.com/directedbits/recur/src/infra/os/process"
	"github.com/spf13/cobra"
)

func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Graceful daemon shutdown",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			if err := processos.SendTermSignal(pid); err != nil {
				return fmt.Errorf("could not send shutdown signal: %w", err)
			}

			// Load config for shutdown timeout
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

			// Wait for process to exit
			deadline := time.Now().Add(timeout)
			for time.Now().Before(deadline) {
				running, _, _ = processos.IsRunning(pidPath)
				if !running {
					quiet, _ := cmd.Flags().GetBool("quiet")
					if !quiet {
						fmt.Println("Daemon stopped")
					}
					return nil
				}
				time.Sleep(100 * time.Millisecond)
			}

			return fmt.Errorf("daemon did not stop within %s", timeout)
		},
	}
}
