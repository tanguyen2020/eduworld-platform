# Backend Design — Go Modular Monolith (CQRS + Clean Architecture)

> Trạng thái: Draft · Phiên bản: 0.7 · Owner: Senior Golang Developer + Backend Architect · Cập nhật: 2026-11-17 (ADR-0090: middleware `featuregate`; ADR-0091: đăng nhập portal qua module identity) · 2026-11-17 (Áp ADR-0074: một `go.mod`, module `src/backend/modules/<module>/` có facade `api/`, ba binary `core-api`/`realtime`/`worker` + profile worker, facade gọi trong tiến trình qua `src/backend/pkg/modcall`, resilience chỉ cho tích hợp ngoài) · 2026-11-17 (Áp ADR-0089: bộ mixin `src/backend/pkg/entx`, ba loại scope, danh sách đóng `tenancy.Bypass`, RLS chỉ lọc tenant, role outbox; làm rõ: `TenantOptionalAppendOnly`, `SystemPlatformScope`, idempotency trong `UnitOfWork`, phạm vi migration viết tay)
> Liên quan: [ADR-0074](../adr/ADR-0074-modular-monolith-co-ranh-gioi-cuong-che.md) · [ADR-0037](../adr/ADR-0037-moi-service-mot-database-postgresql-thay-mot-phan-adr-0003.md) · [ADR-0038](../adr/ADR-0038-cqrs-clean-architecture-trong-moi-service.md) · [ADR-0087](../adr/ADR-0087-cqrs-tach-read-write-khong-mediator.md) · [ADR-0089](../adr/ADR-0089-chot-tang-du-lieu-rls-mixin-outbox.md) · [ADR-0039](../adr/ADR-0039-orm-ent-cho-go-tuong-duong-entity-framework-migration-bang-a.md) · [ADR-0040](../adr/ADR-0040-goi-service-to-service-bang-typed-rest-client-sinh-tu-openap.md) · [ADR-0041](../adr/ADR-0041-moi-cot-truong-kieu-chuoi-phai-co-gioi-han-do-dai.md) · [ADR-0042](../adr/ADR-0042-domain-event-qua-transactional-outbox-nats-jetstream-thay-tr.md) · [SYSTEM_ARCHITECTURE](../06-architecture/SYSTEM_ARCHITECTURE.md) · [DATABASE_DESIGN](DATABASE_DESIGN.md) · [EVENT_DESIGN](EVENT_DESIGN.md) · [VALKEY_DESIGN](VALKEY_DESIGN.md) · [API_DESIGN](API_DESIGN.md) · [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [DEPLOYMENT](../11-operations/DEPLOYMENT.md)

**Mục đích & phạm vi.** Cách xây backend Go: cấu trúc repository (một Go module), cấu trúc một module, composition root của ba binary, platform kit, Clean Architecture, CQRS, ent + Atlas, facade giữa module, outbox, cấu hình, health, kiểm thử. Quy ước API ở API_DESIGN; luồng event ở EVENT_DESIGN; DDL ở DATABASE_DESIGN; image, Swarm service và replica ở DEPLOYMENT.

> **Thay đổi v0.3:** viết lại cho mô hình 6 microservice (ADR-0036…0042). Bản v0.1 (modular monolith không ranh giới, sqlc + goose, outbox → Valkey Streams) đã bị thay.
>
> **Thay đổi v0.5 (ADR-0087):** bỏ `src/backend/pkg/mediator` và pipeline behavior. CQRS tách rõ **write context** (`application/command`, `port/write`, `persistence/write`, `UnitOfWork` + outbox) và **read context** (`application/query`, `port/read`, `persistence/read`, replica/cache); presentation gọi handler qua interface có kiểu; cross-cutting qua middleware `src/backend/pkg/httpx`, `authz.Require` tường minh, validation trong DTO/handler, span use case qua `src/backend/pkg/observability` (§4).
>
> **Thay đổi v0.7 (ADR-0074, thay ADR-0036):** backend là **modular monolith có ranh giới cưỡng chế**. Một `go.mod` (nay là `src/backend/go.mod`, [ADR-0092](../adr/ADR-0092-to-chuc-ma-nguon-trong-src.md)); năm module domain (`identity`, `learning`, `game`, `competition`, `chat`) + module `realtime` trong `src/backend/modules/<module>/`; ba binary `src/backend/cmd/core-api`, `src/backend/cmd/realtime`, `src/backend/cmd/worker` (profile `events`, `chat`, `jobs`). Mỗi module domain vẫn có database riêng trên một cụm PostgreSQL. Gọi giữa module qua **facade** `src/backend/modules/<module>/api` sinh từ spec internal, trong tiến trình, bằng `src/backend/pkg/modcall` (§7); không HTTP nội bộ, không token `svc-*`.

## 1. Mục tiêu

| # | Mục tiêu | Cách đạt được |
|---|---|---|
| 1 | Sáu module cùng một khuôn, ranh giới cưỡng chế | Mọi module theo cùng cấu trúc Clean Architecture (§3) và dùng platform kit `src/backend/pkg/` (§2); quy tắc `internal/` của Go + `depguard`/`go-arch-lint` chặn import chéo (§3.2) [Confirmed — ADR-0074, ADR-0038] |
| 2 | Tách đọc/ghi rõ ràng | CQRS tách write context (command qua aggregate + ent trong `UnitOfWork` + outbox) và read context (query đọc read model, replica/cache); không mediator, ranh giới kiểm bằng linter (§3, §4) [Confirmed — ADR-0038, ADR-0087] |
| 3 | Tenant-safe và soft-delete mặc định | ent interceptor (global filter) + hook soft-delete + RLS (§5) [Confirmed — ADR-0005/0028/0039] |
| 4 | Gọi module khác an toàn, type-safe | Facade sinh từ spec internal, gọi trong tiến trình qua `src/backend/pkg/modcall` (span, deadline, `TenantScope`, Principal gốc, `Idempotency-Key`, ánh xạ lỗi); resilience chỉ ở adapter tích hợp ngoài (§7) [Confirmed — ADR-0074, ADR-0040] |
| 5 | Không mất event | Transactional outbox → NATS JetStream; consumer idempotent (§8) [Confirmed — ADR-0042] |
| 6 | Dữ liệu chuỗi có giới hạn | `MaxLen` trong ent + `maxLength` trong OpenAPI + validation (§5.3) [Confirmed — ADR-0041] |
| 7 | Hồ sơ chạy khác nhau scale riêng | `core-api`, `realtime` và từng profile `worker-*` là Swarm service riêng, tăng replica độc lập (§3.3, [DEPLOYMENT §3.1](../11-operations/DEPLOYMENT.md)) [Confirmed — ADR-0074] |
| 8 | Quan sát được xuyên module | slog JSON + OpenTelemetry; span cho mỗi lời gọi facade, `traceparent` qua HTTP và NATS (§11) |

## 2. Platform kit `src/backend/pkg/` (package dùng chung)

| Gói | Nội dung | Ghi chú |
|---|---|---|
| `src/backend/pkg/authz` | `Principal`, `Permission`, **permission catalog** (role → permission, `ValidateCatalog`), `Require` (gọi tường minh đầu mỗi use case, §4.1) | Module `identity` gán role; mọi module kiểm tra permission bằng catalog dùng chung + Principal lấy qua facade identity (operation `resolvePrincipal`, §6) |
| `src/backend/pkg/tenancy` | Ba loại scope `TenantScope`, `CrossTenantScope`, `PlatformScope` (tạo qua `authz.ScopeFor(p)`, `authz.CrossTenantScope(p, ids, reason)`, `authz.PlatformScope(p)`, `authz.SystemPlatformScope(job)`); `Bypass(ctx, reason)` với `reason` là hằng số kiểu `BypassReason` thuộc danh sách đóng của ADR-0089 §3 (`system_admin_cross_tenant`, `moderation_case`, `break_glass`, `trash`, `auth_login`, `tenant_clone`, `inactive_cleanup`, `re_evaluation`, `efficacy_study`, `platform_job`, `platform_analytics`; audit `cross_tenant.accessed` + metric `eduworld_tenant_filter_bypass_total{module,reason}`; linter cấm `reason` khác). API công khai (`TenantScope`, `CrossTenantScope`, `PlatformScope`, `SystemPlatformScope`, `Bypass`, `BypassReason`, `Principal`) khóa từ F002, đổi chữ ký cần ADR (ADR-0089 §3) | §5.1, [MULTITENANCY §4.4](../06-architecture/MULTITENANCY.md#44-quy-tắc-interceptor-theo-mixin-ba-loại-scope-và-cổng-bypass-adr-0089) |
| `src/backend/pkg/entx` | Mixin theo loại bảng (ADR-0089 §2): `Base` (T: id, tenant_id, created/updated/deleted, version), `TenantAppendOnly` (TA), `TenantOptional` (T\*: tenant_id nullable), `TenantOptionalAppendOnly` (TA\*: append-only, tenant_id nullable), `HybridContent` (H, đổi tên từ `PlatformBase`: tenant_id nullable + `owner_scope`), `GlobalBase` (G: có soft-delete, không tenant), `AppendOnly` (GA); mỗi entity đúng một mixin, CI từ chối schema không có mixin; **interceptor** global filter theo mixin; **hook** soft-delete; **`UnitOfWork`** (write context: tx ent + `set_config('app.tenant_id')` + outbox writer trong tx); **`ReadSession`** (read context: tx `READ ONLY`, scope tenant, được trỏ replica) | §4, §5.1–5.2 |
| `src/backend/pkg/outbox` | `Writer` (trong tx của `UnitOfWork`), `Relay` (→ JetStream), `Consumer` (dedupe `processed_events`, retry, DLQ) | §8 |
| `src/backend/pkg/natsx` | Kết nối, JetStream publish/consume, request/reply cho lệnh realtime, header ngữ cảnh | §8, §9 |
| `src/backend/pkg/httpx` | Server chi, middleware cross-cutting: auth, principal, tenant, `portal_staff`, `featuregate` (ADR-0090, §6), idempotency (`Idempotency-Key`), Problem Details, recover, log, tracing; validator request sinh từ OpenAPI | §4.1; thay pipeline behavior của mediator cũ |
| `src/backend/pkg/idempotency` | Store `infra.idempotency_records` (kiểm UUIDv7, tính `key_issued_on`, `INSERT` trong tx của `UnitOfWork`, đọc response để replay), dùng bởi middleware idempotency của `src/backend/pkg/httpx` | [API_DESIGN §6](API_DESIGN.md#6-idempotency) |
| `src/backend/pkg/modcall` | Helper gọi facade module khác **trong tiến trình**: mở span `<caller>→<callee>.<operationId>`, đặt deadline theo loại thao tác, kiểm tra và truyền `TenantScope`, Principal gốc (on-behalf), `Idempotency-Key` và module gọi qua `context.Context`, ánh xạ lỗi sang `problem`, metric. **Không** HTTP, token, retry hay circuit breaker (ADR-0074 mục 3) | §7 |
| `src/backend/pkg/module` [Confirmed] | Kiểu `module.Deps` (pool DB theo module, Valkey, NATS, config, logger) và `module.Registry` mà `Register(...)` của mỗi module dùng để gắn route, handler NATS, consumer, job và facade | §3.3 |
| `src/backend/pkg/problem` | RFC 9457, catalog mã lỗi chung | §10 |
| `src/backend/pkg/observability` | slog/OTel/Prometheus; helper `observability.UseCase(ctx, "<module>.command.<Name>")` mở span + metric + log kết quả cho từng use case | §4.1, §11 |
| `src/backend/pkg/config`, `src/backend/pkg/health`, `src/backend/pkg/clock`, `src/backend/pkg/ids` | env config, health registry, Clock, UUIDv7 | |

Tích hợp **bên ngoài** (Keycloak token endpoint và Admin API, SIS, object storage, SMTP) không dùng `src/backend/pkg/modcall`: mỗi tích hợp có adapter riêng trong `infrastructure` của module sở hữu, bọc timeout, retry chỉ cho thao tác idempotent, circuit breaker và bulkhead bằng `failsafe-go` ([API_DESIGN §12.3](API_DESIGN.md#123-resilience-mặc-định-confirmed)).

Không có `src/backend/pkg/mediator`, bus hay dispatcher dùng chung (ADR-0087). Khi một nhóm handler lặp lại cùng phần cross-cutting, được viết decorator có kiểu (generic) cho handler đó; decorator không phải bus và không dispatch theo `any`.

Valkey: mọi key có prefix `ew:<module>:` ([VALKEY_DESIGN §1](VALKEY_DESIGN.md)); key builder trong `src/backend/pkg/` tự thêm prefix theo module sở hữu key.

`src/backend/pkg/` không chứa logic nghiệp vụ và không import `src/backend/modules/*`. Thay đổi `src/backend/pkg/` là thay đổi chung, cần review của Backend Architect và chạy test mọi module.

## 3. Cấu trúc repository và module (Clean Architecture)

Backend là **một Go module** ở `src/backend/` (`src/backend/go.mod` duy nhất, module path `github.com/tanguyen2020/eduworld-platform/src/backend`; ADR-0074 mục 1, [ADR-0092](../adr/ADR-0092-to-chuc-ma-nguon-trong-src.md)). Lệnh Go chạy trong `src/backend` (`go -C src/backend …` hoặc qua `make`). Mã chia theo module; chỉ composition root ở `src/backend/cmd/` được nối mọi module.

```text
src/backend/
├── go.mod  go.sum                      # module github.com/tanguyen2020/eduworld-platform/src/backend — go.mod duy nhất
├── cmd/                                # composition root của từng binary (§3.3)
│   ├── core-api/main.go                # nạp identity, learning, game, competition, chat: REST public /api/v1/*,
│   │                                   # route gateway nội bộ (game-admin, admin, sis), handler NATS ew.cmd.<module>.>
│   ├── realtime/main.go                # nạp realtime: WebSocket /realtime/v1, ticket, presence, định tuyến lệnh (room host ở module competition trong core-api)
│   ├── worker/main.go                  # --profile=events|chat|jobs (local: all): outbox relay, consumer, job định kỳ
│   └── migrate/main.go                 # job Atlas `migrate --module=<module>` trước rollout (không phục vụ traffic)
├── modules/
│   ├── identity/                       # module domain · eduworld_identity
│   ├── learning/                       # module domain · eduworld_learning
│   ├── game/                           # module domain · eduworld_game
│   ├── competition/                    # module domain · eduworld_competition
│   ├── chat/                           # module domain · eduworld_chat
│   └── realtime/                       # không database (chỉ Valkey, NATS)
├── pkg/                                # platform kit (§2); không import modules/*
├── tools/                              # công cụ Go của repo: archgen, entlint, sqlguard, allowedcallers (sinh luật import chéo từ x-allowed-callers)
└── test/                               # integration test không thuộc module nào
```

Cấu trúc một module (ví dụ `src/backend/modules/learning/`; mọi module cùng khuôn, `realtime` không có `ent/`, `migrations/`):

```text
src/backend/modules/learning/
├── module.go                           # Register(...): nối port ↔ adapter, gắn route/handler/consumer/job, trả facade (§3.3)
├── api/                                # FACADE CÔNG KHAI — package duy nhất module khác được import (ADR-0074 mục 1)
│   └── facade.gen.go                   # make gen: kiểu DTO + interface strict-server sinh từ
│                                       # specs/openapi/modules/learning.internal.v1.yaml (không sửa tay)
├── internal/                           # Go chặn mọi import từ ngoài src/backend/modules/learning/
│   ├── domain/                         # TẦNG DOMAIN — thuần, không I/O
│   │   ├── attempt/                    # aggregate Attempt: entity, value object, rule, domain event
│   │   ├── question/
│   │   └── mastery/
│   ├── application/                    # TẦNG APPLICATION
│   │   ├── command/                    # WRITE CONTEXT: StartAttempt, SubmitAnswer, SubmitAttempt, PublishQuestionVersion...
│   │   ├── query/                      # READ CONTEXT: GetAttempt, ListAssignments, SchoolDashboard...
│   │   ├── port/
│   │   │   ├── write/                  # AttemptRepository (aggregate), UnitOfWork, EventPublisher — chỉ command dùng
│   │   │   ├── read/                   # DashboardReadModel, AttemptView... — chỉ query dùng
│   │   │   └── (gốc)                   # port dùng chung: Identity (facade module identity), Clock
│   │   └── dto/                        # input/output có kiểu + Validate()
│   ├── infrastructure/                 # TẦNG INFRASTRUCTURE
│   │   ├── persistence/
│   │   │   ├── write/                  # ent repository aggregate trên primary DB, dùng entx.UnitOfWork
│   │   │   └── read/                   # read model: ent query/SQL tường minh, entx.ReadSession (replica), cache Valkey
│   │   ├── modules/                    # adapter gọi facade module khác: identity (src/backend/modules/identity/api + src/backend/pkg/modcall)
│   │   ├── messaging/                  # outbox writer wiring, JetStream consumer adapter
│   │   ├── storage/                    # adapter S3 (AWS SDK for Go v2: s3 + s3 presign), endpoint EW_S3_ENDPOINT,
│   │   │                               # path-style EW_S3_FORCE_PATH_STYLE; floci ở local/CI, S3-compatible ở
│   │   │                               # staging/production (ADR-0044, ADR-0075); tích hợp ngoài nên có resilience (§7.3)
│   │   └── cache/                      # Valkey
│   └── presentation/                   # TẦNG PRESENTATION — gọi handler qua interface có kiểu, không mediator
│       ├── http/                       # handler public; code sinh từ specs/openapi/modules/learning.v1.yaml ở http/gen/
│       ├── facade/                     # hiện thực api.StrictServerInterface → gọi command/query handler (§7.1)
│       ├── natscmd/                    # handler NATS request/reply ew.cmd.learning.* (khi module có lệnh realtime)
│       ├── consumer/                   # handler event NATS → command handler
│       └── job/                        # job theo lịch → command handler
├── ent/
│   ├── schema/                         # entity schema (code-first), dùng src/backend/pkg/entx
│   └── (generated)
├── migrations/                         # Atlas versioned migrations (SQL review trong MR)
└── tests/{unit,integration,facade}/
```

| Tầng | Được import | Cấm import |
|---|---|---|
| `api` (facade) | stdlib, `uuid`, runtime của `oapi-codegen` | `internal/*` của chính module, mọi package của module khác, ent, pgx, nats, valkey |
| `domain` | stdlib, `uuid`, `domain` cùng module | ent, pgx, net/http, nats, valkey, `application`, `infrastructure`, `presentation`, `src/backend/pkg/*` trừ `src/backend/pkg/clock`/`src/backend/pkg/ids` |
| `application/command` (write) | `domain`, `application/port`, `application/port/write`, `application/dto`, `src/backend/pkg/authz`, `src/backend/pkg/problem`, `src/backend/pkg/observability`, `src/backend/pkg/clock` | ent, pgx, net/http, nats, valkey, `application/query`, `application/port/read`, `infrastructure`, `presentation`, `src/backend/modules/<khác>/api` (chỉ qua port) |
| `application/query` (read) | `domain` (chỉ kiểu giá trị), `application/port`, `application/port/read`, `application/dto`, `src/backend/pkg/authz`, `src/backend/pkg/problem`, `src/backend/pkg/observability` | `application/port/write`, `src/backend/pkg/entx` (`UnitOfWork`), `src/backend/pkg/outbox` (publisher), `application/command`, ent, pgx, net/http, nats, valkey, `infrastructure`, `presentation`, `src/backend/modules/<khác>/api` |
| `infrastructure/persistence/write` | `application/port`, `application/port/write`, `domain`, ent, `src/backend/pkg/entx`, `src/backend/pkg/outbox` | `application/port/read`, `infrastructure/persistence/read`, `presentation` |
| `infrastructure/persistence/read` | `application/port/read`, `application/dto`, ent, pgx, `src/backend/pkg/entx` (`ReadSession`), valkey | `application/port/write`, `src/backend/pkg/outbox`, `infrastructure/persistence/write`, `presentation` |
| `infrastructure/modules` | `application/port`, `application/dto`, `src/backend/pkg/modcall`, `src/backend/modules/<callee>/api` khi module này có trong `x-allowed-callers` | `src/backend/modules/<callee>/internal/*`, `presentation` |
| `infrastructure` (khác) | `application/port`, `domain`, nats, valkey, SDK tích hợp ngoài + `failsafe-go` | `presentation`, `src/backend/modules/<khác>/*` |
| `presentation` | `application` (command/query/dto), `src/backend/pkg/httpx`, code sinh `http/gen`, `api` của chính module (`presentation/facade`) | `infrastructure`, `src/backend/modules/<khác>/*` |
| `module.go` | mọi tầng của chính module, `src/backend/pkg/module`, `src/backend/modules/<callee>/api` (kiểu facade trong `Imports`) | `src/backend/modules/<khác>/internal/*` |
| `src/backend/cmd/*` | `module.go` và `api` của mọi module, `src/backend/pkg/*` (composition root) | `src/backend/modules/*/internal/*` (Go đã chặn) |

### 3.1. Phân bổ tầng theo module

| Module | Aggregate chính (domain) | Command tiêu biểu | Query tiêu biểu | Việc nền (profile worker) |
|---|---|---|---|---|
| identity | Tenant, User, ExternalIdentity, Enrollment, Role, UserRole, SyncRun | BootstrapSession, ProvisionStudent, CreateRole, GrantUserRole, RevokeUserRole, RunSisSync, CleanupStudents, StartTenantClone | GetPrincipal, ListTenants, ListStudents, TrashList | `events`: outbox relay, Keycloak membership, audit sink · `jobs`: SIS sync, student cleanup, saga platform |
| learning | QuestionVersion, Attempt, ExamSession, Assignment, ObjectiveMastery | StartAttempt, SubmitAnswer, SubmitAttempt, CreateAssignment, PublishContent, CreateQuestionPlan, ScoreAnswer | GetAttempt, SearchQuestions, SchoolDashboard, MasteryReport | `events`: outbox relay, mastery updater, rollup dashboard, read model login activity · `jobs`: chốt ngày dashboard, auto-submit đề hết giờ |
| game | GameProfile, QuestProgress, GameSession, XPLedger, GameConfigVersion | AcceptQuest, ReportStep, StartGameSession (takeover), GrantReward, PublishGameConfig | GetWorldManifest, GetQuestLog, LoginEvents | `events`: outbox relay, quest/XP consumers, analytics · `jobs`: session timeout |
| competition | Party, Room, Match, Rating, Tournament | CreateParty, InviteMember, StartPartySession, EnqueueTicket, SubmitBattleAnswer, FinalizeMatch | GetParty, Leaderboard, Bracket | `events`: outbox relay, rating updater · `jobs`: matchmaker loop, leaderboard snapshot, tournament scheduler |
| chat | Channel, Message, ModerationCase, SensitiveCatalog, Friendship | SendMessage, EditMessage, RecallMessage, ToggleModerationSwitch, ResolveCase | History, Search, Unread, CaseQueue | `events`: outbox relay · `chat`: chat-persister, notifier, moderation-async, stats, channel-closer, gameplay purge (§9.1) |
| realtime | (không có domain nghiệp vụ) | — | — | — (binary `realtime`: connection hub, router NATS, subscriber Valkey) |

Danh sách đầy đủ consumer và job theo Swarm service: [DEPLOYMENT §3.1](../11-operations/DEPLOYMENT.md).

### 3.2. Cưỡng chế ranh giới: compiler, depguard, go-arch-lint

Ba lớp chặn (ADR-0074 mục 1), vi phạm chặn merge:

1. **Compiler:** mọi thứ của module nằm dưới `src/backend/modules/<module>/internal/`, nên module khác không import được.
2. **depguard** (`src/backend/.golangci.yml`): luật tầng trong module (bảng trên) và luật import chéo. Phần import chéo do `src/backend/tools/allowedcallers` **sinh** từ `x-allowed-callers` của spec internal: module chỉ được import `src/backend/modules/<callee>/api` (và chỉ từ `infrastructure/modules/` hoặc `module.go`) khi có tên trong `x-allowed-callers` của ít nhất một operation của callee.
3. **`make arch-check`:** analyzer của `src/backend/tools/allowedcallers` kiểm từng lời gọi phương thức facade khớp `x-allowed-callers` của **đúng operation đó**; `go-arch-lint` kiểm ma trận tầng. Lúc chạy, `src/backend/pkg/modcall` kiểm lại và trả `403 CALLER_NOT_ALLOWED` (lớp phòng thủ cuối, không thay linter).

```yaml
# src/backend/.golangci.yml (trích; khối "learning-*" có tương ứng cho từng module)
linters-settings:
  depguard:
    rules:
      learning-domain:
        files: ["**/modules/learning/internal/domain/**"]
        allow:
          - $gostd
          - github.com/google/uuid
          - github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain
          - github.com/tanguyen2020/eduworld-platform/src/backend/pkg/clock
          - github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids
        deny:
          - { pkg: "entgo.io/ent", desc: "domain không biết ORM" }
          - { pkg: "github.com/jackc/pgx", desc: "domain không biết DB" }
          - { pkg: "net/http", desc: "domain không biết transport" }
          - { pkg: "github.com/nats-io", desc: "domain không biết messaging" }
          - { pkg: "github.com/tanguyen2020/eduworld-platform/src/backend/modules", desc: "phụ thuộc hướng vào trong; không biết module khác" }
      learning-application:
        files: ["**/modules/learning/internal/application/**"]
        deny:
          - { pkg: "entgo.io/ent" }
          - { pkg: "github.com/jackc/pgx" }
          - { pkg: "net/http" }
          - { pkg: "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/entx", desc: "application chỉ biết port UnitOfWork" }
          - { pkg: "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox", desc: "application chỉ biết port EventPublisher" }
          - { pkg: "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/modcall", desc: "gọi module khác qua port; adapter ở infrastructure/modules" }
          - { pkg: "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure" }
          - { pkg: "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/presentation" }
      learning-query: # read context (ADR-0087)
        files: ["**/modules/learning/internal/application/query/**"]
        deny:
          - { pkg: "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write", desc: "query không dùng repository ghi, UnitOfWork, publisher" }
          - { pkg: "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command", desc: "query không gọi command" }
      learning-command: # write context
        files: ["**/modules/learning/internal/application/command/**"]
        deny:
          - { pkg: "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query", desc: "command không phụ thuộc read model" }
          - { pkg: "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/read" }
      learning-persistence-read:
        files: ["**/modules/learning/internal/infrastructure/persistence/read/**"]
        deny:
          - { pkg: "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write" }
          - { pkg: "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/outbox", desc: "read model không phát event" }
          - { pkg: "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write" }
      learning-presentation:
        files: ["**/modules/learning/internal/presentation/**"]
        deny:
          - { pkg: "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure", desc: "presentation chỉ gọi application" }
          - { pkg: "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/mediator", desc: "không mediator (ADR-0087)" }
      # --- sinh bởi src/backend/tools/allowedcallers từ x-allowed-callers (không sửa tay) ---
      learning-cross-module:
        files: ["**/modules/learning/**", "!**/modules/learning/internal/infrastructure/modules/**", "!**/modules/learning/module.go"]
        deny:
          - { pkg: "github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity", desc: "chỉ infrastructure/modules/ và module.go được dùng facade (R29)" }
          - { pkg: "github.com/tanguyen2020/eduworld-platform/src/backend/modules/game" }
          - { pkg: "github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition" }
          - { pkg: "github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat" }
          - { pkg: "github.com/tanguyen2020/eduworld-platform/src/backend/modules/realtime" }
      learning-facade-allowlist:
        files: ["**/modules/learning/internal/infrastructure/modules/**", "**/modules/learning/module.go"]
        allow:
          - $gostd
          - github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/api   # learning ∈ x-allowed-callers của resolvePrincipal, getTenant, checkMemberships
          - github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning
          - github.com/tanguyen2020/eduworld-platform/src/backend/pkg
        deny:
          - { pkg: "github.com/tanguyen2020/eduworld-platform/src/backend/modules", desc: "module không có trong x-allowed-callers của callee" }
```

```yaml
# src/backend/modules/<module>/.go-arch-lint.yml (trích) — ma trận tầng trong một module
version: 3
workdir: internal
components:
  domain:          { in: domain/** }
  command:         { in: application/command/** }
  query:           { in: application/query/** }
  port:            { in: application/port }
  port-write:      { in: application/port/write }
  port-read:       { in: application/port/read }
  dto:             { in: application/dto }
  persist-write:   { in: infrastructure/persistence/write/** }
  persist-read:    { in: infrastructure/persistence/read/** }
  infra-modules:   { in: infrastructure/modules/** }
  infra-other:     { in: [infrastructure/messaging/**, infrastructure/storage/**, infrastructure/cache/**] }
  presentation:    { in: presentation/** }
deps:
  domain:        { mayDependOn: [] }
  command:       { mayDependOn: [domain, port, port-write, dto] }
  query:         { mayDependOn: [domain, port, port-read, dto] }            # không port-write → không UnitOfWork/publisher
  port-write:    { mayDependOn: [domain] }
  port-read:     { mayDependOn: [dto] }
  persist-write: { mayDependOn: [domain, port, port-write] }
  persist-read:  { mayDependOn: [port-read, dto] }
  infra-modules: { mayDependOn: [port, dto] }                               # + src/backend/modules/<callee>/api theo depguard
  infra-other:   { mayDependOn: [domain, port, dto] }
  presentation:  { mayDependOn: [command, query, dto] }
```

### 3.3. Composition root, đăng ký module và ba binary

Mỗi module có `module.go` với hàm `Register` (ADR-0074 mục 1). `Register` dựng adapter, nối port, gắn mọi điểm vào của module vào registry của binary đang chạy và trả về đối tượng module có `Facade()`. Facade của module khác được nối sau bằng `Wire`: `identity` gọi module khác trong saga platform còn module khác gọi `identity`, nên phụ thuộc vòng ở mức đối tượng; không có vòng import vì module chỉ import package `api` của nhau.

```go
// src/backend/modules/learning/module.go
package learning

// Imports liệt kê đúng các facade learning được gọi (khớp x-allowed-callers; depguard kiểm).
type Imports struct {
    Identity identityapi.StrictServerInterface
}

func Register(r *module.Registry) (*Module, error) {
    db, err := r.Database("learning") // pool eduworld_learning: learning_app, learning_platform, replica cho read context
    if err != nil {
        return nil, fmt.Errorf("learning: open database: %w", err)
    }
    m := &Module{identity: modules.NewIdentity()} // adapter infrastructure/modules; facade thật gắn ở Wire
    uow := entx.NewUnitOfWork(db.App, db.Platform, outbox.NewWriter())
    submit := command.SubmitAttemptHandler{UoW: uow, Attempts: write.NewAttemptRepo(uow), Events: uow.Publisher(), Scorer: scoring.New(), Clock: clock.System()}
    dashboard := query.SchoolDashboardHandler{ReadModel: read.NewDashboardReadModel(entx.NewReadSession(db.Replica), r.Valkey())}
    m.facade = facade.New(submit /*, createAttempt, scorePlanAnswer… */) // hiện thực api.StrictServerInterface

    r.HTTP(http.Routes(http.AttemptHandlers{Submit: submit, Dashboard: dashboard})) // chỉ hiệu lực ở core-api
    r.Relay("learning", db.Relay)                                                   // profile events
    r.Consumer("learning.rollup_dashboard", consumer.RollupDashboard(uow))          // profile events
    r.Job("dashboard-day-close", job.DashboardDayClose(uow))                        // profile jobs (khóa leader)
    return m, nil
}

func (m *Module) Facade() api.StrictServerInterface { return m.facade }
func (m *Module) Wire(in Imports)                   { m.identity.Set(in.Identity) }
```

```go
// src/backend/cmd/core-api/main.go — composition root; src/backend/cmd/worker giống hệt, chỉ khác vai trò bật trên registry
func main() {
    ctx := signalContext()
    deps := module.MustLoadDeps(ctx, "core-api") // config, pool theo database module, Valkey, NATS, OTel
    reg := module.NewRegistry(deps, module.ServeHTTP, module.ServeNATSCommands)

    idm := must(identity.Register(reg))
    lrn := must(learning.Register(reg))
    gam := must(game.Register(reg))
    cmp := must(competition.Register(reg))
    cht := must(chat.Register(reg))

    idm.Wire(identity.Imports{Learning: lrn.Facade(), Game: gam.Facade(), Competition: cmp.Facade(), Chat: cht.Facade()}) // saga platform ops
    lrn.Wire(learning.Imports{Identity: idm.Facade()})
    gam.Wire(game.Imports{Identity: idm.Facade(), Learning: lrn.Facade()})
    cmp.Wire(competition.Imports{Identity: idm.Facade(), Learning: lrn.Facade()})
    cht.Wire(chat.Imports{Identity: idm.Facade()})

    reg.MustRun(ctx) // từ chối khởi động nếu còn facade chưa Wire; chạy HTTP/NATS; graceful shutdown (§13)
}
```

| Binary | Vai trò bật trên registry | Module nạp | Ghi chú |
|---|---|---|---|
| `src/backend/cmd/core-api` | `ServeHTTP` (REST public + route gateway nội bộ trên một HTTP server, chuỗi middleware `src/backend/pkg/httpx` dùng chung), `ServeNATSCommands` (`ew.cmd.<module>.>`) | 5 module domain | Không chạy relay, consumer, job |
| `src/backend/cmd/realtime` | WebSocket hub, router NATS, subscriber Valkey | `realtime` | Không mở pool database; không gọi facade module domain trong tiến trình, mọi lệnh nghiệp vụ đi qua NATS request/reply tới `core-api` (§9) |
| `src/backend/cmd/worker --profile=events` | `Relay`, `Consumer` (domain event), media pipeline (`src/backend/pkg/media`) | 5 module domain | |
| `src/backend/cmd/worker --profile=chat` | `Consumer` của stream `CHAT_MESSAGES`, job purge | `chat` và module mà các consumer đó gọi facade | Role `chat_purger` chỉ có ở profile này (ADR-0033) |
| `src/backend/cmd/worker --profile=jobs` | `Job` (khóa leader) | 5 module domain | Saga platform, SIS sync, cleanup, `infra-partition-maintenance` |
| `src/backend/cmd/worker projection rebuild --module=<module> --source=<module> [--table=ref.<bảng>]` | Lệnh một lần (vận hành, khôi phục): dựng lại projection `ref.*` qua facade phân trang của module nguồn, upsert theo `source_version` (F003-AC-39) | module nhận + module nguồn | Dùng ở [BACKUP_RECOVERY](../11-operations/BACKUP_RECOVERY.md) §3.5 |

Quy tắc: binary nạp module nào thì phải nạp cả các module mà điểm vào đang bật của nó gọi facade (registry kiểm lúc khởi động). Mỗi module mở pool riêng tới database của mình bằng role của module đó; không dùng chung pool giữa module.

## 4. CQRS (ADR-0038, ADR-0087)

### 4.1. Write context, read context và cross-cutting

CQRS **không dùng mediator**: không có bus, registry hay dispatch theo `any`. Mỗi use case là một kiểu Go có phụ thuộc tường minh; presentation giữ interface có kiểu của handler và gọi trực tiếp.

| | Write context (command) | Read context (query) |
|---|---|---|
| Package handler | `internal/application/command/` | `internal/application/query/` |
| Port | `internal/application/port/write/` (repository aggregate, `UnitOfWork`, `EventPublisher`) | `internal/application/port/read/` (read model/projection) |
| Persistence | `internal/infrastructure/persistence/write/` — ent trên primary DB, `entx.UnitOfWork` | `internal/infrastructure/persistence/read/` — ent query/SQL tường minh, `entx.ReadSession` (được trỏ replica), cache Valkey |
| Được làm | Nạp aggregate → gọi phương thức domain → lưu trong transaction → ghi outbox cùng transaction → trả ID/version | Đọc, ghép DTO cho màn hình |
| Không được làm | Trả read model nặng; gọi facade module khác trong transaction | Ghi DB, phát event, dùng `UnitOfWork`/publisher (linter chặn, §3) |

Cross-cutting (thay pipeline behavior của mediator cũ):

| Mối quan tâm | Nơi đặt | Command | Query |
|---|---|---|---|
| Xác thực, Principal, tenant (`TenantScope` vào context) | Middleware `src/backend/pkg/httpx` (`auth`, `principal`, `tenant`); adapter NATS cho lệnh realtime (§9); `outbox.Consumer` cho event (§8); `src/backend/pkg/modcall` cho lời gọi facade (§7) | ✓ | ✓ |
| Idempotency (`Idempotency-Key`) | Middleware `httpx.Idempotency` kiểm key UUIDv7 + replay response đã lưu; bản ghi `infra.idempotency_records` được `INSERT` trong cùng `UnitOfWork` với thay đổi nghiệp vụ (store `src/backend/pkg/idempotency`, ADR-0089 §4, [API_DESIGN §6](API_DESIGN.md#6-idempotency)); lớp cuối là unique constraint nghiệp vụ trong DB | ✓ (operation có cột Idem) | — |
| Bật/tắt menu/action portal (ADR-0090) | Middleware `featuregate` (`src/backend/pkg/httpx`) sau `auth` → `principal` → `tenant` → `portal_staff`, **trước** handler (và do đó trước `authz.Require`); chi tiết §6 | ✓ | ✓ |
| Problem Details, recover | Middleware `src/backend/pkg/httpx` + `problem.Map(err)` (§10) | ✓ | ✓ |
| Log truy cập, trace HTTP | Middleware `src/backend/pkg/httpx` (otelhttp, slog) | ✓ | ✓ |
| Span + metric theo use case | `observability.UseCase(ctx, "<module>.command.<Name>")` / `"<module>.query.<Name>"` ở đầu handler | ✓ | ✓ |
| Validation | Validator request sinh từ OpenAPI (`maxLength`, enum) ở `src/backend/pkg/httpx` + `Validate()` của DTO gọi trong handler + invariant domain | ✓ | ✓ |
| Authorization | Handler gọi **tường minh** `authz.Require(ctx, p, perm, resource)` ở đầu use case (resource nạp từ DB khi cần, [MULTITENANCY §4.1](../06-architecture/MULTITENANCY.md#41-principal-và-tenantscope)) | ✓ | ✓ |
| Transaction + outbox | `UnitOfWork.Do` (`src/backend/pkg/entx`) chỉ trong command handler | ✓ | — |
| Cache | Adapter `persistence/read` (Valkey `ew:<module>:q:<key>`) | — | tùy chọn |

Vì authorization và validation là lời gọi tường minh, mỗi handler mới **bắt buộc** có test authorization (thiếu permission, ngoài tenant → 404, ngoài phạm vi school/subject) và test validation; test sinh từ catalog (§15) phát hiện route chưa kiểm quyền. Khi nhiều handler lặp cùng đoạn cross-cutting, được viết decorator generic có kiểu bọc **một** handler (ví dụ `observability.WrapCommand[C, R]`), không phải bus chung.

```go
// src/backend/modules/learning/internal/application/port/write/uow.go — port của write context
type UnitOfWork interface {
    // Do mở transaction trên primary DB (set_config app.tenant_id theo scope trong ctx),
    // gắn tx vào ctx cho repository và EventPublisher; commit khi fn trả nil, rollback khi lỗi.
    Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type AttemptRepository interface {
    Get(ctx context.Context, id uuid.UUID) (*attempt.Attempt, error)
    Save(ctx context.Context, a *attempt.Attempt) error // optimistic concurrency theo version
}

type EventPublisher interface {
    Publish(ctx context.Context, events ...domain.Event) error // ghi infra.outbox_events trong tx của ctx
}
```

### 4.2. Command (ghi) — mẫu

```go
// src/backend/modules/learning/internal/application/command/submit_attempt.go
package command

type SubmitAttemptInput struct {
    AttemptID uuid.UUID
}

func (in SubmitAttemptInput) Validate() error {
    if in.AttemptID == uuid.Nil {
        return problem.Invalid("attempt_id", "REQUIRED")
    }
    return nil
}

// Interface có kiểu mà presentation phụ thuộc.
type SubmitAttempt interface {
    Handle(ctx context.Context, in SubmitAttemptInput) (dto.AttemptResult, error)
}

type SubmitAttemptHandler struct {
    UoW      write.UnitOfWork        // entx.UnitOfWork qua port
    Attempts write.AttemptRepository // ent adapter, persistence/write
    Events   write.EventPublisher    // outbox writer (cùng tx)
    Scorer   port.Scorer             // domain service, chạy trong module learning
    Clock    clock.Clock
}

func (h SubmitAttemptHandler) Handle(ctx context.Context, in SubmitAttemptInput) (res dto.AttemptResult, err error) {
    ctx, end := observability.UseCase(ctx, "learning.command.SubmitAttempt")
    defer func() { end(err) }()

    if err = in.Validate(); err != nil {
        return res, err
    }
    p := authz.MustPrincipal(ctx)
    if err = authz.Require(ctx, p, authz.PermAttemptSubmitOwn, authz.Resource{TenantID: p.TenantID}); err != nil {
        return res, err
    }

    err = h.UoW.Do(ctx, func(ctx context.Context) error {
        a, err := h.Attempts.Get(ctx, in.AttemptID) // interceptor đã thêm tenant_id + deleted_at IS NULL
        if err != nil {
            return err
        }
        if a.StudentID() != p.UserID {
            return problem.NotFound("attempt") // ngoài phạm vi → 404 (chống dò ID)
        }
        if err := a.Submit(h.Clock.Now()); err != nil { // quy tắc nghiệp vụ trong aggregate
            return err
        }
        score := h.Scorer.Score(a)
        if err := h.Attempts.Save(ctx, a); err != nil {
            return err
        }
        if err := h.Events.Publish(ctx, a.PullEvents()...); err != nil { // activity.submitted... vào outbox, cùng tx
            return err
        }
        res = dto.FromAttempt(a, score)
        return nil
    })
    if err != nil {
        return dto.AttemptResult{}, fmt.Errorf("submit attempt %s: %w", in.AttemptID, err)
    }
    return res, nil
}
```

### 4.3. Query (đọc) — mẫu

```go
// src/backend/modules/learning/internal/application/query/school_dashboard.go
// School Learning Dashboard (ADR-0072): school_admin / academic_manager, trường → lớp → học sinh.
package query

type SchoolDashboardInput struct {
    SchoolID  uuid.UUID
    ClassID   *uuid.UUID // nil = cấp trường; có giá trị = drill-down cấp lớp
    SubjectID *uuid.UUID // academic_manager scope subject chỉ được môn trong Principal.Scopes
    Period    string     // ≤ 16, "7d" | "30d" | "term"
}

func (in SchoolDashboardInput) Validate() error {
    if len(in.Period) > 16 {
        return problem.MaxLength("period", 16)
    }
    return nil
}

type SchoolDashboard interface {
    Handle(ctx context.Context, in SchoolDashboardInput) (dto.Dashboard, error)
}

type SchoolDashboardHandler struct {
    ReadModel read.DashboardReadModel // persistence/read: SQL tường minh trên bảng rollup, ReadSession (replica)
}

func (h SchoolDashboardHandler) Handle(ctx context.Context, in SchoolDashboardInput) (out dto.Dashboard, err error) {
    ctx, end := observability.UseCase(ctx, "learning.query.SchoolDashboard")
    defer func() { end(err) }()

    if err = in.Validate(); err != nil {
        return out, err
    }
    p := authz.MustPrincipal(ctx)
    res := authz.Resource{TenantID: p.TenantID, SchoolID: &in.SchoolID, SubjectID: in.SubjectID}
    if err = authz.Require(ctx, p, authz.PermAnalyticsSchoolRead, res); err != nil {
        return out, err
    }
    // Không nạp aggregate, không UnitOfWork, không publisher (package query không import port/write).
    return h.ReadModel.SchoolSummary(ctx, in.SchoolID, in.ClassID, in.SubjectID, in.Period)
}
```

### 4.4. Presentation và composition root

```go
// src/backend/modules/learning/internal/presentation/http/attempts.go
type AttemptHandlers struct {
    Submit    command.SubmitAttempt // interface có kiểu, không mediator
    Dashboard query.SchoolDashboard
}

func (h AttemptHandlers) SubmitAttempt(w http.ResponseWriter, r *http.Request, id gen.AttemptId) {
    // httpx đã chạy: auth → principal → tenant → idempotency → validator OpenAPI → recover/log/trace
    res, err := h.Submit.Handle(r.Context(), command.SubmitAttemptInput{AttemptID: id})
    if err != nil {
        httpx.WriteProblem(w, r, err) // problem.Map → RFC 9457
        return
    }
    httpx.WriteJSON(w, http.StatusOK, toAPI(res))
}

```

Việc nối port với adapter (`UnitOfWork`, repository, read model, facade module khác) nằm trong `module.go` của module (mẫu ở §3.3); `src/backend/cmd/*` chỉ gọi `Register` và `Wire`.

Consumer NATS (§8), job theo lịch và facade (§7.1) gọi command/query handler theo cùng cách, sau khi dựng `Principal`/`TenantScope` từ envelope, cấu hình job hoặc context do `src/backend/pkg/modcall` gắn.

Quy tắc CQRS:

1. Command không trả read model nặng (chỉ ID, version, kết quả cần thiết cho UI tức thời như kết quả chấm).
2. Query không thay đổi trạng thái, không ghi outbox, không dùng `UnitOfWork` (package `query` không import `port/write`; linter chặn, §3).
3. Không có bus/mediator: thêm use case = thêm kiểu handler + interface + nối trong `module.go`. `module.Registry` chỉ gắn điểm vào (route, handler NATS, consumer, job, facade) cho binary, không dispatch command/query theo kiểu `any`.
4. Read model cập nhật **đồng bộ** trong cùng transaction khi đơn giản (ví dụ bảng tóm tắt attempt), hoặc **bất đồng bộ** qua consumer (dashboard, leaderboard, analytics). Màn hình dùng read model bất đồng bộ ghi rõ độ trễ chấp nhận được (thường ≤ 5 s).
5. Không event sourcing.

## 5. Persistence: ent + Atlas (tương đương Entity Framework)

### 5.1. Interceptor global filter + soft-delete

```go
// src/backend/pkg/entx/interceptor.go
// Tương đương HasQueryFilter của EF Core: điều kiện thêm vào phụ thuộc mixin của entity (ADR-0089 §2).
// Mixin Base dùng interceptor dưới đây; TenantAppendOnly bỏ deleted_at; TenantOptional/TenantOptionalAppendOnly/HybridContent
// có nhánh tenant_id IS NULL (chỉ PlatformScope / được gán); GlobalBase chỉ deleted_at; AppendOnly không filter.
func TenantFilterInterceptor() ent.Interceptor {
    return intercept.TraverseFunc(func(ctx context.Context, q intercept.Query) error {
        if tenancy.Bypassed(ctx) { // chỉ bật qua tenancy.Bypass(ctx, reason), reason thuộc danh sách đóng (audit + metric)
            return nil
        }
        scope, ok := authz.ScopeFromContext(ctx)   // TenantScope | CrossTenantScope | PlatformScope
        if !ok {
            return ErrMissingTenantScope // không có scope → lỗi, không trả dữ liệu
        }
        q.WhereP(
            sql.FieldIn("tenant_id", scope.TenantIDs()...), // PlatformScope: tenant_id IS NULL
            sql.FieldIsNull("deleted_at"),                  // RLS không lọc deleted_at (ADR-0089 §1)
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

- Bảng điều kiện interceptor đầy đủ theo mixin và ba loại scope: [MULTITENANCY §4.4](../06-architecture/MULTITENANCY.md#44-quy-tắc-interceptor-theo-mixin-ba-loại-scope-và-cổng-bypass-adr-0089). Entity mixin `GlobalBase`/`AppendOnly` không cần scope; mọi mixin khác thiếu scope → `ErrMissingTenantScope`.
- Truy cập không filter chỉ qua `tenancy.Bypass(ctx, reason)` với `reason` thuộc danh sách đóng của ADR-0089 §3, mỗi reason gắn một quyền DB (bảng ở [MULTITENANCY §4.4](../06-architecture/MULTITENANCY.md#44-quy-tắc-interceptor-theo-mixin-ba-loại-scope-và-cổng-bypass-adr-0089)); mỗi lần gọi ghi audit `cross_tenant.accessed` (`reason`, `tenant_ids`, `mode`) và tăng `eduworld_tenant_filter_bypass_total{module,reason}`; linter cấm `reason` ngoài danh sách. `auth_login` chỉ dùng ở module identity để gọi hàm `identity.memberships_for_user(user_id)` (`SECURITY DEFINER`, `EXECUTE` cho `identity_app`, không `BYPASSRLS`) trên `/auth/session`, `/auth/game/*` và `login-credentials:verify` (ADR-0089 §3).
- `UnitOfWork` chọn client theo scope: `TenantScope` → `<module>_app`; `CrossTenantScope`, `PlatformScope` và các reason `Bypass` có quyền `<module>_platform` → `<module>_platform`. Dòng `tenant_id IS NULL` của bảng T\*/TA\*/H chỉ ghi được dưới `PlatformScope` (ADR-0089 §2).
- RLS ở database vẫn bật (lớp thứ hai) nhưng **chỉ lọc tenant**; `deleted_at IS NULL` do interceptor bảo đảm vì PostgreSQL buộc dòng mới của `UPDATE` thỏa policy `SELECT`, nên policy có `deleted_at` sẽ chặn soft-delete ([DATABASE_DESIGN §4](DATABASE_DESIGN.md#4-row-level-security)). `entx.UnitOfWork` (write) và `entx.ReadSession` (read, transaction `READ ONLY`) đều mở tx ent và chạy `SET LOCAL app.tenant_id` ([MULTITENANCY §5.2](../06-architecture/MULTITENANCY.md#52-set-ngữ-cảnh-mỗi-transaction)).
- Ngoại lệ hard-delete duy nhất: package `src/backend/modules/chat/internal/infrastructure/purge` dùng DSN của role `chat_purger` (ADR-0033), không đi qua ent hook.

### 5.2. Mixin chung và schema mẫu

```go
// src/backend/pkg/entx/base.go — mixin Base cho bảng T (tenant-scoped có soft-delete).
// Cùng package: TenantAppendOnly (TA), TenantOptional (T*), HybridContent (H, tên cũ PlatformBase),
// GlobalBase (G), AppendOnly (GA). Mỗi entity khai báo đúng một mixin (ADR-0089 §2).
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

// src/backend/modules/competition/ent/schema/party.go (minh họa)
func (Party) Mixin() []ent.Mixin { return []ent.Mixin{entx.Base{}} }

func (Party) Fields() []ent.Field {
    return []ent.Field{
        field.String("name").MaxLen(128).NotEmpty(),                    // ADR-0041: tên ngắn ≤ 128
        field.Enum("party_type").Values("friend", "study_group", "quest", "competition_team", "custom"),
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
- Linter `src/backend/tools/entlint` (chạy trong `make lint`) duyệt `ent/schema` và **fail** nếu có `field.String`/`field.Text` không có `MaxLen` hoặc `field.Text` không có annotation `// maxlen-justified:` + CHECK độ dài.
- Nội dung dài có cấu trúc (render payload, rich text) dùng `field.JSON` + CHECK `octet_length(col::text) <= N` khai báo bằng `entsql.Check`.
- Validation tầng presentation sinh từ OpenAPI (`maxLength`), tầng application kiểm tra lại trong `Validate()`.

### 5.4. Migration (Atlas)

- Migration quản lý bằng Atlas (`src/backend/modules/<module>/migrations/`, `atlas.sum`): `atlas migrate diff --env <module>` sinh phần bảng/cột/index từ ent schema; phần ent không biểu diễn được viết tay trong cùng thư mục (RLS policy, trigger, `GRANT`/`REVOKE`, view `_live`, hàm `SECURITY DEFINER`, DDL partition trong `partitions/`) và được `atlas migrate lint` kiểm tra, review trong MR như migration sinh tự động ([DATABASE_DESIGN §7.1](DATABASE_DESIGN.md#71-migration-viết-tay-adr-0089-6)).
- `atlas migrate lint` trong CI: chặn thao tác phá dữ liệu (DROP COLUMN/TABLE, đổi kiểu thu hẹp) nếu thiếu nhãn `-- atlas:nolint destructive` kèm ADR/ticket.
- Chiến lược expand/contract; không bao giờ auto-migrate khi binary khởi động; job `migrate --module=<module>` (image `eduworld/migrate`, `src/backend/cmd/migrate`) chạy cho từng database module trước rollout ([DEPLOYMENT](../11-operations/DEPLOYMENT.md)).

### 5.5. Repository adapter (mẫu)

```go
// src/backend/modules/learning/internal/infrastructure/persistence/write/attempt_repo.go — hiện thực write.AttemptRepository
type AttemptRepo struct{ uow *entx.UnitOfWork }

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
        return problem.Conflict("CONFLICT") // 409 CONFLICT: version lệch (optimistic concurrency)
    }
    return nil
}
```

## 6. Principal, authorization, tenant

- Chuỗi middleware public: `auth` (verify JWT: `iss`, `aud`, `exp`, chữ ký; JWKS cache) → `principal` → `tenant` → (`game_player` | `portal_staff`) → `featuregate` (chỉ token portal) → handler (handler gọi `authz.Require`).
- **Module identity** là nơi xác định tenant khi đăng nhập (game: `POST /api/v1/auth/game/login` qua backend, ADR-0068; portal: `POST /api/v1/auth/portal/login` qua backend bằng client confidential `eduworld-portal` (ADR-0091) → `GET /api/v1/me/tenants` → chọn tenant (chỉ đổi `X-Tenant-Id`) → `GET /api/v1/me/permissions`, ADR-0026/0063/0067; portal không gọi `/auth/session`) và quản lý Role/UserRole (ADR-0066). Middleware `principal` dùng chung của `core-api` (và adapter NATS, consumer ở `worker`) dựng `Principal` **một lần mỗi request** qua facade identity, operation `resolvePrincipal` (input `{user_id, tenant_id, client_id}`; gọi trong tiến trình qua `src/backend/pkg/modcall`; cache L1 + Valkey 60 s theo `(user_id, tenant_id)`, invalidate bằng event `user_role.changed`, `role.updated`, `tenant_membership.changed`, `tenant.updated`). `src/backend/pkg/httpx` chỉ biết interface `PrincipalResolver`; adapter bọc facade identity được nối ở composition root. Database của module identity lỗi hoặc vượt deadline và không có cache hợp lệ → **503** (`UNAVAILABLE`, retryable); **không fail-open**.
- **Middleware `tenant` (ADR-0065):** đọc header **`X-Tenant-Id` (bắt buộc)** trên mọi route tenant-scoped. Thiếu/sai định dạng → `400 TENANT_HEADER_REQUIRED` (APISIX đã chặn trước, middleware kiểm tra lại). Header phải **khớp** tenant của phiên = claim `organization` đã đối chiếu + membership (`memberships:check`, cache 60 s); lệch → `403 TENANT_MISMATCH` + audit/security event. `Principal.TenantID` lấy từ **kết quả kiểm tra**, không lấy thẳng từ header; repository, ent interceptor, RLS dùng `Principal.TenantID`. Route miễn header: `POST /auth/session`, `GET /game/client-config`, `/livez`, `/readyz`, `GET /me/tenants`. **System Admin:** thao tác ghi bắt buộc `X-Tenant-Id` = tenant đích (phải tồn tại, `status = active`); đọc liên tenant dùng `tenant_ids` (permission `platform.cross_tenant.read`, `azp = eduworld-portal`; không còn điều kiện MFA — ADR-0094) và không gửi header. Người dùng tenant gửi `tenant_ids` → bị bỏ qua.
- Route người chơi (`/game/*` trừ `client-config`, `/multiplayer/*`, `/competition/*` phía người chơi, `/social/*`, ticket realtime) đi qua middleware `game_player`: yêu cầu `azp = eduworld-game-backend` (token cấp qua `/auth/game/login`, ADR-0068) và `user_type = student` (ADR-0031), lỗi `403 GAME_ACCESS_STUDENT_ONLY`. Route portal (`/cms/*`, `/lms/*`, `/tenant/*`, `/schools/*`, `/game-admin/*`, `/admin/*`) đi qua middleware `portal_staff`: `azp = eduworld-portal` và `user_type = staff` (ADR-0066, ADR-0081), lỗi `403 PORTAL_ACCESS_STAFF_ONLY`. `portal_staff` không phân biệt nhân sự platform với nhân sự trường; phân biệt bằng role (role platform `tenant_id = NULL`) và `Principal.TenantID`, không bằng `user_type`.
- **Middleware `featuregate` (ADR-0090)** — bật/tắt menu/action của Admin Portal, **không** phải lớp authorization:
  - Chỉ áp khi token `azp = eduworld-portal` và operation khớp route có `x-portal-action`; bảng route → action key (kể cả dạng định tuyến theo tham số như `config_type`, `content_type`) và cờ `can_disable` được **sinh** bởi `make gen` từ OpenAPI + `specs/portal/menu-catalog.yaml`, không viết tay. Operation `x-portal-shell` (đăng nhập, `/me/*`, danh mục tra cứu) và action `can_disable = false` bỏ qua tra cứu.
  - Tra trạng thái hiệu lực qua port `featuregate.Source` (khai báo trong `src/backend/pkg/httpx`, hiện thực ở module identity đọc database/Valkey của chính nó `ew:fg:<tenant_id|platform>`, TTL `feature_toggle_cache_ttl`, xóa khi nhận `platform_settings.changed` scope `feature_flag`; nối ở `src/backend/cmd/core-api`, không module nào đọc database của identity). Tenant = `Principal.TenantID`; System Admin liên tenant không gửi `X-Tenant-Id` → mặc định platform.
  - Action hoặc menu cha đang tắt → `403 FEATURE_DISABLED` (Problem Details có `feature_key`). Không lấy được trạng thái (Valkey miss và database identity lỗi) → `503 UNAVAILABLE` cho operation bị gate (**fail-closed**); operation không gate vẫn chạy.
  - Chạy sau xác thực/tenant nên request chưa xác thực luôn nhận `401`, và trước handler nên action tắt trả `FEATURE_DISABLED` kể cả khi người gọi thiếu permission; bật lại action không cấp thêm quyền — handler vẫn gọi `authz.Require`. Thứ tự và mã lỗi: [API_DESIGN §11](API_DESIGN.md).
- Permission của `Principal` = hợp permission của mọi **UserRole** còn hiệu lực (`valid_from ≤ now < valid_to`, `deleted_at IS NULL`) trong tenant hiện tại (role platform: phạm vi platform), kèm phạm vi (school/subject) để kiểm tra theo tài nguyên; không có phạm vi lớp theo phân công giảng dạy (ADR-0072). Ngoại lệ đọc liên tenant: `system_admin` (`CrossTenantScope`) và `safety_moderator` chỉ trong module moderation (`tenancy.Bypass(ctx, "moderation_case")`, audit) — [MULTITENANCY §4.1](../06-architecture/MULTITENANCY.md#41-principal-và-tenantscope).

```go
// src/backend/pkg/authz/principal.go
type Principal struct {
    UserID        uuid.UUID
    UserType      UserType   // student|staff (ADR-0081)
    TenantID      *uuid.UUID // nil = System Admin / thành phần hệ thống (SystemPlatformScope)
    TenantType    TenantType // school|unit|common
    Permissions   PermissionSet // hợp permission các UserRole hiệu lực (ADR-0066)
    Scopes        []Scope    // school/subject từ UserRole
    IsSystemAdmin bool
    ClientID      string     // azp
    SessionID     string     // sid (thu hồi khi takeover)
}
```

Lời gọi facade giữa module mang **chính Principal gốc** trong `context.Context` (on-behalf); module gọi được `src/backend/pkg/modcall` ghi riêng (`modcall.CallerFrom(ctx)`), không nằm trong `Principal`. Không có header `X-On-Behalf-Of` hay danh tính service.

### 6.1. Fast path đăng nhập ở 10.000 CCU (ADR-0045)

| Bước | Tối ưu | Mục tiêu |
|---|---|---|
| Verify JWT | JWKS cache trong bộ nhớ mỗi replica, làm mới nền 10 phút, làm mới ngay khi gặp `kid` lạ (tối đa 1 lần/30 s); không gọi Keycloak introspection | < 1 ms |
| Resolve `(iss, sub)` → User, memberships, Principal | L1 LRU in-process 15 s + Valkey `principal_cache_ttl` (`ew:<module>:principal:<user_id>:<tenant_id>`, [VALKEY_DESIGN §3.1](VALKEY_DESIGN.md#31-tổng-quan)); cache miss gọi facade identity trong tiến trình; invalidate bằng event | hit ≥ 95% |
| Thông tin tenant, settings, chat policy | Valkey 60 s + L1 | — |
| Kiểm tra SIS nguồn (ADR-0046) | Đọc `sis_identity_links` + trạng thái bản ghi đồng bộ cùng Principal (cùng cache 60 s); **không gọi SIS** | — |
| `POST /auth/game/login` (ADR-0068) | Một lời gọi token endpoint Keycloak (client `eduworld-game-backend`, pool kết nối HTTP giữ sẵn, timeout 3 s, circuit breaker → `503`), rồi resolve membership/enrollment từ cache; không gọi SIS; không log body; hàng chờ vượt năng lực → `429 LOGIN_QUEUE_BUSY` | p95 ≤ 400 ms (gồm băm mật khẩu Keycloak ≤ 80 ms) [Confirmed] |
| `POST /auth/session` (portal) | Chỉ đọc DB của module identity khi cache miss; không gọi facade module khác | p95 ≤ 150 ms |
| `POST /game/sessions` | Một transaction (game session + login event + `last_game_login_at`), ticket realtime ghi Valkey; bootstrap trả manifest/hồ sơ từ cache | p95 ≤ 150 ms |
| Ghi `last_login_at` sang identity | Qua event `student.game_logged_in` (bất đồng bộ), không chặn đường đăng nhập | — |

Khi Valkey lỗi: đọc DB trực tiếp với giới hạn concurrency (bulkhead) để không đánh sập DB trong bão đăng nhập; vượt giới hạn → `503 + Retry-After`.

Không có cổng internal hay middleware xác thực service: facade chỉ gọi được trong tiến trình. Bên nhận áp `TenantScope` có trong context (vẫn kiểm tra tenant tồn tại/hoạt động khi cần) và quyền nghiệp vụ của Principal gốc (§7).

## 7. Gọi module khác: facade trong tiến trình (ADR-0074, ADR-0040)

Quy ước facade (spec, `x-allowed-callers`, ngữ cảnh, bản đồ gọi, deadline) chỉ định nghĩa ở [API_DESIGN §12](API_DESIGN.md#12-facade-giữa-module-adr-0074-adr-0040); mục này mô tả cách triển khai.

### 7.1. Bên nhận: hiện thực facade

`make gen MOD=learning` sinh `src/backend/modules/learning/api/facade.gen.go` từ `specs/openapi/modules/learning.internal.v1.yaml` bằng `oapi-codegen` (chế độ strict-server, chỉ kiểu + interface): mỗi operation thành một phương thức `(<OperationId>(ctx, <OperationId>RequestObject) (<OperationId>ResponseObject, error))`, mỗi schema thành một kiểu DTO có tag validate theo `maxLength`. Path `/internal/v1/...` chỉ là định danh operation; không có router hay HTTP server nội bộ. Module đích hiện thực interface trong `presentation/facade`, gọi thẳng command/query handler:

```go
// src/backend/modules/learning/internal/presentation/facade/attempts.go
type Facade struct {
    CreateAttemptCmd command.CreateQuestAttempt // interface có kiểu, không mediator
    // ...
}

var _ api.StrictServerInterface = Facade{}

func (f Facade) CreateAttempt(ctx context.Context, req api.CreateAttemptRequestObject) (api.CreateAttemptResponseObject, error) {
    // ctx đã có TenantScope, Principal gốc, Idempotency-Key, module gọi, deadline do src/backend/pkg/modcall gắn
    res, err := f.CreateAttemptCmd.Handle(ctx, toInput(*req.Body))
    if err != nil {
        return nil, err // lỗi domain/problem; modcall giữ nguyên code Problem cho bên gọi
    }
    return api.CreateAttempt201JSONResponse(toAPI(res)), nil
}
```

Command handler phía nhận vẫn gọi `authz.Require` với Principal gốc trong context và đọc dữ liệu qua repository có scope, như khi request đến từ HTTP (ADR-0005).

### 7.2. Bên gọi: port + adapter + `src/backend/pkg/modcall`

```go
// src/backend/modules/game/internal/application/port/learning.go — game chỉ biết port, không biết facade
type Learning interface {
    StartQuestAttempt(ctx context.Context, in StartAttemptInput) (StartedAttempt, error)
}

// src/backend/modules/game/internal/infrastructure/modules/learning.go — adapter, chỉ nơi này (và module.go) import learningapi
type Learning struct{ facade learningapi.StrictServerInterface } // gắn ở Wire (§3.3)

func (a *Learning) StartQuestAttempt(ctx context.Context, in port.StartAttemptInput) (port.StartedAttempt, error) {
    res, err := modcall.Call(ctx, modcall.Op{
        Caller: "game", Callee: "learning", Operation: "createAttempt",
        Kind: modcall.Write, IdempotencyKey: in.IdempotencyKey,
    }, func(ctx context.Context) (learningapi.CreateAttemptResponseObject, error) {
        return a.facade.CreateAttempt(ctx, learningapi.CreateAttemptRequestObject{Body: toBody(in)})
    })
    if err != nil {
        return port.StartedAttempt{}, err // đã ánh xạ sang problem (giữ code gốc; deadline → 503 UNAVAILABLE)
    }
    created, ok := res.(learningapi.CreateAttempt201JSONResponse)
    if !ok {
        return port.StartedAttempt{}, modcall.Unexpected("learning.createAttempt", res)
    }
    return toPort(created), nil
}
```

`modcall.Call` làm, theo thứ tự:

1. Mở span `<caller>→<callee>.<operationId>` (con của span use case hiện tại) và ghi metric (API_DESIGN §12.3).
2. Đặt deadline theo `Kind` (đọc/ghi/bước saga) nếu context chưa có deadline ngắn hơn.
3. Kiểm `TenantScope` (hoặc `CrossTenantScope`/`PlatformScope`) có trong context khi operation khai báo `x-context-required: [tenant_scope]`; thiếu → lỗi lập trình (`500`, không gọi).
4. Giữ nguyên Principal gốc (on-behalf) trong context; ghi module gọi (`modcall.CallerFrom(ctx)`) để audit và để kiểm `x-allowed-callers` lần cuối (`403 CALLER_NOT_ALLOWED`).
5. Gắn `Idempotency-Key` vào context khi operation khai báo `x-context-required: [idempotency_key]`; thiếu → lỗi lập trình.
6. Gọi facade; `panic` của bên nhận được recover thành `500 INTERNAL`; lỗi được ánh xạ qua `problem.Map` (giữ `code` Problem gốc khi có nghĩa với người dùng; deadline vượt hoặc database module đích không sẵn sàng → `503 UNAVAILABLE` có `retryable`).

Không có HTTP, token `svc-*`, retry, circuit breaker hay bulkhead cho lời gọi facade (ADR-0074 mục 3). Bên gọi không tự retry trong request; saga platform thử lại bước lỗi ở tầng saga với cùng `Idempotency-Key` ([ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md)).

### 7.3. Tích hợp bên ngoài giữ resilience

Keycloak (token endpoint, Admin API), SIS (pull API, webhook), object storage và SMTP là hệ thống ngoài tiến trình. Adapter của chúng nằm trong `infrastructure/` của module sở hữu (identity: Keycloak, SIS, SMTP; learning/game/chat: object storage), dùng `failsafe-go`: timeout, retry chỉ cho thao tác idempotent (GET hoặc có khóa idempotency), circuit breaker, bulkhead, giá trị ở [API_DESIGN §12.3](API_DESIGN.md#123-resilience-mặc-định-confirmed). Client Keycloak `svc-identity` (client credentials) chỉ dùng để gọi Keycloak Admin API (ADR-0074 mục 5).

### 7.4. Quy tắc

- Spec internal là hợp đồng; thay đổi qua MR vào `specs/openapi/modules/<module>.internal.v1.yaml`, chạy `make gen`, `oasdiff breaking` và facade test (bên gọi với fake facade sinh từ interface, bên nhận với ví dụ trong spec; §15).
- Không gọi facade module khác trong transaction ghi DB (gọi trước, hoặc chuyển sang event). Mỗi module có database và transaction riêng; không có transaction chéo module.
- Phản ứng phụ (XP, quest, projection, mở channel chat) đi qua outbox → NATS, không gọi facade (ADR-0042).
- Khi tách một module thành service (ADR-0074 mục 7), cùng spec internal sinh HTTP server/client; bên gọi chỉ đổi adapter trong `infrastructure/modules/`, port và use case giữ nguyên.

## 8. Outbox → NATS JetStream (ADR-0042)

```go
// src/backend/pkg/outbox/writer.go
type Event struct {
    ID            uuid.UUID // UUIDv7
    Type          string    // ≤ 64, "activity.submitted"
    TenantID      *uuid.UUID
    Key           string    // ≤ 128, ordering key
    SchemaVersion int
    Payload       any
    Trace         map[string]string // traceparent
}

// Publish ghi vào bảng outbox_events trong cùng ent tx của command.
func (w *Writer) Publish(ctx context.Context, e Event) error
```

- `Writer` ghi bằng role `<module>_app` (chỉ `INSERT` trên `infra.outbox_events`, bảng `infra` không bật RLS); `Relay` chạy bằng role `<module>_relay` (`SELECT, UPDATE (published_at, attempts)`); consumer ghi `processed_events` với `event_occurred_at` của envelope (khóa partition ổn định, chống trùng xuyên ngày); retention bằng gỡ partition ngày qua job `infra-partition-maintenance` (ADR-0089 §4–5, [DATABASE_DESIGN §3.9](DATABASE_DESIGN.md#39-outbox--processed-events)).
- **Relay** và **Consumer** (dedupe `processed_events`, gọi trực tiếp command handler có kiểu trong `UnitOfWork`, ack sau commit, DLQ): luồng, stream, subject, consumer và DDL ở [EVENT_DESIGN §3](EVENT_DESIGN.md#3-outbox--nats-jetstream-adr-0042) và [DATABASE_DESIGN §3.9](DATABASE_DESIGN.md#39-outbox--processed-events). `src/backend/pkg/outbox` triển khai đúng luồng đó.
- Relay của mọi module chạy ở profile `worker-events` (mỗi module một relay, role `<module>_relay` trên database của module đó); consumer domain event chạy ở `worker-events`, consumer của stream `CHAT_MESSAGES` ở `worker-chat`; không consumer nào chạy trong `core-api` ([DEPLOYMENT §3.1](../11-operations/DEPLOYMENT.md)).
- Event vẫn là cơ chế tích hợp giữa module dù cùng tiến trình: mỗi module có database và transaction riêng, nên phản ứng phụ không được gọi thẳng handler của module khác.
- Không I/O ngoài DB trong transaction nghiệp vụ. Danh sách stream/subject/consumer: [EVENT_DESIGN](EVENT_DESIGN.md).

## 9. Realtime: binary `realtime` ↔ `core-api`

- Binary `realtime` nhận lệnh WS, chuyển thành **NATS request** `ew.cmd.<module>.<domain>.<action>` với header `Ew-Tenant-Id`, `Ew-User-Id`, `Ew-Game-Session-Id`, `traceparent`; timeout 2 s; lỗi → `system.command.failed`.
- Module đăng ký handler bằng `natsx.HandleCommand(subject, adapter)` trong `Register` (chỉ có hiệu lực ở `core-api`): adapter trong `presentation/natscmd` dựng `Principal` + `TenantScope` từ header (`realtime` đã xác thực ticket), gọi trực tiếp command/query handler có kiểu, trả reply. Đây là đường duy nhất `realtime` chạm logic nghiệp vụ: `realtime` không mở database và không gọi facade module domain trong tiến trình.
- Module đẩy sự kiện tới client bằng Valkey Pub/Sub `ew:rt:user:<user_id>`, `ew:rt:room:<room_id>`, `ew:chat:fanout:<channel_id>`; `realtime` subscribe theo kết nối đang giữ. Chi tiết: [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md).

### 9.1. Module chat với NATS + Valkey (ADR-0035)

| Thành phần | Binary (Swarm service) | Mô tả |
|---|---|---|
| `command.SendMessage/Edit/Recall/Forward/React/Pin` | `core-api` (handler NATS `ew.cmd.chat.*` + REST fallback) | Membership, policy, features, rate limit → moderation pipeline → `SeqAllocator` (Valkey Lua) → `ChatPublisher` (JetStream `ew.chat.<tenant>.msg.<channel>`) → reply acked → `Fanout` (Valkey Pub/Sub) |
| `chat-persister` | `worker-chat` | Pull consumer `persister`; ghi lô `eduworld_chat` (`ON CONFLICT DO NOTHING`), cập nhật `channels.last_seq`, con trỏ; ack sau commit; DLQ sau 5 lần |
| `chat-notifier`, `chat-moderation-async`, `chat-stats` | `worker-chat` | Mention offline, bước `async_analysis` (ADR-0030), rollup trước purge (ADR-0033) |
| `chat-channel-closer`, `gameplay-chat-purge` | `worker-chat` | Đóng channel gameplay khi `match.completed`/`match.aborted`/`party.disbanded`; purge bằng DSN của role `chat_purger` (chỉ cấp cho profile `chat`) |

Stream/consumer tạo idempotent khi `worker` khởi động (`CreateOrUpdateStream`). Integration test dùng testcontainers `nats:2.10-alpine -js` + Valkey + PostgreSQL.

## 10. Error model

| Lớp | Kiểu | Ví dụ |
|---|---|---|
| Domain | `domain.Error{Code, Kind}` (`NotFound, Conflict, Invalid, Forbidden, PreconditionFailed, RateLimited, Unavailable`) | `ErrAttemptAlreadySubmitted = Conflict("ATTEMPT_ALREADY_SUBMITTED")` |
| Application | Wrap `fmt.Errorf("submit attempt %s: %w", id, err)` | giữ chuỗi nguyên nhân cho log |
| Presentation | `problem.Map(err)` → RFC 9457 | `409 application/problem+json` |
| Lời gọi facade module khác | `src/backend/pkg/modcall` giữ `code` Problem của module đích; deadline vượt hoặc database module đích không sẵn sàng → `503 UNAVAILABLE` (có `retryable`); panic bên nhận → `500 INTERNAL` | |
| Adapter tích hợp ngoài | Lỗi mạng, timeout, circuit mở → `503 UNAVAILABLE` (có `retryable`) | Keycloak, SIS, object storage |

Mapping `Kind → HTTP`: NotFound 404, Conflict 409, Invalid 422, Forbidden 403, Unauthenticated 401, PreconditionFailed 412, RateLimited 429, Unavailable 503; lỗi không nhận diện → 500 `INTERNAL`, không lộ chi tiết. Tài nguyên ngoài tenant/scope luôn trả **404**. Catalog: [API_DESIGN §4](API_DESIGN.md).

## 11. Context, logging, tracing

- `ctx` là tham số đầu mọi hàm I/O; timeout ở biên (HTTP `ReadHeaderTimeout`, DB `statement_timeout` 5 s request / 60 s job, deadline lời gọi facade và timeout tích hợp ngoài theo §7).
- `slog` JSON: `ts, level, msg, binary, profile, module, env, version, trace_id, span_id, request_id, user_id, tenant_id, route, status, duration_ms, error.code`. **Không log** token, nội dung chat, câu trả lời, họ tên, ngày sinh.
- OpenTelemetry: `otelhttp` (server + client tích hợp ngoài), `otelpgx`/ent driver, span use case (`observability.UseCase`, tên `<module>.command.<Name>`/`<module>.query.<Name>`), span lời gọi facade (`src/backend/pkg/modcall`, tên `<caller>→<callee>.<operationId>`), propagation `traceparent` qua HTTP header và NATS header (outbox lưu trace để nối consumer); lời gọi facade truyền span qua `context`.
- Metric chuẩn mỗi binary (nhãn `module` khi metric thuộc một module): `eduworld_http_server_duration_seconds`, `eduworld_internal_client_requests_total{caller_module,callee_module,operation,outcome}`, `eduworld_internal_client_duration_seconds` (lời gọi facade), `eduworld_external_client_requests_total{client,operation,outcome}`, `eduworld_external_client_circuit_state{client}` (tích hợp ngoài; tên chuẩn: [API_DESIGN §12.3](API_DESIGN.md#123-resilience-mặc-định-confirmed)), `eduworld_ratelimit_rejected_total`, `eduworld_tenant_filter_bypass_total{module,reason}`, `eduworld_outbox_lag_seconds`, `eduworld_consumer_pending{stream,consumer}`, `eduworld_db_pool_*`. Danh sách: [MONITORING](../11-operations/MONITORING.md).

## 12. Cấu hình (environment variables)

Cấu hình qua env (12-factor): prefix `EW_`; binary và profile (`--profile` hoặc `EW_WORKER_PROFILE`) quyết định module nạp; staging/prod đọc secret qua file `*_FILE` (Docker secrets) [Confirmed — ADR-0024]; OpenTelemetry dùng biến chuẩn `OTEL_*`. **Bảng biến môi trường duy nhất** (tên, giá trị local, theo binary: `EW_HTTP_ADDR`, `EW_ADMIN_ADDR` (cổng admin health + metrics, `:9090` trong container), `EW_DB_<MODULE>_*_DSN` ở local / `EW_DB_<MODULE>_*_URL_FILE` ở staging-prod cho từng database module (gồm `READ` cho read replica), `EW_KC_ADMIN_*`, `EW_S3_*`, `EW_SMTP_*` của module identity…; không có biến URL hay token cho gọi giữa module) nằm ở [LOCAL_DEVELOPMENT §4](../11-operations/LOCAL_DEVELOPMENT.md#4-biến-môi-trường); bảng cổng ở [LOCAL_DEVELOPMENT §3.2](../11-operations/LOCAL_DEVELOPMENT.md#32-service-ứng-dụng-profile-app). Tham số nghiệp vụ (cửa sổ sửa/thu hồi tin, thời hạn, ngưỡng) không phải biến môi trường: đọc từ platform/tenant settings hoặc cấu hình có version ([CONFIG_DEFAULTS](../00-overview/CONFIG_DEFAULTS.md), AGENTS R33).

Cấu hình validate khi khởi động; thiếu biến bắt buộc → exit 1, log tên biến (không in giá trị).

## 13. Graceful shutdown

| Binary | Trình tự khi nhận SIGTERM |
|---|---|
| `core-api` | readiness `503`; chờ drain 5 s; `srv.Shutdown` (20 s) cho HTTP; drain NATS subscription request/reply `ew.cmd.*`; đóng pool ent/pgx của từng module, Valkey; flush OTel |
| `worker` (mọi profile) | ngừng fetch; chờ handler đang chạy (≤ 60 s); commit + ack lô đang xử lý, phần còn lại được redeliver; `nc.Drain()`; nhả Valkey lock |
| `realtime` | readiness `503`; gửi `system.server.draining` (`reconnect_after_ms`) tới client; đóng WS `1012` sau tối đa 30 s; huỷ subscribe Valkey |

## 14. Health checks

| Endpoint (cổng admin, không public) | Ý nghĩa | Kiểm tra |
|---|---|---|
| `GET /livez` (cổng admin `EW_ADMIN_ADDR`) | Tiến trình còn sống | Không gọi phụ thuộc |
| `GET /readyz` | Sẵn sàng nhận traffic | Database của từng module được nạp, Valkey, NATS, JWKS đã nạp (`core-api`), Atlas revision của từng database ≥ yêu cầu, mọi facade đã `Wire`. Không gọi Keycloak, SIS hay hệ thống ngoài (tránh lỗi dây chuyền) |
| `GET /healthz` | Chi tiết cho vận hành | JSON từng check theo module, trạng thái circuit breaker của từng adapter tích hợp ngoài |

## 15. Kiểm thử backend

| Tầng | Phạm vi | Công cụ | Chạy khi |
|---|---|---|---|
| Unit | `domain/` (aggregate rule), `application/` (handler với fake port, gồm ca thiếu quyền và validation), middleware `src/backend/pkg/httpx`, scoring, mastery, Glicko-2, policy chat | `testing`, `testify`, table-driven, fuzz | Mọi commit |
| Integration (trong module) | ent repository + interceptor tenant/soft-delete, RLS, Atlas migrate up, outbox relay → NATS, consumer idempotent | `testcontainers-go` (postgres:17, valkey:8, nats:2.10 -js) | Mọi MR |
| Contract (HTTP public) | Public handler khớp `specs/openapi/modules/<module>.v1.yaml` | `kin-openapi` validator, Prism mock, `oasdiff` | Mọi MR |
| Facade (`make test-contract`) | Bên nhận: facade trả đúng kiểu và mã lỗi theo ví dụ trong `<module>.internal.v1.yaml`; bên gọi (vd game) chạy với fake facade sinh từ interface `src/backend/modules/learning/api`; `make gen-check` bảo đảm interface khớp spec | `go test`, fake sinh từ interface, `oasdiff` | Mọi MR |
| Contract (event) | Payload event khớp `specs/events/*.schema.json` cả phía publish và consume | ajv/jsonschema Go | Mọi MR |
| Kiến trúc (`make arch-check`) | depguard/go-arch-lint, import chéo module khớp `x-allowed-callers` (`src/backend/tools/allowedcallers`), `entlint` (MaxLen), `sqlguard` (không DELETE ngoài purge) | CI | Mọi MR |
| Authz / tenant | Ma trận permission × endpoint; học sinh tenant A không thấy dữ liệu tenant B; interceptor từ chối khi thiếu scope | test sinh từ catalog | Mọi MR |
| E2E | Login → bootstrap → game session → quest activity (game → learning qua facade) → XP (event) → chat trong party | docker compose 3 binary + Keycloak test realm | Nightly + trước release |
| Resilience | Database module learning chậm/lỗi → lời gọi facade của game vượt deadline, trả 503 có retryable; Keycloak/SIS lỗi → circuit adapter mở/đóng; NATS mất → outbox tồn và bù; một profile `worker-*` dừng không ảnh hưởng `core-api` | toxiproxy / dừng container | Nightly |
| Performance | REST, WS, chat, matchmaking, lời gọi facade | k6 / Postman performance | Theo [TEST_STRATEGY](../12-project/TEST_STRATEGY.md) |

Không mock database trong integration test; mỗi bug fix kèm test tái hiện; coverage mục tiêu ≥ 80% cho `domain/` và `application/` của learning, competition, chat [Confirmed].
