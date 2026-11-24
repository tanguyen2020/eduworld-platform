# Implementation Plan: F102 — Giao diện party trong Godot: panel, lời mời, lobby, ready

**Branch**: `102-godot-party-ui` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Thêm domain party vào game client theo GODOT_ARCHITECTURE §4, §6: `PartyService` (`RefCounted`, tạo bởi `ServiceLocator`, nhận `ApiClient`/`RealtimeClient`) gói mọi REST (F100/F101) và lệnh WS party; `PartyStore` giữ trạng thái theo `state_version`, nhận message `party.*` từ `RealtimeClient` (F092), phát signal typed; `AppState.party` trỏ snapshot hiện hành. UI chỉ đọc store: `PartyLobby` (PC hai cột, mobile thẻ ngang), `PartyMemberCard`, `InviteFriendPicker`, `ActivitySelector`, `InvitationToast`, `PartyHud`. Chuyển cảnh phiên chơi chung qua `SceneRouter`. Phát triển trên Prism mock (nhãn MOCK) tới khi F100/F101 Done.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.x ghim trong `src/game/.godot-version`), static typing bắt buộc (ADR-0014) |
| Primary Dependencies | GdUnit4; autoload `Config`, `Log`, `EventBus`, `AppState`, `ApiClient`, `RealtimeClient`, `SceneRouter`, `Settings`, `QualityManager` (F060–F062, F092) |
| Storage | Không có dữ liệu bền; không ghi party xuống `user://` |
| Testing | GdUnit4 unit (`FakeApiClient`, `FakeRealtimeClient`) + scene runner headless; integration headless với backend docker compose |
| Target Platform | Windows 10+, Android 9+ (Release 1.0 cũng iOS, Web — ADR-0013) |
| Project Type | game client |
| Performance Goals | Lobby ≥ 30 FPS tầm trung, cập nhật ≤ 1 s ([PERFORMANCE_BUDGET](../../docs/08-game-client/PERFORMANCE_BUDGET.md), AC-UX-104 của STUDENT_GAME_UX_SOCIAL §12) |
| Constraints | AGENTS R1, R2, R3, R16, R17, R32 · ADR-0047, ADR-0056, ADR-0064, ADR-0065 |
| Scale/Scope | Party ≤ 8 thẻ; 1 lobby + HUD mini + toast lời mời |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Chỉ dùng contract của F100/F101 (delta đã nằm ở Phase 2 của F101); fixture sinh từ `specs/` |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Client chỉ gọi API public/WS qua `ApiClient`/`RealtimeClient`; tách Service → Store → UI |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `X-Tenant-Id` của phiên do `ApiClient` gắn; không lưu party xuống đĩa; xóa store khi đăng xuất/takeover |
| IV Server-authoritative | ✅ | Ready/leader/điều kiện hiển thị theo server xác nhận; nút leader chỉ là tiện ích, server kiểm |
| V Xác thực, phân quyền | ✅ | Token RAM của F061; lỗi `GAME_ACCESS_STUDENT_ONLY`/`TENANT_MISMATCH` về màn đăng nhập (F061) |
| VI An toàn trẻ em | ✅ | Chỉ nickname/avatar/level; mời bằng avatar cho lớp 1–3; không popup che màn học tập |
| VII Một nguồn sự thật, cấu hình | ✅ | Thời hạn/grace lấy từ server; chuỗi qua `.po` |
| VIII UI tách logic, token | ✅ | UI chỉ gọi `PartyService`; Theme sinh từ `design/game-tokens`, `lint_theme_overrides.gd` |
| IX Test có bằng chứng | ✅ | Mỗi F102-AC có task test; mock gắn nhãn, không vào build phát hành |

## Project Structure

### Documentation (feature này)

