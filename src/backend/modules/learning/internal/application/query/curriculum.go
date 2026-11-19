package query

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/curriculum"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// Page is one page of a list (PageInfo of the API).
type Page[T any] struct {
	Data       []T
	NextCursor *string
	HasMore    bool
	Limit      int
}

// PageInput is the cursor and limit of a list request.
type PageInput struct {
	Cursor string
	Limit  int
}

// paginate pages an ordered list in memory: the lists of a curriculum version are bounded by the import limits
// (≤ 200 subject × grade, ≤ 500 topics each), so the read model returns them whole and the cursor is the id of the
// last row.
func paginate[T any](items []T, id func(T) uuid.UUID, in PageInput, codec *page.Codec) (Page[T], error) {
	limit := page.Limit(in.Limit)
	start := 0
	cur, err := codec.Decode(in.Cursor)
	if err != nil {
		return Page[T]{}, problem.New(problem.BadRequest, "invalid cursor")
	}
	if cur != nil {
		i := slices.IndexFunc(items, func(x T) bool { return id(x) == cur.ID })
		if i < 0 {
			return Page[T]{}, problem.New(problem.BadRequest, "invalid cursor")
		}
		start = i + 1
	}
	end := min(start+limit, len(items))
	out := Page[T]{Data: items[start:end], Limit: limit, HasMore: end < len(items)}
	if out.HasMore {
		s := codec.Encode(page.Cursor{ID: id(items[end-1])})
		out.NextCursor = &s
	}
	if out.Data == nil {
		out.Data = []T{}
	}
	return out, nil
}

// Curriculum holds the read side of the curriculum (F021): CMS lists and details (curriculum.read) and the public
// reads of /learning/* (any principal of the tenant, published versions only).
type Curriculum struct {
	Session  read.Session
	Model    read.CurriculumReadModel
	Settings read.SchoolCurriculumSettings
	Codec    port.CurriculumCodec
	Cursors  *page.Codec
	Clock    func() time.Time
}

func (q Curriculum) cms(ctx context.Context) error {
	p, err := principal(ctx)
	if err != nil {
		return err
	}
	return authz.Require(ctx, p, authz.PermCurriculumRead, authz.Resource{})
}

// ListCurricula lists the curricula of the session tenant and the platform curricula assigned to it (platform
// staff: platform curricula) — F021-AC-08.
func (q Curriculum) ListCurricula(ctx context.Context, ownerScope string, in PageInput) (out Page[dto.Curriculum], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListCmsCurricula")
	defer func() { done(err) }()
	if err := q.cms(ctx); err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		cs, err := q.Model.Curricula(ctx, ownerScope)
		if err != nil {
			return err
		}
		out, err = paginate(cs, func(c dto.Curriculum) uuid.UUID { return c.ID }, in, q.Cursors)
		return err
	})
	return out, err
}

// GetCurriculum returns one visible curriculum (404 outside the scope).
func (q Curriculum) GetCurriculum(ctx context.Context, id uuid.UUID) (out dto.Curriculum, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetCmsCurriculum")
	defer func() { done(err) }()
	if err := q.cms(ctx); err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		out, err = q.Model.Curriculum(ctx, id)
		return err
	})
	return out, err
}

// ListVersions lists the versions of a curriculum, newest first.
func (q Curriculum) ListVersions(ctx context.Context, curriculumID uuid.UUID, in PageInput) (out Page[dto.CurriculumVersion], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListCurriculumVersions")
	defer func() { done(err) }()
	if err := q.cms(ctx); err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		vs, err := q.Model.Versions(ctx, curriculumID)
		if err != nil {
			return err
		}
		out, err = paginate(vs, func(v dto.CurriculumVersion) uuid.UUID { return v.ID }, in, q.Cursors)
		return err
	})
	return out, err
}

// GetVersion returns one version of a visible curriculum.
func (q Curriculum) GetVersion(ctx context.Context, curriculumID uuid.UUID, versionNo int) (out dto.CurriculumVersion, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetCurriculumVersion")
	defer func() { done(err) }()
	if err := q.cms(ctx); err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		out, err = q.Model.Version(ctx, curriculumID, versionNo)
		return err
	})
	return out, err
}

