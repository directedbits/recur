package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// ContainerEvent represents a parsed Docker container lifecycle event.
type ContainerEvent struct {
	ContainerID   string
	ContainerName string
	Image         string
	Status        string            // start, stop, die, health_status, etc.
	ExitCode      string            // only for die events
	HealthStatus  string            // only for health_status events
	Labels        map[string]string // container labels (from Actor.Attributes minus reserved keys)
}

// dockerEvent is the raw JSON structure from the Docker Events API.
type dockerEvent struct {
	Type   string `json:"Type"`
	Action string `json:"Action"`
	Actor  struct {
		ID         string            `json:"ID"`
		Attributes map[string]string `json:"Attributes"`
	} `json:"Actor"`
}

// DockerClient communicates with the Docker Engine API over HTTP.
type DockerClient struct {
	httpClient *http.Client
	scheme     string
	addr       string
}

// NewDockerClient creates a client from a host string.
// Supported formats:
//   - unix:///var/run/docker.sock (default)
//   - tcp://host:port
func NewDockerClient(host string) (*DockerClient, error) {
	if host == "" {
		host = "unix:///var/run/docker.sock"
	}

	u, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("parsing host %q: %w", host, err)
	}

	c := &DockerClient{}

	switch u.Scheme {
	case "unix":
		socketPath := u.Path
		c.scheme = "http"
		c.addr = "localhost"
		c.httpClient = &http.Client{
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", socketPath)
				},
			},
		}
	case "tcp":
		c.scheme = "http"
		c.addr = u.Host
		c.httpClient = &http.Client{}
	default:
		return nil, fmt.Errorf("unsupported scheme %q (use unix:// or tcp://)", u.Scheme)
	}

	return c, nil
}

// baseURL returns the base URL for API requests.
func (c *DockerClient) baseURL() string {
	return fmt.Sprintf("%s://%s", c.scheme, c.addr)
}

// Events connects to the Docker Events API and streams container events.
// The returned channels are closed when the context is cancelled or an error occurs.
func (c *DockerClient) Events(ctx context.Context, filters map[string][]string) (<-chan ContainerEvent, <-chan error) {
	events := make(chan ContainerEvent, 64)
	errCh := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errCh)

		// Build the filters JSON for the Docker API.
		apiFilters := map[string][]string{
			"type": {"container"},
		}
		for k, v := range filters {
			apiFilters[k] = v
		}

		filtersJSON, err := json.Marshal(apiFilters)
		if err != nil {
			errCh <- fmt.Errorf("encoding filters: %w", err)
			return
		}

		reqURL := fmt.Sprintf("%s/events?filters=%s", c.baseURL(), url.QueryEscape(string(filtersJSON)))
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			errCh <- fmt.Errorf("creating request: %w", err)
			return
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			errCh <- fmt.Errorf("events request: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			errCh <- fmt.Errorf("events API returned %d: %s", resp.StatusCode, string(body))
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			evt, ok := parseDockerEvent(line)
			if !ok {
				continue
			}

			select {
			case events <- evt:
			case <-ctx.Done():
				return
			}
		}

		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			errCh <- fmt.Errorf("reading events: %w", err)
		}
	}()

	return events, errCh
}

// ContainerStart starts a container by name or ID.
func (c *DockerClient) ContainerStart(ctx context.Context, containerID string) error {
	reqURL := fmt.Sprintf("%s/containers/%s/start", c.baseURL(), url.PathEscape(containerID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("start request: %w", err)
	}
	defer resp.Body.Close()

	// 204 = started, 304 = already started — both are OK.
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("start container %s: %d %s", containerID, resp.StatusCode, string(body))
	}

	return nil
}

// ContainerStop stops a container by name or ID.
func (c *DockerClient) ContainerStop(ctx context.Context, containerID string, timeout int) error {
	reqURL := fmt.Sprintf("%s/containers/%s/stop?t=%d", c.baseURL(), url.PathEscape(containerID), timeout)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("stop request: %w", err)
	}
	defer resp.Body.Close()

	// 204 = stopped, 304 = already stopped — both are OK.
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotModified {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("stop container %s: %d %s", containerID, resp.StatusCode, string(body))
	}

	return nil
}

// ContainerRestart restarts a container by name or ID.
func (c *DockerClient) ContainerRestart(ctx context.Context, containerID string, timeout int) error {
	reqURL := fmt.Sprintf("%s/containers/%s/restart?t=%d", c.baseURL(), url.PathEscape(containerID), timeout)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("restart request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("restart container %s: %d %s", containerID, resp.StatusCode, string(body))
	}

	return nil
}

// parseDockerEvent parses a raw JSON line from the Docker Events API into a ContainerEvent.
// Returns false if the event is not a container event or cannot be parsed.
func parseDockerEvent(data []byte) (ContainerEvent, bool) {
	var raw dockerEvent
	if err := json.Unmarshal(data, &raw); err != nil {
		return ContainerEvent{}, false
	}

	if raw.Type != "container" {
		return ContainerEvent{}, false
	}

	name := raw.Actor.Attributes["name"]
	name = strings.TrimPrefix(name, "/")

	evt := ContainerEvent{
		ContainerID:   raw.Actor.ID,
		ContainerName: name,
		Image:         raw.Actor.Attributes["image"],
		Status:        raw.Action,
		Labels:        extractLabels(raw.Actor.Attributes),
	}

	// For die events, extract exit code.
	if raw.Action == "die" {
		evt.ExitCode = raw.Actor.Attributes["exitCode"]
	}

	// For health_status events, the action contains the status after a colon.
	// e.g., "health_status: healthy"
	if strings.HasPrefix(raw.Action, "health_status") {
		parts := strings.SplitN(raw.Action, ": ", 2)
		if len(parts) == 2 {
			evt.HealthStatus = parts[1]
		}
		evt.Status = "health_status"
	}

	return evt, true
}

// extractLabels returns the container labels from a Docker event's
// Actor.Attributes, omitting the reserved keys Docker uses for built-in
// metadata (name, image, exitCode, execID, signal). Container labels are
// flattened into Attributes alongside these keys; everything else is a label.
func extractLabels(attrs map[string]string) map[string]string {
	if len(attrs) == 0 {
		return nil
	}
	reserved := map[string]bool{
		"name":     true,
		"image":    true,
		"exitCode": true,
		"execID":   true,
		"signal":   true,
	}
	labels := make(map[string]string, len(attrs))
	for k, v := range attrs {
		if reserved[k] {
			continue
		}
		labels[k] = v
	}
	return labels
}

// matchesFilter checks whether a ContainerEvent passes the configured filters.
func matchesFilter(evt ContainerEvent, filterName, filterImage, filterLabel string, labelAttrs map[string]string) bool {
	if filterName != "" && !strings.Contains(evt.ContainerName, filterName) {
		return false
	}
	if filterImage != "" && !strings.Contains(evt.Image, filterImage) {
		return false
	}
	if filterLabel != "" {
		parts := strings.SplitN(filterLabel, "=", 2)
		key := parts[0]
		val, hasLabel := labelAttrs[key]
		if !hasLabel {
			return false
		}
		if len(parts) == 2 && val != parts[1] {
			return false
		}
	}
	return true
}

// optStr extracts a string value from a map with a default fallback.
func optStr(m map[string]any, key, fallback string) string {
	if m == nil {
		return fallback
	}
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return fallback
}
