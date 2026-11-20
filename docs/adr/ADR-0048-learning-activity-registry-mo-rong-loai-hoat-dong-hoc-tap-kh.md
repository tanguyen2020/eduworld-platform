# ADR-0048 — Learning Activity Registry: mở rộng loại hoạt động học tập không sửa lõi

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

EduWorld cần nhiều định dạng học tập: quiz, bài tập tương tác, tự luận/rubric, video, audio/phát âm, mô phỏng, mini-game, dự án (Master Prompt "Re-architecture" §15.3). Nếu mỗi module (quest, assignment, exam, challenge, analytics) tự xử lý từng loại thì mỗi loại mới phải sửa khắp nơi.

## Phương án

1. Mỗi loại là một module riêng có API riêng.
2. **Registry plugin trong learning-service + renderer registry trong Godot + editor registry trong portal.**
3. Nội dung tự do dạng script do CMS nạp (loại: rủi ro bảo mật, không kiểm soát chấm điểm).

## Quyết định (đề xuất phương án 2)

- learning-service có **Activity Registry**; mỗi `activity_type` trong Release (`quiz`, `interactive_exercise`, `minigame`, `video_lesson`, `audio_lesson`, `simulation`; `written_response`, `speaking`, `project` ngoài Release theo ADR-0072) đăng ký một plugin Go triển khai interface chung: `ConfigSchema()` (JSON Schema trong `specs/schemas/activities/`), `BuildRenderPayload()` (không chứa đáp án, ADR-0016), `Respond()`, `Progress()`, `Evaluate()` (`auto | rule | external` trong Release; `rubric`/chấm tay ngoài Release theo ADR-0072; AI theo ADR-0053), `CompletionRule()`, `EmitEvidence()`.
- Quest, assignment, exam, challenge, analytics **chỉ tham chiếu `activity_id` + version**, xử lý qua interface chung.
- Godot có renderer registry theo `activity_type`; portal có editor registry. Client cũ không có renderer → hiển thị "cần cập nhật game", không crash.
- Thêm loại mới = plugin backend + schema + renderer Godot + editor portal; không sửa quest/assignment/exam.

## Hệ quả

(+) Mở rộng định dạng nhanh, chấm điểm và bằng chứng học tập nhất quán. (−) Interface chung phải đủ tổng quát; schema từng loại cần version.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Audit theo Master Prompt mới |
| 2026-11-15 | Interface thêm `Respond()` | ACTIVITY_ENGINE |
| 2026-11-15 | Điều chỉnh theo ADR-0072 (bỏ vai trò giáo viên, bỏ hoạt động chấm tay khỏi Release) | Technical Architect |
| 2026-11-20 | Owner chốt (Accepted) | Owner |
