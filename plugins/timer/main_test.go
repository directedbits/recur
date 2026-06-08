package main

import (
	"strings"
	"testing"
)

func TestParseInput_Cron(t *testing.T) {
	jsonStr := `{"trigger_type":"Cron","options":{"expression":"*/5 * * * *","timezone":"UTC","fire_on_start":"true"},"config":{}}`
	parsed, err := parseInput(strings.NewReader(jsonStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Input.TriggerType != "Cron" {
		t.Errorf("TriggerType = %q", parsed.Input.TriggerType)
	}
	if parsed.Expression != "*/5 * * * *" {
		t.Errorf("Expression = %q", parsed.Expression)
	}
	if parsed.Timezone != "UTC" {
		t.Errorf("Timezone = %q", parsed.Timezone)
	}
	if !parsed.FireOnStart {
		t.Error("FireOnStart = false, want true")
	}
}

func TestParseInput_CronDefaults(t *testing.T) {
	jsonStr := `{"trigger_type":"Cron","options":{"expression":"0 * * * *"},"config":{}}`
	parsed, err := parseInput(strings.NewReader(jsonStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Timezone != "Local" {
		t.Errorf("Timezone = %q, want Local", parsed.Timezone)
	}
	if parsed.FireOnStart {
		t.Error("FireOnStart = true, want false (default)")
	}
}

func TestParseInput_Interval(t *testing.T) {
	jsonStr := `{"trigger_type":"Interval","options":{"every":"30s"},"config":{}}`
	parsed, err := parseInput(strings.NewReader(jsonStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.Input.TriggerType != "Interval" {
		t.Errorf("TriggerType = %q", parsed.Input.TriggerType)
	}
	if parsed.Every != "30s" {
		t.Errorf("Every = %q", parsed.Every)
	}
}

func TestParseInput_CronMissingExpression(t *testing.T) {
	jsonStr := `{"trigger_type":"Cron","options":{},"config":{}}`
	_, err := parseInput(strings.NewReader(jsonStr))
	if err == nil {
		t.Fatal("expected error for missing expression")
	}
}

func TestParseInput_IntervalMissingEvery(t *testing.T) {
	jsonStr := `{"trigger_type":"Interval","options":{},"config":{}}`
	_, err := parseInput(strings.NewReader(jsonStr))
	if err == nil {
		t.Fatal("expected error for missing every")
	}
}

func TestParseInput_InvalidTriggerType(t *testing.T) {
	jsonStr := `{"trigger_type":"BadType","options":{},"config":{}}`
	_, err := parseInput(strings.NewReader(jsonStr))
	if err == nil {
		t.Fatal("expected error for invalid trigger_type")
	}
}

func TestParseInput_InvalidJSON(t *testing.T) {
	_, err := parseInput(strings.NewReader("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
