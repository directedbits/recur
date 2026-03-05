package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resolutionCase represents one row in the exclude_paths resolution table.
type resolutionCase struct {
	name           string
	configOpts     map[string]any
	triggerOpts    map[string]any
	wantList       []string
	wantDefaultsOn bool
}

func TestParseOptions_ExcludePathsResolutionTable(t *testing.T) {
	home, _ := os.UserHomeDir()
	expandedDefaults := []string{filepath.ToSlash(filepath.Join(home, ".config/recur/**"))}

	cases := []resolutionCase{
		{
			name:           "neither set: defaults active",
			configOpts:     nil,
			triggerOpts:    map[string]any{"path": "/tmp"},
			wantList:       expandedDefaults,
			wantDefaultsOn: true,
		},
		{
			name:           "only trigger set: additive to defaults",
			configOpts:     nil,
			triggerOpts:    map[string]any{"path": "/tmp", "exclude_paths": []any{"**/build/**"}},
			wantList:       append(append([]string{}, expandedDefaults...), "**/build/**"),
			wantDefaultsOn: true,
		},
		{
			name:           "only config set: replaces defaults",
			configOpts:     map[string]any{"exclude_paths": []any{"**/node_modules/**"}},
			triggerOpts:    map[string]any{"path": "/tmp"},
			wantList:       []string{"**/node_modules/**"},
			wantDefaultsOn: false,
		},
		{
			name:           "both set: trigger wins",
			configOpts:     map[string]any{"exclude_paths": []any{"**/node_modules/**"}},
			triggerOpts:    map[string]any{"path": "/tmp", "exclude_paths": []any{"**/build/**"}},
			wantList:       []string{"**/build/**"},
			wantDefaultsOn: false,
		},
		{
			name:           "explicit empty config: drops defaults",
			configOpts:     map[string]any{"exclude_paths": []any{}},
			triggerOpts:    map[string]any{"path": "/tmp"},
			wantList:       nil,
			wantDefaultsOn: false,
		},
		{
			name:           "explicit empty trigger: drops defaults",
			configOpts:     nil,
			triggerOpts:    map[string]any{"path": "/tmp", "exclude_paths": []any{}},
			wantList:       nil,
			wantDefaultsOn: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts, err := parseOptions(tc.triggerOpts, tc.configOpts)
			if err != nil {
				t.Fatalf("parseOptions: %v", err)
			}
			if !slicesEqual(opts.ExcludePaths, tc.wantList) {
				t.Errorf("ExcludePaths = %v, want %v", opts.ExcludePaths, tc.wantList)
			}
			if opts.DefaultsActive != tc.wantDefaultsOn {
				t.Errorf("DefaultsActive = %v, want %v", opts.DefaultsActive, tc.wantDefaultsOn)
			}
		})
	}
}

func TestParseOptions_ExcludePathsTildeExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	opts, err := parseOptions(
		map[string]any{"path": "/tmp", "exclude_paths": []any{"~/.cache/**"}},
		nil,
	)
	if err != nil {
		t.Fatalf("parseOptions: %v", err)
	}
	want := filepath.ToSlash(filepath.Join(home, ".cache/**"))
	found := false
	for _, p := range opts.ExcludePaths {
		if p == want {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected expanded pattern %q in %v", want, opts.ExcludePaths)
	}
}

func TestParseOptions_ExcludePathsInvalidPattern(t *testing.T) {
	_, err := parseOptions(
		map[string]any{"path": "/tmp", "exclude_paths": []any{"**/[unclosed"}},
		nil,
	)
	if err == nil {
		t.Fatal("expected error for invalid pattern")
	}
	if !strings.Contains(err.Error(), "invalid exclude_paths pattern") {
		t.Errorf("error = %q, want 'invalid exclude_paths pattern'", err.Error())
	}
}

func TestParseOptions_ExcludePathsWrongType(t *testing.T) {
	_, err := parseOptions(
		map[string]any{"path": "/tmp", "exclude_paths": "not a list"},
		nil,
	)
	if err == nil {
		t.Fatal("expected error for non-list value")
	}
}

func TestMatchesExclude_RecurDirGlob(t *testing.T) {
	home, _ := os.UserHomeDir()
	patterns := []string{filepath.ToSlash(filepath.Join(home, ".config/recur/**"))}

	included := filepath.Join(home, ".config/recur/state/state.json")
	excluded := filepath.Join(home, "projects/foo.txt")

	if !matchesExclude(patterns, included) {
		t.Errorf("expected %q to be excluded", included)
	}
	if matchesExclude(patterns, excluded) {
		t.Errorf("expected %q to NOT be excluded", excluded)
	}
}

func TestMatchesExclude_DoubleStarAtAnyDepth(t *testing.T) {
	patterns := []string{"**/build/**"}

	hits := []string{"/foo/build/out.o", "/a/b/c/build/sub/x"}
	misses := []string{"/foo/src/main.go", "/build_no_slash/x"}

	for _, p := range hits {
		if !matchesExclude(patterns, p) {
			t.Errorf("%q should match", p)
		}
	}
	for _, p := range misses {
		if matchesExclude(patterns, p) {
			t.Errorf("%q should NOT match", p)
		}
	}
}

func TestIsRecurfileName_DefaultExclusion(t *testing.T) {
	hits := []string{"Recurfile", "recurfile.yaml", "Recurfile.YML", "RECURFILE"}
	misses := []string{".recurfile.yaml", "myrecurfile.yaml", "Recurfile.json", "Recurfilex"}
	for _, n := range hits {
		if !isRecurfileName(n) {
			t.Errorf("%q should match", n)
		}
	}
	for _, n := range misses {
		if isRecurfileName(n) {
			t.Errorf("%q should NOT match", n)
		}
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
