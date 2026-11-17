# Specification Quality Checklist: F003 — Platform kit dữ liệu: entx + UnitOfWork, CQRS read/write, idempotency, outbox, modcall, testkit

**Purpose**: Kiểm chất lượng spec trước khi lập kế hoạch (`/speckit.plan`) · **Created**: 2026-11-16 · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không có chi tiết hiện thực (ngôn ngữ, framework, tên bảng) trong spec.md — các chi tiết đó ở plan.md (tên bảng `infra.*` trong tiêu chí là hợp đồng dữ liệu của platform kit)
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
- [x] Không mâu thuẫn tài liệu sở hữu và ADR hiện hành (D5, D6, tên metric và mã xung đột đã chốt ở Clarifications; bỏ mediator theo ADR-0087)

## Notes

- 2026-11-16: viết lại User Story 2 theo ADR-0087 (write/read context, `UnitOfWork`, `ReadSession`, `observability.UseCase`, rule linter); thêm F003-AC-33; idempotency chuyển sang middleware `httpx.Idempotency`.
- Kiểu cột `infra.dead_letters` và CHECK kích thước `idempotency_records` là đề xuất, có task cập nhật DATABASE_DESIGN.
