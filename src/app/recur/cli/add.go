package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	domainplugin "github.com/directedbits/recur/src/domain/plugin"
	domainrf "github.com/directedbits/recur/src/domain/recurfile"
	configyaml "github.com/directedbits/recur/src/infra/yaml/config"
	recurv1 "github.com/directedbits/recur/src/infra/grpc/recur/v1"
	pluginfs "github.com/directedbits/recur/src/infra/fs/plugin"
	recurfileyaml "github.com/directedbits/recur/src/infra/yaml/recurfile"
	"github.com/directedbits/recur/src/app/recur/text"
	"github.com/directedbits/recur/src/infra/fs/atomicfile"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [group] [trigger-type]",
		Short: "Add a new group to a recurfile",
		Long: `Add a new group to a recurfile with triggers and actions.

If no triggers or actions are specified, opens $EDITOR for manual editing.
Use --stub to pre-populate trigger/action options from plugin manifests.`,
		Args: cobra.MaximumNArgs(2),
		RunE: runAdd,
	}

	cmd.Flags().Bool("local", true, "Target recur.yaml in current directory (default)")
	cmd.Flags().Bool("user", false, "Target ~/.config/recur/Recurfile.yaml")
	cmd.Flags().String("triggers", "", "Comma-separated trigger types")
	cmd.Flags().String("actions", "", "Comma-separated action names")
	cmd.Flags().Bool("edit", false, "Open $EDITOR after generating stub")
	cmd.Flags().Bool("stub", false, "Pre-populate options from plugin manifests")

	return cmd
}

func runAdd(cmd *cobra.Command, args []string) error {
	localFlag, _ := cmd.Flags().GetBool("local")
	userFlag, _ := cmd.Flags().GetBool("user")
	triggersFlag, _ := cmd.Flags().GetString("triggers")
	actionsFlag, _ := cmd.Flags().GetString("actions")
	editFlag, _ := cmd.Flags().GetBool("edit")
	stubFlag, _ := cmd.Flags().GetBool("stub")

	// Mutually exclusive scope flags
	if userFlag && cmd.Flags().Changed("local") && localFlag {
		return fmt.Errorf("--local and --user are mutually exclusive")
	}

	// Resolve target file path
	targetPath, err := resolveAddTarget(userFlag)
	if err != nil {
		return err
	}

	// Resolve group name
	groupName := resolveGroupName(args, userFlag)

	// Collect triggers from positional arg + --triggers flag
	triggers := collectTriggers(args, triggersFlag)

	// Collect actions from --actions flag
	actions := splitCSV(actionsFlag)

	// Discover plugins and validate trigger/action types against installed manifests
	plugins := discoverPlugins(cmd)
	if err := validateKnownTypes(triggers, actions, plugins); err != nil {
		return err
	}

	// Resolve stub options if --stub is set
	var stubTriggers []recurfileyaml.StubTrigger
	var stubActions []recurfileyaml.StubAction
	if stubFlag {
		stubTriggers = resolveStubTriggers(triggers, plugins)
		stubActions = resolveStubActions(actions, plugins)
	} else {
		for _, t := range triggers {
			stubTriggers = append(stubTriggers, recurfileyaml.StubTrigger{Type: t})
		}
		for _, a := range actions {
			stubActions = append(stubActions, recurfileyaml.StubAction{Type: a})
		}
	}

	// Generate the YAML fragment
	fragment := recurfileyaml.GenerateGroupStub(groupName, stubTriggers, stubActions)

	// Determine if editor should open
	hasTriggers := len(triggers) > 0
	hasActions := len(actions) > 0
	needEditor := editFlag || (!hasTriggers && !hasActions)

	if needEditor {
		fragment, err = runEditor(fragment)
		if err != nil {
			return err
		}
		if fragment == "" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Validate the fragment
	_, err = recurfileyaml.Parse([]byte(fragment))
	if err != nil {
		return fmt.Errorf("invalid recurfile fragment: %w", err)
	}

	// Merge with existing file
	merged, err := mergeFragment(targetPath, fragment)
	if err != nil {
		return err
	}

	// Validate the merged result
	_, err = recurfileyaml.Parse([]byte(merged))
	if err != nil {
		return fmt.Errorf("merged recurfile is invalid: %w", err)
	}

	// Atomic write
	if err := atomicfile.Write(targetPath, []byte(merged)); err != nil {
		return fmt.Errorf("failed to write recurfile: %w", err)
	}

	fmt.Printf("Wrote: %s\n", targetPath)

	// Auto-register with daemon if running
	autoRegister(cmd, targetPath)

	return nil
}

// resolveAddTarget returns the absolute path for the target recurfileyaml.
func resolveAddTarget(userScope bool) (string, error) {
	if userScope {
		dir, err := configyaml.ConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "Recurfile.yaml"), nil
	}
	absPath, err := filepath.Abs("Recurfile.yaml")
	if err != nil {
		return "", fmt.Errorf("could not resolve path: %w", err)
	}
	return absPath, nil
}

// resolveGroupName determines the group name from args or scope defaults.
// With two args, the first is the group name. With one arg, it's the trigger
// type and the group name falls back to the scope default.
func resolveGroupName(args []string, userScope bool) string {
	if len(args) >= 2 {
		return args[0]
	}
	if userScope {
		return "User"
	}
	return "Local"
}

