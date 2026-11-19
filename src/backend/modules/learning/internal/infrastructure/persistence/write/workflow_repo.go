package write

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent"
	entab "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/assessmentblueprintversion"
	entlm "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/learningmaterialversion"
	entlv "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/lessonversion"
	entqsv "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/questionsetversion"
	entqv "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/questionversion"
	entrt "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/ent/reviewtask"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/workflow"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// WorkflowRepo implements write.VersionStatusStore and write.ReviewStore with ent (F025): the filter of the scope of ctx
// applies (tenant rows, assigned platform rows read-only by RLS, platform rows under PlatformScope).
type WorkflowRepo struct{}

var (
	_ write.VersionStatusStore = WorkflowRepo{}
	_ write.ReviewStore        = WorkflowRepo{}
)

// statusUpdate is the subset of the generated update builders of the version tables used by a status change.
type statusUpdate[U any] interface {
	SetStatus(string) U
	SetReviewerID(uuid.UUID) U
	ClearReviewerID() U
	SetAudioExemptApprovedBy(uuid.UUID) U
	ClearAudioExemptApprovedBy() U
	SetPublishedAt(time.Time) U
	SetPublishedBy(uuid.UUID) U
	SetSnapshotKey(string) U
	SetSnapshotSha256(string) U
	AddVersion(int64) U
	Save(context.Context) (int, error)
}

func applyStatus[U statusUpdate[U]](ctx context.Context, u U, c write.StatusChange) (int, error) {
	u.SetStatus(string(c.Status)).AddVersion(1)
	if c.ClearReview {
		u.ClearReviewerID().ClearAudioExemptApprovedBy()
	}
	if c.ReviewerID != nil {
		u.SetReviewerID(*c.ReviewerID)
	}
	if c.ExemptApprovedBy != nil {
		u.SetAudioExemptApprovedBy(*c.ExemptApprovedBy)
	}
	if c.PublishedAt != nil {
		u.SetPublishedAt(c.PublishedAt.UTC())
	}
	if c.PublishedBy != nil {
		u.SetPublishedBy(*c.PublishedBy)
	}
	if c.SnapshotKey != nil {
		u.SetSnapshotKey(*c.SnapshotKey)
	}
	if c.SnapshotSHA != nil {
		u.SetSnapshotSha256(*c.SnapshotSHA)
	}
	return u.Save(ctx)
}

// setWorkflowDraft writes the translation and exemption columns of a draft save (absent values clear them).
func setWorkflowDraft[U any](status *string, meta json.RawMessage, exempt *string, setS func(string) U, clrS func() U,
	setM func(json.RawMessage) U, clrM func() U, setE func(string) U, clrE func() U) {
	if status != nil {
		setS(*status)
	} else {
		clrS()
	}
	if present(meta) {
		setM(meta)
	} else {
		clrM()
	}
	if exempt != nil && *exempt != "" {
		setE(*exempt)
	} else {
		clrE()
	}
}

// SetVersionStatus implements write.VersionStatusStore.
func (WorkflowRepo) SetVersionStatus(ctx context.Context, c write.StatusChange) (int64, error) {
	cl := client(ctx)
	var (
		n      int
		err    error
		exists func() (bool, error)
	)
	switch c.Type {
	case contentmodel.TypeQuestion:
		n, err = applyStatus(ctx, cl.QuestionVersion.Update().Where(entqv.ID(c.VersionID), entqv.Version(c.IfRow)), c)
		exists = func() (bool, error) { return cl.QuestionVersion.Query().Where(entqv.ID(c.VersionID)).Exist(ctx) }
	case contentmodel.TypeQuestionSet:
		n, err = applyStatus(ctx, cl.QuestionSetVersion.Update().Where(entqsv.ID(c.VersionID), entqsv.Version(c.IfRow)), c)
		exists = func() (bool, error) { return cl.QuestionSetVersion.Query().Where(entqsv.ID(c.VersionID)).Exist(ctx) }
	case contentmodel.TypeLesson:
		n, err = applyStatus(ctx, cl.LessonVersion.Update().Where(entlv.ID(c.VersionID), entlv.Version(c.IfRow)), c)
		exists = func() (bool, error) { return cl.LessonVersion.Query().Where(entlv.ID(c.VersionID)).Exist(ctx) }
	case contentmodel.TypeLearningMaterial:
		n, err = applyStatus(ctx, cl.LearningMaterialVersion.Update().Where(entlm.ID(c.VersionID), entlm.Version(c.IfRow)), c)
		exists = func() (bool, error) {
			return cl.LearningMaterialVersion.Query().Where(entlm.ID(c.VersionID)).Exist(ctx)
		}
	case contentmodel.TypeAssessmentBlueprint:
		n, err = applyStatus(ctx, cl.AssessmentBlueprintVersion.Update().Where(entab.ID(c.VersionID), entab.Version(c.IfRow)), c)
		exists = func() (bool, error) {
			return cl.AssessmentBlueprintVersion.Query().Where(entab.ID(c.VersionID)).Exist(ctx)
		}
	default:
		return 0, problem.New(problem.Internal, "no version table for content type "+string(c.Type))
	}
	if err != nil {
		return 0, MapCurriculumErr(err)
	}
	if n == 0 {
		return 0, mismatchOrMissing(exists())
	}
	return c.IfRow + 1, nil
}

