# Tasks: F110 — Godot — màn boss co-op, thẻ dữ kiện bước `interdependent`, danh sách bạn bè

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Thêm input action `open_friends` (`F`) — file: `src/game/project.godot`
- [ ] T002 [P] Khóa dịch `ui.boss.*` (energy, phase, need_contribution, retry_phase, leave_confirm, defeated, reached_phase, reconnecting, solo, invite_friends), `ui.quest.coop.*`, `ui.quest.clue.*` (share, shared_by, extra_role, answer_final), `ui.friends.*`, `ui.friendcode.*`, `ui.error.<CODE>` cho mã của feature — file: `src/game/locale/vi.po`, `src/game/locale/en.po`
- [ ] T003 [P] Fixture từ example OpenAPI/AsyncAPI (boss solo/party, mọi message `boss.*`, câu hỏi mọi loại, `party_progress`, vai/thẻ, bạn bè, lời mời, mã, chặn/tắt tiếng, Problem) — nhãn MOCK — file: `src/game/tests/fixtures/{boss,coop_quest,social}/`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Theo dõi delta F103 (boss/quest co-op/thẻ dữ kiện, AsyncAPI `boss.question.assigned`, `boss.answer.evaluated`, `quest.roles.assigned`, `quest.clue.shared`, `party_progress`) — owner F103 — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=game`
- [ ] T005 Theo dõi delta F101 (`room_type: boss`), F095 (mã lỗi social), F091 (enum presence), đề xuất `PlayerCard.title` — owner tương ứng — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`
- [ ] T006 Sinh lại fixture sau khi T004/T005 merge + test contract parse mọi example — file: `src/game/tests/contract/test_f110_fixtures.gd` — kiểm: `make godot-test`
- [ ] T007 Model typed theo data-model (`from_dict` bỏ trường lạ) + test parse — file: `src/game/scripts/{challenge,quest,social}/models/`, `src/game/tests/unit/models/test_f110_models.gd`
- [ ] T008 `AppState` thêm `boss`, `friends`; `EventBus` thêm `friend_request_received`; đăng ký `BossClient`, `CoopQuestService`, `SocialService`, `PresenceService` trong `ServiceLocator`; dispatcher `boss.*`, `quest.roles.*`, `quest.clue.*`, `presence.friend.updated` — file: `src/game/scripts/core/app_state.gd`, `src/game/scripts/core/event_bus.gd`, `src/game/scripts/core/service_locator.gd`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Đánh boss nhiều phase, solo và party (Priority: P1) 🎯 MVP

**Goal**: `BossArena` solo/party chơi hết vòng. **Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T009 [P] [US1] Unit test F110-AC-01, F110-AC-10: `start_solo` giữ key khi retry, boss không `solo` → chỉ "Rủ bạn"; map `CHALLENGE_NOT_ELIGIBLE`/`CHALLENGE_ATTEMPT_LIMIT`; `leave` có key, `ROOM_NOT_MEMBER`/`404` → về Hub — file: `src/game/tests/unit/boss/test_boss_client.gd` — phải FAIL trước khi code
- [ ] T010 [P] [US1] Unit test F110-AC-02, F110-AC-03, F110-AC-09: `party.session.started` `boss` → `SceneRouter.go_to` + `boss.encounter.ready`; áp `state_version`, lệch → `getBossEncounter`; mất mạng khóa nộp, kết nối lại resync + ready, về câu đang mở — file: `src/game/tests/unit/boss/test_boss_store.gd`
- [ ] T011 [P] [US1] Scene test F110-AC-04, F110-AC-05: `QuestionPanelHost` dùng renderer F063, gửi `boss.answer.submit` đủ trường, khóa khi chờ ack, giữ `question_instance_id` khi gửi lại; đúng/sai chỉ cho mình, năng lượng chỉ theo server, `BOSS_PHASE_CLOSED` đóng câu — file: `src/game/tests/scene/test_question_panel_host.gd`
- [ ] T012 [P] [US1] Scene test F110-AC-06, F110-AC-07: `ContributionList` chỉ ✓/đồng hồ cho bạn, số câu chỉ của mình, không xếp hạng; băng thử lại phase; chuyển phase ≤ 2 s; `PhaseTimer` `g1_3` dạng thanh — file: `src/game/tests/scene/test_boss_hud.gd`
- [ ] T013 [P] [US1] Scene test F110-AC-08: `BossResult` `defeated`/`time_up`/`abandoned`, không chữ "Thất bại", phần thưởng của mình; `AppState.blocking_activity` về rỗng sau kết quả — file: `src/game/tests/scene/test_boss_result.gd`
- [ ] T014 [P] [US1] Benchmark F110-AC-11: arena 4 nhân vật + boss, particle theo preset, ≥ 30 FPS, camera cố định mobile — file: `src/game/tests/perf/BossArenaBenchmark.tscn`

