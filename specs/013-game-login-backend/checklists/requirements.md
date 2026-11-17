# Specification Quality Checklist: F013 — Đăng nhập game qua backend: login, chọn tenant, refresh, logout, bảo vệ đăng nhập, hàng chờ

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

- D15 đã chốt bởi ADR-0083: login không tạo game session ([research.md](../research.md) R3).
- ADR-0086 (2026-11-16): refresh có `device_id`, rotation + phát hiện dùng lại, `mode = resume`, thu hồi phiên game theo sự kiện — F013-AC-12…15 viết lại, thêm F013-AC-27…29 ([research.md](../research.md) R4).
- `min_client_version` chưa có trong CONFIG_DEFAULTS — task Polish đăng ký.
- Đăng nhập không nhận `Idempotency-Key` (API_DESIGN §9.1: không lưu token vào kho idempotency); logout idempotent theo ngữ nghĩa.
