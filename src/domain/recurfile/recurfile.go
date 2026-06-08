// Package recurfile defines the recurfile aggregate — a YAML configuration file
// that declaratively specifies triggers and actions.
package recurfile

import "github.com/directedbits/recur/src/domain/secret"

// Recurfile represents a registered recurfile tracked by the daemon.
type Recurfile struct {
	ID       string
	FilePath string
	Groups   []string
	Secrets  []secret.SecretDef
}
