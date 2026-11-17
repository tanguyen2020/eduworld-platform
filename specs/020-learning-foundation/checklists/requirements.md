# Specification Quality Checklist: F020 — Nền module learning: projection, mô hình nội dung, gán platform→tenant, thùng rác tenant

**Purpose**: Kiểm chất lượng spec trước khi lập kế hoạch (`/speckit.plan`) · **Created**: 2026-11-16 · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không có chi tiết hiện thực (ngôn ngữ, framework, tên bảng) trong spec.md — các chi tiết đó ở plan.md (ngoại lệ có chủ đích: tên bảng `ref.*` và `infra.processed_events` trong AC vì là quy tắc kiểm được của DATABASE_DESIGN)
- [x] Tập trung vào giá trị cho người dùng và nhà trường
- [x] Mọi mục bắt buộc đã điền

## Requirement Completeness

- [x] Không còn marker [NEEDS CLARIFICATION]
- [x] Mọi FR chính của feature (theo CODE_PLAN) có mặt và có mã PRD đúng (FR-CNT-001, FR-CNT-002, FR-CNT-005, FR-CMS-014, FR-CMS-015)
- [x] Mọi tiêu chí viết theo EARS, kiểm thử được, không mơ hồ
- [x] Có tiêu chí cho authorization, tenant isolation, idempotency/audit (nếu có thao tác ghi), lỗi chính
- [x] Success criteria đo được và không phụ thuộc công nghệ
- [x] Edge case đã liệt kê
- [x] Phạm vi và "Ngoài phạm vi" rõ ràng; không có nội dung deferred
- [x] Phụ thuộc và giả định đã ghi

## Feature Readiness

- [x] Mỗi user story có Independent Test
- [x] Tham số dùng tên CONFIG_DEFAULTS, không chép giá trị
- [x] Không mâu thuẫn tài liệu sở hữu và ADR hiện hành (mâu thuẫn tài liệu với nhau đã ghi ở báo cáo: D6, D8, AC-CMS-003 vs AC-ORG-024)

## Notes

- Mục chưa đạt phải sửa trước khi chuyển Status sang Ready.
