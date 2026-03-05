package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/helshabini/fsbroker"
)

// parsedOptions holds the validated watcher configuration.
type parsedOptions struct {
	WatchPath      string
	Recursive      bool
	Filters        []string
	IgnoreHidden   bool
	IgnoreSystem   bool
	EntityType     string // "file", "directory", or "all"
	ExcludePaths   []string
	DefaultsActive bool // true when no exclude_paths was set at any layer; enables Recurfile-basename filtering
}

// defaultExcludePaths returns the built-in default exclude_paths globs.
// Used when neither daemon config nor trigger options set exclude_paths.
func defaultExcludePaths() []string {
	return []string{"~/.config/recur/**"}
}

// parseOptions extracts and validates watcher options from the plugin input.
// `config` is the plugin-namespace block from daemon config; `opts` is the
// merged trigger options. exclude_paths follows the resolution table:
//
//	config unset, opts unset → defaults active (recur dir + Recurfile names)
//	config unset, opts set   → defaults + opts (additive); recur dir kept
//	config set,   opts unset → config (replaces defaults)
//	config set,   opts set   → opts (replaces defaults)
//
// Explicit empty list at either level counts as "set" → defaults dropped.
func parseOptions(opts, config map[string]any) (*parsedOptions, error) {
	p := &parsedOptions{
		IgnoreHidden: true,
		IgnoreSystem: true,
		EntityType:   "file",
	}

	if v, ok := opts["path"].(string); ok && v != "" {
		p.WatchPath = v
	}

	if v, ok := opts["recursive"].(bool); ok {
		p.Recursive = v
	}

	if v, ok := opts["ignore_hidden"].(bool); ok {
		p.IgnoreHidden = v
	}

	if v, ok := opts["ignore_system"].(bool); ok {
		p.IgnoreSystem = v
	}

	if v, ok := opts["entity_type"].(string); ok && v != "" {
		p.EntityType = strings.ToLower(v)
	}
	switch p.EntityType {
	case "file", "directory", "all":
		// valid
	default:
		return nil, fmt.Errorf("invalid entity_type %q: must be file, directory, or all", p.EntityType)
	}

	if f, ok := opts["filter"]; ok {
		switch v := f.(type) {
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok && s != "" {
					if _, err := filepath.Match(s, ""); err != nil {
						return nil, fmt.Errorf("invalid filter pattern %q: %w", s, err)
					}
					p.Filters = append(p.Filters, s)
				}
			}
		case []string:
			for _, s := range v {
				if s != "" {
					if _, err := filepath.Match(s, ""); err != nil {
						return nil, fmt.Errorf("invalid filter pattern %q: %w", s, err)
					}
					p.Filters = append(p.Filters, s)
				}
			}
		}
	}

	configList, configSet, err := readStringList(config, "exclude_paths")
	if err != nil {
		return nil, fmt.Errorf("invalid plugins.core.fileevents.exclude_paths: %w", err)
	}
	optsList, optsSet, err := readStringList(opts, "exclude_paths")
	if err != nil {
		return nil, fmt.Errorf("invalid exclude_paths: %w", err)
	}

	switch {
	case configSet && optsSet:
		p.ExcludePaths = optsList
		p.DefaultsActive = false
	case configSet:
		p.ExcludePaths = configList
		p.DefaultsActive = false
	case optsSet:
		// Explicit empty list at trigger level is an escape hatch: drop
		// defaults without adding anything. A non-empty list is additive
		// to the plugin defaults.
		if len(optsList) == 0 {
			p.ExcludePaths = nil
			p.DefaultsActive = false
		} else {
			p.ExcludePaths = append(defaultExcludePaths(), optsList...)
			p.DefaultsActive = true
		}
	default:
		p.ExcludePaths = defaultExcludePaths()
		p.DefaultsActive = true
	}

	expanded, err := expandExcludePatterns(p.ExcludePaths)
	if err != nil {
		return nil, err
	}
	p.ExcludePaths = expanded

	return p, nil
}

// readStringList extracts a []string from a map[string]any, accepting either
// []any or []string at the key. Returns (list, true, nil) when the key is
// present (even if the list is empty), (nil, false, nil) when absent.
func readStringList(m map[string]any, key string) ([]string, bool, error) {
	if m == nil {
		return nil, false, nil
	}
	v, ok := m[key]
	if !ok {
		return nil, false, nil
	}
	switch list := v.(type) {
	case []any:
		out := make([]string, 0, len(list))
		for _, item := range list {
			s, ok := item.(string)
			if !ok {
				return nil, true, fmt.Errorf("entry must be a string, got %T", item)
			}
			if s != "" {
				out = append(out, s)
			}
		}
		return out, true, nil
	case []string:
		out := make([]string, 0, len(list))
		for _, s := range list {
			if s != "" {
				out = append(out, s)
			}
		}
		return out, true, nil
	default:
		return nil, true, fmt.Errorf("must be a list of strings, got %T", v)
	}
}

