# Specification Quality Checklist: F097 — Student UX pass 2: màn làm bài như một trò chơi, tạo nhân vật bằng hình, marker không che nhân vật

**Purpose**: kiểm chất lượng spec trước khi lập plan · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không mô tả chi tiết cài đặt trong spec (chỉ hành vi và tiêu chí)
- [x] Tập trung giá trị người dùng (làm bài vui và dễ đọc, chọn nhân vật bằng hình, thấy trọn nhân vật)
- [x] Mã FR/NFR chỉ từ PRD

## Requirement Completeness

- [x] Không còn [NEEDS CLARIFICATION]; điểm thiếu chốt [Confirmed] ở Session 2026-11-26
- [x] Mỗi tiêu chí F097-AC-* theo EARS và kiểm được
- [x] Tham số trỏ về tài liệu sở hữu (LEARNING_EXPERIENCE §8, DESIGN_SYSTEM §5.2, WORLD_RENDERING §5.1, STUDENT_GAME_UX_CORE S02)
- [x] Edge case có tiêu chí
- [x] Phạm vi và ngoài phạm vi rõ ràng (không đổi contract, không chấm ở client)
- [x] Phụ thuộc (F063, F065, F066, F070, F071, F094, F096) và ADR (0047, 0103) ghi rõ

## Feature Readiness

- [x] Mọi AC có task test trong tasks.md
- [x] User story độc lập, có Independent Test
- [x] Success criteria đo được
