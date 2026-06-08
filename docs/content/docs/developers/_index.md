---
title: "Developer Guide"
weight: 9
description: "Internal documentation for contributors and plugin authors"
---

This section contains internal documentation for two audiences:

- **Core contributors** working on the daemon, CLI, trigger engine, or infrastructure packages.
- **Plugin authors** building new trigger or action plugins that run as external binaries.

## Sub-pages

- [Architecture Deep Dive](architecture/) -- package structure, request flows, and key interfaces inside the daemon.
- [Writing a Plugin](writing-a-plugin/) -- step-by-step tutorial for creating a new trigger or action plugin.
- [Testing Guide](testing/) -- unit test patterns, mock strategies, and coverage enforcement.
- [Debugging](debugging/) -- practical tips for diagnosing daemon, plugin, and recurfile issues.
- [Release Process](release/) -- how tag-based releases are built and published via GitHub Actions.

## Prerequisites

This guide assumes you have already read [CONTRIBUTING.md](https://github.com/directedbits/recur/blob/main/.github/CONTRIBUTING.md), have a working Go toolchain, and can run:

```sh
task build && task test
```

For build setup details, see the [Contributing](../contributing/) section. For formal API and manifest specs, see the [Reference](../reference/) section.
