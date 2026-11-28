# Tasks: F111 — Godot — club trong game, Event center, nhóm "Vừa chơi cùng", nút "Tham gia bạn"

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 [P] Khóa dịch `ui.events.*` (title, reset_time, time_left_days, last_day, upcoming, suspended, ended_auto_grant, all_done, join, claim, community_goal, contributed), `ui.season.*` (tier, points, daily_remaining, auto_grant, no_season, history), `ui.club.*` (mine, discover, create, pending_approval, request_pending, invitations, requests, invite, leave_confirm, chat, read_only, not_enabled), `ui.friends.join_friend`, `ui.friends.join_unavailable.*`, `ui.friends.joined`, `ui.friends.suggest.*`, `ui.error.<CODE>` cho mã của feature — file: `src/game/locale/vi.po`, `src/game/locale/en.po` — **xong 2026-11-28 — 175 khóa vi+en mới (ui.events.*, ui.season.*, ui.club.*, ui.friends.join_*, ui.friends.suggest.*, ui.error.<CODE>, ui.notification.club.*, ui.chat.title.club, ui.menu.events, ui.hub.board.events); messages.pot + xx_PS.po sinh lại; `extract_pot --check`, `pseudo_locale --check` OK**
- [x] T002 [P] Fixture từ example OpenAPI/AsyncAPI (sự kiện mọi trạng thái, chi tiết có community goal/mốc, season track, club mọi trạng thái/vai trò, lời mời, yêu cầu, gợi ý kết bạn, `hub.instance.assigned`, `system.command.failed` `ROOM_FULL`/`PERMISSION_DENIED`, Problem) — nhãn MOCK — file: `src/game/tests/fixtures/{club,liveops,social_suggestions}/` — **xong 2026-11-28 — commit riêng 2864cb5f: `gen_fixtures.mjs` thêm 18 schema F111, 3 response example (`listFriendSuggestions`, `listMyClubInvitations`, `listClubGoals`) và mẫu tất định `event.*`, `season.track.updated` (ajv); fixture điền giá trị mẫu trong `tests/fakes/f111_fixtures.gd` (TEST-ONLY, nhãn R17) thay cho thư mục `fixtures/{club,liveops,social_suggestions}` — một nguồn với `tests/fixtures/contract`; `gen_fixtures --check` OK (273 file)**
- [x] T003 [P] Interactable bảng tin Quảng trường và nút lịch HUD (chưa nối service) — file: `src/game/scenes/world/interactables/EventBoard.tscn`, `src/game/scenes/ui/hud/EventCenterButton.tscn` — **xong 2026-11-28 — `EventCenterButton` (cột phải HUD, dưới "Thêm") + bảng tin `EventBoard` (`kind = event_board`, `board.events`) trong `main_hub/Zone.tscn` + ô "Sự kiện" menu "Thêm"; test `test_event_center_entry` (đỏ: Parse Error EventCenterButton → xanh 3/3)**

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Theo dõi delta F095: `listFriendSuggestions` + tham số `friend_suggestion_recent_window` — owner F095 — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=chat` — **xong 2026-11-28 (lượt server F110/F111: delta trên main, xem contracts §6 "Trạng thái trên main")**
- [x] T005 Theo dõi delta F120 (`listMyClubInvitations`, `ClubInvitation.club_name`/`inviter_nickname`, `451` cho `createClub`), WP-C1 (`ClubType` bỏ `study_group` — ADR-0095) và F123 (`kind` thông báo club, `action` deep link) — owner F120, F123 — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking` — **xong 2026-11-28 (lượt server F110/F111: delta trên main, xem contracts §6 "Trạng thái trên main")**
- [x] T006 Theo dõi delta F013/F015 (`PlayerBootstrap.policies.social`), F091/F071 (`HubInstanceJoin.follow_user_id`, bỏ/đổi `joinFriendInWorld`), F077 (`event.community_goal.updated`, `428` cho `joinGameEvent`) — owner tương ứng — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=identity` — **xong 2026-11-28 (lượt server F110/F111: delta trên main, xem contracts §6 "Trạng thái trên main")**
- [x] T007 Sinh lại fixture sau khi T004–T006 merge + test contract parse mọi example — file: `src/game/tests/contract/test_f111_fixtures.gd` — kiểm: `make godot-test` — **xong 2026-11-28 — `tests/contract/test_f111_fixtures.gd` (5 test); đỏ trước: Parse Error model chưa có + `test_realtime_fixtures` 4 lỗi (event.*/season.* chưa có trong WsMessageSchemas) → xanh**
- [x] T008 Model typed theo data-model (`from_dict` bỏ trường lạ, lọc `study_group`) + test parse — file: `src/game/scripts/social/models/`, `src/game/scripts/liveops/models/`, `src/game/tests/unit/models/test_f111_models.gd` — **xong 2026-11-28 — 15 model (`scripts/social/models/`, `scripts/liveops/models/`), `LiveOpsTime`; `tests/unit/models/test_f111_models.gd` 6 test; đỏ (Parse Error FriendSuggestionModel…) → xanh**
- [x] T009 `AppState` thêm `social_policy`, `live_events`, `clubs` (đọc `policies.social` trong `apply_bootstrap`); đăng ký `JoinFriendService`, `ClubService`, `LiveEventService`, `SeasonTrackService` trong `ServiceLocator`; dispatcher `event.*`, `season.track.updated`; handler `EventBus.ui_requested` cho `event_center`, `club`, `friends` — file: `src/game/scripts/core/app_state.gd`, `src/game/scripts/core/service_locator.gd`, `src/game/scripts/core/event_bus.gd` — **xong 2026-11-28 — `AppState.social_policy`, `ServiceLocator._init_f111` (JoinFriendService, ClubService, LiveEventService, SeasonTrackService; xóa ở end_session), dispatcher `event`/`season`, `WsMessageSchemas`, deep link `open_club`/`open_event`, `UiScreenRegistry.event_center`, `ApiClient` query mảng; handler `ui_requested` dùng cơ chế sẵn có của WorldRoot (club = tab của `friends`); `tests/unit/test_f111_wiring.gd` 4 test, đỏ → xanh**
- [x] T010 `HubSync.join(area_id, spawn_point, follow_user_id)` và `SceneRouter.request_area(area_id, args)` chuyển `follow_user_id` (không đổi hành vi cũ) + test hồi quy F092/F071 — file: `src/game/scripts/multiplayer/hub_sync.gd`, `src/game/scripts/core/scene_router.gd`, `src/game/tests/unit/multiplayer/test_hub_sync_follow.gd` — **xong 2026-11-28 — `HubSync.enter_area(…, follow_user_id)`, `HubSync.follow`, `follow_failed`; `SceneRouter.request_area(area_id, portal_id, world, args)` + `follow_of`, `WorldRoot._start_hub(…, follow)`; `test_hub_sync_follow.gd` 4 test + hồi quy `test_hub_move_throttle`, `test_scene_router_area` xanh (37/37)**

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — "Tham gia bạn" trong thế giới (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [x] T011 [P] [US1] Unit test F111-AC-01, F111-AC-03: cùng khu → `hub.instance.join{follow_user_id}`; khác khu → `request_area` rồi join; không bao giờ gọi `joinFriendInWorld`; `hub.instance.assigned` → toast — file: `src/game/tests/unit/multiplayer/test_join_friend_service.gd` — phải FAIL trước khi code — **xong — 3 test; đỏ (gt us1red: Parse Error JoinAvailability) → xanh**
- [x] T012 [P] [US1] Unit test F111-AC-02: `JoinAvailability.evaluate` cho `offline` (gồm ẩn trạng thái)/`in_match`/thiếu `area_id`/`exam`/bản thân đang boss/mất mạng → lý do đúng — file: `src/game/tests/unit/social/test_join_availability.gd` — **xong — 4 test (gồm T015 profile); đỏ → xanh**
- [x] T013 [P] [US1] Unit test F111-AC-04, F111-AC-05: `ROOM_FULL` → join lại không follow + thông điệp; `PERMISSION_DENIED`/`NOT_FOUND` → thông điệp chung, không chứa `room_id`/khu; `AREA_LOCKED` → ở lại, không gửi join; nút khóa khi chờ — file: `src/game/tests/unit/multiplayer/test_join_friend_failures.gd` — **xong — 4 test; đỏ → xanh (us1 11/11)**

### Implementation cho User Story 1

- [x] T014 [US1] `JoinFriendService`, `JoinAvailability`; `FriendsStore` (F110) lưu `area_id`, `activity` — file: `src/game/scripts/multiplayer/join_friend_service.gd`, `src/game/scripts/social/join_availability.gd`, `src/game/scripts/social/friends_store.gd` — **xong — `JoinFriendService` (state idle/entering_area/joining, notice_requested, khóa một lần), `JoinAvailability` (8 lý do có chữ); `FriendsStore` đã có `area_id`/`activity` từ F110**
- [x] T015 [US1] `JoinFriendButton` gắn vào `FriendProfilePanel`, `FriendRow` (mobile) — file: `src/game/scenes/social/JoinFriendButton.tscn`, `src/game/scenes/social/FriendProfilePanel.tscn` — **xong — nút "Tham gia bạn" + dòng lý do trong `FriendProfilePanel` (hồ sơ luôn hiện cạnh danh sách kể cả điện thoại — không thêm vào `FriendRow`, [Confirmed] Clarifications 2026-11-28); câu kết quả qua `show_hub_notice(key, args)` của HUD**

## Phase 4: User Story 2 — Nhóm "Vừa chơi cùng" (Priority: P2)

### Tests cho User Story 2

- [x] T016 [P] [US2] Unit test F111-AC-06, F111-AC-07: `SocialService.list_suggestions(source)` cursor; lọc thêm bạn/lời mời đi/chặn; chỉ nickname/avatar/level; "Kết bạn" dùng key và map lỗi F110 — file: `src/game/tests/unit/social/test_friend_suggestions.gd` — phải FAIL trước khi code — **xong — 4 test; đỏ (`list_suggestions` không tồn tại, 4 lỗi) → xanh**
- [x] T017 [P] [US2] Scene test F111-AC-06, F111-AC-08: `FindFriendsTab` đủ 3 nhóm ("Ở gần đây", "Vừa chơi cùng", Mã bạn bè), không có nhóm "Bạn cùng lớp" (ADR-0095), không có ô nhập tên; nhóm rỗng → trạng thái rỗng; lỗi API → ẩn nhóm; `g1_3` chạm avatar lớn — file: `src/game/tests/scene/test_find_friends_tab.gd` — **xong — 5 test; đỏ (Parse Error SuggestionGroup) → xanh (us2 9/9)**

### Implementation cho User Story 2

- [x] T018 [US2] `SocialService.list_suggestions`; `SuggestionGroup`; sửa `FindFriendsTab` — file: `src/game/scripts/social/social_service.gd`, `src/game/scenes/social/SuggestionGroup.tscn`, `src/game/scenes/social/FindFriendsTab.tscn` — **xong — `SocialService.list_suggestions`/`visible_suggestions`, `FriendsStore.suggestions*`, `SuggestionGroup` (hàng thường / ô chân dung lớn lớp 1–3), `FindFriendsTab` 3 nhóm**

## Phase 5: User Story 3 — Event center: sự kiện và mục tiêu cộng đồng (Priority: P1)

### Tests cho User Story 3

- [x] T019 [P] [US3] Unit test F111-AC-09, F111-AC-10, F111-AC-12: `list` theo `status`; tên thiếu `en` → `vi`; số ngày từ `ends_at` theo `Clock`; `join` giữ key; `404`/`409 EVENT_ENDED` → đóng/làm mới; `event.status.changed` mọi trạng thái — file: `src/game/tests/unit/liveops/test_live_event_service.gd` — phải FAIL trước khi code — **xong — 4 test; đỏ (`now_unix`, `join`, `get_detail` không tồn tại) → xanh**
- [x] T020 [P] [US3] Unit test F111-AC-11, F111-AC-14: `claim_milestone` giữ key khi retry, chỉ `claimed` sau response; `409 EVENT_MILESTONE_NOT_REACHED` → làm mới; `event.progress.updated` thay giá trị, nhận trùng không cộng — file: `src/game/tests/unit/liveops/test_live_event_store.gd` — **xong — 4 test; đỏ → xanh**
- [x] T021 [P] [US3] Scene test F111-AC-09, F111-AC-12, F111-AC-13: `EventCard` không có giờ/phút/giây; trạng thái tạm dừng/kết thúc; `CommunityGoalBar` mốc, "Em đã góp sức", không xếp hạng, `by_grade` sắp theo khối, không tên, ẩn với `g1_3` — file: `src/game/tests/scene/test_event_center.gd` — **xong — 4 test; đỏ (Parse Error EventCard/EventDetailPanel) → xanh (us3 16/16)**

### Implementation cho User Story 3

- [x] T022 [US3] `LiveEventService`, `LiveEventStore` — file: `src/game/scripts/liveops/live_event_service.gd`, `src/game/scripts/liveops/live_event_store.gd` — **xong — `LiveEventService` trên `KeyedWriteService` (khóa theo ý định), `LiveEventStore` (thay giá trị, chấm "mới" của phiên)**
- [x] T023 [US3] `EventCenterScreen`, `EventsTab`, `EventCard`, `EventDetailPanel`, `MilestoneList`, `CommunityGoalBar`; nối `EventBoard`, `EventCenterButton` — file: `src/game/scenes/liveops/` — **xong — `scenes/liveops/` EventCenterScreen.tscn + event_center_screen.gd, event_card.gd, event_detail_panel.gd (mốc thưởng gộp trong panel), community_goal_bar.gd; `QuestGo` cho "Đi"; nối nút HUD, bảng tin, ô menu, deep link**

## Phase 6: User Story 4 — Hôm nay, Tuần này, Mùa (Priority: P2)

### Tests cho User Story 4

- [x] T024 [P] [US4] Unit test F111-AC-15: tab Hôm nay/Tuần này đọc `QuestService` giả theo `quest_type`, tiến độ theo `step_states`, giờ đặt lại từ `available_until`; xong hết → thông điệp nghỉ, không gợi ý chơi thêm — file: `src/game/tests/unit/liveops/test_today_week_tabs.gd` — phải FAIL trước khi code — **xong — 3 test; đỏ (Parse Error QuestDayTab) → xanh**
- [x] T025 [P] [US4] Unit test F111-AC-16, F111-AC-17: `SeasonTrackService.load`/`claim_tier` (key giữ khi retry), `404` → "Mùa mới sắp bắt đầu", `409 SEASON_TIER_NOT_REACHED`, `season.track.updated`, `history` — file: `src/game/tests/unit/liveops/test_season_track_service.gd` — **xong — 4 test; đỏ (`reward_claimed`, `claim_tier` không tồn tại) → giữa chừng đỏ "Too few arguments for load()" (trùng tên hàm toàn cục) → đổi `load_track` → xanh**
- [x] T026 [P] [US4] Scene test F111-AC-16, F111-AC-18: `SeasonTrack` ảo hóa (số node ≤ khung nhìn), không có giá/nút mua; `g1_3` chỉ 2 tab, khối khác 4 tab — file: `src/game/tests/scene/test_season_track.gd` — **xong — 3 test; đỏ → xanh (us4 29/29)**

### Implementation cho User Story 4

- [x] T027 [US4] `SeasonTrackService`, `SeasonTrackStore` — file: `src/game/scripts/liveops/season_track_service.gd`, `src/game/scripts/liveops/season_track_store.gd` — **xong — `SeasonTrackService` (load_track, claim_tier, load_history, `season.track.updated`), `SeasonTrackStore`; `QuestService.load_type`/`of_type`, `QuestModel.available_until`**
- [x] T028 [US4] `TodayTab`, `WeekTab`, `SeasonTab`, `SeasonTrack`, `SeasonTierItem`, `SeasonHistoryList` — file: `src/game/scenes/liveops/` — **xong — `quest_day_tab.gd` (Hôm nay/Tuần này), `season_tab.gd`, `season_track_view.gd` (ảo hóa, pool; "Mùa trước" trong SeasonTab)**

## Phase 7: User Story 5 — Club: xem, xin vào, nhận lời mời, tạo (Priority: P2)

### Tests cho User Story 5

- [x] T029 [P] [US5] Unit test F111-AC-19, F111-AC-21: `list_mine`/`list_discoverable` (bỏ discover với `g1_3`), `list_invitations`, lọc `study_group`; `request_join`/`accept_invitation` giữ key; map `CLUB_FULL`, `CLUB_LIMIT_REACHED`, `CLUB_NOT_APPROVED`, `SOCIAL_SCOPE_VIOLATION` — file: `src/game/tests/unit/social/test_club_service.gd` — phải FAIL trước khi code — **xong — 5 test; đỏ (`load_lists`, `notice_requested` không tồn tại) → xanh**
- [x] T030 [P] [US5] Scene test F111-AC-20, F111-AC-24: `ClubPage` chỉ nickname/avatar/vai trò, mục tiêu tập thể + `my_contribution`, không đóng góp người khác; rời có xác nhận; Chat club mở `ChatDock` giả với `chat_channel_id`; `locked`/`archived` chỉ xem — file: `src/game/tests/scene/test_club_page.gd` — **xong — 5 test; đỏ (Parse Error ClubTab/ClubPage) → giữa chừng đỏ do thứ tự route FakeHttp → xanh**
- [x] T031 [P] [US5] Scene test F111-AC-22, F111-AC-23: nút "Tạo club" theo `policies.social`; form chặn tên < 2 hoặc > 64, mô tả > 500, chỉ `interest_group`/`competition_team`; `createClub` key; `451 CHAT_CONTENT_BLOCKED` thông điệp chung không echo; `403` ẩn nút — file: `src/game/tests/scene/test_club_create_dialog.gd` — **xong — 4 test; đỏ → xanh**
- [x] T032 [P] [US5] Scene test F111-AC-19: `ClubTab` thẻ club, trạng thái `pending_approval`, lời mời đang chờ — file: `src/game/tests/scene/test_club_tab.gd` — **xong — 3 test; đỏ → xanh (us5 21/21)**

### Implementation cho User Story 5

- [x] T033 [US5] `ClubService`, `ClubStore` — file: `src/game/scripts/social/club_service.gd`, `src/game/scripts/social/club_store.gd` — **xong — `ClubService` (KeyedWriteService), `ClubStore`, `ClubForm`**
- [x] T034 [US5] `ClubTab`, `ClubCard`, `ClubPage`, `ClubGoalProgress`, `ClubEventList`, `ClubMemberList`, `ClubCreateDialog`; tab Club trong `FriendsScreen` (F110) — file: `src/game/scenes/social/` — **xong — `FriendsClubs` (tab Club trong `FriendsScreen`), `club_tab.gd`, `club_card.gd`, `club_page.gd` (thành viên 20 người/lần, mục tiêu, lịch), `club_create_dialog.gd`; `ChatDock.show_club`**

## Phase 8: User Story 6 — Owner/officer duyệt yêu cầu và mời bạn (Priority: P3)

### Tests cho User Story 6

- [x] T035 [P] [US6] Unit + scene test F111-AC-25, F111-AC-26: tab Yêu cầu chỉ với owner/officer; accept giữ key, reject; `403` ẩn nút; `ClubInvitePicker` chỉ từ bạn bè, ≤ 50, kết quả lỗi từng invitee hiện "Chưa mời được" không lý do — file: `src/game/tests/scene/test_club_requests_invite.gd` — phải FAIL trước khi code — **xong — 4 test; đỏ → giữa chừng đỏ (PackedStringArray sao chép trong kết quả mời) → xanh**

### Implementation cho User Story 6

- [x] T036 [US6] `ClubRequestsTab`, `ClubInvitePicker`; `ClubService` phần yêu cầu/mời — file: `src/game/scenes/social/ClubRequestsTab.tscn`, `src/game/scenes/social/ClubInvitePicker.tscn`, `src/game/scripts/social/club_service.gd` — **xong — mục "Yêu cầu" trong `ClubPage`, `club_invite_picker.gd`; `ClubService.list_requests/accept_request/reject_request/invite`**

## Phase 9: User Story 7 — Song ngữ, trợ năng, token, an toàn, phân quyền (Priority: P2)

### Tests cho User Story 7

- [x] T037 [P] [US7] Scene test F111-AC-27: đổi `vi`↔`en` ngay, pseudo-locale `xx` không cắt `ui.events.all_done`, `ui.friends.join_unavailable.*`; ngày theo ICT và locale — file: `src/game/tests/scene/test_f111_i18n_layout.gd` — phải FAIL trước khi code — **xong — `test_f111_i18n_layout.gd` 3 test (khóa vi+en, đổi vi↔en, pseudo-locale qua ma trận 4 màn × 4 khối × 4 cửa sổ); viết trước T043, chạy xanh ngay lần đầu (không cần sửa)**
- [x] T038 [P] [US7] `lint_theme_overrides.gd` + density + trạng thái có chữ (F111-AC-28) — file: `src/game/tests/scene/test_f111_theme.gd` — **xong — `test_f111_theme.gd` 3 test; xanh lần đầu; `lint_theme_overrides` OK**
- [x] T039 [P] [US7] Unit test F111-AC-29: quét scene `liveops/`, `social/Club*` không có node đếm giây, animation nhấp nháy, khóa dịch streak/"chỉ còn"; chấm HUD tĩnh, tắt khi đã mở — file: `src/game/tests/unit/test_f111_no_dark_pattern.gd` — **xong — `test_f111_no_dark_pattern.gd` 3 test; đỏ (en `ui.club.read_only` có "only") → sửa câu → xanh**
- [x] T040 [P] [US7] Unit test F111-AC-30: chỉ nickname/avatar/level/vai trò; không ghi `user://`; xóa store khi logout/`session.session.replaced`; quét `scenes/**` không gọi `ApiClient`/`RealtimeClient` — file: `src/game/tests/unit/test_f111_privacy_arch.gd` — **xong — `test_f111_privacy_arch.gd` 4 test; xanh lần đầu**
- [x] T041 [P] [US7] Unit test F111-AC-31 (authorization + tenant ở client): `X-Tenant-Id` mọi request; `GAME_ACCESS_STUDENT_ONLY`/`TENANT_MISMATCH` → `EventBus.session_expired`; `404` sự kiện/club/lời mời → đóng màn; `422`/`428` → telemetry `client_bug` không PII — file: `src/game/tests/unit/test_f111_auth_errors.gd` — **xong — `test_f111_auth_errors.gd` 4 test; xanh lần đầu**
- [x] T042 [P] [US7] Benchmark F111-AC-32: Event center + season track 30 cấp + trang club 50 thành viên, ≥ 30 FPS preset Trung bình — file: `src/game/tests/perf/EventCenterBenchmark.tscn` — **xong (phần CI) — `tests/perf/test_event_center_budget.gd` 2 test; đỏ (cập nhật mùa 47,9 ms > 3,15 ms; trang club 493 control > 450) → vẽ gộp mỗi khung + ô cấp chỉ bind lại khi đổi + thành viên 20/lần → 1,4 ms, 224 control; **FPS trên Android tầm trung (SC-005) là đo lab — chưa chạy**. Benchmark ở dạng test GdUnit thay cho `EventCenterBenchmark.tscn`**