func reviewTaskOf(r *ent.ReviewTask) write.ReviewTask {
	return write.ReviewTask{ID: r.ID, Owner: ownerOfTenant(r.TenantID), Lane: workflow.Lane(r.ReviewLane),
		ContentType: contentmodel.ContentType(r.ContentType), ContentID: r.ContentID, ContentVersionID: r.ContentVersionID, VersionNo: r.VersionNo,
		Title: deref(r.Title), SchoolID: r.SchoolID, AuthorID: r.AuthorID, AssigneeID: r.AssigneeID, Status: workflow.TaskStatus(r.Status),
		Comments: r.Comments, SubmittedAt: r.SubmittedAt, DueAt: r.DueAt, ResolvedAt: r.ResolvedAt, ResolvedBy: r.ResolvedBy}
}

func ownerOfTenant(t *uuid.UUID) contentmodel.Owner {
	if t == nil {
		return contentmodel.Owner{Scope: contentmodel.OwnerPlatform}
	}
	return owner(string(contentmodel.OwnerTenant), t)
}

// OpenTask implements write.ReviewStore.
func (WorkflowRepo) OpenTask(ctx context.Context, versionID uuid.UUID) (write.ReviewTask, bool, error) {
	r, err := client(ctx).ReviewTask.Query().Where(entrt.ContentVersionID(versionID), entrt.Status(string(workflow.TaskOpen))).Only(ctx)
	if ent.IsNotFound(err) {
		return write.ReviewTask{}, false, nil
	}
	if err != nil {
		return write.ReviewTask{}, false, MapCurriculumErr(err)
	}
	return reviewTaskOf(r), true, nil
}

// InsertTask implements write.ReviewStore.
func (WorkflowRepo) InsertTask(ctx context.Context, t write.ReviewTask) error {
	q := client(ctx).ReviewTask.Create().SetID(t.ID).SetReviewLane(string(t.Lane)).SetContentType(string(t.ContentType)).
		SetContentID(t.ContentID).SetContentVersionID(t.ContentVersionID).SetVersionNo(t.VersionNo).SetNillableSchoolID(t.SchoolID).
		SetAuthorID(t.AuthorID).SetNillableAssigneeID(t.AssigneeID).SetStatus(string(t.Status)).SetSubmittedAt(t.SubmittedAt.UTC()).
		SetNillableDueAt(t.DueAt)
	if t.Title != "" {
		q.SetTitle(t.Title)
	}
	if present(t.Comments) {
		q.SetComments(t.Comments)
	}
	if t.Owner.TenantID != nil {
		q.SetTenantID(*t.Owner.TenantID)
	}
	return MapCurriculumErr(q.Exec(ctx))
}

// ResolveTask implements write.ReviewStore.
func (WorkflowRepo) ResolveTask(ctx context.Context, t write.ReviewTask) error {
	n, err := client(ctx).ReviewTask.Update().Where(entrt.ID(t.ID), entrt.Status(string(workflow.TaskOpen))).SetStatus(string(t.Status)).
		SetNillableResolvedAt(t.ResolvedAt).SetNillableResolvedBy(t.ResolvedBy).AddVersion(1).Save(ctx)
	if err != nil {
		return MapCurriculumErr(err)
	}
	if n == 0 {
		return problem.New(problem.VersionMismatch, "the review task is no longer open")
	}
	return nil
}

// CancelOpenTasks implements write.ReviewStore.
func (WorkflowRepo) CancelOpenTasks(ctx context.Context, versionIDs []uuid.UUID, at time.Time) error {
	if len(versionIDs) == 0 {
		return nil
	}
	_, err := client(ctx).ReviewTask.Update().Where(entrt.ContentVersionIDIn(versionIDs...), entrt.Status(string(workflow.TaskOpen))).
		SetStatus(string(workflow.TaskCancelled)).SetResolvedAt(at.UTC()).AddVersion(1).Save(ctx)
	return MapCurriculumErr(err)
}

// InsertTransition implements write.ReviewStore.
func (WorkflowRepo) InsertTransition(ctx context.Context, t write.Transition) error {
	q := client(ctx).ContentTransition.Create().SetID(ids.New()).SetContentType(string(t.ContentType)).SetContentID(t.ContentID).
		SetContentVersionID(t.ContentVersionID).SetAction(string(t.Action)).SetFromStatus(string(t.From)).SetToStatus(string(t.To)).
		SetActorID(t.ActorID).SetAt(t.At.UTC())
	if t.Note != "" {
		q.SetNote(t.Note)
	}
	if t.IdempotencyKey != "" {
		q.SetIdempotencyKey(t.IdempotencyKey)
	}
	if t.Owner.TenantID != nil {
		q.SetTenantID(*t.Owner.TenantID)
	}
	return MapCurriculumErr(q.Exec(ctx))
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
