//go:build integration

package integration

import (
	"bytes"
	"context"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/contract"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/csvreader"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz/authztest"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// memStorage is a TEST DOUBLE (R17) of the S3 adapter (unit-tested against the AWS SigV4 vector).
type memStorage struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func (m *memStorage) PresignPut(_ context.Context, key string, _ int64, _ time.Duration) (string, error) {
	return "http://storage.test/ew-sis-imports/" + key + "?X-Amz-Signature=test", nil
}

func (m *memStorage) Head(_ context.Context, key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objects[key]
	if !ok {
		return 0, port.ErrObjectNotFound
	}
	return int64(len(b)), nil
}

func (m *memStorage) Open(_ context.Context, key string) (io.ReadCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.objects[key]
	if !ok {
		return nil, port.ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(b)), nil
}

// eicarScanner is a TEST DOUBLE (R17) of the clamd adapter: infected when the file contains "EICAR".
type eicarScanner struct{}

func (eicarScanner) Scan(_ context.Context, r io.Reader) (bool, string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return false, "", err
	}
	if strings.Contains(string(b), "EICAR") {
		return false, "Eicar-Test-Signature", nil
	}
	return true, "", nil
}

// queueSpy is a TEST DOUBLE (R17) of the IDENTITY_SIS_JOBS publisher (the stream is covered by TestLoginSync).
type queueSpy struct{ jobs []port.CsvImportJob }

func (q *queueSpy) EnqueueCsvImport(_ context.Context, j port.CsvImportJob) error {
	q.jobs = append(q.jobs, j)
	return nil
}

