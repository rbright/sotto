# Verification

## Required local gate

Run before hand-off:

```bash
just ci-check
nix build 'path:.#sotto'
```

## Optional integration-tag tests

These are local-resource tests and are not part of the default CI gate:

```bash
just test-integration
```

## Coverage snapshot

```bash
go test ./apps/sotto/... -cover
```

## Manual runtime smoke (non-CI)

Prerequisites:

- local Riva endpoint is reachable
- active Wayland/Hyprland session
- valid `sotto` config

Quick helpers:

```bash
just smoke-riva-doctor
just smoke-riva-manual
```

Checklist:

1. `sotto doctor` reports config/audio/Riva ready.
2. `sotto toggle` start -> speak -> `sotto toggle` stop.
3. Confirm non-empty transcript commit.
4. Confirm clipboard contains transcript after commit.
5. Confirm paste behavior for your configured adapter.
6. Run `sotto cancel` and verify clipboard is unchanged.
7. Stop Riva and confirm safe failure (no unintended clipboard/paste side effects).
8. Kill active `sotto` process mid-session and verify stale-socket recovery on next command.
