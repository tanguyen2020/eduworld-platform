# Tasks: F104 — Matchmaking 8 chế độ: queue, ticket, proposal, ready check, ràng buộc khối

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Tạo package `matchmaking`, `matchmaking/duel`; đăng ký handler HTTP, NATS command `ew.cmd.competition.matchmaking.*`, job `matchmaker`, `ticket_expiry`, `duel_expiry` (profile `worker-jobs`) qua `Register(...)` — file: `src/backend/modules/competition/module.go` (nối vào `src/backend/cmd/core-api`, `src/backend/cmd/worker`) — **Xong 2026-11-21:** package `internal/domain/matchmaking` (duel trong cùng package), `command.Matchmaking`; `module.go`: route HTTP, lệnh NATS `matchmaking.ticket.cancel`/`proposal.accept`/`proposal.decline` (core-api), job `matchmaker`, `matchmaking-ticket-expiry`, `duel-expiry` (worker jobs), consumer `competition.matchmaking_party`; wiring `go test ./cmd/...` xanh
- [x] T002 [P] Biến môi trường `EW_MM_TICK`, ACL Valkey prefix `ew:competition:` — file: `src/backend/modules/competition/internal/infrastructure/config/config.go`, `deploy/compose/.env.example` — **Xong 2026-11-21:** `internal/infrastructure/config/matchmaking.go`: `EW_MM_TICK` (1 s), `EW_MM_LOCK_TTL` (5 s), `EW_MM_CONFIG_TTL` (30 s), owner = `EW_INSTANCE_ID`; prefix Valkey `ew:competition:mm:`; `.env.example` không sửa (deny-list) — biến liệt kê ở LOCAL_DEVELOPMENT và báo cáo

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Delta OpenAPI: sửa `MatchmakingTicketCreate`, `MatchmakingTicket`, `Match`; thêm `declineMatchProposal`, `startBotMatch`, `listCompetitionModes` + schema `CompetitionMode` — file: `specs/openapi/modules/competition.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=competition` — **Xong 2026-11-21:** eduworld-api.v1.yaml + split: `MatchmakingTicketCreate.allow_fill`, `MatchmakingTicket` (+type, topic, party, wait, search_widened, bot_offer_available, requeued, cancel_reason, proposal_id), `Match` (+matchmaking_type, ranked, is_bot), `declineMatchProposal`, `startBotMatch`, `listCompetitionModes` + `CompetitionMode`, 401/403/422/429/503; portal `pnpm gen:api` + `gen:check` xanh; contract `TestPartyOperationsMatchSpec` (28 operation)
- [x] T004 [P] Delta AsyncAPI `matchmaking.ticket.updated` (`bot_offer_available`, `matchmaking_type`) — file: `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint` — **Xong 2026-11-21:** AsyncAPI `matchmaking.ticket.updated`: `matchmaking_type`, `bot_offer_available`, `requeued`; `gen-routes --check` 0 lệch
- [x] T005 [P] Payload schema `match.created.v1`, `duel.sent/accepted/declined/expired.v1` — file: `specs/events/payloads/competition/` — kiểm: `make specs-lint` — **Xong 2026-11-21:** `specs/events/payloads/competition/match.created.v1`, `duel.sent/accepted/declined/expired.v1`; validate-schemas, check-maxlength 0 lỗi; outbox kiểm schema trong `TestMMProposalAccept`, `TestMMDuel`
- [x] T006 ent schema `matches`, `match_participants`, `duels`, `competition_restrictions`, `game_configs`, `game_config_versions`, `content_releases`, `ref.enrollments`, cột `ref.schools.settings_subset` (`ref.blocks` có sẵn từ F091) — mọi chuỗi `MaxLen` — file: `src/backend/modules/competition/ent/schema/` — kiểm: `make migrate-diff MOD=competition name=f104_matchmaking`, `make migrate-lint MOD=competition` — **Xong 2026-11-21:** `ent/schema/match.go` (Match G*, MatchParticipant, Duel, CompetitionRestriction, GameConfig, GameConfigVersion `version_no`, ContentRelease) — mọi chuỗi MaxLen; `TestMigrationMatchesEnt` xanh; ref.enrollments có sẵn (F100), `ref.schools` viết tay
- [x] T007 RLS, partial unique duel pending, trigger bất biến `game_config_versions` published — file: `src/backend/modules/competition/migrations/<ts>_f104_matchmaking.sql` — **Xong 2026-11-21:** `migrations/20261231000001_f104_matchmaking.sql`: `tenant_ids uuid[]`, CHECK, FK, RLS + FORCE, REVOKE DELETE/TRUNCATE, duel pending unique, trigger `game_config_versions_immutable`, `competition_app` không ghi cấu hình (R20), view `_live`, seed cấu hình; atlas.sum làm mới; `TestMatchmakingRLS`, `TestSeedMatchmakingConfig`
- [x] T008 [P] Port `ConfigReader` + repo ent + seed cấu hình matchmaking hiệu lực — file: `src/backend/modules/competition/internal/application/port/config.go`, `src/backend/modules/competition/internal/infrastructure/persistence/config_repo.go`, `src/backend/modules/competition/seed/matchmaking_config.json` — **Xong 2026-11-21:** port `write.MatchRepository.ActiveConfig` + `read.MatchmakingReader`, repo `persistence.Matchmaking`, `seed/matchmaking_config.json` (CONFIG_DEFAULTS), cache `command.ConfigCache`; `TestSeedMatchmakingConfig`, `TestMMDirectProposalAndModes` (version mới hiệu lực)
- [x] T009 [P] Port `RatingReader` + `DefaultRatingReader` (trả `glicko2_params`, F106 thay) — file: `src/backend/modules/competition/internal/application/port/rating.go` — **Xong 2026-11-21:** port `port.RatingReader`; mặc định = `default_rating` của cấu hình (1500/350, `glicko2_params`) khi `Ratings` nil (F106 thay); test dùng `fakeRatings` (TEST DOUBLE)
- [x] T010 [P] Consumer projection `ref.enrollments`, `ref.schools.settings_subset` (`ref.blocks` do consumer F091 ghi) — file: `src/backend/modules/competition/internal/presentation/consumer/ref_projection.go` — **Xong 2026-11-21:** consumer `competition.projection_identity` + `school.updated` → `ref.schools` (`Refs.School`, gộp settings_subset); `ref.enrollments`/`ref.blocks` có sẵn (F100/F091); contract `events_test` cập nhật
- [x] T011 [P] Cập nhật DATABASE_DESIGN §2.10/§2.16/§2.17, VALKEY_DESIGN §3.1, API_DESIGN §9.7 — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/07-backend/VALKEY_DESIGN.md`, `docs/07-backend/API_DESIGN.md` — **Xong 2026-11-21:** DATABASE_DESIGN §2.10/§2.17, VALKEY_DESIGN §3.1, API_DESIGN §9.7, MATCHMAKING (key, `cancel_reason = system`), CONFIG_DEFAULTS `mm_ticket_rate`, LOCAL_DEVELOPMENT; check-links/check-mermaid xanh

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Ghép 1v1 cùng khối theo rating (Priority: P1) 🎯 MVP

**Goal**: Ticket → queue → proposal → ready check → trận.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T012 [P] [US1] Unit test F104-AC-02: `QueueKey`/H1 cho lớp 1–9 (bảng đầy đủ), không bao giờ ghép lớp 1–5 khác khối, ±1 lớp 6–9 sau `grade_window_g6_9`; property test 10.000 cặp — file: `src/backend/modules/competition/tests/unit/matchmaking/constraints_test.go` — phải FAIL trước khi code — **Xong 2026-11-21:** `tests/unit/mm_constraints_test.go`: `TestGradeKeyTable`, `TestGradeCompatible`, `TestPairingNeverBreaksGradeConstraint` (10.000 cặp, 0 vi phạm H1), `TestCheckRequestTypes`, `TestCheckPlayers`, `TestRatingWindow`; mutation `grade <= 4` → FAIL (log f104-us1-mutant)
- [x] T013 [P] [US1] Unit test F104-AC-04: `Window(t)` theo `mm_rating_window`, cost (rating, RD, recent opponent; không còn `same_class` — ADR-0095), greedy chọn ticket cũ nhất — file: `src/backend/modules/competition/tests/unit/matchmaking/pairing_test.go` — **Xong 2026-11-21:** `tests/unit/mm_pairing_test.go`: `TestPairWithinWindow`, `TestPairOldestFirstLowestCost`, `TestPairSkipsBlocked`, `TestRandomIsFIFO`, `TestBucketedQueue` (1.200 ticket < 1 s)
- [x] T014 [P] [US1] Integration test F104-AC-01, F104-AC-03: ticket lấy khối/trường/rating từ server, bỏ qua field client; phạm vi trường mặc định, tenant khi bật liên trường — file: `src/backend/modules/competition/tests/integration/matchmaking/create_ticket_test.go` — **Xong 2026-11-21:** `tests/integration/mm_us1_test.go`: `TestMMCreateTicketServerSide`, `TestMMScope`, `TestSeedMatchmakingConfig`
- [x] T015 [P] [US1] Integration test F104-AC-05, F104-AC-06: proposal không lộ tên/trường; accept → `match_id = proposal_id`, `match.created`; decline/timeout → requeue giữ `enqueued_at`, cooldown sau 3 lần — file: `src/backend/modules/competition/tests/integration/matchmaking/proposal_test.go` — **Xong 2026-11-21:** `tests/integration/mm_us1_test.go`: `TestMMProposalAccept`, `TestMMDeclineTimeoutCooldown`
- [x] T016 [P] [US1] Integration test F104-AC-07, F104-AC-08: ticket thứ hai `409 MM_TICKET_ALREADY_ACTIVE` (kể cả đồng thời); block không cùng proposal; hạn chế → `403 POLICY_DENIED` — file: `src/backend/modules/competition/tests/integration/matchmaking/eligibility_test.go` — **Xong 2026-11-21:** `tests/integration/mm_us1_test.go`: `TestMMOneTicketPerStudent` (8 request đồng thời → 1×201, 7×409), `TestMMEligibilityAndBlocks` (mutation bỏ H6 → FAIL "blocked students proposed together")
- [x] T017 [P] [US1] Integration test F104-AC-09: 2 matchmaker song song, 200 ticket, không trùng trận — file: `src/backend/modules/competition/tests/integration/matchmaking/concurrency_test.go` — **Xong 2026-11-21:** `tests/integration/mm_us1_test.go`: `TestMMParallelMatchmakers` (2 matchmaker, 200 ticket → 100 proposal, 100 trận, 200 participant duy nhất)

### Implementation cho User Story 1

- [x] T018 [P] [US1] Domain `QueueKey`, `HardConstraints`, `Cost`, `Window`, `Pairing`, `Proposal` — file: `src/backend/modules/competition/internal/domain/matchmaking/constraints.go`, `src/backend/modules/competition/internal/domain/matchmaking/cost.go`, `src/backend/modules/competition/internal/domain/matchmaking/window.go`, `src/backend/modules/competition/internal/domain/matchmaking/pairing.go`, `src/backend/modules/competition/internal/domain/matchmaking/proposal.go` — **Xong 2026-11-21:** `internal/domain/matchmaking/{config,constraints,pairing,team,proposal}.go`
- [x] T019 [US1] Adapter Valkey `QueueStore`/`TicketStore`/`ProposalStore` (Lua nguyên tử, lock per-queue) — file: `src/backend/modules/competition/internal/infrastructure/cache/mm_store.go` — **Xong 2026-11-21:** `internal/infrastructure/cache/matchmaking.go`: Lua nguyên tử (create NX, end, propose, accept, cancel/requeue, complete, sliding window, blame), lock per-queue `SET NX PX`, key qua KEYS[]
- [x] T020 [US1] Command `CreateTicket`, `AcceptProposal`, `DeclineProposal`, `RunMatchmakerTick`, `CreateMatch` (tx + outbox + audit) — file: `src/backend/modules/competition/internal/application/command/matchmaking/` — **Xong 2026-11-21:** `command/matchmaking.go` (`CreateTicket`), `matchmaking_proposal.go` (`AcceptProposal`, `DeclineProposal`, `finalize`/`createMatch` tx + outbox `match.created` + audit), `matchmaking_tick.go` (`Tick`)
- [x] T021 [US1] Handler `createMatchmakingTicket`, `getMatchmakingTicket`, `acceptMatchProposal`, `declineMatchProposal`; NATS command handler; publisher `ew:rt:user:<id>` — file: `src/backend/modules/competition/internal/presentation/http/public/matchmaking_handler.go`, `src/backend/modules/competition/internal/presentation/realtime/matchmaking_cmd.go` — **Xong 2026-11-21:** `presentation/http/matchmaking.go` (10 operation), `presentation/natscmd/matchmaking.go`; publisher `ew:rt:user:<id>` (`realtime.PartyPublisher`)
- [x] T022 [US1] Job `matchmaker` theo `EW_MM_TICK` — file: `src/backend/modules/competition/internal/presentation/job/matchmaker.go` — **Xong 2026-11-21:** `presentation/job/jobs.go` `Matchmaker` theo `EW_MM_TICK`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Hủy, hết hạn, mất Valkey, bot luyện tập (Priority: P1)

**Goal**: Hủy/hết hạn, phục hồi khi mất Valkey, đề nghị bot.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [x] T023 [P] [US2] Integration test F104-AC-10: cancel REST + WS; hết `mm_ticket_ttl` → `expired` — file: `src/backend/modules/competition/tests/integration/matchmaking/cancel_expire_test.go` — phải FAIL trước khi code — **Xong 2026-11-21:** `tests/integration/mm_us2_test.go`: `TestMMCancelAndExpire` (REST + WS, 5 phút → expired, offline > 30 s → system)
- [x] T024 [P] [US2] Integration test F104-AC-11: `FLUSHALL` → `cancelled` (`system`), không trận ma trong PG — file: `src/backend/modules/competition/tests/integration/matchmaking/valkey_reset_test.go` — **Xong 2026-11-21:** `tests/integration/mm_us2_test.go`: `TestMMValkeyReset` (FLUSHALL → cancelled system cho 3 học sinh, accept proposal mất → 404, 0 trận)
- [x] T025 [P] [US2] Integration test F104-AC-12, F104-AC-13: `bot_offer_available` sau `mm_bot_after`; `startBotMatch` → trận bot unranked; vượt `mm_ticket_rate` → `429` + `Retry-After` — file: `src/backend/modules/competition/tests/integration/matchmaking/bot_ratelimit_test.go` — **Xong 2026-11-21:** `tests/integration/mm_us2_test.go`: `TestMMBotAndTicketRate`

### Implementation cho User Story 2

- [x] T026 [US2] Command `CancelTicket`, `ExpireTickets`, phát hiện ticket mất (đối chiếu `mm:user:*` với kết nối) — file: `src/backend/modules/competition/internal/application/command/matchmaking/cancel_ticket.go`, `src/backend/modules/competition/internal/application/command/matchmaking/expire_tickets.go`, `src/backend/modules/competition/internal/presentation/job/ticket_expiry.go` — **Xong 2026-11-21:** `command.Matchmaking.CancelTicket`, `ExpireTickets` (`matchmaking_tick.go`), `TicketTracker` phát hiện ticket mất, job `matchmaking-ticket-expiry`
- [x] T027 [US2] Command `StartBotMatch` + handler `startBotMatch`, `cancelMatchmakingTicket`; rate limit/cooldown — file: `src/backend/modules/competition/internal/application/command/matchmaking/start_bot_match.go`, `src/backend/modules/competition/internal/presentation/http/public/matchmaking_handler.go` — **Xong 2026-11-21:** `command/matchmaking_direct.go` `StartBotMatch`; handler `startBotMatch`, `cancelMatchmakingTicket`; rate/cooldown `TicketRate`, `Cooldown`, `Blame`

**Checkpoint**: User Story 2 chạy độc lập.

## Phase 5: User Story 3 — Ticket đội từ party (Priority: P1)

**Goal**: Ticket đội, chia đội, hủy khi party đổi.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [x] T028 [P] [US3] Integration test F104-AC-14, F104-AC-15: leader-only, member online, H1 mọi member, `μ_team`; member rời → `party_changed` tới mọi member — file: `src/backend/modules/competition/tests/integration/matchmaking/party_ticket_test.go` — phải FAIL trước khi code — **Xong 2026-11-21:** `tests/integration/mm_us3_test.go`: `TestMMPartyTicket`, `TestMMPartyChangedWhileProposed`, `TestMMAllowFill`
- [x] T029 [P] [US3] Unit test F104-AC-16, F104-AC-17: snake draft, `allow_fill`, party giữ đội; `team_battle` lớp < 4 và party khác khối ranked → `403 POLICY_DENIED` — file: `src/backend/modules/competition/tests/unit/matchmaking/team_draft_test.go` — **Xong 2026-11-21:** `tests/unit/mm_pairing_test.go` `TestTeamRating`, `TestTeamDraft` (snake A1 B1 B2 A2 A3 B3), `mm_constraints_test.go` `TestCheckPlayers`; integration `TestMMTeamPolicy`

### Implementation cho User Story 3

- [x] T030 [P] [US3] Domain `TeamDraft`, team rating — file: `src/backend/modules/competition/internal/domain/matchmaking/team.go` — **Xong 2026-11-21:** `internal/domain/matchmaking/team.go` (`pairTeams`, `Draft`), `constraints.go` (`TeamRating`, `TeamSize`)
- [x] T031 [US3] Nhánh party trong `CreateTicket`; consumer `party.member_left`/`party.disbanded` hủy ticket — file: `src/backend/modules/competition/internal/application/command/matchmaking/create_ticket.go`, `src/backend/modules/competition/internal/presentation/consumer/party_ticket_consumer.go` — **Xong 2026-11-21:** `CreateTicket` nhánh party (`ticketStudents`), `command.Matchmaking.PartyChanged`; consumer `presentation/consumer/matchmaking_party.go`

**Checkpoint**: User Story 3 chạy độc lập.

## Phase 6: User Story 4 — Tám kiểu matchmaking và thách đấu bạn (Priority: P2)

**Goal**: Đủ 8 kiểu ghép, duel, proposal trực tiếp cho giải, danh sách mode.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [x] T032 [P] [US4] Unit test F104-AC-18: hành vi từng `matchmaking_type` (FIFO random, grade chính xác, topic bắt buộc, ranked flag); `friend`/`tournament` → `422` — file: `src/backend/modules/competition/tests/unit/matchmaking/types_test.go` — phải FAIL trước khi code — **Xong 2026-11-21:** `tests/unit/mm_constraints_test.go` `TestCheckRequestTypes`, `mm_pairing_test.go` `TestRandomIsFIFO`; integration `TestMMTypeValidation`
- [x] T033 [P] [US4] Integration test F104-AC-19, F104-AC-20 (TC-CMP-010): duel bạn bè, hết hạn 60 s → `410 DUEL_EXPIRED`, block/khác grade band → `SOCIAL_SCOPE_VIOLATION`, accept → trận unranked, event duel — file: `src/backend/modules/competition/tests/integration/duel/duel_test.go` — **Xong 2026-11-21:** `tests/integration/mm_us4_test.go`: `TestMMDuel`
- [x] T034 [P] [US4] Integration test F104-AC-21, F104-AC-22: `CreateDirectProposal` có `tournament_node_id`; `listCompetitionModes` theo khối/cấu hình — file: `src/backend/modules/competition/tests/integration/matchmaking/direct_proposal_modes_test.go` — **Xong 2026-11-21:** `tests/integration/mm_us4_test.go`: `TestMMDirectProposalAndModes`

### Implementation cho User Story 4

- [x] T035 [P] [US4] Domain `Duel` + command `CreateDuel`, `AcceptDuel`, `DeclineDuel`, `ExpireDuels` + handler + job — file: `src/backend/modules/competition/internal/domain/matchmaking/duel/duel.go`, `src/backend/modules/competition/internal/application/command/duel/`, `src/backend/modules/competition/internal/presentation/http/public/duel_handler.go`, `src/backend/modules/competition/internal/presentation/job/duel_expiry.go` — **Xong 2026-11-21:** `domain/matchmaking/duel.go`, `command/duel.go` (`CreateDuel`, `AcceptDuel`, `DeclineDuel`, `ExpireDuels`), handler trong `presentation/http/matchmaking.go`, job `DuelExpiry` (5 s)
- [x] T036 [US4] Command `CreateDirectProposal`; query `ListCompetitionModes` + handler — file: `src/backend/modules/competition/internal/application/command/matchmaking/create_direct_proposal.go`, `src/backend/modules/competition/internal/application/query/matchmaking/list_competition_modes.go` — **Xong 2026-11-21:** `command/matchmaking_direct.go` `CreateDirectProposal`; `query/matchmaking.go` `ListCompetitionModes`, `GetTicket`

**Checkpoint**: User Story 4 chạy độc lập.

## Phase 7: User Story 5 — Phân quyền, cô lập tenant, idempotency, hiệu năng (Priority: P1)

**Goal**: Chặn nhân sự, cô lập tenant, idempotency, hiệu năng.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [x] T037 [P] [US5] Test authorization F104-AC-23 (TC-AUTH-014): mọi route matchmaking/duel × {school_admin, academic_manager, tenant_admin, system_admin} → `403 GAME_ACCESS_STUDENT_ONLY` — file: `src/backend/modules/competition/tests/integration/matchmaking/authz_test.go` — phải FAIL trước khi code — **Xong 2026-11-21:** `tests/integration/mm_us5_test.go`: `TestMMRoutesStaffDenied` (10 route × school_admin, academic_manager, tenant_admin, system_admin)
- [x] T038 [P] [US5] Test tenant isolation F104-AC-24: ticket/duel/proposal của học sinh khác/tenant khác → 404; header sai → `403 TENANT_MISMATCH` — file: `src/backend/modules/competition/tests/integration/matchmaking/tenant_isolation_test.go` — **Xong 2026-11-21:** `tests/integration/mm_us5_test.go`: `TestMMTenantIsolation`; `tests/integration/mm_rls_test.go` `TestMatchmakingRLS`
- [x] T039 [P] [US5] Test F104-AC-25, F104-AC-26: Idempotency-Key bắt buộc/replay/reused; audit `match.created`, `duel.*`; enum/maxLength → `422` — file: `src/backend/modules/competition/tests/integration/matchmaking/idempotency_validation_test.go` — **Xong 2026-11-21:** `tests/integration/mm_us5_test.go`: `TestMMIdempotency`; `mm_us4_test.go` `TestMMTypeValidation`; audit trong `TestMMProposalAccept`, `TestMMDuel`
- [x] T040 [US5] Kịch bản tải F104-AC-27 (TC-PERF-004) — file: `tests/perf/k6/matchmaking_queue.js` — **Xong 2026-11-21:** `tests/perf/matchmaking_queue.js` (k6, staging); local: `TestMMQueueLoad` 400 học sinh → p90 1 s, lượt chậm nhất 54 ms

### Implementation cho User Story 5

- [x] T041 [US5] Gắn middleware `game_player`, owner check, idempotency vào router; metric `eduworld_mm_*` — file: `src/backend/modules/competition/internal/presentation/http/public/router.go`, `src/backend/modules/competition/internal/infrastructure/observability/mm_metrics.go` — **Xong 2026-11-21:** mọi route F104 qua `op()` với `GuardGamePlayer` + `TenantRequired`; owner check ở use case; metric `observability/matchmaking.go` (`eduworld_mm_*`, không nhãn trường/học sinh)

**Checkpoint**: User Story 5 đạt.

## Phase 8: Polish & Cross-Cutting

- [x] T042 [P] Kiểm log không có nickname/student_id ở INFO (R14); alert p95 chờ > 60 s — file: `deploy/observability/alerts/competition.yaml` — **Xong 2026-11-21:** log chỉ có id/loại lỗi (không nickname/student_id); `TestMMQueueLoad` kiểm nhãn metric; alert `MatchmakingWaitHigh` (p95 chờ > 60 s/15 phút), `MatchmakerLoopSlow` (> 1 s) trong `deploy/observability/prometheus/alerts/competition.yml` (thư mục alert hiện hành)
- [x] T043 [P] Cập nhật CHANGELOG, trạng thái F104 trong CODE_PLAN — file: `CHANGELOG.md`, `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md` — **Xong 2026-11-21:** CHANGELOG, CODE_PLAN cập nhật
- [x] T044 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/104-matchmaking/quickstart.md` — **Xong 2026-11-21:** quickstart tay cần stack compose (`make up`) — kịch bản 1–21 được phủ bằng integration (bảng bằng chứng trong quickstart.md); kịch bản 22 (k6 staging) chờ môi trường

