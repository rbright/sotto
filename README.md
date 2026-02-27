# sotto

[![CI](https://github.com/rbright/sotto/actions/workflows/ci.yml/badge.svg)](https://github.com/rbright/sotto/actions/workflows/ci.yml)

Local-first speech-to-text CLI.

`sotto` captures microphone audio, streams to a local ASR backend (Riva by default), assembles transcript text, and commits output to the clipboard with optional paste dispatch.

## Why this exists

- single-process CLI (no daemon)
- local-first by default (localhost Riva endpoints)
- explicit state machine (`toggle`, `stop`, `cancel`)
- deterministic config + observable runtime logs

## Feature summary

- single-instance command coordination via unix socket
- audio capture via PipeWire/Pulse
- streaming ASR via NVIDIA Riva gRPC
- transcript normalization + sentence capitalization + optional trailing space
- output adapters:
  - clipboard command (`clipboard_cmd`)
  - optional paste command override (`paste_cmd`)
  - default Hyprland paste path (`hyprctl sendshortcut`) when `paste_cmd` is unset
- indicator backends:
  - `hypr` notifications
  - `desktop` (freedesktop notifications, e.g. mako)
- embedded cue WAV assets for start/stop/complete/cancel (not user-configurable)
- built-in indicator localization scaffolding (English catalog currently shipped)
- built-in environment diagnostics via `sotto doctor`

## Platform scope (current)

`sotto` is currently optimized for **Wayland + Hyprland** workflows.

- default paste behavior uses `hyprctl`
- `doctor` currently checks for a Hyprland session

You can still reduce Hyprland coupling by using:

- `indicator.backend = desktop`
- `paste_cmd = "..."` (explicit command override)

## Install

### Nix (recommended)

```bash
nix build 'path:.#sotto'
nix run 'path:.#sotto' -- --help
```

### From source

```bash
just tools
go test ./apps/sotto/...
go build ./apps/sotto/cmd/sotto
```

## Quickstart

```bash
sotto doctor
sotto toggle   # start
sotto toggle   # stop + commit
```

Core commands:

```bash
sotto toggle
sotto stop
sotto cancel
sotto status
sotto devices
sotto doctor
sotto version
```

## Configuration

Config resolution order:

1. `--config <path>`
2. `$XDG_CONFIG_HOME/sotto/config.jsonc`
3. `~/.config/sotto/config.jsonc`

Compatibility note:

- if the default `.jsonc` file is missing, sotto will fall back to legacy `config.conf` automatically.

See full key reference and examples in:

- [`docs/configuration.md`](./docs/configuration.md)

## Verification

Required local gate before hand-off:

```bash
just ci-check
nix build 'path:.#sotto'
```

Manual/local runtime checklist:

- [`docs/verification.md`](./docs/verification.md)

## Architecture and design docs

- [`docs/architecture.md`](./docs/architecture.md)
- [`docs/modularity.md`](./docs/modularity.md)
- [`AGENTS.md`](./AGENTS.md)
