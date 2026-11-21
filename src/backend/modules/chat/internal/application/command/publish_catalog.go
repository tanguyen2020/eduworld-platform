package command

import (
	"context"
	"log/slog"
	"math"
	"strings"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
)

// round4 keeps the precision of numeric(5,4).
func round4(v float64) float64 { return math.Round(v*10000) / 10000 }

// PublishCatalog implements publishSensitiveCatalog (System Admin, platform catalogue).
func (h CatalogAdmin) PublishCatalog(ctx context.Context, summary string) (out catalog.StoredVersion, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.PublishSensitiveCatalog")
	defer func() { done(err) }()
	p, sc, err := platformWrite(ctx, authz.PermChatSensitiveTermsWrite)
	if err != nil {
		return out, err
	}
	return h.publish(ctx, p, sc, nil, summary)
}

// PublishTenantCatalog implements publishTenantSensitiveCatalog (Safety Team, extra terms of the target tenant).
func (h CatalogAdmin) PublishTenantCatalog(ctx context.Context, summary string) (out catalog.StoredVersion, err error) {
	ctx, done := observability.UseCase(ctx, "chat.command.PublishTenantSensitiveCatalog")
	defer func() { done(err) }()
	p, tenant, sc, err := tenantWrite(ctx, authz.PermChatTenantTermsWrite)
	if err != nil {
		return out, err
	}
	return h.publish(ctx, p, sc, &tenant, summary)
}

// publish builds the candidate snapshot of the scope from the live rows, runs the regression suite against it
// (F116-AC-14: failing filter_quality_gate → 422 SENSITIVE_CATALOG_REGRESSION_FAILED, no version), then writes the
// immutable version (next number of the scope, sha256 checksum, regression numbers), marks the included terms
// published, writes sensitive_catalog.published and the audit in the same transaction; after the commit every instance
// is notified (Valkey Pub/Sub) and reconciles anyway within moderation_config_propagation_max (F116-AC-15).
func (h CatalogAdmin) publish(ctx context.Context, p authz.Principal, sc tenancy.Scope, tenant *uuid.UUID, summary string) (catalog.StoredVersion, error) {
	var out catalog.StoredVersion
	summary = strings.TrimSpace(summary)
	if len([]rune(summary)) < 5 {
		return out, problem.Invalid("change_summary", problem.SubInvalid, "at least 5 characters")
	}
	if h.Regression == nil {
		return out, problem.New(problem.Unavailable, "regression suite not configured")
	}
	err := h.UoW.InTenantTx(ctx, sc, func(ctx context.Context) error {
		if err := h.Repo.LockScope(ctx, tenant); err != nil {
			return err
		}
		v, err := h.Repo.NextVersion(ctx, tenant)
		if err != nil {
			return err
		}
		cand, termIDs, err := h.Repo.LiveSnapshot(ctx, tenant)
		if err != nil {
			return err
		}
		cand.Version = v
		in := port.RegressionInput{Platform: cand}
		if tenant != nil {
			platform, err := h.Repo.PublishedSnapshot(ctx)
			if err != nil {
				return err
			}
			settings, err := h.Repo.TenantSettings(ctx, *tenant)
			if err != nil {
				return err
			}
			in = port.RegressionInput{Platform: platform, Tenant: &cand, Settings: settings}
		}
		res, err := h.Regression.Run(ctx, in)
		if err != nil {
			return err
		}
		recall, fp := round4(res.Recall), round4(res.FPRate)
		if !moderation.PassesQualityGate(res.Recall, res.FPRate) {
			return problem.New(problem.SensitiveCatalogRegressionFailed, "").With("regression",
				map[string]any{"recall": recall, "false_positive_rate": fp, "min_recall": moderation.RegressionMinRecall,
					"max_false_positive_rate": moderation.RegressionMaxFPRate})
		}
		raw, sum, err := catalog.Encode(cand)
		if err != nil {
			return err
		}
		scope := catalog.ScopePlatform
		if tenant != nil {
			scope = catalog.ScopeTenant
		}
		by := p.UserID
		out = catalog.StoredVersion{ID: ids.New(), TenantID: tenant, Scope: scope, Version: v, PublishedAt: h.now(), PublishedBy: &by,
			ChangeSummary: summary, TermCount: len(cand.Terms), Checksum: sum, Snapshot: raw, Recall: &recall, FPRate: &fp}
		if err := h.Repo.InsertVersion(ctx, out); err != nil {
			return err
		}
		if err := h.Repo.MarkPublished(ctx, tenant, termIDs, v); err != nil {
			return err
		}
		payload := map[string]any{"tenant_id": tenant, "scope": scope, "catalog_version": v, "published_at": out.PublishedAt,
			"published_by": by, "term_count": out.TermCount, "checksum": sum, "regression_recall": recall, "regression_fp_rate": fp}
		e := eventOf(EventCatalogPublished, "sensitive_catalog", scopeKey(tenant), tenant, out.PublishedAt, payload)
		if err := h.Events.Publish(ctx, e); err != nil {
			return err
		}
		if err := h.record(ctx, AuditCatalogPublished, "sensitive_catalog", out.ID, "", map[string]any{"scope": scope,
			"catalog_version": v, "term_count": out.TermCount, "checksum": sum, "regression_recall": recall,
			"regression_fp_rate": fp}); err != nil {
			return err
		}
		if h.AfterCommit == nil {
			return nil
		}
		return h.AfterCommit(ctx, func(ctx context.Context) {
			if h.OnPublished != nil {
				h.OnPublished(scope, v)
			}
			if h.Notifier != nil {
				if err := h.Notifier.CatalogPublished(context.WithoutCancel(ctx), tenant, v); err != nil {
					slog.WarnContext(ctx, "chat: catalogue version not announced (instances reconcile)", "error", err)
				}
			}
		})
	})
	return out, err
}

func scopeKey(tenant *uuid.UUID) string {
	if tenant == nil {
		return "platform"
	}
	return tenant.String()
}
