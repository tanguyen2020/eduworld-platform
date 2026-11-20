# Specification Quality Checklist: F107 — Tournament: 6 format, bracket, lịch, thử thách lớp/giải trường từ template

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

- Mâu thuẫn chốt [Confirmed]: `/lms/challenges` ghi `x-service: learning` (API_DESIGN §9.14, `learning.v1.yaml`) nhưng SCHOOL_LMS §7 và COMPETITION_SYSTEM §10.2 đặt thực thi + template ở module competition → chuyển route sang module competition.
- COMPETITION_SYSTEM §10.2 cho Tenant Admin tenant `unit` tạo giải liên trường, trái ADMIN_PORTAL §2.3 (chỉ `school_admin` có `lms.challenge.write`) — spec theo ADMIN_PORTAL/ADR-0072.
- Format double elim, vòng bảng, giải đội là P6 theo PRD (FR-TRN-001) — để ở US5 (P3).
