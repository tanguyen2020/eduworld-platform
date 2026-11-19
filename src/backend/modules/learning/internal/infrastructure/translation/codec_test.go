package translation

import (
	"slices"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
)

// F025-AC-22, AC-23: both formats round-trip the keys <content_type>/<id>/<version>/<pointer> and the texts.
func TestCodecRoundTrip(t *testing.T) {
	units := []port.TranslationUnit{{Key: "question/0193a5f4-6c1e-7b2a-9d1f-000000000601/1/stem/blocks/0/text", Source: "7 × 8 = ?", Target: ""},
		{Key: "lesson/0193a5f4-6c1e-7b2a-9d1f-000000000602/2/title", Source: "Bảng nhân, \"7\"", Target: "Times table"}}
	for _, f := range []string{"csv", "xliff"} {
		b, mime, err := Codec{}.Encode(f, "vi", "en", units)
		if err != nil || mime == "" {
			t.Fatalf("%s encode: %v", f, err)
		}
		got, err := Codec{}.Decode(f, b)
		if err != nil || !slices.Equal(got, units) {
			t.Fatalf("%s round trip: %v %+v\n%s", f, err, got, b)
		}
	}
	if _, err := (Codec{}).Decode("xliff", []byte("<nope")); err == nil {
		t.Fatal("broken xliff accepted")
	}
	if _, _, err := (Codec{}).Encode("docx", "vi", "en", nil); err == nil {
		t.Fatal("unknown format accepted")
	}
}
