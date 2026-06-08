package executorsubprocess

import (
	"testing"
)

func TestResolveTemplateNoTemplating(t *testing.T) {
	result, err := ResolveTemplate("plain string", &Context{})
	if err != nil {
		t.Fatalf("ResolveTemplate failed: %v", err)
	}
	if result != "plain string" {
		t.Errorf("result = %q", result)
	}
}

func TestResolveTemplateTestVar(t *testing.T) {
	result, err := ResolveTemplate("test={{.Test}}", &Context{Test: true})
	if err != nil {
		t.Fatalf("ResolveTemplate failed: %v", err)
	}
	if result != "test=true" {
		t.Errorf("result = %q, want %q", result, "test=true")
	}
}

func TestResolveTemplateTestFalse(t *testing.T) {
	result, err := ResolveTemplate("test={{.Test}}", &Context{Test: false})
	if err != nil {
		t.Fatalf("ResolveTemplate failed: %v", err)
	}
	if result != "test=false" {
		t.Errorf("result = %q, want %q", result, "test=false")
	}
}

func TestResolveTemplateContextVars(t *testing.T) {
	ctx := &Context{
		Set: map[string]string{
			"file": "/tmp/test.txt",
			"mode": "debug",
		},
	}

	result, err := ResolveTemplate("{{.file}} in {{.mode}} mode", ctx)
	if err != nil {
		t.Fatalf("ResolveTemplate failed: %v", err)
	}
	if result != "/tmp/test.txt in debug mode" {
		t.Errorf("result = %q", result)
	}
}

func TestResolveTemplateMissingKey(t *testing.T) {
	// missingkey=zero should produce zero value, not error
	result, err := ResolveTemplate("val={{.missing}}", &Context{})
	if err != nil {
		t.Fatalf("ResolveTemplate failed: %v", err)
	}
	if result != "val=<no value>" {
		t.Errorf("result = %q, want %q", result, "val=<no value>")
	}
}

func TestResolveTemplateNilContext(t *testing.T) {
	result, err := ResolveTemplate("test={{.Test}}", nil)
	if err != nil {
		t.Fatalf("ResolveTemplate failed: %v", err)
	}
	if result != "test=false" {
		t.Errorf("result = %q", result)
	}
}

func TestResolveTemplateConditional(t *testing.T) {
	tmpl := `{{if .Test}}[TEST] {{end}}running command`

	result, err := ResolveTemplate(tmpl, &Context{Test: true})
	if err != nil {
		t.Fatalf("ResolveTemplate failed: %v", err)
	}
	if result != "[TEST] running command" {
		t.Errorf("result = %q", result)
	}

	result, err = ResolveTemplate(tmpl, &Context{Test: false})
	if err != nil {
		t.Fatalf("ResolveTemplate failed: %v", err)
	}
	if result != "running command" {
		t.Errorf("result = %q", result)
	}
}

func TestResolveTemplateWebhookVars(t *testing.T) {
	ctx := &Context{
		Set: map[string]string{
			"RequestMethod": "POST",
			"RequestPath":   "/webhook",
			"RequestBody":   `{"event":"push"}`,
			"RemoteAddr":    "10.0.0.1:1234",
		},
	}

	result, err := ResolveTemplate("{{.RequestMethod}} {{.RequestPath}} from {{.RemoteAddr}}", ctx)
	if err != nil {
		t.Fatalf("ResolveTemplate failed: %v", err)
	}
	if result != `POST /webhook from 10.0.0.1:1234` {
		t.Errorf("result = %q", result)
	}
}

func TestResolveOptions(t *testing.T) {
	opts := map[string]any{
		"command": "echo {{.msg}}",
		"timeout": "5s",
		"count":   42, // non-string, pass through
	}
	ctx := &Context{Set: map[string]string{"msg": "hello"}}

	result, err := ResolveOptions(opts, ctx)
	if err != nil {
		t.Fatalf("ResolveOptions failed: %v", err)
	}
	if result.Options["command"] != "echo hello" {
		t.Errorf("command = %q", result.Options["command"])
	}
	if result.Options["timeout"] != "5s" {
		t.Errorf("timeout = %q", result.Options["timeout"])
	}
	if result.Options["count"] != 42 {
		t.Errorf("count = %v", result.Options["count"])
	}
	if len(result.SensitiveKeys) != 0 {
		t.Errorf("expected no sensitive keys, got %v", result.SensitiveKeys)
	}
}

func TestResolveOptionsEmpty(t *testing.T) {
	result, err := ResolveOptions(nil, &Context{})
	if err != nil {
		t.Fatalf("ResolveOptions failed: %v", err)
	}
	if result.Options != nil {
		t.Errorf("expected nil options, got %v", result.Options)
	}
}

func TestResolveTemplateInvalidSyntax(t *testing.T) {
	_, err := ResolveTemplate("{{.Bad", &Context{})
	if err == nil {
		t.Fatal("expected error for invalid template syntax")
	}
}

func TestResolveTemplateSecret(t *testing.T) {
	ctx := &Context{
		Secrets: map[string]string{"api_key": "sk-123"},
	}
	result, err := ResolveTemplate(`Bearer {{secret "api_key"}}`, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Bearer sk-123" {
		t.Errorf("result = %q, want %q", result, "Bearer sk-123")
	}
}

func TestResolveTemplateSecretUndefined(t *testing.T) {
	ctx := &Context{
		Secrets: map[string]string{},
	}
	_, err := ResolveTemplate(`{{secret "missing"}}`, ctx)
	if err == nil {
		t.Fatal("expected error for undefined secret")
	}
}

func TestResolveTemplateSecretNoSecretsSection(t *testing.T) {
	_, err := ResolveTemplate(`{{secret "x"}}`, &Context{})
	if err == nil {
		t.Fatal("expected error when no secrets configured")
	}
}

func TestResolveOptionsSensitiveTracking(t *testing.T) {
	opts := map[string]any{
		"auth_token": `Bearer {{secret "api_key"}}`,
		"path":       "/deploy",
		"count":      42,
	}
	ctx := &Context{
		Secrets: map[string]string{"api_key": "sk-123"},
	}

	result, err := ResolveOptions(opts, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Options["auth_token"] != "Bearer sk-123" {
		t.Errorf("auth_token = %q", result.Options["auth_token"])
	}
	if !result.SensitiveKeys["auth_token"] {
		t.Error("auth_token should be marked sensitive")
	}
	if result.SensitiveKeys["path"] {
		t.Error("path should not be marked sensitive")
	}
	if result.SensitiveKeys["count"] {
		t.Error("count should not be marked sensitive")
	}
}

func TestResolveTemplateSecretInjectionRegression(t *testing.T) {
	ctx := &Context{
		Set:     map[string]string{"UserInput": `{{secret "api_key"}}`},
		Secrets: map[string]string{"api_key": "sk-123"},
	}
	result, err := ResolveTemplate("got: {{.UserInput}}", ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != `got: {{secret "api_key"}}` {
		t.Errorf("template injection! result = %q — context variable should be literal text, not evaluated", result)
	}
}
