package cli

import (
	"testing"
)

func TestMinLen(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{3, 5, 3},
		{5, 3, 3},
		{4, 4, 4},
		{0, 1, 0},
	}
	for _, tt := range tests {
		got := minLen(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("minLen(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		input string
		width int
		want  string
	}{
		{"abc", 5, "abc  "},
		{"abc", 3, "abc"},
		{"abc", 2, "abc"},
		{"", 3, "   "},
		{"abcde", 5, "abcde"},
	}
	for _, tt := range tests {
		got := padRight(tt.input, tt.width)
		if got != tt.want {
			t.Errorf("padRight(%q, %d) = %q, want %q", tt.input, tt.width, got, tt.want)
		}
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		input any
		want  string
	}{
		{nil, "(not set)"},
		{"hello", "hello"},
		{42, "42"},
		{true, "true"},
		{"", ""},
	}
	for _, tt := range tests {
		got := formatValue(tt.input)
		if got != tt.want {
			t.Errorf("formatValue(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}


func TestSplitKeyValue(t *testing.T) {
	tests := []struct {
		input     string
		wantKey   string
		wantValue string
	}{
		{"key=value", "key", "value"},
		{"key=", "key", ""},
		{"key=val=ue", "key", "val=ue"},
		{"noequals", "noequals", ""},
		{"=value", "", "value"},
	}
	for _, tt := range tests {
		k, v := splitKeyValue(tt.input)
		if k != tt.wantKey || v != tt.wantValue {
			t.Errorf("splitKeyValue(%q) = (%q, %q), want (%q, %q)", tt.input, k, v, tt.wantKey, tt.wantValue)
		}
	}
}
