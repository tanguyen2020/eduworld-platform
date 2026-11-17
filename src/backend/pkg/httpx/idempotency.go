package httpx

import (
	"bytes"
	"io"
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/idempotency"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// IdempotencyMode says whether an operation takes Idempotency-Key (spec parameter IdempotencyKey).
type IdempotencyMode int

const (
	IdempotencyNone     IdempotencyMode = iota
	IdempotencyOptional                 // used when present
	IdempotencyRequired                 // missing → 428 IDEMPOTENCY_KEY_REQUIRED
)

const (
	idempotencyHeader = "Idempotency-Key"
	replayedHeader    = "Idempotent-Replayed"
	maxIdemBody       = 1 << 20
)

var idempotencyReplays = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "eduworld_idempotency_replays_total", Help: "Responses replayed from infra.idempotency_records.",
}, []string{"route"})

// IdempotencyReplays exposes eduworld_idempotency_replays_total.
func IdempotencyReplays() *prometheus.CounterVec { return idempotencyReplays }

type bufferedWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (b *bufferedWriter) Header() http.Header         { return b.header }
func (b *bufferedWriter) WriteHeader(code int)        { b.status = code }
func (b *bufferedWriter) Write(p []byte) (int, error) { return b.body.Write(p) }

func replay(w http.ResponseWriter, r *http.Request, op Operation, rec *idempotency.Record) {
	idempotencyReplays.WithLabelValues(BasePath + op.Pattern).Inc()
	w.Header().Set(replayedHeader, "true")
	if len(rec.Body) > 0 {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(rec.Status)
	_, _ = w.Write(rec.Body)
}

// Idempotency returns the middleware of an idempotent operation (API_DESIGN §6, F003-AC-13…16, AC-37):
// validates the key (UUIDv7, clock skew ≤ ttl) before any transaction, replays a stored response of
// the same user + route + key (409 IDEMPOTENCY_KEY_REUSED on another payload), otherwise runs the
// handler with a Pending record that the command writes in its transaction (idempotency.SetResult).
// A handler error caused by a concurrent duplicate is answered with the committed response.
func Idempotency(store *idempotency.Store, op Operation) func(http.Handler) http.Handler {
	status := op.SuccessStatus
	if status == 0 {
		status = http.StatusOK
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := r.Header.Get(idempotencyHeader)
			if raw == "" {
				if op.Idempotency == IdempotencyRequired {
					WriteError(w, r, problem.New(problem.IdempotencyKeyRequired, ""))
					return
				}
				next.ServeHTTP(w, r)
				return
			}
			key, issued, err := idempotency.ParseKey(raw, store.Clock().Now(), store.TTL())
			if err != nil {
				WriteError(w, r, err)
				return
			}
			p, ok := authz.PrincipalFrom(r.Context())
			if !ok {
				WriteError(w, r, unauthenticated(""))
				return
			}
			body, err := io.ReadAll(io.LimitReader(r.Body, maxIdemBody+1))
			if err != nil || len(body) > maxIdemBody {
				WriteError(w, r, problem.New(problem.BadRequest, "request body too large or unreadable"))
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			k := idempotency.Key{UserID: p.UserID, Method: r.Method, Route: BasePath + op.Pattern, Key: key, IssuedOn: issued}
			hash := idempotency.Hash(body)

			lookup := func() (*idempotency.Record, bool) {
				rec, err := store.Lookup(r.Context(), k)
				if err != nil {
					WriteError(w, r, problem.New(problem.Unavailable, "").Retry().Wrap(err))
					return nil, true
				}
				if rec == nil {
					return nil, false
				}
				if rec.Hash != hash {
					WriteError(w, r, problem.New(problem.IdempotencyKeyReused, ""))
					return nil, true
				}
				replay(w, r, op, rec)
				return rec, true
			}
			if _, done := lookup(); done {
				return
			}

			pending := store.NewPending(k, hash, status)
			buf := &bufferedWriter{header: http.Header{}}
			next.ServeHTTP(buf, r.WithContext(idempotency.WithPending(r.Context(), pending)))
			if buf.status == 0 {
				buf.status = http.StatusOK
			}
			if buf.status >= 400 {
				// A concurrent duplicate may have committed first (our INSERT hit its key): replay it.
				if _, done := lookup(); done {
					return
				}
				if bytes.Contains(buf.body.Bytes(), []byte(strconv.Quote(string(problem.IdempotencyInProgress)))) {
					buf.header.Set("Retry-After", "1")
				}
			}
			for k, vs := range buf.header {
				for _, v := range vs {
					w.Header().Add(k, v)
				}
			}
			w.WriteHeader(buf.status)
			_, _ = w.Write(buf.body.Bytes())
		})
	}
}
