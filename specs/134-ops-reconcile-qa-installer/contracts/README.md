# Contracts: F134 — Công cụ vận hành bổ sung: lệnh `reconcile-<module>`, loại tenant QA khỏi analytics, trang tải bộ cài cho trường

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listSchoolClientInstallers` | `GET /api/v1/tenant/client-installers?school_id=` (`X-Tenant-Id`) | `specs/openapi/modules/game.v1.yaml` + `eduworld-api.v1.yaml` | `school.installer.read` [Confirmed] | Cần thêm |
| `updateTenant` | `PATCH /api/v1/admin/tenants/{tenant_id}` | identity.v1 | `platform.tenant.manage` | Cần thêm (F010) — Cần sửa: body thêm `is_qa` (boolean) |
| `listTenants` | `GET /api/v1/admin/tenants` | eduworld-api | `platform.cross_tenant.read` | Có sẵn — Cần sửa: `TenantSummary` thêm `is_qa` |
| `getGameMetrics` · `getGameRetention` · `getGameFunnel` · `getGameContentStats` · `getGamePerformance` · `createGameAnalyticsExport` | `/api/v1/admin/analytics/game/*` | eduworld-api | `game.analytics.read` | Có sẵn — Cần sửa: response thêm `excluded_tenant_ids[]` |
| `getTenantGameSummary` | `GET /api/v1/tenant/analytics/game/summary` | eduworld-api | `analytics.tenant.read` / `analytics.school.read` | Có sẵn |
| `getPlatformLearningSummary` · `createPlatformReport` | `GET /api/v1/admin/analytics/learning/summary` · `POST /api/v1/admin/reports` | eduworld-api | `platform.cross_tenant.read` · `report.export` | Có sẵn — Cần sửa: thêm `excluded_tenant_ids[]` |
| `postTelemetry` | `POST /api/v1/game/telemetry` | eduworld-api | học sinh | Có sẵn (hành vi bỏ qua tenant QA, không đổi contract) |

## 2. Internal API

Mỗi module có DB (`identity`, `learning`, `game`, `competition`, `chat`) hiện thực cùng hai operation facade (gọi trong tiến trình qua `src/backend/pkg/modcall`, ADR-0074) khai báo trong `specs/openapi/modules/<module>.internal.v1.yaml`; định nghĩa schema chung một lần trong `specs/openapi/modules/common/internal-common.v1.yaml` rồi `$ref`.

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `listOutboxEventIds` | module được đối soát (`worker reconcile --module=<module>`) → mọi module nguồn mà nó tiêu thụ | Facade: tham số `from`, `to`, `event_types`, `cursor`, `limit` → `{items[{event_id, event_type, tenant_id?, created_at, published_at?}], next_cursor}` | `[identity, learning, game, competition, chat]` | Cần thêm |
| `listProcessedEventIds` | module được đối soát → mọi module tiêu thụ event của nó | Facade: tham số `source_module`, `from`, `to`, `cursor`, `limit` → `{consumers[{consumer, event_types[]}], items[{consumer, event_id, processed_at, status (processed/dead_lettered)}], next_cursor}` | như trên | Cần thêm |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `tenant.updated` | identity → game, learning (projection `ref.tenants`) | `specs/events/payloads/identity/tenant.updated.v1.schema.json` | Cần thêm (owner F010; `is_qa` bắt buộc khi có trong `changed_fields`) |
| `audit.recorded` | identity, mọi module → identity | envelope chuẩn | Có sẵn |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/ops/reconcile-report.schema.json` | Báo cáo đối soát ([data-model](../data-model.md)) | Cần thêm |
| `specs/schemas/school-kit-index.schema.json` | Chỉ mục bộ cài trường có chữ ký | Cần thêm (cạnh `client-release-manifest.schema.json` của F133) |

## 6. Delta cần thực hiện

- **Cần thêm** `listOutboxEventIds`, `listProcessedEventIds` vào 5 file `*.internal.v1.yaml` (`limit` maximum 5000, `cursor` maxLength 512, `event_types` mỗi phần tử maxLength 64, `source_module` enum, `from`/`to` `format: date-time`); facade test cho mọi cặp module (bên gọi với fake facade, bên nhận với ví dụ trong spec — `make test-contract`).
- **Cần thêm** `listSchoolClientInstallers` + schema `SchoolClientInstaller` (`kit_id` maxLength 64, `client_version` maxLength 32, `url` format uri maxLength 2048, `sha256` pattern `^[a-f0-9]{64}$`, `min_os` maxLength 32, `grade_bands` enum `GradeBand`, `platform` enum `windows/android`), lỗi `503 UNAVAILABLE`.
- **Cần sửa** `TenantSummary` (+`is_qa`), `TenantUpdate` của F010 (+`is_qa`), response analytics platform (+`excluded_tenant_ids[]`, `format: uuid`).
- **Cần thêm** JSON Schema `reconcile-report.schema.json`, `school-kit-index.schema.json`; payload `tenant.updated`.
- Catalog permission: `school.installer.read` (tenant được phép).
