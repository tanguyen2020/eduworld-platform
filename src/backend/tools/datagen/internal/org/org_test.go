package org

import (
	"reflect"
	"regexp"
	"testing"
)

var extID = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,64}$`) // ExternalId of the SIS contract

func TestBuildSpreadsStudentsAndIsDeterministic(t *testing.T) {
	o := Options{Prefix: "perf", Tenants: 3, SchoolsPerTenant: 2, Students: 100, Seed: 7}
	a, err := Build(o)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Build(o)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("same seed must give the same organisation")
	}
	total, devices, users := 0, map[string]bool{}, map[string]bool{}
	for _, tn := range a {
		if tn.Code[:5] != "perf-" {
			t.Fatalf("tenant %q", tn.Code)
		}
		for _, s := range tn.Schools {
			if !extID.MatchString(s.ExternalID) || len(s.Staff) != 2 {
				t.Fatalf("school %+v", s.ExternalID)
			}
			if n := len(s.Students); n < 16 || n > 17 {
				t.Fatalf("school %s has %d students, want an even spread", s.ExternalID, n)
			}
			for _, st := range s.Students {
				total++
				if st.Grade < 1 || st.Grade > 9 || !extID.MatchString(st.ExternalID) {
					t.Fatalf("student %+v", st)
				}
				if devices[st.DeviceID] || users[st.Username] {
					t.Fatalf("duplicate device or username %+v", st)
				}
				devices[st.DeviceID], users[st.Username] = true, true
			}
		}
	}
	if total != 100 {
		t.Fatalf("total = %d", total)
	}
	c, _ := Build(Options{Prefix: "perf", Tenants: 3, SchoolsPerTenant: 2, Students: 100, Seed: 8})
	if reflect.DeepEqual(a, c) {
		t.Fatal("another seed must give other names/devices")
	}
}

func TestBuildRejectsBadSizes(t *testing.T) {
	for _, o := range []Options{{Prefix: "perf"}, {Prefix: "perf", Tenants: 1000, SchoolsPerTenant: 1}, {Prefix: "perf", Tenants: 1, SchoolsPerTenant: 1, Students: -1}} {
		if _, err := Build(o); err == nil {
			t.Fatalf("Build(%+v) accepted", o)
		}
	}
}

func TestGradesFollowSchoolLevel(t *testing.T) {
	for i := 0; i < 20; i++ {
		if g := gradeFor("primary", i); g < 1 || g > 5 {
			t.Fatal(g)
		}
		if g := gradeFor("lower_secondary", i); g < 6 || g > 9 {
			t.Fatal(g)
		}
	}
}
