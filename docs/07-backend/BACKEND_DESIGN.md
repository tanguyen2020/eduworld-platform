# Backend Design — Go Microservices (CQRS + Clean Architecture)

> Trạng thái: Draft · Phiên bản: 0.3 · Owner: Senior Golang Developer + Backend Architect · Cập nhật: 2026-11-15
> Liên quan: [ADR-0036](../adr/ADR-0036-microservices-theo-he-thong-lon-thay-adr-0001.md) · [ADR-0037](../adr/ADR-0037-moi-service-mot-database-postgresql-thay-mot-phan-adr-0003.md) · [ADR-0038](../adr/ADR-0038-cqrs-clean-architecture-trong-moi-service.md) · [ADR-0039](../adr/ADR-0039-orm-ent-cho-go-tuong-duong-entity-framework-migration-bang-a.md) · [ADR-0040](../adr/ADR-0040-goi-service-to-service-bang-typed-rest-client-sinh-tu-openap.md) · [ADR-0041](../adr/ADR-0041-moi-cot-truong-kieu-chuoi-phai-co-gioi-han-do-dai.md) · [ADR-0042](../adr/ADR-0042-domain-event-qua-transactional-outbox-nats-jetstream-thay-tr.md) · [SYSTEM_ARCHITECTURE](../06-architecture/SYSTEM_ARCHITECTURE.md) · [DATABASE_DESIGN](DATABASE_DESIGN.md) · [EVENT_DESIGN](EVENT_DESIGN.md) · [VALKEY_DESIGN](VALKEY_DESIGN.md) · [API_DESIGN](API_DESIGN.md) · [MULTITENANCY](../06-architecture/MULTITENANCY.md)

**Mục đích & phạm vi.** Cách xây một service Go: platform kit, Clean Architecture, CQRS, ent + Atlas, typed client, outbox, cấu hình, health, kiểm thử. Quy ước API ở API_DESIGN; luồng event ở EVENT_DESIGN; DDL ở DATABASE_DESIGN.

> **Thay đổi v0.3:** viết lại toàn bộ cho mô hình microservices. Bản v0.1 (modular monolith, sqlc + goose, outbox → Valkey Streams) đã bị thay bởi ADR-0036…0042.

## 1. Mục tiêu

| # | Mục tiêu | Cách đạt được |
|---|---|---|
| 1 | 6 service độc lập, cùng một khuôn | Mọi service theo cùng cấu trúc Clean Architecture (§3) và dùng platform kit `pkg/` (§2) [Confirmed — ADR-0036/0038] |
| 2 | Tách đọc/ghi rõ ràng | CQRS: command qua aggregate + ent trong transaction + outbox; query đọc read model (§4) [Confirmed — ADR-0038] |
| 3 | Tenant-safe và soft-delete mặc định | ent interceptor (global filter) + hook soft-delete + RLS (§5) [Confirmed — ADR-0005/0028/0039] |
| 4 | Gọi service khác an toàn, type-safe | Typed client sinh từ OpenAPI + failsafe-go + token service (§7) [Confirmed — ADR-0040] |
| 5 | Không mất event | Transactional outbox → NATS JetStream; consumer idempotent (§8) [Confirmed — ADR-0042] |
| 6 | Dữ liệu chuỗi có giới hạn | `MaxLen` trong ent + `maxLength` trong OpenAPI + validation (§5.3) [Confirmed — ADR-0041] |
| 7 | Quan sát được xuyên service | slog JSON + OpenTelemetry, `traceparent` qua HTTP và NATS (§11) |

## 2. Platform kit `pkg/` (module Go dùng chung)

| Gói | Nội dung | Ghi chú |
|---|---|---|
| `pkg/mediator` | `CommandHandler[C,R]`, `QueryHandler[Q,R]`, `Behavior` pipeline, đăng ký handler | Không dùng thư viện mediator ngoài |
| `pkg/authz` | `Principal`, `Permission`, **permission catalog** (role → permission, `ValidateCatalog`) | identity-service gán role; mọi service kiểm tra permission bằng catalog dùng chung + Principal từ `POST /internal/v1/principals:resolve` |
| `pkg/tenancy` | `TenantScope`, `ScopeFor`, `Bypass(ctx, scope, reason)` (liên tenant / Thùng rác, bắt buộc lý do + audit), `SET LOCAL app.tenant_id` trong unit of work | §5, [MULTITENANCY](../06-architecture/MULTITENANCY.md) |
| `pkg/entx` | Mixin `Base` (id, tenant_id, created/updated/deleted, version), `PlatformBase` (tenant_id nullable), `AppendOnly`; **interceptor** global filter tenant + soft-delete; **hook** soft-delete | §5.1–5.2 |
| `pkg/outbox` | `Writer` (trong tx ent), `Relay` (→ JetStream), `Consumer` (dedupe `processed_events`, retry, DLQ) | §8 |
| `pkg/natsx` | Kết nối, JetStream publish/consume, request/reply cho lệnh realtime, header ngữ cảnh | §9 |
| `pkg/httpx` | Server chi, middleware (auth, principal, tenant, problem, idempotency, recover, log, otel) | |
| `pkg/svcclient` | Wrapper cho typed client: token service (client credentials), failsafe-go, header ngữ cảnh | §7 |
| `pkg/problem` | RFC 9457, catalog mã lỗi chung | §10 |
| `pkg/observability`, `pkg/config`, `pkg/health`, `pkg/clock`, `pkg/ids` | slog/OTel, env config, health registry, Clock, UUIDv7 | |

Valkey: mọi key có prefix `ew:<svc>:` (user ACL theo service, [VALKEY_DESIGN §1](VALKEY_DESIGN.md)); key builder trong `pkg/` tự thêm prefix.

`pkg/` không chứa logic nghiệp vụ. Thay đổi `pkg/` là thay đổi chung, cần review của Backend Architect và chạy test mọi service.

## 3. Cấu trúc một service (Clean Architecture)

