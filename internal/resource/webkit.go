package resource

import "strings"

// webKitMarker penanda baris ps milik layanan XPC WebKit.
const webKitMarker = "com.apple.WebKit"

// filterWebKit mengembalikan PID proses WebKit yang menurut responsible
// bertanggung jawab ke appPid. responsible mengembalikan ok=false bila
// pencarian gagal; PID itu dilewati.
func filterWebKit(appPid int, procs []RawProc, responsible func(pid int) (int, bool)) []int {
	if appPid <= 0 || responsible == nil {
		return nil
	}
	var out []int
	for _, p := range procs {
		if p.PID <= 0 || p.PID == appPid || !strings.Contains(p.Command, webKitMarker) {
			continue
		}
		r, ok := responsible(p.PID)
		if !ok || r != appPid {
			continue
		}
		out = append(out, p.PID)
	}
	return out
}
