# Implementation Plan: F123 — Thông báo trong game: realtime, lưu bền vững, không push

**Branch**: `123-in-game-notifications` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Backend: bounded context `notification` trong module chat. Consumer `chat-notifier` (profile `worker-chat`) có các durable consumer (`chat.notify_friend_request`, `chat.notify_party_invitation`, `chat.notify_assignment`, `chat.notify_progression`) dịch event thành command `CreateNotifications` (lô, idempotent theo `(recipient_id, source_event_id, kind)`), ghi `social.notifications`, sau commit publish `notification.item.created` lên `ew:rt:user:<id>` (không droppable) và tăng cache `ew:chat:ntf:unread:<user_id>`. Người nhận bài giao lấy bằng typed client `learning.internal` `listAssignmentRecipients` (mới). API: `listNotifications` (thêm `unread_count`), `markNotificationRead`, `markAllNotificationsRead` (mới) và NATS `ew.cmd.chat.notification.item.mark_read`. Không có bất kỳ thành phần push. Godot: `NotificationService` (REST + WS), `NotificationBadge` trên HUD, `NotificationToast`, `NotificationInbox`.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 · GDScript (Godot 4.x, static typing) |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen (server public), facade `learning` (`modules/learning/api`), `pkg/modcall`, `pkg/outbox` (consumer), `pkg/natsx`, `pkg/realtime` · GdUnit4, `ApiClient`/`RealtimeClient` (F061, F092) |
| Storage | PostgreSQL `eduworld_chat` (`social.notifications` mới, `ref.students`); Valkey cache số chưa đọc |
| Testing | go test + testcontainers (PostgreSQL, NATS, Valkey); facade test với learning (fake facade phía chat, ví dụ spec phía learning); GdUnit4 unit/scene; integration headless |
| Target Platform | Linux container (module `chat` nạp trong `core-api` + profile `worker-chat`); Windows/Android/iOS/Web |
| Project Type | backend module + game client |
| Performance Goals | Thông báo tới client online p95 ≤ 2 s từ lúc event nguồn được phát (SC-001) |
| Constraints | AGENTS R1, R8, R9, R13, R14, R16, R19, R21, R26, R29, R32 · ADR-0035, ADR-0040, ADR-0074, ADR-0042, ADR-0070 |
| Scale/Scope | Bài giao cả khối (≤ 500 học sinh) tạo lô ≤ 500 thông báo/event |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta OpenAPI (2 endpoint mới, `unread_count`, enum), AsyncAPI, operation facade `listAssignmentRecipients`, payload event ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Người nhận qua facade `learning`; nguồn khác qua event; command tạo thông báo, query hộp thông báo |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `social.notifications` T + RLS; chỉ người nhận đọc được; không xóa vật lý |
| IV Server-authoritative | ✅ | Không nhận dữ liệu thông báo từ client; consumer idempotent |
| V Xác thực, phân quyền | ✅ | Chỉ chủ thông báo đọc/đánh dấu; operation facade `x-allowed-callers: [chat]` (arch-check) |
| VI An toàn trẻ em | ✅ | Không push ngoài game; tham số chỉ nickname; âm thanh tắt mặc định; không popup trong trận/bài kiểm tra |
| VII Một nguồn sự thật, cấu hình | ✅ | Chuỗi theo `message_code` trong file dịch; tham số CONFIG_DEFAULTS |
| VIII UI tách logic, token | ✅ | UI chỉ đọc `NotificationService`; Theme từ token |
| IX Test có bằng chứng | ✅ | Test mọi F123-AC-*, isolation, CI quét SDK push |

## Project Structure

### Documentation (feature này)

