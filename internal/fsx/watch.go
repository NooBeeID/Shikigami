package fsx

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const debounceDelay = 100 * time.Millisecond

// ChangeEvent menggambarkan perubahan pada file terpantau.
type ChangeEvent struct {
	Op    string `json:"op"` // write|remove|rename|create
	Mtime int64  `json:"mtime"`
}

type watchEntry struct {
	root, rel string
	count     int
	timer     *time.Timer
	pending   string
}

// Watcher memantau file yang sedang tampil lewat folder induknya.
type Watcher struct {
	mu       sync.Mutex
	fw       *fsnotify.Watcher
	onChange func(root, relPath string, ev ChangeEvent)
	files    map[string]*watchEntry
	dirs     map[string]int
	closed   bool
	done     chan struct{}
}

// NewWatcher membuat Watcher dengan satu goroutine pembaca event.
func NewWatcher(onChange func(root, relPath string, ev ChangeEvent)) (*Watcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Watcher{
		fw:       fw,
		onChange: onChange,
		files:    map[string]*watchEntry{},
		dirs:     map[string]int{},
		done:     make(chan struct{}),
	}
	go w.loop()
	return w, nil
}

func normRel(relPath string) string {
	r := filepath.ToSlash(filepath.Clean(filepath.FromSlash(relPath)))
	return r
}

// Watch mulai memantau satu file (refcount).
func (w *Watcher) Watch(root, relPath string) error {
	abs, err := ResolveUnder(root, relPath)
	if err != nil {
		return err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return Wrap(ErrNotFound, "%s", relPath)
		}
		return Wrap(ErrInvalid, "gagal membaca %s: %v", relPath, err)
	}
	if !fi.Mode().IsRegular() {
		return Wrap(ErrInvalid, "bukan file biasa: %s", relPath)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return Wrap(ErrInvalid, "watcher ditutup")
	}
	if e, ok := w.files[abs]; ok {
		e.count++
		return nil
	}
	dir := filepath.Dir(abs)
	if w.dirs[dir] == 0 {
		if err := w.fw.Add(dir); err != nil {
			return Wrap(ErrInvalid, "gagal memantau: %v", err)
		}
	}
	w.dirs[dir]++
	w.files[abs] = &watchEntry{root: root, rel: normRel(relPath), count: 1}
	return nil
}

// Unwatch mengurangi refcount; idempoten.
func (w *Watcher) Unwatch(root, relPath string) error {
	abs, err := ResolveUnder(root, relPath)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	e, ok := w.files[abs]
	if !ok {
		return nil
	}
	e.count--
	if e.count > 0 {
		return nil
	}
	if e.timer != nil {
		e.timer.Stop()
	}
	delete(w.files, abs)
	dir := filepath.Dir(abs)
	w.dirs[dir]--
	if w.dirs[dir] <= 0 {
		delete(w.dirs, dir)
		if err := w.fw.Remove(dir); err != nil && !errors.Is(err, fsnotify.ErrNonExistentWatch) {
			// folder bisa sudah hilang; abaikan
			_ = err
		}
	}
	return nil
}

func (w *Watcher) loop() {
	defer close(w.done)
	for {
		select {
		case ev, ok := <-w.fw.Events:
			if !ok {
				return
			}
			w.handle(ev)
		case _, ok := <-w.fw.Errors:
			if !ok {
				return
			}
		}
	}
}

func (w *Watcher) handle(ev fsnotify.Event) {
	abs := filepath.Clean(ev.Name)
	var op string
	switch {
	case ev.Op&fsnotify.Write != 0:
		op = "write"
	case ev.Op&fsnotify.Create != 0:
		op = "create"
	case ev.Op&fsnotify.Remove != 0:
		op = "remove"
	case ev.Op&fsnotify.Rename != 0:
		op = "rename"
	}
	if op == "" {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	e := w.files[abs]
	if e == nil {
		return
	}
	if !(op == "create" && e.pending == "write") {
		e.pending = op
	}
	if e.timer == nil {
		e.timer = time.AfterFunc(debounceDelay, func() { w.fire(abs) })
	} else {
		e.timer.Reset(debounceDelay)
	}
}

func (w *Watcher) fire(abs string) {
	w.mu.Lock()
	e := w.files[abs]
	if e == nil || w.closed {
		w.mu.Unlock()
		return
	}
	op := e.pending
	e.pending = ""
	e.timer = nil
	root, rel := e.root, e.rel
	w.mu.Unlock()

	var mtime int64
	fi, err := os.Stat(abs)
	if err != nil {
		mtime = 0
		if op != "remove" && op != "rename" {
			op = "remove"
		}
	} else {
		mtime = fi.ModTime().UnixNano()
		if op == "remove" || op == "rename" {
			op = "write"
		}
	}
	w.onChange(root, rel, ChangeEvent{Op: op, Mtime: mtime})
}

// Close menghentikan goroutine dan semua timer.
func (w *Watcher) Close() error {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return nil
	}
	w.closed = true
	for _, e := range w.files {
		if e.timer != nil {
			e.timer.Stop()
		}
	}
	w.files = map[string]*watchEntry{}
	w.dirs = map[string]int{}
	w.mu.Unlock()
	err := w.fw.Close()
	<-w.done
	return err
}
