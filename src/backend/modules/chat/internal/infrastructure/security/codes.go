// Package security holds the random friend code generator of module chat.
package security

import (
	"crypto/rand"
	"math/big"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/social"
)

// Codes implements port.CodeGenerator with crypto/rand (uniform over social.CodeAlphabet).
type Codes struct{}

var _ port.CodeGenerator = Codes{}

var alphabetSize = big.NewInt(int64(len(social.CodeAlphabet)))

// NewCode implements port.CodeGenerator.
func (Codes) NewCode() (string, error) {
	b := make([]byte, social.CodeLength)
	for i := range b {
		n, err := rand.Int(rand.Reader, alphabetSize)
		if err != nil {
			return "", err
		}
		b[i] = social.CodeAlphabet[n.Int64()]
	}
	return string(b), nil
}
