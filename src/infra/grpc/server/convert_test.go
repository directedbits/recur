package servergrpc

import (
	"testing"

	"github.com/directedbits/recur/src/domain/action"
	recurv1 "github.com/directedbits/recur/src/infra/grpc/v1"
	manifestyaml "github.com/directedbits/recur/src/infra/yaml/manifest"
	pluginfs "github.com/directedbits/recur/src/infra/fs/plugin"
)

func TestDomainStatusToProto(t *testing.T) {
	tests := []struct {
		input string
		want  recurv1.EntityStatus
	}{
		{"active", recurv1.EntityStatus_ENTITY_STATUS_ACTIVE},
		{"suspended", recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED},
		{"error", recurv1.EntityStatus_ENTITY_STATUS_ERROR},
		{"unknown", recurv1.EntityStatus_ENTITY_STATUS_UNSPECIFIED},
		{"", recurv1.EntityStatus_ENTITY_STATUS_UNSPECIFIED},
	}
	for _, tt := range tests {
		got := DomainStatusToProto(tt.input)
		if got != tt.want {
			t.Errorf("DomainStatusToProto(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestExecutionResultToProto(t *testing.T) {
	r := &action.ExecutionResult{
		ActionID:   "act1",
		ActionType: "Shell",
		Success:    true,
		ExitCode:   0,
		Output:     "hello",
		Error:      "",
		Duration:   "100ms",
	}
	proto := ExecutionResultToProto(r)
	if proto.ActionId != "act1" {
		t.Errorf("ActionId = %q", proto.ActionId)
	}
	if proto.ActionType != "Shell" {
		t.Errorf("ActionType = %q", proto.ActionType)
	}
	if !proto.Success {
		t.Error("expected Success = true")
	}
	if proto.ExitCode != 0 {
		t.Errorf("ExitCode = %d", proto.ExitCode)
	}
	if proto.Output != "hello" {
		t.Errorf("Output = %q", proto.Output)
	}
}

func TestPluginToSummary(t *testing.T) {
	p := &pluginfs.InstalledPlugin{
		ID:  "abc123",
		Dir: "/tmp/plugin",
		Manifest: &manifestyaml.Manifest{
			Name:      "testplugin",
			Namespace: "com.test",
			Version:   "1.0.0",
			Triggers:  []manifestyaml.TriggerDef{{Name: "T1"}, {Name: "T2"}},
			Actions:   []manifestyaml.ActionDef{{Name: "A1"}},
		},
	}
	summary := PluginToSummary(p)
	if summary.Id != "abc123" {
		t.Errorf("Id = %q", summary.Id)
	}
	if summary.Name != "testplugin" {
		t.Errorf("Name = %q", summary.Name)
	}
	if summary.TriggerCount != 2 {
		t.Errorf("TriggerCount = %d", summary.TriggerCount)
	}
	if summary.ActionCount != 1 {
		t.Errorf("ActionCount = %d", summary.ActionCount)
	}
}

func TestPluginToDetail(t *testing.T) {
	p := &pluginfs.InstalledPlugin{
		ID:  "abc123",
		Dir: "/tmp/plugin",
		Manifest: &manifestyaml.Manifest{
			Name:        "testplugin",
			Namespace:   "com.test",
			Version:     "1.0.0",
			Description: "A test plugin",
			Triggers:    []manifestyaml.TriggerDef{{Name: "T1"}},
			Actions:     []manifestyaml.ActionDef{{Name: "A1"}},
			Configuration: []manifestyaml.ConfigEntry{
				{Key: "interval", Type: "string", Default: "30s", Description: "Poll interval"},
			},
		},
	}
	detail := PluginToDetail(p)
	if detail.Description != "A test plugin" {
		t.Errorf("Description = %q", detail.Description)
	}
	if len(detail.Triggers) != 1 {
		t.Fatalf("Triggers len = %d", len(detail.Triggers))
	}
	if detail.Triggers[0].Name != "T1" {
		t.Errorf("Trigger name = %q", detail.Triggers[0].Name)
	}
	if len(detail.Actions) != 1 {
		t.Fatalf("Actions len = %d", len(detail.Actions))
	}
	if detail.Actions[0].Name != "A1" {
		t.Errorf("Action name = %q", detail.Actions[0].Name)
	}
	if len(detail.Configuration) != 1 {
		t.Fatalf("Configuration len = %d", len(detail.Configuration))
	}
	if detail.Configuration[0].Key != "interval" {
		t.Errorf("Config key = %q", detail.Configuration[0].Key)
	}
}

func TestPluginToDetail_Empty(t *testing.T) {
	p := &pluginfs.InstalledPlugin{
		ID:  "empty",
		Dir: "/tmp/empty",
		Manifest: &manifestyaml.Manifest{
			Name:      "empty",
			Namespace: "com.empty",
			Version:   "0.1.0",
		},
	}
	detail := PluginToDetail(p)
	if len(detail.Triggers) != 0 {
		t.Errorf("expected 0 triggers, got %d", len(detail.Triggers))
	}
	if len(detail.Actions) != 0 {
		t.Errorf("expected 0 actions, got %d", len(detail.Actions))
	}
	if len(detail.Configuration) != 0 {
		t.Errorf("expected 0 configuration, got %d", len(detail.Configuration))
	}
}
