# Contracts: F042 — Exam Engine: 7 hình thức đề, random theo blueprint, thi có giờ, chấm lại đề

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/modules/learning.v1.yaml` (bundle `specs/openapi/eduworld-api.v1.yaml`), `x-service: learning`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `createExamSession` | `POST /api/v1/exams/sessions` | `learning.v1.yaml` (`ExamSessionCreate`) | `lms.assignment.write` · `Idempotency-Key` | Cần sửa |
| `listExamSessions` | `GET /api/v1/exams/sessions` | `learning.v1.yaml` | học sinh (đề được giao) / `lms.assignment.read` | Có sẵn |
| `getExamSession` | `GET /api/v1/exams/sessions/{exam_session_id}` | `learning.v1.yaml` (`ExamSession`) | học sinh (đối tượng) / `lms.assignment.read` | Có sẵn |
| `startExamAttempt` | `POST /api/v1/exams/sessions/{exam_session_id}/attempts` | `learning.v1.yaml` (`Attempt`) | học sinh (self) · `Idempotency-Key` | Cần sửa (thiết bị) |
| `saveExamAnswer` | `PUT /api/v1/exams/attempts/{attempt_id}/answers/{position}` | `learning.v1.yaml` | học sinh (self) | Có sẵn |
| `submitExamAttempt` | `POST /api/v1/exams/attempts/{attempt_id}:submit` | `learning.v1.yaml` | học sinh (self) · `Idempotency-Key` | Có sẵn |
| `getExamResult` | `GET /api/v1/exams/attempts/{attempt_id}/result` | `learning.v1.yaml` (`AttemptReview`) | học sinh (self) / `lms.assignment.read` | Có sẵn |
| `releaseExamResults` | `POST /api/v1/exams/sessions/{exam_session_id}:release-results` | `learning.v1.yaml` | `lms.result.release` · `Idempotency-Key` | Có sẵn |
| `createRegradeRequest` | `POST /api/v1/cms/learning/regrade-requests` (`scope = exam_session`) | `learning.v1.yaml` (`RegradeRequestCreate`) | `learning_content.re_evaluate` · `Idempotency-Key` | Có sẵn |
| `listExamLibrary` | `GET /api/v1/exams/library?subject_id=&grade_level=&type=` | — | học sinh | Cần thêm |
| `listExamSessionPapers` | `GET /api/v1/exams/sessions/{exam_session_id}/papers` | — | `lms.assignment.write` | Cần thêm |
| `getExamAttemptAnalysis` | `GET /api/v1/exams/attempts/{attempt_id}/analysis` | — | học sinh (self) / `lms.assignment.read` (SA, AM trong phạm vi) | Cần thêm |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có (adapter `ExamSessionCreator` cho port của F041 gọi command trong cùng module) | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không có; đồng hồ client dựa trên `deadline_at` + `server_time` qua REST | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `exam.started`, `exam.submitted`, `exam.graded`, `exam.results_released`, `exam.regraded` | Phát (`EVT_LEARNING`) | `specs/events/payloads/learning/<event>.v1.schema.json` | Cần thêm |
| `attempt.re_evaluated` | Phát | `specs/events/payloads/learning/attempt.re_evaluated.v1.schema.json` | Cần thêm (owner F031) |
| `question.answered` (learning event) | Phát khi chấm | `specs/schemas/learning-event.schema.json` | Có sẵn |
| `exam.opened`, `exam.closed` | Phát | `specs/events/payloads/learning/exam.opened.v1.schema.json`, `exam.closed.v1.schema.json` | Cần thêm |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/assessment-blueprint.schema.json` | Sections, rules, `variants`, `result_release`, `feedback_timing`, `attempt_policy` | Có sẵn |
| `specs/schemas/question.schema.json` (`$defs.response`) | Câu trả lời `saveExamAnswer` | Có sẵn |

## 6. Delta cần thực hiện

- `ExamSessionCreate`: thêm `accommodations[] {student_id (uuid), extra_time_percent (integer 1–100)}` (maxItems 200), `late_start_policy` (enum `full_duration`, `until_window_end`), `audience` (enum `assigned`, `library`), `show_correct_answers`, `show_explanations` (boolean), `score_policy`; xác nhận `type` dùng `ExamType`. `ExamSession` thêm `accommodations`, `late_start_policy`, `audience`, `server_time`.
- `startExamAttempt`: thêm header/body `device_token` (string, `maxLength: 128`) và `take_over` (boolean) để chuyển thiết bị; response `Attempt` có `deadline_at`, `server_time`; thêm response `409 EXAM_ACTIVE_ON_OTHER_DEVICE`.
- Thêm `GET /exams/library` (`listExamLibrary`, cursor, `ExamSession[]`) và `GET /exams/sessions/{exam_session_id}/papers` (`listExamSessionPapers`, `ExamPaper {exam_paper_id, variant_no, sections[] {section_id, title (maxLength 255), items[] {position, question_version_id, points}}}`).
- `AttemptReview` (kết quả đề): xác nhận có `section_scores[]`, `objective_breakdown[]`, `score_scaled`; thêm nếu thiếu.
- Event payload schema mới cho `exam.*`, `attempt.re_evaluated`; bổ sung `exam.opened`, `exam.closed` vào EVENT_DESIGN §4.3.
- Thêm `GET /exams/attempts/{attempt_id}/analysis` (`getExamAttemptAnalysis`, schema `ExamAttemptAnalysis {objective_breakdown[], wrong_items[], review_suggestions[] (maxItems 3)}`, `403 EXAM_RESULTS_NOT_RELEASED` khi chưa công bố) theo API_DESIGN §9.5 (RB-213); `/exams/sessions/{id}/report` (`getExamSessionReport`) thuộc F044. Task T003.
