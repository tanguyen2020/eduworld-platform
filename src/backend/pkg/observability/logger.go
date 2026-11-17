// Package observability provides the JSON slog logger with redaction, OpenTelemetry tracing and the
// standard HTTP metrics of every binary (F002 T044, BACKEND_DESIGN §11, MONITORING §2, §8).
package observability

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

// LoggerOptions are the constant fields of every log line.
type LoggerOptions struct {
	Binary  string
	Profile string // worker profile, empty for other binaries
	Env     string
	Version string
	Level   slog.Leveler
	Text    bool // EW_LOG_FORMAT=text for local runs; redaction applies the same way
}

// Redacted replaces a sensitive value.
const Redacted = "[REDACTED]"

// sensitiveKeys are attribute keys whose value is never logged (MONITORING §8, AGENTS R14).
var sensitiveKeys = map[string]bool{
	"authorization": true, "cookie": true, "set-cookie": true, "headers": true, "header": true,
	"password": true, "passwd": true, "pin": true, "secret": true, "client_secret": true,
	"token": true, "access_token": true, "refresh_token": true, "id_token": true, "ticket": true, "otp": true, "totp": true,
	"email": true, "phone": true, "full_name": true, "first_name": true, "last_name": true, "given_name": true,
	"family_name": true, "dob": true, "date_of_birth": true, "birth_date": true,
	"answer": true, "answers": true, "response_text": true, "content": true, "chat_content": true, "message_content": true,
	"body": true, "ip": true, "ip_address": true, "remote_addr": true, "x-forwarded-for": true, "dsn": true,
}

var valuePatterns = []struct {
	re   *regexp.Regexp
	repl string
}{
	{regexp.MustCompile(`eyJ[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]*`), "[REDACTED_JWT]"},
	{regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._~+/=-]+`), "Bearer [REDACTED]"},
	{regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`), "[REDACTED_EMAIL]"},
	{regexp.MustCompile(`://[^/\s:@]+:[^/\s@]+@`), "://[REDACTED]@"},
	{regexp.MustCompile(`\b(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})\b`), "[REDACTED_IP]"},
}

// RedactString removes tokens, emails, URL credentials and IPv4 addresses from free text.
func RedactString(s string) string {
	for _, p := range valuePatterns {
		s = p.re.ReplaceAllString(s, p.repl)
	}
	return s
}

func redactAttr(_ []string, a slog.Attr) slog.Attr {
	if a.Key == slog.TimeKey {
		a.Key = "ts"
		return a
	}
	if sensitiveKeys[strings.ToLower(a.Key)] {
		return slog.String(a.Key, Redacted)
	}
	v := a.Value.Resolve()
	switch v.Kind() {
	case slog.KindString:
		return slog.String(a.Key, RedactString(v.String()))
	case slog.KindAny:
		s := fmt.Sprint(v.Any())
		if r := RedactString(s); r != s {
			return slog.String(a.Key, r)
		}
	}
	return a
}

// NewLogger returns the JSON logger of a binary. Every attribute passes redaction; request fields
// (request_id, trace_id, span_id, user_id, tenant_id, module) are added from the context.
func NewLogger(w io.Writer, o LoggerOptions) *slog.Logger {
	if o.Level == nil {
		o.Level = slog.LevelInfo
	}
	hopts := &slog.HandlerOptions{Level: o.Level, ReplaceAttr: redactAttr}
	var h slog.Handler = slog.NewJSONHandler(w, hopts)
	if o.Text {
		h = slog.NewTextHandler(w, hopts)
	}
	attrs := []slog.Attr{slog.String("binary", o.Binary), slog.String("env", o.Env), slog.String("version", o.Version)}
	if o.Profile != "" {
		attrs = append(attrs, slog.String("profile", o.Profile))
	}
	return slog.New(&ctxHandler{inner: h.WithAttrs(attrs)})
}

// ctxHandler adds request fields from the context.
type ctxHandler struct {
	inner     slog.Handler
	hasModule bool
}

func (h *ctxHandler) Enabled(ctx context.Context, l slog.Level) bool { return h.inner.Enabled(ctx, l) }

func (h *ctxHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(slog.String("trace_id", sc.TraceID().String()), slog.String("span_id", sc.SpanID().String()))
	}
	if info, ok := ctx.Value(infoKey{}).(*RequestInfo); ok {
		s := info.snapshot()
		add := func(k, v string) {
			if v != "" {
				r.AddAttrs(slog.String(k, v))
			}
		}
		add("request_id", s.requestID)
		add("user_id", s.userID)
		add("tenant_id", s.tenantID)
		if !h.hasModule {
			add("module", s.module)
		}
	}
	return h.inner.Handle(ctx, r)
}

func (h *ctxHandler) WithAttrs(as []slog.Attr) slog.Handler {
	has := h.hasModule
	for _, a := range as {
		if a.Key == "module" {
			has = true
		}
	}
	return &ctxHandler{inner: h.inner.WithAttrs(as), hasModule: has}
}

func (h *ctxHandler) WithGroup(name string) slog.Handler {
	return &ctxHandler{inner: h.inner.WithGroup(name), hasModule: h.hasModule}
}
