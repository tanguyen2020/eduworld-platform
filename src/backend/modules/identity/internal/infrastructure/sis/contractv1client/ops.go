package contractv1client

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strconv"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/sis"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/contract"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/metrics"
)

type rawPage struct {
	Data *[]map[string]any `json:"data"`
	Page *struct {
		NextCursor *string `json:"next_cursor"`
		HasMore    *bool   `json:"has_more"`
	} `json:"page"`
}

// entity names of metrics and record errors.
var entityOf = map[string]string{contract.KindSchool: sis.KindSchool, contract.KindStudent: sis.KindStudent,
	contract.KindAcademicYear: "academic_year"}

func externalIDOf(kind string, r map[string]any) string {
	switch kind {
	case contract.KindSchool:
		return sis.Str(r, "school_external_id")
	case contract.KindStudent:
		return sis.Str(r, "student_external_id")
	case contract.KindAcademicYear:
		return sis.Str(r, "academic_year")
	}
	return ""
}

// record strips and validates one record: ok = valid against the DTO.
func (c *Client) record(kind string, raw map[string]any) (map[string]any, []port.SisViolation, int) {
	clean, dropped := c.f.Validator.Strip(kind, raw)
	metrics.UnknownFields(c.conn.ID, entityOf[kind], dropped)
	vs := c.f.Validator.Validate(kind, clean)
	if len(vs) == 0 {
		return clean, nil, dropped
	}
	ext := externalIDOf(kind, clean)
	if len(ext) > 64 {
		ext = ext[:64]
	}
	out := make([]port.SisViolation, 0, len(vs))
	for _, v := range vs {
		out = append(out, port.SisViolation{Kind: entityOf[kind], ExternalID: ext, Field: v.Field, Message: v.Message})
	}
	return nil, out, dropped
}

// list fetches one page of a list operation and validates it (cursor loop detection: F017 edge case).
func (c *Client) list(ctx context.Context, op, kind, path string, q url.Values, cursor string) (port.SisPage, error) {
	if q == nil {
		q = url.Values{}
	}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	q.Set("limit", strconv.Itoa(DefaultPageLimit))
	var raw rawPage
	if err := c.get(ctx, op, path, q, &raw, true); err != nil {
		return port.SisPage{}, err
	}
	if raw.Data == nil || raw.Page == nil || raw.Page.HasMore == nil {
		return port.SisPage{}, fmt.Errorf("sis %s: data/page missing: %w", op, port.ErrSisBadResponse)
	}
	out := port.SisPage{HasMore: *raw.Page.HasMore}
	if raw.Page.NextCursor != nil {
		out.NextCursor = *raw.Page.NextCursor
	}
	if out.HasMore && (out.NextCursor == "" || out.NextCursor == cursor) {
		return port.SisPage{}, fmt.Errorf("sis %s: has_more without a new next_cursor: %w", op, port.ErrSisPagination)
	}
	for _, r := range *raw.Data {
		rec, vs, dropped := c.record(kind, r)
		out.UnknownFields += dropped
		if vs != nil {
			out.Violations = append(out.Violations, vs...)
			continue
		}
		out.Records = append(out.Records, rec)
	}
	return out, nil
}

// Health calls getHealth. contract_version is read even when its major is not 1 (version check of the
// caller); capabilities outside the enum are ignored.
func (c *Client) Health(ctx context.Context) (port.SisHealth, error) {
	var raw map[string]any
	if err := c.get(ctx, "getHealth", "/health", nil, &raw, true); err != nil {
		return port.SisHealth{}, err
	}
	h := port.SisHealth{Status: sis.Str(raw, "status"), ContractVersion: sis.Str(raw, "contract_version")}
	caps, ok := raw["capabilities"].([]any)
	if (h.Status != "ok" && h.Status != "degraded") || h.ContractVersion == "" || len(h.ContractVersion) > 64 || !ok {
		return port.SisHealth{}, fmt.Errorf("sis getHealth: %w", port.ErrSisBadResponse)
	}
	var names []string
	for _, x := range caps {
		if s, ok := x.(string); ok {
			names = append(names, s)
		}
	}
	h.Capabilities = sis.ValidCapabilities(names)
	return h, nil
}

// ListSchools calls listSchools.
func (c *Client) ListSchools(ctx context.Context, cursor string) (port.SisPage, error) {
	return c.list(ctx, "listSchools", contract.KindSchool, "/schools", nil, cursor)
}

// ListAcademicYears calls listAcademicYears (not paginated).
func (c *Client) ListAcademicYears(ctx context.Context, school string) (port.SisPage, error) {
	var raw rawPage
	if err := c.get(ctx, "listAcademicYears", "/schools/"+url.PathEscape(school)+"/academic-years", nil, &raw, true); err != nil {
		return port.SisPage{}, err
	}
	if raw.Data == nil {
		return port.SisPage{}, fmt.Errorf("sis listAcademicYears: data missing: %w", port.ErrSisBadResponse)
	}
	var out port.SisPage
	for _, r := range *raw.Data {
		rec, vs, dropped := c.record(contract.KindAcademicYear, r)
		out.UnknownFields += dropped
		if vs != nil {
			out.Violations = append(out.Violations, vs...)
			continue
		}
		out.Records = append(out.Records, rec)
	}
	return out, nil
}

