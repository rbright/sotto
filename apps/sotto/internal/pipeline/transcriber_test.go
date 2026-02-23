package pipeline

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rbright/sotto/internal/audio"
	"github.com/rbright/sotto/internal/config"
	"github.com/rbright/sotto/internal/riva"
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

func TestSendLoopSignalsPipelineUnavailableWhenChannelPresent(t *testing.T) {
	transcriber := NewTranscriber(config.Default(), nil)
	transcriber.sendErrCh = make(chan error, 1)

	transcriber.sendLoop()

	err := <-transcriber.sendErrCh
	require.ErrorIs(t, err, session.ErrPipelineUnavailable)
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

func TestCloseDebugArtifactsLockedClosesFileWhileMutexHeld(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "*.json")
	require.NoError(t, err)

	transcriber := NewTranscriber(config.Default(), nil)
	transcriber.debugGRPCFile = file

	transcriber.mu.Lock()
	transcriber.closeDebugArtifactsLocked()
	transcriber.mu.Unlock()

	_, err = file.Write([]byte("x"))
	require.Error(t, err)
	require.Nil(t, transcriber.debugGRPCFile)
}

func TestStartWiresDependenciesAndBootsSendLoop(t *testing.T) {
	cfg := config.Default()
	transcriber := NewTranscriber(cfg, nil)

	chunks := make(chan []byte)
	close(chunks)
	capture := &fakeCapture{chunks: chunks}
	stream := &fakeStream{}

	transcriber.selectDevice = func(context.Context, string, string) (audio.Selection, error) {
		return audio.Selection{Device: audio.Device{ID: "mic-1", Description: "Mic"}}, nil
	}
	transcriber.dialStream = func(context.Context, riva.StreamConfig) (streamClient, error) {
		return stream, nil
	}
	transcriber.startCapture = func(context.Context, audio.Device) (captureClient, error) {
		return capture, nil
	}

	err := transcriber.Start(context.Background())
	require.NoError(t, err)
	require.True(t, transcriber.started)
	require.Equal(t, "mic-1", transcriber.selection.Device.ID)
	require.NotNil(t, transcriber.sendErrCh)

	require.NoError(t, transcriber.Cancel(context.Background()))
}

func TestStartFailsOnSpeechPhraseBuildError(t *testing.T) {
	cfg := config.Default()
	cfg.Vocab.GlobalSets = []string{"missing"}

	transcriber := NewTranscriber(cfg, nil)
	transcriber.selectDevice = func(context.Context, string, string) (audio.Selection, error) {
		return audio.Selection{Device: audio.Device{ID: "mic-1", Description: "Mic"}}, nil
	}
	transcriber.dialStream = func(context.Context, riva.StreamConfig) (streamClient, error) {
		t.Fatal("dialStream should not be called when speech phrase build fails")
		return nil, nil
	}
	transcriber.startCapture = func(context.Context, audio.Device) (captureClient, error) {
		t.Fatal("startCapture should not be called when speech phrase build fails")
		return nil, nil
	}

	err := transcriber.Start(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "build speech contexts")
	require.False(t, transcriber.started)
}

func TestStopAndTranscribeSuccessPath(t *testing.T) {
	cfg := config.Default()
	cfg.Transcript.TrailingSpace = true

	capture := &fakeCapture{
		chunks: make(chan []byte),
		raw:    []byte{1, 2, 3, 4},
		bytes:  4096,
	}
	close(capture.chunks)

	stream := &fakeStream{
		closeSegments: []string{"hello", "world"},
		closeLatency:  12 * time.Millisecond,
	}

	transcriber := NewTranscriber(cfg, nil)
	transcriber.started = true
	transcriber.selection = audio.Selection{Device: audio.Device{ID: "mic-1", Description: "Mic"}}
	transcriber.capture = capture
	transcriber.stream = stream
	transcriber.sendErrCh = make(chan error, 1)
	transcriber.sendErrCh <- nil

	result, err := transcriber.StopAndTranscribe(context.Background())
	require.NoError(t, err)
	require.Equal(t, "hello world ", result.Transcript)
	require.Equal(t, "Mic (mic-1)", result.AudioDevice)
	require.Equal(t, int64(4096), result.BytesCaptured)
	require.Equal(t, 12*time.Millisecond, result.GRPCLatency)
	require.True(t, capture.stopCalled)
	require.False(t, transcriber.started)
	require.Nil(t, transcriber.capture)
	require.Nil(t, transcriber.stream)
}

func TestStopAndTranscribeSendErrorCancelsStream(t *testing.T) {
	capture := &fakeCapture{
		chunks: make(chan []byte),
		raw:    []byte{1, 2, 3, 4},
		bytes:  2048,
	}
	close(capture.chunks)
	stream := &fakeStream{}

	transcriber := NewTranscriber(config.Default(), nil)
	transcriber.started = true
	transcriber.selection = audio.Selection{Device: audio.Device{ID: "mic-1", Description: "Mic"}}
	transcriber.capture = capture
	transcriber.stream = stream
	transcriber.sendErrCh = make(chan error, 1)
	transcriber.sendErrCh <- errors.New("send failed")

	result, err := transcriber.StopAndTranscribe(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "send audio stream")
	require.Equal(t, "Mic (mic-1)", result.AudioDevice)
	require.True(t, stream.cancelCalled)
}

