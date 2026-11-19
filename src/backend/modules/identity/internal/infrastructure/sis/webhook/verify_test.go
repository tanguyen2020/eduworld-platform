package webhook_test

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/contract"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/webhook"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/tests/fakesis"
)

func batch(t *testing.T, changes int) []byte {
	t.Helper()
	var cs []map[string]any
	for i := range changes {
		cs = append(cs, map[string]any{"sequence": i, "entity_kind": "student", "operation": "upsert", "external_id": "HS-" + strconv.Itoa(i),
			"occurred_at": fakesis.T, "record": fakesis.Student("HS-"+strconv.Itoa(i), "TH-DEMO-001", "A", "An", "hs.x", "active")})
	}
	b, err := json.Marshal(map[string]any{"delivery_id": "0193a5f4-6c1e-7b2a-9d1f-0000000000d1", "contract_version": "1.2.1", "changes": cs})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// F017-AC-18: HMAC-SHA256 over timestamp + "." + raw body, constant-time compare, timestamp within ±5 min;
// every failure is the same error (no reason disclosed). F017-AC-19: the batch is validated.
func TestVerify(t *testing.T) {
	v, err := contract.New()
	if err != nil {
		t.Fatal(err)
	}
	ver := webhook.Verifier{Validator: v}
	secret := []byte("whsec-test")
	now := time.Unix(1_800_000_000, 0)
	ts := strconv.FormatInt(now.Unix(), 10)
	body := batch(t, 2)
	b, err := ver.Verify(secret, webhook.Sign(secret, ts, body), ts, body, now)
	if err != nil || b.DeliveryID.String() != "0193a5f4-6c1e-7b2a-9d1f-0000000000d1" || len(b.Changes) != 2 {
		t.Fatalf("valid delivery: %+v %v", b, err)
	}
	old := strconv.FormatInt(now.Add(-6*time.Minute).Unix(), 10)
	for name, tc := range map[string]struct {
		sig, ts string
		body    []byte
		secret  []byte
	}{
		"wrong secret":    {webhook.Sign([]byte("other"), ts, body), ts, body, secret},
		"tampered body":   {webhook.Sign(secret, ts, body), ts, append([]byte(" "), body...), secret},
		"stale timestamp": {webhook.Sign(secret, old, body), old, body, secret},
		"future":          {webhook.Sign(secret, strconv.FormatInt(now.Add(6*time.Minute).Unix(), 10), body), strconv.FormatInt(now.Add(6*time.Minute).Unix(), 10), body, secret},
		"no prefix":       {strings.TrimPrefix(webhook.Sign(secret, ts, body), "sha256="), ts, body, secret},
		"bad timestamp":   {webhook.Sign(secret, "17", body), "17", body, secret},
		"no secret":       {webhook.Sign(nil, ts, body), ts, body, nil},
	} {
		if _, err := ver.Verify(tc.secret, tc.sig, tc.ts, tc.body, now); !errors.Is(err, port.ErrWebhookSignature) {
			t.Errorf("%s: %v", name, err)
		}
	}
	for name, raw := range map[string]string{
		"major 2":       `{"delivery_id":"0193a5f4-6c1e-7b2a-9d1f-0000000000d1","contract_version":"2.0.0","changes":[]}`,
		"empty":         `{"delivery_id":"0193a5f4-6c1e-7b2a-9d1f-0000000000d1","contract_version":"1.2.1","changes":[]}`,
		"no delivery":   `{"contract_version":"1.2.1","changes":[{}]}`,
		"unknown field": `{"delivery_id":"0193a5f4-6c1e-7b2a-9d1f-0000000000d1","contract_version":"1.2.1","changes":[{}],"x":1}`,
	} {
		if _, err := ver.Verify(secret, webhook.Sign(secret, ts, []byte(raw)), ts, []byte(raw), now); !errors.Is(err, port.ErrWebhookPayload) {
			t.Errorf("%s: %v", name, err)
		}
	}
	big := batch(t, webhook.MaxChanges+1)
	if _, err := ver.Verify(secret, webhook.Sign(secret, ts, big), ts, big, now); !errors.Is(err, port.ErrWebhookPayload) {
		t.Errorf("more than maxItems: %v", err)
	}
}
