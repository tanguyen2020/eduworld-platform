# Implementation Plan: F117 — Chat gameplay luôn có (`match`/`team`/`party`) và purge chat gameplay

**Branch**: `117-gameplay-chat-purge` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Ba phần trong module chat. (1) **Vòng đời channel gameplay**: consumer JetStream `EVT_COMPETITION` (`party.*`, `match.started/completed/aborted`) gọi command `OpenGameplayChannel`, `SyncGameplayMembers`, `CloseGameplayChannel` (id UUIDv5 từ `pkg/chatid`), cập nhật projection `ref.parties`, phát `chat.channel_opened`/`chat.channel_closed` qua outbox; worker `chat-channel-closer` đặt `purge_after` theo retention tenant. (2) **Always-on**: `policy.ApplyAlwaysOn(effective)` sau `Intersect` (F115) và `policy.ValidateAlwaysOn(rule, level)` dùng cho mọi đường ghi rule; ẩn tin theo block/mute khi fan-out (`exclude_user_ids`) và trong lịch sử; unread qua `chat.receipt.read` → `ew:chat:cursor_buf` → JetStream `cursor` → persister. (3) **Purge**: job `gameplay-chat-purge` (worker, kết nối DB role `chat_purger`, lock Valkey), rollup → bằng chứng → xóa lô → drop partition → `gameplay_purge_runs` + audit + `chat.gameplay_purged`; fail-closed khi module moderation chưa có. Job `partition-maintainer` tạo trước partition ngày.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | ent, Atlas, `nats.go`, `valkey-go`, `pgx` (SQL tường minh của purge ở infrastructure), `github.com/google/uuid` (UUIDv5), `pkg/entx` (`UnitOfWork`), `pkg/outbox`, `pkg/modcall` (identity `getTenant` dự phòng) |
| Storage | PostgreSQL `eduworld_chat` (`chat.channels`, `chat.channel_members`, `chat.gameplay_messages`, `chat.gameplay_partition_channels`, `chat.gameplay_chat_daily_stats`, `chat.gameplay_purge_runs`, `chat.moderation_evidence`, `chat.ephemeral_messages`, `ref.parties`); Valkey `ew:chat:cursor_buf`, `ew:chat:unread`, `ew:lock:job:gameplay-chat-purge` |
| Testing | go test + testcontainers (PG với role `chat_purger` thật, NATS, Valkey), đồng hồ giả lập cho purge |
| Target Platform | Linux container: module `chat` nạp trong `core-api` + profile `worker-chat` (job `gameplay-chat-purge` dùng role `chat_purger`) |
| Project Type | backend module |
| Performance Goals | SC-001 (≤ 2 s từ event tới chat dùng được); purge ≤ 2 h sau hạn |
| Constraints | AGENTS R8, R13, R15, R21, R25, R27, R29; ADR-0020, ADR-0031, ADR-0033, ADR-0035, ADR-0037, ADR-0042, ADR-0070 |
| Scale/Scope | 20% CCU trong trận/co-op (TC-PERF-014) → nhiều channel ngắn hạn mỗi ngày |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta payload competition, event chat, fan-out `exclude_user_ids` ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ tiêu thụ event competition; `pkg/chatid` thuần dùng chung; không đọc DB competition |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Hard-delete **chỉ** trong `internal/infrastructure/purge` với role `chat_purger` — ngoại lệ ADR-0033 cho phép (R21); mọi bảng khác soft-delete; RLS purger chỉ channel gameplay đã đóng |
| IV Server-authoritative | ✅ | Membership từ event server; client không tạo channel |
| V Xác thực, phân quyền | ✅ | Không endpoint mới; route chat giữ `game_player` + membership |
| VI An toàn trẻ em | ✅ | Always-on không cờ tắt (R15); giữ bằng chứng trước purge; block/mute ẩn tin |
| VII Một nguồn sự thật, cấu hình | ✅ | `gameplay_chat_retention_days` từ cài đặt tenant (identity), không env |
| VIII UI tách logic, token | ✅ | Không có UI |
| IX Test có bằng chứng | ✅ | Test mọi F117-AC-*, test quyền DB `chat_purger`, đếm dòng trước/sau |

## Project Structure

### Documentation (feature này)

