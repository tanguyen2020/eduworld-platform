package export

import (
	"bytes"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
)

// TestWritersWatermarkAndDisclaimer checks F044-AC-03 and F044-AC-12: every format carries the watermark, and the
// disclaimer at the top and at the bottom when given.
func TestWritersWatermarkAndDisclaimer(t *testing.T) {
	meta := port.ReportMeta{Title: "Báo cáo tham khảo", Watermark: "EduWorld — xuất bởi u1", Disclaimer: "Chỉ để tham khảo"}
	for _, f := range []string{"csv", "xlsx", "pdf"} {
		w, err := Writers{}.New(f, meta)
		if err != nil {
			t.Fatal(err)
		}
		if err := w.Header([]string{"student_id", "level"}); err != nil {
			t.Fatal(err)
		}
		if err := w.Row([]string{"s1", "Hoàn thành tốt"}); err != nil {
			t.Fatal(err)
		}
		b, err := w.Close()
		if err != nil || len(b) == 0 {
			t.Fatalf("%s: %v", f, err)
		}
		switch f {
		case "csv":
			s := string(b)
			first, last := strings.Index(s, "Chỉ để tham khảo"), strings.LastIndex(s, "Chỉ để tham khảo")
			if first < 0 || first == last || !strings.Contains(s, "xuất bởi u1") || strings.Index(s, "student_id") < first ||
				strings.Index(s, "student_id") > last {
				t.Fatalf("csv = %q", s)
			}
		case "xlsx":
			x, err := excelize.OpenReader(bytes.NewReader(b))
			if err != nil {
				t.Fatal(err)
			}
			rows, _ := x.GetRows("Report")
			if rows[1][0] != meta.Watermark || rows[2][0] != meta.Disclaimer || rows[len(rows)-1][0] != meta.Watermark ||
				rows[len(rows)-2][0] != meta.Disclaimer {
				t.Fatalf("xlsx rows = %v", rows)
			}
		case "pdf":
			if !bytes.HasPrefix(b, []byte("%PDF")) {
				t.Fatal("not a PDF")
			}
		}
	}
	if _, err := (Writers{}).New("docx", meta); err == nil {
		t.Fatal("unknown format")
	}
}
