package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	configyaml "github.com/directedbits/recur/src/infra/yaml/config"
	recurv1 "github.com/directedbits/recur/src/infra/grpc/v1"
	pluginfs "github.com/directedbits/recur/src/infra/fs/plugin"
	processos "github.com/directedbits/recur/src/infra/os/process"
	statejsonfile "github.com/directedbits/recur/src/infra/jsonfile/state"
	"github.com/spf13/cobra"
)

// ErrDaemonNotRunning is returned by status when the daemon is not running.
// The root command can check for this to set exit code 1.
var ErrDaemonNotRunning = errors.New("daemon is not running")

type statusMessage struct {
	Importance string
	Message    string
}

// statusJSON is the typed payload emitted by `recur status --json`. Stable
// shape suitable for scripting. Pointer fields are omitted from the JSON
// when nil so a "not running" payload is concise.
type statusJSON struct {
	Running     bool             `json:"running"`
	PID         *int32           `json:"pid,omitempty"`
	Uptime      string           `json:"uptime,omitempty"`
	Version     string           `json:"version,omitempty"`
	Triggers    *statusCountsRow `json:"triggers,omitempty"`
	Actions     *statusCountsRow `json:"actions,omitempty"`
	Plugins     *int32           `json:"plugins,omitempty"`
	Recurfiles  *int32           `json:"recurfiles,omitempty"`
	ConfigPath  string           `json:"config_path,omitempty"`
	SocketPath  string           `json:"socket_path,omitempty"`
	StatePath   string           `json:"state_path,omitempty"`
	PIDPath     string           `json:"pid_path,omitempty"`
	PluginsDir  string           `json:"plugins_dir,omitempty"`
	LogLevel    string           `json:"log_level,omitempty"`
	Foreground  bool             `json:"foreground,omitempty"`
	Warnings    []string         `json:"warnings,omitempty"`
}

type statusCountsRow struct {
	Active    int32 `json:"active"`
	Suspended int32 `json:"suspended"`
}

// statusEntry holds a key-value pair for status output.
type statusEntry struct {
	Key   string
	Value any
}

type statusSection struct {
	Name    string
	Entries []statusEntry
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon health and active counts",
		RunE: func(command *cobra.Command, args []string) error {
			jsonFlag, _ := command.Flags().GetBool("json")
			verbose, _ := command.Flags().GetBool("verbose")

			pidPath, err := processos.PIDPath()
			if err != nil {
				return err
			}

			running, pid, err := processos.IsRunning(pidPath)
			if err != nil {
				return fmt.Errorf("could not check daemon status: %w", err)
			}

			// JSON path: build the typed payload directly and bypass the
			// row-based text formatting entirely. Scripts get a stable
			// shape with running:bool/pid:int/counts:int.
			if jsonFlag {
				payload := buildStatusJSON(command, running, pid, verbose)
				data, _ := json.MarshalIndent(payload, "", "  ")
				fmt.Println(string(data))
				if !running {
					command.SilenceErrors = true
					return ErrDaemonNotRunning
				}
				return nil
			}

			messages := []statusMessage{}
			headerEntries := statusSection{Name: "Header", Entries: []statusEntry{}}
			launchArgEntries := statusSection{Name: "Launch args", Entries: []statusEntry{}}
			statusEntries := []statusEntry{}

			var failure error = nil
			if !running {
				headerEntries.Entries = append(headerEntries.Entries, statusEntry{"Daemon", "not running"})

				command.SilenceErrors = true
				failure = ErrDaemonNotRunning
			} else {
				daemonMessages, daemonHeaders, daemonLaunchArgs, daemonStatus := getRunningDaemonStatus(command)

				if len(daemonHeaders.Entries) == 0 {
					// Fallback: PID-only info
					headerEntries.Entries = append(headerEntries.Entries, statusEntry{"Daemon", fmt.Sprintf("running (pid %d)", pid)})
				} else {
					messages = append(messages, daemonMessages...)
					headerEntries.Entries = append(headerEntries.Entries, daemonHeaders.Entries...)
					launchArgEntries.Entries = append(launchArgEntries.Entries, daemonLaunchArgs.Entries...)
					statusEntries = append(statusEntries, daemonStatus...)
				}
			}

			if verbose {
				statusEntries = append(statusEntries, verbosePathEntries()...)
			}

			printStatusReport(messages, mergeEntries(headerEntries, launchArgEntries, statusEntries))

			return failure
		},
	}
}

