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
func startDaemonForTest(t *testing.T) (watchBin string, home string, cleanup func()) {
	t.Helper()
	home = t.TempDir()
	watchBin = recurBinary

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
