# Tasks: F015 — Audit log tập trung và cài đặt platform/tenant/trường

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Khởi tạo package `internal/domain/audit`, `internal/domain/settings` — file: `src/backend/modules/identity/internal/domain/` — **xong 2026-11-19 — `internal/domain/audit`, `internal/domain/settings`**
- [x] T002 [P] Kiểm `audit.Recorder` (F002 T007) + `outbox.AuditRecorder` (F003 T042, F003-AC-22: lọc khóa cấm, giới hạn kích thước) đáp ứng consumer `audit-sink`; thiếu thì bổ sung ở F002/F003, không tạo helper riêng — file: `src/backend/modules/identity/internal/presentation/consumer/audit_sink.go` — **xong 2026-11-19 — `outbox.AuditRecorder` (F003) đã lọc khóa cấm + giới hạn; bổ sung `audit.Event.SchoolID` → `school_id` của payload (lọc School Admin); lớp lọc thứ hai ở sink (`audit.Sanitize`)**

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Thêm `listFeatureFlags`, `putFeatureFlag`, schema `AllowedHours`, `FeatureFlag` (`getSchoolSettings`, `updateSchoolSettings`, `SchoolSettings`, `CurfewWindow`, `getPlatformCurfewSettings`/`putPlatformCurfewSettings` đã có trong spec gộp — chỉ kiểm khớp); sửa `AuditLogEntry`, `TenantSettings.allowed_hours` — file: `specs/openapi/modules/identity.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=identity` — **xong 2026-11-17** (kiểm bằng script `scripts/specs/*`, `scripts/openapi/*`; target `make` chưa có tới F001); `listFeatureFlags`, `putFeatureFlag`, `FeatureFlag`, `updateSchoolSettings` đã có trong spec gộp
- [x] T004 [P] Thêm payload schema `platform_settings.changed` (scope `school`) — `audit.recorded` dùng schema chung `specs/events/payloads/common/audit.recorded.v1.schema.json` của F003 T004 (không tạo) — file: `specs/events/payloads/identity/` — kiểm: `make specs-lint` — **xong 2026-11-17** (kiểm bằng script `scripts/specs/*`, `scripts/openapi/*`; target `make` chưa có tới F001)
- [x] T005 ent schema `AuditLog` (AppendOnly), `PlatformSetting`, `FeatureFlag`; cột `settings_version` cho `platform.tenants`, `org.schools` — file: `src/backend/modules/identity/ent/schema/` — kiểm: `make migrate-diff MOD=identity name=f015_audit_settings`, `make migrate-lint MOD=identity` — **xong 2026-11-19 — bảng viết tay trong `migrations/20261123000001_f015_audit_settings.sql` (partition tháng không DEFAULT, mảng, cột sinh — ent không biểu diễn được; truy cập bằng SQL tường minh); `settings_version` thay bằng cột `version` của dòng tenant/trường — OQ F015-1**
- [x] T006 Migration partition tháng `audit.audit_logs`, RLS TA, `REVOKE UPDATE, DELETE, TRUNCATE`, role `identity_retention` (UPDATE cột ẩn danh), seed `platform_settings` mặc định — file: `src/backend/modules/identity/migrations/` — **xong 2026-11-19 — partition tháng, RLS, REVOKE; `identity_retention` thay bằng hàm SECURITY DEFINER `audit.anonymize_expired` chạy bằng `identity_partition_admin` — OQ F015-2; không seed `platform_settings` (mặc định từ cấu hình, R33); `TestAuditSettingsSchema`**

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Audit log tập trung, append-only (Priority: P1) 🎯 MVP