```text
services/learning/
├── go.mod                              # module github.com/<org>/eduworld/services/learning
├── cmd/
│   ├── api/main.go                     # HTTP public + internal (cổng riêng), composition root
│   ├── worker/main.go                  # outbox relay, NATS consumers, job theo lịch
│   └── migrate/main.go                 # Atlas migrate apply (job CI/CD)
├── internal/
│   ├── domain/                         # TẦNG DOMAIN — thuần, không I/O
│   │   ├── attempt/                    # aggregate Attempt: entity, value object, rule, domain event
│   │   ├── question/
│   │   └── mastery/
│   ├── application/                    # TẦNG APPLICATION
│   │   ├── command/                    # StartAttempt, SubmitAnswer, SubmitAttempt, PublishQuestionVersion...
│   │   ├── query/                      # GetAttempt, ListAssignments, TeacherDashboard...
│   │   ├── port/                       # interface: AttemptRepository, ReadModel, IdentityClient, EventPublisher, Clock
│   │   └── dto/
│   ├── infrastructure/                 # TẦNG INFRASTRUCTURE
│   │   ├── persistence/                # ent repository, read model SQL, unit of work
│   │   ├── client/                     # identityclient (typed client oapi-codegen + svcclient)
│   │   ├── messaging/                  # outbox writer wiring, JetStream consumer adapter
│   │   ├── storage/                    # adapter S3: AWS SDK for Go v2 (s3 + s3 presign), endpoint cấu hình EW_S3_ENDPOINT, path-style EW_S3_FORCE_PATH_STYLE; mọi môi trường trỏ floci (ADR-0044, ADR-0071); đổi nhà cung cấp chỉ đổi cấu hình
│   │   └── cache/                      # Valkey
│   └── presentation/                   # TẦNG PRESENTATION
│       ├── http/public/                # handler sinh từ specs/openapi/services/learning.v1.yaml
│       ├── http/internal/              # handler sinh từ specs/openapi/services/learning.internal.v1.yaml
│       ├── consumer/                   # handler event NATS → command
│       └── job/                        # job theo lịch → command
├── ent/
│   ├── schema/                         # entity schema (code-first), dùng pkg/entx
│   └── (generated)
├── migrations/                         # Atlas versioned migrations (SQL review trong MR)
├── api/                                # code sinh (server + types) từ OpenAPI
└── tests/{unit,integration,contract}/
```

| Tầng | Được import | Cấm import |
|---|---|---|
| `domain` | stdlib, `uuid`, `domain` cùng service | ent, pgx, net/http, nats, valkey, `application`, `infrastructure`, `presentation`, `pkg/*` trừ `pkg/clock`/`pkg/ids` |
| `application` | `domain`, `pkg/mediator`, `pkg/authz`, `pkg/problem` | ent, pgx, net/http, nats, valkey, `infrastructure`, `presentation` |
| `infrastructure` | `application/port`, `domain`, ent, pgx, nats, valkey, typed client | `presentation` |
| `presentation` | `application` (command/query/dto), `pkg/httpx`, code sinh `api/` | `infrastructure` (trừ composition root ở `cmd/`) |
| `cmd/*` | mọi tầng (composition root) | — |
| Mọi tầng | — | package `internal` của **service khác** (Go đã chặn); chỉ dùng client sinh từ spec |

```yaml
# services/<svc>/.golangci.yml (trích) — depguard bảo vệ luật phụ thuộc
linters-settings:
  depguard:
    rules:
      domain:
        files: ["**/internal/domain/**"]
        deny:
          - { pkg: "entgo.io/ent", desc: "domain không biết ORM" }
          - { pkg: "github.com/jackc/pgx", desc: "domain không biết DB" }
          - { pkg: "net/http", desc: "domain không biết transport" }
          - { pkg: "github.com/nats-io", desc: "domain không biết messaging" }
          - { pkg: "github.com/<org>/eduworld/services/learning/internal/application", desc: "phụ thuộc hướng vào trong" }
          - { pkg: "github.com/<org>/eduworld/services/learning/internal/infrastructure" }
      application:
        files: ["**/internal/application/**"]
        deny:
          - { pkg: "entgo.io/ent" }
          - { pkg: "github.com/jackc/pgx" }
          - { pkg: "net/http" }
          - { pkg: "github.com/<org>/eduworld/services/learning/internal/infrastructure" }
          - { pkg: "github.com/<org>/eduworld/services/learning/internal/presentation" }
      presentation:
        files: ["**/internal/presentation/**"]
        deny:
          - { pkg: "github.com/<org>/eduworld/services/learning/internal/infrastructure", desc: "presentation chỉ gọi application" }
```

`go-arch-lint` chạy kèm để kiểm tra ma trận tầng đầy đủ; vi phạm chặn merge.

### 3.1. Phân bổ tầng theo service

| Service | Aggregate chính (domain) | Command tiêu biểu | Query tiêu biểu | Worker |
|---|---|---|---|---|
| identity | Tenant, User, ExternalIdentity, Enrollment, Role, UserRole, SyncRun | BootstrapSession, ProvisionStudent, CreateRole, GrantUserRole, RevokeUserRole, RunSisSync, CleanupStudents, StartTenantClone | GetPrincipal, ListTenants, ListStudents, TrashList | outbox relay, SIS sync, Keycloak membership, student cleanup, saga platform, audit sink |
| learning | QuestionVersion, Attempt, ExamSession, Assignment, ObjectiveMastery | StartAttempt, SubmitAnswer, SubmitAttempt, CreateAssignment, PublishContent, CreateQuestionPlan, ScoreAnswer | GetAttempt, SearchQuestions, TeacherDashboard, MasteryReport | outbox relay, mastery updater, analytics rollup, read model login activity |
| game | GameProfile, QuestProgress, GameSession, XPLedger, GameConfigVersion | AcceptQuest, ReportStep, StartGameSession (takeover), GrantReward, PublishGameConfig | GetWorldManifest, GetQuestLog, LoginEvents | outbox relay, quest/XP consumers, session timeout |
| competition | Party, Room, Match, Rating, Tournament | CreateParty, InviteMember, StartPartySession, EnqueueTicket, SubmitBattleAnswer, FinalizeMatch | GetParty, Leaderboard, Bracket | matchmaker, leaderboard rollup, rating updater |
| chat | Channel, Message, ModerationCase, SensitiveCatalog, Friendship | SendMessage, EditMessage, RecallMessage, ToggleModerationSwitch, ResolveCase | History, Search, Unread, CaseQueue | chat-persister, notifier, moderation-async, stats, channel-closer, gameplay purge |
| realtime-gateway | (không có domain nghiệp vụ) | — | — | — (chỉ connection hub, router NATS, subscriber Valkey) |

