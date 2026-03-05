package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestStartInterval_Fires(t *testing.T) {
	events, stop, err := StartInterval("20ms", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer stop()

	select {
	case tick := <-events:
		if tick.TickCount != "1" {
			t.Errorf("TickCount = %q, want %q", tick.TickCount, "1")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for first tick")
	}

	select {
	case tick := <-events:
		if tick.TickCount != "2" {
			t.Errorf("TickCount = %q, want %q", tick.TickCount, "2")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for second tick")
	}
}

func TestStartInterval_FireOnStart(t *testing.T) {
	events, stop, err := StartInterval("1h", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer stop()

	// With fire_on_start=true, should get an immediate event even with 1h interval
	select {
	case tick := <-events:
		if tick.TickCount != "1" {
			t.Errorf("TickCount = %q, want %q", tick.TickCount, "1")
		}
		if tick.TimeSinceStarted != "0s" {
			t.Errorf("TimeSinceStarted = %q, want %q", tick.TimeSinceStarted, "0s")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("fire_on_start=true should fire immediately")
	}
}

func TestStartInterval_NoFireOnStart(t *testing.T) {
	events, stop, err := StartInterval("1h", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer stop()

	// With fire_on_start=false and 1h interval, no event should come quickly
	select {
	case tick := <-events:
		t.Fatalf("should not fire immediately, got tick %+v", tick)
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestStartInterval_InvalidDuration(t *testing.T) {
	_, _, err := StartInterval("not-a-duration", false)
	if err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

func TestStartInterval_NegativeDuration(t *testing.T) {
	_, _, err := StartInterval("-5s", false)
	if err == nil {
		t.Fatal("expected error for negative duration")
	}
}

func TestStartInterval_ZeroDuration(t *testing.T) {
	_, _, err := StartInterval("0s", false)
	if err == nil {
		t.Fatal("expected error for zero duration")
	}
}

func TestStartCron_InvalidExpression(t *testing.T) {
	_, _, err := StartCron("not a cron expression", "Local", false)
	if err == nil {
		t.Fatal("expected error for invalid cron expression")
	}
}

func TestStartCron_InvalidTimezone(t *testing.T) {
	_, _, err := StartCron("* * * * *", "Not/A/Timezone", false)
	if err == nil {
		t.Fatal("expected error for invalid timezone")
	}
}

func TestStartCron_ValidExpression(t *testing.T) {
	events, stop, err := StartCron("* * * * *", "UTC", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer stop()

	// Just verify it starts without error; cron fires at most once per minute
	// so we don't wait for an actual tick in unit tests
	if events == nil {
		t.Fatal("events channel should not be nil")
	}
}

func TestStartCron_FireOnStart(t *testing.T) {
	// Use an expression that won't fire soon (yearly)
	events, stop, err := StartCron("0 0 1 1 *", "UTC", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer stop()

	// fire_on_start=true should produce an immediate event
	select {
	case tick := <-events:
		if tick.TickCount != "1" {
			t.Errorf("TickCount = %q, want %q", tick.TickCount, "1")
		}
		if tick.TimeSinceStarted != "0s" {
			t.Errorf("TimeSinceStarted = %q, want %q", tick.TimeSinceStarted, "0s")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("fire_on_start=true should fire immediately")
	}
}

func TestStartCron_NoFireOnStart(t *testing.T) {
	// Yearly expression, fire_on_start=false — no event expected
	events, stop, err := StartCron("0 0 1 1 *", "UTC", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer stop()

	select {
	case tick := <-events:
		t.Fatalf("should not fire immediately, got tick %+v", tick)
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestStartCron_Presets(t *testing.T) {
	presets := []string{"@yearly", "@monthly", "@weekly", "@daily", "@hourly", "@every 1h"}
	for _, preset := range presets {
		t.Run(preset, func(t *testing.T) {
			_, stop, err := StartCron(preset, "Local", false)
			if err != nil {
				t.Fatalf("preset %q should be valid: %v", preset, err)
			}
			stop()
		})
	}
}

func TestStartCron_Timezones(t *testing.T) {
	timezones := []string{"UTC", "Local", "America/New_York", "Europe/London"}
	for _, tz := range timezones {
		t.Run(tz, func(t *testing.T) {
			_, stop, err := StartCron("* * * * *", tz, false)
			if err != nil {
				t.Fatalf("timezone %q should be valid: %v", tz, err)
			}
			stop()
		})
	}
}

func TestTickCountIncrements(t *testing.T) {
	events, stop, err := StartInterval("10ms", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer stop()

	for i := 1; i <= 5; i++ {
		select {
		case tick := <-events:
			expected := fmt.Sprintf("%d", i)
			if tick.TickCount != expected {
				t.Errorf("tick %d: TickCount = %q, want %q", i, tick.TickCount, expected)
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("timed out waiting for tick %d", i)
		}
	}
}

func TestTimeSinceStarted_Format(t *testing.T) {
	events, stop, err := StartInterval("20ms", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer stop()

	select {
	case tick := <-events:
		// TimeSinceStarted should be a valid Go duration string
		_, parseErr := time.ParseDuration(tick.TimeSinceStarted)
		if parseErr != nil {
			t.Errorf("TimeSinceStarted %q is not a valid duration: %v", tick.TimeSinceStarted, parseErr)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out")
	}
}

func TestPluginInputParsing(t *testing.T) {
	jsonStr := `{
		"trigger_type": "Cron",
		"options": {"expression": "*/5 * * * *", "timezone": "UTC", "fire_on_start": "true"},
		"config": {}
	}`

	var input pluginInput
	if err := json.NewDecoder(strings.NewReader(jsonStr)).Decode(&input); err != nil {
		t.Fatalf("decoding: %v", err)
	}

	if input.TriggerType != "Cron" {
		t.Errorf("TriggerType = %q, want %q", input.TriggerType, "Cron")
	}
	if expr, ok := input.Options["expression"].(string); !ok || expr != "*/5 * * * *" {
		t.Errorf("Options[expression] = %v, want %q", input.Options["expression"], "*/5 * * * *")
	}
	if tz, ok := input.Options["timezone"].(string); !ok || tz != "UTC" {
		t.Errorf("Options[timezone] = %v, want %q", input.Options["timezone"], "UTC")
	}
	if fos, ok := input.Options["fire_on_start"].(string); !ok || fos != "true" {
		t.Errorf("Options[fire_on_start] = %v, want %q", input.Options["fire_on_start"], "true")
	}
}

func TestStartInterval_Stop(t *testing.T) {
	events, stop, err := StartInterval("10ms", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Consume one event
	select {
	case <-events:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out")
	}

	// Stop should close the channel
	stop()

	// Channel should eventually close
	select {
	case _, ok := <-events:
		if ok {
			// Got another event before close, that's fine - drain more
			for range events {
			}
		}
		// channel closed
	case <-time.After(500 * time.Millisecond):
		t.Fatal("channel not closed after stop")
	}
}

func TestStartCron_Stop(t *testing.T) {
	events, stop, err := StartCron("* * * * *", "UTC", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Consume fire_on_start event
	select {
	case <-events:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected fire_on_start event")
	}

	stop()

	// Channel should close
	select {
	case _, ok := <-events:
		if ok {
			for range events {
			}
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("channel not closed after stop")
	}
}

func TestStartInterval_RapidFireOnStart(t *testing.T) {
	// Very short interval with fire_on_start - should get immediate event plus ticker events
	events, stop, err := StartInterval("10ms", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer stop()

	// First event should be immediate (fire_on_start)
	select {
	case tick := <-events:
		if tick.TickCount != "1" {
			t.Errorf("TickCount = %q, want %q", tick.TickCount, "1")
		}
		if tick.TimeSinceStarted != "0s" {
			t.Errorf("TimeSinceStarted = %q, want %q", tick.TimeSinceStarted, "0s")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for fire_on_start event")
	}

	// Second event should come from the ticker
	select {
	case tick := <-events:
		if tick.TickCount != "2" {
			t.Errorf("TickCount = %q, want %q", tick.TickCount, "2")
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for ticker event")
	}
}

func TestStartCron_EveryMinute_FireOnStart(t *testing.T) {
	// Verify fire_on_start works with every-minute cron
	events, stop, err := StartCron("* * * * *", "UTC", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer stop()

	select {
	case tick := <-events:
		if tick.TickCount != "1" {
			t.Errorf("TickCount = %q, want %q", tick.TickCount, "1")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected fire_on_start event")
	}
}

func TestStartCron_LocalTimezone(t *testing.T) {
	_, stop, err := StartCron("0 0 1 1 *", "Local", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	stop()
}

func TestStartInterval_VeryShort(t *testing.T) {
	events, stop, err := StartInterval("1ms", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer stop()

	// Should fire very quickly
	select {
	case <-events:
		// ok
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out")
	}
}

func TestOptString(t *testing.T) {
	opts := map[string]any{
		"expression": "* * * * *",
		"empty":      "",
		"number":     42,
	}

	if got := optString(opts, "expression", "default"); got != "* * * * *" {
		t.Errorf("got %q, want %q", got, "* * * * *")
	}
	if got := optString(opts, "missing", "default"); got != "default" {
		t.Errorf("got %q, want %q", got, "default")
	}
	if got := optString(opts, "empty", "default"); got != "default" {
		t.Errorf("got %q for empty string, want fallback %q", got, "default")
	}
	if got := optString(opts, "number", "default"); got != "default" {
		t.Errorf("got %q for non-string, want fallback %q", got, "default")
	}
}
