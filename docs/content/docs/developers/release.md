---
title: "Release Process"
weight: 5
description: "How releases are built and published"
---

## Overview

Releases are fully automated via GitHub Actions. Pushing a version tag triggers the release workflow, which cross-compiles binaries for all supported platforms, packages them into tarballs, and publishes a GitHub Release.

## Triggering a Release

```sh
git tag v0.2.0
git push origin v0.2.0
```

The workflow triggers on any tag matching `v*`.

## What Gets Built

The release workflow builds for 8 platform targets:

| OS | Architecture | Notes |
|----|-------------|-------|
| linux | amd64 | |
| linux | arm64 | |
| linux | arm (v7) | |
| linux | 386 | |
| darwin | amd64 | |
| darwin | arm64 | Apple Silicon |
| windows | amd64 | `.exe` extension |
| windows | arm64 | `.exe` extension |

All builds use `CGO_ENABLED=0` for fully static binaries.

Tarball naming follows the pattern:

```
recur-v0.2.0-linux-amd64.tar.gz
recur-v0.2.0-darwin-arm64.tar.gz
recur-v0.2.0-windows-amd64.tar.gz
recur-v0.2.0-linux-armv7.tar.gz
```

A source archive (`recur-v0.2.0-source.tar.gz`) is also created from `git archive`.

## Archive Contents

Each platform tarball contains:

```
recur-v0.2.0-linux-amd64/
  recur                          # CLI binary
  recurd                         # daemon binary
  plugins/
    calendar/
      calendar                  # plugin binary
      manifest.yaml
    docker/
      docker
      manifest.yaml
    mqtt/
      mqtt
      manifest.yaml
    timer/
      timer
      manifest.yaml
    webhook/
      webhook
      manifest.yaml
    devicemonitor/              # linux + windows only
      devicemonitor
      manifest.yaml
```

## Version Injection

The release workflow injects the version tag into the binaries via Go linker flags:

```
-X github.com/directedbits/recur/src/app/cli.Version=v0.2.0
```

This sets the `cli.Version` variable used by `recur version`.

## Platform-Specific Plugins

The **devicemonitor** plugin (D-Bus on Linux, WMI on Windows) is only built for `linux` and `windows` targets. It is excluded from `darwin` builds because it has no macOS implementation.

All other plugins (calendar, docker, mqtt, timer, webhook) are built for every platform.

## Smoke Tests

The workflow runs basic smoke tests on the **linux-amd64** build only:

```sh
recur version        # verify binary runs and prints version
recur config get     # verify config subsystem works
```

Binary sizes for all platforms are reported in the GitHub Actions job summary.

## Release Checklist

1. Ensure all tests pass: `task test:all`
2. Tag the release: `git tag v0.X.0`
3. Push the tag: `git push origin v0.X.0`
4. Wait for the [Release workflow](../../.github/workflows/release.yml) to complete.
5. Verify the [GitHub Releases page](https://github.com/directedbits/recur/releases) shows:
   - All 8 platform tarballs plus the source archive
   - Auto-generated release notes
   - Correct version in binary names
