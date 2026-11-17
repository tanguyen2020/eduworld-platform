# Specification Quality Checklist: F122 — Portal an toàn chat: kiểm duyệt (E), danh mục và công tắc (D), chính sách chat trường (C)

**Purpose**: Kiểm chất lượng spec trước khi lập kế hoạch (`/speckit.plan`) · **Created**: 2026-11-16 · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không có chi tiết hiện thực (ngôn ngữ, framework, tên bảng) trong spec.md — các chi tiết đó ở plan.md
- [x] Tập trung vào giá trị cho người dùng và nhà trường
- [x] Mọi mục bắt buộc đã điền

## Requirement Completeness

- [x] Không còn marker [NEEDS CLARIFICATION]
- [x] Mọi FR chính của feature (theo CODE_PLAN) có mặt và có mã PRD đúng (FR-MOD-003, FR-MOD-004, FR-MOD-009, FR-MOD-011, FR-MOD-017)
- [x] Mọi tiêu chí viết theo EARS, kiểm thử được, không mơ hồ
- [x] Có tiêu chí cho authorization (AC-09, AC-24, AC-25), tenant isolation (AC-01, AC-14, AC-18), idempotency/audit (AC-03, AC-19, AC-26), lỗi chính (AC-04, AC-11, AC-16, AC-19, AC-22), giới hạn độ dài (AC-26)
- [x] Success criteria đo được và không phụ thuộc công nghệ
- [x] Edge case đã liệt kê
- [x] Phạm vi và "Ngoài phạm vi" rõ ràng; không có nội dung deferred
- [x] Phụ thuộc và giả định đã ghi

## Feature Readiness

- [x] Mỗi user story có Independent Test
- [x] Tham số dùng tên CONFIG_DEFAULTS, không chép giá trị
- [x] Không mâu thuẫn tài liệu sở hữu và ADR hiện hành

## Notes

- F122-AC-21, F122-AC-22, F122-AC-23 phụ thuộc delta F120; F122-AC-13 (banner người dùng tenant) phụ thuộc delta `getTenantModerationStatus` (T004–T005).
- MODERATION §11.3 lệch ADMIN_PORTAL §3.9 về workspace và người bật/tắt nhóm từ tenant — theo ADMIN_PORTAL + ADR-0072, đã báo cáo.
