package query

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/club"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/social"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// ClubDirectory reads the projected student and tenant (read side of write.Directory).
type ClubDirectory interface {
	Student(ctx context.Context, tenant, student uuid.UUID) (*social.Student, error)
	Tenant(ctx context.Context, tenant uuid.UUID) (*social.Tenant, error)
}

// Clubs are the read use cases of the clubs (F120 US4): a student sees its clubs and the active clubs of its scope
// (same school; the whole tenant of a unit with cross-school social, ADR-0054) — nothing else exists for it (404); staff
// sees the clubs of the schools its social.club.approve grants cover. Member lists carry nicknames only (R16).
type Clubs struct {
	Read      ReadSession
	Reader    read.ClubReader
	Directory ClubDirectory
	Cursors   *page.Codec
}

// ClubPage is a page of clubs.
type ClubPage struct {
	Data  []read.ClubRow
	Page  page.Info
	Staff bool
}

// ClubListInput is listClubs.
type ClubListInput struct {
	Scope  string // mine | discoverable (students)
	Type   *club.Type
	Status *string
	School *uuid.UUID // staff
	Cursor string
	Limit  int
}

type clubViewer struct {
	p       authz.Principal
	tenant  uuid.UUID
	student *social.Student
}

func (q Clubs) viewer(ctx context.Context) (clubViewer, error) {
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() || p.TenantID == nil {
		return clubViewer{}, problem.New(problem.Unauthenticated, "")
	}
	v := clubViewer{p: p, tenant: *p.TenantID}
	switch {
	case p.UserType == authz.UserTypeStudent && p.Channel == authz.ChannelGame:
		return v, nil
	case p.UserType == authz.UserTypeStaff && p.Channel == authz.ChannelPortal:
		return v, nil
	case p.UserType == authz.UserTypeStudent:
		return clubViewer{}, problem.New(problem.GameAccessStudentOnly, "")
	}
	return clubViewer{}, problem.New(problem.PortalAccessStaffOnly, "")
}

func (v clubViewer) isStudent() bool { return v.p.UserType == authz.UserTypeStudent }

func (q Clubs) loadStudent(ctx context.Context, v *clubViewer) error {
	if !v.isStudent() {
		return nil
	}
	s, err := q.Directory.Student(ctx, v.tenant, v.p.UserID)
	if err != nil {
		return err
	}
	if s == nil || !s.Active() {
		return problem.New(problem.GameAccessStudentOnly, "")
	}
	v.student = s
	return nil
}

// schools is the club scope of the viewer: a student's school (nil = every school of a unit with cross-school social);
// staff: the schools of its grants holding social.club.approve (nil = the whole tenant).
func (q Clubs) schools(ctx context.Context, v clubViewer) ([]uuid.UUID, error) {
	if v.isStudent() {
		t, err := q.Directory.Tenant(ctx, v.tenant)
		if err != nil {
			return nil, err
		}
		if t != nil && t.Type == social.TenantUnit && t.CrossSchool {
			return nil, nil
		}
		return []uuid.UUID{v.student.SchoolID}, nil
	}
	var out []uuid.UUID
	held := false
	for _, g := range v.p.Grants {
		if !slices.Contains(g.Permissions, authz.PermSocialClubApprove) {
			continue
		}
		held = true
		if g.SchoolID == nil {
			return nil, nil
		}
		out = append(out, *g.SchoolID)
	}
	if !held {
		t := v.tenant
		return nil, authz.Require(ctx, v.p, authz.PermSocialClubApprove, authz.Resource{TenantID: &t})
	}
	return out, nil
}

func (q Clubs) cursor(raw string) (*read.ClubCursor, error) {
	c, err := q.Cursors.Decode(raw)
	if err != nil {
		return nil, problem.New(problem.BadRequest, "invalid cursor")
	}
	if c == nil {
		return nil, nil
	}
	at, err := time.Parse(time.RFC3339Nano, c.Key)
	if err != nil {
		return nil, problem.New(problem.BadRequest, "invalid cursor")
	}
	return &read.ClubCursor{At: at, ID: c.ID}, nil
}

func (q Clubs) next(at time.Time, id uuid.UUID) *string {
	s := q.Cursors.Encode(page.Cursor{Key: at.UTC().Format(time.RFC3339Nano), ID: id})
	return &s
}

