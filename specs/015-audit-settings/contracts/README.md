# Contracts: F015 — Audit log tập trung và cài đặt platform/tenant/trường

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/modules/identity.v1.yaml`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listTenantAuditLogs` | `GET /api/v1/tenant/audit-logs?school_id=&actor_id=&action=&from=&to=` | identity.v1.yaml | `tenant.audit.read` (School Admin: trường mình) | Có sẵn |
| `listPlatformAuditLogs` | `GET /api/v1/admin/audit-logs?tenant_ids=&…` | identity.v1.yaml | `platform.audit.read` (MFA, gateway nội bộ) | Có sẵn (schema `AuditLogEntry` Cần sửa) |
| `getStudentCleanupSettings` · `putStudentCleanupSettings` | `GET/PUT /api/v1/admin/settings/student-cleanup` | identity.v1.yaml | `platform.settings.write` | Có sẵn |
| `getChatRetentionSettings` · `putChatRetentionSettings` | `GET/PUT /api/v1/admin/settings/chat-retention` | identity.v1.yaml | `platform.settings.write` | Có sẵn |
| `getTenantSettings` · `updateTenantSettings` | `GET/PATCH /api/v1/tenant/settings` (`If-Match`) | identity.v1.yaml | `tenant.settings.read` / `tenant.settings.write` (+ `tenant.student_cleanup.configure`, `tenant.chat_retention.configure`) | Có sẵn |
| `getSchoolSettings` | `GET /api/v1/schools/{school_id}/settings` | identity.v1.yaml | `school.settings.read` (scope trường) | Có sẵn (`SchoolSettings`, ETag; gồm `platform_curfew`, `effective_curfew` chỉ đọc) |
| `getPlatformCurfewSettings` · `putPlatformCurfewSettings` | `GET/PUT /api/v1/admin/settings/curfew` (`If-Match`, `reason`) | identity.v1.yaml | `platform.settings.write` (gateway nội bộ + MFA) | Có sẵn (`PlatformCurfewSettings`, F015-AC-21) |
| `updateSchoolSettings` | `PATCH /api/v1/schools/{school_id}/settings` (`If-Match`, `reason`) | identity.v1.yaml | `school.settings.write` (scope trường) | Cần thêm |
| `listFeatureFlags` · `putFeatureFlag` | `GET /api/v1/admin/feature-flags?flag_kind=&tenant_id=` · `PUT /api/v1/admin/feature-flags/{flag_key}` (body `{scope, state, reason}`, `If-Match`, `X-Tenant-Id` đích khi `scope = tenant`) | spec gộp `eduworld-api.v1.yaml` → identity.v1.yaml (module identity, `x-internal`) | `platform.feature_flag.write` (gateway nội bộ + MFA) | Có trong spec gộp (ADR-0090) — F015 kiểm khớp; lỗi `422 FEATURE_NOT_DISABLEABLE`, `412`, `428`, `404` |
| `listPortalMenuCatalog` | `GET /api/v1/admin/portal-menu-catalog?tenant_id=` | spec gộp → identity.v1.yaml | `platform.feature_flag.write` | Có trong spec gộp (ADR-0090) — cây workspace/menu/action + trạng thái + `catalog_version` |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `getTenant` | mọi module → facade `identity` (settings hiệu lực; realtime qua NATS request/reply) | identity.internal.v1.yaml | learning, game, competition, chat, realtime | Có sẵn |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không có | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `audit.recorded` | mọi module → identity `audit-sink` | `specs/events/payloads/common/audit.recorded.v1.schema.json` (một schema cho mọi producer, owner F003 T004) | Cần thêm (F003) |
| `platform_settings.changed` | identity → mọi module (invalidate cache) | `platform_settings.changed.v1.schema.json` | Cần thêm |
| `tenant.updated`, `school.updated` | identity → mọi module (`ref.tenants.settings_subset`, `ref.schools`) | xem F010 (owner F010) | Cần thêm (F010) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `TenantSettings`, `StudentCleanupSettings`, `ChatRetentionSettings` (trong identity.v1.yaml) | Validate cài đặt | Có sẵn (`allowed_hours` đang `additionalProperties: true` — Cần sửa thành schema có giới hạn) |
| `SchoolSettings`, `FeatureFlag` | Validate cài đặt trường, flag (`FeatureFlag` có `flag_kind`, `parent_key`) | `SchoolSettings` Cần thêm; `FeatureFlag` có trong spec gộp |
| `specs/portal/menu-catalog.yaml` + `specs/schemas/portal-menu-catalog.schema.json` | Nguồn danh mục menu/action đồng bộ vào `platform.portal_menu_catalog` | Có (ADR-0090) — kiểm bằng `node scripts/specs/check-portal-catalog.mjs` |

## 6. Delta cần thực hiện

- `identity.v1.yaml`: `getSchoolSettings`, `updateSchoolSettings`, schema `SchoolSettings` (`curfew_window`, `platform_curfew`, `effective_curfew`, `class_leaderboard_enabled`, `school_leaderboard_enabled`, `student_club_creation`), `CurfewWindow`, `getPlatformCurfewSettings`/`putPlatformCurfewSettings` + `PlatformCurfewSettings` đã có trong spec gộp (2026-11-17); còn thêm `listFeatureFlags`, `putFeatureFlag`, schema `AllowedHours` (thay `allowed_hours` dạng object tự do ở `TenantSettings` và `SchoolSettings`) (dùng chung cho tenant/trường, mọi chuỗi có `maxLength`/`pattern`), `FeatureFlag`.
- `AuditLogEntry`: `actor_id` nullable (hệ thống), `actor_type` enum `student`/`staff`/`service`/`system`, `action` `maxLength 64`, đổi `resource_type`/`resource_id` thành `target_type`/`target_id` (chuỗi ≤ 128) hoặc thống nhất tên với DATABASE_DESIGN; thêm `result`, `school_id`.
- Event payload `audit.recorded`, `platform_settings.changed` (thêm `scope = school`, `scope = feature_flag`) — do đợt cập nhật event sở hữu.
- ADR-0090: `listFeatureFlags`, `putFeatureFlag`, `listPortalMenuCatalog`, schema `FeatureFlag`, `FeatureFlagWrite`, `PortalMenuCatalog` đã có trong spec gộp; F015 chỉ kiểm khớp (`make gen MOD=identity`, task T035).
- Kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=identity`.
- ADR-0078: khóa `platform_curfew` (`PlatformCurfewSettings`, `/admin/settings/curfew`) và `curfew_window` (`SchoolSettings`, chỉ siết thêm, `422 VALIDATION_FAILED`) đã có trong spec gộp; còn lại: `PlayerBootstrap.policies.play_time` thêm khung giới nghiêm hiệu lực (`curfew.start`, `curfew.end`, `warning_minutes` = `curfew_warning_minutes`); payload `platform_settings.changed` có khung giới nghiêm (task T032, T034).
