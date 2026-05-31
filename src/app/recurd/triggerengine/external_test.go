package triggerengine

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	manifestyaml "github.com/directedbits/recur/src/infra/yaml/manifest"
	pluginfs "github.com/directedbits/recur/src/infra/fs/plugin"
)

// helper to create a mock InstalledPlugin
func newMockPlugin(name, namespace string, triggerNames []string) *pluginfs.InstalledPlugin {
	var triggers []manifestyaml.TriggerDef
	for _, tn := range triggerNames {
		triggers = append(triggers, manifestyaml.TriggerDef{Name: tn})
	}
	return &pluginfs.InstalledPlugin{
		ID:  "test-plugin-id",
		Dir: "/tmp/test-plugin",
		Manifest: &manifestyaml.Manifest{
			Name:      name,
			Namespace: namespace,
			Version:   "1.0.0",
			Triggers:  triggers,
		},
	}
}

func TestExternalPluginFactoryReturnsNilForUnknownType(t *testing.T) {
	plugin := newMockPlugin("test-plugin", "test.plugin", []string{"DeviceConnected"})
	router := NewPluginEventRouter()

	factory := ExternalPluginFactory(plugin, "/tmp/test.sock", router, 0, nil)

	driver, err := factory("trigger-1", "FileCreated", map[string]any{}, "/tmp/watch.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if driver != nil {
		t.Error("expected nil driver for unknown trigger type")
	}
}

func TestExternalPluginFactoryReturnsDriverForKnownType(t *testing.T) {
	plugin := newMockPlugin("test-plugin", "test.plugin", []string{"DeviceConnected"})
	router := NewPluginEventRouter()

	factory := ExternalPluginFactory(plugin, "/tmp/test.sock", router, 0, nil)

	driver, err := factory("trigger-1", "DeviceConnected", map[string]any{"path": "/dev"}, "/tmp/watch.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if driver == nil {
		t.Fatal("expected non-nil driver for known trigger type")
	}

	d := driver.(*externalDriver)
	if d.binaryPath != "/tmp/test-plugin/test-plugin" {
		t.Errorf("binaryPath = %q, want %q", d.binaryPath, "/tmp/test-plugin/test-plugin")
	}
	if d.triggerType != "DeviceConnected" {
		t.Errorf("triggerType = %q, want %q", d.triggerType, "DeviceConnected")
	}
	if d.workDir != "/tmp" {
		t.Errorf("workDir = %q, want %q", d.workDir, "/tmp")
	}
}

func TestExternalPluginFactoryCaseInsensitive(t *testing.T) {
	plugin := newMockPlugin("test-plugin", "test.plugin", []string{"DeviceConnected"})
	router := NewPluginEventRouter()

	factory := ExternalPluginFactory(plugin, "/tmp/test.sock", router, 0, nil)

	// lowercase should match
	driver, err := factory("trigger-1", "deviceconnected", map[string]any{}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if driver == nil {
		t.Error("expected driver for case-insensitive match")
	}
}

func TestFlattenToEnvVarsSimple(t *testing.T) {
	m := map[string]any{
		"path":          "/dev",
		"ignore_hidden": true,
		"poll_interval": 5,
		"threshold":     1.5,
	}

	vars := flattenToEnvVars("", m)
	lookup := make(map[string]string, len(vars))
	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		lookup[parts[0]] = parts[1]
	}

	if lookup["RECUR_PATH"] != "/dev" {
		t.Errorf("RECUR_PATH = %q, want /dev", lookup["RECUR_PATH"])
	}
	if lookup["RECUR_IGNORE_HIDDEN"] != "true" {
		t.Errorf("RECUR_IGNORE_HIDDEN = %q, want true", lookup["RECUR_IGNORE_HIDDEN"])
	}
	if lookup["RECUR_POLL_INTERVAL"] != "5" {
		t.Errorf("RECUR_POLL_INTERVAL = %q, want 5", lookup["RECUR_POLL_INTERVAL"])
	}
	if lookup["RECUR_THRESHOLD"] != "1.5" {
		t.Errorf("RECUR_THRESHOLD = %q, want 1.5", lookup["RECUR_THRESHOLD"])
	}
}

func TestFlattenToEnvVarsList(t *testing.T) {
	m := map[string]any{
		"filter": []any{"*.go", "*.md"},
	}

	vars := flattenToEnvVars("", m)
	if len(vars) != 1 {
		t.Fatalf("expected 1 var, got %d", len(vars))
	}
	if vars[0] != "RECUR_FILTER=*.go,*.md" {
		t.Errorf("got %q, want RECUR_FILTER=*.go,*.md", vars[0])
	}
}

