package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/club"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ClubCommands are the write use cases of the clubs (command.Clubs).
type ClubCommands interface {
	Create(ctx context.Context, in command.ClubInput) (command.ClubView, error)
	Update(ctx context.Context, id uuid.UUID, ifMatch int64, in command.ClubInput) (command.ClubView, error)
	Approve(ctx context.Context, id uuid.UUID) (command.ClubView, error)
	Leave(ctx context.Context, id uuid.UUID) error
	Archive(ctx context.Context, id uuid.UUID) error
	SetRole(ctx context.Context, id, user uuid.UUID, role string) (write.ClubMember, error)
	Remove(ctx context.Context, id, user uuid.UUID) error
	RemoveByStaff(ctx context.Context, id, user uuid.UUID, reason string) error
	RequestJoin(ctx context.Context, id uuid.UUID) (write.ClubRequest, error)
	DecideRequest(ctx context.Context, requestID uuid.UUID, accept bool) (write.ClubMember, error)
	Invite(ctx context.Context, id uuid.UUID, invitees []uuid.UUID) ([]write.ClubInvitation, error)
	DecideInvitation(ctx context.Context, invitationID uuid.UUID, accept bool) (write.ClubMember, error)
	CreateGoal(ctx context.Context, id uuid.UUID, in command.GoalInput, loc *time.Location) (write.ClubGoal, error)
	CreateEvent(ctx context.Context, id uuid.UUID, in command.EventInput) (write.ClubEvent, error)
}

// ClubQueries are the read use cases of the clubs (query.Clubs).
type ClubQueries interface {
	List(ctx context.Context, in query.ClubListInput) (query.ClubPage, error)
	Get(ctx context.Context, id uuid.UUID) (read.ClubRow, bool, error)
	Members(ctx context.Context, id uuid.UUID, cursor string, limit int) (query.MemberPage, error)
	StaffMembers(ctx context.Context, id uuid.UUID, cursor string, limit int) (query.MemberPage, error)
	Requests(ctx context.Context, id uuid.UUID, cursor string, limit int) (query.RequestPage, error)
	Goals(ctx context.Context, id uuid.UUID) ([]read.ClubGoalRow, error)
	Events(ctx context.Context, id uuid.UUID, cursor string, limit int) (query.EventPage, error)
}

// ClubHandlers serve the club operations (F120 US4). Player routes carry game_player; approveClub is a portal route
// (portal_staff); listClubs, getClub and archiveClub serve both (students through the game client, School Admins
// through the portal — x-portal-action) and the use case refuses any other caller (R7, R23: staff never joins a club or
// its channel).
type ClubHandlers struct {
	Commands ClubCommands
	Queries  ClubQueries
	// WeekLoc is the timezone of the club goal weeks (policy timezone, ICT).
	WeekLoc *time.Location
}

// ClubOperations are the F120 club operations.
var ClubOperations = []string{"listClubs", "createClub", "getClub", "updateClub", "approveClub", "leaveClub", "archiveClub",
	"listClubMembers", "updateClubMemberRole", "removeClubMember", "listClubJoinRequests", "requestJoinClub", "acceptClubJoinRequest",
	"rejectClubJoinRequest", "inviteToClub", "acceptClubInvitation", "declineClubInvitation", "listClubGoals", "createClubGoal",
	"listClubEvents", "createClubEvent", "listSchoolClubMembers", "removeSchoolClubMember"}

// mixedClubOperations serve students (game client) and staff (portal): the use case checks the caller (x-portal-action).
var mixedClubOperations = map[string]bool{"listClubs": true, "getClub": true, "archiveClub": true}

// staffClubOperations are portal-only routes of school staff (F122 T061).
var staffClubOperations = map[string]bool{"listSchoolClubMembers": true, "removeSchoolClubMember": true}

