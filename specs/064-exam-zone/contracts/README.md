# Contracts: F064 — Exam Zone: thi có giờ theo server, tự nộp, resume, thông báo bài mới

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/eduworld-api.v1.yaml` (bundle), `specs/openapi/modules/learning.v1.yaml`. Route `/exams/*` từ game qua middleware `game_player`; học sinh chỉ thấy đề được giao và attempt của mình.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listExamSessions` | `GET /api/v1/exams/sessions` | eduworld-api | `game_player` (đề được giao) | Có sẵn |
| `getExamSession` | `GET /api/v1/exams/sessions/{exam_session_id}` | eduworld-api | `game_player` + được giao | Cần sửa (§6) |
| `startExamAttempt` | `POST /api/v1/exams/sessions/{exam_session_id}/attempts` | eduworld-api | `game_player` + được giao | Cần sửa (§6) |
| `saveExamAnswer` | `PUT /api/v1/exams/attempts/{attempt_id}/answers/{position}` | eduworld-api | `game_player` + owner | Có sẵn |
| `submitExamAttempt` | `POST /api/v1/exams/attempts/{attempt_id}:submit` | eduworld-api | `game_player` + owner | Có sẵn |
| `getExamResult` | `GET /api/v1/exams/attempts/{attempt_id}/result` | eduworld-api | `game_player` + owner | Có sẵn |
| `listMyAssignments` | `GET /api/v1/learning/my-assignments` | eduworld-api | `game_player` | Cần sửa (chung F062) |
| `startAdHocAttempt` | `POST /api/v1/learning/activity-attempts` | eduworld-api | `game_player` | Có sẵn ("Luyện LO này") |

Schema dùng: `ExamSession`, `Attempt`, `ActivityResponseSubmission`, `AttemptResult`, `AttemptReview`, `MyAssignment`, `Problem` (`EXAM_NOT_ASSIGNED`, `EXAM_NOT_OPEN`, `EXAM_WINDOW_CLOSED`, `EXAM_MAX_ATTEMPTS`, `EXAM_ACTIVE_ON_OTHER_DEVICE`, `EXAM_TIME_OVER`, `EXAM_ALREADY_SUBMITTED`, `EXAM_RESULTS_NOT_RELEASED`, `GAME_ACCESS_STUDENT_ONLY`).

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Client không gọi internal API | | | |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `notification.item.created` | server → client | Có sẵn — **không dùng** ở M-C3 (chưa có realtime F090); F092/F119 thay cơ chế làm mới |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Client không phát/tiêu thụ | | |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/assessment-blueprint.schema.json` (`navigation`) | Nguồn giá trị `navigation` cho `ExamSession` | Có sẵn |

## 6. Delta cần thực hiện

- **Cần thêm** `ExamSession.navigation` (`enum [free, sequential_no_return]`, lấy từ blueprint version đã ghim). Task T004.
- **Cần thêm** body tùy chọn `{takeover: boolean}` cho `startExamAttempt` để xử lý `EXAM_ACTIVE_ON_OTHER_DEVICE` "chuyển sang thiết bị này" ([EXAM_ENGINE §12](../../../docs/02-learning/EXAM_ENGINE.md)); sau khi có, thêm nút chuyển thiết bị (task T031). Task T004.
- **Cần sửa** `MyAssignment` thêm `exam_session_id`, `result_release`, `late_policy` — cùng PR với F062 (T004 của F062).
