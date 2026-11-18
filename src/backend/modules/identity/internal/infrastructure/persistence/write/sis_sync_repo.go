package write

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent"
	entyear "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/academicyear"
	entclass "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/class"
	entext "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/externalidentity"
	entlink "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/sisidentitylink"
	entsislink "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/sislink"
	entstudent "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/student"
	entkey "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/studentloginkey"
	entsynced "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/syncedstudent"
	entrun "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/syncrun"
	entuser "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/user"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// SisSyncRepo implements write.SisSyncRepository.
type SisSyncRepo struct{}

var _ write.SisSyncRepository = SisSyncRepo{}

// CreateRun inserts the run.
func (SisSyncRepo) CreateRun(ctx context.Context, r *write.SyncRun) error {
	c := client(ctx).SyncRun.Create().SetID(r.ID).SetTenantID(r.TenantID).SetSisConnectionID(r.ConnectionID).SetMode(r.Mode).
		SetDryRun(r.DryRun).SetStatus(r.Status).SetStats(map[string]int{})
	if r.SchoolID != nil {
		c = c.SetSchoolID(*r.SchoolID)
	}
	row, err := c.Save(ctx)
	if err != nil {
		return err
	}
	r.StartedAt = row.StartedAt
	return nil
}

// FinishRun records the outcome.
func (SisSyncRepo) FinishRun(ctx context.Context, r *write.SyncRun) error {
	u := client(ctx).SyncRun.Update().Where(entrun.ID(r.ID)).SetStatus(r.Status).SetStats(r.Stats).AddVersion(1)
	if r.FinishedAt != nil {
		u = u.SetFinishedAt(*r.FinishedAt)
	}
	if r.FailureCode != nil {
		u = u.SetFailureCode(*r.FailureCode)
	}
	_, err := u.Save(ctx)
	return err
}

func optStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func optInt(n int) *int {
	if n == 0 {
		return nil
	}
	return &n
}

