# Architecture

`sotto` is a local-first ASR CLI with clear boundaries between state logic and side-effect adapters.

## Component map

```mermaid
flowchart LR
    Trigger["Trigger\n(shell / hotkey / script)"] --> CLI["CLI + command dispatch"]
    CLI --> IPC["IPC socket\n$XDG_RUNTIME_DIR/sotto.sock"]
    IPC --> Session["Session controller\n(FSM + lifecycle)"]

    Session --> Audio["Audio capture\n(PipeWire/Pulse)"]
    Session --> ASR["Riva streaming client\n(gRPC)"]
    ASR --> Transcript["Transcript assembly\n(normalize + trailing space)"]
    Transcript --> Output["Output adapters\n(clipboard + paste)"]

    Session --> Indicator["Indicator adapters\n(hypr or desktop) + cues"]
    Session --> Logs["JSONL logs\n$XDG_STATE_HOME/sotto/log.jsonl"]
```

## Package responsibilities

| Package | Responsibility |
| --- | --- |
| `internal/cli` | command/flag contract |
| `internal/app` | top-level wiring and dispatch |
| `internal/ipc` | single-instance socket lifecycle + forwarding |
| `internal/fsm` | legal session transitions |
| `internal/session` | lifecycle orchestration (`toggle`/`stop`/`cancel`) |
| `internal/audio` | device discovery/selection + capture stream |
| `internal/riva` | ASR stream transport + response accumulation |
| `internal/pipeline` | audio-to-ASR bridge + debug artifacts |
| `internal/transcript` | text normalization and assembly |
| `internal/output` | clipboard + paste adapters |
| `internal/indicator` | visual indicator + cue sound dispatch |
| `internal/doctor` | environment/readiness checks |
| `internal/logging` | session log bootstrap |

## Runtime flow (`toggle` -> `toggle`)

```mermaid
sequenceDiagram
    participant T as Trigger
    participant C as CLI
    participant I as IPC
    participant S as Session
    participant A as Audio
    participant R as Riva
    participant O as Output

    T->>C: sotto toggle (start)
    C->>I: acquire socket / become owner
    I->>S: start
    S->>A: start capture
    S->>R: open stream + send config
    A-->>R: PCM chunks

    T->>C: sotto toggle (stop)
    C->>I: send stop
    I->>S: stop
    S->>R: close stream + gather transcript
    S->>O: commit(transcript)
```

## Platform coupling (today)

Current production path is Wayland + Hyprland:

- default paste path calls `hyprctl sendshortcut`
- doctor checks require Hyprland session context

This coupling is intentionally explicit and isolated in `internal/hypr` + output/doctor adapters so additional desktop targets can be added without changing session/FSM logic.
