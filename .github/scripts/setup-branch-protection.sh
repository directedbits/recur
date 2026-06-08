#!/bin/bash
# One-time setup of branch protection rules for the main branch.
# Requires: gh CLI authenticated with admin access to the repo.
#
# Usage: ./scripts/setup-branch-protection.sh owner/repo [check1,check2,...]
#
# Examples:
#   ./scripts/setup-branch-protection.sh user/myapp
#   ./scripts/setup-branch-protection.sh user/myapp test,golangci-lint,build
#
# If no checks are specified, the status checks rule is omitted entirely.
#
# To remove: gh api repos/OWNER/REPO/rulesets --jq '.[].id' | \
#   xargs -I{} gh api -X DELETE repos/OWNER/REPO/rulesets/{}

set -euo pipefail

REPO="${1:?Usage: $0 owner/repo [check1,check2,...]}"
CHECKS="${2:-}"

# Look up the repo owner's ID for bypass rules
echo "Looking up repo admin..."
OWNER_ID=$(gh api "repos/$REPO" --jq '.owner.node_id')
if [ -z "$OWNER_ID" ]; then
  echo "Warning: could not determine owner ID. Bypass rule will be skipped."
  echo "You may need to add yourself manually in Settings > Rules."
fi

# Build bypass_actors array — only the repo owner can merge
BYPASS='[]'
if [ -n "$OWNER_ID" ]; then
  BYPASS="[{\"actor_id\": $(gh api "repos/$REPO" --jq '.owner.id'), \"actor_type\": \"User\", \"bypass_mode\": \"always\"}]"
fi

# Build status checks rule if checks were specified
STATUS_CHECK_RULE=""
if [ -n "$CHECKS" ]; then
  ENTRIES=""
  IFS=',' read -ra CHECK_LIST <<< "$CHECKS"
  for check in "${CHECK_LIST[@]}"; do
    check=$(echo "$check" | xargs) # trim whitespace
    if [ -n "$ENTRIES" ]; then
      ENTRIES="$ENTRIES,"
    fi
    ENTRIES="$ENTRIES{\"context\": \"$check\"}"
  done

  STATUS_CHECK_RULE=",
    {
      \"type\": \"required_status_checks\",
      \"parameters\": {
        \"strict_required_status_checks_policy\": true,
        \"required_status_checks\": [$ENTRIES]
      }
    }"
fi

echo "Creating branch ruleset for $REPO..."

gh api "repos/$REPO/rulesets" \
  --method POST \
  --input - <<RULES
{
  "name": "main protection",
  "target": "branch",
  "enforcement": "active",
  "conditions": {
    "ref_name": {
      "include": ["refs/heads/main"],
      "exclude": []
    }
  },
  "bypass_actors": $BYPASS,
  "rules": [
    {
      "type": "deletion"
    },
    {
      "type": "non_fast_forward"
    },
    {
      "type": "pull_request",
      "parameters": {
        "required_approving_review_count": 1,
        "dismiss_stale_reviews_on_push": true,
        "require_code_owner_review": true,
        "require_last_push_approval": false,
        "required_review_thread_resolution": true
      }
    }$STATUS_CHECK_RULE
  ]
}
RULES

echo "Done. Ruleset created for main branch."
echo ""
echo "Rules applied:"
echo "  - No force pushes"
echo "  - No branch deletion"
echo "  - Only repo admin can bypass/merge"
echo "  - Require 1 PR review (stale reviews dismissed on push)"
if [ -n "$CHECKS" ]; then
  echo "  - Require status checks: $CHECKS"
  echo "  - Require branch to be up to date before merge"
else
  echo "  - No status checks required (none specified)"
fi
