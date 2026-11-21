// Package http is the public HTTP presentation of module competition (F100): thin handlers that decode the request,
// call one use case and encode the response; authentication, X-Tenant-Id, principal, the game_player guard (students
// only, ADR-0031/R23), OpenAPI validation and idempotency are the pkg/httpx chain declared per operation.
package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Commands are the F100 write use cases (command.Parties).
type Commands interface {
	CreateParty(ctx context.Context, in command.CreateInput) (view.Party, error)
	UpdatePartySettings(ctx context.Context, partyID uuid.UUID, s party.Settings) (view.Party, error)
	InviteToParty(ctx context.Context, partyID, invitee uuid.UUID) (view.Invitation, error)
	AcceptPartyInvitation(ctx context.Context, id uuid.UUID) (view.Party, error)
	RejectPartyInvitation(ctx context.Context, id uuid.UUID) error
	CancelPartyInvitation(ctx context.Context, id uuid.UUID) error
	JoinParty(ctx context.Context, partyID uuid.UUID) (view.Party, error)
	LeaveParty(ctx context.Context, partyID uuid.UUID) error
	KickPartyMember(ctx context.Context, partyID, student uuid.UUID) error
	TransferPartyLeader(ctx context.Context, partyID, to uuid.UUID) (view.Party, error)
	DisbandParty(ctx context.Context, partyID uuid.UUID) error
	// F101
	SetReady(ctx context.Context, partyID uuid.UUID, ready bool) error
	SelectActivity(ctx context.Context, partyID uuid.UUID, in command.SelectInput) (view.Party, error)
	StartPartySession(ctx context.Context, partyID uuid.UUID, in command.StartInput) (view.SessionStart, error)
}

// Queries are the F100 read use cases (query.Parties).
type Queries interface {
	GetMyParty(ctx context.Context) (view.Party, error)
	GetParty(ctx context.Context, id uuid.UUID) (view.Party, error)
	ListMyInvitations(ctx context.Context, status string) ([]view.Invitation, error)
	// F101
	ListEligibleActivities(ctx context.Context, partyID uuid.UUID) ([]view.Eligible, error)
}

// Handlers are the F100 use cases served over HTTP.
type Handlers struct {
	Commands Commands
	Queries  Queries
}

func op(id, method, pattern string, idem httpx.IdempotencyMode, status int) httpx.Operation {
	return httpx.Operation{ID: id, Module: "competition", Method: method, Pattern: pattern, Tenant: httpx.TenantRequired,
		Idempotency: idem, SuccessStatus: status, Guard: httpx.GuardGamePlayer}
}

