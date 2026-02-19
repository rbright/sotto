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

type HyprNotify struct {
	cfg    config.IndicatorConfig
	logger *slog.Logger

	mu                    sync.Mutex
	focusedMonitor        string
	desktopNotificationID uint32
	soundMu               sync.Mutex
}

func NewHyprNotify(cfg config.IndicatorConfig, logger *slog.Logger) *HyprNotify {
	return &HyprNotify{cfg: cfg, logger: logger}
}

func (h *HyprNotify) ShowRecording(ctx context.Context) {
	h.playCue(cueStart)
	if !h.cfg.Enable {
		return
	}
	h.ensureFocusedMonitor(ctx)
	h.run(ctx, func(ctx context.Context) error {
		return h.notify(ctx, 1, 300000, "rgb(89b4fa)", h.cfg.TextRecording)
	})
}

func (h *HyprNotify) ShowTranscribing(ctx context.Context) {
	if !h.cfg.Enable {
		return
	}
	h.run(ctx, func(ctx context.Context) error {
		return h.notify(ctx, 1, 300000, "rgb(cba6f7)", h.cfg.TextProcessing)
	})
}

func (h *HyprNotify) ShowError(ctx context.Context, text string) {
	if !h.cfg.Enable {
		return
	}
	if text == "" {
		text = h.cfg.TextError
	}
	timeout := h.cfg.ErrorTimeoutMS
	if timeout <= 0 {
		timeout = 1200
	}
	h.run(ctx, func(ctx context.Context) error {
		return h.notify(ctx, 3, timeout, "rgb(f38ba8)", text)
	})
}

func (h *HyprNotify) CueStop(context.Context) {
	h.playCue(cueStop)
}

func (h *HyprNotify) CueComplete(context.Context) {
	h.playCue(cueComplete)
}

func (h *HyprNotify) CueCancel(context.Context) {
	h.playCue(cueCancel)
}

func (h *HyprNotify) Hide(ctx context.Context) {
	if !h.cfg.Enable {
		return
	}
	h.run(ctx, h.dismiss)
}

func (h *HyprNotify) FocusedMonitor() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.focusedMonitor
}

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

func (h *HyprNotify) notify(ctx context.Context, icon int, timeoutMS int, color string, text string) error {
	if strings.EqualFold(strings.TrimSpace(h.cfg.Backend), "desktop") {
		return h.notifyDesktop(ctx, timeoutMS, text)
	}
	return hypr.Notify(ctx, icon, timeoutMS, color, text)
}

func (h *HyprNotify) dismiss(ctx context.Context) error {
	if strings.EqualFold(strings.TrimSpace(h.cfg.Backend), "desktop") {
		return h.dismissDesktop(ctx)
	}
	return hypr.DismissNotify(ctx)
}

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

func (h *HyprNotify) run(ctx context.Context, fn func(context.Context) error) {
	runCtx, cancel := context.WithTimeout(ctx, 400*time.Millisecond)
	defer cancel()
	if err := fn(runCtx); err != nil {
		h.log("indicator dispatch failed", err)
	}
}

func (h *HyprNotify) playCue(kind cueKind) {
	if !h.cfg.SoundEnable {
		return
	}
	go func() {
		h.soundMu.Lock()
		defer h.soundMu.Unlock()
		if err := emitCue(kind, h.cfg); err != nil {
			h.log("indicator audio cue failed", err)
		}
	}()
}

func (h *HyprNotify) log(message string, err error) {
	if h.logger == nil || err == nil {
		return
	}
	h.logger.Debug(message, "error", err.Error())
}
