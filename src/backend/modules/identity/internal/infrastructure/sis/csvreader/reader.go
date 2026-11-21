// Package csvreader reads SIS csv_import files (SIS_INTEGRATION §4.3): UTF-8 (BOM accepted), comma, header
// required, column names = field names of the contract DTO; login is split into login_username,
// login_idp_alias, login_idp_subject. Columns outside the contract are ignored and counted (F011-AC-03);
// every row is validated with the contract validator and violations are reported per row and column
// (SIS_CONTRACT_VIOLATION). The caller applies a file only when it has no error (all-or-nothing, F011-AC-02).
package csvreader

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/contract"
)

// FileKinds maps the file names of an import to the DTO they carry (SIS contract 1.3.0: schools and students).
var FileKinds = map[string]string{"schools.csv": contract.KindSchool, "students.csv": contract.KindStudent}

// IgnoredFiles are the files of kinds removed by SIS contract 1.3.0 (ADR-0095 D6) with the record kind they carried:
// an import that still has them skips them with a warning ("tệp không còn dùng"), never an error.
var IgnoredFiles = map[string]string{"classes.csv": "class", "enrollments.csv": "enrollment"}

// ErrorCode of a record violation.
const ErrorCode = "SIS_CONTRACT_VIOLATION"

// Record is one valid row.
type Record struct {
	Row  int
	Data map[string]any
}

// RowError is one violation of a row (row 1 = header).
type RowError struct {
	Row     int    `json:"row"`
	Column  string `json:"column"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Batch is the result of reading one file.
type Batch struct {
	Kind           string
	Records        []Record
	Errors         []RowError
	IgnoredColumns []string
}

// unknownFields counts columns outside the contract (metric of F011-AC-03).
var unknownFields = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "eduworld_sis_contract_unknown_fields_total",
	Help: "Columns of SIS imports outside the contract, ignored."}, []string{"kind"})

// UnknownFieldsCounter exposes eduworld_sis_contract_unknown_fields_total for registration.
func UnknownFieldsCounter() *prometheus.CounterVec { return unknownFields }

const maxRowLen = 64 << 10

// Read parses and validates one file of kind. now fills updated_at when the file has no such column (a CSV
// file is a snapshot taken at import time).
func Read(kind string, r io.Reader, v *contract.Validator, now time.Time) (*Batch, error) {
	br := bufio.NewReader(r)
	if b, err := br.Peek(3); err == nil && bytes.Equal(b, []byte{0xEF, 0xBB, 0xBF}) {
		_, _ = br.Discard(3)
	}
	cr := csv.NewReader(br)
	cr.FieldsPerRecord = -1
	cr.ReuseRecord = false
	header, err := cr.Read()
	if errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("csvreader: %s: empty file (header required)", kind)
	}
	if err != nil {
		return nil, fmt.Errorf("csvreader: %s: header: %w", kind, err)
	}
	b := &Batch{Kind: kind}
	cols := make([]string, len(header))
	for i, h := range header {
		name := strings.ToLower(strings.TrimSpace(h))
		switch {
		case strings.HasPrefix(name, "login_") && kind == contract.KindStudent:
			cols[i] = name
		case v.Known(kind, name):
			cols[i] = name
		default:
			cols[i] = ""
			b.IgnoredColumns = append(b.IgnoredColumns, name)
			unknownFields.WithLabelValues(kind).Inc()
		}
	}
	for row := 2; ; row++ {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			b.Errors = append(b.Errors, RowError{Row: row, Code: ErrorCode, Message: "malformed CSV row"})
			continue
		}
		if len(rec) != len(header) {
			b.Errors = append(b.Errors, RowError{Row: row, Code: ErrorCode, Message: "column count differs from the header"})
			continue
		}
		data, rowErrs := typed(kind, cols, rec, v, row)
		if _, ok := data["updated_at"]; !ok && v.Known(kind, "updated_at") {
			data["updated_at"] = now.UTC().Format(time.RFC3339)
		}
		for _, x := range v.Validate(kind, data) {
			rowErrs = append(rowErrs, RowError{Row: row, Column: x.Field, Code: ErrorCode, Message: x.Message})
		}
		if len(rowErrs) > 0 {
			b.Errors = append(b.Errors, rowErrs...)
			continue
		}
		b.Records = append(b.Records, Record{Row: row, Data: data})
	}
	return b, nil
}

// typed converts the cells of one row: empty cells are absent, integer properties are parsed, login_*
// columns form the login object.
func typed(kind string, cols, rec []string, v *contract.Validator, row int) (map[string]any, []RowError) {
	data := map[string]any{}
	var errs []RowError
	login := map[string]any{}
	for i, c := range cols {
		if c == "" {
			continue
		}
		val := strings.TrimSpace(rec[i])
		if val == "" || len(val) > maxRowLen {
			continue
		}
		if strings.HasPrefix(c, "login_") {
			login[strings.TrimPrefix(c, "login_")] = val
			continue
		}
		switch v.Type(kind, c) {
		case "integer":
			n, err := strconv.Atoi(val)
			if err != nil {
				errs = append(errs, RowError{Row: row, Column: c, Code: ErrorCode, Message: "must be an integer"})
				continue
			}
			data[c] = n
		case "boolean":
			bv, err := strconv.ParseBool(val)
			if err != nil {
				errs = append(errs, RowError{Row: row, Column: c, Code: ErrorCode, Message: "must be true or false"})
				continue
			}
			data[c] = bv
		default:
			data[c] = val
		}
	}
	if len(login) > 0 {
		data["login"] = login
	}
	return data, errs
}

// EntityKind maps a contract DTO kind ("Student") to the entity kind of the sync pipeline ("student").
func EntityKind(contractKind string) string { return strings.ToLower(contractKind) }
