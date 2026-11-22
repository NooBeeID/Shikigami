package grimoire

import (
	"strings"
)

// LLMMaxSourceBytes membatasi isi sumber per modul pada prompt; lebih dari itu dipotong.
const LLMMaxSourceBytes = 48 * 1024

// LLMExpectedOutputBytes adalah perkiraan keluaran per modul (untuk estimasi).
const LLMExpectedOutputBytes = 3 * 1024

// promptTemplate adalah template tetap. Penampung: %MODULE%, %STATIC%, %SOURCES%.
const promptTemplate = `Kamu menulis satu bab dokumentasi "grimoire" (basis pengetahuan kode) dalam bahasa Indonesia.

Tugas: tulis ringkasan snapshot fakta untuk modul yang disebut di bawah, berdasarkan kerangka statik dan isi sumber yang diberikan.

Aturan:
- Tulis keadaan kode saat ini sebagai fakta. Jangan menulis tanggal, changelog, atau narasi proses pengerjaan.
- Jelaskan tanggung jawab modul, hubungannya dengan modul lain, dan "mengapa" yang terlihat dari kode.
- Baris pertama keluaran WAJIB berformat: Sumber: <path relatif repo>, <path relatif repo>, ...
- Keluarkan hanya isi bab dalam Markdown. Tanpa kalimat pembuka atau penutup percakapan, dan tanpa code fence yang membungkus seluruh keluaran.
- Jangan menjalankan tool apa pun dan jangan membaca berkas lain; hanya pakai materi di bawah.

## Nama modul
%MODULE%

## Kerangka statik
%STATIC%

## Sumber
%SOURCES%
`

// promptSource adalah satu berkas sumber pada prompt. Note menandai berkas yang dilewati.
type promptSource struct {
	Path    string
	Content string
	Note    string
}

// LLMPromptOverheadBytes mengembalikan ukuran template tetap tanpa isi modul. Murni, tanpa I/O.
func LLMPromptOverheadBytes() int { return len(promptTemplate) }

// promptBuild merakit prompt untuk satu modul. Pemotongan total dilakukan pemuat sumber.
func promptBuild(m Module, static string, sources []promptSource) string {
	var sb strings.Builder
	for _, s := range sources {
		sb.WriteString("=== " + s.Path + " ===\n")
		if s.Note != "" {
			sb.WriteString("(" + s.Note + ")\n\n")
			continue
		}
		sb.WriteString(s.Content)
		if !strings.HasSuffix(s.Content, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	if len(sources) == 0 {
		sb.WriteString("(tidak ada sumber terbaca)\n")
	}
	r := strings.NewReplacer("%MODULE%", m.Name, "%STATIC%", static, "%SOURCES%", sb.String())
	return r.Replace(promptTemplate)
}
