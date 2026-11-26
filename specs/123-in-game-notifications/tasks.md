# Tasks: F123 — Thông báo trong game: realtime, lưu bền vững, không push

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Durable consumer `chat.notify_friend_request`, `chat.notify_party_invitation`, `chat.notify_assignment`, `chat.notify_progression` (FilterSubjects, DLQ `DLQ_CHAT`) trong consumer `chat-notifier` (profile `worker-chat`) — file: `src/backend/modules/chat/internal/presentation/consumer/notifier.go` — một handler `Notifier` trong `notifier.go` cho cả năm consumer (thêm `chat.notify_moderation` cho `report.resolved` của F121), chạy profile `worker-events`
- [x] T002 [P] Adapter facade `learning` qua `src/backend/pkg/modcall` (TenantScope, deadline, ánh xạ lỗi) — file: `src/backend/modules/chat/internal/infrastructure/modules/learning.go`
- [x] T003 [P] Khóa dịch `ui.notification.*` (mọi `message_code` của `MessageCatalog`, nhãn inbox, toast, cài đặt popup) — file: `src/game/locale/vi.po`, `src/game/locale/en.po` — 7 `message_code` (`friend.request_received`, `party.invitation_received`, `assignment.new`, `assignment.results_released`, `progression.level_reached`, `achievement.unlocked`, `moderation.report_handled`) + `generic`, `bell_unread`, `inbox.*`, `ui.settings.notification*`, `ui.settings.tab.notifications`; `messages.pot` + `xx_PS.po` sinh lại; `extract_pot --check`, `pseudo_locale --check` OK; test `test_every_catalog_message_code_has_vi_and_en_texts`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta OpenAPI: `markNotificationRead`, `markAllNotificationsRead`, `unread_count`, `Notification.action`, enum `kind` chung REST/WS (bỏ `category` ở AsyncAPI) — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/chat.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=chat`
- [x] T005 [P] Operation facade `listAssignmentRecipients` (`x-allowed-callers: [chat]`) — file: `specs/openapi/modules/learning.internal.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=learning`
- [x] T006 [P] AsyncAPI `NotificationItemCreated` (`kind`, `ref_type`, `ref_id`, `action`), `x-routing` mark_read — file: `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`
- [x] T007 [P] Payload event `assignment.results_released`, `party.invitation_sent`, `level.reached`, `achievement.unlocked` — file: `specs/events/payloads/` — kiểm: `make specs-lint` — payload đã có từ feature nguồn (F095, F100, F041, F074/F075, F121); khớp schema kiểm bằng `TestNotificationSourcesContract`
- [x] T008 ent schema `Notification` (`MaxLen`, CHECK jsonb) — file: `src/backend/modules/chat/ent/schema/notification.go` — kiểm: `make migrate-diff MOD=chat name=f123_notifications`, `make migrate-lint MOD=chat` — bảng partition tháng viết tay như `chat.messages` (khóa chính ghép gồm cột partition ngoài ent, tiền lệ F115); không có ent schema, `TestMigrationMatchesEnt` không đổi
- [x] T009 Migration: partition tháng, unique idempotent, index chưa đọc, RLS, `REVOKE DELETE, TRUNCATE` — file: `src/backend/modules/chat/migrations/` — `20270101000001_f123_notifications.sql`
- [x] T010 Hiện thực operation facade ở module learning (đối tượng giao bài trừ học sinh miễn, phân trang ≤ 500, tenant từ `TenantScope` trong context) + test `make arch-check`: `chat` import được, fixture module `game` import → fail (TC-SVC-005, TC-SVC-006) — file: `src/backend/modules/learning/internal/presentation/facade/assignment_recipients.go`, `src/backend/modules/learning/tests/integration/assignment_recipients_test.go` — arch: chat được gọi; caller `game` → `CALLER_NOT_ALLOWED` (`TestListAssignmentRecipientsFacade`, `TestAssignmentRecipientsFacade`)

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Nhận thông báo realtime khi có sự kiện (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [x] T011 [P] [US1] Unit `MessageCatalog`, `RecipientPolicy` F123-AC-01, F123-AC-05, F123-AC-06: ánh xạ event → kind/message_code/action; whitelist `params` không tên thật/trường/khối; người nhận khác tenant/không hoạt động bị loại — file: `src/backend/modules/chat/tests/unit/notification_catalog_test.go` — phải FAIL trước khi code
- [x] T012 [P] [US1] Integration F123-AC-01, F123-AC-03, F123-AC-04: event → một dòng `social.notifications` → publish `ew:rt:user:<id>` sau commit (không droppable); giao lại event → không trùng (TC-SVC-003) — file: `src/backend/modules/chat/tests/integration/notification_create_test.go`
- [x] T013 [P] [US1] Integration F123-AC-02, F123-AC-07: `assignment.published` → recipients từ learning (container thật), trừ học sinh miễn; learning down → không ack, retry, DLQ sau ngưỡng (TC-SVC-004) — file: `src/backend/modules/chat/tests/integration/notification_assignment_test.go` — facade learning là TEST DOUBLE `learningapi`-shape qua `pkg/modcall` (NATS/PG/Valkey thật); phía learning có integration DB thật
- [x] T014 [P] [US1] Facade test chat ↔ learning `listAssignmentRecipients` (fake facade + ví dụ spec) và contract test payload các event tiêu thụ — file: `src/backend/modules/chat/tests/contract/notification_sources_test.go` — `src/backend/modules/chat/internal/infrastructure/modules/learning_facade_test.go` (bên gọi, fake + ví dụ spec), `src/backend/modules/learning/internal/presentation/facade/assignment_recipients_facade_test.go` (bên nhận), `src/backend/modules/chat/tests/contract/notification_sources_test.go` (payload)

### Implementation cho User Story 1

- [x] T015 [P] [US1] Domain `notification` (`Notification`, `MessageCatalog`, `RecipientPolicy`) — file: `src/backend/modules/chat/internal/domain/notification/`
- [x] T016 [US1] Command `CreateNotifications` (lô, `ON CONFLICT DO NOTHING`) + repo ent + publisher sau commit + `UnreadCounter` — file: `src/backend/modules/chat/internal/application/command/create_notifications.go`, `src/backend/modules/chat/internal/infrastructure/persistence/notification/`, `src/backend/modules/chat/internal/infrastructure/cache/unread_counter.go` — gộp trong `command/notifications.go`, `persistence/notifications.go`, `cache/notifications.go`
- [x] T017 [US1] Consumer `notify_*` — file: `src/backend/modules/chat/internal/presentation/consumer/notify_friend_request.go`, `notify_party_invitation.go`, `notify_assignment.go`, `notify_progression.go` — gộp trong `consumer/notifier.go`

## Phase 4: User Story 2 — Hộp thông báo lưu bền vững và đánh dấu đã đọc (Priority: P1)

### Tests cho User Story 2

- [x] T018 [P] [US2] Integration F123-AC-08, F123-AC-09, F123-AC-10: list chỉ của mình + `unread_count`; mark read một lần (REST 404 khi không phải chủ; WS bỏ qua id lạ, ≤ 100 id); read-all — file: `src/backend/modules/chat/tests/integration/notification_inbox_test.go` — phải FAIL trước khi code
- [x] T019 [P] [US2] Test tenant isolation + authorization F123-AC-11: thiếu header → 400, header khác → 403 `TENANT_MISMATCH`, không token → 401, id của tenant khác → 404 (TC-TEN-001, TC-TEN-011) — file: `src/backend/modules/chat/tests/integration/notification_authz_test.go` — `TestNotificationAuthz` trong `notification_inbox_test.go`
- [x] T020 [P] [US2] Test F123-AC-12: role `chat_app` không DELETE được `social.notifications`; mất Valkey → `unread_count` đếm lại từ PG — file: `src/backend/modules/chat/tests/integration/notification_durability_test.go`

### Implementation cho User Story 2

- [x] T021 [US2] Query `ListNotifications`; command `MarkNotificationsRead`, `MarkAllNotificationsRead`; handler REST + NATS `notification.item.mark_read` — file: `src/backend/modules/chat/internal/application/query/list_notifications.go`, `src/backend/modules/chat/internal/presentation/http/public/notifications.go`, `src/backend/modules/chat/internal/presentation/natscmd/notification_mark_read.go` — `query/notifications.go`, `http/notifications.go`, `natscmd/notifications.go`

## Phase 5: User Story 3 — Không push ngoài game tới học sinh (Priority: P1)

### Tests cho User Story 3

- [x] T022 [P] [US3] Test F123-AC-13: quét OpenAPI/AsyncAPI không có endpoint/message đăng ký token push; module chat không có client gửi email/SMS cho học sinh — file: `src/backend/modules/chat/tests/unit/no_push_contract_test.go` — phải FAIL trước khi code
- [x] T023 [P] [US3] Script CI F123-AC-14: fail khi `src/backend/go.mod` hoặc `src/game/addons`/export preset chứa SDK push (FCM, APNs, OneSignal, Web Push) — file: `scripts/ci/check-no-push-sdk.sh`, `.github/workflows/ci.yml` — job `no-push-sdk` trong `.github/workflows/security.yml` (có self-test)

## Phase 6: User Story 4 — Hiển thị thông báo trong game (Priority: P2)

### Tests cho User Story 4

- [x] T024 [P] [US4] Unit F123-AC-15, F123-AC-18: `NotificationService` tải `listNotifications?unread=true` khi vào game/resync; `notification.item.created` → badge; mở inbox → `notification.item.mark_read`; bấm → `EventBus.ui_requested` theo `action` (FakeApiClient, FakeRealtime) — file: `src/game/tests/unit/notifications/test_notification_service.gd` — phải FAIL trước khi code — 24/24 pass (RealtimeHarness: FakeHttp + FakeWebSocketPeer, fixture sinh `openapi/listNotifications.200.json`, `asyncapi/notification.item.created.*`)
- [x] T025 [P] [US4] Scene test F123-AC-16, F123-AC-17, F123-AC-19: toast theo `kind` (lời mời, bài giao), tắt popup/đang trong trận hoặc bài kiểm tra → chỉ badge, âm thanh tắt mặc định; nội dung theo `message_code` + `params`, fallback `title`/`body`; Theme token, lint override — file: `src/game/tests/scene/notifications/test_notification_ui.gd` — 12/12 pass (thay `test_notification_bell.gd` của F064); ma trận khối `test_grade_band_matrix.gd` thêm 3 test + tab Cài đặt (27/27 pass); `lint_theme_overrides` OK
- [x] T026 [US4] Integration headless: lời mời kết bạn thật → toast + badge trên client nhận — file: `src/game/tests/integration/test_notifications_e2e.gd` — viết xong (realtime thật + `sendFriendRequest` + từ chối lại để chạy lặp), bỏ qua khi thiếu `EDUWORLD_IT_API_BASE_URL` như các test live khác; chưa chạy với backend (xem T031)

### Implementation cho User Story 4

- [x] T027 [P] [US4] `NotificationService` + `NotificationModel` (đăng ký dispatcher `notification.*` của `RealtimeClient`) — file: `src/game/scripts/notifications/notification_service.gd`, `src/game/scripts/notifications/models/notification_model.gd` — cùng `WsMessageSchemas` (`notification.item.created`), `ServiceLocator` (resync, `quiet_check` bài kiểm tra/thử thách), `ExamService.in_progress()`, `InventoryService` đọc `kind`; fixture sinh bằng `node src/game/tools/gen_fixtures.mjs` (không sửa tay, `--check` OK)
- [x] T028 [US4] `NotificationBadge` (HUD), `NotificationToast`, `NotificationInbox`, tùy chọn popup trong Cài đặt — file: `src/game/scenes/ui/notifications/`, `src/game/scripts/core/settings.gd` — badge = `NotificationBell` + viên thuốc `CountBadge` (token mới `tokens@0.18.0`), `ToastStack` dùng `NotificationToast`, `NotificationInbox` (màn `notification_inbox` của `UiScreenRegistry`), tab Cài đặt "Thông báo" (`Settings.notification_popups`); ảnh trước/sau + bảng khối ở [quickstart §5](quickstart.md)

## Phase 7: Polish & Cross-Cutting

- [x] T029 [P] DATABASE_DESIGN §2.11 thêm `social.notifications`; VALKEY_DESIGN §3.1 `ew:chat:ntf:unread:*`; EVENT_DESIGN §3.2 consumer chat cho `party.invitation_sent`, `level.reached`, `achievement.unlocked`, `assignment.results_released`; CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/07-backend/VALKEY_DESIGN.md`, `docs/07-backend/EVENT_DESIGN.md`, `CHANGELOG.md` — thêm CHAT_SYSTEM §10, SOCIAL_SYSTEM §8, API_DESIGN §9.11, MONITORING
- [x] T030 [P] Metric `eduworld_notifications_*` + alert consumer pending — file: `src/backend/modules/chat/internal/infrastructure/observability/metrics.go` — `observability/notification_metrics.go`, alert `deploy/observability/prometheus/alerts/chat.yml`
- [ ] T031 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — backend: bằng chứng test ở T011–T023; Godot: test + ảnh ở [quickstart §4–§5](quickstart.md). Kịch bản end-to-end với full stack chưa chạy (không làm xáo trộn stack local đang chạy của owner)

