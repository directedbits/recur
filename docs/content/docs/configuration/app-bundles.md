---
title: "App Bundles"
weight: 4
description: "Package, share, and install recur automations as .recur bundles"
---

An **app bundle** packages a recurfile together with the local scripts it needs
into a single `.recur` file. Instead of sharing a loose recurfile plus a folder
of scripts, you can hand someone one file — they install it with a single
command and it registers itself with their daemon.

A bundle is just a **zip archive** with a `.recur` extension. It contains:

- a **recurfile** — the single YAML file at the archive root, and
- any **scripts or assets** your automation calls, in whatever layout you like.

There is no separate manifest and no special schema — a bundle is a recurfile
and its files. See the **Recurfile Format** guide for how to write the recurfile
itself.

## Installing an app

```sh
recur app install ./habits.recur
```

This unpacks the bundle into `~/.config/recur/app/<name>/` and registers its
recurfile. You can also install directly from a URL:

```sh
recur app install https://example.com/apps/habits.recur
```

Downloads are only allowed from hosts you have permitted, using the same
`allowed_hosts` setting as plugin installs:

```sh
recur config set allowed_hosts example.com
```

**If the daemon isn't running,** the app is still unpacked and will be
registered automatically the next time the daemon starts — you don't need to run
`recur register` yourself.

### Choosing the app name

By default the app name comes from the recurfile's filename (e.g. `habits.yaml`
→ app `habits`). If the recurfile uses the generic name `recurfile.yaml`, the
bundle's own filename is used instead. Override it explicitly with `--name`:

```sh
recur app install ./habits.recur --name morning-routine
```

If an app with that name already exists, you'll be prompted to overwrite or
abandon. Use `--force` to overwrite without prompting.

## Listing and removing apps

```sh
recur app list           # installed apps and whether each is registered
recur app remove habits  # deregister and delete the app
```

## Creating a bundle

Lay out a directory with your recurfile at the top and scripts alongside it:

```
habits/
├── habits.yaml        # the recurfile (any single root YAML works)
└── scripts/
    ├── prompt.sh
    └── record.sh
```

Reference the scripts from the recurfile as usual, then pack the directory:

```sh
recur app pack ./habits            # writes ./habits.recur
recur app pack ./habits -o out.recur
```

`pack` preserves your directory layout and file modes (so an executable script
stays executable after install), and it fails if the directory has no root
recurfile.

### Naming the recurfile

Inside a bundle the recurfile is simply the one YAML file at the root, so you can
give it a meaningful name like `habits.yaml` — that name becomes the default app
name. YAML files inside subdirectories are treated as assets, not the recurfile.
If you include more than one YAML at the root, name one of them `recurfile.yaml`
to disambiguate; otherwise the bundle is rejected as ambiguous.

## Where apps live

Installed apps are unpacked as-is under:

```
~/.config/recur/app/<name>/
```

You can inspect an installed app's files there directly. Removing an app deletes
this directory.
