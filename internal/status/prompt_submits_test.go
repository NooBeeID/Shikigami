package status

import "testing"

// PromptSubmits menghitung event UserPromptSubmit, termasuk saat status sudah
// "running" (Set tidak berubah), karena itulah satu-satunya bukti bahwa prompt
// benar-benar ter-submit (SessionStart juga membuat status running).
func TestPromptSubmitsCountsOnlyUserPromptSubmit(t *testing.T) {
	e := newListenerEngine()
	e.RegisterLaunch(LaunchInfo{Token: "tok", WorkspaceID: "ws1", ProviderID: "claude"})

	if err := e.Apply("s1", HookSessionStart, nil); err != nil {
		t.Fatal(err)
	}
	n, reliable := e.PromptSubmits("s1")
	if n != 0 || !reliable {
		t.Fatalf("setelah SessionStart: n=%d reliable=%v, want 0 true", n, reliable)
	}

	_ = e.Apply("s1", HookUserPromptSubmit, nil)
	if n, _ := e.PromptSubmits("s1"); n != 1 {
		t.Fatalf("setelah UserPromptSubmit: n=%d, want 1", n)
	}

	_ = e.Apply("s1", HookStop, nil)
	if n, _ := e.PromptSubmits("s1"); n != 1 {
		t.Fatalf("Stop tidak boleh menambah: n=%d, want 1", n)
	}

	// status sudah running dari prompt sebelumnya? Paksa running lalu submit lagi.
	_ = e.Apply("s1", HookSessionStart, nil)
	_ = e.Apply("s1", HookUserPromptSubmit, nil)
	if n, _ := e.PromptSubmits("s1"); n != 2 {
		t.Fatalf("submit kedua saat status sudah running: n=%d, want 2", n)
	}
}

// Sesi tanpa hooks aktif (tidak terikat launch token) tidak bisa dikonfirmasi.
func TestPromptSubmitsUnreliableWithoutHooks(t *testing.T) {
	e := newListenerEngine()
	if _, reliable := e.PromptSubmits("s1"); reliable {
		t.Fatal("sesi tanpa token seharusnya tidak reliable")
	}
}
