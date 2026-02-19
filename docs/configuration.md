# Configuration

## Resolution order

`sotto` loads configuration in this order:

1. `--config <path>`
2. `$XDG_CONFIG_HOME/sotto/config.conf`
3. `~/.config/sotto/config.conf`

If no config file exists, defaults are used.

## Grammar

- `key = value`
- comments start with `#`
- string values may be quoted (`"..."` or `'...'`) or unquoted
- vocab blocks:

```conf
vocabset <name> {
  boost = <float>
  phrases = [ "phrase one", "phrase two" ]
}
```

Unknown keys are hard errors.

## Keys and defaults

### Core endpoints and audio

| Key | Default | Notes |
| --- | --- | --- |
| `riva_grpc` | `127.0.0.1:50051` | gRPC ASR endpoint |
| `riva_http` | `127.0.0.1:9000` | HTTP endpoint for readiness checks |
| `riva_health_path` | `/v1/health/ready` | must start with `/` |
| `audio.input` | `default` | preferred device match |
| `audio.fallback` | `default` | fallback device match |

### Output and transcript

| Key | Default | Notes |
| --- | --- | --- |
| `paste.enable` | `true` | run paste adapter after clipboard commit |
| `paste.shortcut` | `CTRL,V` | used by default Hyprland paste path when `paste_cmd` unset |
| `clipboard_cmd` | `wl-copy --trim-newline` | command argv; no shell execution |
| `paste_cmd` | empty | optional explicit paste command override |
| `transcript.trailing_space` | `true` | append space after assembled transcript |

### ASR

| Key | Default | Notes |
| --- | --- | --- |
| `asr.automatic_punctuation` | `true` | punctuation hint |
| `asr.language_code` | `en-US` | language code |
| `asr.model` | empty | optional explicit model |

### Indicator + cues

| Key | Default | Notes |
| --- | --- | --- |
| `indicator.enable` | `true` | visual indicator switch |
| `indicator.backend` | `hypr` | `hypr` or `desktop` |
| `indicator.desktop_app_name` | `sotto-indicator` | required for desktop backend |
| `indicator.sound_enable` | `true` | cue sounds switch |
| `indicator.sound_start_file` | empty | optional WAV path |
| `indicator.sound_stop_file` | empty | optional WAV path |
| `indicator.sound_complete_file` | empty | optional WAV path |
| `indicator.sound_cancel_file` | empty | optional WAV path |
| `indicator.height` | `28` | indicator size parameter |
| `indicator.text_recording` | `Recording…` | recording label |
| `indicator.text_processing` | `Transcribing…` | processing label |
| `indicator.text_transcribing` | alias | compatibility alias for `indicator.text_processing` |
| `indicator.text_error` | `Speech recognition error` | error label |
| `indicator.error_timeout_ms` | `1600` | `>= 0` |

### Vocabulary and debug

| Key | Default | Notes |
| --- | --- | --- |
| `vocab.global` | empty | comma-separated enabled vocabsets |
| `vocab.max_phrases` | `1024` | hard cap after dedupe |
| `debug.audio_dump` | `false` | write debug WAV artifacts |
| `debug.grpc_dump` | `false` | write raw ASR response JSON |

## Desktop-notification placement example (mako)

```conf
[app-name="sotto-indicator"]
anchor=top-center
default-timeout=0
```

## Example config

```conf
riva_grpc = 127.0.0.1:50051
riva_http = 127.0.0.1:9000
riva_health_path = /v1/health/ready

audio.input = default
audio.fallback = default

paste.enable = true
paste.shortcut = CTRL,V
clipboard_cmd = wl-copy --trim-newline
paste_cmd = ""

asr.automatic_punctuation = true
asr.language_code = en-US
asr.model =
transcript.trailing_space = true

indicator.enable = true
indicator.backend = hypr
indicator.desktop_app_name = sotto-indicator
indicator.sound_enable = true
indicator.sound_start_file =
indicator.sound_stop_file =
indicator.sound_complete_file =
indicator.sound_cancel_file =
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
