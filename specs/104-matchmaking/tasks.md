# Tasks: F104 — Matchmaking 8 chế độ: queue, ticket, proposal, ready check, ràng buộc khối

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo package `matchmaking`, `matchmaking/duel`; đăng ký handler HTTP, NATS command `ew.cmd.competition.matchmaking.*`, job `matchmaker`, `ticket_expiry`, `duel_expiry` (profile `worker-jobs`) qua `Register(...)` — file: `src/backend/modules/competition/module.go` (nối vào `src/backend/cmd/core-api`, `src/backend/cmd/worker`)
- [ ] T002 [P] Biến môi trường `EW_MM_TICK`, ACL Valkey prefix `ew:competition:` — file: `src/backend/modules/competition/internal/infrastructure/config/config.go`, `deploy/compose/.env.example`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T003 Delta OpenAPI: sửa `MatchmakingTicketCreate`, `MatchmakingTicket`, `Match`; thêm `declineMatchProposal`, `startBotMatch`, `listCompetitionModes` + schema `CompetitionMode` — file: `specs/openapi/modules/competition.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=competition`
- [ ] T004 [P] Delta AsyncAPI `matchmaking.ticket.updated` (`bot_offer_available`, `matchmaking_type`) — file: `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`
- [ ] T005 [P] Payload schema `match.created.v1`, `duel.sent/accepted/declined/expired.v1` — file: `specs/events/payloads/competition/` — kiểm: `make specs-lint`
- [ ] T006 ent schema `matches`, `match_participants`, `duels`, `competition_restrictions`, `game_configs`, `game_config_versions`, `content_releases`, `ref.enrollments`, cột `ref.schools.settings_subset` (`ref.blocks` có sẵn từ F091) — mọi chuỗi `MaxLen` — file: `src/backend/modules/competition/ent/schema/` — kiểm: `make migrate-diff MOD=competition name=f104_matchmaking`, `make migrate-lint MOD=competition`
- [ ] T007 RLS, partial unique duel pending, trigger bất biến `game_config_versions` published — file: `src/backend/modules/competition/migrations/<ts>_f104_matchmaking.sql`
- [ ] T008 [P] Port `ConfigReader` + repo ent + seed cấu hình matchmaking hiệu lực — file: `src/backend/modules/competition/internal/application/port/config.go`, `src/backend/modules/competition/internal/infrastructure/persistence/config_repo.go`, `src/backend/modules/competition/seed/matchmaking_config.json`
- [ ] T009 [P] Port `RatingReader` + `DefaultRatingReader` (trả `glicko2_params`, F106 thay) — file: `src/backend/modules/competition/internal/application/port/rating.go`
- [ ] T010 [P] Consumer projection `ref.enrollments`, `ref.schools.settings_subset` (`ref.blocks` do consumer F091 ghi) — file: `src/backend/modules/competition/internal/presentation/consumer/ref_projection.go`
- [ ] T011 [P] Cập nhật DATABASE_DESIGN §2.10/§2.16/§2.17, VALKEY_DESIGN §3.1, API_DESIGN §9.7 — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/07-backend/VALKEY_DESIGN.md`, `docs/07-backend/API_DESIGN.md`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Ghép 1v1 cùng khối theo rating (Priority: P1) 🎯 MVP

**Goal**: Ticket → queue → proposal → ready check → trận.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T012 [P] [US1] Unit test F104-AC-02: `QueueKey`/H1 cho lớp 1–9 (bảng đầy đủ), không bao giờ ghép lớp 1–5 khác khối, ±1 lớp 6–9 sau `grade_window_g6_9`; property test 10.000 cặp — file: `src/backend/modules/competition/tests/unit/matchmaking/constraints_test.go` — phải FAIL trước khi code
- [ ] T013 [P] [US1] Unit test F104-AC-04: `Window(t)` theo `mm_rating_window`, cost (rating, RD, recent opponent, cùng lớp), greedy chọn ticket cũ nhất — file: `src/backend/modules/competition/tests/unit/matchmaking/pairing_test.go`
- [ ] T014 [P] [US1] Integration test F104-AC-01, F104-AC-03: ticket lấy khối/trường/rating từ server, bỏ qua field client; phạm vi trường mặc định, tenant khi bật liên trường — file: `src/backend/modules/competition/tests/integration/matchmaking/create_ticket_test.go`
- [ ] T015 [P] [US1] Integration test F104-AC-05, F104-AC-06: proposal không lộ tên/trường; accept → `match_id = proposal_id`, `match.created`; decline/timeout → requeue giữ `enqueued_at`, cooldown sau 3 lần — file: `src/backend/modules/competition/tests/integration/matchmaking/proposal_test.go`
- [ ] T016 [P] [US1] Integration test F104-AC-07, F104-AC-08: ticket thứ hai `409 MM_TICKET_ALREADY_ACTIVE` (kể cả đồng thời); block không cùng proposal; hạn chế → `403 POLICY_DENIED` — file: `src/backend/modules/competition/tests/integration/matchmaking/eligibility_test.go`
- [ ] T017 [P] [US1] Integration test F104-AC-09: 2 matchmaker song song, 200 ticket, không trùng trận — file: `src/backend/modules/competition/tests/integration/matchmaking/concurrency_test.go`

### Implementation cho User Story 1

- [ ] T018 [P] [US1] Domain `QueueKey`, `HardConstraints`, `Cost`, `Window`, `Pairing`, `Proposal` — file: `src/backend/modules/competition/internal/domain/matchmaking/constraints.go`, `src/backend/modules/competition/internal/domain/matchmaking/cost.go`, `src/backend/modules/competition/internal/domain/matchmaking/window.go`, `src/backend/modules/competition/internal/domain/matchmaking/pairing.go`, `src/backend/modules/competition/internal/domain/matchmaking/proposal.go`
- [ ] T019 [US1] Adapter Valkey `QueueStore`/`TicketStore`/`ProposalStore` (Lua nguyên tử, lock per-queue) — file: `src/backend/modules/competition/internal/infrastructure/cache/mm_store.go`
- [ ] T020 [US1] Command `CreateTicket`, `AcceptProposal`, `DeclineProposal`, `RunMatchmakerTick`, `CreateMatch` (tx + outbox + audit) — file: `src/backend/modules/competition/internal/application/command/matchmaking/`
- [ ] T021 [US1] Handler `createMatchmakingTicket`, `getMatchmakingTicket`, `acceptMatchProposal`, `declineMatchProposal`; NATS command handler; publisher `ew:rt:user:<id>` — file: `src/backend/modules/competition/internal/presentation/http/public/matchmaking_handler.go`, `src/backend/modules/competition/internal/presentation/realtime/matchmaking_cmd.go`
- [ ] T022 [US1] Job `matchmaker` theo `EW_MM_TICK` — file: `src/backend/modules/competition/internal/presentation/job/matchmaker.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Hủy, hết hạn, mất Valkey, bot luyện tập (Priority: P1)

