# Tasks: F119 — Giao diện chat trong Godot: panel, quick message, tab, tắt tiếng, báo cáo

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 [P] Khóa dịch `ui.chat.*` (gồm `ui.chat.feedback.*` theo bảng CHAT_SYSTEM §4.3, `ui.chat.report.reason.*`, `ui.chat.retention_notice`, `ui.chat.reconnecting`, `ui.chat.muted_until`) — file: `src/game/locale/vi.po`, `src/game/locale/en.po` — 159 khóa `ui.chat.*` + `ui.party.chat`, `ui.chat.keyboard` trong vi.po/en.po; `messages.pot`, `xx_PS.po` sinh lại (2026-11-27)
- [x] T002 [P] Fixture sinh từ example OpenAPI/AsyncAPI (`Channel`, `Message`, `QuickMessage`, `StickerPack`, `GifAsset`, `chat.*` WS, `system.command.failed` các mã CHAT_*) — file: `src/game/tests/fixtures/chat/`, `src/game/tools/gen_fixtures.gd` — fixture sinh vào `tests/fixtures/contract/` bằng `tools/gen_fixtures.mjs` (tiền tố `chat.`, 6 response + 34 message chat); `tests/fakes/chat_fixtures.gd`
- [x] T003 [P] Typed model `ChatMessage`, `ChatChannel`, `ReceiptState` — file: `src/game/scripts/chat/models/` — `scripts/chat/models/{chat_channel_model,chat_message_model,receipt_state_model}.gd`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 `ChatStore` (insert theo `seq`, pending/sent/delivered/read, gap detect, giới hạn `chat_recent_buffer`, unread) — file: `src/game/scripts/chat/chat_store.gd` — `scripts/chat/chat_store.gd`
- [x] T005 `ChatService` khung: join/leave, send, sync, xử lý `system.command.failed`, retry `client_message_id` theo `client_reconnect_backoff` — file: `src/game/scripts/chat/chat_service.gd` — `scripts/chat/chat_service.gd` (+ REST fallback `sendChannelMessage`, `CommandResult.details`)
- [x] T006 [P] `ChatDock` (chế độ expanded/collapsed/mini) + đăng ký trong `ServiceLocator`; Prism mock cho `createReport` gắn nhãn MOCK chỉ build debug — file: `src/game/scenes/chat/ChatDock.tscn`, `src/game/scripts/core/app_state.gd`, `src/game/tests/fixtures/prism/README.md` — `scenes/chat/ChatDock.tscn` + `chat_dock.gd`; `ServiceLocator.chat/social/reports`; **không tạo Prism mock** — F121 đã Done, `createReport` gọi backend thật (spec Clarifications 2026-11-27)

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Panel chat luôn có trong party và trận (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [x] T007 [P] [US1] Scene test F119-AC-01, F119-AC-02, F119-AC-03: panel có trong lobby/trận/boss, không có nút tắt, badge khi thu gọn, 2 tab trận đội, thu nhỏ khi `EventBus.question_shown` — file: `src/game/tests/scene/chat/test_chat_dock.gd` — phải FAIL trước khi code — `tests/scene/chat/test_chat_dock.gd` + `tests/scene/social/test_party_lobby.gd::test_lobby_shows_the_party_chat_in_a_full_height_column`
- [x] T008 [P] [US1] Unit F119-AC-04: thứ tự `seq`, sent chỉ khi `acked`, gap → `chat.channel.sync`, > buffer → REST, gap không bù được chỉ thử một lần — file: `src/game/tests/unit/chat/test_chat_store.gd` — `tests/unit/chat/test_chat_store.gd`, `test_chat_service_retry.gd` (gap → sync, > buffer → REST)
- [x] T009 [P] [US1] Unit F119-AC-05, F119-AC-06: mất mạng/`CHAT_TEMPORARILY_UNAVAILABLE` → pending + gửi lại cùng `client_message_id`; join `NOT_FOUND` → retry; `CHAT_CHANNEL_CLOSED` → khóa ô nhập — file: `src/game/tests/unit/chat/test_chat_service_retry.gd` — `tests/unit/chat/test_chat_service_retry.gd`
- [x] T010 [P] [US1] Scene test F119-AC-07: thông báo "giữ N ngày" lấy N từ dữ liệu server — file: `src/game/tests/scene/chat/test_retention_notice.gd` — `tests/scene/chat/test_chat_dock.gd` (retention theo `purge_after`; gộp vào cùng file)

### Implementation cho User Story 1

- [x] T011 [US1] `ChatPanel`, `ChannelTabs`, `MessageList` (ảo hóa), `MessageBubble`, `RetentionNotice` — file: `src/game/scenes/chat/` — dựng bằng code: `chat_dock.gd` (tab, thu gọn, dải 2 dòng, lưu giữ), `chat_message_list.gd` (ảo hóa), `chat_bubble.gd`
- [x] T012 [US1] Gắn `ChatDock` vào `PartyLobby.tscn`, `BattleArena.tscn`, màn boss (theo `chat_channel_id`/`chat_channel_ids`) — file: `src/game/scenes/social/PartyLobby.tscn`, `src/game/scenes/competition/BattleArena.tscn` — `PartyLobby` (slot `EmbeddedChat`: PC cột hoạt động, điện thoại nút "Chat nhóm" + overlay); `BattleArena` chưa có (F109 Ready) → F109 gọi `ChatDock.show_match()` — chờ F109

**Checkpoint**: US1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Quick message một chạm và phản hồi kiểm duyệt thân thiện (Priority: P1)

### Tests cho User Story 2

- [x] T013 [P] [US2] Unit/scene F119-AC-08, F119-AC-09: quick bar luôn hiện, một chạm gửi `quick_message_id`, hiển thị theo locale người xem, density `g1_3`; đếm ký tự, Enter/Shift+Enter (TC-I18N-008) — file: `src/game/tests/scene/chat/test_quick_message_composer.gd` — phải FAIL trước khi code — `tests/scene/chat/test_chat_composer.gd`, `tests/unit/chat/test_quick_message_catalog.gd`
- [x] T014 [P] [US2] Unit F119-AC-10, F119-AC-11: `ChatFeedbackMapper` ánh xạ đúng bảng §4.3, không hiển thị chuỗi khớp; mute/sanction khóa ô nhập kèm giờ — file: `src/game/tests/unit/chat/test_feedback_mapper.gd` — `tests/unit/chat/test_feedback_mapper.gd`, `tests/unit/chat/test_chat_service.gd`

### Implementation cho User Story 2

- [x] T015 [US2] `QuickMessageCatalog`, `QuickMessageBar`, `Composer` — file: `src/game/scripts/chat/quick_message_catalog.gd`, `src/game/scenes/chat/QuickMessageBar.tscn`, `src/game/scenes/chat/Composer.tscn` — `quick_message_catalog.gd`, `chat_composer.gd` (hàng quick message + ô nhập + bảng mặt cười ⇄ gõ chữ)
- [x] T016 [US2] `ChatFeedbackMapper` + `ModerationFeedbackToast` — file: `src/game/scripts/chat/chat_feedback_mapper.gd`, `src/game/scenes/chat/ModerationFeedbackToast.tscn` — `chat_feedback_mapper.gd`; thông báo hiển thị trong `ChatDock` (băng + thông điệp quan tâm có Mây)

## Phase 5: User Story 3 — Tắt tiếng, chặn, báo cáo trong ≤ 2 chạm (Priority: P1)

### Tests cho User Story 3

- [x] T017 [P] [US3] Scene test F119-AC-12: menu Tắt tiếng/Chặn/Báo cáo ≤ 2 thao tác trên tin người khác, không có trên tin của mình (TC-CHAT-019, AC-MOD-023) — file: `src/game/tests/scene/chat/test_message_action_menu.gd` — phải FAIL trước khi code — `tests/scene/chat/test_chat_action_menu.gd`
- [x] T018 [P] [US3] Unit F119-AC-13, F119-AC-14: `ReportService` gửi `createReport` với reason enum, `Idempotency-Key` giữ khi retry, `match_id`/`party_id`; nút báo cáo/chặn ở màn kết quả trận (TC-CHAT-006) — file: `src/game/tests/unit/chat/test_report_service.gd` — `tests/unit/chat/test_report_service.gd`, `test_chat_action_menu.gd::test_match_result_safety_row_reports_with_the_match_id`
- [x] T019 [P] [US3] Unit F119-AC-15: chặn/tắt tiếng gọi `blockUser`/`muteUser`, ẩn ngay tin của người đó (TC-SOC-003) — file: `src/game/tests/unit/chat/test_block_mute_hide.gd` — `tests/unit/chat/test_block_mute_hide.gd`, `test_chat_action_menu.gd::test_block_from_the_menu_hides_the_player_at_once`

### Implementation cho User Story 3

- [x] T020 [US3] `MessageActionMenu` (mục an toàn), `ReportDialog`, `ReportService`, `SocialService` — file: `src/game/scenes/chat/MessageActionMenu.tscn`, `src/game/scenes/chat/ReportDialog.tscn`, `src/game/scripts/chat/report_service.gd`, `src/game/scripts/social/social_service.gd` — `chat_dialogs.gd` + `chat_sheet.gd` (menu, báo cáo), `report_service.gd`, `scripts/social/social_service.gd`
- [x] T021 [US3] Nút "Báo cáo người chơi"/"Chặn" ở màn kết quả trận và lịch sử party — file: `src/game/scenes/competition/BattleResult.tscn` — `chat_player_safety.gd` (thành phần cho `BattleResult` của F109) + nút "Người chơi cùng em" trong dock; nhúng vào `BattleResult` chờ F109

## Phase 6: User Story 4 — Emoji, sticker, GIF và thao tác trên tin (Priority: P2)

### Tests cho User Story 4

- [x] T022 [P] [US4] Scene test F119-AC-16, F119-AC-17: picker 3 tab, pack khóa có ổ khóa, GIF theo chủ đề nội bộ, không ô tìm Internet; ≤ 3 ảnh động phát, chỉ trong khung nhìn (TC-CHAT-033) — file: `src/game/tests/scene/chat/test_picker.gd` — phải FAIL trước khi code — `tests/scene/chat/test_chat_bubbles_picker.gd`, `tests/unit/chat/test_chat_assets_models.gd`
- [x] T023 [P] [US4] Unit/scene F119-AC-18, F119-AC-19, F119-AC-20: reaction toggle + lỗi giới hạn; mục menu theo `features`/quyền; thu hồi hỏi xác nhận; copy bản đã che (TC-CHAT-041); nhãn sửa/chuyển tiếp/placeholder; ✓/✓✓ (TC-CHAT-038) — file: `src/game/tests/scene/chat/test_message_features.gd` — `tests/scene/chat/test_chat_bubbles_picker.gd`, `test_chat_action_menu.gd`, `tests/unit/chat/test_chat_service.gd`

### Implementation cho User Story 4

- [x] T024 [US4] `EmojiStickerGifPicker`, `ChatAssetCache` — file: `src/game/scenes/chat/EmojiStickerGifPicker.tscn`, `src/game/scripts/chat/chat_asset_cache.gd` — `chat_picker.gd`, `chat_asset_cache.gd`, `chat_animation_budget.gd`
- [x] T025 [US4] Mục thao tác tin (reply/quote/forward/copy/pin/edit/recall/reaction), `PinnedBar`, `TypingIndicator`, `ReceiptBadge`, `MentionAutocomplete` — file: `src/game/scenes/chat/` — `chat_dialogs.gd` (trả lời, trích dẫn, chuyển tiếp, sao chép, ghim, sửa, thu hồi, ai đã xem, 6 cảm xúc), thanh ghim/typing/trạng thái trong `chat_dock.gd`/`chat_bubble.gd`, nhắc tên `@` trong `chat_composer.gd`

## Phase 7: User Story 5 — Hiệu năng, song ngữ, theme và ranh giới an toàn của client (Priority: P2)

### Tests cho User Story 5

- [x] T026 [P] [US5] Benchmark F119-AC-21: 500 tin, danh sách ảo hóa, gộp cập nhật ≤ 1/s, FPS thiết bị tầm trung (TC-PERF-009) — file: `src/game/tests/perf/test_chat_list_perf.gd` — phải FAIL trước khi code — `tests/perf/test_chat_list_perf.gd` (FPS máy tầm trung: đo lab — chờ)
- [x] T027 [P] [US5] CI F119-AC-22: `extract_pot` đủ khóa `ui.chat.*`, pseudo-locale `xx-PS`, `lint_theme_overrides.gd` không override trong `scenes/chat` (TC-I18N-006, TC-UI-003) — file: `src/game/tests/ci/test_chat_i18n_theme.gd` — `tests/unit/chat/test_chat_boundaries.gd` (khóa vi/en, không override/màu) + `extract_pot --check`, `pseudo_locale --check`, `lint_theme_overrides`, ma trận pseudo-locale
- [x] T028 [P] [US5] Test tĩnh F119-AC-23: không import/URL NATS/Valkey, không danh mục/logic kiểm duyệt, không tùy chọn tắt chat gameplay trong `scripts/chat`, `scenes/chat` — file: `src/game/tests/ci/test_chat_boundaries.gd` — `tests/unit/chat/test_chat_boundaries.gd`

### Implementation cho User Story 5

- [x] T029 [US5] Tối ưu `MessageList` (pool bubble, gộp cập nhật), lọc `body` khỏi logger client — file: `src/game/scenes/chat/MessageList.tscn`, `src/game/scripts/core/logger.gd` — pool bubble + đo chiều cao trong `chat_message_list.gd`, gộp typing/receipt ≤ 1 lần/giây trong `chat_dock.gd`; `logger.gd` lọc `body`, `excerpt`, `preview`, `reply_excerpt`, `quote_excerpt`, `draft`, `chat_text`

## Phase 8: Polish & Cross-Cutting

- [x] T030 [P] Sửa wireframe STUDENT_GAME_UX_SOCIAL §7 (bỏ tin ghim "Nhà trường"); CHANGELOG — file: `docs/05-ux/STUDENT_GAME_UX_SOCIAL.md`, `CHANGELOG.md` — wireframe §7 đã không còn tin ghim "Nhà trường"; thêm lát cắt client §7; CHANGELOG
- [x] T031 Gỡ Prism mock `createReport` khi F121 Done, chạy lại US3 với backend thật — file: `src/game/tests/fixtures/prism/` — không có Prism mock để gỡ; chạy lại US3 với backend thật = `tests/integration/test_chat_flow_headless.gd` (bỏ qua khi chưa có backend — **chờ chạy thật**)
- [x] T033 Sửa bố cục "≥ 3 tin" (F119 Clarifications 2026-11-27; STUDENT_GAME_UX_SOCIAL §7): tab bảng một dòng (`ChatTabButton`), "Gõ chữ" · tab · hình một hàng khi panel rộng, giờ cạnh tên trong bong bóng, tab trận đội trên dòng tiêu đề, màn Nhóm PC cột chat cao hết cửa sổ, panel điện thoại từ đỉnh thanh tiêu đề, sửa `ChipScroller`; probe `ChatSpaceProbe` + test âm trên bố cục cũ — file: `src/game/scenes/chat/chat_{picker,bubble,dock}.gd`, `src/game/scenes/social/party_lobby.gd`, `src/game/scenes/ui/components/chip_scroller.gd`, `design/sd/formats/godot-tres.mjs`, `src/game/tests/fakes/chat_space_probe.gd`, `src/game/tests/scene/test_grade_band_matrix.gd` — `test_chat_list_shows_three_messages_in_every_band`, `test_chat_space_probe_flags_the_old_thin_message_list`, `test_chat_dock.gd::test_team_tabs_take_the_place_of_the_title_on_the_header_line`, `test_chat_bubbles_picker.gd::test_time_sits_beside_the_name_of_another_player`, `::test_board_tabs_keep_one_line_and_share_the_row_when_wide`, `test_party_lobby.gd::test_lobby_shows_the_party_chat_in_a_full_height_column`; ảnh ở quickstart §5.3
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