**Goal**: Mọi audit vào một nhật ký bất biến.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [x] T007 [P] [US1] Test F015-AC-01: event từ identity và game ghi đúng một dòng, redeliver không trùng — file: `src/backend/modules/identity/tests/integration/audit_sink_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `tests/integration/audit_sink_integration_test.go` `TestAuditSink`**
- [x] T008 [P] [US1] Test F015-AC-02, F015-AC-03: UPDATE/DELETE/TRUNCATE bị từ chối với `identity_app`/`identity_platform`; job tạo trước partition, không ghi vào partition thiếu — file: `src/backend/modules/identity/tests/integration/audit_append_only_test.go` — **xong 2026-11-19 — `TestAuditSettingsSchema` (UPDATE/DELETE/TRUNCATE bị từ chối, không partition DEFAULT, job tạo trước partition) + `TestAuditSink` (job)**
- [x] T009 [P] [US1] Test F015-AC-04, F015-AC-05: khóa cấm bị lọc + metric; envelope lệch tenant → `DLQ_IDENTITY` — file: `src/backend/modules/identity/tests/integration/audit_sanitize_test.go` — **xong 2026-11-19 — `TestAuditSink` (khóa cấm + metric; trường của tenant khác → permanent/DLQ — OQ F015-3)**
- [x] T010 [P] [US1] Test F015-AC-06: job retention (Clock giả) ẩn danh hóa bản ghi quá hạn, không DELETE — file: `src/backend/modules/identity/tests/integration/audit_retention_test.go` — **xong 2026-11-19 — `TestAuditSink` (Clock +3 năm, ẩn danh hóa, không DELETE, lần hai 0) + `TestAuditSettingsSchema`**

### Implementation cho User Story 1

- [x] T011 [P] [US1] Domain `AuditEntry.Sanitize` — file: `src/backend/modules/identity/internal/domain/audit/entry.go` — **xong 2026-11-19 — `domain/audit/entry.go` + `entry_test.go`**
- [x] T012 [US1] Command `RecordAuditEntry` + consumer `audit-sink` (durable mỗi stream `EVT_*`, `FilterSubjects` `ew.evt.<module>.audit.recorded`) — file: `src/backend/modules/identity/internal/presentation/consumer/audit_sink.go` — **xong 2026-11-19 — `command/audit_sink.go` + `presentation/consumer/audit_sink.go` (durable `identity.audit_sink`, `AuditSinkSubject(module)`)**
- [x] T013 [US1] Job `partition-maintainer` (audit, `identity.login_events`, `sis.staging_records`, `sis.sync_record_errors`) và `audit-retention` — file: `src/backend/modules/identity/internal/presentation/job/partition_maintainer.go`, `src/backend/modules/identity/internal/presentation/job/audit_retention.go` — **xong 2026-11-19 — `command.AuditMaintenance` + `presentation/job/audit.go` + `infrastructure/auditmaint` (audit + `identity.login_events`; `sis.*` giữ partition DEFAULT của F011 — OQ F015-7)**

## Phase 4: User Story 2 — Truy vấn audit trong phạm vi (Priority: P1)

### Tests cho User Story 2

- [x] T014 [P] [US2] Test tenant isolation + authorization F015-AC-07, F015-AC-08, F015-AC-09: School Admin chỉ trường mình; người dùng tenant gửi `tenant_ids` bị bỏ qua; System Admin + audit `cross_tenant.accessed`; Content Admin → 403 — file: `src/backend/modules/identity/tests/integration/audit_query_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `tests/integration/audit_query_integration_test.go` `TestAuditLogQueries`**
- [x] T015 [P] [US2] Test F015-AC-10: khoảng thời gian vượt giới hạn → 422; response không PII — file: `src/backend/modules/identity/tests/integration/audit_query_validation_test.go` — **xong 2026-11-19 — `TestAuditLogQueries` (thiếu/quá khoảng → 422) + `domain/audit` `TestValidateAndRange`**

### Implementation cho User Story 2

- [x] T016 [US2] Query `ListTenantAuditLogs`, `ListPlatformAuditLogs` (SQL tường minh có `tenant_id`, khoảng thời gian, cursor) + handler — file: `src/backend/modules/identity/internal/application/query/audit_logs.go`, `src/backend/modules/identity/internal/presentation/http/public/audit.go` — **xong 2026-11-19 — `query/audit_logs.go`, `persistence/read/audit.go`, `presentation/http/settings.go`**

## Phase 5: User Story 3 — Cài đặt platform và cài đặt tenant (Priority: P2)

### Tests cho User Story 3

