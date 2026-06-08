// Package trigger provides the generic trigger engine that manages drivers
// and dispatches trigger events to action execution.
//
// The engine is trigger-type agnostic. It delegates event production to
// Driver implementations (file events, D-Bus, cron, etc.) and handles
// lifecycle management, status checking, and action dispatch.
package triggerengine

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	domainaction "github.com/directedbits/recur/src/domain/action"
	domaintrigger "github.com/directedbits/recur/src/domain/trigger"
	executorsubprocess "github.com/directedbits/recur/src/infra/subprocess/executor"
)

// ActionExecutor is called when a trigger fires to execute an action.
type ActionExecutor func(ctx context.Context, a *domainaction.Action, execCtx *executorsubprocess.Context)

// TriggerLookup provides access to trigger-related registry data.
type TriggerLookup interface {
	GetTrigger(id string) *domaintrigger.Trigger
	GetActionsForTrigger(triggerID string) []*domainaction.Action
}

// activeTrigger tracks a running driver for a single trigger.
type activeTrigger struct {
	driver Driver
	cancel context.CancelFunc

	// Concurrency/debounce state
	mu             sync.Mutex
	running        bool
	queue          []TriggerEvent
	debounceTimer  *time.Timer
	debouncePending *TriggerEvent

	// Abort mode state (guarded by mu)
	abortCancel context.CancelFunc // cancel for the currently running action's context, nil when idle
	abortDone   chan struct{}       // closed when the running action's goroutine finishes
}

// Engine manages trigger drivers and dispatches events to actions.
type Engine struct {
	mutex          sync.Mutex
	active         map[string]*activeTrigger // keyed by trigger ID
	factories      []DriverFactory
	lookup         TriggerLookup
	execute        ActionExecutor
	onFired        func()                // called after a trigger fires, used to persist state
	onTriggerError func(triggerID string) // called when a trigger's driver exits unexpectedly
	stopped        bool
}

// NewEngine creates a new trigger engine with the given driver factories.
func NewEngine(lookup TriggerLookup, execute ActionExecutor, factories ...DriverFactory) *Engine {
	return &Engine{
		active:    make(map[string]*activeTrigger),
		factories: factories,
		lookup:    lookup,
		execute:   execute,
	}
}

// SetOnFired sets a callback invoked after a trigger fires and actions execute.
// Used by the daemon to persist state (e.g., LastFired timestamps).
func (e *Engine) SetOnFired(fn func()) {
	e.onFired = fn
}

// SetOnTriggerError sets a callback invoked when a trigger's driver exits
// unexpectedly (e.g., plugin process crash). The daemon uses this to
// increment error counts and enforce thresholds.
func (e *Engine) SetOnTriggerError(fn func(triggerID string)) {
	e.onTriggerError = fn
}

