package triggerengine

import (
	"sync"
	"testing"
)

func TestRouterRegisterAndDeliver(t *testing.T) {
	r := NewPluginEventRouter()
	ch := make(chan TriggerEvent, 1)

	r.Register("trigger-1", ch)

	event := TriggerEvent{
		TriggerType: "TestEvent",
		Context:     map[string]string{"key": "value"},
	}
	if err := r.Deliver("trigger-1", event); err != nil {
		t.Fatalf("deliver: %v", err)
	}

	got := <-ch
	if got.TriggerType != "TestEvent" {
		t.Errorf("TriggerType = %q, want %q", got.TriggerType, "TestEvent")
	}
	if got.Context["key"] != "value" {
		t.Errorf("Context[key] = %q, want %q", got.Context["key"], "value")
	}
}

func TestRouterDeliverUnknownID(t *testing.T) {
	r := NewPluginEventRouter()

	err := r.Deliver("nonexistent", TriggerEvent{TriggerType: "Test"})
	if err == nil {
		t.Fatal("expected error for unknown trigger ID")
	}
}

func TestRouterDeregisterStopsDelivery(t *testing.T) {
	r := NewPluginEventRouter()
	ch := make(chan TriggerEvent, 1)

	r.Register("trigger-2", ch)
	r.Deregister("trigger-2")

	err := r.Deliver("trigger-2", TriggerEvent{TriggerType: "Test"})
	if err == nil {
		t.Fatal("expected error after deregister")
	}
}

func TestRouterDeregisterNonexistent(t *testing.T) {
	r := NewPluginEventRouter()
	// Should not panic
	r.Deregister("nonexistent")
}

func TestRouterChannelFull(t *testing.T) {
	r := NewPluginEventRouter()
	ch := make(chan TriggerEvent) // unbuffered

	r.Register("trigger-full", ch)

	// Deliver should return error since channel is full (no reader)
	err := r.Deliver("trigger-full", TriggerEvent{TriggerType: "Test"})
	if err == nil {
		t.Fatal("expected error for full channel")
	}
}

func TestRouterConcurrentAccess(t *testing.T) {
	r := NewPluginEventRouter()
	const n = 100

	var wg sync.WaitGroup

	// Concurrent register/deliver/deregister
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			triggerID := "trigger-concurrent"
			ch := make(chan TriggerEvent, 1)

			r.Register(triggerID, ch)
			r.Deliver(triggerID, TriggerEvent{TriggerType: "Test"})
			r.Deregister(triggerID)
		}(i)
	}

	wg.Wait()
}

func TestRouterMultipleTriggers(t *testing.T) {
	r := NewPluginEventRouter()
	ch1 := make(chan TriggerEvent, 1)
	ch2 := make(chan TriggerEvent, 1)

	r.Register("trigger-a", ch1)
	r.Register("trigger-b", ch2)

	r.Deliver("trigger-a", TriggerEvent{TriggerType: "EventA"})
	r.Deliver("trigger-b", TriggerEvent{TriggerType: "EventB"})

	got1 := <-ch1
	got2 := <-ch2

	if got1.TriggerType != "EventA" {
		t.Errorf("ch1 got %q, want EventA", got1.TriggerType)
	}
	if got2.TriggerType != "EventB" {
		t.Errorf("ch2 got %q, want EventB", got2.TriggerType)
	}
}
