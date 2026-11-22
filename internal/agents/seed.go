package agents

// builtinCategories mengembalikan kategori bawaan berurutan.
func builtinCategories() []Category {
	return []Category{
		{ID: idOrchestrator, Name: "Orchestrator", Builtin: true, Role: "Mengoordinasi pekerjaan: memecah tujuan, memilih agent yang tepat, dan memantau hasilnya; tidak mengerjakan implementasi sendiri."},
		{ID: idResearcher, Name: "Researcher", Builtin: true, Role: "Menyelidiki dan mengumpulkan informasi; hanya membaca, tidak mengubah kode."},
		{ID: idPlanner, Name: "Planner", Builtin: true, Role: "Hanya merencanakan: menyusun rencana langkah demi langkah; tidak mengubah kode."},
		{ID: idExecutor, Name: "Executor", Builtin: true, Role: "Mengerjakan perubahan kode sesuai rencana dan melaporkan hasilnya."},
	}
}

// seedMissing menambahkan kategori bawaan yang belum ada (di akhir) dan
// memaksa Builtin=true pada yang ber-id bawaan. Nama/peran ubahan pengguna
// dipertahankan. Mengembalikan true bila ada perubahan.
func seedMissing(st *fileState) (changed bool) {
	for _, b := range builtinCategories() {
		found := false
		for i := range st.Categories {
			if st.Categories[i].ID == b.ID {
				found = true
				if !st.Categories[i].Builtin {
					st.Categories[i].Builtin = true
					changed = true
				}
				break
			}
		}
		if !found {
			st.Categories = append(st.Categories, b)
			changed = true
		}
	}
	return changed
}
