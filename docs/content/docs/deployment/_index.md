---
title: "Deployment"
weight: 5
description: "Running recur as a system service"
---

The daemon should be managed by your init system for automatic startup and restart on failure.

The recommended path on macOS and Linux is Homebrew: `brew install recur && brew services start recur` sets up launchd (macOS) or systemd (Linux) for you.

- [Docker](docker/) -- Run recur in a container
