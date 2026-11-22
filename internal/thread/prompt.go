package thread

import (
	"fmt"
	"strings"
)

// ToolsSection menghasilkan bagian tool pada prompt master. Tiket thread-agents
// mengganti variabel ini; string kosong menghilangkan bagian tersebut.
var ToolsSection = func() string {
	return "Tool thread (untuk men-spawn peserta, mengirim pesan antar peserta, dan mengusulkan penutupan) segera tersedia; sampai saat itu kamu belum bisa memanggilnya, jadi balas user langsung di terminal ini."
}

// BuildMasterPrompt menyusun briefing awal master. Antrean menambahkan awalan
// "[pesan dari user] ", jadi kalimat pertama dibuat wajar setelah awalan itu.
func BuildMasterPrompt(t Thread, projects []ProjectInfo) string {
	var b strings.Builder
	b.WriteString("Ini briefing awal untukmu. Kamu adalah master dari sebuah thread, yaitu diskusi beberapa agent lintas project dalam satu workspace untuk mencapai satu goal.\n\n")
	fmt.Fprintf(&b, "Judul thread: %s\n\n", t.Title)
	b.WriteString("Goal dari user:\n")
	b.WriteString(t.Goal)
	b.WriteString("\n\n")
	if len(projects) == 0 {
		b.WriteString("Workspace ini belum punya project terdaftar.\n\n")
	} else {
		b.WriteString("Project di workspace ini:\n")
		for _, p := range projects {
			fmt.Fprintf(&b, "- %s (id: %s) path: %s\n", p.Name, p.ID, p.Path)
		}
		b.WriteString("\n")
	}
	b.WriteString("Peranmu: merutekan pekerjaan, memutuskan project mana yang perlu dilibatkan, meneruskan instruksi user ke agent yang sesuai, dan mengusulkan penutupan saat goal tercapai. Kamu TIDAK menutup thread sendiri; hanya user yang menutup.\n\n")
	b.WriteString("Aturan: pesan user datang lewat thread dan akan tiba di terminal ini. Kamu bekerja di direktori netral milik thread dan TIDAK mengedit file project mana pun. Jangan menyentuh sesi lain milik user. Jawab singkat dan jelas.\n\n")
	if s := strings.TrimSpace(ToolsSection()); s != "" {
		b.WriteString(s)
		b.WriteString("\n\n")
	}
	b.WriteString("Mulailah dengan memahami goal, lalu sampaikan rencana singkatmu ke user.")
	return b.String()
}
