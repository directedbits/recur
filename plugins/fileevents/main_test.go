package main

import (
	"strings"
	"testing"
)

func TestParseInput_ValidFileCreated(t *testing.T) {
	json := `{"trigger_type":"FileCreated","options":{"path":"/tmp/test","recursive":true},"config":{}}`
	input, opts, err := parseInput(strings.NewReader(json))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.TriggerType != "FileCreated" {
		t.Errorf("TriggerType = %q", input.TriggerType)
	}
	if opts.WatchPath != "/tmp/test" {
		t.Errorf("WatchPath = %q", opts.WatchPath)
	}
	if !opts.Recursive {
		t.Error("Recursive should be true")
	}
}

func TestParseInput_AllTriggerTypes(t *testing.T) {
	types := []string{"FileCreated", "FileModified", "FileDeleted", "FileMoved"}
	for _, tt := range types {
		json := `{"trigger_type":"` + tt + `","options":{"path":"/tmp"},"config":{}}`
		input, _, err := parseInput(strings.NewReader(json))
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", tt, err)
		}
		if input.TriggerType != tt {
			t.Errorf("TriggerType = %q, want %q", input.TriggerType, tt)
		}
	}
}

func TestParseInput_UnsupportedTriggerType(t *testing.T) {
	json := `{"trigger_type":"Cron","options":{},"config":{}}`
	_, _, err := parseInput(strings.NewReader(json))
	if err == nil {
		t.Fatal("expected error for unsupported trigger type")
	}
}

func TestParseInput_InvalidJSON(t *testing.T) {
	_, _, err := parseInput(strings.NewReader("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParseInput_EmptyPathUsesWd(t *testing.T) {
	json := `{"trigger_type":"FileCreated","options":{},"config":{}}`
	_, opts, err := parseInput(strings.NewReader(json))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With no path, should fall back to working directory
	if opts.WatchPath == "" {
		t.Error("WatchPath should not be empty when falling back to wd")
	}
}

func TestParseInput_InvalidEntityType(t *testing.T) {
	json := `{"trigger_type":"FileCreated","options":{"path":"/tmp","entity_type":"symlink"},"config":{}}`
	_, _, err := parseInput(strings.NewReader(json))
	if err == nil {
		t.Fatal("expected error for invalid entity_type")
	}
}

func TestParseInput_InvalidFilterPattern(t *testing.T) {
	json := `{"trigger_type":"FileCreated","options":{"path":"/tmp","filter":["[invalid"]},"config":{}}`
	_, _, err := parseInput(strings.NewReader(json))
	if err == nil {
		t.Fatal("expected error for invalid filter pattern")
	}
}

func TestParseInput_OptionsCarryThrough(t *testing.T) {
	json := `{"trigger_type":"FileModified","options":{"path":"/tmp","recursive":true,"ignore_hidden":false,"ignore_system":false,"entity_type":"all","filter":["*.go"]},"config":{}}`
	_, opts, err := parseInput(strings.NewReader(json))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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
	if opts.EntityType != "all" {
		t.Errorf("EntityType = %q", opts.EntityType)
	}
	if len(opts.Filters) != 1 || opts.Filters[0] != "*.go" {
		t.Errorf("Filters = %v", opts.Filters)
	}
}