```text
specs/123-in-game-notifications/
├── spec.md · plan.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
modules/chat/
├── ent/schema/notification.go
├── internal/domain/notification/        # Notification, Category/Kind, RecipientPolicy, MessageCatalog (event → kind, message_code, action)
├── internal/application/command/        # CreateNotifications, MarkNotificationsRead, MarkAllNotificationsRead
├── internal/application/query/          # ListNotifications (+ unread_count)
├── internal/application/port/           # NotificationRepository, AssignmentRecipients, RealtimePublisher, UnreadCounter
├── internal/infrastructure/modules/learning.go        # adapter facade listAssignmentRecipients (pkg/modcall)
├── internal/infrastructure/persistence/notification/
├── internal/infrastructure/cache/unread_counter.go
├── internal/presentation/http/public/notifications.go
├── internal/presentation/natscmd/notification_mark_read.go
├── internal/presentation/consumer/notify_*.go
└── tests/{unit,integration,contract}/
modules/learning/internal/presentation/facade/assignment_recipients.go   # hiện thực operation facade mới
godot/
├── scripts/notifications/notification_service.gd
├── scripts/notifications/models/notification_model.gd
├── scenes/ui/notifications/{NotificationInbox,NotificationToast,NotificationBadge}.tscn (+ .gd)
├── locale/{vi,en}.po                    # ui.notification.*
└── tests/{unit,scene,integration}/notifications/
scripts/ci/check-no-push-sdk.sh
```

**Structure Decision**: Notification là bounded context của module chat ([CHAT_SYSTEM §10](../../docs/03-social-multiplayer/CHAT_SYSTEM.md), API_DESIGN §9.11); thư mục Godot `scripts/notifications` theo GODOT_ARCHITECTURE §3. Endpoint internal ở module learning vì learning sở hữu đối tượng giao bài.

## Phase 0: Research

Không có — CHAT_SYSTEM §10, SOCIAL_SYSTEM §8, EVENT_DESIGN §3.2/§4 đủ; quyết định nhỏ ở spec Clarifications.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `Notification` (MarkRead một lần), `MessageCatalog` (bảng event → `kind`/`message_code`/`action`), `RecipientPolicy` (học sinh hoạt động cùng tenant) | Thuần |
| application | `CreateNotifications` (lô, `ON CONFLICT DO NOTHING` theo khóa idempotent), `MarkNotificationsRead`, `MarkAllNotificationsRead`, `ListNotifications` | Chủ thể = Principal người gọi |
| infrastructure | ent repo, adapter facade `learning` (deadline, ánh xạ lỗi; không retry/circuit breaker), cache Valkey, publisher `pkg/realtime` | |
| presentation | `listNotifications`, `markNotificationRead`, `markAllNotificationsRead`, NATS `notification.item.mark_read`, consumer `notify_*` | |

Bảng `MessageCatalog`:

| Event | `kind` | `message_code` | `action.kind` |
|---|---|---|---|
| `friend_request.sent` | `friend_request` | `friend.request_received` | `open_friend_requests` |
| `party.invitation_sent` | `party_invitation` | `party.invitation_received` | `open_party_invitation` |
| `assignment.published` (status `open`) | `assignment_new` | `assignment.new` | `open_assignment` |
| `assignment.results_released` | `assignment_results_released` | `assignment.results_released` | `open_assignment_result` |
| `level.reached` | `level_reached` | `progression.level_reached` | `open_profile` |
| `achievement.unlocked` | `achievement_unlocked` | `achievement.unlocked` | `open_achievements` |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F090/F092 (realtime), F115 (worker chat), F095 (`friend_request.sent`), F100 (`party.invitation_sent`), F041 (`assignment.*`, endpoint recipients), F074/F075 (`level.reached`, `achievement.unlocked`).
- Facade: `listAssignmentRecipients` (`learning`, Cần thêm, `x-allowed-callers: [chat]`).

### Bảo mật, tenant, idempotency, audit

- REST yêu cầu token hợp lệ + `X-Tenant-Id`; query luôn `recipient_id = principal.user_id`.
- Idempotency: consumer theo `processed_events` + unique `(recipient_id, source_event_id, kind)`; mark read idempotent theo trạng thái.
- Không audit từng thông báo (khối lượng lớn, không phải thao tác quan trọng); metric thay thế.

### Quan sát

- Metric: `eduworld_notifications_created_total{kind}`, `eduworld_notification_delivery_latency_seconds`, `eduworld_notifications_read_total`, `eduworld_consumer_pending{stream,consumer}`.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
