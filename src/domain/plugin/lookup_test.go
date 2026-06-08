package plugin

import (
	"reflect"
	"testing"
)

func TestTriggerDefaultsFor_FoundWithDefaults(t *testing.T) {
	plugins := []*Plugin{
		{
			Namespace: "core.timer",
			Triggers: []TriggerType{
				{Name: "Cron", Defaults: map[string]any{"concurrency_mode": "queue", "debounce": "100ms"}},
			},
		},
	}
	ns, defs := TriggerDefaultsFor(plugins, "Cron")
	if ns != "core.timer" {
		t.Errorf("namespace = %q, want core.timer", ns)
	}
	want := map[string]any{"concurrency_mode": "queue", "debounce": "100ms"}
	if !reflect.DeepEqual(defs, want) {
		t.Errorf("defaults = %v, want %v", defs, want)
	}
}

func TestTriggerDefaultsFor_FoundWithoutDefaults(t *testing.T) {
	plugins := []*Plugin{
		{
			Namespace: "core.fileevents",
			Triggers:  []TriggerType{{Name: "FileCreated"}},
		},
	}
	ns, defs := TriggerDefaultsFor(plugins, "FileCreated")
	if ns != "core.fileevents" {
		t.Errorf("namespace = %q, want core.fileevents (returned even without defaults)", ns)
	}
	if defs != nil {
		t.Errorf("defaults = %v, want nil", defs)
	}
}

func TestTriggerDefaultsFor_NotFound(t *testing.T) {
	plugins := []*Plugin{
		{
			Namespace: "core.timer",
			Triggers:  []TriggerType{{Name: "Cron"}},
		},
	}
	ns, defs := TriggerDefaultsFor(plugins, "Nope")
	if ns != "" || defs != nil {
		t.Errorf("got (%q, %v), want (\"\", nil)", ns, defs)
	}
}

func TestTriggerDefaultsFor_NilPlugins(t *testing.T) {
	ns, defs := TriggerDefaultsFor(nil, "Cron")
	if ns != "" || defs != nil {
		t.Errorf("got (%q, %v), want (\"\", nil)", ns, defs)
	}
}

func TestTriggerDefaultsFor_FirstMatchWins(t *testing.T) {
	plugins := []*Plugin{
		{Namespace: "first.timer", Triggers: []TriggerType{{Name: "Cron", Defaults: map[string]any{"x": "1"}}}},
		{Namespace: "second.timer", Triggers: []TriggerType{{Name: "Cron", Defaults: map[string]any{"x": "2"}}}},
	}
	ns, defs := TriggerDefaultsFor(plugins, "Cron")
	if ns != "first.timer" || defs["x"] != "1" {
		t.Errorf("got (%q, %v), want (first.timer, {x:1})", ns, defs)
	}
}
