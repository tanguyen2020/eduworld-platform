package outbox

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// forbiddenMetadata are keys never sent in audit metadata (F015 data-model §3, AGENTS R14).
var forbiddenMetadata = map[string]bool{
	"password": true, "refresh_token": true, "access_token": true, "login_ticket": true, "body": true,
	"message": true, "full_name": true, "date_of_birth": true, "email": true, "username": true,
	"token": true, "content": true, "answer": true, "ip": true,
}

var metaKeyRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// AuditPayload is the payload of audit.recorded v1.
type AuditPayload struct {
	ActorID    *string           `json:"actor_id"`
	ActorType  string            `json:"actor_type"`
	Action     string            `json:"action"`
	TargetType string            `json:"target_type"`
	TargetID   string            `json:"target_id"`
	Result     string            `json:"result"`
	Reason     *string           `json:"reason"`
	Metadata   map[string]string `json:"metadata"`
	SchoolID   *string           `json:"school_id,omitempty"`
}

// AuditRecorder implements audit.Recorder by writing audit.recorded into the outbox (F003-AC-22). Inside
// a UnitOfWork it joins that transaction; otherwise (bypass audit before a transaction, guards) it
// writes in its own short transaction on the app pool so the audit is durable either way.
type AuditRecorder struct {
	w   *Writer
	app *sql.DB
}

// NewAuditRecorder builds the recorder; app is the <module>_app pool.
func NewAuditRecorder(w *Writer, app *sql.DB) *AuditRecorder { return &AuditRecorder{w: w, app: app} }

// Payload converts an audit event; forbidden metadata keys are dropped, values redacted and bounded.
func Payload(ctx context.Context, e audit.Event) AuditPayload {
	p := AuditPayload{ActorType: "system", Action: e.Action, TargetType: e.TargetType, TargetID: e.TargetID,
		Result: string(e.Result), Metadata: map[string]string{}}
	if pr, ok := authz.PrincipalFrom(ctx); ok {
		id := pr.UserID.String()
		p.ActorID, p.ActorType = &id, "user"
	}
	if p.Result == string(audit.ResultFailure) {
		p.Result = "error"
	}
	if p.TargetType == "" {
		p.TargetType = "none"
	}
	if p.TargetID == "" {
		p.TargetID = "-"
	}
	if _, err := uuid.Parse(e.SchoolID); err == nil {
		s := e.SchoolID
		p.SchoolID = &s
	}
	if e.Reason != "" {
		r := bound(observability.RedactString(e.Reason), 1000)
		p.Reason = &r
	}
	for k, v := range e.Metadata {
		if forbiddenMetadata[k] || !metaKeyRe.MatchString(k) || len(k) > 64 || len(p.Metadata) >= 50 {
			continue
		}
		p.Metadata[k] = bound(observability.RedactString(fmt.Sprint(v)), 255)
	}
	return p
}

func bound(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// Record implements audit.Recorder.
func (r *AuditRecorder) Record(ctx context.Context, e audit.Event) error {
	m := Message{Type: "audit.recorded", Subject: Subject{Type: "audit", ID: bound(e.TargetID, 64)}, Payload: Payload(ctx, e)}
	if m.Subject.ID == "" {
		m.Subject.ID = "-"
	}
	if s, ok := tenancy.ScopeFrom(ctx); ok {
		if id, single := s.SessionTenantID(); single {
			m.TenantID = &id
		} else {
			m.Platform = true
		}
	} else {
		m.Platform = true
	}
	if entx.InWriteTx(ctx) {
		_, err := r.w.Publish(ctx, m)
		return err
	}
	env, err := r.w.Envelope(ctx, m)
	if err != nil {
		return err
	}
	tx, err := r.app.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("outbox: audit tx: %w", err)
	}
	if err := insert(ctx, tx, r.w.Subject(env.EventType), "", env); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
