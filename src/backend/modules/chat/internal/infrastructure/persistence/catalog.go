package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/catalog"
)

// Catalog implements write.CatalogRepository and read.CatalogAdminReader on chat.sensitive_* (F116 US2): explicit SQL
// with tenant and deleted_at conditions. The platform rows (tenant_id NULL) are written in the platform scope
// (chat_platform); the extra terms of a tenant in the tenant scope (RLS). Term contents are never logged (R14).
type Catalog struct{}

var (
	_ write.CatalogRepository = Catalog{}
	_ read.CatalogAdminReader = Catalog{}
)

const nilTenant = "00000000-0000-0000-0000-000000000000"

func uniqueViolation(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && pe.Code == "23505"
}

func dup(err error, what string) error {
	if uniqueViolation(err) {
		return write.ErrDuplicate
	}
	return fmt.Errorf("chat: %s: %w", what, err)
}

// ---------- categories

const categoryColumns = `c.category_id, c.code, c.name, c.description, c.severity, c.default_action, c.enabled, c.allow_tenant_toggle,
	c.version, c.updated_at`

func scanCategory(sc interface{ Scan(...any) error }, extra ...any) (catalog.StoredCategory, error) {
	var c catalog.StoredCategory
	var desc sql.NullString
	dst := append([]any{&c.ID, &c.Code, &c.Name, &desc, &c.Severity, &c.DefaultAction, &c.Enabled, &c.AllowTenantToggle, &c.Version,
		&c.UpdatedAt}, extra...)
	if err := sc.Scan(dst...); err != nil {
		return c, err
	}
	c.Description = nullString(desc)
	c.UpdatedAt = c.UpdatedAt.UTC()
	return c, nil
}

func (Catalog) category(ctx context.Context, where string, arg any) (*catalog.StoredCategory, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	// tenancyguard:ok categories are platform-only (CHECK tenant_id IS NULL); no FOR UPDATE: a tenant session cannot lock platform rows (RLS)
	c, err := scanCategory(t.QueryRowContext(ctx, `SELECT `+categoryColumns+` FROM chat.sensitive_term_categories c
		WHERE c.tenant_id IS NULL AND c.deleted_at IS NULL AND `+where, arg))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: category: %w", err)
	}
	return &c, nil
}

// Category implements write.CatalogRepository.
func (r Catalog) Category(ctx context.Context, id uuid.UUID) (*catalog.StoredCategory, error) {
	return r.category(ctx, "c.category_id = $1", id)
}

// CategoryByCode implements write.CatalogRepository.
func (r Catalog) CategoryByCode(ctx context.Context, code string) (*catalog.StoredCategory, error) {
	return r.category(ctx, "c.code = $1", code)
}

// InsertCategory implements write.CatalogRepository.
func (Catalog) InsertCategory(ctx context.Context, c catalog.StoredCategory, actor uuid.UUID) error {
	_, err := exec(ctx, `INSERT INTO chat.sensitive_term_categories (category_id, tenant_id, created_at, created_by, updated_at, updated_by,
		code, name, description, severity, default_action, enabled, allow_tenant_toggle)
		VALUES ($1, NULL, now(), $2, now(), $2, $3, $4, $5, $6, $7, $8, $9)`,
		c.ID, actor, c.Code, c.Name, c.Description, c.Severity, c.DefaultAction, c.Enabled, c.AllowTenantToggle)
	if err != nil {
		return dup(err, "insert category")
	}
	return nil
}

// UpdateCategory implements write.CatalogRepository.
func (Catalog) UpdateCategory(ctx context.Context, c catalog.StoredCategory, actor uuid.UUID) error {
	n, err := exec(ctx, `UPDATE chat.sensitive_term_categories SET name = $2, description = $3, severity = $4, default_action = $5,
		enabled = $6, allow_tenant_toggle = $7, version = version + 1, updated_at = now(), updated_by = $8
		WHERE category_id = $1 AND tenant_id IS NULL AND deleted_at IS NULL AND version = $9`,
		c.ID, c.Name, c.Description, c.Severity, c.DefaultAction, c.Enabled, c.AllowTenantToggle, actor, c.Version-1)
	if err != nil {
		return fmt.Errorf("chat: update category: %w", err)
	}
	if n == 0 {
		return write.ErrStale
	}
	return nil
}

