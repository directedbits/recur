package recurfileyaml

import (
	"strings"
	"testing"

	manifestyaml "github.com/directedbits/recur/src/infra/yaml/manifest"
)

func TestGenerateGroupStub_MinimalGroup(t *testing.T) {
	result := GenerateGroupStub("MyGroup", nil, nil)
	expected := "MyGroup:\n"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestGenerateGroupStub_TriggersOnly(t *testing.T) {
	triggers := []StubTrigger{
		{Type: "Cron"},
		{Type: "FileChanged"},
	}
	result := GenerateGroupStub("Jobs", triggers, nil)

	assertContains(t, result, "Jobs:\n")
	assertContains(t, result, "  on:\n")
	assertContains(t, result, "    - type: Cron\n")
	assertContains(t, result, "    - type: FileChanged\n")
	assertNotContains(t, result, "  do:\n")
}

func TestGenerateGroupStub_ActionsOnly(t *testing.T) {
	actions := []StubAction{
		{Type: "Shell"},
	}
	result := GenerateGroupStub("Tasks", nil, actions)

	assertContains(t, result, "  do:\n")
	assertContains(t, result, "    - type: Shell\n")
	assertNotContains(t, result, "  on:\n")
}

func TestGenerateGroupStub_WithStubOptions(t *testing.T) {
	triggers := []StubTrigger{
		{
			Type: "MessageReceived",
			Options: []manifestyaml.OptionDef{
				{Name: "broker", Type: "string", Description: "Broker URL"},
				{Name: "topic", Type: "string", Description: "Topic filter"},
				{Name: "qos", Type: "string", Default: "0", Description: "QoS level"},
			},
		},
	}
	result := GenerateGroupStub("MQTT", triggers, nil)

	// Required options: uncommented with empty value
	assertContains(t, result, `        broker: ""  # Broker URL`)
	assertContains(t, result, `        topic: ""  # Topic filter`)
	// Optional: commented out with default
	assertContains(t, result, `        # qos: "0"  # QoS level`)
}

func TestGenerateGroupStub_ShorthandAction(t *testing.T) {
	actions := []StubAction{
		{
			Type:      "Publish",
			Shorthand: "topic",
			Options: []manifestyaml.OptionDef{
				{Name: "topic", Type: "string", Shorthand: true, Description: "Topic to publish to"},
				{Name: "qos", Type: "string", Default: "0", Description: "QoS level"},
			},
		},
	}
	result := GenerateGroupStub("Test", nil, actions)

	assertContains(t, result, `    - Publish: ""  # Topic to publish to`)
	assertNotContains(t, result, "type: Publish")
}

func TestGenerateGroupStub_ShorthandActionWithDefault(t *testing.T) {
	actions := []StubAction{
		{
			Type:      "Delay",
			Shorthand: "duration",
			Options: []manifestyaml.OptionDef{
				{Name: "duration", Type: "string", Default: "5s", Shorthand: true, Description: "Wait time"},
			},
		},
	}
	result := GenerateGroupStub("Test", nil, actions)

	assertContains(t, result, `    - Delay: "5s"  # Wait time`)
}

func TestGenerateGroupStub_DetailedActionWithOptions(t *testing.T) {
	actions := []StubAction{
		{
			Type: "Shell",
			Options: []manifestyaml.OptionDef{
				{Name: "command", Type: "string", Description: "Command to run"},
				{Name: "timeout", Type: "string", Default: "30", Description: "Timeout in seconds"},
			},
		},
	}
	result := GenerateGroupStub("Run", nil, actions)

	assertContains(t, result, "    - type: Shell\n")
	assertContains(t, result, "      options:\n")
	assertContains(t, result, `        command: ""  # Command to run`)
	assertContains(t, result, `        # timeout: "30"  # Timeout in seconds`)
}

func TestGenerateGroupStub_OptionWithoutDescription(t *testing.T) {
	triggers := []StubTrigger{
		{
			Type: "Test",
			Options: []manifestyaml.OptionDef{
				{Name: "path", Type: "string"},
				{Name: "interval", Type: "string", Default: "10"},
			},
		},
	}
	result := GenerateGroupStub("G", triggers, nil)

	assertContains(t, result, `        path: ""`)
	assertContains(t, result, `        # interval: "10"`)
	// No trailing comment when no description
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		if strings.Contains(line, "path:") && !strings.Contains(line, "#") {
			// good — no comment
		}
		if strings.Contains(line, "# interval:") && strings.Count(line, "#") > 1 {
			t.Error("expected no description comment for interval")
		}
	}
}

