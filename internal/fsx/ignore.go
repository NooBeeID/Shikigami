package fsx

// DefaultIgnore adalah sumber kebenaran daftar nama yang disembunyikan dari tree.
// ignoreSet dibangun dari slice ini sekali saat inisialisasi paket; mengubah
// slice setelah startup tidak berpengaruh.
var DefaultIgnore = []string{
	"node_modules", ".git", "target", "dist", "build", ".next", ".nuxt",
	".svelte-kit", ".cache", ".venv", "venv", "__pycache__", ".idea",
	".DS_Store", "vendor", "Pods", ".gradle", "coverage",
}

var ignoreSet = buildIgnoreSet(DefaultIgnore)

func buildIgnoreSet(names []string) map[string]struct{} {
	m := make(map[string]struct{}, len(names))
	for _, n := range names {
		m[n] = struct{}{}
	}
	return m
}

// IsIgnored mencocokkan nama persis (peka huruf besar), bukan path.
func IsIgnored(name string) bool {
	_, ok := ignoreSet[name]
	return ok
}
