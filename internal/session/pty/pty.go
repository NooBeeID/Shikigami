// Paket pty menyediakan pembungkus PTY tipis untuk Unix. Paket ini tidak
// menjalankan goroutine; pembacaan dilakukan oleh pemanggil.
package pty

// Size adalah ukuran terminal dalam kolom dan baris.
type Size struct{ Cols, Rows uint16 }

// PTY adalah pembungkus tipis atas pseudo-terminal.
type PTY interface {
	// Start menjalankan proses di dalam PTY baru dengan ukuran awal size.
	// Cwd kosong berarti mewarisi direktori kerja proses.
	Start(cmd string, args []string, env []string, cwd string, size Size) error
	// Read membaca keluaran proses; mengembalikan error (io.EOF atau EIO)
	// setelah proses selesai atau master fd ditutup.
	Read(p []byte) (int, error)
	// Write menulis masukan ke proses.
	Write(p []byte) (int, error)
	// Resize mengubah ukuran jendela terminal.
	Resize(size Size) error
	// Close mengirim SIGHUP ke process group dan menutup master fd;
	// idempoten dan aman dipanggil dari goroutine lain.
	Close() error
	// Wait memblok sampai proses selesai. Kode keluar normal = kode proses,
	// bila di-signal = 128+signal. Hanya boleh dipanggil sekali.
	Wait() (exitCode int, err error)
	// Pid mengembalikan PID proses anak; 0 bila belum start.
	Pid() int
}