// version loads a visible version by id (404 otherwise).
func (q Curriculum) version(ctx context.Context, id uuid.UUID) (dto.CurriculumVersion, error) {
	vs, err := q.Model.VersionsByID(ctx, []uuid.UUID{id})
	if err != nil {
		return dto.CurriculumVersion{}, err
	}
	if len(vs) == 0 {
		return dto.CurriculumVersion{}, problem.New(problem.NotFound, "")
	}
	return vs[0], nil
}

// ListSubjects lists the subject catalog (optionally for a grade).
func (q Curriculum) ListSubjects(ctx context.Context, grade *int, in PageInput) (out Page[dto.Subject], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListCmsSubjects")
	defer func() { done(err) }()
	if err := q.cms(ctx); err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		ss, err := q.Model.Subjects(ctx, grade)
		if err != nil {
			return err
		}
		out, err = paginate(ss, func(s dto.Subject) uuid.UUID { return s.ID }, in, q.Cursors)
		return err
	})
	return out, err
}

// ListTopics lists the topics of a visible version (CMS).
func (q Curriculum) ListTopics(ctx context.Context, versionID uuid.UUID, subject *uuid.UUID, grade *int, in PageInput) (out Page[dto.Topic], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListCmsTopics")
	defer func() { done(err) }()
	if err := q.cms(ctx); err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		if _, err := q.version(ctx, versionID); err != nil {
			if pe, ok := asProblem(err); ok && pe.Code == problem.NotFound {
				out = Page[dto.Topic]{Data: []dto.Topic{}, Limit: page.Limit(in.Limit)}
				return nil
			}
			return err
		}
		ts, err := q.Model.Topics(ctx, read.TopicFilter{VersionIDs: []uuid.UUID{versionID}, SubjectID: subject, GradeLevel: grade})
		if err != nil {
			return err
		}
		out, err = paginate(ts, func(t dto.Topic) uuid.UUID { return t.ID }, in, q.Cursors)
		return err
	})
	return out, err
}

// ObjectiveListInput filters listCmsObjectives.
type ObjectiveListInput struct {
	VersionID *uuid.UUID
	TopicID   *uuid.UUID
	SubjectID *uuid.UUID
	Grade     *int
	SectionID *uuid.UUID
	YccdRef   *string
	Page      PageInput
}

// ListObjectives lists the visible objectives of a version, a topic or a textbook section (F021-AC-29).
func (q Curriculum) ListObjectives(ctx context.Context, in ObjectiveListInput) (out Page[dto.Objective], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListCmsObjectives")
	defer func() { done(err) }()
	if err := q.cms(ctx); err != nil {
		return out, err
	}
	if in.VersionID == nil && in.TopicID == nil && in.SectionID == nil {
		return out, problem.Invalid("curriculum_version_id", problem.SubRequired, "give curriculum_version_id, topic_id or textbook_section_id")
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		f := read.ObjectiveFilter{TopicID: in.TopicID, SubjectID: in.SubjectID, GradeLevel: in.Grade, SectionID: in.SectionID, YccdRef: in.YccdRef}
		if in.VersionID != nil {
			f.VersionIDs = []uuid.UUID{*in.VersionID}
		}
		los, err := q.Model.Objectives(ctx, f)
		if err != nil {
			return err
		}
		los, err = q.liveObjectives(ctx, los)
		if err != nil {
			return err
		}
		out, err = paginate(los, func(o dto.Objective) uuid.UUID { return o.ID }, in.Page, q.Cursors)
		return err
	})
	return out, err
}

// liveObjectives keeps the objectives whose version (and curriculum) is visible and live.
func (q Curriculum) liveObjectives(ctx context.Context, los []dto.Objective) ([]dto.Objective, error) {
	seen := map[uuid.UUID]bool{}
	var vids []uuid.UUID
	for _, o := range los {
		if !seen[o.VersionID] {
			seen[o.VersionID] = true
			vids = append(vids, o.VersionID)
		}
	}
	vs, err := q.Model.VersionsByID(ctx, vids)
	if err != nil {
		return nil, err
	}
	live := map[uuid.UUID]bool{}
	for _, v := range vs {
		live[v.ID] = true
	}
	out := los[:0:0]
	for _, o := range los {
		if live[o.VersionID] {
			out = append(out, o)
		}
	}
	return out, nil
}

