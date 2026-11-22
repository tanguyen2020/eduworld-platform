package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/questrules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// QuestCommands are the quest write use cases of a student (command.Quests, F072).
type QuestCommands interface {
	Accept(ctx context.Context, questID uuid.UUID, partyID *uuid.UUID) (questrules.ProgressView, error)
	Abandon(ctx context.Context, questID uuid.UUID) (questrules.ProgressView, error)
	Report(ctx context.Context, questID uuid.UUID, stepID string, in command.ReportInput) (questrules.ProgressView, error)
	StartActivity(ctx context.Context, questID uuid.UUID, stepID string, gameSession *uuid.UUID) (command.ActivityStart, error)
}

// QuestQueries are the quest read use cases of a student (query.Quests, F072).
type QuestQueries interface {
	List(ctx context.Context, in query.ListInput) (query.QuestPage, error)
	Get(ctx context.Context, questID uuid.UUID) (query.QuestDetail, error)
}

// QuestOperations are the player routes of F072 (F072-AC-08: staff tokens get 403 GAME_ACCESS_STUDENT_ONLY).
var QuestOperations = []string{"listQuests", "getQuest", "acceptQuest", "abandonQuest", "reportQuestStep", "startQuestActivity"}

func (h Handlers) questRoutes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"listQuests":         h.listQuests,
		"getQuest":           h.getQuest,
		"acceptQuest":        h.acceptQuest,
		"abandonQuest":       h.abandonQuest,
		"reportQuestStep":    h.reportQuestStep,
		"startQuestActivity": h.startQuestActivity,
	}
}

var (
	stepIDRe = regexp.MustCompile(`^[a-z0-9_\-]{1,64}$`)
	anchorRe = regexp.MustCompile(`^.{1,64}$`)
)

func (h Handlers) listQuests(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit := 0
	if s := q.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			fail(w, r, problem.Invalid("limit", problem.SubInvalid, "an integer"))
			return
		}
		limit = n
	}
	out, err := h.QuestQueries.List(r.Context(), query.ListInput{QuestType: q.Get("quest_type"), Status: q.Get("status"), AreaID: q.Get("area_id"),
		Limit: limit, Cursor: q.Get("cursor")})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) getQuest(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "quest_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.QuestQueries.Get(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// optionalBody decodes an optional JSON body into v (empty body = zero value).
func optionalBody(r *http.Request, v any) error {
	if r.Body == nil {
		return nil
	}
	err := json.NewDecoder(r.Body).Decode(v)
	if err == nil || errors.Is(err, io.EOF) {
		return nil
	}
	return problem.New(problem.BadRequest, "invalid JSON body")
}

func (h Handlers) acceptQuest(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "quest_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		PartyID *uuid.UUID `json:"party_id"`
	}
	if err := optionalBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Quests.Accept(r.Context(), id, b.PartyID)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) abandonQuest(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "quest_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Quests.Abandon(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func stepParam(r *http.Request) (string, error) {
	s := chi.URLParam(r, "step_id")
	if !stepIDRe.MatchString(s) {
		return "", problem.Invalid("step_id", problem.SubInvalid, "1..64 of [a-z0-9_-]")
	}
	return s, nil
}

func (h Handlers) reportQuestStep(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "quest_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	step, err := stepParam(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		GameSessionID  uuid.UUID `json:"game_session_id"`
		AreaID         string    `json:"area_id"`
		InteractableID string    `json:"interactable_id"`
		NPCID          string    `json:"npc_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.GameSessionID == uuid.Nil {
		fail(w, r, problem.Invalid("game_session_id", problem.SubRequired, "a UUID is required"))
		return
	}
	for name, v := range map[string]string{"area_id": b.AreaID, "interactable_id": b.InteractableID, "npc_id": b.NPCID} {
		if v != "" && !anchorRe.MatchString(v) {
			fail(w, r, problem.Invalid(name, problem.SubInvalid, "at most 64 characters")) // F072-AC-28 (also the OpenAPI maxLength)
			return
		}
	}
	out, err := h.Quests.Report(r.Context(), id, step, command.ReportInput{GameSessionID: b.GameSessionID, AreaID: b.AreaID,
		InteractableID: b.InteractableID, NPCID: b.NPCID})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) startQuestActivity(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "quest_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	step, err := stepParam(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		GameSessionID *uuid.UUID `json:"game_session_id"`
	}
	if err := optionalBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Quests.StartActivity(r.Context(), id, step, b.GameSessionID)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, out)
}
