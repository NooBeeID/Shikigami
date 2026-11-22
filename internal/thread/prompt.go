package thread

import (
	"fmt"
	"sort"
	"strings"

	"github.com/NooBeeID/shikigami/internal/agents"
)

// ToolsSection menghasilkan bagian tool pada prompt master. Tiket thread-agents
// mengganti variabel ini; string kosong menghilangkan bagian tersebut.
var ToolsSection = func() string {
	return "Tool thread (untuk men-spawn peserta, mengirim pesan antar peserta, dan mengusulkan penutupan) segera tersedia; sampai saat itu kamu belum bisa memanggilnya, jadi balas user langsung di terminal ini."
}

// BuildMasterPrompt menyusun briefing awal master. Antrean menambahkan awalan
// "[pesan dari user] ", jadi kalimat pertama dibuat wajar setelah awalan itu.
func BuildMasterPrompt(t Thread, projects []ProjectInfo, agentsSection string) string {
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
	if s := strings.TrimSpace(agentsSection); s != "" {
		b.WriteString(s)
		b.WriteString("\n\n")
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

// agentsSection menghasilkan bagian "Agent yang tersedia" untuk prompt master.
// Resolver nil, galat, atau tanpa agent -> "" (bagian dihilangkan).
func (m *Manager) agentsSection() (out string) {
	if m.cfg.Agents == nil {
		return ""
	}
	defer func() {
		if r := recover(); r != nil {
			out = ""
		}
	}()
	cats, ags := m.cfg.Agents.List()
	if len(ags) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Agent yang tersedia:\n")
	line := func(a agents.Agent, name, role string) {
		fmt.Fprintf(&b, "- %s (kategori %s)", a.Name, name)
		if a.IsDefault {
			b.WriteString(", default")
		}
		if r := agentRoleLine(role); r != "" {
			b.WriteString(" : " + r)
		}
		b.WriteString("\n")
	}
	known := map[string]bool{}
	for _, c := range cats {
		known[c.ID] = true
		var in []agents.Agent
		for _, a := range ags {
			if a.CategoryID == c.ID {
				in = append(in, a)
			}
		}
		sort.SliceStable(in, func(i, j int) bool { return strings.ToLower(in[i].Name) < strings.ToLower(in[j].Name) })
		for _, a := range in {
			line(a, c.Name, c.Role)
		}
	}
	var rest []agents.Agent
	for _, a := range ags {
		if !known[a.CategoryID] {
			rest = append(rest, a)
		}
	}
	sort.SliceStable(rest, func(i, j int) bool { return strings.ToLower(rest[i].Name) < strings.ToLower(rest[j].Name) })
	for _, a := range rest {
		line(a, a.CategoryID, "")
	}
	b.WriteString("\nPilih agent atau kategori menurut sifat pekerjaan (perencanaan -> Planner, riset -> Researcher, pengubahan kode -> Executor), lalu isi parameter `agent` (nama agent) atau `category` (id/nama kategori) pada `thread_spawn`. Bila tidak yakin, biarkan kosong (sistem memakai default).")
	return b.String()
}
