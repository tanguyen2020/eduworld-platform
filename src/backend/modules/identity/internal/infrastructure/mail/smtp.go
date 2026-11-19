// Package mail is a MINIMAL SMTP adapter of port.Mailer for the business emails of module identity
// (EW_SMTP_*, LOCAL_DEVELOPMENT §4; local runs deliver to mailpit). It sends one plain-text message per
// call with a timeout, STARTTLS when the server offers it and AUTH PLAIN only over TLS. The shared
// notification infrastructure (outbox, templates by translation files, retries) is outside F012; this
// adapter carries only the password reset email, whose content is the reset link and nothing else
// (ADR-0091 §3). Messages and addresses are never logged.
package mail

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// Config of the SMTP adapter. Nothing is hardcoded (AGENTS R3); the password comes from
// EW_SMTP_PASSWORD(_FILE).
type Config struct {
	Host     string
	Port     int
	From     string
	Username string
	Password string
	Timeout  time.Duration
}

// ConfigFrom reads EW_SMTP_HOST, EW_SMTP_PORT, EW_SMTP_FROM (required) and EW_SMTP_USERNAME/PASSWORD.
func ConfigFrom(c *config.Config) (Config, error) {
	if err := c.Require("EW_SMTP_HOST", "EW_SMTP_PORT", "EW_SMTP_FROM"); err != nil {
		return Config{}, err
	}
	p, err := strconv.Atoi(c.String("EW_SMTP_PORT"))
	if err != nil || p < 1 || p > 65535 {
		return Config{}, errors.New("mail: EW_SMTP_PORT must be a port number")
	}
	if _, err := mail.ParseAddress(c.String("EW_SMTP_FROM")); err != nil {
		return Config{}, errors.New("mail: EW_SMTP_FROM must be an email address")
	}
	return Config{Host: c.String("EW_SMTP_HOST"), Port: p, From: c.String("EW_SMTP_FROM"), Username: c.String("EW_SMTP_USERNAME"),
		Password: c.String("EW_SMTP_PASSWORD"), Timeout: c.Duration("EW_EXT_CLIENT_TIMEOUT", 5*time.Second)}, nil
}

// SMTP implements port.Mailer.
type SMTP struct{ cfg Config }

var _ port.Mailer = SMTP{}

// New builds the adapter.
func New(cfg Config) (SMTP, error) {
	if cfg.Host == "" || cfg.Port == 0 || cfg.From == "" {
		return SMTP{}, errors.New("mail: host, port and from are required")
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	return SMTP{cfg: cfg}, nil
}

// resetText holds the only content of the reset email per language: a subject and one line before the
// link [Proposed] wording, until the shared notification templates exist.
var resetText = map[string][2]string{
	"vi": {"Đặt lại mật khẩu EduWorld", "Mở liên kết sau để đặt lại mật khẩu Admin Portal:"},
	"en": {"Reset your EduWorld password", "Open this link to reset your Admin Portal password:"},
}

func lang(locale string) string {
	if strings.HasPrefix(strings.ToLower(locale), "en") {
		return "en"
	}
	return "vi"
}

// SendPasswordReset implements port.Mailer.
func (s SMTP) SendPasswordReset(ctx context.Context, m port.PasswordResetMail) error {
	to, err := mail.ParseAddress(m.To)
	if err != nil || strings.ContainsAny(m.To, "\r\n") {
		return errors.New("mail: invalid recipient")
	}
	if strings.ContainsAny(m.Link, "\r\n") || !strings.HasPrefix(m.Link, "http") {
		return errors.New("mail: invalid link")
	}
	text := resetText[lang(m.Locale)]
	msg, err := message(s.cfg.From, to.Address, text[0], text[1]+"\r\n\r\n"+m.Link+"\r\n")
	if err != nil {
		return err
	}
	return s.send(ctx, to.Address, msg)
}

func message(from, to, subject, body string) ([]byte, error) {
	var b bytes.Buffer
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", subject) + "\r\n")
	b.WriteString("Date: " + time.Now().UTC().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\n")
	qp := quotedprintable.NewWriter(&b)
	if _, err := qp.Write([]byte(body)); err != nil {
		return nil, err
	}
	if err := qp.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// send delivers one message within the timeout (the whole SMTP dialogue shares one deadline).
func (s SMTP) send(ctx context.Context, to string, msg []byte) error {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.Timeout)
	defer cancel()
	addr := net.JoinHostPort(s.cfg.Host, strconv.Itoa(s.cfg.Port))
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("mail: dial: %w", err)
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	c, err := smtp.NewClient(conn, s.cfg.Host)
	if err != nil {
		_ = conn.Close()
		return smtpErr("hello", err)
	}
	defer func() { _ = c.Close() }()
	tlsOn := false
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: s.cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return smtpErr("starttls", err)
		}
		tlsOn = true
	}
	if s.cfg.Username != "" {
		if !tlsOn {
			return errors.New("mail: refusing AUTH without TLS")
		}
		if err := c.Auth(smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)); err != nil {
			return smtpErr("auth", err)
		}
	}
	from, _ := mail.ParseAddress(s.cfg.From)
	if err := c.Mail(from.Address); err != nil {
		return smtpErr("from", err)
	}
	if err := c.Rcpt(to); err != nil {
		return smtpErr("rcpt", err)
	}
	w, err := c.Data()
	if err != nil {
		return smtpErr("data", err)
	}
	if _, err := w.Write(msg); err != nil {
		return smtpErr("write", err)
	}
	if err := w.Close(); err != nil {
		return smtpErr("end of data", err)
	}
	if err := c.Quit(); err != nil {
		return smtpErr("quit", err)
	}
	return nil
}

// smtpErr keeps the SMTP reply code only: server replies may echo the recipient address, which must not
// reach logs (AGENTS R14).
func smtpErr(stage string, err error) error {
	var te *textproto.Error
	if errors.As(err, &te) {
		return fmt.Errorf("mail: %s: smtp reply %d", stage, te.Code)
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return fmt.Errorf("mail: %s: timeout", stage)
	}
	return fmt.Errorf("mail: %s failed", stage)
}
