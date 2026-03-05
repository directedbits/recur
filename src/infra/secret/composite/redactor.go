package secretcomposite

import (
	"context"
	"log/slog"
	"strings"
	"sync"
)

// Redactor wraps a slog.Handler and replaces known secret values with [REDACTED].
type Redactor struct {
	inner   slog.Handler
	mu      sync.RWMutex
	secrets map[string]bool
}

// NewRedactor creates a Redactor that wraps the given handler.
func NewRedactor(inner slog.Handler) *Redactor {
	return &Redactor{
		inner:   inner,
		secrets: make(map[string]bool),
	}
}

// UpdateSecrets replaces the set of known secret values.
func (r *Redactor) UpdateSecrets(values map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.secrets = make(map[string]bool, len(values))
	for _, v := range values {
		if v != "" {
			r.secrets[v] = true
		}
	}
}

func (r *Redactor) Enabled(ctx context.Context, level slog.Level) bool {
	return r.inner.Enabled(ctx, level)
}

func (r *Redactor) Handle(ctx context.Context, record slog.Record) error {
	r.mu.RLock()
	secrets := r.secrets
	r.mu.RUnlock()

	if len(secrets) == 0 {
		return r.inner.Handle(ctx, record)
	}

	record.Message = r.redact(record.Message, secrets)

	var redactedAttrs []slog.Attr
	record.Attrs(func(a slog.Attr) bool {
		redactedAttrs = append(redactedAttrs, r.redactAttr(a, secrets))
		return true
	})

	newRecord := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	newRecord.AddAttrs(redactedAttrs...)
	return r.inner.Handle(ctx, newRecord)
}

func (r *Redactor) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Redactor{
		inner:   r.inner.WithAttrs(attrs),
		secrets: r.secrets,
	}
}

func (r *Redactor) WithGroup(name string) slog.Handler {
	return &Redactor{
		inner:   r.inner.WithGroup(name),
		secrets: r.secrets,
	}
}

func (r *Redactor) redact(s string, secrets map[string]bool) string {
	for secret := range secrets {
		if strings.Contains(s, secret) {
			s = strings.ReplaceAll(s, secret, "[REDACTED]")
		}
	}
	return s
}

func (r *Redactor) redactAttr(a slog.Attr, secrets map[string]bool) slog.Attr {
	if a.Value.Kind() == slog.KindString {
		a.Value = slog.StringValue(r.redact(a.Value.String(), secrets))
	}
	return a
}
