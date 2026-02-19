package pipeline

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/rbright/sotto/internal/audio"
	"github.com/rbright/sotto/internal/config"
	"github.com/rbright/sotto/internal/riva"
	"github.com/rbright/sotto/internal/session"
	"github.com/rbright/sotto/internal/transcript"
)

// Transcriber owns one end-to-end capture -> ASR -> transcript pipeline instance.
type Transcriber struct {
	cfg    config.Config
	logger *slog.Logger

	mu      sync.Mutex
	started bool

	selection audio.Selection
	capture   *audio.Capture
	stream    *riva.Stream

	sendErrCh chan error

	debugGRPCFile *os.File
}

// NewTranscriber constructs a pipeline transcriber from runtime config.
func NewTranscriber(cfg config.Config, logger *slog.Logger) *Transcriber {
	return &Transcriber{cfg: cfg, logger: logger}
}

// Start resolves device selection, opens Riva stream, and starts audio capture.
func (t *Transcriber) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.started {
		return fmt.Errorf("transcriber already started")
	}

	selection, err := audio.SelectDevice(ctx, t.cfg.Audio.Input, t.cfg.Audio.Fallback)
	if err != nil {
		return err
	}
	t.selection = selection
	if selection.Warning != "" {
		t.logWarn(selection.Warning)
	}

	speechPhrases, _, err := config.BuildSpeechPhrases(t.cfg)
	if err != nil {
		return fmt.Errorf("build speech contexts: %w", err)
	}

	if t.cfg.Debug.EnableGRPCDump {
		file, ferr := createDebugFile("grpc", "json")
		if ferr != nil {
			return ferr
		}
		t.debugGRPCFile = file
	}

	rivaPhrases := make([]riva.SpeechPhrase, 0, len(speechPhrases))
	for _, phrase := range speechPhrases {
		rivaPhrases = append(rivaPhrases, riva.SpeechPhrase{Phrase: phrase.Phrase, Boost: phrase.Boost})
	}

	stream, err := riva.DialStream(ctx, riva.StreamConfig{
		Endpoint:             t.cfg.RivaGRPC,
		LanguageCode:         t.cfg.ASR.LanguageCode,
		Model:                t.cfg.ASR.Model,
		AutomaticPunctuation: t.cfg.ASR.AutomaticPunctuation,
		SpeechPhrases:        rivaPhrases,
		DialTimeout:          3 * time.Second,
		DebugResponseSinkJSON: func() *os.File {
			if t.debugGRPCFile == nil {
				return nil
			}
			return t.debugGRPCFile
		}(),
	})
	if err != nil {
		t.closeDebugArtifacts()
		return err
	}
	t.stream = stream

	capture, err := audio.StartCapture(ctx, selection.Device)
	if err != nil {
		_ = stream.Cancel()
		t.closeDebugArtifacts()
		return err
	}
	t.capture = capture

	t.sendErrCh = make(chan error, 1)
	go t.sendLoop()

	t.started = true
	return nil
}

// StopAndTranscribe stops capture, closes stream, and assembles the transcript.
func (t *Transcriber) StopAndTranscribe(ctx context.Context) (session.StopResult, error) {
	t.mu.Lock()
	started := t.started
	capture := t.capture
	stream := t.stream
	sendErrCh := t.sendErrCh
	selection := t.selection
	t.mu.Unlock()

	if !started || capture == nil || stream == nil {
		return session.StopResult{}, session.ErrPipelineUnavailable
	}

	_ = capture.Stop()

	var sendErr error
	if sendErrCh != nil {
		sendErr = <-sendErrCh
	}
	if sendErr != nil {
		_ = stream.Cancel()
		result := session.StopResult{
			AudioDevice:   describeDevice(selection.Device),
			BytesCaptured: capture.BytesCaptured(),
		}
		t.writeDebugAudio(capture.RawPCM())
		t.closeDebugArtifacts()
		return result, fmt.Errorf("send audio stream: %w", sendErr)
	}

	closeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	segments, grpcLatency, err := stream.CloseAndCollect(closeCtx)
	if err != nil {
		result := session.StopResult{
			AudioDevice:   describeDevice(selection.Device),
			BytesCaptured: capture.BytesCaptured(),
			GRPCLatency:   grpcLatency,
		}
		t.writeDebugAudio(capture.RawPCM())
		t.closeDebugArtifacts()
		return result, fmt.Errorf("collect final transcript: %w", err)
	}

	transcribed := transcript.Assemble(segments, t.cfg.Transcript.TrailingSpace)
	rawPCM := capture.RawPCM()
	t.writeDebugAudio(rawPCM)
	t.closeDebugArtifacts()

	return session.StopResult{
		Transcript:    transcribed,
		AudioDevice:   describeDevice(selection.Device),
		BytesCaptured: capture.BytesCaptured(),
		GRPCLatency:   grpcLatency,
	}, nil
}

