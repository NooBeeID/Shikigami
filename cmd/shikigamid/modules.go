package main

import (
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/NooBeeID/shikigami/internal/ipc"
)

var (
	modulesMu sync.Mutex
	modules   = map[string]func(srv *ipc.Server, d *Deps){}
)

// Deps adalah dependensi bersama yang diberikan ke setiap modul daemon.
// Selalu dipakai lewat pointer; nilai nol aman dipakai.
type Deps struct {
	DataDir    string
	SocketPath string
	Logger     *slog.Logger
	Version    string
	Started    time.Time

	services map[string]any
	ready    []func()
	mu       sync.Mutex
}

// Provide menyediakan layanan bernama untuk modul lain.
func (d *Deps) Provide(name string, v any) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.services == nil {
		d.services = map[string]any{}
	}
	if _, ok := d.services[name]; ok {
		panic(fmt.Sprintf("layanan %q sudah disediakan", name))
	}
	d.services[name] = v
}

// Lookup mengambil layanan bernama.
func (d *Deps) Lookup(name string) (any, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	v, ok := d.services[name]
	return v, ok
}

// OnReady mendaftarkan callback yang jalan setelah semua modul ter-wire.
func (d *Deps) OnReady(fn func()) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.ready = append(d.ready, fn)
}

// RegisterModule mendaftarkan modul daemon; dipanggil dari init().
func RegisterModule(name string, wire func(srv *ipc.Server, d *Deps)) {
	if name == "" {
		panic("nama modul kosong")
	}
	if wire == nil {
		panic(fmt.Sprintf("modul %q tidak punya fungsi wire", name))
	}
	modulesMu.Lock()
	defer modulesMu.Unlock()
	if _, ok := modules[name]; ok {
		panic(fmt.Sprintf("modul %q sudah terdaftar", name))
	}
	modules[name] = wire
}

func moduleNames() []string {
	modulesMu.Lock()
	defer modulesMu.Unlock()
	names := make([]string, 0, len(modules))
	for n := range modules {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// wireAll menyambungkan semua modul. Urutan antar modul hanya alfabetis;
// modul yang butuh layanan modul lain memakai d.Lookup di dalam d.OnReady
// atau secara malas saat handler dipanggil.
func wireAll(srv *ipc.Server, d *Deps) {
	for _, name := range moduleNames() {
		d.Logger.Info("wire modul", "module", name)
		modulesMu.Lock()
		wire := modules[name]
		modulesMu.Unlock()
		wire(srv, d)
	}
	done := 0
	for {
		d.mu.Lock()
		if done >= len(d.ready) {
			d.mu.Unlock()
			return
		}
		fn := d.ready[done]
		done++
		d.mu.Unlock()
		fn()
	}
}
