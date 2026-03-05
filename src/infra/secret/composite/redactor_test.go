package secretcomposite

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
)

func TestRedactorRedactsMessage(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{})
	r := NewRedactor(inner)
	r.UpdateSecrets(map[string]string{"pw": "hunter2"})

	logger := slog.New(r)
	logger.Info("connecting with password hunter2")

	if got := buf.String(); !contains(got, "[REDACTED]") {
		t.Errorf("expected redaction in %q", got)
	}
	if got := buf.String(); contains(got, "hunter2") {
		t.Errorf("secret still present in %q", got)
	}
}

func TestRedactorRedactsAttrs(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{})
	r := NewRedactor(inner)
	r.UpdateSecrets(map[string]string{"key": "sk-123"})

	logger := slog.New(r)
	logger.Info("request", "auth", "Bearer sk-123")

	if got := buf.String(); contains(got, "sk-123") {
		t.Errorf("secret still present in attr: %q", got)
	}
}

func TestRedactorNoSecretsPassthrough(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{})
	r := NewRedactor(inner)

	logger := slog.New(r)
	logger.Info("normal message")

	if got := buf.String(); !contains(got, "normal message") {
		t.Errorf("message missing: %q", got)
	}
}

func TestRedactorEnabled(t *testing.T) {
	inner := slog.NewTextHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelWarn})
	r := NewRedactor(inner)
	if r.Enabled(context.Background(), slog.LevelInfo) {
		t.Error("should not be enabled for Info when inner requires Warn")
	}
	if !r.Enabled(context.Background(), slog.LevelWarn) {
		t.Error("should be enabled for Warn")
	}
}

func TestRedactorWithAttrs(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{})
	r := NewRedactor(inner)
	r.UpdateSecrets(map[string]string{"k": "hunter2"})

	logger := slog.New(r.WithAttrs([]slog.Attr{slog.String("service", "test")}))
	logger.Info("password is hunter2")

	if got := buf.String(); contains(got, "hunter2") {
		t.Errorf("secret still present after WithAttrs: %q", got)
	}
	if got := buf.String(); !contains(got, "service=test") {
		t.Errorf("expected pre-set attr, got %q", got)
	}
}

func TestRedactorWithGroup(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{})
	r := NewRedactor(inner)
	r.UpdateSecrets(map[string]string{"k": "hunter2"})

	logger := slog.New(r.WithGroup("mygroup"))
	logger.Info("password is hunter2")

	if got := buf.String(); contains(got, "hunter2") {
		t.Errorf("secret still present after WithGroup: %q", got)
	}
}

func TestRedactorUpdateSecretsIgnoresEmpty(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewTextHandler(&buf, &slog.HandlerOptions{})
	r := NewRedactor(inner)
	r.UpdateSecrets(map[string]string{"k": ""})

	logger := slog.New(r)
	logger.Info("nothing to redact")

	if got := buf.String(); contains(got, "[REDACTED]") {
		t.Errorf("empty secret should not cause redaction: %q", got)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && bytes.Contains([]byte(s), []byte(substr))
}