// SoftDeleteCategory implements write.CatalogRepository (R21: soft-delete only).
func (Catalog) SoftDeleteCategory(ctx context.Context, id, actor uuid.UUID) error {
	_, err := exec(ctx, `UPDATE chat.sensitive_term_categories SET deleted_at = now(), deleted_by = $2, version = version + 1,
		updated_at = now(), updated_by = $2 WHERE category_id = $1 AND tenant_id IS NULL AND deleted_at IS NULL`, id, actor)
	if err != nil {
		return fmt.Errorf("chat: delete category: %w", err)
	}
	return nil
}

// LiveTerms implements write.CatalogRepository.
func (Catalog) LiveTerms(ctx context.Context, category uuid.UUID) (int, error) {
	// tenancyguard:ok platform scope (chat_platform): live terms of every tenant_id of the category (deletion guard)
	return count(ctx, `SELECT count(*) FROM chat.sensitive_terms t WHERE t.category_id = $1 AND t.deleted_at IS NULL`, category)
}

// ---------- terms

const termColumns = `t.term_id, t.tenant_id, t.category_id, t.term, t.match_type, t.locale, t.variants, t.severity_override,
	t.action_override, t.enabled, t.status, t.note, t.published_in_version, t.version, t.created_at, t.updated_at`

func scanTerm(sc interface{ Scan(...any) error }) (catalog.StoredTerm, error) {
	var t catalog.StoredTerm
	var tenant uuid.NullUUID
	var variants []byte
	var sev, act, note sql.NullString
	var pub sql.NullInt64
	if err := sc.Scan(&t.ID, &tenant, &t.CategoryID, &t.Term, &t.MatchType, &t.Locale, &variants, &sev, &act, &t.Enabled, &t.Status,
		&note, &pub, &t.Version, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return t, err
	}
	t.TenantID, t.SeverityOverride, t.ActionOverride, t.Note = nullUUID(tenant), nullString(sev), nullString(act), nullString(note)
	if pub.Valid {
		v := pub.Int64
		t.PublishedInVersion = &v
	}
	if len(variants) > 0 {
		if err := json.Unmarshal(variants, &t.Variants); err != nil {
			return t, fmt.Errorf("variants: %w", err)
		}
	}
	t.CreatedAt, t.UpdatedAt = t.CreatedAt.UTC(), t.UpdatedAt.UTC()
	return t, nil
}

func variantsJSON(v []string) any {
	if len(v) == 0 {
		return nil
	}
	b, _ := json.Marshal(v)
	return string(b)
}

// Term implements write.CatalogRepository.
func (Catalog) Term(ctx context.Context, tenant *uuid.UUID, id uuid.UUID) (*catalog.StoredTerm, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var row *sql.Row
	if tenant == nil {
		row = t.QueryRowContext(ctx, `SELECT `+termColumns+` FROM chat.sensitive_terms t
			WHERE t.term_id = $1 AND t.tenant_id IS NULL AND t.deleted_at IS NULL`, id)
	} else {
		row = t.QueryRowContext(ctx, `SELECT `+termColumns+` FROM chat.sensitive_terms t
			WHERE t.term_id = $1 AND (t.tenant_id = $2 OR t.tenant_id IS NULL) AND t.deleted_at IS NULL`, id, *tenant)
	}
	st, err := scanTerm(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: term: %w", err)
	}
	return &st, nil
}

