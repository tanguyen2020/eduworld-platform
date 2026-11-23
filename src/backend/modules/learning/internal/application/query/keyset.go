package query

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// keyKind is the type of one sort value of a cursor (the cursor carries the values as JSON, signed by the codec).
type keyKind uint8

const (
	kInt keyKind = iota
	kText
	kTime
	kUUID
)

// seekOf decodes the cursor of a list into the keyset position of its read model (API_DESIGN §5): kinds are the types
// of the list's sort columns before the id. The read model is asked limit+1 rows (has_more).
func seekOf(codec *page.Codec, in PageInput, kinds ...keyKind) (read.Seek, int, error) {
	limit := page.Limit(in.Limit)
	seek := read.Seek{Limit: limit + 1}
	cur, err := codec.Decode(in.Cursor)
	if err != nil {
		return seek, limit, problem.New(problem.BadRequest, "invalid cursor")
	}
	if cur == nil {
		return seek, limit, nil
	}
	vals, ok := decodeVals(cur.Key, kinds)
	if !ok {
		return seek, limit, problem.New(problem.BadRequest, "invalid cursor")
	}
	seek.After = &read.Key{Vals: vals, ID: cur.ID}
	return seek, limit, nil
}

func decodeVals(key string, kinds []keyKind) ([]any, bool) {
	if len(kinds) == 0 {
		return nil, key == ""
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(key)))
	dec.UseNumber()
	var raw []any
	if err := dec.Decode(&raw); err != nil || len(raw) != len(kinds) {
		return nil, false
	}
	vals := make([]any, len(kinds))
	for i, k := range kinds {
		switch k {
		case kInt:
			n, ok := raw[i].(json.Number)
			if !ok {
				return nil, false
			}
			v, err := n.Int64()
			if err != nil {
				return nil, false
			}
			vals[i] = v
		case kText:
			s, ok := raw[i].(string)
			if !ok {
				return nil, false
			}
			vals[i] = s
		case kTime:
			s, ok := raw[i].(string)
			if !ok {
				return nil, false
			}
			t, err := time.Parse(time.RFC3339Nano, s)
			if err != nil {
				return nil, false
			}
			vals[i] = t
		case kUUID:
			s, ok := raw[i].(string)
			if !ok {
				return nil, false
			}
			id, err := uuid.Parse(s)
			if err != nil {
				return nil, false
			}
			vals[i] = id
		}
	}
	return vals, true
}

func encodeVals(vals []any) string {
	if len(vals) == 0 {
		return ""
	}
	out := make([]any, len(vals))
	for i, v := range vals {
		switch x := v.(type) {
		case time.Time:
			out[i] = x.UTC().Format(time.RFC3339Nano)
		case int:
			out[i] = int64(x)
		default:
			out[i] = x
		}
	}
	b, _ := json.Marshal(out)
	return string(b)
}

// keysetPage turns the limit+1 rows of a keyset read into a page: has_more when the extra row came back, next_cursor =
// the sort values and id of the last row shown (key returns them in the order of the list's ORDER BY).
func keysetPage[T any](rows []T, limit int, codec *page.Codec, key func(T) (uuid.UUID, []any)) Page[T] {
	out := Page[T]{Data: []T{}, Limit: limit}
	if len(rows) > limit {
		rows, out.HasMore = rows[:limit], true
		id, vals := key(rows[limit-1])
		s := codec.Encode(page.Cursor{Key: encodeVals(vals), ID: id})
		out.NextCursor = &s
	}
	out.Data = append(out.Data, rows...)
	return out
}
