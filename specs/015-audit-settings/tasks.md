# Tasks: F015 — Audit log tập trung và cài đặt platform/tenant/trường

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Khởi tạo package `internal/domain/audit`, `internal/domain/settings` — file: `src/backend/modules/identity/internal/domain/`
- [ ] T002 [P] Kiểm `audit.Recorder` (F002 T007) + `outbox.AuditRecorder` (F003 T042, F003-AC-22: lọc khóa cấm, giới hạn kích thước) đáp ứng consumer `audit-sink`; thiếu thì bổ sung ở F002/F003, không tạo helper riêng — file: `src/backend/modules/identity/internal/presentation/consumer/audit_sink.go`

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Thêm `listFeatureFlags`, `putFeatureFlag`, schema `AllowedHours`, `FeatureFlag` (`getSchoolSettings`, `updateSchoolSettings`, `SchoolSettings`, `CurfewWindow`, `getPlatformCurfewSettings`/`putPlatformCurfewSettings` đã có trong spec gộp — chỉ kiểm khớp); sửa `AuditLogEntry`, `TenantSettings.allowed_hours` — file: `specs/openapi/modules/identity.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=identity` — **xong 2026-11-17** (kiểm bằng script `scripts/specs/*`, `scripts/openapi/*`; target `make` chưa có tới F001); `listFeatureFlags`, `putFeatureFlag`, `FeatureFlag`, `updateSchoolSettings` đã có trong spec gộp
- [x] T004 [P] Thêm payload schema `platform_settings.changed` (scope `school`) — `audit.recorded` dùng schema chung `specs/events/payloads/common/audit.recorded.v1.schema.json` của F003 T004 (không tạo) — file: `specs/events/payloads/identity/` — kiểm: `make specs-lint` — **xong 2026-11-17** (kiểm bằng script `scripts/specs/*`, `scripts/openapi/*`; target `make` chưa có tới F001)
- [ ] T005 ent schema `AuditLog` (AppendOnly), `PlatformSetting`, `FeatureFlag`; cột `settings_version` cho `platform.tenants`, `org.schools` — file: `src/backend/modules/identity/ent/schema/` — kiểm: `make migrate-diff MOD=identity name=f015_audit_settings`, `make migrate-lint MOD=identity`
- [ ] T006 Migration partition tháng `audit.audit_logs`, RLS TA, `REVOKE UPDATE, DELETE, TRUNCATE`, role `identity_retention` (UPDATE cột ẩn danh), seed `platform_settings` mặc định — file: `src/backend/modules/identity/migrations/`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Audit log tập trung, append-only (Priority: P1) 🎯 MVP

**Goal**: Mọi audit vào một nhật ký bất biến.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [ ] T007 [P] [US1] Test F015-AC-01: event từ identity và game ghi đúng một dòng, redeliver không trùng — file: `src/backend/modules/identity/tests/integration/audit_sink_test.go` — phải FAIL trước khi code
- [ ] T008 [P] [US1] Test F015-AC-02, F015-AC-03: UPDATE/DELETE/TRUNCATE bị từ chối với `identity_app`/`identity_platform`; job tạo trước partition, không ghi vào partition thiếu — file: `src/backend/modules/identity/tests/integration/audit_append_only_test.go`
- [ ] T009 [P] [US1] Test F015-AC-04, F015-AC-05: khóa cấm bị lọc + metric; envelope lệch tenant → `DLQ_IDENTITY` — file: `src/backend/modules/identity/tests/integration/audit_sanitize_test.go`
- [ ] T010 [P] [US1] Test F015-AC-06: job retention (Clock giả) ẩn danh hóa bản ghi quá hạn, không DELETE — file: `src/backend/modules/identity/tests/integration/audit_retention_test.go`

### Implementation cho User Story 1

- [ ] T011 [P] [US1] Domain `AuditEntry.Sanitize` — file: `src/backend/modules/identity/internal/domain/audit/entry.go`
- [ ] T012 [US1] Command `RecordAuditEntry` + consumer `audit-sink` (durable mỗi stream `EVT_*`, `FilterSubjects` `ew.evt.<module>.audit.recorded`) — file: `src/backend/modules/identity/internal/presentation/consumer/audit_sink.go`
- [ ] T013 [US1] Job `partition-maintainer` (audit, `identity.login_events`, `sis.staging_records`, `sis.sync_record_errors`) và `audit-retention` — file: `src/backend/modules/identity/internal/presentation/job/partition_maintainer.go`, `src/backend/modules/identity/internal/presentation/job/audit_retention.go`

