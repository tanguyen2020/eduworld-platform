# Implementation Plan: F110 — Godot — màn boss co-op, thẻ dữ kiện bước `interdependent`, danh sách bạn bè

**Branch**: `110-godot-boss-coop-friends` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Thêm ba domain phía client theo [GODOT_ARCHITECTURE §4, §6](../../docs/08-game-client/GODOT_ARCHITECTURE.md): (1) boss — `BossClient` (REST `listBosses`/`createBossEncounter`/`getBossEncounter`/`leaveBossEncounter` + lệnh/message `boss.*`; dispatcher `boss.*` theo [GAME_NETWORKING §4.2](../../docs/08-game-client/GAME_NETWORKING.md)) với `BossStore` áp theo `state_version`, scene `BossArena` (arena nhẹ, HUD năng lượng/phase/đồng hồ, `QuestionPanelHost` dùng renderer F063); (2) quest co-op — `CoopQuestService` (`getQuestStepClues`, `shareQuestStepClue`, `assignQuestStepRoles`, message `quest.roles.assigned`, `quest.clue.shared`, `quest.progress.updated.party_progress`) gắn `PartyProgressStrip` và `InterdependentStepPanel` vào `QuestTracker` của F072; (3) bạn bè — `SocialService` (REST `/social/*`) và `PresenceService` (message `presence.friend.updated`, lệnh `presence.status.set`) với `FriendsScreen`. Service là `RefCounted` tạo bởi `ServiceLocator`, nhận `ApiClient`/`RealtimeClient` qua constructor; UI chỉ đọc store. Phát triển trên Prism mock (nhãn MOCK) tới khi F095/F103 `Done`.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.x ghim trong `src/game/.godot-version`), static typing bắt buộc (ADR-0014) |
| Primary Dependencies | GdUnit4; autoload `ApiClient`, `RealtimeClient`, `SceneRouter`, `AppState`, `EventBus`, `QualityManager`, `ContentCache`; F063 `QuestionRendererRegistry`, `ActivityHost`; F072 `QuestService`/`QuestTracker`; F102 `PartyService`/`PartyStore`; F092 hub player registry; F069 `LevelUpQueue`; F119 `ChatDock`, `ReportDialog` (tùy chọn) |
| Storage | Không có dữ liệu bền; store trong RAM (ADR-0064) |
| Testing | GdUnit4 unit (`FakeApiClient`, `FakeRealtimeClient`, `FakeClock`) + scene runner; integration headless 2 client với docker compose; benchmark `tests/perf/BossArenaBenchmark.tscn` |
| Target Platform | Windows 10+, Android 9+, iOS, Web (ADR-0013) |
| Project Type | game client |
| Performance Goals | Boss co-op ≥ 30 FPS p10 tầm trung, ngân sách cảnh boss ([PERFORMANCE_BUDGET §7](../../docs/08-game-client/PERFORMANCE_BUDGET.md)), cập nhật ≤ 1 s |
| Constraints | AGENTS R1, R5, R6, R16, R17, R19, R25, R32 · ADR-0016, ADR-0020, ADR-0031, ADR-0047, ADR-0054, ADR-0056, ADR-0057, ADR-0064, ADR-0065 |
| Scale/Scope | Party ≤ `party_max_members`; 2–4 vai/bước; danh sách bạn ảo hóa (≤ giới hạn `friend_limits`) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Chỉ tiêu thụ contract F095/F101/F103; delta thuộc owner backend, theo dõi ở task Phase 2 (T004–T006) |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Client chỉ gọi API public/WS; Service → Store → UI; không NATS/Valkey (R25) |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `X-Tenant-Id` do `ApiClient`; không lưu bạn bè/thẻ xuống đĩa; xóa store khi đăng xuất/takeover |
| IV Server-authoritative | ✅ | Không gửi điểm/đóng góp/năng lượng; câu hỏi không đáp án; người trả lời lấy từ phiên ở server; lệnh ghi có `Idempotency-Key` |
| V Xác thực, phân quyền | ✅ | Token RAM (F061); leader-only UI chỉ là tiện ích, `PARTY_NOT_LEADER` do server; `GAME_ACCESS_STUDENT_ONLY` về đăng nhập |
| VI An toàn trẻ em | ✅ | Không xếp hạng/không lộ ai sai; chỉ nickname/avatar; không tìm kiếm tự do; chặn/tắt tiếng ≤ 2 chạm; không lộ ai chặn mình; chat qua F119 (pipeline server) |
| VII Một nguồn sự thật, cấu hình | ✅ | Grace, hạn mã, giới hạn từ server; tham số theo tên CONFIG_DEFAULTS; chuỗi qua `.po` |
| VIII UI tách logic, token | ✅ | UI chỉ gọi method public; Theme sinh từ `design/game-tokens`; `lint_theme_overrides.gd` |
| IX Test có bằng chứng | ✅ | Mỗi F110-AC có task test viết trước; mock gắn nhãn, không vào build phát hành |

