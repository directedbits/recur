package secretkeyring

import (
	"errors"
	"strings"
	"testing"

	"github.com/directedbits/recur/src/domain/secret"
)

type fakeProvider struct {
	val string
	err error
	got struct{ service, key string }
}

func (f *fakeProvider) Get(service, key string) (string, error) {
	f.got.service = service
	f.got.key = key
	return f.val, f.err
}

func TestResolve_Success(t *testing.T) {
	fp := &fakeProvider{val: "secret"}
	r := New(fp)
	got, err := r.Resolve(secret.SecretDef{Ref: "myservice/mykey"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "secret" {
		t.Errorf("got %q, want %q", got, "secret")
	}
	if fp.got.service != "myservice" || fp.got.key != "mykey" {
		t.Errorf("provider got (%q, %q), want (myservice, mykey)", fp.got.service, fp.got.key)
	}
}

func TestResolve_BadRef(t *testing.T) {
	r := New(&fakeProvider{})
	_, err := r.Resolve(secret.SecretDef{Ref: "noseparator"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "service/key") {
		t.Errorf("error %q should explain the expected format", err)
	}
}

func TestResolve_ProviderError(t *testing.T) {
	fp := &fakeProvider{err: errors.New("not found")}
	r := New(fp)
	_, err := r.Resolve(secret.SecretDef{Ref: "svc/key"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "svc/key") {
		t.Errorf("error %q should include the ref", err)
	}
}
