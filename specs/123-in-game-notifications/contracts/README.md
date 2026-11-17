# Contracts: F123 — Thông báo trong game: realtime, lưu bền vững, không push

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn `specs/openapi/eduworld-api.v1.yaml` → `specs/openapi/modules/chat.v1.yaml`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listNotifications` | `GET /api/v1/notifications?unread=&limit=&cursor=` → `{data: Notification[], page, unread_count}` | chat.v1.yaml | Đã xác thực, chỉ thông báo của mình | Cần sửa — thêm `unread_count`, `action` vào `Notification`; mở rộng enum `kind` (enum chung với WS) |
| `markNotificationRead` | `POST /api/v1/notifications/{notification_id}:read` → `200 Notification` | chat.v1.yaml | chủ thông báo | Cần thêm (API_DESIGN §9.11) |
| `markAllNotificationsRead` | `POST /api/v1/notifications:read-all` → `200 {unread_count}` | chat.v1.yaml | chủ thông báo | Cần thêm (API_DESIGN §9.11) |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `listAssignmentRecipients` | chat → learning (facade; `assignment_id`, `cursor`) → `{student_ids: uuid[≤ 500], page}` (đã trừ học sinh miễn; tenant từ `TenantScope` trong context) | `specs/openapi/modules/learning.internal.v1.yaml` | `[chat]` | Cần thêm (+ contract test) |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `notification.item.created` | server → client | Cần sửa — bỏ `category`, thêm `kind` (cùng enum REST), `ref_type`, `ref_id`; `action` có schema `{kind ≤ 64, id uuid}` |
| `notification.item.mark_read` | client → server (`ew.cmd.chat.notification.item.mark_read`) | Có sẵn (thêm `x-routing`) |
| `system.service.degraded` (feature `notification`) | server → client | Có sẵn |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `friend_request.sent` | chat (F095) → chat notifier | `specs/events/payloads/chat/friend_request.sent.v1.schema.json` | Cần thêm (F095) |
| `party.invitation_sent` | competition (F100) → chat | `specs/events/payloads/competition/party.invitation_sent.v1.schema.json` | Cần thêm |
| `assignment.published` | learning (F041) → chat | `specs/events/payloads/learning/assignment.created.v1.schema.json` | Có sẵn |
| `assignment.results_released` | learning (F041) → chat | `specs/events/payloads/learning/assignment.results_released.v1.schema.json` | Cần thêm |
| `level.reached`, `achievement.unlocked` | game (F074/F075) → chat | `specs/events/payloads/game/level.reached.v1.schema.json`, `achievement.unlocked.v1.schema.json` | Cần thêm |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không có | — |

## 6. Delta cần thực hiện

- OpenAPI public: thêm `markNotificationRead`, `markAllNotificationsRead`; `listNotifications` thêm `unread_count`; `Notification` thêm `action`; `kind` thêm `assignment_results_released`, `level_reached`, `achievement_unlocked` (giữ giá trị cũ). Enum `kind` khai báo một lần (`specs/schemas/common/enums.schema.json` hoặc component dùng chung) cho cả REST và WS (RB-513).
- AsyncAPI `NotificationItemCreated`: bỏ `category`, thêm `kind` (cùng enum REST, `required`), `ref_type`, `ref_id`; schema chặt cho `action`; `x-routing` cho `notification.item.mark_read`.
- `learning.internal.v1.yaml`: thêm `listAssignmentRecipients` (`x-allowed-callers: [chat]`).
- Payload event: `assignment.results_released`, `party.invitation_sent`, `level.reached`, `achievement.unlocked`.
- Kiểm: `make specs-lint`, `make gen MOD=chat`, `make gen MOD=learning`, `make test-contract`.
