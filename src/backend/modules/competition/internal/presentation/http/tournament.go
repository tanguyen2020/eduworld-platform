package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/challenge"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// TournamentCommands are the F107 write use cases (command.Tournaments).
type TournamentCommands interface {
	CreateChallenge(ctx context.Context, in command.ChallengeInput) (map[string]any, error)
	CancelChallenge(ctx context.Context, id uuid.UUID, reason string) (map[string]any, error)
	JoinChallenge(ctx context.Context, id uuid.UUID) (command.JoinResult, error)
	PublishTournament(ctx context.Context, id uuid.UUID) (map[string]any, error)
	CancelTournament(ctx context.Context, id uuid.UUID, reason string) (map[string]any, error)
	RegisterTournamentEntry(ctx context.Context, id uuid.UUID, in command.EntryInput) (command.EntryResult, error)
	WithdrawTournamentEntry(ctx context.Context, id, entryID uuid.UUID) error
	ConfirmTournamentEntries(ctx context.Context, id uuid.UUID, only []uuid.UUID) (command.EntryResult, error)
	CheckInTournamentEntry(ctx context.Context, id, entryID uuid.UUID) (map[string]any, error)
	GenerateTournamentBracket(ctx context.Context, id uuid.UUID, order []uuid.UUID) (map[string]any, error)
	ConfirmBracketNode(ctx context.Context, id, nodeID uuid.UUID) (map[string]any, error)
	OverrideBracketNode(ctx context.Context, id, nodeID, winner uuid.UUID, reason string) (map[string]any, error)
}

// TournamentQueries are the F107 read use cases (query.Tournaments).
type TournamentQueries interface {
	ListChallenges(ctx context.Context, grade *int, status *string, limit int, cursor string) (query.Page, error)
	GetChallenge(ctx context.Context, id uuid.UUID) (map[string]any, error)
	ListChallengeTemplates(ctx context.Context, grade *int, subject *uuid.UUID, limit int, cursor string) (query.Page, error)
	ListMyChallenges(ctx context.Context, limit int, cursor string) (query.Page, error)
	ListTournaments(ctx context.Context, scope string, status *string, school *uuid.UUID, limit int, cursor string) (query.Page, error)
	GetTournament(ctx context.Context, id uuid.UUID) (map[string]any, error)
	GetTournamentBracket(ctx context.Context, id uuid.UUID) (map[string]any, error)
}

func tenantOp(id, method, pattern string, idem httpx.IdempotencyMode, status int, guard httpx.Guard) httpx.Operation {
	return httpx.Operation{ID: id, Module: "competition", Method: method, Pattern: pattern, Tenant: httpx.TenantRequired, Idempotency: idem,
		SuccessStatus: status, Guard: guard}
}

const tp = "/competition/tournaments/{tournament_id}"