## ADR-0095 — bỏ khái niệm lớp (WP-C1, 2026-11-21)

- [x] T050 Bỏ `same_class`: `CostWeights`, `Candidate.Classes`, `Player.ClassID`, `Ticket.Classes` (trường Valkey `classes`), truy vấn `ref.enrollments`; cấu hình version 2 không có `same_class` (seed + migration) — test `TestPairOldestFirstLowestCost` (unit: không còn ưu tiên khác lớp), `TestMMCreateTicketServerSide` (ticket Valkey không có `classes`), `TestSeedMatchmakingConfig` (version 2 hiện hành bằng seed) pass

## Dependencies & Execution Order

- Setup → Foundational (T003–T011) → US1 → US2, US3 (song song) → US4 → US5 (test authz/tenant có thể viết song song từ đầu) → Polish.
- T020, T031 cùng sửa `create_ticket.go`: tuần tự. F105 cần T020 (`CreateMatch`); F107 cần T036 (`CreateDirectProposal`); F106 thay `DefaultRatingReader` (T009).

## Definition of Done

- [x] Mọi tiêu chí F104-AC-* có test pass (ghi mã test/TC) — 27/27: AC-01/03 `TestMMCreateTicketServerSide`/`TestMMScope`; AC-02 unit H1 + 10.000 cặp; AC-04/05 `TestMMProposalAccept` + unit window/cost; AC-06 `TestMMDeclineTimeoutCooldown`; AC-07 `TestMMOneTicketPerStudent`; AC-08 `TestMMEligibilityAndBlocks`; AC-09 `TestMMParallelMatchmakers`; AC-10 `TestMMCancelAndExpire`; AC-11 `TestMMValkeyReset`; AC-12/13 `TestMMBotAndTicketRate`; AC-14/15 `TestMMPartyTicket`, `TestMMPartyChangedWhileProposed`; AC-16 `TestMMAllowFill` + `TestTeamDraft`; AC-17 `TestMMTeamPolicy`; AC-18/26 `TestMMTypeValidation` + `TestCheckRequestTypes`; AC-19/20 `TestMMDuel`; AC-21/22 `TestMMDirectProposalAndModes`; AC-23 `TestMMRoutesStaffDenied`; AC-24 `TestMMTenantIsolation`, `TestMatchmakingRLS`; AC-25 `TestMMIdempotency`; AC-27 `TestMMQueueLoad` (local; k6 staging chờ môi trường)
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh — chạy từng lệnh tương đương (gofmt, vet -tags integration, golangci-lint + arch, ewlint, entlint, tenancyguard, archgen, routematrix, allowedcallers, facadegen, gen-routes, unit competition/cmd/tools, integration competition qua tc-lock); không có client Godot/web mới (portal chỉ sinh lại kiểu API)
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — test double (`fakeRatings`, presence, push) chỉ trong test, có nhãn TEST DOUBLE
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
