# Research: F065 — Bài học video: plugin `video_lesson` và renderer Godot

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Lưu câu trả lời câu chèn

- **Decision**: Mỗi câu chèn (`embedded_questions[]`) là một item của attempt: `assessment.attempt_items` (`position` = thứ tự theo `at_ms`, bắt đầu từ 1; `question_version_id` ghim lúc start). Câu trả lời ghi `assessment.attempt_answers` qua cùng đường chấm của F030. Ánh xạ `cue_id → position` lưu trong render payload và trong plugin (tính lại từ version đã ghim).
- **Rationale**: `revealNextHint` định danh item bằng `position`; `hints_used`, `response_ms`, `rapid_guess` và evidence `question.answered` chỉ có ở `attempt_answers` ([DATABASE_DESIGN §3.5](../../docs/07-backend/DATABASE_DESIGN.md)); review (`AttemptReview.items`) đọc từ item. Dùng chung scorer, test hợp đồng và mastery mà không nhân đôi logic.
- **Alternatives considered**: Ghi `assessment.activity_attempt_responses` (`part_id = cue_id`) như [VIDEO_LEARNING §8](../../docs/02-learning/VIDEO_LEARNING.md) và [DATABASE_DESIGN §2.6a](../../docs/07-backend/DATABASE_DESIGN.md) — loại vì thiếu cột `hints_used`/`rapid_guess`, phải viết lại đường chấm và review riêng; bảng đó vẫn để dành cho bước mô phỏng (F066). Cần cập nhật ghi chú ở hai tài liệu (task T040).

## R2. Kiểm tra tua và báo khống tiến độ

- **Decision**: Với mỗi heartbeat: (1) bỏ qua nếu `client_seq` ≤ đã lưu; (2) khoảng mới được chấp nhận tối đa `(now − last_heartbeat_at) × max(interaction.playback_rates)` mili-giây phần chưa xem (cộng vào mép hiện tại); phần vượt bị cắt và ghi `assessment.integrity_signals` (`signal_type = video_progress_implausible`); (3) với `allow_seek = watched_only` khi chưa hoàn thành, `position_ms` > mép đã chấp nhận → `422 VIDEO_SEEK_NOT_ALLOWED`, không ghi; `backward_only` chỉ cho `position_ms` ≤ mép; `free` không giới hạn vị trí nhưng `watched_ratio` vẫn chỉ tính phần hợp lý.
- **Rationale**: Khớp [VIDEO_LEARNING §4, §7, §12](../../docs/02-learning/VIDEO_LEARNING.md) và [ANTI_CHEAT](../../docs/10-security/ANTI_CHEAT.md) (không tự phạt, chỉ tín hiệu); không cần hằng số mới ngoài dữ liệu version.
- **Alternatives considered**: Dung sai cố định (giây) cho `position_ms` — loại vì cần tham số mới chưa đăng ký CONFIG_DEFAULTS; tin hoàn toàn `watched_ranges` từ client — vi phạm ADR-0016.

## R3. Chọn biến thể video phía client

- **Decision**: Server trả mọi biến thể Theora có sẵn trong `media[]` (`variant = theora_480p | theora_720p`); client chọn theo preset (`QualityManager`) và hạ cấp khi đệm lặp lại.
- **Rationale**: Preset là dữ liệu thiết bị, không gửi server ([PERFORMANCE_BUDGET §5](../../docs/08-game-client/PERFORMANCE_BUDGET.md)); tránh round-trip khi hạ cấp.
- **Alternatives considered**: Server chọn theo header thiết bị — loại vì server không biết preset/đệm thực tế.
