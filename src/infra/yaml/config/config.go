// Package config handles loading and persisting daemon configuration from
// ~/.config/recur/config.yaml. The Config type and key schema live in
// domain/config; this package re-exports them as aliases so existing
// infra-side callers compile unchanged while the canonical types live in
// the domain layer.
package configyaml

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"gopkg.in/yaml.v3"

	domainconfig "github.com/directedbits/recur/src/domain/config"
	defaultsos "github.com/directedbits/recur/src/infra/os/defaults"
	"github.com/directedbits/recur/src/infra/fs/atomicfile"
)

// Type aliases — the canonical types live in domain/config. infra/config is
// retained as the I/O layer (load, save, defaults, store wiring).
type (
	Config   = domainconfig.Config
	KeyDef   = domainconfig.KeyDef
	KeyValue = domainconfig.KeyValue
)

// Function aliases — re-export the key-schema helpers so existing callers
// of config.GetByKey, etc., compile.
var (
	Keys                  = domainconfig.Keys
	ValidConcurrencyModes = domainconfig.ValidConcurrencyModes
	ValidLogLevels        = domainconfig.ValidLogLevels
	LookupKey             = domainconfig.LookupKey
	GetByKey              = domainconfig.GetByKey
	SetByKey              = domainconfig.SetByKey
	DeleteByKey           = domainconfig.DeleteByKey
	AllKeys               = domainconfig.AllKeys
	SplitPluginKey        = domainconfig.SplitPluginKey
	DerefStr              = domainconfig.DerefStr
	DerefInt              = domainconfig.DerefInt
	EffectiveInt          = domainconfig.EffectiveInt
)

var envVarPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)\}`)

// Pointer helpers for building Config literals.
func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

// DefaultConfig returns a Config with all default values. Platform-specific
// defaults live in infra/os/defaults.
func DefaultConfig() *Config {
	return &Config{
		DefaultShell:    strPtr(defaultsos.DefaultShell),
		ErrorThreshold:  intPtr(5),
		ConcurrencyMode: strPtr("queue"),
		MaxQueueSize:    intPtr(100),
		Debounce:        strPtr("300ms"),
		ShutdownTimeout: strPtr("30s"),
		LogLevel:        strPtr(""),
		SocketAddress:   strPtr(defaultsos.DefaultSocketAddress),
		AllowedHosts:    strPtr(""),
	}
}

// ConfigDir returns the recur configuration directory, creating it if needed.
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	dir := filepath.Join(home, ".config", "recur")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("could not create config directory: %w", err)
	}
	return dir, nil
}

// DefaultPath returns the default config file path (~/.config/recur/config.yaml).
func DefaultPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// loadRaw reads the config file without applying defaults.
// Only fields explicitly present in the YAML will be non-nil.
// If the file does not exist, returns a zero-value Config (all nil pointers).
func loadRaw(path string) (*Config, error) {
	cfg := &Config{}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("could not read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("could not parse config file: %w", err)
	}

	interpolatePluginEnvVars(cfg)

	return cfg, nil
}

// interpolatePluginEnvVars replaces ${VAR} references in plugin config string values
// with the corresponding environment variable value.
func interpolatePluginEnvVars(cfg *Config) {
	for ns, entries := range cfg.Plugins {
		for k, v := range entries {
			if s, ok := v.(string); ok {
				expanded := envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
					varName := envVarPattern.FindStringSubmatch(match)[1]
					if val, ok := os.LookupEnv(varName); ok {
						return val
					}
					return match
				})
				cfg.Plugins[ns][k] = expanded
			}
		}
	}
}

// Save writes the config to the given path using atomic write (timestamped temp + rename).
// Only non-nil values are written to keep the file clean.
func Save(cfg *Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("could not marshal config: %w", err)
	}

	return atomicfile.Write(path, data)
}

// ToJSON converts a config value to a JSON string.
func ToJSON(v any) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
