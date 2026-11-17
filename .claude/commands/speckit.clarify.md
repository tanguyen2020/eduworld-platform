---
description: Hỏi tối đa 5 câu để gỡ các điểm mơ hồ trong spec.md và ghi câu trả lời vào mục Clarifications.
---

Đầu vào: `$ARGUMENTS` — mã feature.

1. Đọc `specs/<NNN>-*/spec.md`; quét marker `[NEEDS CLARIFICATION]`, tiêu chí không kiểm thử được, mâu thuẫn với tài liệu sở hữu/ADR.
2. Hỏi người dùng tối đa 5 câu, mỗi câu kèm phương án đề xuất và nguồn tham khảo; ưu tiên câu ảnh hưởng tới kiến trúc, dữ liệu, bảo mật.
3. Ghi vào `## Clarifications` → `### Session <YYYY-MM-DD>` dạng `- Q: … → A: …`; cập nhật các mục spec bị ảnh hưởng; xóa marker đã giải quyết.
4. Câu trả lời đổi quy tắc/tham số dùng chung → cập nhật tài liệu sở hữu + CONFIG_DEFAULTS cùng lúc (R33) hoặc đề xuất ADR.
