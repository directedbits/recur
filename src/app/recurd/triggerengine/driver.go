package triggerengine

// TriggerEvent represents a generic event produced by a driver.
// Each driver populates Context with driver-specific variables
// (e.g., FilePath, DeviceName) that become available in templates.
type TriggerEvent struct {
	// TriggerType is the specific event type (e.g., "FileCreated", "DeviceConnected").
	TriggerType string
	// Context contains key-value pairs available as template variables.
	Context map[string]string
}

// Driver is the interface that trigger type implementations must satisfy.
// Each driver manages a single trigger instance's event source (file watcher,
// D-Bus subscription, poll loop, etc.) and delivers events via a channel.
type Driver interface {
	// Start begins watching/subscribing/polling for events.
	// The returned channel delivers events until Stop is called.
	Start() (<-chan TriggerEvent, error)

	// Stop shuts down the driver and closes the event channel.
	Stop()
}

// DriverFactory creates a Driver for a given trigger's type and options.
// Returns nil if the factory does not handle the given trigger type.
type DriverFactory func(triggerID, triggerType string, options map[string]any, recurfilePath string) (Driver, error)