### Implementation cho User Story 7

- [x] T043 [US7] Hoàn thiện khóa dịch, density, announce trợ năng, giảm tần số thế giới nền khi mở màn phụ, xóa store khi đăng xuất/takeover — file: `src/game/scripts/social/club_store.gd`, `src/game/scripts/liveops/live_event_store.gd`, `src/game/scripts/liveops/season_track_store.gd` — **xong — khóa dịch, density (ma trận `test_f111_band_matrix` 5 test × 4 khối × 4 cửa sổ: đỏ lần đầu — form tạo club kích thước 0, câu điểm ngày cắt chữ → xanh), Event center tạm dừng thế giới 3D, xóa store khi đăng xuất/takeover**

## Phase 10: Polish & Cross-Cutting

- [ ] T044 [P] Integration headless 2 client: tham gia bạn (cùng/khác khu, đầy), sự kiện + nhận mốc, club tạo → duyệt → xin vào (TC-EVT-001, TC-EVT-003, TC-EVT-006, TC-EVT-007, TC-EVT-012, TC-SOC-004, TC-SOC-005, TC-SOC-006) — file: `src/game/tests/integration/test_join_friend_headless.gd`, `src/game/tests/integration/test_event_center_headless.gd`, `src/game/tests/integration/test_club_headless.gd` — **chưa: cần backend compose + hai client thật; thay thế trong CI bằng test headless trên fake (US1–US6). Pending**
- [x] T045 [P] Cập nhật GODOT_ARCHITECTURE §3 (`scenes/liveops`, `scripts/liveops`, file club), STUDENT_GAME_UX_SOCIAL §6, §11 ("Tham gia bạn" qua `hub.instance.join.follow_user_id`, endpoint sự kiện đã có), CHANGELOG — file: `docs/08-game-client/GODOT_ARCHITECTURE.md`, `docs/05-ux/STUDENT_GAME_UX_SOCIAL.md`, `CHANGELOG.md` — **xong 2026-11-28 (cùng GAME_NETWORKING §4.2 dispatcher, CHANGELOG, CODE_PLAN)**
- [ ] T046 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — **một phần: kịch bản tự động đã ghi ở quickstart §5; kịch bản 1–4, 9, 15–17, 21 cần môi trường thật — Pending**