// TournamentOperations of F107: the organizer routes under /lms (portal staff guard implied) and the tournament routes
// shared by students (game) and organizers (portal) — the use cases check the channel: staff with a game token is
// GAME_ACCESS_STUDENT_ONLY, a student calling an organizer route PERMISSION_DENIED (F107-AC-26). listMyChallenges,
// joinChallenge and checkInTournamentEntry are player routes (game_player).
var TournamentOperations = map[string]httpx.Operation{
	"createChallenge":           tenantOp("createChallenge", http.MethodPost, "/lms/challenges", httpx.IdempotencyRequired, http.StatusCreated, httpx.GuardNone),
	"listChallenges":            tenantOp("listChallenges", http.MethodGet, "/lms/challenges", httpx.IdempotencyNone, http.StatusOK, httpx.GuardNone),
	"getChallenge":              tenantOp("getChallenge", http.MethodGet, "/lms/challenges/{challenge_id}", httpx.IdempotencyNone, http.StatusOK, httpx.GuardNone),
	"cancelChallenge":           tenantOp("cancelChallenge", http.MethodPost, "/lms/challenges/{challenge_id}:cancel", httpx.IdempotencyRequired, http.StatusOK, httpx.GuardNone),
	"listChallengeTemplates":    tenantOp("listChallengeTemplates", http.MethodGet, "/lms/catalog/challenge-templates", httpx.IdempotencyNone, http.StatusOK, httpx.GuardNone),
	"listMyChallenges":          op("listMyChallenges", http.MethodGet, "/competition/challenges", httpx.IdempotencyNone, http.StatusOK),
	"joinChallenge":             op("joinChallenge", http.MethodPost, "/competition/challenges/{challenge_id}:join", httpx.IdempotencyRequired, http.StatusOK),
	"listTournaments":           tenantOp("listTournaments", http.MethodGet, "/competition/tournaments", httpx.IdempotencyNone, http.StatusOK, httpx.GuardNone),
	"getTournament":             tenantOp("getTournament", http.MethodGet, tp, httpx.IdempotencyNone, http.StatusOK, httpx.GuardNone),
	"publishTournament":         tenantOp("publishTournament", http.MethodPost, tp+":publish", httpx.IdempotencyRequired, http.StatusOK, httpx.GuardNone),
	"cancelTournament":          tenantOp("cancelTournament", http.MethodPost, tp+":cancel", httpx.IdempotencyRequired, http.StatusOK, httpx.GuardNone),
	"registerTournamentEntry":   tenantOp("registerTournamentEntry", http.MethodPost, tp+"/entries", httpx.IdempotencyRequired, http.StatusCreated, httpx.GuardNone),
	"withdrawTournamentEntry":   tenantOp("withdrawTournamentEntry", http.MethodDelete, tp+"/entries/{entry_id}", httpx.IdempotencyNone, http.StatusNoContent, httpx.GuardNone),
	"confirmTournamentEntries":  tenantOp("confirmTournamentEntries", http.MethodPost, tp+"/entries:confirm", httpx.IdempotencyRequired, http.StatusOK, httpx.GuardNone),
	"checkInTournamentEntry":    op("checkInTournamentEntry", http.MethodPost, tp+"/entries/{entry_id}:check-in", httpx.IdempotencyNone, http.StatusOK),
	"generateTournamentBracket": tenantOp("generateTournamentBracket", http.MethodPost, tp+":generate-bracket", httpx.IdempotencyRequired, http.StatusOK, httpx.GuardNone),
	"getTournamentBracket":      tenantOp("getTournamentBracket", http.MethodGet, tp+"/bracket", httpx.IdempotencyNone, http.StatusOK, httpx.GuardNone),
	"confirmBracketNode":        tenantOp("confirmBracketNode", http.MethodPost, tp+"/nodes/{node_id}:confirm", httpx.IdempotencyNone, http.StatusOK, httpx.GuardNone),
	"overrideBracketNode":       tenantOp("overrideBracketNode", http.MethodPost, tp+"/nodes/{node_id}:override", httpx.IdempotencyRequired, http.StatusOK, httpx.GuardNone),
}

// TournamentOperationIDs are the F107 routes in registration order.
var TournamentOperationIDs = []string{"createChallenge", "listChallenges", "getChallenge", "cancelChallenge", "listChallengeTemplates",
	"listMyChallenges", "joinChallenge", "listTournaments", "getTournament", "publishTournament", "cancelTournament", "registerTournamentEntry",
	"withdrawTournamentEntry", "confirmTournamentEntries", "checkInTournamentEntry", "generateTournamentBracket", "getTournamentBracket",
	"confirmBracketNode", "overrideBracketNode"}

func init() {
	for id, o := range TournamentOperations {
		Operations[id] = o
		if o.Guard&httpx.GuardGamePlayer == 0 {
			StaffOperations = append(StaffOperations, id)
		}
	}
}

func decodeBody(r *http.Request, v any, required bool) error {
	err := json.NewDecoder(r.Body).Decode(v)
	if errors.Is(err, io.EOF) && !required {
		return nil
	}
	if err != nil {
		return problem.Invalid("body", problem.SubInvalid, "is not valid JSON")
	}
	return nil
}

func queryGrade(r *http.Request, name string) (*int, error) {
	v := r.URL.Query().Get(name)
	if v == "" {
		return nil, nil
	}
	g, err := strconv.Atoi(v)
	if err != nil || g < 1 || g > 9 {
		return nil, problem.Invalid(name, problem.SubInvalid, "is between 1 and 9")
	}
	return &g, nil
}

func queryStr(r *http.Request, name string) *string {
	v := r.URL.Query().Get(name)
	if v == "" {
		return nil
	}
	return &v
}

func pageBody(p query.Page) map[string]any {
	data := p.Data
	if data == nil {
		data = []map[string]any{}
	}
	page := map[string]any{"has_more": p.NextCursor != nil, "limit": p.Limit, "next_cursor": nil}
	if p.NextCursor != nil {
		page["next_cursor"] = *p.NextCursor
	}
	return map[string]any{"data": data, "page": page}
}

type reasonBody struct {
	Reason string `json:"reason"`
}

