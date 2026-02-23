package audio

import (
	"context"
	"io"
	"reflect"
	"testing"

	pulseproto "github.com/jfreymuth/pulse/proto"
	"github.com/stretchr/testify/require"
)

func TestSelectDeviceFromListPrimaryDefault(t *testing.T) {
	devices := []Device{
		{ID: "elgato", Description: "Elgato Wave 3 Mono", Available: true, Default: true},
		{ID: "sony", Description: "Sony WH-1000XM6", Available: true},
	}

	selection, err := selectDeviceFromList(devices, "default", "default")
	require.NoError(t, err)
	require.Equal(t, "elgato", selection.Device.ID)
	require.Empty(t, selection.Warning)
}

func TestSelectDeviceFromListMutedPrimaryUsesFallback(t *testing.T) {
	devices := []Device{
		{ID: "elgato", Description: "Elgato Wave 3 Mono", Available: true, Muted: true, Default: true},
		{ID: "sony", Description: "Sony WH-1000XM6", Available: true},
	}

	selection, err := selectDeviceFromList(devices, "elgato", "sony")
	require.NoError(t, err)
	require.Equal(t, "sony", selection.Device.ID)
	require.Contains(t, selection.Warning, "muted")
	require.True(t, selection.Fallback)
}

func TestSelectDeviceFromListFailsWhenSelectedAndFallbackMuted(t *testing.T) {
	devices := []Device{
		{ID: "elgato", Description: "Elgato Wave 3 Mono", Available: true, Muted: true, Default: true},
	}

	_, err := selectDeviceFromList(devices, "default", "default")
	require.Error(t, err)
	require.Contains(t, err.Error(), "muted")
}

func TestSelectDeviceFromListUnknownInput(t *testing.T) {
	devices := []Device{{ID: "elgato", Description: "Elgato Wave 3 Mono", Available: true, Default: true}}

	_, err := selectDeviceFromList(devices, "missing", "default")
	require.Error(t, err)
	require.Contains(t, err.Error(), "did not match")
}

func TestDeviceMatchesByIDAndDescription(t *testing.T) {
	dev := Device{ID: "alsa_input.usb-elgato", Description: "Elgato Wave 3 Mono"}
	require.True(t, deviceMatches(dev, "elgato"))
	require.True(t, deviceMatches(dev, "wave 3"))
	require.False(t, deviceMatches(dev, "missing"))
}

func TestListDevicesFailsWhenPulseUnavailable(t *testing.T) {
	t.Setenv("PULSE_SERVER", "unix:/tmp/definitely-missing-pulse-server")
	_, err := ListDevices(context.Background())
	require.Error(t, err)
}

func TestSelectDeviceFailsWhenPulseUnavailable(t *testing.T) {
	t.Setenv("PULSE_SERVER", "unix:/tmp/definitely-missing-pulse-server")
	_, err := SelectDevice(context.Background(), "default", "default")
	require.Error(t, err)
}

func TestSourceStateString(t *testing.T) {
	require.Equal(t, "running", sourceStateString(0))
	require.Equal(t, "idle", sourceStateString(1))
	require.Equal(t, "suspended", sourceStateString(2))
	require.Equal(t, "unknown(99)", sourceStateString(99))
}

func TestSourceAvailable(t *testing.T) {
	require.False(t, sourceAvailable(nil))
	require.True(t, sourceAvailable(&pulseproto.GetSourceInfoReply{})) // no ports => available

	available := &pulseproto.GetSourceInfoReply{ActivePortName: "mic"}
	setSourcePorts(t, available, []sourcePort{{name: "mic", available: 2}})
	require.True(t, sourceAvailable(available))

	notAvailable := &pulseproto.GetSourceInfoReply{ActivePortName: "mic"}
	setSourcePorts(t, notAvailable, []sourcePort{{name: "mic", available: 1}})
	require.False(t, sourceAvailable(notAvailable))
}

func TestWriterFuncDelegatesWrite(t *testing.T) {
	called := false
	writer := writerFunc(func(b []byte) (int, error) {
		called = true
		require.Equal(t, []byte{1, 2, 3}, b)
		return len(b), nil
	})

	n, err := writer.Write([]byte{1, 2, 3})
	require.NoError(t, err)
	require.Equal(t, 3, n)
	require.True(t, called)
}

func TestCaptureOnPCMChunkingAndStopFlushesPending(t *testing.T) {
	capture := &Capture{
		chunks: make(chan []byte, 8),
		stopCh: make(chan struct{}),
	}

	input := make([]byte, chunkSizeBytes+111)
	for i := range input {
		input[i] = byte(i % 255)
	}

	n, err := capture.onPCM(input)
	require.NoError(t, err)
	require.Equal(t, len(input), n)
	require.Equal(t, int64(len(input)), capture.BytesCaptured())
	require.Equal(t, len(input), len(capture.RawPCM()))

	firstChunk := <-capture.Chunks()
	require.Len(t, firstChunk, chunkSizeBytes)

	require.NoError(t, capture.Stop())

	remaining, ok := <-capture.Chunks()
	require.True(t, ok)
	require.Len(t, remaining, 111)

	_, ok = <-capture.Chunks()
	require.False(t, ok)
}

func TestCaptureOnPCMReturnsEOFWhenStopped(t *testing.T) {
	capture := &Capture{
		chunks: make(chan []byte, 1),
		stopCh: make(chan struct{}),
	}
	close(capture.stopCh)

	n, err := capture.onPCM([]byte{1, 2, 3})
	require.Equal(t, 0, n)
	require.ErrorIs(t, err, io.EOF)
	require.Equal(t, int64(0), capture.BytesCaptured())
}

func TestCaptureDeviceAndCloseAlias(t *testing.T) {
	capture := &Capture{
		device: Device{ID: "mic-1", Description: "Mic"},
		chunks: make(chan []byte, 1),
		stopCh: make(chan struct{}),
	}
	require.Equal(t, "mic-1", capture.Device().ID)

	capture.Close()
	_, ok := <-capture.Chunks()
	require.False(t, ok)
}

type sourcePort struct {
	name      string
	available uint32
}

func setSourcePorts(t *testing.T, reply *pulseproto.GetSourceInfoReply, ports []sourcePort) {
	t.Helper()

	sliceType := reflect.TypeOf(reply.Ports)
	sliceValue := reflect.MakeSlice(sliceType, len(ports), len(ports))

	for i, port := range ports {
		item := sliceValue.Index(i)
		item.FieldByName("Name").SetString(port.name)
		item.FieldByName("Available").SetUint(uint64(port.available))
	}

	replyValue := reflect.ValueOf(reply).Elem().FieldByName("Ports")
	replyValue.Set(sliceValue)
}
