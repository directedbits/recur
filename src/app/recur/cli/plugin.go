package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	configyaml "github.com/directedbits/recur/src/infra/yaml/config"
	displayterminal "github.com/directedbits/recur/src/infra/terminal/display"
	recurv1 "github.com/directedbits/recur/src/infra/grpc/v1"
	pluginfs "github.com/directedbits/recur/src/infra/fs/plugin"
	"github.com/spf13/cobra"
)

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install <path|url>",
		Short: "Install a plugin from a directory, archive, or URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			link, _ := cmd.Flags().GetBool("link")
			source := args[0]

			srcDir, err := resolvePluginSource(source, link)
			if err != nil {
				return fmt.Errorf("install failed: %v", err)
			}

			// Install to ~/.watch/plugins/ (copy or symlink)
			installed, err := pluginfs.Install(srcDir, link)
			if err != nil {
				return fmt.Errorf("install failed: %v", err)
			}

			// Check for trigger/action name conflicts
			allPlugins, _ := pluginfs.Discover()
			for _, w := range pluginfs.CheckConflicts(allPlugins) {
				fmt.Printf("Warning: %s\n", w)
			}

			// Register with daemon if running
			client, err := requireDaemon(cmd)
			if err != nil {
				// Daemon not running — plugin is on disk, will be discovered on next start
				quiet, _ := cmd.Flags().GetBool("quiet")
				if !quiet {
					mode := installMode(link, source)
					fmt.Printf("Installed (%s): %s (%s) v%s\n", mode, installed.Manifest.Name, installed.Manifest.Namespace, installed.Manifest.Version)
					fmt.Println("Hint: daemon is not running. Plugin will be loaded on next start.")
				}
				return nil
			}
			defer client.Close()

			resp, err := client.Service.InstallPlugin(context.Background(), &recurv1.InstallPluginRequest{
				Path: installed.Dir,
			})
			if err != nil {
				return fmt.Errorf("plugin installed to disk but daemon failed to load it: %v", err)
			}

			quiet, _ := cmd.Flags().GetBool("quiet")
			if !quiet {
				jsonFlag, _ := cmd.Flags().GetBool("json")
				if jsonFlag {
					return displayterminal.PrintJSON(resp)
				}
				mode := installMode(link, source)
				fmt.Printf("Installed (%s): %s (%s) v%s\n", mode, resp.Name, resp.Namespace, resp.Version)
			}
			return nil
		},
	}

	cmd.Flags().Bool("link", false, "Symlink instead of copying the plugin directory")

	return cmd
}

// resolvePluginSource handles URLs, archives, and plain directories.
// Returns the path to a directory containing manifest.yaml.
func resolvePluginSource(source string, link bool) (string, error) {
	if pluginfs.IsURL(source) {
		if link {
			return "", fmt.Errorf("--link cannot be used with URLs")
		}

		// Check allowed hosts
		host, err := pluginfs.HostFromURL(source)
		if err != nil {
			return "", err
		}

		store, _, _ := configyaml.InitStore(nil, nil)
		effective := store.Get()
		if !effective.IsHostAllowed(host) {
			return "", fmt.Errorf("host %q is not in allowed_hosts (configure with: recur config set allowed_hosts %q)", host, host)
		}

		fmt.Printf("Downloading from %s...\n", host)
		archivePath, err := pluginfs.Download(source)
		if err != nil {
			return "", err
		}
		defer os.Remove(archivePath)

		dir, err := pluginfs.Extract(archivePath)
		if err != nil {
			return "", err
		}
		return dir, nil
	}

	absPath, _ := filepath.Abs(source)

	if pluginfs.IsArchive(absPath) {
		if link {
			return "", fmt.Errorf("--link cannot be used with archives")
		}
		dir, err := pluginfs.Extract(absPath)
		if err != nil {
			return "", err
		}
		return dir, nil
	}

	return absPath, nil
}

// installMode returns a human-readable label for the install method.
func installMode(link bool, source string) string {
	if link {
		return "linked"
	}
	if pluginfs.IsURL(source) {
		return "downloaded"
	}
	if pluginfs.IsArchive(source) {
		return "extracted"
	}
	return "copied"
}

func newUninstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall <id>",
		Short: "Remove a plugin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			identifier := args[0]

			// Find the plugin on disk to get its directory
			plugins, _ := pluginfs.Discover()
			installed := pluginfs.FindByIdentifier(plugins, identifier)

			// Unregister from daemon if running
			client, err := requireDaemon(cmd)
			if err == nil {
				defer client.Close()
				_, err = client.Service.UninstallPlugin(context.Background(), &recurv1.UninstallPluginRequest{
					Identifier: identifier,
				})
				if err != nil {
					return fmt.Errorf("uninstall failed: %v", err)
				}
			}

			// Remove from disk
			if installed != nil {
				if err := pluginfs.Remove(installed.Dir); err != nil {
					return fmt.Errorf("removed from daemon but failed to delete from disk: %v", err)
				}
			}

			quiet, _ := cmd.Flags().GetBool("quiet")
			if !quiet {
				fmt.Printf("Plugin %s uninstalled.\n", identifier)
			}
			return nil
		},
	}
}
