package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	appbundle "github.com/directedbits/recur/src/infra/fs/appbundle"
	recurv1 "github.com/directedbits/recur/src/infra/grpc/recur/v1"
)

// makeBundle writes a bundle containing a recurfile named rfName plus a script,
// and returns its path.
func makeBundle(t *testing.T, rfName string) string {
	t.Helper()
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, rfName), []byte("App:\n  on: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "scripts", "run.sh"), []byte("echo hi\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	bundle := filepath.Join(t.TempDir(), "download.recur")
	if err := appbundle.Pack(src, bundle); err != nil {
		t.Fatal(err)
	}
	return bundle
}

// runApp executes `app <args...>` with an isolated HOME so AppDir resolves to a
// temp directory. Returns the resolved app base dir.
func runApp(t *testing.T, args ...string) (appBase string, err error) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)

	root := NewRootCmd()
	root.SetArgs(append([]string{"app"}, args...))
	err = root.Execute()
	return filepath.Join(home, ".config", "recur", "app"), err
}

func TestAppInstallRegistersWithDaemon(t *testing.T) {
	svc := &mockService{registerRecurfileResp: &recurv1.RegisterRecurfileResponse{
		Id: "rf-123", TriggerCount: 1, ActionCount: 2,
	}}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	bundle := makeBundle(t, "habits.yaml")
	base, err := runApp(t, "install", bundle)
	if err != nil {
		t.Fatalf("install: %v", err)
	}

	// Name defaults to the recurfile stem.
	installed := filepath.Join(base, "habits")
	if _, err := os.Stat(filepath.Join(installed, "habits.yaml")); err != nil {
		t.Errorf("recurfile not installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(installed, "scripts", "run.sh")); err != nil {
		t.Errorf("script not installed: %v", err)
	}
}

func TestAppInstallNameFlagOverrides(t *testing.T) {
	cleanup := startMockDaemon(t, &mockService{})
	defer cleanup()

	bundle := makeBundle(t, "habits.yaml")
	base, err := runApp(t, "install", bundle, "--name", "custom")
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "custom", "habits.yaml")); err != nil {
		t.Errorf("app not installed under --name: %v", err)
	}
}

func TestAppInstallGenericRecurfileFallsBackToBundleName(t *testing.T) {
	cleanup := startMockDaemon(t, &mockService{})
	defer cleanup()

	// Recurfile uses the generic name; app name should fall back to the bundle.
	bundle := makeBundle(t, "recurfile.yaml")
	base, err := runApp(t, "install", bundle)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "download", "recurfile.yaml")); err != nil {
		t.Errorf("expected app installed under bundle stem 'download': %v", err)
	}
}

func TestAppInstallDaemonDownStillUnpacks(t *testing.T) {
	// No mock daemon: connectFunc fails, so registration is skipped but the
	// bundle must still be unpacked for the startup scan to pick up later.
	bundle := makeBundle(t, "habits.yaml")
	base, err := runApp(t, "install", bundle)
	if err != nil {
		t.Fatalf("install should succeed with daemon down: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "habits", "habits.yaml")); err != nil {
		t.Errorf("bundle not unpacked while daemon down: %v", err)
	}
}

func TestAppRemove(t *testing.T) {
	cleanup := startMockDaemon(t, &mockService{})
	defer cleanup()

	home := t.TempDir()
	t.Setenv("HOME", home)
	base := filepath.Join(home, ".config", "recur", "app")

	// Install then remove within the same HOME.
	bundle := makeBundle(t, "habits.yaml")
	root := NewRootCmd()
	root.SetArgs([]string{"app", "install", bundle})
	if err := root.Execute(); err != nil {
		t.Fatalf("install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "habits")); err != nil {
		t.Fatalf("precondition: app not installed: %v", err)
	}

	root = NewRootCmd()
	root.SetArgs([]string{"app", "remove", "habits"})
	if err := root.Execute(); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, "habits")); !os.IsNotExist(err) {
		t.Errorf("app dir still present after remove: %v", err)
	}
}

func TestAppInstallOverwriteAbortAndForce(t *testing.T) {
	cleanup := startMockDaemon(t, &mockService{})
	defer cleanup()

	home := t.TempDir()
	t.Setenv("HOME", home)
	base := filepath.Join(home, ".config", "recur", "app")
	bundle := makeBundle(t, "habits.yaml")

	install := func(stdin string, extra ...string) error {
		root := NewRootCmd()
		if stdin != "" {
			root.SetIn(strings.NewReader(stdin))
		}
		root.SetArgs(append([]string{"app", "install", bundle}, extra...))
		return root.Execute()
	}

	if err := install(""); err != nil {
		t.Fatalf("first install: %v", err)
	}
	// Sentinel lets us detect whether the app dir was replaced.
	sentinel := filepath.Join(base, "habits", "SENTINEL")
	if err := os.WriteFile(sentinel, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Answering "n" aborts without error and leaves the app intact.
	if err := install("n\n"); err != nil {
		t.Fatalf("aborted install should not error: %v", err)
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Errorf("abort should leave existing app intact: %v", err)
	}

	// --force replaces the app dir (sentinel gone).
	if err := install("", "--force"); err != nil {
		t.Fatalf("force install: %v", err)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Errorf("--force should replace the app dir; sentinel still present")
	}
}

func TestAppPackThenInstall(t *testing.T) {
	cleanup := startMockDaemon(t, &mockService{})
	defer cleanup()

	home := t.TempDir()
	t.Setenv("HOME", home)

	// A source directory with a recurfile and a script.
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "habits.yaml"), []byte("App:\n  on: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "habits.recur")

	root := NewRootCmd()
	root.SetArgs([]string{"app", "pack", src, "-o", out})
	if err := root.Execute(); err != nil {
		t.Fatalf("pack: %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("bundle not created: %v", err)
	}

	// The packed bundle installs cleanly.
	root = NewRootCmd()
	root.SetArgs([]string{"app", "install", out})
	if err := root.Execute(); err != nil {
		t.Fatalf("install packed bundle: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".config", "recur", "app", "habits", "habits.yaml")); err != nil {
		t.Errorf("packed app not installed: %v", err)
	}
}

func TestDefaultAppName(t *testing.T) {
	tests := []struct {
		recurfile string
		source    string
		want      string
	}{
		{"/x/habits.yaml", "/dl/download.recur", "habits"},
		{"/x/recurfile.yaml", "/dl/my-app.recur", "my-app"},
		{"/x/RECURFILE.yml", "/dl/my-app.recur", "my-app"}, // case-insensitive generic
		{"/x/tracker.yml", "https://ex.com/d/tracker.recur", "tracker"},
	}
	for _, tc := range tests {
		if got := defaultAppName(tc.recurfile, tc.source); got != tc.want {
			t.Errorf("defaultAppName(%q, %q) = %q, want %q", tc.recurfile, tc.source, got, tc.want)
		}
	}
}

func TestValidateAppName(t *testing.T) {
	for _, bad := range []string{"", ".", "..", "a/b", `a\b`} {
		if err := validateAppName(bad); err == nil {
			t.Errorf("expected %q to be rejected", bad)
		}
	}
	if err := validateAppName("habits"); err != nil {
		t.Errorf("unexpected rejection: %v", err)
	}
}
