//go:build darwin && cgo

package resource

/*
#include <dlfcn.h>
#include <sys/types.h>

// responsibility_get_pid_responsible_for_pid adalah simbol privat dari
// /usr/lib/system/libquarantine.dylib (di-reekspor oleh libSystem): PID
// proses yang "bertanggung jawab" atas pid, mis. app pemilik layanan XPC
// WebKit; -1 bila pid tidak ada. Simbol dicari lewat dlsym saat dipakai,
// bukan ditautkan langsung, supaya bila macOS menghapusnya shikigamid tetap
// jalan (tanpa atribusi WebKit) alih-alih gagal saat link atau dyld load.
typedef pid_t (*resp_fn)(pid_t);

static resp_fn resp_lookup(void) {
	return (resp_fn)dlsym(RTLD_DEFAULT, "responsibility_get_pid_responsible_for_pid");
}

static pid_t resp_call(resp_fn f, pid_t pid) {
	return f(pid);
}
*/
import "C"

import "sync"

var (
	respOnce sync.Once
	respFn   C.resp_fn
)

// responsiblePID membungkus panggilan privat; ok=false bila gagal.
func responsiblePID(pid int) (int, bool) {
	r := int(C.resp_call(respFn, C.pid_t(pid)))
	if r <= 0 {
		return 0, false
	}
	return r, true
}

// WebKitPIDs mengembalikan PID proses com.apple.WebKit.* (GPU, WebContent,
// Networking) yang bertanggung jawab ke appPid. Proses ini layanan XPC
// (induknya launchd), jadi tidak terlihat lewat pohon proses app. Bila
// simbol privat tidak tersedia di macOS ini, mengembalikan nil (WebKit tidak
// dihitung), sama seperti build tanpa cgo.
func WebKitPIDs(appPid int, procs []RawProc) []int {
	respOnce.Do(func() { respFn = C.resp_lookup() })
	if respFn == nil {
		return nil
	}
	return filterWebKit(appPid, procs, responsiblePID)
}
