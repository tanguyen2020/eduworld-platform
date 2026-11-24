package query

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/view"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/challenge"
	trn "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/tournament"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// Tournaments are the read use cases of F107: challenges and templates of the organizer, the challenges of a student,
// the tournaments and their brackets (students in scope see nicknames only — R16; staff of the owning school also see
// the student ids, the portal resolves the names from module identity).
type Tournaments struct {
	Read   read.ReadSession
	Reader read.TournamentReader
	Now    func() time.Time
}

// Page is a keyset page (data + PageInfo).
type Page struct {
	Data       []map[string]any
	Limit      int
	NextCursor *string
}

func encodeKeyset(k read.Keyset) string {
	b, _ := json.Marshal(map[string]any{"a": k.At.UTC().Format(time.RFC3339Nano), "i": k.ID})
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeKeyset(cursor string) (*read.Keyset, error) {
	if cursor == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(cursor)
	var raw struct {
		A string    `json:"a"`
		I uuid.UUID `json:"i"`
	}
	if err != nil || json.Unmarshal(b, &raw) != nil || raw.I == uuid.Nil {
		return nil, problem.Invalid("cursor", problem.SubInvalid, "is not a cursor of this listing")
	}
	at, err := time.Parse(time.RFC3339Nano, raw.A)
	if err != nil {
		return nil, problem.Invalid("cursor", problem.SubInvalid, "is not a cursor of this listing")
	}
	return &read.Keyset{At: at, ID: raw.I}, nil
}

// organizerViewer is portal staff with lms.challenge.write (403 otherwise), with the schools its grants cover (nil:
// every school of the tenant).
func organizerViewer(ctx context.Context) (viewer, []uuid.UUID, error) {
	v, err := matchViewer(ctx)
	if err != nil {
		return v, nil, err
	}
	if v.student {
		authz.RecordDenied(ctx, string(problem.PermissionDenied), map[string]any{"permission": string(authz.PermLmsChallengeWrite)})
		return v, nil, problem.New(problem.PermissionDenied, "")
	}
	t := v.tenant
	if !v.p.Has(authz.PermLmsChallengeWrite) {
		return v, nil, authz.Require(ctx, v.p, authz.PermLmsChallengeWrite, authz.Resource{TenantID: &t})
	}
	return v, organizerSchools(v.p), nil
}

// organizerSchools returns the schools of the lms.challenge.write grants of p (nil = the whole tenant).
func organizerSchools(p authz.Principal) []uuid.UUID {
	var out []uuid.UUID
	for _, g := range p.Grants {
		if !slices.Contains(g.Permissions, authz.PermLmsChallengeWrite) {
			continue
		}
		if g.SchoolID == nil {
			return nil
		}
		out = append(out, *g.SchoolID)
	}
	return out
}

func covers(schools []uuid.UUID, school *uuid.UUID) bool {
	return schools == nil || (school != nil && slices.Contains(schools, *school))
}

// ListChallenges implements listChallenges (F107-AC-06): the challenges of the schools of the organizer, filtered by
// grade and status, keyset (starts_at, challenge_id) descending cut in SQL.
func (q Tournaments) ListChallenges(ctx context.Context, grade *int, status *string, limit int, cursor string) (page Page, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.ListChallenges")
	defer func() { done(err) }()
	v, schools, err := organizerViewer(ctx)
	if err != nil {
		return page, err
	}
	after, err := decodeKeyset(cursor)
	if err != nil {
		return page, err
	}
	page.Limit = limit
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		rows, err := q.Reader.ChallengePage(ctx, v.tenant, read.ChallengeFilter{Schools: schools, Grade: grade, Status: status}, after, limit+1)
		if err != nil {
			return err
		}
		if len(rows) > limit {
			rows = rows[:limit]
			last := rows[len(rows)-1]
			c := encodeKeyset(read.Keyset{At: last.StartsAt, ID: last.ID})
			page.NextCursor = &c
		}
		counts, err := q.Reader.ChallengeParticipants(ctx, v.tenant, rows)
		if err != nil {
			return err
		}
		page.Data = make([]map[string]any, 0, len(rows))
		for _, c := range rows {
			aud, err := q.Reader.AudienceSize(ctx, c)
			if err != nil {
				return err
			}
			page.Data = append(page.Data, view.Challenge(c, aud, counts[c.ID]))
		}
		return nil
	})
	return page, err
}