## 4. CQRS

### 4.1. Mediator và pipeline behavior

```go
// pkg/mediator/mediator.go
type CommandHandler[C any, R any] interface {
    Handle(ctx context.Context, cmd C) (R, error)
}
type QueryHandler[Q any, R any] interface {
    Handle(ctx context.Context, q Q) (R, error)
}

// Behavior bọc handler theo thứ tự đăng ký (giống pipeline behavior của MediatR).
type Behavior func(ctx context.Context, req any, next func(ctx context.Context) (any, error)) (any, error)

// Send dispatch command: Logging → Tracing → Validation → Authorization → Idempotency → Transaction → handler.
func Send[C any, R any](ctx context.Context, m *Mediator, cmd C) (R, error)

// Ask dispatch query: Logging → Tracing → Validation → Authorization → (Cache) → handler. Không có Transaction ghi.
func Ask[Q any, R any](ctx context.Context, m *Mediator, q Q) (R, error)
```

| Behavior | Command | Query | Mô tả |
|---|---|---|---|
| Logging | ✓ | ✓ | slog có `trace_id`, `tenant_id`, `user_id`, tên request, thời gian, kết quả (không log payload nhạy cảm) |
| Tracing | ✓ | ✓ | span `<service>.<RequestName>` |
| Validation | ✓ | ✓ | `Validate() error` của request (độ dài chuỗi theo ADR-0041, enum, khoảng giá trị) |
| Authorization | ✓ | ✓ | Request khai báo `RequiredPermission()` + resource; kiểm tra `Principal` + `TenantScope` |
| Idempotency | ✓ (khi có `IdempotencyKey`) | — | Lưu/đọc `idempotency_records` |
| Transaction | ✓ | — | Mở ent tx, `SET LOCAL app.tenant_id`, commit kèm outbox |
| Cache | — | tùy chọn | Cache Valkey cho query khai báo `CacheKey()`/TTL |

### 4.2. Command (ghi) — mẫu

```go
// services/learning/internal/application/command/submit_attempt.go
type SubmitAttempt struct {
    AttemptID      uuid.UUID
    IdempotencyKey string // ≤ 64
}

func (c SubmitAttempt) RequiredPermission() authz.Permission { return authz.PermAttemptSubmitOwn }
func (c SubmitAttempt) Validate() error {
    if len(c.IdempotencyKey) > 64 { return problem.Invalid("idempotency_key", "max 64") }
    return nil
}

type SubmitAttemptHandler struct {
    Attempts port.AttemptRepository // ent adapter
    Scorer   port.Scorer            // domain service, chạy trong service
    Events   port.EventPublisher    // outbox writer (cùng tx)
    Clock    clock.Clock
}

func (h SubmitAttemptHandler) Handle(ctx context.Context, cmd SubmitAttempt) (dto.AttemptResult, error) {
    p := authz.MustPrincipal(ctx)
    a, err := h.Attempts.Get(ctx, cmd.AttemptID) // interceptor đã thêm tenant_id + deleted_at IS NULL
    if err != nil {
        return dto.AttemptResult{}, err
    }
    if a.StudentID() != p.UserID {
        return dto.AttemptResult{}, problem.NotFound("attempt") // ngoài phạm vi → 404 (chống dò ID)
    }
    if err := a.Submit(h.Clock.Now()); err != nil { // quy tắc nghiệp vụ trong aggregate
        return dto.AttemptResult{}, err
    }
    result := h.Scorer.Score(a)
    if err := h.Attempts.Save(ctx, a); err != nil { // optimistic concurrency theo version
        return dto.AttemptResult{}, err
    }
    for _, e := range a.PullEvents() { // exercise.submitted, question.answered...
        if err := h.Events.Publish(ctx, e); err != nil { // ghi outbox_events trong cùng tx
            return dto.AttemptResult{}, err
        }
    }
    return dto.FromAttempt(a, result), nil
}
```

### 4.3. Query (đọc) — mẫu

```go
// services/learning/internal/application/query/teacher_dashboard.go
type TeacherDashboard struct {
    ClassID uuid.UUID
    Period  string // ≤ 16, "7d" | "30d" | "term"
}

func (q TeacherDashboard) RequiredPermission() authz.Permission { return authz.PermAnalyticsClassRead }

type TeacherDashboardHandler struct {
    ReadModel port.DashboardReadModel // SQL tường minh trên bảng read model, có thể dùng replica
}

func (h TeacherDashboardHandler) Handle(ctx context.Context, q TeacherDashboard) (dto.Dashboard, error) {
    return h.ReadModel.ClassSummary(ctx, q.ClassID, q.Period) // không load aggregate, không transaction ghi
}
```

Quy tắc CQRS:

1. Command không trả read model nặng (chỉ ID, version, kết quả cần thiết cho UI tức thời như kết quả chấm).
2. Query không thay đổi trạng thái, không ghi outbox.
3. Read model cập nhật **đồng bộ** trong cùng transaction khi đơn giản (ví dụ bảng tóm tắt attempt), hoặc **bất đồng bộ** qua consumer (dashboard, leaderboard, analytics). Màn hình dùng read model bất đồng bộ ghi rõ độ trễ chấp nhận được (thường ≤ 5 s).
4. Không event sourcing.

## 5. Persistence: ent + Atlas (tương đương Entity Framework)

### 5.1. Interceptor global filter + soft-delete

