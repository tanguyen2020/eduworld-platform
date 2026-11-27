package ids

import (
	"regexp"
	"testing"
	"time"
)

var (
	v7 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	v4 = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
)

func TestFormats(t *testing.T) {
	a, b := V7(time.UnixMilli(1000)), V7(time.UnixMilli(2000))
	if !v7.MatchString(a) || !v4.MatchString(V4()) {
		t.Fatalf("bad ids %s", a)
	}
	if a[:13] >= b[:13] {
		t.Fatalf("v7 must be time ordered: %s %s", a, b)
	}
}