// mergeEntries flattens the three sections into a single slice for the
// text formatter. Used now that the JSON path lives in newStatusCmd
// directly and printStatus is no longer the splitter.
func mergeEntries(header, launchArgs statusSection, status []statusEntry) []statusEntry {
	out := make([]statusEntry, 0, len(header.Entries)+len(launchArgs.Entries)+len(status))
	out = append(out, header.Entries...)
	out = append(out, launchArgs.Entries...)
	out = append(out, status...)
	return out
}

// buildStatusJSON assembles the typed --json payload. When the daemon is
// not running it returns {running: false} plus any path entries the user
// asked for via --verbose; when running, it queries the daemon via gRPC
// for full counts and launch args.
func buildStatusJSON(command *cobra.Command, running bool, pid int, verbose bool) *statusJSON {
	out := &statusJSON{Running: running}

	if verbose {
		if p, err := configyaml.DefaultPath(); err == nil {
			out.ConfigPath = p
		}
		if p, err := statejsonfile.DefaultPath(); err == nil {
			out.StatePath = p
		}
		if p, err := processos.DefaultSocketPath(); err == nil {
			out.SocketPath = p
		}
		if p, err := processos.PIDPath(); err == nil {
			out.PIDPath = p
		}
		if p, err := pluginfs.PluginsDir(); err == nil {
			out.PluginsDir = p
		}
	}

	if !running {
		return out
	}

	pid32 := int32(pid)
	out.PID = &pid32

	socketPath, _ := resolveSocketPath(command)
	client := connectOrNilFunc(socketPath)
	if client == nil {
		out.Warnings = append(out.Warnings, "could not connect to daemon socket")
		return out
	}
	defer client.Close()

	resp, err := client.Service.GetStatus(context.Background(), &recurv1.GetStatusRequest{})
	if err != nil {
		out.Warnings = append(out.Warnings, fmt.Sprintf("could not reach daemon via gRPC: %v", err))
		return out
	}

	if resp.Pid != 0 {
		p := resp.Pid
		out.PID = &p
	}
	out.Uptime = resp.Uptime
	out.Version = resp.Version
	out.Triggers = &statusCountsRow{Active: resp.ActiveTriggers, Suspended: resp.SuspendedTriggers}
	out.Actions = &statusCountsRow{Active: resp.ActiveActions, Suspended: resp.SuspendedActions}
	plg := resp.RegisteredPlugins
	out.Plugins = &plg
	wf := resp.RegisteredRecurfiles
	out.Recurfiles = &wf

	if resp.LaunchArgs != nil {
		out.LogLevel = resp.LaunchArgs.LogLevel
		out.Foreground = resp.LaunchArgs.Foreground
		if resp.LaunchArgs.ConfigPath != "" {
			out.ConfigPath = resp.LaunchArgs.ConfigPath
		}
		if resp.LaunchArgs.SocketAddress != "" {
			out.SocketPath = resp.LaunchArgs.SocketAddress
		}
	}

	if resp.Version != "" && resp.Version != Version {
		out.Warnings = append(out.Warnings, fmt.Sprintf("daemon version %q does not match CLI version %q", resp.Version, Version))
	}

	return out
}

func getRunningDaemonStatus(command *cobra.Command) (messages []statusMessage, headers statusSection, launchArgs statusSection, status []statusEntry) {
	messages = []statusMessage{}
	headers = statusSection{Name: "Header", Entries: []statusEntry{}}
	launchArgs = statusSection{Name: "Launch args", Entries: []statusEntry{}}
	status = []statusEntry{}

	socketPath, _ := resolveSocketPath(command)
	client := connectOrNilFunc(socketPath)

	if client == nil {
		daemonConnectionMessage := "Warning: could not connect to daemon socket (limited info available)"
		messages = append(messages, statusMessage{"warning", daemonConnectionMessage})
		return
	}

	defer client.Close()
	response, err := client.Service.GetStatus(context.Background(), &recurv1.GetStatusRequest{})

	if err != nil {
		daemonUnreachableMessage := fmt.Sprintf("Warning: could not reach daemon via gRPC: %v\n", err)
		messages = append(messages, statusMessage{"warning", daemonUnreachableMessage})
		return
	}

	headers, launchArgs, status = parseStatus(response)

	if response.Version != "" && response.Version != Version {
		versionMismatchMessage := fmt.Sprintf("Warning: daemon version %q does not match CLI version %q\n", response.Version, Version)
		messages = append(messages, statusMessage{"warning", versionMismatchMessage})
	}

	return
}