## Dependencies & Execution Order

- Setup → Foundational → US1 và US3 (song song, P1) → US2, US4, US5 (P2) → US6 → US7 → Polish.
- US1 cần delta F091 (`follow_user_id`) và T010; US2 cần `listFriendSuggestions` (F095); US5 cần `listMyClubInvitations`, `policies.social`; US3/US4 dùng contract F077 đã có. Trước khi F077/F095/F120 Done chạy trên Prism mock (profile `mock`, nhãn MOCK, không vào build phát hành — R17).

## Bằng chứng: F111-AC → test (2026-11-28, GdUnit4 headless, fake + fixture contract)

| AC | Test |
|---|---|
| F111-AC-01, 03 | `tests/unit/multiplayer/test_join_friend_service.gd`, `tests/unit/multiplayer/test_hub_sync_follow.gd` |
| F111-AC-02 | `tests/unit/social/test_join_availability.gd`, `test_f111_band_matrix` (`friends_join`) |
| F111-AC-04, 05 | `tests/unit/multiplayer/test_join_friend_failures.gd` |
| F111-AC-06, 07, 08 | `tests/unit/social/test_friend_suggestions.gd`, `tests/scene/test_find_friends_tab.gd` |
| F111-AC-09, 12 | `tests/unit/liveops/test_live_event_service.gd`, `tests/scene/test_event_center.gd`, `tests/scene/test_event_center_entry.gd` |
| F111-AC-10 | `test_live_event_service.gd`, `tests/unit/liveops/test_today_week_tabs.gd` (QuestGo) |
| F111-AC-11, 14 | `tests/unit/liveops/test_live_event_store.gd` |
| F111-AC-13 | `tests/scene/test_event_center.gd`, `test_live_event_store.gd` |
| F111-AC-15 | `tests/unit/liveops/test_today_week_tabs.gd`, `tests/unit/test_f111_no_dark_pattern.gd` |
| F111-AC-16, 17 | `tests/unit/liveops/test_season_track_service.gd`, `tests/scene/test_season_track.gd` |
| F111-AC-18 | `tests/scene/test_season_track.gd` |
| F111-AC-19 | `tests/unit/social/test_club_service.gd`, `tests/scene/test_club_tab.gd` |
| F111-AC-20, 24 | `tests/scene/test_club_page.gd` |
| F111-AC-21 | `test_club_service.gd`, `test_club_tab.gd` |
| F111-AC-22, 23 | `tests/scene/test_club_create_dialog.gd` |
| F111-AC-25, 26 | `tests/scene/test_club_requests_invite.gd` |
| F111-AC-27 | `tests/scene/test_f111_i18n_layout.gd` |
| F111-AC-28 | `tests/scene/test_f111_theme.gd`, `tests/scene/test_f111_band_matrix.gd` |
| F111-AC-29 | `tests/unit/test_f111_no_dark_pattern.gd`, `test_event_center_entry.gd` |
| F111-AC-30 | `tests/unit/test_f111_privacy_arch.gd`, `tests/unit/test_f111_wiring.gd` |
| F111-AC-31 | `tests/unit/test_f111_auth_errors.gd` |
| F111-AC-32 | `tests/perf/test_event_center_budget.gd` (CI guard); FPS Android tầm trung — **chưa đo (lab)** |

Còn mở trước `Done`: T044 (hai client thật trên backend compose), T046 kịch bản quickstart với môi trường thật, đo FPS SC-005 trên thiết bị tham chiếu.

## Definition of Done

- [x] Mọi tiêu chí F111-AC-* có test pass (ghi mã test/TC) — bảng trên (với fake + fixture contract)
- [ ] `make godot-test`, `make godot-export-check`, `make design-tokens-check` xanh
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — fake/fixture chỉ trong `tests/`
- [x] Tài liệu sở hữu, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật (In progress)
