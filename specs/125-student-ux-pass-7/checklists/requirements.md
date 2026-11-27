# Specification Quality Checklist: F125 — Student UX pass 7: phóng to giao diện trên điện thoại và nút đóng đỏ cho mọi cửa sổ

**Purpose**: kiểm chất lượng spec trước khi lập plan · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không mô tả chi tiết cài đặt trong spec (chỉ hành vi, ngưỡng đã duyệt và tiêu chí)
- [x] Tập trung giá trị người dùng (đọc rõ, bấm trúng trên điện thoại; mọi cửa sổ đóng cùng một cách)
- [x] Mã FR/NFR chỉ từ PRD (FR-GAME-010, NFR-A11Y-001, NFR-COMP-001)

## Requirement Completeness

- [x] Không còn [NEEDS CLARIFICATION]; mọi điểm chốt [Confirmed] (owner 2026-11-27, "chốt hết"; Clarifications plan)
- [x] Mỗi tiêu chí F125-AC-* theo EARS và kiểm được
- [x] Tham số trỏ về tài liệu sở hữu (STUDENT_GAME_UX §2; DESIGN_SYSTEM §5.2, §9)
- [x] Edge case có tiêu chí (xoay giữa hai chiều ngang, cửa sổ trình duyệt thấp, cỡ chữ +2, `en`/pseudo, bàn phím ảo)
- [x] Phạm vi và ngoài phạm vi rõ ràng (không đổi canvas máy tính/tablet, không bố cục dọc, không API)
- [x] Phụ thuộc (F113, F114, F119, F123, F102) và ADR (0047, 0057) ghi rõ

## Feature Readiness

- [x] Mọi AC có task test trong tasks.md
- [x] User story độc lập, có Independent Test
- [x] Success criteria đo được