// GetChallenge implements getChallenge (F107-AC-06): a challenge of a school of the organizer, 404 otherwise.
func (q Tournaments) GetChallenge(ctx context.Context, id uuid.UUID) (out map[string]any, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.GetChallenge")
	defer func() { done(err) }()
	v, schools, err := organizerViewer(ctx)
	if err != nil {
		return nil, err
	}
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		c, err := q.Reader.Challenge(ctx, v.tenant, id)
		if err != nil {
			return err
		}
		if c == nil || !covers(schools, &c.School) {
			return problem.New(problem.NotFound, "")
		}
		counts, err := q.Reader.ChallengeParticipants(ctx, v.tenant, []port.ChallengeRow{*c})
		if err != nil {
			return err
		}
		aud, err := q.Reader.AudienceSize(ctx, *c)
		if err != nil {
			return err
		}
		out = view.Challenge(*c, aud, counts[c.ID])
		return nil
	})
	return out, err
}

type templateCursor struct {
	C string    `json:"c"`
	I uuid.UUID `json:"i"`
}

// ListChallengeTemplates implements listChallengeTemplates (F107-AC-01, AC-02): the published templates assigned to the
// tenant, keyset (code, config_id).
func (q Tournaments) ListChallengeTemplates(ctx context.Context, grade *int, subject *uuid.UUID, limit int, cursor string) (page Page, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.ListChallengeTemplates")
	defer func() { done(err) }()
	v, _, err := organizerViewer(ctx)
	if err != nil {
		return page, err
	}
	var after *read.TemplateKeyset
	if cursor != "" {
		b, err := base64.RawURLEncoding.DecodeString(cursor)
		var c templateCursor
		if err != nil || json.Unmarshal(b, &c) != nil || c.I == uuid.Nil {
			return page, problem.Invalid("cursor", problem.SubInvalid, "is not a cursor of this listing")
		}
		after = &read.TemplateKeyset{Code: c.C, ID: c.I}
	}
	page.Limit = limit
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		rows, err := q.Reader.TemplatePage(ctx, v.tenant, grade, subject, after, limit+1)
		if err != nil {
			return err
		}
		if len(rows) > limit {
			rows = rows[:limit]
			last := rows[len(rows)-1]
			b, _ := json.Marshal(templateCursor{C: last.Code, I: last.ConfigID})
			c := base64.RawURLEncoding.EncodeToString(b)
			page.NextCursor = &c
		}
		page.Data = make([]map[string]any, 0, len(rows))
		for _, r := range rows {
			page.Data = append(page.Data, view.Template(r))
		}
		return nil
	})
	return page, err
}

// ListMyChallenges implements listMyChallenges (F107-AC-27): the scheduled or running challenges of the student's
// audience (school, grade or listed, not excluded), keyset (starts_at, challenge_id).
func (q Tournaments) ListMyChallenges(ctx context.Context, limit int, cursor string) (page Page, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.ListMyChallenges")
	defer func() { done(err) }()
	v, err := matchViewer(ctx)
	if err != nil {
		return page, err
	}
	if !v.student {
		return page, problem.New(problem.GameAccessStudentOnly, "")
	}
	after, err := decodeKeyset(cursor)
	if err != nil {
		return page, err
	}
	page.Limit, page.Data = limit, []map[string]any{}
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		s, err := q.Reader.Student(ctx, v.tenant, v.user)
		if err != nil || s == nil || !s.Active {
			return err
		}
		rows, err := q.Reader.MyChallenges(ctx, v.tenant, read.StudentScope{Student: v.user, School: s.School, Grade: s.Grade}, after, limit+1)
		if err != nil {
			return err
		}
		if len(rows) > limit {
			rows = rows[:limit]
			last := rows[len(rows)-1]
			c := encodeKeyset(read.Keyset{At: last.StartsAt, ID: last.ID})
			page.NextCursor = &c
		}
		for _, c := range rows {
			page.Data = append(page.Data, view.CompetitionChallenge(c))
		}
		return nil
	})
	return page, err
}

