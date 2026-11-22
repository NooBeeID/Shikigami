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
