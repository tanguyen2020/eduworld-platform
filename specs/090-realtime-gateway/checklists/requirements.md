# Specification Quality Checklist: F090 — realtime: ticket, WebSocket, heartbeat/reconnect, định tuyến lệnh, cắt kết nối khi thu hồi

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

- Spec nêu tên key/kênh ở mức khái niệm (subject `ew.cmd.*`, topic Pub/Sub) vì đó là hành vi kiểm thử được của gateway; chi tiết hiện thực ở plan/data-model.
- Mâu thuẫn tài liệu cần sửa (task Polish): close code thu hồi `4403` và tắt node `system.server.draining` + `1012` (đã chốt RB-401, tài liệu sở hữu đã sửa); `ws_connections_per_user = 2` ở CONFIG_DEFAULTS; định tuyến `hub.*` trong AsyncAPI.
- Tham số `ws_frame_rate`, `ws_inflight_commands`, `ws_send_queue`, `ws_accept_rate` chưa có trong CONFIG_DEFAULTS — đăng ký ở Polish.
