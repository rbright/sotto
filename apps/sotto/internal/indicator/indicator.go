// Package indicator handles visual state notifications and audio cue playback.
package indicator

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/rbright/sotto/internal/config"
	"github.com/rbright/sotto/internal/hypr"
)

// Controller is the session-facing indicator contract.
type Controller interface {
	ShowRecording(context.Context)
	ShowTranscribing(context.Context)
	ShowError(context.Context, string)
	CueStop(context.Context)
	CueComplete(context.Context)
	CueCancel(context.Context)
	Hide(context.Context)
	FocusedMonitor() string
}

// HyprNotify is the concrete indicator implementation used by runtime sessions.
// It can route notifications via Hyprland or desktop DBus based on config backend.
type HyprNotify struct {
	cfg      config.IndicatorConfig
	logger   *slog.Logger
	messages messages

	mu                    sync.Mutex
	focusedMonitor        string
	desktopNotificationID uint32
	soundMu               sync.Mutex
}

// NewHyprNotify creates an indicator controller from config.
func NewHyprNotify(cfg config.IndicatorConfig, logger *slog.Logger) *HyprNotify {
	return &HyprNotify{
		cfg:      cfg,
		logger:   logger,
		messages: indicatorMessagesFromEnv(),
	}
}

// ShowRecording signals recording start and emits the start cue.
func (h *HyprNotify) ShowRecording(ctx context.Context) {
	h.playCue(cueStart)
	if !h.cfg.Enable {
		return
	}
	h.ensureFocusedMonitor(ctx)
	h.run(ctx, func(ctx context.Context) error {
		return h.notify(ctx, 1, 300000, "rgb(89b4fa)", h.messages.recording)
	})
}

// ShowTranscribing signals the post-capture transcription state.
func (h *HyprNotify) ShowTranscribing(ctx context.Context) {
	if !h.cfg.Enable {
		return
	}
	h.run(ctx, func(ctx context.Context) error {
		return h.notify(ctx, 1, 300000, "rgb(cba6f7)", h.messages.processing)
	})
}

// ShowError displays an error-state indicator message.
func (h *HyprNotify) ShowError(ctx context.Context, text string) {
	if !h.cfg.Enable {
		return
	}
	if text == "" {
		text = h.messages.errorText
	}
	timeout := h.cfg.ErrorTimeoutMS
	if timeout <= 0 {
		timeout = 1200
	}
	h.run(ctx, func(ctx context.Context) error {
		return h.notify(ctx, 3, timeout, "rgb(f38ba8)", text)
	})
}

// CueStop emits the stop cue.
func (h *HyprNotify) CueStop(context.Context) {
	h.playCue(cueStop)
}

// CueComplete emits the successful-commit cue.
func (h *HyprNotify) CueComplete(context.Context) {
	h.playCue(cueComplete)
}

// CueCancel emits the cancel cue.
func (h *HyprNotify) CueCancel(context.Context) {
	h.playCue(cueCancel)
}

// Hide dismisses the active indicator surface.
func (h *HyprNotify) Hide(ctx context.Context) {
	if !h.cfg.Enable {
		return
	}
	h.run(ctx, h.dismiss)
}

// FocusedMonitor returns the monitor captured when recording began.
func (h *HyprNotify) FocusedMonitor() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.focusedMonitor
}

// ensureFocusedMonitor resolves and caches the focused monitor once per session.
func (h *HyprNotify) ensureFocusedMonitor(ctx context.Context) {
	h.mu.Lock()
	alreadySet := h.focusedMonitor != ""
	h.mu.Unlock()
	if alreadySet {
		return
	}

	monitor, err := hypr.QueryFocusedMonitor(ctx)
	if err != nil {
		h.log("indicator focused monitor query failed", err)
		return
	}

	h.mu.Lock()
	h.focusedMonitor = monitor
	h.mu.Unlock()
}

// notify dispatches indicator output through the configured backend.
func (h *HyprNotify) notify(ctx context.Context, icon int, timeoutMS int, color string, text string) error {
	if strings.EqualFold(strings.TrimSpace(h.cfg.Backend), "desktop") {
		return h.notifyDesktop(ctx, timeoutMS, text)
	}
	return hypr.Notify(ctx, icon, timeoutMS, color, text)
}

// dismiss removes indicator output from the configured backend.
func (h *HyprNotify) dismiss(ctx context.Context) error {
	if strings.EqualFold(strings.TrimSpace(h.cfg.Backend), "desktop") {
		return h.dismissDesktop(ctx)
	}
	return hypr.DismissNotify(ctx)
}

// notifyDesktop sends a replaceable desktop notification and stores its ID.
func (h *HyprNotify) notifyDesktop(ctx context.Context, timeoutMS int, text string) error {
	h.mu.Lock()
	replaceID := h.desktopNotificationID
	h.mu.Unlock()

	appName := strings.TrimSpace(h.cfg.DesktopAppName)
	if appName == "" {
		appName = "sotto-indicator"
	}

	id, err := desktopNotify(ctx, appName, replaceID, text, timeoutMS)
	if err != nil {
		return err
	}

	h.mu.Lock()
	h.desktopNotificationID = id
	h.mu.Unlock()
	return nil
}

// dismissDesktop closes the current desktop notification ID when present.
func (h *HyprNotify) dismissDesktop(ctx context.Context) error {
	h.mu.Lock()
	id := h.desktopNotificationID
	h.desktopNotificationID = 0
	h.mu.Unlock()

	if id == 0 {
		return nil
	}
	return desktopDismiss(ctx, id)
}

// run executes an indicator operation with a bounded timeout.
func (h *HyprNotify) run(ctx context.Context, fn func(context.Context) error) {
	runCtx, cancel := context.WithTimeout(ctx, 400*time.Millisecond)
	defer cancel()
	if err := fn(runCtx); err != nil {
		h.log("indicator dispatch failed", err)
	}
}

// playCue serializes cue playback and emits audio asynchronously.
func (h *HyprNotify) playCue(kind cueKind) {
	if !h.cfg.SoundEnable {
		return
	}
	go func() {
		h.soundMu.Lock()
		defer h.soundMu.Unlock()
		if err := emitCue(kind); err != nil {
			h.log("indicator audio cue failed", err)
		}
	}()
}

// log emits debug-only indicator failures to the runtime logger.
func (h *HyprNotify) log(message string, err error) {
	if h.logger == nil || err == nil {
		return
	}
	h.logger.Debug(message, "error", err.Error())
}
