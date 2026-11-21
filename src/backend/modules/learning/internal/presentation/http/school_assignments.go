package http

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SchoolAssignmentCommands are the write use cases of the school assignments (command.Assignments, F041).
type SchoolAssignmentCommands interface {
	CreateAssignment(ctx context.Context, in command.AssignmentInput) (dto.AssignmentView, error)
	BaseInput(ctx context.Context, id uuid.UUID) (command.AssignmentInput, error)
	UpdateAssignment(ctx context.Context, id uuid.UUID, patch command.AssignmentPatch, ifMatch int64) (dto.AssignmentView, error)
	PublishAssignment(ctx context.Context, id uuid.UUID, ifMatch int64) (dto.AssignmentView, error)
	CancelAssignment(ctx context.Context, id uuid.UUID, reason string) (dto.AssignmentView, error)
	ArchiveAssignment(ctx context.Context, id uuid.UUID) (dto.AssignmentView, error)
	ReleaseAssignmentResults(ctx context.Context, id uuid.UUID) (dto.AssignmentView, error)
	RemindAssignment(ctx context.Context, id uuid.UUID) (dto.RemindView, error)
	ExtendAssignmentForStudent(ctx context.Context, id, studentID uuid.UUID, due time.Time, reason string) (dto.AssignmentSubmissionView, error)
	ExcuseAssignmentForStudent(ctx context.Context, id, studentID uuid.UUID, reason string) (dto.AssignmentSubmissionView, error)
}

// SchoolAssignmentQueries are the reads of the school assignments (query.SchoolAssignments, F041).
type SchoolAssignmentQueries interface {
	Assignments(ctx context.Context, in query.AssignmentListInput, pg query.PageInput) (query.Page[dto.AssignmentView], error)
	Assignment(ctx context.Context, id uuid.UUID) (dto.AssignmentView, error)
	Submissions(ctx context.Context, id uuid.UUID, in query.SubmissionListInput, pg query.PageInput) (query.Page[dto.AssignmentSubmissionView], error)
	MyAssignments(ctx context.Context, status string, pg query.PageInput) (query.Page[dto.MyAssignmentView], error)
	GameActivities(ctx context.Context, in query.GameActivityInput, pg query.PageInput) (query.Page[dto.GameActivityView], error)
}

func init() {
	one := "/lms/assignments/{assignment_id}"
	student := one + "/students/{student_id}"
	for id, o := range map[string]httpx.Operation{
		"listAssignments":            lmsOp("listAssignments", http.MethodGet, "/lms/assignments", httpx.IdempotencyNone, 200),
		"createAssignment":           lmsOp("createAssignment", http.MethodPost, "/lms/assignments", httpx.IdempotencyRequired, 201),
		"getAssignment":              lmsOp("getAssignment", http.MethodGet, one, httpx.IdempotencyNone, 200),
		"updateAssignment":           lmsOp("updateAssignment", http.MethodPatch, one, httpx.IdempotencyNone, 200),
		"publishAssignment":          lmsOp("publishAssignment", http.MethodPost, one+":publish", httpx.IdempotencyRequired, 200),
		"cancelAssignment":           lmsOp("cancelAssignment", http.MethodPost, one+":cancel", httpx.IdempotencyRequired, 200),
		"releaseAssignmentResults":   lmsOp("releaseAssignmentResults", http.MethodPost, one+":release-results", httpx.IdempotencyRequired, 200),
		"archiveAssignment":          lmsOp("archiveAssignment", http.MethodPost, one+":archive", httpx.IdempotencyRequired, 200),
		"remindAssignment":           lmsOp("remindAssignment", http.MethodPost, one+":remind", httpx.IdempotencyRequired, 202),
		"extendAssignmentForStudent": lmsOp("extendAssignmentForStudent", http.MethodPost, student+":extend", httpx.IdempotencyRequired, 200),
		"excuseAssignmentForStudent": lmsOp("excuseAssignmentForStudent", http.MethodPost, student+":excuse", httpx.IdempotencyRequired, 200),
		"listAssignmentSubmissions":  lmsOp("listAssignmentSubmissions", http.MethodGet, one+"/submissions", httpx.IdempotencyNone, 200),
		"listGameActivities":         lmsOp("listGameActivities", http.MethodGet, "/lms/catalog/game-activities", httpx.IdempotencyNone, 200),
		"listMyAssignments":          player("listMyAssignments", http.MethodGet, "/learning/my-assignments", httpx.IdempotencyNone, 200),
	} {
		Operations[id] = o
	}
}

