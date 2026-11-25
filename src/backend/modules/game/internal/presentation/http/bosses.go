package http

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/partyrules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/partyquest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// BossCommands are the boss write use cases of a student (command.Bosses, F103).
type BossCommands interface {
	CreateSolo(ctx context.Context, bossID uuid.UUID, mode string) (partyrules.EncounterView, error)
	Leave(ctx context.Context, encounterID uuid.UUID) error
}

// BossQueries are the boss read use cases of a student (query.Bosses, F103).
type BossQueries interface {
	ListBosses(ctx context.Context, in query.BossListInput) (query.BossPage, error)
	GetBossEncounter(ctx context.Context, id uuid.UUID) (partyrules.EncounterView, error)
}

// CoopQuestCommands are the interdependent step use cases (command.PartyQuests, F103 US3).
type CoopQuestCommands interface {
	AssignRoles(ctx context.Context, questID uuid.UUID, stepID string, as []partyquest.Assignment) (partyrules.RolesView, error)
	ShareClue(ctx context.Context, questID uuid.UUID, stepID, role string) (partyrules.CluesView, error)
}

// CoopQuestQueries is getQuestStepClues (query.QuestClues).
type CoopQuestQueries interface {
	GetClues(ctx context.Context, questID uuid.UUID, stepID string) (partyrules.CluesView, error)
}

// BossOperations are the player routes of F103 (F103-AC-25: game_player, staff tokens get 403 GAME_ACCESS_STUDENT_ONLY).
var BossOperations = []string{"listBosses", "createBossEncounter", "getBossEncounter", "leaveBossEncounter", "getQuestStepClues",
	"shareQuestStepClue", "assignQuestStepRoles"}

// bossOperations are the F103 operations (method, path, X-Tenant-Id mode, Idempotency-Key, success status and guard of
// the spec; the contract test checks them against specs/openapi/eduworld-api.v1.yaml). No route takes a damage, a phase
// result, a contribution or a reward from the client (R5).
var bossOperations = []httpx.Operation{
	op("listBosses", http.MethodGet, "/game/bosses", httpx.TenantRequired, httpx.IdempotencyNone, http.StatusOK, httpx.GuardGamePlayer),
	op("createBossEncounter", http.MethodPost, "/game/bosses/{boss_id}/encounters", httpx.TenantRequired, httpx.IdempotencyRequired,
		http.StatusCreated, httpx.GuardGamePlayer),
	op("getBossEncounter", http.MethodGet, "/game/boss-encounters/{encounter_id}", httpx.TenantRequired, httpx.IdempotencyNone, http.StatusOK,
		httpx.GuardGamePlayer),
	op("leaveBossEncounter", http.MethodPost, "/game/boss-encounters/{encounter_id}:leave", httpx.TenantRequired, httpx.IdempotencyRequired,
		http.StatusNoContent, httpx.GuardGamePlayer),
	op("getQuestStepClues", http.MethodGet, "/game/quests/{quest_id}/steps/{step_id}/clues", httpx.TenantRequired, httpx.IdempotencyNone,
		http.StatusOK, httpx.GuardGamePlayer),
	op("shareQuestStepClue", http.MethodPost, "/game/quests/{quest_id}/steps/{step_id}/clues/{role}:share", httpx.TenantRequired,
		httpx.IdempotencyRequired, http.StatusOK, httpx.GuardGamePlayer),
	op("assignQuestStepRoles", http.MethodPut, "/game/quests/{quest_id}/steps/{step_id}/roles", httpx.TenantRequired,
		httpx.IdempotencyRequired, http.StatusOK, httpx.GuardGamePlayer),
}

func init() {
	for _, o := range bossOperations {
		Operations[o.ID] = o
	}
	PlayerOperations = append(PlayerOperations, BossOperations...)
}

func (h Handlers) bossRoutes() map[string]http.HandlerFunc {
	return map[string]http.HandlerFunc{
		"listBosses":           h.listBosses,
		"createBossEncounter":  h.createBossEncounter,
		"getBossEncounter":     h.getBossEncounter,
		"leaveBossEncounter":   h.leaveBossEncounter,
		"getQuestStepClues":    h.getQuestStepClues,
		"shareQuestStepClue":   h.shareQuestStepClue,
		"assignQuestStepRoles": h.assignQuestStepRoles,
	}
}

func (h Handlers) listBosses(w http.ResponseWriter, r *http.Request) {
	limit, err := limitParam(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	q := r.URL.Query()
	out, err := h.BossQueries.ListBosses(r.Context(), query.BossListInput{AreaID: q.Get("area_id"), Limit: limit, Cursor: q.Get("cursor")})
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) createBossEncounter(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "boss_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Mode string `json:"mode"`
	}
	if err := optionalBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.Bosses.CreateSolo(r.Context(), id, b.Mode)
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusCreated, out)
}

func (h Handlers) getBossEncounter(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "encounter_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.BossQueries.GetBossEncounter(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) leaveBossEncounter(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "encounter_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := h.Bosses.Leave(r.Context(), id); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// questStep reads the quest id and step id of a clue route (step ids ≤ 64, the spec pattern).
func questStep(r *http.Request) (uuid.UUID, string, error) {
	id, err := pathUUID(r, "quest_id")
	if err != nil {
		return uuid.Nil, "", err
	}
	step := chi.URLParam(r, "step_id")
	if step == "" || len(step) > 64 {
		return uuid.Nil, "", problem.Invalid("step_id", problem.SubInvalid, "1 to 64 characters")
	}
	return id, step, nil
}

func (h Handlers) getQuestStepClues(w http.ResponseWriter, r *http.Request) {
	id, step, err := questStep(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	out, err := h.CoopQuestQueries.GetClues(r.Context(), id, step)
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) shareQuestStepClue(w http.ResponseWriter, r *http.Request) {
	id, step, err := questStep(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	role := chi.URLParam(r, "role")
	if role == "" || len(role) > 32 {
		fail(w, r, problem.Invalid("role", problem.SubInvalid, "1 to 32 characters"))
		return
	}
	out, err := h.CoopQuests.ShareClue(r.Context(), id, step, role)
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) assignQuestStepRoles(w http.ResponseWriter, r *http.Request) {
	id, step, err := questStep(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Assignments []struct {
			Role      string    `json:"role"`
			StudentID uuid.UUID `json:"student_id"`
		} `json:"assignments"`
	}
	if err := decodeBody(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	as := make([]partyquest.Assignment, 0, len(b.Assignments))
	for _, a := range b.Assignments {
		as = append(as, partyquest.Assignment{Role: a.Role, StudentID: a.StudentID})
	}
	out, err := h.CoopQuests.AssignRoles(r.Context(), id, step, as)
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, lang(r))
	writeJSON(w, http.StatusOK, out)
}
