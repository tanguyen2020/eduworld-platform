# Specification Quality Checklist: F043 — Rollup analytics và School Learning Dashboard API

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

- spec.md dùng tên tham số/tên trường nghiệp vụ (`data_as_of`, `is_partial`, `rapid_guess`) theo tài liệu sở hữu; tên bảng rollup chỉ ở plan.md/data-model.md; riêng F043-AC-18 nêu bảng nghiệp vụ bị cấm đọc vì đó là ràng buộc kiểm thử được của ADR-0062.
- Feature chỉ đọc (không có thao tác ghi từ API); idempotency áp cho consumer rollup (`processed_events`), audit cho xem hồ sơ học sinh và đọc liên tenant.