// Activate starts a driver for the given trigger.
func (e *Engine) Activate(t *domaintrigger.Trigger) error {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if e.stopped {
		return fmt.Errorf("engine is stopped")
	}

	if _, ok := e.active[t.ID]; ok {
		return nil // already active
	}

	// Find a factory that handles this trigger type
	var driver Driver
	for _, factory := range e.factories {
		d, err := factory(t.ID, t.Type, t.Options, t.RecurfilePath)
		if err != nil {
			return fmt.Errorf("plugin error for trigger type %s: %w", t.Type, err)
		}
		if d != nil {
			driver = d
			break
		}
	}
	if driver == nil {
		return fmt.Errorf("no plugin found for trigger type %q", t.Type)
	}

	events, err := driver.Start()
	if err != nil {
		driver.Stop()
		return fmt.Errorf("failed to start plugin for trigger type %s: %w", t.Type, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	e.active[t.ID] = &activeTrigger{
		driver: driver,
		cancel: cancel,
	}

	go e.dispatchLoop(ctx, t.ID, events)

	slog.Info("trigger activated", "trigger", t.ID[:8], "type", t.Type)
	return nil
}

// Deactivate stops the driver for the given trigger.
func (e *Engine) Deactivate(triggerID string) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	at, ok := e.active[triggerID]
	if !ok {
		return
	}

	at.mu.Lock()
	if at.debounceTimer != nil {
		at.debounceTimer.Stop()
	}
	at.mu.Unlock()

	at.cancel()
	at.driver.Stop()
	delete(e.active, triggerID)

	// Wait for any running abort-mode action to finish
	at.mu.Lock()
	doneCh := at.abortDone
	at.mu.Unlock()
	if doneCh != nil {
		<-doneCh
	}
}

// StopAll stops all active drivers.
func (e *Engine) StopAll() {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.stopped = true
	for id, at := range e.active {
		at.cancel()
		at.driver.Stop()
		delete(e.active, id)
	}
}

// dispatchLoop reads events from a driver and dispatches them to actions.
func (e *Engine) dispatchLoop(ctx context.Context, triggerID string, events <-chan TriggerEvent) {
	for {
		select {
		case <-ctx.Done():
			return

		case event, ok := <-events:
			if !ok {
				// Channel closed — driver exited (plugin crash or unexpected exit).
				if e.onTriggerError != nil {
					e.onTriggerError(triggerID)
				}
				return
			}
			e.submitEvent(ctx, triggerID, event)
		}
	}
}

// submitEvent applies debounce and concurrency control before dispatching.
func (e *Engine) submitEvent(ctx context.Context, triggerID string, event TriggerEvent) {
	current := e.lookup.GetTrigger(triggerID)
	if current == nil || current.Status != domaintrigger.StatusActive {
		return
	}

	e.mutex.Lock()
	at, ok := e.active[triggerID]
	e.mutex.Unlock()
	if !ok {
		return
	}

	debounce := current.Debounce
	if debounce > 0 {
		at.mu.Lock()
		at.debouncePending = &event
		if at.debounceTimer != nil {
			at.debounceTimer.Stop()
		}
		at.debounceTimer = time.AfterFunc(debounce, func() {
			at.mu.Lock()
			pending := at.debouncePending
			at.debouncePending = nil
			at.mu.Unlock()
			if pending != nil {
				e.dispatchWithConcurrency(ctx, triggerID, *pending, at)
			}
		})
		at.mu.Unlock()
		return
	}

	e.dispatchWithConcurrency(ctx, triggerID, event, at)
}

// dispatchWithConcurrency applies concurrency mode logic to an event.
func (e *Engine) dispatchWithConcurrency(ctx context.Context, triggerID string, event TriggerEvent, at *activeTrigger) {
	current := e.lookup.GetTrigger(triggerID)
	if current == nil || current.Status != domaintrigger.StatusActive {
		return
	}

	mode := current.ConcurrencyMode
	if mode == "" {
		mode = "queue"
	}

	switch mode {
	case "parallel":
		go e.handleEvent(ctx, triggerID, event)

	case "drop":
		at.mu.Lock()
		if at.running {
			at.mu.Unlock()
			slog.Warn("dropping event, action already running", "trigger", triggerID[:8], "mode", "drop")
			return
		}
		at.running = true
		at.mu.Unlock()
		go func() {
			e.handleEvent(ctx, triggerID, event)
			at.mu.Lock()
			at.running = false
			at.mu.Unlock()
		}()

	case "abort":
		at.mu.Lock()
		if at.running {
			at.abortCancel()
			doneCh := at.abortDone
			at.mu.Unlock()
			<-doneCh
			at.mu.Lock()
		}
		actionCtx, actionCancel := context.WithCancel(ctx)
		doneCh := make(chan struct{})
		at.abortCancel = actionCancel
		at.abortDone = doneCh
		at.running = true
		at.mu.Unlock()
		go func() {
			e.handleEvent(actionCtx, triggerID, event)
			at.mu.Lock()
			at.running = false
			at.abortCancel = nil
			at.abortDone = nil
			at.mu.Unlock()
			close(doneCh)
		}()

	default: // "queue"
		e.enqueueAndDrain(ctx, triggerID, event, at, current.MaxQueueSize)
	}
}

// enqueueAndDrain adds an event to the trigger's queue and drains sequentially.
func (e *Engine) enqueueAndDrain(ctx context.Context, triggerID string, event TriggerEvent, at *activeTrigger, maxQueueSize int) {
	if maxQueueSize <= 0 {
		maxQueueSize = 100
	}

	at.mu.Lock()
	if at.running {
		if len(at.queue) >= maxQueueSize {
			at.mu.Unlock()
			slog.Warn("event queue full, dropping event", "trigger", triggerID[:8], "queue_size", maxQueueSize)
			return
		}
		at.queue = append(at.queue, event)
		at.mu.Unlock()
		return
	}
	at.running = true
	at.mu.Unlock()

	go func() {
		e.handleEvent(ctx, triggerID, event)

		for {
			at.mu.Lock()
			if len(at.queue) == 0 {
				at.running = false
				at.mu.Unlock()
				return
			}
			next := at.queue[0]
			at.queue = at.queue[1:]
			at.mu.Unlock()

			e.handleEvent(ctx, triggerID, next)
		}
	}()
}

// handleEvent checks trigger/action status and executes matching actions.
func (e *Engine) handleEvent(ctx context.Context, triggerID string, event TriggerEvent) {
	current := e.lookup.GetTrigger(triggerID)
	if current == nil || current.Status != domaintrigger.StatusActive {
		return
	}

	// Build template context from driver event
	setVars := make(map[string]string, len(event.Context)+2)
	for k, v := range event.Context {
		setVars[k] = v
	}
	setVars["TriggeredOn"] = time.Now().UTC().Format(time.RFC3339)
	setVars["TriggerType"] = event.TriggerType

	execCtx := &executorsubprocess.Context{
		Test: false,
		Set:  setVars,
	}

	current.LastFired = time.Now().UTC()

	slog.Info("trigger event", "trigger", triggerID[:8], "type", event.TriggerType)

	actions := e.lookup.GetActionsForTrigger(triggerID)
	for _, a := range actions {
		if a.Status != domainaction.StatusActive {
			continue
		}
		e.execute(ctx, a, execCtx)
	}

	if e.onFired != nil {
		e.onFired()
	}
}
