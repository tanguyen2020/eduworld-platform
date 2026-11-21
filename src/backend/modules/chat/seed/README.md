# Seed — module chat

Dữ liệu giả cho local ([LOCAL_DEVELOPMENT §5](../../../../../docs/11-operations/LOCAL_DEVELOPMENT.md)). F001 chưa có seed; feature sở hữu bảng thêm seed ở đây. Không seed vào schema `ref` (projection nạp qua event).

- `chat_policies.platform.json` (F115 T009): chính sách chat mặc định cấp platform (CHAT_SYSTEM §3.1, schema `specs/schemas/chat-policy.schema.json` v5, bằng `specs/examples/chat-policy.platform-default.json`). Không phải dữ liệu dev: migration `20261229000001_f115_chat_core.sql` nạp đúng tài liệu này cho mọi môi trường; test `TestSeedPlatformPolicy` kiểm hai bản khớp nhau.
- `regression/corpus.v1.json` (F116 T022): bộ hồi quy placeholder của danh mục từ nhạy cảm (`filter_quality_gate`), nhúng vào `internal/infrastructure/regression`. Không có từ nhạy cảm thật (R15): câu dương tính sinh lúc chạy từ term high/critical của danh mục ứng viên; mẫu PII bịa; câu sạch học tập. Bộ vận hành ≥ 2.000 câu ở ngoài repo, nạp qua `regression.Source`.
