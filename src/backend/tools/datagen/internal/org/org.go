// Package org builds the synthetic organisation of a load run (F132-AC-07, TEST_STRATEGY §5): N tenants perf-<n>,
// M schools per tenant, students spread over grades 1–9, two staff members per school (school_admin,
// academic_manager). Everything is derived from one seed, so two runs with the same flags produce the same data and
// the same bot list. Every student gets its own device_id, as in a school computer room behind one NAT address
// (spec edge case, AGENTS R30): the per-device login limit never throttles the whole run.
package org

import (
	"fmt"
	"math/rand/v2"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/datagen/internal/names"
)

// Options sizes the organisation.
type Options struct {
	Prefix           string // tenant prefix, "perf" (guard)
	Tenants          int
	SchoolsPerTenant int
	Students         int // total over every tenant
	Seed             uint64
}

// Tenant is one generated tenant.
type Tenant struct {
	Code    string // perf-001
	Schools []School
}

// School is one school of a tenant.
type School struct {
	ExternalID string // PERF001-S01
	Code       string
	Name       string
	Level      string // primary | lower_secondary | multi_level (SIS contract)
	Students   []Student
	Staff      []Staff
}

// Student is one generated student.
type Student struct {
	ExternalID string
	Code       string
	FullName   string
	Grade      int
	Username   string
	DeviceID   string
}

// Staff is one generated school staff account (created through the identity API, not the SIS import).
type Staff struct {
	Username string
	FullName string
	Role     string // school_admin | academic_manager
}

// Build generates the organisation.
func Build(o Options) ([]Tenant, error) {
	if o.Tenants <= 0 || o.SchoolsPerTenant <= 0 || o.Students < 0 {
		return nil, fmt.Errorf("org: tenants and schools per tenant must be > 0, students >= 0")
	}
	if o.Tenants > 999 || o.SchoolsPerTenant > 99 {
		return nil, fmt.Errorf("org: at most 999 tenants and 99 schools per tenant")
	}
	r := rand.New(rand.NewPCG(o.Seed, o.Seed^0x9e3779b97f4a7c15))
	schools := o.Tenants * o.SchoolsPerTenant
	out := make([]Tenant, o.Tenants)
	for t := range out {
		code := fmt.Sprintf("%s-%03d", o.Prefix, t+1)
		out[t].Code = code
		for s := 0; s < o.SchoolsPerTenant; s++ {
			ext := fmt.Sprintf("%s%03d-S%02d", upper(o.Prefix), t+1, s+1)
			sc := School{ExternalID: ext, Code: fmt.Sprintf("S%02d", s+1), Name: fmt.Sprintf("Trường thử tải %03d-%02d", t+1, s+1),
				Level: levels[s%len(levels)]}
			idx := t*o.SchoolsPerTenant + s
			n := o.Students / schools
			if idx < o.Students%schools {
				n++
			}
			for i := 0; i < n; i++ {
				sc.Students = append(sc.Students, Student{
					ExternalID: fmt.Sprintf("%s-HS%05d", ext, i+1),
					Code:       fmt.Sprintf("HS%05d", i+1),
					FullName:   names.Full(r),
					Grade:      gradeFor(sc.Level, i),
					Username:   fmt.Sprintf("%s.%03d.%02d.%05d", o.Prefix, t+1, s+1, i+1),
					DeviceID:   uuid4(r),
				})
			}
			for j, role := range []string{"school_admin", "academic_manager"} {
				sc.Staff = append(sc.Staff, Staff{Username: fmt.Sprintf("%s.%03d.%02d.gv%d", o.Prefix, t+1, s+1, j+1),
					FullName: names.Full(r), Role: role})
			}
			out[t].Schools = append(out[t].Schools, sc)
		}
	}
	return out, nil
}

var levels = []string{"primary", "lower_secondary", "multi_level"}

// gradeFor spreads students over the grades of the school level (primary 1–5, lower secondary 6–9, both 1–9).
func gradeFor(level string, i int) int {
	switch level {
	case "primary":
		return 1 + i%5
	case "lower_secondary":
		return 6 + i%4
	default:
		return 1 + i%9
	}
}

func upper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		}
		if c == '-' {
			b[i] = '_'
		}
	}
	return string(b)
}

// uuid4 is a random (version 4) UUID drawn from the seeded generator: deterministic per seed, unique per student.
func uuid4(r *rand.Rand) string {
	var b [16]byte
	for i := 0; i < 16; i += 8 {
		v := r.Uint64()
		for j := 0; j < 8; j++ {
			b[i+j] = byte(v >> (8 * j))
		}
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
