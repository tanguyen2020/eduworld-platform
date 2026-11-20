package identity

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go/jetstream"

	gameapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/api"
	identityapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/loginsync"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
	portwrite "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/saga"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/auditmaint"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/featuretoggle"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/gameauth"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/keycloak"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/lifecyclemetrics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/mail"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/messaging"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/modules"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/participants"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/portalcatalog"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/portalmetrics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/scanner"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/settingscfg"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/contract"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/contractv1client"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/csvreader"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/lookup"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/metrics"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/sis/webhook"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/infrastructure/storage"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/consumer"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/facade"
	identityhttp "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/http"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/job"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/presentation/sisinternal"
	learningapi "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/api"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/httpx/featuregate"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/module"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/natsx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox"

	_ "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/ent/runtime" // ent mixin hooks and interceptors (R27)
)

// Intervals of the identity jobs that have no constant in presentation/job [Proposed] (OQ F010-2).
const (
	OrgReconcileInterval   = 24 * time.Hour  // org-membership-reconcile (F010: nightly)
	UserRoleExpiryInterval = 5 * time.Minute // user-role-expiry (F012 T032)
	RetentionInterval      = 24 * time.Hour  // retention-anonymizer (F016 US5)
)

// Facade is the identity facade (identityapi.StrictServerInterface) implemented in this release: projection
// snapshots (F010) and Principal resolution (F012). getTenant is not implemented yet and no module calls it.
type Facade interface {
	ResolvePrincipal(ctx context.Context, req identityapi.ResolvePrincipalRequestObject) (identityapi.ResolvePrincipalResponseObject, error)
	GetPrincipal(ctx context.Context, req identityapi.GetPrincipalRequestObject) (identityapi.GetPrincipalResponseObject, error)
	CheckMemberships(ctx context.Context, req identityapi.CheckMembershipsRequestObject) (identityapi.CheckMembershipsResponseObject, error)
	ListProjectionSnapshot(ctx context.Context, req identityapi.ListProjectionSnapshotRequestObject) (identityapi.ListProjectionSnapshotResponseObject, error)
}

type identityFacade struct {
	facade.Facade
	facade.Principals
}

var _ Facade = identityFacade{}

// LearningFacade is what module identity calls on module learning (x-allowed-callers: identity).
type LearningFacade interface {
	modules.LearningPlatformOps
	participants.LearningFacade
	AnonymizeUserData(ctx context.Context, req learningapi.AnonymizeUserDataRequestObject) (learningapi.AnonymizeUserDataResponseObject, error)
}

// GameFacade is what module identity calls on module game (x-allowed-callers: identity).
type GameFacade interface {
	participants.GameFacade
	AnonymizeUserData(ctx context.Context, req gameapi.AnonymizeUserDataRequestObject) (gameapi.AnonymizeUserDataResponseObject, error)
}

// Imports are the facades of the other modules called by the sagas and platform operations of identity.
// Competition and chat implement no facade operation yet (OQ F045-3): their steps answer 503 UNAVAILABLE.
type Imports struct {
	Learning LearningFacade
	Game     GameFacade
}