func TestFlattenToEnvVarsNestedMap(t *testing.T) {
	m := map[string]any{
		"env": map[string]any{
			"HOME": "/home/user",
			"LANG": "en_US.UTF-8",
		},
	}

	vars := flattenToEnvVars("", m)
	sort.Strings(vars)

	lookup := make(map[string]string, len(vars))
	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		lookup[parts[0]] = parts[1]
	}

	if lookup["RECUR_ENV_HOME"] != "/home/user" {
		t.Errorf("RECUR_ENV_HOME = %q, want /home/user", lookup["RECUR_ENV_HOME"])
	}
	if lookup["RECUR_ENV_LANG"] != "en_US.UTF-8" {
		t.Errorf("RECUR_ENV_LANG = %q, want en_US.UTF-8", lookup["RECUR_ENV_LANG"])
	}
}

func TestFlattenToEnvVarsDeepNesting(t *testing.T) {
	m := map[string]any{
		"somemap": map[string]any{
			"someobj": map[string]any{
				"somekey": "val",
			},
		},
	}

	vars := flattenToEnvVars("", m)
	if len(vars) != 1 {
		t.Fatalf("expected 1 var, got %d: %v", len(vars), vars)
	}
	if vars[0] != "RECUR_SOMEMAP_SOMEOBJ_SOMEKEY=val" {
		t.Errorf("got %q, want RECUR_SOMEMAP_SOMEOBJ_SOMEKEY=val", vars[0])
	}
}

func TestFlattenToEnvVarsWithPrefix(t *testing.T) {
	m := map[string]any{
		"key": "value",
	}

	vars := flattenToEnvVars("RECUR_CFG", m)
	if len(vars) != 1 {
		t.Fatalf("expected 1 var, got %d", len(vars))
	}
	if vars[0] != "RECUR_CFG_KEY=value" {
		t.Errorf("got %q, want RECUR_CFG_KEY=value", vars[0])
	}
}

func TestExternalDriverBuildEnvVars(t *testing.T) {
	d := &externalDriver{
		socketPath:  "/tmp/watch.sock",
		triggerID:   "abc123",
		triggerType: "DeviceConnected",
		options: map[string]any{
			"device_type": "usb",
		},
		config: map[string]any{
			"poll_interval": 5,
		},
	}

	vars := d.buildEnvVars()
	lookup := make(map[string]string, len(vars))
	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		lookup[parts[0]] = parts[1]
	}

	if lookup["RECUR_SOCKET"] != "/tmp/watch.sock" {
		t.Errorf("RECUR_SOCKET = %q", lookup["RECUR_SOCKET"])
	}
	if lookup["RECUR_TRIGGER_ID"] != "abc123" {
		t.Errorf("RECUR_TRIGGER_ID = %q", lookup["RECUR_TRIGGER_ID"])
	}
	if lookup["RECUR_TRIGGER_TYPE"] != "DeviceConnected" {
		t.Errorf("RECUR_TRIGGER_TYPE = %q", lookup["RECUR_TRIGGER_TYPE"])
	}
	if lookup["RECUR_DEVICE_TYPE"] != "usb" {
		t.Errorf("RECUR_DEVICE_TYPE = %q", lookup["RECUR_DEVICE_TYPE"])
	}
	if lookup["RECUR_POLL_INTERVAL"] != "5" {
		t.Errorf("RECUR_POLL_INTERVAL = %q", lookup["RECUR_POLL_INTERVAL"])
	}
}

func TestExternalDriverStartStopWithMockBinary(t *testing.T) {
	// Build a mock plugin binary that just sleeps
	tmpDir := t.TempDir()
	mockSrc := filepath.Join(tmpDir, "mock.go")
	os.WriteFile(mockSrc, []byte(`package main

import (
	"os"
	"os/signal"
	"syscall"
)

func main() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM)
	<-sig
}
`), 0644)

	mockBin := filepath.Join(tmpDir, "mock-plugin")
	cmd := exec.Command("go", "build", "-o", mockBin, mockSrc)
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building mock plugin: %v\n%s", err, out)
	}

	router := NewPluginEventRouter()
	d := &externalDriver{
		binaryPath:  mockBin,
		socketPath:  "/tmp/nonexistent.sock",
		triggerID:   "test-trigger-1",
		triggerType: "MockEvent",
		options:     map[string]any{},
		config:      map[string]any{},
		workDir:     tmpDir,
		pluginName:  "mock-plugin",
		shutdownTimeout: DefaultShutdownTimeout,
		events:      make(chan TriggerEvent, 16),
		router:      router,
		done:        make(chan struct{}),
		exited:      make(chan struct{}),
	}

	events, err := d.Start()
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if events == nil {
		t.Fatal("expected non-nil events channel")
	}

	// Verify process is running
	if d.cmd.Process == nil {
		t.Fatal("expected process to be running")
	}

	// Stop should terminate gracefully
	d.Stop()

	// Events channel should eventually close (or emit at least once).
	select {
	case <-events:
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for events channel to close")
	}
}

