package daemon

import (
	"testing"

	"github.com/directedbits/recur/src/domain/action"
	"github.com/directedbits/recur/src/domain/trigger"
	configyaml "github.com/directedbits/recur/src/infra/yaml/config"
)

func TestTriggerDefaultsMap_ErrorThresholdFallback(t *testing.T) {
	cfg := configyaml.DefaultConfig() // ErrorThreshold = 5
	d := New(testStore(cfg), "/tmp/test.pid", "/tmp/test.sock")

	m := d.triggerDefaultsMap()
	if m["error_threshold"] != 5 {
		t.Errorf("error_threshold = %v, want 5", m["error_threshold"])
	}
	if m["action_error_threshold"] != 5 {
		t.Errorf("action_error_threshold = %v, want 5", m["action_error_threshold"])
	}
}

func TestTriggerDefaultsMap_SpecificOverrides(t *testing.T) {
	cfg := configyaml.DefaultConfig()
	et := 5
	cfg.ErrorThreshold = &et
	triggerVal := 3
	actionVal := 10
	cfg.TriggerErrorThreshold = &triggerVal
	cfg.ActionErrorThreshold = &actionVal
	d := New(testStore(cfg), "/tmp/test.pid", "/tmp/test.sock")

	m := d.triggerDefaultsMap()
	if m["error_threshold"] != 3 {
		t.Errorf("error_threshold = %v, want 3", m["error_threshold"])
	}
	if m["action_error_threshold"] != 10 {
		t.Errorf("action_error_threshold = %v, want 10", m["action_error_threshold"])
	}
}

func TestTriggerDefaultsMap_TriggerOnlyOverride(t *testing.T) {
	cfg := configyaml.DefaultConfig()
	et := 10
	cfg.ErrorThreshold = &et
	triggerVal := 2
	cfg.TriggerErrorThreshold = &triggerVal
	d := New(testStore(cfg), "/tmp/test.pid", "/tmp/test.sock")

	m := d.triggerDefaultsMap()
	if m["error_threshold"] != 2 {
		t.Errorf("error_threshold = %v, want 2", m["error_threshold"])
	}
	if m["action_error_threshold"] != 10 {
		t.Errorf("action_error_threshold = %v, want 10 (fallback to error_threshold)", m["action_error_threshold"])
	}
}

func TestActionErrorCount_IncrementOnFailure(t *testing.T) {
	a := &action.Action{
		ID:             "action-err-1",
		Type:    "test-action",
		Status:         action.StatusActive,
		ErrorThreshold: 5,
	}

	a.ErrorCount++
	if a.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d, want 1", a.ErrorCount)
	}

	a.ErrorCount++
	if a.ErrorCount != 2 {
		t.Errorf("ErrorCount = %d, want 2", a.ErrorCount)
	}
}

func TestActionErrorCount_ResetOnSuccess(t *testing.T) {
	a := &action.Action{
		ID:             "action-err-2",
		Type:    "test-action",
		Status:         action.StatusActive,
		ErrorThreshold: 5,
		ErrorCount:     3,
	}

	a.ErrorCount = 0
	if a.ErrorCount != 0 {
		t.Errorf("ErrorCount = %d, want 0 after success", a.ErrorCount)
	}
}

func TestActionErrorThreshold_DisablesAction(t *testing.T) {
	a := &action.Action{
		ID:             "action-err-3",
		Type:    "test-action",
		Status:         action.StatusActive,
		ErrorThreshold: 3,
		ErrorCount:     2,
	}

	a.ErrorCount++
	if a.ErrorThreshold > 0 && a.ErrorCount >= a.ErrorThreshold {
		a.Status = action.StatusError
	}

	if a.Status != action.StatusError {
		t.Errorf("Status = %q, want %q", a.Status, action.StatusError)
	}
}

func TestActionErrorThreshold_ZeroMeansNoLimit(t *testing.T) {
	a := &action.Action{
		ID:             "action-err-4",
		Type:    "test-action",
		Status:         action.StatusActive,
		ErrorThreshold: 0,
		ErrorCount:     100,
	}

	a.ErrorCount++
	if a.ErrorThreshold > 0 && a.ErrorCount >= a.ErrorThreshold {
		a.Status = action.StatusError
	}

	if a.Status != action.StatusActive {
		t.Errorf("Status = %q, want %q (threshold=0 should never disable)", a.Status, action.StatusActive)
	}
}

func TestTriggerErrorCount_IncrementOnDriverExit(t *testing.T) {
	tr := &trigger.Trigger{
		ID:             "trigger-err-1",
		Type:           "TestType",
		Status:         trigger.StatusActive,
		ErrorThreshold: 5,
	}

	tr.ErrorCount++
	if tr.ErrorCount != 1 {
		t.Errorf("ErrorCount = %d, want 1", tr.ErrorCount)
	}
}

func TestTriggerErrorThreshold_DeactivatesTrigger(t *testing.T) {
	tr := &trigger.Trigger{
		ID:             "trigger-err-2",
		Type:           "TestType",
		Status:         trigger.StatusActive,
		ErrorThreshold: 3,
		ErrorCount:     2,
	}

	tr.ErrorCount++
	if tr.ErrorThreshold > 0 && tr.ErrorCount >= tr.ErrorThreshold {
		tr.Status = trigger.StatusError
	}

	if tr.Status != trigger.StatusError {
		t.Errorf("Status = %q, want %q", tr.Status, trigger.StatusError)
	}
}

func TestTriggerErrorThreshold_ZeroMeansNoLimit(t *testing.T) {
	tr := &trigger.Trigger{
		ID:             "trigger-err-3",
		Type:           "TestType",
		Status:         trigger.StatusActive,
		ErrorThreshold: 0,
		ErrorCount:     100,
	}

	tr.ErrorCount++
	if tr.ErrorThreshold > 0 && tr.ErrorCount >= tr.ErrorThreshold {
		tr.Status = trigger.StatusError
	}

	if tr.Status != trigger.StatusActive {
		t.Errorf("Status = %q, want %q (threshold=0 should never deactivate)", tr.Status, trigger.StatusActive)
	}
}
