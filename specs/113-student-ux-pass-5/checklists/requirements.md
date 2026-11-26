# Specification Quality Checklist: F113 — Student UX pass 5: game luôn chơi ngang (khóa ngang Android/iOS, màn "Xoay ngang" trên Web)

**Purpose**: kiểm chất lượng spec trước khi lập plan · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không mô tả chi tiết cài đặt trong spec (chỉ hành vi và tiêu chí)
- [x] Tập trung giá trị người dùng (luôn chơi ngang, nhắc xoay thân thiện, không mất bài)
- [x] Mã FR/NFR chỉ từ PRD

## Requirement Completeness

- [x] Không còn [NEEDS CLARIFICATION]; điểm thiếu chốt [Confirmed] ở Session 2026-11-26 (F099-1, F099-2 superseded)
- [x] Mỗi tiêu chí F113-AC-* theo EARS và kiểm được
- [x] Tham số trỏ về tài liệu sở hữu (STUDENT_GAME_UX §2, GODOT_ARCHITECTURE §10.2, DESIGN_SYSTEM §5.2, §9)
- [x] Edge case có tiêu chí
- [x] Phạm vi và ngoài phạm vi rõ ràng (không bố cục dọc mới, không chặn trình duyệt điện thoại)
- [x] Phụ thuộc (F096–F099, F133) và ADR (0047, 0086, 0101) ghi rõ

## Feature Readiness

- [x] Mọi AC có task test trong tasks.md
- [x] User story độc lập, có Independent Test
- [x] Success criteria đo được
