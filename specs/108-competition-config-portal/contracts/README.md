# Contracts: F108 — Cấu hình thi đấu `/game-admin/*` và UI workspace A

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/modules/competition.v1.yaml` (`x-internal: true`, `security: keycloakAdminBearer`). Path `/api/v1/game-admin/{config_type}…` với `config_type` ∈ `matchmaking-configs`, `ranking-rules`, `seasons`, `tournament-templates`, `challenge-templates`, `leaderboard-definitions`, `competition-rewards` (Cần thêm, RB-415).

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listGameConfigs_competition` | `GET /game-admin/{config_type}` | `competition.v1.yaml` | `competition.config.*` theo loại | Có sẵn |
| `createGameConfig_competition` | `POST /game-admin/{config_type}` | `competition.v1.yaml` (`GameConfigInput`) | `competition.config.*` theo loại | Cần sửa (`config_id`, `based_on_version_id`) |
| `getGameConfigVersion_competition` | `GET /game-admin/{config_type}/{config_id}/versions/{version}` | `competition.v1.yaml` | theo loại | Có sẵn |
| `updateGameConfigDraft_competition` | `PATCH /game-admin/{config_type}/{config_id}/versions/{version}` | `competition.v1.yaml` | theo loại · `If-Match` | Có sẵn |
| `transitionGameConfigVersion_competition` | `POST …/versions/{version}:transition` | `competition.v1.yaml` | theo loại; `game.content.review` (approve/reject); `game.content.publish` (publish) · `If-Match` · `Idempotency-Key` | Có sẵn |
| `listGameConfigVersions_competition` | `GET …/{config_id}/versions` | `competition.v1.yaml` | theo loại | Có sẵn |
| `diffGameConfigVersions_competition` | `GET …/versions/{version}:diff?with=` | `competition.v1.yaml` | theo loại | Có sẵn |
| `publishGameConfigVersion_competition` | `POST …/versions/{version}:publish` | `competition.v1.yaml` (`PublishRequest`) | `game.content.publish` · `Idempotency-Key` | Có sẵn |
| `rollbackGameConfig_competition` | `POST …/{config_id}:rollback` | `competition.v1.yaml` (`RollbackRequest`) | `game.content.publish` · `Idempotency-Key` | Có sẵn |
| `unpublishGameConfig_competition` | `POST …/{config_id}:unpublish` | `competition.v1.yaml` (`ReasonRequest`) | `game.content.publish` · `Idempotency-Key` | Có sẵn |
| `listGameConfigReleases_competition` | `GET …/{config_id}/releases` | `competition.v1.yaml` (`ContentRelease`) | theo loại | Có sẵn |
| `getMyPermissions` (dựng menu) | `GET /api/v1/me/permissions` | `identity.v1.yaml` | đã đăng nhập portal | Có sẵn (F012) |
| Workspace C: `listChallengeTemplates`, `createChallenge`, `listChallenges`, `getChallenge`, `cancelChallenge`, `listTournaments`, `getTournament`, `publishTournament`, `cancelTournament`, `registerTournamentEntry`, `confirmTournamentEntries`, `generateTournamentBracket`, `getTournamentBracket`, `confirmBracketNode`, `overrideBracketNode` | xem [F107 contracts](../../107-tournament-challenges/contracts/README.md) | F107 | `lms.challenge.write` | Có sẵn / Cần thêm (F107) |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không có (portal không dùng WS cho cấu hình) | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `competition_config.published` | Phát + tiêu thụ nội bộ module competition | `specs/events/payloads/competition/competition_config.published.v1.schema.json` | Cần thêm (+ catalog EVENT_DESIGN §4.8) |
| `audit.recorded` | Phát | envelope | Có sẵn |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/matchmaking-config.schema.json` | Queue/grade band/cửa sổ rating/ready check/bot profile | Cần thêm |
| `specs/schemas/ranking-rules.schema.json` | Tham số Glicko-2, provisional, tier, soft reset, bảng Competition Points, trần, `effort_points` | Cần thêm |
| `specs/schemas/season.schema.json` | Mã, tên vi/en, chủ đề, `starts_at`/`ends_at` | Cần thêm |
| `specs/schemas/tournament-template.schema.json` | `kind`, format, mode, participant_type, `allowed_params`, `reward_rule_refs`, timers | Cần thêm |
| `specs/schemas/challenge-template.schema.json` | Thử thách khối `grade_challenge` (Timed Quiz): số câu, thời gian, khung tham gia, `grade_levels`, `allowed_params` (không có tham số lớp — ADR-0095) | Cần thêm |
| `specs/schemas/leaderboard-definition.schema.json` | metric × scope × period × filter, khối, hiển thị (scope không có `class`; metric `grade_goal` + `display.grade_goal_target` — ADR-0095, F106) | Cần thêm |
| `specs/schemas/competition-reward.schema.json` | Phần thưởng mùa/giải/thử thách theo tier/hạng; `reward_rule_refs` chỉ cosmetic/badge/title | Cần thêm |

## 6. Delta cần thực hiện

- `competition.v1.yaml`: `GameConfigInput` thêm `config_id` (uuid, tạo version mới cho config sẵn có) và `based_on_version_id` (uuid); `updateGameConfigDraft_competition` mô tả `409 CONTENT_NOT_EDITABLE` khi version không phải draft; `412` dùng code `CONTENT_VERSION_CONFLICT`; `GameConfigVersion.payload` mô tả `$ref` theo `config_type` (6 schema §5).
- `GameConfigType` (phần competition) thêm `competition-rewards`, permission `competition.config.reward.write` (RB-415).
- 7 JSON Schema mới (§5), mọi string có `maxLength`, enum dùng `specs/schemas/common/enums.schema.json` khi có.
- Payload `competition_config.published.v1`; catalog EVENT_DESIGN §4.8.
- ADR-0095: op learning của màn workspace C (`listChallenges` lọc `grade_level`, `ChallengeTemplate.kind` `grade_challenge`, `ChallengeCreate` `school_id`/`targets`/`exclude_student_ids`) — xem [F107 contracts §6](../../107-tournament-challenges/contracts/README.md#6-delta-cần-thực-hiện) (chờ áp sau WP-L1; F108 không định nghĩa lại).
- Tài liệu: ADMIN_PORTAL §3.4 ghi rõ màn "Thi đấu" (5 nhóm) và màn workspace C "Thử thách khối / giải trường" (nhãn đổi theo ADR-0095, khóa menu giữ); `competition.config.reward.write` dùng cho `competition-rewards` (đã chốt RB-415).
- APISIX: chỉ route `/api/v1/game-admin/{matchmaking-configs|ranking-rules|seasons|tournament-templates|challenge-templates|leaderboard-definitions|competition-rewards}` tới module competition ở gateway nội bộ.