```text
specs/102-godot-party-ui/
├── spec.md · plan.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
src/game/
├── scenes/
│   ├── social/PartyLobby.tscn             # + party_lobby.gd (bố cục PC/mobile theo kích thước màn)
│   ├── social/PartyMemberCard.tscn        # + party_member_card.gd
│   ├── social/InviteFriendPicker.tscn     # + invite_friend_picker.gd
│   ├── social/ActivitySelector.tscn       # + activity_selector.gd (thẻ lớn g1_3 / dropdown g6_9)
│   ├── social/InvitationToast.tscn        # + invitation_toast.gd
│   └── ui/hud/PartyHud.tscn               # + party_hud.gd (PartyChip + khung thành viên)
├── scripts/
│   ├── multiplayer/party_service.gd       # RefCounted: create/invite/accept/reject/leave/kick/transfer/disband/ready/select/start
│   ├── multiplayer/party_store.gd         # RefCounted: apply(message), state_version, signals
│   ├── multiplayer/models/{party_model.gd, party_member_model.gd, invitation_model.gd, eligible_activity_model.gd}
│   └── core/event_bus.gd                  # thêm signal party_invitation_received(invitation: InvitationModel)
├── locale/{vi.po,en.po}                   # ui.party.*, ui.party.error.*
└── tests/
    ├── unit/{test_party_service.gd, test_party_store.gd, test_party_models.gd}
    ├── scene/{test_party_lobby.gd, test_invitation_toast.gd, test_party_hud.gd, test_activity_selector.gd}
    ├── integration/test_party_flow_headless.gd
    └── fixtures/multiplayer/{party_*.json, invitation_*.json, ws_party_*.json, eligible_activities_*.json}
```

**Structure Decision**: theo GODOT_ARCHITECTURE §3 (tên scene `PascalCase`, script `snake_case` trong `scripts/multiplayer`); `party_store.gd` tách khỏi service để test áp message độc lập (như `ChatStore` §6.1).

## Phase 0: Research

Không có — luồng, message và bố cục đã có ở PARTY_SYSTEM, STUDENT_GAME_UX_SOCIAL §5, AsyncAPI; quyết định đặt tên ghi ở Clarifications.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain (client) | `PartyModel`, `PartyMemberModel`, `InvitationModel`, `EligibleActivityModel` (`from_dict` bỏ trường lạ) | Không logic nghiệp vụ |
| application (client) | `PartyService` (intent → REST/WS, `Idempotency-Key` theo intent, map Problem → khóa dịch), `PartyStore` (`apply_message`, `apply_snapshot`, phát hiện lệch `state_version`) | Signal: `party_updated(party: PartyModel)`, `party_closed(reason: StringName)`, `party_session_started(info: Dictionary)`, `party_session_ended(summary: Dictionary)` |
| presentation (UI) | `PartyLobby`, `PartyMemberCard`, `InviteFriendPicker`, `ActivitySelector`, `InvitationToast`, `PartyHud` | Chỉ đọc store, gọi method public |
| tích hợp | `RealtimeClient.message_received` lọc `type` bắt đầu `party.`; `SceneRouter.go_to(scene_ref, {room_id})`; `EventBus.network_state_changed` | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F092 (`RealtimeClient`, Main Hub nhiều người, `PartyChip` slot), F100/F101 (API party), F095 (`listFriends`), F061 (`ApiClient`, token), F060 (Theme, i18n, Prism mock).
- Không gọi backend nào ngoài API public; không kết nối NATS/Valkey (R25).

### Bảo mật, tenant, idempotency, audit

- `Idempotency-Key` sinh mới mỗi ý định (`createParty`, `inviteToParty`, `acceptPartyInvitation`, `joinParty`, `transferPartyLeader`, `startPartySession`), tái dùng khi thử lại cùng ý định.
- Log client không chứa nickname/ID học sinh khác ở mức INFO (R14).

### Quan sát

- Telemetry client (F080 nếu có): `party_lobby_open`, `party_start_tap`, thời gian từ `party.session.started` đến tải xong cảnh; không gửi PII.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