### Implementation cho User Story 1

- [ ] T015 [US1] `BossClient`, `BossStore` — file: `src/game/scripts/challenge/boss_client.gd`, `src/game/scripts/challenge/boss_store.gd`
- [ ] T016 [P] [US1] Zone `boss_arena` (ngân sách cảnh boss, LOD, preset) — file: `src/game/scenes/world/zones/boss_arena/Zone.tscn`
- [ ] T017 [US1] `BossInfoCard`, `BossArena`, `BossHud`, `BossEnergyBar`, `PhaseTimer`, `ContributionList`, `PartyReadyList`, `QuestionPanelHost`, `BossResult`; slot `ChatDock` (F119, ẩn khi chưa có) — file: `src/game/scenes/challenge/boss/`

## Phase 4: User Story 2 — Tiến độ quest co-op chung (Priority: P2)

### Tests cho User Story 2

- [ ] T018 [P] [US2] Scene test F110-AC-12, F110-AC-13: `PartyProgressStrip` theo `party_progress`, không số liệu của bạn; bước học tập gọi `startQuestActivity` của chính mình → `ActivityHost` — file: `src/game/tests/scene/test_party_progress_strip.gd` — phải FAIL trước khi code

### Implementation cho User Story 2

- [ ] T019 [US2] `CoopQuestService` (phần `party_progress`), `CoopQuestStore`; gắn `PartyProgressStrip` vào `QuestTracker` (F072) — file: `src/game/scripts/quest/coop_quest_service.gd`, `src/game/scripts/quest/coop_quest_store.gd`, `src/game/scenes/quest/PartyProgressStrip.tscn`

## Phase 5: User Story 3 — Bước `interdependent`: vai, thẻ dữ kiện, chia sẻ (Priority: P2)

### Tests cho User Story 3

- [ ] T020 [P] [US3] Unit test F110-AC-14, F110-AC-15, F110-AC-16: vai của mình/bạn; chỉ leader thấy `RoleAssignDialog`, `PARTY_NOT_LEADER` thân thiện; chỉ hiện thẻ server trả, không ghi đĩa; `share` giữ key khi retry, `quest.clue.shared` không tạo thẻ trùng — file: `src/game/tests/unit/quest/test_coop_quest_service.gd` — phải FAIL trước khi code
- [ ] T021 [P] [US3] Scene test F110-AC-16, F110-AC-17, F110-AC-18: thẻ hiện ở bạn ≤ 1 s; nút chat chỉ khi `clue_card_and_chat`; "Trả lời câu cuối" mở activity của mình; vai thêm khi bạn rớt mạng + toast; solo hiện mọi vai — file: `src/game/tests/scene/test_interdependent_panel.gd`

### Implementation cho User Story 3

- [ ] T022 [US3] `CoopQuestService` (vai, thẻ, chia sẻ, gán vai) — file: `src/game/scripts/quest/coop_quest_service.gd`
- [ ] T023 [US3] `InterdependentStepPanel`, `ClueCard`, `SharedClueBoard`, `RoleAssignDialog` — file: `src/game/scenes/quest/InterdependentStepPanel.tscn`, `src/game/scenes/quest/ClueCard.tscn`, `src/game/scenes/quest/SharedClueBoard.tscn`, `src/game/scenes/quest/RoleAssignDialog.tscn`

## Phase 6: User Story 4 — Bạn bè: danh sách, lời mời, mã bạn bè, chặn, tắt tiếng (Priority: P1)

### Tests cho User Story 4

- [ ] T024 [P] [US4] Unit test F110-AC-19, F110-AC-20, F110-AC-21: `listFriends` cursor, nhóm theo presence, không trường ngoài thẻ công khai; gộp presence ≤ 1 lần/s, mặc định offline; lời mời đi luôn "Đang chờ" — file: `src/game/tests/unit/social/test_social_service.gd` — phải FAIL trước khi code
- [ ] T025 [P] [US4] Unit test F110-AC-22, F110-AC-24, F110-AC-25: `sendFriendRequest` key + map `SOCIAL_SCOPE_VIOLATION`/`FRIEND_LIMIT_REACHED`/`RATE_LIMITED` (`Retry-After`); mời party qua `PartyService` giả (tạo party nếu chưa có); chặn/tắt tiếng/bỏ chặn; không có API tìm theo tên — file: `src/game/tests/unit/social/test_social_actions.gd`
- [ ] T026 [P] [US4] Scene test F110-AC-23: ô mã tự viết hoa, chặn ký tự lạ, tối đa 12, bật nút khi khớp pattern; đếm ngược theo `expires_at`; `FRIEND_CODE_INVALID` — file: `src/game/tests/scene/test_friend_code_widget.gd`
- [ ] T027 [P] [US4] Scene test F110-AC-19, F110-AC-24, F110-AC-26: `FriendsScreen` ≤ 2 chạm tới Chặn/Tắt tiếng/Báo cáo (ẩn khi chưa có F119); bật "Ẩn trạng thái" gửi `presence.status.set` `hidden = true` — file: `src/game/tests/scene/test_friends_screen.gd`

