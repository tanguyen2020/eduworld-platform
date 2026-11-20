package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// LMSCommands are the write use cases of the LMS (command.LMS, F040).
type LMSCommands interface {
	UpdateCourse(ctx context.Context, id uuid.UUID, status string, ifMatch int64) (dto.CourseView, error)
	PutCurriculumSettings(ctx context.Context, school uuid.UUID, items []command.CurriculumSettingInput, ifMatch int64) (dto.CurriculumSettingsView, error)
	CreatePath(ctx context.Context, courseID uuid.UUID, in command.PathInput) (dto.LearningPathView, error)
	UpdatePath(ctx context.Context, courseID, pathID uuid.UUID, in command.PathInput, ifMatch int64) (dto.LearningPathView, error)
	UnlockNode(ctx context.Context, courseID, pathID, nodeID, studentID uuid.UUID, reason string) error
	CreateSchedule(ctx context.Context, in command.ScheduleInput) (dto.ScheduleItemView, error)
	UpdateSchedule(ctx context.Context, id uuid.UUID, in command.ScheduleInput, ifMatch int64) (dto.ScheduleItemView, error)
	DeleteSchedule(ctx context.Context, id uuid.UUID) error
	CreateStudyGroup(ctx context.Context, school uuid.UUID, in command.GroupInput) (dto.StudyGroupView, error)
	UpdateStudyGroup(ctx context.Context, school, id uuid.UUID, in command.GroupInput, ifMatch int64) (dto.StudyGroupView, error)
	DeleteStudyGroup(ctx context.Context, school, id uuid.UUID) error
}

// LMSQueries are the reads of the LMS (query.LMS, F040).
type LMSQueries interface {
	Courses(ctx context.Context, f read.CourseFilter, in query.PageInput) (query.Page[dto.CourseView], error)
	Paths(ctx context.Context, courseID uuid.UUID) ([]dto.LearningPathView, error)
	Schedule(ctx context.Context, in query.ScheduleInput, pg query.PageInput) (query.Page[dto.ScheduleItemView], error)
	StudyGroups(ctx context.Context, school uuid.UUID, classID *uuid.UUID, grade *int, in query.PageInput) (query.Page[dto.StudyGroupView], error)
	StudyGroup(ctx context.Context, school, id uuid.UUID) (dto.StudyGroupView, error)
	CurriculumSettings(ctx context.Context, school uuid.UUID, yearID *uuid.UUID) (dto.CurriculumSettingsView, error)
	MyLearningPaths(ctx context.Context, subject *uuid.UUID) (dto.MyLearningPathsView, error)
	MySchedule(ctx context.Context, from, to string) (dto.MyScheduleView, error)
}

func lmsOp(id, method, pattern string, idem httpx.IdempotencyMode, status int) httpx.Operation {
	return op(id, method, pattern, httpx.TenantRequired, idem, status)
}

func init() {
	course := "/lms/courses/{course_id}"
	paths := course + "/paths"
	groups := "/lms/schools/{school_id}/study-groups"
	for id, o := range map[string]httpx.Operation{
		"listCourses":                 lmsOp("listCourses", http.MethodGet, "/lms/courses", httpx.IdempotencyNone, 200),
		"updateCourse":                lmsOp("updateCourse", http.MethodPatch, course, httpx.IdempotencyNone, 200),
		"listLearningPaths":           lmsOp("listLearningPaths", http.MethodGet, paths, httpx.IdempotencyNone, 200),
		"createLearningPath":          lmsOp("createLearningPath", http.MethodPost, paths, httpx.IdempotencyRequired, 201),
		"updateLearningPath":          lmsOp("updateLearningPath", http.MethodPatch, paths+"/{path_id}", httpx.IdempotencyNone, 200),
		"unlockPathNodeForStudent":    lmsOp("unlockPathNodeForStudent", http.MethodPost, paths+"/{path_id}/nodes/{node_id}/students/{student_id}:unlock", httpx.IdempotencyRequired, 204),
		"listScheduleItems":           lmsOp("listScheduleItems", http.MethodGet, "/lms/schedules", httpx.IdempotencyNone, 200),
		"createScheduleItem":          lmsOp("createScheduleItem", http.MethodPost, "/lms/schedules", httpx.IdempotencyRequired, 201),
		"updateScheduleItem":          lmsOp("updateScheduleItem", http.MethodPatch, "/lms/schedules/{schedule_item_id}", httpx.IdempotencyNone, 200),
		"deleteScheduleItem":          lmsOp("deleteScheduleItem", http.MethodDelete, "/lms/schedules/{schedule_item_id}", httpx.IdempotencyNone, 204),
		"listStudyGroups":             lmsOp("listStudyGroups", http.MethodGet, groups, httpx.IdempotencyNone, 200),
		"createStudyGroup":            lmsOp("createStudyGroup", http.MethodPost, groups, httpx.IdempotencyRequired, 201),
		"getStudyGroup":               lmsOp("getStudyGroup", http.MethodGet, groups+"/{study_group_id}", httpx.IdempotencyNone, 200),
		"updateStudyGroup":            lmsOp("updateStudyGroup", http.MethodPatch, groups+"/{study_group_id}", httpx.IdempotencyNone, 200),
		"deleteStudyGroup":            lmsOp("deleteStudyGroup", http.MethodDelete, groups+"/{study_group_id}", httpx.IdempotencyNone, 204),
		"getSchoolCurriculumSettings": lmsOp("getSchoolCurriculumSettings", http.MethodGet, "/lms/schools/{school_id}/curriculum-settings", httpx.IdempotencyNone, 200),
		"putSchoolCurriculumSettings": lmsOp("putSchoolCurriculumSettings", http.MethodPut, "/lms/schools/{school_id}/curriculum-settings", httpx.IdempotencyNone, 200),
		"listMyLearningPaths":         player("listMyLearningPaths", http.MethodGet, "/learning/my-learning-paths", httpx.IdempotencyNone, 200),
		"listMySchedule":              player("listMySchedule", http.MethodGet, "/learning/my-schedule", httpx.IdempotencyNone, 200),
	} {
		Operations[id] = o
	}
}

