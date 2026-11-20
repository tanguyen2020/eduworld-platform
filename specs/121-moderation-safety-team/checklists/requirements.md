# Specification Quality Checklist: F121 — Moderation: báo cáo, sanction, hàng đợi Safety Team liên tenant, thông báo vụ việc cho trường

**Purpose**: Kiểm chất lượng spec trước khi lập kế hoạch (`/speckit.plan`) · **Created**: 2026-11-16 · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không có chi tiết hiện thực (ngôn ngữ, framework, tên bảng) trong spec.md — các chi tiết đó ở plan.md
- [x] Tập trung vào giá trị cho người dùng và nhà trường
- [x] Mọi mục bắt buộc đã điền

## Requirement Completeness

- [x] Không còn marker [NEEDS CLARIFICATION]
- [x] Mọi FR chính của feature (theo CODE_PLAN) có mặt và có mã PRD đúng (FR-MOD-002, 003, 005, 006, 008, 016, 017, 018)
- [x] Mọi tiêu chí viết theo EARS, kiểm thử được, không mơ hồ
- [x] Có tiêu chí cho authorization (AC-06, 07, 09, 10, 13), tenant isolation (AC-05, 06, 18, 19), idempotency/audit (AC-01, 07, 08, 11, 17, 19, 25), lỗi chính (AC-05, 08, 11, 17, 19, 21), giới hạn độ dài (AC-01, 11, 13, 17, 19)
- [x] Success criteria đo được và không phụ thuộc công nghệ
- [x] Edge case đã liệt kê
- [x] Phạm vi và "Ngoài phạm vi" rõ ràng; không có nội dung deferred
- [x] Phụ thuộc và giả định đã ghi

## Feature Readiness

- [x] Mỗi user story có Independent Test
- [x] Tham số dùng tên CONFIG_DEFAULTS, không chép giá trị
- [x] Không mâu thuẫn tài liệu sở hữu và ADR hiện hành

## Notes

- Enum OpenAPI lệch MODERATION (trạng thái case, sanction, reason) → theo MODERATION, delta Phase 2.
- `moderation.case.lead` [Confirmed] chưa có trong catalog; khóa tài khoản và email trường cần consumer ở module identity — đã báo cáo.
