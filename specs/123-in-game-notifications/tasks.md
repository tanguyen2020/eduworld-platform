# Tasks: F123 — Thông báo trong game: realtime, lưu bền vững, không push

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Durable consumer `chat.notify_friend_request`, `chat.notify_party_invitation`, `chat.notify_assignment`, `chat.notify_progression` (FilterSubjects, DLQ `DLQ_CHAT`) trong consumer `chat-notifier` (profile `worker-chat`) — file: `src/backend/modules/chat/internal/presentation/consumer/notifier.go`
- [ ] T002 [P] Adapter facade `learning` qua `src/backend/pkg/modcall` (TenantScope, deadline, ánh xạ lỗi) — file: `src/backend/modules/chat/internal/infrastructure/modules/learning.go`
- [ ] T003 [P] Khóa dịch `ui.notification.*` (mọi `message_code` của `MessageCatalog`, nhãn inbox, toast, cài đặt popup) — file: `src/game/locale/vi.po`, `src/game/locale/en.po`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta OpenAPI: `markNotificationRead`, `markAllNotificationsRead`, `unread_count`, `Notification.action`, enum `kind` chung REST/WS (bỏ `category` ở AsyncAPI) — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/chat.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=chat`
- [ ] T005 [P] Operation facade `listAssignmentRecipients` (`x-allowed-callers: [chat]`) — file: `specs/openapi/modules/learning.internal.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=learning`
- [ ] T006 [P] AsyncAPI `NotificationItemCreated` (`kind`, `ref_type`, `ref_id`, `action`), `x-routing` mark_read — file: `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`
- [ ] T007 [P] Payload event `assignment.results_released`, `party.invitation_sent`, `level.reached`, `achievement.unlocked` — file: `specs/events/payloads/` — kiểm: `make specs-lint`
- [ ] T008 ent schema `Notification` (`MaxLen`, CHECK jsonb) — file: `src/backend/modules/chat/ent/schema/notification.go` — kiểm: `make migrate-diff MOD=chat name=f123_notifications`, `make migrate-lint MOD=chat`
- [ ] T009 Migration: partition tháng, unique idempotent, index chưa đọc, RLS, `REVOKE DELETE, TRUNCATE` — file: `src/backend/modules/chat/migrations/`
- [ ] T010 Hiện thực operation facade ở module learning (đối tượng giao bài trừ học sinh miễn, phân trang ≤ 500, tenant từ `TenantScope` trong context) + test `make arch-check`: `chat` import được, fixture module `game` import → fail (TC-SVC-005, TC-SVC-006) — file: `src/backend/modules/learning/internal/presentation/facade/assignment_recipients.go`, `src/backend/modules/learning/tests/integration/assignment_recipients_test.go`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Nhận thông báo realtime khi có sự kiện (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [ ] T011 [P] [US1] Unit `MessageCatalog`, `RecipientPolicy` F123-AC-01, F123-AC-05, F123-AC-06: ánh xạ event → kind/message_code/action; whitelist `params` không tên thật/trường/lớp; người nhận khác tenant/không hoạt động bị loại — file: `src/backend/modules/chat/tests/unit/notification_catalog_test.go` — phải FAIL trước khi code
- [ ] T012 [P] [US1] Integration F123-AC-01, F123-AC-03, F123-AC-04: event → một dòng `social.notifications` → publish `ew:rt:user:<id>` sau commit (không droppable); giao lại event → không trùng (TC-SVC-003) — file: `src/backend/modules/chat/tests/integration/notification_create_test.go`
- [ ] T013 [P] [US1] Integration F123-AC-02, F123-AC-07: `assignment.published` → recipients từ learning (container thật), trừ học sinh miễn; learning down → không ack, retry, DLQ sau ngưỡng (TC-SVC-004) — file: `src/backend/modules/chat/tests/integration/notification_assignment_test.go`
- [ ] T014 [P] [US1] Facade test chat ↔ learning `listAssignmentRecipients` (fake facade + ví dụ spec) và contract test payload các event tiêu thụ — file: `src/backend/modules/chat/tests/contract/notification_sources_test.go`

### Implementation cho User Story 1

- [ ] T015 [P] [US1] Domain `notification` (`Notification`, `MessageCatalog`, `RecipientPolicy`) — file: `src/backend/modules/chat/internal/domain/notification/`
- [ ] T016 [US1] Command `CreateNotifications` (lô, `ON CONFLICT DO NOTHING`) + repo ent + publisher sau commit + `UnreadCounter` — file: `src/backend/modules/chat/internal/application/command/create_notifications.go`, `src/backend/modules/chat/internal/infrastructure/persistence/notification/`, `src/backend/modules/chat/internal/infrastructure/cache/unread_counter.go`
- [ ] T017 [US1] Consumer `notify_*` — file: `src/backend/modules/chat/internal/presentation/consumer/notify_friend_request.go`, `notify_party_invitation.go`, `notify_assignment.go`, `notify_progression.go`

## Phase 4: User Story 2 — Hộp thông báo lưu bền vững và đánh dấu đã đọc (Priority: P1)

### Tests cho User Story 2

- [ ] T018 [P] [US2] Integration F123-AC-08, F123-AC-09, F123-AC-10: list chỉ của mình + `unread_count`; mark read một lần (REST 404 khi không phải chủ; WS bỏ qua id lạ, ≤ 100 id); read-all — file: `src/backend/modules/chat/tests/integration/notification_inbox_test.go` — phải FAIL trước khi code
- [ ] T019 [P] [US2] Test tenant isolation + authorization F123-AC-11: thiếu header → 400, header khác → 403 `TENANT_MISMATCH`, không token → 401, id của tenant khác → 404 (TC-TEN-001, TC-TEN-011) — file: `src/backend/modules/chat/tests/integration/notification_authz_test.go`
- [ ] T020 [P] [US2] Test F123-AC-12: role `chat_app` không DELETE được `social.notifications`; mất Valkey → `unread_count` đếm lại từ PG — file: `src/backend/modules/chat/tests/integration/notification_durability_test.go`

### Implementation cho User Story 2

- [ ] T021 [US2] Query `ListNotifications`; command `MarkNotificationsRead`, `MarkAllNotificationsRead`; handler REST + NATS `notification.item.mark_read` — file: `src/backend/modules/chat/internal/application/query/list_notifications.go`, `src/backend/modules/chat/internal/presentation/http/public/notifications.go`, `src/backend/modules/chat/internal/presentation/natscmd/notification_mark_read.go`

## Phase 5: User Story 3 — Không push ngoài game tới học sinh (Priority: P1)

### Tests cho User Story 3

- [ ] T022 [P] [US3] Test F123-AC-13: quét OpenAPI/AsyncAPI không có endpoint/message đăng ký token push; module chat không có client gửi email/SMS cho học sinh — file: `src/backend/modules/chat/tests/unit/no_push_contract_test.go` — phải FAIL trước khi code
- [ ] T023 [P] [US3] Script CI F123-AC-14: fail khi `src/backend/go.mod` hoặc `src/game/addons`/export preset chứa SDK push (FCM, APNs, OneSignal, Web Push) — file: `scripts/ci/check-no-push-sdk.sh`, `.github/workflows/ci.yml`

## Phase 6: User Story 4 — Hiển thị thông báo trong game (Priority: P2)

### Tests cho User Story 4

- [ ] T024 [P] [US4] Unit F123-AC-15, F123-AC-18: `NotificationService` tải `listNotifications?unread=true` khi vào game/resync; `notification.item.created` → badge; mở inbox → `notification.item.mark_read`; bấm → `EventBus.ui_requested` theo `action` (FakeApiClient, FakeRealtime) — file: `src/game/tests/unit/notifications/test_notification_service.gd` — phải FAIL trước khi code
- [ ] T025 [P] [US4] Scene test F123-AC-16, F123-AC-17, F123-AC-19: toast theo `kind` (lời mời, bài giao), tắt popup/đang trong trận hoặc bài kiểm tra → chỉ badge, âm thanh tắt mặc định; nội dung theo `message_code` + `params`, fallback `title`/`body`; Theme token, lint override — file: `src/game/tests/scene/notifications/test_notification_ui.gd`
- [ ] T026 [US4] Integration headless: lời mời kết bạn thật → toast + badge trên client nhận — file: `src/game/tests/integration/test_notifications_e2e.gd`

### Implementation cho User Story 4

- [ ] T027 [P] [US4] `NotificationService` + `NotificationModel` (đăng ký dispatcher `notification.*` của `RealtimeClient`) — file: `src/game/scripts/notifications/notification_service.gd`, `src/game/scripts/notifications/models/notification_model.gd`
- [ ] T028 [US4] `NotificationBadge` (HUD), `NotificationToast`, `NotificationInbox`, tùy chọn popup trong Cài đặt — file: `src/game/scenes/ui/notifications/`, `src/game/scripts/core/settings.gd`

## Phase 7: Polish & Cross-Cutting

- [ ] T029 [P] DATABASE_DESIGN §2.11 thêm `social.notifications`; VALKEY_DESIGN §3.1 `ew:chat:ntf:unread:*`; EVENT_DESIGN §3.2 consumer chat cho `party.invitation_sent`, `level.reached`, `achievement.unlocked`, `assignment.results_released`; CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/07-backend/VALKEY_DESIGN.md`, `docs/07-backend/EVENT_DESIGN.md`, `CHANGELOG.md`
- [ ] T030 [P] Metric `eduworld_notifications_*` + alert consumer pending — file: `src/backend/modules/chat/internal/infrastructure/observability/metrics.go`
- [ ] T031 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational (T010 learning trước US1 phần bài giao) → US1 → US2 → US3 (song song từ sau Foundational) → US4 (cần F092) → Polish.
- Nguồn party/phần thưởng chỉ kiểm end-to-end khi F100/F074 Done; trước đó test bằng event phát thật lên NATS testcontainer (không mock trong đường chạy thật).

## Definition of Done

- [ ] Mọi tiêu chí F123-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make test-contract` xanh cho module `chat` (và phần module `learning`); `make godot-test` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
