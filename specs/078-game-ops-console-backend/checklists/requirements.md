# Specification Quality Checklist: F078 — Game Operations Console backend: workflow nội dung game, thumbnail 3D, catalog cho nhà trường

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

- `listGameActivities` có `x-service: learning` nhưng dữ liệu ở module game — chọn projection qua event (research R1); cần F041 xác nhận.
- ADMIN_PORTAL §3.11 (rollback dùng `content.release.rollback`) lệch §2.3 (role `liveops` không có permission này) — F078 dùng `game.content.publish`.
- `media_assets.kind` thiếu `render_thumbnail`/`render_turntable` (DESIGN_SYSTEM §7) — delta.
- Dịch nội dung game (`/game-admin/translations*`) chưa có feature sở hữu.
- Realm local thiếu tài khoản `game_designer`, `liveops`.
