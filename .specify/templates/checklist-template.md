# Specification Quality Checklist: F[NNN] — [Tên tính năng]

**Purpose**: Kiểm chất lượng spec trước khi lập kế hoạch (`/speckit.plan`) · **Created**: [YYYY-MM-DD] · **Feature**: [spec.md](../spec.md)

## Content Quality

- [ ] Không có chi tiết hiện thực (ngôn ngữ, framework, tên bảng) trong spec.md — các chi tiết đó ở plan.md
- [ ] Tập trung vào giá trị cho người dùng và nhà trường
- [ ] Mọi mục bắt buộc đã điền

## Requirement Completeness

- [ ] Không còn marker [NEEDS CLARIFICATION]
- [ ] Mọi FR chính của feature (theo CODE_PLAN) có mặt và có mã PRD đúng
- [ ] Mọi tiêu chí viết theo EARS, kiểm thử được, không mơ hồ
- [ ] Có tiêu chí cho authorization, tenant isolation, idempotency/audit (nếu có thao tác ghi), lỗi chính
- [ ] Success criteria đo được và không phụ thuộc công nghệ
- [ ] Edge case đã liệt kê
- [ ] Phạm vi và "Ngoài phạm vi" rõ ràng; không có nội dung deferred
- [ ] Phụ thuộc và giả định đã ghi

## Feature Readiness

- [ ] Mỗi user story có Independent Test
- [ ] Tham số dùng tên CONFIG_DEFAULTS, không chép giá trị
- [ ] Không mâu thuẫn tài liệu sở hữu và ADR hiện hành

## Notes

- Mục chưa đạt phải sửa trước khi chuyển Status sang Ready.
