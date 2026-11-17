# Specification Quality Checklist: F116 — Pipeline kiểm tra tin nhắn, danh mục từ nhạy cảm, công tắc kiểm duyệt

**Purpose**: Kiểm chất lượng spec trước khi lập kế hoạch (`/speckit.plan`) · **Created**: 2026-11-16 · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không có chi tiết hiện thực (ngôn ngữ, framework, tên bảng) trong spec.md — các chi tiết đó ở plan.md
- [x] Tập trung vào giá trị cho người dùng và nhà trường
- [x] Mọi mục bắt buộc đã điền

## Requirement Completeness

- [x] Không còn marker [NEEDS CLARIFICATION]
- [x] Mọi FR chính của feature (theo CODE_PLAN) có mặt và có mã PRD đúng
- [x] Mọi tiêu chí viết theo EARS, kiểm thử được, không mơ hồ
- [x] Có tiêu chí cho authorization, tenant isolation, idempotency/audit (nếu có thao tác ghi), lỗi chính
- [x] Success criteria đo được và không phụ thuộc công nghệ
- [x] Edge case đã liệt kê
- [x] Phạm vi và "Ngoài phạm vi" rõ ràng; không có nội dung deferred
- [x] Phụ thuộc và giả định đã ghi

## Feature Readiness

- [x] Mỗi user story có Independent Test
- [x] Tham số dùng tên CONFIG_DEFAULTS, không chép giá trị
- [x] Không mâu thuẫn tài liệu sở hữu và ADR hiện hành

## Notes

- Ai bật/tắt category tenant: theo ADR-0072 + contract (`/admin/chat/tenant-categories/{code}:toggle`, Safety Team); PRD FR-MOD-013/AC-MOD-022 và MODERATION §11.3 còn ghi Tenant Admin — cần sửa tài liệu.
- Permission ghi term tenant: `chat.tenant_terms.write` (ADMIN_PORTAL, MODERATION) khác API_DESIGN §9.9 (`chat.sensitive_terms.write`).
- Mã tham chiếu `AC-MOD-0xx` lấy từ PRD (020–022, 026–027) và MODERATION §13 (còn lại); hai nguồn không trùng số.
- Ngưỡng spam, thời gian reload, timeout RE2 chưa có tên trong CONFIG_DEFAULTS — task Polish đề xuất.