## Project Structure

### Documentation (feature này)

```text
specs/110-godot-boss-coop-friends/
├── spec.md · plan.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
src/game/
├── project.godot                                  # input action open_friends (F)
├── scenes/
│   ├── challenge/boss/{BossInfoCard.tscn, BossArena.tscn, BossHud.tscn, BossEnergyBar.tscn, PhaseTimer.tscn,
│   │                   ContributionList.tscn, PartyReadyList.tscn, QuestionPanelHost.tscn, BossResult.tscn}   # + *.gd
│   ├── world/zones/boss_arena/Zone.tscn          # arena nhẹ theo WORLD_RENDERING, camera cố định trên mobile
│   ├── quest/{PartyProgressStrip.tscn, InterdependentStepPanel.tscn, ClueCard.tscn, SharedClueBoard.tscn, RoleAssignDialog.tscn}
│   └── social/{FriendsScreen.tscn, FriendRow.tscn, FriendProfilePanel.tscn, FriendRequestList.tscn,
│               FindFriendsTab.tscn, FriendCodeWidget.tscn, BlockList.tscn}
├── scripts/
│   ├── challenge/{boss_client.gd, boss_store.gd, models/{boss_summary.gd, boss_encounter_model.gd, boss_question_model.gd}}
│   ├── quest/{coop_quest_service.gd, coop_quest_store.gd, models/{party_quest_progress_model.gd, role_assignment_model.gd, clue_card_model.gd}}
│   ├── social/{social_service.gd, friends_store.gd, models/{friend_model.gd, friend_request_model.gd, friend_code_model.gd, relation_model.gd}}
│   ├── multiplayer/presence_service.gd           # presence.friend.updated, presence.status.set
│   └── core/{app_state.gd (thêm boss, friends), event_bus.gd (thêm friend_request_received), service_locator.gd}
├── locale/{vi.po,en.po}                           # ui.boss.*, ui.quest.coop.*, ui.quest.clue.*, ui.friends.*, ui.friendcode.*
└── tests/
    ├── unit/{boss,quest,social}/test_*.gd
    ├── scene/{test_boss_hud.gd, test_question_panel_host.gd, test_boss_result.gd, test_interdependent_panel.gd, test_friends_screen.gd, test_friend_code_widget.gd}
    ├── perf/BossArenaBenchmark.tscn
    ├── integration/{test_boss_party_headless.gd, test_friends_headless.gd}
    └── fixtures/{boss,coop_quest,social}/*.json   # sinh từ example OpenAPI/AsyncAPI
```

**Structure Decision**: Theo [GODOT_ARCHITECTURE §3](../../docs/08-game-client/GODOT_ARCHITECTURE.md): `scenes/social/` (đã có `FriendList`, `PartyLobby`), `scenes/quest/`, `scripts/multiplayer/presence_service.gd` (đã liệt kê); thêm `scenes/challenge/boss/`, `scripts/challenge/` (cùng F069), `scripts/social/` (F119 dùng lại) — cập nhật §3 ở task Polish. `FriendList.tscn` trong §3 hiện thực bằng `FriendsScreen.tscn` + `FriendRow.tscn`.

