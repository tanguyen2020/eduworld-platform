package sis

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func validConfig() ContractConfig {
	return ContractConfig{BaseURL: "https://sis.th-demo-a.example/eduworld/v1", AuthType: AuthAPIKey,
		Schools: []ContractSchool{{ExternalID: "TH-DEMO-001", SchoolID: uuid.New()}}}
}

func fields(fes []FieldError) string {
	var out []string
	for _, f := range fes {
		out = append(out, f.Field)
	}
	return strings.Join(out, ",")
}

// F017-AC-01, F017-AC-30: https base URL ≤ 2048, auth type, token URL for OAuth2, secret reference, major 1.
func TestValidateContract(t *testing.T) {
	if fes := ValidateContract(validConfig(), "sis/th-demo-a", "", "1.2.0", "", false); len(fes) != 0 {
		t.Fatalf("valid config: %v", fes)
	}
	cases := []struct {
		name   string
		mutate func(c *ContractConfig, secret, version, schedule *string)
		field  string
	}{
		{"http base url", func(c *ContractConfig, _, _, _ *string) { c.BaseURL = "http://sis.example/eduworld/v1" }, "config.base_url"},
		{"ftp", func(c *ContractConfig, _, _, _ *string) { c.BaseURL = "ftp://sis.example" }, "config.base_url"},
		{"long url", func(c *ContractConfig, _, _, _ *string) { c.BaseURL = "https://x.example/" + strings.Repeat("a", 3000) }, "config.base_url"},
		{"credentials in url", func(c *ContractConfig, _, _, _ *string) { c.BaseURL = "https://u:p@sis.example" }, "config.base_url"},
		{"auth type", func(c *ContractConfig, _, _, _ *string) { c.AuthType = "basic" }, "config.auth_type"},
		{"oauth without token url", func(c *ContractConfig, _, _, _ *string) { c.AuthType = AuthOAuth2 }, "config.token_url"},
		{"missing secret", func(_ *ContractConfig, s, _, _ *string) { *s = "" }, "secret_ref"},
		{"secret escaping", func(_ *ContractConfig, s, _, _ *string) { *s = "sis/../../etc/passwd" }, "secret_ref"},
		{"secret too long", func(_ *ContractConfig, s, _, _ *string) { *s = strings.Repeat("a", 513) }, "secret_ref"},
		{"major 2", func(_ *ContractConfig, _, v, _ *string) { *v = "2.0.0" }, "contract_version"},
		{"external id", func(c *ContractConfig, _, _, _ *string) { c.Schools[0].ExternalID = strings.Repeat("x", 65) }, "config.schools[0].school_external_id"},
		{"duplicate school", func(c *ContractConfig, _, _, _ *string) { c.Schools = append(c.Schools, c.Schools[0]) }, "config.schools[1]"},
		{"schedule", func(_ *ContractConfig, _, _, s *string) { *s = "every night" }, "schedule"},
	}
	for _, tc := range cases {
		c, secret, version, schedule := validConfig(), "sis/th-demo-a", "1.2.0", ""
		tc.mutate(&c, &secret, &version, &schedule)
		if got := fields(ValidateContract(c, secret, "", version, schedule, false)); !strings.Contains(got, tc.field) {
			t.Errorf("%s: fields %q, want %s", tc.name, got, tc.field)
		}
	}
	// The dev flag of compose (EW_SIS_ALLOW_HTTP_BASE_URL) accepts http to sis-mock only when set.
	c := validConfig()
	c.BaseURL = "http://sis-mock:4010/eduworld/v1"
	if fes := ValidateContract(c, "sis/th-demo-a", "", "1.2.0", "", true); len(fes) != 0 {
		t.Fatalf("dev http: %v", fes)
	}
}

// F017-AC-02: only the major counts.
func TestCompatibleMajor(t *testing.T) {
	for _, tc := range []struct {
		conn, facade string
		ok           bool
	}{{"1.2.0", "1.2.1", true}, {"1.0.0", "1.9.3", true}, {"1.2.0", "2.0.0", false}, {"1.2.0", "", false}, {"1.2.0", "x.1", false}} {
		if CompatibleMajor(tc.conn, tc.facade) != tc.ok {
			t.Errorf("%s vs %s", tc.conn, tc.facade)
		}
	}
}

// F017-AC-13: 5 consecutive failed runs pause the connection (circuit_open); a success resets the counter;
// reactivation restarts it.
func TestCircuitBreaker(t *testing.T) {
	s := ConnectionState{Status: StatusActive}
	for i := 1; i <= 4; i++ {
		if s.RecordRun(true, CircuitThreshold) {
			t.Fatalf("opened after %d failures", i)
		}
	}
	s.RecordRun(false, CircuitThreshold)
	if s.FailedRuns != 0 {
		t.Fatal("a success must reset the counter")
	}
	for i := 1; i < CircuitThreshold; i++ {
		s.RecordRun(true, CircuitThreshold)
	}
	if !s.RecordRun(true, CircuitThreshold) || s.Status != StatusPaused || s.Reason != ReasonCircuitOpen || s.Active() {
		t.Fatalf("state after 5 failures: %+v", s)
	}
	s.Activate()
	if !s.Active() || s.FailedRuns != 0 || s.Reason != "" {
		t.Fatalf("reactivated: %+v", s)
	}
	s.MarkAuthError(ReasonCredentialRejected)
	if s.Active() || s.Status != StatusAuthError {
		t.Fatal("auth_error must not be active")
	}
}

// F017-AC-06: more than 10% of the records.
func TestViolationRatio(t *testing.T) {
	if ViolationRatioExceeded(10, 100, ViolationThreshold) || !ViolationRatioExceeded(11, 100, ViolationThreshold) ||
		ViolationRatioExceeded(0, 0, ViolationThreshold) {
		t.Fatal("threshold is > 10%")
	}
}

// sis_sync_schedule: full 01:00 ICT, delta every 15 minutes in office hours.
func TestCron(t *testing.T) {
	full, err := ParseCron(DefaultFullSchedule)
	if err != nil {
		t.Fatal(err)
	}
	if !full.Matches(time.Date(2026, 11, 19, 18, 0, 0, 0, time.UTC)) { // 01:00 ICT on the 20th
		t.Fatal("01:00 ICT")
	}
	if full.Matches(time.Date(2026, 11, 19, 18, 1, 0, 0, time.UTC)) {
		t.Fatal("01:01 ICT")
	}
	delta, err := ParseCron(DefaultDeltaSchedule)
	if err != nil {
		t.Fatal(err)
	}
	if !delta.Matches(time.Date(2026, 11, 19, 2, 30, 0, 0, time.UTC)) || delta.Matches(time.Date(2026, 11, 19, 2, 31, 0, 0, time.UTC)) ||
		delta.Matches(time.Date(2026, 11, 22, 2, 30, 0, 0, time.UTC)) { // 09:30 Thu yes, 09:31 no, Sunday no
		t.Fatal("delta schedule")
	}
	for _, bad := range []string{"", "* * * *", "60 * * * *", "*/0 * * * *", "5-1 * * * *", "a * * * *"} {
		if _, err := ParseCron(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
