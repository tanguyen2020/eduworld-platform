# Specification Quality Checklist: F112 — Godot — channel chat bền vững (khu vực, DM) và công cụ chủ club (sửa club, phó nhóm, mục tiêu/sự kiện club)

**Purpose**: Kiểm chất lượng spec trước khi lập kế hoạch (`/speckit.plan`) · **Created**: 2026-11-28 · **Feature**: [spec.md](../spec.md)

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

- Cột "FR chính" của CODE_PLAN cho F112 trước đây ghi "(FR CHAT/SOC phía client — ghi khi viết spec)"; spec chọn FR phía client: FR-CHAT-001, FR-CHAT-003, FR-CHAT-004, FR-CHAT-006, FR-CHAT-016, FR-SOC-003, FR-SOC-004, FR-MOD-002, FR-GAME-010 — đã ghi vào dòng CODE_PLAN.
- spec.md nêu operationId/message/khóa dịch vì là contract client tiêu thụ; chi tiết scene/script ở plan.md.
- Authorization/tenant ở client: chỉ hiển thị theo vai trò/trạng thái server trả, `X-Tenant-Id` do `ApiClient`, `403` → ẩn công cụ, `404` → đóng màn (F112-AC-18, F112-AC-31); kiểm tra thật ở F120 (`game_player`, owner/officer, membership). Idempotency: `createClubGoal`, `createClubEvent` có `Idempotency-Key`; `updateClub` khóa lạc quan `If-Match`; `updateClubMemberRole` là đặt giá trị (lặp lại an toàn). Audit do backend.
- Lệch giữa tài liệu sở hữu và contract đã chốt ở Clarifications (client chọn phạm vi hẹp của SOCIAL_SYSTEM §6.2, câu hỏi mở F112-1); thiếu contract có đường dự phòng (F112-3…F112-9), không chặn Ready.
- Ma trận ảnh khối × cỡ điện thoại theo F125 là tiêu chí F112-AC-28.