- [x] T017 [P] [US3] Test F015-AC-11, F015-AC-12: ghi cài đặt platform cần `If-Match` + `reason`, audit trước/sau, `platform_settings.changed`; retention < tối thiểu → `RETENTION_BELOW_MINIMUM` — file: `src/backend/modules/identity/tests/integration/platform_settings_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `tests/integration/settings_integration_test.go` `TestPlatformAndTenantSettings`**
- [x] T018 [P] [US3] Test F015-AC-13, F015-AC-14, F015-AC-15: ghi đè bị cấm → 403; giá trị hiệu lực ở `getTenantSettings` và `getTenant`; `If-Match` sai → 412; tenant khác → 404 — file: `src/backend/modules/identity/tests/integration/tenant_settings_test.go` — **xong 2026-11-19 — `TestPlatformAndTenantSettings` (403 khi cấm ghi đè, giá trị hiệu lực, 412, tenant khác); `getTenant` (internal) chưa hiện thực — OQ F015-6**
- [x] T031 [P] [US3] Test F015-AC-21: System Admin đổi `platform_curfew` có `reason` + `If-Match` → lưu, audit, `platform_settings.changed`; role tenant → 403; chưa cấu hình → mặc định CONFIG_DEFAULTS — file: `src/backend/modules/identity/tests/integration/platform_curfew_settings_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `TestPlatformAndTenantSettings` (mặc định CONFIG_DEFAULTS, `reason` + If-Match, audit, event, role tenant 403)**

### Implementation cho User Story 3

- [x] T019 [P] [US3] Domain `PlatformSettings`, `TenantSettings`, `EffectiveValue` — file: `src/backend/modules/identity/internal/domain/settings/` — **xong 2026-11-19 — `domain/settings/settings.go` + test**
- [x] T020 [US3] Command `PutStudentCleanupSettings`, `PutChatRetentionSettings`, `UpdateTenantSettings`; query tương ứng; handler — file: `src/backend/modules/identity/internal/application/command/settings.go`, `src/backend/modules/identity/internal/presentation/http/public/settings.go` — **xong 2026-11-19 — `command/settings.go`, `query/settings.go`, handler trong `presentation/http/settings.go`**
- [x] T032 [US3] Khóa cài đặt `platform_curfew` + handler `getPlatformCurfewSettings`/`putPlatformCurfewSettings` (schema JSON settings, giá trị mặc định, validate khung qua nửa đêm) + payload `platform_settings.changed` có khung giới nghiêm — file: `src/backend/modules/identity/internal/domain/settings/curfew.go`, `specs/events/payloads/identity/platform_settings.changed.v1.schema.json` — **2026-11-17:** phần contract đã áp (payload `platform_settings.changed` có `curfew_window`); còn domain/handler — **xong 2026-11-19 — khóa `platform_curfew`, `getPlatformCurfewSettings`/`putPlatformCurfewSettings`, payload `curfew_window`**

## Phase 6: User Story 4 — Cài đặt trường (Priority: P2)

### Tests cho User Story 4

- [x] T021 [P] [US4] Test F015-AC-16, F015-AC-17, F015-AC-19: schema khung giờ/chính sách, audit, `school.updated`; trường khác → 403/404; không nới lỏng giới hạn platform — file: `src/backend/modules/identity/tests/integration/school_settings_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `tests/integration/school_settings_integration_test.go` `TestSchoolSettings`**
- [x] T022 [P] [US4] Test F015-AC-18: `gameLogin` ngoài khung giờ (Clock giả) → `ACCESS_OUTSIDE_ALLOWED_HOURS` đúng thứ tự kiểm tra — file: `src/backend/modules/identity/tests/integration/access_hours_login_test.go` — **xong 2026-11-19 — `TestGameLoginSchoolPolicy` (ngoài khung giờ → `ACCESS_OUTSIDE_ALLOWED_HOURS`, nhật ký `access_outside_allowed_hours`)**
- [x] T033 [P] [US4] Test F015-AC-22: khung trường hẹp hơn/ lệch `platform_curfew` → `422 VALIDATION_FAILED`; khung rộng hơn → lưu; giới nghiêm hiệu lực = hợp hai khung; tenant `common` chỉ `platform_curfew`; bootstrap `policies.play_time` có khung hiệu lực (Clock giả, ICT) — file: `src/backend/modules/identity/tests/integration/school_curfew_settings_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `TestSchoolSettings` (khung hẹp → 422, rộng hơn → lưu, hợp hai khung) + `TestGameLoginSchoolPolicy` (giới nghiêm trường khi đăng nhập) + `domain/settings` `TestCurfewRules`**

