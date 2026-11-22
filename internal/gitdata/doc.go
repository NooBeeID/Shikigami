// Package gitdata adalah paket baca-saja di atas CLI glab, gh, dan git.
//
// Hanya perintah yang lolos allowlist yang dijalankan; selain itu ditolak
// dengan CodeNotAllowed sebelum proses dimulai. Arah dependensi:
// gitdata -> connector. Paket ini TIDAK mengimpor workspace: path dan id
// project diteruskan pemanggil sebagai argumen. Data yang dikembalikan
// berupa tipe ternormalisasi, bukan JSON mentah dari CLI.
package gitdata
