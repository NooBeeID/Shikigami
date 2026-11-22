package mention

import (
	"errors"
	"testing"
	"time"
)

const (
	testInterval = 2 * time.Millisecond
	testWait     = 30 * time.Millisecond
)

// Sudah ter-submit sebelum batas tunggu pertama: tidak ada Enter ulang.
func TestConfirmSubmitAlreadySubmitted(t *testing.T) {
	count := 1 // melebihi before=0
	resends := 0
	ok, n, err := confirmSubmit(func() int { return count }, 0,
		func() error { resends++; return nil }, testInterval, testWait, 3)
	if err != nil || !ok || n != 0 || resends != 0 {
		t.Fatalf("ok=%v n=%d resends=%d err=%v, want true 0 0 nil", ok, n, resends, err)
	}
}

// Enter pertama hilang; Enter ulang ke-2 yang berhasil men-submit.
func TestConfirmSubmitResendsUntilSubmitted(t *testing.T) {
	count := 0
	resends := 0
	ok, n, err := confirmSubmit(func() int { return count }, 0,
		func() error {
			resends++
			if resends == 2 {
				count++
			}
			return nil
		}, testInterval, testWait, 3)
	if err != nil || !ok || n != 2 || resends != 2 {
		t.Fatalf("ok=%v n=%d resends=%d err=%v, want true 2 2 nil", ok, n, resends, err)
	}
}

// Tidak pernah ter-submit: berhenti setelah batas retry, tanpa loop tak hingga.
func TestConfirmSubmitGivesUpAfterRetries(t *testing.T) {
	resends := 0
	ok, n, err := confirmSubmit(func() int { return 0 }, 0,
		func() error { resends++; return nil }, testInterval, testWait, 3)
	if err != nil || ok || n != 3 || resends != 3 {
		t.Fatalf("ok=%v n=%d resends=%d err=%v, want false 3 3 nil", ok, n, resends, err)
	}
}

// Sesi mati saat Enter ulang: galat dikembalikan dan retry berhenti.
func TestConfirmSubmitStopsOnResendError(t *testing.T) {
	boom := errors.New("sesi tidak berjalan")
	resends := 0
	ok, _, err := confirmSubmit(func() int { return 0 }, 0,
		func() error { resends++; return boom }, testInterval, testWait, 3)
	if ok || !errors.Is(err, boom) || resends != 1 {
		t.Fatalf("ok=%v resends=%d err=%v, want false 1 boom", ok, resends, err)
	}
}