## Phase 4: User Story 2 — Truy vấn audit trong phạm vi (Priority: P1)

### Tests cho User Story 2

- [ ] T014 [P] [US2] Test tenant isolation + authorization F015-AC-07, F015-AC-08, F015-AC-09: School Admin chỉ trường mình; người dùng tenant gửi `tenant_ids` bị bỏ qua; System Admin + audit `cross_tenant.accessed`; Content Admin → 403 — file: `src/backend/modules/identity/tests/integration/audit_query_test.go` — phải FAIL trước khi code
- [ ] T015 [P] [US2] Test F015-AC-10: khoảng thời gian vượt giới hạn → 422; response không PII — file: `src/backend/modules/identity/tests/integration/audit_query_validation_test.go`

### Implementation cho User Story 2

- [ ] T016 [US2] Query `ListTenantAuditLogs`, `ListPlatformAuditLogs` (SQL tường minh có `tenant_id`, khoảng thời gian, cursor) + handler — file: `src/backend/modules/identity/internal/application/query/audit_logs.go`, `src/backend/modules/identity/internal/presentation/http/public/audit.go`

## Phase 5: User Story 3 — Cài đặt platform và cài đặt tenant (Priority: P2)

### Tests cho User Story 3

- [ ] T017 [P] [US3] Test F015-AC-11, F015-AC-12: ghi cài đặt platform cần `If-Match` + `reason`, audit trước/sau, `platform_settings.changed`; retention < tối thiểu → `RETENTION_BELOW_MINIMUM` — file: `src/backend/modules/identity/tests/integration/platform_settings_test.go` — phải FAIL trước khi code
- [ ] T018 [P] [US3] Test F015-AC-13, F015-AC-14, F015-AC-15: ghi đè bị cấm → 403; giá trị hiệu lực ở `getTenantSettings` và `getTenant`; `If-Match` sai → 412; tenant khác → 404 — file: `src/backend/modules/identity/tests/integration/tenant_settings_test.go`
- [ ] T031 [P] [US3] Test F015-AC-21: System Admin đổi `platform_curfew` có `reason` + `If-Match` → lưu, audit, `platform_settings.changed`; role tenant → 403; chưa cấu hình → mặc định CONFIG_DEFAULTS — file: `src/backend/modules/identity/tests/integration/platform_curfew_settings_test.go` — phải FAIL trước khi code

### Implementation cho User Story 3

- [ ] T019 [P] [US3] Domain `PlatformSettings`, `TenantSettings`, `EffectiveValue` — file: `src/backend/modules/identity/internal/domain/settings/`
- [ ] T020 [US3] Command `PutStudentCleanupSettings`, `PutChatRetentionSettings`, `UpdateTenantSettings`; query tương ứng; handler — file: `src/backend/modules/identity/internal/application/command/settings.go`, `src/backend/modules/identity/internal/presentation/http/public/settings.go`
- [ ] T032 [US3] Khóa cài đặt `platform_curfew` + handler `getPlatformCurfewSettings`/`putPlatformCurfewSettings` (schema JSON settings, giá trị mặc định, validate khung qua nửa đêm) + payload `platform_settings.changed` có khung giới nghiêm — file: `src/backend/modules/identity/internal/domain/settings/curfew.go`, `specs/events/payloads/identity/platform_settings.changed.v1.schema.json` — **2026-11-17:** phần contract đã áp (payload `platform_settings.changed` có `curfew_window`); còn domain/handler

## Phase 6: User Story 4 — Cài đặt trường (Priority: P2)

### Tests cho User Story 4