// List serves listClubs.
func (q Clubs) List(ctx context.Context, in ClubListInput) (out ClubPage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListClubs")
	defer func() { done(err) }()
	v, err := q.viewer(ctx)
	if err != nil {
		return out, err
	}
	after, err := q.cursor(in.Cursor)
	if err != nil {
		return out, err
	}
	limit := page.Limit(in.Limit)
	f := read.ClubFilter{Tenant: v.tenant, Type: in.Type, After: after, Limit: limit + 1}
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		if err := q.loadStudent(ctx, &v); err != nil {
			return err
		}
		if v.isStudent() {
			me := v.p.UserID
			f.Viewer = &me
			if in.Scope == "discoverable" {
				schools, err := q.schools(ctx, v)
				if err != nil {
					return err
				}
				f.Schools, f.Status = schools, []string{club.StatusActive}
			} else {
				f.Member = &me
			}
		} else {
			schools, err := q.schools(ctx, v)
			if err != nil {
				return err
			}
			if in.School != nil {
				if schools != nil && !containsID(schools, *in.School) {
					t := v.tenant
					return authz.Require(ctx, v.p, authz.PermSocialClubApprove, authz.Resource{TenantID: &t, SchoolID: in.School})
				}
				schools = []uuid.UUID{*in.School}
			}
			f.Schools = schools
			if in.Status != nil {
				f.Status = []string{*in.Status}
			}
		}
		var err error
		out.Data, err = q.Reader.Clubs(ctx, f)
		return err
	})
	if err != nil {
		return out, err
	}
	out.Staff = !v.isStudent()
	out.Page = page.Info{Limit: limit}
	if len(out.Data) > limit {
		out.Data = out.Data[:limit]
		last := out.Data[len(out.Data)-1].Club
		out.Page.NextCursor, out.Page.HasMore = q.next(last.CreatedAt, last.ID), true
	}
	return out, nil
}

func containsID(ids []uuid.UUID, id uuid.UUID) bool { return slices.Contains(ids, id) }

// visible loads a club the viewer may see (404 otherwise) and reports whether it is a member (students).
func (q Clubs) visible(ctx context.Context, v *clubViewer, id uuid.UUID) (*read.ClubRow, error) {
	if err := q.loadStudent(ctx, v); err != nil {
		return nil, err
	}
	var viewer *uuid.UUID
	if v.isStudent() {
		me := v.p.UserID
		viewer = &me
	}
	row, err := q.Reader.Club(ctx, v.tenant, id, viewer)
	if err != nil {
		return nil, err
	}
	if row == nil {
		return nil, problem.New(problem.NotFound, "")
	}
	if !v.isStudent() {
		return row, nil // staff: the caller checks social.club.approve outside the read-only transaction
	}
	if row.MyRole != "" {
		return row, nil
	}
	if row.Club.Status != club.StatusActive && row.Club.Status != club.StatusPending {
		return nil, problem.New(problem.NotFound, "")
	}
	schools, err := q.schools(ctx, *v)
	if err != nil {
		return nil, err
	}
	if schools != nil && (row.Club.SchoolID == nil || !containsID(schools, *row.Club.SchoolID)) {
		return nil, problem.New(problem.NotFound, "")
	}
	return row, nil
}

// Get serves getClub.
func (q Clubs) Get(ctx context.Context, id uuid.UUID) (row read.ClubRow, staff bool, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.GetClub")
	defer func() { done(err) }()
	v, err := q.viewer(ctx)
	if err != nil {
		return row, false, err
	}
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		r, err := q.visible(ctx, &v, id)
		if err != nil {
			return err
		}
		row = *r
		return nil
	})
	if err == nil && !v.isStudent() {
		t := v.tenant
		if err := authz.Require(ctx, v.p, authz.PermSocialClubApprove, authz.Resource{TenantID: &t, SchoolID: row.Club.SchoolID}); err != nil {
			return read.ClubRow{}, true, err
		}
	}
	return row, !v.isStudent(), err
}

// memberOnly loads a club the student is an active member of (members, goals, events), 404 otherwise.
func (q Clubs) memberOnly(ctx context.Context, v *clubViewer, id uuid.UUID) (*read.ClubRow, error) {
	if !v.isStudent() {
		return nil, problem.New(problem.GameAccessStudentOnly, "")
	}
	row, err := q.visible(ctx, v, id)
	if err != nil {
		return nil, err
	}
	if row.MyRole == "" {
		return nil, problem.New(problem.NotFound, "")
	}
	return row, nil
}

