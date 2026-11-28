# Tasks: F110 — Godot — màn boss co-op, thẻ dữ kiện bước `interdependent`, danh sách bạn bè

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Thêm input action `open_friends` (`F`) — file: `src/game/project.godot` — **xong 2026-11-28:** `open_friends` (phím F, physical keycode 70) trong `project.godot`; WorldRoot mở màn `friends`
- [x] T002 [P] Khóa dịch `ui.boss.*` (energy, phase, need_contribution, retry_phase, leave_confirm, defeated, reached_phase, reconnecting, solo, invite_friends), `ui.quest.coop.*`, `ui.quest.clue.*` (share, shared_by, extra_role, answer_final), `ui.friends.*`, `ui.friendcode.*`, `ui.error.<CODE>` cho mã của feature — file: `src/game/locale/vi.po`, `src/game/locale/en.po` — **xong 2026-11-28:** 156 khóa vi + en (`ui.boss.*`, `ui.quest.coop.*`, `ui.quest.clue.*`, `ui.friends.*`, `ui.friendcode.*`, `ui.menu.friends`, `ui.quest.coop_step`, `ui.error.BOSS_ENCOUNTER_FULL|BOSS_PHASE_CLOSED|ROOM_NOT_MEMBER|PARTY_NOT_LEADER|PERMISSION_DENIED|SOCIAL_SCOPE_VIOLATION|FRIEND_LIMIT_REACHED|FRIEND_CODE_INVALID`); `extract_pot --check`, `pseudo_locale --check` xanh; test `test_f110_i18n_layout.gd::test_every_f110_key_has_vi_and_en`
- [x] T003 [P] Fixture từ example OpenAPI/AsyncAPI (boss solo/party, mọi message `boss.*`, câu hỏi mọi loại, `party_progress`, vai/thẻ, bạn bè, lời mời, mã, chặn/tắt tiếng, Problem) — nhãn MOCK — file: `src/game/tests/fixtures/{boss,coop_quest,social}/` — **xong 2026-11-28:** `gen_fixtures.mjs` thêm schema F110 + `boss.*`/`quest.*`; message AsyncAPI chưa có `examples` có mẫu tất định kiểm ajv (`asyncapi/<type>.sample.json`); dữ liệu mẫu trong `tests/fakes/f110_fixtures.gd` (nhãn TEST-ONLY, R17); commit riêng `test(game-client): regenerate contract fixtures …`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Theo dõi delta F103 (boss/quest co-op/thẻ dữ kiện, AsyncAPI `boss.question.assigned`, `boss.answer.evaluated`, `quest.roles.assigned`, `quest.clue.shared`, `party_progress`) — owner F103 — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=game` — **xong 2026-11-28 (lượt server F110/F111: delta trên main, xem contracts §6 "Trạng thái trên main")**
- [x] T005 Theo dõi delta F101 (`room_type: boss`), F095 (mã lỗi social), F091 (enum presence), đề xuất `PlayerCard.title` — owner tương ứng — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking` — **xong 2026-11-28 (lượt server F110/F111: delta trên main, xem contracts §6 "Trạng thái trên main")**
- [x] T006 Sinh lại fixture sau khi T004/T005 merge + test contract parse mọi example — file: `src/game/tests/contract/test_f110_fixtures.gd` — kiểm: `make godot-test` — **xong 2026-11-28:** `tests/contract/test_f110_fixtures.gd` 5/5 pass (mọi fixture F110 parse vào model, mẫu AsyncAPI áp vào store)
- [x] T007 Model typed theo data-model (`from_dict` bỏ trường lạ) + test parse — file: `src/game/scripts/{challenge,quest,social}/models/`, `src/game/tests/unit/models/test_f110_models.gd` — **xong 2026-11-28:** model typed `scripts/challenge/models/{boss_summary,boss_encounter_model,boss_member_model,boss_question_model}.gd`, `scripts/quest/models/{party_quest_progress_model,role_assignment_model,clue_card_model}.gd`, `scripts/social/models/{friend_model,friend_request_model,friend_code_model,relation_model}.gd`; parse kiểm ở `test_f110_fixtures.gd`
- [x] T008 `AppState` thêm `boss`, `friends`; `EventBus` thêm `friend_request_received`; đăng ký `BossClient`, `CoopQuestService`, `SocialService`, `PresenceService` trong `ServiceLocator`; dispatcher `boss.*`, `quest.roles.*`, `quest.clue.*`, `presence.friend.updated` — file: `src/game/scripts/core/app_state.gd`, `src/game/scripts/core/event_bus.gd`, `src/game/scripts/core/service_locator.gd` — **xong 2026-11-28:** `EventBus.friend_request_received`; `ServiceLocator.boss`/`coop_quest` + `social` (realtime, party), `presence` → `FriendsStore`; dispatcher `boss`, `quest`, `presence`, `notification` (friend request); `AppState.blocking_activity = boss` qua `BossStore` (store F110 nằm trong service của `AppState.services`, không thêm trường AppState); `ServiceLocator.end_session`/`start_session` xóa store F110

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Đánh boss nhiều phase, solo và party (Priority: P1) 🎯 MVP