- [ ] T021 [P] [US4] Test F015-AC-16, F015-AC-17, F015-AC-19: schema khung giờ/chính sách, audit, `school.updated`; trường khác → 403/404; không nới lỏng giới hạn platform — file: `src/backend/modules/identity/tests/integration/school_settings_test.go` — phải FAIL trước khi code
- [ ] T022 [P] [US4] Test F015-AC-18: `gameLogin` ngoài khung giờ (Clock giả) → `ACCESS_OUTSIDE_ALLOWED_HOURS` đúng thứ tự kiểm tra — file: `src/backend/modules/identity/tests/integration/access_hours_login_test.go`
- [ ] T033 [P] [US4] Test F015-AC-22: khung trường hẹp hơn/ lệch `platform_curfew` → `422 VALIDATION_FAILED`; khung rộng hơn → lưu; giới nghiêm hiệu lực = hợp hai khung; tenant `common` chỉ `platform_curfew`; bootstrap `policies.play_time` có khung hiệu lực (Clock giả, ICT) — file: `src/backend/modules/identity/tests/integration/school_curfew_settings_test.go` — phải FAIL trước khi code

### Implementation cho User Story 4

- [ ] T023 [P] [US4] Domain `SchoolSettings`, `AllowedHours.Contains` — file: `src/backend/modules/identity/internal/domain/settings/school.go`
- [ ] T024 [US4] Command `UpdateSchoolSettings`; query `GetSchoolSettings`; handler `getSchoolSettings`, `updateSchoolSettings` — file: `src/backend/modules/identity/internal/presentation/http/public/school_settings.go`
- [ ] T025 [US4] Hook kiểm khung giờ trong `GameLogin` (F013) — file: `src/backend/modules/identity/internal/application/command/game_login.go`
- [ ] T034 [US4] `EffectiveCurfew` (domain thuần, hợp khung platform + trường, múi giờ tenant) dùng chung cho login (F013) và bootstrap; khóa cài đặt trường `curfew_window` — file: `src/backend/modules/identity/internal/domain/settings/effective_curfew.go`, `src/backend/modules/identity/internal/application/command/update_school_settings.go` — **2026-11-17:** phần contract đã áp (`PlayerBootstrap.policies.play_time.curfew` {`start`, `end`, `warning_minutes`}); còn `EffectiveCurfew` + command

## Phase 7: User Story 5 — Feature flag platform (Priority: P3)

### Tests cho User Story 5

- [ ] T026 [P] [US5] Test F015-AC-20: flag mặc định + ghi đè tenant, audit, bootstrap phản ánh ≤ TTL, authorization `platform.feature_flag.write` — file: `src/backend/modules/identity/tests/integration/feature_flags_test.go` — phải FAIL trước khi code

### Implementation cho User Story 5

- [ ] T027 [US5] Command `PutFeatureFlag`; query `ListFeatureFlags`; đọc flag hiệu lực (chỉ `flag_kind = feature`) trong `BootstrapReader` — file: `src/backend/modules/identity/internal/application/command/feature_flags.go`

### Bổ sung ADR-0090 — bật/tắt menu/action của Admin Portal

