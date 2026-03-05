package main

import (
	"context"
	"fmt"
	"sync"
)

// mockDockerAPI implements DockerAPI for testing.
type mockDockerAPI struct {
	mu         sync.Mutex
	calls      []string
	events     []ContainerEvent // events to emit from Events()
	startErr   error
	stopErr    error
	restartErr error
	eventsErr  error
}

func (m *mockDockerAPI) Events(ctx context.Context, filters map[string][]string) (<-chan ContainerEvent, <-chan error) {
	ch := make(chan ContainerEvent, len(m.events))
	errCh := make(chan error, 1)

	go func() {
		defer close(ch)
		defer close(errCh)

		if m.eventsErr != nil {
			errCh <- m.eventsErr
			return
		}

		for _, evt := range m.events {
			select {
			case ch <- evt:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch, errCh
}

func (m *mockDockerAPI) ContainerStart(ctx context.Context, containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, fmt.Sprintf("start:%s", containerID))
	return m.startErr
}

func (m *mockDockerAPI) ContainerStop(ctx context.Context, containerID string, timeout int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, fmt.Sprintf("stop:%s:%d", containerID, timeout))
	return m.stopErr
}

func (m *mockDockerAPI) ContainerRestart(ctx context.Context, containerID string, timeout int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, fmt.Sprintf("restart:%s:%d", containerID, timeout))
	return m.restartErr
}
