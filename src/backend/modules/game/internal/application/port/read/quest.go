package read

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
)

// QuestKey is the keyset position of listQuests: (status rank, code, quest_id), compared byte-wise on code.
type QuestKey struct {
	Rank    int
	Code    string
	QuestID uuid.UUID
}

// QuestListQuery selects one page of the quests of a student (F072-AC-01) in SQL: the quests whose latest progress is
// active (on the pinned version) and the published quests assigned to the tenant for the grade whose availability
// (quest.Decide: subject, window, discovery trigger) and repeat state (quest.CanRepeat) make them listed, filtered,
// ordered by (status rank, code, quest_id) after After, at most Limit rows.
type QuestListQuery struct {
	Tenant, Student uuid.UUID
	Grade           int
	// EnabledSubjects are the subject codes the school turned on; nil = every subject.
	EnabledSubjects []string
	// Discovered are the interactables the student found (a discovery quest is listed once one of them is found).
	Discovered []string
	Now        time.Time
	// DailyKey and WeeklyKey are the period keys of now (quest_reset_time) a per_period quest checks.
	DailyKey, WeeklyKey string
	// QuestID restricts the page to one quest (getQuest).
	QuestID   *uuid.UUID
	QuestType string
	Status    string
	AreaID    string
	After     *QuestKey
	Limit     int
}

// QuestListRow is one quest of a page: its version (the pinned version of an active progress, else the current
// version), the status and its rank computed in SQL.
type QuestListRow struct {
	port.QuestRow
	Status string
	Rank   int
}

// QuestListReadModel reads the quest list of a student page by page in SQL (API_DESIGN §5: never the whole list in
// memory) and the history of the quests of a page only.
type QuestListReadModel interface {
	// Page returns one page of QuestListQuery.
	Page(ctx context.Context, q QuestListQuery) ([]QuestListRow, error)
	// Histories returns the history of the given quests of a student: the latest progress, the completions, the last
	// completion and, among periods, the completed ones that are in periods.
	Histories(ctx context.Context, tenant, student uuid.UUID, quests []uuid.UUID, periods []string) (map[uuid.UUID]*port.QuestHistory, error)
	// CompletedQuests returns the ids of the quests the student completed at least once (prerequisites).
	CompletedQuests(ctx context.Context, tenant, student uuid.UUID) (map[uuid.UUID]bool, error)
}
