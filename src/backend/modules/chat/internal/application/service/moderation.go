package service

import (
	"context"
	"crypto/sha1" //nolint:gosec // UUIDv5 namespace hash (identifier, not security)
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/moderation/catalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
)

// ErrCatalogNotLoaded: the platform catalogue could not be loaded and no compiled copy exists: the instance is not
// ready and messages fail closed (F116 edge case: never run with an empty catalogue).
var ErrCatalogNotLoaded = errors.New("chat: sensitive catalogue not loaded")

// CatalogView is the catalogue in force for a tenant: the compiled platform and tenant parts, the category lookup
// (platform definition, tenant on/off setting) and the versions used (decision log).
type CatalogView struct {
	Parts           []*catalog.Compiled
	Categories      catalog.Categories
	PlatformVersion int64
	TenantVersion   int64
}

type platformPart struct {
	version  int64
	compiled *catalog.Compiled
	cats     map[uuid.UUID]catalog.Category
}

type tenantPart struct {
	platform  *platformPart
	version   int64
	compiled  *catalog.Compiled
	settings  map[uuid.UUID]bool
	checkedAt time.Time
}

type categoryLookup struct {
	cats     map[uuid.UUID]catalog.Category
	settings map[uuid.UUID]bool
}

// Lookup implements catalog.Categories: a category is in force when enabled on the platform, unless the tenant
// setting (allowed only when allow_tenant_toggle) says otherwise.
func (c categoryLookup) Lookup(id uuid.UUID) (catalog.Category, bool) {
	cat, ok := c.cats[id]
	if !ok {
		return catalog.Category{}, false
	}
	on := cat.Enabled
	if v, set := c.settings[id]; set && cat.AllowTenantToggle {
		on = v
	}
	return cat, on
}

// Catalogs is the compiled catalogue store of one instance (MODERATION §11.6, F116-AC-15): the platform part and each
// tenant part are compiled in the background and swapped atomically; a message in flight keeps the view it got.
// Reconcile re-reads the versions every Refresh (≤ moderation_config_propagation_max); Invalidate (Valkey pub/sub
// ew:chat:sensitive:version) reloads at once.
type Catalogs struct {
	Read    ReadRunner
	Reader  read.ModerationReader
	Engine  *moderation.Engine
	Refresh time.Duration
	Now     func() time.Time
	// OnInvalid is called with the number of invalid patterns skipped at compilation (metric, no content).
	OnInvalid func(n int)

	platform atomic.Pointer[platformPart]
	tenants  sync.Map // uuid.UUID → *atomic.Pointer[tenantPart]
	loadMu   sync.Mutex
}

func (c *Catalogs) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *Catalogs) refresh() time.Duration {
	if c.Refresh > 0 {
		return c.Refresh
	}
	return 30 * time.Second
}

// Ready reports a loaded platform catalogue (readiness of the instance).
func (c *Catalogs) Ready() bool { return c.platform.Load() != nil }

func (c *Catalogs) compile(s catalog.Snapshot) *catalog.Compiled {
	comp, invalid := catalog.Compile(c.Engine.Normalizer(), s)
	if len(invalid) > 0 {
		slog.Warn("chat: invalid sensitive patterns skipped", "count", len(invalid), "scope", s.Scope, "version", s.Version)
		if c.OnInvalid != nil {
			c.OnInvalid(len(invalid))
		}
	}
	return comp
}

// loadPlatform reads the newest platform version (in the read session of ctx) and compiles it when it changed.
func (c *Catalogs) loadPlatform(ctx context.Context) (*platformPart, error) {
	cur := c.platform.Load()
	var after int64
	if cur != nil {
		after = cur.version
	}
	row, err := c.Reader.LatestCatalog(ctx, nil, after)
	if err != nil {
		return cur, err
	}
	if row == nil {
		if cur == nil {
			return nil, ErrCatalogNotLoaded
		}
		return cur, nil
	}
	snap, err := catalog.ParseSnapshot(row.Snapshot)
	if err != nil {
		return cur, err
	}
	p := &platformPart{version: row.Version, compiled: c.compile(snap), cats: map[uuid.UUID]catalog.Category{}}
	p.compiled.Checksum = row.Checksum
	for _, cat := range snap.Categories {
		p.cats[cat.ID] = cat
	}
	c.platform.Store(p)
	return p, nil
}

func (c *Catalogs) slot(tenant uuid.UUID) *atomic.Pointer[tenantPart] {
	v, _ := c.tenants.LoadOrStore(tenant, &atomic.Pointer[tenantPart]{})
	return v.(*atomic.Pointer[tenantPart])
}

