package port

import "context"

// StaffMail is one email to a staff member for the Platform Safety Team (F124): the adapter renders the template of the
// locale from the translation files with the allowed parameters (ids, instants), the school name and the portal link —
// never a student name, a case summary, a category or chat content (R14, F124-AC-09). The address is read when sending.
type StaffMail struct {
	To         string
	Locale     string
	Template   string
	Params     map[string]string
	SchoolName string // school templates only
	Link       string // portal link (EW_PORTAL_BASE_URL + route of the portal catalog)
}

// StaffMailer sends the staff emails (EW_SMTP_*; local: mailpit). Errors carry an SMTP reply code at most, never the
// address (R14).
type StaffMailer interface {
	SendStaffMail(ctx context.Context, m StaffMail) error
}
