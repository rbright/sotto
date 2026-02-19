# Configuration Reference

## Path resolution order

`sotto` resolves config in this order:

1. `--config <path>`
2. `$XDG_CONFIG_HOME/sotto/config.conf`
3. `~/.config/sotto/config.conf`

If no file exists, defaults are used and a warning is emitted.

## Grammar

- `key = value`
- comments begin with `#`
- quoted and unquoted strings are supported
- vocab blocks use:

```conf
vocabset <name> {
  boost = <float>
  phrases = [ "phrase one", "phrase two" ]
}
```

Unknown keys fail fast with line-numbered errors.

## Top-level keys

| Key | Type | Default | Notes |
| --- | --- | --- | --- |
| `riva_grpc` | string | `127.0.0.1:50051` | gRPC ASR endpoint |
| `riva_http` | string | `127.0.0.1:9000` | HTTP endpoint used by doctor readiness check |
| `riva_health_path` | string | `/v1/health/ready` | must start with `/` |
| `audio.input` | string | `default` | preferred input device match |
| `audio.fallback` | string | `default` | fallback device match |
| `paste.enable` | bool | `true` | enables paste dispatch after clipboard set |
| `asr.automatic_punctuation` | bool | `true` | ASR punctuation hint |
| `asr.language_code` | string | `en-US` | ASR language code |
| `asr.model` | string | `` (empty) | optional explicit ASR model |
| `transcript.trailing_space` | bool | `true` | append trailing space to assembled transcript |
| `clipboard_cmd` | command string | `wl-copy --trim-newline` | required command |
| `paste_cmd` | command string | empty | optional override; argv parsed, no shell |
| `vocab.global` | comma list | empty | enabled vocabset names |
| `vocab.max_phrases` | int | `1024` | hard cap after dedupe |
| `debug.audio_dump` | bool | `false` | writes debug WAV artifacts |
| `debug.grpc_dump` | bool | `false` | writes raw ASR response JSON |

## Indicator keys

| Key | Type | Default | Notes |
| --- | --- | --- | --- |
| `indicator.enable` | bool | `true` | visual indicator on/off |
| `indicator.sound_enable` | bool | `true` | cue sounds on/off |
| `indicator.sound_start_file` | string | empty | optional WAV path |
| `indicator.sound_stop_file` | string | empty | optional WAV path |
| `indicator.sound_complete_file` | string | empty | optional WAV path |
| `indicator.sound_cancel_file` | string | empty | optional WAV path |
| `indicator.height` | int | `28` | visual indicator size parameter |
| `indicator.text_recording` | string | `Recording…` | recording label |
| `indicator.text_processing` | string | `Transcribing…` | processing label |
| `indicator.text_transcribing` | string | alias of `indicator.text_processing` | compatibility alias |
| `indicator.text_error` | string | `Speech recognition error` | error label |
| `indicator.error_timeout_ms` | int | `1600` | must be `>= 0`; `0` uses runtime fallback |

## Vocabulary behavior

- `vocab.global` enables one or more `vocabset` blocks.
- duplicate phrases across sets are deduped by highest boost.
- dedupe conflicts emit warnings.
- exceeding `vocab.max_phrases` is a hard error.

## Example configuration

```conf
riva_grpc = 127.0.0.1:50051
riva_http = 127.0.0.1:9000
riva_health_path = /v1/health/ready

audio.input = default
audio.fallback = default

paste.enable = true
clipboard_cmd = wl-copy --trim-newline
paste_cmd =

asr.automatic_punctuation = true
asr.language_code = en-US
asr.model =
transcript.trailing_space = true

indicator.enable = true
indicator.sound_enable = true
indicator.sound_start_file = /home/user/sounds/toggle_on.wav
indicator.sound_stop_file = /home/user/sounds/toggle_off.wav
indicator.sound_complete_file = /home/user/sounds/complete.wav
indicator.sound_cancel_file = /home/user/sounds/cancel.wav
indicator.text_recording = Recording…
indicator.text_processing = Transcribing…
indicator.text_error = Speech recognition error
indicator.error_timeout_ms = 1600

vocab.global = internal
vocab.max_phrases = 1024

vocabset internal {
  boost = 14
  phrases = [ "Parakeet", "Riva", "local ASR" ]
}

debug.audio_dump = false
debug.grpc_dump = false
```
