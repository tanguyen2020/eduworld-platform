# Implementation Plan: F119 — Giao diện chat trong Godot: panel, quick message, tab, tắt tiếng, báo cáo

**Branch**: `119-godot-chat-ui` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Dựng tầng chat của game client theo GODOT_ARCHITECTURE §6.1: `ChatService` (`RefCounted`, tạo bởi `ServiceLocator`, nhận `ApiClient` + `RealtimeClient`) gửi lệnh WS/REST và chuyển phản hồi thành signal; `ChatStore` (`RefCounted`, thuần, test được) giữ tin theo `seq`, tin chờ, con trỏ, unread, phát hiện gap; scene `ChatPanel` và thành phần con chỉ đọc state và phát signal ý định (R1). Panel nhúng vào party lobby (F102), arena/battle (F109), boss (F103) qua một `ChatDock` chung với chế độ `expanded/collapsed/mini`. Menu Tắt tiếng/Chặn/Báo cáo gọi `SocialService` (`blockUser`, `muteUser`) và `ReportService` (`createReport`). Mọi chuỗi qua `ui.chat.*`, mọi style từ Theme sinh từ token.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.x), static typing |
| Primary Dependencies | `ApiClient`, `RealtimeClient` (F061, F092), `EventBus`, `ServiceLocator`, `UiTokens`, gettext `TranslationServer`, GdUnit4 |
| Storage | Không có (RAM); cache asset sticker/GIF trong `user://cache/chat_assets/` theo version |
| Testing | GdUnit4 headless (unit `ChatStore`/`ChatService` với `FakeApiClient`/`FakeRealtimeClient`, scene runner), visual pseudo-locale, profiler thiết bị lab |
| Target Platform | Windows, Android, iOS, Web |
| Project Type | game client |
| Performance Goals | [PERFORMANCE_BUDGET](../../docs/08-game-client/PERFORMANCE_BUDGET.md); TC-PERF-009, TC-PERF-019 |
| Constraints | AGENTS R1, R2, R3, R15, R17, R25, R32; ADR-0020, ADR-0034, ADR-0047, ADR-0056 |
| Scale/Scope | ≤ `chat_recent_buffer` tin/channel trong RAM; ≤ 3 ảnh động phát cùng lúc |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Dùng contract F115–F118, F095, F121; delta chỉ phụ thuộc các feature đó |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Client chỉ gọi REST/WS công khai; không có logic nghiệp vụ server ở client |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `X-Tenant-Id` lấy từ phiên; không lưu chat xuống đĩa |
| IV Server-authoritative | ✅ | Trạng thái "đã gửi" chỉ theo `chat.message.acked`; kiểm duyệt ở server |
| V Xác thực, phân quyền | ✅ | Token game do F061 quản lý (access token RAM, refresh token `SecureStorage` — ADR-0086); menu theo `Channel.features` chỉ là hiển thị, server vẫn kiểm |
| VI An toàn trẻ em | ✅ | Không lộ từ khớp; Tắt tiếng/Chặn/Báo cáo ≤ 2 thao tác; không tắt được chat gameplay; không URL GIF ngoài |
| VII Một nguồn sự thật, cấu hình | ✅ | N ngày lưu, giới hạn, quick message lấy từ server; tham số client theo CONFIG_DEFAULTS |
| VIII UI tách logic, token | ✅ | Scene chỉ bind state; Theme/`UiTokens` sinh từ `design/game-tokens/` |
| IX Test có bằng chứng | ✅ | Test GdUnit4 mọi F119-AC-*; mock Prism gắn nhãn, `Done` chỉ với backend thật |

## Project Structure

### Documentation (feature này)

```text
specs/119-godot-chat-ui/
├── spec.md · plan.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
src/game/
├── scenes/chat/        ChatDock.tscn, ChatPanel.tscn, ChannelTabs.tscn, MessageList.tscn, MessageBubble.tscn,
│                       MessageActionMenu.tscn, Composer.tscn, QuickMessageBar.tscn, EmojiStickerGifPicker.tscn,
│                       MentionAutocomplete.tscn, PinnedBar.tscn, TypingIndicator.tscn, ReceiptBadge.tscn,
│                       ReportDialog.tscn, ModerationFeedbackToast.tscn, RetentionNotice.tscn
├── scripts/chat/       chat_service.gd, chat_store.gd, chat_asset_cache.gd, quick_message_catalog.gd,
│                       report_service.gd, chat_feedback_mapper.gd (mã lỗi/decision → khóa ui.chat.feedback.*),
│                       models/{chat_message.gd, channel.gd, receipt_state.gd}
├── scripts/social/     social_service.gd (blockUser, muteUser — dùng chung F095 UI)
├── locale/             vi.po, en.po (ui.chat.*)
└── tests/{unit/chat, scene/chat, fixtures/chat}/
```

**Structure Decision**: Theo cây thư mục GODOT_ARCHITECTURE §3; `ChatDock` là điểm nhúng duy nhất để F102/F103/F109 gắn panel mà không biết chi tiết chat.

## Phase 0: Research

Không có — GODOT_ARCHITECTURE §6.1 và STUDENT_GAME_UX_SOCIAL §7 đã mô tả thành phần và bố cục.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain (client) | `ChatStore` (insert theo `seq`, pending → sent → delivered/read, gap detect, unread), `ChatFeedbackMapper` | Thuần, không Node |
| application (client) | `ChatService` (send/edit/recall/forward/reaction/pin/typing/receipt/sync/join, retry `client_message_id`), `ReportService`, `SocialService`, `QuickMessageCatalog`, `ChatAssetCache` | Signal thì quá khứ: `message_acked`, `message_received`, `gap_detected`, `send_failed` |
| infrastructure | `ApiClient` (REST, `Idempotency-Key`), `RealtimeClient` (WS) | F061, F092 |
| presentation | Scene `scenes/chat/*`; `ChatDock` nhúng vào `PartyLobby.tscn`, `BattleArena.tscn`, màn kết quả trận | Không gọi API trực tiếp từ scene |

### Tích hợp và phụ thuộc

- F092 `RealtimeClient`; F102/F109/F103 khung màn; F115–F118 contract chat; F095 `blockUser`/`muteUser`; F121 `createReport` (trước đó Prism mock có nhãn).
- `EventBus.question_shown/question_hidden` (từ F063/F109) để thu nhỏ panel.

### Bảo mật, tenant, idempotency, audit

- `Idempotency-Key` mới cho mỗi báo cáo/forward, giữ khi retry; `client_message_id` UUID cho mỗi tin.
- Không log nội dung chat (logger client lọc `body`).

### Quan sát

- Telemetry client (không nội dung): số gap, số lần gửi lại, thời gian từ gửi tới `acked` — gửi theo cơ chế telemetry chung (F080).

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
