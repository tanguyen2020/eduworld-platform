---
description: Kiểm tra nhất quán giữa spec.md, plan.md, tasks.md, PRD, tài liệu sở hữu và contract (chỉ đọc, không sửa).
---

Đầu vào: `$ARGUMENTS` — mã feature (bỏ trống = mọi feature).

1. Chạy `node scripts/specs/check-features.mjs --verbose`.
2. Đối chiếu: mọi FR chính trong CODE_PLAN có trong spec; mọi tiêu chí AC có task test; mọi delta contract có task; operationId/event/schema/tham số được nhắc có thật; plan không trái constitution/ADR hiện hành; không tham chiếu nội dung deferred.
3. Báo cáo bảng phát hiện (mức Critical/High/Medium/Low, vị trí, đề xuất sửa). Không tự sửa file.
