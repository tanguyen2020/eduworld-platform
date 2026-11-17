# Implementation Plan: F115 — module chat lõi: channel, policy deny-by-default, JetStream, `chat-persister`, gửi/nhận

**Branch**: `115-chat-core` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Thêm bounded context `chat` vào module chat (skeleton từ F095): aggregate `Channel` + `ChannelMember`, `PolicyResolver` (giao 4 cấp, deny-by-default, cache Valkey), use case duy nhất `SendMessage` dùng chung cho handler NATS `ew.cmd.chat.message.send` và REST `sendChannelMessage`. Chuỗi xử lý: membership → effective policy (kind, độ dài, rate) → `ContentValidator` (port, F116 hiện thực) → `seq_next.lua` → publish JetStream `ew.chat.<tenant_id>.msg.<channel_id>` (`Nats-Msg-Id = message_id`, chờ PubAck ≤ 2 s) → reply `chat.message.acked` → `LPUSH ew:chat:recent` + `PUBLISH ew:chat:fanout:<channel_id>`. Worker `chat-persister` (durable pull `persister`) ghi `chat.messages`/`chat.gameplay_messages` theo lô, `ON CONFLICT DO NOTHING`, cập nhật `channels.last_seq` bằng `GREATEST`, outbox `chat.message_created`, ack sau commit, DLQ. Query lịch sử hợp nhất PG + `ew:chat:recent`.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, `nats.go` (JetStream + core request/reply), Valkey client (`valkey-go`), `pkg/outbox`, `pkg/idempotency`, `pkg/httpx` (`game_player`, `tenant`), `pkg/tenancy`, `pkg/entx` |
| Storage | PostgreSQL `eduworld_chat` (`chat.channels`, `chat.channel_members`, `chat.messages` P tháng, `chat.gameplay_messages` P ngày, `chat.chat_policies`, `infra.*`); Valkey (`ew:chat:seq|recent|cmid`, `ew:rl:chat`, `ew:cache:policy:chat`); NATS JetStream stream `CHAT_MESSAGES`, `CHAT_DLQ` |
| Testing | go test + testcontainers (PostgreSQL 17, NATS 2.10 `-js`, Valkey); bot Go WS cho TC-PERF-010/011 |
| Target Platform | Linux container: module `chat` nạp trong `core-api` + profile `worker-chat` |
| Project Type | backend module |
| Performance Goals | TC-PERF-010, TC-PERF-011; [CHAT_SYSTEM §5.3](../../docs/03-social-multiplayer/CHAT_SYSTEM.md) |
| Constraints | AGENTS R8, R13, R14, R15, R17, R21, R25, R26, R27, R28; ADR-0020, ADR-0035, ADR-0074, ADR-0037, ADR-0038, ADR-0039, ADR-0042 |
| Scale/Scope | `chat_rate_per_user`, `chat_recent_buffer`; NFR-SCAL-003 (10.000 CCU) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | spec Ready; delta contract (AsyncAPI `held`→`moderation_state`, envelope schema, `chat.message_created` payload) ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ module chat; gateway chỉ định tuyến (F090); không gọi module khác đồng bộ; command (write context) tách query (read context) hợp nhất read-only (ADR-0087) |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Mọi bảng T + RLS + interceptor; subject JetStream chứa `tenant_id`, persister đối chiếu tenant với channel; test isolation mọi route |
| IV Server-authoritative | ✅ | `seq`, thời điểm, quyết định kiểm duyệt ở server; REST có `Idempotency-Key`, WS có `client_message_id` |
| V Xác thực, phân quyền | ✅ | `game_player` cho `/chat/*`; NATS chỉ tin header từ user NATS `realtime` (binary `realtime`); membership = authorization nghiệp vụ |
| VI An toàn trẻ em | ✅ | R15 (một use case, không điều kiện khối), R25 (ack trước fan-out), R14, R16 (`PlayerCard`) |
| VII Một nguồn sự thật, cấu hình | ✅ | Giới hạn đọc từ effective policy + CONFIG_DEFAULTS; chuỗi `varchar(n)` theo DATABASE_DESIGN |
| VIII UI tách logic, token | ✅ | Không có UI |
| IX Test có bằng chứng | ✅ | Test mọi F115-AC-*, chaos NATS/PG/Valkey, test tĩnh không `grade_*` |

## Project Structure

### Documentation (feature này)