func init() {
	for _, o := range []httpx.Operation{
		{ID: "listClubs", Method: http.MethodGet, Pattern: "/social/clubs", SuccessStatus: http.StatusOK},
		{ID: "createClub", Method: http.MethodPost, Pattern: "/social/clubs", Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusCreated},
		{ID: "getClub", Method: http.MethodGet, Pattern: "/social/clubs/{club_id}", SuccessStatus: http.StatusOK},
		{ID: "updateClub", Method: http.MethodPatch, Pattern: "/social/clubs/{club_id}", SuccessStatus: http.StatusOK},
		{ID: "approveClub", Method: http.MethodPost, Pattern: "/social/clubs/{club_id}:approve", Idempotency: httpx.IdempotencyRequired,
			SuccessStatus: http.StatusOK},
		{ID: "leaveClub", Method: http.MethodPost, Pattern: "/social/clubs/{club_id}:leave", SuccessStatus: http.StatusNoContent},
		{ID: "archiveClub", Method: http.MethodPost, Pattern: "/social/clubs/{club_id}:archive", Idempotency: httpx.IdempotencyRequired,
			SuccessStatus: http.StatusNoContent},
		{ID: "listClubMembers", Method: http.MethodGet, Pattern: "/social/clubs/{club_id}/members", SuccessStatus: http.StatusOK},
		{ID: "updateClubMemberRole", Method: http.MethodPatch, Pattern: "/social/clubs/{club_id}/members/{user_id}", SuccessStatus: http.StatusOK},
		{ID: "removeClubMember", Method: http.MethodDelete, Pattern: "/social/clubs/{club_id}/members/{user_id}", SuccessStatus: http.StatusNoContent},
		{ID: "listClubJoinRequests", Method: http.MethodGet, Pattern: "/social/clubs/{club_id}/join-requests", SuccessStatus: http.StatusOK},
		{ID: "requestJoinClub", Method: http.MethodPost, Pattern: "/social/clubs/{club_id}/join-requests", Idempotency: httpx.IdempotencyRequired,
			SuccessStatus: http.StatusCreated},
		{ID: "acceptClubJoinRequest", Method: http.MethodPost, Pattern: "/social/club-join-requests/{request_id}:accept",
			Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK},
		{ID: "rejectClubJoinRequest", Method: http.MethodPost, Pattern: "/social/club-join-requests/{request_id}:reject", SuccessStatus: http.StatusNoContent},
		{ID: "inviteToClub", Method: http.MethodPost, Pattern: "/social/clubs/{club_id}/invitations", Idempotency: httpx.IdempotencyRequired,
			SuccessStatus: http.StatusCreated},
		{ID: "acceptClubInvitation", Method: http.MethodPost, Pattern: "/social/club-invitations/{invitation_id}:accept",
			Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusOK},
		{ID: "declineClubInvitation", Method: http.MethodPost, Pattern: "/social/club-invitations/{invitation_id}:decline",
			SuccessStatus: http.StatusNoContent},
		{ID: "listClubGoals", Method: http.MethodGet, Pattern: "/social/clubs/{club_id}/goals", SuccessStatus: http.StatusOK},
		{ID: "createClubGoal", Method: http.MethodPost, Pattern: "/social/clubs/{club_id}/goals", Idempotency: httpx.IdempotencyRequired,
			SuccessStatus: http.StatusCreated},
		{ID: "listClubEvents", Method: http.MethodGet, Pattern: "/social/clubs/{club_id}/events", SuccessStatus: http.StatusOK},
		{ID: "createClubEvent", Method: http.MethodPost, Pattern: "/social/clubs/{club_id}/events", Idempotency: httpx.IdempotencyRequired,
			SuccessStatus: http.StatusCreated},
		// Staff routes of F122 T061 (portal, /tenant/: GuardPortalStaff added by httpx).
		{ID: "listSchoolClubMembers", Method: http.MethodGet, Pattern: "/tenant/social/clubs/{club_id}/members", SuccessStatus: http.StatusOK},
		{ID: "removeSchoolClubMember", Method: http.MethodPost, Pattern: "/tenant/social/clubs/{club_id}/members/{user_id}:remove",
			Idempotency: httpx.IdempotencyRequired, SuccessStatus: http.StatusNoContent},
	} {
		o.Module, o.Tenant = "chat", httpx.TenantRequired
		switch {
		case o.ID == "approveClub", staffClubOperations[o.ID]:
			o.Guard = httpx.GuardPortalStaff
		case mixedClubOperations[o.ID]:
			o.Guard = httpx.GuardNone
		default:
			o.Guard = httpx.GuardGamePlayer
		}
		Operations[o.ID] = o
	}
}

func registerClubs(s *httpx.Server, h *ClubHandlers) error {
	if h.Commands == nil || h.Queries == nil {
		return errors.New("chat: the club handlers are required")
	}
	routes := map[string]http.HandlerFunc{"listClubs": h.list, "createClub": h.create, "getClub": h.get, "updateClub": h.update,
		"approveClub": h.approve, "leaveClub": h.leave, "archiveClub": h.archive, "listClubMembers": h.members,
		"updateClubMemberRole": h.setRole, "removeClubMember": h.remove, "listClubJoinRequests": h.requests,
		"requestJoinClub": h.requestJoin, "acceptClubJoinRequest": h.decideRequest(true), "rejectClubJoinRequest": h.decideRequest(false),
		"inviteToClub": h.invite, "acceptClubInvitation": h.decideInvitation(true), "declineClubInvitation": h.decideInvitation(false),
		"listClubGoals": h.goals, "createClubGoal": h.createGoal, "listClubEvents": h.events, "createClubEvent": h.createEvent,
		"listSchoolClubMembers": h.staffMembers, "removeSchoolClubMember": h.staffRemove}
	for _, id := range ClubOperations {
		if err := s.Handle(Operations[id], routes[id]); err != nil {
			return err
		}
	}
	return nil
}

