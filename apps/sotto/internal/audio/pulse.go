// Package audio handles device discovery, selection, and PCM capture streams.
package audio

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/jfreymuth/pulse"
	pulseproto "github.com/jfreymuth/pulse/proto"
)

const (
	chunkSizeBytes = 640 // 20ms @ 16kHz mono s16
)

// Device describes one Pulse input source surfaced to sotto.
type Device struct {
	ID          string
	Description string
	State       string
	Available   bool
	Muted       bool
	Default     bool
}

// Selection is the resolved capture source plus optional fallback warning context.
type Selection struct {
	Device   Device
	Warning  string
	Fallback bool
}

// ListDevices returns available Pulse input sources with default/availability metadata.
func ListDevices(_ context.Context) ([]Device, error) {
	client, err := pulse.NewClient(
		pulse.ClientApplicationName("sotto"),
		pulse.ClientApplicationIconName("audio-input-microphone"),
	)
	if err != nil {
		return nil, fmt.Errorf("connect pulse server: %w", err)
	}
	defer client.Close()

	defaultSource, err := client.DefaultSource()
	if err != nil {
		return nil, fmt.Errorf("read default source: %w", err)
	}
	defaultID := defaultSource.ID()

	var sourceInfos pulseproto.GetSourceInfoListReply
	if err := client.RawRequest(&pulseproto.GetSourceInfoList{}, &sourceInfos); err != nil {
		return nil, fmt.Errorf("list sources: %w", err)
	}

	devices := make([]Device, 0, len(sourceInfos))
	for _, source := range sourceInfos {
		if source == nil {
			continue
		}
		devices = append(devices, Device{
			ID:          source.SourceName,
			Description: source.Device,
			State:       sourceStateString(source.State),
			Available:   sourceAvailable(source),
			Muted:       source.Mute,
			Default:     source.SourceName == defaultID,
		})
	}
	return devices, nil
}

// SelectDevice resolves audio.input/audio.fallback preferences against live devices.
func SelectDevice(ctx context.Context, input string, fallback string) (Selection, error) {
	devices, err := ListDevices(ctx)
	if err != nil {
		return Selection{}, err
	}
	return selectDeviceFromList(devices, input, fallback)
}

// selectDeviceFromList applies selection policy to a pre-fetched device list.
func selectDeviceFromList(devices []Device, input string, fallback string) (Selection, error) {
	if len(devices) == 0 {
		return Selection{}, errors.New("no audio input devices found")
	}

	var (
		defaultDevice *Device
		byInput       *Device
		byFallback    *Device
	)

	input = strings.TrimSpace(strings.ToLower(input))
	fallback = strings.TrimSpace(strings.ToLower(fallback))

	for i := range devices {
		dev := &devices[i]
		if dev.Default {
			defaultDevice = dev
		}
		if byInput == nil && input != "" && input != "default" && deviceMatches(*dev, input) {
			byInput = dev
		}
		if byFallback == nil && fallback != "" && fallback != "default" && deviceMatches(*dev, fallback) {
			byFallback = dev
		}
	}

	chooseDefault := func() (*Device, error) {
		if defaultDevice == nil {
			return nil, errors.New("default audio source is unavailable")
		}
		return defaultDevice, nil
	}

	selectPrimary := func() (*Device, error) {
		if input == "" || input == "default" {
			return chooseDefault()
		}
		if byInput != nil {
			return byInput, nil
		}
		return nil, fmt.Errorf("audio.input %q did not match any device", input)
	}

	primary, err := selectPrimary()
	if err != nil {
		return Selection{}, err
	}
	if primary.Available && !primary.Muted {
		return Selection{Device: *primary}, nil
	}

	primaryReason := "unavailable"
	if primary.Muted {
		primaryReason = "muted"
	}

	fallbackDevice := primary
	if fallback != "" && fallback != "default" {
		if byFallback == nil {
			return Selection{}, fmt.Errorf("primary input %q is %s and fallback %q not found", primary.ID, primaryReason, fallback)
		}
		fallbackDevice = byFallback
	} else {
		d, derr := chooseDefault()
		if derr != nil {
			return Selection{}, fmt.Errorf("primary input %q is %s and no usable fallback: %w", primary.ID, primaryReason, derr)
		}
		fallbackDevice = d
	}

	if !fallbackDevice.Available {
		return Selection{}, fmt.Errorf("audio fallback device %q is not available", fallbackDevice.ID)
	}
	if fallbackDevice.Muted {
		return Selection{}, fmt.Errorf("audio fallback device %q is muted", fallbackDevice.ID)
	}

	return Selection{
		Device:   *fallbackDevice,
		Warning:  fmt.Sprintf("audio.input %q is %s; falling back to %q", primary.ID, primaryReason, fallbackDevice.ID),
		Fallback: primary.ID != fallbackDevice.ID,
	}, nil
}

// deviceMatches reports whether a search term matches a device id or description.
func deviceMatches(device Device, term string) bool {
	if term == "" {
		return false
	}
	id := strings.ToLower(device.ID)
	desc := strings.ToLower(device.Description)
	return strings.Contains(id, term) || strings.Contains(desc, term)
}