// ListCompetencies lists the competencies of a visible version.
func (q Curriculum) ListCompetencies(ctx context.Context, versionID uuid.UUID, in PageInput) (out Page[dto.Competency], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListCmsCompetencies")
	defer func() { done(err) }()
	if err := q.cms(ctx); err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		if _, err := q.version(ctx, versionID); err != nil {
			if pe, ok := asProblem(err); ok && pe.Code == problem.NotFound {
				out = Page[dto.Competency]{Data: []dto.Competency{}, Limit: page.Limit(in.Limit)}
				return nil
			}
			return err
		}
		cs, err := q.Model.Competencies(ctx, versionID)
		if err != nil {
			return err
		}
		out, err = paginate(cs, func(c dto.Competency) uuid.UUID { return c.ID }, in, q.Cursors)
		return err
	})
	return out, err
}

// LoMappings returns the mapping table of a version and the LOs of the superseded version without mapping.
func (q Curriculum) LoMappings(ctx context.Context, curriculumID uuid.UUID, versionNo int) (out dto.LoMappingListView, rowVersion int64, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetCurriculumVersionLoMappings")
	defer func() { done(err) }()
	if err := q.cms(ctx); err != nil {
		return out, 0, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		v, err := q.Model.Version(ctx, curriculumID, versionNo)
		if err != nil {
			return err
		}
		rowVersion = v.RowVersion
		set, err := q.Model.Mappings(ctx, v.ID)
		if err != nil {
			return err
		}
		out = dto.LoMappingListView{CurriculumVersionID: v.ID, SupersedesVersionID: v.SupersedesID, Items: set, UnmappedLoIDs: []uuid.UUID{}}
		if out.Items == nil {
			out.Items = curriculum.MappingSet{}
		}
		if v.SupersedesID == nil {
			return nil
		}
		old, err := q.Model.Objectives(ctx, read.ObjectiveFilter{VersionIDs: []uuid.UUID{*v.SupersedesID}})
		if err != nil {
			return err
		}
		ids := make([]uuid.UUID, len(old))
		for i, o := range old {
			ids[i] = o.ID
		}
		if un := set.Uncovered(ids); un != nil {
			out.UnmappedLoIDs = un
		}
		return nil
	})
	return out, rowVersion, err
}

// ListSeries lists the textbook series.
func (q Curriculum) ListSeries(ctx context.Context, in PageInput) (out Page[dto.TextbookSeries], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListTextbookSeries")
	defer func() { done(err) }()
	if err := q.cms(ctx); err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		ss, err := q.Model.TextbookSeries(ctx)
		if err != nil {
			return err
		}
		out, err = paginate(ss, func(s dto.TextbookSeries) uuid.UUID { return s.ID }, in, q.Cursors)
		return err
	})
	return out, err
}

// ListTextbooks lists textbooks.
func (q Curriculum) ListTextbooks(ctx context.Context, f read.TextbookFilter, in PageInput) (out Page[dto.Textbook], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListTextbooks")
	defer func() { done(err) }()
	if err := q.cms(ctx); err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		ts, err := q.Model.Textbooks(ctx, f)
		if err != nil {
			return err
		}
		out, err = paginate(ts, func(t dto.Textbook) uuid.UUID { return t.ID }, in, q.Cursors)
		return err
	})
	return out, err
}

// ListSections lists the sections of a textbook with the LOs visible to the scope.
func (q Curriculum) ListSections(ctx context.Context, textbookID uuid.UUID, in PageInput) (out Page[dto.TextbookSection], err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListTextbookSections")
	defer func() { done(err) }()
	if err := q.cms(ctx); err != nil {
		return out, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		ss, err := q.Model.Sections(ctx, textbookID)
		if err != nil {
			return err
		}
		out, err = paginate(ss, func(s dto.TextbookSection) uuid.UUID { return s.ID }, in, q.Cursors)
		return err
	})
	return out, err
}

// GetImport returns an import job of the scope (curriculum.write or learning_content.import).
func (q Curriculum) GetImport(ctx context.Context, id uuid.UUID) (out dto.ImportJob, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetContentImport")
	defer func() { done(err) }()
	p, err := principal(ctx)
	if err != nil {
		return out, err
	}
	if !p.Has(authz.PermCurriculumWrite) && !p.Has(authz.PermLearningContentImport) {
		return out, authz.Require(ctx, p, authz.PermCurriculumWrite, authz.Resource{})
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		out, err = q.Model.ImportJob(ctx, id)
		return err
	})
	return out, err
}

