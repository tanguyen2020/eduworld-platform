# Specification Quality Checklist: F001 — Monorepo, Docker Compose local, CI GitHub Actions theo path, khung modular monolith

**Purpose**: Kiểm chất lượng spec trước khi lập kế hoạch (`/speckit.plan`) · **Created**: 2026-11-16 · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không có chi tiết hiện thực (ngôn ngữ, framework, tên bảng) trong spec.md — các chi tiết đó ở plan.md (ngoại lệ có chủ đích: tên lệnh `make`, tên binary, tên workflow, tên database/role là hợp đồng vận hành của feature hạ tầng)
- [x] Tập trung vào giá trị cho người dùng và nhà trường
- [x] Mọi mục bắt buộc đã điền

## Requirement Completeness

- [x] Không còn marker [NEEDS CLARIFICATION]
- [x] Mọi FR chính của feature (theo CODE_PLAN) có mặt và có mã PRD đúng
- [x] Mọi tiêu chí viết theo EARS, kiểm thử được, không mơ hồ
- [x] Có tiêu chí cho authorization, tenant isolation, idempotency/audit (nếu có thao tác ghi), lỗi chính (không có endpoint nghiệp vụ — ghi rõ ở Edge Cases)
- [x] Success criteria đo được và không phụ thuộc công nghệ
- [x] Edge case đã liệt kê
- [x] Phạm vi và "Ngoài phạm vi" rõ ràng; không có nội dung deferred
- [x] Phụ thuộc và giả định đã ghi

## Feature Readiness

- [x] Mỗi user story có Independent Test
- [x] Tham số dùng tên CONFIG_DEFAULTS, không chép giá trị
- [x] Không mâu thuẫn tài liệu sở hữu và ADR hiện hành (ADR-0074 modular monolith, ADR-0088 GitHub Actions; cổng/biến/target theo LOCAL_DEVELOPMENT §3.2, §4, §7)

## Notes

- Mục chưa đạt phải sửa trước khi chuyển Status sang Ready.
- 2026-11-17: viết lại theo ADR-0074 (một `go.mod`, ba binary, `modules/<module>/`, ranh giới module F001-AC-21…23) và ADR-0088 (GitHub Actions, F001-AC-12…16, F001-AC-24…26); bỏ toàn bộ GitLab CI, `go.work`, image/tag theo service, cổng internal 82xx.
