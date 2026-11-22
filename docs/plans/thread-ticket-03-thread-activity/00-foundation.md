# Foundation: thread-ticket-03-thread-activity

Rujukan: `docs/plans/thread-ticket-03-thread-activity/overview.md` (bagian 2 = kontrak dan keputusan). Foundation dikerjakan SENDIRIAN sebelum fitur paralel. Tanpa unit test; guard hanya build.

Tujuan: setelah selesai, field `count`/`items` ada di tipe pesan (Go dan TS), primitif log ganti-di-tempat tanpa hook tersedia, dan `activity.go` berupa stub yang berkompilasi. Aplikasi berperilaku sama seperti sebelumnya.

Berkas yang boleh disentuh: `internal/thread/types.go` (hanya menambah dua field), `internal/thread/activity_support.go` (baru), `internal/thread/activity.go` (stub baru), `frontend/src/modules/thread/types.ts` (hanya menambah dua field). Tidak ada yang lain.

## Langkah 0: verifikasi prasyarat

1. Pastikan ada `internal/thread/{types,store,manager,ipc}.go` dan `frontend/src/modules/thread/{types.ts,thread-store.ts,MessageList.svelte}`; `go build ./...` lulus.
2. Pastikan simbol ini ada (nama dibaca saat overview ditulis): `Store.storeLock`, `storeWriteFileAtomic`, `storeFileMessages`, `storePermFile`, `storeValidID`, `Store.ThreadDir`, `Store.logger`, `Manager.managerPublish`, `Manager.Get`, `managerNormalize`, `managerAllThreads`, `ErrClosed`, `ErrInvalid`, `ErrNotFound`, `EventMessage`, `KindMessage`, `KindActivity`, `ToAll`, `FromSystem`, `NewID`. Bila nama berbeda, ikuti kode dan catat di bagian "Catatan" bawah (jangan mengubah berkas tiket 01 untuk menyesuaikan).
3. Boleh ada atau tidak kode tiket 02; epic ini tidak bergantung padanya.

## Langkah 1: field opsional di `types.go`

Pada struct `ThreadMessage` tambahkan setelah `Kind`:

```go
Count int      `json:"count,omitempty"` // hanya kind=activity: jumlah aksi yang digabung
Items []string `json:"items,omitempty"` // hanya kind=activity: ringkasan per aksi (maks 50 terbaru), tanpa awalan nama
```

Tambah komentar singkat bahwa field ini ditambah epic thread-ticket-03. Tidak mengubah field lain, tag, atau tipe lain.

## Langkah 2: `internal/thread/activity_support.go`

Isi persis kontrak overview 2.3.

1. `func (s *Store) UpsertMessage(m ThreadMessage) error`:
   - validasi sama seperti `AppendMessage` (threadId valid, id tidak kosong, ts tidak nol, direktori thread ada -> `ErrNotFound`), `m.TS = m.TS.UTC()`, marshal satu baris;
   - ambil `storeLock(m.ThreadID)` dan tahan selama baca-tulis;
   - baca `messages.jsonl` (tidak ada -> anggap kosong); pecah per baris (`bytes.Split` `\n`, buang baris kosong tetapi pertahankan baris rusak apa adanya agar tidak hilang);
   - cari dari BELAKANG baris yang JSON-nya ber-`id` sama (unmarshal ke struct kecil `{ID string}`; baris tak terbaca dilewati). Ketemu: ganti baris itu dengan baris baru. Tidak ketemu: tambahkan di akhir;
   - tulis: jalur cepat bila tidak ketemu -> `os.OpenFile(O_APPEND|O_CREATE|O_WRONLY)` tulis satu baris (seperti `AppendMessage`); bila ketemu -> `storeWriteFileAtomic(path, gabungan, storePermFile)` (semua baris + `\n` penutup);
   - galat dibungkus pesan Indonesia (`fmt.Errorf("upsert pesan: %w", err)`).
2. `func (m *Manager) UpsertMessage(threadID string, msg ThreadMessage) (ThreadMessage, error)`: salin isi default dari `Post` (ThreadID, ID via `NewID()` bila kosong, TS UTC sekarang bila nol, Kind default `KindMessage`, To default `ToAll`, From default `FromSystem`); `Get(threadID)` dan tolak `ErrClosed` bila `State == StateClosed`; `m.cfg.Store.UpsertMessage(msg)`; `m.managerPublish(EventMessage, msg)`; kembalikan `msg`. JANGAN memanggil `managerFireMessage` (keputusan 2.2 no. 2); beri komentar yang menjelaskan alasannya.
3. `func (m *Manager) ActiveThreads() []Thread`: lewati `managerAllThreads()`, ambil yang `State == StateActive`, `managerNormalize` tiap thread, kembalikan slice non-nil.

## Langkah 3: stub `internal/thread/activity.go`

Tulis `ActivitySource`, konstanta `ActivityMergeGap`, `ActivityMaxItems`, `ActivitySummaryMax`, dan `InstallActivity(m *Manager, src ActivitySource) (stop func())` dengan signature persis overview 2.3. Badan stub mengembalikan `func() {}` tanpa efek. Komentar `// STUB(foundation): diganti oleh thread-activity-daemon`. Package comment/komentar bahasa Indonesia.

## Langkah 4: `frontend/src/modules/thread/types.ts`

Pada `interface ThreadMessage` tambahkan `count?: number;` dan `items?: string[];` (beri komentar "hanya kind 'activity'"). Tidak ada perubahan lain.

## Langkah 5: verifikasi

1. `gofmt -l internal/thread` kosong; `go vet ./internal/thread/...`; `go build ./...`.
2. `cd frontend && npm run check && npm run build`.
3. `git status`: hanya berkas pada daftar "boleh disentuh" yang berubah.

## Catatan

(Foundation mengisi di sini bila ada penyesuaian nama terhadap kode tiket 01.)
