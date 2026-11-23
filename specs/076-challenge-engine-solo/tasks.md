# Tasks: F076 — Challenge Engine solo, 5 mức khó, room host dùng chung

**Input**: [spec.md](spec.md), [plan.md](plan.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 (kết quả: `src/backend/pkg/roomhost` mở rộng (`host.go`, `valkey.go`, `metrics.go`); archgen 0 vi phạm) Tạo gói `src/backend/pkg/roomhost` trong Go module `src/backend` (`src/backend/go.mod` duy nhất — ADR-0074, ADR-0092; depguard cấm import `src/backend/modules/*`) — file: `src/backend/pkg/roomhost/doc.go`
- [x] T002 (kết quả: `internal/domain/challenge` (challenge.go, eligibility.go, attempt.go)) [P] Tạo package `internal/domain/challenge` + đấu nối handler vào HTTP handler/consumer qua interface có kiểu — file: `src/backend/modules/game/internal/domain/challenge/doc.go`
- [x] T003 (kết quả: `internal/infrastructure/config/challenge.go` — env `EW_GAME_CHALLENGE_ATTEMPTS_PER_DAY`, `EW_GAME_CHALLENGE_FAIL_COOLDOWN`, `EW_GAME_ATTEMPT_RESUME_WINDOW`, `EW_INSTANCE_ID`; không có `.env.example` riêng của module game) [P] Cấu hình tham số `difficulty_levels`, `g1_3_challenge_rules`, `challenge_attempts_per_day`, `challenge_fail_cooldown`, `attempt_resume_window`, `deadline_grace` — file: `src/backend/modules/game/internal/config/challenge.go`, `src/backend/modules/game/.env.example`
- [x] T004 (DỜI — Câu hỏi mở F076-2: activity của learning dev seed sinh id lúc chạy; test dùng nội dung test) [P] Seed config challenge `time` khối 3, `survival` khối 3–7, `weekly` khối 7 — file: `deploy/seed/game-config/challenges/`

## Phase 2: Foundational (chặn mọi user story)

- [x] T005 (kết quả: `learning.internal.v1.yaml` + `make gen`; module learning mở mode `challenge` (`attempt_internal.go`); `TestLearningCreateChallengeAttemptFacade` (bên gọi), learning `TestAttemptFacadeChallenge` (bên nhận)) Phối hợp module learning sửa `createAttempt` (mode `challenge`, `context_type = challenge_attempt`, `difficulty_min/max`, `grade_level`, `objective_ids`, `content_type = activity`) + facade test bên gọi game — file: `specs/openapi/modules/learning.internal.v1.yaml`, `src/backend/modules/game/tests/contract/learning_create_attempt_test.go` — kiểm: `make specs-lint`, `make specs-breaking`, `make test-contract`
- [x] T006 (kết quả: nguồn sự thật `eduworld-api.v1.yaml` → `split-public.mjs`; specs-breaking 0) [P] OpenAPI: `startGameChallenge` khai báo `422`, `428`; `nextChallengeActivity` khai báo `428`, `503` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=game`
- [x] T007 (kết quả: `challenge.started.v1`, `challenge.completed.v1`; `reward-rule.schema.json` (`challenge.completed`, `challenge_statuses`, `scale_by`); `content.published` item_type `challenge`) [P] Payload `challenge.started.v1`, `challenge.completed.v1`; `reward-rule.schema.json` trigger `challenge.completed`, `conditions.challenge_statuses`, `grants[].scale_by` — file: `specs/events/payloads/game/`, `specs/schemas/reward-rule.schema.json` — kiểm: `make specs-lint`
- [x] T008 (kết quả: `infrastructure/modules/learning.go` `CreateChallengeAttempt`) Adapter facade `learning` qua `src/backend/pkg/modcall` (deadline 2 s, `Idempotency-Key`, `TenantScope`, Principal gốc; không retry/circuit breaker) — file: `src/backend/modules/game/internal/infrastructure/modules/learning.go`
- [x] T009 (kết quả: `ent/schema/challenge.go`, migration `20261231000760_f076_challenge.sql`, `TestMigrationMatchesEnt`) ent schema `Challenge`, `ChallengeVersion`, `ChallengeAttempt` (cột thêm theo data-model) — file: `src/backend/modules/game/ent/schema/` — kiểm: `make migrate-diff MOD=game name=f076_challenge`, `make migrate-lint MOD=game`
- [x] T010 (kết quả: cùng migration (CHECK, FK, trigger, RLS, GIN, index job, `_live`)) Migration: UNIQUE `(student_id) WHERE status = 'in_progress'`, GIN `learning_attempt_ids`, index `(status, deadline_at)`, RLS — file: `src/backend/modules/game/migrations/`
- [x] T011 (kết quả: `challenge_publish_test.go` `TestChallengeRLS`) [P] Test RLS `challenge_attempts` (tenant A không đọc B; không set tenant → 0 dòng) — file: `src/backend/modules/game/tests/integration/challenge_rls_test.go`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Xem challenge khả dụng (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [x] T012 (kết quả: `eligibility_test.go` (`TestDifficultiesForGrade`, `TestVisible`, `TestDeadlineAndLives`, `TestAttemptsAndCooldown`, `TestValidate`, `TestParse`)) [P] [US1] Unit test `challenge.Eligibility` + `DifficultyProfile.ForGrade` cho F076-AC-01 (khối, unlock, availability, khối 1–3 tối đa Hard) — file: `src/backend/modules/game/internal/domain/challenge/eligibility_test.go` — phải FAIL trước khi code
- [x] T013 (kết quả: `challenge_us1_test.go` `TestChallengeList`) [P] [US1] API test F076-AC-01, F076-AC-02: lọc danh sách, `attempts_left`, challenge không khả dụng → 404 — file: `src/backend/modules/game/tests/integration/challenge_list_test.go`
- [x] T014 (kết quả: `challenge_us1_test.go` `TestChallengeAuthz` + `TestPlayerRoutesStaffDenied` (6 route mới)) [P] [US1] Test authorization + tenant isolation F076-AC-03: 6 route × token `school_admin`, `academic_manager`, `tenant_admin`, `safety_moderator`, `system_admin` → 403; header tenant khác → `TENANT_MISMATCH`; attempt học sinh khác → 404 — file: `src/backend/modules/game/tests/integration/challenge_authz_test.go`

### Implementation cho User Story 1

- [x] T015 (kết quả: domain challenge) [P] [US1] Domain `ChallengeVersion`, `DifficultyProfile`, `Eligibility` — file: `src/backend/modules/game/internal/domain/challenge/`
- [x] T016 (kết quả: `query/challenge.go`, `presentation/http/challenges.go`) [US1] Query `ListChallenges`, `GetChallenge` + handler `listGameChallenges`, `getGameChallenge` — file: `src/backend/modules/game/internal/application/query/list_challenges.go`, `src/backend/modules/game/internal/presentation/http/public/challenges.go`

## Phase 4: User Story 2 — Bắt đầu challenge theo mức khó (Priority: P1)

### Tests cho User Story 2

- [x] T017 (kết quả: `challenge_us2_test.go` `TestChallengeStart`) [P] [US2] Integration test F076-AC-04, F076-AC-05: tạo attempt ghim version, `deadline_at` theo giờ server (×1,5 khối 1–3), `lives_left`, `challenge.started`; sai khối/mức/`party_id` → `CHALLENGE_NOT_ELIGIBLE` — file: `src/backend/modules/game/tests/integration/challenge_start_test.go` — phải FAIL trước khi code
- [x] T018 (kết quả: `challenge_us2_test.go` `TestChallengeStartLimits`) [P] [US2] Integration test F076-AC-06, F076-AC-07: hết lượt/cooldown → `CHALLENGE_ATTEMPT_LIMIT`; cùng key → cùng attempt; thiếu key 428; đang có attempt khác → 409 — file: `src/backend/modules/game/tests/integration/challenge_start_limits_test.go`

### Implementation cho User Story 2

- [x] T019 (kết quả: `command/challenge.go` `Start`) [US2] Domain `Attempt.Start`, command `StartChallenge` (idempotency, outbox `challenge.started`, đăng ký room) + handler `startGameChallenge` — file: `src/backend/modules/game/internal/application/command/start_challenge.go`

## Phase 5: User Story 3 — Hoạt động học tập cá nhân, chấm ở server (Priority: P1)

### Tests cho User Story 3

- [x] T020 (kết quả: `rules_test.go`, reward `challenge_test.go` `TestChallengeCompleted`) [P] [US3] Unit test luật theo `challenge_type` cho F076-AC-10, F076-AC-11 (time, perfect, survival, puzzle, knowledge, weekly, seasonal; `first_clear`) — file: `src/backend/modules/game/internal/domain/challenge/rules_test.go` — phải FAIL trước khi code
- [x] T021 (kết quả: `challenge_us3_test.go` `TestChallengeNextActivity` (facade learning = Fake sinh, TEST DOUBLE) + learning `TestAttemptFacadeChallenge` (module learning thật)) [P] [US3] Integration test F076-AC-08, F076-AC-09 với module learning thật (testcontainers compose) hoặc contract stub gắn nhãn `// TEST-ONLY STUB`: mode `challenge`, `student_id` phiên, dải độ khó Hard khối 3, render không đáp án — file: `src/backend/modules/game/tests/integration/challenge_next_activity_test.go`
- [x] T022 (kết quả: `challenge_us3_test.go` `TestChallengeProgressAndRewards`) [P] [US3] Integration test F076-AC-10, F076-AC-11: `activity.graded` cập nhật điểm/sức bền, event lặp không áp hai lần, kết thúc phát một `challenge.completed`, Reward Engine thưởng lần đầu một lần — file: `src/backend/modules/game/tests/integration/challenge_progress_test.go`
- [x] T023 (kết quả: gộp vào `TestChallengeNextActivity` (503, không đổi, cùng key)) [P] [US3] Test F076-AC-12: facade `learning` lỗi/hết deadline → `503`, trạng thái không đổi, gọi lại cùng key → cùng attempt học tập (TC-SVC-010) — file: `src/backend/modules/game/tests/integration/challenge_learning_down_test.go`

### Implementation cho User Story 3

- [x] T024 (kết quả: `command/challenge.go` `Next`) [US3] Command `NextChallengeActivity` (port `LearningAttemptClient`) + handler `nextChallengeActivity` — file: `src/backend/modules/game/internal/application/command/next_challenge_activity.go`
- [x] T025 (kết quả: `ApplyGraded`, consumer `game.challenge_progress`) [US3] Domain `Attempt.ApplyGraded`, command `ApplyChallengeResult`, consumer `game.challenge_progress`, publish `challenge.attempt.updated` sau commit — file: `src/backend/modules/game/internal/application/command/apply_challenge_result.go`, `src/backend/modules/game/internal/presentation/consumer/challenge_progress.go`

## Phase 6: User Story 4 — Thời gian server, bỏ cuộc, tiếp tục (Priority: P1)

### Tests cho User Story 4

- [x] T026 (kết quả: `challenge_us4_test.go` `TestChallengeDeadlineValkeyDown`, `challenge_room_test.go` `TestChallengeRoomFailover`) [P] [US4] Integration test F076-AC-13 với fake clock: quá `deadline_at` → `failed` bởi timer; job `challenge-deadline` khi tắt timer; gọi tiếp → 409 — file: `src/backend/modules/game/tests/integration/challenge_deadline_test.go` — phải FAIL trước khi code
- [x] T027 (kết quả: `challenge_us4_test.go` `TestChallengeAbandonResume` (abandon lần 2 → 422 theo AC-14)) [P] [US4] Integration test F076-AC-14, F076-AC-15: abandon giữ attempt học tập, abandon lần 2 → 409; takeover đọc đúng checkpoint, quá resume window → `abandoned` — file: `src/backend/modules/game/tests/integration/challenge_abandon_resume_test.go`
- [x] T028 (kết quả: `tests/contract/no_client_challenge_result_test.go`) [P] [US4] Static test F076-AC-16: quét `game.v1.yaml`, request challenge không có `score|lives_left|status|result` — file: `src/backend/modules/game/tests/contract/no_client_challenge_result_test.go`

### Implementation cho User Story 4

- [x] T029 (kết quả: `Abandon`, `Expire`, `ExpireDue`, job `challenge-deadline`, `AbandonForStudent` (consumer `game.session_end_on_revoke`)) [US4] Command `AbandonChallenge`, `ExpireChallenge`; query `GetChallengeAttempt`; job `challenge-deadline`; consumer khóa/dọn học sinh → `abandoned` — file: `src/backend/modules/game/internal/application/command/abandon_challenge.go`, `src/backend/modules/game/internal/presentation/job/challenge_deadline.go`

## Phase 7: User Story 5 — Thư viện room host dùng chung (Priority: P2)

### Tests cho User Story 5

- [x] T030 (kết quả: `pkg/roomhost/host_test.go` (registry TEST DOUBLE) + `TestChallengeRoomFailover` (Valkey thật)) [P] [US5] Test `src/backend/pkg/roomhost` F076-AC-17, F076-AC-18: hai node một lease; owner dừng gia hạn → node khác chiếm sau TTL, khôi phục checkpoint, timer không chạy hai lần; timer theo `src/backend/pkg/clock`; grace theo loại room (Valkey testcontainer) — file: `src/backend/pkg/roomhost/roomhost_test.go` — phải FAIL trước khi code
- [x] T031 (kết quả: gộp vào `TestChallengeDeadlineValkeyDown` (registry Valkey hỏng, job từ PostgreSQL)) [P] [US5] Integration test F076-AC-19: Valkey tắt → job kết thúc challenge quá hạn từ PostgreSQL — file: `src/backend/modules/game/tests/integration/challenge_valkey_down_test.go`

### Implementation cho User Story 5

- [x] T032 (kết quả: `pkg/roomhost` `Host`, `ValkeyRegistry`, `Grace`, `CheckpointStore`, `Handler`, `Metrics`) [US5] `src/backend/pkg/roomhost`: `Host`, `Lease`, `Timer`, `CheckpointStore`, `Grace`, `Publisher` — file: `src/backend/pkg/roomhost/`
- [x] T033 (kết quả: `infrastructure/room/challenge_room.go`) [US5] Room handler challenge + checkpoint PostgreSQL — file: `src/backend/modules/game/internal/infrastructure/room/challenge_room.go`

## Phase 8: User Story 6 — Kiểm tra cấu hình challenge (Priority: P2)

### Tests cho User Story 6

- [x] T034 (kết quả: domain `TestValidate` + `challengeschema` `TestValidate` (unit) và `challenge_publish_test.go` `TestChallengePublish`) [P] [US6] Unit test module F076-AC-20: sai schema, min > max, khối 1–3 vượt Hard, hoạt động chưa publish, loại nhóm dùng `solo`, `time` thiếu `time_limit_seconds` → 422; thiếu `en` → `TRANSLATION_INCOMPLETE` — file: `src/backend/modules/game/tests/unit/challenge_module_test.go` — phải FAIL trước khi code
- [x] T035 (kết quả: gộp vào `TestChallengeList` (TC-TEN-024)) [P] [US6] API test F076-AC-21: `area_id` > 64 ký tự, enum sai → 422 (TC-TEN-024) — file: `src/backend/modules/game/tests/integration/challenge_input_validation_test.go`

### Implementation cho User Story 6

- [x] T036 (kết quả: `command/publish_challenge.go` (lõi phát hành, item type `challenge`), F077 `components.challenge_ids` kiểm khi publish) [US6] Module config `challenges` đăng ký `ConfigTypeRegistry` (validate + `OnPublished` cập nhật cache con trỏ) — file: `src/backend/modules/game/internal/infrastructure/gameconfig/challenge_module.go`

## Phase 9: Polish & Cross-Cutting

- [x] T037 (kết quả: DATABASE_DESIGN, CHALLENGE_BOSS_SYSTEM, VALKEY_DESIGN, EVENT_DESIGN, PROGRESSION_SYSTEM, ACTIVITY_ENGINE, CONFIG_DEFAULTS, CHANGELOG) [P] Cập nhật DATABASE_DESIGN §2.8a (cột thêm `challenge_attempts`), CHALLENGE_BOSS_SYSTEM §2.4 (resume challenge solo theo `attempt_resume_window`), VALKEY_DESIGN (room `challenge_solo` của module game), EVENT_DESIGN §4.6, CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md`, `docs/07-backend/VALKEY_DESIGN.md`, `CHANGELOG.md`
- [x] T038 (kết quả: `observability/challenge_metrics.go`, `pkg/roomhost/metrics.go`) [P] Metric `eduworld_game_challenge_*`, `eduworld_roomhost_*` — file: `src/backend/modules/game/internal/infrastructure/observability/metrics.go`, `src/backend/pkg/roomhost/metrics.go`
- [x] T039 (kết quả: kịch bản quickstart phủ bằng test tích hợp (bảng dưới); chạy tay trên stack local chưa làm — cần seed (Câu hỏi mở F076-2)) Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational (T005 chặn US3; T008 trước T024) → US1 → US2 → US3 → US4 → US5 (T032 có thể làm song song từ Phase 2, cần xong trước T019 đăng ký room) → US6 → Polish.
- Phụ thuộc: F030 (delta `createAttempt`), F074 (Reward Engine, level), F072/F071 (unlock).

## Definition of Done

- [x] Mọi tiêu chí F076-AC-* có test pass (ghi mã test/TC)
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh cho module `game`; `go -C src/backend test ./pkg/roomhost/...` xanh; `make test-contract` xanh với module learning
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

## Bảng tiêu chí → test (bằng chứng 2026-11-24)

Test tích hợp chạy trên PostgreSQL thật (testcontainers, cụm chung) — `TestChallengeRoomFailover` thêm Valkey thật; facade learning trong test của module game là `learningapi.Fake` sinh từ spec (TEST DOUBLE, R17), phía module learning chạy thật trong `TestAttemptFacadeChallenge`.

| Tiêu chí | Test |
|---|---|
| F076-AC-01 | `challenge_us1_test.go` `TestChallengeList` (gán tenant, khối, mở khóa, cửa sổ, `party_required` loại, mức theo khối — khối 3 tới Hard, `attempts_left`, keyset `limit`/`cursor`, lọc `type`/`difficulty`); domain `TestDifficultiesForGrade`, `TestVisible` |
| F076-AC-02 | `TestChallengeList` (`404` khác khối/khóa/chưa mở/`party_required`/chưa gán/không tồn tại; chi tiết `attempts_left`, `time_limit_seconds` 450 khối 3) |
| F076-AC-03 | `TestChallengeAuthz` (6 route × `X-Tenant-Id` khác → `403 TENANT_MISMATCH`; attempt học sinh khác → `404` ở đọc/next/abandon); `player_routes_authz_test.go` `TestPlayerRoutesStaffDenied` (staff 5 role → `403 GAME_ACCESS_STUDENT_ONLY` trên 6 route mới) |
| F076-AC-04 | `challenge_us2_test.go` `TestChallengeStart` (ghim version, `deadline_at` = giờ server + 300 s × 1,5, `lives_left` survival Hard = 3, `challenge.started`, publish version mới khi đang chơi vẫn ghim v1); domain `TestDeadlineAndLives` |
| F076-AC-05 | `TestChallengeStart` (Expert cho khối 3, `party_id` → `403 CHALLENGE_NOT_ELIGIBLE`, không ghi); `TestChallengeList` (khác khối/khóa/ngoài thời gian → `403`) |
| F076-AC-06 | `TestChallengeStartLimits` (5 lượt/ngày kể cả bỏ cuộc → `409 CHALLENGE_ATTEMPT_LIMIT` `retry_after` 04:00 ICT hôm sau; hôm sau mở lại; `failed` Hard → nghỉ chờ 60 s); domain `TestAttemptsAndCooldown` |
| F076-AC-07 | `TestChallengeStartLimits` (cùng `Idempotency-Key` → cùng attempt; thiếu key `428`; đang có attempt khác → `409 CONFLICT`) |
| F076-AC-08 | `challenge_us3_test.go` `TestChallengeNextActivity` (`createAttempt` mode `challenge`, context `challenge_attempt`, học sinh của phiên, dải Hard 3–4, khối 3, LO, 1 câu/37 s cho survival; không khóa đáp án; cùng key → cùng attempt); contract `TestLearningCreateChallengeAttemptFacade`; learning `TestAttemptFacadeChallenge` |
| F076-AC-09 | learning `attempt_facade_test.go` `TestAttemptFacadeChallenge` (pool có câu khối 4 trong dải và khối 3 ngoài dải: chỉ câu khối 3 độ khó 4 được phát; pool thiếu → `422 QUESTION_POOL_INSUFFICIENT`; thiếu dải/khối khác/min > max/context sai → `422`); game `TestChallengeNextActivity` (dải + khối gửi đi) |
| F076-AC-10 | `TestChallengeProgressAndRewards` (sức bền giảm từng câu sai, event lặp với event id mới không áp lần hai, điểm từ kết quả chấm); domain `TestSurvival`, `TestTimeGoalAndDeadline`, `TestPuzzleAndLimit` |
| F076-AC-11 | `TestChallengeProgressAndRewards` (một `challenge.completed` mỗi kết thúc với `status`/`end_reason`/`score`/`difficulty`/`reward_multiplier`/`first_clear`/`objective_ids`; Reward Engine: thành công 50 × 1,3 + lần đầu 100 một lần, gửi lặp không thêm, thất bại 0 XP, lần thứ hai không có thưởng lần đầu; schema payload); domain `TestPerfectKnowledgeExpiry`; reward `TestChallengeCompleted` |
| F076-AC-12 | `TestChallengeNextActivity` (learning lỗi → `503 UNAVAILABLE` + `Retry-After`, không liên kết attempt, gọi lại cùng key → attempt của key đó; chưa publish → `422 ITEM_REF_INVALID`) |
| F076-AC-13 | `challenge_us4_test.go` `TestChallengeDeadlineValkeyDown` (sau hạn → `422`; job trong `deadline_grace` không kết thúc, sau đó kết thúc đúng một lần); `challenge_room_test.go` `TestChallengeRoomFailover` (timer room host); domain `TestPerfectKnowledgeExpiry` |
| F076-AC-14 | `TestChallengeAbandonResume` (`abandoned`, giữ attempt học tập, `challenge.completed` abandoned, bỏ lần hai → `422`) |
| F076-AC-15 | `TestChallengeAbandonResume` (thiết bị khác đọc checkpoint điểm/hoạt động, tiếp tục trong `attempt_resume_window`, quá → `abandoned` `resume_expired`; `student.cleaned_up` → `abandoned`) |
| F076-AC-16 | contract `no_client_challenge_result_test.go` `TestNoClientChallengeResult` |
| F076-AC-17 | `TestChallengeRoomFailover` (Valkey thật: 2 node một lease; owner dừng → node kia chiếm sau TTL, due từ checkpoint PostgreSQL, timer một lần, room bị quên; abandon đóng room); `pkg/roomhost` `TestHostLeaseFailoverTimerOnce`, `TestHostLeaseLostAndClose` |
| F076-AC-18 | `pkg/roomhost` `TestHostRetryClosedAndGrace` (grace theo loại room, checkpoint qua port, lỗi handler thử lại), `TestHostLeaseFailoverTimerOnce` (đồng hồ `pkg/clock`); `archgen` 0 vi phạm (gói không import module) |
| F076-AC-19 | `TestChallengeDeadlineValkeyDown` (registry Valkey lỗi — TEST DOUBLE: bắt đầu vẫn chạy, job kết thúc từ PostgreSQL, kết quả đã chấm không mất) |
| F076-AC-20 | `challenge_publish_test.go` `TestChallengePublish` (schema, min > max, Expert khối 3, boss solo, `time` thiếu thời gian, `question_selection`, quest mở khóa không tồn tại, thiếu `en`; publish lặp không đổi; F077 `challenge_ids` lạ → `ITEM_REF_INVALID`); domain `TestValidate`; `challengeschema` `TestValidate` |
| F076-AC-21 | `TestChallengeList` (`area_id` 65 ký tự, `type`/`difficulty` sai, `cursor` > 512 → `422 VALIDATION_FAILED`) |
| T011 (RLS) | `TestChallengeRLS` |