// MemberPage is a page of club members.
type MemberPage struct {
	Data []read.ClubMemberRow
	Page page.Info
}

// Members serves listClubMembers (members only).
func (q Clubs) Members(ctx context.Context, id uuid.UUID, cursor string, limit int) (out MemberPage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListClubMembers")
	defer func() { done(err) }()
	v, err := q.viewer(ctx)
	if err != nil {
		return out, err
	}
	after, err := q.cursor(cursor)
	if err != nil {
		return out, err
	}
	limit = page.Limit(limit)
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		if _, err := q.memberOnly(ctx, &v, id); err != nil {
			return err
		}
		var err error
		out.Data, err = q.Reader.Members(ctx, v.tenant, id, after, limit+1)
		return err
	})
	if err != nil {
		return out, err
	}
	out.Page = page.Info{Limit: limit}
	if len(out.Data) > limit {
		out.Data = out.Data[:limit]
		last := out.Data[len(out.Data)-1]
		out.Page.NextCursor, out.Page.HasMore = q.next(last.JoinedAt, last.UserID), true
	}
	return out, nil
}

// RequestPage is a page of pending join requests.
type RequestPage struct {
	Data []read.ClubRequestRow
	Page page.Info
}

// Requests serves listClubJoinRequests (owner/officer).
func (q Clubs) Requests(ctx context.Context, id uuid.UUID, cursor string, limit int) (out RequestPage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListClubJoinRequests")
	defer func() { done(err) }()
	v, err := q.viewer(ctx)
	if err != nil {
		return out, err
	}
	after, err := q.cursor(cursor)
	if err != nil {
		return out, err
	}
	limit = page.Limit(limit)
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		row, err := q.memberOnly(ctx, &v, id)
		if err != nil {
			return err
		}
		if !club.Manager(row.MyRole) {
			return problem.New(problem.PermissionDenied, "")
		}
		out.Data, err = q.Reader.Requests(ctx, v.tenant, id, after, limit+1)
		return err
	})
	if err != nil {
		return out, err
	}
	out.Page = page.Info{Limit: limit}
	if len(out.Data) > limit {
		out.Data = out.Data[:limit]
		last := out.Data[len(out.Data)-1]
		out.Page.NextCursor, out.Page.HasMore = q.next(last.CreatedAt, last.ID), true
	}
	return out, nil
}

// Goals serves listClubGoals (members; my contribution only — never another member's score).
func (q Clubs) Goals(ctx context.Context, id uuid.UUID) (out []read.ClubGoalRow, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListClubGoals")
	defer func() { done(err) }()
	v, err := q.viewer(ctx)
	if err != nil {
		return nil, err
	}
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		if _, err := q.memberOnly(ctx, &v, id); err != nil {
			return err
		}
		var err error
		out, err = q.Reader.Goals(ctx, v.tenant, id, v.p.UserID)
		return err
	})
	return out, err
}

// EventPage is a page of club events.
type EventPage struct {
	Data []read.ClubEventRow
	Page page.Info
}

// Events serves listClubEvents (members).
func (q Clubs) Events(ctx context.Context, id uuid.UUID, cursor string, limit int) (out EventPage, err error) {
	ctx, done := observability.UseCase(ctx, "chat.query.ListClubEvents")
	defer func() { done(err) }()
	v, err := q.viewer(ctx)
	if err != nil {
		return out, err
	}
	after, err := q.cursor(cursor)
	if err != nil {
		return out, err
	}
	limit = page.Limit(limit)
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		if _, err := q.memberOnly(ctx, &v, id); err != nil {
			return err
		}
		var err error
		out.Data, err = q.Reader.Events(ctx, v.tenant, id, after, limit+1)
		return err
	})
	if err != nil {
		return out, err
	}
	out.Page = page.Info{Limit: limit}
	if len(out.Data) > limit {
		out.Data = out.Data[:limit]
		last := out.Data[len(out.Data)-1]
		out.Page.NextCursor, out.Page.HasMore = q.next(last.StartsAt, last.ID), true
	}
	return out, nil
}
