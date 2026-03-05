package text

import (
	"reflect"
	"testing"
)

func TestCloseMatches_FindsTypos(t *testing.T) {
	choices := []string{"Cron", "Interval", "FileCreated", "FileChanged", "Notify"}
	got := CloseMatches("Crom", choices, 3)
	if len(got) == 0 || got[0] != "Cron" {
		t.Errorf("expected 'Cron' first, got %v", got)
	}
}

func TestCloseMatches_CaseInsensitive(t *testing.T) {
	got := CloseMatches("interval", []string{"Interval", "Cron"}, 3)
	if len(got) == 0 || got[0] != "Interval" {
		t.Errorf("expected case-insensitive match, got %v", got)
	}
}

func TestCloseMatches_NoMatchBelowThreshold(t *testing.T) {
	got := CloseMatches("xyzzy", []string{"Cron", "Notify"}, 3)
	if len(got) != 0 {
		t.Errorf("expected no matches, got %v", got)
	}
}

func TestCloseMatches_LimitsToN(t *testing.T) {
	choices := []string{"File1", "File2", "File3", "File4", "File5"}
	got := CloseMatches("File", choices, 2)
	if len(got) != 2 {
		t.Errorf("expected 2 matches, got %d: %v", len(got), got)
	}
}

func TestLevenshtein(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"", "abc", 3},
		{"abc", "", 3},
		{"abc", "abc", 0},
		{"kitten", "sitting", 3},
		{"Cron", "Crom", 1},
	}
	for _, c := range cases {
		if got := Levenshtein(c.a, c.b); got != c.want {
			t.Errorf("Levenshtein(%q,%q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestSimilarity_IdenticalIsOne(t *testing.T) {
	if Similarity("abc", "abc") != 1.0 {
		t.Errorf("expected 1.0 for identical strings")
	}
}

func TestCloseMatches_OrderedByScore(t *testing.T) {
	// Exact-case-difference "cron" is a closer match than "cronb" (extra char)
	got := CloseMatches("cron", []string{"cronb", "Cron"}, 2)
	want := []string{"Cron", "cronb"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