### Implementation cho User Story 4

- [x] T023 [P] [US4] Domain `SchoolSettings`, `AllowedHours.Contains` — file: `src/backend/modules/identity/internal/domain/settings/school.go` — **xong 2026-11-19 — `domain/settings` `AllowedHours.Contains/Within`**
- [x] T024 [US4] Command `UpdateSchoolSettings`; query `GetSchoolSettings`; handler `getSchoolSettings`, `updateSchoolSettings` — file: `src/backend/modules/identity/internal/presentation/http/public/school_settings.go` — **xong 2026-11-19 — `command/school_settings.go`, `query.Settings.SchoolSettings`, handler**
- [x] T025 [US4] Hook kiểm khung giờ trong `GameLogin` (F013) — file: `src/backend/modules/identity/internal/application/command/game_login.go` — **xong 2026-11-19 — `GameAuth.applyHours` (login, chọn tenant, resume không thu hồi phiên) qua `gameauth.SettingsPolicy`; migration `20261123000002_f015_login_result.sql`**
- [x] T034 [US4] `EffectiveCurfew` (domain thuần, hợp khung platform + trường, múi giờ tenant) dùng chung cho login (F013) và bootstrap; khóa cài đặt trường `curfew_window` — file: `src/backend/modules/identity/internal/domain/settings/effective_curfew.go`, `src/backend/modules/identity/internal/application/command/update_school_settings.go` — **2026-11-17:** phần contract đã áp (`PlayerBootstrap.policies.play_time.curfew` {`start`, `end`, `warning_minutes`}); còn `EffectiveCurfew` + command — **xong 2026-11-19 — `settings.Union`/`Contains` + `gameauth.SettingsPolicy.EffectiveCurfew` (thay `PlatformCurfew` của F013)**

## Phase 7: User Story 5 — Feature flag platform (Priority: P3)

### Tests cho User Story 5