// collectTriggers merges the positional trigger-type arg with --triggers flag values.
// With two args, args[1] is the trigger type. With one arg, args[0] is the trigger type.
func collectTriggers(args []string, triggersFlag string) []string {
	var result []string
	if len(args) == 1 {
		result = append(result, args[0])
	} else if len(args) >= 2 {
		result = append(result, args[1])
	}
	result = append(result, splitCSV(triggersFlag)...)
	return result
}

// splitCSV splits a comma-separated string into trimmed, non-empty values.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// discoverPlugins finds installed plugins via local discovery.
func discoverPlugins(cmd *cobra.Command) []*pluginfs.InstalledPlugin {
	plugins, _ := pluginfs.Discover()
	return plugins
}

// validateKnownTypes checks that every trigger/action type name resolves to a
// built-in or installed plugin manifest. Classification lives in
// domain/pluginfs.UnknownTypes; this function only formats the error message
// for the terminal (close-match suggestions via infra/text).
func validateKnownTypes(triggers, actions []string, plugins []*pluginfs.InstalledPlugin) error {
	domainPlugins := pluginfs.DomainAll(plugins)
	unknownTriggers, unknownActions := domainplugin.UnknownTypes(domainPlugins, triggers, actions)
	if len(unknownTriggers) == 0 && len(unknownActions) == 0 {
		return nil
	}

	knownTriggers := domainplugin.KnownTriggerNames(domainPlugins)
	knownActions := domainplugin.KnownActionNames(domainPlugins)

	var msgs []string
	for _, t := range unknownTriggers {
		msgs = append(msgs, formatUnknownType("trigger", t, knownTriggers))
	}
	for _, a := range unknownActions {
		msgs = append(msgs, formatUnknownType("action", a, knownActions))
	}
	return fmt.Errorf("%s", strings.Join(msgs, "\n"))
}

// formatUnknownType renders a single "unknown X type" error line, appending
// close-match suggestions when any are found.
func formatUnknownType(kind, name string, known []string) string {
	suggestions := text.CloseMatches(name, known, 3)
	if len(suggestions) > 0 {
		return fmt.Sprintf("unknown %s type %q (did you mean: %s?)", kind, name, strings.Join(suggestions, ", "))
	}
	return fmt.Sprintf("unknown %s type %q", kind, name)
}

// resolveStubTriggers builds StubTrigger entries with manifest options when available.
func resolveStubTriggers(triggerTypes []string, plugins []*pluginfs.InstalledPlugin) []recurfileyaml.StubTrigger {
	var result []recurfileyaml.StubTrigger
	for _, tt := range triggerTypes {
		st := recurfileyaml.StubTrigger{Type: tt}
		for _, p := range plugins {
			if def := p.FindTriggerDefinition(tt); def != nil {
				st.Options = def.Options
				break
			}
		}
		result = append(result, st)
	}
	return result
}

// resolveStubActions builds StubAction entries with manifest options when available.
func resolveStubActions(actionNames []string, plugins []*pluginfs.InstalledPlugin) []recurfileyaml.StubAction {
	var result []recurfileyaml.StubAction
	for _, name := range actionNames {
		sa := recurfileyaml.StubAction{Type: name}
		for _, p := range plugins {
			if def := p.FindActionDefinition(name); def != nil {
				sa.Options = def.Options
				sa.Shorthand, _ = p.FindShorthandOption(name)
				break
			}
		}
		result = append(result, sa)
	}
	return result
}

// runEditor writes the fragment to a temp file, opens $EDITOR, and returns the result.
func runEditor(content string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	tmpFile, err := os.CreateTemp("", "recur-add-*.yaml")
	if err != nil {
		return "", fmt.Errorf("could not create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()
		return "", fmt.Errorf("could not write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("could not close temp file: %w", err)
	}

	editorCmd := exec.Command(editor, tmpPath)
	editorCmd.Stdin = os.Stdin
	editorCmd.Stdout = os.Stdout
	editorCmd.Stderr = os.Stderr

	if err := editorCmd.Run(); err != nil {
		return "", fmt.Errorf("editor exited with error: %w", err)
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("could not read temp file: %w", err)
	}

	result := strings.TrimSpace(string(data))
	if result == "" {
		return "", nil
	}
	return result + "\n", nil
}

// mergeFragment reads the existing target file (if any) and merges the new fragment.
func mergeFragment(targetPath, fragment string) (string, error) {
	existing, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fragment, nil
		}
		return "", fmt.Errorf("could not read existing recurfile: %w", err)
	}

	merged, err := domainrf.Merge(existing, []byte(fragment))
	if err != nil {
		return "", err
	}
	return string(merged), nil
}

// autoRegister attempts to register the recurfile with the daemon if running.
// The daemon handles deregister-if-exists atomically during registration.
func autoRegister(cmd *cobra.Command, targetPath string) {
	socketPath, err := resolveSocketPath(cmd)
	if err != nil {
		return
	}

	client := connectOrNilFunc(socketPath)
	if client == nil {
		fmt.Println("Hint: daemon is not running. Register later with: recur register " + targetPath)
		return
	}
	defer func() { _ = client.Close() }()

	absPath, _ := filepath.Abs(targetPath)

	resp, err := client.Service.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{
		Path: absPath,
	})
	if err != nil {
		fmt.Printf("Warning: could not register with daemon: %v\n", err)
		return
	}

	verb := "Registered"
	if resp.Reloaded {
		verb = "Reloaded"
	}
	fmt.Printf("%s: %s (id: %s)\n", verb, resp.Path, resp.Id)
	fmt.Printf("  Triggers: %d\n", resp.TriggerCount)
	fmt.Printf("  Actions:  %d\n", resp.ActionCount)
}