// Cancel stops capture and stream immediately without transcript commit.
func (t *Transcriber) Cancel(_ context.Context) error {
	t.mu.Lock()
	capture := t.capture
	stream := t.stream
	t.mu.Unlock()

	if capture != nil {
		_ = capture.Stop()
		t.writeDebugAudio(capture.RawPCM())
	}
	if stream != nil {
		_ = stream.Cancel()
	}
	t.closeDebugArtifacts()
	return nil
}

// sendLoop forwards capture chunks to Riva and reports the first send failure.
func (t *Transcriber) sendLoop() {
	t.mu.Lock()
	capture := t.capture
	stream := t.stream
	errCh := t.sendErrCh
	t.mu.Unlock()

	if errCh == nil {
		return
	}

	sent := false
	sendResult := func(err error) {
		if sent {
			return
		}
		errCh <- err
		sent = true
	}
	defer sendResult(nil)

	if capture == nil || stream == nil {
		sendResult(session.ErrPipelineUnavailable)
		return
	}

	for chunk := range capture.Chunks() {
		if len(chunk) == 0 {
			continue
		}
		if err := stream.SendAudio(chunk); err != nil {
			_ = capture.Stop()
			sendResult(err)
			return
		}
	}
}

// describeDevice formats device metadata for logs/session results.
func describeDevice(device audio.Device) string {
	description := strings.TrimSpace(device.Description)
	id := strings.TrimSpace(device.ID)
	if description == "" {
		return id
	}
	if id == "" {
		return description
	}
	return fmt.Sprintf("%s (%s)", description, id)
}

// logWarn emits warning-level logs when logger is configured.
func (t *Transcriber) logWarn(message string) {
	if t.logger == nil {
		return
	}
	t.logger.Warn(message)
}

// createDebugFile creates timestamped debug artifacts under state/sotto/debug.
func createDebugFile(prefix string, extension string) (*os.File, error) {
	stateDir, err := resolveStateDir()
	if err != nil {
		return nil, err
	}
	debugDir := filepath.Join(stateDir, "sotto", "debug")
	if err := os.MkdirAll(debugDir, 0o700); err != nil {
		return nil, fmt.Errorf("create debug dir: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405.000")
	path := filepath.Join(debugDir, fmt.Sprintf("%s-%s.%s", prefix, timestamp, extension))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open debug file %q: %w", path, err)
	}
	return file, nil
}

// resolveStateDir returns XDG_STATE_HOME fallback path for debug artifacts.
func resolveStateDir() (string, error) {
	if xdg := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); xdg != "" {
		return xdg, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory for state: %w", err)
	}
	return filepath.Join(home, ".local", "state"), nil
}

// closeDebugArtifacts closes open debug sinks.
func (t *Transcriber) closeDebugArtifacts() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.debugGRPCFile != nil {
		_ = t.debugGRPCFile.Close()
		t.debugGRPCFile = nil
	}
}

// writeDebugAudio writes raw PCM to WAV when debug.audio_dump is enabled.
func (t *Transcriber) writeDebugAudio(rawPCM []byte) {
	if !t.cfg.Debug.EnableAudioDump || len(rawPCM) == 0 {
		return
	}

	file, err := createDebugFile("audio", "wav")
	if err != nil {
		t.logWarn(fmt.Sprintf("unable to create debug audio dump: %v", err))
		return
	}
	defer file.Close()

	if err := writePCM16WAV(file, rawPCM, 16000, 1); err != nil {
		t.logWarn(fmt.Sprintf("unable to write debug audio dump: %v", err))
	}
}

// writePCM16WAV writes raw little-endian PCM bytes with a minimal WAV header.
func writePCM16WAV(file *os.File, pcm []byte, sampleRate int, channels int) error {
	if channels <= 0 {
		channels = 1
	}
	const bitsPerSample = 16
	byteRate := sampleRate * channels * (bitsPerSample / 8)
	blockAlign := channels * (bitsPerSample / 8)

	chunkSize := uint32(36 + len(pcm))
	subChunk2Size := uint32(len(pcm))

	header := make([]byte, 44)
	copy(header[0:4], []byte("RIFF"))
	binary.LittleEndian.PutUint32(header[4:8], chunkSize)
	copy(header[8:12], []byte("WAVE"))
	copy(header[12:16], []byte("fmt "))
	binary.LittleEndian.PutUint32(header[16:20], 16)
	binary.LittleEndian.PutUint16(header[20:22], 1) // PCM
	binary.LittleEndian.PutUint16(header[22:24], uint16(channels))
	binary.LittleEndian.PutUint32(header[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(header[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(header[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(header[34:36], bitsPerSample)
	copy(header[36:40], []byte("data"))
	binary.LittleEndian.PutUint32(header[40:44], subChunk2Size)

	if _, err := file.Write(header); err != nil {
		return err
	}
	_, err := file.Write(pcm)
	return err
}