func clubRow(r read.ClubRow, staff bool) map[string]any {
	return command.ClubBody(command.ClubView{Club: r.Club, MyRole: r.MyRole, Staff: staff})
}

func (h ClubHandlers) list(w http.ResponseWriter, r *http.Request) {
	in := query.ClubListInput{Scope: r.URL.Query().Get("scope"), Cursor: r.URL.Query().Get("cursor"), Limit: limitOf(r)}
	if t := r.URL.Query().Get("club_type"); t != "" {
		ct := club.Type(t)
		in.Type = &ct
	}
	if s := r.URL.Query().Get("status"); s != "" {
		in.Status = &s
	}
	school, err := optUUID(r, "school_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	in.School = school
	p, err := h.Queries.List(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(p.Data))
	for i, row := range p.Data {
		data[i] = clubRow(row, p.Staff)
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": p.Page})
}

// clubInput decodes ClubCreate / ClubUpdate strictly: an unknown field (class_id, add_whole_class — ADR-0095) → 422.
func clubInput(r *http.Request) (command.ClubInput, error) {
	var b struct {
		ClubType    string     `json:"club_type"`
		Name        *string    `json:"name"`
		Description *string    `json:"description"`
		IconID      *string    `json:"icon_id"`
		SubjectID   *uuid.UUID `json:"subject_id"`
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return command.ClubInput{}, problem.Invalid("body", problem.SubInvalid, "ClubCreate fields only (no class option, ADR-0095)")
	}
	return command.ClubInput{Type: club.Type(b.ClubType), Name: b.Name, Description: b.Description, IconID: b.IconID, SubjectID: b.SubjectID}, nil
}

func (h ClubHandlers) create(w http.ResponseWriter, r *http.Request) {
	in, err := clubInput(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Commands.Create(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, command.ClubBody(v))
}

func (h ClubHandlers) get(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "club_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	row, staff, err := h.Queries.Get(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, clubRow(row, staff))
}

func (h ClubHandlers) update(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "club_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	version, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	in, err := clubInput(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Commands.Update(r.Context(), id, version, in)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, command.ClubBody(v))
}

func (h ClubHandlers) approve(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "club_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Commands.Approve(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, command.ClubBody(v))
}

func (h ClubHandlers) noContent(fn func(ctx context.Context, id uuid.UUID) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathUUID(r, "club_id")
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

func (h ClubHandlers) leave(w http.ResponseWriter, r *http.Request) {
	h.noContent(h.Commands.Leave)(w, r)
}
func (h ClubHandlers) archive(w http.ResponseWriter, r *http.Request) {
	h.noContent(h.Commands.Archive)(w, r)
}

func (h ClubHandlers) members(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "club_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.Queries.Members(r.Context(), id, r.URL.Query().Get("cursor"), limitOf(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeMembers(w, id, p)
}

// writeMembers renders a page of ClubMember (nicknames only, R16).
func writeMembers(w http.ResponseWriter, id uuid.UUID, p query.MemberPage) {
	data := make([]map[string]any, len(p.Data))
	for i, m := range p.Data {
		nick := ""
		if m.Nickname != nil {
			nick = *m.Nickname
		}
		data[i] = map[string]any{"club_id": id, "user_id": m.UserID, "nickname": nick, "role": m.Role, "joined_at": m.JoinedAt.UTC()}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": p.Page})
}

func (h ClubHandlers) setRole(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "club_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	user, err := pathUUID(r, "user_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.Invalid("role", problem.SubInvalid, "officer or member"))
		return
	}
	m, err := h.Commands.SetRole(r.Context(), id, user, b.Role)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, command.MemberBody(m, ""))
}

func (h ClubHandlers) remove(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "club_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	user, err := pathUUID(r, "user_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := h.Commands.Remove(r.Context(), id, user); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h ClubHandlers) requests(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "club_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.Queries.Requests(r.Context(), id, r.URL.Query().Get("cursor"), limitOf(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(p.Data))
	for i, x := range p.Data {
		m := map[string]any{"request_id": x.ID, "club_id": x.ClubID, "requester_id": x.RequesterID, "status": x.Status,
			"created_at": x.CreatedAt.UTC()}
		if x.Nickname != nil {
			m["requester_nickname"] = *x.Nickname
		}
		data[i] = m
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": p.Page})
}

func (h ClubHandlers) requestJoin(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "club_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	req, err := h.Commands.RequestJoin(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, command.RequestBody(req))
}

func (h ClubHandlers) decideRequest(accept bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathUUID(r, "request_id")
		if err != nil {
			fail(w, r, err)
			return
		}
		m, err := h.Commands.DecideRequest(r.Context(), id, accept)
		if err != nil {
			fail(w, r, err)
			return
		}
		if !accept {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, http.StatusOK, command.MemberBody(m, ""))
	}
}

func (h ClubHandlers) invite(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "club_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		InviteeIDs []uuid.UUID `json:"invitee_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.Invalid("invitee_ids", problem.SubInvalid, "1..50 student ids"))
		return
	}
	out, err := h.Commands.Invite(r.Context(), id, b.InviteeIDs)
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(out))
	for i, v := range out {
		data[i] = command.InvitationBody(v)
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": data})
}

