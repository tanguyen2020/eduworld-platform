# ADR-0004 — Valkey chỉ cho trạng thái tạm thời, cache và phối hợp realtime

> **Trạng thái:** Proposed · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [VALKEY_DESIGN](../07-backend/VALKEY_DESIGN.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Realtime (presence, matchmaking, lobby, fan-out) cần kho dữ liệu nhanh, nhưng dữ liệu học tập và kết quả chính thức phải bền vững.

## Quyết định

- Valkey (≥8) dùng cho: cache, presence, matchmaking queue, lobby/party realtime state, room registry, rate limiting, realtime ticket, leaderboard cache (sorted set), **Pub/Sub fan-out** tới các node `realtime-gateway`.
- Valkey **không** dùng Streams cho domain event (đã chuyển sang NATS JetStream, ADR-0042).
- Valkey **không** là nguồn dữ liệu duy nhất cho learning record, kết quả thi/trận, rating, bracket, thông tin học sinh, nội dung đã publish, ledger kinh tế. Mọi dữ liệu trong Valkey phải tái tạo được từ PostgreSQL hoặc chấp nhận mất (presence, queue → client tạo lại).
- Key có prefix theo service `ew:<svc>:` và ACL theo service (VALKEY_DESIGN).

## Hệ quả

Valkey chạy AOF `everysec` nhưng thiết kế chịu được mất dữ liệu: mất Valkey = suy giảm realtime, không mất dữ liệu học tập.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Phase 0 |
| 2026-11-15 | Bỏ vai trò Streams cho event nội bộ; prefix/ACL theo service | ADR-0042, ADR-0036 |
