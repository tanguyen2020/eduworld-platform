// Package secret reads the secrets of module learning from files (Docker secrets): never from an environment variable
// value, never logged (R3, R14).
package secret

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

// ErrMissing: the secret file is not configured or empty.
var ErrMissing = errors.New("secret: missing")

// PseudonymKey reads the key of the pseudonymous codes of the efficacy dataset from EW_EFFICACY_PSEUDONYM_KEY_FILE
// (F035-AC-33): read at each export, so a rotated or removed secret takes effect without restart.
type PseudonymKey struct {
	Path string
}

// Key implements command.PseudonymKeys.
func (k PseudonymKey) Key(context.Context) ([]byte, error) {
	if k.Path == "" {
		return nil, ErrMissing
	}
	b, err := os.ReadFile(k.Path)
	if err != nil {
		return nil, fmt.Errorf("secret: pseudonym key file: %w", ErrMissing)
	}
	key := []byte(strings.TrimSpace(string(b)))
	if len(key) == 0 {
		return nil, ErrMissing
	}
	return key, nil
}
