#!/usr/bin/env bash
set -euo pipefail

REPO=$(git rev-parse --show-toplevel)
COVDIR=$(mktemp -d)
MERGED="$COVDIR/merged.out"

cd "$REPO"

echo "Running tests with coverage..."
echo ""

packages=$(go list ./src/... 2>/dev/null)
total_pass=0
total_fail=0

for pkg in $packages; do
    short="${pkg#github.com/directedbits/recur/}"
    covfile="$COVDIR/$(echo "$short" | tr '/' '_').out"

    output=$(go test "$pkg" -coverprofile="$covfile" -count=1 -timeout=120s 2>&1) || true

    if echo "$output" | grep -q "^ok"; then
        cov=$(echo "$output" | grep "^ok" | grep -oP 'coverage: \K[0-9.]+%' || echo "?")
        pass=$(echo "$output" | grep -c "^--- PASS" || true)
        printf "  %-50s  %6s  (%d passed)\n" "$short" "$cov" "$pass"
        total_pass=$((total_pass + pass))
    elif echo "$output" | grep -q "no test files"; then
        printf "  %-50s  %6s\n" "$short" "no tests"
    else
        fail=$(echo "$output" | grep -c "^--- FAIL" || true)
        cov=$(echo "$output" | grep -oP 'coverage: \K[0-9.]+%' || echo "?")
        printf "  %-50s  %6s  (%d FAILED)\n" "$short" "$cov" "$fail"
        total_fail=$((total_fail + fail))
    fi
done

# Merge coverage files
echo "mode: set" > "$MERGED"
for f in "$COVDIR"/*.out; do
    [ -f "$f" ] && grep -v "^mode:" "$f" >> "$MERGED" 2>/dev/null || true
done

echo ""
echo "=========================================="
echo "Total passed: $total_pass"
echo "Total failed: $total_fail"
echo ""

if command -v go &>/dev/null && [ -s "$MERGED" ]; then
    total_line=$(go tool cover -func="$MERGED" 2>/dev/null | tail -1)
    echo "Overall coverage: $(echo "$total_line" | awk '{print $NF}')"
fi

echo ""
echo "Coverage profile: $MERGED"
echo "  View in browser: go tool cover -html=$MERGED"

rm -rf "$COVDIR"
