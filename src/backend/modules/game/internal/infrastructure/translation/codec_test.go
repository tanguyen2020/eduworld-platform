package translation_test

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/i18ntext"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/infrastructure/translation"
)

var file = i18ntext.File{SourceLanguage: "vi", TargetLanguage: "en", Units: []i18ntext.Unit{
	{Key: "quests/0193a5d2-0000-7000-8000-000000000001/3/name", Source: "Rồng <lửa> & \"bạn\"", Target: "", Note: "missing"},
	{Key: "quests/0193a5d2-0000-7000-8000-000000000001/3/steps/0/title", Source: "Bước, một\ndòng hai", Target: "Step, one", Note: "outdated"},
}}

func TestXLIFFRoundTrip(t *testing.T) {
	c := translation.XLIFF{}
	var buf bytes.Buffer
	if err := c.Encode(&buf, file); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	for _, want := range []string{`<xliff xmlns="urn:oasis:names:tc:xliff:document:2.0" version="2.1" srcLang="vi" trgLang="en">`, `<file id="quests">`,
		`<unit id="quests/0193a5d2-0000-7000-8000-000000000001/3/name">`, "&lt;lửa&gt; &amp;"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in\n%s", want, s)
		}
	}
	got, err := c.Decode(strings.NewReader(s))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, file) {
		t.Fatalf("round trip\n%+v\nwant\n%+v", got, file)
	}
}

func TestXLIFFRefusesDTDAndEntities(t *testing.T) {
	xxe := `<?xml version="1.0"?><!DOCTYPE x [<!ENTITY e SYSTEM "file:///etc/passwd">]>
<xliff version="2.1" srcLang="vi" trgLang="en"><file id="quests"><unit id="k"><segment><source>&e;</source></segment></unit></file></xliff>`
	if _, err := (translation.XLIFF{}).Decode(strings.NewReader(xxe)); !errors.Is(err, port.ErrMalformedFile) {
		t.Fatalf("DTD accepted: %v", err)
	}
	for _, bad := range []string{"not xml", `<xliff version="1.2"></xliff>`, `<root/>`} {
		if _, err := (translation.XLIFF{}).Decode(strings.NewReader(bad)); !errors.Is(err, port.ErrMalformedFile) {
			t.Errorf("%q accepted: %v", bad, err)
		}
	}
}

func TestCSVRoundTripWithBOM(t *testing.T) {
	c := translation.CSV{}
	var buf bytes.Buffer
	if err := c.Encode(&buf, file); err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("\xEF\xBB\xBFkey,source,target,note\n")) {
		t.Fatalf("header/BOM: %q", buf.String()[:40])
	}
	got, err := c.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	want := file
	want.TargetLanguage = ""
	if !reflect.DeepEqual(got.Units, want.Units) {
		t.Fatalf("round trip %+v", got)
	}
	// without BOM and with CRLF is read too; a wrong header is malformed.
	if _, err := c.Decode(strings.NewReader("key,source,target,note\r\nquests/x/1/name,a,b,\r\n")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Decode(strings.NewReader("a,b\n1,2\n")); !errors.Is(err, port.ErrMalformedFile) {
		t.Fatalf("bad header accepted: %v", err)
	}
}
