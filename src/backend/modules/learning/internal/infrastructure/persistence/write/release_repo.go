package write

import (
	"context"
	"errors"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	entab "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/assessmentblueprint"
	entlm "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/learningmaterial"
	entl "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/lesson"
	entq "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/question"
	entqs "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/questionset"
	entsp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/scheduledpublication"
	enttra "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/tenantresourceassignment"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/release"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// ReleaseRepo implements write.ReleaseStore and write.ScheduleStore with ent (F026).
type ReleaseRepo struct{}

var (
	_ write.ReleaseStore  = ReleaseRepo{}
	_ write.ScheduleStore = ReleaseRepo{}
)

// pointerUpdate is the subset of the generated update builders of the item tables used to move a release pointer.
type pointerUpdate[U any] interface {
	SetCurrentVersionID(uuid.UUID) U
	ClearCurrentVersionID() U
	AddVersion(int64) U
	Save(context.Context) (int, error)
}

func movePointer[U pointerUpdate[U]](ctx context.Context, u U, current *uuid.UUID) (int, error) {
	if current != nil {
		u.SetCurrentVersionID(*current)
	} else {
		u.ClearCurrentVersionID()
	}
	u.AddVersion(1)
	return u.Save(ctx)
}

func mapRelease(err error) error {
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "P0001" && pg.Message == "RELEASE_TARGET_INVALID" {
		return problem.New(problem.RollbackTargetInvalid, "the target must be a published version of the item").Wrap(err)
	}
	return MapCurriculumErr(err)
}

// SetCurrentVersion implements write.ReleaseStore.
func (ReleaseRepo) SetCurrentVersion(ctx context.Context, t contentmodel.ContentType, id uuid.UUID, current *uuid.UUID, ifRow int64) error {
	cl := client(ctx)
	var (
		n      int
		err    error
		exists func() (bool, error)
	)
	switch t {
	case contentmodel.TypeQuestion:
		n, err = movePointer(ctx, cl.Question.Update().Where(entq.ID(id), entq.Version(ifRow)), current)
		exists = func() (bool, error) { return cl.Question.Query().Where(entq.ID(id)).Exist(ctx) }
	case contentmodel.TypeQuestionSet:
		n, err = movePointer(ctx, cl.QuestionSet.Update().Where(entqs.ID(id), entqs.Version(ifRow)), current)
		exists = func() (bool, error) { return cl.QuestionSet.Query().Where(entqs.ID(id)).Exist(ctx) }
	case contentmodel.TypeLesson:
		n, err = movePointer(ctx, cl.Lesson.Update().Where(entl.ID(id), entl.Version(ifRow)), current)
		exists = func() (bool, error) { return cl.Lesson.Query().Where(entl.ID(id)).Exist(ctx) }
	case contentmodel.TypeLearningMaterial:
		n, err = movePointer(ctx, cl.LearningMaterial.Update().Where(entlm.ID(id), entlm.Version(ifRow)), current)
		exists = func() (bool, error) { return cl.LearningMaterial.Query().Where(entlm.ID(id)).Exist(ctx) }
	case contentmodel.TypeAssessmentBlueprint:
		n, err = movePointer(ctx, cl.AssessmentBlueprint.Update().Where(entab.ID(id), entab.Version(ifRow)), current)
		exists = func() (bool, error) { return cl.AssessmentBlueprint.Query().Where(entab.ID(id)).Exist(ctx) }
	default:
		return problem.New(problem.Internal, "no item table for content type "+string(t))
	}
	if err != nil {
		return mapRelease(err)
	}
	if n == 0 {
		return mismatchOrMissing(exists())
	}
	return nil
}

// InsertRelease implements write.ReleaseStore.
func (ReleaseRepo) InsertRelease(ctx context.Context, r write.ReleaseRow) error {
	q := client(ctx).ContentRelease.Create().SetID(r.ID).SetItemType(string(r.ItemType)).SetItemID(r.ItemID).SetAction(string(r.Action)).
		SetNillableFromVersionID(r.FromVersionID).SetNillableToVersionID(r.ToVersionID).SetNillableScheduledFor(r.ScheduledFor).
		SetReleasedBy(r.ReleasedBy).SetReleasedAt(r.ReleasedAt.UTC()).SetIdempotencyKey(r.IdempotencyKey)
	if r.Reason != "" {
		q.SetReason(r.Reason)
	}
	if r.Owner.TenantID != nil {
		q.SetTenantID(*r.Owner.TenantID)
	}
	return MapCurriculumErr(q.Exec(ctx))
}

// AssignedTenants implements write.ReleaseStore: the active assignments of a platform resource, read through
// tenancy.Bypass(platform_job) for the release fan-out of a platform item (ADR-0089 §3) [Proposed].
func (ReleaseRepo) AssignedTenants(ctx context.Context, rt contentmodel.ResourceType, id uuid.UUID) ([]uuid.UUID, error) {
	bctx, err := tenancy.Bypass(tenancy.WithModule(ctx, "learning"), tenancy.ReasonPlatformJob, tenancy.WithMode(tenancy.ModeRead))
	if err != nil {
		return nil, err
	}
	rows, err := client(bctx).TenantResourceAssignment.Query().Where(enttra.ResourceType(string(rt)), enttra.ResourceID(id)).
		Order(enttra.ByTenantID()).All(bctx)
	if err != nil {
		return nil, MapCurriculumErr(err)
	}
	out := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.TenantID)
	}
	return out, nil
}