func TestExternalDriverProcessCrash(t *testing.T) {
	// Build a mock plugin binary that exits immediately
	tmpDir := t.TempDir()
	mockSrc := filepath.Join(tmpDir, "crash.go")
	os.WriteFile(mockSrc, []byte(`package main

import "os"

func main() {
	os.Exit(1)
}
`), 0644)

	mockBin := filepath.Join(tmpDir, "crash-plugin")
	cmd := exec.Command("go", "build", "-o", mockBin, mockSrc)
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building crash plugin: %v\n%s", err, out)
	}

	router := NewPluginEventRouter()
	d := &externalDriver{
		binaryPath:  mockBin,
		socketPath:  "/tmp/nonexistent.sock",
		triggerID:   "test-crash-trigger",
		triggerType: "CrashEvent",
		options:     map[string]any{},
		config:      map[string]any{},
		workDir:     tmpDir,
		pluginName:  "crash-plugin",
		shutdownTimeout: DefaultShutdownTimeout,
		events:      make(chan TriggerEvent, 16),
		router:      router,
		done:        make(chan struct{}),
		exited:      make(chan struct{}),
	}

	events, err := d.Start()
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	// Events channel should close when process exits
	select {
	case _, ok := <-events:
		if ok {
			t.Error("expected events channel to be closed, got event")
		}
		// ok == false means channel closed, which is correct
	case <-time.After(10 * time.Second):
		t.Fatal("timeout waiting for events channel to close after crash")
	}
}

func TestExternalDriverEventsFlowThroughRouter(t *testing.T) {
	router := NewPluginEventRouter()
	events := make(chan TriggerEvent, 16)
	triggerID := "test-router-flow"

	router.Register(triggerID, events)

	// Simulate what the gRPC handler does
	event := TriggerEvent{
		TriggerType: "TestEvent",
		Context:     map[string]string{"key": "value"},
	}
	if err := router.Deliver(triggerID, event); err != nil {
		t.Fatalf("deliver: %v", err)
	}

	got := <-events
	if got.TriggerType != "TestEvent" {
		t.Errorf("TriggerType = %q, want TestEvent", got.TriggerType)
	}
	if got.Context["key"] != "value" {
		t.Errorf("Context[key] = %q, want value", got.Context["key"])
	}

	router.Deregister(triggerID)

	// After deregister, deliver should fail
	err := router.Deliver(triggerID, event)
	if err == nil {
		t.Error("expected error after deregister")
	}
}

func TestFactoryDefaultShutdownTimeout(t *testing.T) {
	plugin := newMockPlugin("test-plugin", "test.plugin", []string{"SomeEvent"})
	router := NewPluginEventRouter()

	factory := ExternalPluginFactory(plugin, "/tmp/test.sock", router, 0, nil)
	driver, _ := factory("trigger-1", "SomeEvent", map[string]any{}, "/tmp/watch.yaml")

	d := driver.(*externalDriver)
	if d.shutdownTimeout != DefaultShutdownTimeout {
		t.Errorf("shutdownTimeout = %v, want %v", d.shutdownTimeout, DefaultShutdownTimeout)
	}
}

func TestFactoryCustomShutdownTimeout(t *testing.T) {
	plugin := newMockPlugin("test-plugin", "test.plugin", []string{"SomeEvent"})
	router := NewPluginEventRouter()

	custom := 10 * time.Second
	factory := ExternalPluginFactory(plugin, "/tmp/test.sock", router, custom, nil)
	driver, _ := factory("trigger-1", "SomeEvent", map[string]any{}, "/tmp/watch.yaml")

	d := driver.(*externalDriver)
	if d.shutdownTimeout != custom {
		t.Errorf("shutdownTimeout = %v, want %v", d.shutdownTimeout, custom)
	}
}

