# Implementation Plan: F109 — Giao diện Arena, trận đấu, bảng xếp hạng, giải đấu trong Godot

**Branch**: `109-godot-arena-competition-ui` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Bốn service `RefCounted` (tạo bởi `ServiceLocator` trong `AppState`): `MatchmakingService` (chế độ, ticket, proposal, duel), `BattleClient` (state machine hiển thị trận theo [GAME_NETWORKING §7](../../docs/08-game-client/GAME_NETWORKING.md), dựng từ `battle.state.snapshot`), `LeaderboardService`, `TournamentService`; `RatingService` gộp vào `LeaderboardService` (chỉ đọc tier). UI ở `scenes/competition/` chỉ gọi method service và đọc model typed. Câu hỏi trong trận render bằng `QuestionRendererRegistry` (F063) ở chế độ "không phản hồi tại chỗ". Mọi điểm/kết quả/thứ hạng/tier lấy từ server; model bỏ trường rating thô. Cách trình bày theo grade band (density, ẩn số giây `g1_3`, bracket "con đường") lấy từ `AppState.player.grade_band`; quyết định nghiệp vụ (chế độ khả dụng, bonus tốc độ, `display_mode` bảng) do server trả qua delta contract.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.x ghim), static typing |
| Primary Dependencies | GdUnit4; `ApiClient`, `Clock` (F061); `RealtimeClient` (F092); `QuestionRendererRegistry` (F063); `QualityManager`, `SceneRouter`, `EventBus`, `AppState` (F060/F062); `MatchChatStrip` (F119) |
| Storage | Không có dữ liệu bền; state trong RAM |
| Testing | GdUnit4 unit + scene runner; contract test parse fixture sinh từ example OpenAPI/AsyncAPI; integration hai client headless với compose |
| Target Platform | Windows, Android (Web/iOS theo ADR-0013) |
| Project Type | game client |
| Performance Goals | [PERFORMANCE_BUDGET](../../docs/08-game-client/PERFORMANCE_BUDGET.md); nền đấu trường theo [STUDENT_GAME_UX_SOCIAL §8](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md) |
| Constraints | AGENTS R1, R5, R6, R10, R14, R15, R16, R19, R25, R32 · ADR-0016, ADR-0020, ADR-0047, ADR-0056 |
| Scale/Scope | Trận 2–8 người (`battle_room_size`); bảng top N theo `leaderboard_visibility`; bracket ≤ 64 entry |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | spec Ready; delta OpenAPI/AsyncAPI là task Phase 2 (T004–T007) |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Client chỉ gọi API public + AsyncAPI; Service ← UI, không logic ở scene |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `X-Tenant-Id` phiên; chỉ gửi id server cấp; test 404 ngoài phạm vi (F109-AC-21, F109-AC-29) |
| IV Server-authoritative | ✅ | Không gửi điểm; đúng/sai/điểm/hạng chỉ từ message server; ghi có `Idempotency-Key` |
| V Xác thực, phân quyền | ✅ | Route người chơi qua `game_player` (backend); client xử lý `GAME_ACCESS_STUDENT_ONLY` theo F061; không URL hardcode |
| VI An toàn trẻ em | ✅ | Chỉ nickname/avatar/tier; không rating; `g1_3` không thứ hạng cá nhân; chat trận always-on không tắt; không "bị vượt qua" |
| VII Một nguồn sự thật, cấu hình | ✅ | Chế độ, bonus, `display_mode`, cửa sổ check-in từ server; tham số theo tên |
| VIII UI tách logic, token | ✅ | Theme `eduworld_theme.tres` + density; `lint_theme_overrides.gd` |
| IX Test có bằng chứng | ✅ | Mỗi F109-AC có task test viết trước; Prism mock gắn nhãn MOCK |

## Project Structure

### Documentation (feature này)

