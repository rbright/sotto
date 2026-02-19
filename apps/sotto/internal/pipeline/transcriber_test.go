package pipeline

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/rbright/sotto/internal/audio"
	"github.com/rbright/sotto/internal/config"
	"github.com/rbright/sotto/internal/session"
	"github.com/stretchr/testify/require"
)

func TestDescribeDevice(t *testing.T) {
	require.Equal(t, "Elgato (alsa_input.wave3)", describeDevice(audio.Device{Description: "Elgato", ID: "alsa_input.wave3"}))
	require.Equal(t, "Elgato", describeDevice(audio.Device{Description: "Elgato"}))
	require.Equal(t, "alsa_input.wave3", describeDevice(audio.Device{ID: "alsa_input.wave3"}))
}

func TestResolveStateDirUsesXDGStateHome(t *testing.T) {
	xdgStateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", xdgStateHome)
	t.Setenv("HOME", t.TempDir())

	dir, err := resolveStateDir()
	require.NoError(t, err)
	require.Equal(t, xdgStateHome, dir)
}

func TestResolveStateDirFallsBackToHome(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", home)

	dir, err := resolveStateDir()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(home, ".local", "state"), dir)
}

func TestCreateDebugFileCreatesExpectedPath(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	file, err := createDebugFile("grpc", "json")
	require.NoError(t, err)
	path := file.Name()
	require.NoError(t, file.Close())

	require.FileExists(t, path)
	require.Contains(t, path, string(filepath.Separator)+"sotto"+string(filepath.Separator)+"debug"+string(filepath.Separator))
	require.Contains(t, filepath.Base(path), "grpc-")
	require.Equal(t, ".json", filepath.Ext(path))

	stat, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), stat.Mode().Perm())
}

func TestWritePCM16WAVWritesHeaderAndPCM(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "*.wav")
	require.NoError(t, err)

	pcm := []byte{0x01, 0x00, 0xFF, 0x7F}
	require.NoError(t, writePCM16WAV(file, pcm, 16000, 0))
	require.NoError(t, file.Close())

	data, err := os.ReadFile(file.Name())
	require.NoError(t, err)
	require.Len(t, data, 44+len(pcm))

	require.Equal(t, "RIFF", string(data[0:4]))
	require.Equal(t, "WAVE", string(data[8:12]))
	require.Equal(t, "fmt ", string(data[12:16]))
	require.Equal(t, "data", string(data[36:40]))
	require.Equal(t, uint16(1), binary.LittleEndian.Uint16(data[22:24])) // channels default to mono
	require.Equal(t, uint32(len(pcm)), binary.LittleEndian.Uint32(data[40:44]))
	require.Equal(t, pcm, data[44:])
}

func TestWriteDebugAudioCreatesWavWhenEnabled(t *testing.T) {
	xdgStateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", xdgStateHome)

	cfg := config.Default()
	cfg.Debug.EnableAudioDump = true
	transcriber := NewTranscriber(cfg, nil)

	transcriber.writeDebugAudio([]byte{0x01, 0x00, 0x02, 0x00})

	matches, err := filepath.Glob(filepath.Join(xdgStateHome, "sotto", "debug", "audio-*.wav"))
	require.NoError(t, err)
	require.NotEmpty(t, matches)
}

func TestWriteDebugAudioSkippedWhenDisabled(t *testing.T) {
	xdgStateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", xdgStateHome)

	cfg := config.Default()
	cfg.Debug.EnableAudioDump = false
	transcriber := NewTranscriber(cfg, nil)

	transcriber.writeDebugAudio([]byte{0x01, 0x00, 0x02, 0x00})

	matches, err := filepath.Glob(filepath.Join(xdgStateHome, "sotto", "debug", "audio-*.wav"))
	require.NoError(t, err)
	require.Empty(t, matches)
}

func TestStartFailsWhenAlreadyStarted(t *testing.T) {
	transcriber := NewTranscriber(config.Default(), nil)
	transcriber.started = true

	err := transcriber.Start(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "already started")
}

func TestStartFailsWhenAudioSelectionUnavailable(t *testing.T) {
	t.Setenv("PULSE_SERVER", "unix:/tmp/definitely-missing-pulse-server")

	transcriber := NewTranscriber(config.Default(), nil)
	err := transcriber.Start(context.Background())
	require.Error(t, err)
}

func TestStopAndTranscribeUnavailableWhenNotStarted(t *testing.T) {
	result, err := NewTranscriber(config.Default(), nil).StopAndTranscribe(context.Background())
	require.ErrorIs(t, err, session.ErrPipelineUnavailable)
	require.Equal(t, session.StopResult{}, result)
}

func TestCancelWithoutInitializedPipeline(t *testing.T) {
	transcriber := NewTranscriber(config.Default(), nil)
	require.NoError(t, transcriber.Cancel(context.Background()))
}

func TestSendLoopNoopWhenUninitialized(t *testing.T) {
	transcriber := NewTranscriber(config.Default(), nil)
	transcriber.sendLoop() // should return immediately without panic
}

func TestCloseDebugArtifactsClosesFile(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "*.json")
	require.NoError(t, err)

	transcriber := NewTranscriber(config.Default(), nil)
	transcriber.debugGRPCFile = file
	transcriber.closeDebugArtifacts()

	_, err = file.Write([]byte("x"))
	require.Error(t, err)
	require.Nil(t, transcriber.debugGRPCFile)
}