// F011-AC-01, AC-02, AC-03, AC-06, AC-07 through createSisImport → upload → worker → getSisImport.
func TestSisImportFlow(t *testing.T) {
	ctx := context.Background()
	e := setup(ctx, t)
	a := seedTenant(ctx, t, e.platform, "pgd-imp-a")
	b := seedTenant(ctx, t, e.platform, "pgd-imp-b")
	ta, school := uuid.MustParse(a.tenant), uuid.MustParse(a.school)
	sa := authztest.Ctx(ctx, authztest.Platform("system_admin"))
	conns := command.SisConnections{UoW: e.uow, Connections: write.SisConnectionRepo{}, Audit: e.deps.Audit}
	conn, err := conns.Create(sa, ta, "csv_import", "pilot")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conns.SetSchools(sa, conn.ID, []uuid.UUID{school}, "pilot"); err != nil {
		t.Fatal(err)
	}
	v, err := contract.New()
	if err != nil {
		t.Fatal(err)
	}
	store := &memStorage{objects: map[string][]byte{}}
	queue := &queueSpy{}
	create := command.CreateSisImport{UoW: e.uow, Imports: write.SisSyncRepo{}, Storage: store, Audit: e.deps.Audit}
	run := command.SisRun{UoW: e.uow, Savepoints: e.uow, Connections: write.SisConnectionRepo{}, Sync: write.SisSyncRepo{},
		Enrollments: write.EnrollmentRepo{}, Missing: write.SisSyncRepo{}, Transfers: write.SisSyncRepo{}, Students: write.StudentRepo{},
		Events: e.deps.Events, Audit: e.deps.Audit}
	process := command.ProcessSisImport{UoW: e.uow, Imports: write.SisSyncRepo{}, Storage: store, Scanner: eicarScanner{},
		Parser: csvreader.Parser{V: v}, Run: run, Audit: e.deps.Audit}
	watch := command.WatchSisImports{UoW: e.uow, Imports: write.SisSyncRepo{}, Storage: store, Jobs: queue}
	get := query.SisImports{Model: read.SisImportModel{Session: e.read}}
	admin := authztest.Ctx(ctx, authztest.Staff(ta, "school_admin"))

	upload := func(dry bool, body string) uuid.UUID {
		t.Helper()
		imp, err := create.Handle(admin, command.CreateSisImportInput{SchoolID: school, FileName: "students.csv", Size: int64(len(body)),
			EntityKinds: []string{"students"}, DryRun: &dry})
		if err != nil || imp.Status != "awaiting_upload" || imp.UploadURL == nil || imp.UploadExpiresAt == nil {
			t.Fatalf("create = %+v %v", imp, err)
		}
		var key string
		if err := e.platform.QueryRowContext(ctx, `SELECT storage_key FROM sis.sis_imports WHERE import_id = $1`, imp.ImportID).Scan(&key); err != nil {
			t.Fatal(err)
		}
		store.mu.Lock()
		store.objects[key] = []byte(body)
		store.mu.Unlock()
		return imp.ImportID
	}
	drain := func() {
		t.Helper()
		if _, err := watch.RunOnce(ctx); err != nil {
			t.Fatal(err)
		}
		for _, j := range queue.jobs {
			if err := process.Handle(ctx, j.TenantID, j.ImportID); err != nil {
				t.Fatal(err)
			}
		}
		queue.jobs = nil
	}
	const good = "student_external_id,school_external_id,student_code,full_name,grade_level,status,login_username,teacher_name\n" +
		"HS-1,TH-A,001,An,3,active,hs.imp.an,Co Lan\nHS-2,TH-A,002,Binh,3,active,hs.imp.binh,Co Lan\n"

	// AC-01: dry run → diff, no business change; AC-03: the teacher column is ignored.
	dry := upload(true, good)
	drain()
	got, err := get.Get(admin, school, dry)
	if err != nil || got.Status != "completed" || got.Diff["students"]["create"] != 2 || *got.RowsTotal != 2 {
		t.Fatalf("dry run = %+v %v", got, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE username LIKE 'hs.imp.%'`); n != 0 {
		t.Fatalf("dry run created %d users", n)
	}
	// Apply.
	applied := upload(false, good)
	drain()
	if got, err = get.Get(admin, school, applied); err != nil || got.Status != "completed_with_issues" || *got.RowsApplied != 2 { // missing_class issues
		t.Fatalf("apply = %+v %v", got, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE username LIKE 'hs.imp.%'`); n != 2 {
		t.Fatalf("users = %d", n)
	}
	// AC-02: a file with a violation is not applied; errors by row and column.
	bad := upload(false, "student_external_id,school_external_id,student_code,full_name,grade_level,status,login_username\n"+
		"HS-3,TH-A,003,Chi,3,active,hs.imp.chi\nHS-4,TH-A,004,,ten,active,hs.imp.dung\n")
	drain()
	got, err = get.Get(admin, school, bad)
	if err != nil || got.Status != "failed" || len(got.Errors) < 2 || *got.Errors[0].RowNo != 3 {
		t.Fatalf("violations = %+v %v", got, err)
	}
	if n := count(ctx, t, e.platform, `SELECT count(*) FROM identity.users WHERE username = 'hs.imp.chi'`); n != 0 {
		t.Fatal("a valid row of a rejected file was applied")
	}
	// Malware is never read.
	infected := upload(false, "student_external_id\nEICAR\n")
	drain()
	if got, err = get.Get(admin, school, infected); err != nil || got.Status != "failed" || got.SyncRunID != nil {
		t.Fatalf("malware = %+v %v", got, err)
	}

	// AC-06: limits, permission, connection.
	tooBig := int64(command.CsvImportMax + 1)
	if _, err := create.Handle(admin, command.CreateSisImportInput{SchoolID: school, FileName: "s.csv", Size: tooBig, EntityKinds: []string{"students"}}); code(err) != problem.ValidationFailed {
		t.Fatalf("too big: %v", err)
	}
	if _, err := create.Handle(admin, command.CreateSisImportInput{SchoolID: uuid.MustParse(a.otherSchool), FileName: "s.csv", Size: 10, EntityKinds: []string{"students"}}); code(err) != problem.ValidationFailed {
		t.Fatalf("school without csv connection: %v", err)
	}
	authztest.ExpectDenied(t, func() error {
		_, err := create.Handle(authztest.Ctx(ctx, authztest.Staff(ta, "academic_manager")), command.CreateSisImportInput{SchoolID: school,
			FileName: "s.csv", Size: 10, EntityKinds: []string{"students"}})
		return err
	}())
	// AC-07: another tenant or another school → 404.
	if _, err := get.Get(authztest.Ctx(ctx, authztest.Staff(uuid.MustParse(b.tenant), "school_admin")), uuid.MustParse(b.school), applied); code(err) != problem.NotFound {
		t.Fatalf("other tenant: %v", err)
	}
	other := authztest.Ctx(ctx, authztest.Staff(ta, "school_admin", authztest.AtSchool(uuid.MustParse(a.otherSchool))))
	if _, err := get.Get(other, uuid.MustParse(a.otherSchool), applied); code(err) != problem.NotFound {
		t.Fatalf("other school: %v", err)
	}

	// An import never uploaded expires after the upload TTL.
	never, err := create.Handle(admin, command.CreateSisImportInput{SchoolID: school, FileName: "s.csv", Size: 10, EntityKinds: []string{"classes"}})
	if err != nil {
		t.Fatal(err)
	}
	later := func() time.Time { return time.Now().Add(time.Hour) }
	watch.Clock, process.Clock = later, later
	drain()
	if got, err = get.Get(admin, school, never.ImportID); err != nil || got.Status != "expired" {
		t.Fatalf("expired = %+v %v", got, err)
	}
	if n := count(ctx, t, e.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = 'audit.recorded'
		AND envelope->'payload'->>'action' IN ('sis_import.created', 'sis_import.processed')`); n != 10 {
		t.Fatalf("import audits = %d", n)
	}
}
