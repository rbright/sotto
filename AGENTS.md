# sotto Agent Guide

## Scope

Applies to the entire `sotto/` repository.

## Mission

Deliver a production-grade, local-first ASR CLI:

- single Go binary
- no daemon/background service
- deterministic toggle/stop/cancel behavior
- strong cleanup and failure safety
- reproducible tooling + packaging

## Fast Start (read in order)

1. `README.md` (user-facing behavior)
2. `PLAN.md` (active checklist)
3. `SESSION.md` (what was actually executed)
4. `just --list` (task entrypoints)
5. Only then open the package(s) you need to change

## Component Map

| Task area | Primary paths |
| --- | --- |
| CLI + dispatch | `apps/sotto/internal/cli/`, `apps/sotto/internal/app/` |
| IPC + single-instance | `apps/sotto/internal/ipc/` |
| Session/FSM | `apps/sotto/internal/session/`, `apps/sotto/internal/fsm/` |
| Audio capture | `apps/sotto/internal/audio/` |
| Riva streaming | `apps/sotto/internal/riva/`, `apps/sotto/internal/pipeline/` |
| Transcript assembly | `apps/sotto/internal/transcript/` |
| Clipboard/paste output | `apps/sotto/internal/output/`, `apps/sotto/internal/hypr/` |
| Indicator + cues | `apps/sotto/internal/indicator/` |
| Config system | `apps/sotto/internal/config/` |
| Diagnostics/logging | `apps/sotto/internal/doctor/`, `apps/sotto/internal/logging/` |
| Tooling/packaging | `justfile`, `.just/`, `flake.nix`, `.github/workflows/` |
| Proto/codegen | `apps/sotto/proto/third_party/`, `apps/sotto/proto/gen/`, `buf.gen.yaml` |

## Non-Negotiable Workflow Rules

1. Read target files before editing.
2. Keep scope tight to the requested behavior.
3. Update `PLAN.md` checklist items only when executed + verified.
4. Log key decisions and commands in `SESSION.md`.
5. Add or update regression tests for behavior changes when feasible.
6. Do not claim runtime verification unless it was actually run.

## Go Engineering Standards

Write canonical, idiomatic Go:

- `gofmt` clean, straightforward naming, small focused functions
- explicit constructors and dependency wiring (no hidden globals)
- `context.Context` first for cancelable/timeout-aware operations
- wrap errors with actionable context; use `errors.Is` for branching
- keep interfaces near consumers; avoid broad shared interfaces
- separate state/policy logic from I/O adapters
- avoid clever abstractions; prefer explicit control flow

## Testing Policy

- Use `testing` + `testify` (`require`/`assert`) as needed.
- Prefer real boundaries/resources (temp files, unix sockets, `httptest`, PATH fixtures).
- Do **not** introduce mocking frameworks.
- Riva runtime inference remains manual smoke (non-CI).

## Config Change Contract (Mandatory)

Any config-key change must update all relevant locations:

1. `internal/config/types.go`
2. `internal/config/defaults.go`
3. `internal/config/parser.go`
4. `internal/config/validate.go` (if constraints change)
5. parser/validation tests
6. `docs/configuration.md` and any README examples
7. consuming defaults in external config repos when in scope

## Required Checks Before Hand-off

Run and report:

1. `just ci-check`
2. `nix build 'path:.#sotto'`

If skipped, state exactly what was skipped, why, and how to run it.

## Safety

- Never commit secrets (e.g., `NGC_API_KEY`).
- Avoid destructive shell operations unless explicitly requested.
- Do not edit outside `sotto/` unless explicitly asked.
