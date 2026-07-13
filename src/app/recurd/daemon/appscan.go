package daemon

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	appbundle "github.com/directedbits/recur/src/infra/fs/appbundle"
	configyaml "github.com/directedbits/recur/src/infra/yaml/config"
	recurfileyaml "github.com/directedbits/recur/src/infra/yaml/recurfile"
)

// scanAppFolder discovers app bundles installed under ~/.config/recur/app and
// registers any whose recurfile is not already registered. This lets apps
// installed while the daemon was stopped register themselves on the next start.
// It runs after state replay, so apps already restored from state are skipped;
// registration is idempotent regardless.
func (d *Daemon) scanAppFolder() {
	base, err := configyaml.AppDir()
	if err != nil {
		slog.Warn("app scan: could not resolve app dir", "error", err)
		return
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("app scan: could not read app dir", "error", err)
		}
		return
	}

	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := filepath.Join(base, e.Name())
		rfPath, err := appbundle.FindRecurfile(dir)
		if err != nil {
			continue // not an app directory
		}
		if d.registry.hasRecurfilePath(rfPath) {
			continue // already registered (e.g. restored from state)
		}
		f, err := recurfileyaml.Load(rfPath)
		if err != nil {
			slog.Warn("app scan: skipping app", "app", e.Name(), "error", err)
			continue
		}
		result, err := d.registry.registerRecurfile(f, d.plugins, d.triggerDefaultsMap(), d.pluginTriggerOverrides())
		if err != nil {
			slog.Warn("app scan: skipping app", "app", e.Name(), "error", err)
			continue
		}
		slog.Info("app scan: registered app",
			"app", e.Name(), "path", rfPath, "triggers", result.TriggerCount, "actions", result.ActionCount)
	}
}
