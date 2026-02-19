# sotto Agent Guide

## Scope

These rules apply to the entire `sotto/` repository.

## Mission

Ship a production-grade, local-first ASR CLI with:

- a single Go binary
- no background daemon
- strong crash/cleanup guarantees
- clear modular boundaries for safe refactoring
- reproducible packaging and CI

## First 60 Seconds (Progressive Disclosure)

1. Read this file fully.
2. Read `README.md` for user-facing behavior.
3. Read `PLAN.md` (current milestones/checklist) and `SESSION.md` (what actually ran).
4. Run `just --list` to see current task entrypoints.
5. Open only the component(s) you are changing (map below).

---

## Project Map (What to read by task)

| Area | Primary paths | Notes |
| --- | --- | --- |
| CLI contract + dispatch | `apps/sotto/internal/cli/`, `apps/sotto/internal/app/` | Commands, flags, top-level flow |
| Session state machine | `apps/sotto/internal/session/`, `apps/sotto/internal/fsm/`, `apps/sotto/internal/ipc/` | Toggle/stop/cancel semantics, single-instance behavior |
| Audio capture + device selection | `apps/sotto/internal/audio/` | PipeWire/Pulse capture, device fallback/mute handling |
| Riva streaming ASR | `apps/sotto/internal/riva/`, `apps/sotto/internal/pipeline/` | gRPC stream config, segment assembly inputs |
| Transcript assembly | `apps/sotto/internal/transcript/` | Whitespace normalization + trailing-space behavior |
| Output dispatch | `apps/sotto/internal/output/`, `apps/sotto/internal/hypr/` | Clipboard + paste behavior |
| Indicator + cues | `apps/sotto/internal/indicator/` | Visual notify + audio cue lifecycle |
| Config grammar/defaults | `apps/sotto/internal/config/` | Any new key must update parser/defaults/tests/docs |
| Packaging + tooling | `justfile`, `flake.nix`, `.github/workflows/` | CI/tooling changes |
| Protobuf contracts | `apps/sotto/proto/third_party/`, `proto/gen/go/` | Run codegen when proto inputs change |

---

## Engineering Workflow Rules

1. Read target files before editing.
2. Keep changes aligned to `PLAN.md` milestones; avoid drive-by refactors.
3. Keep `PLAN.md` checkboxes accurate (only mark executed + verified work).
4. Log key decisions/trade-offs/blockers/commands in `SESSION.md`.
5. Prefer additive changes with regression tests.
6. Never claim runtime integrations (Riva, PipeWire, Hyprland) were verified unless actually exercised.

### Design principles (repo-wide)

- Prefer boring, explicit code over clever code.
- Fail fast at boundaries (config parse, startup checks, I/O preconditions).
- Keep business/state logic separate from transport/I/O adapters.
- Use guard clauses to reduce nesting.
- Keep files top-down readable (public entrypoints first, private helpers below).

### Dependency + architecture rules

- Prefer manual constructor-based dependency injection.
- Keep package responsibilities narrow; avoid utility dumping grounds.
- Do not couple domain/state transitions directly to shell command details.

---

## Go Conventions

- Use table-driven tests for branch-heavy logic.
- Prefer `errors.Is` / wrapped errors with context.
- Keep I/O timeouts explicit.
- Use `testing` + `testify` (`require`/`assert`) for expressive assertions when useful.
- Add focused regression tests for bug fixes whenever feasible.

### Testing boundaries (repo policy)

- Prefer real interfaces/adapters and real resources (temp files, unix sockets, `httptest`, PATH fixtures).
- Do **not** introduce mocking frameworks or expectation-driven mock suites.
- Riva runtime/model inference remains a local-manual smoke concern (non-CI); use lightweight protocol/contract tests in CI.

### File size / readability guardrails

- Handwritten files should target `<= 250` LOC where practical.
- Files above `~350` LOC require extraction-plan notes in `PLAN.md` before refactor work.
- Exclude generated code from these thresholds: `apps/sotto/vendor/**`, `apps/sotto/proto/gen/**`.

---

## Config Change Contract (Mandatory)

When adding or changing a config key, update all of:

1. `apps/sotto/internal/config/types.go`
2. `apps/sotto/internal/config/defaults.go`
3. `apps/sotto/internal/config/parser.go`
4. validation if required (`validate.go`)
5. parser/validation tests
6. `README.md` config example + notes
7. any deployed default config in consuming repos (when in scope)

---

## Required Local Checks Before Hand-off

Run and report status for:

1. `just ci-check`
2. `nix build 'path:.#sotto'`

If any check is skipped, state exactly what was skipped, why, and the exact command to run.

### Pre-commit Hooks (`prek`)

- Install hooks: `just precommit-install`
- Run lightweight pre-commit hooks: `just precommit-run`
- Run heavier pre-push hooks: `just prepush-run`

Use hooks to catch formatting/lint drift early and run full guardrails before pushing.

---

## Safety

- Never store secrets in repo files.
- Assume `NGC_API_KEY` and other credentials are external env/secrets only.
- Avoid destructive shell operations unless explicitly requested.
- Do not edit files outside `sotto/` unless explicitly requested.
