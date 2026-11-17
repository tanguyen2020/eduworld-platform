# Specification Quality Checklist: F080 — Game Analytics và Live Ops dashboard (workspace E)

**Purpose**: Kiểm chất lượng spec trước khi lập kế hoạch (`/speckit.plan`) · **Created**: 2026-11-16 · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không có chi tiết hiện thực (ngôn ngữ, framework, tên bảng) trong spec.md — các chi tiết đó ở plan.md
- [x] Tập trung vào giá trị cho người dùng và nhà trường
- [x] Mọi mục bắt buộc đã điền

## Requirement Completeness

- [x] Không còn marker [NEEDS CLARIFICATION]
- [x] Mọi FR chính của feature (theo CODE_PLAN) có mặt và có mã PRD đúng (FR-ANL-006…011, FR-CMS-016, FR-GAME-011, FR-OPS-003, NFR-OBS-003)
- [x] Mọi tiêu chí viết theo EARS, kiểm thử được, không mơ hồ
- [x] Có tiêu chí cho authorization (AC-02, AC-14, AC-15, AC-24, AC-25), tenant isolation (AC-11, AC-15, AC-16), idempotency/audit (AC-05, AC-13, AC-19), lỗi chính (AC-01, AC-02, AC-12, AC-17), giới hạn độ dài (AC-01, AC-26)
- [x] Success criteria đo được và không phụ thuộc công nghệ
- [x] Edge case đã liệt kê
- [x] Phạm vi và "Ngoài phạm vi" rõ ràng; không có nội dung deferred (FR-ANL-011 chỉ phần cơ chế kỹ thuật)
- [x] Phụ thuộc và giả định đã ghi

## Feature Readiness

- [x] Mỗi user story có Independent Test
- [x] Tham số dùng tên CONFIG_DEFAULTS, không chép giá trị
- [x] Không mâu thuẫn tài liệu sở hữu và ADR hiện hành

## Notes

- Spec nhắc tên bảng `analytics.events_raw`, `player_days` để chỉ rõ phạm vi dữ liệu thô vs tổng hợp (ADR-0062).
- GAME_ANALYTICS §6 lệch DATABASE_DESIGN §2.8b về cột chiều rollup — theo DATABASE_DESIGN, đã báo cáo.
- 2026-11-17: UI báo cáo game tổng hợp ở workspace C ("Hoạt động game") chuyển từ F081 US4 vào US5 (F080-AC-27…32), thêm FR-ANL-012, FR-ANL-013; phụ thuộc thêm F052, F056.
