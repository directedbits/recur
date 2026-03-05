package main

import (
	"testing"

	"github.com/helshabini/fsbroker"
)

func TestParseOptions_Defaults(t *testing.T) {
	opts, err := parseOptions(map[string]any{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.WatchPath != "" {
		t.Errorf("WatchPath = %q, want empty", opts.WatchPath)
	}
	if opts.Recursive {
		t.Error("Recursive should default to false")
	}
	if !opts.IgnoreHidden {
		t.Error("IgnoreHidden should default to true")
	}
	if !opts.IgnoreSystem {
		t.Error("IgnoreSystem should default to true")
	}
	if opts.EntityType != "file" {
		t.Errorf("EntityType = %q, want %q", opts.EntityType, "file")
	}
	if len(opts.Filters) != 0 {
		t.Errorf("Filters = %v, want empty", opts.Filters)
	}
}

func TestParseOptions_AllFields(t *testing.T) {
	opts, err := parseOptions(map[string]any{
		"path":          "/tmp/watch",
		"recursive":     true,
		"ignore_hidden": false,
		"ignore_system": false,
		"entity_type":   "directory",
		"filter":        []any{"*.go", "*.md"},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.WatchPath != "/tmp/watch" {
		t.Errorf("WatchPath = %q", opts.WatchPath)
	}
	if !opts.Recursive {
		t.Error("Recursive should be true")
	}
	if opts.IgnoreHidden {
		t.Error("IgnoreHidden should be false")
	}
	if opts.IgnoreSystem {
		t.Error("IgnoreSystem should be false")
	}
	if opts.EntityType != "directory" {
		t.Errorf("EntityType = %q", opts.EntityType)
	}
	if len(opts.Filters) != 2 {
		t.Fatalf("Filters len = %d, want 2", len(opts.Filters))
	}
}

func TestParseOptions_EntityTypeAll(t *testing.T) {
	opts, err := parseOptions(map[string]any{"entity_type": "all"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.EntityType != "all" {
		t.Errorf("EntityType = %q, want %q", opts.EntityType, "all")
	}
}

func TestParseOptions_EntityTypeCaseInsensitive(t *testing.T) {
	opts, err := parseOptions(map[string]any{"entity_type": "Directory"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.EntityType != "directory" {
		t.Errorf("EntityType = %q, want %q", opts.EntityType, "directory")
	}
}

func TestParseOptions_InvalidEntityType(t *testing.T) {
	_, err := parseOptions(map[string]any{"entity_type": "symlink"}, nil)
	if err == nil {
		t.Fatal("expected error for invalid entity_type")
	}
}

func TestParseOptions_InvalidFilterPattern(t *testing.T) {
	_, err := parseOptions(map[string]any{"filter": []any{"[invalid"}}, nil)
	if err == nil {
		t.Fatal("expected error for invalid filter pattern")
	}
}

func TestParseOptions_FilterStringSlice(t *testing.T) {
	opts, err := parseOptions(map[string]any{"filter": []string{"*.txt", "*.log"}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts.Filters) != 2 {
		t.Errorf("Filters len = %d, want 2", len(opts.Filters))
	}
}

func TestParseOptions_EmptyFilterItems(t *testing.T) {
	opts, err := parseOptions(map[string]any{"filter": []any{"", "*.go", ""}}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(opts.Filters) != 1 {
		t.Errorf("Filters len = %d, want 1", len(opts.Filters))
	}
}

func TestMatchesTriggerType(t *testing.T) {
	tests := []struct {
		trigger  string
		op       fsbroker.OpType
		expected bool
	}{
		{"FileCreated", fsbroker.Create, true},
		{"FileCreated", fsbroker.Write, false},
		{"FileModified", fsbroker.Write, true},
		{"FileModified", fsbroker.Create, false},
		{"FileDeleted", fsbroker.Remove, true},
		{"FileDeleted", fsbroker.Rename, false},
		{"FileMoved", fsbroker.Rename, true},
		{"FileMoved", fsbroker.Remove, false},
		{"FileAttributeChanged", fsbroker.Chmod, false},
		{"filecreated", fsbroker.Create, true},
		{"FILECREATED", fsbroker.Create, true},
		{"Unknown", fsbroker.Create, false},
	}

	for _, tt := range tests {
		got := matchesTriggerType(tt.trigger, tt.op)
		if got != tt.expected {
			t.Errorf("matchesTriggerType(%q, %v) = %v, want %v", tt.trigger, tt.op, got, tt.expected)
		}
	}
}

func TestMatchesFilter(t *testing.T) {
	tests := []struct {
		name     string
		filters  []string
		path     string
		expected bool
	}{
		{"no filters matches all", nil, "/tmp/foo.txt", true},
		{"empty filters matches all", []string{}, "/tmp/foo.txt", true},
		{"matching pattern", []string{"*.txt"}, "/tmp/foo.txt", true},
		{"non-matching pattern", []string{"*.go"}, "/tmp/foo.txt", false},
		{"multiple patterns, one matches", []string{"*.go", "*.txt"}, "/tmp/foo.txt", true},
		{"multiple patterns, none match", []string{"*.go", "*.md"}, "/tmp/foo.txt", false},
		{"matches on basename only", []string{"*.txt"}, "/deep/nested/path/file.txt", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchesFilter(tt.filters, tt.path)
			if got != tt.expected {
				t.Errorf("matchesFilter(%v, %q) = %v, want %v", tt.filters, tt.path, got, tt.expected)
			}
		})
	}
}

func TestMatchesEntityType(t *testing.T) {
	tests := []struct {
		entityType string
		isDir      bool
		expected   bool
	}{
		{"file", false, true},
		{"file", true, false},
		{"directory", true, true},
		{"directory", false, false},
		{"all", false, true},
		{"all", true, true},
	}

	for _, tt := range tests {
		got := matchesEntityType(tt.entityType, tt.isDir)
		if got != tt.expected {
			t.Errorf("matchesEntityType(%q, %v) = %v, want %v", tt.entityType, tt.isDir, got, tt.expected)
		}
	}
}

func TestBuildContext_FileCreated(t *testing.T) {
	action := &fsbroker.FSAction{
		Type:    fsbroker.Create,
		Subject: &fsbroker.FSInfo{Path: "/tmp/test.txt"},
	}
	ctx := buildContext("FileCreated", action)
	if ctx["FilePath"] != "/tmp/test.txt" {
		t.Errorf("FilePath = %q", ctx["FilePath"])
	}
	if ctx["IsDirectory"] != "false" {
		t.Errorf("IsDirectory = %q", ctx["IsDirectory"])
	}
	if _, ok := ctx["From"]; ok {
		t.Error("FileCreated should not have From")
	}
}

func TestBuildContext_FileMoved(t *testing.T) {
	action := &fsbroker.FSAction{
		Type:    fsbroker.Rename,
		Subject: &fsbroker.FSInfo{Path: "/tmp/new-name.txt"},
	}
	ctx := buildContext("FileMoved", action)
	if ctx["To"] != "/tmp/new-name.txt" {
		t.Errorf("To = %q", ctx["To"])
	}
	if ctx["From"] != "" {
		t.Errorf("From should be empty, got %q", ctx["From"])
	}
	if _, ok := ctx["FilePath"]; ok {
		t.Error("FileMoved should not have FilePath")
	}
}

func TestBuildContext_FileDeleted(t *testing.T) {
	action := &fsbroker.FSAction{
		Type:    fsbroker.Remove,
		Subject: &fsbroker.FSInfo{Path: "/tmp/deleted.txt"},
	}
	ctx := buildContext("FileDeleted", action)
	if ctx["FilePath"] != "/tmp/deleted.txt" {
		t.Errorf("FilePath = %q", ctx["FilePath"])
	}
	if ctx["PermanentlyDeleted"] != "true" {
		t.Errorf("PermanentlyDeleted = %q", ctx["PermanentlyDeleted"])
	}
}

func TestBuildContext_NilSubject(t *testing.T) {
	action := &fsbroker.FSAction{
		Type:    fsbroker.Create,
		Subject: nil,
	}
	ctx := buildContext("FileCreated", action)
	if ctx["FilePath"] != "" {
		t.Errorf("FilePath should be empty, got %q", ctx["FilePath"])
	}
	if ctx["IsDirectory"] != "false" {
		t.Errorf("IsDirectory = %q", ctx["IsDirectory"])
	}
}

func TestIsFileEventType(t *testing.T) {
	valid := []string{"FileCreated", "FileModified", "FileDeleted", "FileMoved",
		"filecreated", "FILEMODIFIED"}
	for _, tt := range valid {
		if !isFileEventType(tt) {
			t.Errorf("isFileEventType(%q) should be true", tt)
		}
	}
	invalid := []string{"Cron", "Interval", "DeviceConnected", "FileAttributeChanged", "Unknown", ""}
	for _, tt := range invalid {
		if isFileEventType(tt) {
			t.Errorf("isFileEventType(%q) should be false", tt)
		}
	}
}
