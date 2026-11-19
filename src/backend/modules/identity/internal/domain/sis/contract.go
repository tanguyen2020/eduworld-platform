package sis

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
)

// Provider kinds of a SIS connection (ADR-0021).
const (
	ProviderContractV1 = "contract_v1"
	ProviderCSVImport  = "csv_import"
)

// Connection statuses and reasons (data-model F017 §2).
const (
	StatusActive        = "active"
	StatusAuthError     = "auth_error"
	StatusContractError = "contract_error"
	StatusPaused        = "paused"

	ReasonVersionMismatch    = "version_mismatch"
	ReasonViolationThreshold = "violation_threshold"
	ReasonCredentialRejected = "credential_rejected"
	ReasonSecretUnavailable  = "secret_unavailable"
	ReasonCircuitOpen        = "circuit_open"
	ReasonManual             = "manual"
)

// Capabilities declared by a facade in Health.capabilities.
const (
	CapFullSync      = "full_sync"
	CapDelta         = "delta"
	CapWebhook       = "webhook"
	CapStudentLookup = "student_lookup"
)

// Auth types of a contract_v1 connection.
const (
	AuthOAuth2 = "oauth2_client_credentials"
	AuthAPIKey = "api_key"
	AuthMTLS   = "mtls"
)

// Run modes, triggers and failure codes of a SyncRun (DATABASE_DESIGN §2.3).
const (
	ModeFull    = "full"
	ModeDelta   = "delta"
	ModeWebhook = "webhook"

	TriggerSchedule     = "schedule"
	TriggerManual       = "manual"
	TriggerWebhook      = "webhook"
	TriggerFallbackFull = "fallback_full"

	FailAuth                = "SIS_AUTH_FAILED"
	FailVersionUnsupported  = "SIS_CONTRACT_VERSION_UNSUPPORTED"
	FailViolationThreshold  = "SIS_CONTRACT_VIOLATION_THRESHOLD"
	FailUnavailable         = "SIS_UNAVAILABLE"
	FailPaginationInvalid   = "SIS_PAGINATION_INVALID"
	FailWatermarkExpired    = "SIS_WATERMARK_EXPIRED"
	FailConnectionNotActive = "SIS_CONNECTION_NOT_ACTIVE"
	CodeContractViolation   = "SIS_CONTRACT_VIOLATION"
)

// Parameters registered in CONFIG_DEFAULTS (code scope).
const (
	// ViolationThreshold is sis_contract_violation_threshold: more than 10% of the records of a run violate
	// the contract → the run stops and the connection goes contract_error (SIS_INTEGRATION §2.5).
	ViolationThreshold = 0.10
	// CircuitThreshold is sis_circuit_breaker: 5 consecutive failed runs pause the schedule (SIS_INTEGRATION §7).
	CircuitThreshold = 5
	// MaxSchools is maxItems of config.schools[].
	MaxSchools = 500
)

// ContractSchool maps a school_external_id of the SIS to a school of the tenant (OQ F017-1).
type ContractSchool struct {
	ExternalID string    `json:"school_external_id"`
	SchoolID   uuid.UUID `json:"school_id"`
}

// ContractConfig is sis_connections.config of a contract_v1 connection: never a secret nor a field mapping.
type ContractConfig struct {
	BaseURL    string           `json:"base_url"`
	AuthType   string           `json:"auth_type"`
	TokenURL   string           `json:"token_url,omitempty"`
	OAuthScope string           `json:"oauth_scope,omitempty"`
	Schools    []ContractSchool `json:"schools"`
}

// FieldError is one invalid field of a connection.
type FieldError struct {
	Field   string
	Message string
}

var (
	externalIDRe = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,64}$`)
	secretRefRe  = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]{0,511}$`)
	versionRe    = regexp.MustCompile(`^1\.[0-9]+\.[0-9]+$`)
)

// ValidSecretRef reports whether ref is a secret manager reference (pattern of SisSecretRef).
func ValidSecretRef(ref string) bool {
	return secretRefRe.MatchString(ref) && !strings.Contains(ref, "..")
}

