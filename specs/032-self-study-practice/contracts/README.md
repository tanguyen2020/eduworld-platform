# Contracts: F032 — Chế độ luyện tập, hỗ trợ tự học (gợi ý, chẩn đoán, `rapid_guess`), gợi ý ôn tập tự động

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/modules/learning.v1.yaml` (bundle `specs/openapi/eduworld-api.v1.yaml`). Mọi route là route học sinh (principal `user_type = student`, self-only, header `X-Tenant-Id`).

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `startAdHocAttempt` | `POST /api/v1/learning/activity-attempts` | `learning.v1.yaml` | học sinh (self) · `Idempotency-Key` | Cần sửa (`AdHocAttemptStart.source`: thêm `grade_level`, `topic_ids[]` maxItems 4) |
| `startActivityAttempt` | `POST /api/v1/learning/activities/{activity_id}/attempts` | `learning.v1.yaml` | học sinh (self) | Có sẵn (dùng `context.recommendation_id`) |
| `revealNextHint` | `POST /api/v1/learning/activity-attempts/{attempt_id}/items/{position}/hints:next` | `learning.v1.yaml` (`HintResult`) | học sinh (self) · `Idempotency-Key` | Có sẵn |
| `listMyWrongQuestions` | `GET /api/v1/learning/wrong-questions` | `learning.v1.yaml` (`WrongQuestion`) | học sinh (self) | Có sẵn (kiểm thêm `next_due_at`) |
| `getMyRecommendations` | `GET /api/v1/learning/recommendations/me` | `learning.v1.yaml` (`RecommendationList`) | học sinh (self) | Có (F032) |
| `getMyProgress` | `GET /api/v1/learning/progress/me` | `learning.v1.yaml` (`LearningProgress`) | học sinh (self) | Có sẵn |
| `getMyMastery` | `GET /api/v1/learning/mastery/me?subject_id=&grade_level=` | `learning.v1.yaml` (`MyMasteryList`) | học sinh (self) | Có (F032) |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không có (gợi ý đọc qua REST; thông báo trong game thuộc F123) | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `question.answered` (learning event) | Phát (attempt) / tiêu thụ nội bộ (mastery, danh sách ôn, gợi ý) | `specs/schemas/learning-event.schema.json` | Có sẵn |
| `activity.submitted` | Tiêu thụ nội bộ | `specs/events/payloads/learning/activity.submitted.v1.schema.json` | Có sẵn |
| `activity.graded` | Tiêu thụ nội bộ (diagnostic prior, gợi ý) | `specs/events/payloads/learning/activity.graded.v1.schema.json` | Có sẵn |
| `activity.started` | Phát qua engine F030 (F032 truyền `recommendation_id`) | `specs/events/payloads/learning/activity.started.v1.schema.json` | Cần thêm (owner F030 T052) |
| `activity.submitted`, `attempt.re_evaluated` | Tiêu thụ nội bộ | `specs/events/payloads/learning/<event>.v1.schema.json` | Cần thêm (thuộc F030/F031; F032 chỉ tiêu thụ) |
| `learning.recommendation_generated` | Phát (consumer: RT, LMS) | `specs/events/payloads/learning/learning.recommendation_generated.v1.schema.json` | Cần thêm |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/question.schema.json` (`hints[]`, `min_response_ms`, `options[].feedback`) | Gợi ý, ngưỡng `rapid_guess`, phản hồi phương án | Có sẵn |
| `specs/schemas/learning-event.schema.json` | `hints_used`, `response_ms`, `rapid_guess`, `context.mode = diagnostic` | Có sẵn |

## 6. Delta cần thực hiện

- `RecommendationList.items[]`: thêm `recommendation_id` (uuid, bắt buộc), `rule_code` (enum `R1`…`R9`), `reason_code` (string, `maxLength: 64`), `reason_text` (string, `maxLength: 300`, theo `Accept-Language`), `priority` (integer 1–9); mở rộng `kind` thêm `assignment`, `prerequisite_review`, `diagnostic`, `interleave_practice`, `quest`; mở rộng `reason` thêm `prerequisite_weak`, `decay_review`, `diagnostic_due`, `interleave_ready`, `exam_followup`, `next_topic`; `items.maxItems` giữ 20, server trả ≤ 3 `active`.
- Thêm `GET /learning/mastery/me` (`getMyMastery`, `x-service: learning`): query `subject_id` (uuid), `grade_level`; response `items[] {objective_id, objective_name (maxLength 255), level (enum MasteryLevel), icon_code (maxLength 64), value (number|null — chỉ khối 6–9), evidence_count, last_evidence_at}`.
- `AdHocAttemptStart.source`: thêm `grade_level` (`GradeLevel`) cho `diagnostic`, `topic_ids[]` (uuid, maxItems 4) cho `interleave`.
- `ProblemCode`: thêm `ITEM_POSITION_INVALID` (ACTIVITY_ENGINE §9 đã dùng, enum chưa có) — đồng bộ catalog API_DESIGN §4.1.
- Event payload schema mới: `learning.recommendation_generated.v1` (`activity.started.v1` + `recommendation_id?` thuộc F030) (`student_id`, `recommendation_ids[]`, `rule_codes[]`, `objective_ids[]`, `content_refs[]`, `generated_at`; mọi string có `maxLength`).