**Goal**: `BossArena` solo/party chơi hết vòng. **Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T009 [P] [US1] Unit test F110-AC-01, F110-AC-10: `start_solo` giữ key khi retry, boss không `solo` → chỉ "Rủ bạn"; map `CHALLENGE_NOT_ELIGIBLE`/`CHALLENGE_ATTEMPT_LIMIT`; `leave` có key, `ROOM_NOT_MEMBER`/`404` → về Hub — file: `src/game/tests/unit/boss/test_boss_client.gd` — phải FAIL trước khi code — **xong 2026-11-28:** `tests/unit/boss/test_boss_client.gd` 7/7 pass
- [x] T010 [P] [US1] Unit test F110-AC-02, F110-AC-03, F110-AC-09: `party.session.started` `boss` → `SceneRouter.go_to` + `boss.encounter.ready`; áp `state_version`, lệch → `getBossEncounter`; mất mạng khóa nộp, kết nối lại resync + ready, về câu đang mở — file: `src/game/tests/unit/boss/test_boss_store.gd` — **xong 2026-11-28:** `tests/unit/boss/test_boss_store.gd` 7/7 pass
- [x] T011 [P] [US1] Scene test F110-AC-04, F110-AC-05: `QuestionPanelHost` dùng renderer F063, gửi `boss.answer.submit` đủ trường, khóa khi chờ ack, giữ `question_instance_id` khi gửi lại; đúng/sai chỉ cho mình, năng lượng chỉ theo server, `BOSS_PHASE_CLOSED` đóng câu — file: `src/game/tests/scene/test_question_panel_host.gd` — **xong 2026-11-28:** `tests/scene/test_question_panel_host.gd` 5/5 pass
- [x] T012 [P] [US1] Scene test F110-AC-06, F110-AC-07: `ContributionList` chỉ ✓/đồng hồ cho bạn, số câu chỉ của mình, không xếp hạng; băng thử lại phase; chuyển phase ≤ 2 s; `PhaseTimer` `g1_3` dạng thanh — file: `src/game/tests/scene/test_boss_hud.gd` — **xong 2026-11-28:** `tests/scene/test_boss_hud.gd` 6/6 pass
- [x] T013 [P] [US1] Scene test F110-AC-08: `BossResult` `defeated`/`time_up`/`abandoned`, không chữ "Thất bại", phần thưởng của mình; `AppState.blocking_activity` về rỗng sau kết quả — file: `src/game/tests/scene/test_boss_result.gd` — **xong 2026-11-28:** `tests/scene/test_boss_result.gd` 4/4 pass
- [x] T014 [P] [US1] Benchmark F110-AC-11: arena 4 nhân vật + boss, particle theo preset, ≥ 30 FPS, camera cố định mobile — file: `src/game/tests/perf/BossArenaBenchmark.tscn` — **xong 2026-11-28:** thay `.tscn` bằng `tests/perf/test_boss_arena_budget.gd` 1/1 pass (2D, không Node3D/SubViewport/Camera, không polling, ngân sách Control + cập nhật, particle ≤ 200 / tắt ở Thấp); số FPS trên Android tầm trung (SC-004) là đo lab — **chưa chạy**

### Implementation cho User Story 1