```go
// pkg/entx/interceptor.go
// Tương đương HasQueryFilter của EF Core: mọi query trên entity có mixin Base bị thêm điều kiện.
func TenantFilterInterceptor() ent.Interceptor {
    return intercept.TraverseFunc(func(ctx context.Context, q intercept.Query) error {
        if tenancy.Bypassed(ctx) { // chỉ bật qua tenancy.Bypass (lý do + audit)
            return nil
        }
        scope, ok := authz.ScopeFromContext(ctx)
        if !ok {
            return ErrMissingTenantScope // không có scope → lỗi, không trả dữ liệu
        }
        q.WhereP(
            sql.FieldIn("tenant_id", scope.TenantIDs()...),
            sql.FieldIsNull("deleted_at"),
        )
        return nil
    })
}

// Hook soft-delete: chuyển Delete/DeleteOne thành Update deleted_at (ADR-0028).
func SoftDeleteHook() ent.Hook {
    return hook.On(func(next ent.Mutator) ent.Mutator {
        return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
            sd, ok := m.(interface{ SetOp(ent.Op); SetDeletedAt(time.Time); SetDeletedBy(uuid.UUID) })
            if !ok {
                return nil, ErrHardDeleteForbidden
            }
            sd.SetOp(ent.OpUpdate)
            sd.SetDeletedAt(clock.Now(ctx))
            sd.SetDeletedBy(authz.MustPrincipal(ctx).UserID)
            return next.Mutate(ctx, m)
        })
    }, ent.OpDelete|ent.OpDeleteOne)
}
```

- Truy cập không filter (System Admin liên tenant, Thùng rác) chỉ qua `tenancy.Bypass(ctx, scope, reason)` (System Admin liên tenant, Thùng rác), tự ghi audit `cross_tenant_access`/`trash_access`.
- RLS ở database vẫn bật (lớp thứ hai). Unit of work mở tx ent và chạy `SET LOCAL app.tenant_id` ([MULTITENANCY](../06-architecture/MULTITENANCY.md)).
- Ngoại lệ hard-delete duy nhất: package `services/chat/internal/infrastructure/purge` dùng DSN của role `chat_purger` (ADR-0033), không đi qua ent hook.

### 5.2. Mixin chung và schema mẫu

```go
// pkg/entx/base.go — mọi entity nghiệp vụ tenant-scoped
type Base struct{ mixin.Schema }

func (Base) Fields() []ent.Field {
    return []ent.Field{
        field.UUID("id", uuid.UUID{}).Default(ids.NewV7).Immutable(),
        field.UUID("tenant_id", uuid.UUID{}).Immutable(),
        field.Time("created_at").Default(time.Now).Immutable(),
        field.UUID("created_by", uuid.UUID{}).Optional().Nillable().Immutable(),
        field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
        field.UUID("updated_by", uuid.UUID{}).Optional().Nillable(),
        field.Time("deleted_at").Optional().Nillable(),
        field.UUID("deleted_by", uuid.UUID{}).Optional().Nillable(),
        field.String("delete_reason").MaxLen(64).Optional().Nillable(),
        field.Int64("version").Default(1), // optimistic concurrency (tương đương RowVersion)
    }
}

// services/game/ent/schema/party.go (minh họa; Party thuộc competition-service)
func (Party) Mixin() []ent.Mixin { return []ent.Mixin{entx.Base{}} }

func (Party) Fields() []ent.Field {
    return []ent.Field{
        field.String("name").MaxLen(128).NotEmpty(),                    // ADR-0041: tên ngắn ≤ 128
        field.Enum("party_type").Values("friend", "study_group", "quest", "competition_team", "class_group", "custom"),
        field.Enum("visibility").Values("private", "friends", "class"),
        field.Int("max_members").Range(2, 8).Default(4),
        field.UUID("leader_id", uuid.UUID{}),
        field.String("status").MaxLen(64).Default("open"),
    }
}

func (Party) Indexes() []ent.Index {
    return []ent.Index{
        index.Fields("tenant_id", "leader_id").
            Annotations(entsql.IndexWhere("deleted_at IS NULL AND status <> 'disbanded'")),
    }
}
```

### 5.3. Giới hạn độ dài chuỗi (ADR-0041)

- Mọi `field.String` **phải** có `MaxLen(n)` theo bảng độ dài chuẩn ([DATABASE_DESIGN §1.4](DATABASE_DESIGN.md)). ent sinh `varchar(n)`.
- Linter `tools/entlint` (chạy trong `make lint`) duyệt `ent/schema` và **fail** nếu có `field.String`/`field.Text` không có `MaxLen` hoặc `field.Text` không có annotation `// maxlen-justified:` + CHECK độ dài.
- Nội dung dài có cấu trúc (render payload, rich text) dùng `field.JSON` + CHECK `octet_length(col::text) <= N` khai báo bằng `entsql.Check`.
- Validation tầng presentation sinh từ OpenAPI (`maxLength`), tầng application kiểm tra lại trong `Validate()`.

### 5.4. Migration (Atlas)

- `atlas migrate diff --env <svc>` sinh SQL từ ent schema vào `services/<svc>/migrations/`; file SQL được review trong MR (đặc biệt RLS policy, partition, CHECK, REVOKE — viết tay bổ sung trong cùng thư mục).
- `atlas migrate lint` trong CI: chặn thao tác phá dữ liệu (DROP COLUMN/TABLE, đổi kiểu thu hẹp) nếu thiếu nhãn `-- atlas:nolint destructive` kèm ADR/ticket.
- Chiến lược expand/contract; không bao giờ auto-migrate khi service khởi động; job `cmd/migrate` chạy trước rollout ([DEPLOYMENT](../11-operations/DEPLOYMENT.md)).

### 5.5. Repository adapter (mẫu)

