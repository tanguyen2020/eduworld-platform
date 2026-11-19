package contractv1client

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
)

// SecretCacheTTL bounds how long a secret read from the directory is reused (OQ F017-3: cache ≤ 5 minutes), so a
// rotated Swarm secret is picked up without a restart.
const SecretCacheTTL = 5 * time.Minute

// FileSecrets implements port.SecretStore over a directory of secret files (EW_SIS_SECRETS_DIR/<secret_ref>, the
// Swarm secrets of the worker; OQ F017-3 [Proposed]). A reference is a relative path of [A-Za-z0-9._/-] that never
// leaves the directory. Values are never logged; errors carry the reference only.
type FileSecrets struct {
	Dir string
	Now func() time.Time // tests; nil = time.Now

	mu    sync.Mutex
	cache map[string]cachedSecret
}

type cachedSecret struct {
	value   []byte
	expires time.Time
}

var _ port.SecretStore = (*FileSecrets)(nil)

// ErrSecretRef: the reference is empty, absolute, escapes the directory or has characters outside the allowlist.
var ErrSecretRef = errors.New("contractv1client: invalid secret reference")

func validRef(ref string) bool {
	if ref == "" || len(ref) > 255 || strings.HasPrefix(ref, "/") || strings.Contains(ref, "..") {
		return false
	}
	for _, c := range ref {
		ok := c == '.' || c == '_' || c == '-' || c == '/' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		if !ok {
			return false
		}
	}
	return true
}

// Get implements port.SecretStore.
func (s *FileSecrets) Get(_ context.Context, ref string) ([]byte, error) {
	if s.Dir == "" {
		return nil, errors.New("contractv1client: EW_SIS_SECRETS_DIR is not set")
	}
	if !validRef(ref) {
		return nil, ErrSecretRef
	}
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.cache[ref]; ok && now().Before(c.expires) {
		return append([]byte(nil), c.value...), nil
	}
	path := filepath.Join(s.Dir, filepath.FromSlash(ref))
	if rel, err := filepath.Rel(s.Dir, path); err != nil || strings.HasPrefix(rel, "..") {
		return nil, ErrSecretRef
	}
	b, err := os.ReadFile(path) //nolint:gosec // path is confined to the secrets directory above
	if err != nil {
		return nil, fmt.Errorf("contractv1client: secret %s: %w", ref, errors.Unwrap(err))
	}
	v := []byte(strings.TrimRight(string(b), "\r\n"))
	if s.cache == nil {
		s.cache = map[string]cachedSecret{}
	}
	s.cache[ref] = cachedSecret{value: v, expires: now().Add(SecretCacheTTL)}
	return append([]byte(nil), v...), nil
}