// tournamentViewer is a student of the game client or an organizer of the portal (staff with a game token:
// GAME_ACCESS_STUDENT_ONLY).
func tournamentViewer(ctx context.Context) (viewer, []uuid.UUID, error) {
	v, err := matchViewer(ctx)
	if err != nil {
		return v, nil, err
	}
	if v.student {
		return v, nil, nil
	}
	return organizerViewer(ctx)
}

// sees reports whether a student sees a tournament (F107-AC-25): not a draft, its school, a grade of the tournament and
// the audience of its challenge.
func sees(t *port.TournamentRow, c *port.ChallengeRow, me uuid.UUID, s *port.StudentInfo) bool {
	if s == nil || !s.Active || t.Status == trn.StatusDraft || (t.School != nil && *t.School != s.School) || !slices.Contains(t.Grades, s.Grade) {
		return false
	}
	return c == nil || challenge.InAudience(me, s.School, s.Grade, c.School, c.Grade, c.Listed, c.Exclude)
}

// ListTournaments implements listTournaments (F107-AC-25): a student gets the tournaments of its scope (scope=mine: those
// it has an entry in); an organizer gets the tournaments of its schools; keyset (starts_at, tournament_id) in SQL.
func (q Tournaments) ListTournaments(ctx context.Context, scope string, status *string, school *uuid.UUID, limit int, cursor string) (page Page, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.ListTournaments")
	defer func() { done(err) }()
	v, schools, err := tournamentViewer(ctx)
	if err != nil {
		return page, err
	}
	after, err := decodeKeyset(cursor)
	if err != nil {
		return page, err
	}
	page.Limit, page.Data = limit, []map[string]any{}
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		f := read.TournamentFilter{Status: status}
		if v.student {
			s, err := q.Reader.Student(ctx, v.tenant, v.user)
			if err != nil || s == nil || !s.Active {
				return err
			}
			f.Student = &read.StudentScope{Student: v.user, School: s.School, Grade: s.Grade, Mine: scope == "mine"}
		} else {
			if school != nil && !covers(schools, school) {
				return nil // a school outside the grants: nothing (no existence leak)
			}
			f.Schools, f.School = schools, school
		}
		rows, err := q.Reader.TournamentPage(ctx, v.tenant, f, after, limit+1)
		if err != nil {
			return err
		}
		if len(rows) > limit {
			rows = rows[:limit]
			last := rows[len(rows)-1]
			c := encodeKeyset(read.Keyset{At: last.StartsAt, ID: last.ID})
			page.NextCursor = &c
		}
		ids := make([]uuid.UUID, len(rows))
		for i, t := range rows {
			ids[i] = t.ID
		}
		counts, err := q.Reader.EntryCounts(ctx, v.tenant, ids)
		if err != nil {
			return err
		}
		for _, t := range rows {
			page.Data = append(page.Data, view.Tournament(t, counts[t.ID], nil, v.student))
		}
		return nil
	})
	return page, err
}