- [x] T026 [P] [US5] Test F015-AC-20: flag mặc định + ghi đè tenant, audit, bootstrap phản ánh ≤ TTL, authorization `platform.feature_flag.write` — file: `src/backend/modules/identity/tests/integration/feature_flags_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `tests/integration/feature_flags_integration_test.go` `TestFeatureFlagsAndMenuCatalog`**

### Implementation cho User Story 5

- [x] T027 [US5] Command `PutFeatureFlag`; query `ListFeatureFlags`; đọc flag hiệu lực (chỉ `flag_kind = feature`) trong `BootstrapReader` — file: `src/backend/modules/identity/internal/application/command/feature_flags.go` — **xong 2026-11-19 — `command/feature_flags.go`, `query/feature_flags.go`, cờ `feature` trong `BootstrapReader`**

### Bổ sung ADR-0090 — bật/tắt menu/action của Admin Portal

- [x] T035 [US5] Kiểm khớp contract `listFeatureFlags`, `putFeatureFlag`, `listPortalMenuCatalog`, schema `FeatureFlag`/`FeatureFlagWrite`/`PortalMenuCatalog` trong spec gộp; `make gen MOD=identity` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `node scripts/specs/check-portal-catalog.mjs` — **xong 2026-11-19 — contract khớp: `tests/contract/settings_routes_test.go` (15 operation F015)**
- [x] T036 [P] [US5] Test F015-AC-23, F015-AC-24: `src/backend/cmd/migrate` đồng bộ catalog (idempotent, mục bị gỡ → soft-delete), `identity_app` ghi catalog bị từ chối; FK/CHECK chặn flag menu/action có khóa ngoài catalog hoặc action thiếu `parent_key` (TC-CMS-MENU-01, TC-CMS-MENU-03) — file: `src/backend/modules/identity/tests/integration/portal_catalog_sync_test.go` — phải FAIL trước khi code — **xong 2026-11-19 — `TestAuditSettingsSchema` (app không ghi catalog, FK/CHECK) + `TestFeatureFlagsAndMenuCatalog` (đồng bộ lại idempotent, mục bị gỡ → soft-delete kèm flag)**
- [x] T037 [P] [US5] Test F015-AC-25, F015-AC-26, F015-AC-28: `putFeatureFlag` platform/tenant/`inherit` + audit + `platform_settings.changed`; mục bảo vệ (sinh từ catalog) → `422 FEATURE_NOT_DISABLEABLE`; thiếu `reason`/`If-Match`, lệch version, khóa lạ, role khác, thiếu MFA; `listPortalMenuCatalog`/`listFeatureFlags` theo tenant + audit `cross_tenant.accessed` (TC-CMS-MENU-02, TC-CMS-MENU-03) — file: `src/backend/modules/identity/tests/integration/portal_toggles_test.go` — **xong 2026-11-19 — `TestFeatureFlagsAndMenuCatalog`**
- [x] T038 [P] [US5] Test F015-AC-27, F015-AC-29: unit `PortalToggles.Effective` (bảng quyết định: không dòng, mặc định, ghi đè, action AND menu, phạm vi platform); tích hợp `featuregate.Source` + cache Valkey làm mới ≤ `feature_toggle_cache_ttl`, tenant khác không đổi; flag menu/action không có trong `PlayerBootstrap` (TC-CMS-MENU-02, TC-CMS-MENU-04, TC-CMS-MENU-06) — file: `src/backend/modules/identity/internal/domain/settings/portal_toggles_test.go`, `src/backend/modules/identity/tests/integration/featuregate_source_test.go` — **xong 2026-11-19 — `domain/settings` `TestToggles` + `TestFeatureFlagsAndMenuCatalog` (Source + Valkey, `getMyPermissions`, bootstrap)**
- [x] T039 [US5] ent schema `PortalMenuCatalogItem` (đọc-chỉ) + cột `flag_kind`, `parent_key`, `catalog_key` (generated) cho `FeatureFlag`, `flag_key` → `varchar(128)`; migration FK, CHECK, `REVOKE` ghi catalog khỏi `identity_app` — file: `src/backend/modules/identity/ent/schema/portalmenucatalogitem.go`, `src/backend/modules/identity/migrations/` — kiểm: `make migrate-diff MOD=identity name=f015_portal_toggles`, `make migrate-lint MOD=identity` — **xong 2026-11-19 — bảng `platform.portal_menu_catalog`, `platform.feature_flags` (FK, CHECK, cột sinh) trong migration F015**
- [x] T040 [US5] `src/backend/cmd/migrate` đồng bộ `specs/portal/menu-catalog.yaml` (validate schema, upsert theo `item_key`, `catalog_version`, soft-delete mục bị gỡ + flag liên quan) — file: `src/backend/modules/identity/cmd/migrate/sync_portal_catalog.go` — **xong 2026-11-19 — `identity.SyncPortalCatalog` + `persistence/seed/portal_catalog.go`, gọi từ `cmd/migrate` (`EW_PORTAL_MENU_CATALOG`)**
- [x] T041 [US5] Domain `PortalToggles`; command `PutFeatureFlag` mở rộng (suy `flag_kind`/`parent_key` từ catalog, `scope`/`state`, `can_disable`, audit, outbox `platform_settings.changed` scope `feature_flag`); query `ListPortalMenuCatalog`; handler — file: `src/backend/modules/identity/internal/domain/settings/portal_toggles.go`, `src/backend/modules/identity/internal/application/command/feature_flags.go`, `src/backend/modules/identity/internal/presentation/http/public/portal_menu_catalog.go` — **xong 2026-11-19 — `domain/settings/toggles.go`, `command.FeatureFlags.Put`, `query.FeatureFlags.Catalog`, handler**
- [x] T042 [US5] Hiện thực `featuregate.Source` (port của F012 trong `src/backend/pkg/httpx`): đọc catalog + flag của module identity, cache `ew:fg:{tenant_id|platform}` TTL `feature_toggle_cache_ttl`, consumer xóa cache khi `platform_settings.changed` scope `feature_flag`; đăng ký trong `module.go` để `src/backend/cmd/core-api` gắn vào middleware — file: `src/backend/modules/identity/internal/infrastructure/featuregate/source.go` — **xong 2026-11-19 — `infrastructure/featuretoggle/source.go` (featuregate.Source + port.FeatureToggles, cache `ew:fg:<gen>:<scope>`) + consumer `presentation/consumer/feature_toggle_cache.go`**

## Phase 8: Polish & Cross-Cutting

- [x] T028 [P] Cập nhật DATABASE_DESIGN (§3.11 `actor_type`, §2.15 cột `platform.feature_flags` gồm `flag_kind`/`parent_key` và bảng `platform.portal_menu_catalog` nếu đợt ADR-0090 chưa ghi, `settings_version`, role `identity_retention`), CONFIG_DEFAULTS (`audit_log_retention`, giới hạn khoảng truy vấn audit), ADMIN_PORTAL §2.2 (gỡ `platform.audit.read_all`), CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/00-overview/CONFIG_DEFAULTS.md` — **xong 2026-11-19 — DATABASE_DESIGN §3.11 (`producer`, actor_type), hàm SECURITY DEFINER thay role `identity_retention`**
- [x] T029 [P] Metric `eduworld_audit_*`, `eduworld_partition_missing_total`; alert partition — file: `src/backend/modules/identity/internal/infrastructure/observability/audit_metrics.go` — **xong 2026-11-19 — `infrastructure/auditmaint` (`eduworld_audit_records_total`, `eduworld_audit_sanitized_fields_total`, `eduworld_partition_missing_total`); MONITORING**
- [x] T030 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — **xong 2026-11-19 (local, quickstart §5)**

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Đã làm 2026-11-20.

