package command

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/placement"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/domain/school"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// placementStore is an in-memory org.students for the placement tracker.
type placementStore struct {
	rows map[uuid.UUID]*placement.Record
}

func (s *placementStore) Placement(_ context.Context, id uuid.UUID) (*placement.Record, error) {
	r, ok := s.rows[id]
	if !ok {
		return nil, nil
	}
	c := *r
	return &c, nil
}

func (s *placementStore) Move(_ context.Context, id uuid.UUID, to placement.Placement) error {
	r := s.rows[id]
	r.TenantID, r.SchoolID, r.Grade = to.TenantID, to.SchoolID, to.Grade
	r.Version++
	return nil
}

func (s *placementStore) SetStatus(_ context.Context, id uuid.UUID, status string) error {
	r := s.rows[id]
	if active := status == placement.StatusActive; r.Active != active {
		r.Active = active
		r.Version++
	}
	return nil
}

type eventLog struct{ events []write.Event }

func (l *eventLog) Publish(_ context.Context, e write.Event) error {
	l.events = append(l.events, e)
	return nil
}

func (l *eventLog) placements() []PlacementPayload {
	var out []PlacementPayload
	for _, e := range l.events {
		if e.Type == "student.placement_changed" {
			out = append(out, e.Payload.(PlacementPayload))
		}
	}
	return out
}

// ADR-0095 mục 2, 7: a first placement carries every field, a change carries the previous values, no change no event.
func TestPlacementTracker(t *testing.T) {
	ctx := context.Background()
	tenant, school1, year := uuid.New(), uuid.New(), uuid.New()
	student := uuid.New()
	store := &placementStore{rows: map[uuid.UUID]*placement.Record{}}
	log := &eventLog{}

	tr, err := trackPlacement(ctx, store, log, student)
	if err != nil || tr.Before() != nil {
		t.Fatalf("new student: %v %v", tr.Before(), err)
	}
	store.rows[student] = &placement.Record{Placement: placement.Placement{TenantID: tenant, SchoolID: school1, Grade: 3, Active: true},
		Version: 1, AcademicYearID: &year}
	run := uuid.New()
	if err := tr.done(ctx, PlacementChange{Source: placement.SourceSIS, SyncRunID: &run}); err != nil {
		t.Fatal(err)
	}
	got := log.placements()
	if len(got) != 1 || len(got[0].ChangedFields) != 4 || got[0].PreviousSchoolID != nil || got[0].Status != "active" ||
		got[0].GradeLevel != 3 || got[0].AcademicYearID == nil || *got[0].AcademicYearID != year.String() || got[0].SourceVersion != 1 ||
		got[0].Source != "sis" || got[0].SyncRunID == nil {
		t.Fatalf("first placement: %+v", got)
	}
	if log.events[0].TenantID == nil || *log.events[0].TenantID != tenant || log.events[0].SubjectID != student.String() {
		t.Fatalf("envelope: %+v", log.events[0])
	}

	// No change (a SIS update of the name only): no event.
	tr, _ = trackPlacement(ctx, store, log, student)
	store.rows[student].Version++
	if err := tr.done(ctx, PlacementChange{Source: placement.SourceSIS}); err != nil || len(log.placements()) != 1 {
		t.Fatalf("unchanged: %d %v", len(log.placements()), err)
	}

	// Grade and status change (lên khối, then deactivated in the same record).
	tr, _ = trackPlacement(ctx, store, log, student)
	_ = store.Move(ctx, student, placement.Placement{TenantID: tenant, SchoolID: school1, Grade: 4})
	_ = store.SetStatus(ctx, student, placement.StatusInactive)
	on := time.Date(2027, 9, 5, 0, 0, 0, 0, time.UTC)
	if err := tr.done(ctx, PlacementChange{Source: placement.SourceManual, EffectiveFrom: &on}); err != nil {
		t.Fatal(err)
	}
	p := log.placements()[1]
	if !slices.Equal(p.ChangedFields, []string{"grade_level", "status"}) || p.PreviousGradeLevel == nil || *p.PreviousGradeLevel != 3 ||
		p.PreviousStatus == nil || *p.PreviousStatus != "active" || p.Status != "inactive" || p.EffectiveFrom == nil || *p.EffectiveFrom != "2027-09-05" {
		t.Fatalf("grade and status: %+v", p)
	}
}

