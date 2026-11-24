# Implementation Plan: F111 — Godot — club trong game, Event center, nhóm "Vừa chơi cùng", nút "Tham gia bạn"

**Branch**: `111-godot-clubs-events-social` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Mở rộng tầng xã hội và thêm tầng live-ops của game client theo [GODOT_ARCHITECTURE §3, §4, §6](../../docs/08-game-client/GODOT_ARCHITECTURE.md): (1) `JoinFriendService` điều phối "Tham gia bạn" qua `SceneRouter.request_area` (F071) và `HubSync.join(area_id, spawn_point, follow_user_id)` (F092) — không REST; (2) `SocialService` (F110) thêm `list_suggestions(source)` cho nhóm "Vừa chơi cùng" trong `FindFriendsTab` (không có nhóm "Bạn cùng lớp" — ADR-0095); (3) `ClubService` + `ClubStore` (REST `/social/clubs*`, `/social/club-*`) với tab Club, `ClubPage`, `ClubCreateDialog`, tab Yêu cầu cho owner/officer; (4) `LiveEventService` + `LiveEventStore` (REST `/game/events*`, `/game/community-goals/*`, message `event.*`) và `SeasonTrackService` (REST `/game/season-track*`, message `season.track.updated`) với `EventCenterScreen` 4 tab, tab Hôm nay/Tuần này đọc `QuestService` (F072). Service là `RefCounted` do `ServiceLocator` tạo, nhận `ApiClient`/`RealtimeClient` qua constructor; UI chỉ đọc store. Phát triển trên Prism mock (nhãn MOCK) tới khi F077/F095/F120 `Done`.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.x ghim trong `src/game/.godot-version`), static typing bắt buộc |
| Primary Dependencies | GdUnit4; autoload `ApiClient`, `RealtimeClient`, `SceneRouter`, `AppState`, `EventBus`, `QualityManager`, `ContentCache`; F092 `HubSync`; F071 `SceneRouter.request_area`; F072 `QuestService`/`QuestTracker`; F110 `SocialService`/`FriendsStore`/`FriendsScreen`/`FindFriendsTab`; F119 `ChatDock`; F123 `NotificationService` (deep link `EventBus.ui_requested`); F069 hiển thị phần thưởng |
| Storage | Không có dữ liệu bền; store trong RAM (ADR-0064) |
| Testing | GdUnit4 unit (`FakeApiClient`, `FakeRealtimeClient`, `FakeClock`) + scene runner; integration headless 2 client với docker compose; benchmark `tests/perf/EventCenterBenchmark.tscn` |
| Target Platform | Android 9+, iOS, Web (ADR-0101) |
| Project Type | game client |
| Performance Goals | ≥ 30 FPS màn phụ trên máy tầm trung ([PERFORMANCE_BUDGET](../../docs/08-game-client/PERFORMANCE_BUDGET.md), NFR-COMP-001); cập nhật sự kiện theo nhịp server |
| Constraints | AGENTS R1, R5, R15, R16, R17, R19, R23, R25, R32 · ADR-0016, ADR-0020, ADR-0047, ADR-0049, ADR-0050, ADR-0054, ADR-0056, ADR-0057, ADR-0064, ADR-0065, ADR-0084 |
| Scale/Scope | ≤ `clubs_per_student_max` club của em; thành viên ≤ `club_max_members`; ≤ `season_track_tiers` cấp; gợi ý phân trang cursor |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Chỉ tiêu thụ contract; delta (`listFriendSuggestions`, `listMyClubInvitations`, `policies.social`, `follow_user_id`, kind thông báo club) thuộc owner backend, theo dõi ở Phase 2 (T004–T007) |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Client chỉ gọi API public/WS; Service → Store → UI; không NATS/Valkey (R25) |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `X-Tenant-Id` do `ApiClient`; không ghi `user://`; xóa store khi đăng xuất/takeover; `404` tenant khác → đóng màn |
| IV Server-authoritative | ✅ | Không gửi điểm sự kiện/mùa/đóng góp; chỉ đánh dấu đã nhận khi server trả; lệnh ghi REST có `Idempotency-Key` giữ khi retry; audit do backend |
| V Xác thực, phân quyền | ✅ | Ẩn/hiện theo `my_role`/policy chỉ là tiện ích; quyết định ở F120/F077/F091; `GAME_ACCESS_STUDENT_ONLY` → luồng F061 |
| VI An toàn trẻ em | ✅ | Không tìm kiếm tự do; chỉ nickname/avatar; không lộ vị trí/lý do khi từ chối tham gia; không xếp hạng cá nhân; không FOMO (R19); văn bản club qua pipeline server, không echo từ khớp (R15) |
| VII Một nguồn sự thật, cấu hình | ✅ | Giới hạn, trần, hạn nhận thưởng lấy từ response; tham số theo tên CONFIG_DEFAULTS; chuỗi qua `.po`; độ dài tên/mô tả club theo `ClubCreate` |
| VIII UI tách logic, token | ✅ | UI chỉ gọi method public; Theme sinh từ `design/game-tokens`; `lint_theme_overrides.gd` |
| IX Test có bằng chứng | ✅ | Mỗi F111-AC có task test viết trước; mock gắn nhãn, không vào build phát hành |

