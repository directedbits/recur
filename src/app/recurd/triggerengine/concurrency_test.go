package triggerengine

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	domainaction "github.com/directedbits/recur/src/domain/action"
	domaintrigger "github.com/directedbits/recur/src/domain/trigger"
	executorsubprocess "github.com/directedbits/recur/src/infra/subprocess/executor"
)

// channelDriverFactory creates a DriverFactory that always returns a channelDriver.
func channelDriverFactory(events chan TriggerEvent) DriverFactory {
	return func(triggerID, triggerType string, options map[string]any, recurfilePath string) (Driver, error) {
		return &channelDriver{events: events}, nil
	}
}

func TestDebounceCoalescesRapidEvents(t *testing.T) {
	events := make(chan TriggerEvent, 100)
	var fireCount atomic.Int32

	tr := &domaintrigger.Trigger{
		ID:       "trigger-debounce-test",
		Type:     "Test",
		Options:  map[string]any{},
		Status:   domaintrigger.StatusActive,
		Debounce: 100 * time.Millisecond,
	}

	act := &domainaction.Action{
		ID:     "action-debounce-1",
		Type:   "Shell",
		Status: domainaction.StatusActive,
	}

	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{tr.ID: tr},
		actions:  map[string][]*domainaction.Action{tr.ID: {act}},
	}

	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {
		fireCount.Add(1)
	}, channelDriverFactory(events))

	if err := e.Activate(tr); err != nil {
		t.Fatalf("activate: %v", err)
	}
	defer e.StopAll()

	// Send 5 rapid events within the debounce window
	for i := 0; i < 5; i++ {
		events <- TriggerEvent{TriggerType: "Test"}
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for debounce to fire (100ms after last event + some margin)
	time.Sleep(250 * time.Millisecond)

	count := fireCount.Load()
	if count != 1 {
		t.Errorf("expected 1 debounced fire, got %d", count)
	}
}

func TestQueueModeQueuesAndDrains(t *testing.T) {
	events := make(chan TriggerEvent, 100)
	var mu sync.Mutex
	var fireOrder []int
	actionDone := make(chan struct{})
	firstCall := true

	tr := &domaintrigger.Trigger{
		ID:              "trigger-queue-test",
		Type:            "Test",
		Options:         map[string]any{},
		Status:          domaintrigger.StatusActive,
		ConcurrencyMode: "queue",
		MaxQueueSize:    10,
	}

	act := &domainaction.Action{
		ID:     "action-queue-1",
		Type:   "Shell",
		Status: domainaction.StatusActive,
	}

	callIdx := 0
	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{tr.ID: tr},
		actions:  map[string][]*domainaction.Action{tr.ID: {act}},
	}

	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {
		mu.Lock()
		idx := callIdx
		callIdx++
		fireOrder = append(fireOrder, idx)
		isFirst := firstCall
		firstCall = false
		mu.Unlock()

		if isFirst {
			// First action takes a while, allowing events to queue
			<-actionDone
		}
	}, channelDriverFactory(events))

	if err := e.Activate(tr); err != nil {
		t.Fatalf("activate: %v", err)
	}
	defer e.StopAll()

	// Send first event — starts processing
	events <- TriggerEvent{TriggerType: "Test"}
	time.Sleep(50 * time.Millisecond)

	// Send 3 more events while first is still running
	for i := 0; i < 3; i++ {
		events <- TriggerEvent{TriggerType: "Test"}
		time.Sleep(10 * time.Millisecond)
	}

	// Release the first action
	close(actionDone)

	// Wait for all to drain
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(fireOrder) != 4 {
		t.Errorf("expected 4 fires, got %d", len(fireOrder))
	}
	// Verify sequential ordering
	for i, v := range fireOrder {
		if v != i {
			t.Errorf("fire order[%d] = %d, want %d", i, v, i)
		}
	}
}

func TestQueueModeDropsWhenFull(t *testing.T) {
	events := make(chan TriggerEvent, 100)
	var fireCount atomic.Int32
	actionDone := make(chan struct{})

	tr := &domaintrigger.Trigger{
		ID:              "trigger-queue-full-test",
		Type:            "Test",
		Options:         map[string]any{},
		Status:          domaintrigger.StatusActive,
		ConcurrencyMode: "queue",
		MaxQueueSize:    2,
	}

	act := &domainaction.Action{
		ID:     "action-queue-full-1",
		Type:   "Shell",
		Status: domainaction.StatusActive,
	}

	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{tr.ID: tr},
		actions:  map[string][]*domainaction.Action{tr.ID: {act}},
	}

	first := true
	var mu sync.Mutex
	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {
		fireCount.Add(1)
		mu.Lock()
		isFirst := first
		first = false
		mu.Unlock()
		if isFirst {
			<-actionDone
		}
	}, channelDriverFactory(events))

	if err := e.Activate(tr); err != nil {
		t.Fatalf("activate: %v", err)
	}
	defer e.StopAll()

	// Send first event — starts processing
	events <- TriggerEvent{TriggerType: "Test"}
	time.Sleep(50 * time.Millisecond)

	// Send 5 events while first is running (queue size is 2)
	for i := 0; i < 5; i++ {
		events <- TriggerEvent{TriggerType: "Test"}
		time.Sleep(10 * time.Millisecond)
	}

	close(actionDone)
	time.Sleep(200 * time.Millisecond)

	// Should have: 1 (running) + 2 (queued) = 3 max fires, rest dropped
	count := fireCount.Load()
	if count > 3 {
		t.Errorf("expected at most 3 fires (1 running + 2 queued), got %d", count)
	}
	if count < 1 {
		t.Error("expected at least 1 fire")
	}
}

