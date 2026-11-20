package http

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
)

// ProgressQueries are the progress reads of F031 US3 (query.Progress).
type ProgressQueries interface {
	Mine(ctx context.Context, subjectID *uuid.UUID) (dto.LearningProgressView, error)
	Student(ctx context.Context, studentID uuid.UUID, subjectID *uuid.UUID) (dto.LearningProgressView, error)
}

// ProfileAudit records learning.profile_viewed after a read of a student's profile (command.ProfileViews).
type ProfileAudit interface {
	Record(ctx context.Context, studentID, schoolID uuid.UUID, subjectID *uuid.UUID) error
}

func init() {
	Operations["getMyProgress"] = player("getMyProgress", http.MethodGet, "/learning/progress/me", httpx.IdempotencyNone, 200)
	// getStudentProgress: portal staff (x-portal-action action.school_lms.classes.view) of the session tenant.
	o := op("getStudentProgress", http.MethodGet, "/students/{student_id}/progress", httpx.TenantRequired, httpx.IdempotencyNone, 200)
	o.Guard = httpx.GuardPortalStaff
	Operations["getStudentProgress"] = o
}

var progressRoutes = map[string]func(Handlers, http.ResponseWriter, *http.Request){
	"getMyProgress":      Handlers.getMyProgress,
	"getStudentProgress": Handlers.getStudentProgress,
}

func (h Handlers) getMyProgress(w http.ResponseWriter, r *http.Request) {
	subject, err := queryUUID(r, "subject_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Progress.Mine(r.Context(), subject)
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, "vi")
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) getStudentProgress(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "student_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	subject, err := queryUUID(r, "subject_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.Progress.Student(r.Context(), id, subject)
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := h.ProfileAudit.Record(r.Context(), id, v.SchoolID, subject); err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
