// Package fixtures is the F020 test fixture.
//
// TEST FIXTURE — không dùng trong runtime (R17): a ResourceHandler over the fixture tables
// content.fixture_contents / fixture_content_versions / fixture_attempts (ent package ./ent), and the SQL that
// installs them in a test database. Only tests import this package; no binary links it.
package fixtures

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/trash"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
	fent "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/tests/integration/fixtures/ent"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/tests/integration/fixtures/ent/fixturecontent"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/tests/integration/fixtures/ent/fixturecontentversion"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/tests/integration/fixtures/ent/migrate"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"

	_ "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/tests/integration/fixtures/ent/runtime" // mixin hooks, interceptors
)

// Client returns the fixture ent client bound to the transaction of ctx.
func Client(ctx context.Context) *fent.Client { return fent.NewClient(fent.Driver(entx.Driver(ctx))) }

// Handler is the fixture ResourceHandler (resource type "question").
type Handler struct{}

var _ port.ResourceHandler = Handler{}

// ResourceType implements port.ResourceHandler.
func (Handler) ResourceType() contentmodel.ResourceType { return contentmodel.ResourceQuestion }

// WritePermission implements port.ResourceHandler.
func (Handler) WritePermission() authz.Permission { return authz.PermLearningContentWrite }

func owner(r *fent.FixtureContent) contentmodel.Owner {
	return contentmodel.Owner{Scope: contentmodel.OwnerScope(r.OwnerScope), TenantID: r.TenantID}
}

// Owners implements port.ResourceHandler.
func (Handler) Owners(ctx context.Context, idList []uuid.UUID) (map[uuid.UUID]contentmodel.Owner, error) {
	rows, err := Client(ctx).FixtureContent.Query().Where(fixturecontent.IDIn(idList...)).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]contentmodel.Owner, len(rows))
	for _, r := range rows {
		out[r.ID] = owner(r)
	}
	return out, nil
}

// SoftDelete implements port.ResourceHandler.
func (Handler) SoftDelete(ctx context.Context, id uuid.UUID, reason string) (port.Deleted, error) {
	c := Client(ctx)
	r, err := c.FixtureContent.Get(ctx, id)
	if fent.IsNotFound(err) {
		return port.Deleted{}, problem.New(problem.NotFound, "")
	}
	if err != nil {
		return port.Deleted{}, err
	}
	if err := contentmodel.CanDeleteItem(r.CurrentVersionID); err != nil {
		return port.Deleted{}, err
	}
	if err := c.FixtureContent.DeleteOneID(id).Exec(entx.WithDeleteReason(ctx, reason)); err != nil {
		return port.Deleted{}, err
	}
	return port.Deleted{Owner: owner(r), Title: r.Title}, nil
}

func (Handler) entry(ctx context.Context, r *fent.FixtureContent) (trash.Entry, error) {
	conflict, err := Client(ctx).FixtureContent.Query().Where(fixturecontent.Code(r.Code), fixturecontent.DeletedAtIsNil(),
		fixturecontent.IDNEQ(r.ID), sameTenant(r.TenantID)).Exist(ctx)
	if err != nil {
		return trash.Entry{}, err
	}
	e := trash.Entry{ResourceType: contentmodel.ResourceQuestion, ResourceID: r.ID, TenantID: r.TenantID, Title: r.Title,
		DeletedBy: r.DeletedBy, Conflict: conflict}
	if r.DeletedAt != nil {
		e.DeletedAt = *r.DeletedAt
	}
	if r.DeleteReason != nil {
		e.DeleteReason = *r.DeleteReason
	}
	return e, nil
}

func sameTenant(t *uuid.UUID) func(*entsql.Selector) {
	if t == nil {
		return fixturecontent.TenantIDIsNil()
	}
	return fixturecontent.TenantID(*t)
}

