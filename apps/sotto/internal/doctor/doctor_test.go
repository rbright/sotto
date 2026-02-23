package doctor

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rbright/sotto/internal/config"
	"github.com/stretchr/testify/require"
)

func TestReportOKAndString(t *testing.T) {
	report := Report{Checks: []Check{
		{Name: "one", Pass: true, Message: "good"},
		{Name: "two", Pass: false, Message: "bad"},
	}}

	require.False(t, report.OK())
	text := report.String()
	require.Contains(t, text, "[OK] one: good")
	require.Contains(t, text, "[FAIL] two: bad")
}

func TestCheckEnv(t *testing.T) {
	t.Setenv("TEST_DOCTOR_ENV", "wayland")

	check := checkEnv(
		"TEST_DOCTOR_ENV",
		func(v string) bool { return strings.EqualFold(v, "wayland") },
		"looks good",
		"unexpected",
	)

	require.True(t, check.Pass)
	require.Equal(t, "looks good", check.Message)
}

func TestCheckCommandEmpty(t *testing.T) {
	check := checkCommand(nil, "clipboard_cmd")
	require.False(t, check.Pass)
	require.Contains(t, check.Message, "command is empty")
}

func TestCheckBinaryFound(t *testing.T) {
	check := checkBinary("sh", "shell available")
	require.True(t, check.Pass)
	require.Contains(t, check.Message, "shell available")
}

func TestCheckBinaryMissing(t *testing.T) {
	check := checkBinary("definitely-not-a-real-binary", "unused")
	require.False(t, check.Pass)
	require.Contains(t, check.Message, "binary not found")
}

func TestCheckCommandUsesBinaryFromPath(t *testing.T) {
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "fake-bin")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/usr/bin/env bash\nexit 0\n"), 0o755))
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	check := checkCommand([]string{"fake-bin", "--arg"}, "clipboard_cmd")
	require.True(t, check.Pass)
	require.Contains(t, check.Message, "clipboard_cmd command is available")
}

func TestCheckRivaReadySuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/health/ready", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	}))
	t.Cleanup(server.Close)

	cfg := config.Default()
	cfg.RivaHTTP = strings.TrimPrefix(server.URL, "http://")
	cfg.RivaHealthPath = "/v1/health/ready"

	check := checkRivaReady(cfg)
	require.True(t, check.Pass)
	require.Contains(t, check.Message, "ready at")
}

func TestCheckRivaReadyFailureStatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	cfg := config.Default()
	cfg.RivaHTTP = strings.TrimPrefix(server.URL, "http://")
	cfg.RivaHealthPath = "/v1/health/ready"

	check := checkRivaReady(cfg)
	require.False(t, check.Pass)
	require.Contains(t, check.Message, "HTTP 503")
}

func TestCheckRivaReadyPassesOnHTTP200NonReadyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("warming-up"))
	}))
	t.Cleanup(server.Close)

	cfg := config.Default()
	cfg.RivaHTTP = strings.TrimPrefix(server.URL, "http://")
	cfg.RivaHealthPath = "/v1/health/ready"

	check := checkRivaReady(cfg)
	require.True(t, check.Pass)
	require.Contains(t, check.Message, "HTTP 200")
}

func TestCheckRivaReadyEmptyBaseURL(t *testing.T) {
	cfg := config.Default()
	cfg.RivaHTTP = ""

	check := checkRivaReady(cfg)
	require.False(t, check.Pass)
	require.Contains(t, check.Message, "riva_http is empty")
}

func TestCheckAudioSelectionFailureWithInvalidPulseServer(t *testing.T) {
	t.Setenv("PULSE_SERVER", "unix:/tmp/definitely-missing-pulse-server")

	check := checkAudioSelection(config.Default())
	require.False(t, check.Pass)
	require.Contains(t, check.Name, "audio.device")
}

func TestReportOKAllPassing(t *testing.T) {
	report := Report{Checks: []Check{{Name: "one", Pass: true}, {Name: "two", Pass: true}}}
	require.True(t, report.OK())
}

func TestRunUsesPasteCmdOverrideCheck(t *testing.T) {
	binDir := t.TempDir()
	fakePaste := filepath.Join(binDir, "fake-paste")
	require.NoError(t, os.WriteFile(fakePaste, []byte("#!/usr/bin/env sh\nexit 0\n"), 0o755))
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	t.Setenv("PULSE_SERVER", "unix:/tmp/definitely-missing-pulse-server")
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "abc123")

	cfg := config.Default()
	cfg.Paste.Enable = true
	cfg.PasteCmd = config.CommandConfig{Raw: fakePaste, Argv: []string{"fake-paste"}}
	cfg.RivaHTTP = ""

	report := Run(config.Loaded{Path: "/tmp/config.jsonc", Config: cfg})
	require.NotEmpty(t, report.Checks)

	var sawPasteCmd, sawHypr bool
	for _, check := range report.Checks {
		if check.Name == "fake-paste" {
			sawPasteCmd = true
		}
		if check.Name == "hyprctl" {
			sawHypr = true
		}
	}
	require.True(t, sawPasteCmd)
	require.False(t, sawHypr)
}

func TestRunUsesHyprctlWhenPasteCmdUnset(t *testing.T) {
	binDir := t.TempDir()
	fakeHypr := filepath.Join(binDir, "hyprctl")
	require.NoError(t, os.WriteFile(fakeHypr, []byte("#!/usr/bin/env sh\nexit 0\n"), 0o755))
	t.Setenv("PATH", binDir+":"+os.Getenv("PATH"))
	t.Setenv("PULSE_SERVER", "unix:/tmp/definitely-missing-pulse-server")
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	t.Setenv("HYPRLAND_INSTANCE_SIGNATURE", "abc123")

	cfg := config.Default()
	cfg.Paste.Enable = true
	cfg.PasteCmd = config.CommandConfig{}
	cfg.RivaHTTP = ""

	report := Run(config.Loaded{Path: "/tmp/config.jsonc", Config: cfg})
	require.NotEmpty(t, report.Checks)

	var sawHypr bool
	for _, check := range report.Checks {
		if check.Name == "hyprctl" {
			sawHypr = true
			break
		}
	}
	require.True(t, sawHypr)
}
