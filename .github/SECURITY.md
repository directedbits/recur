# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability, please report it responsibly.

**Do not open a public issue.** Use [GitHub's private vulnerability reporting](https://github.com/directedbits/recur/security/advisories/new)
to file a draft advisory that only the maintainers can see.

Include:
- Description of the vulnerability
- Steps to reproduce
- Impact assessment (what an attacker could do)
- Suggested fix, if you have one

You should receive an acknowledgment within 48 hours and a resolution timeline
within 7 days.

## Scope

Security-relevant areas of this project include:
- **Webhook plugin**: Listens on a network port, handles HMAC signature verification
- **MQTT plugin**: Connects to external brokers with credentials
- **Plugin installation**: Downloads and extracts archives from remote URLs
- **Daemon socket**: Unix socket / TCP communication between CLI and daemon
- **Config file handling**: Reads YAML from disk, persists state as JSON

## Supported Versions

Only the latest release is supported with security updates.