// Export returns the file of a version (YAML/CSV) that re-imports to the same structure (F021-AC-24).
func (q Curriculum) Export(ctx context.Context, curriculumID uuid.UUID, versionNo int, format string) (out []byte, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ExportCurriculumVersion")
	defer func() { done(err) }()
	if err := q.cms(ctx); err != nil {
		return nil, err
	}
	if format != port.FormatYAML && format != port.FormatCSV {
		return nil, problem.Invalid("format", problem.SubInvalid, "must be yaml or csv")
	}
	var f dto.CurriculumFile
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		f, err = q.fileOf(ctx, curriculumID, versionNo)
		return err
	})
	if err != nil {
		return nil, err
	}
	return q.Codec.Export(format, f)
}

// fileOf builds the import/export tree of a version.
func (q Curriculum) fileOf(ctx context.Context, curriculumID uuid.UUID, versionNo int) (dto.CurriculumFile, error) {
	c, err := q.Model.Curriculum(ctx, curriculumID)
	if err != nil {
		return dto.CurriculumFile{}, err
	}
	v, err := q.Model.Version(ctx, curriculumID, versionNo)
	if err != nil {
		return dto.CurriculumFile{}, err
	}
	subjects, err := q.Model.Subjects(ctx, nil)
	if err != nil {
		return dto.CurriculumFile{}, err
	}
	subjectCode := map[uuid.UUID]string{}
	for _, s := range subjects {
		subjectCode[s.ID] = s.Code
	}
	comps, err := q.Model.Competencies(ctx, v.ID)
	if err != nil {
		return dto.CurriculumFile{}, err
	}
	compCode := map[uuid.UUID]string{}
	f := dto.CurriculumFile{FormatVersion: 1, Curriculum: dto.FileCurriculum{Code: c.Code, Name: c.Name, Framework: c.Framework},
		Version: dto.FileVersion{Label: v.Label, EffectiveFrom: v.EffectiveFrom, EffectiveTo: v.EffectiveTo, ChangeNote: v.ChangeNote,
			ContentLanguage: v.ContentLanguage}}
	for _, cp := range comps {
		compCode[cp.ID] = cp.Code
		fc := dto.FileCompetency{Code: cp.Code, Name: cp.Name, Kind: cp.Kind}
		if cp.SubjectID != nil {
			fc.SubjectCode = subjectCode[*cp.SubjectID]
		}
		f.Competencies = append(f.Competencies, fc)
	}
	topics, err := q.Model.Topics(ctx, read.TopicFilter{VersionIDs: []uuid.UUID{v.ID}})
	if err != nil {
		return f, err
	}
	los, err := q.Model.Objectives(ctx, read.ObjectiveFilter{VersionIDs: []uuid.UUID{v.ID}})
	if err != nil {
		return f, err
	}
	loCode := map[uuid.UUID]string{}
	for _, o := range los {
		loCode[o.ID] = o.Code
	}
	topicCode := map[uuid.UUID]string{}
	for _, t := range topics {
		topicCode[t.ID] = t.Code
	}
	for _, sg := range v.SubjectGrades {
		fsg := dto.FileSubjectGrade{SubjectCode: subjectCode[sg.SubjectID], GradeLevel: sg.GradeLevel, PeriodsPerYear: sg.PeriodsPerYear}
		for _, t := range topics {
			if t.SubjectID != sg.SubjectID || t.GradeLevel != sg.GradeLevel {
				continue
			}
			ft := dto.FileTopic{Code: t.Code, Name: t.Name, SortOrder: t.SortOrder}
			if t.ParentID != nil {
				ft.ParentCode = topicCode[*t.ParentID]
			}
			for _, o := range los {
				if o.TopicID != t.ID {
					continue
				}
				fo := dto.FileObjective{Code: o.Code, Description: o.Description, SortOrder: o.SortOrder}
				if o.BloomLevel != nil {
					fo.BloomLevel = *o.BloomLevel
				}
				if o.YccdRef != nil {
					fo.YccdRef = *o.YccdRef
				}
				for _, id := range o.CompetencyIDs {
					fo.CompetencyCodes = append(fo.CompetencyCodes, compCode[id])
				}
				for _, id := range o.PrerequisiteIDs {
					fo.PrerequisiteCodes = append(fo.PrerequisiteCodes, loCode[id])
				}
				ft.Objectives = append(ft.Objectives, fo)
			}
			fsg.Topics = append(fsg.Topics, ft)
		}
		f.SubjectGrades = append(f.SubjectGrades, fsg)
	}
	return f, nil
}

