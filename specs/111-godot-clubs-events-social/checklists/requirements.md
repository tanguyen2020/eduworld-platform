# Specification Quality Checklist: F111 — Godot — club trong game, Event center, nhóm "Vừa chơi cùng", nút "Tham gia bạn"

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

- Cột "FR chính" của CODE_PLAN cho F111 đang trống; spec chọn FR phía client: FR-SOC-001, FR-SOC-002, FR-SOC-003, FR-SOC-004, FR-MP-002, FR-MP-003, FR-EVT-001, FR-EVT-003, FR-EVT-004, FR-EVT-005, FR-EVT-006, FR-EVT-007, FR-EVT-008, FR-GAM-001, FR-GAME-010 — người điều phối cập nhật CODE_PLAN §4.
- spec.md nêu operationId/message/khóa dịch vì là contract client tiêu thụ; chi tiết scene/script ở plan.md.
- Authorization/tenant ở client: không hiển thị dữ liệu ngoài phạm vi, `X-Tenant-Id` do `ApiClient`, `404` → đóng màn (F111-AC-31); kiểm tra thật ở F077/F095/F120/F091. Idempotency cho mọi lệnh ghi REST; audit do backend.
- Mâu thuẫn đã chốt ở Clarifications: "Tham gia bạn" dùng `hub.instance.join.follow_user_id` (F091) thay vì `joinFriendInWorld` (đề xuất của STUDENT_GAME_UX_SOCIAL §11 và F071); `g1_3` tab Club rút gọn (UX) trong khuôn khổ "cùng quyền xã hội" (ADR-0020).
- Contract "Cần thêm" (không chặn Ready vì có đường dự phòng ẩn nhóm/nút): `listFriendSuggestions`, `listMyClubInvitations`, `PlayerBootstrap.policies.social`, kind thông báo club.
