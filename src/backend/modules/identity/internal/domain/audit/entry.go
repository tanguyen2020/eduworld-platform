// Package audit holds the rules of the central audit log of module identity (F015 US1, FR-AUD-001,
// FR-AUD-002): validation of an entry received from any module and the second-layer sanitizing of its
// metadata (the producer already filters with outbox.AuditRecorder). Standard library and uuid only.
package audit

import (
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Entry is one row of audit.audit_logs.
type Entry struct {
	ID         uuid.UUID // event_id of audit.recorded
	OccurredAt time.Time
	ActorID    *uuid.UUID
	ActorType  string // user | service | system
	TenantID   *uuid.UUID
	SchoolID   *uuid.UUID
	Action     string
	TargetType string
	TargetID   string
	Result     string // success | denied | error
	Reason     string
	Metadata   map[string]any
	TraceID    string
	Producer   string
}

// Limits of the sink (DATABASE_DESIGN §3.11, payload audit.recorded v1).
const (
	MaxMetadataKeys  = 50
	MaxMetadataValue = 255
	MaxMetadataBytes = 16384
	MaxReason        = 1000
)

// forbidden are metadata keys never stored (F015 data-model §3, AGENTS R14): credentials, chat content, PII.
var forbidden = map[string]bool{
	"password": true, "passwd": true, "new_password": true, "pin": true, "otp": true, "secret": true, "client_secret": true,
	"token": true, "refresh_token": true, "access_token": true, "id_token": true, "login_ticket": true, "setup_ticket": true,
	"reset_token": true, "body": true, "message": true, "content": true, "chat_content": true, "message_content": true,
	"full_name": true, "first_name": true, "last_name": true, "date_of_birth": true, "dob": true, "birth_date": true,
	"email": true, "phone": true, "username": true, "ip": true, "ip_address": true, "answer": true, "answers": true,
}

var (
	actionRe = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)
	keyRe    = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
)

// ErrInvalid is a structurally invalid entry (the event goes to the dead letters).
var ErrInvalid = errors.New("audit: invalid entry")

// Validate checks the columns of the entry.
func (e Entry) Validate() error {
	switch {
	case e.ID == uuid.Nil, e.OccurredAt.IsZero():
		return ErrInvalid
	case e.ActorType != "user" && e.ActorType != "service" && e.ActorType != "system":
		return ErrInvalid
	case !actionRe.MatchString(e.Action) || len(e.Action) > 64:
		return ErrInvalid
	case e.TargetType == "" || len(e.TargetType) > 64 || e.TargetID == "" || len(e.TargetID) > 128:
		return ErrInvalid
	case e.Result != "success" && e.Result != "denied" && e.Result != "error":
		return ErrInvalid
	case len(e.TraceID) > 64 || e.Producer == "" || len(e.Producer) > 64:
		return ErrInvalid
	}
	return nil
}

// Sanitize returns metadata without forbidden or malformed keys, with string values bounded to
// MaxMetadataValue, at most MaxMetadataKeys keys (sorted, extra keys dropped) and at most MaxMetadataBytes of
// JSON; removed counts the dropped or truncated fields (F015-AC-04). Nested objects are not kept.
func Sanitize(m map[string]any) (clean map[string]any, removed int) {
	clean = map[string]any{}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := m[k]
		if forbidden[strings.ToLower(k)] || !keyRe.MatchString(k) || len(clean) >= MaxMetadataKeys {
			removed++
			continue
		}
		switch x := v.(type) {
		case string:
			if len(x) > MaxMetadataValue {
				x, removed = x[:MaxMetadataValue], removed+1
			}
			clean[k] = x
		case bool, float64, int, int64, nil:
			clean[k] = x
		case []any:
			if allStrings(x) && len(x) <= MaxMetadataKeys {
				clean[k] = x
			} else {
				removed++
			}
		default:
			removed++ // nested objects may hide forbidden keys
		}
	}
	for {
		raw, err := json.Marshal(clean)
		if err == nil && len(raw) <= MaxMetadataBytes {
			return clean, removed
		}
		// Drop the largest field until the document fits.
		var big string
		size := -1
		for k, v := range clean {
			b, _ := json.Marshal(v)
			if len(b) > size {
				big, size = k, len(b)
			}
		}
		delete(clean, big)
		removed++
	}
}

func allStrings(xs []any) bool {
	for _, x := range xs {
		s, ok := x.(string)
		if !ok || len(s) > MaxMetadataValue {
			return false
		}
	}
	return true
}

// MaxQueryRange bounds the from–to window of an audit query (audit_query_max_range [Proposed] 93 days).
const MaxQueryRange = 93 * 24 * time.Hour

// ErrRange: from/to missing, inverted or wider than the maximum.
var ErrRange = errors.New("audit: invalid time range")

// CheckRange validates a query window (F015-AC-10).
func CheckRange(from, to time.Time, maxRange time.Duration) error {
	if maxRange <= 0 {
		maxRange = MaxQueryRange
	}
	if from.IsZero() || to.IsZero() || !to.After(from) || to.Sub(from) > maxRange {
		return ErrRange
	}
	return nil
}
