# Specification Quality Checklist: F051 — Portal shell: đăng nhập, chọn tenant sau đăng nhập, menu theo quyền, bộ lọc tenant

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

- Spec có nêu tên operationId và mã lỗi (contract) để kiểm thử được; không nêu framework.
- F051 không có thao tác ghi nghiệp vụ nên không có tiêu chí idempotency; audit liên tenant do backend (F012/F015).
- Mâu thuẫn tài liệu (`createAuthSession`, `me/permissions?tenant_id=`) đã chốt theo RB-115 (portal không dùng `createAuthSession`; header `X-Tenant-Id`), xem [contracts](../contracts/README.md) §6.