// InsertTerm implements write.CatalogRepository.
func (Catalog) InsertTerm(ctx context.Context, t catalog.StoredTerm, actor uuid.UUID) error {
	_, err := exec(ctx, `INSERT INTO chat.sensitive_terms (term_id, tenant_id, created_at, created_by, updated_at, updated_by, category_id,
		term, match_type, locale, variants, severity_override, action_override, enabled, status, note)
		VALUES ($1, $2, now(), $3, now(), $3, $4, $5, $6, $7, $8::jsonb, $9, $10, $11, 'draft', $12)`,
		t.ID, t.TenantID, actor, t.CategoryID, t.Term, t.MatchType, t.Locale, variantsJSON(t.Variants), t.SeverityOverride,
		t.ActionOverride, t.Enabled, t.Note)
	if err != nil {
		return dup(err, "insert term")
	}
	return nil
}

// UpdateTerm implements write.CatalogRepository.
func (Catalog) UpdateTerm(ctx context.Context, t catalog.StoredTerm, actor uuid.UUID) error {
	// tenancyguard:ok tenant_id is matched with coalesce (NULL = platform scope) and deleted_at IS NULL in the concatenated literal
	n, err := exec(ctx, `UPDATE chat.sensitive_terms SET category_id = $3, term = $4, match_type = $5, locale = $6, variants = $7::jsonb,
		severity_override = $8, action_override = $9, enabled = $10, note = $11, status = 'draft', version = version + 1,
		updated_at = now(), updated_by = $12
		WHERE term_id = $1 AND coalesce(tenant_id, '`+nilTenant+`'::uuid) = coalesce($2::uuid, '`+nilTenant+`'::uuid) AND deleted_at IS NULL
		AND version = $13`,
		t.ID, t.TenantID, t.CategoryID, t.Term, t.MatchType, t.Locale, variantsJSON(t.Variants), t.SeverityOverride, t.ActionOverride,
		t.Enabled, t.Note, actor, t.Version-1)
	if err != nil {
		return dup(err, "update term")
	}
	if n == 0 {
		return write.ErrStale
	}
	return nil
}

// SoftDeleteTerm implements write.CatalogRepository (R21).
func (Catalog) SoftDeleteTerm(ctx context.Context, tenant *uuid.UUID, id, actor uuid.UUID) error {
	_, err := exec(ctx, `UPDATE chat.sensitive_terms SET deleted_at = now(), deleted_by = $3, version = version + 1, updated_at = now(),
		updated_by = $3 WHERE term_id = $1 AND coalesce(tenant_id, '`+nilTenant+`'::uuid) = coalesce($2::uuid, '`+nilTenant+`'::uuid)
		AND deleted_at IS NULL`, id, tenant, actor)
	if err != nil {
		return fmt.Errorf("chat: delete term: %w", err)
	}
	return nil
}

// Patterns implements write.CatalogRepository.
func (Catalog) Patterns(ctx context.Context, tenant *uuid.UUID, except uuid.UUID) (int, error) {
	// tenancyguard:ok tenant_id is matched with coalesce (NULL = platform scope) and deleted_at IS NULL in the concatenated literal
	return count(ctx, `SELECT count(*) FROM chat.sensitive_terms t WHERE coalesce(t.tenant_id, '`+nilTenant+`'::uuid) =
		coalesce($1::uuid, '`+nilTenant+`'::uuid) AND t.match_type = 'pattern' AND t.term_id <> $2 AND t.deleted_at IS NULL`, tenant, except)
}

// ---------- allowlist (platform only)

// Allow implements write.CatalogRepository.
func (Catalog) Allow(ctx context.Context, id uuid.UUID) (*catalog.StoredAllow, error) {
	t, err := tx(ctx)
	if err != nil {
		return nil, err
	}
	var a catalog.StoredAllow
	var reason sql.NullString
	err = t.QueryRowContext(ctx, `SELECT a.entry_id, a.phrase, a.locale, a.reason, a.created_at FROM chat.sensitive_allowlist a
		WHERE a.entry_id = $1 AND a.tenant_id IS NULL AND a.deleted_at IS NULL FOR UPDATE`, id).
		Scan(&a.ID, &a.Phrase, &a.Locale, &reason, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("chat: allowlist entry: %w", err)
	}
	a.Reason = nullString(reason)
	return &a, nil
}