- [x] T043 **OQ chốt 2026-11-20** (OQ-17, F015-AC-30) `SchoolSettings.daily_play_limit_minutes` (contract + lưu + audit + `school.updated` payload thêm field tùy chọn); test F015-AC-30 — file: `specs/openapi/modules/identity.v1.yaml`, `specs/events/payloads/identity/school.updated.v1.schema.json`, `src/backend/modules/identity/internal/application/command/settings.go` — kiểm: `make specs-lint`, `make test-integration MOD=identity` — cỡ S — ✅ 2026-11-20: `SchoolSettings.daily_play_limit_minutes` (15–300 phút [Confirmed], `null` = tắt; đổi giá trị cần `reason`, audit `school.settings_updated`), `school.updated.settings_subset` mang giá trị; domain `settings.ValidateDailyPlayLimit`; test `TestSchoolSettings` (F015-AC-30), `TestValidateDailyPlayLimit` PASS
- [ ] T044 **Nhận từ F095** (A-1, phạm vi kết bạn) Payload `tenant.updated` mang `settings_subset` gồm `cross_school_social_enabled` và `common_social_enabled` (contract + emit trong command cập nhật tenant settings + test contract/integration) để projection `ref.tenants.settings_subset` của module chat bật/tắt kết bạn liên trường từ đầu đến cuối — file: `specs/events/payloads/identity/tenant.updated.v1.schema.json`, `src/backend/modules/identity/internal/application/command/` — kiểm: `make specs-lint`, `make test-integration MOD=identity` — cỡ S

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → US3, US4 song song → US5 → Polish.
- T039 trước T040–T042; T042 cần port `featuregate.Source` của F012 (T053) — có thể làm song song với interface đã chốt; US5 (ADR-0090) trước F051 (menu sinh từ catalog), F059 (màn bật/tắt) và các workspace F052+.
- `src/backend/pkg/audit` (F002) + `outbox.AuditRecorder` (F003) có từ M-C0 nên F010–F014 dùng chung; F015 chỉ thêm consumer `audit-sink`.

## Definition of Done

- [x] Mọi tiêu chí F015-AC-* có test pass (ghi mã test/TC)
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh cho module `identity` — local xanh (lệnh tương đương)
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
