# ADR-0008 — Access token của Keycloak là credential API; realtime dùng ticket ngắn hạn

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md), [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Tránh phát hành JWT song song với Keycloak; không đặt JWT trong query string của WebSocket.

## Quyết định

- **REST:** client gửi `Authorization: Bearer <keycloak access token>` (audience `eduworld-api`, TTL 5 phút [Confirmed]). APISIX kiểm tra chữ ký/exp; backend kiểm tra lại chữ ký, `iss`, `aud`, `exp`, rồi ánh xạ `(iss, sub)` → `ExternalIdentity` → `User` → `Principal`.
- Backend **không** phát hành JWT riêng. "Game session" là bản ghi phía server (`game.game_sessions`, game-service) tạo sau khi bootstrap thành công.
- **WebSocket:** client gọi `POST /api/v1/game/sessions/{id}/realtime-tickets` → nhận ticket opaque dùng 1 lần, TTL 60 giây, lưu trong Valkey. Kết nối `wss://.../realtime/v1?ticket=...` tới `realtime-gateway`.
- Khi access token hết hạn, kết nối WS không bị cắt ngay; gateway cắt kết nối khi nhận event `user.access_revoked` (tài khoản bị khóa/vô hiệu hóa) hoặc khi phiên bị thay thế (ADR-0031).

## Hệ quả

(+) Một nguồn phát hành token; ticket ngắn hạn hạn chế lộ credential. (−) Thêm bước lấy ticket trước khi mở WebSocket.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Phase 0 |
| 2026-11-20 | Owner chốt (Accepted) | Owner |
