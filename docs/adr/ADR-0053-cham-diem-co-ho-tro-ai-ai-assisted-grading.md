# ADR-0053 — Chấm điểm có hỗ trợ AI (AI-assisted Grading)

> **Trạng thái:** Proposed · chưa thuộc Release 1.0 · **Ngày:** 2026-11-15 · **OQ:** OQ-12 · **Tài liệu:** [WRITTEN_RESPONSE_RUBRIC](../02-learning/WRITTEN_RESPONSE_RUBRIC.md), [DATA_PRIVACY](../10-security/DATA_PRIVACY.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Master Prompt "Re-architecture" §15.5: AI hỗ trợ phản hồi là tùy chọn tương lai; không tự quyết định kết quả chính thức.

## Quyết định (đề xuất)

- AI chỉ **gợi ý** nhận xét/điểm cho tự luận, nói, dự án; **giáo viên luôn duyệt** trước khi có hiệu lực; AI **không** quyết định kết quả kiểm tra chính thức.
- Bật theo tenant (opt-in), thông báo minh bạch cho nhà trường.
- Dữ liệu gửi AI đã **loại bỏ định danh**; xử lý ưu tiên tại Việt Nam hoặc theo yêu cầu pháp lý (OQ-12); không dùng bài học sinh để huấn luyện mô hình bên ngoài.
- Mọi gợi ý lưu kèm phiên bản mô hình, prompt template, người duyệt để audit.
- Triển khai thật cần ADR mới chọn nhà cung cấp/mô hình và đánh giá độ chính xác so với giáo viên.

## Hệ quả

(+) Có hướng mở rộng rõ ràng, an toàn. (−) Chưa giảm tải chấm cho giáo viên trong Release 1.0.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Audit theo Master Prompt mới |
