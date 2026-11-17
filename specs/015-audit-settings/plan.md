# Implementation Plan: F015 — Audit log tập trung và cài đặt platform/tenant/trường

**Branch**: `015-audit-settings` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Dùng `src/backend/pkg/audit` (interface `audit.Recorder` của F002) và hiện thực outbox `outbox.AuditRecorder` của F003 (ghi `audit.recorded` trong cùng transaction, lọc khóa cấm — F003-AC-22); feature này không định nghĩa lại `src/backend/pkg/audit`; module identity có consumer `audit-sink` trong profile `worker-events` (durable trên `EVT_IDENTITY`, `EVT_LEARNING`, `EVT_GAME`, `EVT_COMPETITION`, `EVT_CHAT` với `FilterSubjects` `ew.evt.<module>.audit.recorded`) ghi `audit.audit_logs` (partition tháng). Job `partition-maintainer` tạo trước partition cho bảng partitioned của `eduworld_identity`; job `audit-retention` ẩn danh hóa. Query audit tenant/platform. Command cài đặt platform/tenant/trường/feature flag (gồm bật/tắt menu/action của Admin Portal theo ADR-0090: `flag_kind`, `parent_key`, catalog đọc-chỉ `platform.portal_menu_catalog` đồng bộ bởi `src/backend/cmd/migrate`, hiện thực `featuregate.Source` cho middleware của F012) với `If-Match` + `reason`, event `platform_settings.changed`, `tenant.updated`, `school.updated`; hook khung giờ truy cập trong `GameLogin` (F013).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | ent, Atlas, nats.go JetStream, `src/backend/pkg/outbox`, `src/backend/pkg/entx` (`UnitOfWork`), JSON Schema validator cho settings |
| Storage | PostgreSQL `eduworld_identity` (`audit.audit_logs` TA P, `platform.platform_settings` G, `platform.feature_flags` G, `platform.portal_menu_catalog` G (đọc-chỉ), `platform.tenants.settings`, `org.schools.settings`); Valkey cache settings/flag `ew:identity:settings:<tenant_id>` |
| Testing | go test + testcontainers (PostgreSQL, NATS); Clock giả cho retention/khung giờ |
| Target Platform | Linux container |
| Project Type | backend module (dùng `src/backend/pkg/audit` của F002/F003) |
| Performance Goals | SC-003 (truy vấn audit 30 ngày ≤ 2 s); sink bắt kịp không vượt `EventConsumerLag` (`alert_thresholds`) |
| Constraints | AGENTS R14, R21, R26, R28, R33; ADR-0028, ADR-0032, ADR-0033, ADR-0042, ADR-0070 |
| Scale/Scope | NFR-SCAL-004 |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta `getSchoolSettings`, `updateSchoolSettings`, `getPlatformCurfewSettings`/`putPlatformCurfewSettings` (đã có trong spec gộp), feature flag API, sửa `AuditLogEntry`, payload `audit.recorded`, `platform_settings.changed` ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Module khác chỉ phát event; không ghi DB identity |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Audit có `tenant_id` + RLS (TA); truy vấn qua TenantScope; System Admin CrossTenantScope + audit; append-only |
| IV Server-authoritative | ✅ | Ghi cài đặt có `If-Match` + `reason` + audit; không có điểm/thưởng |
| V Xác thực, phân quyền | ✅ | `tenant.audit.read`, `platform.audit.read`, `platform.settings.write`, `tenant.settings.*`, `tenant.student_cleanup.configure`, `school.settings.*`, `platform.feature_flag.write` (chỉ System Admin, gateway nội bộ + MFA); bật/tắt menu không thay authorization |
| VI An toàn trẻ em | ✅ | Lọc PII/nội dung chat khỏi audit; khung giờ truy cập do trường đặt |
| VII Một nguồn sự thật, cấu hình | ✅ | Giá trị cài đặt lưu một nơi (identity), module khác nhận qua event/facade `getTenant` |
| VIII UI tách logic, token | ✅ | Không có UI |
| IX Test có bằng chứng | ✅ | Test mọi F015-AC-* gồm quyền DB append-only |

## Project Structure

### Documentation (feature này)

