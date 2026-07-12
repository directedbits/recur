#!/usr/bin/env bash
# Mirror source-of-truth READMEs into the Hugo content tree.
#
# Some doc pages are direct mirrors of READMEs that live next to the
# code they document — the contributor guide at .github/CONTRIBUTING.md
# and the docker example at examples/docker/README.md. Edit only the
# SOURCE files; this script runs in the docs:sync Task and is invoked by
# docs:serve / docs. (Plugin docs now live in their own repositories.)
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
