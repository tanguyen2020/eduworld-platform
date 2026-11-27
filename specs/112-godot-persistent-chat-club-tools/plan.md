# Implementation Plan: F112 — Godot — channel chat bền vững (khu vực, DM) và công cụ chủ club (sửa club, phó nhóm, mục tiêu/sự kiện club)

**Branch**: `112-godot-persistent-chat-club-tools` · **Date**: 2026-11-28 · **Spec**: [spec.md](spec.md)

## Summary

Thêm khung chat HUD `ChatHub` (điền slot `ChatDock` có sẵn trong `MainHud.tscn`, comment cũ ghi F110) cho channel bền vững của F120: `ChatHubState` (thuần, `RefCounted`) dựng danh sách tab từ `listMyChannels` + club của em (`ClubService`, F111) + bạn bè (`SocialService`, F110), quyết định join/leave theo tab đang mở và đổi channel `area` khi nhận `hub.instance.assigned`. Phần hiển thị tin dùng nguyên thành phần F119 (`ChatMessageList`, bong bóng, `ChatComposer`, menu, báo cáo) qua chế độ mới `ChatDock.show_channel(channel_id)` (cũng là điểm vào cho nút "Chat club" của F111-AC-24). DM mở qua `ChatService.open_direct(user_id)` (`openDirectChannel`) với `DirectPeerMap` chỉ trong RAM. Tìm kiếm là `ChatSearchService` + `ChatSearchPanel` (văn bản thuần). Công cụ chủ club là `ClubManageService` (bọc `updateClub`, `updateClubMemberRole`, `createClubGoal`, `createClubEvent` với `If-Match`/`Idempotency-Key`) + màn `ClubManage` mở từ trang club của F111. Không đổi contract, không có code server; mọi thiếu hụt contract là câu hỏi mở (contracts §6).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.x, bản đang dùng của repo), static typing |
| Primary Dependencies | `ApiClient`, `RealtimeClient` (F061, F092), `ChatService`/`ChatStore`/`ChatDock` (F119), `ClubService` + trang club (F111), `SocialService.list_friends` + `FriendProfilePanel` (F110), `SceneRouter`/`AppState.blocking_activity`, `CompactPolicy` (F125), `UiBuild`, `UiTokens`, gettext, GdUnit4 |
| Storage | Không có (RAM); không lưu tin, ánh xạ bạn, truy vấn tìm kiếm xuống đĩa |
| Testing | GdUnit4 headless: unit (`ChatHubState`, `DirectPeerMap`, `ChatSearchService`, `ClubManageService`, `HighlightText`) với `FakeApiClient`/`FakeRealtimeClient` + fixture contract; scene test; ma trận khối `test_grade_band_matrix.gd` + `BandLayoutProbe`/`ChatSpaceProbe`; ảnh `tools/capture_screens.gd`; perf `tests/perf/` |
| Target Platform | Web, Android, iOS (ADR-0101) |
| Project Type | game client (`src/game/`) |
| Performance Goals | ≥ 30 FPS tầm trung preset Trung bình ([PERFORMANCE_BUDGET](../../docs/08-game-client/PERFORMANCE_BUDGET.md), NFR-COMP-001); TC-PERF-009 |
| Constraints | AGENTS R1, R7, R14, R15, R16, R17, R19, R25, R32; ADR-0020, ADR-0034, ADR-0047, ADR-0054, ADR-0056, ADR-0078, ADR-0095 |
| Scale/Scope | ≤ `chat_recent_buffer` tin/channel; ≤ 5 club (`clubs_per_student_max`) + DM ≤ số bạn (`friend_limits`) + 1 World + 1 Area |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Spec Ready; chỉ dùng contract có sẵn của F115–F121, F095; không delta contract trong F112 (thiếu hụt → câu hỏi mở cho feature backend). Không chạm nội dung deferred (ADR-0082) |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Client chỉ gọi REST/WS public; không facade, không DB. Tầng client: state/service thuần, scene chỉ bind |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `X-Tenant-Id` từ phiên qua `ApiClient`; dữ liệu chỉ RAM, xóa khi đăng xuất/`session.session.replaced`; server lọc tenant/membership |
| IV Server-authoritative | ✅ | Quyền gửi, `sendable_kinds`, `features`, vai trò club, tiến độ mục tiêu đều từ server; "đã gửi" chỉ theo `chat.message.acked`; ghi club có `Idempotency-Key`/`If-Match`, audit ở backend |
| V Xác thực, phân quyền | ✅ | Token game F061; nút theo vai trò/cờ chỉ là hiển thị, server kiểm (`game_player`, owner/officer); `403` → ẩn công cụ |
| VI An toàn trẻ em | ✅ | R15: không nhánh theo khối, mọi tin qua pipeline server (client không có đường gửi khác); R16: chỉ nickname/avatar, câu từ chối trung tính, không lộ ai chặn; R19: không đếm ngược/nhấp nháy/thông báo ngoài game; R25: không NATS/Valkey; văn bản thuần (không BBCode); không công cụ gỡ thành viên giữa học sinh |
| VII Một nguồn sự thật, cấu hình | ✅ | Giới hạn (`max_text_length`, giới hạn `q`, độ dài tên/mô tả) lấy từ contract/server; tham số theo tên CONFIG_DEFAULTS; danh sách `metric`/`event_kind` cho phép nằm trong một file dữ liệu client có câu hỏi mở F112-3/F112-4 |
| VIII UI tách logic, token | ✅ | Scene chỉ đọc state, phát signal ý định; Theme/`UiTokens` sinh từ `design/game-tokens/`, phong cách tầng F114, `compact` F125; không token mới (nếu cần thì sửa `design/` + `make design-tokens`) |
| IX Test có bằng chứng | ✅ | Mỗi F112-AC có test (bảng dưới), test viết trước và FAIL trước khi code; fake/fixture gắn nhãn TEST DOUBLE; ảnh ma trận khối ở quickstart §5 |

