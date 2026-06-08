# Plugin: calendar

External trigger plugin that monitors iCal/ICS calendar sources (HTTP URLs or local
files) and fires events when calendar entries are upcoming, started, or ended. Uses
`apognu/gocal` for iCal parsing with RRULE expansion and a poll-based event loop.

## Plugin Identity

| Field       | Value                                          |
|-------------|------------------------------------------------|
| Name        | `calendar`                                     |
| Namespace   | `core.calendar`                                |
| Version     | `0.1.0`                                        |
| Binary      | `calendar`                                     |

## Trigger Types

### EventUpcoming

Fires when a calendar event's start time is within `look_ahead` of the current time.
Checked each poll cycle.

**Options:**

| Option          | Type   | Default  | Description                                    |
|-----------------|--------|----------|------------------------------------------------|
| `source`        | string | *required* | URL or local file path to .ics calendar      |
| `look_ahead`    | string | `"15m"`  | Go duration: how far ahead to check            |
| `poll_interval` | string | `"5m"`   | Go duration: how often to poll the source      |
| `filter_title`  | string | *(optional)* | Include only events whose title contains this substring (case-insensitive) |
| `filter_location` | string | *(optional)* | Include only events whose location contains this substring (case-insensitive) |
| `filter_description` | string | *(optional)* | Include only events whose description contains this substring (case-insensitive) |
| `filter_category` | string | *(optional)* | Include only events with a matching category (case-insensitive substring) |
| `exclude_title` | string | *(optional)* | Skip events whose title contains this substring (case-insensitive) |
| `exclude_location` | string | *(optional)* | Skip events whose location contains this substring (case-insensitive) |
| `exclude_description` | string | *(optional)* | Skip events whose description contains this substring (case-insensitive) |

**Context Variables:**

| Variable           | Type   | Description                              |
|--------------------|--------|------------------------------------------|
| `EventUID`         | string | Unique identifier of the calendar event  |
| `EventTitle`       | string | Event summary/title                      |
| `EventStart`       | string | Start time in RFC 3339 format            |
| `EventEnd`         | string | End time in RFC 3339 format              |
| `EventDescription` | string | Event description, empty if not set      |
| `EventLocation`    | string | Event location, empty if not set         |
| `EventCategories`  | string | Comma-separated event categories, empty if not set |
| `StartsIn`         | string | Go duration until event starts           |

### EventStarted

Fires when a calendar event's start time has passed within the current poll window
(i.e., start time is in `[now - poll_interval, now]`).

**Options:**

| Option          | Type   | Default  | Description                                    |
|-----------------|--------|----------|------------------------------------------------|
| `source`        | string | *required* | URL or local file path to .ics calendar      |
| `poll_interval` | string | `"5m"`   | Go duration: how often to poll the source      |
| `filter_title`  | string | *(optional)* | Include only events whose title contains this substring (case-insensitive) |
| `filter_location` | string | *(optional)* | Include only events whose location contains this substring (case-insensitive) |
| `filter_description` | string | *(optional)* | Include only events whose description contains this substring (case-insensitive) |
| `filter_category` | string | *(optional)* | Include only events with a matching category (case-insensitive substring) |
| `exclude_title` | string | *(optional)* | Skip events whose title contains this substring (case-insensitive) |
| `exclude_location` | string | *(optional)* | Skip events whose location contains this substring (case-insensitive) |
| `exclude_description` | string | *(optional)* | Skip events whose description contains this substring (case-insensitive) |

**Context Variables:**

| Variable           | Type   | Description                              |
|--------------------|--------|------------------------------------------|
| `EventUID`         | string | Unique identifier of the calendar event  |
| `EventTitle`       | string | Event summary/title                      |
| `EventStart`       | string | Start time in RFC 3339 format            |
| `EventEnd`         | string | End time in RFC 3339 format              |
| `EventDescription` | string | Event description, empty if not set      |
| `EventLocation`    | string | Event location, empty if not set         |
| `EventCategories`  | string | Comma-separated event categories, empty if not set |

### EventEnded

Fires when a calendar event's end time has passed within the current poll window
(i.e., end time is in `[now - poll_interval, now]`).

**Options:**

| Option          | Type   | Default  | Description                                    |
|-----------------|--------|----------|------------------------------------------------|
| `source`        | string | *required* | URL or local file path to .ics calendar      |
| `poll_interval` | string | `"5m"`   | Go duration: how often to poll the source      |
| `filter_title`  | string | *(optional)* | Include only events whose title contains this substring (case-insensitive) |
| `filter_location` | string | *(optional)* | Include only events whose location contains this substring (case-insensitive) |
| `filter_description` | string | *(optional)* | Include only events whose description contains this substring (case-insensitive) |
| `filter_category` | string | *(optional)* | Include only events with a matching category (case-insensitive substring) |
| `exclude_title` | string | *(optional)* | Skip events whose title contains this substring (case-insensitive) |
| `exclude_location` | string | *(optional)* | Skip events whose location contains this substring (case-insensitive) |
| `exclude_description` | string | *(optional)* | Skip events whose description contains this substring (case-insensitive) |