// expandExcludePatterns expands a leading "~/" in each pattern and validates
// it as a doublestar glob. Returns slash-normalized patterns.
func expandExcludePatterns(patterns []string) ([]string, error) {
	if len(patterns) == 0 {
		return nil, nil
	}
	home, _ := os.UserHomeDir()
	out := make([]string, 0, len(patterns))
	for _, p := range patterns {
		if strings.HasPrefix(p, "~/") && home != "" {
			p = filepath.Join(home, p[2:])
		}
		p = filepath.ToSlash(p)
		if !doublestar.ValidatePattern(p) {
			return nil, fmt.Errorf("invalid exclude_paths pattern %q", p)
		}
		out = append(out, p)
	}
	return out, nil
}

// matchesExclude reports whether absPath matches any of the patterns.
// Patterns and absPath are matched as slash-normalized strings using
// doublestar (** supported).
func matchesExclude(patterns []string, absPath string) bool {
	if len(patterns) == 0 {
		return false
	}
	abs, err := filepath.Abs(absPath)
	if err != nil {
		abs = absPath
	}
	abs = filepath.ToSlash(abs)
	for _, p := range patterns {
		if ok, _ := doublestar.PathMatch(p, abs); ok {
			return true
		}
	}
	return false
}

// createBroker creates and configures an fsbroker instance.
func createBroker(opts *parsedOptions) (*fsbroker.FSBroker, error) {
	cfg := fsbroker.DefaultFSConfig()
	cfg.IgnoreHiddenFiles = opts.IgnoreHidden
	cfg.IgnoreSysFiles = opts.IgnoreSystem

	broker, err := fsbroker.NewFSBroker(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating watcher: %w", err)
	}

	if opts.Recursive {
		if err := broker.AddRecursiveWatch(opts.WatchPath); err != nil {
			broker.Stop()
			return nil, fmt.Errorf("watching %s (recursive): %w", opts.WatchPath, err)
		}
	} else {
		if err := broker.AddWatch(opts.WatchPath); err != nil {
			broker.Stop()
			return nil, fmt.Errorf("watching %s: %w", opts.WatchPath, err)
		}
	}

	return broker, nil
}

// matchesTriggerType returns true if the fsbroker OpType matches the trigger type.
func matchesTriggerType(triggerType string, opType fsbroker.OpType) bool {
	switch strings.ToLower(triggerType) {
	case "filecreated":
		return opType == fsbroker.Create
	case "filemodified":
		return opType == fsbroker.Write
	case "filedeleted":
		return opType == fsbroker.Remove
	case "filemoved":
		return opType == fsbroker.Rename
	default:
		return false
	}
}

// matchesFilter returns true if the file path matches at least one filter
// pattern, or if no filters are configured (empty list = match all).
func matchesFilter(filters []string, filePath string) bool {
	if len(filters) == 0 {
		return true
	}
	base := filepath.Base(filePath)
	for _, pattern := range filters {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}
	return false
}

// matchesEntityType returns true if the entity (file or directory) matches
// the configured entity_type filter.
func matchesEntityType(entityType string, isDir bool) bool {
	switch entityType {
	case "file":
		return !isDir
	case "directory":
		return isDir
	default:
		return true
	}
}

// buildContext creates the context variable map for a trigger event.
func buildContext(triggerType string, action *fsbroker.FSAction) map[string]string {
	filePath := ""
	isDir := false
	if action.Subject != nil {
		filePath = action.Subject.Path
		isDir = action.Subject.IsDir()
	}

	ctx := map[string]string{
		"IsDirectory": fmt.Sprintf("%t", isDir),
	}

	switch strings.ToLower(triggerType) {
	case "filemoved":
		// fsnotify only provides the new name reliably; From may not be available.
		ctx["From"] = ""
		ctx["To"] = filePath
	case "filedeleted":
		ctx["FilePath"] = filePath
		// Platform-dependent; fsbroker does not distinguish trash vs permanent delete.
		ctx["PermanentlyDeleted"] = "true"
	default:
		ctx["FilePath"] = filePath
	}

	return ctx
}

// isFileEventType returns true if the trigger type is a known file event.
func isFileEventType(triggerType string) bool {
	switch strings.ToLower(triggerType) {
	case "filecreated", "filemodified", "filedeleted", "filemoved":
		return true
	default:
		return false
	}
}
