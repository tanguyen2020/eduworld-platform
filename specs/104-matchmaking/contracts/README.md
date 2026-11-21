# Contracts: F104 — Matchmaking 8 chế độ: queue, ticket, proposal, ready check, ràng buộc khối

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md) · **Đã áp dụng 2026-11-21** (mọi dòng "Cần thêm/Cần sửa" của F104 đã vào contract; khác biệt ghi ở [Clarifications 2026-11-21](../spec.md#clarifications))

## 1. API (OpenAPI)

Spec: `specs/openapi/modules/competition.v1.yaml` (bundle `specs/openapi/eduworld-api.v1.yaml`), `x-service: competition`. Mọi route qua middleware `game_player`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `createMatchmakingTicket` | `POST /api/v1/competition/matchmaking/tickets` | `competition.v1.yaml` (`MatchmakingTicketCreate`, `MatchmakingTicket`) | học sinh · `Idempotency-Key` | Cần sửa |
| `getMatchmakingTicket` | `GET /api/v1/competition/matchmaking/tickets/{ticket_id}` | `competition.v1.yaml` | học sinh (chủ ticket) | Có sẵn |
| `cancelMatchmakingTicket` | `DELETE /api/v1/competition/matchmaking/tickets/{ticket_id}` | `competition.v1.yaml` | học sinh (chủ ticket/leader) | Có sẵn |
| `acceptMatchProposal` | `POST /api/v1/competition/matches/{match_id}/proposal:accept` | `competition.v1.yaml` | học sinh (participant) | Có sẵn |
| `declineMatchProposal` | `POST /api/v1/competition/matches/{match_id}/proposal:decline` | — | học sinh (participant) | Cần thêm |
| `startBotMatch` | `POST /api/v1/competition/matchmaking/tickets/{ticket_id}:bot-match` | — | học sinh (chủ ticket) · `Idempotency-Key` | Cần thêm |
| `listCompetitionModes` | `GET /api/v1/competition/modes` | — | học sinh | Cần thêm |
| `createDuel` | `POST /api/v1/competition/duels` | `competition.v1.yaml` (`Duel`) | học sinh · `Idempotency-Key` | Có sẵn |
| `acceptDuel` | `POST /api/v1/competition/duels/{duel_id}:accept` | `competition.v1.yaml` | học sinh (opponent) · `Idempotency-Key` | Có sẵn |
| `declineDuel` | `POST /api/v1/competition/duels/{duel_id}:decline` | `competition.v1.yaml` | học sinh (opponent) | Có sẵn |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có. F107 gọi command `CreateDirectProposal` trong cùng module | — | — | — |

## 3. Realtime (AsyncAPI)

Spec: `specs/asyncapi/eduworld-realtime.v1.yaml`; gateway định tuyến `matchmaking.*` → NATS `ew.cmd.competition.matchmaking.*`.

| Message | Hướng | Trạng thái |
|---|---|---|
| `matchmaking.ticket.cancel` | client → server | Có sẵn |
| `matchmaking.proposal.accept`, `matchmaking.proposal.decline` | client → server | Có sẵn |
| `matchmaking.ticket.updated` | server → client | Cần sửa (thêm `bot_offer_available`, `matchmaking_type`) |
| `matchmaking.proposal.created`, `matchmaking.proposal.updated`, `matchmaking.proposal.cancelled` | server → client | Có sẵn |
| `matchmaking.match.assigned` | server → client | Có sẵn |
| Lỗi `PROPOSAL_EXPIRED`, `ROOM_NOT_MEMBER` (`ErrorCode`) | server → client | Có sẵn |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `match.created` | Phát (`EVT_COMPETITION`) | `specs/events/payloads/competition/match.created.v1.schema.json` | Cần thêm |
| `duel.sent`, `duel.accepted`, `duel.declined`, `duel.expired` | Phát | `specs/events/payloads/competition/duel.*.v1.schema.json` | Cần thêm |
| `audit.recorded` | Phát | `specs/events/envelope.schema.json` | Có sẵn |
| `party.member_left`, `party.disbanded` | Tiêu thụ (nội bộ module) | — | Cần thêm (thuộc F100) |
| `user.blocked`, `user.unblocked` | Tiêu thụ (module chat) | — | Cần thêm (thuộc F095) |
| `student.synced`, `enrollment.changed`, `school.updated`, `tenant.updated` | Tiêu thụ (projection `ref.*`) | `specs/events/payloads/identity/…` | Có sẵn (áp 2026-11-17, owner F010/F011; `school.updated` có `settings_subset`) |
| `game_profile.updated` | Tiêu thụ (projection `ref.player_profiles`) | `specs/events/payloads/game/game_profile.updated.v1.schema.json` | Cần thêm (owner F070) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/matchmaking-config.schema.json` | Validate payload cấu hình matchmaking (seed, F108) | Cần thêm (F108 làm; F104 dùng seed hợp lệ) |

## 6. Delta cần thực hiện

- `competition.v1.yaml`:
  - `MatchmakingTicketCreate`: thêm `allow_fill` (boolean, default false); response thêm `422`, `429`. Mô tả: `matchmaking_type` `friend`/`tournament` → `422`.
  - `MatchmakingTicket`: thêm `matchmaking_type`, `bot_offer_available` (boolean), `search_widened` (boolean), `cancel_reason` (enum `user`, `party_changed`, `timeout`, `system`, `declined_proposal`), `proposal_id` (uuid); ghi chú `proposal_id` = `match_id`.
  - Thêm `declineMatchProposal` (`204`, `409 CONFLICT`), `startBotMatch` (`201` → `Match`, `409` khi chưa đủ `mm_bot_after`), `listCompetitionModes` (`200` `{data: CompetitionMode[]}`; `CompetitionMode {mode (MatchMode), matchmaking_types[] (enum), ranked (boolean), min_grade (GradeLevel), subjects[] (uuid, maxItems 50)}`).
  - `Match`: thêm `matchmaking_type`, `ranked`, `is_bot`.
- `eduworld-realtime.v1.yaml`: `matchmaking.ticket.updated.payload` thêm `bot_offer_available` (boolean), `matchmaking_type` (string, maxLength 64).
- Event payload schema mới `specs/events/payloads/competition/`: `match.created.v1` (`match_id`, `mode`, `matchmaking_type`, `ranked`, `is_bot`, `subject_id`, `grade_level`, `participants[] {student_id, school_id, team_no, party_id}` maxItems 40), `duel.sent/accepted/declined/expired.v1` (theo [EVENT_DESIGN §4.8](../../../docs/07-backend/EVENT_DESIGN.md#48-competition)). Mọi string có `maxLength`.
- identity `school.updated` payload: thêm `settings_subset` (`access_hours`, `disabled_leaderboard_definitions` maxItems 50, `allow_cross_school_listing`) — cần nhóm identity đồng ý.
- Tài liệu: VALKEY_DESIGN §3.1 thêm đoạn `<scope>` vào key queue và key `mm:cooldown:*`; DATABASE_DESIGN §2.10/§2.16/§2.17 thêm cột `matches`, bảng `competition_restrictions`, `game_configs`, `game_config_versions`, `content_releases`, `ref.enrollments`/`ref.schools.settings_subset` cho competition; API_DESIGN §9.7 thêm `startBotMatch`, ghi rõ `listCompetitionModes`.