// Capture streams fixed-size PCM chunks from one selected Pulse source.
type Capture struct {
	device Device

	client *pulse.Client
	stream *pulse.RecordStream

	chunks chan []byte
	stopCh chan struct{}

	mu      sync.Mutex
	pending []byte
	rawPCM  []byte
	stopped bool

	inflight sync.WaitGroup
	bytes    atomic.Int64
}

// StartCapture creates and starts a 16kHz mono s16 record stream.
func StartCapture(ctx context.Context, selected Device) (*Capture, error) {
	client, err := pulse.NewClient(
		pulse.ClientApplicationName("sotto"),
		pulse.ClientApplicationIconName("audio-input-microphone"),
	)
	if err != nil {
		return nil, fmt.Errorf("connect pulse server: %w", err)
	}

	source, err := client.SourceByID(selected.ID)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("resolve source %q: %w", selected.ID, err)
	}

	capture := &Capture{
		device: selected,
		client: client,
		chunks: make(chan []byte, 128),
		stopCh: make(chan struct{}),
	}

	writer := pulse.NewWriter(writerFunc(capture.onPCM), pulseproto.FormatInt16LE)
	stream, err := client.NewRecord(
		writer,
		pulse.RecordSource(source),
		pulse.RecordMono,
		pulse.RecordSampleRate(16000),
		pulse.RecordBufferFragmentSize(chunkSizeBytes),
		pulse.RecordMediaName("sotto dictation"),
	)
	if err != nil {
		capture.Close()
		return nil, fmt.Errorf("create pulse record stream: %w", err)
	}

	capture.stream = stream
	stream.Start()

	go func() {
		<-ctx.Done()
		_ = capture.Stop()
	}()

	return capture, nil
}

// Device returns capture metadata for logging and diagnostics.
func (c *Capture) Device() Device {
	return c.device
}

// Chunks returns the PCM stream as fixed-size byte slices.
func (c *Capture) Chunks() <-chan []byte {
	return c.chunks
}

// BytesCaptured reports total bytes accepted from Pulse.
func (c *Capture) BytesCaptured() int64 {
	return c.bytes.Load()
}

// RawPCM returns a snapshot of all captured raw PCM bytes.
func (c *Capture) RawPCM() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]byte, len(c.rawPCM))
	copy(out, c.rawPCM)
	return out
}

// Stop halts the stream, flushes residual PCM, and closes Chunks exactly once.
func (c *Capture) Stop() error {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return nil
	}
	c.stopped = true
	close(c.stopCh)
	c.mu.Unlock()

	if c.stream != nil {
		c.stream.Stop()
		c.stream.Close()
	}
	if c.client != nil {
		c.client.Close()
	}

	c.inflight.Wait()

	c.mu.Lock()
	pending := append([]byte(nil), c.pending...)
	c.pending = nil
	c.mu.Unlock()

	if len(pending) > 0 {
		chunk := make([]byte, len(pending))
		copy(chunk, pending)
		select {
		case c.chunks <- chunk:
		default:
		}
	}

	close(c.chunks)
	return nil
}

// Close is a convenience alias for Stop.
func (c *Capture) Close() {
	_ = c.Stop()
}

// onPCM receives raw Pulse frames and emits chunkSizeBytes slices to c.chunks.
func (c *Capture) onPCM(buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, nil
	}

	select {
	case <-c.stopCh:
		return 0, io.EOF
	default:
	}

	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return 0, io.EOF
	}
	// Guard Add under the same mutex as c.stopped to avoid Add/Wait races.
	c.inflight.Add(1)

	c.rawPCM = append(c.rawPCM, buffer...)
	c.pending = append(c.pending, buffer...)

	chunks := make([][]byte, 0, len(c.pending)/chunkSizeBytes)
	for len(c.pending) >= chunkSizeBytes {
		chunk := make([]byte, chunkSizeBytes)
		copy(chunk, c.pending[:chunkSizeBytes])
		c.pending = c.pending[chunkSizeBytes:]
		chunks = append(chunks, chunk)
	}
	c.mu.Unlock()
	defer c.inflight.Done()

	c.bytes.Add(int64(len(buffer)))

	for _, chunk := range chunks {
		select {
		case <-c.stopCh:
			return 0, io.EOF
		case c.chunks <- chunk:
		}
	}

	return len(buffer), nil
}

// writerFunc adapts a function to io.Writer for pulse.NewWriter.
type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(b []byte) (int, error) {
	return f(b)
}

// sourceStateString maps Pulse source state constants to human-readable values.
func sourceStateString(state uint32) string {
	switch state {
	case 0:
		return "running"
	case 1:
		return "idle"
	case 2:
		return "suspended"
	default:
		return fmt.Sprintf("unknown(%d)", state)
	}
}

// sourceAvailable maps Pulse source port availability to a simple boolean.
func sourceAvailable(source *pulseproto.GetSourceInfoReply) bool {
	if source == nil {
		return false
	}
	if len(source.Ports) == 0 {
		return true
	}
	for _, port := range source.Ports {
		if port.Name != source.ActivePortName {
			continue
		}
		// PulseAudio values: unknown=0, no=1, yes=2.
		return port.Available == 0 || port.Available == 2
	}
	return true
}