## Project Structure

### Documentation (feature này)

```text
specs/111-godot-clubs-events-social/
├── spec.md · plan.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
src/game/
├── scenes/
│   ├── social/{FindFriendsTab.tscn (sửa: nhóm gợi ý), SuggestionGroup.tscn, JoinFriendButton.tscn,
│   │           ClubTab.tscn, ClubCard.tscn, ClubPage.tscn, ClubGoalProgress.tscn, ClubEventList.tscn,
│   │           ClubMemberList.tscn, ClubCreateDialog.tscn, ClubRequestsTab.tscn, ClubInvitePicker.tscn}   # + *.gd
│   ├── liveops/{EventCenterScreen.tscn, TodayTab.tscn, WeekTab.tscn, EventsTab.tscn, EventCard.tscn,
│   │            EventDetailPanel.tscn, MilestoneList.tscn, CommunityGoalBar.tscn, SeasonTab.tscn,
│   │            SeasonTrack.tscn, SeasonTierItem.tscn, SeasonHistoryList.tscn}                          # + *.gd
│   ├── world/interactables/EventBoard.tscn       # bảng tin Quảng trường Tia Sáng → mở Event center
│   └── ui/hud/EventCenterButton.tscn             # biểu tượng lịch, chấm tĩnh khi có nội dung mới
├── scripts/
│   ├── social/{social_service.gd (thêm list_suggestions), friends_store.gd (thêm area_id, activity),
│   │           join_availability.gd, club_service.gd, club_store.gd,
│   │           models/{friend_suggestion_model.gd, club_model.gd, club_member_model.gd, club_goal_model.gd,
│   │                   club_event_model.gd, club_invitation_model.gd, club_join_request_model.gd, social_policy_model.gd}}
│   ├── multiplayer/{join_friend_service.gd, hub_sync.gd (sửa: follow_user_id)}
│   ├── liveops/{live_event_service.gd, live_event_store.gd, season_track_service.gd, season_track_store.gd,
│   │            models/{live_event_model.gd, event_milestone_model.gd, community_goal_model.gd,
│   │                    season_track_model.gd, season_tier_model.gd}}
│   └── core/{app_state.gd (thêm social_policy, live_events, clubs), event_bus.gd, service_locator.gd,
│             scene_router.gd (sửa: request_area(area_id, args) chuyển follow_user_id cho HubSync)}
├── locale/{vi.po,en.po}                           # ui.events.*, ui.season.*, ui.club.*, ui.friends.join_*, ui.error.<CODE>
└── tests/
    ├── unit/{social,liveops,multiplayer}/test_*.gd
    ├── scene/{test_find_friends_tab.gd, test_club_tab.gd, test_club_page.gd, test_club_create_dialog.gd,
    │          test_event_center.gd, test_event_detail.gd, test_season_track.gd}
    ├── perf/EventCenterBenchmark.tscn
    ├── integration/{test_join_friend_headless.gd, test_club_headless.gd, test_event_center_headless.gd}
    └── fixtures/{club,liveops,social_suggestions}/*.json   # sinh từ example OpenAPI/AsyncAPI, nhãn MOCK
```

**Structure Decision**: Theo [GODOT_ARCHITECTURE §3](../../docs/08-game-client/GODOT_ARCHITECTURE.md): club là phần của màn Bạn bè (`scenes/social/`, `scripts/social/` của F110); thêm thư mục `scenes/liveops/`, `scripts/liveops/` (tránh nhầm "event" với `EventBus`) — cập nhật §3 ở task Polish. `HubSync` (F092) và `SceneRouter.request_area` (F071) chỉ thêm tham số `follow_user_id`, không đổi hành vi cũ.

## Phase 0: Research

