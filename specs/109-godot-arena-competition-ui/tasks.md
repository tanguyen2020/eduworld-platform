# Tasks: F109 — Giao diện Arena, trận đấu, bảng xếp hạng, giải đấu trong Godot

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 [P] Khóa dịch `ui.arena.*`, `ui.leaderboard.*`, `ui.tournament.*`, `ui.error.<CODE>` (`MM_TICKET_ALREADY_ACTIVE`, `ANSWER_LATE`, `ANSWER_ALREADY_SUBMITTED`, `MATCH_NOT_ACTIVE`, `DUEL_EXPIRED`, `SOCIAL_SCOPE_VIOLATION`, `TOURNAMENT_*`) — file: `src/game/locale/vi.po`, `src/game/locale/en.po` — `src/game/locale/{vi,en}.po` +204 khóa `ui.arena.*`, `ui.leaderboard.*`, `ui.tournament.*` (lỗi theo mã trong `CompetitionText`), `extract_pot --check` OK
- [x] T002 [P] Sinh fixture từ example OpenAPI/AsyncAPI (ticket, proposal, `battle.*` đủ trạng thái, `Match`, `Rating`, `LeaderboardPage` 4 `display_mode`, `Tournament`, bracket, `Duel`) — file: `src/game/tests/fixtures/competition/`, `src/game/tools/gen_fixtures.gd` — example OpenAPI (named examples theo `visibility`) + AsyncAPI trong spec, `gen_fixtures.mjs` sinh 58 fixture F109 (`openapi/*`, `asyncapi/matchmaking.*|battle.*|competition.*|tournament.*`), `gen_fixtures --check` OK
- [x] T003 [P] Khung scene `scenes/competition/` + đăng ký route `arena` trong `SceneRouter` và intent NPC/phím `K` — file: `src/game/scenes/competition/ArenaScreen.tscn`, `src/game/scripts/core/scene_router.gd` — `scenes/competition/ArenaScreen.tscn`, `LeaderboardScreen.tscn`, `BattleArena.tscn`; `UiScreenRegistry` `arena`/`leaderboard`/`battle`; phím `K` (`open_arena`), HUD "Thêm" → Đấu trường / Bảng xếp hạng, NPC `open_ui: arena`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta contract (owner F105): `listArenaModes` + `ArenaMode`; AsyncAPI `speed_bonus_enabled`, `primary_user_ids`, `encouragement` — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/competition.v1.yaml`, `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking` — contract đã có trên main dưới tên khác: `listCompetitionModes` (`CompetitionMode` — chỉ chế độ khả dụng, không `locked_reason_code`); `speed_bonus_enabled`, `primary_user_ids`, `encouragement` **chưa có** trong AsyncAPI → client đọc khi có, mặc định an toàn (không gợi ý tốc độ, không nhãn lượt) — câu hỏi mở cho owner F105 (quickstart §5.3)
- [x] T005 Delta contract (owner F106): sửa `Rating`, bỏ `Match.participants[].rating_delta` khỏi API học sinh; `listLeaderboards`; sửa `LeaderboardPage` — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/competition.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking` — đã có (F106): `Rating` chỉ tier (`tier_code`, `tier_name`, `provisional`), `listLeaderboards`, `LeaderboardPage.visibility` (`grade_goal`/`top_n_percentile`/`ranked` thay `display_mode`), `grade_goal`, `stale`; `Match.participants[]` không có `rating_delta`
- [x] T006 Delta contract (owner F107): `getTournament`, `getTournamentBracket`, `createTournamentEntry`, `withdrawTournamentEntry`, `checkInTournamentEntry`, schema `TournamentDetail`/`TournamentBracket`, enum `Tournament`, `ProblemCode` `TOURNAMENT_*` — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/openapi/modules/competition.v1.yaml` — kiểm: `make specs-lint` — đã có (F107) dưới tên `registerTournamentEntry`; `getTournament`, `getTournamentBracket`, `withdrawTournamentEntry`, `checkInTournamentEntry` có; **không** có `ProblemCode` `TOURNAMENT_*` (server trả 404/403 `POLICY_DENIED`/409 `CONFLICT`/422 `INVALID_STATE_TRANSITION`) → client ánh xạ theo mã đó (`CompetitionText.TOURNAMENT_ERROR_KEYS`) — câu hỏi mở
- [x] T007 Cập nhật GAME_NETWORKING §7 (bỏ "rating delta" khỏi dữ liệu hiển thị) qua owner tài liệu — file: `docs/08-game-client/GAME_NETWORKING.md` — kiểm: `make docs-check` — GAME_NETWORKING §7 đã ghi "Rating và `rating_delta` không hiển thị cho học sinh" — không cần sửa
- [x] T008 Model typed + parser (bỏ `rating`, `rd`, `rating_delta`; kiểm kiểu, `contract_violation`) — file: `src/game/scripts/competition/models/*.gd` — `scripts/competition/models/*.gd` (16 model), bỏ `rating`/`rd`/`rating_delta`, khóa đáp án bị lọc (`QuestionRenderModel`)
- [x] T009 [P] Contract test parse mọi fixture, quét không có trường rating thô/tên thật/`correct_response` trước reveal (F109-AC-15, F109-AC-20) — file: `src/game/tests/contract/competition/test_competition_fixtures.gd` — `tests/contract/competition/test_competition_fixtures.gd` 7/7
- [x] T010 [P] Prism mock profile + fake `RealtimeClient` phát fixture AsyncAPI, gắn nhãn MOCK — file: `src/game/tests/fakes/fake_realtime_client.gd`, `src/game/config/mock.json` — `CompetitionSceneHarness` (RealtimeHarness online + FakeHttp, TEST-ONLY), `CompetitionFixtures`, `CompetitionScreens`; không cần Prism (fixture sinh từ contract)

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Chọn chế độ, tìm trận, ready check (Priority: P1) 🎯 MVP

**Goal**: Học sinh vào hàng chờ và vào trận qua ready check.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T011 [P] [US1] Scene test F109-AC-01: thẻ chế độ chỉ theo `listArenaModes`, khóa có lý do, `TierBadge` từ `getMyRatings` (TC-CMP-001) — file: `src/game/tests/scene/competition/test_arena_screen.gd` — phải FAIL trước khi code — `tests/scene/competition/test_arena_screen.gd` (thẻ theo server, rỗng, tier bằng chữ)
- [x] T012 [P] [US1] Unit test F109-AC-02: `createMatchmakingTicket` + `Idempotency-Key`, `QueueStatusBar` theo `matchmaking.ticket.updated`, hủy qua WS rồi fallback REST (TC-MM-001) — file: `src/game/tests/unit/competition/test_matchmaking_service.gd` — `tests/unit/competition/test_matchmaking_service.gd` (key + `X-Tenant-Id`, ticket WS, hủy WS rồi REST)
- [x] T013 [P] [US1] Unit test F109-AC-03: đếm theo `deadline_at`/`Clock`, accept WS + fallback, `requeued`, `match.assigned` → `BattleArena` (TC-CMP-002) — file: `src/game/tests/unit/competition/test_ready_check.gd` — `test_matchmaking_service.gd::test_ready_check_accept_requeue_and_assignment`, `::test_accept_falls_back_to_rest…`, `test_arena_screen.gd::test_ready_check_shows_players_and_the_server_clock`
- [x] T014 [P] [US1] Unit test F109-AC-04: `409 MM_TICKET_ALREADY_ACTIVE` → `getMatchmakingTicket`; timeout → gợi ý chế độ khác/Robot; `403` → `ui.error.<CODE>` — file: `src/game/tests/unit/competition/test_matchmaking_errors.gd` — `test_matchmaking_service.gd` (409 → `getMatchmakingTicket`, timeout, 403 → câu thân thiện, mất ticket), `test_arena_screen.gd` (gợi ý chế độ khác / Robot)

### Implementation cho User Story 1

- [x] T015 [US1] `MatchmakingService` (modes, ticket, proposal) — file: `src/game/scripts/multiplayer/matchmaking_service.gd` — `scripts/multiplayer/matchmaking_service.gd`
- [x] T016 [US1] `ArenaScreen`, `ModeCard`, `SubjectTopicPicker`, `TierBadge`, `QueueStatusBar`, `ReadyCheck` — file: `src/game/scenes/competition/ArenaScreen.tscn`, `components/*.tscn`, `ReadyCheck.tscn` — `arena_screen.gd`, `components/{mode_card,subject_topic_picker,tier_badge,queue_status_bar,ready_check_dialog}.gd`, `arena_hud.gd`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Chơi trận theo server (Priority: P1)

### Tests cho User Story 2

- [x] T017 [P] [US2] Scene test F109-AC-05, F109-AC-06: render qua `QuestionRendererRegistry`, một `battle.answer.submit`, khóa input, không đúng/sai trước reveal, không gửi điểm (TC-CMP-003, TC-CMP-007) — file: `src/game/tests/scene/competition/test_battle_arena.gd` — phải FAIL trước khi code — `tests/scene/competition/test_battle_arena.gd` (header, renderer, một submit, khóa, chưa có đúng/sai)
- [x] T018 [P] [US2] Unit test F109-AC-07: `ANSWER_LATE`/`ANSWER_ALREADY_SUBMITTED`/`MATCH_NOT_ACTIVE` giữ khóa, không gửi lại — file: `src/game/tests/unit/competition/test_battle_client_errors.gd` — `tests/unit/competition/test_battle_client.gd::test_refusals_keep_the_lock_and_are_not_resent`
- [x] T019 [P] [US2] Scene test F109-AC-08, F109-AC-09: ✓/✗, dòng thưởng tốc độ chỉ khi `time_bonus > 0`; `speed_bonus_enabled = false` không chỉ dẫn tốc độ; `g1_3` không số giây, ô ≥ 64 dp — file: `src/game/tests/scene/competition/test_battle_grade_band.gd` — `test_battle_arena.gd::test_verdict_in_words…`, `::test_no_speed_hint…`, `::test_grade_1_3_has_no_seconds_and_big_tiles`
- [x] T020 [P] [US2] Scene test F109-AC-10: `team_battle` điểm theo đội, lượt chính theo `primary_user_ids` — file: `src/game/tests/scene/competition/test_team_battle.gd` — `test_battle_arena.gd::test_team_battle_scores_and_primary_marker`, `test_battle_client.gd::test_speed_bonus_and_primary_flags…`
- [x] T021 [P] [US2] Unit + integration F109-AC-11: `status_changed`, `paused`/`resumed`, reconnect → `battle.state.request` → dựng từ snapshot; bỏ `state_version` cũ (TC-CMP-004, TC-CMP-005) — file: `src/game/tests/unit/competition/test_battle_client_resync.gd`, `src/game/tests/integration/competition/test_battle_reconnect.gd` — `test_battle_client.gd` (snapshot cũ bị bỏ, kết nối lại → `battle.state.request`, pause/resume replay); integration `tests/integration/competition/test_arena_flow_headless.gd` **bỏ qua khi không có backend — chờ chạy thật**
- [x] T022 [P] [US2] Benchmark headless F109-AC-12: `suspend_world("battle")`, ngân sách cảnh BattleArena (TC-PERF-018) — file: `src/game/tests/perf/test_battle_arena_budget.gd` — `tests/perf/test_battle_arena_budget.gd` 1/1 (0 node 3D/SubViewport, thế giới 3D tạm dừng, không gọi REST trong 120 khung hình, cập nhật ≤ 3,15 ms); FPS thiết bị tầm trung **chờ đo lab**
- [x] T023 [P] [US2] Scene test F109-AC-13: slot `MatchChatStrip` luôn có, thu nhỏ khi câu hiển thị, mobile không che đáp án, không có nút tắt — file: `src/game/tests/scene/competition/test_match_chat_slot.gd` — `test_battle_arena.gd::test_match_chat_is_always_there_and_shrinks_during_a_question`, ma trận `space_battle_*` (≥ 3 tin)

### Implementation cho User Story 2

- [x] T024 [US2] `BattleClient` state machine hiển thị, `state_version`, resync — file: `src/game/scripts/multiplayer/battle_client.gd` — `scripts/multiplayer/battle_client.gd`, `models/battle_view.gd`
- [x] T025 [US2] `BattleArena`, `MatchHeader`, `AnswerRevealOverlay`, `ReconnectOverlay`, slot chat — file: `src/game/scenes/competition/BattleArena.tscn`, `components/MatchHeader.tscn`, `components/AnswerRevealOverlay.tscn`, `components/ReconnectOverlay.tscn` — `battle_arena.gd`, `components/{match_header,answer_reveal_overlay}.gd` (kết nối lại = băng vàng + lớp phủ tạm dừng trong `battle_arena.gd`)

## Phase 5: User Story 3 — Kết quả, tier, lịch sử (Priority: P1)

### Tests cho User Story 3

- [x] T026 [P] [US3] Scene test F109-AC-14: kết cục động viên, points, tier sau `getMyRatings`, `cancelled`/`aborted` (TC-CMP-006) — file: `src/game/tests/scene/competition/test_match_result.gd` — phải FAIL trước khi code — `test_battle_arena.gd::test_result_win…`, `::test_result_loss_is_encouraging_and_cancelled_says_not_counted`
- [x] T027 [P] [US3] Scene test F109-AC-15: quét cây node mọi màn competition không có số rating/RD/delta, tên tier từ server — file: `src/game/tests/scene/competition/test_no_raw_rating.gd` — `test_battle_arena.gd::test_result_win…` (không số rating), contract test quét fixture + model
- [x] T028 [P] [US3] Unit test F109-AC-16: `createReport` (`note` ≤ `maxLength`, bộ đếm, `Idempotency-Key`), `blockUser`, hiển thị `encouragement` (TC-TEN-024) — file: `src/game/tests/unit/competition/test_match_history_service.gd` — `test_competition_services.gd::test_report_and_block_an_opponent`, `test_battle_arena.gd::test_report_from_the_result_uses_the_f119_sheet…`; ô ghi chú (`note`) không có — bảng báo cáo F119 không gửi chữ tự do (câu hỏi mở)
- [x] T029 [P] [US3] Scene test F109-AC-17: lịch sử cursor + lọc `mode`, chi tiết chỉ nickname/avatar/tier — file: `src/game/tests/scene/competition/test_match_history.gd` — `tests/scene/competition/test_arena_tabs.gd::test_history_cards_filter_and_detail`, `::test_history_detail_404…`

### Implementation cho User Story 3

- [x] T030 [US3] `MatchHistoryService`, `MatchResult`, `MatchHistory`, `MatchDetail` — file: `src/game/scripts/competition/match_history_service.gd`, `src/game/scenes/competition/MatchResult.tscn`, `MatchHistory.tscn`, `MatchDetail.tscn` — `scripts/competition/match_history_service.gd`, `match_result_view.gd`, `match_history_view.gd` (lịch sử + chi tiết trong tab)

## Phase 6: User Story 4 — Bảng xếp hạng (Priority: P1)

### Tests cho User Story 4

- [x] T031 [P] [US4] Unit test F109-AC-18: chỉ bảng `listLeaderboards` trả, rỗng → trạng thái rỗng — file: `src/game/tests/unit/competition/test_leaderboard_service.gd` — phải FAIL trước khi code — `test_competition_services.gd::test_boards_are_the_server_list…`, `::test_empty_list_and_a_closed_board`
- [x] T032 [P] [US4] Scene test F109-AC-19: 4 `display_mode`; `grade_goal` (`GradeGoalBar`) không thứ hạng cá nhân, không nickname người khác, `contributors_count` ẩn → không hiện ô; client không cắt/tính hạng — file: `src/game/tests/scene/competition/test_leaderboard_view.gd` — `test_arena_tabs.gd::test_grade_goal_board…`, `::test_grade_goal_helpers_hidden…`, `::test_top_n_percentile…`, `::test_ranked_rows…`
- [x] T033 [P] [US4] Scene test F109-AC-20: dòng chỉ nickname, avatar (không tên lớp, không tên thật) — file: `src/game/tests/scene/competition/test_leaderboard_privacy.gd` — `test_arena_tabs.gd::test_ranked_rows_show_nickname_and_avatar_only_and_stale_tag`
- [x] T034 [P] [US4] Unit test F109-AC-21: `404` → "Bảng này hiện không mở" + bỏ tab, `stale`, chỉ gửi id server cấp (TC-TEN-001) — file: `src/game/tests/unit/competition/test_leaderboard_errors.gd` — `test_arena_tabs.gd::test_a_closed_board_and_an_empty_list`, `test_competition_services.gd` (chỉ id server)
- [x] T035 [P] [US4] Unit test F109-AC-22: không timer làm mới, không xử lý thông báo "bị vượt qua" — file: `src/game/tests/unit/competition/test_leaderboard_no_pressure.gd` — `test_competition_services.gd::test_no_refresh_timer_and_no_overtaken_message_in_the_arena_code`

### Implementation cho User Story 4

- [x] T036 [US4] `LeaderboardService` + `Leaderboard`, `RankedList`, `GradeGoalBar` (thay `ClassGoalBar` — ADR-0095), `PercentileCard` — file: `src/game/scripts/competition/leaderboard_service.gd`, `src/game/scenes/competition/Leaderboard.tscn`, `leaderboard/*.tscn` — `scripts/competition/leaderboard_service.gd`, `leaderboard_view.gd` (dòng xếp hạng, thẻ nhóm %), `components/grade_goal_bar.gd`

## Phase 7: User Story 5 — Giải đấu (Priority: P2)

### Tests cho User Story 5

- [x] T037 [P] [US5] Unit test F109-AC-23, F109-AC-24: list/detail, đăng ký/rút/check-in + `Idempotency-Key`, nút check-in theo cửa sổ server, lỗi `TOURNAMENT_*` — file: `src/game/tests/unit/competition/test_tournament_service.gd` — phải FAIL trước khi code — `test_competition_services.gd` (list/detail, đăng ký có key, cửa sổ check-in, lỗi theo mã)
- [x] T038 [P] [US5] Scene test F109-AC-25: cây cho `g6_7`/`g8_9`, "con đường" cho `g1_3`/`g4_5`, chỉ nickname/tên đội — file: `src/game/tests/scene/competition/test_bracket_views.gd` — `test_arena_tabs.gd::test_bracket_tree_for_grade_6_9_and_path_for_grade_1_5`
- [x] T039 [P] [US5] Unit test F109-AC-26: `notification.item.created` `tournament` → toast; ready check như US1 (TC-CMP-002) — file: `src/game/tests/unit/competition/test_tournament_notification.gd` — `test_competition_services.gd::test_match_notice_raises_the_toast_and_notification_opens_the_tab`

### Implementation cho User Story 5

- [x] T040 [US5] `TournamentService` + `TournamentList`, `TournamentDetail`, `BracketTree`, `BracketPath` — file: `src/game/scripts/competition/tournament_service.gd`, `src/game/scenes/competition/tournament/*.tscn` — `scripts/competition/tournament_service.gd`, `tournament_view.gd`, `components/{bracket_tree,bracket_path}.gd`

## Phase 8: User Story 6 — Thách đấu bạn (Priority: P3)

- [x] T041 [P] [US6] Unit test F109-AC-27: `createDuel`/`acceptDuel`/`declineDuel`, `410 DUEL_EXPIRED`, `SOCIAL_SCOPE_VIOLATION` thông điệp chung (TC-CMP-010) — file: `src/game/tests/unit/competition/test_duel.gd` — phải FAIL trước khi code — `test_matchmaking_service.gd::test_duel_create_accept_expired_and_scope`, `test_arena_screen.gd::test_duel_button…`, `::test_incoming_duel_expired_says_so`
- [x] T042 [US6] Duel trong `MatchmakingService` + `DuelInviteDialog` — file: `src/game/scripts/multiplayer/matchmaking_service.gd`, `src/game/scenes/competition/DuelInviteDialog.tscn` — duel trong `matchmaking_service.gd`, `duel_invite_dialog.gd` (gửi + nhận); đường giao lời thách đấu tới người nhận **chưa có trong contract** (câu hỏi mở)

## Phase 9: User Story 7 — Bảo mật, tenant, song ngữ (Priority: P1)

- [x] T043 [P] [US7] Unit test F109-AC-28: mọi service gửi `X-Tenant-Id`, `Idempotency-Key` cho ghi và giữ khi retry; không socket nào ngoài `RealtimeClient` (TC-TEN-001) — file: `src/game/tests/unit/competition/test_request_headers.gd` — phải FAIL trước khi code — `test_matchmaking_service.gd` (`X-Tenant-Id`, key giữ khi thử lại), `test_competition_services.gd` (header, key); realtime chỉ qua `RealtimeClient`
- [x] T044 [P] [US7] Integration F109-AC-29: `match_id`/`tournament_id` của học sinh tenant khác → 404 "Không tìm thấy"; token nhân sự → `GAME_ACCESS_STUDENT_ONLY` (TC-AUTH-014) — file: `src/game/tests/integration/competition/test_competition_authz.gd` — `test_competition_services.gd::test_detail_of_another_students_match_shows_nothing`, `test_arena_tabs.gd` (404 lịch sử); integration live **chờ chạy thật**
- [x] T045 [P] [US7] CI F109-AC-30: `extract_pot` đủ khóa, pseudo-locale chụp màn Arena/Battle/Result/Leaderboard/Tournament; telemetry không PII (TC-I18N-005, TC-I18N-006) — file: `src/game/tests/visual/test_competition_pseudo_locale.gd` — ma trận `test_arena_screens_fit_every_band_with_long_text` (pseudo +35%), `extract_pot --check`, `pseudo_locale --check`; không thêm loại telemetry mới (không PII)

## Phase N: Polish & Cross-Cutting

- [x] T046 [P] Lint theme override, token game (TC-UI-001, TC-UI-003) — kiểm: `make design-tokens-check`, `make godot-test` — `lint_theme_overrides` OK; không thêm token (icon mới qua `make design-tokens`)
- [x] T047 [P] Cập nhật CHANGELOG — file: `CHANGELOG.md` — `CHANGELOG.md`, `design/CHANGELOG.md`
- [ ] T048 Chạy [quickstart.md](quickstart.md) với backend thật và ghi bằng chứng — **chờ backend thật** (quickstart §5)

## Dependencies & Execution Order

- Setup → Foundational (T004–T006 do owner F105–F107; trước khi merge, dùng Prism mock T010) → US1 → US2 → US3 → US4 (song song US3) → US7 → US5 → US6 → Polish.
- US2 cần F063 (`QuestionRendererRegistry`), F092 (`RealtimeClient`); US2 slot chat cần F119; US6 cần F095.

## Definition of Done

- [x] Mọi tiêu chí F109-AC-* có test pass với fake + fixture contract (quickstart §5.1) — phần chạy thật còn chờ
- [ ] `make godot-test` xanh; integration hai client với backend thật — GdUnit toàn bộ 1387 test: 0 lỗi, chỉ ngân sách thời gian dao động khi máy bận (quickstart §5.1); integration hai client **chờ backend thật**
- [x] Constitution Check trong plan.md vẫn đạt; fake/fixture chỉ trong `tests/` (R17)
- [x] Tài liệu sở hữu, CHANGELOG cập nhật (CONFIG_DEFAULTS không đổi — không thêm tham số)
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật (In progress)
