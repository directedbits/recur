package secretenv

import (
	"strings"
	"testing"

	"github.com/directedbits/recur/src/domain/secret"
)

func TestResolve_Found(t *testing.T) {
	t.Setenv("RECUR_TEST_SECRET", "value")
	r := New()
	got, err := r.Resolve(secret.SecretDef{Ref: "RECUR_TEST_SECRET"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "value" {
		t.Errorf("got %q, want %q", got, "value")
	}
}

func TestResolve_NotSetWithDefault(t *testing.T) {
	r := New()
	got, err := r.Resolve(secret.SecretDef{Ref: "RECUR_DOES_NOT_EXIST", Default: "fallback"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "fallback" {
		t.Errorf("got %q, want %q", got, "fallback")
	}
}

func TestResolve_EmptyWithDefault(t *testing.T) {
	t.Setenv("RECUR_TEST_EMPTY", "")
	r := New()
	got, err := r.Resolve(secret.SecretDef{Ref: "RECUR_TEST_EMPTY", Default: "fallback"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "fallback" {
		t.Errorf("got %q, want %q", got, "fallback")
	}
}

func TestResolve_RequiredMissingDefault(t *testing.T) {
	r := New()
	_, err := r.Resolve(secret.SecretDef{Ref: "RECUR_DOES_NOT_EXIST", Required: true})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "RECUR_DOES_NOT_EXIST") {
		t.Errorf("error %q should mention the var name", err)
	}
}

func TestResolve_RequiredCustomMsg(t *testing.T) {
	r := New()
	_, err := r.Resolve(secret.SecretDef{Ref: "RECUR_DOES_NOT_EXIST", Required: true, ErrorMsg: "custom msg"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "custom msg" {
		t.Errorf("got %q, want %q", err.Error(), "custom msg")
	}
}

func TestResolve_NotSetNotRequired(t *testing.T) {
	r := New()
	_, err := r.Resolve(secret.SecretDef{Ref: "RECUR_DOES_NOT_EXIST"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
