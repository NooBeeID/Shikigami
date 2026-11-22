package grimoire

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
)

// Trigger menjalankan build grimoire single-flight per project dengan
// context akar sendiri (tidak terikat context daemon atau sesi).
type Trigger struct {
	b        *Builder
	publish  func(string, any)
	mu       sync.Mutex
	building map[string]struct{}
	root     context.Context
	cancel   context.CancelFunc
}

// NewTrigger membuat Trigger. publish boleh nil.
func NewTrigger(b *Builder, publish func(event string, payload any)) *Trigger {
	root, cancel := context.WithCancel(context.Background())
	return &Trigger{
		b:        b,
		publish:  publish,
		building: map[string]struct{}{},
		root:     root,
		cancel:   cancel,
	}
}

func triggerLog(t *Trigger) *slog.Logger {
	if t != nil && t.b != nil && t.b.logger != nil {
		return t.b.logger
	}
	return slog.Default()
}

// emit menerbitkan grimoire.changed; panik publish tidak menjatuhkan goroutine.
func (t *Trigger) emit(projectID string) {
	if t.publish == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			triggerLog(t).Error("grimoire: publish panik", "project", projectID, "panic", r)
		}
	}()
	t.publish("grimoire.changed", map[string]any{"projectId": projectID})
}

func (t *Trigger) acquire(projectID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.root.Err() != nil {
		return false
	}
	if _, ok := t.building[projectID]; ok {
		return false
	}
	t.building[projectID] = struct{}{}
	return true
}

func (t *Trigger) release(projectID string) {
	t.mu.Lock()
	delete(t.building, projectID)
	t.mu.Unlock()
}

// Building melaporkan apakah project sedang dibangun.
func (t *Trigger) Building(projectID string) bool {
	if t == nil {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	_, ok := t.building[projectID]
	return ok
}

// execute dipanggil hanya setelah acquire sukses dan emit awal terbit.
func (t *Trigger) execute(ctx context.Context, projectID, projectPath string, full bool) (res Result, err error) {
	// Defer berjalan terbalik: release lebih dulu, lalu emit akhir.
	defer t.emit(projectID)
	defer t.release(projectID)
	defer func() {
		if r := recover(); r != nil {
			triggerLog(t).Error("grimoire: panic build", "project", projectID, "panic", r)
			res = Result{}
			err = fmt.Errorf("panic build grimoire: %v", r)
		}
	}()

	if projectPath == "" {
		return Result{}, errors.New("projectPath wajib")
	}
	if !full {
		d, cerr := t.b.Check(projectID, projectPath)
		if cerr != nil {
			return Result{}, cerr
		}
		if d.Empty() {
			return Result{}, nil
		}
	}
	return t.b.BuildIncremental(ctx, projectID, IncrementalOptions{ProjectPath: projectPath, Full: full})
}

// Kick memicu build inkremental di latar belakang. Non-blocking, tanpa galat.
func (t *Trigger) Kick(projectID, projectPath string) {
	if t == nil || t.b == nil {
		return
	}
	st := t.b.Store()
	if st == nil || st.ValidateProject(projectID) != nil || !st.Exists(projectID) {
		return
	}
	if !t.acquire(projectID) {
		return
	}
	t.emit(projectID)
	go func() {
		if _, err := t.execute(t.root, projectID, projectPath, false); err != nil && !errors.Is(err, context.Canceled) {
			triggerLog(t).Warn("grimoire: build latar belakang gagal", "project", projectID, "err", err)
		}
	}()
}

// Run membangun secara sinkron. started=false bila project sedang dibangun.
func (t *Trigger) Run(ctx context.Context, projectID, projectPath string, full bool) (Result, bool, error) {
	if t == nil || t.b == nil {
		return Result{}, false, errors.New("trigger belum siap")
	}
	if err := t.b.Store().ValidateProject(projectID); err != nil {
		return Result{}, false, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if !t.acquire(projectID) {
		return Result{}, false, nil
	}
	rctx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(t.root, cancel)
	defer stop()
	defer cancel()

	t.emit(projectID)
	res, err := t.execute(rctx, projectID, projectPath, full)
	return res, true, err
}

// Close membatalkan semua build yang berjalan. Idempoten.
func (t *Trigger) Close() {
	if t != nil {
		t.cancel()
	}
}

// RefreshDeps adalah pasangan Builder dan Trigger yang aktif.
type RefreshDeps struct {
	Builder *Builder
	Trigger *Trigger
}

var refreshActive atomic.Pointer[RefreshDeps]

// RegisterRefresh menyimpan pasangan aktif; panggilan ulang menimpa.
func RegisterRefresh(d RefreshDeps) { refreshActive.Store(&d) }

func refreshCurrent() (RefreshDeps, bool) {
	p := refreshActive.Load()
	if p == nil || p.Builder == nil || p.Trigger == nil {
		return RefreshDeps{}, false
	}
	return *p, true
}

// RefreshActive melaporkan apakah refresh sudah terdaftar.
func RefreshActive() bool {
	_, ok := refreshCurrent()
	return ok
}

// RefreshRun menjalankan build sinkron lewat Trigger aktif.
func RefreshRun(ctx context.Context, projectID, projectPath string, full bool) (Result, bool, error) {
	d, ok := refreshCurrent()
	if !ok {
		return Result{}, false, errors.New("refresh grimoire belum terdaftar")
	}
	return d.Trigger.Run(ctx, projectID, projectPath, full)
}

// RefreshBuilding melaporkan apakah project sedang dibangun.
func RefreshBuilding(projectID string) bool {
	d, ok := refreshCurrent()
	if !ok {
		return false
	}
	return d.Trigger.Building(projectID)
}

// RefreshStaleCount menghitung bab basi; 0 bila belum aktif atau belum dibangun.
func RefreshStaleCount(projectID string) int {
	d, ok := refreshCurrent()
	if !ok {
		return 0
	}
	st, err := d.Builder.Store().ReadState(projectID)
	if err != nil {
		return 0
	}
	return StaleCount(st)
}

// RefreshApplyStale menandai bab basi pada books; apa adanya bila tak bisa.
func RefreshApplyStale(projectID string, books []Book) []Book {
	d, ok := refreshCurrent()
	if !ok {
		return books
	}
	st, err := d.Builder.Store().ReadState(projectID)
	if err != nil {
		return books
	}
	return ApplyStale(books, st)
}