var schoolAssignmentRoutes = map[string]func(Handlers, http.ResponseWriter, *http.Request){
	"listAssignments": Handlers.listAssignments, "createAssignment": Handlers.createAssignment, "getAssignment": Handlers.getAssignment,
	"updateAssignment": Handlers.updateAssignment, "publishAssignment": Handlers.publishAssignment, "cancelAssignment": Handlers.cancelAssignment,
	"releaseAssignmentResults": Handlers.releaseAssignmentResults, "archiveAssignment": Handlers.archiveAssignment,
	"remindAssignment": Handlers.remindAssignment, "extendAssignmentForStudent": Handlers.extendAssignmentForStudent,
	"excuseAssignmentForStudent": Handlers.excuseAssignmentForStudent, "listAssignmentSubmissions": Handlers.listAssignmentSubmissions,
	"listGameActivities": Handlers.listGameActivities, "listMyAssignments": Handlers.listMyAssignments,
}

func assignmentResult(w http.ResponseWriter, r *http.Request, status int, v dto.AssignmentView, err error) {
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, v.RowVersion)
	writeJSON(w, status, v)
}

func (h Handlers) listAssignments(w http.ResponseWriter, r *http.Request) {
	var in query.AssignmentListInput
	var err error
	if in.SchoolID, err = queryUUID(r, "school_id"); err != nil {
		fail(w, r, err)
		return
	}
	if in.SubjectID, err = queryUUID(r, "subject_id"); err != nil {
		fail(w, r, err)
		return
	}
	in.Grade = queryInt(r, "grade_level")
	in.Status = r.URL.Query().Get("status")
	p, err := h.Assignments.Assignments(r.Context(), in, pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, p, func(v dto.AssignmentView) dto.AssignmentView { return v })
}

func (h Handlers) createAssignment(w http.ResponseWriter, r *http.Request) {
	var in command.AssignmentInput
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.AssignmentCommands.CreateAssignment(r.Context(), in)
	assignmentResult(w, r, http.StatusCreated, v, err)
}

func (h Handlers) getAssignment(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "assignment_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Assignments.Assignment(r.Context(), id)
	assignmentResult(w, r, http.StatusOK, v, err)
}

// mergeAssignment applies a JSON merge patch (RFC 7396) to the input of a stored assignment: late_policy merges field by
// field, the other fields replace.
func mergeAssignment(base command.AssignmentInput, p patch) (command.AssignmentInput, []string, error) {
	raw, err := json.Marshal(base)
	if err != nil {
		return base, nil, err
	}
	doc := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return base, nil, err
	}
	var present []string
	for k, v := range p {
		present = append(present, k)
		if k == "late_policy" && string(v) != "null" {
			lp := map[string]json.RawMessage{}
			_ = json.Unmarshal(doc[k], &lp)
			upd := map[string]json.RawMessage{}
			if err := json.Unmarshal(v, &upd); err != nil {
				return base, nil, problem.Invalid("/late_policy", problem.SubInvalid, "has the wrong type")
			}
			for kk, vv := range upd {
				lp[kk] = vv
			}
			v, _ = json.Marshal(lp)
		}
		doc[k] = v
	}
	merged, _ := json.Marshal(doc)
	var out command.AssignmentInput
	if err := json.Unmarshal(merged, &out); err != nil {
		return base, nil, problem.New(problem.ValidationFailed, "the patch does not make a valid assignment")
	}
	return out, present, nil
}

