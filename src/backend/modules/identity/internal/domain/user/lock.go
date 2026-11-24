package user

import "slices"

// Sources of an account lock (identity.account_locks.source, ACCOUNT_PROVISIONING §6, F124): each lock has its own
// source and the account is active only when no lock remains (F124-AC-02). The lock of the Safety Team
// (moderation_sanction, a sanction account_lock) is released only by sanction.lifted (MODERATION §7).
const (
	LockSchoolAdmin   = "school_admin"        // lockTenantUser (tenant.user.lock)
	LockPlatformAdmin = "platform_admin"      // lockUser (platform.user.manage)
	LockSIS           = "sis"                 // SIS record status suspended
	LockModeration    = "moderation_sanction" // sanction account_lock of the Platform Safety Team
)

var lockSources = []string{LockSchoolAdmin, LockPlatformAdmin, LockSIS, LockModeration}

// ValidLockSource reports a known source.
func ValidLockSource(s string) bool { return slices.Contains(lockSources, s) }

// SourceOfRoute returns the lock source of a lifecycle route ("tenant" → school_admin, otherwise platform_admin).
func SourceOfRoute(route string) string {
	if route == "tenant" {
		return LockSchoolAdmin
	}
	return LockPlatformAdmin
}

// Release returns the sources of held that remain after releasing every lock of source.
func Release(held []string, source string) []string {
	out := make([]string, 0, len(held))
	for _, h := range held {
		if h != source {
			out = append(out, h)
		}
	}
	return out
}

// ReleaseOutcome tells what a release leaves: active when no lock remains; HeldBySafety when a lock of the Safety Team
// remains (409 ACCOUNT_LOCK_HELD_BY_SAFETY for an unlock of the school or the platform, F124-AC-05).
type ReleaseOutcome struct {
	Active       bool
	HeldBySafety bool
}

// AfterRelease returns the outcome of the remaining sources.
func AfterRelease(remaining []string) ReleaseOutcome {
	return ReleaseOutcome{Active: len(remaining) == 0, HeldBySafety: slices.Contains(remaining, LockModeration)}
}