**Goal**: Hủy/hết hạn, phục hồi khi mất Valkey, đề nghị bot.

**Independent Test**: như spec.md US2.

### Tests cho User Story 2

- [ ] T023 [P] [US2] Integration test F104-AC-10: cancel REST + WS; hết `mm_ticket_ttl` → `expired` — file: `src/backend/modules/competition/tests/integration/matchmaking/cancel_expire_test.go` — phải FAIL trước khi code
- [ ] T024 [P] [US2] Integration test F104-AC-11: `FLUSHALL` → `cancelled` (`system`), không trận ma trong PG — file: `src/backend/modules/competition/tests/integration/matchmaking/valkey_reset_test.go`
- [ ] T025 [P] [US2] Integration test F104-AC-12, F104-AC-13: `bot_offer_available` sau `mm_bot_after`; `startBotMatch` → trận bot unranked; vượt `mm_ticket_rate` → `429` + `Retry-After` — file: `src/backend/modules/competition/tests/integration/matchmaking/bot_ratelimit_test.go`

### Implementation cho User Story 2

- [ ] T026 [US2] Command `CancelTicket`, `ExpireTickets`, phát hiện ticket mất (đối chiếu `mm:user:*` với kết nối) — file: `src/backend/modules/competition/internal/application/command/matchmaking/cancel_ticket.go`, `src/backend/modules/competition/internal/application/command/matchmaking/expire_tickets.go`, `src/backend/modules/competition/internal/presentation/job/ticket_expiry.go`
- [ ] T027 [US2] Command `StartBotMatch` + handler `startBotMatch`, `cancelMatchmakingTicket`; rate limit/cooldown — file: `src/backend/modules/competition/internal/application/command/matchmaking/start_bot_match.go`, `src/backend/modules/competition/internal/presentation/http/public/matchmaking_handler.go`

**Checkpoint**: User Story 2 chạy độc lập.

## Phase 5: User Story 3 — Ticket đội từ party (Priority: P1)

**Goal**: Ticket đội, chia đội, hủy khi party đổi.

**Independent Test**: như spec.md US3.

### Tests cho User Story 3

- [ ] T028 [P] [US3] Integration test F104-AC-14, F104-AC-15: leader-only, member online, H1 mọi member, `μ_team`; member rời → `party_changed` tới mọi member — file: `src/backend/modules/competition/tests/integration/matchmaking/party_ticket_test.go` — phải FAIL trước khi code
- [ ] T029 [P] [US3] Unit test F104-AC-16, F104-AC-17: snake draft, `allow_fill`, party giữ đội; `team_battle` lớp < 4 và party khác khối ranked → `403 POLICY_DENIED` — file: `src/backend/modules/competition/tests/unit/matchmaking/team_draft_test.go`

### Implementation cho User Story 3

- [ ] T030 [P] [US3] Domain `TeamDraft`, team rating — file: `src/backend/modules/competition/internal/domain/matchmaking/team.go`
- [ ] T031 [US3] Nhánh party trong `CreateTicket`; consumer `party.member_left`/`party.disbanded` hủy ticket — file: `src/backend/modules/competition/internal/application/command/matchmaking/create_ticket.go`, `src/backend/modules/competition/internal/presentation/consumer/party_ticket_consumer.go`

