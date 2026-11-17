# Contracts: F074 — XP ledger, level, đường cong XP, danh hiệu, Reward Engine có cap

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/eduworld-api.v1.yaml` → sinh `specs/openapi/modules/game.v1.yaml`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `getProgression` | `GET /api/v1/game/progression` → `ProgressionStatus` | game.v1.yaml | `game_player` (self) | Có sẵn |
| `listXpHistory` | `GET /api/v1/game/progression/xp-history?limit=&cursor=` → `XpLedgerEntry[]` | game.v1.yaml | `game_player` (self) | Có sẵn |
| `listProgressionLevels` | `GET /api/v1/game/progression/levels` → `ProgressionLevel[]` | game.v1.yaml | `game_player` | Có sẵn |
| `getXpStatus` | `GET /api/v1/game/xp` → `XpStatus` | game.v1.yaml | `game_player` (self) | Có sẵn (alias rút gọn của `getProgression`) |
| `createGameConfig_game` … `publishGameConfigVersion_game` | `/api/v1/game-admin/{config_type}` với `config_type` = `progression-curves`, `progression-levels` (chỉ đọc/ghi đè theo level), `reward-rules` | game.v1.yaml | `game.progression.write`, `game.reward_rule.write` (HTTP ở F078) | Có sẵn — F074 chỉ đăng ký module validate/OnPublished |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không gọi đồng bộ module khác (R29) | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `progression.xp.updated` | server → học sinh (Valkey `ew:rt:user:<student_id>`) | Có sẵn (`specs/asyncapi/eduworld-realtime.v1.yaml`) |
| `progression.level.reached` | server → học sinh | Có sẵn |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `activity.graded` | learning → game (`game.reward_engine`) | `specs/events/payloads/learning/activity.graded.v1.schema.json` | Cần sửa (thêm trường, §6) |
| `learning_objective.mastered` | learning → game | `specs/events/payloads/learning/learning_objective.mastered.v1.schema.json` | Cần thêm (owner F031) |
| `quest.completed`, `minigame.completed` | game (F072/F073) → `game.reward_engine` | `specs/events/payloads/game/` | Cần thêm (owner F072, F073) |
| `world.discovery_found`, `world.area_completed` | game (F071) → `game.reward_engine` | `specs/events/payloads/game/` | Cần thêm (owner F071) |
| `student.synced`, `enrollment.changed` | identity → game (`game.grade_transition`) | `specs/events/payloads/identity/` | Cần thêm (owner F010/F011) |
| `content.published` (item_type `progression_curve`, `reward_rule`) | game (F071/F078) → game (`curve-rollout`, cache rule) | `specs/events/payloads/game/content.published.v1.schema.json` | Cần thêm (owner F071, F078 mở rộng; F074 chỉ tiêu thụ) |
| `xp.awarded` | game → RT, LDB, ANA | `specs/events/payloads/game/xp.awarded.v1.schema.json` | Cần thêm |
| `level.reached` | game → RT, NTF, achievement (F075), ANA | `specs/events/payloads/game/level.reached.v1.schema.json` | Cần thêm |
| `reward.granted` | game → RT, AUD | `specs/events/payloads/game/reward.granted.v1.schema.json` | Cần thêm |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/progression-curve.schema.json` | Payload version `progression-curves` | Có sẵn |
| `specs/schemas/reward-rule.schema.json` | Payload version `reward-rules` | Cần sửa (§6) |

## 6. Delta cần thực hiện

- `reward-rule.schema.json`: thêm `source_code` (bắt buộc, `maxLength: 64`, enum theo catalog PROGRESSION_SYSTEM §3.1); thêm `trigger.event_type` = `activity.graded`, `world.discovery_found`, `world.area_completed`; `grants[].type` thêm `title`; `caps.bucket` (`maxLength: 64`). (`currency`, `collection_item`, `item_id` uuid: F075.)
- `activity.graded.v1`: thêm `content_version_id` (uuid), `first_try_correct_count` (integer ≥ 0 — đúng lần đầu, không `rapid_guess`, không sau giải thích), `recommendation_id` (uuid, null), `content_grade_levels` (integer 1–9, `maxItems: 9`). Owner module learning (F030/F031) — phối hợp trong cùng PR hoặc trước US1.
- Payload mới thư mục `specs/events/payloads/game/`: `xp.awarded.v1` (`student_id`, `amount`, `source_type`, `source_code`, `source_id`, `rule_id`, `capped`, `xp_total`), `level.reached.v1` (`student_id`, `from_level`, `to_level`, `curve_version`, `title_code`), `reward.granted.v1` (`grant_id`, `student_id`, `trigger_kind`, `items[]`, `xp`, `source_event_id`).
- `GameConfigType` (`game.v1.yaml`, spec gộp, mọi path `/game-admin/{config_type}`): đổi `levels` → `progression-levels` (RB-308; tên tránh trùng gói level `game.level_definitions`). Task T039.
- `specs/events/README.md`: thêm dòng game vào bảng "Payload đã có schema".
- Kiểm: `make specs-lint`, `make gen MOD=game`, `make test-contract`.
