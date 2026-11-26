package write

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// SubjectCodes resolves subject codes of the platform catalog (curriculum.subjects: global table, code immutable) for the
// subject label of the learning metrics (MONITORING §3). A code is read once per subject and process, in the transaction
// of ctx; the cache only holds subjects that exist, so its size is bounded by the catalog.
type SubjectCodes struct{ cache *sync.Map }

// NewSubjectCodes builds the resolver with an empty cache.
func NewSubjectCodes() SubjectCodes { return SubjectCodes{cache: &sync.Map{}} }

// SubjectCode implements command.SubjectCodes.
func (s SubjectCodes) SubjectCode(ctx context.Context, id uuid.UUID) (string, error) {
	if s.cache != nil {
		if v, ok := s.cache.Load(id); ok {
			c, _ := v.(string)
			return c, nil
		}
	}
	r, err := CatalogRepo{}.Subject(ctx, id)
	if err != nil {
		return "", err
	}
	if s.cache != nil {
		s.cache.Store(id, r.Code)
	}
	return r.Code, nil
}
