// Package fakesis is a MOCK of a school's SIS facade (EduWorld SIS Contract v1) for tests only — never wired
// into a binary nor presented as a real SIS (AGENTS R17, SIS_INTEGRATION §11, F017 T011). It serves the pull
// operations with cursor pages, scripted failures (401/404/410/429/5xx, Retry-After), slow answers, fields
// outside the contract, any contract_version, and an OAuth2 token endpoint; it counts calls per operation.
package fakesis

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Operation names of Calls and Fail.
const (
	OpHealth      = "health"
	OpSchools     = "schools"
	OpYears       = "academic-years"
	OpClasses     = "classes" // removed by SIS contract 1.3.0 (ADR-0095 D6): a legacy facade still answers, EduWorld never calls
	OpStudents    = "students"
	OpEnrollments = "enrollments" // removed by SIS contract 1.3.0, as OpClasses
	OpChanges     = "changes"
	OpLookup      = "lookup"
	OpToken       = "token"
)

// MOCK is the label of every fake SIS (grep-able in reports).
const MOCK = "MOCK fake SIS (tests only)"

// Server is the MOCK facade. Fields may be changed between calls under Lock/Unlock.
type Server struct {
	*httptest.Server
	mu sync.Mutex

	APIKey       string // required X-Api-Key ("" = none)
	BearerToken  string // required Bearer token issued by /oauth2/token ("" = none)
	ClientID     string // OAuth2 client of the token endpoint
	ClientSecret string
	Version      string
	Capabilities []string
	HealthStatus string
	PageSize     int

	Schools     []map[string]any
	Years       map[string][]map[string]any // school_external_id → years
	Classes     map[string][]map[string]any
	Students    map[string][]map[string]any
	Enrollments map[string][]map[string]any
	Changes     []map[string]any
	ExpiredFrom map[string]bool           // listChanges since values answered 410
	Lookups     map[string]map[string]any // login_username → Student
	Fail        map[string][]int          // operation → statuses returned before the normal answer
	RetryAfter  string                    // Retry-After of scripted 429/503
	Delay       map[string]time.Duration  // operation → delay before answering
	Calls       map[string]int            // operation → number of requests
	Queries     map[string][]string       // operation → raw queries received
	Extra       map[string]map[string]any // operation → extra top-level fields of each record (unknown fields)
	StuckCursor bool                      // list pages answer has_more with the cursor they received (broken facade)
}

// New starts a MOCK SIS with the demo data of TH Demo A (one school, three students of grade 3). It still answers the
// classes and enrollments of contract 1.2 like a facade that was not updated (ADR-0095 D6): EduWorld must not call them.
func New() *Server {
	s := &Server{Version: "1.3.0", Capabilities: []string{"full_sync", "delta", "webhook", "student_lookup"}, HealthStatus: "ok", PageSize: 2,
		Years: map[string][]map[string]any{}, Classes: map[string][]map[string]any{}, Students: map[string][]map[string]any{},
		Enrollments: map[string][]map[string]any{}, ExpiredFrom: map[string]bool{}, Lookups: map[string]map[string]any{},
		Fail: map[string][]int{}, Delay: map[string]time.Duration{}, Calls: map[string]int{}, Queries: map[string][]string{},
		Extra: map[string]map[string]any{}}
	s.Seed("TH-DEMO-001")
	s.Server = httptest.NewServer(http.HandlerFunc(s.serve))
	return s
}

// T is the updated_at of the demo records.
const T = "2026-11-01T08:00:00Z"

// Student returns a contract Student record.
func Student(ext, school, code, name, username, status string) map[string]any {
	return map[string]any{"student_external_id": ext, "school_external_id": school, "student_code": code, "full_name": name, "grade_level": 3,
		"status": status, "login": map[string]any{"username": username}, "updated_at": T}
}

// Class returns a contract Class record of 2026-2027.
func Class(ext, school, name string) map[string]any {
	return map[string]any{"class_external_id": ext, "school_external_id": school, "academic_year": "2026-2027", "grade_level": 3, "name": name,
		"updated_at": T}
}

// Enrollment returns an active contract Enrollment record.
func Enrollment(student, class string) map[string]any {
	return map[string]any{"student_external_id": student, "class_external_id": class, "start_date": "2026-09-05", "status": "active", "updated_at": T}
}

// Seed adds a school with the demo classes, students and enrollments (usernames prefixed by the school).
func (s *Server) Seed(school string) {
	p := strings.ToLower(strings.ReplaceAll(school, "-", ""))
	s.Schools = append(s.Schools, map[string]any{"school_external_id": school, "school_code": "S" + p[len(p)-3:], "name": "TH " + school,
		"status": "active", "updated_at": T})
	s.Years[school] = []map[string]any{{"academic_year": "2026-2027", "start_date": "2026-09-05", "end_date": "2027-05-31", "is_current": true}}
	s.Classes[school] = []map[string]any{Class(school+"-3A", school, "3A"), Class(school+"-3B", school, "3B")}
	s.Students[school] = []map[string]any{
		Student(school+"-HS1", school, "A001", "Nguyễn Văn An", "hs.an."+p, "active"),
		Student(school+"-HS2", school, "A002", "Trần Thị Bình", "hs.binh."+p, "active"),
		Student(school+"-HS3", school, "A003", "Lê Minh Chi", "hs.chi."+p, "active"),
	}
	s.Enrollments[school] = []map[string]any{Enrollment(school+"-HS1", school+"-3A"), Enrollment(school+"-HS2", school+"-3A"),
		Enrollment(school+"-HS3", school+"-3B")}
}

// Lock and Unlock guard changes of the fields while the server runs.
func (s *Server) Lock()   { s.mu.Lock() }
func (s *Server) Unlock() { s.mu.Unlock() }

