# Tasks: F112 — Godot — channel chat bền vững (khu vực, DM) và công cụ chủ club (sửa club, phó nhóm, mục tiêu/sự kiện club)

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

Quy ước: test viết trước và phải FAIL trước khi code của story đó; fake/fixture gắn nhãn TEST DOUBLE (R17); mọi đường dẫn dưới `src/game/`.

## Phase 1: Setup

- [ ] T001 [P] Khóa dịch `ui.chat.hub.*`, `ui.chat.dm.*`, `ui.chat.search.*`, `ui.club.manage.*` và `ui.error.<CODE>` còn thiếu (vi/en), sinh lại `messages.pot`, `xx_PS.po` — file: `src/game/locale/vi.po`, `src/game/locale/en.po`
- [ ] T002 [P] Fixture contract (`openDirectChannel` 200/201/403, `searchMessages`, `updateClub` 200/412/451, `updateClubMemberRole`, `createClubGoal`, `createClubEvent`, `chat.channel.membership_changed` frozen/read_only/rời club, `system.command.failed` `CHAT_NOT_ALLOWED`/`CURFEW_ACTIVE`); thiếu example thì thêm chỉ ví dụ vào bundle OpenAPI/AsyncAPI rồi tách module (contracts §6) — file: `src/game/tools/gen_fixtures.mjs`, `src/game/tests/fixtures/contract/`, `specs/openapi/eduworld-api.v1.yaml`
- [ ] T003 [P] Typed model `PersistentChannelTab`, `DirectPeer`, `SearchHit` — file: `src/game/scripts/chat/models/`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Kiểm tiền đề: F110 (`SocialService.list_friends`, `FriendProfilePanel`) và F111 (`ClubService`, trang club) đã có; nếu chưa, dừng và làm F110/F111 trước (CODE_PLAN) — file: `src/game/scripts/social/`, `src/game/scenes/social/`
- [ ] T005 `ChatDock.show_channel(channel_id)` cho channel bền vững (dùng lại danh sách, ô soạn, menu F119; không đổi `show_party`/`show_match`) — file: `src/game/scenes/chat/chat_dock.gd`
- [ ] T006 `ChatHubState` khung (tab từ `listMyChannels` + club + bạn, join/leave theo tab) và đăng ký `ServiceLocator` — file: `src/game/scripts/chat/chat_hub_state.gd`, `src/game/scripts/core/service_locator.gd`
- [ ] T007 Slot HUD: `MainHud` bỏ `ChatDock` khỏi `FUTURE_SLOTS`, gắn nút chat + `ChatHub` (sửa comment ghi F110) — file: `src/game/scenes/ui/hud/main_hud.gd`, `src/game/scenes/ui/hud/MainHud.tscn`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Khung chat HUD với các channel bền vững (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [ ] T008 [P] [US1] Unit F112-AC-01, F112-AC-03, F112-AC-04: tab đúng loại server trả, bỏ loại lạ, không tab lớp/khối; `hub.instance.assigned` → leave area cũ + xóa RAM + area mới; join/leave theo tab; World/Area không unread (TC-CHAT-002, TC-CHAT-008, TC-CHAT-039) — file: `src/game/tests/unit/chat/test_chat_hub_state.gd` — phải FAIL trước khi code
- [ ] T009 [P] [US1] Scene F112-AC-02, F112-AC-05, F112-AC-06: tab dùng `ChatDock.show_channel`, báo cáo có `context.channel_id`; badge HUD "99+" + `@` tĩnh; ẩn khi `blocking_activity` bài thi, chat gameplay không đổi (TC-CHAT-006) — file: `src/game/tests/scene/chat/test_chat_hub.gd` — phải FAIL trước khi code

### Implementation cho User Story 1

- [ ] T010 [US1] `ChatHub` (khung nổi PC/tablet, tấm phủ điện thoại, ✕ Đóng, kính lúp) + `chat_hub_tabs.gd` (ChipScroller, badge) — file: `src/game/scenes/chat/ChatHub.tscn`, `src/game/scenes/chat/chat_hub.gd`, `src/game/scenes/chat/chat_hub_tabs.gd`
- [ ] T011 [US1] `ChatService`: làm mới `listMyChannels` khi `hub.instance.assigned`, leave/join theo `ChatHubState`, badge từ `chat.unread.updated`/`chat.mention.created`; rời mọi channel bền vững khi `blocking_activity` — file: `src/game/scripts/chat/chat_service.gd`

**Checkpoint**: US1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Nhắn tin riêng với bạn bè (Priority: P1)

### Tests cho User Story 2

- [ ] T012 [P] [US2] Unit F112-AC-07, F112-AC-08, F112-AC-10: `open_direct` gọi `openDirectChannel` (200 = 201), ánh xạ RAM, nút theo `friends_enabled`/`direct_message_enabled`, `403`/`404` → câu chung + ẩn nút tới hết phiên, không gọi hàng loạt, DM chưa biết người kia chỉ cộng badge (TC-CHAT-005, TC-CHAT-047, TC-CHAT-039) — file: `src/game/tests/unit/chat/test_direct_channels.gd` — phải FAIL trước khi code
- [ ] T013 [P] [US2] Scene F112-AC-09: `frozen` khóa ô soạn với câu trung tính, đọc lịch sử, `open` mở khóa; không chuỗi nào nêu "chặn" (TC-CHAT-005) — file: `src/game/tests/scene/chat/test_chat_dm.gd` — phải FAIL trước khi code

### Implementation cho User Story 2

- [ ] T014 [US2] `DirectPeerMap` + `ChatService.open_direct(user_id)` — file: `src/game/scripts/chat/direct_peer_map.gd`, `src/game/scripts/chat/chat_service.gd`
- [ ] T015 [US2] Tab "Bạn bè" (`chat_friends_list.gd`) + nút "Nhắn tin" trong `FriendProfilePanel` (F110) — file: `src/game/scenes/chat/chat_friends_list.gd`, `src/game/scenes/social/`

## Phase 5: User Story 3 — Trạng thái và giới hạn của channel bền vững (Priority: P1)

### Tests cho User Story 3

- [ ] T016 [P] [US3] Scene F112-AC-11, F112-AC-12, F112-AC-13, F112-AC-14 × 4 khối: `CHAT_NOT_ALLOWED` gỡ tin + khóa tới lần làm mới; `CURFEW_ACTIVE` giờ từ bootstrap; `sendable_kinds` [quick, emoji] / rỗng; club `read_only`; rời club/`404` gỡ tab; World/Area ẩn chuyển tiếp/ghim/đã xem, không danh sách thành viên; cùng kết quả ở mọi khối (TC-CHAT-046, TC-CHAT-048, TC-CHAT-008, TC-CHAT-036) — file: `src/game/tests/scene/chat/test_persistent_channel_states.gd` — phải FAIL trước khi code

### Implementation cho User Story 3

- [ ] T017 [US3] Khóa/mở ô soạn theo `status`, lỗi `CHAT_NOT_ALLOWED`/`CURFEW_ACTIVE`, `sendable_kinds`; xử lý `chat.channel.membership_changed` gỡ tab — file: `src/game/scenes/chat/chat_dock.gd`, `src/game/scripts/chat/chat_hub_state.gd`, `src/game/scripts/chat/chat_feedback_mapper.gd`

## Phase 6: User Story 4 — Tìm lại tin trong club và DM (Priority: P2)

### Tests cho User Story 4

- [ ] T018 [P] [US4] Unit F112-AC-15, F112-AC-16: `ChatSearchService` (giới hạn `q` theo contract, `cursor`, phạm vi), `HighlightText` bỏ thẻ/BBCode (`[b]`, `<em>`, `[url]`) và tô từ không dấu; không log `q` (TC-CHAT-040) — file: `src/game/tests/unit/chat/test_chat_search_service.gd`, `src/game/tests/unit/chat/test_highlight_text.gd` — phải FAIL trước khi code
- [ ] T019 [P] [US4] Scene F112-AC-17: chạm kết quả → mở tab + lịch sử quanh `seq` + tô 2 s; `404` → "Tin này không còn xem được"; rỗng → trạng thái rỗng (TC-CHAT-040) — file: `src/game/tests/scene/chat/test_chat_search_panel.gd` — phải FAIL trước khi code

### Implementation cho User Story 4

- [ ] T020 [US4] `ChatSearchService`, `HighlightText`, `ChatSearchPanel` (danh sách ảo hóa) — file: `src/game/scripts/chat/chat_search_service.gd`, `src/game/scripts/chat/highlight_text.gd`, `src/game/scenes/chat/chat_search_panel.gd`

## Phase 7: User Story 5 — Chủ club sửa thông tin và chọn phó nhóm (Priority: P2)

### Tests cho User Story 5

- [ ] T021 [P] [US5] Unit F112-AC-18, F112-AC-19, F112-AC-20, F112-AC-21: `ClubManageService` chỉ gửi trường đổi + `If-Match` = `version`; `412` → tải lại; `451`/`422`/`409`/`403` → trạng thái form đúng; `updateClubMemberRole` officer/member; đọc đủ mọi trang thành viên (TC-SOC-004, TC-SOC-006) — file: `src/game/tests/unit/social/test_club_manage_service.gd` — phải FAIL trước khi code
- [ ] T022 [P] [US5] Scene F112-AC-18, F112-AC-21: nút "Quản lý club" chỉ owner + `active`; Phó nhóm không có thao tác trên dòng owner, không tên thật/trường/khối (TC-SOC-004) — file: `src/game/tests/scene/social/test_club_manage.gd` — phải FAIL trước khi code

### Implementation cho User Story 5

- [ ] T023 [US5] `ClubManageService` — file: `src/game/scripts/social/club_manage_service.gd`
- [ ] T024 [US5] Màn `ClubManage` + form sửa + Phó nhóm; nút "Quản lý club" trên trang club F111 — file: `src/game/scenes/social/ClubManage.tscn`, `src/game/scenes/social/club_manage.gd`, `src/game/scenes/social/club_edit_form.gd`, `src/game/scenes/social/club_officers.gd`

## Phase 8: User Story 6 — Chủ club tạo mục tiêu tuần và sự kiện club (Priority: P3)

### Tests cho User Story 6

- [ ] T025 [P] [US6] Unit + scene F112-AC-22, F112-AC-23, F112-AC-24: chỉ 3 `metric`, `target` trong giới hạn contract, `Idempotency-Key` giữ khi thử lại (một bản ghi); `event_kind` không có `internal_tournament`, giờ đã qua bị chặn theo đồng hồ server, ngày + giờ theo locale, không đếm ngược, toast 2 s (TC-SOC-007, TC-SOC-004, TC-EVT-009) — file: `src/game/tests/scene/social/test_club_goal_event_forms.gd`, `src/game/tests/unit/social/test_club_manage_service.gd` — phải FAIL trước khi code

### Implementation cho User Story 6

- [ ] T026 [US6] `ClubFormOptions` (dữ liệu metric/event_kind, ghi câu hỏi mở F112-3/F112-4) + form mục tiêu + form sự kiện — file: `src/game/scripts/social/club_form_options.gd`, `src/game/scenes/social/club_goal_form.gd`, `src/game/scenes/social/club_event_form.gd`

## Phase 9: User Story 7 — Song ngữ, giao diện theo khối và điện thoại, hiệu năng, ranh giới an toàn (Priority: P2)

### Tests cho User Story 7

- [ ] T027 [P] [US7] F112-AC-25: `extract_pot` đủ khóa mới, pseudo-locale `xx-PS` cho mọi màn mới (TC-I18N-006) — file: `src/game/tests/scene/test_pseudo_locale.gd`
- [ ] T028 [P] [US7] F112-AC-26: `lint_theme_overrides.gd` không override trong `scenes/chat`, `scenes/social` mới; một nút ✕ Đóng mỗi cửa sổ, "Hủy" trung tính, trạng thái có chữ/biểu tượng (TC-UI-001, TC-UI-003) — file: `src/game/tests/scene/chat/test_chat_hub.gd`, `src/game/tools/lint_theme_overrides.gd`
- [ ] T029 [P] [US7] F112-AC-27, F112-AC-28: thêm màn mới vào ma trận khối (4 khối × 1280×720 / 1024×600 / 844×390 / 740×360 × `vi`/`en`/pseudo; `ChatSpaceProbe` "≥ 3 tin" mỗi tab; probe vật lý + kiểm ngược) (TC-UI-003) — file: `src/game/tests/scene/test_grade_band_matrix.gd`, `src/game/tests/fakes/chat_screens.gd`
- [ ] T030 [P] [US7] F112-AC-29: perf 10 tin/giây × 60 s vào World, ảo hóa, gộp badge ≤ 1 lần/giây (TC-PERF-009) — file: `src/game/tests/perf/test_chat_hub_perf.gd`
- [ ] T031 [P] [US7] F112-AC-30: quét mã — không NATS/Valkey, không `grade_level`/`grade_band` trong logic `scripts/chat`, `scenes/chat` (ngoài density/Theme), chuỗi người dùng/server không qua BBCode (TC-CHAT-010) — file: `src/game/tests/unit/chat/test_chat_boundaries.gd`
- [ ] T032 [P] [US7] F112-AC-31: chỉ thẻ công khai; store F112 xóa khi đăng xuất/`session.session.replaced`; `403 GAME_ACCESS_STUDENT_ONLY`/`TENANT_MISMATCH` theo F061; `404` đóng màn (TC-AUTH-014) — file: `src/game/tests/unit/chat/test_f112_privacy_authz.gd`

### Implementation cho User Story 7

- [ ] T033 [US7] Bố cục `compact` (tấm phủ, cuộn form, vùng chạm) và sửa theo kết quả ma trận khối tới khi xanh — file: `src/game/scenes/chat/chat_hub.gd`, `src/game/scenes/social/club_manage.gd`
- [ ] T034 [US7] Telemetry không PII (số lỗi theo mã, độ trễ mở DM) — file: `src/game/scripts/chat/chat_service.gd`, `src/game/scripts/social/club_manage_service.gd`

## Phase 10: Polish & Cross-Cutting

- [ ] T035 [P] Cập nhật tài liệu sở hữu: STUDENT_GAME_UX_SOCIAL §6–§7 (lát cắt client F112: khung chat HUD, DM, tìm kiếm, Quản lý club), SOCIAL_SYSTEM §6.2 nếu câu hỏi mở F112-1/F112-2 được chốt; CHANGELOG — file: `docs/05-ux/STUDENT_GAME_UX_SOCIAL.md`, `CHANGELOG.md`
- [ ] T036 [P] Ghi câu hỏi mở F112-1…F112-9 vào backlog đối soát cho feature backend tương ứng — file: `docs/12-project/SPEC_RECONCILIATION_BACKLOG.md`
- [ ] T037 Chạy `make godot-test`, `make godot-export-check` (và `make design-tokens-check` nếu có token) — file: `src/game/`
- [ ] T038 Chạy [quickstart.md](quickstart.md) §2 với backend thật (hai client) và ghi kết quả — file: `specs/112-godot-persistent-chat-club-tools/quickstart.md`
- [ ] T039 Đo FPS thiết bị lab Android tầm trung (SC-005) — file: `specs/112-godot-persistent-chat-club-tools/quickstart.md`
- [ ] T040 Chụp ảnh trước/sau theo F112-AC-28 (`tools/capture_screens.gd`, fixture + fake) — file: `specs/112-godot-persistent-chat-club-tools/screenshots/`
- [ ] T041 Xem từng ảnh, ghi bảng màn × khối và lỗi đã sửa vào quickstart §5 — file: `specs/112-godot-persistent-chat-club-tools/quickstart.md`
- [ ] T042 Cập nhật dòng F112 trong CODE_PLAN (trạng thái, bằng chứng) — file: `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md`
- [ ] T043 Ghi bằng chứng (lệnh, số pass/fail) vào tasks.md và Clarifications triển khai nếu có quyết định mới — file: `specs/112-godot-persistent-chat-club-tools/tasks.md`

## Dependencies & Execution Order

- F110 → F111 → F112 (T004 chặn mọi story); F119, F120, F121 đã có.
- Setup (T001–T003) → Foundational (T004–T007) → US1 (MVP) → US2, US3 (P1, song song được sau US1 vì cùng dùng `ChatHub`) → US4, US5 (P2, độc lập nhau) → US6 (cần màn `ClubManage` của US5) → US7 (ma trận, perf, quét mã trên mọi màn) → Polish.
- Trong mỗi story: test (FAIL) → code → test pass. T029 chạy lại sau mỗi story có màn mới.

## Definition of Done

- [ ] Mọi tiêu chí F112-AC-01…F112-AC-31 có test pass (bảng AC → test ở plan.md) và ghi mã test/TC
- [ ] `make godot-test` (GdUnit4 toàn bộ, ma trận khối, lint theme, `check_typed`) và `make godot-export-check` xanh; `make design-tokens-check` nếu đổi token
- [ ] Ảnh ma trận khối × cỡ điện thoại đã chụp và xem, bảng ở quickstart §5 (F112-AC-28)
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17); kịch bản quickstart §2 chạy với backend thật
- [ ] Tài liệu sở hữu (STUDENT_GAME_UX_SOCIAL), backlog câu hỏi mở, CHANGELOG cập nhật
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
