# Contracts: F077 — Event & Season live-ops: vòng đời sự kiện, community goal, season track, sự kiện trường

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/eduworld-api.v1.yaml` → `specs/openapi/modules/game.v1.yaml`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listGameEvents` | `GET /api/v1/game/events?status=live,upcoming` | game.v1.yaml | `game_player` | Có sẵn |
| `getGameEvent` | `GET /api/v1/game/events/{event_id}` (`EVENT_NOT_AVAILABLE`) | game.v1.yaml | `game_player` | Có sẵn |
| `joinGameEvent` | `POST /api/v1/game/events/{event_id}:join` | game.v1.yaml | `game_player` | Có sẵn |
| `claimEventMilestone` | `POST /api/v1/game/events/{event_id}/milestones/{milestone_id}:claim` (`Idempotency-Key`) | game.v1.yaml | `game_player` | Có sẵn |
| `getCommunityGoal` | `GET /api/v1/game/community-goals/{goal_id}` | game.v1.yaml | `game_player` | Có sẵn |
| `getSeasonTrack` | `GET /api/v1/game/season-track` | game.v1.yaml | `game_player` | Có sẵn |
| `claimSeasonTier` | `POST /api/v1/game/season-track/tiers/{tier}:claim` (`Idempotency-Key`) | game.v1.yaml | `game_player` | Có sẵn |
| `listSeasonTrackHistory` | `GET /api/v1/game/season-track/history` | game.v1.yaml | `game_player` | Có sẵn |
| `suspendEventRun` / `resumeEventRun` / `cancelEventRun` | `POST /api/v1/game-admin/events/{event_id}/runs/{run_id}:suspend\|:resume\|:cancel` (`EventRunOperation`) | game.v1.yaml | `game.event.operate` (role platform, gateway nội bộ) | Đã sửa (thêm `422 INVALID_STATE_TRANSITION`, `428`) |
| `getEventRunStats` | `GET /api/v1/game-admin/events/{event_id}/runs/{run_id}/stats` | game.v1.yaml | `game.liveops.read` | Có sẵn |
| `listEventTemplates` | `GET /api/v1/lms/event-templates` | game.v1.yaml | `lms.school_event.write` | Có sẵn |
| `listSchoolEvents` / `createSchoolEvent` | `GET/POST /api/v1/lms/school-events` | game.v1.yaml | `lms.school_event.write` | Đã sửa (`createSchoolEvent` khai báo `409`, `428`, `503`) |
| `getSchoolEvent` / `updateSchoolEvent` | `GET/PATCH /api/v1/lms/school-events/{event_id}` (`If-Match`, `Idempotency-Key`) | game.v1.yaml | `lms.school_event.write` | Đã sửa (`updateSchoolEvent` thêm `Idempotency-Key`, `428`, `503` — breaking ADR-0010) |
| `cancelSchoolEvent` | `POST /api/v1/lms/school-events/{event_id}:cancel` | game.v1.yaml | `lms.school_event.write` | Có sẵn |
| `/game-admin/{config_type}` (`events`, `event-templates`, `season-tracks`) | workflow ở F078 | game.v1.yaml | `game.event.write`, `game.season_track.write` | Có sẵn — F077 đăng ký module |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `screenText` | game → chat (facade `chat`, input: `tenant_id`, `locale`, `texts[]` ≤ 10 × `maxLength` 4000 → `allowed`, `violations[]` chỉ chỉ số văn bản, không trả từ khớp) | `specs/openapi/modules/chat.internal.v1.yaml` | `[game]` | Đã thêm (F077) |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `event.status.changed` | server → học sinh trong audience | Có sẵn |
| `event.progress.updated` | server → học sinh | Có sẵn |
| `event.community_goal.updated` | server → học sinh đang online của phạm vi | Đã sửa (nhịp theo `community_goal_progress_flush_seconds`) |
| `season.track.updated` | server → học sinh | Có sẵn |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `event.scheduled`, `event.started`, `event.suspended`, `event.resumed`, `event.ended`, `event.cancelled` | game → manifest, RT, NTF, QST, ANA, AUD | `specs/events/payloads/game/event.<action>.v1.schema.json` | Đã thêm (F077) |
| `event.participation_started`, `event.milestone_claimed` | game → ANA, achievement, reward | `specs/events/payloads/game/` | Đã thêm (F077) |
| `community_goal.milestone_reached`, `community_goal.completed` | game → reward, RT, world | `specs/events/payloads/game/` | Đã thêm (F077) |
| `season_track.tier_reached`, `season_track.reward_claimed` | game → reward, RT, ANA, AUD | `specs/events/payloads/game/` | Đã thêm (F077) |
| `season.started`, `season.ended` | competition → game (`game.season_projection`) | `specs/events/payloads/competition/` | Cần thêm (owner F106) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/event.schema.json` | Version `events` | Đã sửa (`decoration.bundle_size_bytes`) |
| `specs/schemas/season-track.schema.json` | Version `season-tracks` | Có sẵn |
| `specs/schemas/event-template.schema.json` | Version `event-templates` | Đã thêm (F077) |
| `specs/schemas/achievement.schema.json` | Category `event`, trigger `event.participation_started`, `event.milestone_claimed`, `season_track.tier_reached` (`community_goal.completed` không là trigger — spec Clarifications 2026-11-23) | Đã sửa |
| `specs/schemas/reward-rule.schema.json` | Trigger nguồn điểm sự kiện (không đổi) | Có sẵn |

## 6. Delta cần thực hiện

Đã thực hiện toàn bộ ngày 2026-11-23 (F077).

- `chat.internal.v1.yaml` (owner module chat, F116): thêm `screenText` (`x-allowed-callers: [game]`, response không chứa từ khớp/vị trí — R15) + facade test game ↔ chat.
- `game.v1.yaml`: `createSchoolEvent` khai báo `409 CONFLICT` (quá số sự kiện đồng thời), `503 UNAVAILABLE` (kiểm văn bản lỗi); `joinGameEvent`, `createSchoolEvent`, `updateSchoolEvent` khai báo `428`.
- `eduworld-realtime.v1.yaml`: mô tả `event.community_goal.updated` theo `community_goal_progress_flush_seconds` (bỏ "10 giây").
- Thêm `event-template.schema.json`; sửa `achievement.schema.json` (category `event`, trigger sự kiện/mùa).
- Payload event ở `specs/events/payloads/game/` (bảng §4).
- Kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`, `make test-contract`.
