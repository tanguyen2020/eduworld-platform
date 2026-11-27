package query

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/domain/installer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// InstallerChannel is the only channel the school page lists (F134-AC-13).
const InstallerChannel = "production"

// SchoolInstallerView is SchoolClientInstallerList (public data of the kits only, F134-AC-16).
type SchoolInstallerView struct {
	Data             []installer.Kit `json:"data"`
	Channel          string          `json:"channel"`
	IndexGeneratedAt time.Time       `json:"index_generated_at"`
	WebClientURL     *string         `json:"web_client_url"`
}

// SchoolInstallers is listSchoolClientInstallers (F134 US3): school.installer.read on the school, the verified
// production index filtered by the grade bands of the school's active students.
type SchoolInstallers struct {
	Read    read.Session
	Reads   port.InstallerReads
	Index   port.SchoolKitIndex // nil = release CDN not configured → 503
	Metrics port.InstallerMetrics
	// WebClientURL is the link of the Web client for school PCs (ADR-0101 §8, EW_GAME_WEB_URL); "" = not configured.
	WebClientURL string
}

// List answers the kits of the school. A school outside the tenant, or outside the grants of a school-scoped caller,
// is 404 (no existence leak, F134-AC-15); a caller without school.installer.read anywhere (Academic Manager, student)
// is 403 PERMISSION_DENIED; an index that cannot be loaded or verified is 503 UNAVAILABLE with no link (AC-14).
func (q SchoolInstallers) List(ctx context.Context, school uuid.UUID) (out SchoolInstallerView, err error) {
	ctx, done := observability.UseCase(ctx, "game.query.ListSchoolClientInstallers")
	defer func() { done(err) }()
	p, tenant, err := staffTenant(ctx)
	if err != nil { // a student never reaches the portal routes (403 PORTAL_ACCESS_STAFF_ONLY)
		return out, err
	}
	held := false
	for _, g := range p.Grants {
		if slices.Contains(g.Permissions, authz.PermSchoolInstallerRead) {
			held = true
			break
		}
	}
	if !held {
		return out, authz.Require(ctx, p, authz.PermSchoolInstallerRead, authz.Resource{TenantID: &tenant}) // records authz.denied
	}
	var levels []int
	err = q.Read.Read(ctx, func(ctx context.Context) error {
		ok, err := q.Reads.SchoolExists(ctx, tenant, school)
		if err != nil {
			return err
		}
		if !ok {
			return problem.New(problem.NotFound, "")
		}
		if err := authz.Require(ctx, p, authz.PermSchoolInstallerRead, authz.Resource{TenantID: &tenant, SchoolID: &school}); err != nil {
			return problem.New(problem.NotFound, "") // another school of the tenant: same answer as another tenant's school
		}
		levels, err = q.Reads.SchoolGradeLevels(ctx, tenant, school)
		return err
	})
	if err != nil {
		return out, err
	}
	if q.Index == nil {
		q.metric("unconfigured")
		return out, problem.New(problem.Unavailable, "release index not configured").Retry()
	}
	ix, err := q.Index.Load(ctx, InstallerChannel)
	if err != nil {
		q.metric("invalid")
		slog.WarnContext(ctx, "school-kit index unavailable", "channel", InstallerChannel, "error", err)
		return out, problem.New(problem.Unavailable, "school-kit index unavailable or not verified").Retry().Wrap(err)
	}
	q.metric("ok")
	out = SchoolInstallerView{Data: installer.ForSchool(ix.Kits, installer.BandsOf(levels)), Channel: ix.Channel,
		IndexGeneratedAt: ix.GeneratedAt.UTC()}
	if q.WebClientURL != "" {
		u := q.WebClientURL
		out.WebClientURL = &u
	}
	return out, nil
}

func (q SchoolInstallers) metric(result string) {
	if q.Metrics != nil {
		q.Metrics.IndexFetch(result)
	}
}