- [x] T015 [US1] `BossClient`, `BossStore` — file: `src/game/scripts/challenge/boss_client.gd`, `src/game/scripts/challenge/boss_store.gd` — **xong 2026-11-28:** `scripts/challenge/boss_client.gd`, `boss_store.gd`
- [x] T016 [P] [US1] Zone `boss_arena` (ngân sách cảnh boss, LOD, preset) — file: `src/game/scenes/world/zones/boss_arena/Zone.tscn` — **xong 2026-11-28:** [Confirmed — Clarifications 2026-11-28] không tạo zone 3D: `BossArena` là màn 2D trên thế giới tạm dừng (`boss_figure.gd` vẽ vector + `CPUParticles2D` theo preset)
- [x] T017 [US1] `BossInfoCard`, `BossArena`, `BossHud`, `BossEnergyBar`, `PhaseTimer`, `ContributionList`, `PartyReadyList`, `QuestionPanelHost`, `BossResult`; slot `ChatDock` (F119, ẩn khi chưa có) — file: `src/game/scenes/challenge/boss/` — **xong 2026-11-28:** `scenes/challenge/boss/` BossInfoCard.tscn, BossArena.tscn + boss_energy_bar.gd, phase_timer.gd, contribution_list.gd (PartyReadyList/ContributionList), question_panel_host.gd, boss_figure.gd, boss_result.gd; `ChatDock` kênh `party` sau nút "Chat nhóm" (boss party)

## Phase 4: User Story 2 — Tiến độ quest co-op chung (Priority: P2)

### Tests cho User Story 2

- [x] T018 [P] [US2] Scene test F110-AC-12, F110-AC-13: `PartyProgressStrip` theo `party_progress`, không số liệu của bạn; bước học tập gọi `startQuestActivity` của chính mình → `ActivityHost` — file: `src/game/tests/scene/test_party_progress_strip.gd` — phải FAIL trước khi code — **xong 2026-11-28:** `tests/scene/test_party_progress_strip.gd` 4/4 pass

### Implementation cho User Story 2

- [x] T019 [US2] `CoopQuestService` (phần `party_progress`), `CoopQuestStore`; gắn `PartyProgressStrip` vào `QuestTracker` (F072) — file: `src/game/scripts/quest/coop_quest_service.gd`, `src/game/scripts/quest/coop_quest_store.gd`, `src/game/scenes/quest/PartyProgressStrip.tscn` — **xong 2026-11-28:** `coop_quest_service.gd`, `coop_quest_store.gd`, `scenes/quest/party_progress_strip.gd` gắn vào `QuestTracker`; `QuestModel.party_progress`

## Phase 5: User Story 3 — Bước `interdependent`: vai, thẻ dữ kiện, chia sẻ (Priority: P2)

### Tests cho User Story 3

- [x] T020 [P] [US3] Unit test F110-AC-14, F110-AC-15, F110-AC-16: vai của mình/bạn; chỉ leader thấy `RoleAssignDialog`, `PARTY_NOT_LEADER` thân thiện; chỉ hiện thẻ server trả, không ghi đĩa; `share` giữ key khi retry, `quest.clue.shared` không tạo thẻ trùng — file: `src/game/tests/unit/quest/test_coop_quest_service.gd` — phải FAIL trước khi code — **xong 2026-11-28:** `tests/unit/quest/test_coop_quest_service.gd` 6/6 pass
- [x] T021 [P] [US3] Scene test F110-AC-16, F110-AC-17, F110-AC-18: thẻ hiện ở bạn ≤ 1 s; nút chat chỉ khi `clue_card_and_chat`; "Trả lời câu cuối" mở activity của mình; vai thêm khi bạn rớt mạng + toast; solo hiện mọi vai — file: `src/game/tests/scene/test_interdependent_panel.gd` — **xong 2026-11-28:** `tests/scene/test_interdependent_panel.gd` 8/8 pass

### Implementation cho User Story 3

- [x] T022 [US3] `CoopQuestService` (vai, thẻ, chia sẻ, gán vai) — file: `src/game/scripts/quest/coop_quest_service.gd` — **xong 2026-11-28:** `CoopQuestService.load_clues/share/assign_roles/start_my_activity`
- [x] T023 [US3] `InterdependentStepPanel`, `ClueCard`, `SharedClueBoard`, `RoleAssignDialog` — file: `src/game/scenes/quest/InterdependentStepPanel.tscn`, `src/game/scenes/quest/ClueCard.tscn`, `src/game/scenes/quest/SharedClueBoard.tscn`, `src/game/scenes/quest/RoleAssignDialog.tscn` — **xong 2026-11-28:** `scenes/quest/InterdependentStepPanel.tscn` + clue_card.gd, shared_clue_board.gd, role_assign_dialog.gd (lớp dựng bằng code, không `.tscn` riêng); Quest Journal: nút "Cùng nhóm giải" cho bước `interdependent`

