package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseInput_Trigger(t *testing.T) {
	jsonStr := `{"trigger_type":"MessageReceived","options":{"broker":"tcp://localhost:1883","topic":"test/topic"},"config":{}}`
	input, err := parseInput(strings.NewReader(jsonStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.TriggerType != "MessageReceived" {
		t.Errorf("TriggerType = %q", input.TriggerType)
	}
}

func TestParseInput_Action(t *testing.T) {
	jsonStr := `{"action_type":"Publish","options":{"broker":"tcp://localhost:1883","topic":"test/topic","payload":"hello"},"config":{}}`
	input, err := parseInput(strings.NewReader(jsonStr))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.ActionType != "Publish" {
		t.Errorf("ActionType = %q", input.ActionType)
	}
}

func TestParseInput_NoTypeSet(t *testing.T) {
	jsonStr := `{"options":{},"config":{}}`
	_, err := parseInput(strings.NewReader(jsonStr))
	if err == nil {
		t.Fatal("expected error when neither trigger_type nor action_type is set")
	}
}

func TestParseInput_InvalidTriggerType(t *testing.T) {
	jsonStr := `{"trigger_type":"BadType","options":{},"config":{}}`
	_, err := parseInput(strings.NewReader(jsonStr))
	if err == nil {
		t.Fatal("expected error for invalid trigger_type")
	}
}

func TestParseInput_InvalidActionType(t *testing.T) {
	jsonStr := `{"action_type":"BadAction","options":{},"config":{}}`
	_, err := parseInput(strings.NewReader(jsonStr))
	if err == nil {
		t.Fatal("expected error for invalid action_type")
	}
}

func TestParseInput_InvalidJSON(t *testing.T) {
	_, err := parseInput(strings.NewReader("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestWriteActionOutputTo(t *testing.T) {
	var buf bytes.Buffer
	writeActionOutputTo(&buf, true, "published to test/topic", "")

	var out actionOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !out.Success {
		t.Error("Success = false, want true")
	}
	if out.Output != "published to test/topic" {
		t.Errorf("Output = %q", out.Output)
	}
	if out.Error != "" {
		t.Errorf("Error = %q, want empty", out.Error)
	}
}

func TestWriteActionOutputTo_Error(t *testing.T) {
	var buf bytes.Buffer
	writeActionOutputTo(&buf, false, "", "something went wrong")

	var out actionOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Success {
		t.Error("Success = true, want false")
	}
	if out.Error != "something went wrong" {
		t.Errorf("Error = %q", out.Error)
	}
}
