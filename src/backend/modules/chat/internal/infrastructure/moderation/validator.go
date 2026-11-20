// Package moderation wires the ContentValidator of the send path (F115 port, F116 implementation). Until F116 provides
// pipeline.Validator there is no real validator: module chat refuses to start outside EW_ENV=test (NFR-SAFE-001,
// F115-AC-06) and EW_ENV=test gets the labelled test double below.
package moderation

import (
	"context"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
)

// Real returns the real ContentValidator (F116 pipeline.Validator); nil until F116 wires it.
func Real() port.ContentValidator { return nil }

// TestDoubleValidator is a TEST DOUBLE (R17) — NOT a moderation pipeline. It runs no check and answers "unchecked"
// with the text unchanged, so the send path can run in EW_ENV=test only; command.RequireValidator refuses it anywhere
// else.
type TestDoubleValidator struct{}

var _ port.TestDouble = TestDoubleValidator{}

// TestDouble marks the test double.
func (TestDoubleValidator) TestDouble() bool { return true }

// Validate implements port.ContentValidator: decision unchecked, every step reported as skipped.
func (TestDoubleValidator) Validate(_ context.Context, in port.ValidationInput) (port.Verdict, error) {
	return port.Verdict{Decision: message.Unchecked, Display: in.Text, StepsSkipped: []string{"test_double"}}, nil
}

// For returns the validator of the environment: the real one, else (EW_ENV=test only) the test double; nil otherwise
// (command.RequireValidator then refuses to start).
func For(env string) port.ContentValidator {
	if v := Real(); v != nil {
		return v
	}
	if env == "test" {
		return TestDoubleValidator{}
	}
	return nil
}