```text
specs/115-chat-core/
├── spec.md · plan.md · research.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
modules/chat/
├── ent/schema/                         # channel.go, channelmember.go, message.go, gameplaymessage.go, chatpolicy.go
├── migrations/                         # Atlas: partition, RLS, unaccent wrapper, REVOKE DELETE
├── internal/domain/channel/            # Channel, ChannelMember, ChannelType, Status
├── internal/domain/policy/             # Rule, EffectivePolicy, Intersect(), ErrDenied (không có trường grade)
├── internal/domain/message/            # Message, Kind, ModerationState, Envelope
├── internal/application/command/       # send_message.go, open_channel.go, add_member.go, remove_member.go
├── internal/application/query/         # list_my_channels.go, list_channel_messages.go, sync_channel.go
├── internal/application/port/          # ContentValidator, SeqAllocator, ChatStream, Fanout, RecentBuffer, RateLimiter, PolicyStore, Clock
├── internal/infrastructure/persistence/  # ent repo channel/member/policy/message reader
├── internal/infrastructure/natsstream/ # jetstream_publisher.go, stream_bootstrap.go (CHAT_MESSAGES, CHAT_DLQ)
├── internal/infrastructure/valkey/     # seq_next.lua, seq_allocator.go, recent_buffer.go, fanout.go, rate_limiter.go, cmid_store.go, policy_cache.go
├── internal/presentation/natscmd/      # handler ew.cmd.chat.message.send, chat.channel.join/leave, chat.history.fetch, chat.sync
├── internal/presentation/http/public/  # channels.go (listMyChannels), messages.go (listChannelMessages, sendChannelMessage)
├── internal/presentation/worker/persister/  # batch_consumer.go, writer.go, dlq.go
└── tests/{unit,integration,contract,chaos,static}/
```

**Structure Decision**: Theo BACKEND_DESIGN §3 (D1: `modules/<name>/`); handler lệnh NATS là một adapter presentation riêng (`natscmd`) ngang hàng HTTP, cùng gọi command/query handler qua interface có kiểu.

## Phase 0: Research

→ [research.md](research.md): R1 bảng ghi tin theo loại channel, R2 kiểm tra policy trên đường gửi (cache), R3 hợp nhất lịch sử PG + bộ đệm.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `Channel` (Open, Close, IsMember), `ChannelMember`, `policy.Intersect(levels...)` → `EffectivePolicy` (deny khi thiếu rule), `Message.NewFromDecision` | Thuần Go, không `grade_*` |
| application | Command `SendMessage` (idempotency `client_message_id`/`Idempotency-Key`), `OpenChannel`, `AddMember`, `RemoveMember` (nội bộ, gọi từ consumer F117/F120); Query `ListMyChannels`, `ListChannelMessages`, `SyncChannel` | Authorization: `game_player` + membership; không có permission catalog cho học sinh |
| infrastructure | ent repo; JetStream publisher (PubAck timeout 2 s); Valkey seq/recent/fanout/rate/cmid; policy cache `ew:cache:policy:chat:<tenant_id>:<school_id>` | `ContentValidator` mặc định ở test là `labeled_test_double` (R17) |
| presentation | `listMyChannels`, `listChannelMessages`, `sendChannelMessage`; NATS `ew.cmd.chat.message.send`, `ew.cmd.chat.channel.join`, `ew.cmd.chat.channel.leave`, `ew.cmd.chat.history.fetch`, `ew.cmd.chat.sync`; worker `persister` | |

### Tích hợp và phụ thuộc

- F090: định tuyến lệnh, header `Ew-*`, subscribe fan-out. F095: skeleton, projection `ref.*`, `social.blocks` (block ẩn tin hai chiều khi fan-out/lịch sử).
- Event phát (outbox): `chat.message_created`, `audit.recorded`. JetStream: `ew.chat.<tenant_id>.msg.<channel_id>`, `CHAT_DLQ.persister`.
- Không gọi facade module khác (R30: đường gửi không gọi đồng bộ module khác).

### Bảo mật, tenant, idempotency, audit

- `game_player` (`azp = eduworld-game-backend`, `user_type = student`) cho `/chat/*`; lệnh NATS chỉ chấp nhận từ user NATS `realtime`.
- `TenantScope` từ Principal/`Ew-Tenant-Id`; persister set `app.tenant_id` theo envelope, bỏ envelope có `tenant_id` khác channel (log + metric).
- Idempotency: Valkey `SET NX ew:chat:cmid:<sender>:<client_message_id>`; REST qua `pkg/idempotency` (`infra.idempotency_records`).
- Audit: từ chối truy cập channel trái phép (`chat.channel.access_denied`, không nội dung).

### Quan sát

- Metric: `eduworld_chat_messages_total{channel_type,kind,decision}`, `eduworld_chat_send_latency_seconds`, `eduworld_chat_delivery_latency_seconds`, `eduworld_chat_persist_lag_seconds`, `eduworld_chat_jetstream_publish_errors_total`, `eduworld_chat_rejected_total{code}`, `eduworld_chat_seq_gaps_total`.
- Span: `chat.send` (gateway → module chat qua `traceparent` header NATS → JetStream publish), `chat.persist.batch`. Log không chứa `body`.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
