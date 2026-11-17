# Tasks: F119 — Giao diện chat trong Godot: panel, quick message, tab, tắt tiếng, báo cáo

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 [P] Khóa dịch `ui.chat.*` (gồm `ui.chat.feedback.*` theo bảng CHAT_SYSTEM §4.3, `ui.chat.report.reason.*`, `ui.chat.retention_notice`, `ui.chat.reconnecting`, `ui.chat.muted_until`) — file: `godot/locale/vi.po`, `godot/locale/en.po`
- [ ] T002 [P] Fixture sinh từ example OpenAPI/AsyncAPI (`Channel`, `Message`, `QuickMessage`, `StickerPack`, `GifAsset`, `chat.*` WS, `system.command.failed` các mã CHAT_*) — file: `godot/tests/fixtures/chat/`, `tools/godot/gen_fixtures.gd`
- [ ] T003 [P] Typed model `ChatMessage`, `ChatChannel`, `ReceiptState` — file: `godot/scripts/chat/models/`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 `ChatStore` (insert theo `seq`, pending/sent/delivered/read, gap detect, giới hạn `chat_recent_buffer`, unread) — file: `godot/scripts/chat/chat_store.gd`
- [ ] T005 `ChatService` khung: join/leave, send, sync, xử lý `system.command.failed`, retry `client_message_id` theo `client_reconnect_backoff` — file: `godot/scripts/chat/chat_service.gd`
- [ ] T006 [P] `ChatDock` (chế độ expanded/collapsed/mini) + đăng ký trong `ServiceLocator`; Prism mock cho `createReport` gắn nhãn MOCK chỉ build debug — file: `godot/scenes/chat/ChatDock.tscn`, `godot/scripts/core/app_state.gd`, `godot/tests/fixtures/prism/README.md`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Panel chat luôn có trong party và trận (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [ ] T007 [P] [US1] Scene test F119-AC-01, F119-AC-02, F119-AC-03: panel có trong lobby/trận/boss, không có nút tắt, badge khi thu gọn, 2 tab trận đội, thu nhỏ khi `EventBus.question_shown` — file: `godot/tests/scene/chat/test_chat_dock.gd` — phải FAIL trước khi code
- [ ] T008 [P] [US1] Unit F119-AC-04: thứ tự `seq`, sent chỉ khi `acked`, gap → `chat.sync`, > buffer → REST, gap không bù được chỉ thử một lần — file: `godot/tests/unit/chat/test_chat_store.gd`
- [ ] T009 [P] [US1] Unit F119-AC-05, F119-AC-06: mất mạng/`CHAT_TEMPORARILY_UNAVAILABLE` → pending + gửi lại cùng `client_message_id`; join `NOT_FOUND` → retry; `CHAT_CHANNEL_CLOSED` → khóa ô nhập — file: `godot/tests/unit/chat/test_chat_service_retry.gd`
- [ ] T010 [P] [US1] Scene test F119-AC-07: thông báo "giữ N ngày" lấy N từ dữ liệu server — file: `godot/tests/scene/chat/test_retention_notice.gd`

### Implementation cho User Story 1

- [ ] T011 [US1] `ChatPanel`, `ChannelTabs`, `MessageList` (ảo hóa), `MessageBubble`, `RetentionNotice` — file: `godot/scenes/chat/`
- [ ] T012 [US1] Gắn `ChatDock` vào `PartyLobby.tscn`, `BattleArena.tscn`, màn boss (theo `chat_channel_id`/`chat_channel_ids`) — file: `godot/scenes/social/PartyLobby.tscn`, `godot/scenes/competition/BattleArena.tscn`

**Checkpoint**: US1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Quick message một chạm và phản hồi kiểm duyệt thân thiện (Priority: P1)

### Tests cho User Story 2

- [ ] T013 [P] [US2] Unit/scene F119-AC-08, F119-AC-09: quick bar luôn hiện, một chạm gửi `quick_message_id`, hiển thị theo locale người xem, density `g1_3`; đếm ký tự, Enter/Shift+Enter (TC-I18N-008) — file: `godot/tests/scene/chat/test_quick_message_composer.gd` — phải FAIL trước khi code
- [ ] T014 [P] [US2] Unit F119-AC-10, F119-AC-11: `ChatFeedbackMapper` ánh xạ đúng bảng §4.3, không hiển thị chuỗi khớp; mute/sanction khóa ô nhập kèm giờ — file: `godot/tests/unit/chat/test_feedback_mapper.gd`

### Implementation cho User Story 2

- [ ] T015 [US2] `QuickMessageCatalog`, `QuickMessageBar`, `Composer` — file: `godot/scripts/chat/quick_message_catalog.gd`, `godot/scenes/chat/QuickMessageBar.tscn`, `godot/scenes/chat/Composer.tscn`
- [ ] T016 [US2] `ChatFeedbackMapper` + `ModerationFeedbackToast` — file: `godot/scripts/chat/chat_feedback_mapper.gd`, `godot/scenes/chat/ModerationFeedbackToast.tscn`

## Phase 5: User Story 3 — Tắt tiếng, chặn, báo cáo trong ≤ 2 chạm (Priority: P1)

### Tests cho User Story 3

- [ ] T017 [P] [US3] Scene test F119-AC-12: menu Tắt tiếng/Chặn/Báo cáo ≤ 2 thao tác trên tin người khác, không có trên tin của mình (TC-CHAT-019, AC-MOD-023) — file: `godot/tests/scene/chat/test_message_action_menu.gd` — phải FAIL trước khi code
- [ ] T018 [P] [US3] Unit F119-AC-13, F119-AC-14: `ReportService` gửi `createReport` với reason enum, `Idempotency-Key` giữ khi retry, `match_id`/`party_id`; nút báo cáo/chặn ở màn kết quả trận (TC-CHAT-006) — file: `godot/tests/unit/chat/test_report_service.gd`
- [ ] T019 [P] [US3] Unit F119-AC-15: chặn/tắt tiếng gọi `blockUser`/`muteUser`, ẩn ngay tin của người đó (TC-SOC-003) — file: `godot/tests/unit/chat/test_block_mute_hide.gd`

### Implementation cho User Story 3

- [ ] T020 [US3] `MessageActionMenu` (mục an toàn), `ReportDialog`, `ReportService`, `SocialService` — file: `godot/scenes/chat/MessageActionMenu.tscn`, `godot/scenes/chat/ReportDialog.tscn`, `godot/scripts/chat/report_service.gd`, `godot/scripts/social/social_service.gd`
- [ ] T021 [US3] Nút "Báo cáo người chơi"/"Chặn" ở màn kết quả trận và lịch sử party — file: `godot/scenes/competition/BattleResult.tscn`

## Phase 6: User Story 4 — Emoji, sticker, GIF và thao tác trên tin (Priority: P2)

### Tests cho User Story 4

- [ ] T022 [P] [US4] Scene test F119-AC-16, F119-AC-17: picker 3 tab, pack khóa có ổ khóa, GIF theo chủ đề nội bộ, không ô tìm Internet; ≤ 3 ảnh động phát, chỉ trong khung nhìn (TC-CHAT-033) — file: `godot/tests/scene/chat/test_picker.gd` — phải FAIL trước khi code
- [ ] T023 [P] [US4] Unit/scene F119-AC-18, F119-AC-19, F119-AC-20: reaction toggle + lỗi giới hạn; mục menu theo `features`/quyền; thu hồi hỏi xác nhận; copy bản đã che (TC-CHAT-041); nhãn sửa/chuyển tiếp/placeholder; ✓/✓✓ (TC-CHAT-038) — file: `godot/tests/scene/chat/test_message_features.gd`

### Implementation cho User Story 4

- [ ] T024 [US4] `EmojiStickerGifPicker`, `ChatAssetCache` — file: `godot/scenes/chat/EmojiStickerGifPicker.tscn`, `godot/scripts/chat/chat_asset_cache.gd`
- [ ] T025 [US4] Mục thao tác tin (reply/quote/forward/copy/pin/edit/recall/reaction), `PinnedBar`, `TypingIndicator`, `ReceiptBadge`, `MentionAutocomplete` — file: `godot/scenes/chat/`

## Phase 7: User Story 5 — Hiệu năng, song ngữ, theme và ranh giới an toàn của client (Priority: P2)

### Tests cho User Story 5

- [ ] T026 [P] [US5] Benchmark F119-AC-21: 500 tin, danh sách ảo hóa, gộp cập nhật ≤ 1/s, FPS thiết bị tầm trung (TC-PERF-009) — file: `godot/tests/perf/test_chat_list_perf.gd` — phải FAIL trước khi code
- [ ] T027 [P] [US5] CI F119-AC-22: `extract_pot` đủ khóa `ui.chat.*`, pseudo-locale `xx-PS`, `lint_theme_overrides.gd` không override trong `scenes/chat` (TC-I18N-006, TC-UI-003) — file: `godot/tests/ci/test_chat_i18n_theme.gd`
- [ ] T028 [P] [US5] Test tĩnh F119-AC-23: không import/URL NATS/Valkey, không danh mục/logic kiểm duyệt, không tùy chọn tắt chat gameplay trong `scripts/chat`, `scenes/chat` — file: `godot/tests/ci/test_chat_boundaries.gd`

### Implementation cho User Story 5

- [ ] T029 [US5] Tối ưu `MessageList` (pool bubble, gộp cập nhật), lọc `body` khỏi logger client — file: `godot/scenes/chat/MessageList.tscn`, `godot/scripts/core/logger.gd`

## Phase 8: Polish & Cross-Cutting

- [ ] T030 [P] Sửa wireframe STUDENT_GAME_UX_SOCIAL §7 (bỏ tin ghim "Nhà trường"); CHANGELOG — file: `docs/05-ux/STUDENT_GAME_UX_SOCIAL.md`, `CHANGELOG.md`
- [ ] T031 Gỡ Prism mock `createReport` khi F121 Done, chạy lại US3 với backend thật — file: `godot/tests/fixtures/prism/`
- [ ] T032 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2, US3 song song → US4 → US5 → Polish.
- Cần F092 (`RealtimeClient`), F102/F109 (màn chứa panel), F115–F118 (backend chat); F121 để chuyển `Done`.

## Definition of Done

- [ ] Mọi tiêu chí F119-AC-* có test pass (ghi mã test/TC)
- [ ] `make godot-test`, `make godot-export-check` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
