# ADR-0053 — Chấm điểm có hỗ trợ AI (AI-assisted Grading)

> **Trạng thái:** Proposed · chưa thuộc Release 1.0 · hoãn — ngoài version này (ADR-0082) · **Ngày:** 2026-11-15 · **OQ:** OQ-12 · **Tài liệu:** [WRITTEN_RESPONSE_RUBRIC](../99-deferred/WRITTEN_RESPONSE_RUBRIC.md), [DATA_PRIVACY](../10-security/DATA_PRIVACY.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Hoãn theo [ADR-0082](ADR-0082-spec-driven-development-va-noi-dung-deferred.md):** kỹ năng viết/nói không làm trong version này. Không triển khai.

## Bối cảnh

Master Prompt "Re-architecture" §15.5: AI hỗ trợ phản hồi là tùy chọn tương lai; không tự quyết định kết quả chính thức.

## Quyết định (đề xuất)

- AI chỉ **gợi ý** nhận xét/điểm cho tự luận, nói, dự án; **người có thẩm quyền luôn duyệt** trước khi có hiệu lực (Release hiện không có giáo viên và không có hoạt động chấm tay, ADR-0072; mở lại phải xác định lại người duyệt); AI **không** quyết định kết quả kiểm tra chính thức.
- Bật theo tenant (opt-in), thông báo minh bạch cho nhà trường.
- Dữ liệu gửi AI đã **loại bỏ định danh**; xử lý ưu tiên tại Việt Nam hoặc theo yêu cầu pháp lý (OQ-12); không dùng bài học sinh để huấn luyện mô hình bên ngoài.
- Mọi gợi ý lưu kèm phiên bản mô hình, prompt template, người duyệt để audit.
- Triển khai thật cần ADR mới chọn nhà cung cấp/mô hình và đánh giá độ chính xác so với người chấm chuyên môn.

## Hệ quả

(+) Có hướng mở rộng rõ ràng, an toàn. (−) Release không có hoạt động chấm tay (ADR-0072), nên ADR này chỉ là hướng mở rộng.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Audit theo Master Prompt mới |
| 2026-11-15 | Điều chỉnh theo ADR-0072 (bỏ vai trò giáo viên, bỏ hoạt động chấm tay khỏi Release) | Technical Architect |
| 2026-11-15 | Hoãn, ngoài version này | ADR-0082 |
