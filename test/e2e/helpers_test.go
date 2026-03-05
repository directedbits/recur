package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// recurBinary is the path to the shared recur binary, built once in TestMain.
var recurBinary string

// binDir is the shared directory containing both recur and recurd binaries.
var binDir string

// fileeventsPluginDir holds a built fileevents plugin (binary + manifest)
// that tests can install into a per-test HOME via installPlugin.
var fileeventsPluginDir string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "recur-e2e-*")
	if err != nil {
		panic("could not create temp dir: " + err.Error())
	}
	defer os.RemoveAll(tmp)

	binDir = tmp
	recurBinary = filepath.Join(tmp, "recur")

	// Build recur
	cmd := exec.Command("go", "build", "-o", filepath.Join(tmp, "recur"), "../../src/app/recur")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("could not build recur binary: " + err.Error())
	}

	// Build recurd (needed for daemon start/stop tests)
	cmd = exec.Command("go", "build", "-o", filepath.Join(tmp, "recurd"), "../../src/app/recurd")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("could not build recurd binary: " + err.Error())
	}

	// Build the fileevents plugin (binary + manifest copied into one dir)
	// so trigger tests can install it into a per-test plugin dir.
	fileeventsPluginDir = filepath.Join(tmp, "fileevents")
	if err := os.MkdirAll(fileeventsPluginDir, 0755); err != nil {
		panic("could not create fileevents plugin dir: " + err.Error())
	}
	cmd = exec.Command("go", "build", "-o", filepath.Join(fileeventsPluginDir, "fileevents"), "../../plugins/fileevents/")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("could not build fileevents plugin: " + err.Error())
	}
	manifestSrc, err := os.ReadFile("../../plugins/fileevents/manifest.yaml")
	if err != nil {
		panic("could not read fileevents manifest: " + err.Error())
	}
	if err := os.WriteFile(filepath.Join(fileeventsPluginDir, "manifest.yaml"), manifestSrc, 0644); err != nil {
		panic("could not write fileevents manifest: " + err.Error())
	}

	os.Exit(m.Run())
}

// runRecur runs the shared recur binary with the given args and a temporary HOME.
func runRecur(t *testing.T, home string, args ...string) (string, string, int) {
	t.Helper()
	return runBin(t, home, recurBinary, args...)
}

// runBin runs an arbitrary binary with the given HOME and returns stdout, stderr, exit code.
func runBin(t *testing.T, home string, name string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(), "HOME="+home)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", name, err)
		}
	}

	return stdout.String(), stderr.String(), exitCode
}

// runBinInDir runs a binary with a custom working directory.
func runBinInDir(t *testing.T, home string, dir string, name string, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "HOME="+home)

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("failed to run %s: %v", name, err)
		}
	}
	return stdout.String(), stderr.String(), exitCode
}

// startDaemonForTest starts a daemon using the shared binaries and returns
// the recur binary path, home dir, and a cleanup function.
func startDaemonForTest(t *testing.T, plugins ...string) (watchBin string, home string, cleanup func()) {
	t.Helper()
	home = t.TempDir()
	watchBin = recurBinary

	// Install any requested plugins before the daemon starts so it picks
	// them up at startup. Daemon-side discovery scans the plugins dir at
	// boot (and only at boot); installing post-start would require a
	// daemon restart, so we do it up front.
	for _, name := range plugins {
		installPlugin(t, home, name)
	}

	_, stderr, code := runBin(t, home, watchBin, "start")
	if code != 0 {
		t.Fatalf("start exit code = %d, stderr: %s", code, stderr)
	}
	time.Sleep(300 * time.Millisecond)

	cleanup = func() {
		runBin(t, home, watchBin, "stop")
		time.Sleep(200 * time.Millisecond)
	}
	return
}

// installPlugin copies the built plugin from the suite-wide bin dir into
// the per-test HOME's plugins directory so the daemon discovers it at
// startup. Currently supports "fileevents".
func installPlugin(t *testing.T, home string, name string) {
	t.Helper()
	var src string
	switch name {
	case "fileevents":
		src = fileeventsPluginDir
	default:
		t.Fatalf("installPlugin: unknown plugin %q", name)
	}
	dst := filepath.Join(home, ".config", "recur", "plugins", name)
	if err := os.MkdirAll(dst, 0755); err != nil {
		t.Fatalf("installPlugin: mkdir %s: %v", dst, err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("installPlugin: read %s: %v", src, err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatalf("installPlugin: read %s: %v", e.Name(), err)
		}
		mode := os.FileMode(0644)
		if !strings.HasSuffix(e.Name(), ".yaml") {
			mode = 0755 // binary
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), data, mode); err != nil {
			t.Fatalf("installPlugin: write %s: %v", e.Name(), err)
		}
	}
}
