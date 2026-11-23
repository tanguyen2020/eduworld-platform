package contract

import (
	"regexp"
	"slices"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"

	gamehttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/presentation/http"
)

// challengeBanned are the request fields a client could use to claim a challenge result (ADR-0016, R5).
var challengeBanned = regexp.MustCompile(`^(score|.*_score|lives|lives_left|status|result|results|completed|.*_completed|succeeded|correct.*|` +
	`goal_progress|deadline_at|ended_at|reward.*|first_clear)$`)

// F076-AC-16 (TC-CHL-006): no challenge route takes a score, lives, a status, a result or a completion from the client
// (body or query); the only body field of start is the difficulty (and party_id, refused). Every challenge route is a
// player route of module game (game_player).
func TestNoClientChallengeResult(t *testing.T) {
	doc := spec(t)
	checked := 0
	for _, item := range doc.Paths.Map() {
		for _, op := range item.Operations() {
			if !slices.Contains(gamehttp.ChallengeOperations, op.OperationID) {
				continue
			}
			checked++
			if !slices.Contains(gamehttp.PlayerOperations, op.OperationID) {
				t.Errorf("%s is not a player route", op.OperationID)
			}
			fields := map[string]bool{}
			for _, p := range op.Parameters {
				if p.Value != nil && p.Value.In != "header" {
					fields[p.Value.Name] = true
				}
			}
			if op.RequestBody != nil && op.RequestBody.Value != nil {
				for _, mt := range op.RequestBody.Value.Content {
					requestFields(mt.Schema, map[*openapi3.Schema]bool{}, fields)
				}
			}
			for f := range fields {
				if challengeBanned.MatchString(f) {
					t.Errorf("%s: request field %q lets the client claim a challenge result", op.OperationID, f)
				}
			}
			if op.OperationID == "startGameChallenge" {
				for f := range fields {
					if !slices.Contains([]string{"difficulty", "party_id", "challenge_id"}, f) {
						t.Errorf("startGameChallenge: unexpected request field %q", f)
					}
				}
			}
		}
	}
	if checked != len(gamehttp.ChallengeOperations) {
		t.Fatalf("checked %d of %d challenge operations", checked, len(gamehttp.ChallengeOperations))
	}
}
