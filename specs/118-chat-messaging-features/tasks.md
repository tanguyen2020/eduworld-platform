# Tasks: F118 — Bộ tính năng nhắn tin đầy đủ (sửa/thu hồi, reply/forward, mention, receipts) và sticker/GIF

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Đăng ký permission platform-only `game.chat_assets.write` (kiểm khởi động, không trong role tenant) — file: `pkg/authz/catalog.go`
- [ ] T002 [P] Cấu hình bucket/CDN asset chat, giới hạn tệp, bộ emoji/reaction cho phép (cấu hình platform có version) — file: `modules/chat/internal/config/assets.go`, `modules/chat/seed/emoji_sets.json`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T003 Delta OpenAPI: `toggleMessageReaction`, `CHAT_REACTION_LIMIT`, presign asset chat, `forwardMessage` `428`; AsyncAPI `ErrorCode` thêm `CHAT_REACTION_LIMIT` — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=chat`
- [ ] T004 [P] Event payload `chat-evt`, `chat.message_edited/recalled/forwarded`, `chat.message_pinned/unpinned`, `content.published` (chat) — file: `specs/events/payloads/chat/` — kiểm: `make specs-lint`
- [ ] T005 [P] Xác nhận payload tiêu thụ đã có: `party.leader_transferred` (owner F100), `item.granted` (owner F075, cần `item_id` để tra sticker mở khóa); thiếu → chặn consumer tương ứng, không tự tạo — file: `specs/events/payloads/competition/`, `specs/events/payloads/game/` (chỉ đọc)
- [ ] T006 ent schema `MessageRevision`, `MessageReaction`, `PinnedMessage`, `StickerPack`, `Sticker` (+`alt_text_en`), `GifAsset` (+`created_by`), `GifTag`, `ChatAssetVersion`, `RefStickerUnlock` — file: `modules/chat/ent/schema/` — kiểm: `make migrate-diff MOD=chat name=f118_features`, `make migrate-lint MOD=chat`
- [ ] T007 Migration: trigger ≤ `chat_pins_max` pin active, bất biến `chat_asset_versions` sau publish, RLS bảng T — file: `modules/chat/migrations/`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Sửa và thu hồi tin mà không xóa được bằng chứng (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [ ] T008 [P] [US1] Integration F118-AC-01, F118-AC-02, F118-AC-03: sửa chạy lại pipeline, revision, ack trước fan-out; quá hạn/không phải người gửi/không phải text → 409; `If-Match` lệch → 412; bản sửa bị block/hold giữ bản cũ (TC-CHAT-029) — file: `modules/chat/tests/integration/edit_message_test.go` — phải FAIL trước khi code
- [ ] T009 [P] [US1] Integration F118-AC-04, F118-AC-05, F118-AC-06: thu hồi → placeholder, bỏ ghim, nội dung gốc + revision còn; lịch sử/quote/reply/forward/mention preview không lộ nội dung (TC-CHAT-030, TC-CHAT-031) — file: `modules/chat/tests/integration/recall_message_test.go`
- [ ] T010 [P] [US1] Integration persister áp `evt` theo `revision`, bỏ bản đến muộn — file: `modules/chat/tests/integration/persister_evt_order_test.go`

### Implementation cho User Story 1

- [ ] T011 [P] [US1] Domain `Edit`, `Recall` — file: `modules/chat/internal/domain/message/edit.go`, `modules/chat/internal/domain/message/recall.go`
- [ ] T012 [US1] Command `EditMessage`, `RecallMessage` + JetStream `evt` publisher — file: `modules/chat/internal/application/command/edit_message.go`, `modules/chat/internal/application/command/recall_message.go`
- [ ] T013 [US1] Handler `editMessage`, `recallMessage`, NATS `ew.cmd.chat.message.edit/recall`; persister `apply_evt.go` — file: `modules/chat/internal/presentation/http/public/messages_edit.go`, `modules/chat/internal/presentation/worker/persister/apply_evt.go`

## Phase 4: User Story 2 — Trả lời, trích dẫn, chuyển tiếp an toàn (Priority: P1)

### Tests cho User Story 2

- [ ] T014 [P] [US2] Unit/integration F118-AC-07, F118-AC-08: reply cùng channel, preview ≤ 80; excerpt server cắt từ bản đã che, client không gửi excerpt (TC-CHAT-031) — file: `modules/chat/tests/integration/reply_quote_test.go` — phải FAIL trước khi code
- [ ] T015 [P] [US2] Integration F118-AC-09, F118-AC-10, F118-AC-11: forward chạy pipeline đích, khác tenant/không quyền/nguồn world-area/tin thu hồi-che-gỡ-held → `CHAT_FORWARD_NOT_ALLOWED`, quá số đích → 422; idempotency (TC-CHAT-032) — file: `modules/chat/tests/integration/forward_message_test.go`

### Implementation cho User Story 2

- [ ] T016 [P] [US2] Domain `QuoteExcerpt`, `ForwardTo` — file: `modules/chat/internal/domain/message/quote.go`, `modules/chat/internal/domain/message/forward.go`
- [ ] T017 [US2] Mở rộng `SendMessage` (reply/quote) + command `ForwardMessage` + handler `forwardMessage`, NATS `chat.message.forward` — file: `modules/chat/internal/application/command/forward_message.go`

## Phase 5: User Story 3 — Emoji, sticker, GIF và reaction từ thư viện đã duyệt (Priority: P2)

### Tests cho User Story 3

- [ ] T018 [P] [US3] Integration F118-AC-12, F118-AC-13, F118-AC-14, F118-AC-15: sticker chưa publish/chưa gán tenant/chưa mở khóa → `CHAT_ASSET_NOT_AVAILABLE`; GIF chỉ nội bộ; list pack theo tenant + trạng thái mở khóa; rate sticker/GIF (TC-CHAT-033) — file: `modules/chat/tests/integration/sticker_gif_send_test.go` — phải FAIL trước khi code
- [ ] T019 [P] [US3] Integration F118-AC-16: toggle reaction, ≤ `chat_reactions_per_user`, không trên tin thu hồi, số đếm đồng bộ (TC-CHAT-034) — file: `modules/chat/tests/integration/reaction_test.go`
- [ ] T020 [P] [US3] Test authorization + kiểm tệp F118-AC-17: kích thước/định dạng, người duyệt ≠ người upload, version bất biến, role tenant 403 — file: `modules/chat/tests/integration/chat_assets_admin_test.go`

### Implementation cho User Story 3

- [ ] T021 [P] [US3] Domain `chatasset` + `Reactions.Toggle` — file: `modules/chat/internal/domain/chatasset/`, `modules/chat/internal/domain/message/reaction.go`
- [ ] T022 [US3] Query `ListStickerPacks`, `ListGifs`; command `ToggleReaction`; handler REST/NATS — file: `modules/chat/internal/application/query/list_sticker_packs.go`, `modules/chat/internal/application/command/toggle_reaction.go`
- [ ] T023 [US3] Game-admin `*_chat` (sticker-packs, gifs) + presign + kiểm tệp + `content.published`; consumer unlock → `ref.sticker_unlocks` — file: `modules/chat/internal/presentation/http/gameadmin/chat_assets.go`, `modules/chat/internal/presentation/consumer/game_unlocks.go`

## Phase 6: User Story 4 — Mention, ghim, đang soạn tin (Priority: P2)

### Tests cho User Story 4

- [ ] T024 [P] [US4] Integration F118-AC-18, F118-AC-19: > max → `CHAT_MENTION_LIMIT`; loại mention không hợp lệ; `unread_mentions`, `chat.mention.created` không username (TC-CHAT-035) — file: `modules/chat/tests/integration/mention_test.go` — phải FAIL trước khi code
- [ ] T025 [P] [US4] Integration F118-AC-20: quyền pin theo loại channel/leader, giới hạn, audit, `CHAT_FEATURE_DISABLED` (TC-CHAT-036) — file: `modules/chat/tests/integration/pin_test.go`
- [ ] T026 [P] [US4] Integration F118-AC-21: typing throttle/hết hạn, không DB, không tới người chặn, không world/area (TC-CHAT-037) — file: `modules/chat/tests/integration/typing_test.go`

### Implementation cho User Story 4

- [ ] T027 [US4] `Mentions.Filter` trong `SendMessage`/`EditMessage`, bộ đếm mention — file: `modules/chat/internal/domain/message/mention.go`
- [ ] T028 [US4] Command `PinMessage`/`UnpinMessage` + consumer `party.created`/`party.leader_transferred` (role leader) — file: `modules/chat/internal/application/command/pin_message.go`, `modules/chat/internal/presentation/consumer/party_leader.go`
- [ ] T029 [US4] Typing qua Valkey pub/sub — file: `modules/chat/internal/infrastructure/valkey/typing.go`, `modules/chat/internal/presentation/natscmd/typing.go`

## Phase 7: User Story 5 — Đã gửi / đã nhận / đã xem và bộ đếm chưa đọc (Priority: P2)

### Tests cho User Story 5

- [ ] T030 [P] [US5] Integration F118-AC-22, F118-AC-23, F118-AC-24: con trỏ gộp không lùi, không bảng tin × người; receipts per_member/count/disabled; ẩn giữa người chặn (TC-CHAT-038) — file: `modules/chat/tests/integration/receipts_test.go` — phải FAIL trước khi code
- [ ] T031 [P] [US5] Integration F118-AC-25: unread + mention count đa thiết bị (TC-CHAT-039) — file: `modules/chat/tests/integration/unread_mentions_test.go`

### Implementation cho User Story 5

- [ ] T032 [US5] Domain `MemberCursor`, command `UpdateCursor` (delivered), query `GetMessageReceipts`, handler `getMessageReceipts`, NATS `chat.receipt.delivered` — file: `modules/chat/internal/domain/cursor/`, `modules/chat/internal/application/query/get_message_receipts.go`

## Phase 8: Polish & Cross-Cutting

- [ ] T033 [P] Test tenant isolation + authorization chung F118-AC-26, F118-AC-27: mọi endpoint/lệnh mới × {không member, tenant khác, 5 role nhân sự, header lệch}; tính năng tắt trong `features` → `CHAT_FEATURE_DISABLED`; tính năng "luôn" ở gameplay không tắt được (TC-TEN-001) — file: `modules/chat/tests/integration/features_authz_test.go`
- [ ] T034 [P] Metric `eduworld_chat_events_total{event}`, receipt/typing/asset — file: `modules/chat/internal/infrastructure/observability/feature_metrics.go`
- [ ] T035 [P] Cập nhật tài liệu: CONFIG_DEFAULTS (trần sticker/GIF, typing, gộp receipt, giới hạn tệp), DATABASE_DESIGN (`chat_asset_versions`, `ref.sticker_unlocks`, cột mới), EVENT_DESIGN §4.7 (chat tiêu thụ `party.leader_transferred`), API_DESIGN §5 (`CHAT_MENTION_LIMIT`, `CHAT_REACTION_LIMIT`), CHANGELOG — file: `docs/00-overview/CONFIG_DEFAULTS.md`, `docs/07-backend/DATABASE_DESIGN.md`, `docs/07-backend/EVENT_DESIGN.md`, `docs/07-backend/API_DESIGN.md`, `CHANGELOG.md`
- [ ] T036 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → US3, US4, US5 song song → Polish.
- Cần F115–F117; F075 cho pack khóa; F045 cho gán tenant.

## Definition of Done

- [ ] Mọi tiêu chí F118-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make migrate-lint`, `make test-contract` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
