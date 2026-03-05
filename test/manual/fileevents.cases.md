# File Events Plugin - Manual Test Cases

Minimal set of manual tests to cover the plugin surface.

## Setup

```sh
task build:fileevents
mkdir -p ~/.config/recur/plugins/fileevents
cp bin/plugins/fileevents/* ~/.config/recur/plugins/fileevents/
```

## Test 1: Basic FileCreated + FileModified + FileDeleted

Covers: all three common trigger types, default options, group-level actions, template variables.

```yaml
# ~/test/recur.yaml
FileOps:
  on:
    - type: FileCreated
      do:
        - shell: "echo 'CREATED: {{.FilePath}} dir={{.IsDirectory}} at={{.TriggeredOn}}'"
    - type: FileModified
      do:
        - shell: "echo 'MODIFIED: {{.FilePath}}'"
    - type: FileDeleted
      do:
        - shell: "echo 'DELETED: {{.FilePath}} perm={{.PermanentlyDeleted}}'"
```

```sh
cd ~/test && recur start --foreground &
recur register
touch testfile.txt          # expect CREATED
echo "hello" >> testfile.txt # expect MODIFIED
rm testfile.txt             # expect DELETED
```

## Test 2: Filters + entity_type + recursive

Covers: glob filtering, entity_type=directory, recursive watch, ignore_hidden.

```yaml
# ~/test2/recur.yaml
Filtered:
  options:
    path: /tmp/fileevents-test
    recursive: true
    ignore_hidden: false
  on:
    - type: FileCreated
      options:
        filter:
          - "*.txt"
          - "*.log"
        entity_type: all
      do:
        - shell: "echo 'MATCH: {{.FilePath}} dir={{.IsDirectory}}'"
```

```sh
mkdir -p /tmp/fileevents-test/sub
cd ~/test2 && recur start --foreground &
recur register

touch /tmp/fileevents-test/match.txt      # expect MATCH
touch /tmp/fileevents-test/ignore.go       # expect NO event (filtered out)
touch /tmp/fileevents-test/.hidden.txt     # expect MATCH (ignore_hidden=false)
mkdir /tmp/fileevents-test/sub/newdir      # expect NO event (*.txt/*.log filter)
touch /tmp/fileevents-test/sub/nested.log  # expect MATCH (recursive + filter)
```

## Test 3: FileMoved

Covers: rename detection, From/To context.

```yaml
# ~/test3/recur.yaml
Moves:
  options:
    entity_type: all
  on:
    - type: FileMoved
      do:
        - shell: "echo 'MOVED: from={{.From}} to={{.To}}'"
```

```sh
cd ~/test3 && recur start --foreground &
recur register
touch target.txt
mv target.txt renamed.txt   # expect MOVED (From may be empty)
```

## Test 4: Default path (recurfile directory)

Covers: empty path option falls back to recurfile parent directory.

```yaml
# /tmp/pathtest/recur.yaml
DefaultPath:
  on:
    - type: FileCreated
      do:
        - shell: "echo 'CREATED in recurfile dir: {{.FilePath}}'"
```

```sh
mkdir -p /tmp/pathtest
# register from a different directory
cd /tmp && recur register /tmp/pathtest/recur.yaml
touch /tmp/pathtest/newfile.txt  # expect event
touch /tmp/other.txt             # expect NO event
```

## What to verify

- [ ] All trigger types fire at the right time
- [ ] Filter patterns correctly include/exclude files
- [ ] entity_type correctly filters files vs directories
- [ ] recursive watches catch events in subdirectories
- [ ] ignore_hidden=true (default) skips dotfiles
- [ ] Template variables (FilePath, IsDirectory, TriggeredOn, PermanentlyDeleted, From, To) are populated
- [ ] Plugin shows up in `recur list plugins`
- [ ] `recur inspect plugin fileevents` shows all 5 triggers
- [ ] State persists across daemon restarts (LastFired timestamp)
