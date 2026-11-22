package thread

// Kontrak epic thread-ticket-05-thread-recovery. Teks pesan sistem adalah penanda yang
// dibaca frontend (awalan tetap); jangan diubah tanpa mengubah frontend/src/modules/thread/types.ts.
const (
	RecoveryDeadNotice    = "daemon dimulai ulang: agent mati, akan dihidupkan saat pesan berikutnya"
	RecoveryRespawnPrefix = "agent dihidupkan ulang: "
	RecoveryFailedPrefix  = "gagal menghidupkan ulang agent: "

	RecoveryContextMaxMessages = 40
	RecoveryContextMaxRunes    = 12000
	RecoveryPreviousIDsMax     = 20
)
