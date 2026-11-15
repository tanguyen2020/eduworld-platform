# ADR-0011 — Realtime transport: WebSocket qua Go realtime server

> **Trạng thái:** Proposed · **Ngày:** 2026-11-15 · **OQ:** OQ-05, OQ-06 · **Tài liệu:** [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md), [GAME_NETWORKING](../08-game-client/GAME_NETWORKING.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Các chế độ chơi chính (Knowledge Battle, Timed Quiz, Co-op puzzle, Boss, Party Lobby, Hub) là **theo lượt/câu hỏi hoặc đồng bộ vị trí tần suất thấp**, không cần vật lý chính xác theo frame. Godot Web export không hỗ trợ ENet/UDP.

## Quyết định

- Một giao thức **WebSocket** (`wss`) tới `realtime-gateway` (ADR-0036) cho: chat, notification, presence, party/lobby, matchmaking events, hub position sync, battle/co-op/boss room. Gateway chuyển lệnh tới service sở hữu qua NATS request/reply và nhận sự kiện qua Valkey Pub/Sub.
- Envelope JSON có phiên bản (`v`, `type`, `id`, `seq`, `payload`); có thể đổi sang MessagePack khi đo đạc cho thấy cần.
- Hub sync: client gửi trạng thái di chuyển ≤ 10 Hz, server kiểm tra tốc độ/biên bản đồ và phát snapshot 10 Hz theo interest grid; client nội suy với buffer ~100 ms.
- Battle/co-op/boss: state machine server-authoritative trong service sở hữu (competition-service, game-service).

## Hệ quả

(+) Một transport cho mọi nền tảng kể cả Web. (−) Không phù hợp cho gameplay vật lý thời gian thực tốc độ cao.

## Phương án chưa chọn

Godot dedicated headless server + ENet/WebRTC: chỉ cân nhắc khi có chế độ chơi cần mô phỏng vật lý/va chạm thời gian thực (ADR mới). Valkey không phải game simulation server.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR (server `cmd/realtime`) | Phase 0 |
| 2026-11-15 | Server realtime là `realtime-gateway` mỏng, logic ở service | ADR-0036 |