var lmsRoutes = map[string]func(Handlers, http.ResponseWriter, *http.Request){
	"listCourses": Handlers.listCourses, "updateCourse": Handlers.updateCourse, "listLearningPaths": Handlers.listLearningPaths,
	"createLearningPath": Handlers.createLearningPath, "updateLearningPath": Handlers.updateLearningPath,
	"unlockPathNodeForStudent": Handlers.unlockPathNode, "listScheduleItems": Handlers.listScheduleItems,
	"createScheduleItem": Handlers.createScheduleItem, "updateScheduleItem": Handlers.updateScheduleItem,
	"deleteScheduleItem": Handlers.deleteScheduleItem, "listStudyGroups": Handlers.listStudyGroups, "createStudyGroup": Handlers.createStudyGroup,
	"getStudyGroup": Handlers.getStudyGroup, "updateStudyGroup": Handlers.updateStudyGroup, "deleteStudyGroup": Handlers.deleteStudyGroup,
	"getSchoolCurriculumSettings": Handlers.getSchoolCurriculumSettings, "putSchoolCurriculumSettings": Handlers.putSchoolCurriculumSettings,
	"listMyLearningPaths": Handlers.listMyLearningPaths, "listMySchedule": Handlers.listMySchedule,
}

// uuids reads the path parameters named.
func uuids(r *http.Request, names ...string) ([]uuid.UUID, error) {
	out := make([]uuid.UUID, len(names))
	for i, n := range names {
		id, err := pathUUID(r, n)
		if err != nil {
			return nil, err
		}
		out[i] = id
	}
	return out, nil
}

func (h Handlers) listCourses(w http.ResponseWriter, r *http.Request) {
	f := read.CourseFilter{Status: r.URL.Query().Get("status")}
	var err error
	for _, p := range []struct {
		name string
		dst  **uuid.UUID
	}{{"school_id", &f.SchoolID}, {"class_id", &f.ClassID}, {"subject_id", &f.SubjectID}, {"academic_year_id", &f.AcademicYearID}} {
		if *p.dst, err = queryUUID(r, p.name); err != nil {
			fail(w, r, err)
			return
		}
	}
	page, err := h.LMS.Courses(r.Context(), f, pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, page, func(c dto.CourseView) dto.CourseView { return c })
}

func (h Handlers) updateCourse(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "course_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Status string `json:"status"`
	}
	if err := decode(r, &b); err != nil {
		fail(w, r, err)
		return
	}
	c, err := h.LMSCommands.UpdateCourse(r.Context(), id, b.Status, v)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, c.Version)
	writeJSON(w, http.StatusOK, c)
}

func (h Handlers) listLearningPaths(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "course_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	ps, err := h.LMS.Paths(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": ps})
}

func (h Handlers) createLearningPath(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "course_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var in command.PathInput
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.LMSCommands.CreatePath(r.Context(), id, in)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, p.Version)
	writeJSON(w, http.StatusCreated, p)
}

func (h Handlers) updateLearningPath(w http.ResponseWriter, r *http.Request) {
	ps, err := uuids(r, "course_id", "path_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var in command.PathInput
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	p, err := h.LMSCommands.UpdatePath(r.Context(), ps[0], ps[1], in, v)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, p.Version)
	writeJSON(w, http.StatusOK, p)
}