// ListDeleted implements port.ResourceHandler (ctx carries tenancy.Bypass(trash)).
func (h Handler) ListDeleted(ctx context.Context, f port.TrashFilter) ([]trash.Entry, error) {
	if r, ok := tenancy.Bypassed(ctx); !ok || r != tenancy.ReasonTrash {
		return nil, errors.New("fixtures: ListDeleted needs tenancy.Bypass(trash)")
	}
	q := Client(ctx).FixtureContent.Query().Where(fixturecontent.DeletedAtNotNil())
	if len(f.TenantIDs) > 0 {
		if f.PlatformToo {
			q = q.Where(fixturecontent.Or(fixturecontent.TenantIDIn(f.TenantIDs...), fixturecontent.TenantIDIsNil()))
		} else {
			q = q.Where(fixturecontent.TenantIDIn(f.TenantIDs...))
		}
	}
	if f.AfterAt != nil {
		q = q.Where(fixturecontent.Or(fixturecontent.DeletedAtLT(*f.AfterAt),
			fixturecontent.And(fixturecontent.DeletedAt(*f.AfterAt), fixturecontent.IDLT(f.AfterID))))
	}
	rows, err := q.Order(fixturecontent.ByDeletedAt(entsql.OrderDesc()), fixturecontent.ByID(entsql.OrderDesc())).
		Limit(f.Limit + 1).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]trash.Entry, 0, len(rows))
	for _, r := range rows {
		e, err := h.entry(ctx, r)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// Restore implements port.ResourceHandler (ctx carries tenancy.Bypass(trash)).
func (h Handler) Restore(ctx context.Context, id uuid.UUID) (trash.Entry, error) {
	c := Client(ctx)
	r, err := c.FixtureContent.Get(ctx, id)
	if fent.IsNotFound(err) {
		return trash.Entry{}, problem.New(problem.NotFound, "")
	}
	if err != nil {
		return trash.Entry{}, err
	}
	if r.DeletedAt == nil {
		return trash.Entry{}, trash.ErrNotDeleted
	}
	e, err := h.entry(ctx, r)
	if err != nil {
		return trash.Entry{}, err
	}
	if err := trash.CheckRestore(e); err != nil {
		return e, err
	}
	if err := c.FixtureContent.UpdateOneID(id).ClearDeletedAt().ClearDeletedBy().ClearDeleteReason().Exec(ctx); err != nil {
		if fent.IsConstraintError(err) {
			return e, trash.ErrConflict
		}
		return e, err
	}
	return e, nil
}

// Clone implements port.ResourceHandler (ctx carries tenancy.Bypass(tenant_clone)): the item and its latest
// version become a tenant draft; attempts (student data) are never copied.
func (Handler) Clone(ctx context.Context, in port.CloneInput) (uuid.UUID, error) {
	c := Client(ctx)
	src, err := c.FixtureContent.Query().Where(fixturecontent.ID(in.SourceID), fixturecontent.TenantID(in.SourceTenant)).Only(ctx)
	if fent.IsNotFound(err) {
		return uuid.Nil, port.ErrNotInSourceTenant
	}
	if err != nil {
		return uuid.Nil, err
	}
	v, err := c.FixtureContentVersion.Query().Where(fixturecontentversion.FixtureContentID(src.ID)).
		Order(fixturecontentversion.ByVersionNo(entsql.OrderDesc())).First(ctx)
	if err != nil && !fent.IsNotFound(err) {
		return uuid.Nil, err
	}
	newID, target, source := ids.New(), in.TargetTenant, in.SourceTenant
	item := c.FixtureContent.Create().SetID(newID).SetTenantID(target).SetOwnerScope(string(contentmodel.OwnerTenant)).
		SetCode(src.Code).SetTitle(src.Title).SetClonedFromID(src.ID).SetClonedFromTenantID(source).
		SetClonedAt(in.At).SetClonedBy(in.Actor)
	if v != nil {
		item.SetLatestVersionNo(1)
	}
	if err := item.Exec(ctx); err != nil {
		return uuid.Nil, err
	}
	if v == nil {
		return newID, nil
	}
	err = c.FixtureContentVersion.Create().SetID(ids.New()).SetTenantID(target).SetOwnerScope(string(contentmodel.OwnerTenant)).
		SetFixtureContentID(newID).SetVersionNo(1).SetStatus(string(contentmodel.StatusDraft)).SetAuthorID(in.Actor).
		SetContentLanguage(v.ContentLanguage).SetCurriculumVersionID(v.CurriculumVersionID).SetBody(v.Body).
		SetBasedOnVersionID(v.ID).SetClonedFromID(v.ID).SetClonedFromTenantID(source).SetClonedAt(in.At).SetClonedBy(in.Actor).
		Exec(ctx)
	return newID, err
}

// Install creates the fixture tables in a learning database (as learning_migrator): ent DDL, the owner CHECK,
// FKs, RLS (H for items/versions, T for attempts), the _live view and the immutability trigger.
func Install(ctx context.Context, migrator *sql.DB) error {
	schemaOf := func(string) string { return "content" }
	ddl, err := entx.DDL(migrate.Tables, schemaOf)
	if err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString(ddl)
	for _, t := range []string{"content.fixture_contents", "content.fixture_content_versions"} {
		fmt.Fprintf(&b, "ALTER TABLE %s ADD CHECK (owner_scope IN ('platform','tenant')), ADD CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),"+
			" ADD FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);\n", t)
	}
	b.WriteString("ALTER TABLE content.fixture_content_versions ADD CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),\n" +
		"  ADD CHECK (content_language IN ('vi','en'));\n")
	b.WriteString("ALTER TABLE content.fixture_attempts ADD FOREIGN KEY (tenant_id) REFERENCES ref.tenants (tenant_id);\n")
	for _, x := range []struct {
		table string
		kind  tenancy.TableKind
		col   string
	}{{"content.fixture_contents", tenancy.KindH, "fixture_content_id"}, {"content.fixture_content_versions", tenancy.KindH, "fixture_content_id"},
		{"content.fixture_attempts", tenancy.KindT, ""}} {
		var opts []tenancy.RLSOption
		if x.kind == tenancy.KindH {
			opts = append(opts, tenancy.HybridResource("question", x.col))
		}
		rls, err := tenancy.RLSPolicies(x.table, x.kind, opts...)
		if err != nil {
			return err
		}
		b.WriteString(rls)
	}
	live, err := tenancy.LiveView("content.fixture_contents", "learning")
	if err != nil {
		return err
	}
	b.WriteString(live)
	b.WriteString("REVOKE ALL ON content.fixture_contents FROM learning_readonly;\n")
	b.WriteString("CREATE TRIGGER fixture_content_versions_immutable BEFORE UPDATE ON content.fixture_content_versions\n" +
		"  FOR EACH ROW EXECUTE FUNCTION content.forbid_published_update();\n")
	_, err = migrator.ExecContext(ctx, b.String())
	return err
}

// Tenant is a row of the tenant projection seeded directly by tests that do not exercise the consumer.
type Tenant struct {
	ID     uuid.UUID
	Code   string
	Status string
}

// SeedTenants inserts tenants into ref.tenants (as learning_migrator).
func SeedTenants(ctx context.Context, migrator *sql.DB, ts ...Tenant) error {
	for _, t := range ts {
		status := t.Status
		if status == "" {
			status = "active"
		}
		if _, err := migrator.ExecContext(ctx, `INSERT INTO ref.tenants (tenant_id, code, tenant_type, status, source_version)
			VALUES ($1, $2, 'school', $3, 1)`, t.ID, t.Code, status); err != nil {
			return err
		}
	}
	return nil
}

// Row is a fixture item as listed by the read helpers.
type Row struct {
	ID       uuid.UUID  `json:"id"`
	TenantID *uuid.UUID `json:"tenant_id"`
	Code     string     `json:"code"`
}

// CreateItem creates a fixture item with the scope of ctx (tenant scope: the session tenant; PlatformScope:
// a platform item) and returns its id.
func CreateItem(ctx context.Context, code string) (uuid.UUID, error) {
	s, ok := tenancy.ScopeFrom(ctx)
	if !ok {
		return uuid.Nil, entx.ErrMissingTenantScope
	}
	c := Client(ctx).FixtureContent.Create().SetCode(code).SetTitle("Fixture " + code)
	if s.PlatformOnly() {
		c.SetOwnerScope(string(contentmodel.OwnerPlatform))
	} else if id, single := s.SessionTenantID(); single {
		c.SetTenantID(id).SetOwnerScope(string(contentmodel.OwnerTenant))
	} else if ids := s.TenantIDs(); len(ids) == 1 {
		c.SetTenantID(ids[0]).SetOwnerScope(string(contentmodel.OwnerTenant))
	}
	r, err := c.Save(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return r.ID, nil
}

// List lists the fixture items visible to the scope of ctx through ent (global filter of entx.HybridContent).
func List(ctx context.Context) ([]Row, error) {
	rows, err := Client(ctx).FixtureContent.Query().Order(fixturecontent.ByCode()).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Row, len(rows))
	for i, r := range rows {
		out[i] = Row{ID: r.ID, TenantID: r.TenantID, Code: r.Code}
	}
	return out, nil
}

// Get loads one fixture item visible to the scope (404 NOT_FOUND otherwise, F020-AC-09).
func Get(ctx context.Context, id uuid.UUID) (Row, error) {
	r, err := Client(ctx).FixtureContent.Get(ctx, id)
	if fent.IsNotFound(err) {
		return Row{}, problem.New(problem.NotFound, "")
	}
	if err != nil {
		return Row{}, err
	}
	return Row{ID: r.ID, TenantID: r.TenantID, Code: r.Code}, nil
}

// ListExplicit lists the visible fixture items with explicit SQL and read.HybridCondition (MULTITENANCY §4.2).
func ListExplicit(ctx context.Context) ([]Row, error) {
	cond, args, err := read.HybridCondition(ctx, "fc", contentmodel.ResourceQuestion, "fixture_content_id", 1)
	if err != nil {
		return nil, err
	}
	tx, ok := entx.Tx(ctx)
	if !ok {
		return nil, entx.ErrNoTransaction
	}
	// tenancyguard:ok the condition of read.HybridCondition holds tenant_id and deleted_at IS NULL
	rows, err := tx.QueryContext(ctx, "SELECT fc.fixture_content_id, fc.tenant_id, fc.code FROM content.fixture_contents fc WHERE "+cond+
		" ORDER BY fc.code", args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []Row
	for rows.Next() {
		var r Row
		var t uuid.NullUUID
		if err := rows.Scan(&r.ID, &t, &r.Code); err != nil {
			return nil, err
		}
		if t.Valid {
			r.TenantID = &t.UUID
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Assign inserts an active assignment of a fixture item to tenant (as learning_platform; US3 tests use the
// command instead).
func Assign(ctx context.Context, platform *sql.DB, item, tenant uuid.UUID) error {
	_, err := platform.ExecContext(ctx, `INSERT INTO infra.tenant_resource_assignments
		(assignment_id, tenant_id, created_at, updated_at, resource_type, resource_id, assigned_by, assigned_at)
		VALUES ($1, $2, now(), now(), 'question', $3, $1, now())`, ids.New(), tenant, item)
	return err
}
