// Package secretenv resolves SecretDefs whose source is "env" by reading
// environment variables. Implements domain/secret.Resolver for the
// SecretDef.Source == "env" case.
package secretenv

import (
	"fmt"
	"os"

	"github.com/directedbits/recur/src/domain/secret"
)

// Resolver reads secrets from environment variables.
type Resolver struct{}

// New returns a Resolver. The zero value is also usable.
func New() *Resolver { return &Resolver{} }

// Resolve looks up def.Ref as an environment variable. Handles the
// ${VAR:-default} and ${VAR:?required} forms via def.Default and
// def.Required.
func (r *Resolver) Resolve(def secret.SecretDef) (string, error) {
	val, ok := os.LookupEnv(def.Ref)
	if !ok || val == "" {
		if def.Default != "" {
			return def.Default, nil
		}
		if def.Required {
			msg := def.ErrorMsg
			if msg == "" {
				msg = fmt.Sprintf("required environment variable %s is not set", def.Ref)
			}
			return "", fmt.Errorf("%s", msg)
		}
		if !ok {
			return "", fmt.Errorf("environment variable %s is not set", def.Ref)
		}
	}
	return val, nil
}
