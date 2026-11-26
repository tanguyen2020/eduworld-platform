# Contracts: F081 — Workspace C — sự kiện trường từ template

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/eduworld-api.v1.yaml` → `specs/openapi/modules/game.v1.yaml` (route ngoại lệ `/lms/event-templates`, `/lms/school-events*` → module game, [API_DESIGN](../../../docs/07-backend/API_DESIGN.md)); gateway public, `X-Tenant-Id`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listEventTemplates` | `GET /api/v1/lms/event-templates` | game.v1.yaml | `lms.school_event.write` | Có sẵn · **Đã sửa (F081)** `EventTemplate.quest_option_items`/`challenge_option_items` |
| `listSchoolEvents` | `GET /api/v1/lms/school-events` (`tenant_ids`, `status`) | game.v1.yaml | `lms.school_event.write` | Có sẵn · **Đã sửa (F081)** `status` = `SchoolEventStatus`, `X-Tenant-Id` tùy chọn (System Admin liên tenant) |
| `createSchoolEvent` | `POST /api/v1/lms/school-events` (`Idempotency-Key`) | game.v1.yaml | `lms.school_event.write` | Có sẵn (`409`/`503`/`428`: delta của F077 T005) |
| `getSchoolEvent` · `updateSchoolEvent` | `GET/PATCH /api/v1/lms/school-events/{event_id}` (`If-Match`) | game.v1.yaml | `lms.school_event.write` | Có sẵn |
| `cancelSchoolEvent` | `POST /api/v1/lms/school-events/{event_id}:cancel` (`EventRunOperation`) | game.v1.yaml | `lms.school_event.write` | Có sẵn |
| `getSchoolEventProgress` | `GET /api/v1/lms/school-events/{event_id}/progress` (`school_id` tùy chọn) | game.v1.yaml | `lms.school_event.write` | **Đã thêm (F081)** · `x-portal-action: action.school_lms.school_events.view` |
| `getMyPermissions` · `listTenants` | `GET /api/v1/me/permissions` · `GET /api/v1/admin/tenants` | identity.v1.yaml | đã đăng nhập / `platform.cross_tenant.read` | Có sẵn |

Schema dùng: `EventTemplate`, `SchoolEvent`, `SchoolEventCreate`, `SchoolEventUpdate`, `EventRunOperation`, `EventRunStatus`, `TenantLocalizedName`, `TenantLocalizedText`, `Problem` (`EVENT_TEMPLATE_PARAM_NOT_ALLOWED`, `VALIDATION_FAILED`, `UNAVAILABLE`, `CONFLICT`, `VERSION_MISMATCH`, `NOT_FOUND`, `PERMISSION_DENIED`, `TENANT_MISMATCH`).

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng (portal làm mới thủ công) | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Không phát/tiêu thụ | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/event-template.schema.json` | Hiểu `allowed_params` (mã tham số) | Cần thêm — owner F077 (task F077 T004) |

## 6. Delta cần thực hiện

Task Phase 2: T004, T006 (delta `TenantGameSummary` cũ ở T005 đã chuyển sang F080 T055). Mọi thay đổi là cộng thêm (không breaking); F081 tạo contract nếu owner chưa làm. **Trạng thái 2026-11-25: đã áp toàn bộ** (thêm `school_id` query và `SchoolEventProgress.school_id` theo Clarifications Session 2026-11-25; `SchoolEventGoalProgress` tách schema; `422 EVENT_TEMPLATE_PARAM_NOT_ALLOWED` có `errors[].field`).

- **Cần thêm** `getSchoolEventProgress` (`x-service: game`, tags `[lms]`, `TenantIdHeader`, `EventIdPath`) → `200 SchoolEventProgress` {`event_id` Uuid, `event_run_id` Uuid, `status` $ref `EventRunStatus`, `participants_count` integer ≥ 0, `community_goal` (nullable) {`goal_id` Uuid, `metric` string maxLength 32, `target` integer, `total` integer, `milestones[]` maxItems 10 {`pct` integer 1–100, `reached` boolean}}, `by_grade[]` maxItems 9 {`grade_level` integer 1–9, `contribution_total` integer, `contributors_count` integer} (ADR-0095: thay `by_class`; chỉ khối 4–9, khối < 5 học sinh đóng góp không trả), `data_as_of` Timestamp}; `401`, `403`, `404`.
- **Cần sửa** `EventTemplate`: thêm `quest_option_items[]` và `challenge_option_items[]` (maxItems 100) {`id` Uuid, `title` $ref `LocalizedName`, `grade_levels[]` integer 1–12 maxItems 12} (owner F077; module game lấy tiêu đề từ version đã publish).
- **Cần sửa** `SchoolEvent.status`: enum thêm `suspended`, `archived` (khớp `EventRunStatus`); `listSchoolEvents` tham số `status` dùng enum đó thay `string maxLength 32`.
- **Tài liệu**: API_DESIGN thêm `getSchoolEventProgress`.
- Kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`, `pnpm gen:api` (web).