// InsertAllow implements write.CatalogRepository.
func (Catalog) InsertAllow(ctx context.Context, a catalog.StoredAllow, actor uuid.UUID) error {
	_, err := exec(ctx, `INSERT INTO chat.sensitive_allowlist (entry_id, tenant_id, created_at, created_by, updated_at, updated_by, phrase,
		locale, reason) VALUES ($1, NULL, now(), $2, now(), $2, $3, $4, $5)`, a.ID, actor, a.Phrase, a.Locale, a.Reason)
	if err != nil {
		return dup(err, "insert allowlist entry")
	}
	return nil
}

// SoftDeleteAllow implements write.CatalogRepository (R21).
func (Catalog) SoftDeleteAllow(ctx context.Context, id, actor uuid.UUID) error {
	_, err := exec(ctx, `UPDATE chat.sensitive_allowlist SET deleted_at = now(), deleted_by = $2, version = version + 1, updated_at = now(),
		updated_by = $2 WHERE entry_id = $1 AND tenant_id IS NULL AND deleted_at IS NULL`, id, actor)
	if err != nil {
		return fmt.Errorf("chat: delete allowlist entry: %w", err)
	}
	return nil
}

// ---------- snapshots and versions

func snapshotCategories(ctx context.Context, q *sql.Tx) ([]catalog.Category, error) {
	// tenancyguard:ok categories are platform-only (tenant_id NULL): every live category, enabled or not
	rows, err := q.QueryContext(ctx, `SELECT `+categoryColumns+` FROM chat.sensitive_term_categories c
		WHERE c.tenant_id IS NULL AND c.deleted_at IS NULL ORDER BY c.code`)
	if err != nil {
		return nil, fmt.Errorf("chat: snapshot categories: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []catalog.Category
	for rows.Next() {
		c, err := scanCategory(rows)
		if err != nil {
			return nil, fmt.Errorf("chat: snapshot categories: %w", err)
		}
		out = append(out, c.Snapshot())
	}
	return out, rows.Err()
}

func snapshotTerms(ctx context.Context, q *sql.Tx, tenant *uuid.UUID) ([]catalog.Term, []uuid.UUID, error) {
	var rows *sql.Rows
	var err error
	if tenant == nil {
		rows, err = q.QueryContext(ctx, `SELECT `+termColumns+` FROM chat.sensitive_terms t
			JOIN chat.sensitive_term_categories c ON c.category_id = t.category_id AND c.deleted_at IS NULL
			WHERE t.tenant_id IS NULL AND t.deleted_at IS NULL AND t.enabled ORDER BY t.created_at, t.term_id`)
	} else {
		rows, err = q.QueryContext(ctx, `SELECT `+termColumns+` FROM chat.sensitive_terms t
			JOIN chat.sensitive_term_categories c ON c.category_id = t.category_id AND c.deleted_at IS NULL
			WHERE t.tenant_id = $1 AND t.deleted_at IS NULL AND t.enabled ORDER BY t.created_at, t.term_id`, *tenant)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("chat: snapshot terms: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []catalog.Term{}
	var ids []uuid.UUID
	for rows.Next() {
		t, err := scanTerm(rows)
		if err != nil {
			return nil, nil, fmt.Errorf("chat: snapshot terms: %w", err)
		}
		out = append(out, t.Snapshot())
		ids = append(ids, t.ID)
	}
	return out, ids, rows.Err()
}

func snapshotAllowlist(ctx context.Context, q *sql.Tx) ([]catalog.AllowEntry, error) {
	rows, err := q.QueryContext(ctx, `SELECT a.phrase, a.locale FROM chat.sensitive_allowlist a
		WHERE a.tenant_id IS NULL AND a.deleted_at IS NULL ORDER BY a.created_at, a.entry_id`)
	if err != nil {
		return nil, fmt.Errorf("chat: snapshot allowlist: %w", err)
	}
	defer func() { _ = rows.Close() }()
	out := []catalog.AllowEntry{}
	for rows.Next() {
		var a catalog.AllowEntry
		if err := rows.Scan(&a.Phrase, &a.Locale); err != nil {
			return nil, fmt.Errorf("chat: snapshot allowlist: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// liveSnapshot builds the catalogue of the scope from the live rows: the platform part has the live categories, the
// enabled live terms of live categories and the allowlist; a tenant part only the enabled live extra terms of the
// tenant (a tenant only tightens: no tenant allowlist, MODERATION §11.3).
func liveSnapshot(ctx context.Context, q *sql.Tx, tenant *uuid.UUID) (catalog.Snapshot, []uuid.UUID, error) {
	s := catalog.Snapshot{Scope: catalog.ScopePlatform, Terms: []catalog.Term{}, Allowlist: []catalog.AllowEntry{}}
	if tenant != nil {
		s.Scope = catalog.ScopeTenant
	}
	var err error
	if tenant == nil {
		if s.Categories, err = snapshotCategories(ctx, q); err != nil {
			return s, nil, err
		}
	}
	var ids []uuid.UUID
	if s.Terms, ids, err = snapshotTerms(ctx, q, tenant); err != nil {
		return s, nil, err
	}
	if tenant == nil {
		if s.Allowlist, err = snapshotAllowlist(ctx, q); err != nil {
			return s, nil, err
		}
	}
	return s, ids, nil
}

// LiveSnapshot implements write.CatalogRepository and read.CatalogAdminReader.
func (Catalog) LiveSnapshot(ctx context.Context, tenant *uuid.UUID) (catalog.Snapshot, []uuid.UUID, error) {
	q, err := anyTx(ctx)
	if err != nil {
		return catalog.Snapshot{}, nil, err
	}
	return liveSnapshot(ctx, q, tenant)
}

// LockScope implements write.CatalogRepository.
func (Catalog) LockScope(ctx context.Context, tenant *uuid.UUID) error {
	key := "platform"
	if tenant != nil {
		key = tenant.String()
	}
	t, err := tx(ctx)
	if err != nil {
		return err
	}
	if _, err := t.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('chat.sensitive_catalog:' || $1))`, key); err != nil {
		return fmt.Errorf("chat: lock catalogue scope: %w", err)
	}
	return nil
}

// NextVersion implements write.CatalogRepository.
func (Catalog) NextVersion(ctx context.Context, tenant *uuid.UUID) (int64, error) {
	t, err := tx(ctx)
	if err != nil {
		return 0, err
	}
	var v int64
	// tenancyguard:ok chat.sensitive_catalog_versions is append-only (no deleted_at): versions of this scope only
	err = t.QueryRowContext(ctx, `SELECT coalesce(max(v.catalog_version), 0) + 1 FROM chat.sensitive_catalog_versions v
		WHERE coalesce(v.tenant_id, '`+nilTenant+`'::uuid) = coalesce($1::uuid, '`+nilTenant+`'::uuid)`, tenant).Scan(&v)
	if err != nil {
		return 0, fmt.Errorf("chat: next catalogue version: %w", err)
	}
	return v, nil
}

// PublishedSnapshot implements write.CatalogRepository.
func (Catalog) PublishedSnapshot(ctx context.Context) (catalog.Snapshot, error) {
	row, err := ModerationReader{}.LatestCatalog(ctx, nil, 0)
	if err != nil {
		return catalog.Snapshot{}, err
	}
	if row == nil {
		return catalog.Snapshot{}, errors.New("chat: no platform catalogue version")
	}
	return catalog.ParseSnapshot(row.Snapshot)
}

// InsertVersion implements write.CatalogRepository.
func (Catalog) InsertVersion(ctx context.Context, v catalog.StoredVersion) error {
	_, err := exec(ctx, `INSERT INTO chat.sensitive_catalog_versions (catalog_version_id, tenant_id, created_at, created_by, catalog_version,
		scope, published_at, published_by, change_summary, term_count, checksum, snapshot, regression_recall, regression_fp_rate)
		VALUES ($1, $2, $3, $4, $5, $6, $3, $4, $7, $8, $9, $10::jsonb, $11, $12)`,
		v.ID, v.TenantID, v.PublishedAt, v.PublishedBy, v.Version, v.Scope, v.ChangeSummary, v.TermCount, v.Checksum, string(v.Snapshot),
		v.Recall, v.FPRate)
	if err != nil {
		return dup(err, "insert catalogue version")
	}
	return nil
}

// MarkPublished implements write.CatalogRepository.
func (Catalog) MarkPublished(ctx context.Context, tenant *uuid.UUID, ids []uuid.UUID, version int64) error {
	if len(ids) == 0 {
		return nil
	}
	// tenancyguard:ok tenant_id is matched with coalesce (NULL = platform scope) and deleted_at IS NULL in the concatenated literal
	_, err := exec(ctx, `UPDATE chat.sensitive_terms SET status = 'published', published_in_version = $3, updated_at = now()
		WHERE term_id = ANY($2::uuid[]) AND coalesce(tenant_id, '`+nilTenant+`'::uuid) = coalesce($1::uuid, '`+nilTenant+`'::uuid)
		AND deleted_at IS NULL`, tenant, idStrings(ids), version)
	if err != nil {
		return fmt.Errorf("chat: mark terms published: %w", err)
	}
	return nil
}

// TenantSettings implements write.CatalogRepository.
func (Catalog) TenantSettings(ctx context.Context, tenant uuid.UUID) (map[uuid.UUID]bool, error) {
	return ModerationReader{}.TenantCategorySettings(ctx, tenant)
}

// SetTenantCategory implements write.CatalogRepository.
func (Catalog) SetTenantCategory(ctx context.Context, tenant, category uuid.UUID, enabled bool, reason string, actor uuid.UUID) error {
	n, err := exec(ctx, `UPDATE chat.tenant_category_settings SET enabled = $3, reason = $4, version = version + 1, updated_at = now(),
		updated_by = $5 WHERE tenant_id = $1 AND category_id = $2 AND deleted_at IS NULL`, tenant, category, enabled, reason, actor)
	if err != nil {
		return fmt.Errorf("chat: tenant category setting: %w", err)
	}
	if n > 0 {
		return nil
	}
	_, err = exec(ctx, `INSERT INTO chat.tenant_category_settings (setting_id, tenant_id, created_at, created_by, updated_at, updated_by,
		category_id, enabled, reason) VALUES ($1, $2, now(), $5, now(), $5, $3, $4, $6)`, uuid.Must(uuid.NewV7()), tenant, category, enabled,
		actor, reason)
	if err != nil {
		return fmt.Errorf("chat: tenant category setting: %w", err)
	}
	return nil
}

// TenantTerms implements write.CatalogRepository.
func (Catalog) TenantTerms(ctx context.Context, tenant, category uuid.UUID) (int, error) {
	return count(ctx, `SELECT count(*) FROM chat.sensitive_terms t WHERE t.tenant_id = $1 AND t.category_id = $2 AND t.deleted_at IS NULL`,
		tenant, category)
}

// ---------- read side

// Categories implements read.CatalogAdminReader.
func (Catalog) Categories(ctx context.Context, after *read.CategoryCursor, limit int) ([]read.CategoryRow, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	code, id := categoryKeyset(after)
	// tenancyguard:ok categories are platform-only (tenant_id NULL); the term count covers live platform terms only
	rows, err := t.QueryContext(ctx, `SELECT `+categoryColumns+`, (SELECT count(*) FROM chat.sensitive_terms t
			WHERE t.category_id = c.category_id AND t.tenant_id IS NULL AND t.deleted_at IS NULL)
		FROM chat.sensitive_term_categories c WHERE c.tenant_id IS NULL AND c.deleted_at IS NULL
			AND ($1::varchar IS NULL OR (c.code, c.category_id) > ($1::varchar, $2::uuid))
		ORDER BY c.code, c.category_id LIMIT $3`, code, id, limit)
	if err != nil {
		return nil, fmt.Errorf("chat: categories: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.CategoryRow
	for rows.Next() {
		var r read.CategoryRow
		if r.StoredCategory, err = scanCategory(rows, &r.TermCount); err != nil {
			return nil, fmt.Errorf("chat: categories: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// categoryKeyset returns the (code, category_id) bounds of a category page (nil code: first page).
func categoryKeyset(after *read.CategoryCursor) (*string, uuid.UUID) {
	if after == nil {
		return nil, uuid.Nil
	}
	c := after.Code
	return &c, after.ID
}

// Terms implements read.CatalogAdminReader.
func (Catalog) Terms(ctx context.Context, q read.TermQuery) ([]catalog.StoredTerm, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	var args []any
	arg := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	conds := []string{"t.deleted_at IS NULL"}
	if q.Tenants == nil {
		conds = append(conds, "t.tenant_id IS NULL")
	} else {
		conds = append(conds, "t.tenant_id = ANY("+arg(idStrings(q.Tenants))+"::uuid[])")
	}
	if q.CategoryID != nil {
		conds = append(conds, "t.category_id = "+arg(*q.CategoryID))
	}
	if q.Locale != "" {
		conds = append(conds, "t.locale = "+arg(q.Locale))
	}
	if q.MatchType != "" {
		conds = append(conds, "t.match_type = "+arg(q.MatchType))
	}
	if q.Status != "" {
		conds = append(conds, "t.status = "+arg(q.Status))
	}
	if q.Text != "" {
		esc := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(strings.ToLower(q.Text))
		conds = append(conds, "lower(t.term) LIKE "+arg("%"+esc+"%"))
	}
	if q.After != nil {
		conds = append(conds, "(t.created_at, t.term_id) > ("+arg(q.After.At)+", "+arg(q.After.ID)+")")
	}
	lim := arg(q.Limit)
	rows, err := t.QueryContext(ctx, `SELECT `+termColumns+` FROM chat.sensitive_terms t WHERE `+strings.Join(conds, " AND ")+`
		ORDER BY t.created_at, t.term_id LIMIT `+lim, args...)
	if err != nil {
		return nil, fmt.Errorf("chat: terms: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []catalog.StoredTerm
	for rows.Next() {
		st, err := scanTerm(rows)
		if err != nil {
			return nil, fmt.Errorf("chat: terms: %w", err)
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// Allowlist implements read.CatalogAdminReader.
func (Catalog) Allowlist(ctx context.Context, after *read.Cursor, limit int) ([]catalog.StoredAllow, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	var rows *sql.Rows
	if after == nil {
		rows, err = t.QueryContext(ctx, `SELECT a.entry_id, a.phrase, a.locale, a.reason, a.created_at FROM chat.sensitive_allowlist a
			WHERE a.tenant_id IS NULL AND a.deleted_at IS NULL ORDER BY a.created_at, a.entry_id LIMIT $1`, limit)
	} else {
		rows, err = t.QueryContext(ctx, `SELECT a.entry_id, a.phrase, a.locale, a.reason, a.created_at FROM chat.sensitive_allowlist a
			WHERE a.tenant_id IS NULL AND a.deleted_at IS NULL AND (a.created_at, a.entry_id) > ($1, $2)
			ORDER BY a.created_at, a.entry_id LIMIT $3`, after.At, after.ID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("chat: allowlist: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []catalog.StoredAllow
	for rows.Next() {
		var a catalog.StoredAllow
		var reason sql.NullString
		if err := rows.Scan(&a.ID, &a.Phrase, &a.Locale, &reason, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("chat: allowlist: %w", err)
		}
		a.Reason, a.CreatedAt = nullString(reason), a.CreatedAt.UTC()
		out = append(out, a)
	}
	return out, rows.Err()
}

// Versions implements read.CatalogAdminReader.
func (Catalog) Versions(ctx context.Context, tenants []uuid.UUID, before int64, limit int) ([]catalog.StoredVersion, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	if before <= 0 {
		before = 1 << 62
	}
	const cols = `v.catalog_version_id, v.tenant_id, v.scope, v.catalog_version, v.published_at, v.published_by, v.change_summary,
		v.term_count, v.checksum, v.regression_recall::float8, v.regression_fp_rate::float8`
	var rows *sql.Rows
	if tenants == nil {
		// tenancyguard:ok chat.sensitive_catalog_versions is append-only (no deleted_at); platform versions only
		rows, err = t.QueryContext(ctx, `SELECT `+cols+` FROM chat.sensitive_catalog_versions v
			WHERE v.tenant_id IS NULL AND v.catalog_version < $1 ORDER BY v.catalog_version DESC LIMIT $2`, before, limit)
	} else {
		// tenancyguard:ok chat.sensitive_catalog_versions is append-only (no deleted_at); versions of the given tenants only
		rows, err = t.QueryContext(ctx, `SELECT `+cols+` FROM chat.sensitive_catalog_versions v
			WHERE v.tenant_id = ANY($1::uuid[]) AND v.catalog_version < $2 ORDER BY v.catalog_version DESC, v.tenant_id LIMIT $3`,
			idStrings(tenants), before, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("chat: catalogue versions: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []catalog.StoredVersion
	for rows.Next() {
		var v catalog.StoredVersion
		var tenant, by uuid.NullUUID
		var recall, fp sql.NullFloat64
		if err := rows.Scan(&v.ID, &tenant, &v.Scope, &v.Version, &v.PublishedAt, &by, &v.ChangeSummary, &v.TermCount, &v.Checksum,
			&recall, &fp); err != nil {
			return nil, fmt.Errorf("chat: catalogue versions: %w", err)
		}
		v.TenantID, v.PublishedBy, v.PublishedAt = nullUUID(tenant), nullUUID(by), v.PublishedAt.UTC()
		if recall.Valid {
			v.Recall = &recall.Float64
		}
		if fp.Valid {
			v.FPRate = &fp.Float64
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// TenantCategories implements read.CatalogAdminReader.
func (Catalog) TenantCategories(ctx context.Context, tenant uuid.UUID, after *read.CategoryCursor, limit int) ([]read.TenantCategoryRow, error) {
	t, err := readTx(ctx)
	if err != nil {
		return nil, err
	}
	code, id := categoryKeyset(after)
	// tenancyguard:ok categories are platform-only (tenant_id NULL); the setting and the term count are of this tenant only
	rows, err := t.QueryContext(ctx, `SELECT `+categoryColumns+`,
			(SELECT s.enabled FROM chat.tenant_category_settings s WHERE s.tenant_id = $1 AND s.category_id = c.category_id
				AND s.deleted_at IS NULL),
			(SELECT count(*) FROM chat.sensitive_terms t WHERE t.tenant_id = $1 AND t.category_id = c.category_id AND t.deleted_at IS NULL)
		FROM chat.sensitive_term_categories c WHERE c.tenant_id IS NULL AND c.deleted_at IS NULL
			AND ($2::varchar IS NULL OR (c.code, c.category_id) > ($2::varchar, $3::uuid))
		ORDER BY c.code, c.category_id LIMIT $4`, tenant, code, id, limit)
	if err != nil {
		return nil, fmt.Errorf("chat: tenant categories: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []read.TenantCategoryRow
	for rows.Next() {
		var r read.TenantCategoryRow
		var in sql.NullBool
		if r.StoredCategory, err = scanCategory(rows, &in, &r.TenantTermCount); err != nil {
			return nil, fmt.Errorf("chat: tenant categories: %w", err)
		}
		if in.Valid {
			v := in.Bool
			r.InTenant = &v
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
