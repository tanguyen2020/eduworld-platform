# Data Model: F109 — Giao diện Arena, trận đấu, bảng xếp hạng, giải đấu trong Godot

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — F109 là game client, không tạo hay sửa bảng. Dữ liệu trận, rating, leaderboard, tournament thuộc module competition (`eduworld_competition`, F105–F107; [COMPETITION_SYSTEM §14.1](../../docs/01-game-design/COMPETITION_SYSTEM.md)).

Model typed phía client (chỉ RAM, `src/game/scripts/competition/models/`):

| Model | Nguồn | Trường giữ lại | Trường bị bỏ khi parse |
|---|---|---|---|
| `ArenaMode` | `listArenaModes` (delta) | `mode` (`MatchMode`), `available`, `locked_reason_code`, `speed_bonus_enabled` | — |
| `MatchmakingTicketModel` | `MatchmakingTicket`, `matchmaking.ticket.updated` | `ticket_id`, `status`, `mode`, `subject_id`, `wait_ms`, `search_widened`, `cancel_reason`, `match_id` | — |
| `MatchProposalModel` | `matchmaking.proposal.created/updated/cancelled` | `proposal_id`, `deadline_at`, `participants[]` (`PublicPlayer`), `accepted_count`, `total`, `requeued` | — |
| `BattleView` | `battle.state.snapshot` + message `battle.*` | `match_id`, `mode`, `state`, `state_version`, `question_index`, `question_count`, `current_question` (`QuestionRender`), `deadline_at`, `my_answer_locked`, `participants[]` (player, team, score, connected, `grace_until`), `speed_bonus_enabled` (delta), `primary_user_ids` (delta) | — |
| `MatchSummary` | `battle.match.ended`, `Match` | `status`, `results[]` (`outcome`, `score`, `correct_count`, `competition_points`), `encouragement` (delta) | `rating_delta` |
| `TierView` | `Rating` | `mode`, `subject_id`, `season_id`, `tier_code`/`tier_name` (delta; tạm `tier`), `provisional`, `games_played` | `rating`, `rd` |
| `LeaderboardViewModel` | `LeaderboardPage` | `leaderboard_id`, `period_key`, `display_mode` (delta), `entries[]` (`rank`, `player` `PlayerCard`, `value`, `is_me`), `class_goal` (delta), `highlights` (delta), `stale` (delta), `page` | mọi trường ngoài `PlayerCard` |
| `TournamentViewModel`, `BracketViewModel`, `MyEntry` | `Tournament`, `getTournament`, `getTournamentBracket` (delta) | trạng thái, lịch, cửa sổ check-in, node (`round`, `position`, hai entry nickname/tên đội, `winner`, `status`) | tên thật, trường |
| `DuelModel` | `Duel` | `duel_id`, `status`, `expires_at`, `match_id` | — |

Quy tắc: kiểm kiểu mọi trường nhận từ mạng; thiếu trường bắt buộc → bỏ message và gửi telemetry `contract_violation` ([GAME_NETWORKING §9](../../docs/08-game-client/GAME_NETWORKING.md)); message `battle.*` có `state_version` ≤ hiện tại bị bỏ.

## 2. Vòng đời / trạng thái

Trạng thái hiển thị trận theo [GAME_NETWORKING §7](../../docs/08-game-client/GAME_NETWORKING.md): `Queueing → ProposalShown → WaitingOthers → Loading → Countdown → Question → Answered → Reveal → RoundSummary → Result`, nhánh `Reconnecting`. `BattleClient` ánh xạ `battle.state.snapshot.state` (`lobby`, `ready_check`, `countdown`, `question_shown`, `reveal`, `round_end`, `paused`, `finalizing`, `ended`, `cancelled`) sang trạng thái trên.

## 3. Quy tắc kiểm tra dữ liệu

- Không giữ đáp án trước reveal (`correct_response` chỉ có trong `battle.answer.evaluated`) — ADR-0016.
- Không lưu xuống đĩa bất kỳ dữ liệu trận/bảng/người chơi khác.
