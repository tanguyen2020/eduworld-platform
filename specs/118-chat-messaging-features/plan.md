# Implementation Plan: F118 — Bộ tính năng nhắn tin đầy đủ (sửa/thu hồi, reply/forward, mention, receipts) và sticker/GIF

**Branch**: `118-chat-messaging-features` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Mở rộng aggregate `Message` (F115) với `Edit`, `Recall`, `Reply`, `Quote`, `ForwardTo`, `ToggleReaction`, `Pin/Unpin`, và aggregate `MemberCursor`. Mọi thay đổi tin là command handler (write context): kiểm quyền + `features` của effective policy → (nội dung mới) `ContentValidator` (F116) → publish JetStream `ew.chat.<tenant_id>.evt.<channel_id>` (`edit`, `recall`, `reaction_set/unset`, `pin/unpin`) hoặc `msg` (forward = tin mới ở đích) → reply ack → fan-out. `chat-persister` áp sự kiện theo `revision` (bỏ bản cũ đến muộn), ghi `message_revisions`, `message_reactions`, `pinned_messages`, con trỏ. Typing và broadcast receipts chỉ qua Valkey Pub/Sub. Sticker/GIF: query cho học sinh + command `/game-admin/{sticker-packs|gifs}` cho Game Studio (presign upload object storage, kiểm tệp, duyệt, publish, version). Projection `ref.sticker_unlocks` từ event module game.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | ent, Atlas, oapi-codegen, `nats.go`, `valkey-go`, S3 SDK (floci, ADR-0071), `image/gif`/`golang.org/x/image/webp` kiểm kích thước, `src/backend/pkg/entx` (`UnitOfWork`), `src/backend/pkg/outbox`, `src/backend/pkg/idempotency` |
| Storage | PostgreSQL `eduworld_chat` (`chat.message_revisions`, `chat.message_reactions`, `chat.pinned_messages`, `chat.sticker_packs`, `chat.stickers`, `chat.gif_assets`, `chat.gif_tags`, `chat.chat_asset_versions` (Mới), `ref.sticker_unlocks` (Mới), `infra.tenant_resource_assignments`); Valkey `ew:chat:typing`, `ew:chat:cursor_buf`, `ew:chat:mentions`, `ew:chat:unread`; object storage + CDN EduWorld cho asset |
| Testing | go test + testcontainers (PG, NATS, Valkey, floci S3) |
| Target Platform | Linux container: module `chat` nạp trong `core-api` + profile `worker-chat` |
| Project Type | backend module |
| Performance Goals | Receipts ≤ 2 s (SC-002); tải typing/receipt TC-PERF-010 |
| Constraints | AGENTS R10, R14, R16, R20, R21, R25, R26, R28, R31; ADR-0027, ADR-0029, ADR-0034, ADR-0035, ADR-0044 |
| Scale/Scope | `chat_pins_max`, `chat_mentions_max`, `chat_reactions_per_user`, `chat_forward_targets_max` |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta: `toggleMessageReaction`, `CHAT_REACTION_LIMIT`, event chat `*`, unlock event game ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ module chat; quyền sở hữu sticker qua projection event module game, không gọi đồng bộ |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Forward chặn khác tenant; bỏ reaction/pin = soft-delete; thu hồi = soft (ADR-0028) |
| IV Server-authoritative | ✅ | Excerpt quote do server cắt; thời hạn sửa/thu hồi theo giờ server; forward có `Idempotency-Key`; edit có `If-Match` |
| V Xác thực, phân quyền | ✅ | `game_player` + membership + quyền pin theo loại channel; `game.chat_assets.write` platform-only |
| VI An toàn trẻ em | ✅ | R25 (chạy lại kiểm duyệt, giữ bằng chứng), R16 (nickname, không username), không upload của học sinh |
| VII Một nguồn sự thật, cấu hình | ✅ | Giới hạn theo CONFIG_DEFAULTS; giá trị chưa có tên đề xuất đăng ký |
| VIII UI tách logic, token | ✅ | Không có UI |
| IX Test có bằng chứng | ✅ | Test mọi F118-AC-*, tenant isolation + authorization mọi endpoint mới |

## Project Structure

### Documentation (feature này)

