#!/usr/bin/env bash
# Usage: scripts/check-coverage.sh [threshold]
# Default threshold: 75
#
# Runs src/ tests with coverage, filters out packages that are not
# meaningfully unit-tested (entry points, CLI surface, generated proto),
# and fails if the resulting total is below the threshold.
#
# In GitHub Actions, also writes a coverage summary to $GITHUB_STEP_SUMMARY.

set -euo pipefail

THRESHOLD="${1:-75}"

# Exclusions:
# - src/cmd/        entry points (main packages, no unit tests expected)
# - src/app/cli/    CLI commands (integration-tested via e2e)
# - *.pb.go         generated protobuf code (any location)
EXCLUDE_PATTERN='src/cmd/|src/app/cli/|\.pb\.go:'

go test -coverprofile=coverage.out ./src/...

head -1 coverage.out > coverage-filtered.out
grep -vE "$EXCLUDE_PATTERN" coverage.out | tail -n +2 >> coverage-filtered.out || true

TOTAL=$(go tool cover -func=coverage-filtered.out | grep '^total:' | awk '{print $3}' | tr -d '%')

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    echo "### Test Coverage: ${TOTAL}%"
    echo "_Excluding cmd/, CLI (e2e-tested), and generated proto code_"
    echo '```'
    go tool cover -func=coverage-filtered.out
    echo '```'
  } >> "$GITHUB_STEP_SUMMARY"
fi

echo "Coverage: ${TOTAL}% (threshold: ${THRESHOLD}%)"

if (( $(echo "$TOTAL < $THRESHOLD" | bc -l) )); then
  echo "FAIL: coverage ${TOTAL}% below ${THRESHOLD}% threshold"
  exit 1
fi
echo "PASS"