```go
// services/learning/internal/infrastructure/persistence/attempt_repo.go
type AttemptRepo struct{ uow *UnitOfWork }

func (r AttemptRepo) Get(ctx context.Context, id uuid.UUID) (*attempt.Attempt, error) {
    row, err := r.uow.Client(ctx).Attempt.Query().
        Where(entattempt.ID(id)).          // tenant_id + deleted_at do interceptor thêm
        WithItems().
        Only(ctx)
    if ent.IsNotFound(err) {
        return nil, problem.NotFound("attempt")
    }
    if err != nil {
        return nil, fmt.Errorf("get attempt %s: %w", id, err)
    }
    return toDomain(row), nil
}

func (r AttemptRepo) Save(ctx context.Context, a *attempt.Attempt) error {
    n, err := r.uow.Client(ctx).Attempt.Update().
        Where(entattempt.ID(a.ID()), entattempt.Version(a.Version())).
        SetStatus(string(a.Status())).
        SetNillableSubmittedAt(a.SubmittedAt()).
        SetScore(a.Score()).
        AddVersion(1).
        Save(ctx)
    if err != nil {
        return fmt.Errorf("save attempt %s: %w", a.ID(), err)
    }
    if n == 0 {
        return problem.Conflict("CONCURRENT_MODIFICATION")
    }
    return nil
}
```

## 6. Principal, authorization, tenant

- Chuỗi middleware public: `auth` (verify JWT: `iss`, `aud`, `exp`, chữ ký; JWKS cache) → `principal` → `tenant` → handler.
- **identity-service** là nơi xác định tenant khi đăng nhập (game: `POST /api/v1/auth/game/login` qua backend, ADR-0068; portal: `POST /api/v1/auth/session` sau luồng trình duyệt, ADR-0026/0063/0067) và quản lý Role/UserRole (ADR-0066). Các service khác dựng `Principal` bằng `POST /internal/v1/principals:resolve` (body `{user_id, tenant_id, client_id}`; typed client, cache Valkey 60 s theo `(user_id, tenant_id)`, invalidate bằng event `user_role.changed`, `role.updated`, `tenant_membership.changed`, `tenant.updated`). identity-service lỗi/timeout và không có cache hợp lệ → **503** (`UNAVAILABLE`, retryable); **không fail-open**.
- **Middleware `tenant` (ADR-0065):** đọc header **`X-Tenant-Id` (bắt buộc)** trên mọi route tenant-scoped. Thiếu/sai định dạng → `400 TENANT_HEADER_REQUIRED` (APISIX đã chặn trước, middleware kiểm tra lại). Header phải **khớp** tenant của phiên = claim `organization` đã đối chiếu + membership (`memberships:check`, cache 60 s); lệch → `403 TENANT_MISMATCH` + audit/security event. `Principal.TenantID` lấy từ **kết quả kiểm tra**, không lấy thẳng từ header; repository, ent interceptor, RLS dùng `Principal.TenantID`. Route miễn header: `POST /auth/session`, `GET /game/client-config`, `/livez`, `/readyz`, `GET /me/tenants`. **System Admin:** thao tác ghi bắt buộc `X-Tenant-Id` = tenant đích (phải tồn tại, `status = active`); đọc liên tenant dùng `tenant_ids` (permission `platform.cross_tenant.read`, `azp = eduworld-portal` có MFA) và không gửi header. Người dùng tenant gửi `tenant_ids` → bị bỏ qua.
- Route người chơi (`/game/*`, `/multiplayer/*`, `/competition/*` phía người chơi, ticket realtime) yêu cầu `azp = eduworld-game-backend` (token cấp qua `/auth/game/login`, ADR-0068) và `user_type = student` (ADR-0031), lỗi `403 GAME_ACCESS_STUDENT_ONLY`. Route portal (`/cms/*`, `/lms/*`, `/tenant/*`, `/schools/*`, `/game-admin/*`, `/admin/*`) yêu cầu `azp = eduworld-portal` và `user_type ≠ student` (ADR-0066), lỗi `403 PORTAL_ACCESS_STAFF_ONLY`.
- Permission của `Principal` = hợp permission của mọi **UserRole** còn hiệu lực (`valid_from ≤ now < valid_to`, `deleted_at IS NULL`) trong tenant hiện tại (role platform: phạm vi platform), kèm phạm vi (school/class/subject) để kiểm tra theo tài nguyên.

```go
// pkg/authz/principal.go
type Principal struct {
    UserID        uuid.UUID
    UserType      UserType   // student|teacher|staff|admin
    TenantID      *uuid.UUID // nil = System Admin / service
    TenantType    TenantType // school|unit|common
    Permissions   PermissionSet // hợp permission các UserRole hiệu lực (ADR-0066)
    Scopes        []Scope    // school/class/subject từ UserRole + TeachingAssignment
    IsSystemAdmin bool
    ClientID      string     // azp
    SessionID     string     // sid (thu hồi khi takeover)
    OnBehalfOf    *uuid.UUID // khi request đến từ service khác (X-On-Behalf-Of)
    ServiceName   string     // "svc-game"... khi caller là service
}
```

### 6.1. Fast path đăng nhập ở 10.000 CCU (ADR-0045)

| Bước | Tối ưu | Mục tiêu |
|---|---|---|
| Verify JWT | JWKS cache trong bộ nhớ mỗi replica, làm mới nền 10 phút, làm mới ngay khi gặp `kid` lạ (tối đa 1 lần/30 s); không gọi Keycloak introspection | < 1 ms |
| Resolve `(iss, sub)` → User, memberships, Principal | L1 LRU in-process 15 s + Valkey 60 s (`ew:identity:principal:<user>:<tenant>`); invalidate bằng event | hit ≥ 95% |
| Thông tin tenant, settings, chat policy | Valkey 60 s + L1 | — |
| Kiểm tra SIS nguồn (ADR-0046) | Đọc `sis_identity_links` + trạng thái bản ghi đồng bộ cùng Principal (cùng cache 60 s); **không gọi SIS** | — |
| `POST /auth/game/login` (ADR-0068) | Một lời gọi token endpoint Keycloak (client `eduworld-game-backend`, pool kết nối HTTP giữ sẵn, timeout 3 s, circuit breaker → `503`), rồi resolve membership/enrollment từ cache; không gọi SIS; không log body; hàng chờ vượt năng lực → `429 LOGIN_QUEUE_BUSY` | p95 ≤ 400 ms (gồm băm mật khẩu Keycloak ≤ 80 ms) [Proposed] |
| `POST /auth/session` (portal) | Chỉ đọc DB của identity-service khi cache miss; không gọi đồng bộ service khác | p95 ≤ 150 ms |
| `POST /game/sessions` | Một transaction (game session + login event + `last_game_login_at`), ticket realtime ghi Valkey; bootstrap trả manifest/hồ sơ từ cache | p95 ≤ 150 ms |
| Ghi `last_login_at` sang identity | Qua event `student.game_logged_in` (bất đồng bộ), không chặn đường đăng nhập | — |

