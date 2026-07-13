package appbundle

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

// writeFile creates a file with the given content and mode under dir.
func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func TestPackUnpackRoundTrip(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "habits.yaml"), "Habit:\n  on: []\n", 0o644)
	writeFile(t, filepath.Join(src, "scripts", "hello.sh"), "#!/bin/sh\necho hi\n", 0o755)

	bundle := filepath.Join(t.TempDir(), "habits.recur")
	if err := Pack(src, bundle); err != nil {
		t.Fatalf("Pack: %v", err)
	}

	dest := t.TempDir()
	if err := Unpack(bundle, dest); err != nil {
		t.Fatalf("Unpack: %v", err)
	}

	// recurfile content preserved
	got, err := os.ReadFile(filepath.Join(dest, "habits.yaml"))
	if err != nil || string(got) != "Habit:\n  on: []\n" {
		t.Fatalf("recurfile content = %q, err %v", got, err)
	}

	// nested script preserved with its executable bit
	info, err := os.Stat(filepath.Join(dest, "scripts", "hello.sh"))
	if err != nil {
		t.Fatalf("stat script: %v", err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("script lost its executable bit: mode %v", info.Mode())
	}
}

func TestPackRejectsBundleWithoutRecurfile(t *testing.T) {
	src := t.TempDir()
	writeFile(t, filepath.Join(src, "scripts", "hello.sh"), "echo hi\n", 0o755)
	if err := Pack(src, filepath.Join(t.TempDir(), "x.recur")); err == nil {
		t.Fatal("expected Pack to reject a bundle with no root recurfile")
	}
}

func TestUnpackRejectsBundleWithoutRecurfile(t *testing.T) {
	// A zip whose only YAML is nested, not at the root.
	bundle := filepath.Join(t.TempDir(), "bad.recur")
	f, err := os.Create(bundle)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, _ := zw.Create("scripts/config.yaml")
	_, _ = w.Write([]byte("nope"))
	_ = zw.Close()
	_ = f.Close()

	if err := Unpack(bundle, t.TempDir()); err == nil {
		t.Fatal("expected Unpack to reject a bundle with no root recurfile")
	}
}

func TestUnpackRejectsTraversal(t *testing.T) {
	bundle := filepath.Join(t.TempDir(), "evil.recur")
	f, err := os.Create(bundle)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	// valid root recurfile so we get past the recurfile check...
	rf, _ := zw.Create("app.yaml")
	_, _ = rf.Write([]byte("x"))
	// ...and a traversal entry that must be rejected
	ev, _ := zw.Create("../escape.txt")
	_, _ = ev.Write([]byte("pwned"))
	_ = zw.Close()
	_ = f.Close()

	dest := filepath.Join(t.TempDir(), "app")
	if err := Unpack(bundle, dest); err == nil {
		t.Fatal("expected Unpack to reject a traversal entry")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dest), "escape.txt")); err == nil {
		t.Fatal("traversal entry escaped the destination")
	}
}

func TestFindRecurfileSingleYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "my-app.yml"), "x", 0o644)
	writeFile(t, filepath.Join(dir, "scripts", "run.sh"), "x", 0o755) // nested, ignored

	got, err := FindRecurfile(dir)
	if err != nil {
		t.Fatalf("FindRecurfile: %v", err)
	}
	if filepath.Base(got) != "my-app.yml" {
		t.Errorf("got %q, want my-app.yml", filepath.Base(got))
	}
}

func TestSelectRecurfile(t *testing.T) {
	tests := []struct {
		name    string
		yamls   []string
		want    string
		wantErr bool
	}{
		{"single arbitrary", []string{"habits.yaml"}, "habits.yaml", false},
		{"none", nil, "", true},
		{"multiple with conventional", []string{"settings.yaml", "recurfile.yaml"}, "recurfile.yaml", false},
		{"multiple ambiguous", []string{"a.yaml", "b.yaml"}, "", true},
		{"multiple conventional ambiguous", []string{"recurfile.yaml", "recurfile.yml"}, "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := selectRecurfile(tc.yamls)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
