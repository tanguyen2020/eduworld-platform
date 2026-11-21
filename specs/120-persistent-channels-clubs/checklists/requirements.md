# Specification Quality Checklist: F120 — Channel bền vững (world, area, team, DM), club, chính sách theo cấp, tìm kiếm, retention

**Purpose**: Kiểm chất lượng spec trước khi lập kế hoạch (`/speckit.plan`) · **Created**: 2026-11-16 · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không có chi tiết hiện thực (ngôn ngữ, framework, tên bảng) trong spec.md — các chi tiết đó ở plan.md
- [x] Tập trung vào giá trị cho người dùng và nhà trường
- [x] Mọi mục bắt buộc đã điền

## Requirement Completeness

- [x] Không còn marker [NEEDS CLARIFICATION]
- [x] Mọi FR chính của feature (theo CODE_PLAN) có mặt và có mã PRD đúng (FR-CHAT-001, FR-CHAT-004, FR-CHAT-006, FR-CHAT-007, FR-CHAT-016, FR-MOD-004, FR-SOC-004)
- [x] Mọi tiêu chí viết theo EARS, kiểm thử được, không mơ hồ
- [x] Có tiêu chí cho authorization (AC-06, AC-11, AC-17, AC-18), tenant isolation (AC-05, AC-19, AC-22), idempotency/audit (AC-04, AC-06, AC-17, AC-21, AC-24), lỗi chính (AC-07…09, AC-14, AC-19), giới hạn độ dài (AC-17, AC-22)
- [x] Success criteria đo được và không phụ thuộc công nghệ
- [x] Edge case đã liệt kê
- [x] Phạm vi và "Ngoài phạm vi" rõ ràng; không có nội dung deferred
- [x] Phụ thuộc và giả định đã ghi

## Feature Readiness

- [x] Mỗi user story có Independent Test
- [x] Tham số dùng tên CONFIG_DEFAULTS, không chép giá trị
- [x] Không mâu thuẫn tài liệu sở hữu và ADR hiện hành

## Notes

- Spec nhắc vài tên bảng trong Clarifications/AC (`chat.channel_members`, `processed_events`, `chat.ephemeral_messages`) để chỉ rõ ranh giới ADR-0033; chi tiết ở data-model.md.
- 2026-11-21 (ADR-0095, WP-S): bỏ channel lớp/nhóm học tập và cấp policy lớp; F120-AC-01…03 giữ mã với nhãn **Removed (ADR-0095)**; checklist vẫn đạt.
- Tài liệu cần sửa theo ADR-0084/ADR-0095 (SOCIAL_SYSTEM §6.1 — đã sửa WP-S 2026-11-21; CHAT_SYSTEM §4 — WP-C1; DATABASE_DESIGN §2.11) và tham số `persistent_chat_retention_days` chưa đăng ký — đã báo cáo, có task Polish.
