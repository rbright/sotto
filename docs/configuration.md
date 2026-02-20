# Configuration

## Resolution order

`sotto` loads configuration in this order:

1. `--config <path>`
2. `$XDG_CONFIG_HOME/sotto/config.jsonc`
3. `~/.config/sotto/config.jsonc`

If no `.jsonc` file exists at the default path, sotto falls back to legacy `config.conf` for compatibility.

## Format

Preferred format is **JSONC**:

- JSON object root (`{ ... }`)
- line comments (`// ...`)
- block comments (`/* ... */`)
- trailing commas are accepted

Unknown fields are hard errors.

## Schema overview

Top-level object keys:

- `riva`
- `audio`
- `paste`
- `asr`
- `transcript`
- `indicator`
- `clipboard_cmd`
- `paste_cmd`
- `vocab`
- `debug`

## Keys and defaults

### `riva`

| Key | Default | Notes |
| --- | --- | --- |
| `riva.grpc` | `127.0.0.1:50051` | gRPC ASR endpoint |
| `riva.http` | `127.0.0.1:9000` | HTTP endpoint for readiness checks |
| `riva.health_path` | `/v1/health/ready` | must start with `/` |

### `audio`

| Key | Default | Notes |
| --- | --- | --- |
| `audio.input` | `default` | preferred device match |
| `audio.fallback` | `default` | fallback device match |

### `paste`

| Key | Default | Notes |
| --- | --- | --- |
| `paste.enable` | `true` | run paste adapter after clipboard commit |
| `paste.shortcut` | `CTRL,V` | used by default Hyprland paste path when `paste_cmd` unset |

### `asr`

| Key | Default | Notes |
| --- | --- | --- |
| `asr.automatic_punctuation` | `true` | punctuation hint |
| `asr.language_code` | `en-US` | language code |
| `asr.model` | empty | optional explicit model |

### `transcript`

| Key | Default | Notes |
| --- | --- | --- |
| `transcript.trailing_space` | `true` | append space after assembled transcript |

### `indicator`

| Key | Default | Notes |
| --- | --- | --- |
| `indicator.enable` | `true` | visual indicator switch |
| `indicator.backend` | `hypr` | `hypr` or `desktop` |
| `indicator.desktop_app_name` | `sotto-indicator` | required for desktop backend |
| `indicator.sound_enable` | `true` | cue sounds switch |
| `indicator.height` | `28` | indicator size parameter |
| `indicator.error_timeout_ms` | `1600` | `>= 0` |

Indicator text and cue assets are now application-owned (embedded in the binary) and are not user-configurable.
Localization support exists in-code with an English catalog shipped by default.

### command keys

| Key | Default | Notes |
| --- | --- | --- |
| `clipboard_cmd` | `wl-copy --trim-newline` | command argv; no shell execution |
| `paste_cmd` | empty | optional explicit paste command override |

### `vocab`

| Key | Default | Notes |
| --- | --- | --- |
| `vocab.global` | empty | enabled vocab set names (array preferred; comma string also accepted) |
| `vocab.max_phrases` | `1024` | hard cap after dedupe |
| `vocab.sets` | empty map | map of named vocab sets |

Each vocab set object supports:

- `boost` (number)
- `phrases` (string array)

### `debug`

| Key | Default | Notes |
| --- | --- | --- |
| `debug.audio_dump` | `false` | write debug WAV artifacts |
| `debug.grpc_dump` | `false` | write raw ASR response JSON |

## Desktop-notification placement example (mako)

```conf
[app-name="sotto-indicator"]
anchor=top-center
default-timeout=0
```

## Example (`config.jsonc`)

```jsonc
{
  "riva": {
    "grpc": "127.0.0.1:50051",
    "http": "127.0.0.1:9000",
    "health_path": "/v1/health/ready"
  },

  "audio": {
    "input": "default",
    "fallback": "default"
  },

  "paste": {
    "enable": true,
    "shortcut": "CTRL,V"
  },

  "clipboard_cmd": "wl-copy --trim-newline",
  "paste_cmd": "",

  "asr": {
    "automatic_punctuation": true,
    "language_code": "en-US",
    "model": ""
  },

  "transcript": {
    "trailing_space": true
  },

  "indicator": {
    "enable": true,
    "backend": "hypr",
    "desktop_app_name": "sotto-indicator",
    "sound_enable": true,
    "error_timeout_ms": 1600
  },

  "vocab": {
    "global": ["internal"],
    "max_phrases": 1024,
    "sets": {
      "internal": {
        "boost": 14,
        "phrases": ["Parakeet", "Riva", "local ASR"]
      }
    }
  },

  "debug": {
    "audio_dump": false,
    "grpc_dump": false
  }
}
```

## Legacy format compatibility

Legacy `key = value` config files are still accepted to avoid breaking deployed setups.

When a legacy file is parsed, sotto emits a warning so you can migrate to JSONC.
