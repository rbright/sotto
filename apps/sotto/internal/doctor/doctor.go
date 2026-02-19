// Package doctor runs runtime readiness diagnostics for config, tools, audio, and Riva.
package doctor

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/rbright/sotto/internal/audio"
	"github.com/rbright/sotto/internal/config"
)

// Check is one doctor assertion result.
type Check struct {
	Name    string
	Pass    bool
	Message string
}

// Report is the full doctor output contract.
type Report struct {
	Checks []Check
}

// OK returns true when all checks pass.
func (r Report) OK() bool {
	for _, check := range r.Checks {
		if !check.Pass {
			return false
		}
	}
	return true
}

// String renders the report as user-facing text output.
func (r Report) String() string {
	var b strings.Builder
	for _, check := range r.Checks {
		status := "OK"
		if !check.Pass {
			status = "FAIL"
		}
		b.WriteString(fmt.Sprintf("[%s] %s: %s\n", status, check.Name, check.Message))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// Run executes environment/config/runtime checks for a loaded config.
func Run(cfg config.Loaded) Report {
	checks := []Check{}

	checks = append(checks, Check{
		Name:    "config",
		Pass:    true,
		Message: fmt.Sprintf("loaded %q", cfg.Path),
	})

	checks = append(checks, checkEnv("XDG_SESSION_TYPE", func(v string) bool {
		return strings.EqualFold(strings.TrimSpace(v), "wayland")
	}, "session type is wayland", "expected XDG_SESSION_TYPE=wayland"))

	checks = append(checks, checkEnv("HYPRLAND_INSTANCE_SIGNATURE", func(v string) bool {
		return strings.TrimSpace(v) != ""
	}, "Hyprland session detected", "HYPRLAND_INSTANCE_SIGNATURE is empty"))

	checks = append(checks, checkCommand(cfg.Config.Clipboard.Argv, "clipboard_cmd"))

	if cfg.Config.Paste.Enable {
		if len(cfg.Config.PasteCmd.Argv) > 0 {
			checks = append(checks, checkCommand(cfg.Config.PasteCmd.Argv, "paste_cmd"))
		} else {
			checks = append(checks, checkBinary("hyprctl", "default paste path requires hyprctl"))
		}
	}

	checks = append(checks, checkAudioSelection(cfg.Config))
	checks = append(checks, checkRivaReady(cfg.Config))

	return Report{Checks: checks}
}

// checkEnv validates an environment variable through a caller-supplied predicate.
func checkEnv(name string, predicate func(string) bool, okMsg, failMsg string) Check {
	value := os.Getenv(name)
	if predicate(value) {
		return Check{Name: name, Pass: true, Message: okMsg}
	}
	return Check{Name: name, Pass: false, Message: failMsg}
}

// checkCommand validates that argv contains a runnable command.
func checkCommand(argv []string, name string) Check {
	if len(argv) == 0 {
		return Check{Name: name, Pass: false, Message: "command is empty"}
	}
	return checkBinary(argv[0], fmt.Sprintf("%s command is available", name))
}

// checkBinary validates that a binary exists in PATH.
func checkBinary(bin string, okMsg string) Check {
	path, err := exec.LookPath(bin)
	if err != nil {
		return Check{Name: bin, Pass: false, Message: fmt.Sprintf("binary not found in PATH: %s", bin)}
	}
	return Check{Name: bin, Pass: true, Message: fmt.Sprintf("found at %s (%s)", path, okMsg)}
}

// checkAudioSelection runs live device selection to surface selection/fallback issues.
func checkAudioSelection(cfg config.Config) Check {
	selection, err := audio.SelectDevice(context.Background(), cfg.Audio.Input, cfg.Audio.Fallback)
	if err != nil {
		return Check{Name: "audio.device", Pass: false, Message: err.Error()}
	}
	message := fmt.Sprintf("selected %q", selection.Device.ID)
	if selection.Warning != "" {
		message = message + " (" + selection.Warning + ")"
	}
	return Check{Name: "audio.device", Pass: true, Message: message}
}

// checkRivaReady probes the configured Riva HTTP ready endpoint.
func checkRivaReady(cfg config.Config) Check {
	base := strings.TrimSpace(cfg.RivaHTTP)
	if base == "" {
		return Check{Name: "riva.ready", Pass: false, Message: "riva_http is empty"}
	}
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "http://" + base
	}

	url := strings.TrimRight(base, "/") + cfg.RivaHealthPath
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return Check{Name: "riva.ready", Pass: false, Message: fmt.Sprintf("request failed: %v", err)}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Check{Name: "riva.ready", Pass: false, Message: fmt.Sprintf("HTTP %d from %s", resp.StatusCode, url)}
	}

	bodyText := strings.ToLower(strings.TrimSpace(string(body)))
	if bodyText != "" && !strings.Contains(bodyText, "ready") {
		return Check{Name: "riva.ready", Pass: true, Message: fmt.Sprintf("HTTP %d from %s", resp.StatusCode, url)}
	}

	return Check{Name: "riva.ready", Pass: true, Message: fmt.Sprintf("ready at %s", url)}
}