func registerTournaments(s *httpx.Server, c TournamentCommands, q TournamentQueries) error {
	twoIDs := func(r *http.Request, a, b string) (uuid.UUID, uuid.UUID, error) {
		x, err := pathUUID(r, a)
		if err != nil {
			return x, uuid.Nil, err
		}
		y, err := pathUUID(r, b)
		return x, y, err
	}
	withReason := func(param string, fn func(ctx context.Context, id uuid.UUID, reason string) (map[string]any, error)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			id, err := pathUUID(r, param)
			if err != nil {
				fail(w, r, err)
				return
			}
			var b reasonBody
			if err := decodeBody(r, &b, true); err != nil {
				fail(w, r, err)
				return
			}
			out, err := fn(r.Context(), id, b.Reason)
			if err != nil {
				fail(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		}
	}
	list := func(fn func(r *http.Request, limit int, cursor string) (query.Page, error)) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			limit, err := queryLimit(r)
			if err != nil {
				fail(w, r, err)
				return
			}
			p, err := fn(r, limit, r.URL.Query().Get("cursor"))
			if err != nil {
				fail(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, pageBody(p))
		}
	}
	routes := map[string]http.HandlerFunc{
		"createChallenge": func(w http.ResponseWriter, r *http.Request) {
			in, err := challengeInput(r)
			if err != nil {
				fail(w, r, err)
				return
			}
			out, err := c.CreateChallenge(r.Context(), in)
			if err != nil {
				fail(w, r, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		},
		"listChallenges": list(func(r *http.Request, limit int, cursor string) (query.Page, error) {
			grade, err := queryGrade(r, "grade_level")
			if err != nil {
				return query.Page{}, err
			}
			return q.ListChallenges(r.Context(), grade, queryStr(r, "status"), limit, cursor)
		}),
		"getChallenge":    idView("challenge_id", func(ctx context.Context, id uuid.UUID) (any, error) { return q.GetChallenge(ctx, id) }),
		"cancelChallenge": withReason("challenge_id", c.CancelChallenge),
		"listChallengeTemplates": list(func(r *http.Request, limit int, cursor string) (query.Page, error) {
			grade, err := queryGrade(r, "grade_level")
			if err != nil {
				return query.Page{}, err
			}
			subject, err := queryUUID(r, "subject_id")
			if err != nil {
				return query.Page{}, err
			}
			return q.ListChallengeTemplates(r.Context(), grade, subject, limit, cursor)
		}),
		"listMyChallenges": list(func(r *http.Request, limit int, cursor string) (query.Page, error) {
			return q.ListMyChallenges(r.Context(), limit, cursor)
		}),
		"joinChallenge": idView("challenge_id", func(ctx context.Context, id uuid.UUID) (any, error) { return c.JoinChallenge(ctx, id) }),
		"listTournaments": list(func(r *http.Request, limit int, cursor string) (query.Page, error) {
			school, err := queryUUID(r, "school_id")
			if err != nil {
				return query.Page{}, err
			}
			return q.ListTournaments(r.Context(), r.URL.Query().Get("scope"), queryStr(r, "status"), school, limit, cursor)
		}),
		"getTournament":     idView("tournament_id", func(ctx context.Context, id uuid.UUID) (any, error) { return q.GetTournament(ctx, id) }),
		"publishTournament": idView("tournament_id", func(ctx context.Context, id uuid.UUID) (any, error) { return c.PublishTournament(ctx, id) }),
		"cancelTournament":  withReason("tournament_id", c.CancelTournament),
		"registerTournamentEntry": func(w http.ResponseWriter, r *http.Request) {
			id, err := pathUUID(r, "tournament_id")
			if err != nil {
				fail(w, r, err)
				return
			}
			var b struct {
				StudentIDs []uuid.UUID `json:"student_ids"`
				PartyID    *uuid.UUID  `json:"party_id"`
			}
			if err := decodeBody(r, &b, false); err != nil {
				fail(w, r, err)
				return
			}
			out, err := c.RegisterTournamentEntry(r.Context(), id, command.EntryInput{StudentIDs: b.StudentIDs, PartyID: b.PartyID})
			if err != nil {
				fail(w, r, err)
				return
			}
			writeJSON(w, http.StatusCreated, out)
		},
		"withdrawTournamentEntry": func(w http.ResponseWriter, r *http.Request) {
			id, entry, err := twoIDs(r, "tournament_id", "entry_id")
			if err != nil {
				fail(w, r, err)
				return
			}
			if err := c.WithdrawTournamentEntry(r.Context(), id, entry); err != nil {
				fail(w, r, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		},
		"confirmTournamentEntries": func(w http.ResponseWriter, r *http.Request) {
			id, err := pathUUID(r, "tournament_id")
			if err != nil {
				fail(w, r, err)
				return
			}
			var b struct {
				EntryIDs []uuid.UUID `json:"entry_ids"`
			}
			if err := decodeBody(r, &b, false); err != nil {
				fail(w, r, err)
				return
			}
			out, err := c.ConfirmTournamentEntries(r.Context(), id, b.EntryIDs)
			if err != nil {
				fail(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		},
		"checkInTournamentEntry": func(w http.ResponseWriter, r *http.Request) {
			id, entry, err := twoIDs(r, "tournament_id", "entry_id")
			if err != nil {
				fail(w, r, err)
				return
			}
			out, err := c.CheckInTournamentEntry(r.Context(), id, entry)
			if err != nil {
				fail(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		},
		"generateTournamentBracket": func(w http.ResponseWriter, r *http.Request) {
			id, err := pathUUID(r, "tournament_id")
			if err != nil {
				fail(w, r, err)
				return
			}
			var b struct {
				EntryOrder []uuid.UUID `json:"entry_order"`
			}
			if err := decodeBody(r, &b, false); err != nil {
				fail(w, r, err)
				return
			}
			out, err := c.GenerateTournamentBracket(r.Context(), id, b.EntryOrder)
			if err != nil {
				fail(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		},
		"getTournamentBracket": idView("tournament_id", func(ctx context.Context, id uuid.UUID) (any, error) { return q.GetTournamentBracket(ctx, id) }),
		"confirmBracketNode": func(w http.ResponseWriter, r *http.Request) {
			id, node, err := twoIDs(r, "tournament_id", "node_id")
			if err != nil {
				fail(w, r, err)
				return
			}
			out, err := c.ConfirmBracketNode(r.Context(), id, node)
			if err != nil {
				fail(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		},
		"overrideBracketNode": func(w http.ResponseWriter, r *http.Request) {
			id, node, err := twoIDs(r, "tournament_id", "node_id")
			if err != nil {
				fail(w, r, err)
				return
			}
			var b struct {
				WinnerEntryID uuid.UUID `json:"winner_entry_id"`
				Reason        string    `json:"reason"`
			}
			if err := decodeBody(r, &b, true); err != nil {
				fail(w, r, err)
				return
			}
			out, err := c.OverrideBracketNode(r.Context(), id, node, b.WinnerEntryID, b.Reason)
			if err != nil {
				fail(w, r, err)
				return
			}
			writeJSON(w, http.StatusOK, out)
		},
	}
	for _, id := range TournamentOperationIDs {
		if err := s.Handle(TournamentOperations[id], routes[id]); err != nil {
			return err
		}
	}
	return nil
}

// challengeInput decodes ChallengeCreate.
func challengeInput(r *http.Request) (command.ChallengeInput, error) {
	var b struct {
		TenantID   *uuid.UUID `json:"tenant_id"`
		TemplateID uuid.UUID  `json:"template_id"`
		Name       string     `json:"name"`
		SchoolID   uuid.UUID  `json:"school_id"`
		Targets    []struct {
			TargetType string      `json:"target_type"`
			GradeLevel *int        `json:"grade_level"`
			StudentIDs []uuid.UUID `json:"student_ids"`
		} `json:"targets"`
		ExcludeStudentIDs    []uuid.UUID    `json:"exclude_student_ids"`
		StartsAt             time.Time      `json:"starts_at"`
		EndsAt               *time.Time     `json:"ends_at"`
		Participation        string         `json:"participation"`
		QuestionSetVersionID *uuid.UUID     `json:"question_set_version_id"`
		Params               map[string]any `json:"params"`
	}
	if err := decodeBody(r, &b, true); err != nil {
		return command.ChallengeInput{}, err
	}
	in := command.ChallengeInput{TenantID: b.TenantID, TemplateID: b.TemplateID, QuestionSetVersionID: b.QuestionSetVersionID,
		Request: challenge.Request{Name: b.Name, SchoolID: b.SchoolID, Exclude: b.ExcludeStudentIDs, StartsAt: b.StartsAt, EndsAt: b.EndsAt,
			Participation: b.Participation, Params: b.Params}}
	for _, t := range b.Targets {
		in.Request.Targets = append(in.Request.Targets, challenge.Target{Type: t.TargetType, Grade: t.GradeLevel, StudentIDs: t.StudentIDs})
	}
	return in, nil
}