Khi Valkey lỗi: đọc DB trực tiếp với giới hạn concurrency (bulkhead) để không đánh sập DB trong bão đăng nhập; vượt giới hạn → `503 + Retry-After`.

Internal API (`/internal/v1/*`, cổng riêng): middleware `serviceauth` kiểm tra token client credentials Keycloak, `azp` thuộc allowlist của endpoint (ví dụ `/internal/v1/attempts` chỉ cho `svc-game`), đọc `X-Tenant-Id` để tạo `TenantScope` (vẫn kiểm tra tenant tồn tại/hoạt động) ([SECURITY_ARCHITECTURE §5.5](../10-security/SECURITY_ARCHITECTURE.md)).

## 7. Gọi service-to-service: typed client (tương đương Refit)

```go
// services/game/internal/infrastructure/client/learning.go
// learningapi được sinh bằng oapi-codegen từ specs/openapi/services/learning.internal.v1.yaml
type LearningClient struct {
    api *learningapi.ClientWithResponses
}

func NewLearningClient(cfg svcclient.Config) (*LearningClient, error) {
    hc := svcclient.NewHTTPClient(cfg,
        svcclient.WithServiceToken("svc-game"),                 // Keycloak client credentials, cache token
        svcclient.WithContextHeaders(),                         // traceparent, X-Tenant-Id, X-Request-Id, X-On-Behalf-Of
        svcclient.WithPolicies(svcclient.DefaultPolicies(cfg)), // failsafe-go; giá trị mặc định = API_DESIGN §12.3 (timeout, retry chỉ idempotent, circuit breaker, bulkhead)
    )
    api, err := learningapi.NewClientWithResponses(cfg.BaseURL, learningapi.WithHTTPClient(hc))
    return &LearningClient{api: api}, err
}

// Implement port của application: game không biết HTTP.
func (c *LearningClient) StartAttempt(ctx context.Context, in port.StartAttemptInput) (port.StartedAttempt, error) {
    res, err := c.api.StartAttemptWithResponse(ctx,
        &learningapi.StartAttemptParams{IdempotencyKey: in.IdempotencyKey},
        learningapi.StartAttemptJSONRequestBody{
            StudentId: in.StudentID, ActivityRef: in.ActivityRef, Context: "quest",
        })
    if err != nil {
        return port.StartedAttempt{}, problem.Unavailable("UNAVAILABLE", err) // circuit open/timeout; mã chuẩn API_DESIGN §4.1
    }
    if res.JSON201 == nil {
        return port.StartedAttempt{}, svcclient.MapProblem(res.HTTPResponse, res.Body) // giữ code Problem gốc
    }
    return toPort(*res.JSON201), nil
}
```

