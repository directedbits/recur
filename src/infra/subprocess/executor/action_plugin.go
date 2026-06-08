package executorsubprocess

import (
	"encoding/json"
	"time"
)

// ActionPluginInput is the JSON payload sent to a plugin binary's stdin
// when executing an action.
type ActionPluginInput struct {
	ActionType string         `json:"action_type"`
	Options    map[string]any `json:"options"`
	Config     map[string]any `json:"config"`
	Test       bool           `json:"test"`
}

// ActionPluginOutput is the JSON response read from the plugin binary's stdout.
type ActionPluginOutput struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error"`
}

// ActionPluginRequest builds a Request that spawns the plugin binary with
// the ActionPluginInput serialized as JSON on stdin.
func ActionPluginRequest(binaryPath string, input *ActionPluginInput, env []string, workDir string, timeout time.Duration) (*Request, error) {
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}

	return &Request{
		Command:    binaryPath,
		Stdin:      string(payload),
		Env:        env,
		WorkingDir: workDir,
		Timeout:    timeout,
	}, nil
}

// ParseActionPluginOutput parses the stdout of a plugin action binary.
func ParseActionPluginOutput(stdout string) (*ActionPluginOutput, error) {
	var out ActionPluginOutput
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		return nil, err
	}
	return &out, nil
}
