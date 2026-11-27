# Contracts: F109 — Giao diện Arena, trận đấu, bảng xếp hạng, giải đấu trong Godot

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/eduworld-api.v1.yaml` (`x-service: competition`, `chat`). Mọi route người chơi qua middleware `game_player` (AGENTS R23); header `X-Tenant-Id` phiên.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listArenaModes` | `GET /api/v1/competition/modes` | eduworld-api (competition) | `game_player` | Cần thêm |
| `createMatchmakingTicket` | `POST /api/v1/competition/matchmaking/tickets` | eduworld-api | `game_player` | Có sẵn |
| `getMatchmakingTicket` · `cancelMatchmakingTicket` | `GET`/`DELETE /api/v1/competition/matchmaking/tickets/{ticket_id}` | eduworld-api | `game_player` | Có sẵn |
| `acceptMatchProposal` | `POST /api/v1/competition/matches/{match_id}/proposal:accept` | eduworld-api | `game_player` | Có sẵn |
| `listMyMatches` · `getMatch` | `GET /api/v1/competition/matches` · `GET …/{match_id}` | eduworld-api | `game_player` | Có sẵn (`Match.participants[].rating_delta` Cần sửa) |
| `getMyRatings` | `GET /api/v1/competition/ratings/me` | eduworld-api | `game_player` | Cần sửa (`Rating`) |
| `createDuel` · `acceptDuel` · `declineDuel` | `POST /api/v1/competition/duels` · `…/{duel_id}:accept` · `…:decline` | eduworld-api | `game_player` | Có sẵn |
| `listLeaderboards` | `GET /api/v1/leaderboards` | eduworld-api (competition) | `game_player` | Cần thêm |
| `getLeaderboard` · `getMyLeaderboardPosition` | `GET /api/v1/leaderboards/{leaderboard_id}` · `…/me` | eduworld-api | `game_player` | Cần sửa (`LeaderboardPage`) |
| `listTournaments` | `GET /api/v1/competition/tournaments` | eduworld-api | `game_player` | Có sẵn (thêm query `status` — Cần sửa) |
| `getTournament` | `GET /api/v1/competition/tournaments/{tournament_id}` | eduworld-api | `game_player` | Cần thêm |
| `getTournamentBracket` | `GET /api/v1/competition/tournaments/{tournament_id}/bracket` | eduworld-api | `game_player` | Cần thêm |
| `createTournamentEntry` | `POST /api/v1/competition/tournaments/{tournament_id}/entries` | eduworld-api | `game_player` | Cần thêm |
| `withdrawTournamentEntry` | `DELETE /api/v1/competition/tournaments/{tournament_id}/entries/{entry_id}` | eduworld-api | `game_player` | Cần thêm |
| `checkInTournamentEntry` | `POST /api/v1/competition/tournaments/{tournament_id}/entries/{entry_id}:check-in` | eduworld-api | `game_player` | Cần thêm |
| `createReport` | `POST /api/v1/chat/reports` | eduworld-api (chat) | `game_player` | Có sẵn |
| `blockUser` | `PUT /api/v1/social/blocks/{user_id}` | eduworld-api (chat) | `game_player` | Có sẵn |
| `createRealtimeTicket` | ticket WebSocket (F092) | eduworld-api | `game_player` | Có sẵn |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có (game client) | — | — | — |

## 3. Realtime (AsyncAPI)

Spec: `specs/asyncapi/eduworld-realtime.v1.yaml`.

| Message | Hướng | Trạng thái |
|---|---|---|
| `matchmaking.ticket.cancel`, `matchmaking.proposal.accept`, `matchmaking.proposal.decline` | client → server | Có sẵn |
| `matchmaking.ticket.updated`, `matchmaking.proposal.created`, `matchmaking.proposal.updated`, `matchmaking.proposal.cancelled`, `matchmaking.match.assigned` | server → client | Có sẵn |
| `battle.answer.submit`, `battle.state.request` | client → server | Có sẵn |
| `battle.state.snapshot`, `battle.countdown.started` | server → client | Có sẵn 2026-11-27 (`speed_bonus_enabled`; snapshot thêm `primary_user_ids`, `chat_channel_ids`) |
| `battle.question.shown` | server → client | Có sẵn 2026-11-27 (`primary_user_ids`; `QuestionRender.options[]` có `id` chuẩn + alias `option_id`) |
| `battle.answer.recorded`, `battle.answer.evaluated`, `battle.question.revealed`, `battle.round.ended`, `battle.participant.status_changed`, `battle.match.paused`, `battle.match.resumed` | server → client | Có sẵn |
| `battle.match.ended` | server → client | Có sẵn 2026-11-27 (`encouragement` riêng người nhận) |
| `matchmaking.match.assigned` (`chat_channel_ids`), `competition.duel.received` | server → client | Có sẵn 2026-11-27 |
| `notification.item.created` (`kind = tournament_update`) | server → client | Cần sửa (owner F123: enum `kind` chung, bỏ `category`) |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Client không phát/tiêu thụ event NATS (AGENTS R25) | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `QuestionRender`, `AnswerResponse` (AsyncAPI components) | Render câu hỏi trong trận, gửi câu trả lời | Có sẵn |

