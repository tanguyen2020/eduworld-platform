# Specification Quality Checklist: F130 — Staging/production Docker Swarm, secret, backup PITR và DR drill

**Purpose**: Kiểm chất lượng spec trước khi lập kế hoạch (`/speckit.plan`) · **Created**: 2026-11-16 · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không có chi tiết hiện thực (ngôn ngữ, framework, tên bảng) trong spec.md — các chi tiết đó ở plan.md (ngoại lệ có chủ đích: tên stack file, lệnh `make`, tên metric là hợp đồng vận hành của feature hạ tầng)
- [x] Tập trung vào giá trị cho người dùng và nhà trường
- [x] Mọi mục bắt buộc đã điền

## Requirement Completeness

- [x] Không còn marker [NEEDS CLARIFICATION]
- [x] Mọi FR chính của feature (theo CODE_PLAN) có mặt và có mã PRD đúng (FR-OPS-002, FR-OPS-004, NFR-DEPL-001)
- [x] Mọi tiêu chí viết theo EARS, kiểm thử được, không mơ hồ
- [x] Có tiêu chí cho authorization, tenant isolation, idempotency/audit (nếu có thao tác ghi), lỗi chính (không có endpoint nghiệp vụ — ghi rõ ở Edge Cases; cô lập bằng network/credential F130-AC-07, F130-AC-09; giới hạn input F130-AC-24)
- [x] Success criteria đo được và không phụ thuộc công nghệ
- [x] Edge case đã liệt kê
- [x] Phạm vi và "Ngoài phạm vi" rõ ràng; không có nội dung deferred
- [x] Phụ thuộc và giả định đã ghi

## Feature Readiness

- [x] Mỗi user story có Independent Test
- [x] Tham số dùng tên CONFIG_DEFAULTS, không chép giá trị
- [x] Không mâu thuẫn tài liệu sở hữu và ADR hiện hành (ADR-0024, ADR-0074, ADR-0088 đã áp; ADR-0075 Accepted — F130-AC-14, F130-AC-20 viết lại theo backend S3 managed/Garage/SeaweedFS, floci chỉ local/CI)

## Notes

- Mục chưa đạt phải sửa trước khi chuyển Status sang Ready.
- `reconcile-<module>` chưa có feature sở hữu — rủi ro cho F130-AC-17; đã ghi Assumptions.
- AsyncAPI chưa có `system.server.draining` (DEPLOYMENT §11.2) — delta T004.
