package partyrules

import (
	"slices"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/partyquest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/quest"
)

// ClueView is QuestStepClue (the render only for a card the caller may see, F103-AC-21).
type ClueView struct {
	Role             string           `json:"role"`
	Title            quest.Text       `json:"title"`
	IconKey          string           `json:"icon_key,omitempty"`
	Shared           bool             `json:"shared"`
	SharedByNickname string           `json:"shared_by_nickname,omitempty"`
	Content          *port.ClueRender `json:"content,omitempty"`
}

// CluesView is QuestStepClues.
type CluesView struct {
	QuestID uuid.UUID  `json:"quest_id"`
	StepID  string     `json:"step_id"`
	MyRoles []string   `json:"my_roles"`
	Clues   []ClueView `json:"clues"`
}

// RoleView is one assignment of QuestStepRoles (nickname only, R16).
type RoleView struct {
	Role      string    `json:"role"`
	StudentID uuid.UUID `json:"student_id"`
	Nickname  string    `json:"nickname"`
}

// RolesView is QuestStepRoles.
type RolesView struct {
	QuestID     uuid.UUID  `json:"quest_id"`
	StepID      string     `json:"step_id"`
	Assignments []RoleView `json:"assignments"`
}

// Clues builds QuestStepClues for a member (F103-AC-21): its roles and the shared cards with their render; the other
// roles appear without content (never a card not shared). s nil = a solo player holding every role (AC-24).
func Clues(s *partyquest.Session, st quest.Step, student uuid.UUID, rs map[string]port.ClueRender, names map[uuid.UUID]string) CluesView {
	out := CluesView{QuestID: uuid.Nil, StepID: st.ID, MyRoles: []string{}, Clues: []ClueView{}}
	var mine, visible []string
	if s == nil {
		for _, r := range st.Roles {
			mine = append(mine, r.Role)
		}
		visible = mine
	} else {
		out.QuestID = s.QuestID
		mine, visible = s.RolesOf(st, student), s.Visible(st, student)
	}
	out.MyRoles = append(out.MyRoles, mine...)
	for _, r := range st.Roles {
		c := ClueView{Role: r.Role, Title: r.Title, IconKey: r.IconKey}
		if s != nil {
			if sh, ok := s.Shared(st.ID, r.Role); ok {
				c.Shared, c.SharedByNickname = true, names[sh.SharedBy]
			}
		}
		if slices.Contains(visible, r.Role) {
			if render, ok := rs[r.Role]; ok {
				rr := render
				c.Content = &rr
			}
		}
		out.Clues = append(out.Clues, c)
	}
	return out
}
