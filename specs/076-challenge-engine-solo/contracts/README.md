# Contracts: F076 — Challenge Engine solo, 5 mức khó, room host dùng chung

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/eduworld-api.v1.yaml` → `specs/openapi/modules/game.v1.yaml`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listGameChallenges` | `GET /api/v1/game/challenges?type=&area_id=&difficulty=` → `GameChallengeSummary[]` | game.v1.yaml | `game_player` | Có sẵn |
| `getGameChallenge` | `GET /api/v1/game/challenges/{challenge_id}` → `GameChallengeDetail` | game.v1.yaml | `game_player` | Có sẵn |
| `startGameChallenge` | `POST /api/v1/game/challenges/{challenge_id}:start` (`Idempotency-Key`, body `difficulty`) → `201 GameChallengeAttempt` | game.v1.yaml | `game_player` | Cần sửa (khai báo `422`, `428`) |
| `getGameChallengeAttempt` | `GET /api/v1/game/challenge-attempts/{challenge_attempt_id}` | game.v1.yaml | `game_player` (self) | Có sẵn |
| `nextChallengeActivity` | `POST /api/v1/game/challenge-attempts/{challenge_attempt_id}/activities:next` (`Idempotency-Key`) → `ChallengeNextActivity` | game.v1.yaml | `game_player` (self) | Cần sửa (khai báo `503`, `428`) |
| `abandonGameChallengeAttempt` | `POST /api/v1/game/challenge-attempts/{challenge_attempt_id}:abandon` (`Idempotency-Key`) | game.v1.yaml | `game_player` (self) | Có sẵn |
| `/game-admin/challenges` | workflow ở F078 | game.v1.yaml | `game.challenge.write` | Có sẵn — F076 đăng ký module |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `createAttempt` | game → learning | `specs/openapi/modules/learning.internal.v1.yaml` | `[game]` | Cần sửa (mode `challenge`, ngữ cảnh, dải độ khó) |
| `getAttemptResult` | game → learning (đối soát) | learning.internal.v1.yaml | `[game]` | Có sẵn |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `challenge.attempt.updated` | server → học sinh (Valkey `ew:rt:user:<student_id>`) | Có sẵn (`specs/asyncapi/eduworld-realtime.v1.yaml`) |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `challenge.started` | game → LA, ANA | `specs/events/payloads/game/challenge.started.v1.schema.json` | Cần thêm |
| `challenge.completed` | game → reward (F074), achievement (F075), QST (F072), LA, ANA | `specs/events/payloads/game/challenge.completed.v1.schema.json` | Cần thêm |
| `activity.graded` | learning → game (`game.challenge_progress`) | `specs/events/payloads/learning/activity.graded.v1.schema.json` | Có sẵn |
| `student.cleaned_up`, `user.access_revoked` | identity → game | `specs/events/payloads/identity/` | Cần thêm (owner F016/F011) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/challenge.schema.json` | Version `challenges` | Có sẵn |
| `specs/schemas/reward-rule.schema.json` | Trigger `challenge.completed` | Cần sửa |

## 6. Delta cần thực hiện

- `learning.internal.v1.yaml` (owner module learning, F030): `AttemptCreateRequest.mode` cho phép `challenge` với caller `game`; `context.context_type` thêm `challenge_attempt` (`context_id` = `challenge_attempt_id`; delta do F030 task T052 làm); `ActivityRef` thêm `difficulty_min`, `difficulty_max` (1–5), `grade_level` (1–9), `objective_ids` (`maxItems: 30`); `ActivityRef.content_type` cho phép `activity` (activity_ref theo ADR-0048). Facade test game ↔ learning (bên gọi với fake facade, bên nhận theo ví dụ spec internal).
- `game.v1.yaml`: `startGameChallenge` khai báo `422 VALIDATION_FAILED`, `428 PRECONDITION_REQUIRED`; `nextChallengeActivity` khai báo `428`, `503 UNAVAILABLE`.
- `reward-rule.schema.json`: `trigger.event_type` thêm `challenge.completed`; `conditions` thêm `challenge_statuses` (enum `succeeded`/`failed`) và `grants[].scale_by` (enum `reward_multiplier`).
- Payload mới `challenge.started.v1`, `challenge.completed.v1` (§4).
- Kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`, `make gen MOD=learning`, `make test-contract`.
