# Specification Quality Checklist: F124 — Thực thi xử phạt và vận hành Safety Team: consumer `sanction.applied` (khóa tài khoản), email thông báo vụ việc cho trường, vòng trực, break-glass

**Purpose**: Kiểm chất lượng spec trước khi lập kế hoạch (`/speckit.plan`) · **Created**: 2026-11-16 · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không có chi tiết hiện thực (ngôn ngữ, framework, tên bảng) trong spec.md — các chi tiết đó ở plan.md
- [x] Tập trung vào giá trị cho người dùng và nhà trường
- [x] Mọi mục bắt buộc đã điền

## Requirement Completeness

- [x] Không còn marker [NEEDS CLARIFICATION]
- [x] Mọi FR chính của feature (theo CODE_PLAN) có mặt và có mã PRD đúng — CODE_PLAN chưa ghi FR; spec chọn FR-MOD-006, FR-AUTH-010, FR-MOD-017, FR-MOD-018, FR-MOD-005, FR-AUD-001
- [x] Mọi tiêu chí viết theo EARS, kiểm thử được, không mơ hồ
- [x] Có tiêu chí cho authorization (AC-05, AC-16, AC-18, AC-21), tenant isolation (AC-04, AC-11, AC-19), idempotency/audit (AC-03, AC-10, AC-12, AC-17), lỗi chính (`ACCOUNT_LOCK_HELD_BY_SAFETY`, `ONCALL_SHIFT_OVERLAP`, `BREAK_GLASS_*`), giới hạn độ dài (AC-12, AC-17, AC-21)
- [x] Success criteria đo được và không phụ thuộc công nghệ
- [x] Edge case đã liệt kê
- [x] Phạm vi và "Ngoài phạm vi" rõ ràng; không có nội dung deferred
- [x] Phụ thuộc và giả định đã ghi

## Feature Readiness

- [x] Mỗi user story có Independent Test
- [x] Tham số dùng tên CONFIG_DEFAULTS, không chép giá trị (4 tham số mới ghi [Proposed] ở Assumptions, task T046 đăng ký)
- [x] Không mâu thuẫn tài liệu sở hữu và ADR hiện hành

## Notes

- Phụ thuộc chéo: payload `sanction.applied/lifted`, `moderation.school_notified` do F121 thêm; F124 cần bổ sung `notification_id`, `ack_due_at`.
- Khóa đa nguồn `identity.account_locks` đã chốt (RB-621) và có trong DATABASE_DESIGN; F016 đồng bộ theo.
- UI lịch trực và break-glass chưa thuộc feature nào (đề xuất F122).
- EVENT_DESIGN chưa liệt kê identity là consumer của `sanction.*`, `moderation.school_notified` (task T007).
