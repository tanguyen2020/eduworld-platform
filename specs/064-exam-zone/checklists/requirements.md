# Specification Quality Checklist: F064 — Exam Zone: thi có giờ theo server, tự nộp, resume, thông báo bài mới

**Purpose**: Kiểm chất lượng spec trước khi lập kế hoạch (`/speckit.plan`) · **Created**: 2026-11-16 · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không có chi tiết hiện thực (ngôn ngữ, framework, tên bảng) trong spec.md — các chi tiết đó ở plan.md
- [x] Tập trung vào giá trị cho người dùng và nhà trường
- [x] Mọi mục bắt buộc đã điền

## Requirement Completeness

- [x] Không còn marker [NEEDS CLARIFICATION]
- [x] Mọi FR chính của feature (theo CODE_PLAN) có mặt và có mã PRD đúng
- [x] Mọi tiêu chí viết theo EARS, kiểm thử được, không mơ hồ
- [x] Có tiêu chí cho authorization, tenant isolation (F064-AC-10), idempotency (F064-AC-07, F064-AC-08), lỗi chính (F064-AC-09, F064-AC-13), giới hạn độ dài (F064-AC-11)
- [x] Success criteria đo được và không phụ thuộc công nghệ
- [x] Edge case đã liệt kê
- [x] Phạm vi và "Ngoài phạm vi" rõ ràng; không có nội dung deferred
- [x] Phụ thuộc và giả định đã ghi

## Feature Readiness

- [x] Mỗi user story có Independent Test
- [x] Tham số dùng tên CONFIG_DEFAULTS, không chép giá trị
- [x] Không mâu thuẫn tài liệu sở hữu và ADR hiện hành (mâu thuẫn EXAM_ENGINE §6 ↔ GODOT_ARCHITECTURE §8.2 đã chọn theo tài liệu sở hữu client và báo cáo)

## Notes

- Delta `ExamSession.navigation`, `takeover`, `MyAssignment.exam_session_id` phải merge trước khi F064 chuyển Done.
