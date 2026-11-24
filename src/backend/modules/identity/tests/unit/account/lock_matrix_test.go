package account

import (
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/user"
)

// F124 T012 / F124-AC-02, AC-05 (TC-AUTH-006, SC-002): the matrix lock sources × release. An account is active only
// when no lock of any source remains; a lock of the Safety Team (moderation_sanction) is released only by sanction.lifted,
// never by the unlock of a school or of the platform (409 ACCOUNT_LOCK_HELD_BY_SAFETY).
func TestLockMatrix(t *testing.T) {
	all := []string{user.LockSchoolAdmin, user.LockPlatformAdmin, user.LockSIS, user.LockModeration}
	for _, held := range [][]string{{}, {user.LockSchoolAdmin}, {user.LockModeration}, {user.LockSchoolAdmin, user.LockModeration},
		{user.LockSIS, user.LockPlatformAdmin}, all} {
		for _, releaser := range all {
			remaining := user.Release(held, releaser)
			for _, r := range remaining {
				if r == releaser {
					t.Errorf("%v released by %s keeps %s", held, releaser, r)
				}
			}
			out := user.AfterRelease(remaining)
			if out.Active != (len(remaining) == 0) {
				t.Errorf("%v - %s: active %v with %v", held, releaser, out.Active, remaining)
			}
			if out.HeldBySafety != contains(remaining, user.LockModeration) {
				t.Errorf("%v - %s: held by safety %v", held, releaser, out.HeldBySafety)
			}
		}
	}
	// Only the moderation source is released by a sanction; the unlock routes release their own source only.
	if user.SourceOfRoute("tenant") != user.LockSchoolAdmin || user.SourceOfRoute("platform") != user.LockPlatformAdmin {
		t.Fatal("route sources")
	}
	for _, s := range all {
		if !user.ValidLockSource(s) {
			t.Fatalf("source %s", s)
		}
	}
	if user.ValidLockSource("teacher") {
		t.Fatal("unknown source")
	}
}

func contains(list []string, x string) bool {
	for _, v := range list {
		if v == x {
			return true
		}
	}
	return false
}