## Phase 6: User Story 4 — Bạn bè: danh sách, lời mời, mã bạn bè, chặn, tắt tiếng (Priority: P1)

### Tests cho User Story 4

- [x] T024 [P] [US4] Unit test F110-AC-19, F110-AC-20, F110-AC-21: `listFriends` cursor, nhóm theo presence, không trường ngoài thẻ công khai; gộp presence ≤ 1 lần/s, mặc định offline; lời mời đi luôn "Đang chờ" — file: `src/game/tests/unit/social/test_social_service.gd` — phải FAIL trước khi code — **xong 2026-11-28:** `tests/unit/social/test_social_service.gd` 5/5 pass
- [x] T025 [P] [US4] Unit test F110-AC-22, F110-AC-24, F110-AC-25: `sendFriendRequest` key + map `SOCIAL_SCOPE_VIOLATION`/`FRIEND_LIMIT_REACHED`/`RATE_LIMITED` (`Retry-After`); mời party qua `PartyService` giả (tạo party nếu chưa có); chặn/tắt tiếng/bỏ chặn; không có API tìm theo tên — file: `src/game/tests/unit/social/test_social_actions.gd` — **xong 2026-11-28:** `tests/unit/social/test_social_actions.gd` 7/7 pass
- [x] T026 [P] [US4] Scene test F110-AC-23: ô mã tự viết hoa, chặn ký tự lạ, tối đa 12, bật nút khi khớp pattern; đếm ngược theo `expires_at`; `FRIEND_CODE_INVALID` — file: `src/game/tests/scene/test_friend_code_widget.gd` — **xong 2026-11-28:** `tests/scene/test_friend_code_widget.gd` 3/3 pass
- [x] T027 [P] [US4] Scene test F110-AC-19, F110-AC-24, F110-AC-26: `FriendsScreen` ≤ 2 chạm tới Chặn/Tắt tiếng/Báo cáo (ẩn khi chưa có F119); bật "Ẩn trạng thái" gửi `presence.status.set` `hidden = true` — file: `src/game/tests/scene/test_friends_screen.gd` — **xong 2026-11-28:** `tests/scene/test_friends_screen.gd` 8/8 pass

### Implementation cho User Story 4

- [x] T028 [US4] `SocialService`, `FriendsStore`, `PresenceService` — file: `src/game/scripts/social/social_service.gd`, `src/game/scripts/social/friends_store.gd`, `src/game/scripts/multiplayer/presence_service.gd` — **xong 2026-11-28:** `social_service.gd` (mở rộng từ F119), `friends_store.gd`, `presence_service.gd` (`presence.friend.updated`, `set_hidden`)
- [x] T029 [US4] `FriendsScreen`, `FriendRow`, `FriendProfilePanel`, `FriendRequestList`, `FindFriendsTab` (nhóm "Ở gần đây" từ hub F092), `FriendCodeWidget`, `BlockList`; nút "Kết bạn" trên thẻ người chơi Hub — file: `src/game/scenes/social/` — **xong 2026-11-28:** `scenes/social/FriendsScreen.tscn` + friend_list_view.gd (ảo hóa), friend_row.gd, friend_profile_panel.gd, friend_request_list.gd, find_friends_tab.gd (nhóm "Ở gần đây" từ `HubSync.room.players`), friend_code_widget.gd, block_list.gd, friends_ui.gd; ô "Bạn bè" trong menu "Thêm"; thẻ người chơi chạm-chọn trong Hub: không có ở lát này (Clarifications 2026-11-28)

## Phase 7: User Story 5 — Song ngữ, trợ năng, token, an toàn, phân quyền (Priority: P2)

### Tests cho User Story 5

