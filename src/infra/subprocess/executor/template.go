package executorsubprocess

import (
	"bytes"
	"fmt"
	"text/template"
)

// Context holds the variables available during template resolution.
type Context struct {
	// Test is true when the action is invoked via `watch test` (dry-run/testing mode).
	// Plugins can use this to skip destructive operations or produce test output.
	Test bool

	// Set contains context variables from trigger events and --set flags.
	// These are promoted to top-level template variables (e.g. {{.RequestBody}}).
	Set map[string]string

	// Secrets holds resolved secret values keyed by secret name.
	// Available in templates via {{secret "name"}}.
	Secrets map[string]string

	// secretAccessed is called when the secret template function is invoked.
	// Used internally by ResolveOptions for sensitivity tracking.
	secretAccessed func()
}

// ResolveResult holds resolved options and tracks which keys accessed secrets.
type ResolveResult struct {
	Options       map[string]any
	SensitiveKeys map[string]bool
}

// templateData builds a flat map for template execution. Context variables
// from Set are promoted to top-level keys so users write {{.RequestBody}}
// instead of {{.Set.RequestBody}}.
func (ctx *Context) templateData() map[string]any {
	data := make(map[string]any, len(ctx.Set)+1)
	data["Test"] = ctx.Test
	for k, v := range ctx.Set {
		data[k] = v
	}
	return data
}

// ResolveTemplate applies Go text/template substitution to s using the given context.
// Returns the original string unchanged if it contains no template expressions.
func ResolveTemplate(s string, ctx *Context) (string, error) {
	if ctx == nil {
		ctx = &Context{}
	}

	funcMap := template.FuncMap{}
	if ctx.Secrets != nil {
		funcMap["secret"] = func(name string) (string, error) {
			val, ok := ctx.Secrets[name]
			if !ok {
				return "", fmt.Errorf("undefined secret %q", name)
			}
			if ctx.secretAccessed != nil {
				ctx.secretAccessed()
			}
			return val, nil
		}
	} else {
		funcMap["secret"] = func(name string) (string, error) {
			return "", fmt.Errorf("undefined secret %q (no secrets section in recurfile)", name)
		}
	}

	tmpl, err := template.New("").Option("missingkey=zero").Funcs(funcMap).Parse(s)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx.templateData()); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}

	return buf.String(), nil
}

// ResolveOptions resolves templates in all string values of an options map.
// Non-string values are passed through unchanged. Returns a ResolveResult
// that includes which option keys accessed secrets via {{secret "name"}}.
func ResolveOptions(opts map[string]any, ctx *Context) (*ResolveResult, error) {
	if len(opts) == 0 {
		return &ResolveResult{Options: opts}, nil
	}

	if ctx == nil {
		ctx = &Context{}
	}

	sensitiveKeys := make(map[string]bool)
	origCallback := ctx.secretAccessed

	resolved := make(map[string]any, len(opts))
	for k, v := range opts {
		switch val := v.(type) {
		case string:
			currentKey := k
			ctx.secretAccessed = func() {
				sensitiveKeys[currentKey] = true
			}
			r, err := ResolveTemplate(val, ctx)
			if err != nil {
				ctx.secretAccessed = origCallback
				return nil, fmt.Errorf("option %q: %w", k, err)
			}
			resolved[k] = r
		default:
			resolved[k] = v
		}
	}
	ctx.secretAccessed = origCallback
	return &ResolveResult{Options: resolved, SensitiveKeys: sensitiveKeys}, nil
}
