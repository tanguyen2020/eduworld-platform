# Tasks: F109 — Giao diện Arena, trận đấu, bảng xếp hạng, giải đấu trong Godot

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 [P] Khóa dịch `ui.arena.*`, `ui.leaderboard.*`, `ui.tournament.*`, `ui.error.<CODE>` (`MM_TICKET_ALREADY_ACTIVE`, `ANSWER_LATE`, `ANSWER_ALREADY_SUBMITTED`, `MATCH_NOT_ACTIVE`, `DUEL_EXPIRED`, `SOCIAL_SCOPE_VIOLATION`, `TOURNAMENT_*`) — file: `godot/locale/vi.po`, `godot/locale/en.po`
- [ ] T002 [P] Sinh fixture từ example OpenAPI/AsyncAPI (ticket, proposal, `battle.*` đủ trạng thái, `Match`, `Rating`, `LeaderboardPage` 4 `display_mode`, `Tournament`, bracket, `Duel`) — file: `godot/tests/fixtures/competition/`, `tools/godot/gen_fixtures.gd`
- [ ] T003 [P] Khung scene `scenes/competition/` + đăng ký route `arena` trong `SceneRouter` và intent NPC/phím `K` — file: `godot/scenes/competition/ArenaScreen.tscn`, `godot/scripts/core/scene_router.gd`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta contract (owner F105): `listArenaModes` + `ArenaMode`; AsyncAPI `speed_bonus_enabled`, `primary_user_ids`, `encouragement` — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/competition.v1.yaml`, `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`
- [ ] T005 Delta contract (owner F106): sửa `Rating`, bỏ `Match.participants[].rating_delta` khỏi API học sinh; `listLeaderboards`; sửa `LeaderboardPage` — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/competition.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`
- [ ] T006 Delta contract (owner F107): `getTournament`, `getTournamentBracket`, `createTournamentEntry`, `withdrawTournamentEntry`, `checkInTournamentEntry`, schema `TournamentDetail`/`TournamentBracket`, enum `Tournament`, `ProblemCode` `TOURNAMENT_*` — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/competition.v1.yaml` — kiểm: `make specs-lint`
- [ ] T007 Cập nhật GAME_NETWORKING §7 (bỏ "rating delta" khỏi dữ liệu hiển thị) qua owner tài liệu — file: `docs/08-game-client/GAME_NETWORKING.md` — kiểm: `make docs-check`
- [ ] T008 Model typed + parser (bỏ `rating`, `rd`, `rating_delta`; kiểm kiểu, `contract_violation`) — file: `godot/scripts/competition/models/*.gd`
- [ ] T009 [P] Contract test parse mọi fixture, quét không có trường rating thô/tên thật/`correct_response` trước reveal (F109-AC-15, F109-AC-20) — file: `godot/tests/contract/competition/test_competition_fixtures.gd`
- [ ] T010 [P] Prism mock profile + fake `RealtimeClient` phát fixture AsyncAPI, gắn nhãn MOCK — file: `godot/tests/fakes/fake_realtime_client.gd`, `godot/config/mock.json`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Chọn chế độ, tìm trận, ready check (Priority: P1) 🎯 MVP

**Goal**: Học sinh vào hàng chờ và vào trận qua ready check.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T011 [P] [US1] Scene test F109-AC-01: thẻ chế độ chỉ theo `listArenaModes`, khóa có lý do, `TierBadge` từ `getMyRatings` (TC-CMP-001) — file: `godot/tests/scene/competition/test_arena_screen.gd` — phải FAIL trước khi code
- [ ] T012 [P] [US1] Unit test F109-AC-02: `createMatchmakingTicket` + `Idempotency-Key`, `QueueStatusBar` theo `matchmaking.ticket.updated`, hủy qua WS rồi fallback REST (TC-MM-001) — file: `godot/tests/unit/competition/test_matchmaking_service.gd`
- [ ] T013 [P] [US1] Unit test F109-AC-03: đếm theo `deadline_at`/`Clock`, accept WS + fallback, `requeued`, `match.assigned` → `BattleArena` (TC-CMP-002) — file: `godot/tests/unit/competition/test_ready_check.gd`
- [ ] T014 [P] [US1] Unit test F109-AC-04: `409 MM_TICKET_ALREADY_ACTIVE` → `getMatchmakingTicket`; timeout → gợi ý chế độ khác/Robot; `403` → `ui.error.<CODE>` — file: `godot/tests/unit/competition/test_matchmaking_errors.gd`

### Implementation cho User Story 1

- [ ] T015 [US1] `MatchmakingService` (modes, ticket, proposal) — file: `godot/scripts/multiplayer/matchmaking_service.gd`
- [ ] T016 [US1] `ArenaScreen`, `ModeCard`, `SubjectTopicPicker`, `TierBadge`, `QueueStatusBar`, `ReadyCheck` — file: `godot/scenes/competition/ArenaScreen.tscn`, `components/*.tscn`, `ReadyCheck.tscn`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Chơi trận theo server (Priority: P1)

### Tests cho User Story 2

- [ ] T017 [P] [US2] Scene test F109-AC-05, F109-AC-06: render qua `QuestionRendererRegistry`, một `battle.answer.submit`, khóa input, không đúng/sai trước reveal, không gửi điểm (TC-CMP-003, TC-CMP-007) — file: `godot/tests/scene/competition/test_battle_arena.gd` — phải FAIL trước khi code
- [ ] T018 [P] [US2] Unit test F109-AC-07: `ANSWER_LATE`/`ANSWER_ALREADY_SUBMITTED`/`MATCH_NOT_ACTIVE` giữ khóa, không gửi lại — file: `godot/tests/unit/competition/test_battle_client_errors.gd`
- [ ] T019 [P] [US2] Scene test F109-AC-08, F109-AC-09: ✓/✗, dòng thưởng tốc độ chỉ khi `time_bonus > 0`; `speed_bonus_enabled = false` không chỉ dẫn tốc độ; `g1_3` không số giây, ô ≥ 64 dp — file: `godot/tests/scene/competition/test_battle_grade_band.gd`
- [ ] T020 [P] [US2] Scene test F109-AC-10: `team_battle` điểm theo đội, lượt chính theo `primary_user_ids` — file: `godot/tests/scene/competition/test_team_battle.gd`
- [ ] T021 [P] [US2] Unit + integration F109-AC-11: `status_changed`, `paused`/`resumed`, reconnect → `battle.state.request` → dựng từ snapshot; bỏ `state_version` cũ (TC-CMP-004, TC-CMP-005) — file: `godot/tests/unit/competition/test_battle_client_resync.gd`, `godot/tests/integration/competition/test_battle_reconnect.gd`
- [ ] T022 [P] [US2] Benchmark headless F109-AC-12: `suspend_world("battle")`, ngân sách cảnh BattleArena (TC-PERF-018) — file: `godot/tests/perf/test_battle_arena_budget.gd`
- [ ] T023 [P] [US2] Scene test F109-AC-13: slot `MatchChatStrip` luôn có, thu nhỏ khi câu hiển thị, mobile không che đáp án, không có nút tắt — file: `godot/tests/scene/competition/test_match_chat_slot.gd`

### Implementation cho User Story 2

- [ ] T024 [US2] `BattleClient` state machine hiển thị, `state_version`, resync — file: `godot/scripts/multiplayer/battle_client.gd`
- [ ] T025 [US2] `BattleArena`, `MatchHeader`, `AnswerRevealOverlay`, `ReconnectOverlay`, slot chat — file: `godot/scenes/competition/BattleArena.tscn`, `components/MatchHeader.tscn`, `components/AnswerRevealOverlay.tscn`, `components/ReconnectOverlay.tscn`

## Phase 5: User Story 3 — Kết quả, tier, lịch sử (Priority: P1)

### Tests cho User Story 3

- [ ] T026 [P] [US3] Scene test F109-AC-14: kết cục động viên, points, tier sau `getMyRatings`, `cancelled`/`aborted` (TC-CMP-006) — file: `godot/tests/scene/competition/test_match_result.gd` — phải FAIL trước khi code
- [ ] T027 [P] [US3] Scene test F109-AC-15: quét cây node mọi màn competition không có số rating/RD/delta, tên tier từ server — file: `godot/tests/scene/competition/test_no_raw_rating.gd`
- [ ] T028 [P] [US3] Unit test F109-AC-16: `createReport` (`note` ≤ `maxLength`, bộ đếm, `Idempotency-Key`), `blockUser`, hiển thị `encouragement` (TC-TEN-024) — file: `godot/tests/unit/competition/test_match_history_service.gd`
- [ ] T029 [P] [US3] Scene test F109-AC-17: lịch sử cursor + lọc `mode`, chi tiết chỉ nickname/avatar/tier — file: `godot/tests/scene/competition/test_match_history.gd`

### Implementation cho User Story 3

- [ ] T030 [US3] `MatchHistoryService`, `MatchResult`, `MatchHistory`, `MatchDetail` — file: `godot/scripts/competition/match_history_service.gd`, `godot/scenes/competition/MatchResult.tscn`, `MatchHistory.tscn`, `MatchDetail.tscn`

## Phase 6: User Story 4 — Bảng xếp hạng (Priority: P1)

### Tests cho User Story 4

- [ ] T031 [P] [US4] Unit test F109-AC-18: chỉ bảng `listLeaderboards` trả, rỗng → trạng thái rỗng — file: `godot/tests/unit/competition/test_leaderboard_service.gd` — phải FAIL trước khi code
- [ ] T032 [P] [US4] Scene test F109-AC-19: 4 `display_mode`; `class_goal` không có thứ hạng cá nhân; client không cắt/tính hạng — file: `godot/tests/scene/competition/test_leaderboard_view.gd`
- [ ] T033 [P] [US4] Scene test F109-AC-20: dòng chỉ nickname, avatar, lớp (scope lớp) — file: `godot/tests/scene/competition/test_leaderboard_privacy.gd`
- [ ] T034 [P] [US4] Unit test F109-AC-21: `404` → "Bảng này hiện không mở" + bỏ tab, `stale`, chỉ gửi id server cấp (TC-TEN-001) — file: `godot/tests/unit/competition/test_leaderboard_errors.gd`
- [ ] T035 [P] [US4] Unit test F109-AC-22: không timer làm mới, không xử lý thông báo "bị vượt qua" — file: `godot/tests/unit/competition/test_leaderboard_no_pressure.gd`

### Implementation cho User Story 4

- [ ] T036 [US4] `LeaderboardService` + `Leaderboard`, `RankedList`, `ClassGoalBar`, `PercentileCard` — file: `godot/scripts/competition/leaderboard_service.gd`, `godot/scenes/competition/Leaderboard.tscn`, `leaderboard/*.tscn`

## Phase 7: User Story 5 — Giải đấu (Priority: P2)

### Tests cho User Story 5

- [ ] T037 [P] [US5] Unit test F109-AC-23, F109-AC-24: list/detail, đăng ký/rút/check-in + `Idempotency-Key`, nút check-in theo cửa sổ server, lỗi `TOURNAMENT_*` — file: `godot/tests/unit/competition/test_tournament_service.gd` — phải FAIL trước khi code
- [ ] T038 [P] [US5] Scene test F109-AC-25: cây cho `g6_7`/`g8_9`, "con đường" cho `g1_3`/`g4_5`, chỉ nickname/tên đội — file: `godot/tests/scene/competition/test_bracket_views.gd`
- [ ] T039 [P] [US5] Unit test F109-AC-26: `notification.item.created` `tournament` → toast; ready check như US1 (TC-CMP-002) — file: `godot/tests/unit/competition/test_tournament_notification.gd`

### Implementation cho User Story 5

- [ ] T040 [US5] `TournamentService` + `TournamentList`, `TournamentDetail`, `BracketTree`, `BracketPath` — file: `godot/scripts/competition/tournament_service.gd`, `godot/scenes/competition/tournament/*.tscn`

## Phase 8: User Story 6 — Thách đấu bạn (Priority: P3)

- [ ] T041 [P] [US6] Unit test F109-AC-27: `createDuel`/`acceptDuel`/`declineDuel`, `410 DUEL_EXPIRED`, `SOCIAL_SCOPE_VIOLATION` thông điệp chung (TC-CMP-010) — file: `godot/tests/unit/competition/test_duel.gd` — phải FAIL trước khi code
- [ ] T042 [US6] Duel trong `MatchmakingService` + `DuelInviteDialog` — file: `godot/scripts/multiplayer/matchmaking_service.gd`, `godot/scenes/competition/DuelInviteDialog.tscn`

## Phase 9: User Story 7 — Bảo mật, tenant, song ngữ (Priority: P1)

- [ ] T043 [P] [US7] Unit test F109-AC-28: mọi service gửi `X-Tenant-Id`, `Idempotency-Key` cho ghi và giữ khi retry; không socket nào ngoài `RealtimeClient` (TC-TEN-001) — file: `godot/tests/unit/competition/test_request_headers.gd` — phải FAIL trước khi code
- [ ] T044 [P] [US7] Integration F109-AC-29: `match_id`/`tournament_id` của học sinh tenant khác → 404 "Không tìm thấy"; token nhân sự → `GAME_ACCESS_STUDENT_ONLY` (TC-AUTH-014) — file: `godot/tests/integration/competition/test_competition_authz.gd`
- [ ] T045 [P] [US7] CI F109-AC-30: `extract_pot` đủ khóa, pseudo-locale chụp màn Arena/Battle/Result/Leaderboard/Tournament; telemetry không PII (TC-I18N-005, TC-I18N-006) — file: `godot/tests/visual/test_competition_pseudo_locale.gd`

## Phase N: Polish & Cross-Cutting

- [ ] T046 [P] Lint theme override, token game (TC-UI-001, TC-UI-003) — kiểm: `make design-tokens-check`, `make godot-test`
- [ ] T047 [P] Cập nhật CHANGELOG — file: `CHANGELOG.md`
- [ ] T048 Chạy [quickstart.md](quickstart.md) với backend thật và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational (T004–T006 do owner F105–F107; trước khi merge, dùng Prism mock T010) → US1 → US2 → US3 → US4 (song song US3) → US7 → US5 → US6 → Polish.
- US2 cần F063 (`QuestionRendererRegistry`), F092 (`RealtimeClient`); US2 slot chat cần F119; US6 cần F095.

## Definition of Done

- [ ] Mọi tiêu chí F109-AC-* có test pass (ghi mã test/TC)
- [ ] `make godot-test` xanh; integration hai client với backend thật
- [ ] Constitution Check trong plan.md vẫn đạt; Prism mock chỉ trong profile `mock`/test (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
