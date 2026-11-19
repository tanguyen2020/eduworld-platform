package mail

import (
	"bufio"
	"context"
	"io"
	"mime/quotedprintable"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/config"
)

// fakeSMTP is a TEST FAKE SMTP server (like mailpit: no TLS, no auth) recording one message per session.
type fakeSMTP struct {
	ln   net.Listener
	mu   sync.Mutex
	msgs []string
	rcpt []string
	code string // reply to RCPT (default 250)
}

func newFakeSMTP(t *testing.T) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeSMTP{ln: ln}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(c)
		}
	}()
	return f
}

func (f *fakeSMTP) serve(c net.Conn) {
	defer func() { _ = c.Close() }()
	r := bufio.NewReader(c)
	w := func(s string) { _, _ = io.WriteString(c, s+"\r\n") }
	w("220 fake ESMTP")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
			w("250-fake")
			w("250 8BITMIME")
		case strings.HasPrefix(cmd, "MAIL FROM"):
			w("250 ok")
		case strings.HasPrefix(cmd, "RCPT TO"):
			f.mu.Lock()
			f.rcpt = append(f.rcpt, strings.TrimSpace(line[len("RCPT TO:"):]))
			code := f.code
			f.mu.Unlock()
			if code != "" {
				w(code + " 5.1.1 <" + strings.Trim(strings.TrimSpace(line[len("RCPT TO:"):]), "<>") + ">: rejected")
				continue
			}
			w("250 ok")
		case cmd == "DATA":
			w("354 go")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil || l == ".\r\n" {
					break
				}
				b.WriteString(l)
			}
			f.mu.Lock()
			f.msgs = append(f.msgs, b.String())
			f.mu.Unlock()
			w("250 queued")
		case cmd == "QUIT":
			w("221 bye")
			return
		default:
			w("250 ok")
		}
	}
}

func newSMTP(t *testing.T, f *fakeSMTP) SMTP {
	t.Helper()
	host, p, _ := net.SplitHostPort(f.ln.Addr().String())
	var port int
	for _, ch := range p {
		port = port*10 + int(ch-'0')
	}
	s, err := New(Config{Host: host, Port: port, From: "no-reply@eduworld.local", Timeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// F012-AC-36: the reset email holds the link and nothing else (no username, no name), per language.
func TestSendPasswordReset(t *testing.T) {
	f := newFakeSMTP(t)
	s := newSMTP(t, f)
	link := "https://portal.eduworld.test/reset-password?token=AbC-123_xyz"
	if err := s.SendPasswordReset(context.Background(), port.PasswordResetMail{To: "ad.truong.a@school.test", Locale: "vi-VN", Link: link}); err != nil {
		t.Fatal(err)
	}
	if len(f.msgs) != 1 || f.rcpt[0] != "<ad.truong.a@school.test>" {
		t.Fatalf("rcpt = %v msgs=%d", f.rcpt, len(f.msgs))
	}
	head, body, _ := strings.Cut(f.msgs[0], "\r\n\r\n")
	dec, _ := io.ReadAll(quotedprintable.NewReader(strings.NewReader(body)))
	text := string(dec)
	if !strings.Contains(head, "Subject: =?utf-8?q?") || !strings.Contains(text, link) || !strings.Contains(text, "đặt lại mật khẩu") {
		t.Fatalf("message = %q / %q", head, text)
	}
	lines := strings.Split(strings.TrimSpace(text), "\r\n")
	if len(lines) != 3 || lines[2] != link {
		t.Fatalf("body must be one line and the link: %q", lines)
	}
	if err := s.SendPasswordReset(context.Background(), port.PasswordResetMail{To: "a@b.test", Locale: "en-US", Link: link}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.msgs[1], "reset your Admin Portal password") {
		t.Fatalf("english = %q", f.msgs[1])
	}
}

func TestSendPasswordResetRejections(t *testing.T) {
	f := newFakeSMTP(t)
	s := newSMTP(t, f)
	ctx := context.Background()
	for _, m := range []port.PasswordResetMail{
		{To: "a@b.test\r\nBcc: x@y.test", Link: "https://p/x"},
		{To: "not an address", Link: "https://p/x"},
		{To: "a@b.test", Link: "https://p/x\r\nBcc: x"},
	} {
		if err := s.SendPasswordReset(ctx, m); err == nil {
			t.Fatalf("header injection accepted: %+v", m)
		}
	}
	// Server replies may echo the recipient: the error keeps the code only (AGENTS R14).
	f.code = "550"
	err := s.SendPasswordReset(ctx, port.PasswordResetMail{To: "secret.person@school.test", Link: "https://p/x"})
	if err == nil || strings.Contains(err.Error(), "secret.person") || !strings.Contains(err.Error(), "550") {
		t.Fatalf("err = %v", err)
	}
	// AUTH is never sent in clear text.
	s.cfg.Username, s.cfg.Password = "u", "p"
	f.code = ""
	if err := s.SendPasswordReset(ctx, port.PasswordResetMail{To: "a@b.test", Link: "https://p/x"}); err == nil || !strings.Contains(err.Error(), "without TLS") {
		t.Fatalf("auth without TLS: %v", err)
	}
}

func TestConfigFrom(t *testing.T) {
	t.Setenv("EW_SMTP_HOST", "localhost")
	t.Setenv("EW_SMTP_PORT", "1025")
	t.Setenv("EW_SMTP_FROM", "no-reply@eduworld.local")
	c, err := ConfigFrom(config.Load())
	if err != nil || c.Port != 1025 || c.Host != "localhost" {
		t.Fatalf("config = %+v %v", c, err)
	}
	t.Setenv("EW_SMTP_PORT", "x")
	if _, err := ConfigFrom(config.Load()); err == nil {
		t.Fatal("bad port accepted")
	}
}
