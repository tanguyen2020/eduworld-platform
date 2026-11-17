# Specification Quality Checklist: F062 — Main Hub greybox, character controller, `QualityManager`, tab "Nhà trường giao"

**Purpose**: Kiểm chất lượng spec trước khi lập kế hoạch (`/speckit.plan`) · **Created**: 2026-11-16 · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không có chi tiết hiện thực (ngôn ngữ, framework, tên bảng) trong spec.md — các chi tiết đó ở plan.md (spec chỉ nêu tên service/intent cần cho tiêu chí kiểm được)
- [x] Tập trung vào giá trị cho người dùng và nhà trường
- [x] Mọi mục bắt buộc đã điền

## Requirement Completeness

- [x] Không còn marker [NEEDS CLARIFICATION]
- [x] Mọi FR chính của feature (theo CODE_PLAN) có mặt và có mã PRD đúng (NFR-COMP-001, NFR-PERF-005 ở Ràng buộc phi chức năng + AC)
- [x] Mọi tiêu chí viết theo EARS, kiểm thử được, không mơ hồ
- [x] Có tiêu chí cho authorization, tenant isolation, lỗi chính (F062-AC-11, F062-AC-12); không có thao tác ghi quan trọng nên không có idempotency/audit
- [x] Success criteria đo được và không phụ thuộc công nghệ
- [x] Edge case đã liệt kê
- [x] Phạm vi và "Ngoài phạm vi" rõ ràng; không có nội dung deferred
- [x] Phụ thuộc và giả định đã ghi (Prism mock cho F041)

## Feature Readiness

- [x] Mỗi user story có Independent Test
- [x] Tham số dùng tên CONFIG_DEFAULTS, không chép giá trị
- [x] Không mâu thuẫn tài liệu sở hữu và ADR hiện hành (ghi nhận lệch nhỏ về nhắc nghỉ ở plan.md Phase 0)

## Notes

- Delta contract `MyAssignment` phải merge trước khi F062 chuyển Done.