```text
specs/109-godot-arena-competition-ui/
├── spec.md
├── plan.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
godot/
├── scenes/competition/
│   ├── ArenaScreen.tscn (+ arena_screen.gd)            # ModeCard, SubjectTopicPicker, TierBadge, nút Bảng xếp hạng/Lịch sử/Giải đấu
│   ├── components/{ModeCard,TierBadge,SubjectTopicPicker,QueueStatusBar,MatchHeader,AnswerRevealOverlay,ReconnectOverlay}.tscn
│   ├── MatchmakingPanel.tscn, ReadyCheck.tscn           # ReadyCheckDialog
│   ├── BattleArena.tscn (+ battle_arena.gd)             # QuestionPanel (F063 registry), AnswerButtons, slot MatchChatStrip (F119)
│   ├── MatchResult.tscn, MatchHistory.tscn, MatchDetail.tscn
│   ├── Leaderboard.tscn (+ leaderboard_view.gd), leaderboard/{RankedList,ClassGoalBar,PercentileCard}.tscn
│   ├── tournament/{TournamentList,TournamentDetail,BracketTree,BracketPath}.tscn
│   └── DuelInviteDialog.tscn
├── scripts/multiplayer/
│   ├── matchmaking_service.gd        # listArenaModes, ticket, proposal, duel
│   └── battle_client.gd              # state machine hiển thị, state_version, snapshot
├── scripts/competition/
│   ├── leaderboard_service.gd        # listLeaderboards, getLeaderboard, getMyLeaderboardPosition, getMyRatings
│   ├── tournament_service.gd
│   ├── match_history_service.gd      # listMyMatches, getMatch, createReport/blockUser cho đối thủ
│   └── models/{arena_mode,matchmaking_ticket,match_proposal,battle_view,match_summary,tier_view,leaderboard_view,tournament_view,bracket_view,duel}.gd
├── locale/{vi.po,en.po}              # ui.arena.*, ui.leaderboard.*, ui.tournament.*
└── tests/{unit,scene,contract,integration}/competition/, tests/fixtures/competition/*.json
```

**Structure Decision**: Theo [GODOT_ARCHITECTURE §3](../../docs/08-game-client/GODOT_ARCHITECTURE.md) (`scenes/competition`, `scripts/multiplayer/{matchmaking_service,battle_client}.gd`, `scripts/competition/leaderboard_service.gd`); thêm `tournament_service.gd`, `match_history_service.gd` cùng thư mục.

## Phase 0: Research

Không có. State machine trận ở [GAME_NETWORKING §7](../../docs/08-game-client/GAME_NETWORKING.md), quy tắc hiển thị ở [COMPETITION_SYSTEM §8–§12](../../docs/01-game-design/COMPETITION_SYSTEM.md) và [STUDENT_GAME_UX_SOCIAL §8](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md).

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| Service | `MatchmakingService.load_modes()`, `find_match(mode, subject_id, topic_id)`, `cancel()`, `accept_proposal()`, `decline_proposal()`, `challenge_friend()`, `respond_duel()`; `BattleClient.submit_answer(response)`, `request_state()`; `LeaderboardService.list()`, `page(id, period_key, cursor)`, `me(id)`, `my_tier()`; `TournamentService.list()`, `detail()`, `register()`, `withdraw()`, `check_in()`, `bracket()`; `MatchHistoryService.list()`, `detail()`, `report_opponent()`, `block_opponent()` | Signal: `modes_loaded`, `ticket_updated`, `proposal_received`, `match_assigned`, `battle_view_changed(view)`, `answer_locked`, `answer_evaluated`, `match_ended(summary)`, `leaderboard_loaded`, `tournament_updated`, `request_failed(problem)` |
| Gameplay | NPC Trọng tài Sấm phát intent `arena` qua `EventBus.ui_requested`; phím `K` | Không logic thi đấu |
| UI | Scene ở `scenes/competition/` | Chỉ gọi method service, đọc model |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F105 (battle, `/competition/matches*`, AsyncAPI `battle.*`), F104 (matchmaking — qua F105), F106 (rating, leaderboard), F107 (tournament), F102 (party; hàng chờ party), F092 (`RealtimeClient`), F063 (renderer), F119 (chat strip), F095 (`blockUser`), F121 (`createReport`), F123 (`notification.item.created`).
- Dùng bởi: F102 (hàng chờ của leader dùng `MatchmakingService`).

### Bảo mật, tenant, idempotency, audit

- `Idempotency-Key` UUIDv7: `createMatchmakingTicket`, `createDuel`, `acceptDuel`, `createReport`, `createTournamentEntry`, `checkInTournamentEntry`; giữ khi retry, không tự retry ghi không có key.
- Model typed bỏ `rating`, `rd`, `rating_delta`, mọi trường tên thật/trường nếu server lỡ gửi; contract test quét fixture.
- Audit ghi phía server (F105–F107); client không log body câu trả lời, nickname người khác.

### Quan sát

- Telemetry (`POST /api/v1/game/telemetry`, batch): `arena_queue_wait_ms`, `battle_reconnect_count`, `battle_resync_ok`, `battle_scene_fps_p10`; không PII, không câu trả lời (NFR-OBS-003).

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
| — | — | — |
