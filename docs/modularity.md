# Modularity Review

This document tracks readability risk and safe refactor slices.

## Current state

### What is working well

- package boundaries are responsibility-oriented (`session`, `audio`, `riva`, `output`, `indicator`)
- state/FSM logic is separated from most I/O adapters
- test coverage is broad enough to support behavior-preserving extraction

### Remaining readability hotspots

Large handwritten files still carry higher review/refactor risk:

- `internal/config/parser.go`
- `internal/audio/pulse.go`
- `internal/app/app.go`
- `internal/riva/client.go`
- `internal/pipeline/transcriber.go`
- `internal/session/session.go`

Generated code is out of scope for these thresholds.

## Refactor slices (behavior-preserving)

1. `internal/config/parser.go`
   - isolate token/scalar helpers
   - isolate vocab block parser
2. `internal/audio/pulse.go`
   - separate device selection from capture loop
3. `internal/app/app.go`
   - split command dispatch, IPC forwarding, runtime bootstrap
4. `internal/riva/client.go`
   - split stream lifecycle from segment merge helpers
5. `internal/pipeline/transcriber.go`
   - split debug artifact writers from orchestration
6. `internal/session/session.go`
   - isolate transition handling from commit/result assembly

## Guardrails

- soft target: handwritten files near `<= 250` LOC
- files above `~350` LOC need an explicit extraction note in `PLAN.md`
- extract in small slices with tests first
