package main

import (
	"context"
	"errors"

	"github.com/NooBeeID/shikigami/internal/appconfig"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	winDefaultW   = 1200
	winDefaultH   = 800
	winMinW       = 800
	winMinH       = 500
	winMinVisible = 100
)

func init() {
	registerStartupHook(func(ctx context.Context, b *Bridge) { restoreWindow(ctx, b) })
}

func winScreenSize(s runtime.Screen) (int, int) {
	w, h := s.Size.Width, s.Size.Height
	if w <= 0 || h <= 0 {
		w, h = s.Width, s.Height
	}
	return w, h
}

// windowVisible memeriksa apakah jendela masih beririsan dengan area layar.
// Ini aproksimasi konservatif karena Wails v2 tidak menyediakan origin per
// layar: layar lain dianggap berada di kiri/atas atau kanan/bawah primary,
// sehingga area kandidat adalah persegi yang diperluas oleh total layar.
func windowVisible(w appconfig.Window, screens []runtime.Screen) bool {
	if len(screens) == 0 {
		return true
	}
	primary := screens[0]
	for _, s := range screens {
		if s.IsPrimary {
			primary = s
			break
		}
	}
	pw, ph := winScreenSize(primary)
	totalW, maxH := 0, 0
	for _, s := range screens {
		sw, sh := winScreenSize(s)
		totalW += sw
		if sh > maxH {
			maxH = sh
		}
	}
	if pw <= 0 || ph <= 0 {
		return true
	}
	left, right := -(totalW - pw), totalW
	top, bottom := -(maxH - ph), maxH
	ix := min(w.X+w.W, right) - max(w.X, left)
	iy := min(w.Y+w.H, bottom) - max(w.Y, top)
	return ix >= winMinVisible && iy >= winMinVisible
}

func applyDefaultWindow(ctx context.Context) {
	runtime.WindowSetSize(ctx, winDefaultW, winDefaultH)
	runtime.WindowCenter(ctx)
}

func restoreWindow(ctx context.Context, b *Bridge) {
	cfg, err := b.AppConfigGet()
	if err != nil || cfg.Window == nil || cfg.Window.W <= 0 || cfg.Window.H <= 0 {
		applyDefaultWindow(ctx)
		return
	}
	win := *cfg.Window
	win.W = max(win.W, winMinW)
	win.H = max(win.H, winMinH)
	screens, _ := runtime.ScreenGetAll(ctx)
	if !windowVisible(win, screens) {
		applyDefaultWindow(ctx)
		return
	}
	runtime.WindowSetSize(ctx, win.W, win.H)
	runtime.WindowSetPosition(ctx, win.X, win.Y)
}

func saveWindow(ctx context.Context, b *Bridge) {
	if ctx == nil {
		return
	}
	if runtime.WindowIsMaximised(ctx) || runtime.WindowIsFullscreen(ctx) || runtime.WindowIsMinimised(ctx) {
		return
	}
	x, y := runtime.WindowGetPosition(ctx)
	w, h := runtime.WindowGetSize(ctx)
	if w <= 0 || h <= 0 {
		return
	}
	w = max(w, winMinW)
	h = max(h, winMinH)
	_, _ = b.AppConfigSet(map[string]any{"window": map[string]any{"x": x, "y": y, "w": w, "h": h}})
}

// WindowSaveState ter-bind ke frontend; debounce dilakukan di frontend.
func (b *Bridge) WindowSaveState() error {
	b.ctxMu.RLock()
	ctx := b.ctx
	b.ctxMu.RUnlock()
	if ctx == nil {
		return errors.New("aplikasi belum siap")
	}
	saveWindow(ctx, b)
	return nil
}

func onBeforeCloseWindow(ctx context.Context, b *Bridge) bool {
	saveWindow(ctx, b)
	return false
}