func TestParallelModeRunsConcurrently(t *testing.T) {
	events := make(chan TriggerEvent, 100)
	var running atomic.Int32
	var maxConcurrent atomic.Int32

	tr := &domaintrigger.Trigger{
		ID:              "trigger-parallel-test",
		Type:            "Test",
		Options:         map[string]any{},
		Status:          domaintrigger.StatusActive,
		ConcurrencyMode: "parallel",
	}

	act := &domainaction.Action{
		ID:     "action-parallel-1",
		Type:   "Shell",
		Status: domainaction.StatusActive,
	}

	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{tr.ID: tr},
		actions:  map[string][]*domainaction.Action{tr.ID: {act}},
	}

	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {
		cur := running.Add(1)
		for {
			prev := maxConcurrent.Load()
			if cur <= prev || maxConcurrent.CompareAndSwap(prev, cur) {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
		running.Add(-1)
	}, channelDriverFactory(events))

	if err := e.Activate(tr); err != nil {
		t.Fatalf("activate: %v", err)
	}
	defer e.StopAll()

	// Send 3 events rapidly
	for i := 0; i < 3; i++ {
		events <- TriggerEvent{TriggerType: "Test"}
	}

	// Wait for all to finish
	time.Sleep(300 * time.Millisecond)

	if maxConcurrent.Load() < 2 {
		t.Errorf("expected concurrent execution, max concurrent = %d", maxConcurrent.Load())
	}
}

func TestDropModeSkipsWhenBusy(t *testing.T) {
	events := make(chan TriggerEvent, 100)
	var fireCount atomic.Int32
	actionDone := make(chan struct{})

	tr := &domaintrigger.Trigger{
		ID:              "trigger-drop-test",
		Type:            "Test",
		Options:         map[string]any{},
		Status:          domaintrigger.StatusActive,
		ConcurrencyMode: "drop",
	}

	act := &domainaction.Action{
		ID:     "action-drop-1",
		Type:   "Shell",
		Status: domainaction.StatusActive,
	}

	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{tr.ID: tr},
		actions:  map[string][]*domainaction.Action{tr.ID: {act}},
	}

	first := true
	var mu sync.Mutex
	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {
		fireCount.Add(1)
		mu.Lock()
		isFirst := first
		first = false
		mu.Unlock()
		if isFirst {
			<-actionDone
		}
	}, channelDriverFactory(events))

	if err := e.Activate(tr); err != nil {
		t.Fatalf("activate: %v", err)
	}
	defer e.StopAll()

	// Send first event — starts processing
	events <- TriggerEvent{TriggerType: "Test"}
	time.Sleep(50 * time.Millisecond)

	// Send 3 more events while first is running — should all be dropped
	for i := 0; i < 3; i++ {
		events <- TriggerEvent{TriggerType: "Test"}
		time.Sleep(10 * time.Millisecond)
	}

	close(actionDone)
	time.Sleep(200 * time.Millisecond)

	count := fireCount.Load()
	if count != 1 {
		t.Errorf("expected exactly 1 fire (drop mode), got %d", count)
	}
}

func TestAbortMode_KillsRunningAction(t *testing.T) {
	events := make(chan TriggerEvent, 100)
	var mu sync.Mutex
	var fireOrder []int
	firstStarted := make(chan struct{})
	callIdx := 0

	tr := &domaintrigger.Trigger{
		ID:              "trigger-abort-test",
		Type:            "Test",
		Options:         map[string]any{},
		Status:          domaintrigger.StatusActive,
		ConcurrencyMode: "abort",
	}

	act := &domainaction.Action{
		ID:     "action-abort-1",
		Type:   "Shell",
		Status: domainaction.StatusActive,
	}

	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{tr.ID: tr},
		actions:  map[string][]*domainaction.Action{tr.ID: {act}},
	}

	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {
		mu.Lock()
		idx := callIdx
		callIdx++
		mu.Unlock()

		if idx == 0 {
			close(firstStarted)
			// Block until cancelled
			<-ctx.Done()
			mu.Lock()
			fireOrder = append(fireOrder, idx)
			mu.Unlock()
			return
		}
		mu.Lock()
		fireOrder = append(fireOrder, idx)
		mu.Unlock()
	}, channelDriverFactory(events))

	if err := e.Activate(tr); err != nil {
		t.Fatalf("activate: %v", err)
	}
	defer e.StopAll()

	// Send first event — starts processing, blocks on ctx.Done
	events <- TriggerEvent{TriggerType: "Test"}
	<-firstStarted

	// Send second event — should abort the first
	events <- TriggerEvent{TriggerType: "Test"}

	// Wait for second to complete
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(fireOrder) != 2 {
		t.Fatalf("expected 2 fires, got %d: %v", len(fireOrder), fireOrder)
	}
	// First action (idx 0) was cancelled, second (idx 1) completed
	if fireOrder[0] != 0 {
		t.Errorf("expected first completed to be idx 0, got %d", fireOrder[0])
	}
	if fireOrder[1] != 1 {
		t.Errorf("expected second completed to be idx 1, got %d", fireOrder[1])
	}
}

