# Quickstart: F115 — module chat lõi: channel, policy deny-by-default, JetStream, `chat-persister`, gửi/nhận

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F090 (realtime), F095 (skeleton module chat, projection `ref.*`), F116 nối `ContentValidator` thật (hoặc chạy `make test` với test double có nhãn).
- Lệnh: `make up`, `make seed`, `make up-app EXCEPT=core-api`, `make run-core-api`, `make run-worker PROFILE=chat`; kiểm `make seed-check`, `nats stream info CHAT_MESSAGES` trong `nats-box` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a` (TH Demo A), `hs.chi.3b`, `hs.dung.7a` (tenant `pgd-demo`), `ad.truong.a`, `sm.antoan`.
- Channel thử: `go -C src/backend run ./tools/devtool chat open-channel --type=party --members=hs.an.3a,hs.binh.3a` (lệnh dev gọi command `OpenChannel`, chỉ build tag `dev`).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.an.3a` và `hs.binh.3a` mở WS, `chat.channel.join`; An gửi `chat.message.send` text | An nhận `chat.message.acked` có `seq`; Bình nhận `chat.message.created`; `nats stream view CHAT_MESSAGES` có envelope `Nats-Msg-Id = message_id` | F115-AC-01, F115-AC-02 |
| 2 | An gửi lại cùng `client_message_id` | Cùng `message_id`/`seq`; PG chỉ một dòng | F115-AC-04 |
| 3 | `POST /chat/channels/{id}/messages` không `Idempotency-Key` | `428 IDEMPOTENCY_KEY_REQUIRED` | F115-AC-05 |
| 4 | `docker compose stop nats`; An gửi | `CHAT_TEMPORARILY_UNAVAILABLE`; Bình không nhận | F115-AC-03 |
| 5 | Gửi tin dài hơn `max_text_length` của policy; gửi dồn quá rate | `VALIDATION_FAILED`; `CHAT_RATE_LIMITED` | F115-AC-07 |
| 6 | `hs.chi.3b` (không member) gọi `listChannelMessages` và `chat.channel.join` channel trên | `404`, WS `NOT_FOUND` | F115-AC-10 |
| 7 | `hs.dung.7a` (tenant khác) gọi cùng channel; gửi `X-Tenant-Id` của `th-demo-a` | `404`; `403 TENANT_MISMATCH` | F115-AC-11, F115-AC-12 |
| 8 | Token portal `ad.truong.a`, `sm.antoan` gọi `listMyChannels` | `403 GAME_ACCESS_STUDENT_ONLY` | F115-AC-13 |
| 9 | Xóa rule `party` platform trong DB test (fixture), gửi tin | `CHAT_NOT_ALLOWED` | F115-AC-08 |
| 10 | `docker compose stop postgres` 10 phút, gửi 200 tin, bật lại | Chat vẫn chạy; sau đó lịch sử đủ 200 tin, không trùng `seq` | F115-AC-15, F115-AC-16 |
| 11 | Xóa `ew:chat:seq:<channel_id>` trong Valkey, gửi tiếp | `seq` tiếp nối, không trùng | F115-AC-20 |
| 12 | Client bỏ qua 3 `chat.message.created` rồi gửi `chat.sync` | Nhận đủ 3 tin qua `chat.history.loaded` | F115-AC-18, F115-AC-19 |
| 13 | Clock giả 21:31: An gửi tin `party` qua WS và `class` qua REST | `CURFEW_ACTIVE` (WS `system.command.failed`), không `seq` | F115-AC-22 |
| 14 | Trường TH Demo A chọn preset `quick_message_emote_only`; An gửi `text`, `sticker`, rồi `quick`, `emoji` ở `party` | `text`/`sticker` → `CHAT_KIND_NOT_ALLOWED`; `quick`/`emoji` gửi được | F115-AC-23 |

## 3. Lệnh test tự động

```bash
make test MOD=chat
make test-integration MOD=chat      # testcontainers PG + NATS + Valkey
go -C src/backend test ./modules/chat/tests/chaos/... -tags chaos
go -C src/backend test ./modules/chat/tests/static/... # không grade_*, không DELETE FROM
```

## 4. Bằng chứng cần nộp

- Kết quả test (pass/fail), trace_id kịch bản 1 (gateway → module chat → JetStream), số liệu TC-PERF-010 (send/delivery/persist p95), ảnh `nats consumer info CHAT_MESSAGES persister` sau kịch bản 10.
