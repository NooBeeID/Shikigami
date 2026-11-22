# Foundation: thread-ticket-05-thread-recovery

Dikerjakan SEBELUM fitur paralel. Semua perubahan aditif; baca file saat ini dulu dan jangan menimpa tambahan tiket 03/04 (mis. `Thread.CloseSummary`, field `Count`/`Items`). Kontrak lengkap: `overview.md`. Tanpa unit test; guard: `go build ./...` dan `cd frontend && npm run build`.

## Langkah 1: field `Participant` (Go)

Edit `internal/thread/types.go` (Edit tool, tambah di struct `Participant` setelah `Name`):

```go
	// Ditambah epic thread-ticket-05 (pemulihan).
	Dead               bool     `json:"dead,omitempty"`
	PreviousSessionIDs []string `json:"previousSessionIds,omitempty"`
```

Jangan mengubah field lain.

## Langkah 2: konstanta pemulihan

Buat `internal/thread/recovery_contract.go`:

```go
package thread

// Kontrak epic thread-ticket-05-thread-recovery. Teks pesan sistem adalah penanda yang
// dibaca frontend (awalan tetap); jangan diubah tanpa mengubah frontend/src/modules/thread/types.ts.
const (
	RecoveryDeadNotice     = "daemon dimulai ulang: agent mati, akan dihidupkan saat pesan berikutnya"
	RecoveryRespawnPrefix  = "agent dihidupkan ulang: "
	RecoveryFailedPrefix   = "gagal menghidupkan ulang agent: "

	RecoveryContextMaxMessages = 40
	RecoveryContextMaxRunes    = 12000
	RecoveryPreviousIDsMax     = 20
)
```

## Langkah 3: hook pra-kirim

Buat `internal/thread/presend.go`:

```go
package thread

// PreSendHook dipanggil Manager.Send dan Router.Route sebelum pesan dicatat/diteruskan.
// targetSessionIDs tidak memuat "user". Mengembalikan peta sessionId lama -> baru untuk target
// yang baru dihidupkan ulang. Galat hanya dicatat; tidak menggagalkan pengiriman.
type PreSendHook func(threadID string, targetSessionIDs []string) (replaced map[string]string, err error)

// SetPreSend memasang hook tunggal (menimpa yang lama; nil melepas).
func (m *Manager) SetPreSend(h PreSendHook) {
	m.hookMu.Lock()
	m.preSend = h
	m.hookMu.Unlock()
}

// runPreSend aman bila hook nil; panic dipulihkan. Hasil bisa nil (indeks peta nil aman).
func (m *Manager) runPreSend(threadID string, ids []string) map[string]string {
	m.hookMu.Lock()
	h := m.preSend
	m.hookMu.Unlock()
	if h == nil || len(ids) == 0 {
		return nil
	}
	var out map[string]string
	m.managerSafe("presend", func() {
		rep, err := h(threadID, ids)
		if err != nil {
			m.log.Warn("thread: pra-kirim gagal", "thread", threadID, "err", err)
		}
		out = rep
	})
	return out
}
```

Edit `internal/thread/manager.go`:

1. Di struct `Manager`, tambah field `preSend PreSendHook` (di blok hook, di bawah `onClose`).
2. Di `func (m *Manager) Send`, tepat setelah baris `master, hasMaster := managerMaster(t)`, sisipkan:

```go
	if hasMaster {
		if rep := m.runPreSend(threadID, []string{master.SessionID}); rep[master.SessionID] != "" {
			master.SessionID = rep[master.SessionID]
		}
	}
```

(Pengecekan `ErrClosed` sudah terjadi sebelum titik ini, jadi thread closed tidak pernah memicu hook.)

Edit `internal/thread/router.go`, di `Route`, tepat setelah blok `targets, logTo, err := r.routerResolveTargets(t, from, to)` beserta `if err != nil { return ... }`, sebelum `r.mgr.Post(...)`, sisipkan:

```go
	if len(targets) > 0 {
		ids := make([]string, 0, len(targets))
		for _, tg := range targets {
			ids = append(ids, tg.SessionID)
		}
		if rep := r.mgr.runPreSend(threadID, ids); len(rep) > 0 {
			targets = append([]Participant(nil), targets...)
			for i := range targets {
				if n := rep[targets[i].SessionID]; n != "" {
					targets[i].SessionID = n
				}
			}
			if n := rep[logTo]; n != "" {
				logTo = n
			}
		}
	}
```

Tulis di commit: "tambah hook pra-kirim aditif di manager.go dan router.go".

## Langkah 4: stub Go

Buat `internal/thread/recovery.go` (akan ditimpa `thread-lazy-respawn`):

```go
package thread

// Stub foundation; diganti oleh thread-lazy-respawn.
type Recovery struct{ m *Manager }

func NewRecovery(m *Manager) *Recovery { return &Recovery{m: m} }

func (r *Recovery) MarkDeadOnStart() error { return nil }

func (r *Recovery) EnsureAlive(threadID string, participantIds ...string) error { return nil }

func (r *Recovery) PreSend(threadID string, targetSessionIDs []string) (map[string]string, error) {
	return nil, nil
}

func BuildRecoveryContext(t Thread, projects []ProjectInfo, msgs []ThreadMessage, self Participant) string {
	return ""
}

var _ PreSendHook = (*Recovery)(nil).PreSend
```

Buat `internal/thread/orphans.go` (akan ditimpa `thread-orphan-cleanup`):

```go
package thread

// Stub foundation; diganti oleh thread-orphan-cleanup.
func CleanOrphans(m *Manager) (cleaned int, err error) { return 0, nil }
```

## Langkah 5: frontend

1. `frontend/src/modules/thread/types.ts`: ubah `Participant` menjadi
   `export interface Participant { sessionId: string; projectId: string; role: ThreadRole; name: string; dead?: boolean; previousSessionIds?: string[] }` dengan komentar "ditambah epic thread-ticket-05". Tambahkan di akhir berkas:
   ```ts
   export const RECOVERY_DEAD_NOTICE = 'daemon dimulai ulang: agent mati, akan dihidupkan saat pesan berikutnya';
   export const RECOVERY_RESPAWN_PREFIX = 'agent dihidupkan ulang: ';
   export const RECOVERY_FAILED_PREFIX = 'gagal menghidupkan ulang agent: ';
   ```
2. `frontend/src/modules/thread/thread-store.ts`, fungsi `participantName`: ubah pencarian menjadi
   `const p = thread?.participants.find((x) => x.sessionId === id || x.previousSessionIds?.includes(id));`
   Tidak ada perubahan lain.
3. Buat `frontend/src/modules/thread/recovery/index.ts` (akan ditimpa `thread-orphan-cleanup`):
   ```ts
   // Stub foundation; diganti oleh thread-orphan-cleanup.
   export function installRecovery(): () => void {
     return () => {};
   }
   ```

## Langkah 6: guard

- `go build ./...`
- `cd frontend && npm run build`

Keduanya harus lulus. Commit satu kali: "foundation thread-ticket-05: field peserta, hook pra-kirim, stub".