func TestAbortMode_NoRunningAction(t *testing.T) {
	events := make(chan TriggerEvent, 100)
	var fireCount atomic.Int32

	tr := &domaintrigger.Trigger{
		ID:              "trigger-abort-single",
		Type:            "Test",
		Options:         map[string]any{},
		Status:          domaintrigger.StatusActive,
		ConcurrencyMode: "abort",
	}

	act := &domainaction.Action{
		ID:     "action-abort-single-1",
		Type:   "Shell",
		Status: domainaction.StatusActive,
	}

	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{tr.ID: tr},
		actions:  map[string][]*domainaction.Action{tr.ID: {act}},
	}

	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {
		fireCount.Add(1)
	}, channelDriverFactory(events))

	if err := e.Activate(tr); err != nil {
		t.Fatalf("activate: %v", err)
	}
	defer e.StopAll()

	events <- TriggerEvent{TriggerType: "Test"}
	time.Sleep(200 * time.Millisecond)

	count := fireCount.Load()
	if count != 1 {
		t.Errorf("expected 1 fire, got %d", count)
	}
}

func TestAbortMode_RapidEvents(t *testing.T) {
	events := make(chan TriggerEvent, 100)
	var mu sync.Mutex
	var completedIndices []int
	callIdx := 0

	tr := &domaintrigger.Trigger{
		ID:              "trigger-abort-rapid",
		Type:            "Test",
		Options:         map[string]any{},
		Status:          domaintrigger.StatusActive,
		ConcurrencyMode: "abort",
	}

	act := &domainaction.Action{
		ID:     "action-abort-rapid-1",
		Type:   "Shell",
		Status: domainaction.StatusActive,
	}

	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{tr.ID: tr},
		actions:  map[string][]*domainaction.Action{tr.ID: {act}},
	}

	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {
		mu.Lock()
		idx := callIdx
		callIdx++
		mu.Unlock()

		// Simulate work that respects cancellation
		select {
		case <-ctx.Done():
			return // aborted — don't record
		case <-time.After(50 * time.Millisecond):
		}

		// Only record if not cancelled after the work
		if ctx.Err() != nil {
			return
		}

		mu.Lock()
		completedIndices = append(completedIndices, idx)
		mu.Unlock()
	}, channelDriverFactory(events))

	if err := e.Activate(tr); err != nil {
		t.Fatalf("activate: %v", err)
	}
	defer e.StopAll()

	// Send 5 rapid events — each should abort the previous
	for i := 0; i < 5; i++ {
		events <- TriggerEvent{TriggerType: "Test"}
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for the last one to complete
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	// Only the last event should complete successfully
	if len(completedIndices) != 1 {
		t.Errorf("expected 1 completed action, got %d: %v", len(completedIndices), completedIndices)
	}
	if len(completedIndices) == 1 && completedIndices[0] != 4 {
		t.Errorf("expected last event (idx 4) to complete, got idx %d", completedIndices[0])
	}
}

func TestDefaultNoDebounceQueueMode(t *testing.T) {
	events := make(chan TriggerEvent, 100)
	var fireCount atomic.Int32

	tr := &domaintrigger.Trigger{
		ID:      "trigger-default-test",
		Type:    "Test",
		Options: map[string]any{},
		Status:  domaintrigger.StatusActive,
		// No ConcurrencyMode, Debounce, or MaxQueueSize set — use defaults
	}

	act := &domainaction.Action{
		ID:     "action-default-1",
		Type:   "Shell",
		Status: domainaction.StatusActive,
	}

	lookup := &mockLookup{
		triggers: map[string]*domaintrigger.Trigger{tr.ID: tr},
		actions:  map[string][]*domainaction.Action{tr.ID: {act}},
	}

	e := NewEngine(lookup, func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context) {
		fireCount.Add(1)
	}, channelDriverFactory(events))

	if err := e.Activate(tr); err != nil {
		t.Fatalf("activate: %v", err)
	}
	defer e.StopAll()

	// Send 3 events
	for i := 0; i < 3; i++ {
		events <- TriggerEvent{TriggerType: "Test"}
	}

	time.Sleep(200 * time.Millisecond)

	count := fireCount.Load()
	if count != 3 {
		t.Errorf("expected 3 fires (no debounce, queue mode), got %d", count)
	}
}
