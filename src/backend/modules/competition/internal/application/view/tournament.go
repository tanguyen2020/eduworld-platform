package view

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/challenge"
	trn "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/tournament"
)

func ts(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC()
}

func idOrNil(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return *id
}

// Challenge is the Challenge of the contract (portal, F107-AC-06): the request fields, status, the audience size and
// the participants; ids only, never a student name (the portal resolves names from identity).
func Challenge(c port.ChallengeRow, audience, participants int) map[string]any {
	targets := []map[string]any{}
	if c.Grade != nil {
		targets = append(targets, map[string]any{"target_type": challenge.TargetGrade, "grade_level": *c.Grade})
	}
	if len(c.Listed) > 0 {
		targets = append(targets, map[string]any{"target_type": challenge.TargetStudents, "student_ids": c.Listed})
	}
	exclude := c.Exclude
	if exclude == nil {
		exclude = []uuid.UUID{}
	}
	matches := c.MatchIDs
	if matches == nil {
		matches = []uuid.UUID{}
	}
	params := c.Params
	if params == nil {
		params = map[string]any{}
	}
	out := map[string]any{"challenge_id": c.ID, "tenant_id": c.Tenant, "template_id": c.TemplateConfigID, "template_version_id": c.TemplateVersionID,
		"kind": c.Kind, "format": c.Format, "name": c.Name, "school_id": c.School, "targets": targets, "exclude_student_ids": exclude,
		"starts_at": c.StartsAt.UTC(), "ends_at": c.EndsAt.UTC(), "participation": challenge.ParticipationOK, "params": params,
		"status": c.Status, "created_by": c.CreatedBy, "tournament_id": idOrNil(c.TournamentID), "match_ids": matches,
		"audience_count": audience, "participant_count": participants, "cancel_reason": c.CancelReason, "created_at": c.CreatedAt.UTC()}
	if c.QuestionSetVersionID != nil {
		out["question_set_version_id"] = *c.QuestionSetVersionID
	}
	return out
}

// CompetitionChallenge is the challenge a student sees (F107-AC-27): no audience, no other student.
func CompetitionChallenge(c port.ChallengeRow) map[string]any {
	return map[string]any{"challenge_id": c.ID, "name": c.Name, "kind": c.Kind, "format": c.Format, "starts_at": c.StartsAt.UTC(),
		"ends_at": c.EndsAt.UTC(), "join_window_opens_at": c.JoinOpensAt.UTC(), "status": c.Status, "tournament_id": idOrNil(c.TournamentID)}
}

// Template is the ChallengeTemplate of the contract.
func Template(t port.TemplateRow) map[string]any {
	tpl := t.Template
	grades := tpl.GradeLevels
	allowed := tpl.AllowedParams
	if allowed == nil {
		allowed = []string{}
	}
	out := map[string]any{"template_id": t.ConfigID, "version": t.VersionNo, "template_version_id": t.VersionID, "kind": tpl.Kind,
		"format": tpl.Format, "name": tpl.Name, "description": tpl.Description, "mode": tpl.Mode, "grade_levels": grades,
		"allowed_params": allowed, "reward_summary": tpl.RewardSummary}
	if tpl.Tournament != nil {
		out["tournament_format"] = tpl.Tournament.Format
		out["participant_type"] = tpl.Tournament.ParticipantType
	}
	return out
}

// Tournament is the Tournament of the contract (F107-AC-25): myEntry for a student, nil for staff.
func Tournament(t port.TournamentRow, entries int, myEntry map[string]any, student bool) map[string]any {
	var grade any // the grade of a grade tournament: the first grade (the grade target of its challenge, never empty)
	if len(t.Grades) > 0 {
		grade = t.Grades[0]
	}
	scope := t.OwnerID
	out := map[string]any{"tournament_id": t.ID, "template_id": t.TemplateConfigID, "name": t.Name, "format": t.Format,
		"scope_type": t.OwnerScope, "scope_id": scope, "subject_id": t.SubjectID, "grade_level": grade, "grade_levels": t.Grades,
		"registration_open_at": t.RegOpenAt.UTC(), "registration_closes_at": t.RegCloseAt.UTC(), "check_in_opens_at": t.CheckInOpensAt.UTC(),
		"starts_at": t.StartsAt.UTC(), "max_entries": t.MaxEntries, "status": t.Status, "entry_count": entries,
		"challenge_id": idOrNil(t.ChallengeID), "school_id": idOrNil(t.School), "participant_type": t.ParticipantType, "team_size": t.TeamSize,
		"mode": t.Mode, "seeding_policy": t.SeedingPolicy, "auto_confirm": t.AutoConfirm, "reward_summary": t.RewardSummary,
		"cancel_reason": t.CancelReason}
	if student {
		out["my_entry"] = myEntry
	}
	return out
}

// Player is the TournamentPlayer of the contract: nickname/avatar only (R16).
func Player(id uuid.UUID, cards map[uuid.UUID]port.Card) map[string]any {
	p := map[string]any{"student_id": id, "nickname": nil}
	if c, ok := cards[id]; ok {
		if c.Nickname != nil {
			p["nickname"] = *c.Nickname
		}
		if len(c.Avatar) > 0 {
			var a any
			if json.Unmarshal(c.Avatar, &a) == nil {
				p["avatar_config"] = a
			}
		}
	}
	return p
}

// Entry is the TournamentEntry of the contract.
func Entry(e port.EntryRow, cards map[uuid.UUID]port.Card, groupCode string, rank *int) map[string]any {
	players := make([]map[string]any, 0, len(e.Members))
	for _, m := range e.Members {
		players = append(players, Player(m, cards))
	}
	var group any
	if groupCode != "" {
		group = groupCode
	}
	var r any
	if rank != nil {
		r = *rank
	}
	return map[string]any{"entry_id": e.ID, "tournament_id": e.Tournament, "status": e.Status, "seed": e.Seed, "group_code": group,
		"party_id": idOrNil(e.Party), "players": players, "checked_in_at": ts(e.CheckedInAt), "final_rank": r}
}

// Node is the BracketNode of the contract.
func Node(n *trn.Node, groupCode string) map[string]any {
	var group any
	if groupCode != "" {
		group = groupCode
	}
	return map[string]any{"node_id": n.ID, "bracket": n.Bracket, "round": n.Round, "position": n.Position, "group_code": group,
		"entry_a_id": idOrNil(n.A), "entry_b_id": idOrNil(n.B), "winner_entry_id": idOrNil(n.Winner), "is_draw": n.Draw,
		"match_id": idOrNil(n.MatchID), "scheduled_at": ts(n.ScheduledAt), "next_win_node_id": idOrNil(n.NextWin),
		"next_lose_node_id": idOrNil(n.NextLose), "status": n.Status, "confirm_after": ts(n.ConfirmAfter),
		"decision_required": n.DecisionRequired}
}

// Group is the TournamentGroup of the contract.
func Group(code string, rows []trn.Row) map[string]any {
	st := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		st = append(st, map[string]any{"entry_id": r.Entry, "rank": r.Rank, "points": r.Points, "wins": r.Wins, "draws": r.Draws,
			"losses": r.Losses, "correct_diff": r.CorrectDiff})
	}
	return map[string]any{"group_code": code, "standings": st}
}