func TestGenerateGroupStub_BoolDefault(t *testing.T) {
	triggers := []StubTrigger{
		{
			Type: "Test",
			Options: []manifestyaml.OptionDef{
				{Name: "verbose", Type: "bool", Default: true},
			},
		},
	}
	result := GenerateGroupStub("G", triggers, nil)
	assertContains(t, result, `# verbose: "true"`)
}

func TestGenerateGroupStub_NumberDefault(t *testing.T) {
	triggers := []StubTrigger{
		{
			Type: "Test",
			Options: []manifestyaml.OptionDef{
				{Name: "count", Type: "number", Default: 42},
			},
		},
	}
	result := GenerateGroupStub("G", triggers, nil)
	assertContains(t, result, `# count: "42"`)
}

func TestGenerateGroupStub_FullExample(t *testing.T) {
	triggers := []StubTrigger{
		{
			Type: "MessageReceived",
			Options: []manifestyaml.OptionDef{
				{Name: "broker", Type: "string", Description: "Broker URL (e.g. tcp://localhost:1883)"},
				{Name: "topic", Type: "string", Description: "Topic filter (supports + and # wildcards)"},
				{Name: "qos", Type: "string", Default: "0", Description: "QoS level (0, 1, or 2)"},
				{Name: "username", Type: "string", Description: "Username for authentication"},
				{Name: "password", Type: "string", Description: "Password or token for authentication"},
				{Name: "client_id", Type: "string", Description: "Client ID (auto-generated if omitted)"},
				{Name: "clean_session", Type: "string", Default: "true", Description: "Start with a clean session"},
				{Name: "keepalive", Type: "string", Default: "30", Description: "Keepalive interval in seconds"},
			},
		},
	}
	actions := []StubAction{
		{
			Type:      "Publish",
			Shorthand: "topic",
			Options: []manifestyaml.OptionDef{
				{Name: "topic", Type: "string", Shorthand: true, Description: "Topic to publish to"},
			},
		},
	}
	result := GenerateGroupStub("HomeAuto", triggers, actions)

	// Verify the structure matches the plan's example
	assertContains(t, result, "HomeAuto:\n")
	assertContains(t, result, "  on:\n")
	assertContains(t, result, "    - type: MessageReceived\n")
	assertContains(t, result, `        broker: ""  # Broker URL`)
	assertContains(t, result, `        topic: ""  # Topic filter`)
	assertContains(t, result, `        # qos: "0"  # QoS level`)
	assertContains(t, result, `        # clean_session: "true"  # Start with a clean session`)
	assertContains(t, result, `        # keepalive: "30"  # Keepalive interval in seconds`)
	assertContains(t, result, "  do:\n")
	assertContains(t, result, `    - Publish: ""  # Topic to publish to`)

	// Validate it parses as valid YAML (via our own parser)
	_, err := Parse([]byte(result))
	if err != nil {
		t.Errorf("generated stub should be valid recurfile YAML, got: %v", err)
	}
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("expected output to contain %q, got:\n%s", substr, s)
	}
}

func assertNotContains(t *testing.T, s, substr string) {
	t.Helper()
	if strings.Contains(s, substr) {
		t.Errorf("expected output NOT to contain %q, got:\n%s", substr, s)
	}
}
