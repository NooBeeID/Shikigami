//go:build !darwin || !cgo

package resource

// WebKitPIDs tanpa macOS+cgo tidak bisa memanggil simbol responsibility;
// selalu nil (WebKit tidak dihitung ke baris App).
func WebKitPIDs(appPid int, procs []RawProc) []int {
	return nil
}