// AddErrors stores record errors (sis.sync_record_errors).
func (SisSyncRepo) AddErrors(ctx context.Context, runID uuid.UUID, errs []write.RecordError) error {
	c := client(ctx)
	bulk := make([]*ent.SyncRecordErrorCreate, 0, len(errs))
	for _, e := range errs {
		msg := e.Message
		if len(msg) > 1000 {
			msg = msg[:1000]
		}
		bulk = append(bulk, c.SyncRecordError.Create().SetSyncRunID(runID).SetEntityKind(e.Kind).SetNillableExternalID(optStr(truncate(e.ExternalID, 64))).
			SetNillableRowNo(optInt(e.Row)).SetNillableColumnName(optStr(truncate(e.Column, 64))).SetErrorCode(e.Code).SetMessage(msg))
	}
	if len(bulk) == 0 {
		return nil
	}
	return c.SyncRecordError.CreateBulk(bulk...).Exec(ctx)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// AddStaging stores the staged records (sis.staging_records).
func (SisSyncRepo) AddStaging(ctx context.Context, runID uuid.UUID, items []write.StagingItem) error {
	c := client(ctx)
	for start := 0; start < len(items); start += 500 {
		end := min(start+500, len(items))
		bulk := make([]*ent.StagingRecordCreate, 0, end-start)
		for _, it := range items[start:end] {
			bulk = append(bulk, c.StagingRecord.Create().SetSyncRunID(runID).SetEntityKind(it.Kind).SetExternalID(it.ExternalID).
				SetNillableRowNo(optInt(it.Row)).SetPayload(it.Payload).SetRawHash(it.Hash).SetAction(it.Action).SetStatus(it.Status))
		}
		if err := c.StagingRecord.CreateBulk(bulk...).Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

// LinkOf returns the link of a source key.
func (SisSyncRepo) LinkOf(ctx context.Context, conn uuid.UUID, kind, ext string) (uuid.UUID, string, bool, error) {
	l, err := client(ctx).SisLink.Query().Where(entsislink.SisConnectionID(conn), entsislink.EntityType(kind), entsislink.ExternalID(truncate(ext, 64))).Only(ctx)
	if ent.IsNotFound(err) {
		return uuid.Nil, "", false, nil
	}
	if err != nil {
		return uuid.Nil, "", false, err
	}
	return l.EntityID, l.LastHash, true, nil
}

// SaveLink creates or updates the link of a source key.
func (SisSyncRepo) SaveLink(ctx context.Context, conn uuid.UUID, kind, ext string, entityID uuid.UUID, schoolID *uuid.UUID, hash string) error {
	c := client(ctx)
	ext = truncate(ext, 64)
	n, err := c.SisLink.Update().Where(entsislink.SisConnectionID(conn), entsislink.EntityType(kind), entsislink.ExternalID(ext)).
		SetEntityID(entityID).SetLastHash(hash).SetLastSyncedAt(time.Now().UTC()).Save(ctx)
	if err != nil || n > 0 {
		return err
	}
	cr := c.SisLink.Create().SetEntityType(kind).SetEntityID(entityID).SetSisConnectionID(conn).SetExternalID(ext).SetLastHash(hash)
	if schoolID != nil {
		cr = cr.SetSchoolID(*schoolID)
	}
	return cr.Exec(ctx)
}

// SyncedStudentOf returns the SIS record of a student with the user of its active link.
func (SisSyncRepo) SyncedStudentOf(ctx context.Context, conn uuid.UUID, ext string) (*write.SyncedStudent, error) {
	c := client(ctx)
	s, err := c.SyncedStudent.Query().Where(entsynced.SisConnectionID(conn), entsynced.StudentExternalID(ext)).Only(ctx)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := &write.SyncedStudent{ConnectionID: conn, ExternalID: ext, SchoolID: s.SchoolID, StudentCode: s.StudentCode, RecordStatus: s.RecordStatus,
		NationalIDHash: s.NationalStudentIDHash, Hash: s.LastHash, MissingRuns: int(s.MissingRuns)}
	l, err := c.SisIdentityLink.Query().Where(entlink.SisConnectionID(conn), entlink.StudentExternalID(ext), entlink.Status("active")).Only(ctx)
	if err == nil {
		id := l.UserID
		out.UserID = &id
	} else if !ent.IsNotFound(err) {
		return nil, err
	}
	return out, nil
}

// SaveSyncedStudent upserts the SIS record by source key.
func (SisSyncRepo) SaveSyncedStudent(ctx context.Context, s write.SyncedStudent) error {
	c := client(ctx)
	n, err := c.SyncedStudent.Update().Where(entsynced.SisConnectionID(s.ConnectionID), entsynced.StudentExternalID(s.ExternalID)).
		SetSchoolID(s.SchoolID).SetStudentCode(s.StudentCode).SetRecordStatus(s.RecordStatus).SetLastHash(s.Hash).
		SetLastSyncedAt(time.Now().UTC()).SetMissingRuns(0).Save(ctx)
	if err != nil || n > 0 {
		return err
	}
	return c.SyncedStudent.Create().SetSisConnectionID(s.ConnectionID).SetStudentExternalID(s.ExternalID).SetSchoolID(s.SchoolID).
		SetStudentCode(s.StudentCode).SetRecordStatus(s.RecordStatus).SetNillableNationalStudentIDHash(s.NationalIDHash).SetLastHash(s.Hash).Exec(ctx)
}

// StudentsOfSchool lists the synced students of a school.
func (SisSyncRepo) StudentsOfSchool(ctx context.Context, conn, school uuid.UUID) ([]write.SyncedStudent, error) {
	rows, err := client(ctx).SyncedStudent.Query().Where(entsynced.SisConnectionID(conn), entsynced.SchoolID(school)).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]write.SyncedStudent, 0, len(rows))
	for _, s := range rows {
		out = append(out, write.SyncedStudent{ConnectionID: conn, ExternalID: s.StudentExternalID, SchoolID: s.SchoolID, StudentCode: s.StudentCode,
			RecordStatus: s.RecordStatus, Hash: s.LastHash, MissingRuns: int(s.MissingRuns)})
	}
	return out, nil
}

// UsernameOwner finds the owner of a username on every row, soft-deleted included (ADR-0069).
func (SisSyncRepo) UsernameOwner(ctx context.Context, username string) (*uuid.UUID, *string, error) {
	tx, ok := entx.Tx(ctx)
	if !ok {
		return nil, nil, entx.ErrNoTransaction
	}
	var id uuid.UUID
	var reason sql.NullString
	// tenancyguard:ok identity.users is global (no tenant_id); soft-deleted rows still hold their username
	err := tx.QueryRowContext(ctx, `SELECT user_id, delete_reason FROM identity.users WHERE lower(username) = lower($1)`, username).Scan(&id, &reason)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if reason.Valid {
		return &id, &reason.String, nil
	}
	return &id, nil, nil
}

// CreateSisStudent inserts the pending user, the student and the active identity link (ADR-0046).
func (SisSyncRepo) CreateSisStudent(ctx context.Context, s write.NewSisStudent) error {
	c := client(ctx)
	if _, err := c.User.Create().SetID(s.UserID).SetUserType("student").SetStatus("pending").SetUsername(s.Username).SetDisplayName(s.DisplayName).
		SetPrimaryTenantID(s.TenantID).SetPrimarySchoolID(s.SchoolID).Save(ctx); err != nil {
		return mapAccountErr(err)
	}
	if _, err := c.Student.Create().SetID(s.UserID).SetTenantID(s.TenantID).SetSchoolID(s.SchoolID).SetGradeLevel(int16(s.Grade)).
		SetNillableStudentCode(optStr(s.StudentCode)).SetFullName(s.DisplayName).Save(ctx); err != nil {
		return mapAccountErr(err)
	}
	_, err := c.SisIdentityLink.Create().SetTenantID(s.TenantID).SetUserID(s.UserID).SetSchoolID(s.SchoolID).SetSisConnectionID(s.ConnectionID).
		SetStudentExternalID(s.ExternalID).SetStatus("active").Save(ctx)
	return mapAccountErr(err)
}

// UpdateStudent updates the SIS-owned fields of a student.
func (SisSyncRepo) UpdateStudent(ctx context.Context, userID uuid.UUID, grade int, code, name string) error {
	c := client(ctx)
	u := c.Student.Update().Where(entstudent.ID(userID)).SetGradeLevel(int16(grade)).SetFullName(name).AddVersion(1)
	if code != "" {
		u = u.SetStudentCode(code)
	}
	if _, err := u.Save(ctx); err != nil {
		return err
	}
	_, err := c.User.Update().Where(entuser.ID(userID)).SetDisplayName(name).Save(ctx)
	return err
}

// SetUserStatus sets the status of a user and returns the previous one.
func (SisSyncRepo) SetUserStatus(ctx context.Context, userID uuid.UUID, status string) (string, bool, error) {
	c := client(ctx)
	u, err := c.User.Get(ctx, userID)
	if ent.IsNotFound(err) {
		return "", false, problem.New(problem.NotFound, "")
	}
	if err != nil {
		return "", false, err
	}
	if u.Status == status {
		return u.Status, false, nil
	}
	if _, err := c.User.Update().Where(entuser.ID(userID)).SetStatus(status).AddVersion(1).Save(ctx); err != nil {
		return "", false, err
	}
	return u.Status, true, nil
}

// SetLoginKey records the username login key of a student record.
func (SisSyncRepo) SetLoginKey(ctx context.Context, conn uuid.UUID, ext, username string) error {
	c := client(ctx)
	n, err := c.StudentLoginKey.Update().Where(entkey.SisConnectionID(conn), entkey.StudentExternalID(ext), entkey.LoginType("username")).
		SetLoginKey(username).Save(ctx)
	if err != nil || n > 0 {
		return err
	}
	return c.StudentLoginKey.Create().SetSisConnectionID(conn).SetLoginType("username").SetLoginKey(username).SetStudentExternalID(ext).Exec(ctx)
}

// AcademicYear finds or creates the academic year (YYYY-YYYY) of a school; a new year is current when the
// school has none.
func (SisSyncRepo) AcademicYear(ctx context.Context, school uuid.UUID, name string) (uuid.UUID, error) {
	c := client(ctx)
	y, err := c.AcademicYear.Query().Where(entyear.SchoolID(school), entyear.Name(name)).Only(ctx)
	if err == nil {
		return y.ID, nil
	}
	if !ent.IsNotFound(err) {
		return uuid.Nil, err
	}
	var from, to int
	if _, err := fmtSscanf(name, &from, &to); err != nil || to != from+1 {
		return uuid.Nil, problem.Invalid("academic_year", problem.SubInvalid, "must be YYYY-YYYY")
	}
	hasCurrent, err := c.AcademicYear.Query().Where(entyear.SchoolID(school), entyear.IsCurrent(true)).Exist(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	row, err := c.AcademicYear.Create().SetSchoolID(school).SetName(name).
		SetStartDate(time.Date(from, 8, 1, 0, 0, 0, 0, time.UTC)).SetEndDate(time.Date(to, 7, 31, 0, 0, 0, 0, time.UTC)).
		SetIsCurrent(!hasCurrent).Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return row.ID, nil
}

// CreateClass inserts a class.
func (SisSyncRepo) CreateClass(ctx context.Context, id, school, year uuid.UUID, grade int, name string) error {
	_, err := client(ctx).Class.Create().SetID(id).SetSchoolID(school).SetAcademicYearID(year).SetGradeLevel(int16(grade)).SetName(name).Save(ctx)
	if ent.IsConstraintError(err) {
		return problem.New(problem.Conflict, "class name exists in this school year").Wrap(err)
	}
	return err
}

// UpdateClass updates grade and name.
func (SisSyncRepo) UpdateClass(ctx context.Context, id uuid.UUID, grade int, name string) error {
	_, err := client(ctx).Class.Update().Where(entclass.ID(id)).SetGradeLevel(int16(grade)).SetName(name).AddVersion(1).Save(ctx)
	return err
}

// ClassSchool returns the school and grade of a class.
func (SisSyncRepo) ClassSchool(ctx context.Context, id uuid.UUID) (uuid.UUID, int, error) {
	k, err := client(ctx).Class.Get(ctx, id)
	if ent.IsNotFound(err) {
		return uuid.Nil, 0, problem.New(problem.NotFound, "")
	}
	if err != nil {
		return uuid.Nil, 0, err
	}
	return k.SchoolID, int(k.GradeLevel), nil
}

// RaiseIssue records a reconciliation issue.
func (SisSyncRepo) RaiseIssue(ctx context.Context, i write.Issue) (uuid.UUID, error) {
	refs := i.Refs
	if refs == nil {
		refs = map[string]any{}
	}
	c := client(ctx).ReconciliationIssue.Create().SetIssueType(i.Type).SetEntityRefs(refs).SetSisConnectionID(i.ConnectionID).
		SetNillableSuggestedAction(optStr(i.Suggested))
	if i.RunID != uuid.Nil { // provisioning issues have no run
		refs["sync_run_id"] = i.RunID.String()
		c = c.SetSyncRunID(i.RunID)
	}
	if i.SchoolID != nil {
		c = c.SetSchoolID(*i.SchoolID)
	}
	row, err := c.Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return row.ID, nil
}

// fmtSscanf parses "YYYY-YYYY".
func fmtSscanf(s string, from, to *int) (int, error) { return fmt.Sscanf(s, "%d-%d", from, to) }

var _ write.ProvisioningRepository = SisSyncRepo{}

// PendingStudent loads the user, its student row, its active SIS link and its local identity.
func (SisSyncRepo) PendingStudent(ctx context.Context, userID uuid.UUID, issuer string) (*write.PendingStudent, error) {
	c := client(ctx)
	u, err := c.User.Get(ctx, userID)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s, err := c.Student.Get(ctx, userID)
	if ent.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	out := &write.PendingStudent{UserID: userID, Username: u.Username, Status: u.Status, TenantID: s.TenantID, SchoolID: s.SchoolID,
		Grade: int(s.GradeLevel)}
	if u.DisplayName != nil {
		out.DisplayName = *u.DisplayName
	}
	l, err := c.SisIdentityLink.Query().Where(entlink.UserID(userID), entlink.Status("active")).Only(ctx)
	switch {
	case err == nil:
		out.ConnectionID, out.ExternalID = l.SisConnectionID, l.StudentExternalID
	case !ent.IsNotFound(err):
		return nil, err
	}
	id, err := c.ExternalIdentity.Query().Where(entext.UserID(userID), entext.Issuer(issuer), entext.Status("active")).Only(ctx)
	switch {
	case err == nil:
		out.Subject = &id.Subject
	case !ent.IsNotFound(err):
		return nil, err
	}
	return out, nil
}

// LinkIdentity records the Keycloak account of the user.
func (SisSyncRepo) LinkIdentity(ctx context.Context, userID uuid.UUID, issuer, subject string) error {
	c := client(ctx)
	if _, err := c.ExternalIdentity.Create().SetIssuer(issuer).SetSubject(subject).SetUserID(userID).Save(ctx); err != nil {
		return mapAccountErr(err)
	}
	_, err := c.User.Update().Where(entuser.ID(userID)).SetProvisionedAt(time.Now().UTC()).Save(ctx)
	return err
}
