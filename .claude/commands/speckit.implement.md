---
description: Triển khai một feature theo tasks.md, từng task có bằng chứng, tuân thủ constitution.
---

Đầu vào: `$ARGUMENTS` — mã feature.

1. Điều kiện: feature `Ready` trong `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md`, mọi phụ thuộc `Done`, có `tasks.md`. Nếu không → dừng.
2. Đọc constitution, AGENTS.md, `spec.md`, `plan.md`, `data-model.md`, `contracts/README.md`, `quickstart.md`, `tasks.md`.
3. Làm task theo thứ tự phase; task `[P]` có thể làm song song; với mỗi user story: viết test trước và xác nhận test FAIL, rồi code tới khi PASS.
4. Sau mỗi task đánh dấu `[x]` trong `tasks.md`. Gặp thiếu/mâu thuẫn trong spec → dừng, ghi vào `## Clarifications` hoặc `Assumptions`, báo người dùng; không tự mở rộng phạm vi (R34).
5. Kết thúc: chạy lệnh kiểm tra (AGENTS §4) và `quickstart.md`, cập nhật trạng thái feature trong CODE_PLAN, báo cáo theo AGENTS §5.