```text
specs/015-audit-settings/
├── spec.md · plan.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
src/backend/modules/identity/
├── cmd/migrate/                                    # sync_portal_catalog.go: specs/portal/menu-catalog.yaml → platform.portal_menu_catalog (ADR-0090)
├── ent/schema/                         # auditlog.go (AppendOnly), platformsetting.go, featureflag.go (flag_kind, parent_key), portalmenucatalogitem.go (đọc-chỉ)
├── migrations/                         # partition audit, REVOKE UPDATE/DELETE, feature_flags (+ FK catalog), REVOKE ghi portal_menu_catalog
├── internal/domain/settings/           # PlatformSettings, TenantSettings, SchoolSettings (AllowedHours), FeatureFlag, EffectiveValue
├── internal/domain/audit/              # AuditEntry (sanitize)
├── internal/application/command/       # PutStudentCleanupSettings, PutChatRetentionSettings, UpdateTenantSettings,
│                                       # UpdateSchoolSettings, PutFeatureFlag, RecordAuditEntry (sink)
├── internal/application/query/         # ListTenantAuditLogs, ListPlatformAuditLogs, GetTenantSettings, GetSchoolSettings, GetPlatformCurfewSettings,
│                                       # GetStudentCleanupSettings, GetChatRetentionSettings, ListFeatureFlags, EffectiveSettings
├── internal/infrastructure/featuregate/source.go   # hiện thực featuregate.Source (F012), cache ew:fg:{tenant_id|platform}
├── internal/presentation/http/public/{audit.go, settings.go, school_settings.go, feature_flags.go, portal_menu_catalog.go}
├── internal/presentation/consumer/audit_sink.go
├── internal/presentation/job/{partition_maintainer.go, audit_retention.go}
└── tests/{unit,integration}/
```

**Structure Decision**: Theo BACKEND_DESIGN §3; `src/backend/pkg/audit` (interface F002) + `outbox.AuditRecorder` (F003) là tiện ích dùng chung; F015 chỉ sở hữu consumer `audit-sink`, bảng `audit.audit_logs` và API truy vấn.

## Phase 0: Research

Không có — DATABASE_DESIGN §1.3, §3.11 và EVENT_DESIGN §3.2 đã chốt cơ chế audit qua outbox.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `AuditEntry.Sanitize` (khóa cấm, giới hạn kích thước), `EffectiveValue(platform, tenant, allowOverride)`, `AllowedHours.Contains(t, tz)`, `FeatureFlag.ValueFor(tenant)`, `PortalToggles.Effective(catalog, flags, tenant)` (action AND menu cha, không dòng = bật), `CatalogItem.CanDisable` | Thuần |
| application | Command/Query ở cây trên | Permission: [contracts](contracts/README.md) §1 |
| infrastructure | ent repo, read model audit (SQL tường minh có `tenant_id` + khoảng thời gian), cache settings | |
| presentation | `listTenantAuditLogs`, `listPlatformAuditLogs`, `getStudentCleanupSettings`, `putStudentCleanupSettings`, `getChatRetentionSettings`, `putChatRetentionSettings`, `getTenantSettings`, `updateTenantSettings`, `getSchoolSettings`, `updateSchoolSettings`, `getPlatformCurfewSettings`, `putPlatformCurfewSettings`, `listFeatureFlags`*, `putFeatureFlag`*, `listPortalMenuCatalog`* · consumer `audit-sink` · job `partition-maintainer`, `audit-retention` | * = Cần thêm |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F012 (permission, Principal), F010 (tenant/school), F013 (hook khung giờ trong `GameLogin`).
- Event phát: `platform_settings.changed`, `tenant.updated`, `school.updated`, `audit.recorded`.
- Event tiêu thụ: `ew.evt.*.audit.recorded` (mọi module).

### Bảo mật, tenant, idempotency, audit

- Route `/admin/*`: gateway nội bộ + MFA; `/tenant/*`, `/schools/*`: `portal_staff` + permission + scope.
- `If-Match` + `reason` cho mọi ghi cài đặt; `putFeatureFlag` idempotent theo `If-Match` + trạng thái đích (PUT), `X-Tenant-Id` = tenant đích khi `scope = tenant`.
- Bật/tắt menu/action: audit `feature_flag.updated` (trước/sau, `reason`, tenant), event `platform_settings.changed` scope `feature_flag`; mục `can_disable = false` → `422 FEATURE_NOT_DISABLEABLE`; `identity_app` không ghi được catalog.
- Xem liên tenant: `CrossTenantScope` + audit `cross_tenant.accessed`.

### Quan sát

- Metric `eduworld_audit_records_total{service,result}`, `eduworld_audit_sanitized_fields_total`, `eduworld_audit_sink_lag_seconds`, `eduworld_partition_missing_total{table}`; alert khi partition tháng kế tiếp chưa có trước 7 ngày.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
