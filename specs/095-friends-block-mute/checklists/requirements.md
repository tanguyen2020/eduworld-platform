# Specification Quality Checklist: F095 — Bạn bè, block, mute (module social)

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

- Quyết định [Proposed] cần xác nhận khi review: `/social/*` (bạn bè, chặn, tắt tiếng) qua `game_player`; (đã chốt 2026-11-16: chặn kết thúc tình bạn RB-002, tên cài đặt `cross_school_social_enabled` RB-003); mã bạn bè một mã hiệu lực/người.
- Mâu thuẫn: rate limit lời mời API_DESIGN §8 (10/giờ, 30/ngày) khác `friend_limits` (SOCIAL_SYSTEM); `redeemFriendCode` OpenAPI có `410`, API_DESIGN ghi `404 FRIEND_CODE_INVALID`.
- DATABASE_DESIGN §2.11 dùng PK cặp cho `friendships`/`blocks`/`mutes` — không tương thích soft-delete khi tạo lại quan hệ; data-model đổi sang PK UUID + unique partial (task cập nhật DATABASE_DESIGN).
