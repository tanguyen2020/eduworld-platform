package natscmd

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/partyrules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Boss command types of the realtime gateway (AsyncAPI boss.*, routed to ew.cmd.game.<type>, F103).
const (
	Module          = "game"
	TypeBossReady   = "boss.encounter.ready"
	TypeBossAnswer  = "boss.answer.submit"
	maxResponseSize = 16 << 10
)

// BossPlayer is the use case behind the boss commands (command.Bosses).
type BossPlayer interface {
	Ready(ctx context.Context, encounterID uuid.UUID, ready bool) (partyrules.EncounterView, error)
	Answer(ctx context.Context, in command.AnswerInput) (command.AnswerReply, error)
}

// studentContext builds the request context of a boss command (F103 T037): the Principal of the student and the
// TenantScope of the ticket, from the Ew-* headers only — never from the payload (F103-AC-05: the answerer is the one of
// the session). Only the NATS user of binary realtime publishes ew.cmd.> (SECURITY_ARCHITECTURE §8.1) and the gateway
// accepts game tickets of students only (R23).
func studentContext(ctx context.Context, c rt.Caller) (context.Context, error) {
	tenant := c.TenantID
	p := authz.NewPrincipal(authz.PrincipalData{UserID: c.UserID, UserType: authz.UserTypeStudent, TenantID: &tenant})
	sc, err := authz.ScopeFor(p)
	if err != nil {
		return ctx, err
	}
	return tenancy.WithScope(authz.WithPrincipal(ctx, p), sc), nil
}

// BossReady handles boss.encounter.ready (F103-AC-04): payload {encounter_id, ready}; the reply is the encounter state.
func BossReady(h BossPlayer) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var in struct {
			EncounterID uuid.UUID `json:"encounter_id"`
			Ready       *bool     `json:"ready"`
		}
		if err := json.Unmarshal(payload, &in); err != nil || in.EncounterID == uuid.Nil || in.Ready == nil {
			return rt.Fail(string(problem.ValidationFailed), false)
		}
		ctx, err := studentContext(ctx, c)
		if err != nil {
			return rt.Fail("UNAUTHENTICATED", false)
		}
		v, err := h.Ready(ctx, in.EncounterID, *in.Ready)
		if err != nil {
			return failure(ctx, err)
		}
		return rt.OK(v)
	}
}

// BossAnswer handles boss.answer.submit (F103-AC-05, AC-06): the server time stamps the answer; the reply (to the
// answerer only) says whether it was correct and counted.
func BossAnswer(h BossPlayer) rt.CommandHandler {
	return func(ctx context.Context, c rt.Caller, payload json.RawMessage) rt.Reply {
		var in struct {
			EncounterID        uuid.UUID       `json:"encounter_id"`
			PhaseID            string          `json:"phase_id"`
			RequirementID      string          `json:"requirement_id"`
			QuestionInstanceID string          `json:"question_instance_id"`
			Response           json.RawMessage `json:"response"`
		}
		if len(payload) > maxResponseSize {
			return rt.Fail(string(problem.ValidationFailed), false)
		}
		if err := json.Unmarshal(payload, &in); err != nil || in.EncounterID == uuid.Nil || in.PhaseID == "" || len(in.PhaseID) > 64 ||
			in.RequirementID == "" || len(in.RequirementID) > 64 || in.QuestionInstanceID == "" || len(in.QuestionInstanceID) > 64 ||
			len(in.Response) == 0 {
			return rt.Fail(string(problem.ValidationFailed), false)
		}
		ctx, err := studentContext(ctx, c)
		if err != nil {
			return rt.Fail("UNAUTHENTICATED", false)
		}
		out, err := h.Answer(ctx, command.AnswerInput{EncounterID: in.EncounterID, PhaseID: in.PhaseID, RequirementID: in.RequirementID,
			QuestionInstanceID: in.QuestionInstanceID, Response: in.Response})
		if err != nil {
			return failure(ctx, err)
		}
		return rt.OK(out)
	}
}
