package efficacy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base32"
	"errors"

	"github.com/google/uuid"
)

// ErrNoKey: the pseudonymization key is missing or too short (F035-AC-33: the export fails, nothing is written).
var ErrNoKey = errors.New("efficacy: pseudonymization key missing")

// MinKeyLen is the shortest accepted key (bytes) [Proposed].
const MinKeyLen = 32

// Pseudonymizer computes the codes of the exported dataset (research R3, F035-AC-32, AC-33):
// base32(HMAC-SHA256(key, study_id ‖ id))[:16]. The salt study_id makes the codes stable within a study and
// unlinkable across studies; no mapping table is stored (reversal needs the key). The key is never logged.
type Pseudonymizer struct {
	key []byte
}

// NewPseudonymizer returns ErrNoKey for a missing or short key.
func NewPseudonymizer(key []byte) (Pseudonymizer, error) {
	if len(key) < MinKeyLen {
		return Pseudonymizer{}, ErrNoKey
	}
	return Pseudonymizer{key: append([]byte(nil), key...)}, nil
}

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// Code is the pseudonymous code of id (a student, class or tenant) in a study.
func (p Pseudonymizer) Code(studyID, id uuid.UUID) string {
	m := hmac.New(sha256.New, p.key)
	m.Write(studyID[:])
	m.Write(id[:])
	return b32.EncodeToString(m.Sum(nil))[:16]
}

// String never shows the key.
func (Pseudonymizer) String() string { return "Pseudonymizer{key:redacted}" }
