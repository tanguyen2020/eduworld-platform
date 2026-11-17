# Contracts: F069 — Godot — tiến trình, kho đồ, shop, bộ sưu tập, thành tựu, challenge solo

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

Client chỉ tiêu thụ contract; không sở hữu contract nào. Spec nguồn: `specs/openapi/eduworld-api.v1.yaml` (sinh `specs/openapi/modules/game.v1.yaml`), `specs/asyncapi/eduworld-realtime.v1.yaml`. Mọi operation: middleware `game_player`, header `X-Tenant-Id`.

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `getProgression`, `listXpHistory`, `listProgressionLevels` | `GET /api/v1/game/progression`, `…/xp-history`, `…/levels` | game.v1.yaml | `game_player` (self) | Có sẵn (F074) |
| `getWallet`, `listWalletLedger` | `GET /api/v1/game/wallet`, `…/wallet/ledger` | game.v1.yaml | `game_player` (self) | Có sẵn (F075) |
| `listInventory` | `GET /api/v1/game/inventory?kind=` | game.v1.yaml | `game_player` (self) | Cần sửa — query `kind` (delta F075) |
| `listInventoryHistory` | `GET /api/v1/game/inventory/history` | game.v1.yaml | `game_player` (self) | Có sẵn; Cần sửa `item_name` song ngữ (§6) |
| `listShopItems` | `GET /api/v1/game/shop/items?kind=` | game.v1.yaml | `game_player` | Có sẵn |
| `purchaseShopItem` | `POST /api/v1/game/shop/items/{shop_item_id}:purchase` (`Idempotency-Key`) | game.v1.yaml | `game_player` | Cần sửa — khai báo `422`, `428` (delta F075) |
| `updateAvatar`, `listAvatarPresets`, `saveAvatarPreset` | `/api/v1/game/profile/avatar*` (gọi qua `ProfileService` F070) | game.v1.yaml | `game_player` | Có sẵn (F070) |
| `listAchievements` | `GET /api/v1/game/achievements?category=` | game.v1.yaml | `game_player` | Có sẵn; Cần sửa `hidden` (§6) |
| `listCollections`, `getCollection` | `GET /api/v1/game/collections`, `…/{collection_id}` | game.v1.yaml | `game_player` | Có sẵn; Cần sửa `items[].how_to_obtain` (§6) |
| `listGameChallenges`, `getGameChallenge` | `GET /api/v1/game/challenges`, `…/{challenge_id}` | game.v1.yaml | `game_player` | Có sẵn; Cần sửa `active_attempt_id`, `difficulties[]`, mô tả song ngữ (§6) |
| `startGameChallenge` | `POST /api/v1/game/challenges/{challenge_id}:start` (`Idempotency-Key`, body `difficulty`) | game.v1.yaml | `game_player` | Cần sửa — `422`, `428` (delta F076) |
| `nextChallengeActivity` | `POST /api/v1/game/challenge-attempts/{challenge_attempt_id}/activities:next` (`Idempotency-Key`) | game.v1.yaml | `game_player` (self) | Cần sửa — `428`, `503` (delta F076) |
| `getGameChallengeAttempt`, `abandonGameChallengeAttempt` | `GET …/challenge-attempts/{id}`, `POST …:abandon` | game.v1.yaml | `game_player` (self) | Có sẵn |
| `getActivityAttempt`, `submitActivityResponse`, `submitActivityAttempt` | `/api/v1/learning/activity-attempts/*` (qua `LearningService` F063) | learning.v1.yaml | `game_player` | Có sẵn (F030) |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Client không gọi internal API | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `progression.xp.updated`, `progression.level.reached` | server → học sinh | Có sẵn |
| `challenge.attempt.updated` | server → học sinh | Có sẵn |
| `notification.item.created` (category `reward`, `achievement`) | server → học sinh | Có sẵn (giao bởi F123; chỉ dùng để làm mới ví) |
| `session.session.replaced`, `system.command.failed` | server → client | Có sẵn |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Client không dùng NATS (R25) | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không có | — |

## 6. Delta cần thực hiện

Không có delta do F069 sở hữu. Các thay đổi dưới đây cần owner backend làm trước khi F069 `Done` (task Phase 2 T004–T006 theo dõi; fixture sinh lại sau khi merge):

- **F076 — Cần sửa** `GameChallengeSummary`: thêm `active_attempt_id` (`Uuid`, nullable) để client tìm challenge `in_progress` sau mất kết nối/takeover (F069-AC-20). `GameChallengeDetail`: thêm `difficulties[]` (`maxItems: 5`) gồm `difficulty` (`ChallengeDifficulty`), `available` (boolean), `locked_reason_code` (enum `grade_not_allowed`, `not_unlocked`, `school_disabled`, nullable), `time_limit_seconds`, `time_per_activity_seconds`, `lives`, `reward_multiplier`; `description`, `rules_summary` đổi sang `LocalizedText` (ADR-0056).
- **F075 — Cần sửa** `CollectionDetail.items[]`: thêm `how_to_obtain` (`LocalizedText`), `kind` (`ItemKind`); `name` → `LocalizedName`. `AchievementProgress`: thêm `hidden` (boolean, true khi `hidden_until_unlocked` và chưa mở). `ItemLedgerEntry.item_name`, `CurrencyLedgerEntry.source_label`, `XpLedgerEntry.source_label` → `LocalizedName` (client tạm dùng khóa `ui.journey.source.<source_code>` / `ui.inventory.reason.<reason_code>`).
- Delta đã có trong F075/F076 mà F069 phụ thuộc: `listInventory` query `kind`; `purchaseShopItem`, `startGameChallenge`, `nextChallengeActivity` khai báo `422`/`428`/`503`.
- Fixture `src/game/tests/fixtures/{progression,inventory,achievement,challenge}/*.json` sinh lại từ example; test contract parse mọi example ([GODOT_ARCHITECTURE §12](../../../docs/08-game-client/GODOT_ARCHITECTURE.md)).
- Kiểm (owner backend): `make specs-lint`, `make specs-breaking`, `make gen MOD=game`.
