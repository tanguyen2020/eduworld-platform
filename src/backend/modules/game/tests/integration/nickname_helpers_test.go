//go:build integration

package integration

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// placeholderTerm is the only "sensitive" word of the tests: a placeholder, never a real word (R15). The chat pipeline
// itself (catalogue, PII) is covered by the callee-side facade test of module chat.
const placeholderTerm = "tu cam mau"

// errNickDown is the failure of a TEST DOUBLE facade (fail-closed path, F136-AC-14).
var errNickDown = errors.New("TEST DOUBLE: facade down")

// memNickModeration is a TEST DOUBLE (R17) of port.NicknameModeration (moderateDisplayText, context nickname): blocks a
// text holding the placeholder term; down = every call fails.
type memNickModeration struct {
	mu    sync.Mutex
	calls int
	down  bool
}

func (m *memNickModeration) CheckNickname(_ context.Context, _, _ uuid.UUID, text string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.down {
		return false, errNickDown
	}
	return !strings.Contains(strings.ToLower(text), placeholderTerm), nil
}

func (m *memNickModeration) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

// memNickPersonal is a TEST DOUBLE (R17) of port.NicknamePersonalDataScreen (screenNicknamePersonalData): blocks the
// synthetic "real names" registered per student.
type memNickPersonal struct {
	mu      sync.Mutex
	blocked map[uuid.UUID][]string
	calls   int
	down    bool
}

func (m *memNickPersonal) ScreenNickname(_ context.Context, _, student uuid.UUID, text string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.down {
		return false, errNickDown
	}
	for _, n := range m.blocked[student] {
		if strings.EqualFold(n, text) {
			return false, nil
		}
	}
	return true, nil
}

// memNickAttempts is a TEST DOUBLE (R17) of port.NicknameAttemptCounter on the test clock (sliding window); the Valkey
// adapter has its own test (TestNicknameAttemptCounterValkey).
type memNickAttempts struct {
	mu     sync.Mutex
	now    func() time.Time
	limit  int
	window time.Duration
	hits   map[uuid.UUID][]time.Time
}

func (m *memNickAttempts) Hit(_ context.Context, _, student uuid.UUID, now time.Time) (bool, time.Duration, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var kept []time.Time
	for _, h := range m.hits[student] {
		if now.Sub(h) < m.window {
			kept = append(kept, h)
		}
	}
	kept = append(kept, now)
	m.hits[student] = kept
	if len(kept) > m.limit {
		return false, kept[len(kept)-m.limit-1].Add(m.window).Sub(now), nil
	}
	return true, 0, nil
}

// typeNickname sends a typed nickname for s (updateGameProfile {nickname}).
func (e *env) typeNickname(t *testing.T, s student, nickname string, lang ...string) resp {
	t.Helper()
	h := th(s)
	h["Content-Type"] = "application/merge-patch+json"
	if len(lang) > 0 {
		h["Accept-Language"] = lang[0]
	}
	return e.do(t, http.MethodPatch, "/game/profile", e.gtoken(s), h, map[string]any{"nickname": nickname})
}
