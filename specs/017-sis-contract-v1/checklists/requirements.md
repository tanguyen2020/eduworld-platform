# Specification Quality Checklist: F017 — SIS `contract_v1`: provider API theo EduWorld SIS Contract, webhook, conformance kit, `sis-mock` (tách từ F011)

**Purpose**: Kiểm chất lượng spec trước khi lập kế hoạch (`/speckit.plan`) · **Created**: 2026-11-16 · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không có chi tiết hiện thực (ngôn ngữ, framework, tên bảng) trong spec.md — các chi tiết đó ở plan.md
- [x] Tập trung vào giá trị cho người dùng và nhà trường
- [x] Mọi mục bắt buộc đã điền

## Requirement Completeness

- [x] Không còn marker [NEEDS CLARIFICATION]
- [x] Mọi FR chính của feature (theo CODE_PLAN) có mặt và có mã PRD đúng (FR-SIS-001, FR-SIS-002, FR-SIS-003, FR-SIS-004, FR-SIS-006, FR-SIS-007, FR-SIS-008, FR-SIS-009, FR-ORG-018 — phần `contract_v1` tách từ F011)
- [x] Mọi tiêu chí viết theo EARS, kiểm thử được, không mơ hồ
- [x] Có tiêu chí cho authorization (AC-28), tenant isolation (AC-28), idempotency/audit (AC-19, AC-29), lỗi chính (AC-02, AC-07, AC-12, AC-14, AC-18…20), giới hạn độ dài (AC-30)
- [x] Success criteria đo được và không phụ thuộc công nghệ
- [x] Edge case đã liệt kê
- [x] Phạm vi và "Ngoài phạm vi" rõ ràng; không có nội dung deferred
- [x] Phụ thuộc và giả định đã ghi

## Feature Readiness

- [x] Mỗi user story có Independent Test
- [x] Tham số dùng tên CONFIG_DEFAULTS, không chép giá trị
- [x] Không mâu thuẫn tài liệu sở hữu và ADR hiện hành

## Notes

- Ngưỡng vi phạm, backoff, tốc độ, circuit breaker chưa có tên trong CONFIG_DEFAULTS: spec link SIS_INTEGRATION §2.5/§4.2/§7, task Polish đăng ký tên.
- Mâu thuẫn `SyncRun` OpenAPI ↔ DATABASE_DESIGN, `sis_lookup_timeout` ↔ `sis_login_sync_timeout_ms` (đã chốt: chỉ `sis_login_sync_timeout_ms`), retention staging (đã chốt 30 ngày) — RB-110, RB-111.
