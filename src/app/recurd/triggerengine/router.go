package triggerengine

import (
	"fmt"
	"sync"
)

// PluginEventRouter maps trigger IDs to event channels, bridging gRPC
// ReportTriggerEvent callbacks into the Driver interface so the engine's
// dispatchLoop works unchanged.
//
// All locking is internal — no mutex or channel is exposed outside the router.
type PluginEventRouter struct {
	mutex    sync.RWMutex
	channels map[string]chan<- TriggerEvent
}

// NewPluginEventRouter creates a new router.
func NewPluginEventRouter() *PluginEventRouter {
	return &PluginEventRouter{
		channels: make(map[string]chan<- TriggerEvent),
	}
}

// Register associates a trigger ID with an event channel.
// Called by externalDriver.Start().
func (r *PluginEventRouter) Register(triggerID string, ch chan<- TriggerEvent) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.channels[triggerID] = ch
}

// Deregister removes a trigger ID from the router.
// Called by externalDriver.Stop().
func (r *PluginEventRouter) Deregister(triggerID string) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.channels, triggerID)
}

// Deliver sends an event to the channel registered for the given trigger ID.
// Returns an error if the trigger ID is not registered or the channel is full.
func (r *PluginEventRouter) Deliver(triggerID string, event TriggerEvent) error {
	r.mutex.RLock()
	ch, ok := r.channels[triggerID]
	r.mutex.RUnlock()

	if !ok {
		return fmt.Errorf("trigger %s is not registered", triggerID)
	}

	select {
	case ch <- event:
		return nil
	default:
		return fmt.Errorf("trigger %s event channel is full", triggerID)
	}
}
