# Specification Quality Checklist: F073 — Mini-game Engine: `MiniGameConfig`, plugin `minigame`, scene Godot 11 loại

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

- MINIGAME_ENGINE §5 (bắt đầu qua game) và ACTIVITY_ENGINE §9 (mode `minigame` qua `startActivityAttempt`) được hợp nhất: tự chơi/quest qua module game, bài giao qua learning — cập nhật hai tài liệu ở Polish.
- Tên loại `drag_drop` (OpenAPI `MiniGameSession`) lệch `drag_and_drop` (schema, MINIGAME_ENGINE) — sửa OpenAPI.
- EVENT_DESIGN cho learning tiêu thụ `minigame.completed` để ghi evidence — trùng evidence từ attempt; đề xuất bỏ.