// CallsOf returns the number of requests of an operation.
func (s *Server) CallsOf(op string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Calls[op]
}

func problemJSON(w http.ResponseWriter, status int, retry string) {
	w.Header().Set("Content-Type", "application/problem+json")
	if retry != "" && (status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable) {
		w.Header().Set("Retry-After", retry)
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "about:blank", "title": http.StatusText(status), "status": status})
}

func (s *Server) opOf(path string) (op, school string) {
	path = strings.TrimPrefix(path, "/eduworld/v1")
	switch {
	case path == "/health":
		return OpHealth, ""
	case path == "/schools":
		return OpSchools, ""
	case path == "/changes":
		return OpChanges, ""
	case path == "/oauth2/token":
		return OpToken, ""
	case strings.HasPrefix(path, "/schools/"):
		parts := strings.Split(strings.TrimPrefix(path, "/schools/"), "/")
		if len(parts) == 2 {
			return parts[1], parts[0]
		}
		if len(parts) == 3 && parts[1] == "students" && parts[2] == "lookup" {
			return OpLookup, parts[0]
		}
	}
	return "", ""
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	op, school := s.opOf(r.URL.Path)
	s.mu.Lock()
	s.Calls[op]++
	s.Queries[op] = append(s.Queries[op], r.URL.RawQuery)
	delay := s.Delay[op]
	var scripted int
	if q := s.Fail[op]; len(q) > 0 {
		scripted, s.Fail[op] = q[0], q[1:]
	}
	s.mu.Unlock()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
			return
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if scripted != 0 {
		problemJSON(w, scripted, s.RetryAfter)
		return
	}
	if op == OpToken {
		id, secret, ok := r.BasicAuth()
		if !ok || id != s.ClientID || secret != s.ClientSecret {
			problemJSON(w, http.StatusUnauthorized, "")
			return
		}
		writeJSON(w, map[string]any{"access_token": s.BearerToken, "token_type": "Bearer", "expires_in": 300})
		return
	}
	if (s.APIKey != "" && r.Header.Get("X-Api-Key") != s.APIKey) || (s.BearerToken != "" && r.Header.Get("Authorization") != "Bearer "+s.BearerToken) {
		problemJSON(w, http.StatusUnauthorized, "")
		return
	}
	q := r.URL.Query()
	switch op {
	case OpHealth:
		writeJSON(w, map[string]any{"status": s.HealthStatus, "contract_version": s.Version, "capabilities": s.Capabilities, "server_time": T})
	case OpSchools:
		s.page(w, OpSchools, s.Schools, q.Get("cursor"))
	case OpYears:
		writeJSON(w, map[string]any{"data": s.records(OpYears, s.Years[school])})
	case OpClasses:
		s.page(w, OpClasses, s.Classes[school], q.Get("cursor"))
	case OpStudents:
		s.page(w, OpStudents, s.Students[school], q.Get("cursor"))
	case OpEnrollments:
		s.page(w, OpEnrollments, s.Enrollments[school], q.Get("cursor"))
	case OpChanges:
		since := q.Get("since")
		if s.ExpiredFrom[since] {
			problemJSON(w, http.StatusGone, "")
			return
		}
		s.changes(w, since)
	case OpLookup:
		rec, ok := s.Lookups[q.Get("login_username")]
		if !ok {
			problemJSON(w, http.StatusNotFound, "")
			return
		}
		writeJSON(w, s.with(OpLookup, rec))
	default:
		problemJSON(w, http.StatusNotFound, "")
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) with(op string, rec map[string]any) map[string]any {
	out := make(map[string]any, len(rec))
	for k, v := range rec {
		out[k] = v
	}
	for k, v := range s.Extra[op] {
		out[k] = v
	}
	return out
}

func (s *Server) records(op string, recs []map[string]any) []map[string]any {
	out := make([]map[string]any, 0, len(recs))
	for _, r := range recs {
		out = append(out, s.with(op, r))
	}
	return out
}

// page serves cursor "c<offset>" pages of PageSize records.
func (s *Server) page(w http.ResponseWriter, op string, recs []map[string]any, cursor string) {
	off, _ := strconv.Atoi(strings.TrimPrefix(cursor, "c"))
	size := s.PageSize
	if size <= 0 {
		size = 500
	}
	end := min(off+size, len(recs))
	if off > len(recs) {
		off = len(recs)
	}
	var next any
	more := end < len(recs)
	if more {
		next = "c" + strconv.Itoa(end)
	}
	if s.StuckCursor {
		next, more = cursor, true
	}
	writeJSON(w, map[string]any{"data": s.records(op, recs[off:end]), "page": map[string]any{"has_more": more, "next_cursor": next}})
}

// changes serves Changes after the watermark "wm-<sequence>" in pages of PageSize; the cursor of the last
// page is the watermark of the last change.
func (s *Server) changes(w http.ResponseWriter, since string) {
	from := -1
	if since != "" {
		from, _ = strconv.Atoi(strings.TrimPrefix(since, "wm-"))
	}
	rest := []map[string]any{}
	for _, c := range s.Changes {
		if seq, _ := c["sequence"].(int); seq > from {
			rest = append(rest, c)
		}
	}
	sort.SliceStable(rest, func(i, j int) bool {
		a, _ := rest[i]["sequence"].(int)
		b, _ := rest[j]["sequence"].(int)
		return a < b
	})
	size := s.PageSize
	if size <= 0 {
		size = 500
	}
	end := min(size, len(rest))
	last := from
	for _, c := range rest[:end] {
		last, _ = c["sequence"].(int)
	}
	writeJSON(w, map[string]any{"data": rest[:end], "page": map[string]any{"has_more": end < len(rest), "next_cursor": "wm-" + strconv.Itoa(last)}})
}
