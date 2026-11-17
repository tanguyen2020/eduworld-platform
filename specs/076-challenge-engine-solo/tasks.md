# Tasks: F076 — Challenge Engine solo, 5 mức khó, room host dùng chung

**Input**: [spec.md](spec.md), [plan.md](plan.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo gói `src/backend/pkg/roomhost` trong Go module `src/backend` (`src/backend/go.mod` duy nhất — ADR-0074, ADR-0092; depguard cấm import `src/backend/modules/*`) — file: `src/backend/pkg/roomhost/doc.go`
- [ ] T002 [P] Tạo package `internal/domain/challenge` + đấu nối handler vào HTTP handler/consumer qua interface có kiểu — file: `src/backend/modules/game/internal/domain/challenge/doc.go`
- [ ] T003 [P] Cấu hình tham số `difficulty_levels`, `g1_3_challenge_rules`, `challenge_attempts_per_day`, `challenge_fail_cooldown`, `attempt_resume_window`, `deadline_grace` — file: `src/backend/modules/game/internal/config/challenge.go`, `src/backend/modules/game/.env.example`
- [ ] T004 [P] Seed config challenge `time` khối 3, `survival` khối 3–7, `weekly` khối 7 — file: `deploy/seed/game-config/challenges/`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T005 Phối hợp module learning sửa `createAttempt` (mode `challenge`, `context_type = challenge_attempt`, `difficulty_min/max`, `grade_level`, `objective_ids`, `content_type = activity`) + facade test bên gọi game — file: `specs/openapi/modules/learning.internal.v1.yaml`, `src/backend/modules/game/tests/contract/learning_create_attempt_test.go` — kiểm: `make specs-lint`, `make specs-breaking`, `make test-contract`
- [ ] T006 [P] OpenAPI: `startGameChallenge` khai báo `422`, `428`; `nextChallengeActivity` khai báo `428`, `503` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=game`
- [ ] T007 [P] Payload `challenge.started.v1`, `challenge.completed.v1`; `reward-rule.schema.json` trigger `challenge.completed`, `conditions.challenge_statuses`, `grants[].scale_by` — file: `specs/events/payloads/game/`, `specs/schemas/reward-rule.schema.json` — kiểm: `make specs-lint`
- [ ] T008 Adapter facade `learning` qua `src/backend/pkg/modcall` (deadline 2 s, `Idempotency-Key`, `TenantScope`, Principal gốc; không retry/circuit breaker) — file: `src/backend/modules/game/internal/infrastructure/modules/learning.go`
- [ ] T009 ent schema `Challenge`, `ChallengeVersion`, `ChallengeAttempt` (cột thêm theo data-model) — file: `src/backend/modules/game/ent/schema/` — kiểm: `make migrate-diff MOD=game name=f076_challenge`, `make migrate-lint MOD=game`
- [ ] T010 Migration: UNIQUE `(student_id) WHERE status = 'in_progress'`, GIN `learning_attempt_ids`, index `(status, deadline_at)`, RLS — file: `src/backend/modules/game/migrations/`
- [ ] T011 [P] Test RLS `challenge_attempts` (tenant A không đọc B; không set tenant → 0 dòng) — file: `src/backend/modules/game/tests/integration/challenge_rls_test.go`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Xem challenge khả dụng (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [ ] T012 [P] [US1] Unit test `challenge.Eligibility` + `DifficultyProfile.ForGrade` cho F076-AC-01 (khối, unlock, availability, khối 1–3 tối đa Hard) — file: `src/backend/modules/game/internal/domain/challenge/eligibility_test.go` — phải FAIL trước khi code
- [ ] T013 [P] [US1] API test F076-AC-01, F076-AC-02: lọc danh sách, `attempts_left`, challenge không khả dụng → 404 — file: `src/backend/modules/game/tests/integration/challenge_list_test.go`
- [ ] T014 [P] [US1] Test authorization + tenant isolation F076-AC-03: 6 route × token `school_admin`, `academic_manager`, `tenant_admin`, `safety_moderator`, `system_admin` → 403; header tenant khác → `TENANT_MISMATCH`; attempt học sinh khác → 404 — file: `src/backend/modules/game/tests/integration/challenge_authz_test.go`

### Implementation cho User Story 1

- [ ] T015 [P] [US1] Domain `ChallengeVersion`, `DifficultyProfile`, `Eligibility` — file: `src/backend/modules/game/internal/domain/challenge/`
- [ ] T016 [US1] Query `ListChallenges`, `GetChallenge` + handler `listGameChallenges`, `getGameChallenge` — file: `src/backend/modules/game/internal/application/query/list_challenges.go`, `src/backend/modules/game/internal/presentation/http/public/challenges.go`

## Phase 4: User Story 2 — Bắt đầu challenge theo mức khó (Priority: P1)

### Tests cho User Story 2

- [ ] T017 [P] [US2] Integration test F076-AC-04, F076-AC-05: tạo attempt ghim version, `deadline_at` theo giờ server (×1,5 khối 1–3), `lives_left`, `challenge.started`; sai khối/mức/`party_id` → `CHALLENGE_NOT_ELIGIBLE` — file: `src/backend/modules/game/tests/integration/challenge_start_test.go` — phải FAIL trước khi code
- [ ] T018 [P] [US2] Integration test F076-AC-06, F076-AC-07: hết lượt/cooldown → `CHALLENGE_ATTEMPT_LIMIT`; cùng key → cùng attempt; thiếu key 428; đang có attempt khác → 409 — file: `src/backend/modules/game/tests/integration/challenge_start_limits_test.go`

### Implementation cho User Story 2

- [ ] T019 [US2] Domain `Attempt.Start`, command `StartChallenge` (idempotency, outbox `challenge.started`, đăng ký room) + handler `startGameChallenge` — file: `src/backend/modules/game/internal/application/command/start_challenge.go`

## Phase 5: User Story 3 — Hoạt động học tập cá nhân, chấm ở server (Priority: P1)

### Tests cho User Story 3

- [ ] T020 [P] [US3] Unit test luật theo `challenge_type` cho F076-AC-10, F076-AC-11 (time, perfect, survival, puzzle, knowledge, weekly, seasonal; `first_clear`) — file: `src/backend/modules/game/internal/domain/challenge/rules_test.go` — phải FAIL trước khi code
- [ ] T021 [P] [US3] Integration test F076-AC-08, F076-AC-09 với module learning thật (testcontainers compose) hoặc contract stub gắn nhãn `// TEST-ONLY STUB`: mode `challenge`, `student_id` phiên, dải độ khó Hard khối 3, render không đáp án — file: `src/backend/modules/game/tests/integration/challenge_next_activity_test.go`
- [ ] T022 [P] [US3] Integration test F076-AC-10, F076-AC-11: `activity.graded` cập nhật điểm/sức bền, event lặp không áp hai lần, kết thúc phát một `challenge.completed`, Reward Engine thưởng lần đầu một lần — file: `src/backend/modules/game/tests/integration/challenge_progress_test.go`
- [ ] T023 [P] [US3] Test F076-AC-12: facade `learning` lỗi/hết deadline → `503`, trạng thái không đổi, gọi lại cùng key → cùng attempt học tập (TC-SVC-010) — file: `src/backend/modules/game/tests/integration/challenge_learning_down_test.go`

### Implementation cho User Story 3

- [ ] T024 [US3] Command `NextChallengeActivity` (port `LearningAttemptClient`) + handler `nextChallengeActivity` — file: `src/backend/modules/game/internal/application/command/next_challenge_activity.go`
- [ ] T025 [US3] Domain `Attempt.ApplyGraded`, command `ApplyChallengeResult`, consumer `game.challenge_progress`, publish `challenge.attempt.updated` sau commit — file: `src/backend/modules/game/internal/application/command/apply_challenge_result.go`, `src/backend/modules/game/internal/presentation/consumer/challenge_progress.go`

## Phase 6: User Story 4 — Thời gian server, bỏ cuộc, tiếp tục (Priority: P1)

### Tests cho User Story 4

- [ ] T026 [P] [US4] Integration test F076-AC-13 với fake clock: quá `deadline_at` → `failed` bởi timer; job `challenge-deadline` khi tắt timer; gọi tiếp → 409 — file: `src/backend/modules/game/tests/integration/challenge_deadline_test.go` — phải FAIL trước khi code
- [ ] T027 [P] [US4] Integration test F076-AC-14, F076-AC-15: abandon giữ attempt học tập, abandon lần 2 → 409; takeover đọc đúng checkpoint, quá resume window → `abandoned` — file: `src/backend/modules/game/tests/integration/challenge_abandon_resume_test.go`
- [ ] T028 [P] [US4] Static test F076-AC-16: quét `game.v1.yaml`, request challenge không có `score|lives_left|status|result` — file: `src/backend/modules/game/tests/contract/no_client_challenge_result_test.go`

### Implementation cho User Story 4

- [ ] T029 [US4] Command `AbandonChallenge`, `ExpireChallenge`; query `GetChallengeAttempt`; job `challenge-deadline`; consumer khóa/dọn học sinh → `abandoned` — file: `src/backend/modules/game/internal/application/command/abandon_challenge.go`, `src/backend/modules/game/internal/presentation/job/challenge_deadline.go`

## Phase 7: User Story 5 — Thư viện room host dùng chung (Priority: P2)

### Tests cho User Story 5

- [ ] T030 [P] [US5] Test `src/backend/pkg/roomhost` F076-AC-17, F076-AC-18: hai node một lease; owner dừng gia hạn → node khác chiếm sau TTL, khôi phục checkpoint, timer không chạy hai lần; timer theo `src/backend/pkg/clock`; grace theo loại room (Valkey testcontainer) — file: `src/backend/pkg/roomhost/roomhost_test.go` — phải FAIL trước khi code
- [ ] T031 [P] [US5] Integration test F076-AC-19: Valkey tắt → job kết thúc challenge quá hạn từ PostgreSQL — file: `src/backend/modules/game/tests/integration/challenge_valkey_down_test.go`

### Implementation cho User Story 5

- [ ] T032 [US5] `src/backend/pkg/roomhost`: `Host`, `Lease`, `Timer`, `CheckpointStore`, `Grace`, `Publisher` — file: `src/backend/pkg/roomhost/`
- [ ] T033 [US5] Room handler challenge + checkpoint PostgreSQL — file: `src/backend/modules/game/internal/infrastructure/room/challenge_room.go`

## Phase 8: User Story 6 — Kiểm tra cấu hình challenge (Priority: P2)

### Tests cho User Story 6

- [ ] T034 [P] [US6] Unit test module F076-AC-20: sai schema, min > max, khối 1–3 vượt Hard, hoạt động chưa publish, loại nhóm dùng `solo`, `time` thiếu `time_limit_seconds` → 422; thiếu `en` → `TRANSLATION_INCOMPLETE` — file: `src/backend/modules/game/tests/unit/challenge_module_test.go` — phải FAIL trước khi code
- [ ] T035 [P] [US6] API test F076-AC-21: `area_id` > 64 ký tự, enum sai → 422 (TC-TEN-024) — file: `src/backend/modules/game/tests/integration/challenge_input_validation_test.go`

### Implementation cho User Story 6

- [ ] T036 [US6] Module config `challenges` đăng ký `ConfigTypeRegistry` (validate + `OnPublished` cập nhật cache con trỏ) — file: `src/backend/modules/game/internal/infrastructure/gameconfig/challenge_module.go`

## Phase 9: Polish & Cross-Cutting

- [ ] T037 [P] Cập nhật DATABASE_DESIGN §2.8a (cột thêm `challenge_attempts`), CHALLENGE_BOSS_SYSTEM §2.4 (resume challenge solo theo `attempt_resume_window`), VALKEY_DESIGN (room `challenge_solo` của module game), EVENT_DESIGN §4.6, CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md`, `docs/07-backend/VALKEY_DESIGN.md`, `CHANGELOG.md`
- [ ] T038 [P] Metric `eduworld_game_challenge_*`, `eduworld_roomhost_*` — file: `src/backend/modules/game/internal/infrastructure/observability/metrics.go`, `src/backend/pkg/roomhost/metrics.go`
- [ ] T039 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational (T005 chặn US3; T008 trước T024) → US1 → US2 → US3 → US4 → US5 (T032 có thể làm song song từ Phase 2, cần xong trước T019 đăng ký room) → US6 → Polish.
- Phụ thuộc: F030 (delta `createAttempt`), F074 (Reward Engine, level), F072/F071 (unlock).

## Definition of Done

- [ ] Mọi tiêu chí F076-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh cho module `game`; `go -C src/backend test ./pkg/roomhost/...` xanh; `make test-contract` xanh với module learning
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