// ---------- public reads (/learning/*): published versions only, no editorial metadata (F021-AC-19…21)

// AppliedCurriculum is a curriculum with the version applied to the reader's school.
type AppliedCurriculum struct {
	Curriculum dto.Curriculum
	Version    dto.CurriculumVersion
}

func (q Curriculum) reader(ctx context.Context) (authz.Principal, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, err
	}
	if p.TenantID == nil {
		return p, problem.New(problem.PermissionDenied, "")
	}
	return p, nil
}

func (q Curriculum) today() time.Time {
	if q.Clock != nil {
		return q.Clock().UTC()
	}
	return time.Now().UTC()
}

// applied resolves, for every visible curriculum with a published version, the version applied to the reader
// (F021-AC-19): the school's default version (lms.school_curriculum_settings) when it is a published version of the
// curriculum, else the published version whose effective range contains today (highest version), else the release
// pointer.
func (q Curriculum) applied(ctx context.Context, p authz.Principal, grade *int) ([]AppliedCurriculum, error) {
	cs, err := q.Model.Curricula(ctx, "")
	if err != nil {
		return nil, err
	}
	var school *uuid.UUID
	if p.UserType == authz.UserTypeStudent {
		s, g, err := q.Settings.StudentSchool(ctx, p.UserID)
		if err != nil {
			return nil, err
		}
		school = s
		if grade == nil {
			grade = g
		}
	} else {
		for _, gr := range p.Grants {
			if gr.SchoolID != nil {
				school = gr.SchoolID
				break
			}
		}
	}
	var preferred *uuid.UUID
	if school != nil {
		if preferred, err = q.Settings.DefaultVersion(ctx, *school, grade); err != nil {
			return nil, err
		}
	}
	today := q.today()
	var out []AppliedCurriculum
	for _, c := range cs {
		vs, err := q.Model.Versions(ctx, c.ID)
		if err != nil {
			return nil, err
		}
		var chosen *dto.CurriculumVersion
		for i, v := range vs {
			if v.Status != contentmodel.StatusPublished {
				continue
			}
			switch {
			case preferred != nil && v.ID == *preferred:
				chosen = &vs[i]
			case chosen == nil && (curriculum.Range{From: v.EffectiveFrom, To: v.EffectiveTo}).Contains(today):
				chosen = &vs[i] // versions are newest first: the highest version in effect
			}
			if preferred != nil && chosen != nil && chosen.ID == *preferred {
				break
			}
		}
		if chosen == nil && c.CurrentVersionID != nil {
			if i := slices.IndexFunc(vs, func(v dto.CurriculumVersion) bool { return v.ID == *c.CurrentVersionID }); i >= 0 {
				chosen = &vs[i]
			}
		}
		if chosen == nil {
			continue
		}
		out = append(out, AppliedCurriculum{Curriculum: c, Version: *chosen})
	}
	return out, nil
}

// PublicCurricula lists the curricula available to the reader with the applied version (listCurricula); with a
// subject/grade only the curricula whose applied version has that subject × grade.
func (q Curriculum) PublicCurricula(ctx context.Context, subject *uuid.UUID, grade *int) (out []AppliedCurriculum, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListCurricula")
	defer func() { done(err) }()
	p, err := q.reader(ctx)
	if err != nil {
		return nil, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		all, err := q.applied(ctx, p, grade)
		if err != nil {
			return err
		}
		for _, a := range all {
			if subject != nil || grade != nil {
				if !slices.ContainsFunc(a.Version.SubjectGrades, func(sg dto.SubjectGrade) bool {
					return (subject == nil || sg.SubjectID == *subject) && (grade == nil || sg.GradeLevel == *grade)
				}) {
					continue
				}
			}
			out = append(out, a)
		}
		return nil
	})
	return out, err
}