Không có — bố cục ở STUDENT_GAME_UX_SOCIAL §6, §9; luật ở SOCIAL_SYSTEM, EVENT_SEASON_SYSTEM; cách tham gia bạn đã chốt ở F091 (Clarifications spec.md).

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain (client) | Model typed `from_dict` (bỏ trường lạ) — xem data-model; `JoinAvailability.evaluate(friend, self_state) -> StringName` (thuần, chỉ quyết định hiển thị nút + khóa lý do) | Không luật nghiệp vụ server |
| application (client) | `JoinFriendService` (`join(friend_id)`, signal `join_succeeded(nickname)`, `join_failed(code)`) · `SocialService.list_suggestions(source, cursor)` · `ClubService` (`list_mine`, `list_discoverable`, `list_invitations`, `get_club`, `load_page(club_id)`, `request_join`, `accept_invitation`, `decline_invitation`, `create(form)`, `leave`, `list_requests`, `accept_request`, `reject_request`, `invite(ids)`) · `LiveEventService` (`list`, `get_detail`, `join`, `claim_milestone`, `get_goal`) · `SeasonTrackService` (`load`, `claim_tier`, `history`) | Signal thì quá khứ: `clubs_changed()`, `club_page_loaded(c: ClubModel)`, `events_changed()`, `event_detail_loaded(e: LiveEventModel)`, `community_goal_updated(g: CommunityGoalModel)`, `season_changed()`, `reward_claimed(rewards: Array)`, `request_failed(code: StringName)` |
| presentation (UI) | `FindFriendsTab`/`SuggestionGroup`, `JoinFriendButton` (gắn vào `FriendProfilePanel`/`FriendRow` F110), `ClubTab`, `ClubPage`, `ClubCreateDialog`, `ClubRequestsTab`, `ClubInvitePicker`, `EventCenterScreen` và tab, `EventCenterButton`, `EventBoard` | Chỉ đọc store, gọi method public |
| tích hợp | `RealtimeClient` dispatcher: `event.status.changed`, `event.progress.updated`, `event.community_goal.updated` → `LiveEventService`; `season.track.updated` → `SeasonTrackService`; `hub.instance.assigned`/`system.command.failed` của lệnh join → `JoinFriendService` (qua `HubSync`); `presence.friend.updated` → `FriendsStore` (F110, thêm `area_id`, `activity`). `EventBus.ui_requested("event_center", {event_id?})`, `("club", {club_id})`, `("friends", {tab})` đăng ký handler cho deep link F123. `AppState.apply_bootstrap` đọc `policies.social` | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F077 (API/WS sự kiện, mùa), F120 (club), F095 (gợi ý kết bạn — delta), F091/F092 (`follow_user_id`, `HubSync`), F071 (`enterWorldArea`, `SceneRouter.request_area`, `AREA_LOCKED`), F072 (`QuestService` daily/weekly, dẫn đường), F110 (`SocialService`, `FriendsScreen`), F119 (`ChatDock`), F123 (deep link), F069 (hiển thị phần thưởng), F013/F015 (`policies.social` — delta), F061/F060 (ApiClient, Clock, Theme, i18n, Prism mock).
- Không gọi backend nào ngoài API public; không kết nối NATS/Valkey (R25).

### Bảo mật, tenant, idempotency, audit

- `Idempotency-Key` UUIDv7 mỗi ý định, giữ khi retry: `joinGameEvent`, `claimEventMilestone`, `claimSeasonTier`, `createClub`, `requestJoinClub`, `acceptClubInvitation`, `acceptClubJoinRequest`, `inviteToClub`, `sendFriendRequest` (F110). Lệnh WS `hub.instance.join` dùng `id` UUIDv7 của envelope; nút khóa khi chờ ack.
- Form club: kiểm độ dài theo `ClubCreate` (`name` 2–64, `description` ≤ 500) trước khi gửi; server vẫn kiểm và chạy pipeline kiểm duyệt (F120). Không log tên/mô tả club, nickname người khác ở mức INFO (R14).
- Phản hồi lỗi tham gia không hiển thị `room_id`, khu hay lý do chặn (R16).

### Quan sát

- Telemetry client (F080 khi có, không PII): `join_friend` (kết quả `ok`/`room_full`/`denied`/`area_locked`, thời gian), `event_center_open`, `event_join`, `milestone_claim`, `season_tier_claim`, `club_create` (kết quả), `club_join_request`, `friend_suggestion_add` (nguồn).

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
