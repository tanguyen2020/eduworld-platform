// Package webhook verifies SIS webhooks (receiveSisChanges of the EduWorld SIS Contract, F017-AC-18,
// AC-19): X-EduWorld-Signature = "sha256=" + hex(HMAC-SHA256(secret, timestamp + "." + raw_body)) compared in
// constant time, X-EduWorld-Timestamp within sis_webhook_skew, then the ChangeBatch validated against the
// contract (delivery_id uuid, contract_version major 1, 1–500 changes, every Change and record).
package webhook

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/contract"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/contractv1client"
)

// Limits of a ChangeBatch (contract) and of the request body.
const (
	DefaultSkew = 5 * time.Minute // sis_webhook_skew
	MaxChanges  = 500
	MaxBody     = 8 << 20
)

var (
	timestampRe = regexp.MustCompile(`^[0-9]{10}$`)
	versionRe   = regexp.MustCompile(`^1\.[0-9]+\.[0-9]+$`)
)

// Verifier implements port.WebhookVerifier.
type Verifier struct {
	Validator *contract.Validator
	Skew      time.Duration
}

var _ port.WebhookVerifier = Verifier{}

// Sign returns the signature header of body (SIS side; tests and the conformance kit).
func Sign(secret []byte, timestamp string, body []byte) string {
	m := hmac.New(sha256.New, secret)
	m.Write([]byte(timestamp))
	m.Write([]byte("."))
	m.Write(body)
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

// Verify checks the signature and the timestamp (port.ErrWebhookSignature, no detail) and parses the batch
// (port.ErrWebhookPayload).
func (v Verifier) Verify(secret []byte, signature, timestamp string, body []byte, now time.Time) (port.WebhookBatch, error) {
	skew := v.Skew
	if skew <= 0 {
		skew = DefaultSkew
	}
	if len(secret) == 0 || !timestampRe.MatchString(timestamp) || len(signature) > 512 {
		return port.WebhookBatch{}, port.ErrWebhookSignature
	}
	sec, _ := strconv.ParseInt(timestamp, 10, 64)
	if d := now.Sub(time.Unix(sec, 0)); d > skew || d < -skew {
		return port.WebhookBatch{}, port.ErrWebhookSignature
	}
	got, ok := strings.CutPrefix(signature, "sha256=")
	gotRaw, err := hex.DecodeString(got)
	want, _ := hex.DecodeString(strings.TrimPrefix(Sign(secret, timestamp, body), "sha256="))
	if !ok || err != nil || !hmac.Equal(gotRaw, want) {
		return port.WebhookBatch{}, port.ErrWebhookSignature
	}
	return Parse(v.Validator, body)
}

// Parse validates a ChangeBatch body.
func Parse(v *contract.Validator, body []byte) (port.WebhookBatch, error) {
	var raw struct {
		DeliveryID      string            `json:"delivery_id"`
		ContractVersion string            `json:"contract_version"`
		Changes         *[]map[string]any `json:"changes"`
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields() // ChangeBatch has additionalProperties: false
	if err := dec.Decode(&raw); err != nil {
		return port.WebhookBatch{}, fmt.Errorf("%w: body", port.ErrWebhookPayload)
	}
	id, err := uuid.Parse(raw.DeliveryID)
	if err != nil {
		return port.WebhookBatch{}, fmt.Errorf("%w: delivery_id", port.ErrWebhookPayload)
	}
	if !versionRe.MatchString(raw.ContractVersion) || len(raw.ContractVersion) > 64 {
		return port.WebhookBatch{}, fmt.Errorf("%w: contract_version", port.ErrWebhookPayload)
	}
	if raw.Changes == nil || len(*raw.Changes) == 0 || len(*raw.Changes) > MaxChanges {
		return port.WebhookBatch{}, fmt.Errorf("%w: changes", port.ErrWebhookPayload)
	}
	b := port.WebhookBatch{DeliveryID: id, ContractVersion: raw.ContractVersion}
	b.Changes, b.Violations, b.UnknownFields = contractv1client.ParseChanges(v, *raw.Changes)
	return b, nil
}