```text
specs/117-gameplay-chat-purge/
├── spec.md · plan.md · research.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
pkg/chatid/chatid.go                         # ChannelID(kind, scopeID, teamID?) UUIDv5 — thuần, dùng chung chat + competition
modules/chat/
├── ent/schema/                              # refparty.go, gameplaychatdailystat.go, gameplaypurgerun.go, moderationevidence.go, ephemeralmessage.go
├── migrations/                              # role chat_purger + GRANT/RLS, partition ephemeral, policy purger_gameplay_only
├── internal/domain/channel/gameplay.go      # vòng đời Open → Closed, PurgeAfter(closedAt, retention)
├── internal/domain/policy/always_on.go      # ApplyAlwaysOn, ValidateAlwaysOn → ErrAlwaysOn
├── internal/application/command/            # open_gameplay_channel.go, sync_gameplay_members.go, close_gameplay_channel.go, mark_read.go
├── internal/application/port/evidence.go    # EvidenceHolds (report/case/sanction refs), ModerationReadiness
├── internal/infrastructure/purge/           # purger.go (SQL DELETE lô, role chat_purger), evidence_copier.go, partition_dropper.go, rollup.go
├── internal/infrastructure/valkey/cursor_buffer.go
├── internal/presentation/consumer/          # competition_party.go, competition_match.go, identity_settings.go
├── internal/presentation/natscmd/receipt_read.go
├── internal/presentation/job/               # gameplay_chat_purge.go, channel_closer.go, partition_maintainer.go
└── tests/{unit,integration,static}/
```

**Structure Decision**: Mã xóa vật lý chỉ nằm ở `internal/infrastructure/purge` (đúng R21), kết nối DB riêng bằng DSN role `chat_purger`; linter chặn `DELETE` ở mọi package khác.

## Phase 0: Research

→ [research.md](research.md): R1 id channel tất định, R2 ẩn tin theo block/mute khi fan-out, R3 fail-closed khi chưa có module moderation, R4 nguồn retention.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `GameplayChannel` (Open/Close, `PurgeAfter` với sàn 3 ngày), `ApplyAlwaysOn`, `ValidateAlwaysOn` | Thuần |
| application | Command `OpenGameplayChannel`, `SyncGameplayMembers`, `CloseGameplayChannel`, `MarkRead`; Job use case `PurgeExpiredGameplayChannels`, `PurgeEphemeralPartitions` | Không có permission người dùng (consumer/job hệ thống) |
| infrastructure | Consumer `EVT_COMPETITION` durable `chat-gameplay`; purger SQL (`chat_purger`); cursor buffer; adapter facade `identity` `getTenant` (dự phòng) | |
| presentation | Consumer `party.created`, `party.member_joined/left/kicked`, `party.disbanded`, `match.started`, `match.completed`, `match.aborted`, `platform_settings.changed`/`tenant.updated`; NATS `ew.cmd.chat.receipt.read`; job `gameplay-chat-purge`, `chat-channel-closer`, `partition-maintainer` | |

### Tích hợp và phụ thuộc

- F101/F105: phát event party/trận (payload delta), dùng `pkg/chatid` để trả `chat_channel_id`. F115: `OpenChannel`, persister (ghi `gameplay_partition_channels`), fan-out. F116: sanction `mute`. F095: `social.blocks`, `social.mutes`. F121: nguồn report/case cho `EvidenceHolds`. F090: lọc `exclude_user_ids`, auto-subscribe khi nhận `chat.channel_opened`.
- Event phát: `chat.channel_opened`, `chat.channel_closed`, `chat.gameplay_purged`, `chat.daily_stats`, `audit.recorded`.

### Bảo mật, tenant, idempotency, audit

- Consumer dùng `processed_events`; set `app.tenant_id` theo envelope; người tham gia phải có trong `ref.students` cùng tenant.
- Purge: DSN `chat_purger` chỉ nạp trong worker job purge; RLS `purger_gameplay_only`; audit tổng hợp mỗi lượt (`chat.gameplay_purge.run`).

### Quan sát

- Metric: `eduworld_chat_purge_total{tenant,kind}`, `eduworld_chat_purge_deferred_total{reason}`, `eduworld_chat_purge_backlog_channels`, `eduworld_chat_purge_blocked{reason="moderation_unavailable"}`, `eduworld_chat_active_channels`, `eduworld_chat_gameplay_open_latency_seconds`. Alert tồn đọng > 2 ngày, purge lỗi.

## Complexity Tracking

> Không có vi phạm. Hard-delete của job purge là ngoại lệ đã được ADR-0033 cho phép, không phải vi phạm điều III.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
