# Specification Quality Checklist: F131 — Monitoring, alerting, dashboard vận hành, runbook

**Purpose**: Kiểm chất lượng spec trước khi lập kế hoạch (`/speckit.plan`) · **Created**: 2026-11-16 · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không có chi tiết hiện thực (ngôn ngữ, framework, tên bảng) trong spec.md — các chi tiết đó ở plan.md (ngoại lệ có chủ đích: tên công cụ ADR-0023 và tên metric/alert là hợp đồng vận hành)
- [x] Tập trung vào giá trị cho người dùng và nhà trường
- [x] Mọi mục bắt buộc đã điền

## Requirement Completeness

- [x] Không còn marker [NEEDS CLARIFICATION]
- [x] Mọi FR chính của feature (theo CODE_PLAN) có mặt và có mã PRD đúng (FR-OPS-003, NFR-OBS-002)
- [x] Mọi tiêu chí viết theo EARS, kiểm thử được, không mơ hồ
- [x] Có tiêu chí cho authorization, tenant isolation, idempotency/audit (nếu có thao tác ghi), lỗi chính (authorization công cụ F131-AC-07; không dữ liệu tenant trong metric F131-AC-06; lỗi `429` probe F131-AC-20; không endpoint nghiệp vụ)
- [x] Success criteria đo được và không phụ thuộc công nghệ
- [x] Edge case đã liệt kê
- [x] Phạm vi và "Ngoài phạm vi" rõ ràng; không có nội dung deferred
- [x] Phụ thuộc và giả định đã ghi

## Feature Readiness

- [x] Mỗi user story có Independent Test
- [x] Tham số dùng tên CONFIG_DEFAULTS, không chép giá trị
- [x] Không mâu thuẫn tài liệu sở hữu và ADR hiện hành (ADR-0023, ADR-0024)

## Notes

- Mục chưa đạt phải sửa trước khi chuyển Status sang Ready.
- Alert/metric mới (Watchdog, meta, `SyntheticProbeFailing`, `eduworld_synthetic_probe_*`) phải vào MONITORING cùng PR (T040) để giữ R33.
- Cơ chế loại tenant QA khỏi analytics chưa có trong tài liệu — báo cáo cho F080/F043.
