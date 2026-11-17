# Specification Quality Checklist: F077 — Event & Season live-ops: vòng đời sự kiện, community goal, season track, sự kiện trường

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

- Season track (FR-EVT-005, P4) phụ thuộc lịch mùa F106 (M4b) dù F077 ở M3 — kiểm bằng fixture tới khi F106 Done.
- Endpoint kiểm văn bản của module chat chưa có — delta, phụ thuộc F116.
- AsyncAPI ghi nhịp community goal 10 giây, EVENT_SEASON_SYSTEM/CONFIG_DEFAULTS 30 giây — theo tài liệu sở hữu.
- `event_runs.suspended_reason varchar(500)` ngắn hơn lý do API (1000) — đề xuất nâng `varchar(1000)`.
- Màn sự kiện trường (workspace C) chưa có feature trong danh mục.
