# Specification Quality Checklist: F135 — Học sinh tự đăng ký trong game vào tenant chung (bước phụ huynh, mã khôi phục, chống lạm dụng)

**Purpose**: Kiểm chất lượng spec trước khi lập kế hoạch (`/speckit.plan`) · **Created**: 2026-11-28 · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không có chi tiết hiện thực (ngôn ngữ, framework, tên bảng) trong spec.md ngoài operationId/mã lỗi là contract — chi tiết ở plan.md
- [x] Tập trung vào giá trị cho học sinh, phụ huynh và vận hành
- [x] Mọi mục bắt buộc đã điền

## Requirement Completeness

- [x] Không còn marker [NEEDS CLARIFICATION]
- [x] Mọi FR chính của feature (theo CODE_PLAN) có mặt và có mã PRD đúng (FR-AUTH-001, 017, 021, 033–036)
- [x] Mọi tiêu chí viết theo EARS, kiểm thử được, không mơ hồ
- [x] Có tiêu chí cho authorization (F135-AC-25, AC-31, AC-32), tenant isolation (F135-AC-31), idempotency/audit (F135-AC-24, AC-17), lỗi chính
- [x] Success criteria đo được và không phụ thuộc công nghệ
- [x] Edge case đã liệt kê
- [x] Phạm vi và "Ngoài phạm vi" rõ ràng; không có nội dung deferred (mô hình consent chung vẫn hoãn, chỉ có bằng chứng xác nhận theo ADR-0104)
- [x] Phụ thuộc và giả định đã ghi

## Feature Readiness

- [x] Mỗi user story có Independent Test
- [x] Tham số dùng tên CONFIG_DEFAULTS, không chép giá trị
- [x] Không mâu thuẫn tài liệu sở hữu và ADR hiện hành (ADR-0104 thay FR-AUTH-001 cũ, ADR-0063 mục 2 một phần)

## Notes

- Quyết định owner 2026-11-28 ghi ở Clarifications; chi tiết còn lại chốt theo "chốt hết" (phương án an toàn nhất).
- Câu hỏi mở F135-1…F135-4 không chặn Ready (có mặc định an toàn).
