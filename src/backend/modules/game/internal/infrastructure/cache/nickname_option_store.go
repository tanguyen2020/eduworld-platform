// Package cache holds the Valkey adapters of module game for temporary data (ADR-0004): nothing here is the source
// of truth; a lost key only means the client asks again.
package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
)

// NicknameOptionStore implements port.NicknameOptionStore (VALKEY_DESIGN, F070 data-model): one hash per student
// ew:game:nick:opt:<student_id> (option id → nickname) with the TTL nickname_suggestion_ttl. An option of another
// student is never found (the key is the student's own).
type NicknameOptionStore struct {
	C       valkey.Client
	Timeout time.Duration // per call (default 1 s)
}

var _ port.NicknameOptionStore = NicknameOptionStore{}

// Key is the hash of the options of student.
func Key(student uuid.UUID) string { return "ew:game:nick:opt:" + student.String() }

func (s NicknameOptionStore) ctx(ctx context.Context) (context.Context, context.CancelFunc) {
	t := s.Timeout
	if t <= 0 {
		t = time.Second
	}
	return context.WithTimeout(ctx, t)
}

// Save implements port.NicknameOptionStore.
func (s NicknameOptionStore) Save(ctx context.Context, student uuid.UUID, options map[uuid.UUID]string, ttl time.Duration) error {
	if len(options) == 0 {
		return nil
	}
	if ttl < time.Second {
		ttl = time.Second
	}
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	key := Key(student)
	hset := s.C.B().Hset().Key(key).FieldValue()
	for id, n := range options {
		hset = hset.FieldValue(id.String(), n)
	}
	for _, r := range s.C.DoMulti(ctx, hset.Build(), s.C.B().Expire().Key(key).Seconds(int64(ttl/time.Second)).Build()) {
		if err := r.Error(); err != nil {
			return fmt.Errorf("nickname options: save: %w", err)
		}
	}
	return nil
}

// Get implements port.NicknameOptionStore.
func (s NicknameOptionStore) Get(ctx context.Context, student, option uuid.UUID) (string, bool, error) {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	n, err := s.C.Do(ctx, s.C.B().Hget().Key(Key(student)).Field(option.String()).Build()).ToString()
	if errors.Is(err, valkey.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("nickname options: get: %w", err)
	}
	return n, true, nil
}

// Clear implements port.NicknameOptionStore.
func (s NicknameOptionStore) Clear(ctx context.Context, student uuid.UUID) error {
	ctx, cancel := s.ctx(ctx)
	defer cancel()
	if err := s.C.Do(ctx, s.C.B().Unlink().Key(Key(student)).Build()).Error(); err != nil {
		return fmt.Errorf("nickname options: clear: %w", err)
	}
	return nil
}
