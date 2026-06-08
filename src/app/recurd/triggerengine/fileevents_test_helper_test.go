package triggerengine

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/helshabini/fsbroker"
)

// testFileEventsDriver is a test-only reimplementation of the file events driver
// that was extracted into the external fileevents plugin. It allows the engine
// integration tests to continue testing dispatch logic with real file watchers.
type testFileEventsDriver struct {
	broker     *fsbroker.FSBroker
	watchPath  string
	eventType  string
	filters    []string
	entityType string
	events     chan TriggerEvent
	done       chan struct{}
}

// testFileEventsFactory creates drivers for file event trigger types (test only).
func testFileEventsFactory(triggerID, triggerType string, options map[string]any, recurfilePath string) (Driver, error) {
	if !testIsFileEventType(triggerType) {
		return nil, nil
	}

	watchPath := ""
	if p, ok := options["path"]; ok {
		if s, ok := p.(string); ok && s != "" {
			watchPath = s
		}
	}
	if watchPath == "" && recurfilePath != "" {
		watchPath = filepath.Dir(recurfilePath)
	}
	if watchPath == "" {
		return nil, fmt.Errorf("no watch path specified")
	}

	cfg := fsbroker.DefaultFSConfig()
	if ih, ok := options["ignore_hidden"]; ok {
		if b, ok := ih.(bool); ok {
			cfg.IgnoreHiddenFiles = b
		}
	}
	if is, ok := options["ignore_system"]; ok {
		if b, ok := is.(bool); ok {
			cfg.IgnoreSysFiles = b
		}
	}

	broker, err := fsbroker.NewFSBroker(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating watcher for %s: %w", watchPath, err)
	}

	recursive := false
	if r, ok := options["recursive"]; ok {
		if b, ok := r.(bool); ok {
			recursive = b
		}
	}

	entityType := "all"
	if et, ok := options["entity_type"]; ok {
		if s, ok := et.(string); ok && s != "" {
			entityType = strings.ToLower(s)
		}
	}
	switch entityType {
	case "file", "directory", "all":
		// valid
	default:
		return nil, fmt.Errorf("invalid entity_type %q: must be file, directory, or all", entityType)
	}

	var filters []string
	if f, ok := options["filter"]; ok {
		switch v := f.(type) {
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok && s != "" {
					if _, err := filepath.Match(s, ""); err != nil {
						return nil, fmt.Errorf("invalid filter pattern %q: %w", s, err)
					}
					filters = append(filters, s)
				}
			}
		case []string:
			for _, s := range v {
				if s != "" {
					if _, err := filepath.Match(s, ""); err != nil {
						return nil, fmt.Errorf("invalid filter pattern %q: %w", s, err)
					}
					filters = append(filters, s)
				}
			}
		}
	}

	if recursive {
		if err := broker.AddRecursiveWatch(watchPath); err != nil {
			broker.Stop()
			return nil, fmt.Errorf("watching %s (recursive): %w", watchPath, err)
		}
	} else {
		if err := broker.AddWatch(watchPath); err != nil {
			broker.Stop()
			return nil, fmt.Errorf("watching %s: %w", watchPath, err)
		}
	}

	return &testFileEventsDriver{
		broker:     broker,
		watchPath:  watchPath,
		eventType:  triggerType,
		filters:    filters,
		entityType: entityType,
		events:     make(chan TriggerEvent, 16),
		done:       make(chan struct{}),
	}, nil
}

func (d *testFileEventsDriver) Start() (<-chan TriggerEvent, error) {
	d.broker.Start()
	go d.eventLoop()
	return d.events, nil
}

func (d *testFileEventsDriver) Stop() {
	close(d.done)
	d.broker.Stop()
	for range d.events {
	}
}

func (d *testFileEventsDriver) eventLoop() {
	defer close(d.events)

	for {
		select {
		case <-d.done:
			return
		case action, ok := <-d.broker.Next():
			if !ok {
				return
			}
			if !testMatchesFSOpType(d.eventType, action.Type) {
				continue
			}
			filePath := ""
			isDir := false
			if action.Subject != nil {
				filePath = action.Subject.Path
				isDir = action.Subject.IsDir()
			}
			if !testMatchesFilter(d.filters, filePath) {
				continue
			}
			if !testMatchesEntityType(d.entityType, isDir) {
				continue
			}
			event := TriggerEvent{
				TriggerType: d.eventType,
				Context: map[string]string{
					"FilePath":    filePath,
					"IsDirectory": fmt.Sprintf("%t", isDir),
				},
			}
			select {
			case d.events <- event:
			case <-d.done:
				return
			}
		case err, ok := <-d.broker.Error():
			if !ok {
				return
			}
			_ = err
		}
	}
}

func testMatchesFilter(filters []string, filePath string) bool {
	if len(filters) == 0 {
		return true
	}
	base := filepath.Base(filePath)
	for _, pattern := range filters {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}
	return false
}

func testMatchesEntityType(entityType string, isDir bool) bool {
	switch entityType {
	case "file":
		return !isDir
	case "directory":
		return isDir
	default:
		return true
	}
}

func testIsFileEventType(triggerType string) bool {
	switch strings.ToLower(triggerType) {
	case "filecreated", "filemodified", "filedeleted", "filemoved", "fileattributechanged":
		return true
	default:
		return false
	}
}

func testMatchesFSOpType(triggerType string, opType fsbroker.OpType) bool {
	switch strings.ToLower(triggerType) {
	case "filecreated":
		return opType == fsbroker.Create
	case "filemodified":
		return opType == fsbroker.Write
	case "filedeleted":
		return opType == fsbroker.Remove
	case "filemoved":
		return opType == fsbroker.Rename
	case "fileattributechanged":
		return opType == fsbroker.Chmod
	default:
		return false
	}
}
