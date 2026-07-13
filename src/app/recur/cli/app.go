package cli

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	appbundle "github.com/directedbits/recur/src/infra/fs/appbundle"
	pluginfs "github.com/directedbits/recur/src/infra/fs/plugin"
	recurv1 "github.com/directedbits/recur/src/infra/grpc/recur/v1"
	displayterminal "github.com/directedbits/recur/src/infra/terminal/display"
	configyaml "github.com/directedbits/recur/src/infra/yaml/config"
	"github.com/spf13/cobra"
)

func newAppCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "app",
		Short: "Install and manage app bundles (.recur)",
		Long: `Install and manage app bundles.

An app bundle is a .recur zip archive containing a recurfile (the single YAML
file at its root) plus any local scripts the app needs. Installing unpacks the
bundle into ~/.config/recur/app/<name>/ and registers its recurfile with the
daemon. Apps installed while the daemon is stopped are registered automatically
the next time it starts.`,
	}
	cmd.AddCommand(newAppInstallCmd())
	cmd.AddCommand(newAppListCmd())
	cmd.AddCommand(newAppRemoveCmd())
	cmd.AddCommand(newAppPackCmd())
	return cmd
}

func newAppPackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pack <dir>",
		Short: "Create a .recur bundle from a directory",
		Long:  "Create a .recur app bundle from a directory. The directory must contain a recurfile (a single YAML file) at its root.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			srcDir := args[0]
			out, _ := cmd.Flags().GetString("output")
			if out == "" {
				abs, err := filepath.Abs(strings.TrimRight(srcDir, `/\`))
				if err != nil {
					return err
				}
				out = filepath.Base(abs) + appbundle.Ext
			}
			if err := appbundle.Pack(srcDir, out); err != nil {
				return fmt.Errorf("pack failed: %w", err)
			}
			if quiet, _ := cmd.Flags().GetBool("quiet"); !quiet {
				fmt.Printf("Packed %s -> %s\n", srcDir, out)
			}
			return nil
		},
	}
	cmd.Flags().StringP("output", "o", "", "Output bundle path (default: <dir>.recur)")
	return cmd
}

func newAppInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <bundle.recur | URL>",
		Short: "Install an app bundle and register its recurfile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			source := args[0]
			nameFlag, _ := cmd.Flags().GetString("name")
			force, _ := cmd.Flags().GetBool("force")
			quiet, _ := cmd.Flags().GetBool("quiet")

			// 0. Resolve the source, downloading first if it is a URL.
			bundlePath, cleanup, err := resolveBundleSource(source)
			if err != nil {
				return fmt.Errorf("install failed: %w", err)
			}
			defer cleanup()

			base, err := configyaml.AppDir()
			if err != nil {
				return err
			}

			// Unpack into a staging dir first so the recurfile (and thus the
			// default name) is known before we touch the destination, and so a
			// bad bundle never clobbers an installed app.
			staging, err := os.MkdirTemp(base, ".staging-*")
			if err != nil {
				return err
			}
			defer func() { _ = os.RemoveAll(staging) }()

			if err := appbundle.Unpack(bundlePath, staging); err != nil {
				return fmt.Errorf("install failed: %w", err)
			}

			recurfilePath, err := appbundle.FindRecurfile(staging)
			if err != nil {
				return fmt.Errorf("install failed: %w", err)
			}

			name := nameFlag
			if name == "" {
				name = defaultAppName(recurfilePath, source)
			}
			if err := validateAppName(name); err != nil {
				return err
			}

			dest := filepath.Join(base, name)
			if _, err := os.Stat(dest); err == nil {
				if !force && !confirmOverwrite(cmd, name) {
					fmt.Println("Aborted.")
					return nil
				}
				if err := os.RemoveAll(dest); err != nil {
					return fmt.Errorf("could not replace existing app %q: %w", name, err)
				}
			}
			if err := os.Rename(staging, dest); err != nil {
				return fmt.Errorf("could not install app %q: %w", name, err)
			}
			installedRecurfile := filepath.Join(dest, filepath.Base(recurfilePath))

			// Register with the daemon if it is running; otherwise the startup
			// scan will pick the app up on the next start.
			socketPath, _ := resolveSocketPath(cmd)
			client, err := connectFunc(socketPath)
			if err != nil {
				if !quiet {
					fmt.Printf("Installed app %q to %s\n", name, dest)
					fmt.Println("Hint: daemon is not running. The app will be registered on next start.")
				}
				return nil
			}
			defer func() { _ = client.Close() }()

			resp, err := client.Service.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{
				Path: installedRecurfile,
			})
			if err != nil {
				return fmt.Errorf("app installed to %s but registration failed: %v", dest, err)
			}

			if quiet {
				return nil
			}
			jsonFlag, _ := cmd.Flags().GetBool("json")
			if jsonFlag {
				return displayterminal.PrintJSON(map[string]any{
					"name":          name,
					"id":            resp.Id,
					"path":          dest,
					"trigger_count": resp.TriggerCount,
					"action_count":  resp.ActionCount,
				})
			}
			fmt.Printf("Installed app %q (id: %s)\n", name, resp.Id)
			fmt.Printf("  Path:     %s\n", dest)
			fmt.Printf("  Triggers: %d\n", resp.TriggerCount)
			fmt.Printf("  Actions:  %d\n", resp.ActionCount)
			return nil
		},
	}
	cmd.Flags().String("name", "", "Install under this app name (default: recurfile filename, then bundle filename)")
	cmd.Flags().Bool("force", false, "Overwrite an existing app without prompting")
	return cmd
}

func newAppListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed apps",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			base, err := configyaml.AppDir()
			if err != nil {
				return err
			}
			entries, err := os.ReadDir(base)
			if err != nil {
				return err
			}

			// Registered recurfile paths, if the daemon is up.
			registered := map[string]bool{}
			socketPath, _ := resolveSocketPath(cmd)
			if client := connectOrNilFunc(socketPath); client != nil {
				defer func() { _ = client.Close() }()
				if resp, err := client.Service.ListRecurfiles(context.Background(), &recurv1.ListRecurfilesRequest{}); err == nil {
					for _, w := range resp.Recurfiles {
						registered[filepath.Clean(w.Path)] = true
					}
				}
			}

			type appInfo struct {
				Name       string `json:"name"`
				Path       string `json:"path"`
				Registered bool   `json:"registered"`
			}
			var apps []appInfo
			for _, e := range entries {
				if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
					continue
				}
				dir := filepath.Join(base, e.Name())
				rf, err := appbundle.FindRecurfile(dir)
				if err != nil {
					continue
				}
				apps = append(apps, appInfo{Name: e.Name(), Path: dir, Registered: registered[filepath.Clean(rf)]})
			}

			jsonFlag, _ := cmd.Flags().GetBool("json")
			if jsonFlag {
				return displayterminal.PrintJSON(apps)
			}
			if len(apps) == 0 {
				fmt.Println("No apps installed.")
				return nil
			}
			for _, a := range apps {
				status := "not registered"
				if a.Registered {
					status = "registered"
				}
				fmt.Printf("%-20s %-14s %s\n", a.Name, status, a.Path)
			}
			return nil
		},
	}
}

func newAppRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Deregister and remove an installed app",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if err := validateAppName(name); err != nil {
				return err
			}
			base, err := configyaml.AppDir()
			if err != nil {
				return err
			}
			dir := filepath.Join(base, name)
			if _, err := os.Stat(dir); err != nil {
				return fmt.Errorf("app %q is not installed", name)
			}

			// Best-effort deregister if the daemon is running and the app has a
			// recurfile; removal proceeds regardless.
			if rf, err := appbundle.FindRecurfile(dir); err == nil {
				socketPath, _ := resolveSocketPath(cmd)
				if client := connectOrNilFunc(socketPath); client != nil {
					defer func() { _ = client.Close() }()
					_, _ = client.Service.DeregisterRecurfile(context.Background(), &recurv1.DeregisterRecurfileRequest{
						Identifier: rf,
					})
				}
			}

			if err := os.RemoveAll(dir); err != nil {
				return fmt.Errorf("could not remove app %q: %w", name, err)
			}
			if quiet, _ := cmd.Flags().GetBool("quiet"); !quiet {
				fmt.Printf("Removed app %q\n", name)
			}
			return nil
		},
	}
}

// resolveBundleSource returns a local path to the bundle, downloading it first
// when source is an allowed URL. The returned cleanup removes any temp file.
func resolveBundleSource(source string) (path string, cleanup func(), err error) {
	cleanup = func() {}
	if pluginfs.IsURL(source) {
		host, err := pluginfs.HostFromURL(source)
		if err != nil {
			return "", cleanup, err
		}
		store, _, _ := configyaml.InitStore(nil, nil)
		effective := store.Get()
		if !effective.IsHostAllowed(host) {
			return "", cleanup, fmt.Errorf("host %q is not in allowed_hosts (configure with: recur config set allowed_hosts %q)", host, host)
		}
		fmt.Printf("Downloading from %s...\n", host)
		tmp, err := pluginfs.Download(source)
		if err != nil {
			return "", cleanup, err
		}
		return tmp, func() { _ = os.Remove(tmp) }, nil
	}

	abs, err := filepath.Abs(source)
	if err != nil {
		abs = source
	}
	if _, err := os.Stat(abs); err != nil {
		return "", cleanup, fmt.Errorf("bundle not found: %s", source)
	}
	return abs, cleanup, nil
}

// defaultAppName derives the app name from the recurfile's filename stem,
// falling back to the bundle's filename stem when the recurfile uses the generic
// recurfile.* convention (whose stem carries no meaning).
func defaultAppName(recurfilePath, source string) string {
	stem := strings.TrimSuffix(filepath.Base(recurfilePath), filepath.Ext(recurfilePath))
	if !strings.EqualFold(stem, "recurfile") {
		return stem
	}
	return bundleStem(source)
}

// bundleStem returns the filename stem of a bundle source (URL or local path),
// stripping the .recur extension.
func bundleStem(source string) string {
	base := source
	if pluginfs.IsURL(source) {
		if u, err := url.Parse(source); err == nil {
			base = u.Path
		}
	}
	base = filepath.Base(base)
	base = strings.TrimSuffix(base, appbundle.Ext)
	return base
}

// validateAppName rejects names that are unusable as a single directory segment.
func validateAppName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("invalid app name %q (pass a valid name with --name)", name)
	}
	return nil
}

// confirmOverwrite prompts the user to overwrite an existing app. A non-y answer
// (including a closed/non-interactive stdin) aborts.
func confirmOverwrite(cmd *cobra.Command, name string) bool {
	fmt.Printf("App %q already exists. Overwrite? [y/N]: ", name)
	line, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes"
}
