#!/usr/bin/env bash
# Mirror source-of-truth READMEs into the Hugo content tree.
#
# Some doc pages are direct mirrors of READMEs that live next to the
# code they document — plugin READMEs at plugins/<name>/README.md,
# contributor guide at .github/CONTRIBUTING.md, docker example at
# examples/docker/README.md. Edit only the SOURCE files; this script
# runs in the docs:sync Task and is invoked by docs:serve / docs.
#
# Also strips the duplicate "# Title" H1 that immediately follows
# the front matter, since lotusdocs already renders the title from
# the front matter block.

set -euo pipefail

cd "$(dirname "$0")/.."

# source|destination
mappings=(
  ".github/CONTRIBUTING.md|docs/content/docs/contributing/_index.md"
  "examples/docker/README.md|docs/content/docs/deployment/docker.md"
  "plugins/calendar/README.md|docs/content/docs/plugins/calendar.md"
  "plugins/devicemonitor/README.md|docs/content/docs/plugins/devicemonitor.md"
  "plugins/docker/README.md|docs/content/docs/plugins/docker.md"
  "plugins/fileevents/README.md|docs/content/docs/plugins/fileevents.md"
  "plugins/mqtt/README.md|docs/content/docs/plugins/mqtt.md"
  "plugins/timer/README.md|docs/content/docs/plugins/timer.md"
  "plugins/webhook/README.md|docs/content/docs/plugins/webhook.md"
)

for pair in "${mappings[@]}"; do
  src="${pair%|*}"
  dst="${pair#*|}"

  if [ ! -f "$src" ]; then
    echo "sync-docs: source missing: $src" >&2
    exit 1
  fi

  mkdir -p "$(dirname "$dst")"
  python3 - "$src" "$dst" <<'PY'
import re, sys
src, dst = sys.argv[1], sys.argv[2]
with open(src) as f:
    content = f.read()
# Drop the duplicate H1 that immediately follows the front matter.
content = re.sub(
    r'^(---\n.*?\n---\n\n)# [^\n]+\n\n',
    r'\1',
    content,
    count=1,
    flags=re.DOTALL,
)
with open(dst, 'w') as f:
    f.write(content)
PY
  echo "synced: $src -> $dst"
done