// scoped loads a tournament the viewer may read (404 otherwise) and, for a student, its own entry.
func (q Tournaments) scoped(ctx context.Context, v viewer, schools []uuid.UUID, id uuid.UUID) (*port.TournamentRow, *port.EntryRow, error) {
	t, err := q.Reader.Tournament(ctx, v.tenant, id)
	if err != nil {
		return nil, nil, err
	}
	if t == nil {
		return nil, nil, problem.New(problem.NotFound, "")
	}
	if !v.student {
		if !covers(schools, t.School) {
			return nil, nil, problem.New(problem.NotFound, "")
		}
		return t, nil, nil
	}
	var c *port.ChallengeRow
	if t.ChallengeID != nil {
		if c, err = q.Reader.Challenge(ctx, v.tenant, *t.ChallengeID); err != nil {
			return nil, nil, err
		}
	}
	s, err := q.Reader.Student(ctx, v.tenant, v.user)
	if err != nil {
		return nil, nil, err
	}
	entries, err := q.Reader.Entries(ctx, v.tenant, t.ID)
	if err != nil {
		return nil, nil, err
	}
	var mine *port.EntryRow
	for i := range entries {
		if slices.Contains(entries[i].Members, v.user) && entries[i].Status != trn.EntryWithdrawn {
			mine = &entries[i]
		}
	}
	if mine == nil && !sees(t, c, v.user, s) {
		return nil, nil, problem.New(problem.NotFound, "")
	}
	return t, mine, nil
}

// GetTournament implements getTournament (F107-AC-25).
func (q Tournaments) GetTournament(ctx context.Context, id uuid.UUID) (out map[string]any, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.GetTournament")
	defer func() { done(err) }()
	v, schools, err := tournamentViewer(ctx)
	if err != nil {
		return nil, err
	}
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		t, mine, err := q.scoped(ctx, v, schools, id)
		if err != nil {
			return err
		}
		counts, err := q.Reader.EntryCounts(ctx, v.tenant, []uuid.UUID{t.ID})
		if err != nil {
			return err
		}
		var my map[string]any
		if mine != nil {
			cards, err := q.Reader.Cards(ctx, v.tenant, mine.Members)
			if err != nil {
				return err
			}
			my = view.Entry(*mine, cards, "", nil)
		}
		out = view.Tournament(*t, counts[t.ID], my, v.student)
		return nil
	})
	return out, err
}

// GetTournamentBracket implements getTournamentBracket (F107-AC-25): entries (nickname/avatar only), nodes in schedule
// order, group tables, final ranks; the seed of a random seeding only for the organizer.
func (q Tournaments) GetTournamentBracket(ctx context.Context, id uuid.UUID) (out map[string]any, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.GetTournamentBracket")
	defer func() { done(err) }()
	v, schools, err := tournamentViewer(ctx)
	if err != nil {
		return nil, err
	}
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		t, _, err := q.scoped(ctx, v, schools, id)
		if err != nil {
			return err
		}
		entries, err := q.Reader.Entries(ctx, v.tenant, t.ID)
		if err != nil {
			return err
		}
		nodes, err := q.Reader.Nodes(ctx, v.tenant, t.ID)
		if err != nil {
			return err
		}
		groups, err := q.Reader.Groups(ctx, v.tenant, t.ID)
		if err != nil {
			return err
		}
		ranks, err := q.Reader.Results(ctx, v.tenant, t.ID)
		if err != nil {
			return err
		}
		var students []uuid.UUID
		for _, e := range entries {
			students = append(students, e.Members...)
		}
		cards, err := q.Reader.Cards(ctx, v.tenant, students)
		if err != nil {
			return err
		}
		out = bracketBody(*t, entries, nodes, groups, ranks, cards, v.student)
		return nil
	})
	return out, err
}