// ValidateContract checks a contract_v1 connection (F017-AC-01, AC-30): https base URL ≤ 2048 (http only
// when allowHTTP, the dev flag of compose), auth type, token URL for OAuth2, secret references, major 1,
// at most MaxSchools distinct schools, an optional 5-field cron schedule.
func ValidateContract(cfg ContractConfig, secretRef, webhookRef, version, schedule string, allowHTTP bool) []FieldError {
	var out []FieldError
	add := func(f, m string) { out = append(out, FieldError{f, m}) }
	if msg := checkURL(cfg.BaseURL, allowHTTP); msg != "" {
		add("config.base_url", msg)
	}
	switch cfg.AuthType {
	case AuthOAuth2:
		if cfg.TokenURL == "" {
			add("config.token_url", "required for oauth2_client_credentials")
		} else if msg := checkURL(cfg.TokenURL, allowHTTP); msg != "" {
			add("config.token_url", msg)
		}
	case AuthAPIKey, AuthMTLS:
	default:
		add("config.auth_type", "must be oauth2_client_credentials, api_key or mtls")
	}
	if len(cfg.OAuthScope) > 255 {
		add("config.oauth_scope", "at most 255 characters")
	}
	if len(cfg.Schools) > MaxSchools {
		add("config.schools", "at most 500 schools")
	}
	seenExt, seenSchool := map[string]bool{}, map[uuid.UUID]bool{}
	for i, s := range cfg.Schools {
		f := "config.schools[" + strconv.Itoa(i) + "]"
		if !externalIDRe.MatchString(s.ExternalID) {
			add(f+".school_external_id", "1-64 characters [A-Za-z0-9._:-]")
		}
		if s.SchoolID == uuid.Nil {
			add(f+".school_id", "required")
		}
		if seenExt[s.ExternalID] || seenSchool[s.SchoolID] {
			add(f, "duplicate school")
		}
		seenExt[s.ExternalID], seenSchool[s.SchoolID] = true, true
	}
	if !ValidSecretRef(secretRef) {
		add("secret_ref", "required secret manager reference (≤ 512, [a-z0-9._/-])")
	}
	if webhookRef != "" && !ValidSecretRef(webhookRef) {
		add("webhook_secret_ref", "secret manager reference (≤ 512, [a-z0-9._/-])")
	}
	if !versionRe.MatchString(version) || len(version) > 64 {
		add("contract_version", "must be 1.x.y")
	}
	if schedule != "" {
		if _, err := ParseCron(schedule); err != nil || len(schedule) > 64 {
			add("schedule", "5-field cron expression")
		}
	}
	return out
}

func checkURL(raw string, allowHTTP bool) string {
	if raw == "" {
		return "required"
	}
	if len(raw) > 2048 {
		return "at most 2048 characters"
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "absolute URL without credentials, query or fragment"
	}
	if u.Scheme == "https" || (allowHTTP && u.Scheme == "http") {
		return ""
	}
	return "must use https"
}

// MajorOf returns the major of a semantic version.
func MajorOf(v string) (int, bool) {
	head, _, ok := strings.Cut(v, ".")
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(head)
	return n, err == nil
}

// CompatibleMajor reports whether the facade version has the major of the connection (SIS_INTEGRATION §2.4).
func CompatibleMajor(connection, facade string) bool {
	a, ok1 := MajorOf(connection)
	b, ok2 := MajorOf(facade)
	return ok1 && ok2 && a == b
}

// ValidCapabilities keeps the known capabilities, without duplicates, in a stable order.
func ValidCapabilities(in []string) []string {
	out := []string{}
	for _, c := range []string{CapFullSync, CapDelta, CapWebhook, CapStudentLookup} {
		for _, x := range in {
			if x == c {
				out = append(out, c)
				break
			}
		}
	}
	return out
}

// HasCapability reports whether caps declares c.
func HasCapability(caps []string, c string) bool {
	for _, x := range caps {
		if x == c {
			return true
		}
	}
	return false
}

// ViolationRatioExceeded applies sis_contract_violation_threshold to the records of a run.
func ViolationRatioExceeded(violations, total int, threshold float64) bool {
	if total <= 0 {
		return false
	}
	return float64(violations)/float64(total) > threshold
}

// ConnectionState is the status part of a connection (state machine of data-model F017 §2).
type ConnectionState struct {
	Status     string
	Reason     string // "" = none
	FailedRuns int
}

// Active reports whether runs, triggers and webhooks are accepted (F017-AC-14).
func (s ConnectionState) Active() bool { return s.Status == StatusActive }

// MarkAuthError: 401/403, token or secret failure (F017-AC-12).
func (s *ConnectionState) MarkAuthError(reason string) { s.Status, s.Reason = StatusAuthError, reason }

// MarkContractError: major mismatch or violation threshold (F017-AC-02, AC-06).
func (s *ConnectionState) MarkContractError(reason string) {
	s.Status, s.Reason = StatusContractError, reason
}

// Pause: System Admin pause.
func (s *ConnectionState) Pause() { s.Status, s.Reason = StatusPaused, ReasonManual }

// Activate: System Admin reactivation after a passed health check; the circuit counter restarts.
func (s *ConnectionState) Activate() { s.Status, s.Reason, s.FailedRuns = StatusActive, "", 0 }

// RecordRun counts consecutive failed runs; reaching threshold pauses the connection (circuit_open,
// F017-AC-13). A success resets the counter. It returns true when the run opened the circuit.
func (s *ConnectionState) RecordRun(failed bool, threshold int) bool {
	if !failed {
		s.FailedRuns = 0
		return false
	}
	s.FailedRuns++
	if threshold > 0 && s.FailedRuns >= threshold && s.Status == StatusActive {
		s.Status, s.Reason = StatusPaused, ReasonCircuitOpen
		return true
	}
	return false
}
