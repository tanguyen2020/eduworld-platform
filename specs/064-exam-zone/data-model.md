# Data Model: F064 — Exam Zone: thi có giờ theo server, tự nộp, resume, thông báo bài mới

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — F064 là game client, không tạo/sửa bảng. Exam session, attempt, câu trả lời, kết quả lưu ở module learning (`assessment.exam_sessions`, `assessment.attempts`, `assessment.attempt_answers` — F042). Đề và câu trả lời chỉ giữ trong RAM ([GODOT_ARCHITECTURE §8.2](../../docs/08-game-client/GODOT_ARCHITECTURE.md)).

Mô hình client (tham khảo):

| Model | Nguồn (OpenAPI) | Trường chính | Lưu ở |
|---|---|---|---|
| `ExamSessionModel` | `ExamSession` | `exam_session_id`, `title`, `type`, `is_official`, `window_start/end`, `duration_seconds`, `question_count`, `max_attempts`, `attempts_used`, `status`, `feedback_timing`, `result_release`, `navigation`* | RAM |
| `AttemptModel` (F063) | `Attempt` | `attempt_id`, `deadline_at`, `server_time`, `items[]`, `saved_responses[]` | RAM |
| `ExamRunnerState` | — | `current_position`, `flagged: Set[int]`, `answered: Set[int]` | RAM (cờ đánh dấu không gửi server) |
| `ExamResultModel` | `AttemptReview` | điểm, item, `weak_objectives`, `recommendations` | RAM |
| `InGameNotice` | `MyAssignment` diff | `assignment_id`, `kind = assignment_new`, `seen` | RAM (mất khi đăng xuất) |

\* Cần thêm vào contract — [contracts/README.md](contracts/README.md) §6.

## 2. Vòng đời / trạng thái

`ExamRunner`: `rules → starting → running ⇄ offline → (submitting | time_up) → awaiting_result → result | results_pending`. `ExamSession.status` và `AttemptStatus` do server quyết định.

## 3. Quy tắc kiểm tra dữ liệu

- `remaining_ms = max(0, deadline_at − server_now)`; không dùng `duration_seconds` để tính hết giờ.
- `PendingWrites` gộp lưu đáp án theo `position` (chỉ giữ bản mới nhất), flush theo thứ tự thời gian.

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| — (client không phát domain event; server phát `exam.submitted`, `exam.graded`) | | |
