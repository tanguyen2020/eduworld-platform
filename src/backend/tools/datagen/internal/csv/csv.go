// Package csv writes the files of one generated tenant (F132-AC-07): schools.csv and students.csv in the csv_import
// format of the identity module (SIS_INTEGRATION §4.3: UTF-8, comma, header = field names of the SIS contract DTO,
// login.username as login_username), staff.csv for the identity staff API, and bots.json, the bot list of k6 and
// loadbot. No password is ever written: bot passwords come from a secret of the run (GitHub Environment staging).
package csv

import (
	stdcsv "encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/datagen/internal/org"
)

// SchoolHeader and StudentHeader are the columns read by csvreader of the identity module (SIS contract 1.3.0).
var (
	SchoolHeader  = []string{"school_external_id", "school_code", "name", "school_level", "status", "updated_at"}
	StudentHeader = []string{"student_external_id", "school_external_id", "student_code", "full_name", "grade_level", "status", "login_username", "updated_at"}
	StaffHeader   = []string{"username", "full_name", "role", "school_external_id"}
)

// Bot is one entry of bots.json.
type Bot struct {
	Username   string `json:"username"`
	Tenant     string `json:"tenant"`
	School     string `json:"school_external_id"`
	GradeLevel int    `json:"grade_level"`
	DeviceID   string `json:"device_id"`
}

// Schools writes schools.csv of a tenant.
func Schools(w io.Writer, t org.Tenant, at time.Time) error {
	rows := [][]string{SchoolHeader}
	for _, s := range t.Schools {
		rows = append(rows, []string{s.ExternalID, s.Code, s.Name, s.Level, "active", at.UTC().Format(time.RFC3339)})
	}
	return writeAll(w, rows)
}

// Students writes students.csv of a tenant.
func Students(w io.Writer, t org.Tenant, at time.Time) error {
	rows := [][]string{StudentHeader}
	for _, s := range t.Schools {
		for _, st := range s.Students {
			rows = append(rows, []string{st.ExternalID, s.ExternalID, st.Code, st.FullName, strconv.Itoa(st.Grade), "active", st.Username,
				at.UTC().Format(time.RFC3339)})
		}
	}
	return writeAll(w, rows)
}

// Staff writes staff.csv of a tenant.
func Staff(w io.Writer, t org.Tenant) error {
	rows := [][]string{StaffHeader}
	for _, s := range t.Schools {
		for _, p := range s.Staff {
			rows = append(rows, []string{p.Username, p.FullName, p.Role, s.ExternalID})
		}
	}
	return writeAll(w, rows)
}

// Bots writes bots.json for every tenant.
func Bots(w io.Writer, ts []org.Tenant) error {
	out := []Bot{}
	for _, t := range ts {
		for _, s := range t.Schools {
			for _, st := range s.Students {
				out = append(out, Bot{Username: st.Username, Tenant: t.Code, School: s.ExternalID, GradeLevel: st.Grade, DeviceID: st.DeviceID})
			}
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", " ")
	return enc.Encode(out)
}

func writeAll(w io.Writer, rows [][]string) error {
	if err := stdcsv.NewWriter(w).WriteAll(rows); err != nil {
		return fmt.Errorf("csv: %w", err)
	}
	return nil
}