## Dependencies & Execution Order

- Setup → Foundational (T010 learning trước US1 phần bài giao) → US1 → US2 → US3 (song song từ sau Foundational) → US4 (cần F092) → Polish.
- Nguồn party/phần thưởng chỉ kiểm end-to-end khi F100/F074 Done; trước đó test bằng event phát thật lên NATS testcontainer (không mock trong đường chạy thật).

## Definition of Done

- [x] Mọi tiêu chí F123-AC-* có test pass (ghi mã test/TC) — AC-01…14: T011–T023; AC-15…19: `test_notification_service.gd`, `test_notification_ui.gd`, `test_grade_band_matrix.gd`
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make test-contract` xanh cho module `chat` (và phần module `learning`); `make godot-test` xanh — Godot: 1135 test case — 1123 pass, 0 lỗi, 0 fail, 12 bỏ qua (test live cần backend); `scan_artifact_test` 22/22; export Web + `check_web_export` + `scan_artifact` OK
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — fake chỉ trong `tests/` và `tools/` (capture)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi — STUDENT_GAME_UX_CORE S03 §6c, STUDENT_GAME_UX_SOCIAL §10, DESIGN_SYSTEM §5.1, GODOT_ARCHITECTURE, CONFIG_DEFAULTS `game_notification_popups`, CHANGELOG, design/CHANGELOG
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
