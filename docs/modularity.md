# Modularity Review

This document captures the current architecture cleanliness assessment and the next refactor slices.

## Current status

### What is clean today

- package boundaries are clear and responsibility-oriented (`session`, `audio`, `riva`, `output`, `indicator`, etc.)
- side-effect adapters are mostly isolated from parser/FSM/domain logic
- test coverage is broad enough to begin safe extraction work

### Remaining readability risk

A handful of handwritten files are still large (roughly 300–430 LOC), which increases cognitive load:

- `internal/config/parser.go`
- `internal/audio/pulse.go`
- `internal/app/app.go`
- `internal/riva/client.go`
- `internal/pipeline/transcriber.go`
- `internal/session/session.go`

Generated code (`vendor/**`, `proto/gen/**`) is excluded from readability thresholds.

## Refactor slices (behavior-preserving)

1. `internal/config/parser.go`
   - extract lexer/token helpers
   - extract root-key application map
   - isolate vocabset block parser

2. `internal/audio/pulse.go`
   - separate device selection from capture lifecycle
   - isolate chunker/state bookkeeping from Pulse stream setup

3. `internal/app/app.go`
   - split command dispatch, IPC forwarding, and runtime bootstrap wiring

4. `internal/riva/client.go`
   - split stream transport lifecycle from transcript segment merge logic

5. `internal/pipeline/transcriber.go`
   - split debug artifact writers and file path resolution from transcribe lifecycle

6. `internal/session/session.go`
   - isolate action handling and result assembly from transition orchestration

## Working guardrails

- soft target: keep handwritten files near `<= 250` LOC where practical
- files above `~350` LOC should get explicit extraction-plan notes before changes
- preserve behavior with tests first, then extract in small slices