func scheduleOf(r *ent.ScheduledPublication) write.Schedule {
	return write.Schedule{ID: r.ID, Owner: ownerOfTenant(r.TenantID), ItemType: contentmodel.ContentType(r.ItemType), ItemID: r.ItemID,
		VersionID: r.VersionID, VersionNo: r.VersionNo, PublishAt: r.PublishAt.UTC(), Status: release.ScheduleStatus(r.Status),
		FailureCode: r.FailureCode, AppliedReleaseID: r.AppliedReleaseID, ChangeNote: r.ChangeNote, RequestedBy: r.RequestedBy,
		CancelledBy: r.CancelledBy, IdempotencyKey: r.IdempotencyKey, RowVersion: r.Version, CreatedAt: r.CreatedAt}
}

// InsertSchedule implements write.ScheduleStore.
func (ReleaseRepo) InsertSchedule(ctx context.Context, s write.Schedule) error {
	q := client(ctx).ScheduledPublication.Create().SetID(s.ID).SetItemType(string(s.ItemType)).SetItemID(s.ItemID).SetVersionID(s.VersionID).
		SetVersionNo(s.VersionNo).SetPublishAt(s.PublishAt.UTC()).SetStatus(string(s.Status)).SetNillableChangeNote(s.ChangeNote).
		SetRequestedBy(s.RequestedBy).SetIdempotencyKey(s.IdempotencyKey)
	if s.Owner.TenantID != nil {
		q.SetTenantID(*s.Owner.TenantID)
	}
	return MapCurriculumErr(q.Exec(ctx))
}

// Schedule implements write.ScheduleStore.
func (ReleaseRepo) Schedule(ctx context.Context, id uuid.UUID) (write.Schedule, error) {
	r, err := client(ctx).ScheduledPublication.Query().Where(entsp.ID(id)).Only(ctx)
	if err != nil {
		return write.Schedule{}, MapCurriculumErr(err)
	}
	return scheduleOf(r), nil
}

// UpdateSchedule implements write.ScheduleStore.
func (ReleaseRepo) UpdateSchedule(ctx context.Context, s write.Schedule, ifRow int64) error {
	n, err := client(ctx).ScheduledPublication.Update().Where(entsp.ID(s.ID), entsp.Version(ifRow)).SetStatus(string(s.Status)).
		SetNillableFailureCode(s.FailureCode).SetNillableAppliedReleaseID(s.AppliedReleaseID).SetNillableCancelledBy(s.CancelledBy).
		AddVersion(1).Save(ctx)
	if err != nil {
		return MapCurriculumErr(err)
	}
	if n == 0 {
		return problem.New(problem.VersionMismatch, "the schedule changed")
	}
	return nil
}

// DueSchedules implements write.ScheduleStore.
func (ReleaseRepo) DueSchedules(ctx context.Context, now time.Time, limit int) ([]write.Schedule, error) {
	rows, err := client(ctx).ScheduledPublication.Query().Where(entsp.Status(string(release.ScheduleScheduled)), entsp.PublishAtLTE(now)).
		Order(entsp.ByPublishAt(entsql.OrderAsc()), entsp.ByID()).Limit(limit).All(ctx)
	if err != nil {
		return nil, MapCurriculumErr(err)
	}
	out := make([]write.Schedule, len(rows))
	for i, r := range rows {
		out[i] = scheduleOf(r)
	}
	return out, nil
}
