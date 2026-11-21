# Contracts: F105 — Battle engine server-authoritative và 8 chế độ thi đấu

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/modules/competition.v1.yaml`, `x-service: competition`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `getMatch` | `GET /api/v1/competition/matches/{match_id}` | `competition.v1.yaml` (`Match`) | học sinh participant (`game_player`) / nhân sự `lms.student.read` (portal, trường trong scope; đổi tên theo ADR-0095) | Cần sửa |
| `listMyMatches` | `GET /api/v1/competition/matches?mode=` | `competition.v1.yaml` | học sinh (self) | Có sẵn |
| `getRoom` | `GET /api/v1/multiplayer/rooms/{room_id}` | `competition.v1.yaml` (`Room`) | thành viên room | Có sẵn (dùng lại) |

## 2. Internal API

Spec: `specs/openapi/modules/learning.internal.v1.yaml`.

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `createQuestionPlan` | competition → learning | `learning.internal.v1.yaml` (`QuestionPlanRequest`, `QuestionPlan`) | `[competition]` | Có sẵn (bên nhận làm ở F105) |
| `scorePlanAnswer` | competition → learning | `learning.internal.v1.yaml` (`PlanAnswerScoreRequest`, `PlanAnswerScore`) | `[competition]` | Có sẵn (bên nhận làm ở F105) |
| `revealPlanItem` | competition → learning | — | `[competition]` | Cần thêm |

## 3. Realtime (AsyncAPI)

Spec: `specs/asyncapi/eduworld-realtime.v1.yaml`; gateway định tuyến `battle.*`, `coop.*` → NATS `ew.cmd.competition.<type>`.

| Message | Hướng | Trạng thái |
|---|---|---|
| `battle.answer.submit`, `battle.state.request` | client → server | Có sẵn |
| `battle.state.snapshot`, `battle.countdown.started`, `battle.question.shown`, `battle.answer.recorded`, `battle.answer.evaluated`, `battle.question.revealed`, `battle.round.ended`, `battle.participant.status_changed`, `battle.match.paused`, `battle.match.resumed` | server → client | Có sẵn |
| `battle.match.ended` | server → client | Cần sửa (`outcome` thêm `abandoned`, `forfeit`; thêm `end_reason`; `timed_quiz` thêm `top[]`, `my_rank`, `grade_goal {current, target, contributors_count?}` — lớp 1–3 chỉ `grade_goal`, không `top[]`; thay `class_goal` theo ADR-0095) |
| `coop.action.perform`, `coop.answer.submit` / `coop.state.snapshot`, `coop.state.updated`, `coop.task.assigned`, `coop.answer.evaluated`, `coop.progress.updated`, `coop.session.ended` | hai chiều | Có sẵn |
| `ErrorCode` `ANSWER_LATE`, `ANSWER_ALREADY_SUBMITTED`, `MATCH_NOT_ACTIVE`, `ROOM_NOT_MEMBER`, `ROOM_FULL`, `PERMISSION_DENIED`, `VALIDATION_FAILED` | server → client | Có sẵn |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `match.started`, `match.completed`, `match.aborted` | Phát (`EVT_COMPETITION`) | `specs/events/payloads/competition/match.<verb>.v1.schema.json` | Cần thêm |
| `match.created` | Tiêu thụ nội bộ (F104 phát) | `specs/events/payloads/competition/match.created.v1.schema.json` | Cần thêm (F104) |
| `game_session.replaced` | Tiêu thụ (module game) | — | Có sẵn (áp 2026-11-17, owner F014) |
| `audit.recorded` | Phát | envelope | Có sẵn |
| learning event `question.answered` (`context.mode = competition`, `context.match_id`) | module learning phát khi chấm | `specs/schemas/learning-event.schema.json` | Có sẵn |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/question.schema.json` (`$defs.response`, phần render) | Response trong `battle.answer.submit`, render payload câu | Có sẵn |

## 6. Delta cần thực hiện

- `learning.internal.v1.yaml`: thêm operation facade `revealPlanItem` (`plan_id`, `index`; `x-allowed-callers: [competition]`, `409` khi plan chưa đóng câu — module competition gửi `closed_at`) → `PlanItemReveal {plan_id, index, correct_response ($ref QuestionResponse), explanation (object, maxProperties 20, chuỗi maxLength 2000)}`; `QuestionPlanRequest.mode` thêm mô tả trận bot; `PlanItem` xác nhận không có trường đáp án.
- `competition.v1.yaml`: `Match` thêm `end_reason`, `rated`, `review[]` (`{index, question (render), correct_response, explanation, my_answer_correct, my_points}`, chỉ khi `status = completed` và người gọi là participant); bỏ `participants[].rating_delta` (số rating thô, AC-CMP-102 — cùng delta với F106); `outcome` thêm `forfeit`, `no_contest`; thêm `403` cho `getMatch`.
- `eduworld-realtime.v1.yaml`: `battle.match.ended` như §3; `battle.state.snapshot.mode` thêm `coop_challenge` không cần (co-op dùng `coop.*`).
- Event payload schema mới `match.started.v1`, `match.completed.v1`, `match.aborted.v1` (trường ở [data-model §4](../data-model.md#4-event--outbox)); mọi string có `maxLength`.
- Tài liệu: DATABASE_DESIGN §2.6 thêm `assessment.question_plans`, §2.9/§2.10 cột mới; EVENT_DESIGN §4.8 payload `match.completed` thêm `ranked`, `rated`, `end_reason`, `tournament_node_id`.