func bracketBody(t port.TournamentRow, entries []port.EntryRow, nodes []*trn.Node, groups []port.GroupRow, ranks map[uuid.UUID]int,
	cards map[uuid.UUID]port.Card, student bool) map[string]any {
	codes := map[uuid.UUID]string{}
	for _, g := range groups {
		codes[g.ID] = g.Code
	}
	sort.SliceStable(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if (a.Seed == nil) != (b.Seed == nil) {
			return a.Seed != nil
		}
		return a.Seed != nil && *a.Seed < *b.Seed
	})
	es := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		code := ""
		if e.Group != nil {
			code = codes[*e.Group]
		}
		var rank *int
		if r, ok := ranks[e.ID]; ok {
			rank = &r
		}
		es = append(es, view.Entry(e, cards, code, rank))
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		a, b := nodes[i], nodes[j]
		if a.Wave != b.Wave {
			return a.Wave < b.Wave
		}
		if a.Bracket != b.Bracket {
			return a.Bracket < b.Bracket
		}
		if a.Round != b.Round {
			return a.Round < b.Round
		}
		return a.Position < b.Position
	})
	ns := make([]map[string]any, 0, len(nodes))
	for _, n := range nodes {
		code := ""
		if n.GroupID != nil {
			code = codes[*n.GroupID]
		}
		ns = append(ns, view.Node(n, code))
	}
	gs := make([]map[string]any, 0, len(groups))
	for _, g := range groups {
		gs = append(gs, view.Group(g.Code, g.Standings))
	}
	out := map[string]any{"tournament_id": t.ID, "format": t.Format, "status": t.Status, "entries": es, "nodes": ns, "groups": gs}
	if !student {
		out["seed_value"] = t.SeedValue
	}
	return out
}

// Assignments is listResourceAssignments of the template assignments (F107-AC-29): platform read, keyset
// (assigned_at, assignment_id) cut in SQL.
type Assignments struct {
	Read   read.ReadSession
	Reader read.AssignmentReader
}

// AssignmentPage is one page of listResourceAssignments.
type AssignmentPage struct {
	Data       []port.AssignmentRow
	HasMore    bool
	NextCursor *string
}

type assignmentCursor struct {
	At time.Time `json:"a"`
	ID uuid.UUID `json:"i"`
}

// List implements listResourceAssignments for the tournament and challenge templates.
func (q Assignments) List(ctx context.Context, resourceType string, resourceID uuid.UUID, cursor string, limit int) (page AssignmentPage, err error) {
	ctx, done := observability.UseCase(ctx, "competition.query.ListResourceAssignments")
	defer func() { done(err) }()
	p, ok := authz.PrincipalFrom(ctx)
	if !ok || !p.Valid() {
		return page, problem.New(problem.Unauthenticated, "")
	}
	if err := authz.Require(ctx, p, authz.PermPlatformTenantDataAssign, authz.Resource{}); err != nil {
		return page, err
	}
	if !p.IsPlatform() {
		return page, problem.New(problem.PermissionDenied, "")
	}
	if challenge.ConfigType(resourceType) == "" {
		return page, problem.Invalid("resource_type", problem.SubInvalid, "module competition assigns tournament_template and challenge_template")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	var after *port.AssignmentRow
	if cursor != "" {
		b, err := base64.RawURLEncoding.DecodeString(cursor)
		var c assignmentCursor
		if err != nil || json.Unmarshal(b, &c) != nil || c.ID == uuid.Nil {
			return page, problem.Invalid("cursor", problem.SubInvalid, "is not a cursor of this listing")
		}
		after = &port.AssignmentRow{ID: c.ID, AssignedAt: c.At}
	}
	ps, err := authz.PlatformScope(p)
	if err != nil {
		return page, problem.New(problem.PermissionDenied, "").Wrap(err)
	}
	err = q.Read.Read(tenancy.WithScope(ctx, ps), func(ctx context.Context) error {
		rows, err := q.Reader.AssignmentPage(ctx, resourceType, resourceID, after, limit+1)
		if err != nil {
			return err
		}
		if len(rows) > limit {
			rows = rows[:limit]
			last := rows[len(rows)-1]
			b, _ := json.Marshal(assignmentCursor{At: last.AssignedAt, ID: last.ID})
			c := base64.RawURLEncoding.EncodeToString(b)
			page.HasMore, page.NextCursor = true, &c
		}
		page.Data = rows
		return nil
	})
	return page, err
}
