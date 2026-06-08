package plugin

import (
	"reflect"
	"testing"
)

func testPluginsForValidate() []*Plugin {
	return []*Plugin{
		{
			Namespace: "core.timer",
			Triggers:  []TriggerType{{Name: "Cron"}, {Name: "Interval"}},
		},
		{
			Namespace: "core.fileevents",
			Triggers:  []TriggerType{{Name: "FileCreated"}, {Name: "FileChanged"}},
		},
		{
			Namespace: "core.notify",
			Actions:   []ActionType{{Name: "Notify"}},
		},
	}
}

func TestUnknownTypes_AllKnown(t *testing.T) {
	ut, ua := UnknownTypes(
		testPluginsForValidate(),
		[]string{"Cron", "FileCreated"},
		[]string{"Notify", "shell"},
	)
	if len(ut) != 0 || len(ua) != 0 {
		t.Fatalf("expected no unknowns, got triggers=%v actions=%v", ut, ua)
	}
}

func TestUnknownTypes_CaseInsensitive(t *testing.T) {
	ut, ua := UnknownTypes(testPluginsForValidate(), []string{"cron"}, []string{"notify"})
	if len(ut) != 0 || len(ua) != 0 {
		t.Fatalf("expected case-insensitive match, got triggers=%v actions=%v", ut, ua)
	}
}

func TestUnknownTypes_UnknownTrigger(t *testing.T) {
	ut, _ := UnknownTypes(testPluginsForValidate(), []string{"Crom"}, nil)
	if !reflect.DeepEqual(ut, []string{"Crom"}) {
		t.Errorf("got %v, want [Crom]", ut)
	}
}

func TestUnknownTypes_UnknownAction(t *testing.T) {
	_, ua := UnknownTypes(testPluginsForValidate(), nil, []string{"xyzzy"})
	if !reflect.DeepEqual(ua, []string{"xyzzy"}) {
		t.Errorf("got %v, want [xyzzy]", ua)
	}
}

func TestUnknownTypes_PluginNameInsteadOfTrigger(t *testing.T) {
	// User types plugin name "timer" instead of an actual trigger name.
	ut, _ := UnknownTypes(testPluginsForValidate(), []string{"timer"}, nil)
	if !reflect.DeepEqual(ut, []string{"timer"}) {
		t.Errorf("got %v, want [timer]", ut)
	}
}

func TestUnknownTypes_MultipleAndOrdered(t *testing.T) {
	ut, ua := UnknownTypes(
		testPluginsForValidate(),
		[]string{"BadTrig1", "Cron", "BadTrig2"},
		[]string{"shell", "BadAct"},
	)
	if !reflect.DeepEqual(ut, []string{"BadTrig1", "BadTrig2"}) {
		t.Errorf("triggers got %v, want [BadTrig1 BadTrig2]", ut)
	}
	if !reflect.DeepEqual(ua, []string{"BadAct"}) {
		t.Errorf("actions got %v, want [BadAct]", ua)
	}
}

func TestKnownTriggerNames(t *testing.T) {
	got := KnownTriggerNames(testPluginsForValidate())
	want := []string{"Cron", "Interval", "FileCreated", "FileChanged"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestKnownActionNames_PrependsBuiltins(t *testing.T) {
	got := KnownActionNames(testPluginsForValidate())
	want := []string{"shell", "Notify"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestKnownActionNames_NilPlugins(t *testing.T) {
	got := KnownActionNames(nil)
	want := []string{"shell"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestUnknownTypes_NilPluginsRejectsAll(t *testing.T) {
	ut, ua := UnknownTypes(nil, []string{"FileCreated"}, []string{"Notify"})
	if !reflect.DeepEqual(ut, []string{"FileCreated"}) {
		t.Errorf("triggers got %v, want [FileCreated]", ut)
	}
	// "shell" built-in is still accepted; Notify is not.
	if !reflect.DeepEqual(ua, []string{"Notify"}) {
		t.Errorf("actions got %v, want [Notify]", ua)
	}
}