// load re-reads the versions of tenant and recompiles what changed.
func (c *Catalogs) load(ctx context.Context, tenant uuid.UUID) (*tenantPart, error) {
	c.loadMu.Lock()
	defer c.loadMu.Unlock()
	s, err := authz.EventScope(tenant)
	if err != nil {
		return nil, err
	}
	slot := c.slot(tenant)
	cur := slot.Load()
	var next *tenantPart
	err = c.Read.InTenantRead(ctx, s, func(ctx context.Context) error {
		p, err := c.loadPlatform(ctx)
		if err != nil {
			return err
		}
		next = &tenantPart{platform: p, checkedAt: c.now()}
		var after int64
		if cur != nil {
			next.version, next.compiled, after = cur.version, cur.compiled, cur.version
		}
		row, err := c.Reader.LatestCatalog(ctx, &tenant, after)
		if err != nil {
			return err
		}
		if row != nil {
			snap, err := catalog.ParseSnapshot(row.Snapshot)
			if err != nil {
				return err
			}
			next.version, next.compiled = row.Version, c.compile(snap)
			next.compiled.Checksum = row.Checksum
		}
		next.settings, err = c.Reader.TenantCategorySettings(ctx, tenant)
		return err
	})
	if err != nil {
		if cur != nil { // keep serving the compiled copy
			return cur, nil
		}
		return nil, fmt.Errorf("%w: %w", port.ErrUnavailable, err)
	}
	slot.Store(next)
	return next, nil
}

// Get returns the catalogue in force for tenant: the compiled copy (refreshed in the background when older than
// Refresh), loaded synchronously the first time. Without any copy it fails closed (ErrUnavailable).
func (c *Catalogs) Get(ctx context.Context, tenant uuid.UUID) (CatalogView, error) {
	t := c.slot(tenant).Load()
	if t == nil {
		var err error
		if t, err = c.load(ctx, tenant); err != nil {
			return CatalogView{}, err
		}
	} else if c.now().Sub(t.checkedAt) > c.refresh() {
		go func() {
			if _, err := c.load(context.WithoutCancel(ctx), tenant); err != nil {
				slog.Warn("chat: catalogue refresh failed", "error", err)
			}
		}()
	}
	v := CatalogView{Categories: categoryLookup{cats: t.platform.cats, settings: t.settings}, PlatformVersion: t.platform.version,
		TenantVersion: t.version, Parts: []*catalog.Compiled{t.platform.compiled}}
	if t.compiled != nil {
		v.Parts = append(v.Parts, t.compiled)
	}
	return v, nil
}

// Invalidate reloads the platform (tenant nil: every known tenant) or one tenant now (pub/sub notification).
func (c *Catalogs) Invalidate(ctx context.Context, tenant *uuid.UUID) {
	if tenant != nil {
		if _, err := c.load(ctx, *tenant); err != nil {
			slog.Warn("chat: catalogue reload failed", "error", err)
		}
		return
	}
	c.Reconcile(ctx)
}

// Reconcile reloads every known tenant (periodic check, MODERATION §11.6: a lost notification is caught up within
// Refresh).
func (c *Catalogs) Reconcile(ctx context.Context) {
	c.tenants.Range(func(k, _ any) bool {
		if _, err := c.load(ctx, k.(uuid.UUID)); err != nil {
			slog.Warn("chat: catalogue reconcile failed", "error", err)
		}
		return ctx.Err() == nil
	})
}

// Warm loads the platform catalogue at start (readiness waits for it) in the scope of a tenant-free read: the
// platform rows are tenant_id NULL, read through a system platform scope.
func (c *Catalogs) Warm(ctx context.Context) error {
	s, err := authz.SystemPlatformScope("chat-catalog-loader")
	if err != nil {
		return err
	}
	return c.Read.InTenantRead(ctx, s, func(ctx context.Context) error {
		_, err := c.loadPlatform(ctx)
		return err
	})
}

// Run warms the store then reconciles every Refresh until ctx ends.
func (c *Catalogs) Run(ctx context.Context) error {
	for {
		if err := c.Warm(ctx); err == nil {
			break
		} else {
			slog.Warn("chat: platform catalogue not loaded yet", "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(2 * time.Second):
		}
	}
	t := time.NewTicker(c.refresh())
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
			c.Reconcile(ctx)
		}
	}
}

// Switches resolves the effective moderation switches of a tenant through the cache (ew:chat:facts:switches:<tenant>,
// fresh for Fresh ≤ moderation_config_propagation_max). When neither the cache nor PostgreSQL answers, every step is
// ON: the switches never default to off (F116-AC-22).
type Switches struct {
	Read   ReadRunner
	Reader read.ModerationReader
	Cache  port.Cache
	Fresh  time.Duration
	Now    func() time.Time
}

// SwitchKey is the cache key of the switches of tenant.
func SwitchKey(tenant uuid.UUID) string { return "switches:" + tenant.String() }

type switchDoc struct {
	Platform *read.SwitchRow `json:"platform,omitempty"`
	Tenant   *read.SwitchRow `json:"tenant,omitempty"`
}

var snapshotNS = uuid.MustParse("01936a00-0000-7000-8000-00000000f116")

// ToSwitch converts a stored row (legacy keys such as pii are dropped, ADR-0078).
func ToSwitch(r *read.SwitchRow) moderation.Switch {
	if r == nil {
		return moderation.AllOn()
	}
	steps := map[string]bool{}
	for _, s := range moderation.SwitchableSteps {
		if v, ok := r.Steps[s]; ok {
			steps[s] = v
		}
	}
	return moderation.Switch{EnabledAll: r.EnabledAll, Steps: steps, ExpiresAt: r.ExpiresAt}
}