**Checkpoint**: User Story 3 chạy độc lập.

## Phase 6: User Story 4 — Tám kiểu matchmaking và thách đấu bạn (Priority: P2)

**Goal**: Đủ 8 kiểu ghép, duel, proposal trực tiếp cho giải, danh sách mode.

**Independent Test**: như spec.md US4.

### Tests cho User Story 4

- [ ] T032 [P] [US4] Unit test F104-AC-18: hành vi từng `matchmaking_type` (FIFO random, grade chính xác, topic bắt buộc, ranked flag); `friend`/`tournament` → `422` — file: `src/backend/modules/competition/tests/unit/matchmaking/types_test.go` — phải FAIL trước khi code
- [ ] T033 [P] [US4] Integration test F104-AC-19, F104-AC-20 (TC-CMP-010): duel bạn bè, hết hạn 60 s → `410 DUEL_EXPIRED`, block/khác grade band → `SOCIAL_SCOPE_VIOLATION`, accept → trận unranked, event duel — file: `src/backend/modules/competition/tests/integration/duel/duel_test.go`
- [ ] T034 [P] [US4] Integration test F104-AC-21, F104-AC-22: `CreateDirectProposal` có `tournament_node_id`; `listCompetitionModes` theo khối/cấu hình — file: `src/backend/modules/competition/tests/integration/matchmaking/direct_proposal_modes_test.go`

### Implementation cho User Story 4

- [ ] T035 [P] [US4] Domain `Duel` + command `CreateDuel`, `AcceptDuel`, `DeclineDuel`, `ExpireDuels` + handler + job — file: `src/backend/modules/competition/internal/domain/matchmaking/duel/duel.go`, `src/backend/modules/competition/internal/application/command/duel/`, `src/backend/modules/competition/internal/presentation/http/public/duel_handler.go`, `src/backend/modules/competition/internal/presentation/job/duel_expiry.go`
- [ ] T036 [US4] Command `CreateDirectProposal`; query `ListCompetitionModes` + handler — file: `src/backend/modules/competition/internal/application/command/matchmaking/create_direct_proposal.go`, `src/backend/modules/competition/internal/application/query/matchmaking/list_competition_modes.go`

**Checkpoint**: User Story 4 chạy độc lập.

## Phase 7: User Story 5 — Phân quyền, cô lập tenant, idempotency, hiệu năng (Priority: P1)

**Goal**: Chặn nhân sự, cô lập tenant, idempotency, hiệu năng.

**Independent Test**: như spec.md US5.

### Tests cho User Story 5

- [ ] T037 [P] [US5] Test authorization F104-AC-23 (TC-AUTH-014): mọi route matchmaking/duel × {school_admin, academic_manager, tenant_admin, system_admin} → `403 GAME_ACCESS_STUDENT_ONLY` — file: `src/backend/modules/competition/tests/integration/matchmaking/authz_test.go` — phải FAIL trước khi code
- [ ] T038 [P] [US5] Test tenant isolation F104-AC-24: ticket/duel/proposal của học sinh khác/tenant khác → 404; header sai → `403 TENANT_MISMATCH` — file: `src/backend/modules/competition/tests/integration/matchmaking/tenant_isolation_test.go`
- [ ] T039 [P] [US5] Test F104-AC-25, F104-AC-26: Idempotency-Key bắt buộc/replay/reused; audit `match.created`, `duel.*`; enum/maxLength → `422` — file: `src/backend/modules/competition/tests/integration/matchmaking/idempotency_validation_test.go`
- [ ] T040 [US5] Kịch bản tải F104-AC-27 (TC-PERF-004) — file: `tests/perf/k6/matchmaking_queue.js`

### Implementation cho User Story 5

- [ ] T041 [US5] Gắn middleware `game_player`, owner check, idempotency vào router; metric `eduworld_mm_*` — file: `src/backend/modules/competition/internal/presentation/http/public/router.go`, `src/backend/modules/competition/internal/infrastructure/observability/mm_metrics.go`

**Checkpoint**: User Story 5 đạt.

## Phase 8: Polish & Cross-Cutting

- [ ] T042 [P] Kiểm log không có nickname/student_id ở INFO (R14); alert p95 chờ > 60 s — file: `deploy/observability/alerts/competition.yaml`
- [ ] T043 [P] Cập nhật CHANGELOG, trạng thái F104 trong CODE_PLAN — file: `CHANGELOG.md`, `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md`
- [ ] T044 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/104-matchmaking/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational (T003–T011) → US1 → US2, US3 (song song) → US4 → US5 (test authz/tenant có thể viết song song từ đầu) → Polish.
- T020, T031 cùng sửa `create_ticket.go`: tuần tự. F105 cần T020 (`CreateMatch`); F107 cần T036 (`CreateDirectProposal`); F106 thay `DefaultRatingReader` (T009).

## Definition of Done

- [ ] Mọi tiêu chí F104-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