// buildMockBinary compiles a Go source string into a binary in tmpDir.
func buildMockBinary(t *testing.T, tmpDir, name, src string) string {
	t.Helper()
	srcPath := filepath.Join(tmpDir, name+".go")
	os.WriteFile(srcPath, []byte(src), 0644)
	binPath := filepath.Join(tmpDir, name)
	cmd := exec.Command("go", "build", "-o", binPath, srcPath)
	cmd.Env = os.Environ()
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("building %s: %v\n%s", name, err, out)
	}
	return binPath
}

func TestStopKillsAfterShutdownTimeout(t *testing.T) {
	// Build a plugin that ignores SIGTERM (forces SIGKILL path)
	tmpDir := t.TempDir()
	stubborn := buildMockBinary(t, tmpDir, "stubborn", `package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	signal.Ignore(syscall.SIGTERM)
	// Keep the process alive so only SIGKILL can stop it
	select {
	case <-time.After(30 * time.Second):
	case <-func() chan os.Signal {
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGKILL)
		return c
	}():
	}
}
`)

	router := NewPluginEventRouter()
	d := &externalDriver{
		binaryPath:      stubborn,
		socketPath:      "/tmp/nonexistent.sock",
		triggerID:       "test-stubborn",
		triggerType:     "StubbornEvent",
		options:         map[string]any{},
		config:          map[string]any{},
		workDir:         tmpDir,
		pluginName:      "stubborn",
		shutdownTimeout: 500 * time.Millisecond, // short timeout to keep test fast
		events:          make(chan TriggerEvent, 16),
		router:          router,
		done:            make(chan struct{}),
		exited:          make(chan struct{}),
	}

	_, err := d.Start()
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	// Let the process fully initialize and start ignoring SIGTERM
	time.Sleep(200 * time.Millisecond)

	// Verify process is still alive
	if d.cmd.ProcessState != nil {
		t.Fatal("process already exited before Stop was called")
	}

	start := time.Now()
	d.Stop()
	elapsed := time.Since(start)

	// Should have waited roughly the shutdown timeout before killing
	if elapsed < 400*time.Millisecond {
		t.Errorf("Stop returned too fast (%v), expected to wait ~500ms for shutdown timeout", elapsed)
	}
	if elapsed > 5*time.Second {
		t.Errorf("Stop took too long (%v), SIGKILL should have fired after 500ms", elapsed)
	}
}

func TestStopGracefulExitBeforeTimeout(t *testing.T) {
	// Build a plugin that exits on SIGTERM immediately
	tmpDir := t.TempDir()
	graceful := buildMockBinary(t, tmpDir, "graceful", `package main

import (
	"os"
	"os/signal"
	"syscall"
)

func main() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM)
	<-sig
}
`)

	router := NewPluginEventRouter()
	d := &externalDriver{
		binaryPath:      graceful,
		socketPath:      "/tmp/nonexistent.sock",
		triggerID:       "test-graceful",
		triggerType:     "GracefulEvent",
		options:         map[string]any{},
		config:          map[string]any{},
		workDir:         tmpDir,
		pluginName:      "graceful",
		shutdownTimeout: 10 * time.Second, // long timeout — should not be reached
		events:          make(chan TriggerEvent, 16),
		router:          router,
		done:            make(chan struct{}),
		exited:          make(chan struct{}),
	}

	_, err := d.Start()
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	start := time.Now()
	d.Stop()
	elapsed := time.Since(start)

	// Should return well before the 10s shutdown timeout
	if elapsed > 3*time.Second {
		t.Errorf("Stop took %v, expected fast return for graceful exit", elapsed)
	}
}