func (h ClubHandlers) decideInvitation(accept bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathUUID(r, "invitation_id")
		if err != nil {
			fail(w, r, err)
			return
		}
		m, err := h.Commands.DecideInvitation(r.Context(), id, accept)
		if err != nil {
			fail(w, r, err)
			return
		}
		if !accept {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, http.StatusOK, command.MemberBody(m, ""))
	}
}

func (h ClubHandlers) goals(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "club_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	rows, err := h.Queries.Goals(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(rows))
	for i, g := range rows {
		data[i] = command.GoalBody(write.ClubGoal{ID: g.ID, ClubID: g.ClubID, Title: g.Title, Metric: g.Metric, SubjectID: g.SubjectID,
			Target: g.Target, Progress: g.Progress, WeekStart: g.WeekStart, Status: g.Status}, g.MyContribution)
	}
	// accepted_metrics: the metrics createClubGoal accepts now, the very list it checks (delta F112-4).
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "accepted_metrics": club.AcceptedMetrics})
}

func (h ClubHandlers) createGoal(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "club_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Title     *string    `json:"title"`
		Metric    string     `json:"metric"`
		SubjectID *uuid.UUID `json:"subject_id"`
		Target    int        `json:"target"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.Invalid("metric", problem.SubInvalid, "a ClubGoalCreate body"))
		return
	}
	g, err := h.Commands.CreateGoal(r.Context(), id, command.GoalInput{Title: b.Title, Metric: b.Metric, SubjectID: b.SubjectID,
		Target: b.Target}, h.WeekLoc)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, command.GoalBody(g, 0))
}

func (h ClubHandlers) events(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "club_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.Queries.Events(r.Context(), id, r.URL.Query().Get("cursor"), limitOf(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	data := make([]map[string]any, len(p.Data))
	for i, e := range p.Data {
		data[i] = command.EventBody(write.ClubEvent{ID: e.ID, ClubID: e.ClubID, Title: e.Title, Description: e.Description, Kind: e.Kind,
			StartsAt: e.StartsAt, EndsAt: e.EndsAt})
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": data, "page": p.Page})
}

func (h ClubHandlers) createEvent(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "club_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Title                string     `json:"title"`
		Description          *string    `json:"description"`
		Kind                 string     `json:"event_kind"`
		TournamentTemplateID *uuid.UUID `json:"tournament_template_id"`
		StartsAt             time.Time  `json:"starts_at"`
		EndsAt               *time.Time `json:"ends_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.Invalid("title", problem.SubInvalid, "a ClubEventCreate body"))
		return
	}
	e, err := h.Commands.CreateEvent(r.Context(), id, command.EventInput{Title: b.Title, Description: b.Description, Kind: b.Kind,
		TournamentTemplateID: b.TournamentTemplateID, StartsAt: b.StartsAt, EndsAt: b.EndsAt})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, command.EventBody(e))
}

func (h ClubHandlers) staffMembers(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "club_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.Queries.StaffMembers(r.Context(), id, r.URL.Query().Get("cursor"), limitOf(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	writeMembers(w, id, p)
}

func (h ClubHandlers) staffRemove(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "club_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	user, err := pathUUID(r, "user_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.Invalid("reason", problem.SubRequired, "required"))
		return
	}
	if err := h.Commands.RemoveByStaff(r.Context(), id, user, b.Reason); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
