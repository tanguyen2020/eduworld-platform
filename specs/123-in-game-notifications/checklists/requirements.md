# Specification Quality Checklist: F123 — Thông báo trong game: realtime, lưu bền vững, không push

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

- Bảng `social.notifications` chưa có trong DATABASE_DESIGN — task cập nhật (Polish).
- Enum loại thông báo: đã chốt một enum `kind` dùng chung REST và WS (RB-513); delta AsyncAPI ở Phase 2.
- "Thông báo trường" tự soạn chưa có FR/contract — ghi ngoài phạm vi; cần PM xác nhận có thuộc FR-NTF-001 không.
- Không audit từng thông báo (không phải thao tác quan trọng theo R10); thao tác ghi duy nhất của người dùng là đánh dấu đã đọc (idempotent).