func TestFactoryConfigLookupPopulatesDriverConfig(t *testing.T) {
	plugin := newMockPlugin("test-plugin", "test.plugin", []string{"SomeEvent"})
	router := NewPluginEventRouter()

	lookup := func(namespace string) map[string]any {
		if namespace == "test.plugin" {
			return map[string]any{"api_key": "secret123", "poll_interval": 30}
		}
		return nil
	}

	factory := ExternalPluginFactory(plugin, "/tmp/test.sock", router, 0, lookup)
	driver, err := factory("trigger-1", "SomeEvent", map[string]any{}, "/tmp/watch.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	d := driver.(*externalDriver)
	if d.config == nil {
		t.Fatal("expected non-nil config")
	}
	if d.config["api_key"] != "secret123" {
		t.Errorf("config[api_key] = %v, want secret123", d.config["api_key"])
	}
	if d.config["poll_interval"] != 30 {
		t.Errorf("config[poll_interval] = %v, want 30", d.config["poll_interval"])
	}
}

func TestFactoryNilConfigLookupUsesEmptyMap(t *testing.T) {
	plugin := newMockPlugin("test-plugin", "test.plugin", []string{"SomeEvent"})
	router := NewPluginEventRouter()

	factory := ExternalPluginFactory(plugin, "/tmp/test.sock", router, 0, nil)
	driver, _ := factory("trigger-1", "SomeEvent", map[string]any{}, "/tmp/watch.yaml")

	d := driver.(*externalDriver)
	if d.config == nil {
		t.Fatal("expected non-nil config (empty map)")
	}
	if len(d.config) != 0 {
		t.Errorf("expected empty config, got %v", d.config)
	}
}

func TestFactoryConfigLookupReturningNilUsesEmptyMap(t *testing.T) {
	plugin := newMockPlugin("test-plugin", "test.plugin", []string{"SomeEvent"})
	router := NewPluginEventRouter()

	lookup := func(namespace string) map[string]any {
		return nil // no config for this plugin
	}

	factory := ExternalPluginFactory(plugin, "/tmp/test.sock", router, 0, lookup)
	driver, _ := factory("trigger-1", "SomeEvent", map[string]any{}, "/tmp/watch.yaml")

	d := driver.(*externalDriver)
	if d.config == nil {
		t.Fatal("expected non-nil config (empty map)")
	}
	if len(d.config) != 0 {
		t.Errorf("expected empty config, got %v", d.config)
	}
}

func TestFactoryConfigAppearsInEnvVars(t *testing.T) {
	plugin := newMockPlugin("test-plugin", "test.plugin", []string{"SomeEvent"})
	router := NewPluginEventRouter()

	lookup := func(namespace string) map[string]any {
		return map[string]any{"api_url": "https://example.com"}
	}

	factory := ExternalPluginFactory(plugin, "/tmp/test.sock", router, 0, lookup)
	driver, _ := factory("trigger-1", "SomeEvent", map[string]any{}, "/tmp/watch.yaml")

	d := driver.(*externalDriver)
	vars := d.buildEnvVars()
	envLookup := make(map[string]string, len(vars))
	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		envLookup[parts[0]] = parts[1]
	}

	if envLookup["RECUR_API_URL"] != "https://example.com" {
		t.Errorf("RECUR_API_URL = %q, want https://example.com", envLookup["RECUR_API_URL"])
	}
}

func TestExternalDriverBuildEnvVarsContainsRecurLogLevel(t *testing.T) {
	d := &externalDriver{
		socketPath:  "/tmp/watch.sock",
		triggerID:   "abc123",
		triggerType: "DeviceConnected",
		options:     map[string]any{},
		config:      map[string]any{},
		logLevel:    "debug",
	}

	vars := d.buildEnvVars()
	lookup := make(map[string]string, len(vars))
	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		lookup[parts[0]] = parts[1]
	}

	if lookup["RECUR_LOG_LEVEL"] != "debug" {
		t.Errorf("RECUR_LOG_LEVEL = %q, want %q", lookup["RECUR_LOG_LEVEL"], "debug")
	}
}

func TestExternalPluginFactoryThreadsLogLevel(t *testing.T) {
	plugin := newMockPlugin("test-plugin", "test.plugin", []string{"SomeEvent"})
	router := NewPluginEventRouter()

	factory := ExternalPluginFactory(plugin, "/tmp/test.sock", router, 0, nil, "warn")
	driver, err := factory("trigger-1", "SomeEvent", map[string]any{}, "/tmp/watch.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	d := driver.(*externalDriver)
	if d.logLevel != "warn" {
		t.Errorf("logLevel = %q, want %q", d.logLevel, "warn")
	}
}

func TestFindTriggerDef(t *testing.T) {
	m := &manifestyaml.Manifest{
		Triggers: []manifestyaml.TriggerDef{
			{Name: "DeviceConnected"},
			{Name: "DeviceRemoved"},
		},
	}

	td := FindTriggerDef(m, "DeviceConnected")
	if td == nil {
		t.Fatal("expected to find DeviceConnected")
	}
	if td.Name != "DeviceConnected" {
		t.Errorf("Name = %q, want DeviceConnected", td.Name)
	}

	td = FindTriggerDef(m, "deviceconnected")
	if td == nil {
		t.Fatal("expected case-insensitive match")
	}

	td = FindTriggerDef(m, "NonExistent")
	if td != nil {
		t.Error("expected nil for non-existent trigger")
	}
}
