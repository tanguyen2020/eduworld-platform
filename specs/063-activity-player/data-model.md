# Data Model: F063 — Activity player: renderer registry, renderer 11 loại câu hỏi, gợi ý, kết quả, công thức SVG

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — F063 là game client, không tạo/sửa bảng. Attempt, câu trả lời, gợi ý, evaluation lưu ở module learning (`assessment.attempts`, `assessment.attempt_items`, `assessment.attempt_answers`, `assessment.evaluations` — F030). Render payload chỉ giữ trong RAM trong attempt ([GODOT_ARCHITECTURE §8.1](../../docs/08-game-client/GODOT_ARCHITECTURE.md)).

Mô hình client (tham khảo):

| Model | Nguồn (OpenAPI) | Ghi chú | Lưu ở |
|---|---|---|---|
| `AttemptModel` | `Attempt` | `attempt_id`, `mode`, `status`, `deadline_at`, `server_time`, `feedback_timing`, `result_release`, `items[]`, `saved_responses[]`, `render`, `display_language` | RAM |
| `QuestionRenderModel` | `QuestionRender` | `position`, `question_version_id`, `type`, `render_payload`, `media[]`, `max_score`, `hint_count`* | RAM |
| `ResponseResultModel` | `ActivityResponseResult` | `is_correct`, `score`, `explanation`, `feedback`, `hints_used`, `correct_response` | RAM |
| `HintResultModel` | `HintResult` | `level`, `total_levels`, `hint.text`, `hint.math_assets` | RAM |
| `AttemptReviewModel` | `AttemptReview`, `AttemptResult` | điểm, item, `weak_objectives`, `recommendations`, `rewards` | RAM |
| `PendingWrite` | — | `{method, path, body, idempotency_key, enqueued_at}` | RAM (không ghi đĩa) |
| SVG cache | `MathAssetRef` | khóa `sha256`, bất biến | `user://cache/math/<sha256>.svg` |

\* Cần thêm vào contract — [contracts/README.md](contracts/README.md) §6.

## 2. Vòng đời / trạng thái

`ActivityHost`: `loading → presenting(position) → awaiting_result → feedback → … → submitting → result | results_later | error | update_required`. Trạng thái attempt (`AttemptStatus`) do server quyết định.

## 3. Quy tắc kiểm tra dữ liệu

- Answer model chỉ tạo response đúng shape `QuestionResponse` theo `type` (Bước 0 S1).
- Không đọc/hiển thị khóa đáp án nếu (do lỗi) xuất hiện trong payload; contract test quét fixture (F063-AC-03).

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| — (client không phát domain event; server phát `exercise.*`, `question.answered`) | | |
