# Changelog

Mọi thay đổi đáng chú ý của EduWorld được ghi tại đây.

Định dạng theo [Keep a Changelog](https://keepachangelog.com/en/1.1.0/); dự án dùng [Semantic Versioning](https://semver.org/) cho backend/web. Godot client có `client_version` riêng (xem [DEPLOYMENT §11](docs/11-operations/DEPLOYMENT.md#11-release-và-rollback)).

Nhóm mục: `Added`, `Changed`, `Deprecated`, `Removed`, `Fixed`, `Security`, `Docs`, `Decisions` (ADR), `Answered` (Open Questions).

## [Unreleased]

### Added
- 2026-11-18 — **F003 platform kit dữ liệu** (`src/backend/pkg`): `entx` (7 mixin + interceptor/hook theo loại bảng, soft-delete, `UnitOfWork`/`ReadSession` chọn role theo scope, `SoftDelete`/`Restore`), bảng `infra` partition theo ngày cho 5 module (`tools/infragen`, job `partitionmaint`), `outbox` (writer, relay, consumer idempotent + DLQ, `AuditRecorder`), `idempotency` + `httpx.Idempotency`, `natsx` (stream, `PrincipalInvalidator`), `modcall`, facade sinh bằng `tools/facadegen` (+ `api.Fake`), `cmd/internal/wiring.PrincipalResolver`, `projection` + `worker projection rebuild`, `lifecycle` (graceful shutdown), `observability.UseCase`, `testkit.New`, `testkit/contract`, module mẫu CQRS; công cụ `entlint`, `tenancyguard`, analyzer `nointernalhttp`, luật read/write trong `archgen`.
- 2026-11-18 — **F002 platform kit lõi** (`src/backend/pkg`): `authz` (catalog 121 permission khớp ADMIN_PORTAL §2.2, 12 role hệ thống, `ValidateCatalog` lúc khởi động mọi binary, `Principal`/`Grant`, `Require` 404/403 + audit `authz.denied`, scope Tenant/CrossTenant/Platform, cache Principal L1 + Valkey fail-closed, harness `authztest`); `httpx` (xác thực JWT bằng JWKS trong bộ nhớ, không token service, `X-Tenant-Id`, `TENANT_MISMATCH`, guard `portal_staff`/`game_player`/`platform_mfa`, validator kin-openapi `422 MAX_LENGTH`, không route `/internal/`); `observability` (slog JSON che dữ liệu nhạy cảm, span OTel, metric HTTP); `problem`, `config`, `tenancy` (RLS helper, `_live` view, `Bypass` 11 lý do), `ids`, `clock`, `audit`. Công cụ: `archgen` (thêm luật domain/hạ tầng, core pkg không ent/pgx), `ewlint` (`bypassreason`, `scopeguard`), golden API, `allowedcallers`, `routematrix`; `.golangci.yml`.

### Decisions
- 2026-11-17 — **ADR-0092:** mã nguồn trong `src/`: `src/backend` (Go module duy nhất: `cmd/`, `modules/`, `pkg/`, `tools/`, `test/`), `src/cms/frontend` (Admin Portal), `src/game` (Godot); `deploy/`, `specs/`, `docs/`, `scripts/`, `design/` ở gốc. Đường dẫn trong tài liệu và feature spec đổi theo.
- 2026-11-17 — **ADR-0025 Accepted:** bộ thư viện Go (chi, oapi-codegen, ent + pgx, Atlas, nats.go, valkey-go, coder/websocket, go-oidc + go-jose, failsafe-go cho tích hợp ngoài, koanf, slog + OpenTelemetry, testify + testcontainers-go, golangci-lint + depguard).
- 2026-11-17 — **ADR-0091:** Admin Portal đăng nhập bằng form riêng qua `identity` (Keycloak Direct Access Grant, client `eduworld-portal` confidential), mật khẩu + TOTP gửi một lần, refresh token trong cookie HttpOnly + CSRF, tenant qua `X-Tenant-Id`; đăng ký OTP qua email action-token của Keycloak (ngoại lệ duy nhất). Thay một phần ADR-0066, 0067, 0068.
- 2026-11-17 — **ADR-0090:** danh mục menu/action Admin Portal (`specs/portal/menu-catalog.yaml`, bản sao `platform.portal_menu_catalog`); System Admin bật/tắt qua `platform.feature_flags` (platform/tenant); backend chặn bằng `featuregate` → `403 FEATURE_DISABLED`; 10 menu không tắt được.
- 2026-11-17 — **ADR-0074 (Accepted, thay ADR-0036):** modular monolith. Module `identity`, `learning`, `game`, `competition`, `chat`, `realtime` trong `modules/<module>/` (facade `api/` sinh từ spec internal, phần còn lại trong `internal/`); 3 binary `cmd/core-api`, `cmd/realtime`, `cmd/worker` (profile `events`/`chat`/`jobs`); mỗi module một database trên một cụm PostgreSQL; gọi giữa module trong tiến trình qua `pkg/modcall`, không HTTP nội bộ, không token `svc-*`. Room host là `pkg/roomhost` dùng bởi `competition` và `game` trong `core-api`. ADR-0037, ADR-0040 bị thay một phần.
- 2026-11-17 — **ADR-0088:** CI/CD bằng GitHub Actions (`.github/workflows/` theo lane, ghcr.io, runner macOS cho iOS, GitHub Environments, runner tự host cho deploy).
- 2026-11-17 — **ADR-0089:** tầng dữ liệu — RLS chỉ lọc tenant (`deleted_at` do ent interceptor và view `_live`), 7 mixin theo loại bảng, scope Tenant/CrossTenant/Platform/SystemPlatform, 11 reason `tenancy.Bypass` kèm quyền DB, audit `cross_tenant.accessed`, hàm `identity.memberships_for_user` cho đường đăng nhập, outbox/processed/idempotency/dead letter không RLS và gỡ partition theo retention, `Idempotency-Key` bắt buộc UUIDv7. ADR-0028 bị thay một phần (RLS).
- 2026-11-17 — **ADR-0073, 0075, 0078, 0080 Accepted** (pilot ưu tiên giáo dục trên 4 nền tảng; object storage production không dùng floci, thay ADR-0071; mặc định an toàn trẻ nhỏ, thêm Windows mặc định máy dùng chung và màn "Tiếp tục là…", thay một phần ADR-0030; đo hiệu quả học tập là exit criterion pilot). **ADR-0076, 0079 Rejected** (không đăng nhập QR + PIN; giữ 4 nền tảng). ADR-0077 hoãn (ADR-0082).

### Changed
- 2026-11-18 — Hợp đồng: `Principal.grants[]` trong `identity.internal.v1.yaml` (permission theo phạm vi UserRole; F012 cài đặt); `Idempotency-Key` mô tả UUIDv7 + `pattern`, `400` khi sai hoặc lệch giờ. MONITORING thêm metric và cảnh báo của F003; LOCAL_DEVELOPMENT thêm biến OIDC, client id, shutdown, `PARTITION_ADMIN_DSN`.
- 2026-11-18 — Sửa YAML spec gộp (`TenantCreate.keycloak_org_alias.description` có dấu phẩy không quote tạo key thừa); compose xuất trace OTLP/HTTP `:4318`; SECURITY_ARCHITECTURE §5.2 và VALKEY_DESIGN (khóa phụ cache Principal) theo cài đặt F002.
- 2026-11-17 — Sửa DATABASE_DESIGN bị chèn trùng ~130 dòng đầu file (chuỗi thay thế của `String.replace` chứa `` $` `` — chèn toàn bộ phần văn bản trước chỗ khớp — trong lượt áp ADR-0090) và giá trị mẫu `old_keycloak_sid` trong schema `game_session.replaced` (gitleaks báo nhầm).
- 2026-11-17 — Contract-first cho M-C0/M-C1: áp mọi delta hợp đồng của F001–F005, F010–F015, F050, F051, F060, F061 (và phần F016/F017 mà M-C1 cần) vào `specs/`: 28 schema event mới (identity, game, `common/audit.recorded`), 14 operation public mới (tenant/trường, kết nối SIS, `createCommonTenantStudents`, `resetStudentPassword`), facade `listProjectionSnapshot`, header `Idempotent-Replayed`, bỏ `FORBIDDEN_SCOPE`. Quyết định: tạo học sinh tenant chung (username bắt buộc, mật khẩu sinh và trả một lần, xử lý từng dòng); đặt lại mật khẩu học sinh (F016, School/Tenant/System Admin, thu hồi phiên game); WebSocket `session.session.ended` + close 4410 (giờ giới nghiêm) / 4403 (thu hồi quyền); `end_reason` thêm `access_revoked`; `external_ref` lưu ở `identity.external_identities`.
- 2026-11-17 — `lesson.completed` (owner F031, F074 cộng XP); nhắc nghỉ giải lao là telemetry (F062 → F080); F035 có 24 operation, 3 permission, 2 menu; cài đặt giờ giới nghiêm của trường (F052) và platform (F057); `INVALID_STATE_TRANSITION` thống nhất 422.
- 2026-11-17 — Áp ADR-0090/0091: 6 operation `/auth/portal/*`, `listFeatureFlags`/`putFeatureFlag`/`listPortalMenuCatalog`, 7 ProblemCode mới (`ACCOUNT_SETUP_REQUIRED`, `SETUP_TICKET_INVALID`, `PASSWORD_RESET_TOKEN_INVALID`, `CSRF_TOKEN_INVALID`, `FEATURE_DISABLED`, `FEATURE_NOT_DISABLEABLE`, `AUDIO_REQUIRED`); danh mục 5 workspace / 78 menu / 221 action, 258 operation có `x-portal-action`; script `check-portal-catalog.mjs`; FR-CMS-018; cập nhật KEYCLOAK_SSO, ADMIN_PORTAL, SECURITY_ARCHITECTURE, API_DESIGN, DATABASE_DESIGN, CONFIG_DEFAULTS, TEST_STRATEGY, F004, F012, F015, F051 và các feature portal.
- 2026-11-17 — Chủ sở hữu event: mỗi event một module phát và một feature sở hữu schema; thêm 15 event vào EVENT_DESIGN; bỏ/đổi tên event không có producer; script `check-cross-features.mjs`. F003 thêm `worker projection rebuild`.
- 2026-11-17 — Xung đột giữa feature: F041 → F042 (hết vòng phụ thuộc), báo cáo game của F081 chuyển vào F080, test xuyên suốt F013/F014 chuyển sang F016, `activity.*` thuộc F030, `grade_levels`, `audit.recorded` một nơi; thêm AC cho mặc định an toàn ADR-0078 ở 13 feature và 4 ProblemCode (`CURFEW_ACTIVE`, `MODERATION_SWITCH_*`); F130 theo ADR-0075.
- 2026-11-17 — Đổi tên theo ADR-0074 trên toàn bộ tài liệu và feature spec (`services/<x>-service/` → `modules/<x>/`, `specs/openapi/services/` → `specs/openapi/modules/`, `pkg/svcclient` → `pkg/modcall`, `SVC=` → `MOD=`); viết lại SYSTEM_ARCHITECTURE, BACKEND_DESIGN, API_DESIGN §12 (facade), EVENT_DESIGN, DEPLOYMENT (5 Swarm service, một cụm PostgreSQL), LOCAL_DEVELOPMENT, MONITORING, BACKUP_RECOVERY, REALTIME_ARCHITECTURE, KEYCLOAK_SSO (chỉ còn `svc-identity`), SECURITY_ARCHITECTURE, TEST_STRATEGY (facade test, `make arch-check`), GLOSSARY, ROADMAP, RISKS (RISK-55, RISK-56), constitution v1.2.0, template Spec Kit, AGENTS R13/R29/§3/§4/§6.
- 2026-11-17 — Spec internal: bỏ `servers`/`security`/header, `x-allowed-callers` theo tên module, thêm `x-context-required`; 3 luật Spectral mới. Envelope event: `producer` = `<module>/<thành phần>`.
- 2026-11-17 — CONFIG_DEFAULTS: `internal_api_resilience` tách thành `module_call_deadline` và `external_api_resilience`; thêm tham số an toàn trẻ nhỏ (giờ giới nghiêm, preset chat, công tắc kiểm duyệt có hạn, tenant `common`, máy dùng chung).
- 2026-11-17 — ROADMAP theo giai đoạn Pilot → Mở rộng → Hoàn thiện; CODE_PLAN v0.4 thêm mốc M-P1/M-P2, §5.1 lát cắt pilot, §4.1 phụ thuộc ẩn. F001 viết lại cho GitHub Actions và cấu trúc `modules/` + `cmd/`; F002–F004 và các feature khác bỏ HTTP/token giữa service.
- 2026-11-17 — Áp phần làm rõ của **ADR-0089**: mixin `TenantOptionalAppendOnly` (TA*), dòng tenant NULL chỉ ghi bằng `<svc>_platform` dưới `PlatformScope`, 10 reason `Bypass` kèm quyền DB, audit `cross_tenant.accessed` thay `cross_tenant.viewed`/`cross_tenant_access`, metric `eduworld_tenant_filter_bypass_total{module,reason}`, hàm `identity.memberships_for_user`, khóa partition ổn định (`event_occurred_at`, `key_issued_on`), `Idempotency-Key` UUIDv7 và ghi idempotency trong `UnitOfWork`, dead letters partition theo ngày, phạm vi migration viết tay; AC mới F003-AC-37/38, F013-AC-31, F020-AC-32, AC-ORG-028.
- 2026-11-17 — Áp **ADR-0089** (RLS chỉ lọc tenant, view `<table>_live`, bộ mixin `Base`/`TenantAppendOnly`/`TenantOptional`/`HybridContent`/`GlobalBase`/`AppendOnly`, scope Tenant/CrossTenant/Platform, danh sách đóng `tenancy.Bypass` gồm `auth_login`, bảng `infra` không RLS với role `_relay`/`_partition_admin`, retention outbox/processed/idempotency bằng gỡ partition, migration partition viết tay) vào DATABASE_DESIGN, MULTITENANCY, BACKEND_DESIGN, EVENT_DESIGN, MONITORING, AGENTS R27 và feature F002 (AC-29, AC-30), F003 (AC-34…36), F013 (AC-30), F020 (AC-31).
- 2026-11-17 — Đối soát toàn bộ tài liệu theo SPEC_RECONCILIATION_BACKLOG (quyết định đã chốt theo đề xuất): sửa tài liệu sở hữu ở learning, portal, backend (DATABASE/API/EVENT/VALKEY/BACKEND_DESIGN), identity, security, kiến trúc, game, social, client, vận hành; CONFIG_DEFAULTS đăng ký thêm khoảng 60 tham số; LOCAL_DEVELOPMENT §4 là bảng biến môi trường duy nhất; mã AC cục bộ CHAT_SYSTEM đổi sang `AC-CHS-NNN`; thêm config type `level-packages`, `progression-levels`, `competition-rewards`. Feature spec F001–F134 đồng bộ theo (mỗi feature đổi có Clarifications "Session 2026-11-17"). Còn lại: delta contract trong `specs/` làm theo task Phase 2 của từng feature.

### Decisions
- 2026-11-16 — **ADR-0086:** giữ phiên game bằng refresh token lưu trong kho bảo mật thiết bị cá nhân (xoay vòng, gắn thiết bị); còn hạn thì không đăng nhập lại; máy dùng chung và Web chỉ giữ trong RAM (thay một phần ADR-0064). **ADR-0087:** CQRS tách rõ read/write context, bỏ mediator `pkg/mediator` (thay một phần ADR-0038; sửa AGENTS R26, constitution điều II lên 1.1.0). Các câu hỏi mở trong SPEC_RECONCILIATION_BACKLOG §2, §3 và CODE_PLAN Bước 0 (D1–D18) chốt theo đề xuất.
- 2026-11-16 — **ADR-0081:** `user_type` chỉ gồm `student`, `staff`; nhân sự platform là `staff` có `primary_tenant_id = NULL`. **ADR-0082:** Spec-Driven Development chuẩn GitHub Spec Kit + EARS cho AI coding; kỹ năng viết/nói và pháp lý chuyển vào `docs/99-deferred/`, `specs/_deferred/` (ADR-0077, ADR-0053 hoãn); tham số đợt 2026-11-15 được duyệt [Confirmed]. **ADR-0083:** đăng nhập game và tạo game session là hai bước. **ADR-0084:** study group thuộc learning-service (thay một phần ADR-0050, ADR-0072). Dọn học sinh không đăng nhập giữ user Keycloak enabled (gỡ membership, thu hồi phiên).

### Added
- 2026-11-17 — **F001 (khung monorepo):** Go module `src/backend` với 3 binary (`core-api`, `realtime`, `worker --profile`) + job `migrate`, 6 module có `Register`, `pkg/health` (`/livez` `/readyz` `/healthz` `/metrics` `/version`), `pkg/buildinfo`, `pkg/module`, `pkg/bootstrap`, `pkg/testkit`; baseline Atlas cho 5 database (schema, `infra.current_tenant_id()`, không DELETE/TRUNCATE); init PostgreSQL một cụm (database + role theo module, không CONNECT chéo); Docker Compose hạ tầng/app/observability; Dockerfile distroless UID 10001; `tools/archgen` kiểm ranh giới module từ `x-allowed-callers`; 6 workflow GitHub Actions (action ghim SHA) + Dependabot + CODEOWNERS; script kiểm tra CI (path filter, workflow, cổng quản trị, quyền DB, gitleaks, govulncheck, image, backup/restore).
- 2026-11-17 — Feature spec **F035** nghiên cứu hiệu quả học tập (36 AC, 70 task); PRD thêm FR-ANL-017…023.
- 2026-11-16 — Khung SDD: `.specify/memory/constitution.md` (9 điều khoản + gate), `.specify/templates/*` (spec, plan, tasks, checklist, data-model, contracts, quickstart), lệnh `.claude/commands/speckit.*` (specify, clarify, plan, tasks, analyze, implement), script `scripts/specs/check-features.mjs`; AGENTS.md thêm R34 và quy trình SDD.
- 2026-11-16 — Feature spec `specs/<NNN>-<slug>/` cho toàn bộ Release: 92 feature (CMS/LMS M-C0…M-C3 trước, rồi game, realtime, party, thi đấu, chat, vận hành), mỗi feature gồm spec (EARS), plan (Constitution Check), data-model, contracts, quickstart, checklist, tasks; phủ 321/321 FR thuộc Release. Danh mục còn F112 (Godot channel chat bền vững, công cụ chủ club) chưa viết.
- 2026-11-16 — `docs/12-project/SPEC_RECONCILIATION_BACKLOG.md`: 10 quyết định cần chốt, khoảng 75 mâu thuẫn docs ↔ specs ↔ feature spec, 10 nhóm tham số chưa đăng ký CONFIG_DEFAULTS.
- 2026-11-16 — **ADR-0085:** dọn học sinh không đăng nhập giữ user Keycloak (thay một phần ADR-0032).

### Changed
- 2026-11-16 — Áp dụng **ADR-0086** vào contract, tài liệu và feature spec: `refreshGameToken` nhận `device_id` (bắt buộc), `mode` (`renew`/`resume`), `client_version`; response `GameTokenRefreshResponse` (thêm `bootstrap` khi `resume`); 3 `ProblemCode` mới `REFRESH_TOKEN_INVALID`, `REFRESH_TOKEN_REUSED`, `REFRESH_TOKEN_DEVICE_MISMATCH` (API_DESIGN §4.1, 153 mã); `game_session_sso` = idle 7 ngày / tối đa 30 ngày [Confirmed]; KEYCLOAK_SSO §5.1–5.2 (phiên đăng nhập game `identity.game_auth_sessions` chỉ lưu hash, rotation, phát hiện dùng lại, gắn thiết bị, thu hồi; phiên portal khi tải lại trang); GODOT_ARCHITECTURE (`SecureStorage` theo nền tảng), GAME_NETWORKING, STUDENT_GAME_UX_CORE (S01 tự vào game, Đăng xuất, máy dùng chung), SECURITY_ARCHITECTURE, PRD (FR-AUTH-008/009, AC-AUTH-027…029), TEST_STRATEGY (TC-AUTH-082…089), ROADMAP, CONFIG_DEFAULTS; feature spec F004, F013, F014, F051, F060, F061, F133. TEST_STRATEGY và ROADMAP bỏ mediator theo ADR-0087.
- 2026-11-16 — `CODE_PLAN_CMS_LMS_FIRST.md` v0.3: lộ trình SDD, sở hữu danh mục 85 feature (thứ tự CMS/LMS trước, mốc, phụ thuộc, trạng thái), exit criteria theo feature.
- 2026-11-16 — FR cục bộ trong tài liệu domain quy về PRD (86 dòng; PRD thêm 21 FR, 9 US); thêm nhãn `Deferred (ADR-0082)`. `check-features`/`build-rtm` dùng chung định nghĩa FR thuộc Release (phase P1–P6).
- 2026-11-15 — **ADR-0072 (Accepted):** bỏ vai trò giáo viên. Học sinh tự học; nhà trường (`school_admin`/`academic_manager`) giao bài và xem School Learning Dashboard; `written_response`, `speaking`, `project`, rubric, hàng đợi chấm ra khỏi Release; kiểm duyệt tập trung ở Platform Safety Team (role platform mới `safety_moderator`). ADR-0020, 0021, 0027, 0034, 0043, 0045, 0059, 0061, 0062, 0066 chuyển "một phần superseded by ADR-0072"; ADR-0048, 0050, 0053 (Proposed) điều chỉnh; các ADR khác cập nhật tham chiếu.
- 2026-11-15 — **ADR-0073…0080 (Proposed, chờ stakeholder duyệt):** pilot ưu tiên giáo dục, modular monolith, object storage không dùng floci, đăng nhập nhanh lớp 1–3, consent & vai trò dữ liệu, mặc định an toàn trẻ nhỏ, nền tảng bản đầu Windows + Android, đo hiệu quả học tập là exit criterion. Chưa đổi quyết định hiện hành.

### Answered
- 2026-11-15 — OQ-07: không có vai trò giáo viên (ADR-0072). Thêm OQ-23 (học sinh có nhu cầu đặc biệt/tiếng dân tộc thiểu số), OQ-24 (quy định game cho người dưới 18 tuổi, chờ pháp chế).

### Changed
- 2026-11-15 — Specs theo ADR-0072 và EDU_FIT_REVIEW (SPC-01…09):
  - Spec gộp tăng từ 364 lên 432 operation. Bỏ `/lms/reviews*`, rubric, written-response/recordings. Đổi `/lms/dashboards/teacher` thành `/lms/dashboards/school`.
  - Thêm `hints:next`, assignment đủ vòng đời, endpoint CMS/LMS/Exam còn thiếu, regrade-requests, kiểm duyệt tập trung, audit-logs, data-requests.
  - Thêm `QuestionResponse` cho 11 loại câu, `render_payload` chặn khóa đáp án, enum dùng chung `specs/schemas/common/enums.schema.json` + script `check-enums.mjs`.
  - Thêm schema activity `quiz`/`interactive-exercise`/`minigame`, payload event learning, bước quest `interdependent`.
  - SIS contract 1.2.0 bỏ giáo viên/phân công. Mã lỗi còn 150.

### Docs
- 2026-11-15 — Viết lại toàn bộ tài liệu theo ADR-0072 và lấp chỗ hổng từ EDU_FIT_REVIEW:
  - **Tài liệu sản phẩm, quản trị:** PRD (FR liên quan đánh dấu "Removed (ADR-0072)", thêm FR mới), personas, ADMIN_PORTAL, SCHOOL_LMS.
  - **Học tập:** ACTIVITY_ENGINE, EXAM_ENGINE, LEARNING_ANALYTICS (gợi ý, `rapid_guess`, hệ số đoán trúng, bài chẩn đoán, interleaving), CURRICULUM_MODEL (`yccd_ref`, `competency_ids`), CONTENT_MODEL (audio bắt buộc cho g1_3).
  - **Game:** QUEST/COMPETITION/PROGRESSION (bước `interdependent`, bảng lớp theo nỗ lực, bỏ bonus tốc độ lớp 1–5, chuyển năm học).
  - **Chat, an toàn:** CHAT/MODERATION/CHILD_SAFETY (Safety Team, thông báo cho nhà trường, on-call p1, sửa câu vi phạm R15), DATA_PRIVACY (DPIA trước Phase 4).
  - **Kỹ thuật, dự án:** kiến trúc/DB/identity/ops, UX game ("Nhà trường giao", nút Gợi ý, lồng tiếng g1_3), PERFORMANCE_BUDGET (ngân sách phòng máy), ROADMAP (M3 School Loop), TEST_STRATEGY, RISKS (RISK-47…49), AGENTS.md (R20, R23), CODE_PLAN.
- 2026-11-15 — Tài liệu mới [Proposed]: `02-learning/LEARNING_EFFICACY_PLAN.md`, `02-learning/CONTENT_PRODUCTION_PLAN.md`, `02-learning/ASSESSMENT_REGULATION_PROFILE.md`, `01-game-design/LEARNING_MECHANICS.md`. `WRITTEN_RESPONSE_RUBRIC.md` giữ làm thiết kế tham khảo, gắn nhãn ngoài Release.
- 2026-11-15 — CONFIG_DEFAULTS đăng ký thêm khoảng 30 tham số [Proposed] (học tập, thi đấu, kiểm duyệt, LMS, phòng máy); TRACEABILITY_MATRIX sinh lại (317 FR, 382 TC, 432 operation).
- 2026-11-15 — Thêm `docs/12-project/EDU_FIT_REVIEW.md`: review 5 chiều (sư phạm, game–giáo dục & phạm vi, an toàn/quyền riêng tư/trợ năng/lớp học, specs, công nghệ). Kết luận chưa đáp ứng đủ; 6 vấn đề sống còn K1–K6 và 8 ADR đề xuất. Chưa thay đổi quyết định nào.
- 2026-11-15 — Thêm `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md` [Proposed]: kế hoạch code làm CMS/LMS trước, chia 3 luồng CMS Backend / CMS Frontend / Game Client; liệt kê 17 mâu thuẫn docs–spec (D1–D17) cần chốt ở Bước 0. Chưa đổi ROADMAP, chờ PM + Architect duyệt.
- 2026-11-15 — Khởi tạo bộ tài liệu Phase 0 (Discovery & Architecture) cho EduWorld:
  - Tổng quan & sản phẩm: `docs/00-overview/*`, `docs/01-product/*`.
  - Kiến trúc: `docs/02-architecture/*` gồm System Architecture, Domain Model, Multitenancy, Realtime, Security Architecture.
  - Backend, Identity/SIS, Learning, Game, Multiplayer/Party, CMS/LMS, Security: `docs/03-backend` … `docs/09-security`.
  - DevOps: Deployment, Local Development, Monitoring, Backup & Recovery (`docs/10-devops/*`).
  - Dự án: Roadmap, Milestones, Test Strategy, Risks, Open Questions (`docs/11-project/*`).
  - Đặc tả máy đọc được trong `specs/` (OpenAPI, AsyncAPI, JSON Schema).
  - `README.md`, `AGENTS.md`, `CONTRIBUTING.md`, `CHANGELOG.md`, `VIBE_CODING_PROMPT.md`.
- 2026-11-15 — Vòng đồng bộ nhất quán giữa các tài liệu:
  - Mã lỗi thống nhất là `ACCOUNT_NOT_PROVISIONED`.
  - Ghép trận mặc định chỉ trong cùng trường. Liên trường chỉ khi organization bật.
  - Chat: World chat tắt trong MVP (ADR-0020). *Đã được thay bằng ADR-0020 Accepted, xem mục bên dưới.*
  - XP khi lặp lại: 100/50/25/0% trong 7 ngày, tối đa 1.000 XP/ngày.
  - Party và matchmaking: thao tác ghi đi qua REST, WebSocket chỉ cho ready/activity/start/proposal/cancel. Tên message theo AsyncAPI, tên domain event theo EVENT_DESIGN.
  - Route nội bộ: `/api/v1/admin|sis/*` là API quản trị cho người, `/internal/*` là machine-to-machine. Cả hai chỉ có trên gateway nội bộ.
  - DOMAIN_MODEL bổ sung trạng thái `anonymized`, các entity mới (ExamPaper, RegradeRequest, IntegritySignal, PartyPolicy, CompetitionPointLedger, LeaderboardSnapshot, ModerationEvidence, AssignmentSubmission, ReviewTask) và ánh xạ Keycloak Organization theo organization.
  - Thêm job `identity-relink`.
- 2026-11-15 — Áp dụng 3 quyết định của stakeholder vào toàn bộ docs/specs:
  - SIS: thêm `specs/sis/eduworld-sis-contract.v1.yaml` (contract do EduWorld sở hữu). Viết lại `SIS_INTEGRATION.md` v0.2: chỉ `contract_v1` + `csv_import`, conformance kit, `sis-mock` cho local dev. Gỡ nhắc tới hệ thống SIS của dự án khác.
  - Chat: bỏ mọi phân biệt theo lớp/grade band. `chat-policy.schema.json` v2 cấm điều kiện theo lớp và bắt buộc cấu hình `validation`.
  - Phạm vi: chuyển từ MVP sang Release 1.0 hoàn chỉnh (toàn bộ Master Prompt + Addendum, 4 nền tảng); roadmap ~37 sprint [Proposed]; thêm OQ-19 (voice chat).
- 2026-11-15 — Tenancy, phân quyền CMS, soft-delete, kiểm duyệt chat:
  - Mô hình tenant mới (trường độc lập / đơn vị nhiều trường / tenant chung), khóa `tenant_id` + RLS `app.tenant_id` trên mọi bảng nghiệp vụ; MULTITENANCY v0.3, DOMAIN_MODEL (Tenant thay Organization), DATABASE_DESIGN, BACKEND_DESIGN, VALKEY_DESIGN, EVENT_DESIGN (envelope `tenant_id`).
  - Đăng nhập theo tenant: mỗi tenant là một Keycloak Organization, chọn tenant tùy chọn, kiểm tra chéo membership, không thuộc tenant nào → tenant chung; cập nhật identity docs, Godot (màn hình chọn tenant), seed local.
  - CMS tách 2 bundle (`cms` cho tenant: Learning Management; `admin` cho platform: Game Studio, quản lý tenant), permission catalog, `/api/v1/game-admin/*` chỉ gateway nội bộ, System Admin xem liên tenant với dropdown lọc tenant.
  - Soft-delete toàn hệ thống (`deleted_at`, REVOKE DELETE/TRUNCATE, partial unique, Thùng rác + khôi phục); gán dữ liệu platform cho tenant và clone giữa tenant.
  - Danh mục từ nhạy cảm + công tắc kiểm duyệt (từng bước hoặc toàn bộ) theo platform/tenant; schema `sensitive-term-catalog`, `chat-policy` v3; OpenAPI thêm endpoint admin/tenant chat.
  - Thống nhất tiền tố metric `eduworld_*`.

- 2026-11-15 — Game session, chat, kiến trúc microservices:
  - Chỉ học sinh vào game, lưu lịch sử đăng nhập (`game.login_events`, `last_login_at`), một thiết bị cho mỗi tài khoản (xác nhận takeover, thiết bị cũ nhận thông báo, close 4409).
  - Worker dọn học sinh không đăng nhập theo `last_login_at` + số ngày cấu hình (soft-delete, báo trước, loại trừ, khôi phục khi đăng nhập lại).
  - Chat luôn có trong trận 1v1, trận đội, party (channel `match`/`team`/`party`); purge chat gameplay sau thời hạn lưu trữ.
  - Bộ tính năng chat đầy đủ (sửa, thu hồi, reply, quote, forward, emoji/sticker/GIF nội bộ, reaction, mention, pin, copy, typing, receipts, unread, lịch sử, tìm kiếm).
  - Chat qua NATS JetStream + Valkey Pub/Sub, lưu PostgreSQL.
  - Chuyển sang 6 microservice (identity, learning, game, competition, chat, realtime-gateway), database riêng mỗi service, CQRS + Clean Architecture, ent + Atlas, typed client oapi-codegen, outbox → NATS JetStream; viết lại SYSTEM_ARCHITECTURE, BACKEND_DESIGN, DATABASE_DESIGN, REALTIME_ARCHITECTURE, DEPLOYMENT, LOCAL_DEVELOPMENT.
  - Mọi chuỗi có giới hạn độ dài: DB `varchar(n)` (không còn `text`), `maxLength` ở 1.179 trường chuỗi trong spec; thêm `specs/.spectral.yaml` và `scripts/specs/*`.
  - Specs: OpenAPI public gộp (163 operation, `x-service`) + spec public sinh theo service + 5 spec internal; `specs/events/`; script tách spec `scripts/openapi/*`.

- 2026-11-15 — Phát hành nội dung, media, đăng nhập 10k CCU, SIS nguồn:
  - Draft/Publish với release pointer, lịch sử version (xem, diff, preview), rollback đổi con trỏ, hẹn giờ publish; giữ phiên bản cho lượt chơi/học đang diễn ra (level, quest, mini-game, bài học, attempt, trận, đề kiểm tra, bài giao); content manifest theo tenant; snapshot bất biến lên Valkey/CDN khi publish.
  - Media lưu object storage S3-compatible + CDN (5 bucket, presigned upload, ClamAV, chuyển định dạng cho Godot/web, khóa content-addressed); `media_assets`, `media_usages`.
  - Đăng nhập ở 10.000 CCU: phiên game 7/30 ngày + refresh rotation, đăng nhập nhanh (IdP trường, QR lớp + PIN, thẻ QR, mật khẩu hình ảnh, Device Grant), Keycloak cluster + băm mật khẩu benchmark, rate limit theo tenant/thiết bị, hàng chờ, autoscale theo lịch học; sizing hạ tầng 10k CCU.
  - Nhận biết SIS nguồn khi đăng nhập: `sis_connection_schools`, `synced_students`, `student_login_keys`, `sis_identity_links`; 3 luồng đăng nhập, xử lý khớp 0/1/nhiều, chuyển trường giữa 2 SIS.
  - Specs: OpenAPI gộp 189 operation; 1.371 trường chuỗi có maxLength.

- 2026-11-15 — Triển khai trên Docker và giao diện:
  - Bỏ Kubernetes/KEDA/HPA: local Docker Compose, staging/production Docker Swarm (stack file, overlay network, Docker secrets, rolling update/rollback, canary qua APISIX, scale theo lịch học bằng `ops-scheduler`); sizing 10k CCU theo vai trò node; chaos test bằng Toxiproxy/Pumba.
  - Hai hệ thống giao diện: Game (Godot 3D, token UI + 3D sinh Theme/resource) và Admin/Ops cho CMS/LMS (Ant Design 5 + Pro, compact, dashboard live-ops); dùng chung chỉ logo/màu thương hiệu và mã màu nghiệp vụ; thumbnail/turntable render từ asset 3D; tài liệu mới `docs/05-ux/DESIGN_SYSTEM.md`.

- 2026-11-15 — Audit theo Master Prompt "Specs & Documentation Re-architecture" và bổ sung phần thiếu quan trọng:
  - Báo cáo [DOCUMENTATION_AUDIT](docs/12-project/DOCUMENTATION_AUDIT.md): mức đáp ứng, 7 mâu thuẫn và cách xử lý, nội dung deprecated, danh sách tài liệu còn thiếu.
  - Sửa 21 sơ đồ Mermaid lỗi; thêm `scripts/docs/check-mermaid.mjs`, `check-links.mjs`, `make docs-check`.
  - floci thay MinIO cho local/dev/CI; production object storage còn mở (OQ-21).
  - Game: PROGRESSION_SYSTEM, CHALLENGE_BOSS_SYSTEM, INVENTORY_ECONOMY; quest 12 loại; tạo avatar, emote 3D.
  - Học tập: ACTIVITY_ENGINE (registry), VIDEO_LEARNING, AUDIO_LANGUAGE_LEARNING, WRITTEN_RESPONSE_RUBRIC, SIMULATION_ENGINE; 8 schema activity.
  - CMS: một Admin Portal 5 workspace (information architecture, đặc tả màn chính, client `eduworld-portal`, MFA cho workspace platform).
  - Công thức: MathJax 4 + MathLive trên portal, SVG render trên trình duyệt, game chỉ hiển thị.
  - Specs: OpenAPI gộp 213 path / 245 operation / 126 mã lỗi; AsyncAPI 108 message; 24 JSON Schema; 1.812 trường chuỗi có maxLength.

- 2026-11-15 — Hợp nhất tài liệu theo hướng game-first (review "phù hợp game", không lặp, không đứt quãng; chi tiết: [DOCUMENTATION_AUDIT §8](docs/12-project/DOCUMENTATION_AUDIT.md)):
  - Sao lưu toàn bộ trước khi hợp nhất (`d:\game\eduWorld_backup\`, ngoài repo).
  - Cấu trúc mới: `00-overview`, `01-game-design`, `02-learning`, `03-social-multiplayer`, `04-admin-portal`, `05-ux`, `06-architecture`, `07-backend`, `08-game-client`, `09-identity`, `10-security`, `11-operations`, `12-project`, `adr/`.
  - ADR tách thành 55 file `docs/adr/ADR-NNNN-<slug>.md`, chuẩn hóa (quyết định hiện hành + lịch sử thay đổi), README có "quyết định hiện hành theo chủ đề".
  - 67 tài liệu di chuyển/đổi tên; 9 tài liệu gộp: GAMEPLAY_LOOP → GAME_DESIGN; RANKING, LEADERBOARD, TOURNAMENT → COMPETITION_SYSTEM; EXERCISE_ENGINE → ACTIVITY_ENGINE; TEACHER_DASHBOARD → SCHOOL_LMS; SECURITY → SECURITY_ARCHITECTURE; CHILD_PROTECTION → CHILD_SAFETY; MILESTONES → ROADMAP. Đổi tên: GAME_WORLD_DESIGN → WORLD_DESIGN, 3D_WORLD_DESIGN → WORLD_RENDERING, COMPETITION_DESIGN → COMPETITION_SYSTEM, MULTIPLAYER_DESIGN → MULTIPLAYER_GAMEPLAY, SOCIAL_CHAT_DESIGN → SOCIAL_SYSTEM, CMS_DESIGN → ADMIN_PORTAL, LMS_DESIGN → SCHOOL_LMS.
  - Game-first: thể loại phiêu lưu 3D multiplayer RPG-lite; 8 vùng đất + mạch truyện 7 chương; quy tắc chống chuỗi quiz; roadmap theo trải nghiệm người chơi; luồng chơi end-to-end theo service; club/guild, bạn bè; danh mục 17 màn học sinh; đủ 20 kịch bản nghiệm thu có test case.
  - Mới: [CONFIG_DEFAULTS](docs/00-overview/CONFIG_DEFAULTS.md) — sổ đăng ký tham số mặc định với tài liệu sở hữu; quy tắc "một nguồn sở hữu" (docs/README §3.3, AGENTS R33).
  - 00-overview: PRODUCT_VISION v0.2 (game-first), PRD v0.3 (bảng phạm vi Release 1.0 theo domain, FR-GAME-016…018, §5.8 AC game design/thế giới, truy vết theo cấu trúc mới, sửa NFR-AVAIL trùng mã, FR-NTF-003 không push), USER_PERSONAS (động lực chơi theo khối), GLOSSARY (thế giới/vùng/chương truyện, club, friend duel, Activity & Attempt Engine).
  - Script: `scripts/docs/split-adr.mjs`, `move-docs.mjs`, `redirect-merged.mjs`, `lib-links.mjs`.

- 2026-11-15 — Song ngữ, máy tầm trung, danh mục, dashboard, đăng nhập và phân quyền (ADR-0056 … ADR-0069):
  - Game song ngữ Việt – Anh: `i18nText` trong nội dung, gettext `.po` cho game, `Accept-Language`, export/import bản dịch trong CMS và Game Admin.
  - Máy tầm trung: [PERFORMANCE_BUDGET](docs/08-game-client/PERFORMANCE_BUDGET.md) (hạng thiết bị, preset chất lượng, ngân sách FPS/bộ nhớ/băng thông, FPS đo theo p10), device matrix trong TEST_STRATEGY.
  - 18 màn học sinh có bố cục Web/PC và Mobile: [STUDENT_GAME_UX](docs/05-ux/STUDENT_GAME_UX.md) + 3 tài liệu CORE/LEARNING/SOCIAL.
  - Event & Season ([EVENT_SEASON_SYSTEM](docs/01-game-design/EVENT_SEASON_SYSTEM.md)) và Game Analytics ([GAME_ANALYTICS](docs/11-operations/GAME_ANALYTICS.md)): rollup PostgreSQL từ sự kiện NATS.
  - Một họ API attempt `/learning/activity-attempts`; không có AI giọng nói trong Release 1.0.
  - Danh mục quốc gia / tỉnh thành (34 tỉnh VN đang hiệu lực + 63 tỉnh cũ, bảng kế thừa) / trường: `specs/reference-data/*`, `scripts/specs/validate-reference-data.mjs`; học sinh thuộc đúng một trường và một lớp.
  - Đồng bộ dữ liệu học sinh từ SIS khi đăng nhập (chặn khi thiếu, chạy nền khi cũ hơn 24 giờ); SIS contract v1.1.0.
  - Dashboard không truy vấn dữ liệu thô: [DASHBOARD_DATA](docs/07-backend/DASHBOARD_DATA.md) (bảng rollup, read replica, `statement_timeout`, grain theo khoảng thời gian, cache Valkey, xuất bất đồng bộ).
  - Đăng nhập Release 1.0: username/password, không chọn trường; form trong game gửi qua `POST /auth/game/login` (identity-service → Keycloak); không tự đăng nhập, token chỉ trong RAM; SSO xác thực trước, identity-service kiểm tra tenant sau; một người một tài khoản SSO, username/email không trùng trong tenant. Gỡ đăng nhập nhanh (QR, PIN, mật khẩu hình ảnh, Device Grant) khỏi Release 1.0.
  - Header `X-Tenant-Id` bắt buộc trên request tenant-scoped, luôn kiểm tra chéo với token và membership.
  - Admin Portal có trang đăng nhập riêng, học sinh không vào được; phân quyền Permission (code) → Role (DB) → UserRole (DB, theo phạm vi); màn quản trị Role/UserRole.
  - Specs (SPEC-3): OpenAPI gộp 312 path / 364 operation / 344 schema / 151 `ProblemCode`; endpoint đăng nhập game, `/me/tenants`, RBAC, danh mục, dashboard (`from`/`to`/`grain`, `data_as_of`, `is_partial`), tham số `TenantIdHeader`; 4 event payload identity mới; thêm `scripts/specs/check-problem-codes.mjs` (API_DESIGN khớp `ProblemCode`).
  - Ma trận truy vết: [TRACEABILITY_MATRIX](docs/12-project/TRACEABILITY_MATRIX.md) sinh bởi `scripts/docs/build-rtm.mjs` (FR → tài liệu → service → operation → TC → AC, báo cáo mồ côi), chạy `--check` trong `make docs-check` / CI `docs:check`.
  - Cập nhật [docs/README](docs/README.md) (mục lục đủ 69 ADR và tài liệu mới), [adr/README](docs/adr/README.md) (quyết định hiện hành theo chủ đề), [DOCUMENTATION_AUDIT §9](docs/12-project/DOCUMENTATION_AUDIT.md#9-đợt-cập-nhật-song-ngữ-máy-tầm-trung-đăng-nhập-và-truy-vết-2026-11-15).

### Decisions
- 2026-11-15 — ADR-0001 … ADR-0025 tạo ở trạng thái `Proposed` (xem [README](docs/adr/README.md)).
- 2026-11-15 — ADR-0022 → `Accepted`: frontend CMS/LMS/Admin dùng **React** (stakeholder xác nhận). Chi tiết stack (TypeScript, Vite, TanStack Query, oidc-client-ts…) vẫn là `Proposed`.
- 2026-11-15 — ADR-0020 → `Accepted`: chat dùng một bộ quy tắc cho mọi lớp 1–9, mọi tin nhắn phải qua kiểm tra hợp lệ phía server (thay mục "Age/Grade-based Policy" của Master Prompt §6.3).
- 2026-11-15 — ADR-0021 → `Accepted`: SIS integration theo contract do EduWorld sở hữu; trường cung cấp API theo contract, không dùng DTO của SIS.
- 2026-11-15 — ADR-0013 sửa: Release 1.0 hỗ trợ Windows, Android, iOS, Web (vẫn `Proposed`, chờ OQ-04).
- 2026-11-15 — ADR-0005 → `Accepted` (tenant = trường / đơn vị / chung, khóa `tenant_id`); ADR-0006 sửa ánh xạ Keycloak Organization theo tenant.
- 2026-11-15 — Mới, `Accepted`: ADR-0026 (đăng nhập theo tenant + tenant chung), ADR-0027 (CMS tách vùng, System Admin liên tenant), ADR-0028 (chỉ soft-delete), ADR-0029 (gán & clone dữ liệu giữa tenant), ADR-0030 (danh mục từ nhạy cảm + công tắc kiểm duyệt tắt được toàn bộ pipeline).
- 2026-11-15 — Mới, `Accepted`: ADR-0031 (game chỉ cho học sinh, lịch sử login, một thiết bị), ADR-0032 (worker dọn học sinh), ADR-0033 (purge chat gameplay, ngoại lệ ADR-0028), ADR-0034 (bộ tính năng chat), ADR-0035 (NATS JetStream + Valkey Pub/Sub cho chat), ADR-0036 (6 microservice, supersede ADR-0001), ADR-0037 (DB riêng mỗi service), ADR-0038 (CQRS + Clean Architecture), ADR-0039 (ent + Atlas), ADR-0040 (typed REST client service-to-service), ADR-0041 (maxLength bắt buộc), ADR-0042 (outbox → NATS JetStream). ADR-0020 sửa: 8 channel, chat gameplay luôn có.
- 2026-11-15 — Mới, `Accepted`: ADR-0043 (phát hành nội dung: Draft/Publish, pinning, rollback, cache; sửa cách rollback của ADR-0015), ADR-0044 (media trên object storage + CDN), ADR-0045 (đăng nhập nhanh ở 10.000 CCU), ADR-0046 (nhận biết SIS nguồn khi đăng nhập).
- 2026-11-15 — ADR-0024 → `Accepted`: chạy trên Docker (Compose local, Swarm staging/production), chưa dùng Kubernetes. Mới, `Accepted`: ADR-0047 (hai hệ thống giao diện: Game Godot 3D và Admin/Ops CMS/LMS). ADR-0022 sửa: CMS dùng Ant Design 5 + Pro với theme EduWorld [Proposed].
- 2026-11-15 — ADR-0027 sửa: một Admin Portal duy nhất, 5 workspace. ADR-0044 sửa: floci cho local/CI; ngoại lệ upload có kiểm soát cho bài nộp của học sinh. Mới: ADR-0048…0054 (`Proposed`: activity registry, reward/currency ledger, party vs guild/club, voice chat, simulation, AI-assisted grading, tương tác liên trường), ADR-0055 (`Accepted`: MathJax 4 + MathLive, game chỉ hiển thị).

- 2026-11-15 — Mới, `Accepted` trừ khi ghi khác: ADR-0056 (song ngữ Việt – Anh), ADR-0057 (máy tầm trung, thay một phần ADR-0013), ADR-0058 (Game Analytics trong PostgreSQL), ADR-0059 (hợp nhất API attempt, chưa làm AI giọng nói), ADR-0060 (danh mục quốc gia/tỉnh/trường, một trường một lớp), ADR-0061 (đồng bộ SIS khi đăng nhập), ADR-0062 (dashboard đọc rollup).
- 2026-11-15 — Mới, `Accepted`: ADR-0063 (đăng nhập username/password + quy tắc tenant; thay một phần ADR-0026, ADR-0045 mục 4, ADR-0046 nhánh IdP), ADR-0064 (không tự đăng nhập; thay ADR-0045 mục 3), ADR-0065 (`X-Tenant-Id` trên header), ADR-0066 (đăng nhập portal riêng, Role/UserRole), ADR-0067 (portal không chọn tenant), ADR-0068 (form đăng nhập trong game qua backend; thay ADR-0007 cho game), ADR-0069 (SSO xác thực trước, kiểm tra tenant sau; một người một tài khoản SSO; username/email không trùng trong tenant; làm rõ ADR-0067 mục 2). Bản nháp ADR-0069 "đối chiếu nhiều tài khoản cùng tên" bị loại cùng ngày.
- 2026-11-15 — Mới, `Accepted`: ADR-0070 (Release 1.0 không voice chat, không xóa vật lý theo luật, không purge dữ liệu analytics thô; xác nhận ADR-0051, chốt ADR-0028 mục 8), ADR-0071 (floci là object storage cho staging/production; thay ADR-0044 mục 1; rủi ro được chấp nhận, RISK-41).

### Answered
- 2026-11-15 — OQ-01 (hướng tích hợp SIS), OQ-09 (chính sách chat), phạm vi Release 1.0 hoàn chỉnh, mô hình tenancy, soft-delete, gán/clone, phân quyền CMS. Câu hỏi còn mở: [OPEN_QUESTIONS](docs/12-project/OPEN_QUESTIONS.md) (đến OQ-22). OQ-03 trả lời một phần: 10.000 CCU.
- 2026-11-15 — Song ngữ; chạy trên máy tầm trung; bố cục Web/Mobile; không AI giọng nói trong 1.0; danh mục tỉnh/trường; đồng bộ SIS khi đăng nhập; dashboard dữ liệu lớn; đăng nhập không chọn trường; không tự đăng nhập; `tenant_id` trên header; portal đăng nhập riêng + Role/UserRole; form đăng nhập trong game; SSO trước, tenant sau (OQ-15, OQ-22 Answered). Còn mở: OQ-19, OQ-20, OQ-21 và các OQ cũ (xem [OPEN_QUESTIONS](docs/12-project/OPEN_QUESTIONS.md)).
- 2026-11-15 — OQ-19 (không voice chat), OQ-20 (không xóa vật lý, chỉ ẩn danh hóa), OQ-21 (floci cho mọi môi trường); không làm ngoại lệ purge analytics thô. Thêm RISK-45 (pháp lý), RISK-46 (dung lượng DB); alert object storage và analytics trong MONITORING.