- [ ] T035 [US5] Kiểm khớp contract `listFeatureFlags`, `putFeatureFlag`, `listPortalMenuCatalog`, schema `FeatureFlag`/`FeatureFlagWrite`/`PortalMenuCatalog` trong spec gộp; `make gen MOD=identity` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `node scripts/specs/check-portal-catalog.mjs`
- [ ] T036 [P] [US5] Test F015-AC-23, F015-AC-24: `src/backend/cmd/migrate` đồng bộ catalog (idempotent, mục bị gỡ → soft-delete), `identity_app` ghi catalog bị từ chối; FK/CHECK chặn flag menu/action có khóa ngoài catalog hoặc action thiếu `parent_key` (TC-CMS-MENU-01, TC-CMS-MENU-03) — file: `src/backend/modules/identity/tests/integration/portal_catalog_sync_test.go` — phải FAIL trước khi code
- [ ] T037 [P] [US5] Test F015-AC-25, F015-AC-26, F015-AC-28: `putFeatureFlag` platform/tenant/`inherit` + audit + `platform_settings.changed`; mục bảo vệ (sinh từ catalog) → `422 FEATURE_NOT_DISABLEABLE`; thiếu `reason`/`If-Match`, lệch version, khóa lạ, role khác, thiếu MFA; `listPortalMenuCatalog`/`listFeatureFlags` theo tenant + audit `cross_tenant.accessed` (TC-CMS-MENU-02, TC-CMS-MENU-03) — file: `src/backend/modules/identity/tests/integration/portal_toggles_test.go`
- [ ] T038 [P] [US5] Test F015-AC-27, F015-AC-29: unit `PortalToggles.Effective` (bảng quyết định: không dòng, mặc định, ghi đè, action AND menu, phạm vi platform); tích hợp `featuregate.Source` + cache Valkey làm mới ≤ `feature_toggle_cache_ttl`, tenant khác không đổi; flag menu/action không có trong `PlayerBootstrap` (TC-CMS-MENU-02, TC-CMS-MENU-04, TC-CMS-MENU-06) — file: `src/backend/modules/identity/internal/domain/settings/portal_toggles_test.go`, `src/backend/modules/identity/tests/integration/featuregate_source_test.go`
- [ ] T039 [US5] ent schema `PortalMenuCatalogItem` (đọc-chỉ) + cột `flag_kind`, `parent_key`, `catalog_key` (generated) cho `FeatureFlag`, `flag_key` → `varchar(128)`; migration FK, CHECK, `REVOKE` ghi catalog khỏi `identity_app` — file: `src/backend/modules/identity/ent/schema/portalmenucatalogitem.go`, `src/backend/modules/identity/migrations/` — kiểm: `make migrate-diff MOD=identity name=f015_portal_toggles`, `make migrate-lint MOD=identity`
- [ ] T040 [US5] `src/backend/cmd/migrate` đồng bộ `specs/portal/menu-catalog.yaml` (validate schema, upsert theo `item_key`, `catalog_version`, soft-delete mục bị gỡ + flag liên quan) — file: `src/backend/modules/identity/cmd/migrate/sync_portal_catalog.go`
- [ ] T041 [US5] Domain `PortalToggles`; command `PutFeatureFlag` mở rộng (suy `flag_kind`/`parent_key` từ catalog, `scope`/`state`, `can_disable`, audit, outbox `platform_settings.changed` scope `feature_flag`); query `ListPortalMenuCatalog`; handler — file: `src/backend/modules/identity/internal/domain/settings/portal_toggles.go`, `src/backend/modules/identity/internal/application/command/feature_flags.go`, `src/backend/modules/identity/internal/presentation/http/public/portal_menu_catalog.go`
- [ ] T042 [US5] Hiện thực `featuregate.Source` (port của F012 trong `src/backend/pkg/httpx`): đọc catalog + flag của module identity, cache `ew:fg:{tenant_id|platform}` TTL `feature_toggle_cache_ttl`, consumer xóa cache khi `platform_settings.changed` scope `feature_flag`; đăng ký trong `module.go` để `src/backend/cmd/core-api` gắn vào middleware — file: `src/backend/modules/identity/internal/infrastructure/featuregate/source.go`

## Phase 8: Polish & Cross-Cutting

- [ ] T028 [P] Cập nhật DATABASE_DESIGN (§3.11 `actor_type`, §2.15 cột `platform.feature_flags` gồm `flag_kind`/`parent_key` và bảng `platform.portal_menu_catalog` nếu đợt ADR-0090 chưa ghi, `settings_version`, role `identity_retention`), CONFIG_DEFAULTS (`audit_log_retention`, giới hạn khoảng truy vấn audit), ADMIN_PORTAL §2.2 (gỡ `platform.audit.read_all`), CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/00-overview/CONFIG_DEFAULTS.md`
- [ ] T029 [P] Metric `eduworld_audit_*`, `eduworld_partition_missing_total`; alert partition — file: `src/backend/modules/identity/internal/infrastructure/observability/audit_metrics.go`
- [ ] T030 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → US3, US4 song song → US5 → Polish.
- T039 trước T040–T042; T042 cần port `featuregate.Source` của F012 (T053) — có thể làm song song với interface đã chốt; US5 (ADR-0090) trước F051 (menu sinh từ catalog), F059 (màn bật/tắt) và các workspace F052+.
- `src/backend/pkg/audit` (F002) + `outbox.AuditRecorder` (F003) có từ M-C0 nên F010–F014 dùng chung; F015 chỉ thêm consumer `audit-sink`.

## Definition of Done

- [ ] Mọi tiêu chí F015-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh cho module `identity`
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