**Context Variables:**

| Variable           | Type   | Description                              |
|--------------------|--------|------------------------------------------|
| `EventUID`         | string | Unique identifier of the calendar event  |
| `EventTitle`       | string | Event summary/title                      |
| `EventStart`       | string | Start time in RFC 3339 format            |
| `EventEnd`         | string | End time in RFC 3339 format              |
| `EventDescription` | string | Event description, empty if not set      |
| `EventLocation`    | string | Event location, empty if not set         |
| `EventCategories`  | string | Comma-separated event categories, empty if not set |

## Event Filtering

All three trigger types support the same set of filter options for narrowing which
calendar events fire triggers. Filters are case-insensitive substring matches.

**Include filters** (AND logic — all specified filters must match):
- `filter_title` — match against event summary/title
- `filter_location` — match against event location
- `filter_description` — match against event description
- `filter_category` — match against any of the event's categories

**Exclude filters** (evaluated first, before include filters):
- `exclude_title` — skip events matching this title substring
- `exclude_location` — skip events matching this location substring
- `exclude_description` — skip events matching this description substring

Exclude filters take precedence: if an event matches any exclude filter, it is skipped
regardless of include filter matches. Within include filters, all specified filters must
match (AND logic). Omitted filters match everything.

## Deduplication

Each poll cycle checks a time window. To avoid re-firing for the same event occurrence:

- Track fired events in memory using a composite key: `UID + ":" + DTSTART`
- This handles recurring event instances (same UID, different DTSTART)
- Each entry records the time it was fired
- Prune entries older than `2 * poll_interval` to bound memory usage

## Source Resolution

- If `source` starts with `http://` or `https://` → HTTP GET with 30s timeout, parse response body
- Otherwise → treat as local file path, open with `os.Open`
- Both produce an `io.Reader` passed to gocal for parsing

## Poll Behavior

Each poll cycle:

1. Fetch source → `io.Reader`
2. Set gocal time window based on trigger type:
   - **EventUpcoming:** `[now, now + look_ahead]` — events starting within the look-ahead window
   - **EventStarted:** `[now - poll_interval, now]` — events whose start time fell in the last poll window
   - **EventEnded:** `[now - poll_interval, now]` — events whose end time fell in the last poll window
3. Parse with gocal, filter matches, deduplicate
4. Report matching events via `ReportTriggerEvent`
5. Sleep `poll_interval`, repeat

## Protocol

This plugin follows the [trigger plugin protocol](plugin-protocol.md):

1. Reads trigger type and options from stdin JSON
2. Reads `RECUR_SOCKET` and `RECUR_TRIGGER_ID` from environment
3. Parses options: `source`, `poll_interval`, `look_ahead`
4. Starts poll loop against calendar source
5. Connects to daemon gRPC socket
6. Event loop: poll fires → match events → deduplicate → call `ReportTriggerEvent`
7. On SIGTERM: stop poll loop, close gRPC, exit 0

## Example Recurfile

```yaml
Meeting Reminder:
  on:
    - type: EventUpcoming
      options:
        source: "https://calendar.google.com/calendar/ical/example/basic.ics"
        look_ahead: "15m"
        poll_interval: "5m"
  do:
    - shell: >
        notify-send "Meeting soon" "{{.EventTitle}} starts in {{.StartsIn}}"

Meeting Started:
  on:
    - type: EventStarted
      options:
        source: ~/calendars/work.ics
        poll_interval: "2m"
  do:
    - shell: >
        echo "{{.EventTitle}} started at {{.EventStart}}" >> ~/meeting-log.txt

Meeting Ended:
  on:
    - type: EventEnded
      options:
        source: ~/calendars/work.ics
        poll_interval: "5m"
  do:
    - shell: >
        echo "{{.EventTitle}} ended at {{.EventEnd}}" >> ~/meeting-log.txt

Filtered Standup:
  on:
    - type: EventUpcoming
      options:
        source: "https://calendar.google.com/calendar/ical/example/basic.ics"
        look_ahead: "10m"
        filter_title: "standup"
        exclude_title: "cancelled"
        filter_category: "work"
  do:
    - shell: >
        notify-send "Standup soon" "{{.EventTitle}} ({{.EventCategories}}) in {{.StartsIn}}"
```