- Quy ước internal API (xác thực, header, allowlist, resilience) chỉ định nghĩa ở [API_DESIGN §12](API_DESIGN.md#12-internal-api-giữa-service-adr-0036-adr-0040); đoạn trên chỉ minh họa cách triển khai.
- Spec internal là contract; thay đổi qua MR vào `specs/openapi/services/<svc>.internal.v1.yaml`, chạy `oasdiff breaking` và contract test của bên gọi.
- Lỗi từ service khác giữ `code` Problem gốc khi có nghĩa với người dùng; lỗi hạ tầng → `503` có `retryable`.
- Không gọi service khác trong transaction ghi DB (gọi trước, hoặc chuyển sang event).

## 8. Outbox → NATS JetStream (ADR-0042)

```go
// pkg/outbox/writer.go
type Event struct {
    ID            uuid.UUID // UUIDv7
    Type          string    // ≤ 64, "exercise.submitted"
    TenantID      *uuid.UUID
    Key           string    // ≤ 128, ordering key
    SchemaVersion int
    Payload       any
    Trace         map[string]string // traceparent
}

// Publish ghi vào bảng outbox_events trong cùng ent tx của command.
func (w *Writer) Publish(ctx context.Context, e Event) error
```

- **Relay** và **Consumer** (dedupe `processed_events`, chuyển event thành command qua mediator, ack sau commit, DLQ): luồng, stream, subject, consumer và DDL ở [EVENT_DESIGN §3](EVENT_DESIGN.md#3-outbox--nats-jetstream-adr-0042) và [DATABASE_DESIGN §3.9](DATABASE_DESIGN.md#39-outbox--processed-events). `pkg/outbox` triển khai đúng luồng đó.
- Không I/O ngoài DB trong transaction nghiệp vụ. Danh sách stream/subject/consumer: [EVENT_DESIGN](EVENT_DESIGN.md).

## 9. Realtime: gateway ↔ service

- `realtime-gateway` nhận lệnh WS, chuyển thành **NATS request** `ew.cmd.<service>.<domain>.<action>` với header `Ew-Tenant-Id`, `Ew-User-Id`, `Ew-Game-Session-Id`, `traceparent`; timeout 2 s; lỗi → `system.command.failed`.
- Service đăng ký handler bằng `natsx.HandleCommand(subject, mediatorAdapter)`: dựng `Principal` từ header (gateway đã xác thực ticket), gọi command/query qua mediator, trả reply.
- Service đẩy sự kiện tới client bằng Valkey Pub/Sub `ew:rt:user:<user_id>`, `ew:rt:room:<room_id>`, `ew:chat:fanout:<channel_id>`; gateway subscribe theo kết nối đang giữ. Chi tiết: [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md).

### 9.1. chat-service với NATS + Valkey (ADR-0035)

| Thành phần | Binary chat-service | Mô tả |
|---|---|---|
| `command.SendMessage/Edit/Recall/Forward/React/Pin` | api (handler NATS `ew.cmd.chat.*` + REST fallback) | Membership, policy, features, rate limit → moderation pipeline → `SeqAllocator` (Valkey Lua) → `ChatPublisher` (JetStream `ew.chat.<tenant>.msg.<channel>`) → reply acked → `Fanout` (Valkey Pub/Sub) |
| `chat-persister` | worker | Pull consumer `persister`; ghi lô `eduworld_chat` (`ON CONFLICT DO NOTHING`), cập nhật `channels.last_seq`, con trỏ; ack sau commit; DLQ sau 5 lần |
| `chat-notifier`, `chat-moderation-async`, `chat-stats` | worker | Mention offline, bước `async_analysis` (ADR-0030), rollup trước purge (ADR-0033) |
| `chat-channel-closer`, `gameplay-chat-purge` | worker | Đóng channel gameplay khi `match.ended`/`party.disbanded`; purge bằng DSN purger |

Stream/consumer tạo idempotent khi worker khởi động (`CreateOrUpdateStream`). Integration test dùng testcontainers `nats:2.10-alpine -js` + Valkey + PostgreSQL.

## 10. Error model

| Lớp | Kiểu | Ví dụ |
|---|---|---|
| Domain | `domain.Error{Code, Kind}` (`NotFound, Conflict, Invalid, Forbidden, PreconditionFailed, RateLimited, Unavailable`) | `ErrAttemptAlreadySubmitted = Conflict("ATTEMPT_ALREADY_SUBMITTED")` |
| Application | Wrap `fmt.Errorf("submit attempt %s: %w", id, err)` | giữ chuỗi nguyên nhân cho log |
| Presentation | `problem.Map(err)` → RFC 9457 | `409 application/problem+json` |
| Client service khác | `svcclient.MapProblem` giữ `code`, lỗi hạ tầng → `503 UNAVAILABLE` (có `retryable`) | |

Mapping `Kind → HTTP`: NotFound 404, Conflict 409, Invalid 422, Forbidden 403, Unauthenticated 401, PreconditionFailed 412, RateLimited 429, Unavailable 503; lỗi không nhận diện → 500 `INTERNAL`, không lộ chi tiết. Tài nguyên ngoài tenant/scope luôn trả **404**. Catalog: [API_DESIGN §4](API_DESIGN.md).

## 11. Context, logging, tracing

- `ctx` là tham số đầu mọi hàm I/O; timeout ở biên (HTTP `ReadHeaderTimeout`, DB `statement_timeout` 5 s request / 60 s job, client service theo §7).
- `slog` JSON: `ts, level, msg, service, env, version, trace_id, span_id, request_id, user_id, tenant_id, route, status, duration_ms, error.code`. **Không log** token, nội dung chat, câu trả lời, họ tên, ngày sinh.
- OpenTelemetry: `otelhttp` (server + typed client), `otelpgx`/ent driver, span mediator, propagation `traceparent` qua HTTP header và NATS header (outbox lưu trace để nối consumer).
- Metric chuẩn mỗi service: `eduworld_http_server_duration_seconds`, `eduworld_svcclient_requests_total{target,code}`, `eduworld_svcclient_circuit_state{target}`, `eduworld_outbox_lag_seconds`, `eduworld_consumer_pending{stream,consumer}`, `eduworld_db_pool_*`. Danh sách: [MONITORING](../11-operations/MONITORING.md).

## 12. Cấu hình (environment variables)

Cấu hình qua env (12-factor); secret qua file `*_FILE` [Proposed — ADR-0024]. Mỗi service có prefix chung `EW_` và tên service trong `EW_SERVICE_NAME`.

| Biến | Ví dụ | Ghi chú |
|---|---|---|
| `EW_ENV` | `local` / `dev` / `staging` / `prod` | |
| `EW_SERVICE_NAME` | `learning-service` | dùng cho log/trace/metric |
| `EW_HTTP_ADDR` / `EW_INTERNAL_HTTP_ADDR` | dải `81xx` (vd. identity `:8101` public; internal listener riêng theo bảng cổng) | public / internal API của binary api |
| `EW_HEALTH_ADDR` / `EW_METRICS_ADDR` | dải `91xx` / `92xx` | health (`/livez`, `/readyz`) của api/worker và metrics Prometheus — bảng cổng đầy đủ: [LOCAL_DEVELOPMENT](../11-operations/LOCAL_DEVELOPMENT.md) |
| `EW_HTTP_SHUTDOWN_TIMEOUT` | `20s` | |
| `EW_DB_URL_FILE` | `/run/secrets/<svc>_db_url` | user DB của service (không BYPASSRLS, không DELETE) |
| `EW_DB_PLATFORM_URL_FILE` | `/run/secrets/<svc>_db_platform_url` | chỉ worker/System Admin (BYPASSRLS, có audit) |
| `EW_DB_PURGER_URL_FILE` | `/run/secrets/chat_db_purger_url` | **chỉ chat-service worker** (ADR-0033) |
| `EW_DB_MAX_CONNS` | `30` | |
| `EW_VALKEY_ADDR` / `EW_VALKEY_PASSWORD_FILE` | `valkey:6379` | user ACL riêng mỗi service |
| `EW_NATS_URLS` / `EW_NATS_CREDS_FILE` / `EW_NATS_TLS_CA_FILE` | `nats://nats-1:4222,...` / `/run/secrets/nats_<svc>.creds` | NATS account/user riêng mỗi service |
| `EW_OIDC_ISSUERS` / `EW_OIDC_AUDIENCE` / `EW_OIDC_ALLOWED_CLOCK_SKEW` | `https://sso.example.vn/realms/eduworld` / `eduworld-api` / `30s` | public API |
| `EW_SERVICE_CLIENT_ID` / `EW_SERVICE_CLIENT_SECRET_FILE` | `svc-learning` | Keycloak client credentials (ADR-0040) |
| `EW_INTERNAL_ALLOWED_CALLERS` | `svc-game,svc-competition` | allowlist `azp` mặc định cho internal API (chi tiết theo endpoint trong code) |
| `EW_CLIENT_<TARGET>_URL` | `EW_CLIENT_IDENTITY_URL=http://identity-service:<cổng internal>` | base URL internal của service đích |
| `EW_CLIENT_<TARGET>_TIMEOUT` | `800ms` | ghi đè timeout mặc định |
| `EW_IDEMPOTENCY_TTL` | `24h` | |
| `EW_OBJECT_STORAGE_*` | endpoint, bucket, key file | learning/game/chat |
| `EW_OTEL_EXPORTER_OTLP_ENDPOINT` | `http://otel-collector:4317` | |
| `EW_LOG_LEVEL` | `info` | |

Biến riêng service (ví dụ):

| Service | Biến |
|---|---|
| identity | `EW_KEYCLOAK_ADMIN_CLIENT_ID`/`_SECRET_FILE` (`eduworld-sis-sync`), `EW_JOB_STUDENT_CLEANUP_CRON`/`_DRY_RUN`, `EW_SIS_*`, `EW_SIS_JOBS_STREAM=IDENTITY_SIS_JOBS` (work-queue) |
| game | `EW_GAME_SESSION_HEARTBEAT_TIMEOUT=45s`, `EW_REALTIME_PUBLIC_URL`, `EW_REALTIME_TICKET_TTL=60s` |
| chat | `EW_CHAT_STREAM=CHAT_MESSAGES`, `EW_CHAT_PUBLISH_TIMEOUT=2s`, `EW_CHAT_PERSIST_BATCH=500`, `EW_CHAT_EDIT_WINDOW=15m`, `EW_CHAT_RECALL_WINDOW=24h`, `EW_JOB_GAMEPLAY_CHAT_PURGE_CRON` |
| competition | `EW_MM_TICK=500ms`, `EW_BATTLE_ANSWER_GRACE=300ms` |
| realtime-gateway | `EW_WS_MAX_CONNECTIONS=5000`, `EW_WS_HEARTBEAT=15s`, `EW_WS_TIMEOUT=45s`, `EW_CMD_TIMEOUT=2s` |

Cấu hình validate khi khởi động; thiếu biến bắt buộc → exit 1, log tên biến (không in giá trị).

## 13. Graceful shutdown

| Binary | Trình tự khi nhận SIGTERM |
|---|---|
| `<svc>` api | readiness `503`; chờ drain 5 s; `srv.Shutdown` (20 s) cho public + internal; drain NATS subscription request/reply; đóng ent/pgx, Valkey; flush OTel |
| `<svc>` worker | ngừng fetch; chờ handler đang chạy (≤ 60 s); commit + ack lô đang xử lý, phần còn lại được redeliver; `nc.Drain()`; nhả Valkey lock |
| realtime-gateway | readiness `503`; gửi `system.server.draining` (`reconnect_after_ms`) tới client; đóng WS `1012` sau tối đa 30 s; huỷ subscribe Valkey |

## 14. Health checks

| Endpoint (cổng admin, không public) | Ý nghĩa | Kiểm tra |
|---|---|---|
| `GET /livez` (cổng 91xx) | Tiến trình còn sống | Không gọi phụ thuộc |
| `GET /readyz` | Sẵn sàng nhận traffic | DB của service, Valkey, NATS (nếu service dùng), JWKS đã nạp, Atlas revision ≥ yêu cầu. **Không** phụ thuộc vào service khác (tránh lỗi dây chuyền) |
| `GET /healthz` | Chi tiết cho vận hành | JSON từng check, trạng thái circuit breaker của từng client |

## 15. Kiểm thử backend

| Tầng | Phạm vi | Công cụ | Chạy khi |
|---|---|---|---|
| Unit | `domain/` (aggregate rule), `application/` (handler với fake port), behavior pipeline, scoring, mastery, Glicko-2, policy chat | `testing`, `testify`, table-driven, fuzz | Mọi commit |
| Integration (trong service) | ent repository + interceptor tenant/soft-delete, RLS, Atlas migrate up, outbox relay → NATS, consumer idempotent | `testcontainers-go` (postgres:17, valkey:8, nats:2.10 -js) | Mọi MR |
| Contract (HTTP) | Public handler khớp `specs/openapi/services/<svc>.v1.yaml`; internal handler khớp `<svc>.internal.v1.yaml`; consumer-driven: test của bên gọi (vd game) chạy với stub sinh từ spec learning | `kin-openapi` validator, Prism mock, `oasdiff` | Mọi MR |
| Contract (event) | Payload event khớp `specs/events/*.schema.json` cả phía publish và consume | ajv/jsonschema Go | Mọi MR |
| Kiến trúc | depguard/go-arch-lint, `entlint` (MaxLen), `sqlguard` (không DELETE ngoài purge) | CI | Mọi MR |
| Authz / tenant | Ma trận permission × endpoint; học sinh tenant A không thấy dữ liệu tenant B; interceptor từ chối khi thiếu scope | test sinh từ catalog | Mọi MR |
| E2E | Login → bootstrap → game session → quest activity (game → learning) → XP (event) → chat trong party | docker compose toàn hệ thống + Keycloak test realm | Nightly + trước release |
| Resilience | learning-service chậm/lỗi → game trả 503 có retryable, circuit mở/đóng; NATS mất → outbox tồn và bù | toxiproxy / dừng container | Nightly |
| Performance | REST, WS, chat, matchmaking, internal call | k6 / Postman performance | Theo [TEST_STRATEGY](../12-project/TEST_STRATEGY.md) |

Không mock database trong integration test; mỗi bug fix kèm test tái hiện; coverage mục tiêu ≥ 80% cho `domain/` và `application/` của learning, competition, chat [Proposed].