- [x] T030 [P] [US5] Scene test F110-AC-27: đổi `vi`↔`en` ngay, pseudo-locale `xx` không cắt `ui.boss.need_contribution`, `ui.friendcode.instructions` — file: `src/game/tests/scene/test_f110_i18n_layout.gd` — phải FAIL trước khi code — **xong 2026-11-28:** `tests/scene/test_f110_i18n_layout.gd` 3/3 pass (vi/en, đổi ngôn ngữ, pseudo-locale cả 4 khối × 4 cửa sổ)
- [x] T031 [P] [US5] `lint_theme_overrides.gd` + density + trạng thái có biểu tượng + chữ (F110-AC-28) — file: `src/game/tests/scene/test_f110_theme.gd` — **xong 2026-11-28:** `tests/scene/test_f110_theme.gd` 3/3 pass + `tests/scene/test_f110_band_matrix.gd` 4/4 pass (19 màn × 4 khối × 4 cửa sổ, probe vật lý 12/40 CSS px)
- [x] T032 [P] [US5] Unit test F110-AC-29: chỉ nickname/avatar/level/danh hiệu; không ghi `user://`; xóa store khi logout/`session.session.replaced`; quét `scenes/**` không gọi `ApiClient`/`RealtimeClient` — file: `src/game/tests/unit/test_f110_privacy_arch.gd` — **xong 2026-11-28:** `tests/unit/test_f110_privacy_arch.gd` 5/5 pass
- [x] T033 [P] [US5] Unit test F110-AC-30: `X-Tenant-Id` mọi request; `GAME_ACCESS_STUDENT_ONLY`/`TENANT_MISMATCH` → `EventBus.session_expired`; `404` encounter/thẻ/bạn → đóng màn, làm mới; `422` → telemetry `client_bug` không PII — file: `src/game/tests/unit/test_f110_auth_errors.gd` — **xong 2026-11-28:** `tests/unit/test_f110_auth_errors.gd` 4/4 pass

### Implementation cho User Story 5

- [x] T034 [US5] Hoàn thiện khóa dịch, density, announce trợ năng, xóa store khi đăng xuất/takeover — file: `src/game/scripts/challenge/boss_store.gd`, `src/game/scripts/quest/coop_quest_store.gd`, `src/game/scripts/social/friends_store.gd` — **xong 2026-11-28:** khóa dịch, density, xóa store khi `end_session`; câu thông báo trợ năng qua `accessibility_name` của dòng bạn

## Phase 8: Polish & Cross-Cutting

- [ ] T035 [P] Integration headless 2 client (**còn chờ** 2026-11-28: cần backend + `EW_REALTIME_PUBLIC_URL`, như F102/F119): boss party 2 phase, quest co-op có bước `interdependent`, kết bạn bằng mã (TC-CHL-004, TC-CHL-005, TC-CHL-007, TC-PTY-007, TC-PTY-010, TC-SOC-001) — file: `src/game/tests/integration/test_boss_party_headless.gd`, `src/game/tests/integration/test_friends_headless.gd`
- [x] T036 [P] Cập nhật STUDENT_GAME_UX_CORE S06 (đóng góp chỉ ✓, không số câu của bạn), GODOT_ARCHITECTURE §3 (`scenes/challenge/boss`, `scripts/social`, `scripts/quest`), CHANGELOG — file: `docs/05-ux/STUDENT_GAME_UX_CORE.md`, `docs/08-game-client/GODOT_ARCHITECTURE.md`, `CHANGELOG.md` — **xong 2026-11-28:** GODOT_ARCHITECTURE §3/§4/§6, GAME_NETWORKING §4.2, STUDENT_GAME_UX_CORE S06, STUDENT_GAME_UX_SOCIAL §6, CHANGELOG
- [ ] T037 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — 2026-11-28: phần tự động + ảnh ghi ở quickstart §5; kịch bản tay với backend thật + hai thiết bị **còn chờ**

## Dependencies & Execution Order

- Setup → Foundational → US1 và US4 (song song, P1) → US2 → US3 → US5 → Polish.
- US1 party cần F102 (`PartyStore`) và delta F101/F103; US2/US3 cần `QuestTracker` (F072); US4 độc lập với boss. Trước khi F095/F103 Done chạy trên Prism mock (profile `mock`, nhãn MOCK, không vào build phát hành — R17).

## Definition of Done

- [x] Mọi tiêu chí F110-AC-* có test pass (ghi mã test/TC) — bảng ở [quickstart §5](quickstart.md#5-bằng-chứng-2026-11-28-phần-godot-client) (fake + fixture contract; phần lab FPS và chạy thật hai client còn chờ)
- [ ] `make godot-test`, `make godot-export-check`, `make design-tokens-check` xanh — 2026-11-28: GdUnit4 toàn bộ 1571 test: 1548 pass, 16 skip, 10 fail + 3 error chỉ ở `test_manifest_verifier`/`test_prefetch_scheduler` của F133 (lỗi sẵn có: cùng kết quả trên `origin/main` sạch); lint Godot xanh; export/design-tokens không chạy (không đổi token, không đổi preset export)
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — fake/fixture chỉ ở `tests/`
- [x] Tài liệu sở hữu, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