## 6. Delta cần thực hiện

Phase 2 task T004–T007. F109 không sửa `specs/` trong đợt viết spec; owner thực hiện.

- **Cần thêm** (owner F105) `listArenaModes` `GET /competition/modes` → `{data: ArenaMode[]}`; `ArenaMode` {`mode` `MatchMode`, `available` boolean, `locked_reason_code` string maxLength 64, `speed_bonus_enabled` boolean} ([COMPETITION_SYSTEM §2](../../../docs/01-game-design/COMPETITION_SYSTEM.md)).
- **Đã áp 2026-11-27** (owner F105, AsyncAPI — xem spec Clarifications phiên "delta server đã có") `battle.countdown.started` + `battle.state.snapshot` thêm `speed_bonus_enabled` boolean; `battle.question.shown` thêm `primary_user_ids` (array `Uuid`, maxItems 8, chỉ `team_battle`); `battle.match.ended` thêm `encouragement` {`code` enum `loss_streak_review`, `message_code`, `subject_id`, `topic_id`?} ([COMPETITION_SYSTEM §3.2, §12](../../../docs/01-game-design/COMPETITION_SYSTEM.md)); thêm `chat_channel_ids` (`matchmaking.match.assigned`, `battle.state.snapshot`), `competition.duel.received`, khóa lựa chọn chuẩn `id` (+ alias `option_id`), `409 MM_TICKET_ALREADY_ACTIVE` kèm `ticket_id`.
- **Cần sửa** (owner F106) `Rating` cho API học sinh: bỏ `rating`, `rd`; thêm `tier_code` (maxLength 32), `tier_name` (`LocalizedName`), `provisional` boolean; bỏ `Match.participants[].rating_delta` khỏi API học sinh ([COMPETITION_SYSTEM §8.2](../../../docs/01-game-design/COMPETITION_SYSTEM.md), AC-CMP-101…102).
- **Cần thêm** (owner F106) `listLeaderboards` `GET /leaderboards` → `{data: LeaderboardSummary[]}` {`leaderboard_id`, `name` `LocalizedName`, `metric`, `scope_type`, `periods[]`, `display_mode`}; **Cần sửa** `LeaderboardPage`: `metric` thêm `effort_points`, `grade_goal`, `team_points` (đổi `quests_completed` → `quest_completed`), `scope_type` dùng `tenant` thay `organization` (`class` đã bỏ — **Đã áp 2026-11-21 (WP-S)**), thêm `display_mode` enum [`grade_goal`, `top_n`, `percentile`, `ranked_around_me`], `me` {`rank`?, `percentile_band`? integer}, `grade_goal` {`grade_level`, `current`, `target`, `my_contribution`, `contributors_count`? (null khi < 5)}, `stale` boolean (ADR-0095: thay `class_goal`, bỏ `highlights[]`); `getMyLeaderboardPosition` trả ± theo `leaderboard_visibility` ([COMPETITION_SYSTEM §9](../../../docs/01-game-design/COMPETITION_SYSTEM.md)).
- **Cần thêm** (owner F107) `getTournament`, `getTournamentBracket`, `createTournamentEntry` (`Idempotency-Key`, body {`team_ref`?}), `withdrawTournamentEntry`, `checkInTournamentEntry` (`Idempotency-Key`) theo [COMPETITION_SYSTEM §14.2](../../../docs/01-game-design/COMPETITION_SYSTEM.md); schema `TournamentDetail` (`my_entry` {`entry_id`, `status`}, `check_in_opens_at`, `check_in_closes_at`), `TournamentBracket` (`nodes[]` {`node_id`, `bracket`, `round`, `position`, `entry_a`/`entry_b` {`display_name` maxLength 128}, `winner_entry_id`, `scheduled_at`, `status`}, maxItems 128); sửa `Tournament.status`/`format`/`scope_type` theo §10.2–§10.3; `listTournaments` thêm query `status`.
- **Đã áp 2026-11-27** (owner F107) `ProblemCode`: `TOURNAMENT_NOT_ELIGIBLE` (403), `TOURNAMENT_REGISTRATION_CLOSED` (422), `TOURNAMENT_FULL` (409), `TOURNAMENT_CHECK_IN_CLOSED` (422) — cùng HTTP status như trước.
- **Cần sửa** (owner F123) `notification.item.created`: bỏ `category`, dùng enum `kind` chung REST/WS (RB-513); thông báo giải dùng `kind = tournament_update`.