// Operations of F100 (method, path, Idempotency-Key and success status of the spec; the contract test checks them
// against specs/openapi/eduworld-api.v1.yaml). Every route is a player route: game_player + X-Tenant-Id (F100-AC-23).
var Operations = map[string]httpx.Operation{
	"createParty":           op("createParty", http.MethodPost, "/multiplayer/parties", httpx.IdempotencyRequired, http.StatusCreated),
	"getMyParty":            op("getMyParty", http.MethodGet, "/multiplayer/parties/me", httpx.IdempotencyNone, http.StatusOK),
	"getParty":              op("getParty", http.MethodGet, "/multiplayer/parties/{party_id}", httpx.IdempotencyNone, http.StatusOK),
	"updatePartySettings":   op("updatePartySettings", http.MethodPatch, "/multiplayer/parties/{party_id}", httpx.IdempotencyNone, http.StatusOK),
	"leaveParty":            op("leaveParty", http.MethodPost, "/multiplayer/parties/{party_id}:leave", httpx.IdempotencyNone, http.StatusNoContent),
	"disbandParty":          op("disbandParty", http.MethodPost, "/multiplayer/parties/{party_id}:disband", httpx.IdempotencyNone, http.StatusNoContent),
	"transferPartyLeader":   op("transferPartyLeader", http.MethodPost, "/multiplayer/parties/{party_id}:transfer-leader", httpx.IdempotencyRequired, http.StatusOK),
	"kickPartyMember":       op("kickPartyMember", http.MethodPost, "/multiplayer/parties/{party_id}/members/{student_id}:kick", httpx.IdempotencyNone, http.StatusNoContent),
	"inviteToParty":         op("inviteToParty", http.MethodPost, "/multiplayer/parties/{party_id}/invitations", httpx.IdempotencyRequired, http.StatusCreated),
	"listMyInvitations":     op("listMyInvitations", http.MethodGet, "/multiplayer/invitations", httpx.IdempotencyNone, http.StatusOK),
	"acceptPartyInvitation": op("acceptPartyInvitation", http.MethodPost, "/multiplayer/invitations/{invitation_id}:accept", httpx.IdempotencyRequired, http.StatusOK),
	"rejectPartyInvitation": op("rejectPartyInvitation", http.MethodPost, "/multiplayer/invitations/{invitation_id}:reject", httpx.IdempotencyNone, http.StatusNoContent),
	"cancelPartyInvitation": op("cancelPartyInvitation", http.MethodPost, "/multiplayer/invitations/{invitation_id}:cancel", httpx.IdempotencyNone, http.StatusNoContent),
	"joinParty":             op("joinParty", http.MethodPost, "/multiplayer/parties/{party_id}:join", httpx.IdempotencyRequired, http.StatusOK),
	// F101 — Party Lobby and party sessions.
	"setPartyReady":               op("setPartyReady", http.MethodPut, "/multiplayer/parties/{party_id}/members/me/ready", httpx.IdempotencyNone, http.StatusNoContent),
	"selectPartyActivity":         op("selectPartyActivity", http.MethodPut, "/multiplayer/parties/{party_id}/activity", httpx.IdempotencyNone, http.StatusOK),
	"startPartySession":           op("startPartySession", http.MethodPost, "/multiplayer/parties/{party_id}:start", httpx.IdempotencyRequired, http.StatusCreated),
	"listPartyEligibleActivities": op("listPartyEligibleActivities", http.MethodGet, "/multiplayer/parties/{party_id}/eligible-activities", httpx.IdempotencyNone, http.StatusOK),
}

// PlayerOperations are the player routes of F100 and F101 (all of them).
var PlayerOperations = []string{"createParty", "getMyParty", "getParty", "updatePartySettings", "leaveParty", "disbandParty",
	"transferPartyLeader", "kickPartyMember", "inviteToParty", "listMyInvitations", "acceptPartyInvitation", "rejectPartyInvitation",
	"cancelPartyInvitation", "joinParty", "setPartyReady", "selectPartyActivity", "startPartySession", "listPartyEligibleActivities"}

// Register mounts every F100 and F101 operation on the server.
func Register(s *httpx.Server, h Handlers) error {
	if h.Commands == nil || h.Queries == nil {
		return errors.New("competition: the F100 handlers are required")
	}
	routes := map[string]http.HandlerFunc{
		"createParty":                 h.createParty,
		"getMyParty":                  h.getMyParty,
		"getParty":                    h.partyView(h.Queries.GetParty),
		"updatePartySettings":         h.updatePartySettings,
		"leaveParty":                  h.partyAction(h.Commands.LeaveParty),
		"disbandParty":                h.partyAction(h.Commands.DisbandParty),
		"transferPartyLeader":         h.transferPartyLeader,
		"kickPartyMember":             h.kickPartyMember,
		"inviteToParty":               h.inviteToParty,
		"listMyInvitations":           h.listMyInvitations,
		"acceptPartyInvitation":       h.acceptPartyInvitation,
		"rejectPartyInvitation":       h.invitationAction(h.Commands.RejectPartyInvitation),
		"cancelPartyInvitation":       h.invitationAction(h.Commands.CancelPartyInvitation),
		"joinParty":                   h.partyView(h.Commands.JoinParty),
		"setPartyReady":               h.setPartyReady,
		"selectPartyActivity":         h.selectPartyActivity,
		"startPartySession":           h.startPartySession,
		"listPartyEligibleActivities": h.listPartyEligibleActivities,
	}
	for _, id := range PlayerOperations {
		if err := s.Handle(Operations[id], routes[id]); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, r *http.Request, err error) {
	var ra command.RetryAfter
	if errors.As(err, &ra) {
		secs := int((ra.After + time.Second - 1) / time.Second)
		w.Header().Set("Retry-After", strconv.Itoa(max(secs, 1)))
	}
	var pe *problem.Error
	if errors.As(err, &pe) && pe.Code == problem.Unavailable && w.Header().Get("Retry-After") == "" {
		w.Header().Set("Retry-After", "1")
	}
	httpx.WriteError(w, r, err)
}

func pathUUID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return uuid.Nil, problem.Invalid(name, problem.SubInvalid, "must be a UUID")
	}
	return id, nil
}

