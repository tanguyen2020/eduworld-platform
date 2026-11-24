package mail

import (
	"context"
	"io"
	"mime/quotedprintable"
	"slices"
	"strings"
	"testing"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
)

func decodeQP(t *testing.T, msg string) string {
	t.Helper()
	_, body, _ := strings.Cut(msg, "\r\n\r\n")
	b, err := io.ReadAll(quotedprintable.NewReader(strings.NewReader(body)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// F124 T022, T026 / F124-AC-07, AC-08, AC-09, AC-13, AC-17 (TC-CHAT-043, TC-CHAT-009): every staff template exists in vi and
// en; a school email carries only the fixed subject, the school name, the due instant and the portal link; the on-call page
// the priority and the case link; the break-glass email no reason — whatever extra parameter reaches the adapter.
func TestStaffMailTemplates(t *testing.T) {
	vi, en := StaffTemplateCodes("vi"), StaffTemplateCodes("en")
	slices.Sort(vi)
	slices.Sort(en)
	want := []string{"break_glass_request", "moderation_oncall_page", "school_safety_notification", "school_safety_reminder"}
	if !slices.Equal(vi, want) || !slices.Equal(en, want) {
		t.Fatalf("templates vi=%v en=%v", vi, en)
	}
	f := newFakeSMTP(t)
	s := newSMTP(t, f)
	ctx := context.Background()
	leak := map[string]string{"case_id": "0193a6c1-0000-7000-8000-00000000c5e1", "category_code": "asked_personal_info",
		"student_name": "Nguyen Van Binh", "summary": "noi dung vu viec", "body": "zqtin"}
	params := func(base map[string]string) map[string]string {
		for k, v := range leak {
			base[k] = v
		}
		return base
	}
	mails := []port.StaffMail{
		{To: "ad.truong.a@school.test", Locale: "vi-VN", Template: "school_safety_notification", SchoolName: "TH Demo A",
			Params: params(map[string]string{"ack_due_at": "2026-11-22T09:00:00Z"}), Link: "http://localhost:5173/school-lms/safety-notifications"},
		{To: "ad.truong.a@school.test", Locale: "vi-VN", Template: "school_safety_reminder", SchoolName: "TH Demo A",
			Params: params(map[string]string{"ack_due_at": "2026-11-22T09:00:00Z", "reminder_no": "2"}), Link: "http://localhost:5173/school-lms/safety-notifications"},
		{To: "sm.antoan@eduworld.test", Locale: "en", Template: "moderation_oncall_page", Params: map[string]string{"severity": "p1", "level": "primary"},
			Link: "http://localhost:5173/live-ops/moderation-cases?case_id=x"},
		{To: "sa.admin2@eduworld.test", Locale: "vi", Template: "break_glass_request", Params: map[string]string{"grant_id": "g", "reason": "phap ly"},
			Link: "http://localhost:5173/platform-admin/audit-logs"},
	}
	for i, m := range mails {
		if err := s.SendStaffMail(ctx, m); err != nil {
			t.Fatalf("%s: %v", m.Template, err)
		}
		raw := f.msgs[i]
		body := decodeQP(t, raw)
		for _, bad := range []string{"0193a6c1", "asked_personal_info", "Nguyen Van Binh", "noi dung vu viec", "zqtin", "phap ly"} {
			if strings.Contains(raw, bad) || strings.Contains(body, bad) {
				t.Fatalf("%s carries %q: %s", m.Template, bad, body)
			}
		}
		if !strings.Contains(body, m.Link) {
			t.Fatalf("%s without link: %s", m.Template, body)
		}
	}
	if b := decodeQP(t, f.msgs[0]); !strings.Contains(b, "TH Demo A") || !strings.Contains(b, "16:00 22/11/2026") {
		t.Fatalf("school notification: %s", b)
	}
	if b := decodeQP(t, f.msgs[1]); !strings.Contains(b, "lần 2") {
		t.Fatalf("reminder number: %s", b)
	}
	if !strings.Contains(f.msgs[2], "P1") {
		t.Fatalf("on-call priority: %s", f.msgs[2])
	}
	// Header injection and unknown templates are refused; SMTP replies keep the code only (R14).
	for _, m := range []port.StaffMail{
		{To: "a@b.test\r\nBcc: x@y.test", Template: "break_glass_request", Link: "https://p/x"},
		{To: "a@b.test", Template: "break_glass_request", Link: "https://p/x\r\nBcc: x"},
		{To: "a@b.test", Template: "student_report", Link: "https://p/x"},
	} {
		if err := s.SendStaffMail(ctx, m); err == nil {
			t.Fatalf("accepted: %+v", m)
		}
	}
	f.code = "550"
	if err := s.SendStaffMail(ctx, port.StaffMail{To: "secret.person@school.test", Template: "break_glass_request", Link: "https://p/x"}); err == nil ||
		strings.Contains(err.Error(), "secret.person") {
		t.Fatalf("err = %v", err)
	}
}
