package http

import (
	"encoding/json"
	"net/http"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/party"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// setPartyReady serves PUT /multiplayer/parties/{party_id}/members/me/ready (F101-AC-01, AC-02).
func (h Handlers) setPartyReady(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "party_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Ready *bool `json:"ready"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.Ready == nil {
		fail(w, r, problem.Invalid("ready", problem.SubRequired, "must be a boolean"))
		return
	}
	if err := h.Commands.SetReady(r.Context(), id, *b.Ready); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// selectPartyActivity serves PUT /multiplayer/parties/{party_id}/activity (F101-AC-03, AC-04).
func (h Handlers) selectPartyActivity(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "party_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		ActivityType    string `json:"activity_type"`
		ActivityRef     string `json:"activity_ref"`
		GameMode        string `json:"game_mode"`
		ExpectedVersion *int64 `json:"expected_version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.ExpectedVersion == nil {
		fail(w, r, problem.Invalid("body", problem.SubInvalid, "must be a PartyActivitySelect"))
		return
	}
	v, err := h.Commands.SelectActivity(r.Context(), id, command.SelectInput{Expected: *b.ExpectedVersion,
		Selection: party.Selection{Type: b.ActivityType, Ref: b.ActivityRef, GameMode: b.GameMode}})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v.Body())
}

// startPartySession serves POST /multiplayer/parties/{party_id}:start (F101-AC-07…AC-10; Idempotency-Key by httpx).
func (h Handlers) startPartySession(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "party_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		ExpectedVersion *int64 `json:"expected_version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil || b.ExpectedVersion == nil {
		fail(w, r, problem.Invalid("expected_version", problem.SubRequired, "must be an integer"))
		return
	}
	v, err := h.Commands.StartPartySession(r.Context(), id, command.StartInput{Expected: *b.ExpectedVersion})
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, v.Body())
}

// listPartyEligibleActivities serves GET /multiplayer/parties/{party_id}/eligible-activities (F101-AC-05).
func (h Handlers) listPartyEligibleActivities(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "party_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	rows, err := h.Queries.ListEligibleActivities(r.Context(), id)
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