func (h Handlers) unlockPathNode(w http.ResponseWriter, r *http.Request) {
	ps, err := uuids(r, "course_id", "path_id", "node_id", "student_id")
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
	if err := h.LMSCommands.UnlockNode(r.Context(), ps[0], ps[1], ps[2], ps[3], b.Reason); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) listScheduleItems(w http.ResponseWriter, r *http.Request) {
	in := query.ScheduleInput{From: r.URL.Query().Get("from"), To: r.URL.Query().Get("to")}
	var err error
	if in.CourseID, err = queryUUID(r, "course_id"); err != nil {
		fail(w, r, err)
		return
	}
	if in.ClassID, err = queryUUID(r, "class_id"); err != nil {
		fail(w, r, err)
		return
	}
	page, err := h.LMS.Schedule(r.Context(), in, pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, page, func(s dto.ScheduleItemView) dto.ScheduleItemView { return s })
}

func (h Handlers) createScheduleItem(w http.ResponseWriter, r *http.Request) {
	var in command.ScheduleInput
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	s, err := h.LMSCommands.CreateSchedule(r.Context(), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, s.Version)
	writeJSON(w, http.StatusCreated, s)
}

func (h Handlers) updateScheduleItem(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "schedule_item_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var in command.ScheduleInput
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	s, err := h.LMSCommands.UpdateSchedule(r.Context(), id, in, v)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, s.Version)
	writeJSON(w, http.StatusOK, s)
}

func (h Handlers) deleteScheduleItem(w http.ResponseWriter, r *http.Request) {
	id, err := pathUUID(r, "schedule_item_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := h.LMSCommands.DeleteSchedule(r.Context(), id); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) listStudyGroups(w http.ResponseWriter, r *http.Request) {
	school, err := pathUUID(r, "school_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	class, err := queryUUID(r, "class_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	page, err := h.LMS.StudyGroups(r.Context(), school, class, queryInt(r, "grade_level"), pageIn(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	list(w, page, func(g dto.StudyGroupView) dto.StudyGroupView { return g })
}

func (h Handlers) createStudyGroup(w http.ResponseWriter, r *http.Request) {
	school, err := pathUUID(r, "school_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	var in command.GroupInput
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	g, err := h.LMSCommands.CreateStudyGroup(r.Context(), school, in)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, g.Version)
	writeJSON(w, http.StatusCreated, g)
}

func (h Handlers) getStudyGroup(w http.ResponseWriter, r *http.Request) {
	ps, err := uuids(r, "school_id", "study_group_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	g, err := h.LMS.StudyGroup(r.Context(), ps[0], ps[1])
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, g.Version)
	writeJSON(w, http.StatusOK, g)
}

func (h Handlers) updateStudyGroup(w http.ResponseWriter, r *http.Request) {
	ps, err := uuids(r, "school_id", "study_group_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var in command.GroupInput
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	g, err := h.LMSCommands.UpdateStudyGroup(r.Context(), ps[0], ps[1], in, v)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, g.Version)
	writeJSON(w, http.StatusOK, g)
}

func (h Handlers) deleteStudyGroup(w http.ResponseWriter, r *http.Request) {
	ps, err := uuids(r, "school_id", "study_group_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := h.LMSCommands.DeleteStudyGroup(r.Context(), ps[0], ps[1]); err != nil {
		fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) getSchoolCurriculumSettings(w http.ResponseWriter, r *http.Request) {
	school, err := pathUUID(r, "school_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	year, err := queryUUID(r, "academic_year_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	s, err := h.LMS.CurriculumSettings(r.Context(), school, year)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, s.Version)
	writeJSON(w, http.StatusOK, s)
}

func (h Handlers) putSchoolCurriculumSettings(w http.ResponseWriter, r *http.Request) {
	school, err := pathUUID(r, "school_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := ifMatch(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var b struct {
		Items []command.CurriculumSettingInput `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		fail(w, r, problem.New(problem.BadRequest, "invalid JSON body"))
		return
	}
	s, err := h.LMSCommands.PutCurriculumSettings(r.Context(), school, b.Items, v)
	if err != nil {
		fail(w, r, err)
		return
	}
	etag(w, s.Version)
	writeJSON(w, http.StatusOK, s)
}

func (h Handlers) listMyLearningPaths(w http.ResponseWriter, r *http.Request) {
	subject, err := queryUUID(r, "subject_id")
	if err != nil {
		fail(w, r, err)
		return
	}
	v, err := h.LMS.MyLearningPaths(r.Context(), subject)
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, "vi")
	writeJSON(w, http.StatusOK, v)
}

func (h Handlers) listMySchedule(w http.ResponseWriter, r *http.Request) {
	v, err := h.LMS.MySchedule(r.Context(), r.URL.Query().Get("from"), r.URL.Query().Get("to"))
	if err != nil {
		fail(w, r, err)
		return
	}
	contentLanguage(w, "vi")
	writeJSON(w, http.StatusOK, v)
}
