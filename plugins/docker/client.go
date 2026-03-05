package main

import "context"

// DockerAPI abstracts Docker Engine API operations needed by the plugin.
// This interface enables dependency injection for testing.
type DockerAPI interface {
	// Events streams container lifecycle events. The caller should cancel the
	// context to stop streaming. Filters restrict which events are returned.
	Events(ctx context.Context, filters map[string][]string) (<-chan ContainerEvent, <-chan error)

	// ContainerStart starts a stopped container by name or ID.
	ContainerStart(ctx context.Context, containerID string) error

	// ContainerStop stops a running container, waiting up to timeout seconds.
	ContainerStop(ctx context.Context, containerID string, timeout int) error

	// ContainerRestart restarts a container, waiting up to timeout seconds.
	ContainerRestart(ctx context.Context, containerID string, timeout int) error
}

// dockerAPIFactory creates a DockerAPI from a host string.
type dockerAPIFactory func(host string) (DockerAPI, error)

// defaultDockerAPIFactory creates a real DockerClient.
func defaultDockerAPIFactory(host string) (DockerAPI, error) {
	return NewDockerClient(host)
}

// apiFactory is the package-level factory, overridden in tests.
var apiFactory dockerAPIFactory = defaultDockerAPIFactory
