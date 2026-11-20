# Specification Quality Checklist: F070 — Hồ sơ người chơi, nickname, tạo avatar lần đầu, trang phục, emote

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

- Hai tham số mới `nickname_change_monthly_limit`, `nickname_suggestion_ttl` [Confirmed] — task Polish đăng ký CONFIG_DEFAULTS + CHARACTER_SYSTEM §5.3.
- `AvatarConfig` OpenAPI lệch CHARACTER_SYSTEM §5.1; nickname đã chốt duy nhất theo tenant, `varchar(32)` (RB-302) — contract sửa ở Phase 2.
- Thao tác ghi của feature (avatar, nickname) không thuộc nhóm R10 (submit/thưởng/lời mời); vết thay đổi nickname lưu `game.nickname_changes`.
