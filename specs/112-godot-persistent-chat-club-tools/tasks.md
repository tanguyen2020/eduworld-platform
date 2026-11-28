# Tasks: F112 — Godot — channel chat bền vững (khu vực, DM) và công cụ chủ club (sửa club, phó nhóm, mục tiêu/sự kiện club)

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

Quy ước: test viết trước và phải FAIL trước khi code của story đó; fake/fixture gắn nhãn TEST DOUBLE (R17); mọi đường dẫn dưới `src/game/`.

## Phase 1: Setup

- [x] T001 [P] Khóa dịch `ui.chat.hub.*`, `ui.chat.dm.*`, `ui.chat.search.*`, `ui.club.manage.*` và `ui.error.<CODE>` còn thiếu (vi/en), sinh lại `messages.pot`, `xx_PS.po` — file: `src/game/locale/vi.po`, `src/game/locale/en.po` — **xong — 88 khóa `ui.chat.hub.*`, `ui.chat.dm.*`, `ui.chat.search.*`, `ui.club.manage.*`, `ui.error.VERSION_MISMATCH`, `ui.error.CHAT_NOT_ALLOWED` (vi + en); `messages.pot`, `xx_PS.po` sinh lại (`extract_pot`, `pseudo_locale` --check xanh)**
- [x] T002 [P] Fixture contract (`openDirectChannel` 200/201/403, `searchMessages`, `updateClub` 200/412/451, `updateClubMemberRole`, `createClubGoal`, `createClubEvent`, `chat.channel.membership_changed` frozen/read_only/rời club, `system.command.failed` `CHAT_NOT_ALLOWED`/`CURFEW_ACTIVE`); thiếu example thì thêm chỉ ví dụ vào bundle OpenAPI/AsyncAPI rồi tách module (contracts §6) — file: `src/game/tools/gen_fixtures.mjs`, `src/game/tests/fixtures/contract/`, `specs/openapi/eduworld-api.v1.yaml` — **xong — commit riêng "F112 contract fixtures": `gen_fixtures.mjs` đọc thêm `application/problem+json`, thêm `openDirectChannel` 200/201/403, `searchMessages` 200, `updateClub` 200/412/451, `updateClubMemberRole` 200, `createClubGoal` 201/422, `createClubEvent` 201/422; AsyncAPI `chat.channel.membership_changed` (`dm_frozen`, `club_read_only`, `club_left`) và `system.command.failed` (`chat_not_allowed*`, `curfew_active`) đã có; `--check` OK (285 file). Không cần sửa bundle OpenAPI/AsyncAPI**
- [x] T003 [P] Typed model `PersistentChannelTab`, `DirectPeer`, `SearchHit` — file: `src/game/scripts/chat/models/` — **xong — `scripts/chat/models/persistent_channel_tab.gd`; `DirectPeer` là thẻ công khai trong `DirectPeerMap` (`scripts/chat/direct_peer_map.gd`); `SearchHit` là `{message, channel_id, channel_type, excerpt, ranges}` của `ChatSearchService.hit_of`; `ChatChannelModel` thêm `sendable_kinds`/`has_sendable`/`peer`**

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Kiểm tiền đề: F110 (`SocialService.list_friends`, `FriendProfilePanel`) và F111 (`ClubService`, trang club) đã có; nếu chưa, dừng và làm F110/F111 trước (CODE_PLAN) — file: `src/game/scripts/social/`, `src/game/scenes/social/` — **xong — F110 (`SocialService`, `FriendProfilePanel`) và F111 (`ClubService`, trang club) đã có trên main**
- [x] T005 `ChatDock.show_channel(channel_id)` cho channel bền vững (dùng lại danh sách, ô soạn, menu F119; không đổi `show_party`/`show_match`) — file: `src/game/scenes/chat/chat_dock.gd` — **xong — `ChatDock.show_channel(channel_id, type, title)` (`CONTEXT_PERSISTENT`, `hub_mode` ẩn đầu dock), `jump_to(key)` (tô sáng 2 s), `refresh_lock()`; `show_party`/`show_match`/`show_club` giữ nguyên**
- [x] T006 `ChatHubState` khung (tab từ `listMyChannels` + club + bạn, join/leave theo tab) và đăng ký `ServiceLocator` — file: `src/game/scripts/chat/chat_hub_state.gd`, `src/game/scripts/core/service_locator.gd` — **xong — `ChatHubState` (thuần) + `ChatHubService` (keo ChatService/ClubService/SocialService/HubSync) đăng ký `ServiceLocator.chat_hub` (+ `chat_search`, `club_manage`), xóa ở `start_session`/`end_session`**
- [x] T007 Slot HUD: `MainHud` bỏ `ChatDock` khỏi `FUTURE_SLOTS`, gắn nút chat + `ChatHub` (sửa comment ghi F110) — file: `src/game/scenes/ui/hud/main_hud.gd`, `src/game/scenes/ui/hud/MainHud.tscn` — **xong — `ChatHubButton` ở hàng nút dưới-phải của HUD (trước "Về Quảng trường"; không đè toast/joystick), badge số + "@" tĩnh, ẩn khi thi; slot `ChatDock` cũ vẫn ẩn, comment sửa (bỏ ghi F110); màn `chat` trong `UiScreenRegistry` (`ChatHub.tscn`)**

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Khung chat HUD với các channel bền vững (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [x] T008 [P] [US1] Unit F112-AC-01, F112-AC-03, F112-AC-04: tab đúng loại server trả, bỏ loại lạ, không tab lớp/khối; `hub.instance.assigned` → leave area cũ + xóa RAM + area mới; join/leave theo tab; World/Area không unread (TC-CHAT-002, TC-CHAT-008, TC-CHAT-039) — file: `src/game/tests/unit/chat/test_chat_hub_state.gd` — phải FAIL trước khi code — **xong — `tests/unit/chat/test_chat_hub_state.gd` 7 test; đỏ trước khi code (2026-11-28: 12 file test F112 không nạp được — lỗi phân tích: thiếu `ChatHub`, `ChatHubService`, `ChatHubState`, `ChatSearchService`, `ChatSearchPanel`, `ClubManage`, `ClubManageService`, `ClubFormOptions`, `HighlightText`, `PersistentChannelTab`, `ChatHubButton`, `ClubPage.ACT_MANAGE`, `ChatDock.CONTEXT_PERSISTENT` — 95 lỗi script) → xanh**
- [x] T009 [P] [US1] Scene F112-AC-02, F112-AC-05, F112-AC-06: tab dùng `ChatDock.show_channel`, báo cáo có `context.channel_id`; badge HUD "99+" + `@` tĩnh; ẩn khi `blocking_activity` bài thi, chat gameplay không đổi (TC-CHAT-006) — file: `src/game/tests/scene/chat/test_chat_hub.gd` — phải FAIL trước khi code — **xong — `tests/scene/chat/test_chat_hub.gd` 6 test; đỏ trước khi code (2026-11-28: 12 file test F112 không nạp được — lỗi phân tích: thiếu `ChatHub`, `ChatHubService`, `ChatHubState`, `ChatSearchService`, `ChatSearchPanel`, `ClubManage`, `ClubManageService`, `ClubFormOptions`, `HighlightText`, `PersistentChannelTab`, `ChatHubButton`, `ClubPage.ACT_MANAGE`, `ChatDock.CONTEXT_PERSISTENT` — 95 lỗi script) → lần chạy đầu đỏ 6 lỗi (tab "Khu vực" cần khu của HubSync, tab tự chọn khi đang mở, join cần ack trong fake) → sửa `in_hub()`, `_opening` → xanh**

### Implementation cho User Story 1

- [x] T010 [US1] `ChatHub` (khung nổi PC/tablet, tấm phủ điện thoại, ✕ Đóng, kính lúp) + `chat_hub_tabs.gd` (ChipScroller, badge) — file: `src/game/scenes/chat/ChatHub.tscn`, `src/game/scenes/chat/chat_hub.gd`, `src/game/scenes/chat/chat_hub_tabs.gd` — **xong — `scenes/chat/ChatHub.tscn` + `chat_hub.gd` (PC/tablet khung trái 40% cao gần hết màn; điện thoại tấm phủ cả màn, hàng tab trên dòng thanh tiêu đề; kính lúp; "✕ Đóng" đỏ duy nhất), hàng tab trong `chat_hub.gd` (ChipScroller, badge số/"@", biểu tượng chuông gạch cho kênh tắt thông báo)**
- [x] T011 [US1] `ChatService`: làm mới `listMyChannels` khi `hub.instance.assigned`, leave/join theo `ChatHubState`, badge từ `chat.unread.updated`/`chat.mention.created`; rời mọi channel bền vững khi `blocking_activity` — file: `src/game/scripts/chat/chat_service.gd` — **xong — `ChatService.load_channels` phát `channels_listed` (giữ cả World/Area), `ChatHubService` làm mới khi `HubSync.room_assigned`, rời + xóa RAM channel `area` cũ, badge từ store (`chat.unread.updated` thay giá trị) gộp ≤ 1 lần/giây, rời mọi channel bền vững khi `blocking_activity = exam`**

**Checkpoint**: US1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Nhắn tin riêng với bạn bè (Priority: P1)

### Tests cho User Story 2

- [x] T012 [P] [US2] Unit F112-AC-07, F112-AC-08, F112-AC-10: `open_direct` gọi `openDirectChannel` (200 = 201), ánh xạ RAM, nút theo `friends_enabled`/`direct_message_enabled`, `403`/`404` → câu chung + ẩn nút tới hết phiên, không gọi hàng loạt, DM chưa biết người kia chỉ cộng badge (TC-CHAT-005, TC-CHAT-047, TC-CHAT-039) — file: `src/game/tests/unit/chat/test_direct_channels.gd` — phải FAIL trước khi code — **xong — `tests/unit/chat/test_direct_channels.gd` 5 test; đỏ trước khi code (2026-11-28: 12 file test F112 không nạp được — lỗi phân tích: thiếu `ChatHub`, `ChatHubService`, `ChatHubState`, `ChatSearchService`, `ChatSearchPanel`, `ClubManage`, `ClubManageService`, `ClubFormOptions`, `HighlightText`, `PersistentChannelTab`, `ChatHubButton`, `ClubPage.ACT_MANAGE`, `ChatDock.CONTEXT_PERSISTENT` — 95 lỗi script) → xanh**
- [x] T013 [P] [US2] Scene F112-AC-09: `frozen` khóa ô soạn với câu trung tính, đọc lịch sử, `open` mở khóa; không chuỗi nào nêu "chặn" (TC-CHAT-005) — file: `src/game/tests/scene/chat/test_chat_dm.gd` — phải FAIL trước khi code — **xong — `test_persistent_channel_states.gd::test_frozen_dm_locks_with_a_neutral_sentence_and_unlocks`; đỏ trước khi code (2026-11-28: 12 file test F112 không nạp được — lỗi phân tích: thiếu `ChatHub`, `ChatHubService`, `ChatHubState`, `ChatSearchService`, `ChatSearchPanel`, `ClubManage`, `ClubManageService`, `ClubFormOptions`, `HighlightText`, `PersistentChannelTab`, `ChatHubButton`, `ClubPage.ACT_MANAGE`, `ChatDock.CONTEXT_PERSISTENT` — 95 lỗi script) → xanh**

### Implementation cho User Story 2

- [x] T014 [US2] `DirectPeerMap` + `ChatService.open_direct(user_id)` — file: `src/game/scripts/chat/direct_peer_map.gd`, `src/game/scripts/chat/chat_service.gd` — **xong — `DirectPeerMap` (peer server → "Nhắn tin" → người gửi tin) + `ChatService.open_direct` (`200` = `201`; `403`/`404` → câu chung, ẩn nút tới hết phiên, không thử lại)**
- [x] T015 [US2] Tab "Bạn bè" (`chat_friends_list.gd`) + nút "Nhắn tin" trong `FriendProfilePanel` (F110) — file: `src/game/scenes/chat/chat_friends_list.gd`, `src/game/scenes/social/` — **xong — `scenes/chat/chat_friends_list.gd` (bạn bè + số chưa đọc DM đã biết + "Nhắn tin"), `FriendProfilePanel.ACT_MESSAGE` → `ui_requested("chat", {dm})`**

## Phase 5: User Story 3 — Trạng thái và giới hạn của channel bền vững (Priority: P1)

### Tests cho User Story 3

- [x] T016 [P] [US3] Scene F112-AC-11, F112-AC-12, F112-AC-13, F112-AC-14 × 4 khối: `CHAT_NOT_ALLOWED` gỡ tin + khóa tới lần làm mới; `CURFEW_ACTIVE` giờ từ bootstrap; `sendable_kinds` [quick, emoji] / rỗng; club `read_only`; rời club/`404` gỡ tab; World/Area ẩn chuyển tiếp/ghim/đã xem, không danh sách thành viên; cùng kết quả ở mọi khối (TC-CHAT-046, TC-CHAT-048, TC-CHAT-008, TC-CHAT-036) — file: `src/game/tests/scene/chat/test_persistent_channel_states.gd` — phải FAIL trước khi code — **xong — `tests/scene/chat/test_persistent_channel_states.gd` 7 test (× 4 khối cho `sendable_kinds`); đỏ trước khi code (2026-11-28: 12 file test F112 không nạp được — lỗi phân tích: thiếu `ChatHub`, `ChatHubService`, `ChatHubState`, `ChatSearchService`, `ChatSearchPanel`, `ClubManage`, `ClubManageService`, `ClubFormOptions`, `HighlightText`, `PersistentChannelTab`, `ChatHubButton`, `ClubPage.ACT_MANAGE`, `ChatDock.CONTEXT_PERSISTENT` — 95 lỗi script) → lần chạy đầu đỏ (fake liệt kê lại channel đã rời) → fixture trả danh sách không còn channel → xanh**

### Implementation cho User Story 3

- [x] T017 [US3] Khóa/mở ô soạn theo `status`, lỗi `CHAT_NOT_ALLOWED`/`CURFEW_ACTIVE`, `sendable_kinds`; xử lý `chat.channel.membership_changed` gỡ tab — file: `src/game/scenes/chat/chat_dock.gd`, `src/game/scripts/chat/chat_hub_state.gd`, `src/game/scripts/chat/chat_feedback_mapper.gd` — **xong — `ChatService.lock_of` (giới nghiêm, `frozen`, `read_only`, `CHAT_NOT_ALLOWED` tới lần `listMyChannels` kế, `sendable_kinds` rỗng), gỡ tab khi `membership_changed` `op = removed` hoặc lịch sử `404`; `ChatChannelModel.allows` = `allowed_kinds` ∩ `sendable_kinds`; khóa bền vững ẩn hàng tin nhanh + ô gõ (`ChatComposer.set_lock(…, hide_rows)`)**

## Phase 6: User Story 4 — Tìm lại tin trong club và DM (Priority: P2)

### Tests cho User Story 4

- [x] T018 [P] [US4] Unit F112-AC-15, F112-AC-16: `ChatSearchService` (giới hạn `q` theo contract, `cursor`, phạm vi), `HighlightText` bỏ thẻ/BBCode (`[b]`, `<em>`, `[url]`) và tô từ không dấu; không log `q` (TC-CHAT-040) — file: `src/game/tests/unit/chat/test_chat_search_service.gd`, `src/game/tests/unit/chat/test_highlight_text.gd` — phải FAIL trước khi code — **xong — `tests/unit/chat/test_chat_search_service.gd` 4 test + `test_highlight_text.gd` 5 test; đỏ trước khi code (2026-11-28: 12 file test F112 không nạp được — lỗi phân tích: thiếu `ChatHub`, `ChatHubService`, `ChatHubState`, `ChatSearchService`, `ChatSearchPanel`, `ClubManage`, `ClubManageService`, `ClubFormOptions`, `HighlightText`, `PersistentChannelTab`, `ChatHubButton`, `ClubPage.ACT_MANAGE`, `ChatDock.CONTEXT_PERSISTENT` — 95 lỗi script) → xanh**
- [x] T019 [P] [US4] Scene F112-AC-17: chạm kết quả → mở tab + lịch sử quanh `seq` + tô 2 s; `404` → "Tin này không còn xem được"; rỗng → trạng thái rỗng (TC-CHAT-040) — file: `src/game/tests/scene/chat/test_chat_search_panel.gd` — phải FAIL trước khi code — **xong — `tests/scene/chat/test_chat_search_panel.gd` 5 test; đỏ trước khi code (2026-11-28: 12 file test F112 không nạp được — lỗi phân tích: thiếu `ChatHub`, `ChatHubService`, `ChatHubState`, `ChatSearchService`, `ChatSearchPanel`, `ClubManage`, `ClubManageService`, `ClubFormOptions`, `HighlightText`, `PersistentChannelTab`, `ChatHubButton`, `ClubPage.ACT_MANAGE`, `ChatDock.CONTEXT_PERSISTENT` — 95 lỗi script) → xanh**

### Implementation cho User Story 4

- [x] T020 [US4] `ChatSearchService`, `HighlightText`, `ChatSearchPanel` (danh sách ảo hóa) — file: `src/game/scripts/chat/chat_search_service.gd`, `src/game/scripts/chat/highlight_text.gd`, `src/game/scenes/chat/chat_search_panel.gd` — **xong — `ChatSearchService` (q 2–100, `cursor`, phạm vi), `HighlightText` (quét ký tự — không RegEx: test ranh giới F119 bắt lỗi bản đầu dùng RegEx → sửa), `ChatSearchPanel` (kết quả văn bản thuần, từ khóa đậm + gạch chân, "Xem" → `ChatHub.jump_to`, `404` → "Tin này không còn xem được")**

## Phase 7: User Story 5 — Chủ club sửa thông tin và chọn phó nhóm (Priority: P2)

### Tests cho User Story 5

- [x] T021 [P] [US5] Unit F112-AC-18, F112-AC-19, F112-AC-20, F112-AC-21: `ClubManageService` chỉ gửi trường đổi + `If-Match` = `version`; `412` → tải lại; `451`/`422`/`409`/`403` → trạng thái form đúng; `updateClubMemberRole` officer/member; đọc đủ mọi trang thành viên (TC-SOC-004, TC-SOC-006) — file: `src/game/tests/unit/social/test_club_manage_service.gd` — phải FAIL trước khi code — **xong — `tests/unit/social/test_club_manage_service.gd` 9 test; đỏ trước khi code (2026-11-28: 12 file test F112 không nạp được — lỗi phân tích: thiếu `ChatHub`, `ChatHubService`, `ChatHubState`, `ChatSearchService`, `ChatSearchPanel`, `ClubManage`, `ClubManageService`, `ClubFormOptions`, `HighlightText`, `PersistentChannelTab`, `ChatHubButton`, `ClubPage.ACT_MANAGE`, `ChatDock.CONTEXT_PERSISTENT` — 95 lỗi script) → lần đầu đỏ 1 (giả định sai: ApiClient tự thử lại lỗi mạng với CÙNG khóa) → sửa test → xanh**
- [x] T022 [P] [US5] Scene F112-AC-18, F112-AC-21: nút "Quản lý club" chỉ owner + `active`; Phó nhóm không có thao tác trên dòng owner, không tên thật/trường/khối (TC-SOC-004) — file: `src/game/tests/scene/social/test_club_manage.gd` — phải FAIL trước khi code — **xong — `tests/scene/social/test_club_manage.gd` 6 test; đỏ trước khi code (2026-11-28: 12 file test F112 không nạp được — lỗi phân tích: thiếu `ChatHub`, `ChatHubService`, `ChatHubState`, `ChatSearchService`, `ChatSearchPanel`, `ClubManage`, `ClubManageService`, `ClubFormOptions`, `HighlightText`, `PersistentChannelTab`, `ChatHubButton`, `ClubPage.ACT_MANAGE`, `ChatDock.CONTEXT_PERSISTENT` — 95 lỗi script) → lần đầu đỏ 1 (chờ toast theo khung hình) → chờ theo thời gian → xanh**

### Implementation cho User Story 5

- [x] T023 [US5] `ClubManageService` — file: `src/game/scripts/social/club_manage_service.gd` — **xong — `scripts/social/club_manage_service.gd` (`load_club`, `save_club` + `If-Match: "version"`, `set_role`, `create_goal`/`create_event` giữ `Idempotency-Key`, `403`/`404`/`409`/`412`/`422`/`451`)**
- [x] T024 [US5] Màn `ClubManage` + form sửa + Phó nhóm; nút "Quản lý club" trên trang club F111 — file: `src/game/scenes/social/ClubManage.tscn`, `src/game/scenes/social/club_manage.gd`, `src/game/scenes/social/club_edit_form.gd`, `src/game/scenes/social/club_officers.gd` — **xong — `scenes/social/club_manage.gd` (4 mục, xác nhận một bước "Hủy" trung tính, toast 2 s, hàng nút dưới hộp cuộn) + nút "Quản lý club" (`ClubPage.ACT_MANAGE`) qua `FriendsClubs`**

## Phase 8: User Story 6 — Chủ club tạo mục tiêu tuần và sự kiện club (Priority: P3)

### Tests cho User Story 6

- [x] T025 [P] [US6] Unit + scene F112-AC-22, F112-AC-23, F112-AC-24: chỉ 3 `metric`, `target` trong giới hạn contract, `Idempotency-Key` giữ khi thử lại (một bản ghi); `event_kind` không có `internal_tournament`, giờ đã qua bị chặn theo đồng hồ server, ngày + giờ theo locale, không đếm ngược, toast 2 s (TC-SOC-007, TC-SOC-004, TC-EVT-009) — file: `src/game/tests/scene/social/test_club_goal_event_forms.gd`, `src/game/tests/unit/social/test_club_manage_service.gd` — phải FAIL trước khi code — **xong — `test_club_manage_service.gd` (metric, target, khóa, loại sự kiện, quá khứ) + `test_club_manage.gd` (form mục tiêu/sự kiện, ngày + giờ theo locale, toast 2 s); đỏ trước khi code (2026-11-28: 12 file test F112 không nạp được — lỗi phân tích: thiếu `ChatHub`, `ChatHubService`, `ChatHubState`, `ChatSearchService`, `ChatSearchPanel`, `ClubManage`, `ClubManageService`, `ClubFormOptions`, `HighlightText`, `PersistentChannelTab`, `ChatHubButton`, `ClubPage.ACT_MANAGE`, `ChatDock.CONTEXT_PERSISTENT` — 95 lỗi script) → xanh**

### Implementation cho User Story 6

- [x] T026 [US6] `ClubFormOptions` (dữ liệu metric/event_kind, ghi câu hỏi mở F112-3/F112-4) + form mục tiêu + form sự kiện — file: `src/game/scripts/social/club_form_options.gd`, `src/game/scenes/social/club_goal_form.gd`, `src/game/scenes/social/club_event_form.gd` — **xong — `scripts/social/club_form_options.gd` (3 chỉ số ∩ `accepted_metrics`, 4 loại sự kiện, giới hạn contract) + form trong `club_manage.gd`**

## Phase 9: User Story 7 — Song ngữ, giao diện theo khối và điện thoại, hiệu năng, ranh giới an toàn (Priority: P2)

### Tests cho User Story 7

- [x] T027 [P] [US7] F112-AC-25: `extract_pot` đủ khóa mới, pseudo-locale `xx-PS` cho mọi màn mới (TC-I18N-006) — file: `src/game/tests/scene/test_pseudo_locale.gd` — **xong — `test_f112_band_matrix.gd::test_every_f112_key_has_vi_and_en`, `test_english_and_pseudo_locale_fit` (`en` + `xx`), `extract_pot --check`**
- [x] T028 [P] [US7] F112-AC-26: `lint_theme_overrides.gd` không override trong `scenes/chat`, `scenes/social` mới; một nút ✕ Đóng mỗi cửa sổ, "Hủy" trung tính, trạng thái có chữ/biểu tượng (TC-UI-001, TC-UI-003) — file: `src/game/tests/scene/chat/test_chat_hub.gd`, `src/game/tools/lint_theme_overrides.gd` — **xong — `test_chat_hub.gd::test_window_close_and_tiers`, `test_club_manage.gd::test_one_red_close`; `lint_theme_overrides` xanh**
- [x] T029 [P] [US7] F112-AC-27, F112-AC-28: thêm màn mới vào ma trận khối (4 khối × 1280×720 / 1024×600 / 844×390 / 740×360 × `vi`/`en`/pseudo; `ChatSpaceProbe` "≥ 3 tin" mỗi tab; probe vật lý + kiểm ngược) (TC-UI-003) — file: `src/game/tests/scene/test_grade_band_matrix.gd`, `src/game/tests/fakes/chat_screens.gd` — **xong — `tests/scene/test_f112_band_matrix.gd` 5 test + `tests/fakes/f112_screens.gd`, `f112_matrix.gd` (12 màn × 4 khối × 4 cửa sổ, `ChatSpaceProbe`, probe vật lý, tấm phủ ≥ 90% chiều cao, `footer_cuts`); đỏ lần đầu ("≥ 3 tin": `hub_club 740x360: message list 151 px < 194`, …) → tab lên dòng tiêu đề trên điện thoại, tấm phủ cả màn, khung PC cao hơn, khóa ẩn hàng soạn → xanh**
- [x] T030 [P] [US7] F112-AC-29: perf 10 tin/giây × 60 s vào World, ảo hóa, gộp badge ≤ 1 lần/giây (TC-PERF-009) — file: `src/game/tests/perf/test_chat_hub_perf.gd` — **xong (phần CI) — `tests/perf/test_chat_hub_perf.gd` (600 tin vào World: ≤ `chat_recent_buffer`, ảo hóa, badge ≤ 1 lần/giây, không node 3D); FPS thiết bị lab ở T039**
- [x] T031 [P] [US7] F112-AC-30: quét mã — không NATS/Valkey, không `grade_level`/`grade_band` trong logic `scripts/chat`, `scenes/chat` (ngoài density/Theme), chuỗi người dùng/server không qua BBCode (TC-CHAT-010) — file: `src/game/tests/unit/chat/test_chat_boundaries.gd` — **xong — `tests/unit/chat/test_f112_boundaries.gd` (không NATS/Valkey, không `grade_band`, không BBCode/`append_text`, không đĩa; scene không gọi mạng) + `test_chat_boundaries.gd` của F119 phủ thư mục chat**
- [x] T032 [P] [US7] F112-AC-31: chỉ thẻ công khai; store F112 xóa khi đăng xuất/`session.session.replaced`; `403 GAME_ACCESS_STUDENT_ONLY`/`TENANT_MISMATCH` theo F061; `404` đóng màn (TC-AUTH-014) — file: `src/game/tests/unit/chat/test_f112_privacy_authz.gd` — **xong — `test_f112_boundaries.gd` (thẻ công khai, xóa khi đăng xuất, `GAME_ACCESS_STUDENT_ONLY` → F061, `404` đóng công cụ)**

### Implementation cho User Story 7

- [x] T033 [US7] Bố cục `compact` (tấm phủ, cuộn form, vùng chạm) và sửa theo kết quả ma trận khối tới khi xanh — file: `src/game/scenes/chat/chat_hub.gd`, `src/game/scenes/social/club_manage.gd` — **xong — xem T029**
- [x] T034 [US7] Telemetry không PII (số lỗi theo mã, độ trễ mở DM) — file: `src/game/scripts/chat/chat_service.gd`, `src/game/scripts/social/club_manage_service.gd` — **xong — `client_error` theo mã (`chat_dm`, `club_manage`) không PII; độ trễ mở DM **không** gửi (danh mục `TelemetryBatch` chưa có loại phù hợp — ghi báo cáo, không tự thêm)**

## Phase 10: Polish & Cross-Cutting

- [x] T035 [P] Cập nhật tài liệu sở hữu: STUDENT_GAME_UX_SOCIAL §6–§7 (lát cắt client F112: khung chat HUD, DM, tìm kiếm, Quản lý club), SOCIAL_SYSTEM §6.2 nếu câu hỏi mở F112-1/F112-2 được chốt; CHANGELOG — file: `docs/05-ux/STUDENT_GAME_UX_SOCIAL.md`, `CHANGELOG.md` — **xong — STUDENT_GAME_UX_SOCIAL §7, GODOT_ARCHITECTURE §3/§6.1, CHANGELOG**
- [x] T036 [P] Ghi câu hỏi mở F112-1…F112-9 vào backlog đối soát cho feature backend tương ứng — file: `docs/12-project/SPEC_RECONCILIATION_BACKLOG.md` — **xong — câu hỏi mở F112-1…F112-9 đã được backend đóng 2026-11-28 (contracts §6); không còn mục nào cho backlog**
- [x] T037 Chạy `make godot-test`, `make godot-export-check` (và `make design-tokens-check` nếu có token) — file: `src/game/` — **xong — GdUnit4 toàn bộ + lint Godot + export PCK Android + scan_artifact (số liệu ở quickstart §5)**
- [ ] T038 Chạy [quickstart.md](quickstart.md) §2 với backend thật (hai client) và ghi kết quả — file: `specs/112-godot-persistent-chat-club-tools/quickstart.md` — **chưa — cần backend compose + hai client thật (Pending)**
- [ ] T039 Đo FPS thiết bị lab Android tầm trung (SC-005) — file: `specs/112-godot-persistent-chat-club-tools/quickstart.md` — **chưa — đo trên thiết bị lab (Pending)**
- [x] T040 Chụp ảnh trước/sau theo F112-AC-28 (`tools/capture_screens.gd`, fixture + fake) — file: `specs/112-godot-persistent-chat-club-tools/screenshots/` — **xong — `screenshots/` (xem quickstart §5)**
- [x] T041 Xem từng ảnh, ghi bảng màn × khối và lỗi đã sửa vào quickstart §5 — file: `specs/112-godot-persistent-chat-club-tools/quickstart.md` — **xong — quickstart §5**
- [x] T042 Cập nhật dòng F112 trong CODE_PLAN (trạng thái, bằng chứng) — file: `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md` — **xong — CODE_PLAN F112: In progress (còn T038, T039)**
- [x] T043 Ghi bằng chứng (lệnh, số pass/fail) vào tasks.md và Clarifications triển khai nếu có quyết định mới — file: `specs/112-godot-persistent-chat-club-tools/tasks.md` — **xong — bảng dưới + spec Clarifications "triển khai client"**

## Dependencies & Execution Order

- F110 → F111 → F112 (T004 chặn mọi story); F119, F120, F121 đã có.
- Setup (T001–T003) → Foundational (T004–T007) → US1 (MVP) → US2, US3 (P1, song song được sau US1 vì cùng dùng `ChatHub`) → US4, US5 (P2, độc lập nhau) → US6 (cần màn `ClubManage` của US5) → US7 (ma trận, perf, quét mã trên mọi màn) → Polish.
- Trong mỗi story: test (FAIL) → code → test pass. T029 chạy lại sau mỗi story có màn mới.

## Definition of Done

- [x] Mọi tiêu chí F112-AC-01…F112-AC-31 có test pass (bảng AC → test ở plan.md) và ghi mã test/TC
- [x] `make godot-test` (GdUnit4 toàn bộ, ma trận khối, lint theme, `check_typed`) và `make godot-export-check` xanh; `make design-tokens-check` nếu đổi token
- [x] Ảnh ma trận khối × cỡ điện thoại đã chụp và xem, bảng ở quickstart §5 (F112-AC-28)
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17); kịch bản quickstart §2 chạy với backend thật
- [x] Tài liệu sở hữu (STUDENT_GAME_UX_SOCIAL), backlog câu hỏi mở, CHANGELOG cập nhật
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

