package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/directedbits/recur/src/app/recurd/daemon"
	configyaml "github.com/directedbits/recur/src/infra/yaml/config"
	processos "github.com/directedbits/recur/src/infra/os/process"
	statejsonfile "github.com/directedbits/recur/src/infra/jsonfile/state"
	"github.com/spf13/cobra"
)

func newStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the daemon as a background process",
		RunE: func(cmd *cobra.Command, args []string) error {
			foreground, _ := cmd.Flags().GetBool("foreground")
			configFile, _ := cmd.Flags().GetString("file")
			socketAddr, _ := cmd.Flags().GetString("socket")
			logLevel, _ := cmd.Flags().GetString("log-level")

			pidPath, err := processos.PIDPath()
			if err != nil {
				return fmt.Errorf("could not determine PID path: %w", err)
			}

			// Check if already running
			running, pid, err := processos.IsRunning(pidPath)
			if err != nil {
				return fmt.Errorf("could not check daemon status: %w", err)
			}
			if running {
				return fmt.Errorf("daemon already running (pid %d)", pid)
			}

			// Clean up stale PID file if needed
			if pid != 0 {
				_ = processos.RemovePID(pidPath)
			}

			quiet, _ := cmd.Flags().GetBool("quiet")

			if foreground {
				return runForeground(configFile, pidPath)
			}
			return runBackground(configFile, socketAddr, logLevel, quiet)
		},
	}

	cmd.Flags().Bool("foreground", false, "Run in the foreground with log output to stdout")
	cmd.Flags().String("file", "", "Path to config file (default: ~/.config/recur/configyaml.yaml)")
	cmd.Flags().String("socket", "", "Daemon address: Unix socket path or TCP host:port")
	cmd.Flags().String("log-level", "", "Log level: debug, info, warn, error (overrides config)")

	return cmd
}

func runForeground(configFile string, pidPath string) error {
	configPath := strPtrOrNil(configFile)
	store, cfgPath, err := configyaml.InitStore(configPath, nil)
	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}

	statePath, err := statejsonfile.DefaultPath()
	if err != nil {
		return fmt.Errorf("could not determine state path: %w", err)
	}

	effective := store.Get()
	sockAddr := ""
	if effective.SocketAddress != nil {
		sockAddr = *effective.SocketAddress
	}

	d := daemon.New(store, pidPath, sockAddr)
	d.SetConfigPath(cfgPath)
	d.SetStatePath(statePath)
	d.SetLaunchArgs(&statejsonfile.LaunchArgs{ConfigPath: cfgPath, Foreground: true})
	return d.Run()
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// runBackground starts recurd as a background process with the given arguments.
// Empty strings are omitted from the command line.
func runBackground(configFile, socketAddr, logLevel string, quiet bool) error {
	recurdPath, err := findWatchd()
	if err != nil {
		return err
	}

	var args []string
	if configFile != "" {
		args = append(args, "--config", configFile)
	}
	if socketAddr != "" {
		args = append(args, "--socket", socketAddr)
	}
	if logLevel != "" {
		args = append(args, "--log-level", logLevel)
	}

	proc := exec.Command(recurdPath, args...)
	proc.SysProcAttr = daemonSysProcAttr()

	if err := proc.Start(); err != nil {
		return fmt.Errorf("could not start daemon: %w", err)
	}

	if !quiet {
		fmt.Printf("Daemon started (pid %d)\n", proc.Process.Pid)
	}

	detachProcess(proc)

	return nil
}

// findWatchd locates the recurd binary. It looks in the same directory as the
// current executable first, then falls back to PATH.
func findWatchd() (string, error) {
	// Try same directory as current executable
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		name := "recurd"
		if runtime.GOOS == "windows" {
			name = "recurd.exe"
		}
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	// Fall back to PATH
	path, err := exec.LookPath("recurd")
	if err != nil {
		return "", fmt.Errorf("could not find recurd binary (looked in %s and PATH)", filepath.Dir(exe))
	}
	return path, nil
}