### AC → test

| AC | Test |
|---|---|
| F112-AC-01, AC-04, AC-05 | `tests/unit/chat/test_chat_hub_state.gd`, `tests/scene/chat/test_chat_hub.gd` |
| F112-AC-02 | `tests/scene/chat/test_chat_hub.gd::test_persistent_tab_reuses_dock_and_reports_with_channel_id` |
| F112-AC-03 | `tests/unit/chat/test_chat_hub_state.gd::test_area_switches_on_instance_assigned` |
| F112-AC-06 | `tests/scene/chat/test_chat_hub.gd::test_hub_hidden_during_exam` |
| F112-AC-07…AC-10 | `tests/unit/chat/test_direct_channels.gd`, `tests/scene/chat/test_chat_dm.gd` |
| F112-AC-11…AC-14 | `tests/scene/chat/test_persistent_channel_states.gd` |
| F112-AC-15…AC-17 | `tests/unit/chat/test_chat_search_service.gd`, `tests/unit/chat/test_highlight_text.gd`, `tests/scene/chat/test_chat_search_panel.gd` |
| F112-AC-18…AC-21 | `tests/unit/social/test_club_manage_service.gd`, `tests/scene/social/test_club_manage.gd` |
| F112-AC-22…AC-24 | `tests/unit/social/test_club_manage_service.gd`, `tests/scene/social/test_club_goal_event_forms.gd` |
| F112-AC-25 | `tests/scene/test_pseudo_locale.gd` (màn mới), `tools/extract_pot.gd` |
| F112-AC-26 | `tools/lint_theme_overrides.gd`, `tests/scene/chat/test_chat_hub.gd::test_window_close_and_tiers` |
| F112-AC-27, AC-28 | `tests/scene/test_grade_band_matrix.gd` (màn mới + `ChatSpaceProbe`), `tools/capture_screens.gd`, quickstart §5 |
| F112-AC-29 | `tests/perf/test_chat_hub_perf.gd` + đo thiết bị lab |
| F112-AC-30 | `tests/unit/chat/test_chat_boundaries.gd` (mở rộng: không `grade_band` trong logic chat, không BBCode, không NATS/Valkey) |
| F112-AC-31 | `tests/unit/chat/test_f112_privacy_authz.gd` |

## Project Structure

### Documentation (feature này)

```text
specs/112-godot-persistent-chat-club-tools/
├── spec.md · plan.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
├── checklists/requirements.md
└── screenshots/                     # ảnh ma trận khối (T040)
```

### Source Code (repository)

```text
src/game/
├── scenes/chat/
│   ├── ChatHub.tscn + chat_hub.gd           # khung chat HUD: tab channel bền vững, badge, kính lúp, ✕ Đóng
│   ├── chat_hub_tabs.gd                     # hàng tab cuộn ngang (ChipScroller), badge số / @
│   ├── chat_friends_list.gd                 # tab "Bạn bè": bạn + badge DM, nút "Nhắn tin"
│   ├── chat_search_panel.gd                 # bảng tìm kiếm (ô q, phạm vi, kết quả ảo hóa)
│   └── chat_dock.gd                         # SỬA: thêm show_channel(channel_id) cho channel bền vững + khóa theo trạng thái
├── scripts/chat/
│   ├── chat_hub_state.gd                    # thuần: tab từ listMyChannels + club + bạn; join/leave; area theo instance
│   ├── direct_peer_map.gd                   # thuần: channel_id ↔ bạn (RAM)
│   ├── chat_search_service.gd               # searchMessages, cursor
│   ├── highlight_text.gd                    # thuần: bỏ đánh dấu, tô từ khóa không dấu
│   └── chat_service.gd                      # SỬA: open_direct(user_id) (openDirectChannel), refresh khi hub.instance.assigned
├── scripts/social/
│   ├── club_manage_service.gd               # updateClub (If-Match), updateClubMemberRole, createClubGoal/Event (Idempotency-Key)
│   └── club_form_options.gd                 # dữ liệu: metric/event_kind cho phép (câu hỏi mở F112-3, F112-4)
├── scenes/social/
│   ├── ClubManage.tscn + club_manage.gd     # màn Quản lý club (4 mục), mở từ trang club F111
│   ├── club_edit_form.gd · club_officers.gd · club_goal_form.gd · club_event_form.gd
├── scenes/ui/hud/main_hud.gd                # SỬA: slot ChatDock → ChatHub (bỏ khỏi FUTURE_SLOTS), ẩn khi blocking_activity
├── locale/vi.po, en.po                      # ui.chat.hub.*, ui.chat.dm.*, ui.chat.search.*, ui.club.manage.*
└── tests/{unit/chat, unit/social, scene/chat, scene/social, perf, fakes, fixtures/contract}/
```

