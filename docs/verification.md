# Verification

## CI/local gate

Run before handoff:

```bash
just ci-check
nix build 'path:.#sotto'
```

## Coverage snapshot

```bash
go test ./apps/sotto/... -cover
```

## Optional integration-tag tests (local machine resources)

These tests are excluded from the default CI path:

```bash
just test-integration
```

## Local runtime smoke (manual, non-CI)

Prerequisites:

- local Riva endpoint reachable from configured `riva_grpc` / `riva_http`
- active desktop session compatible with your configured output adapter
- valid config file (or defaults)

Quick helpers:

```bash
just smoke-riva-doctor
just smoke-riva-manual
```

Manual checklist:

1. `sotto doctor` reports audio + Riva ready.
2. `sotto toggle` (start), speak a short phrase, `sotto toggle` (stop).
3. Verify non-empty transcript commit.
4. Verify clipboard retention after commit.
5. Verify configured paste behavior (if enabled).
6. Verify cancel path (`sotto cancel`) does not alter clipboard.
7. Verify Riva-down failure path is safe (no unintended output side effects).
8. Verify stale socket recovery after abnormal process termination.
