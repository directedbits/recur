// Package display formats domain values and gRPC errors for terminal output.
// It belongs in infra because rendering is an I/O concern: it writes to
// stdout/stderr, formats for the user's terminal, and emits JSON for
// scripting. Callers (CLI commands) pass in domain or proto values and let
// display turn them into human-readable text.
package displayterminal

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	recurv1 "github.com/directedbits/recur/src/infra/grpc/v1"
	"google.golang.org/grpc/status"
)

// AmbiguousError signals that the user gave an identifier that matched
// multiple entities. The CLI maps this to exit code 2.
type AmbiguousError struct {
	Message string
}

func (e *AmbiguousError) Error() string { return e.Message }

// ExtractAmbiguousDetail returns the AmbiguousEntity detail from a gRPC
// error, or nil if none is present.
func ExtractAmbiguousDetail(err error) *recurv1.AmbiguousEntity {
	st, ok := status.FromError(err)
	if !ok {
		return nil
	}
	for _, detail := range st.Details() {
		if ambiguous, ok := detail.(*recurv1.AmbiguousEntity); ok {
			return ambiguous
		}
	}
	return nil
}

// FormatAmbiguousError extracts an AmbiguousEntity detail from a gRPC error
// and formats it for display. Returns an empty string if the error does not
// contain an AmbiguousEntity detail.
func FormatAmbiguousError(err error) string {
	ambiguous := ExtractAmbiguousDetail(err)
	if ambiguous == nil {
		return ""
	}
	return FormatCandidates(ambiguous)
}

// FormatCandidates builds the human-readable ambiguity message. Group and
// recurfile columns appear only when candidates share a (type, name) pair.
func FormatCandidates(ambiguous *recurv1.AmbiguousEntity) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Ambiguous identifier %q matches %d entities:\n", ambiguous.Identifier, len(ambiguous.Candidates))

	wide := needsWideDisplay(ambiguous.Candidates)
	for _, c := range ambiguous.Candidates {
		if wide {
			fmt.Fprintf(&b, "  %-10s %s  %-16s group=%s  recurfile=%s\n",
				c.EntityType, c.Id, c.Name, valueOrDash(c.Group), valueOrDash(c.Recurfile))
		} else {
			fmt.Fprintf(&b, "  %-10s %s  %s\n", c.EntityType, c.Id, c.Name)
		}
	}
	b.WriteString("Use the full ID from the list above to disambiguate.")
	return b.String()
}

// needsWideDisplay returns true if any two candidates share (entity_type,
// name), meaning group/recurfile context is needed to tell them apart.
func needsWideDisplay(candidates []*recurv1.EntityCandidate) bool {
	type key struct{ typ, name string }
	seen := make(map[key]bool, len(candidates))
	for _, c := range candidates {
		k := key{c.EntityType, c.Name}
		if seen[k] {
			return true
		}
		seen[k] = true
	}
	return false
}

// HandleAmbiguousError checks for ambiguity and returns the appropriate
// error. When jsonFlag is true and the error is ambiguous, it emits
// structured JSON to stdout and returns an AmbiguousError. When jsonFlag is
// false, it returns a formatted AmbiguousError for stderr. Returns nil if
// the error is not ambiguous.
func HandleAmbiguousError(err error, jsonFlag bool) error {
	ambiguous := ExtractAmbiguousDetail(err)
	if ambiguous == nil {
		return nil
	}

	if jsonFlag {
		return EmitAmbiguousJSON(ambiguous)
	}

	return &AmbiguousError{Message: FormatCandidates(ambiguous)}
}

// EmitAmbiguousJSON writes the structured ambiguity payload to stdout.
func EmitAmbiguousJSON(ambiguous *recurv1.AmbiguousEntity) error {
	type candidate struct {
		EntityType string `json:"entity_type"`
		ID         string `json:"id"`
		Name       string `json:"name"`
		Group      string `json:"group,omitempty"`
		Recurfile  string `json:"recurfile,omitempty"`
	}
	payload := struct {
		Error      string      `json:"error"`
		Identifier string      `json:"identifier"`
		Candidates []candidate `json:"candidates"`
	}{
		Error:      "ambiguous_identifier",
		Identifier: ambiguous.Identifier,
	}
	for _, c := range ambiguous.Candidates {
		payload.Candidates = append(payload.Candidates, candidate{
			EntityType: c.EntityType,
			ID:         c.Id,
			Name:       c.Name,
			Group:      c.Group,
			Recurfile:  c.Recurfile,
		})
	}
	data, _ := json.MarshalIndent(payload, "", "  ")
	fmt.Fprintln(os.Stdout, string(data))
	return &AmbiguousError{Message: fmt.Sprintf("ambiguous identifier %q", ambiguous.Identifier)}
}

func valueOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