type schoolsStub struct{ schools map[uuid.UUID]*school.School }

func (s schoolsStub) Create(context.Context, *school.School) error { return nil }
func (s schoolsStub) Get(_ context.Context, id uuid.UUID) (*school.School, error) {
	if sc, ok := s.schools[id]; ok {
		return sc, nil
	}
	return nil, problem.New(problem.NotFound, "")
}
func (s schoolsStub) Save(context.Context, *school.School) error               { return nil }
func (s schoolsStub) CountInTenant(context.Context, uuid.UUID) (int, error)    { return 0, nil }
func (u uowStub) Do(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }
func (a auditStub) Record(context.Context, audit.Event) error                  { return nil }
func (a auditStub) RecordIn(context.Context, uuid.UUID, audit.Event) error     { return nil }

type (
	uowStub   struct{}
	auditStub struct{}
)

// F010-AC-13/14/15 (ADR-0095): a move changes the grade, the school or the tenant of the student; there is no class.
func TestTransferStudentPlacement(t *testing.T) {
	tenantA, tenantB := uuid.New(), uuid.New()
	s1, s2, s3 := uuid.New(), uuid.New(), uuid.New()
	schools := schoolsStub{schools: map[uuid.UUID]*school.School{
		s1: {ID: s1, TenantID: tenantA}, s2: {ID: s2, TenantID: tenantA}, s3: {ID: s3, TenantID: tenantB}}}
	student := uuid.New()
	store := &placementStore{rows: map[uuid.UUID]*placement.Record{student: {
		Placement: placement.Placement{TenantID: tenantA, SchoolID: s1, Grade: 4, Active: true}, Version: 1}}}
	log := &eventLog{}
	h := TransferStudent{TransferDeps{UoW: uowStub{}, Schools: schools, Students: store, Events: log, Audit: auditStub{}}}
	sc, err := authz.EventScope(tenantA)
	if err != nil {
		t.Fatal(err)
	}
	ctx := tenancy.WithScope(context.Background(), sc)
	on := time.Now()
	cases := []struct {
		school uuid.UUID
		grade  int
		kind   placement.Kind
		events []string
	}{
		{s1, 5, placement.KindGrade, []string{"student.placement_changed"}},
		{s2, 5, placement.KindSchool, []string{"student.placement_changed", "student.transferred"}},
		{s3, 5, placement.KindTenant, []string{"student.placement_changed", "student.transferred", "tenant_membership.changed", "tenant_membership.changed"}},
	}
	for _, c := range cases {
		log.events = nil
		out, err := h.Handle(ctx, TransferStudentInput{StudentID: student, TargetSchoolID: c.school, TargetGrade: c.grade, On: on, Source: placement.SourceManual})
		if err != nil || out.Kind != c.kind {
			t.Fatalf("%s: %+v %v", c.kind, out, err)
		}
		var types []string
		for _, e := range log.events {
			types = append(types, e.Type)
		}
		if !slices.Equal(types, c.events) {
			t.Fatalf("%s events: %v", c.kind, types)
		}
	}
	if _, err := h.Handle(ctx, TransferStudentInput{StudentID: student, TargetSchoolID: s3, TargetGrade: 5, On: on}); problem.From(err, "").Code != problem.ValidationFailed {
		t.Fatalf("same school and grade: %v", err)
	}
	if _, err := h.Handle(ctx, TransferStudentInput{StudentID: student, TargetSchoolID: s1, TargetGrade: 10, On: on}); problem.From(err, "").Code != problem.ValidationFailed {
		t.Fatalf("grade 10: %v", err)
	}
	if _, err := h.Handle(ctx, TransferStudentInput{StudentID: uuid.New(), TargetSchoolID: s1, TargetGrade: 3, On: on}); problem.From(err, "").Code != problem.NotFound {
		t.Fatalf("unknown student: %v", err)
	}
}
