# Implementation Plan: F076 — Challenge Engine solo, 5 mức khó, room host dùng chung

**Branch**: `076-challenge-engine-solo` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Thêm aggregate `ChallengeAttempt` và module config `challenges` vào module game; tạo `src/backend/pkg/roomhost` dùng chung. `StartGameChallenge` (idempotent) kiểm eligibility (khối từ `ref.students`, `unlock_rule` từ progression/world/quest của module game, availability, lượt/cooldown), ghim version, tính `deadline_at`, đăng ký room trong `src/backend/pkg/roomhost`. `NextChallengeActivity` gọi `createAttempt` của module learning qua facade `learning` (`src/backend/pkg/modcall`, mode `challenge`). Consumer `game.challenge_progress` nhận `activity.graded`, cập nhật điểm/sức bền/mục tiêu, kết thúc và phát `challenge.completed` qua outbox; Reward Engine (F074) cấp thưởng. Timer room host + job `challenge-deadline` kết thúc theo giờ server.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen (server game + client `learning.internal`), `src/backend/pkg/modcall`, `src/backend/pkg/entx` (`UnitOfWork`), `src/backend/pkg/idempotency`, `src/backend/pkg/outbox`, `src/backend/pkg/clock`, Valkey client |
| Storage | PostgreSQL `eduworld_game` (`game.challenges`/`challenge_versions` G, `game.challenge_attempts` T); Valkey `ew:rt:room:<id>`, `ew:rt:room:<id>:lease`, `ew:rt:user:<id>` |
| Testing | go test + testcontainers (PostgreSQL, NATS, Valkey); facade test với module learning; fake clock |
| Target Platform | Linux container (`core-api` + `worker`, module `game`) |
| Project Type | backend module + platform kit (`src/backend/pkg/roomhost`) |
| Performance Goals | `startGameChallenge`, `nextChallengeActivity` p95 ≤ 500 ms (gồm gọi learning, timeout 2 s); failover room ≤ 15 s (SC-004) |
| Constraints | AGENTS R5, R6, R8, R10, R11, R13, R26, R29, R31; ADR-0016, ADR-0040, ADR-0043, ADR-0048 |
| Scale/Scope | 10.000 CCU; tối đa một challenge `in_progress`/học sinh |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta `learning.internal` (`createAttempt` mode `challenge`), payload event, trigger reward ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Gọi learning qua facade `learning`; kết quả qua event; `src/backend/pkg/roomhost` không nghiệp vụ |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `challenge_attempts` T + RLS; test isolation mọi route |
| IV Server-authoritative | ✅ | Chấm ở module learning; thời gian server; `Idempotency-Key`; render không đáp án |
| V Xác thực, phân quyền | ✅ | `game_player`; config `game.challenge.write` qua F078; gọi facade `learning` với caller `game` trong `x-allowed-callers` |
| VI An toàn trẻ em | ✅ | Thất bại không bêu tên, thông điệp thân thiện; không log PII |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số mức khó từ challenge version + CONFIG_DEFAULTS |
| VIII UI tách logic, token | ✅ | Không có UI |
| IX Test có bằng chứng | ✅ | Test mỗi F076-AC-*, test lease/failover, facade test learning |

## Project Structure

### Documentation (feature này)

```text
specs/076-challenge-engine-solo/
├── spec.md · plan.md · research.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
src/backend/pkg/roomhost/                         # host.go, lease.go, timer.go, checkpoint.go (port), grace.go, publisher.go, roomhost_test.go
src/backend/modules/game/
├── ent/schema/                       # challenge.go, challengeversion.go, challengeattempt.go
├── internal/domain/challenge/        # ChallengeVersion, DifficultyProfile, Eligibility, Attempt (Apply(graded), End), Rules theo type
├── internal/application/command/     # start_challenge.go, next_challenge_activity.go, apply_challenge_result.go,
│                                     # abandon_challenge.go, expire_challenge.go
├── internal/application/query/       # list_challenges.go, get_challenge.go, get_challenge_attempt.go
├── internal/application/port/        # LearningAttemptClient, ChallengeRepository, RoomHost
├── internal/infrastructure/modules/learning.go         # adapter facade learning (sinh từ learning.internal.v1.yaml)
├── internal/infrastructure/room/challenge_room.go   # RoomHandler + CheckpointStore (PostgreSQL)
├── internal/infrastructure/gameconfig/challenge_module.go
├── internal/presentation/http/public/challenges.go
├── internal/presentation/consumer/challenge_progress.go
├── internal/presentation/job/challenge_deadline.go
└── tests/{unit,integration,contract}/
```

**Structure Decision**: `src/backend/pkg/roomhost` là gói riêng trong `src/backend/pkg/` (cùng Go module `src/backend`, `src/backend/go.mod` duy nhất) để module competition dùng lại không import module game (R13); handler room của challenge nằm ở infrastructure module game.

## Phase 0: Research

→ [research.md](research.md): R1 room host cho challenge solo; R2 ranh giới `src/backend/pkg/roomhost`; R3 cập nhật tiến độ từ `activity.graded`.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `challenge.Eligibility.Check(student, version, now, attemptsUsed, lastFail)`, `challenge.Attempt.ApplyGraded(result)`, luật `time`/`perfect`/`survival`/`puzzle`/`knowledge`/`weekly`/`seasonal`, `DifficultyProfile.ForGrade(grade)` | Thuần, Clock inject |
| application | Command `StartChallenge`, `NextChallengeActivity`, `ApplyChallengeResult`, `AbandonChallenge`, `ExpireChallenge` · Query `ListChallenges`, `GetChallenge`, `GetChallengeAttempt` | Học sinh self; idempotency middleware `src/backend/pkg/httpx` |
| infrastructure | ent repo; adapter facade `learning` (deadline 2 s từ context, `Idempotency-Key`; không retry/circuit breaker — ADR-0074); room handler + checkpoint PostgreSQL; module config | |
| presentation | `listGameChallenges`, `getGameChallenge`, `startGameChallenge`, `getGameChallengeAttempt`, `nextChallengeActivity`, `abandonGameChallengeAttempt` · consumer `game.challenge_progress` · job `challenge-deadline` | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F072 (quest hoàn thành cho `unlock_rule`), F074 (level cho `unlock_rule`, Reward Engine), F071 (area mở khóa), F030 (`createAttempt`, `activity.graded`), F014 (game session).
- Facade: `learning` `createAttempt`, `getAttemptResult` (R29).
- Event phát: `challenge.started`, `challenge.completed`. Tiêu thụ: `activity.graded`, `student.cleaned_up`, `user.access_revoked`.
- Realtime: `challenge.attempt.updated` (Valkey `ew:rt:user:<student_id>`).

### Bảo mật, tenant, idempotency, audit

- `game_player` + `tenant` middleware; `TenantScope`; truy cập attempt theo `student_id = principal.user_id`.
- `Idempotency-Key` bắt buộc: `startGameChallenge`, `nextChallengeActivity`, `abandonGameChallengeAttempt`; truyền tiếp `Idempotency-Key` dẫn xuất sang `createAttempt`.
- Audit: không ghi từng lượt chơi (dữ liệu nghiệp vụ là vết); từ chối nhân sự ghi `authz.denied` theo middleware chung.

### Quan sát

- Metric `eduworld_game_challenge_started_total{type,difficulty}`, `eduworld_game_challenge_completed_total{status}`, `eduworld_roomhost_lease_takeover_total`, `eduworld_roomhost_timer_lag_seconds`; span `challenge.next_activity` gồm call learning.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
