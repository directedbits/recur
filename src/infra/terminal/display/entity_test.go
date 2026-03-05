package displayterminal

import (
	"testing"

	recurv1 "github.com/directedbits/recur/src/infra/grpc/v1"
)

func TestEntityStatus(t *testing.T) {
	tests := []struct {
		status recurv1.EntityStatus
		want   string
	}{
		{recurv1.EntityStatus_ENTITY_STATUS_ACTIVE, "active"},
		{recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED, "suspended"},
		{recurv1.EntityStatus_ENTITY_STATUS_ERROR, "error"},
		{recurv1.EntityStatus(99), "unknown"},
	}
	for _, tt := range tests {
		got := EntityStatus(tt.status)
		if got != tt.want {
			t.Errorf("EntityStatus(%v) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestStatusLabel(t *testing.T) {
	tests := []struct {
		status recurv1.EntityStatus
		want   string
	}{
		{recurv1.EntityStatus_ENTITY_STATUS_ACTIVE, ""},
		{recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED, " (suspended)"},
		{recurv1.EntityStatus_ENTITY_STATUS_ERROR, " (error)"},
		{recurv1.EntityStatus(99), ""},
	}
	for _, tt := range tests {
		got := StatusLabel(tt.status)
		if got != tt.want {
			t.Errorf("StatusLabel(%v) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestSafeID(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"abcdefghij", "abcdefgh"},
		{"abcdef", "abcdef"},
		{"12345678", "12345678"},
		{"123456789", "12345678"},
		{"", ""},
		{"ab", "ab"},
	}
	for _, tt := range tests {
		got := SafeID(tt.input)
		if got != tt.want {
			t.Errorf("SafeID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