// PublicSubjects lists the subjects of the catalog (listSubjects).
func (q Curriculum) PublicSubjects(ctx context.Context, grade *int) (out []dto.Subject, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListSubjects")
	defer func() { done(err) }()
	if _, err := q.reader(ctx); err != nil {
		return nil, err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		out, err = q.Model.Subjects(ctx, grade)
		return err
	})
	return out, err
}

func (q Curriculum) appliedVersions(ctx context.Context, p authz.Principal, curriculumID *uuid.UUID, grade *int) ([]uuid.UUID, string, error) {
	all, err := q.applied(ctx, p, grade)
	if err != nil {
		return nil, "", err
	}
	var ids []uuid.UUID
	lang := ""
	for _, a := range all {
		if curriculumID == nil || a.Curriculum.ID == *curriculumID {
			ids = append(ids, a.Version.ID)
			if lang == "" {
				lang = a.Version.ContentLanguage
			}
		}
	}
	return ids, lang, nil
}

// PublicTopics lists the topic tree of the applied versions for a subject and grade (listTopics); it returns the
// content language of the versions (Content-Language).
func (q Curriculum) PublicTopics(ctx context.Context, curriculumID, subject *uuid.UUID, grade *int) (out []dto.Topic, lang string, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListTopics")
	defer func() { done(err) }()
	p, err := q.reader(ctx)
	if err != nil {
		return nil, "", err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		ids, l, err := q.appliedVersions(ctx, p, curriculumID, grade)
		if err != nil || len(ids) == 0 {
			return err
		}
		lang = l
		out, err = q.Model.Topics(ctx, read.TopicFilter{VersionIDs: ids, SubjectID: subject, GradeLevel: grade})
		return err
	})
	return out, lang, err
}

// PublicObjectiveInput filters listLearningObjectives.
type PublicObjectiveInput struct {
	CurriculumID *uuid.UUID
	TopicID      *uuid.UUID
	SubjectID    *uuid.UUID
	Grade        *int
	Page         PageInput
}

// PublicObjectives lists the LOs of the applied versions (listLearningObjectives).
func (q Curriculum) PublicObjectives(ctx context.Context, in PublicObjectiveInput) (out Page[dto.Objective], lang string, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.ListLearningObjectives")
	defer func() { done(err) }()
	p, err := q.reader(ctx)
	if err != nil {
		return out, "", err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		ids, l, err := q.appliedVersions(ctx, p, in.CurriculumID, in.Grade)
		if err != nil {
			return err
		}
		lang = l
		var los []dto.Objective
		if len(ids) > 0 {
			if los, err = q.Model.Objectives(ctx, read.ObjectiveFilter{VersionIDs: ids, TopicID: in.TopicID, SubjectID: in.SubjectID,
				GradeLevel: in.Grade}); err != nil {
				return err
			}
		}
		out, err = paginate(los, func(o dto.Objective) uuid.UUID { return o.ID }, in.Page, q.Cursors)
		return err
	})
	return out, lang, err
}

// PublicObjective returns an LO of any published (or archived) version visible to the tenant, also of an old
// version, so the history of attempts shows the LO they were made on (F021-AC-21).
func (q Curriculum) PublicObjective(ctx context.Context, id uuid.UUID) (out dto.Objective, lang string, err error) {
	ctx, done := observability.UseCase(ctx, "learning.query.GetLearningObjective")
	defer func() { done(err) }()
	if _, err := q.reader(ctx); err != nil {
		return out, "", err
	}
	err = q.Session.Read(ctx, func(ctx context.Context) error {
		los, err := q.Model.Objectives(ctx, read.ObjectiveFilter{IDs: []uuid.UUID{id}})
		if err != nil {
			return err
		}
		if len(los) == 0 {
			return problem.New(problem.NotFound, "")
		}
		vs, err := q.Model.VersionsByID(ctx, []uuid.UUID{los[0].VersionID})
		if err != nil {
			return err
		}
		if len(vs) == 0 || !vs[0].Status.Immutable() {
			return problem.New(problem.NotFound, "")
		}
		out, lang = los[0], vs[0].ContentLanguage
		return nil
	})
	return out, lang, err
}

func asProblem(err error) (*problem.Error, bool) {
	pe, ok := err.(*problem.Error) //nolint:errorlint // read models return problems unwrapped
	return pe, ok
}