### Implementation cho User Story 4

- [ ] T028 [US4] `SocialService`, `FriendsStore`, `PresenceService` — file: `src/game/scripts/social/social_service.gd`, `src/game/scripts/social/friends_store.gd`, `src/game/scripts/multiplayer/presence_service.gd`
- [ ] T029 [US4] `FriendsScreen`, `FriendRow`, `FriendProfilePanel`, `FriendRequestList`, `FindFriendsTab` (nhóm "Ở gần đây" từ hub F092), `FriendCodeWidget`, `BlockList`; nút "Kết bạn" trên thẻ người chơi Hub — file: `src/game/scenes/social/`

## Phase 7: User Story 5 — Song ngữ, trợ năng, token, an toàn, phân quyền (Priority: P2)

### Tests cho User Story 5

- [ ] T030 [P] [US5] Scene test F110-AC-27: đổi `vi`↔`en` ngay, pseudo-locale `xx` không cắt `ui.boss.need_contribution`, `ui.friendcode.instructions` — file: `src/game/tests/scene/test_f110_i18n_layout.gd` — phải FAIL trước khi code
- [ ] T031 [P] [US5] `lint_theme_overrides.gd` + density + trạng thái có biểu tượng + chữ (F110-AC-28) — file: `src/game/tests/scene/test_f110_theme.gd`
- [ ] T032 [P] [US5] Unit test F110-AC-29: chỉ nickname/avatar/level/danh hiệu; không ghi `user://`; xóa store khi logout/`session.session.replaced`; quét `scenes/**` không gọi `ApiClient`/`RealtimeClient` — file: `src/game/tests/unit/test_f110_privacy_arch.gd`
- [ ] T033 [P] [US5] Unit test F110-AC-30: `X-Tenant-Id` mọi request; `GAME_ACCESS_STUDENT_ONLY`/`TENANT_MISMATCH` → `EventBus.session_expired`; `404` encounter/thẻ/bạn → đóng màn, làm mới; `422` → telemetry `client_bug` không PII — file: `src/game/tests/unit/test_f110_auth_errors.gd`

### Implementation cho User Story 5

- [ ] T034 [US5] Hoàn thiện khóa dịch, density, announce trợ năng, xóa store khi đăng xuất/takeover — file: `src/game/scripts/challenge/boss_store.gd`, `src/game/scripts/quest/coop_quest_store.gd`, `src/game/scripts/social/friends_store.gd`

## Phase 8: Polish & Cross-Cutting

- [ ] T035 [P] Integration headless 2 client: boss party 2 phase, quest co-op có bước `interdependent`, kết bạn bằng mã (TC-CHL-004, TC-CHL-005, TC-CHL-007, TC-PTY-007, TC-PTY-010, TC-SOC-001) — file: `src/game/tests/integration/test_boss_party_headless.gd`, `src/game/tests/integration/test_friends_headless.gd`
- [ ] T036 [P] Cập nhật STUDENT_GAME_UX_CORE S06 (đóng góp chỉ ✓, không số câu của bạn), GODOT_ARCHITECTURE §3 (`scenes/challenge/boss`, `scripts/social`, `scripts/quest`), CHANGELOG — file: `docs/05-ux/STUDENT_GAME_UX_CORE.md`, `docs/08-game-client/GODOT_ARCHITECTURE.md`, `CHANGELOG.md`
- [ ] T037 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational → US1 và US4 (song song, P1) → US2 → US3 → US5 → Polish.
- US1 party cần F102 (`PartyStore`) và delta F101/F103; US2/US3 cần `QuestTracker` (F072); US4 độc lập với boss. Trước khi F095/F103 Done chạy trên Prism mock (profile `mock`, nhãn MOCK, không vào build phát hành — R17).

## Definition of Done

- [ ] Mọi tiêu chí F110-AC-* có test pass (ghi mã test/TC)
- [ ] `make godot-test`, `make godot-export-check`, `make design-tokens-check` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
