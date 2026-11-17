# ADR-0083 — Đăng nhập game và tạo game session là hai bước riêng

> **Trạng thái:** Accepted · **Ngày:** 2026-11-16 · **OQ:** — · **Bổ sung:** ADR-0031, ADR-0045, ADR-0068 · **Tài liệu:** [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [GAME_NETWORKING](../08-game-client/GAME_NETWORKING.md) · [GODOT_ARCHITECTURE](../08-game-client/GODOT_ARCHITECTURE.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Spec `GameLoginResponse` (identity) trả `game_session_id`, ngụ ý `/auth/game/login` tạo luôn game session; GODOT_ARCHITECTURE và feature F013/F014 lại mô tả một lời gọi `POST /game/sessions` riêng (CODE_PLAN D15). Tạo game session trong lúc login buộc identity-service gọi đồng bộ game-service trên đường đăng nhập, trái AGENTS R29/R30 và ADR-0045 (đường đăng nhập nhẹ), đồng thời làm identity phụ thuộc dữ liệu game (ADR-0031 thuộc game-service).

## Quyết định

1. `POST /api/v1/auth/game/login` (và `tenant:select`) của identity-service chỉ **xác thực, xác định tenant, trả token và bootstrap tối thiểu**; **không** tạo game session, **không** trả `game_session_id`, **không** gọi game-service. Token nằm trong `tokens.*`.
2. Ngay sau khi có token, game client gọi `POST /api/v1/game/sessions` của game-service (header `X-Device-Id`, `X-Tenant-Id`) để tạo game session theo quy tắc một thiết bị (ADR-0031). Xung đột thiết bị trả `409 ACTIVE_SESSION_ON_OTHER_DEVICE` từ endpoint này; client hỏi người chơi rồi gọi lại với `takeover: true` — không cần nhập lại mật khẩu.
3. `POST /api/v1/auth/game/logout` chỉ thu hồi token/phiên Keycloak; client gọi `POST /api/v1/game/sessions/{id}:end` trước khi logout; game-service tự đóng phiên theo heartbeat timeout nếu client không gọi được.
4. Sửa spec: bỏ `game_session_id` khỏi `GameLoginResponse`; giữ/hoàn thiện `createGameSession` và response `409` ở game-service.

## Hệ quả

(+) Đường đăng nhập không có lời gọi đồng bộ liên service; ranh giới dữ liệu identity/game rõ; takeover không phải giữ mật khẩu trong bộ nhớ. (−) Thêm một round-trip sau đăng nhập; phải sửa spec identity, F061 và các tài liệu client.

## Phương án đã loại

Login tạo luôn game session (spec cũ): thêm lời gọi đồng bộ identity → game-service trên đường đăng nhập, trái R30.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-16 | Tạo ADR; đóng CODE_PLAN D15 | Technical Architect |
