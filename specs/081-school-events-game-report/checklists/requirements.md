# Specification Quality Checklist: F081 — Workspace C — sự kiện trường từ template

**Purpose**: Kiểm chất lượng spec trước khi lập kế hoạch (`/speckit.plan`) · **Created**: 2026-11-16 · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không có chi tiết hiện thực (ngôn ngữ, framework, tên bảng) trong spec.md — các chi tiết đó ở plan.md
- [x] Tập trung vào giá trị cho người dùng và nhà trường
- [x] Mọi mục bắt buộc đã điền

## Requirement Completeness

- [x] Không còn marker [NEEDS CLARIFICATION]
- [x] Mọi FR chính của feature (theo CODE_PLAN) có mặt và có mã PRD đúng (FR-EVT-006, FR-EVT-004, FR-LMS-007, FR-CMS-010, FR-CMS-013)
- [x] Mọi tiêu chí viết theo EARS, kiểm thử được, không mơ hồ
- [x] Có tiêu chí cho authorization (AC-09, AC-12), tenant isolation (AC-06, AC-09, AC-12), idempotency/audit (AC-03, AC-08), lỗi chính (AC-04, AC-07), giới hạn độ dài (AC-05)
- [x] Success criteria đo được và không phụ thuộc công nghệ
- [x] Edge case đã liệt kê
- [x] Phạm vi và "Ngoài phạm vi" rõ ràng; không có nội dung deferred
- [x] Phụ thuộc và giả định đã ghi

## Feature Readiness

- [x] Mỗi user story có Independent Test
- [x] Tham số dùng tên CONFIG_DEFAULTS, không chép giá trị
- [x] Không mâu thuẫn tài liệu sở hữu và ADR hiện hành

## Notes

- AC-10–AC-12 cần backend nhỏ trong module game (CODE_PLAN ghi `src/cms/frontend/apps/portal` + module `game`).
- 2026-11-17: tách F081 — User Story 4 cũ (báo cáo hoạt động game, tiêu chí 13–18 cũ) chuyển sang F080 US5 (F080-AC-27…32); F081 không còn phụ thuộc F080.
- AC-02 phụ thuộc delta `EventTemplate.*_option_items` (owner F077).
