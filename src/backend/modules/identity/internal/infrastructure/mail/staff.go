package mail

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"text/template"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
)

// Staff emails of the Platform Safety Team (F124): the wording lives in the translation files templates/{vi,en}.json
// (AGENTS §7: UI strings in translation files), rendered with the allowed parameters only — the school name, a due
// instant, the reminder number, the priority and the portal link. Never a student name, a case summary, a category or
// chat content (R14, F124-AC-09); messages and addresses are never logged.

//go:embed templates/*.json
var templateFiles embed.FS

type staffTemplate struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

type compiled struct{ subject, body *template.Template }

// staffTemplates are the compiled templates by locale and code.
var staffTemplates = func() map[string]map[string]compiled {
	out := map[string]map[string]compiled{}
	for _, lang := range []string{"vi", "en"} {
		raw, err := templateFiles.ReadFile("templates/" + lang + ".json")
		if err != nil {
			panic(err)
		}
		var m map[string]staffTemplate
		if err := json.Unmarshal(raw, &m); err != nil {
			panic(fmt.Errorf("mail: templates/%s.json: %w", lang, err))
		}
		out[lang] = map[string]compiled{}
		for code, t := range m {
			out[lang][code] = compiled{subject: template.Must(template.New(code + ".subject").Option("missingkey=error").Parse(t.Subject)),
				body: template.Must(template.New(code + ".body").Option("missingkey=error").Parse(t.Body))}
		}
	}
	return out
}()

// StaffTemplateCodes returns the template codes of a locale (tests: every template exists in every locale).
func StaffTemplateCodes(lang string) []string {
	var out []string
	for code := range staffTemplates[lang] {
		out = append(out, code)
	}
	return out
}

type staffData struct {
	SchoolName string
	AckDueAt   string
	ReminderNo string
	Severity   string
	Link       string
}

// timeZone is the platform time zone of the due instants (EW_PLATFORM_TIMEZONE, default Asia/Ho_Chi_Minh; a fixed
// UTC+7 when the zone database is missing).
func (s SMTP) timeZone() *time.Location {
	if s.cfg.Location != nil {
		return s.cfg.Location
	}
	return time.FixedZone("ICT", 7*3600)
}

// RenderStaffMail renders the subject and the plain-text body of m (exported for the content tests).
func (s SMTP) RenderStaffMail(m port.StaffMail) (string, string, error) {
	t, ok := staffTemplates[lang(m.Locale)][m.Template]
	if !ok {
		return "", "", errors.New("mail: unknown staff template")
	}
	if strings.ContainsAny(m.Link, "\r\n") || !strings.HasPrefix(m.Link, "http") {
		return "", "", errors.New("mail: invalid link")
	}
	d := staffData{SchoolName: strings.TrimSpace(m.SchoolName), ReminderNo: m.Params["reminder_no"], Severity: strings.ToUpper(m.Params["severity"]),
		Link: m.Link}
	if d.SchoolName == "" {
		d.SchoolName = "EduWorld"
	}
	if due, err := time.Parse(time.RFC3339, m.Params["ack_due_at"]); err == nil {
		d.AckDueAt = due.In(s.timeZone()).Format("15:04 02/01/2006")
	}
	var subject, body bytes.Buffer
	if err := t.subject.Execute(&subject, d); err != nil {
		return "", "", fmt.Errorf("mail: render subject: %w", err)
	}
	if err := t.body.Execute(&body, d); err != nil {
		return "", "", fmt.Errorf("mail: render body: %w", err)
	}
	return strings.ReplaceAll(subject.String(), "\n", " "), body.String(), nil
}

// SendStaffMail implements port.StaffMailer.
func (s SMTP) SendStaffMail(ctx context.Context, m port.StaffMail) error {
	to, err := mail.ParseAddress(m.To)
	if err != nil || strings.ContainsAny(m.To, "\r\n") {
		return errors.New("mail: invalid recipient")
	}
	subject, body, err := s.RenderStaffMail(m)
	if err != nil {
		return err
	}
	msg, err := message(s.cfg.From, to.Address, subject, body)
	if err != nil {
		return err
	}
	return s.send(ctx, to.Address, msg)
}

var _ port.StaffMailer = SMTP{}
