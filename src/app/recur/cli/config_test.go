package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	pkgconfig "github.com/directedbits/recur/pkg/config"
	configyaml "github.com/directedbits/recur/src/infra/yaml/config"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("read: %v", err)
	}
	return buf.String()
}

func TestStoreSourceAnnotation_FileLayer(t *testing.T) {
	store := pkgconfig.NewStore[configyaml.Config]("default", "file", "cli args")
	store.Set("default", *configyaml.DefaultConfig())
	et := 10
	store.Set("file", configyaml.Config{ErrorThreshold: &et})

	got := storeSourceAnnotation("error_threshold", store)
	if got != " (set in configyaml.yaml)" {
		t.Errorf("expected configyaml.yaml annotation, got %q", got)
	}
}

func TestStoreSourceAnnotation_Default(t *testing.T) {
	store := pkgconfig.NewStore[configyaml.Config]("default", "file", "cli args")
	store.Set("default", *configyaml.DefaultConfig())

	got := storeSourceAnnotation("concurrency_mode", store)
	if got != " (inherited from default)" {
		t.Errorf("expected inherited annotation, got %q", got)
	}
}

func TestStoreSourceAnnotation_ThresholdFallback(t *testing.T) {
	store := pkgconfig.NewStore[configyaml.Config]("default", "file", "cli args")
	store.Set("default", *configyaml.DefaultConfig())

	got := storeSourceAnnotation("trigger_error_threshold", store)
	if got != " (inherited from error_threshold)" {
		t.Errorf("expected threshold fallback annotation, got %q", got)
	}
}

func TestStoreSourceAnnotation_NilStore(t *testing.T) {
	got := storeSourceAnnotation("anything", nil)
	if got != "" {
		t.Errorf("expected empty annotation for nil store, got %q", got)
	}
}

func TestStoreSourceAnnotation_CLIArgsLayer(t *testing.T) {
	store := pkgconfig.NewStore[configyaml.Config]("default", "file", "cli args")
	store.Set("default", *configyaml.DefaultConfig())
	ll := "debug"
	store.Set("cli args", configyaml.Config{LogLevel: &ll})

	got := storeSourceAnnotation("log_level", store)
	if got != " (set via CLI flag)" {
		t.Errorf("expected CLI flag annotation, got %q", got)
	}
}

func TestStoreSourceAnnotation_PluginKey(t *testing.T) {
	store := pkgconfig.NewStore[configyaml.Config]("default", "file", "cli args")
	store.Set("default", *configyaml.DefaultConfig())

	got := storeSourceAnnotation("plugins.com.recur.foo.bar", store)
	if got != "" {
		t.Errorf("expected empty annotation for plugin key, got %q", got)
	}
}

func TestPrintAllConfig_NoVerboseHidesAnnotations(t *testing.T) {
	store := pkgconfig.NewStore[configyaml.Config]("default", "file", "cli args")
	store.Set("default", *configyaml.DefaultConfig())
	et := 10
	store.Set("file", configyaml.Config{ErrorThreshold: &et})

	out := captureStdout(t, func() {
		if err := printAllConfig(store, false, false); err != nil {
			t.Fatalf("printAllConfig: %v", err)
		}
	})

	if strings.Contains(out, "(set in configyaml.yaml)") ||
		strings.Contains(out, "(inherited from") ||
		strings.Contains(out, "(set via CLI flag)") {
		t.Errorf("expected no source annotations without --verbose, got:\n%s", out)
	}
	if !strings.Contains(out, "error_threshold") {
		t.Errorf("expected error_threshold key in output, got:\n%s", out)
	}
}

func TestPrintAllConfig_VerboseShowsAnnotations(t *testing.T) {
	store := pkgconfig.NewStore[configyaml.Config]("default", "file", "cli args")
	store.Set("default", *configyaml.DefaultConfig())
	et := 10
	store.Set("file", configyaml.Config{ErrorThreshold: &et})

	out := captureStdout(t, func() {
		if err := printAllConfig(store, false, true); err != nil {
			t.Fatalf("printAllConfig: %v", err)
		}
	})

	if !strings.Contains(out, "= 10 (set in configyaml.yaml)") {
		t.Errorf("expected file annotation under verbose, got:\n%s", out)
	}
	if !strings.Contains(out, "(inherited from default)") {
		t.Errorf("expected default annotation for unset fields under verbose, got:\n%s", out)
	}
}

func TestPrintAllConfig_JSONIgnoresVerbose(t *testing.T) {
	store := pkgconfig.NewStore[configyaml.Config]("default", "file", "cli args")
	store.Set("default", *configyaml.DefaultConfig())

	plain := captureStdout(t, func() {
		if err := printAllConfig(store, true, false); err != nil {
			t.Fatalf("printAllConfig: %v", err)
		}
	})
	verbose := captureStdout(t, func() {
		if err := printAllConfig(store, true, true); err != nil {
			t.Fatalf("printAllConfig: %v", err)
		}
	})

	if plain != verbose {
		t.Errorf("JSON output should be identical regardless of verbose:\n--plain--\n%s\n--verbose--\n%s", plain, verbose)
	}
}
