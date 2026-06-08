// Package secretfile resolves SecretDefs whose source is "file" by
// reading the file at def.Ref. Implements domain/secret.Resolver.
package secretfile

import (
	"fmt"
	"os"
	"strings"

	"github.com/directedbits/recur/src/domain/secret"
)

// Resolver reads secrets from files.
type Resolver struct{}

// New returns a Resolver. The zero value is also usable.
func New() *Resolver { return &Resolver{} }

// Resolve reads def.Ref from disk and trims surrounding whitespace.
func (r *Resolver) Resolve(def secret.SecretDef) (string, error) {
	data, err := os.ReadFile(def.Ref)
	if err != nil {
		return "", fmt.Errorf("reading secret file %s: %w", def.Ref, err)
	}
	return strings.TrimSpace(string(data)), nil
}
