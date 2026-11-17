# Specification Quality Checklist: F110 — Godot — màn boss co-op, thẻ dữ kiện bước `interdependent`, danh sách bạn bè

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

- Cột "FR chính" của CODE_PLAN cho F110 đang trống; spec chọn FR phía client: FR-CHL-002, FR-CHL-003, FR-QST-002, FR-QST-007, FR-QST-009, FR-PTY-013, FR-SOC-001, FR-SOC-002, FR-SOC-003, FR-MP-003 — người điều phối cập nhật CODE_PLAN §4.
- spec.md nêu operationId/message/khóa dịch vì là contract client tiêu thụ; chi tiết scene/script ở plan.md.
- Authorization/tenant ở client: không hiển thị dữ liệu ngoài phạm vi, `X-Tenant-Id` do `ApiClient`; kiểm tra thật ở F095/F103. Idempotency cho mọi lệnh ghi REST; audit do backend.
- Mâu thuẫn đã chốt ở Clarifications: wireframe S06 hiện số câu đúng của từng bạn vs CHALLENGE_BOSS_SYSTEM §3.5/F103-AC-09 (chọn chỉ `contribution_met`); `SocialService` F110 tạo, F119 dùng lại.