func (h Handlers) updateAssignment(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "assignment_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	rv, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	p, err := decodePatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	base, err := h.AssignmentCommands.BaseInput(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	in, present, err := mergeAssignment(base, p)
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.AssignmentCommands.UpdateAssignment(r.Context(), id, command.AssignmentPatch{Present: present, Input: in}, rv)
	assignmentResult(w, r, http.StatusOK, v, err)
}

func (h Handlers) publishAssignment(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "assignment_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	rv, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.AssignmentCommands.PublishAssignment(r.Context(), id, rv)
	assignmentResult(w, r, http.StatusOK, v, err)
}

func (h Handlers) cancelAssignment(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "assignment_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Reason string `json:"reason"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.AssignmentCommands.CancelAssignment(r.Context(), id, b.Reason)
	assignmentResult(w, r, http.StatusOK, v, err)
}

func (h Handlers) archiveAssignment(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "assignment_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.AssignmentCommands.ArchiveAssignment(r.Context(), id)
	assignmentResult(w, r, http.StatusOK, v, err)
}

func (h Handlers) releaseAssignmentResults(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "assignment_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.AssignmentCommands.ReleaseAssignmentResults(r.Context(), id)
	assignmentResult(w, r, http.StatusOK, v, err)
}

func (h Handlers) remindAssignment(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "assignment_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.AssignmentCommands.RemindAssignment(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusAccepted, v)
}

func studentPath(r *http.Request) (uuid.UUID, uuid.UUID, error) {
	id, err := pathUUID(r, "assignment_id")
	if err != nil {
		return id, uuid.Nil, err
	}
	student, err := pathUUID(r, "student_id")
	return id, student, err
}

func (h Handlers) extendAssignmentForStudent(w http.ResponseWriter, r *http.Request) {
	id, student, err := studentPath(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		DueAt  time.Time `json:"due_at"`
		Reason string    `json:"reason"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.AssignmentCommands.ExtendAssignmentForStudent(r.Context(), id, student, b.DueAt, b.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) excuseAssignmentForStudent(w http.ResponseWriter, r *http.Request) {
	id, student, err := studentPath(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Reason string `json:"reason"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.AssignmentCommands.ExcuseAssignmentForStudent(r.Context(), id, student, b.Reason)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) listAssignmentSubmissions(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "assignment_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	in := query.SubmissionListInput{Status: r.URL.Query().Get("status")}
	p, err := h.Assignments.Submissions(r.Context(), id, in, pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, p, func(v dto.AssignmentSubmissionView) dto.AssignmentSubmissionView { return v })
}

func (h Handlers) listGameActivities(w http.ResponseWriter, r *http.Request) {
	in := query.GameActivityInput{Grade: queryInt(r, "grade_level"), Language: r.Header.Get("Accept-Language")}
	if one, err := queryUUID(r, "objective_id"); err != nil {
		fail(w, r, err)
		return
	} else if one != nil {
		in.ObjectiveIDs = append(in.ObjectiveIDs, *one)
	}
	for _, s := range r.URL.Query()["objective_ids"] {
		id, err := uuid.Parse(s)
		if err != nil {
			fail(w, r, problem.Invalid("objective_ids", problem.SubInvalid, "must be UUIDs"))
			return
		}
		in.ObjectiveIDs = append(in.ObjectiveIDs, id)
	}
	p, err := h.Assignments.GameActivities(r.Context(), in, pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, p, func(v dto.GameActivityView) dto.GameActivityView { return v })
}

func (h Handlers) listMyAssignments(w http.ResponseWriter, r *http.Request) {
	p, err := h.Assignments.MyAssignments(r.Context(), r.URL.Query().Get("status"), pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, p, func(v dto.MyAssignmentView) dto.MyAssignmentView { return v })
}