func TestStopAndTranscribeCollectErrorIncludesLatency(t *testing.T) {
	capture := &fakeCapture{
		chunks: make(chan []byte),
		raw:    []byte{1, 2, 3, 4},
		bytes:  1024,
	}
	close(capture.chunks)

	stream := &fakeStream{closeErr: errors.New("close failed"), closeLatency: 77 * time.Millisecond}

	transcriber := NewTranscriber(config.Default(), nil)
	transcriber.started = true
	transcriber.selection = audio.Selection{Device: audio.Device{ID: "mic-1", Description: "Mic"}}
	transcriber.capture = capture
	transcriber.stream = stream
	transcriber.sendErrCh = make(chan error, 1)
	transcriber.sendErrCh <- nil

	result, err := transcriber.StopAndTranscribe(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "collect final transcript")
	require.Equal(t, 77*time.Millisecond, result.GRPCLatency)
}

func TestCancelStopsCaptureAndStreamAndResetsState(t *testing.T) {
	capture := &fakeCapture{chunks: make(chan []byte), raw: []byte{1}, bytes: 1}
	close(capture.chunks)
	stream := &fakeStream{}

	transcriber := NewTranscriber(config.Default(), nil)
	transcriber.started = true
	transcriber.capture = capture
	transcriber.stream = stream
	transcriber.sendErrCh = make(chan error, 1)

	err := transcriber.Cancel(context.Background())
	require.NoError(t, err)
	require.True(t, capture.stopCalled)
	require.True(t, stream.cancelCalled)
	require.False(t, transcriber.started)
	require.Nil(t, transcriber.capture)
	require.Nil(t, transcriber.stream)
	require.Nil(t, transcriber.sendErrCh)
}

func TestSendLoopForwardsChunksAndSignalsNil(t *testing.T) {
	chunks := make(chan []byte, 4)
	chunks <- []byte{1, 2, 3}
	chunks <- []byte{}
	chunks <- []byte{4, 5}
	close(chunks)

	capture := &fakeCapture{chunks: chunks}
	stream := &fakeStream{}
	transcriber := NewTranscriber(config.Default(), nil)
	transcriber.capture = capture
	transcriber.stream = stream
	transcriber.sendErrCh = make(chan error, 1)

	transcriber.sendLoop()

	err := <-transcriber.sendErrCh
	require.NoError(t, err)
	require.Equal(t, 2, len(stream.sendChunks))
	require.Equal(t, []byte{1, 2, 3}, stream.sendChunks[0])
	require.Equal(t, []byte{4, 5}, stream.sendChunks[1])
}

func TestSendLoopStopsCaptureOnSendError(t *testing.T) {
	chunks := make(chan []byte, 2)
	chunks <- []byte{1, 2, 3}
	close(chunks)

	capture := &fakeCapture{chunks: chunks}
	stream := &fakeStream{sendErr: errors.New("boom")}
	transcriber := NewTranscriber(config.Default(), nil)
	transcriber.capture = capture
	transcriber.stream = stream
	transcriber.sendErrCh = make(chan error, 1)

	transcriber.sendLoop()

	err := <-transcriber.sendErrCh
	require.Error(t, err)
	require.Contains(t, err.Error(), "boom")
	require.True(t, capture.stopCalled)
}

type fakeCapture struct {
	chunks     chan []byte
	stopErr    error
	raw        []byte
	bytes      int64
	stopCalled bool
}

func (f *fakeCapture) Stop() error {
	f.stopCalled = true
	return f.stopErr
}

func (f *fakeCapture) Chunks() <-chan []byte { return f.chunks }

func (f *fakeCapture) BytesCaptured() int64 { return f.bytes }

func (f *fakeCapture) RawPCM() []byte {
	out := make([]byte, len(f.raw))
	copy(out, f.raw)
	return out
}

type fakeStream struct {
	sendErr       error
	closeErr      error
	closeSegments []string
	closeLatency  time.Duration
	cancelCalled  bool
	sendChunks    [][]byte
}

func (f *fakeStream) SendAudio(chunk []byte) error {
	if f.sendErr != nil {
		return f.sendErr
	}
	copyChunk := make([]byte, len(chunk))
	copy(copyChunk, chunk)
	f.sendChunks = append(f.sendChunks, copyChunk)
	return nil
}

func (f *fakeStream) CloseAndCollect(context.Context) ([]string, time.Duration, error) {
	if f.closeErr != nil {
		return nil, f.closeLatency, f.closeErr
	}
	segments := append([]string(nil), f.closeSegments...)
	return segments, f.closeLatency, nil
}

func (f *fakeStream) Cancel() error {
	f.cancelCalled = true
	return nil
}
