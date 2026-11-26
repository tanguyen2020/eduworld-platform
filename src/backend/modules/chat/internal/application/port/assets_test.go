package port

import "testing"

// TestAssetVersionETag — the entity tag of a sticker/GIF version is its revision; If-Match accepts the strong and weak
// forms of the current revision, the bare number and *, nothing else (F082 Clarifications 2026-11-26).
func TestAssetVersionETag(t *testing.T) {
	v := AssetVersion{Revision: 3}
	if v.ETag() != `"3"` {
		t.Fatalf("ETag: %s", v.ETag())
	}
	for h, want := range map[string]bool{`"3"`: true, `W/"3"`: true, "3": true, "*": true, ` "3" `: true,
		`"2"`: false, `W/"4"`: false, "": false, `"03"`: false, `"3", "4"`: false} {
		if got := v.MatchesETag(h); got != want {
			t.Errorf("MatchesETag(%q) = %v, want %v", h, got, want)
		}
	}
}
