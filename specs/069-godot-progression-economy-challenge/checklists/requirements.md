# Specification Quality Checklist: F069 — Godot — tiến trình, kho đồ, shop, bộ sưu tập, thành tựu, challenge solo

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

- Cột "FR chính" của CODE_PLAN cho F069 đang trống; spec chọn FR phía client: FR-GAM-001, FR-GAM-006, FR-GAM-007, FR-GAM-008, FR-ECO-001…005, FR-RWD-001…003, FR-CHL-001, FR-CHL-003, FR-GAME-004 — người điều phối cập nhật CODE_PLAN §4.
- spec.md nêu tên operationId/message/khóa dịch vì đó là contract người dùng nhìn thấy (client), không phải chi tiết hiện thực tầng lưu trữ.
- Authorization/tenant ở client: không hiển thị dữ liệu ngoài phạm vi, `X-Tenant-Id` do `ApiClient`; kiểm tra thật ở F074–F076. Idempotency: mua, bắt đầu, hoạt động kế, bỏ cuộc; audit do backend.
- Mâu thuẫn tài liệu đã chốt ở Clarifications: phím `J` (SOCIAL §4 vs STUDENT_GAME_UX §4.1), độ hiếm `g1_3` không chữ vs trợ năng "màu + chữ" (chọn màu + biểu tượng). Delta contract owner F075/F076 ở contracts §6.
