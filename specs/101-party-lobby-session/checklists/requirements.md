# Specification Quality Checklist: F101 — Party Lobby, Start Session, leader/thành viên mất kết nối

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

- Phụ thuộc vòng về thời gian: F101 gọi facade `game` do F103 hiện thực, F103 phụ thuộc F101. Contract khai báo ở Phase 2 của F101; F101 chỉ `Done` cho `quest`/`boss` khi F103 Done (`explore` Done độc lập).
- Mâu thuẫn đã chọn hướng: `PARTY_ACTIVITY_NOT_ELIGIBLE` 409 (PARTY_SYSTEM) vs 422 (API_DESIGN); ba enum loại hoạt động lệch nhau; PRD AC-PTY-008 ghi event `party.leader_changed` nhưng AsyncAPI là `party.leader.transferred`; F091 trong danh mục ghi `realtime + module game` nhưng REALTIME §1 đặt room host hub/private instance ở module competition.
- `party_activity_min_members_timeout`, `party_lobby_cmd_rate` chưa có trong CONFIG_DEFAULTS — task Polish.
