# Quickstart: F090 — realtime: ticket, WebSocket, heartbeat/reconnect, định tuyến lệnh, cắt kết nối khi thu hồi

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F003, F004, F013, F014.
- Lệnh ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)): `make up`, `make seed`, `make up-app EXCEPT=realtime`, `make run-realtime` (chạy thêm node thứ hai với `EW_NODE_ID=gw-2` và cổng khác cho kịch bản đa node).
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a` (`th-demo-a`), `hs.dung.7a` (`pgd-demo`), `hs.locked`, `ad.truong.a`.
- Công cụ: `websocat` hoặc client headless `tests/e2e/wsclient`, `toxiproxy` (profile compose `chaos`).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.an.3a` `gameLogin` → `createGameSession` → `createRealtimeTicket` | `201`, `ticket`, `url = ws://localhost:9080/realtime/v1`, key `ew:game:rt:ticket:<sha256>` TTL ≤ 60 s | F090-AC-01 |
| 2 | Token `ad.truong.a` gọi `createRealtimeTicket` | `403 GAME_ACCESS_STUDENT_ONLY`, có audit | F090-AC-02 |
| 3 | `hs.dung.7a` gọi ticket cho `game_session_id` của `hs.an.3a`; `hs.an.3a` gửi `X-Tenant-Id` của `pgd-demo` | `404`; `403 TENANT_MISMATCH` | F090-AC-03 |
| 4 | Mở WS với ticket (subprotocol `eduworld.v1`) | `system.session.welcome` đủ trường | F090-AC-05 |
| 5 | Mở WS lần 2 cùng ticket; mở không subprotocol | close `4401`; handshake bị từ chối | F090-AC-06 |
| 6 | Mở kết nối thứ hai của `hs.an.3a` (ticket mới) trên `gw-2` | Kết nối trên `gw-1` đóng `4409` không có message | F090-AC-08, F090-AC-28 |
| 7 | Không gửi frame 50 s | close `4408` | F090-AC-11 |
| 8 | Ngắt mạng 10 s bằng toxiproxy, publish 3 thông báo thử lên `ew:rt:user:<id>`, reconnect + `system.session.resume` | Nhận 3 message đúng `seq`, rồi `system.session.resumed` | F090-AC-13, F090-AC-14 |
| 9 | Gửi resume với token của `hs.binh.3a` | `system.session.resume_failed {reason: unknown_token}` | F090-AC-15 |
| 10 | Gửi lệnh `chat.channel.join` khi module chat chưa chạy | `system.command.failed UNAVAILABLE` sau ~2 s | F090-AC-18 |
| 11 | Gửi frame 20 KB; gửi `type` lạ | `VALIDATION_FAILED`; `UNKNOWN_TYPE` | F090-AC-20 |
| 12 | Gửi 100 frame/giây trong 10 s | `RATE_LIMITED` rồi close `4429`; audit `realtime.rate_limit_closed` | F090-AC-22 |
| 13 | Khóa `hs.an.3a` (`hs.locked` flow, F016) hoặc publish thủ công message điều khiển `access_revoked` | close `4403` ≤ 5 s | F090-AC-24 |
| 14 | Takeover từ thiết bị B (F014) | A nhận `session.session.replaced` rồi `4409`; B không bị ảnh hưởng | F090-AC-25, F090-AC-26 |
| 15 | `docker kill -s TERM gw-1` khi đang có kết nối | readiness 503, nhận `system.server.draining` rồi close `1012`, client resume trên `gw-2` | F090-AC-27 |
| 16 | Dừng Valkey | `/readyz` gateway fail; kết nối nhận `system.service.degraded` | F090-AC-30 |
| 17 | Clock giả 21:35: `hs.an.3a` gọi `createRealtimeTicket` | `403 CURFEW_ACTIVE` + `reopen_at` | F090-AC-31 |
| 18 | `hs.an.3a` đang kết nối; tua 21:29 → 21:30 (job `curfew-session-closer` của F014) | Client nhận `session.session.ended` (`reason = curfew`, `reopen_at`) rồi close `4410`; không tự kết nối lại | F090-AC-32 |

## 3. Lệnh test tự động

```bash
make test MOD=realtime
make test-integration MOD=realtime
make test-integration MOD=game      # ticket, heartbeat NATS
make test-e2e                               # đa node, takeover, resume
```

## 4. Bằng chứng cần nộp

- Kết quả test (pass/fail), trace_id kịch bản 4 và 10, log gateway chứng minh không chứa ticket/payload, số liệu `eduworld_rt_resume_total{result}` sau kịch bản 8.
