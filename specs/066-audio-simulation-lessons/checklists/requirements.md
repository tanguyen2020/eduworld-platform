# Specification Quality Checklist: F066 — Bài nghe và mô phỏng: plugin `audio_lesson`, `simulation` + checker Go, template Godot

**Purpose**: Kiểm chất lượng spec trước khi lập kế hoạch (`/speckit.plan`) · **Created**: 2026-11-16 · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không có chi tiết hiện thực (ngôn ngữ, framework, tên bảng) trong spec.md — các chi tiết đó ở plan.md
- [x] Tập trung vào giá trị cho người dùng và nhà trường
- [x] Mọi mục bắt buộc đã điền

## Requirement Completeness

- [x] Không còn marker [NEEDS CLARIFICATION]
- [x] Mọi FR chính của feature (theo CODE_PLAN) có mặt và có mã PRD đúng (FR-CNT-015 phần nghe, FR-CNT-016)
- [x] Mọi tiêu chí viết theo EARS, kiểm thử được, không mơ hồ
- [x] Có tiêu chí cho authorization (AC-26, AC-27), tenant isolation (AC-27), idempotency/audit (AC-06, AC-14, AC-16), lỗi chính (AC-03, AC-04, AC-09, AC-13, AC-14), giới hạn độ dài (AC-28)
- [x] Success criteria đo được và không phụ thuộc công nghệ
- [x] Edge case đã liệt kê
- [x] Phạm vi và "Ngoài phạm vi" rõ ràng; không có nội dung deferred
- [x] Phụ thuộc và giả định đã ghi

## Feature Readiness

- [x] Mỗi user story có Independent Test
- [x] Tham số dùng tên CONFIG_DEFAULTS, không chép giá trị
- [x] Không mâu thuẫn tài liệu sở hữu và ADR hiện hành

## Notes

- Mâu thuẫn tài liệu đã xử lý bằng quyết định [Proposed] và báo cáo: transcript bắt buộc (ADMIN_PORTAL §3.13 vs AUDIO_LANGUAGE_LEARNING §2); schema `learning.*` (SIMULATION_ENGINE §7, AUDIO §3) vs `content.*`/`assessment.*` (DATABASE_DESIGN — theo DATABASE_DESIGN); `SimulationTemplate.kind` lệch enum `simulation_kind`; `time_limit_seconds` trong SIMULATION_ENGINE §4 nhưng không có trong `simulation.schema.json` `interaction` (dùng `time_limit_seconds` chung của activity, ACTIVITY_ENGINE §4); `recording_uploader.gd` trong GODOT_ARCHITECTURE §3 trái ADR-0072.
- F066-AC-04, AC-14, AC-19, AC-23 phụ thuộc delta T004.
