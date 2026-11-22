package mention

import (
	"time"

	"github.com/NooBeeID/shikigami/internal/status"
)

const (
	// submitConfirmWait adalah lama menunggu bukti submit sebelum Enter diulang.
	submitConfirmWait = 4 * time.Second
	// submitConfirmInterval adalah jarak pengecekan bukti submit.
	submitConfirmInterval = 100 * time.Millisecond
	// submitMaxResends membatasi Enter ulang per pesan (terburuk ~16 detik total).
	submitMaxResends = 3
)

// confirmSubmit memastikan prompt benar-benar ter-submit. Bukti submit adalah
// count() > before (event hook UserPromptSubmit). Bila belum ada bukti setelah
// wait, resend() dipanggil (Enter ulang), paling banyak maxResends kali.
// Mengembalikan: terkonfirmasi atau tidak, jumlah resend yang dilakukan, dan galat
// dari resend (galat menghentikan retry, mis. sesi sudah mati).
//
// Mengapa perlu: TUI agen (Claude Code) bisa membuang Enter yang tiba saat masih
// mengerjakan startup. Teks sudah ter-paste tetapi menganggur di kotak input,
// sehingga manusia harus menekan Enter. Mengulang Enter pada kotak yang sudah
// kosong tidak berbahaya.
func confirmSubmit(count func() int, before int, resend func() error, interval, wait time.Duration, maxResends int) (confirmed bool, resends int, err error) {
	waitFor := func() bool {
		deadline := time.Now().Add(wait)
		for {
			if count() > before {
				return true
			}
			if !time.Now().Before(deadline) {
				return false
			}
			time.Sleep(interval)
		}
	}
	if waitFor() {
		return true, 0, nil
	}
	for i := 1; i <= maxResends; i++ {
		if err := resend(); err != nil {
			return false, i, err
		}
		if waitFor() {
			return true, i, nil
		}
	}
	return false, maxResends, nil
}

// confirmEnter memastikan pesan ke sesi `to` ter-submit; dijalankan di goroutine
// terpisah agar tidak menahan pemanggil Enqueue (tool MCP agen, alur spawn thread).
func (q *Queue) confirmEnter(to string, before int) {
	defer func() {
		if r := recover(); r != nil {
			q.cfg.Logger.Error("mention: panic saat konfirmasi submit", "to", to, "panic", r)
		}
	}()
	ok, resends, err := confirmSubmit(
		func() int { n, _ := status.PromptSubmits(to); return n },
		before,
		func() error { return q.cfg.Sessions.Write(to, []byte("\r")) },
		submitConfirmInterval, submitConfirmWait, submitMaxResends,
	)
	switch {
	case err != nil:
		q.cfg.Logger.Debug("mention: Enter ulang dihentikan", "to", to, "resends", resends, "err", err)
	case !ok:
		q.cfg.Logger.Warn("mention: pesan belum terkonfirmasi ter-submit setelah Enter diulang", "to", to, "resends", resends)
	case resends > 0:
		q.cfg.Logger.Info("mention: pesan ter-submit setelah Enter diulang", "to", to, "resends", resends)
	}
}
