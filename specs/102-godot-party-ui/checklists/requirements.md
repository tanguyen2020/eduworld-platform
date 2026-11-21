# Specification Quality Checklist: F102 — Giao diện party trong Godot: panel, lời mời, lobby, ready

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

- spec.md nêu tên scene/script ở Clarifications để chốt mâu thuẫn đặt tên PARTY_SYSTEM §12.4 vs GODOT_ARCHITECTURE §3 (chọn GODOT_ARCHITECTURE).
- Authorization/tenant ở client: không hiển thị dữ liệu ngoài phạm vi, `X-Tenant-Id` do `ApiClient`; kiểm tra thật ở F100/F101.
- Thiếu feature Godot cho màn boss S06 và thẻ dữ kiện `interdependent` — báo lên danh mục.
- ADR-0095 (2026-11-21, WP-S): picker không nhóm theo lớp, không ngoại lệ "bạn cùng lớp"; quy tắc party (lớp 1–3 cùng khối, cùng tenant, liên trường theo ADR-0054, `visibility` `private`/`friends`, `study_party`) do PARTY_SYSTEM sở hữu, client không tự kiểm.