## Bằng chứng: F112-AC → test (2026-11-28, GdUnit4 headless, fake + fixture contract)

Đỏ trước khi code: 12 file test F112 chạy trước khi có mã → không nạp được (95 lỗi phân tích: thiếu `ChatHub`, `ChatHubService`, `ChatHubState`, `ChatSearchService`, `ChatSearchPanel`, `ClubManage`, `ClubManageService`, `ClubFormOptions`, `HighlightText`, `PersistentChannelTab`, `ChatHubButton`, `ClubPage.ACT_MANAGE`, `ChatDock.CONTEXT_PERSISTENT`). Lần chạy đầu sau khi có mã: unit 35 test / 2 lỗi, scene 33 / 10 lỗi, ma trận "≥ 3 tin" đỏ ở mọi màn hub điện thoại; test F119 bắt hai hồi quy (RegEx trong `HighlightText`; `sendable_kinds` thay vì giao với `allowed_kinds`) → sửa → xanh.

| AC | Test |
|---|---|
| F112-AC-01, AC-03 | `tests/unit/chat/test_chat_hub_state.gd`, `tests/scene/chat/test_chat_hub.gd::test_tabs_of_the_listed_channels` |
| F112-AC-02 | `test_chat_hub.gd::test_persistent_tab_reuses_dock_and_reports_with_channel_id` |
| F112-AC-04 | `test_chat_hub_state.gd::test_world_and_area_never_count_unread_only_mentions`, `test_chat_hub.gd::test_only_the_tab_on_screen_is_joined` |
| F112-AC-05 | `test_chat_hub_state.gd::test_hud_badge_is_the_sum_of_club_and_dm_unread_and_mentions`, `test_chat_hub.gd::test_hud_badge_total_99_plus_and_static_mention_badge` |
| F112-AC-06 | `test_chat_hub.gd::test_hub_hidden_during_exam` |
| F112-AC-07, AC-08, AC-10 | `tests/unit/chat/test_direct_channels.gd`, `test_chat_hub_state.gd::test_unknown_dm_only_adds_to_the_friends_badge` |
| F112-AC-09 | `tests/scene/chat/test_persistent_channel_states.gd::test_frozen_dm_locks_with_a_neutral_sentence_and_unlocks` |
| F112-AC-11 | `test_persistent_channel_states.gd::test_not_allowed_drops_the_message_and_locks_until_refresh`, `::test_curfew_locks_every_channel_with_the_end_time` |
| F112-AC-12 | `test_persistent_channel_states.gd::test_sendable_kinds_quick_only_and_none_same_in_every_band` |
| F112-AC-13 | `test_persistent_channel_states.gd::test_read_only_club_and_removed_club`, `::test_history_404_removes_the_tab` |
| F112-AC-14 | `test_persistent_channel_states.gd::test_world_has_no_forward_pin_seen_or_members` |
| F112-AC-15, AC-16 | `tests/unit/chat/test_chat_search_service.gd`, `tests/unit/chat/test_highlight_text.gd` |
| F112-AC-17 | `tests/scene/chat/test_chat_search_panel.gd` |
| F112-AC-18…AC-21 | `tests/unit/social/test_club_manage_service.gd`, `tests/scene/social/test_club_manage.gd` |
| F112-AC-22…AC-24 | `test_club_manage_service.gd::test_goal_metrics_target_bounds_and_one_key_when_retried`, `::test_event_kinds_and_no_time_in_the_past`, `test_club_manage.gd::test_goal_form_metrics_target_and_toast`, `::test_event_form_kinds_past_time_and_day_hour` |
| F112-AC-25 | `tests/scene/test_f112_band_matrix.gd::test_every_f112_key_has_vi_and_en`, `::test_english_and_pseudo_locale_fit`, `extract_pot --check` |
| F112-AC-26 | `test_chat_hub.gd::test_window_close_and_tiers`, `test_club_manage.gd::test_one_red_close`, `lint_theme_overrides` |
| F112-AC-27, AC-28 | `tests/scene/test_f112_band_matrix.gd` (12 màn × 4 khối × 4 cửa sổ + `en`/pseudo, `ChatSpaceProbe`, probe vật lý, tấm phủ, `footer_cuts`), ảnh quickstart §5 |
| F112-AC-29 | `tests/perf/test_chat_hub_perf.gd` (CI guard); FPS Android tầm trung — **chưa đo (T039)** |
| F112-AC-30 | `tests/unit/chat/test_f112_boundaries.gd::test_no_broker_no_grade_branch_no_bbcode_no_disk`, `::test_scenes_never_call_the_network`, `tests/unit/chat/test_chat_boundaries.gd` (F119) |
| F112-AC-31 | `test_f112_boundaries.gd::test_screens_show_only_public_cards`, `::test_ram_only_and_cleared_on_logout`, `::test_student_only_refusal_goes_to_f061_and_404_closes_tools` |

Còn mở trước `Done`: T038 (kịch bản §2 với backend compose + hai client), T039 (FPS SC-005 trên thiết bị lab).