## Phase 0: Research

Không có — luồng, message và bố cục đã có ở STUDENT_GAME_UX_CORE S06, STUDENT_GAME_UX_SOCIAL §6, QUEST_SYSTEM §3.2, contract F095/F103; quyết định ranh giới và hiển thị đóng góp ghi ở Clarifications.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain (client) | Model typed `from_dict` (bỏ trường lạ) — xem data-model | Không logic nghiệp vụ |
| application (client) | `BossClient` (`list(area_id)`, `start_solo(boss_id)`, `join(encounter_id)`, `ready()`, `submit(question, response)`, `leave()`, `resync()`) · `BossStore` (`apply_update`, `apply_question`, `apply_evaluated`, phát hiện lệch `state_version`) · `CoopQuestService` (`load_clues(quest_id, step_id)`, `share(role)`, `assign_roles(assignments)`) · `SocialService` (friends, requests, codes, block, mute) · `PresenceService` (`set_status`) | Signal: `boss_updated(e: BossEncounterModel)`, `boss_question_assigned(q: BossQuestionModel)`, `boss_answer_evaluated(r: Dictionary)`, `boss_ended(outcome: StringName, rewards: Array)`, `roles_assigned(a: Array[RoleAssignmentModel])`, `clue_shared(c: ClueCardModel)`, `friends_changed()`, `friend_presence_changed(user_id: String)`, `request_failed(code: StringName)` |
| presentation (UI) | `BossInfoCard`, `BossArena`/`BossHud`, `QuestionPanelHost`, `ContributionList`, `PartyReadyList`, `BossResult`, `PartyProgressStrip`, `InterdependentStepPanel`, `ClueCard`, `SharedClueBoard`, `RoleAssignDialog`, `FriendsScreen` và tab | Chỉ đọc store, gọi method public |
| tích hợp | `RealtimeClient` dispatcher: `boss.*` → `BossClient`, `quest.roles.*`/`quest.clue.*`/`quest.progress.updated` (`party_progress`) → `CoopQuestService` (F072 vẫn nhận phần cá nhân), `presence.friend.updated` → `PresenceService`; `PartyStore.party_session_started` (F102) → `SceneRouter.go_to(scene_ref, {encounter_id})`; `AppState.blocking_activity = &"boss"` cho `LevelUpQueue` (F069) | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F103 (boss, quest co-op, thẻ dữ kiện), F095 (social), F101/F102 (Party Lobby, `party.session.*`, `PartyService.invite`), F092 (`RealtimeClient`, danh sách người chơi cùng instance), F091 (presence), F063 (renderer, `ActivityHost`), F072 (`QuestTracker`, `startQuestActivity`), F069 (`LevelUpQueue`, thư mục `scripts/challenge`), F119 (`ChatDock`, `ReportDialog` — tùy chọn), F061/F060 (ApiClient, Clock, Theme, i18n, Prism mock).
- Không gọi backend nào ngoài API public; không kết nối NATS/Valkey (R25).

### Bảo mật, tenant, idempotency, audit

- `Idempotency-Key` UUIDv7 mỗi ý định cho `createBossEncounter`, `leaveBossEncounter`, `shareQuestStepClue`, `assignQuestStepRoles`, `sendFriendRequest`, `acceptFriendRequest`, `createFriendCode`, `redeemFriendCode`; giữ nguyên khi retry. Lệnh WS dùng `id` UUIDv7 của envelope; `boss.answer.submit` gửi lại giữ `question_instance_id`.
- Không log nickname/ID học sinh khác hay nội dung thẻ dữ kiện ở mức INFO (R14).

### Quan sát

- Telemetry client (F080 khi có): `boss_enter`, `boss_phase_retry`, `boss_end` (outcome), `clue_share`, `friend_request_send`; thời gian từ `party.session.started` đến cảnh boss sẵn sàng; không PII.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