```text
specs/118-chat-messaging-features/
├── spec.md · plan.md · research.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
src/backend/modules/chat/
├── ent/schema/                         # messagerevision.go, messagereaction.go, pinnedmessage.go, stickerpack.go, sticker.go,
│                                       # gifasset.go, giftag.go, chatassetversion.go, refstickerunlock.go
├── internal/domain/message/            # edit.go, recall.go, quote.go, forward.go, reaction.go, pin.go, mention.go
├── internal/domain/cursor/             # MemberCursor (delivered/read, mentions), ReceiptView (per_member/count/none)
├── internal/domain/chatasset/          # StickerPack, GifAsset, file rules, review (người duyệt ≠ người upload)
├── internal/application/command/       # edit_message.go, recall_message.go, forward_message.go, toggle_reaction.go, pin_message.go,
│                                       # unpin_message.go, update_cursor.go, chat_asset_*.go
├── internal/application/query/         # get_message_receipts.go, list_channel_pins.go, list_sticker_packs.go, list_gifs.go, list_game_configs_chat.go
├── internal/infrastructure/valkey/     # typing.go, mentions_cache.go
├── internal/infrastructure/storage/    # presign.go (object storage), asset_inspector.go
├── internal/presentation/natscmd/      # message_edit.go, message_recall.go, message_forward.go, reaction_toggle.go, pin.go, typing.go, receipt.go
├── internal/presentation/http/public/  # messages_edit.go, pins.go, receipts.go, stickers.go, gifs.go, reactions.go
├── internal/presentation/http/gameadmin/ # chat_assets.go (*_chat operationIds)
├── internal/presentation/consumer/     # party_leader.go, game_unlocks.go
└── internal/presentation/worker/persister/ # apply_evt.go (edit/recall/reaction/pin theo revision)
```

**Structure Decision**: Giữ một aggregate `Message` cho mọi thao tác tin; sticker/GIF là bounded context con `chatasset` trong cùng module (API_DESIGN: `game-admin/{sticker-packs|gifs}` thuộc module chat).

## Phase 0: Research

→ [research.md](research.md): R1 thứ tự áp sự kiện thay đổi, R2 receipts không lưu theo tin × người, R3 version sticker/GIF.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `Message.Edit(now, window, body)`, `Recall(now, window)`, `QuoteExcerpt(display, start, end)`, `ForwardTo(targets)`, `Reactions.Toggle(limit)`, `Pins.Add(limit, role, channelType)`, `Mentions.Filter(members, blocks, max)`; `MemberCursor.Advance` | Thời gian từ `Clock` |
| application | Command `EditMessage`, `RecallMessage`, `ForwardMessage`, `ToggleReaction`, `PinMessage`, `UnpinMessage`, `UpdateCursor`, `UpsertStickerPack`, `UpsertGifAsset`, `TransitionChatAsset`, `PublishChatAsset`, `RollbackChatAsset`, `UnpublishChatAsset`; Query `GetMessageReceipts`, `ListChannelPins`, `ListStickerPacks`, `ListGifs`, `ListGameConfigs_chat`, `GetGameConfigVersion_chat`, `DiffGameConfigVersions_chat`, `ListGameConfigReleases_chat` | Học sinh: `game_player` + member; Game Studio: `game.chat_assets.write` |
| infrastructure | JetStream `evt` publisher; Valkey typing/cursor/mentions; presign + kiểm tệp; ent repo | |
| presentation | REST `editMessage`, `recallMessage`, `forwardMessage`, `pinMessage`, `unpinMessage`, `listChannelPins`, `getMessageReceipts`, `listStickerPacks`, `listGifs`, `toggleMessageReaction` (Mới), `*_chat` game-admin; NATS `ew.cmd.chat.message.edit/recall/forward`, `ew.cmd.chat.reaction.toggle`, `ew.cmd.chat.pin.add/remove`, `ew.cmd.chat.typing.start/stop`, `ew.cmd.chat.receipt.delivered`; consumer `party.created`/`party.leader_transferred` (role leader), unlock game | |

### Tích hợp và phụ thuộc

- F115 (send, persister, fan-out), F116 (`ContentValidator`), F117 (receipt read, `exclude_user_ids`, channel gameplay), F075 (event mở khóa), F045 (gán tenant).
- Event phát (outbox, không nội dung): `chat.message_edited`, `chat.message_recalled`, `chat.message_forwarded`, `chat.message_pinned`, `chat.message_unpinned`, `content.published` (`item_type = sticker_pack|gif`), `audit.recorded`.

### Bảo mật, tenant, idempotency, audit

- Forward: `Idempotency-Key`; mọi đích phải cùng `tenant_id` phiên. Edit: `If-Match`/`revision_expected`.
- Audit: pin/unpin, thu hồi, mọi thao tác Game Studio (không nội dung tin).
- `/game-admin/*`: gateway nội bộ, `azp = eduworld-portal` + role platform (R20).

### Quan sát

- Metric `eduworld_chat_events_total{event}` (edit/recall/forward/reaction/pin), `eduworld_chat_receipt_flush_total`, `eduworld_chat_typing_events_total`, `eduworld_chat_asset_rejected_total{reason}`.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