func (h Handlers) partyView(fn func(ctx context.Context, id uuid.UUID) (view.Party, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathUUID(r, "party_id")
		if err != nil {
			fail(w, r, err)
			return
		}
		v, err := fn(r.Context(), id)
		if err != nil {
			fail(w, r, err)
			return
		}
		writeJSON(w, http.StatusOK, v.Body())
	}
}

func (h Handlers) partyAction(fn func(ctx context.Context, id uuid.UUID) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathUUID(r, "party_id")
		if err != nil {
			fail(w, r, err)
			return
		}
		if err := fn(r.Context(), id); err != nil {
			fail(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h Handlers) invitationAction(fn func(ctx context.Context, id uuid.UUID) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathUUID(r, "invitation_id")
		if err != nil {
			fail(w, r, err)
			return
		}
		if err := fn(r.Context(), id); err != nil {
			fail(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h Handlers) createParty(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Name             string  `json:"name"`
		PartyType        string  `json:"party_type"`
		Visibility       *string `json:"visibility"`
		InvitationPolicy *string `json:"invitation_policy"`
		MaxMembers       *int    `json:"max_members"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.Invalid("body", problem.SubInvalid, "must be a PartyCreate"))
		return
	}
	v, err := h.Commands.CreateParty(r.Context(), command.CreateInput{Name: b.Name, Type: b.PartyType,
		Settings: party.Settings{Visibility: b.Visibility, InvitationPolicy: b.InvitationPolicy, MaxMembers: b.MaxMembers}})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, v.Body())
}

func (h Handlers) getMyParty(w http.ResponseWriter, r *http.Request) {
	v, err := h.Queries.GetMyParty(r.Context())
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v.Body())
}

func (h Handlers) updatePartySettings(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "party_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Name             *string `json:"name"`
		Visibility       *string `json:"visibility"`
		InvitationPolicy *string `json:"invitation_policy"`
		MaxMembers       *int    `json:"max_members"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.Invalid("body", problem.SubInvalid, "must be a PartySettingsUpdate"))
		return
	}
	v, err := h.Commands.UpdatePartySettings(r.Context(), id, party.Settings{Name: b.Name, Visibility: b.Visibility,
		InvitationPolicy: b.InvitationPolicy, MaxMembers: b.MaxMembers})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v.Body())
}

func (h Handlers) transferPartyLeader(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "party_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		NewLeaderID uuid.UUID `json:"new_leader_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.NewLeaderID == uuid.Nil {
		fail(w, r, problem.Invalid("new_leader_id", problem.SubInvalid, "must be a UUID"))
		return
	}
	v, err := h.Commands.TransferPartyLeader(r.Context(), id, b.NewLeaderID)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v.Body())
}

func (h Handlers) kickPartyMember(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "party_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	student, err := pathUUID(r, "student_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := h.Commands.KickPartyMember(r.Context(), id, student); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) inviteToParty(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "party_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		InviteeID uuid.UUID `json:"invitee_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.InviteeID == uuid.Nil {
		fail(w, r, problem.Invalid("invitee_id", problem.SubInvalid, "must be a UUID"))
		return
	}
	v, err := h.Commands.InviteToParty(r.Context(), id, b.InviteeID)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, v.Body())
}

func (h Handlers) listMyInvitations(w http.ResponseWriter, r *http.Request) {
	rows, err := h.Queries.ListMyInvitations(r.Context(), r.URL.Query().Get("status"))
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(rows))
	for i, v := range rows {
		data[i] = v.Body()
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data})
}

func (h Handlers) acceptPartyInvitation(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "invitation_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Commands.AcceptPartyInvitation(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v.Body())
}