// parseStatus builds status entries from a gRPC GetStatusResponse.
func parseStatus(response *recurv1.GetStatusResponse) (headers statusSection, launchArgs statusSection, status []statusEntry) {
	headers = statusSection{Name: "Header", Entries: []statusEntry{}}
	launchArgs = statusSection{Name: "Launch args", Entries: []statusEntry{}}
	status = []statusEntry{}

	status = append(status,
		statusEntry{"Daemon", fmt.Sprintf("running (pid %d)", response.Pid)},
		statusEntry{"Uptime", response.Uptime},
	)

	if response.Version != "" {
		status = append(status, statusEntry{"Version", response.Version})
	}

	// Launch args — resolve defaults for empty values
	configPath := ""
	socketPath := ""
	logLevel := "info"
	foreground := false
	if args := response.LaunchArgs; args != nil {
		configPath = args.ConfigPath
		socketPath = args.SocketAddress
		if args.LogLevel != "" {
			logLevel = args.LogLevel
			launchArgs.Entries = append(launchArgs.Entries, statusEntry{"Log Level", logLevel})
		}
		foreground = args.Foreground

		if configPath != "" {
			launchArgs.Entries = append(launchArgs.Entries, statusEntry{"Config", configPath})
		}
		if socketPath != "" {
			launchArgs.Entries = append(launchArgs.Entries, statusEntry{"Socket", socketPath})
		}
		if foreground {
			launchArgs.Entries = append(launchArgs.Entries, statusEntry{"Mode", "foreground"})
		}
	}

	if configPath == "" {
		if path, err := configyaml.DefaultPath(); err == nil {
			configPath = path
		}
	}
	if socketPath == "" {
		if path, err := processos.DefaultSocketPath(); err == nil {
			socketPath = path
		}
	}
	status = append(status,
		statusEntry{"Config", configPath},
		statusEntry{"Socket", socketPath},
		statusEntry{"Log Level", logLevel},
	)

	status = append(status,
		statusEntry{"Triggers", fmt.Sprintf("%d active, %d suspended", response.ActiveTriggers, response.SuspendedTriggers)},
		statusEntry{"Actions", fmt.Sprintf("%d active, %d suspended", response.ActiveActions, response.SuspendedActions)},
		statusEntry{"Plugins", fmt.Sprintf("%d registered", response.RegisteredPlugins)},
		statusEntry{"Recurfiles", fmt.Sprintf("%d registered", response.RegisteredRecurfiles)},
	)

	return
}

// verbosePathEntries returns status entries for file system paths.
func verbosePathEntries() []statusEntry {
	var entries []statusEntry
	if p, err := configyaml.DefaultPath(); err == nil {
		entries = append(entries, statusEntry{"Config Path", p})
	}
	if p, err := statejsonfile.DefaultPath(); err == nil {
		entries = append(entries, statusEntry{"State Path", p})
	}
	if p, err := processos.DefaultSocketPath(); err == nil {
		entries = append(entries, statusEntry{"Socket Path", p})
	}
	if p, err := processos.PIDPath(); err == nil {
		entries = append(entries, statusEntry{"PID Path", p})
	}
	if p, err := pluginfs.PluginsDir(); err == nil {
		entries = append(entries, statusEntry{"Plugins Dir", p})
	}
	return entries
}

func printStatusReport(messages []statusMessage, entries []statusEntry) {
	for _, message := range messages {
		switch message.Importance {
		case "warning", "error":
			fmt.Fprintln(os.Stderr, message.Message)
		default:
			_, _ = fmt.Fprintln(os.Stdout, message.Message)
		}
	}

	// Find max key length for alignment
	maxLen := 0
	for _, entry := range entries {
		if len(entry.Key) > maxLen {
			maxLen = len(entry.Key)
		}
	}

	for _, entry := range entries {
		padding := strings.Repeat(" ", maxLen-len(entry.Key))
		fmt.Printf("%s:%s %v\n", entry.Key, padding, entry.Value)
	}
}

// resolveSocketPath returns the daemon address from (in priority order):
// 1. --socket CLI flag
// 2. socket_address in config file
// 3. Platform default (Unix socket on Linux, TCP on Windows)
func resolveSocketPath(cmd *cobra.Command) (string, error) {
	socketPath, _ := cmd.Flags().GetString("socket")
	if socketPath != "" {
		return socketPath, nil
	}

	if store, _, err := configyaml.InitStore(nil, nil); err == nil {
		cfg := store.Get()
		if cfg.SocketAddress != nil && *cfg.SocketAddress != "" {
			return *cfg.SocketAddress, nil
		}
	}

	return processos.DefaultSocketPath()
}
