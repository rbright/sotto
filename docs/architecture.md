# sotto Architecture

`sotto` is a local-first ASR CLI with explicit component boundaries so behavior can be tested mostly in-process.

## 1) High-level component map

```mermaid
flowchart LR
    Trigger["Trigger source\n(shell / hotkey / script)"] --> CLI["sotto CLI"]
    CLI --> IPC["IPC socket\n$XDG_RUNTIME_DIR/sotto.sock"]
    IPC --> Session["Session Controller\n(FSM + lifecycle)"]

    Session --> Audio["Audio capture\nPipeWire/Pulse"]
    Audio --> Riva["ASR stream\nNVIDIA Riva gRPC"]
    Riva --> Transcript["Transcript assembly\nnormalize + trailing space"]
    Transcript --> Output["Output adapters\nclipboard + optional paste"]

    Session --> Indicator["Indicator adapters\nnotify + audio cues"]
    Session --> Logs["JSONL logging\n$XDG_STATE_HOME/sotto/log.jsonl"]
```

## 2) Package responsibilities

| Package | Responsibility |
| --- | --- |
| `internal/cli` | Parse command/flag contract |
| `internal/app` | Top-level execution and dispatch wiring |
| `internal/ipc` | Single-instance socket ownership + command forwarding |
| `internal/session` | Dictation lifecycle orchestration and FSM transitions |
| `internal/audio` | Device discovery/selection + capture chunk stream |
| `internal/riva` | gRPC stream setup + ASR response accumulation |
| `internal/pipeline` | Bridge audio capture to ASR + debug artifact handling |
| `internal/transcript` | Segment assembly/whitespace normalization |
| `internal/output` | Clipboard and paste adapters |
| `internal/indicator` | Notification and cue adapters |
| `internal/doctor` | Environment/config/tool/readiness diagnostics |
| `internal/logging` | Runtime JSONL log setup |

## 3) Runtime flow (toggle -> stop)

```mermaid
sequenceDiagram
    participant T as Trigger
    participant C as CLI (app.Runner)
    participant I as IPC server
    participant S as Session controller
    participant A as Audio capture
    participant R as Riva stream
    participant O as Output committer

    T->>C: sotto toggle
    C->>I: acquire socket / become owner
    C->>S: Run()
    S->>A: Start capture
    S->>R: Dial stream + send config
    A-->>R: audio chunks (20ms)

    T->>C: sotto toggle (stop)
    C->>I: forward stop action
    I->>S: actionStop
    S->>R: close stream + collect transcript
    S->>O: Commit(transcript)
    O->>O: set clipboard
    O->>O: optional paste adapter
    S-->>C: Result (transcript/metrics/errors)
```

## 4) Session state model

```mermaid
stateDiagram-v2
    [*] --> idle
    idle --> recording: start
    recording --> transcribing: stop
    recording --> idle: cancel
    transcribing --> idle: transcribed
    transcribing --> error: stop/commit failure
    recording --> error: start failure
    error --> idle: reset
```

## 5) External dependencies

- [PipeWire](https://pipewire.org/) for local audio capture backend
- [NVIDIA Riva](https://developer.nvidia.com/riva) for local ASR serving
- [NVIDIA Parakeet model family on Hugging Face](https://huggingface.co/models?search=nvidia%20parakeet)

## 6) Testing boundary policy

- Prefer real adapters/resources (temp files, unix sockets, `httptest`, PATH fixtures).
- Avoid mock frameworks for in-repo behavior.
- Full model inference remains local-manual verification (not CI).
