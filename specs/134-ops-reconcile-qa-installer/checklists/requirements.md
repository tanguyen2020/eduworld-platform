# Specification Quality Checklist: F134 — Công cụ vận hành bổ sung: lệnh `reconcile-<module>`, loại tenant QA khỏi analytics, trang tải bộ cài cho trường

**Purpose**: Kiểm chất lượng spec trước khi lập kế hoạch (`/speckit.plan`) · **Created**: 2026-11-16 · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không có chi tiết hiện thực (ngôn ngữ, framework, tên bảng) trong spec.md — các chi tiết đó ở plan.md (ngoại lệ có chủ ý: tên bảng hạ tầng `infra.*` vì là đối tượng nghiệp vụ của công cụ vận hành)
- [x] Tập trung vào giá trị cho người dùng và nhà trường
- [x] Mọi mục bắt buộc đã điền

## Requirement Completeness

- [x] Không còn marker [NEEDS CLARIFICATION]
- [x] Mọi FR chính của feature (theo CODE_PLAN) có mặt và có mã PRD đúng — CODE_PLAN chưa ghi FR; spec chọn FR-OPS-002, NFR-DEPL-002, FR-ANL-009, FR-ANL-012, FR-OPS-003, NFR-PERF-019, FR-GAME-009
- [x] Mọi tiêu chí viết theo EARS, kiểm thử được, không mơ hồ
- [x] Có tiêu chí cho authorization (AC-05, AC-07, AC-15), tenant isolation (AC-10, AC-15), idempotency/audit (AC-06, AC-07), lỗi chính (`UNAVAILABLE`, `VALIDATION_FAILED`, `401/403/404`), giới hạn độ dài (AC-05, AC-16)
- [x] Success criteria đo được và không phụ thuộc công nghệ
- [x] Edge case đã liệt kê
- [x] Phạm vi và "Ngoài phạm vi" rõ ràng; không có nội dung deferred
- [x] Phụ thuộc và giả định đã ghi

## Feature Readiness

- [x] Mỗi user story có Independent Test
- [x] Tham số dùng tên CONFIG_DEFAULTS, không chép giá trị (`reconcile_settle_minutes` mới ghi [Proposed], task T038)
- [x] Không mâu thuẫn tài liệu sở hữu và ADR hiện hành

## Notes

- Feature chạm thêm module `identity` và module `learning` (cờ QA, tổng hợp học tập platform) — cột Module trong CODE_PLAN chỉ ghi module `game`; đề nghị cập nhật.
- Phụ thuộc thực tế ngoài cột "Phụ thuộc": F010 (`updateTenant`), F043/F044 (tổng hợp platform), F057 (form tenant).
- `TC-SVC-006` mô tả `403 PERMISSION_DENIED` trong khi `ProblemCode` có `CALLER_NOT_ALLOWED` — dùng mã do `pkg/modcall` chuẩn hóa, cần thống nhất ở TEST_STRATEGY/API_DESIGN.
