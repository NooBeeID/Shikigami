package runs

import "sync"

// Notifier menyerialkan "ubah registry + ambil List + publish" supaya snapshot
// runs.changed tidak bisa tiba terbalik urutan saat hook (tiap hook = satu
// goroutine IPC) dan reconcile berjalan serentak. Satu mutex dipakai bersama
// oleh listener hook, listener exit, dan runs.detail. Membaca berkas Claude
// (I/O lambat) TIDAK dilakukan di dalam lock; hanya langkah singkat di
// memori: apply ke registry, List, lalu emit (emit tidak boleh memblokir).
type Notifier struct {
	mu   sync.Mutex
	reg  *Registry
	emit func(sessionID string, runs []Run)
}

func NewNotifier(reg *Registry, emit func(sessionID string, runs []Run)) *Notifier {
	return &Notifier{reg: reg, emit: emit}
}

// commit menjalankan fn di bawah lock; fn mengembalikan ID sesi yang berubah,
// lalu snapshot tiap sesi itu di-emit masih di bawah lock yang sama.
func (n *Notifier) commit(fn func() []string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	for _, sid := range fn() {
		if n.emit != nil {
			n.emit(sid, n.reg.List(map[string]bool{sid: true}))
		}
	}
}

// Apply meneruskan event hook ke registry dan mem-publish bila berubah.
func (n *Notifier) Apply(sessionID, event string, payload map[string]any) {
	n.commit(func() []string {
		if n.reg.Apply(sessionID, event, payload) {
			return []string{sessionID}
		}
		return nil
	})
}

// Forget membuang seluruh run sesi dan mem-publish bila ada yang terbuang.
func (n *Notifier) Forget(sessionID string) {
	n.commit(func() []string {
		if n.reg.Forget(sessionID) {
			return []string{sessionID}
		}
		return nil
	})
}

// Reconcile membaca berkas di luar lock, lalu menerapkan hasilnya dan
// mem-publish di dalam lock.
func (n *Notifier) Reconcile(eng sessionLookup, workspaceID string) {
	plan := scanReconcile(n.reg, eng, workspaceID)
	n.commit(func() []string { return plan.apply(n.reg) })
}