// Effective returns the switches in force for tenant and the id of the snapshot used (decision log).
func (s Switches) Effective(ctx context.Context, tenant uuid.UUID) (moderation.Effective, uuid.UUID) {
	now := time.Now()
	if s.Now != nil {
		now = s.Now()
	}
	fresh := s.Fresh
	if fresh <= 0 {
		fresh = 30 * time.Second
	}
	raw, err := s.Cache.Get(ctx, SwitchKey(tenant), fresh, func(ctx context.Context) ([]byte, error) {
		var d switchDoc
		sc, err := authz.EventScope(tenant)
		if err != nil {
			return nil, err
		}
		err = s.Read.InTenantRead(ctx, sc, func(ctx context.Context) error {
			d.Platform, d.Tenant, err = s.Reader.Switches(ctx, tenant)
			return err
		})
		if err != nil {
			return nil, err
		}
		return json.Marshal(d)
	})
	var d switchDoc
	if err == nil {
		err = json.Unmarshal(raw, &d)
	}
	if err != nil {
		slog.WarnContext(ctx, "chat: moderation switches unavailable, every step on", "error", err)
		d = switchDoc{}
	}
	e := moderation.Effective{Platform: ToSwitch(d.Platform), At: now}
	if d.Tenant != nil {
		t := ToSwitch(d.Tenant)
		e.Tenant = &t
	}
	h := sha1.New() //nolint:gosec // identifier only
	for _, r := range []*read.SwitchRow{d.Platform, d.Tenant} {
		if r != nil {
			_, _ = fmt.Fprintf(h, "%s:%d;", r.ID, r.Version)
		} else {
			_, _ = h.Write([]byte("-;"))
		}
	}
	id := uuid.NewSHA1(snapshotNS, h.Sum(nil))
	return e, id
}

// Sanctions checks the active mutes of a sender (technical gate, F116-AC-08): Valkey ew:chat:sanction:<user_id>
// first, PostgreSQL on a miss (the result is cached until the first end, at most CacheTTL). Without both, the check
// fails closed (ErrUnavailable).
type Sanctions struct {
	Read     ReadRunner
	Reader   read.ModerationReader
	Cache    port.SanctionCache
	CacheTTL time.Duration
}

// Active returns the active sanctions of user in tenant at now.
func (s Sanctions) Active(ctx context.Context, tenant, user uuid.UUID, now time.Time) ([]port.Sanction, error) {
	if list, ok, err := s.Cache.Get(ctx, user); err == nil && ok {
		return list, nil
	}
	sc, err := authz.EventScope(tenant)
	if err != nil {
		return nil, err
	}
	var rows []read.SanctionRow
	err = s.Read.InTenantRead(ctx, sc, func(ctx context.Context) error {
		rows, err = s.Reader.ActiveSanctions(ctx, tenant, user, now)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %w", port.ErrUnavailable, err)
	}
	ttl := s.CacheTTL
	if ttl <= 0 {
		ttl = time.Hour
	}
	list := make([]port.Sanction, 0, len(rows))
	for _, r := range rows {
		list = append(list, port.Sanction{ID: r.ID, Type: r.Type, Scope: r.Scope, ChannelID: r.ChannelID, EndsAt: r.EndsAt})
		if r.EndsAt != nil && r.EndsAt.Sub(now) < ttl {
			ttl = max(r.EndsAt.Sub(now), time.Second)
		}
	}
	if err := s.Cache.Set(ctx, user, list, ttl); err != nil {
		slog.WarnContext(ctx, "chat: sanction cache not written", "error", err)
	}
	return list, nil
}

// QuickMessages serves the approved quick messages from memory, reloaded every Refresh.
type QuickMessages struct {
	Read    ReadRunner
	Reader  read.ModerationReader
	Refresh time.Duration

	mu     sync.Mutex
	list   []read.QuickMessageRow
	loaded time.Time
}

// List returns the active quick messages (fails closed without any copy).
func (q *QuickMessages) List(ctx context.Context) ([]read.QuickMessageRow, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	refresh := q.Refresh
	if refresh <= 0 {
		refresh = time.Minute
	}
	if q.list != nil && time.Since(q.loaded) < refresh {
		return q.list, nil
	}
	s, err := authz.SystemPlatformScope("chat-quick-messages")
	if err != nil {
		return nil, err
	}
	var rows []read.QuickMessageRow
	err = q.Read.InTenantRead(ctx, s, func(ctx context.Context) error {
		rows, err = q.Reader.QuickMessages(ctx)
		return err
	})
	if err != nil {
		if q.list != nil {
			return q.list, nil
		}
		return nil, fmt.Errorf("%w: %w", port.ErrUnavailable, err)
	}
	if rows == nil {
		rows = []read.QuickMessageRow{}
	}
	q.list, q.loaded = rows, time.Now()
	return rows, nil
}

// Get returns one active quick message.
func (q *QuickMessages) Get(ctx context.Context, id string) (*read.QuickMessageRow, error) {
	list, err := q.List(ctx)
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].ID == id {
			return &list[i], nil
		}
	}
	return nil, nil
}
