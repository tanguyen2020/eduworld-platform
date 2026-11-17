package tenancy

import (
	"fmt"
	"regexp"
	"strings"
)

// TableKind is the table class of DATABASE_DESIGN (§1 legend, ADR-0089 §2).
type TableKind string

const (
	KindT      TableKind = "T"   // tenant-scoped, soft-delete
	KindTA     TableKind = "TA"  // tenant-scoped append-only
	KindTStar  TableKind = "T*"  // tenant nullable (NULL rows written by <module>_platform only)
	KindTAStar TableKind = "TA*" // append-only, tenant nullable
	KindH      TableKind = "H"   // hybrid content: tenant rows + platform rows assigned to the tenant
)

var (
	tableRe  = regexp.MustCompile(`^[a-z_][a-z0-9_]*\.[a-z_][a-z0-9_]*$`)
	identRe  = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)
	moduleRe = regexp.MustCompile(`^[a-z][a-z0-9]*$`)
)

type rlsOptions struct {
	resourceType string
	idColumn     string
}

// RLSOption configures RLSPolicies.
type RLSOption func(*rlsOptions)

// HybridResource is required for KindH: the resource_type of infra.tenant_resource_assignments and the
// column of the table holding the assigned resource id (ADR-0029).
func HybridResource(resourceType, idColumn string) RLSOption {
	return func(o *rlsOptions) { o.resourceType, o.idColumn = resourceType, idColumn }
}

// RLSPolicies returns the DDL that enables RLS on table ("schema.table") with the policies of
// DATABASE_DESIGN §4: tenant condition only — never deleted_at (ADR-0089 §1: PostgreSQL checks the new
// row of an UPDATE against SELECT policies, so deleted_at in a policy would block soft-delete). No DELETE
// policy is created (application roles have no DELETE). The SQL goes into a hand-written migration.
func RLSPolicies(table string, kind TableKind, opts ...RLSOption) (string, error) {
	if !tableRe.MatchString(table) {
		return "", fmt.Errorf("tenancy: invalid table %q (want schema.table, lower case)", table)
	}
	o := rlsOptions{}
	for _, f := range opts {
		f(&o)
	}
	tenant := "tenant_id = infra.current_tenant_id()"
	var b strings.Builder
	fmt.Fprintf(&b, "ALTER TABLE %s ENABLE ROW LEVEL SECURITY;\nALTER TABLE %s FORCE ROW LEVEL SECURITY;\n", table, table)

	switch kind {
	case KindT, KindTStar:
		fmt.Fprintf(&b, "CREATE POLICY tenant_read ON %s FOR SELECT USING (%s);\n", table, tenant)
		fmt.Fprintf(&b, "CREATE POLICY tenant_insert ON %s FOR INSERT WITH CHECK (%s);\n", table, tenant)
		fmt.Fprintf(&b, "CREATE POLICY tenant_update ON %s FOR UPDATE USING (%s) WITH CHECK (%s);\n", table, tenant, tenant)
	case KindTA, KindTAStar:
		fmt.Fprintf(&b, "CREATE POLICY tenant_read ON %s FOR SELECT USING (%s);\n", table, tenant)
		fmt.Fprintf(&b, "CREATE POLICY tenant_insert ON %s FOR INSERT WITH CHECK (%s);\n", table, tenant)
	case KindH:
		if !identRe.MatchString(o.resourceType) || !identRe.MatchString(o.idColumn) {
			return "", fmt.Errorf("tenancy: KindH needs HybridResource(resource_type, id_column) with valid identifiers")
		}
		short := table[strings.IndexByte(table, '.')+1:]
		fmt.Fprintf(&b, "CREATE POLICY content_read ON %s FOR SELECT USING (\n  %s\n  OR (tenant_id IS NULL AND EXISTS (\n    SELECT 1 FROM infra.tenant_resource_assignments a\n    WHERE a.resource_type = '%s' AND a.resource_id = %s.%s\n      AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))\n);\n",
			table, tenant, o.resourceType, short, o.idColumn)
		fmt.Fprintf(&b, "CREATE POLICY tenant_insert ON %s FOR INSERT WITH CHECK (%s);\n", table, tenant)
		fmt.Fprintf(&b, "CREATE POLICY tenant_update ON %s FOR UPDATE USING (%s) WITH CHECK (%s);\n", table, tenant, tenant)
	default:
		return "", fmt.Errorf("tenancy: no RLS for table kind %q (G/GA and infra.* tables have no RLS, ADR-0089 §4)", kind)
	}
	return b.String(), nil
}

// LiveView returns the DDL of view <schema>.<table>_live (rows with deleted_at IS NULL) readable by
// <module>_readonly, which has no privilege on the base table (ADR-0089 §1, F002-AC-29). The view runs
// with its owner's privileges (not security_invoker) and security_barrier; RLS of the base table still
// filters by app.tenant_id because FORCE ROW LEVEL SECURITY applies to the owner.
func LiveView(table, module string) (string, error) {
	if !tableRe.MatchString(table) {
		return "", fmt.Errorf("tenancy: invalid table %q", table)
	}
	if !moduleRe.MatchString(module) {
		return "", fmt.Errorf("tenancy: invalid module %q", module)
	}
	schema := table[:strings.IndexByte(table, '.')]
	view := table + "_live"
	role := module + "_readonly"
	return fmt.Sprintf("CREATE OR REPLACE VIEW %s WITH (security_barrier = true) AS\n  SELECT * FROM %s WHERE deleted_at IS NULL;\n"+
		"GRANT USAGE ON SCHEMA %s TO %s;\nGRANT SELECT ON %s TO %s;\n", view, table, schema, role, view, role), nil
}