// build constructs every use case of module identity on the real adapters and declares its entry points.
func build(r *module.Registry, m *Module, in Imports) error {
	if in.Learning == nil || in.Game == nil {
		return fmt.Errorf("identity: the learning and game facades are required (Wire)")
	}
	core, err := r.Core(Name)
	if err != nil {
		return err
	}
	cfg := r.Config()
	cursors, err := r.Cursors()
	if err != nil {
		return err
	}
	vk, err := r.Valkey()
	if err != nil {
		return err
	}
	js, err := r.JetStream()
	if err != nil {
		return err
	}
	if err := cfg.Require("EW_OIDC_ISSUERS", "EW_PORTAL_MENU_CATALOG"); err != nil {
		return err
	}
	issuers := cfg.List("EW_OIDC_ISSUERS")
	localIssuer := issuers[0]
	reg := r.Metrics()

	kcCfg, err := keycloak.ConfigFrom(cfg)
	if err != nil {
		return err
	}
	kc, err := keycloak.New(kcCfg, nil)
	if err != nil {
		return err
	}
	_ = reg.Register(keycloak.OrgSyncCounter())
	_ = reg.Register(keycloak.TokenCallDuration())
	defaults, err := settingscfg.DefaultsFrom(cfg)
	if err != nil {
		return err
	}
	workspaces, err := portalcatalog.Load(cfg.String("EW_PORTAL_MENU_CATALOG"))
	if err != nil {
		return err
	}
	for _, c := range metrics.Collectors() {
		_ = reg.Register(c)
	}

	events, auditor := write.Publisher{W: core.Writer}, write.Auditor{R: core.Recorder}
	deps := command.TenantDeps{UoW: core.UoW, Tenants: write.TenantRepo{}, Events: events, Audit: auditor}
	sd := command.SchoolDeps{UoW: core.UoW, Tenants: write.TenantRepo{}, Schools: write.SchoolRepo{}, Catalog: write.CatalogRepo{},
		Events: events, Audit: auditor}
	principals := read.PrincipalModel{Session: core.ReadSession}
	toggles := featuretoggle.Source{Reader: read.FeatureFlagModel{Session: core.ReadSession}, C: vk}
	m.toggles, m.hasToggles = toggles, true

	// Facade of identity (F010 snapshots, F012 principals).
	m.facade = identityFacade{Facade: facade.Facade{Snapshot: query.ProjectionSnapshot{Model: read.SnapshotModel{Session: core.ReadSession}, Cursors: cursors}},
		Principals: facade.Principals{Resolve: query.ResolvePrincipal{Model: principals},
			Memberships: query.CheckMemberships{Model: principals}}}

	// Facades of learning and game (sagas F016, platform operations F045); mergeUserData is not implemented by
	// any module yet (OQ F045-3): LearningMerge stays nil and the merge step answers 503 UNAVAILABLE.
	gw := modules.Gateway{Learning: in.Learning}
	studentParts := []port.StudentDataParticipant{participants.Game{F: in.Game}, participants.Learning{F: in.Learning}}
	anonymizers := []port.UserDataAnonymizer{participants.GameAnonymize{F: in.Game}, participants.LearningAnonymize{F: in.Learning}}

	lm, err := lifecyclemetrics.New(reg)
	if err != nil {
		return err
	}
	engine := saga.Engine{UoW: core.UoW, Repo: write.SagaRepo{}, Metrics: lm}
	restore := command.StudentRestore{UoW: core.UoW, Repo: write.CleanupRepo{}, Saga: engine, Participants: studentParts,
		Lifecycle: write.LifecycleRepo{}, Events: events, Audit: auditor}
	cleanup := command.StudentCleanup{UoW: core.UoW, Repo: write.CleanupRepo{}, Settings: write.SettingsRepo{}, Defaults: defaults, Saga: engine,
		Participants: studentParts, Lifecycle: write.LifecycleRepo{}, Events: events, Audit: auditor, Metrics: lm, Cursors: cursors}
	anonymize := command.UserAnonymize{UoW: core.UoW, Repo: write.AnonymizeRepo{}, Saga: engine, Participants: anonymizers,
		Lifecycle: write.LifecycleRepo{}, Events: events, Audit: auditor}
	pad := command.PlatformAdminDeps{UoW: core.UoW, Repo: write.PlatformAdminRepo{}, Owners: gw, Events: events, Audit: auditor, Settings: write.SettingsRepo{},
		Sagas: engine, Students: restore}

	// SIS (F011 CSV import, F017 contract_v1).
	validator, err := contract.New()
	if err != nil {
		return err
	}
	sisCfg, err := contractv1client.ConfigFrom(cfg)
	if err != nil {
		return err
	}
	providers := &contractv1client.Factory{Secrets: &contractv1client.FileSecrets{Dir: sisCfg.SecretsDir}, Validator: validator,
		Transport: sisCfg.Transport()}
	queue := messaging.JobQueue{JS: js}
	if err := cfg.Require("EW_S3_ENDPOINT", "EW_S3_ACCESS_KEY", "EW_S3_SECRET_KEY"); err != nil {
		return err
	}
	objects, err := storage.New(storage.Config{Endpoint: cfg.String("EW_S3_ENDPOINT"), Region: cfg.String("EW_S3_REGION"),
		AccessKey: cfg.String("EW_S3_ACCESS_KEY"), SecretKey: cfg.String("EW_S3_SECRET_KEY"), Bucket: cfg.StringOr("EW_SIS_IMPORT_BUCKET", "ew-sis-imports"),
		Timeout: cfg.Duration("EW_S3_TIMEOUT", 0)}, nil)
	if err != nil {
		return err
	}
	run := command.SisRun{UoW: core.UoW, Savepoints: core.UoW, Connections: write.SisConnectionRepo{}, Sync: write.SisSyncRepo{},
		Enrollments: write.EnrollmentRepo{}, Missing: write.SisSyncRepo{}, Transfers: write.SisSyncRepo{}, Students: write.StudentRepo{},
		Rules: write.SisSyncRepo{}, Events: events, Audit: auditor, Observer: metrics.Runs{}, History: write.LifecycleRepo{}}
	if k := cfg.String("EW_SIS_ID_HASH_KEY"); k != "" {
		run.IDHashKey = []byte(k)
	}
	sisContract := command.SisContract{UoW: core.UoW, Connections: write.SisConnectionRepo{}, Repo: write.SisContractRepo{},
		Schools: write.SchoolRepo{}, Providers: providers, Audit: auditor, AllowHTTP: sisCfg.AllowHTTPBaseURL}
	sisSync := command.SisSync{UoW: core.UoW, Connections: write.SisConnectionRepo{}, Repo: write.SisContractRepo{}, Run: run,
		Providers: providers, Lock: cache.SisRunLock{C: vk}, Jobs: queue, Metrics: metrics.Sync{}, Observer: metrics.Runs{}}
	sisLookup := &lookup.Adapter{Providers: providers, Load: func(ctx context.Context, id uuid.UUID) (row *portwrite.ContractConnectionRow, err error) {
		err = core.UoW.Do(ctx, func(ctx context.Context) error {
			row, err = write.SisContractRepo{}.GetContract(ctx, id)
			return err
		})
		return row, err
	}}
	ls := loginsync.LoginSync{UoW: core.UoW, State: write.SisSyncRepo{}, Lookup: sisLookup, Apply: run, Jobs: queue,
		Metrics: metrics.LoginSync{}, Guard: loginsync.NewGuard(20, 5, 30*time.Second)}

	// Game login (F013) and portal login (F012 US7/US8).
	gaCfg, err := gameauth.ConfigFrom(cfg)
	if err != nil {
		return err
	}
	gameTokCfg, err := keycloak.GameConfigFrom(cfg)
	if err != nil {
		return err
	}
	gameTokens, err := keycloak.NewPortalTokens(gameTokCfg, nil)
	if err != nil {
		return err
	}
	tickets, err := cache.NewLoginTickets(vk, gaCfg.TicketKey)
	if err != nil {
		return err
	}
	gm, err := gameauth.NewMetrics(reg)
	if err != nil {
		return err
	}
	policy := gameauth.SettingsPolicy{Reader: read.SettingsModel{Session: core.ReadSession}, PlayTime: read.PlayTimeModel{Session: core.ReadSession},
		Defaults: defaults}
	gameAuth := command.GameAuth{UoW: core.UoW, Accounts: write.GameLoginRepo{}, Memberships: read.LoginMembershipReader{Session: core.ReadSession},
		Sessions: write.GameAuthSessionRepo{}, Bootstrap: read.BootstrapReader{Session: core.ReadSession}, Events: events, Audit: auditor,
		IdP: gameTokens, KC: kc, Tickets: tickets, Limiter: cache.NewFallbackLimiter(cache.RateLimiter{C: vk}), Curfew: policy, Hours: policy, DailyLimit: policy,
		Sync: func(ctx context.Context, userID uuid.UUID) (*uuid.UUID, error) {
			res, err := ls.Ensure(ctx, userID)
			return res.SisConnectionID, err
		},
		Restore: restore, Metrics: gm, Cfg: gaCfg.UseCase}

	portalCfg, err := identityhttp.PortalAuthConfigFrom(cfg)
	if err != nil {
		return err
	}
	portalTokCfg, err := keycloak.PortalConfigFrom(cfg)
	if err != nil {
		return err
	}
	portalTokens, err := keycloak.NewPortalTokens(portalTokCfg, nil)
	if err != nil {
		return err
	}
	mailCfg, err := mail.ConfigFrom(cfg)
	if err != nil {
		return err
	}
	mailer, err := mail.New(mailCfg)
	if err != nil {
		return err
	}
	pm, err := portalmetrics.New(reg)
	if err != nil {
		return err
	}
	portalAuth := &command.PortalAuth{UoW: core.UoW, Accounts: write.PortalAccountRepo{}, Audit: auditor, IdP: portalTokens, KC: kc,
		Tickets: cache.SetupTickets{C: vk}, Sessions: cache.PortalSessions{C: vk}, Limiter: cache.RateLimiter{C: vk}, Metrics: pm,
		Resets: write.PortalResetTokenRepo{}, Mailer: mailer, Cfg: portalCfg}

	userRoles := command.UserRoles{UoW: core.UoW, Repo: write.UserRoleRepo{}, Events: events, Audit: auditor}
	h := identityhttp.Handlers{
		CreateTenant: command.CreateTenant{TenantDeps: deps}, UpdateTenant: command.UpdateTenant{TenantDeps: deps},
		ListTenants:       query.ListTenants{Model: read.TenantModel{Session: core.ReadSession}, Cursors: cursors},
		CreateAdminSchool: command.CreateAdminTenantSchool{SchoolDeps: sd}, UpdateAdminSchool: command.UpdateAdminTenantSchool{SchoolDeps: sd},
		UpdateTenantSchool: command.UpdateTenantSchool{SchoolDeps: sd}, ApproveTenantSchool: command.ApproveTenantSchool{SchoolDeps: sd},
		CreateCommonStudents: command.CreateCommonStudents{UoW: core.UoW, Tenants: write.TenantRepo{}, Accounts: write.AccountRepo{}, KC: kc,
			Events: events, Audit: auditor, LocalIssuer: localIssuer},
		Catalog:         command.Catalog{CatalogDeps: command.CatalogDeps{UoW: core.UoW, Catalog: write.CatalogWriter{}, Events: events, Audit: auditor}},
		Org:             query.Org{Model: read.OrgModel{Session: core.ReadSession}, Cursors: cursors},
		Sis:             query.Sis{Issues: read.SisIssueModel{Session: core.ReadSession}, Cursors: cursors},
		ResolveSisIssue: command.ResolveSisIssue{UoW: core.UoW, Issues: write.SisSyncRepo{}, Events: events, Audit: auditor},
		CreateSisImport: command.CreateSisImport{UoW: core.UoW, Imports: write.SisSyncRepo{}, Storage: objects, Audit: auditor},
		SisImports:      query.SisImports{Model: read.SisImportModel{Session: core.ReadSession}},
		SisAdmin:        query.SisAdmin{Model: read.SisAdminModel{Session: core.ReadSession}, Cursors: cursors},
		SisConnections:  command.SisConnections{UoW: core.UoW, Connections: write.SisConnectionRepo{}, Audit: auditor, Contract: &sisContract},
		Me:              query.Me{Model: principals, Workspaces: workspaces, Toggles: toggles},
		UserRoles:       userRoles, UserRoleList: query.UserRoles{Model: principals, Cursors: cursors},
		Staff: command.Staff{UoW: core.UoW, Repo: write.StaffRepo{}, Roles: userRoles, KC: kc, Events: events, Audit: auditor,
			LocalIssuer: localIssuer},
		UserList: query.TenantUsers{Model: principals, Cursors: cursors},
		Roles:    command.Roles{UoW: core.UoW, Repo: write.RoleRepo{}, Events: events, Audit: auditor},
		RoleList: query.Roles{Model: principals, Cursors: cursors},
	}
	settingsH := identityhttp.SettingsHandlers{AuditLogs: query.AuditLogs{Model: read.AuditModel{Session: core.ReadSession}, Cursors: cursors},
		Settings:    command.Settings{UoW: core.UoW, Repo: write.SettingsRepo{}, Events: events, Audit: auditor, Defaults: defaults},
		SettingsGet: query.Settings{Reader: read.SettingsModel{Session: core.ReadSession}, Defaults: defaults},
		Flags:       command.FeatureFlags{UoW: core.UoW, Repo: write.FeatureFlagRepo{}, Events: events, Audit: auditor, Cache: toggles},
		FlagList:    query.FeatureFlags{Reader: read.FeatureFlagModel{Session: core.ReadSession}, Cursors: cursors}}
	lifecycleH := identityhttp.LifecycleHandlers{
		Lifecycle: command.UserLifecycle{UoW: core.UoW, Repo: write.LifecycleRepo{}, Events: events, Audit: auditor},
		ResetPassword: command.ResetStudentPassword{UoW: core.UoW, Repo: write.LifecycleRepo{}, KC: kc, Sessions: gameAuth, Events: events,
			Audit: auditor, Issuers: issuers},
		Cleanup: cleanup, Anonymize: anonymize}
	platformH := identityhttp.PlatformAdminHandlers{
		Reads:  query.PlatformAdmin{Model: read.PlatformAdminModel{Session: core.ReadSession}, Owners: gw, Cursors: cursors},
		Assign: command.AssignToTenants{PlatformAdminDeps: pad}, Unassign: command.UnassignFromTenants{PlatformAdminDeps: pad},
		Restore: command.RestoreTrashItem{PlatformAdminDeps: pad}, Clone: command.CreateTenantClone{PlatformAdminDeps: pad},
		Merge: command.MergeUsers{PlatformAdminDeps: pad}}
	integrationsCfg, err := integrationConfig(cfg, localIssuer)
	if err != nil {
		return err
	}
	platformUsersH := identityhttp.PlatformUserHandlers{ // F059-1, F059-2 (owner F012)
		List: query.PlatformUsers{Model: principals, Cursors: cursors},
		Create: command.PlatformStaff{UoW: core.UoW, Repo: write.StaffRepo{}, Roles: userRoles, KC: kc, Events: events, Audit: auditor,
			LocalIssuer: localIssuer},
		Integrations: query.Integrations{Config: integrationsCfg, Stats: read.SisAdminModel{Session: core.ReadSession}}}
	webhookRecv := command.ReceiveSisWebhook{UoW: core.UoW, Repo: write.SisContractRepo{}, Secrets: providers.Secrets,
		Verifier: webhook.Verifier{Validator: validator}, Nonces: cache.SisWebhookNonces{C: vk}, Jobs: queue, Metrics: metrics.Sync{}}
	ipKey := gaCfg.IPHashKey

	r.HTTP(Name, func(s *httpx.Server) error {
		for _, reg := range []func(*httpx.Server) error{
			func(s *httpx.Server) error { return identityhttp.Register(s, h) },
			func(s *httpx.Server) error { return identityhttp.RegisterSettings(s, settingsH) },
			func(s *httpx.Server) error { return identityhttp.RegisterLifecycle(s, lifecycleH) },
			func(s *httpx.Server) error { return identityhttp.RegisterPlatformAdmin(s, platformH) },
			func(s *httpx.Server) error { return identityhttp.RegisterPlatformUsers(s, platformUsersH) },
			func(s *httpx.Server) error {
				return identityhttp.RegisterGameAuth(s, identityhttp.GameAuthHandlers{Session: gameAuth, IPHashKey: ipKey})
			},
			func(s *httpx.Server) error {
				return identityhttp.RegisterPortalAuth(s, identityhttp.PortalAuthHandlers{Session: portalAuth, Setup: portalAuth, Metrics: pm})
			},
			func(s *httpx.Server) error {
				return identityhttp.RegisterSisContract(s, identityhttp.SisContractHandlers{Connections: sisContract, Sync: sisSync})
			},
		} {
			if err := reg(s); err != nil {
				return err
			}
		}
		return nil
	})
	m.webhook = sisinternal.Webhook{Receive: webhookRecv}

	// Worker events: relay, the outbox consumers and the SIS job work-queue.
	if err := r.Relay(Name); err != nil {
		return err
	}
	consumers := map[string]struct {
		subjects []string
		handler  outbox.Handler
	}{
		consumer.KeycloakMembershipName: {consumer.KeycloakMembershipSubjects, consumer.KeycloakMembership{
			SyncOrganization: command.SyncTenantOrganization{Tenants: write.TenantRepo{}, KC: kc},
			SyncMembership: command.SyncUserMembership{Reader: read.MembershipModel{Session: core.ReadSession, LocalIssuer: localIssuer}, KC: kc,
				Events: events, Audit: auditor}}.Handle},
		consumer.KeycloakProvisionerName: {consumer.KeycloakProvisionerSubjects, consumer.KeycloakProvisioner{Provision: command.ProvisionStudentAccount{
			UoW: core.UoW, KC: kc, Accounts: write.SisSyncRepo{}, Sync: write.SisSyncRepo{}, Events: events, Audit: auditor,
			LocalIssuer: localIssuer, Issuers: issuers}}.Handle},
		consumer.KeycloakAccessName: {consumer.KeycloakAccessSubjects, consumer.KeycloakAccess{Access: command.KeycloakAccess{UoW: core.UoW,
			Repo: write.LifecycleRepo{}, KC: kc, Anon: kc, Email: kc, Issuers: issuers}}.Handle},
		consumer.GameAuthRevokeName:          {consumer.GameAuthRevokeSubjects, consumer.GameAuthRevoke{Revoker: gameAuth}.Handle},
		consumer.RevokeOnSessionReplacedName: {consumer.RevokeOnSessionReplacedSubjects, consumer.RevokeOnSessionReplaced{Revoker: gameAuth}.Handle},
		consumer.LastLoginName: {consumer.LastLoginSubjects, consumer.LastLoginOnGameLoggedIn{Users: command.RecordGameLogin{
			Users: write.LastLoginRepo{}}}.Handle},
		consumer.DailyPlayTimeName: {consumer.DailyPlayTimeSubjects, consumer.DailyPlayTime{PlayTime: command.RecordPlayTime{
			PlayTime: write.PlayTimeRepo{}, Location: policy.Location()}}.Handle}, // OQ-17
		consumer.DefaultAssignmentsName: {consumer.DefaultAssignmentsSubjects, consumer.DefaultAssignments{Apply: command.ApplyDefaultAssignments{PlatformAdminDeps: pad}}.Handle},
		consumer.FeatureToggleCacheName: {[]string{consumer.FeatureToggleCacheSubject}, consumer.FeatureToggleCache{Cache: toggles}.Handle},
	}
	for name, c := range consumers {
		c := c
		name := name
		stream := streamOf(c.subjects[0])
		r.Consumer(name, func(js jetstream.JetStream) outbox.ConsumerOptions {
			return outbox.ConsumerOptions{Module: Name, Name: name, Stream: stream, Subjects: c.subjects, Platform: true,
				UoW: core.UoW, DLQDB: core.App, JS: js, Handler: c.handler}
		})
	}
	am, err := auditmaint.NewMetrics(reg)
	if err != nil {
		return err
	}
	sink := consumer.AuditSink{Sink: command.AuditSink{UoW: core.UoW, Repo: write.AuditRepo{}, Metrics: am}}
	// One durable per stream EVT_<MODULE> whose producer runs an outbox relay in this release. EVT_REALTIME has no
	// producer yet (module realtime has no database or relay): its audit sink is wired with that relay [Proposed].
	for _, producer := range consumer.AuditSinkModules {
		if producer == "realtime" {
			continue
		}
		subject := consumer.AuditSinkSubject(producer)
		stream := natsx.EventStream(producer)
		r.Consumer(consumer.AuditSinkName+"@"+stream, func(js jetstream.JetStream) outbox.ConsumerOptions {
			return outbox.ConsumerOptions{Module: Name, Name: consumer.AuditSinkName, Stream: stream, Subjects: []string{subject}, Platform: true,
				UoW: core.UoW, DLQDB: core.App, JS: js, Handler: sink.Handle}
		})
	}
	process := command.ProcessSisImport{UoW: core.UoW, Imports: write.SisSyncRepo{}, Storage: objects,
		Scanner: scanner.ClamAV{Addr: cfg.String("EW_CLAMAV_ADDR")}, Parser: csvreader.Parser{V: validator}, Run: run, Audit: auditor}
	r.Background(Name, module.KindConsumer, "identity.sis_jobs", r.RunsEvents(), func(ctx context.Context) error {
		if err := messaging.EnsureSisJobsStream(ctx, js, cfg.Int("EW_NATS_STREAM_REPLICAS", 1)); err != nil {
			return err
		}
		return messaging.ConsumeSisJobs(ctx, js, messaging.SisJobHandlers{LoginRefresh: ls.Refresh, CsvImport: func(ctx context.Context, j port.CsvImportJob) error {
			return process.Handle(ctx, j.TenantID, j.ImportID)
		},
			ScheduledSync: sisSync.Execute, WebhookBatch: sisSync.ExecuteWebhook})
	})

	// Worker jobs.
	var partitionDB = core.Platform
	if r.RunsJobs() {
		if partitionDB, err = r.SQL(Name, "partition_admin"); err != nil {
			return err
		}
	}
	maint := command.AuditMaintenance{Maint: auditmaint.Maint{DB: partitionDB}, Metrics: am}
	sagas := command.PlatformSagas{PlatformAdminDeps: pad}
	lifecycleDefs := map[string]saga.Definition{}
	for _, d := range []saga.Definition{cleanup.Definition(), restore.Definition(), anonymize.Definition()} {
		lifecycleDefs[d.Type] = d
	}
	r.Job(Name, "partition-maintainer", job.AuditPartitionInterval, module.JobFunc(job.AuditPartitions(maint)))
	r.Job(Name, "audit-retention", job.AuditRetentionInterval, module.JobFunc(job.AuditRetention(maint)))
	r.Job(Name, "student-inactive-cleanup", job.StudentCleanupInterval, module.JobFunc(job.StudentInactiveCleanup(cleanup)))
	r.Job(Name, "saga-retry", job.SagaRetryInterval, module.JobFunc(job.SagaRetry(engine, lifecycleDefs)))
	r.Job(Name, "retention-anonymizer", RetentionInterval, module.JobFunc(job.RetentionAnonymizer(anonymize)))
	r.Job(Name, "platform-saga", job.PlatformSagaInterval, module.JobFunc(job.PlatformSaga(sagas)))
	r.Job(Name, command.ReconcileJobName, OrgReconcileInterval, func(ctx context.Context) (int, error) {
		rep, err := command.ReconcileOrganizations{Reader: read.OrgMembershipModel{Session: core.ReadSession, LocalIssuer: localIssuer}, KC: kc,
			UoW: core.UoW, Audit: auditor}.Handle(ctx)
		return rep.Removed, err
	})
	r.Job(Name, "user-role-expiry", UserRoleExpiryInterval, command.ExpireUserRoles{UoW: core.UoW, Expiry: write.UserRoleRepo{},
		Repo: write.UserRoleRepo{}, Events: events, Audit: auditor}.RunOnce)
	r.Job(Name, "sis-import-watch", job.SisImportWatchInterval, command.WatchSisImports{UoW: core.UoW, Imports: write.SisSyncRepo{},
		Storage: objects, Jobs: queue}.RunOnce)
	r.Job(Name, "sis-staging-anonymizer", job.SisStagingAnonymizeTick, command.AnonymizeSisStaging{UoW: core.UoW,
		Staging: write.SisSyncRepo{}}.RunOnce)
	r.Job(Name, "sis-scheduler", job.SisSchedulerInterval, command.ScheduleSisSync{UoW: core.UoW, Repo: write.SisContractRepo{},
		Jobs: queue}.RunOnce)
	return nil
}

// streamOf returns EVT_<PRODUCER> of an event subject ew.evt.<producer>.<type>.
func streamOf(subject string) string {
	const prefix = "ew.evt."
	rest := subject[len(prefix):]
	for i := 0; i < len(rest); i++ {
		if rest[i] == '.' {
			return natsx.EventStream(rest[:i])
		}
	}
	return natsx.EventStream(rest)
}

// FeatureGateSource is the source of disabled menus/actions for pkg/httpx/featuregate (ADR-0090 §3): the
// composition root builds the gate of the shared HTTP server with it (nil before Register built the module).
func (m *Module) FeatureGateSource() featuregate.Source {
	if !m.hasToggles {
		return nil
	}
	return m.toggles
}
