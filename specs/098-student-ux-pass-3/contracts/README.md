# Contracts: F098 — Student UX pass 3: chạm là trả lời, ma trận kiểm tra 4 khối

> Feature chỉ đổi tương tác và giao diện phía client; không đổi hợp đồng giữa client và server.

## 1. API (OpenAPI)

Không có operation mới hay thay đổi. Chạm là trả lời dùng đúng `submitActivityResponse` (`POST /learning/activity-attempts/{attempt_id}/responses`, `Idempotency-Key`) như nút "Kiểm tra" của F063. Client đọc thêm trường **có sẵn** `ActivityResponseResult.correct_response` (`QuestionResponse | null`, "chỉ trả khi FeedbackTiming = immediate và chính sách practice cho phép") để đánh dấu thẻ đáp án đúng; `null`/vắng thì không đánh dấu. Quyết định chạm-là-trả-lời đọc `Attempt.mode`, `Attempt.feedback_timing`, `Attempt.result_release` (có sẵn). Payload hiển thị câu hỏi không chứa đáp án (R6) — giữ nguyên.

## 2. Internal API

Không có.

## 3. Realtime (AsyncAPI)

Không có.

## 4. Event (NATS)

Không có.

## 5. JSON Schema nội dung

Không có. Hợp đồng nội bộ client: token `component.answer_tile.*` ↔ Theme Godot + `UiTokens` (`make design-tokens-check`); khóa `.po` ↔ `messages.pot` (`extract_pot --check`); tín hiệu renderer `choice_tapped(repeated)` (QuestionRendererBase → ActivityRendererBase → ActivityHost); cài đặt client `eduworld/learning/tap_confirm_bands`.

## 6. Delta cần thực hiện

- Không có delta contract API. Delta token: `tokens@0.15.0` (`design/CHANGELOG.md`). Delta cấu hình client: `eduworld/learning/tap_confirm_bands` (CONFIG_DEFAULTS §13).