// ListStudents calls listStudents.
func (c *Client) ListStudents(ctx context.Context, school, cursor string) (port.SisPage, error) {
	return c.list(ctx, "listStudents", contract.KindStudent, "/schools/"+url.PathEscape(school)+"/students", nil, cursor)
}

// kindOfChange maps Change.entity_kind to the DTO.
var kindOfChange = map[string]string{sis.KindSchool: contract.KindSchool, sis.KindStudent: contract.KindStudent}

// ParseChanges validates the Change items of a page or a ChangeBatch (F017-AC-15): sequence ≥ 0,
// entity_kind, operation, external_id ≤ 200, record required for upsert and valid against the DTO of its
// kind (unknown fields dropped); the result is sorted by sequence. A change of a kind removed by contract 1.3.0
// (class, enrollment — ADR-0095 D6) is passed through without record so the sync ignores it with a warning; it is
// never a violation.
func ParseChanges(v *contract.Validator, items []map[string]any) ([]port.SisChange, []port.SisViolation, int) {
	var out []port.SisChange
	var bad []port.SisViolation
	dropped := 0
	for _, it := range items {
		seq, okSeq := it["sequence"].(float64)
		kind, op, ext := sis.Str(it, "entity_kind"), sis.Str(it, "operation"), sis.Str(it, "external_id")
		if sis.IgnoredKind(kind) && okSeq && seq >= 0 {
			out = append(out, port.SisChange{Sequence: int64(seq), Kind: kind, Operation: op, ExternalID: ext})
			continue
		}
		dto, okKind := kindOfChange[kind]
		short := ext
		if len(short) > 64 {
			short = short[:64]
		}
		if !okSeq || seq < 0 || seq != float64(int64(seq)) || !okKind || (op != "upsert" && op != "delete") || ext == "" || len(ext) > 200 ||
			sis.Str(it, "occurred_at") == "" {
			bad = append(bad, port.SisViolation{Kind: "change", ExternalID: short, Field: "change", Message: "sequence, entity_kind, operation, external_id, occurred_at"})
			continue
		}
		ch := port.SisChange{Sequence: int64(seq), Kind: kind, Operation: op, ExternalID: ext}
		if op == "upsert" {
			rec, ok := it["record"].(map[string]any)
			if !ok {
				bad = append(bad, port.SisViolation{Kind: kind, ExternalID: short, Field: "record", Message: "required"})
				continue
			}
			clean, n := v.Strip(dto, rec)
			dropped += n
			if vs := v.Validate(dto, clean); len(vs) > 0 {
				for _, x := range vs {
					bad = append(bad, port.SisViolation{Kind: kind, ExternalID: short, Field: "record." + x.Field, Message: x.Message})
				}
				continue
			}
			ch.Record = clean
		}
		out = append(out, ch)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Sequence < out[j].Sequence })
	return out, bad, dropped
}

// ListChanges calls listChanges (410 → port.ErrSisWatermarkExpired).
func (c *Client) ListChanges(ctx context.Context, since string) (port.SisChangePage, error) {
	q := url.Values{"limit": {strconv.Itoa(DefaultPageLimit)}}
	if since != "" {
		q.Set("since", since)
	}
	var raw rawPage
	if err := c.get(ctx, "listChanges", "/changes", q, &raw, true); err != nil {
		return port.SisChangePage{}, err
	}
	if raw.Data == nil || raw.Page == nil || raw.Page.HasMore == nil {
		return port.SisChangePage{}, fmt.Errorf("sis listChanges: data/page missing: %w", port.ErrSisBadResponse)
	}
	out := port.SisChangePage{HasMore: *raw.Page.HasMore}
	if raw.Page.NextCursor != nil {
		out.NextCursor = *raw.Page.NextCursor
	}
	if out.HasMore && (out.NextCursor == "" || out.NextCursor == since) {
		return port.SisChangePage{}, fmt.Errorf("sis listChanges: %w", port.ErrSisPagination)
	}
	out.Changes, out.Violations, out.UnknownFields = ParseChanges(c.f.Validator, *raw.Data)
	metrics.UnknownFields(c.conn.ID, "change", out.UnknownFields)
	return out, nil
}

// LookupStudent calls lookupStudent once, without retry nor pacing (login path, F017-AC-22): the caller
// sets the timeout (sis_login_sync_timeout_ms).
func (c *Client) LookupStudent(ctx context.Context, school, username string) (map[string]any, error) {
	var raw map[string]any
	q := url.Values{"login_username": {username}}
	if err := c.get(ctx, "lookupStudent", "/schools/"+url.PathEscape(school)+"/students/lookup", q, &raw, false); err != nil {
		return nil, err
	}
	rec, vs, _ := c.record(contract.KindStudent, raw)
	if vs != nil {
		return nil, fmt.Errorf("sis lookupStudent: %w", port.ErrSisBadResponse)
	}
	return rec, nil
}