**Structure Decision**: Theo GODOT_ARCHITECTURE §3 và cách F119/F102 dựng thành phần bằng code (`UiBuild`, biến thể Theme) — chỉ `ChatHub.tscn`, `ClubManage.tscn` là scene. `ChatHub` không nhân bản danh sách tin/ô soạn: mỗi tab gắn một `ChatDock` ở chế độ `show_channel`, nên mọi hành vi tin (F119-AC-08…AC-20) và quy tắc "≥ 3 tin" có sẵn. Thứ tự phụ thuộc: F110 (danh sách bạn, `FriendProfilePanel`) → F111 (`ClubService`, trang club) → F112.

## Phase 0: Research

Không cần `research.md` — bố cục, thành phần và quy tắc đã có ở STUDENT_GAME_UX_SOCIAL §6–§7, CHAT_SYSTEM §2–§9, SOCIAL_SYSTEM §6; các quyết định còn mở đã chốt ở spec Clarifications 2026-11-28.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain (client) | `ChatHubState`, `DirectPeerMap`, `HighlightText`, `ClubFormOptions` | Thuần, không Node, test được |
| application (client) | `ChatService` (mở rộng: `open_direct`, làm mới theo instance, `show_channel`), `ChatSearchService`, `ClubManageService`; dùng lại `ClubService` (F111), `SocialService` (F110), `ReportService` (F119) | Signal thì quá khứ: `hub_tabs_changed`, `direct_opened`, `search_loaded`, `club_updated`, `member_role_changed`, `goal_created`, `event_created` |
| infrastructure | `ApiClient` (REST, `Idempotency-Key`, `If-Match`), `RealtimeClient` (WS) | F061, F092 |
| presentation | `scenes/chat/ChatHub*`, `scenes/social/ClubManage*`; `MainHud` slot; nút "Nhắn tin" trong `FriendProfilePanel` (F110); nút "Quản lý club" trong trang club (F111) | Không gọi API từ scene |

### Tích hợp và phụ thuộc

- F119: `ChatDock` thêm `show_channel(channel_id)`; giữ nguyên `show_party`/`show_match`. F111: trang club gọi `ChatDock.show_channel(chat_channel_id)` (F111-AC-24) và thêm nút "Quản lý club"; `ClubService` cung cấp club của em cho tab. F110: `FriendProfilePanel` thêm nút "Nhắn tin" (F110 Clarifications để "Nhắn tin" ngoài phạm vi). F091/F092: `hub.instance.assigned` qua `HubSync`/`EventBus`. F064: `AppState.blocking_activity`. F123: deep link tới channel/club dùng `ChatHub.open_channel`.
- Không cần thay đổi backend. Câu hỏi mở F112-1…F112-9 (contracts §6) có đường dự phòng trong spec.

### Bảo mật, tenant, idempotency, audit

- `Idempotency-Key` UUID mới cho mỗi `createClubGoal`/`createClubEvent`, giữ khi thử lại (`PendingWrites` của F061); `updateClub` gửi `If-Match` = `version` đọc gần nhất; `updateClubMemberRole` đặt giá trị (lặp an toàn). Audit ở module chat.
- Logger client lọc nội dung tin, `q`, đoạn trích, tên/mô tả club (R14); telemetry chỉ số đếm (số lần tìm, độ trễ mở DM) không có nội dung.
- Mọi chuỗi từ người dùng/server hiển thị bằng `Label`/`RichTextLabel` tắt BBCode (văn bản thuần).

### Quan sát

- Telemetry client (cơ chế chung F080): số tab mở, thời gian từ bấm "Nhắn tin" tới channel sẵn sàng, số lỗi `CHAT_NOT_ALLOWED`/`412`/`451` theo mã — không PII, không nội dung.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
