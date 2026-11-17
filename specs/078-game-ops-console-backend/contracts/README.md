# Contracts: F078 — Game Operations Console backend: workflow nội dung game, thumbnail 3D, catalog cho nhà trường

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/eduworld-api.v1.yaml` → `specs/openapi/modules/game.v1.yaml`. Mọi operation: `x-internal: true`, `keycloakAdminBearer`, gateway nội bộ; permission theo bảng ở [plan.md](../plan.md).

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listGameConfigs_game` | `GET /api/v1/game-admin/{config_type}?status=` | game.v1.yaml | đọc của loại | Có sẵn |
| `createGameConfig_game` | `POST /api/v1/game-admin/{config_type}` (`Idempotency-Key`, `GameConfigInput`) | game.v1.yaml | ghi của loại | Có sẵn |
| `getGameConfigVersion_game` | `GET /api/v1/game-admin/{config_type}/{config_id}/versions/{version}` | game.v1.yaml | đọc | Có sẵn |
| `updateGameConfigDraft_game` | `PATCH …/versions/{version}` (`If-Match`) | game.v1.yaml | ghi | Có sẵn |
| `createGameConfigVersion_game` | `POST /api/v1/game-admin/{config_type}/{config_id}/versions` (`based_on_version`, `Idempotency-Key`) | game.v1.yaml | ghi | Cần thêm |
| `transitionGameConfigVersion_game` | `POST …/versions/{version}:transition` (`submit_review`/`approve`/`reject`/`archive`) | game.v1.yaml | ghi / `game.content.review` | Có sẵn |
| `listGameConfigVersions_game` | `GET …/{config_id}/versions` | game.v1.yaml | đọc | Có sẵn |
| `diffGameConfigVersions_game` | `GET …/versions/{version}:diff?with=` | game.v1.yaml | đọc | Có sẵn |
| `publishGameConfigVersion_game` | `POST …/versions/{version}:publish` (`PublishRequest`) → `200`/`202 ContentRelease` | game.v1.yaml | `game.content.publish` | Có sẵn |
| `cancelScheduledGameConfigPublish_game` | `POST /api/v1/game-admin/{config_type}/{config_id}/releases/{release_id}:cancel` | game.v1.yaml | `game.content.publish` | Cần thêm |
| `rollbackGameConfig_game` | `POST …/{config_id}:rollback` (`RollbackRequest`) | game.v1.yaml | `game.content.publish` | Có sẵn |
| `unpublishGameConfig_game` | `POST …/{config_id}:unpublish` (`ReasonRequest`) → kèm `usages[]` | game.v1.yaml | `game.content.publish` | Cần sửa (response thêm `usages[]`) |
| `listGameConfigReleases_game` | `GET …/{config_id}/releases` | game.v1.yaml | đọc | Có sẵn |
| `createGamePreviewSession` | `POST /api/v1/game-admin/preview/sessions` | game.v1.yaml | đọc của loại | Có sẵn |
| `presignGameMediaUpload` | `POST /api/v1/game-media:presign-upload` | game.v1.yaml | ghi `game.*` | Có sẵn |
| `getGameMediaAsset` | `GET /api/v1/game-media/{asset_id}` | game.v1.yaml | đọc `game.*` | Có sẵn |
| `listGameActivities` | `GET /api/v1/lms/catalog/game-activities` | learning.v1.yaml | (F041) | Có sẵn — dữ liệu từ F078 |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `applyTenantAssignment` | identity → game (facade `game`, input: `action` assign/unassign, `resource_type`, `resource_id`, `tenant_ids[]` ≤ 500, `Idempotency-Key`) | `specs/openapi/modules/game.internal.v1.yaml` | `[identity]` | Cần thêm (phối hợp F045) |
| `listAssignedGameActivities` | learning → game (facade `game`, input: `tenant_id`, `cursor`) | game.internal.v1.yaml | `[learning]` | Cần thêm |
| `executeTenantCloneStep`, `listTrash`, `restoreTrashItem` | identity → game | game.internal.v1.yaml | như spec | Có sẵn (F045; nội dung game platform không clone dữ liệu học sinh) |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `content.manifest.updated` | server → học sinh online của tenant | Có sẵn (giao bởi F090) |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `content.version_submitted`, `content.version_approved`, `content.version_rejected`, `content.version_archived` | game → NTF, AUD | `specs/events/payloads/game/` | Cần thêm |
| `content.published` | game → snapshot/manifest, module loại, learning (catalog, F041), competition (`world_area`, F091), AUD | `specs/events/payloads/game/content.published.v1.schema.json` (khối `catalog`) | Cần sửa (owner F071; F078 thêm khối `catalog`) |
| `content.rolled_back`, `content.unpublished`, `content.publish_scheduled` | game → snapshot/manifest, module loại, learning (catalog, F041), AUD | `specs/events/payloads/game/content.rolled_back.v1.schema.json` (+ `unpublished`, `publish_scheduled`) | Cần thêm |
| `content.manifest_updated` | game → realtime | `specs/events/payloads/game/content.manifest_updated.v1.schema.json` | Cần thêm (owner F026) |
| `tenant_data.assigned`, `tenant_data.unassigned` | identity (F045) → learning (catalog), AUD — module game không phát, nhận gán qua facade `applyTenantAssignment` | `specs/events/payloads/identity/tenant_data.assigned.v1.schema.json` (+ `unassigned`) | Cần thêm (owner F045) |
| `media.uploaded`, `media.processed`, `media.rejected` | game ↔ media worker | `specs/events/payloads/game/` | Cần thêm |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `quest`, `minigame-config`, `npc-dialogue`, `reward-rule`, `progression-curve`, `challenge`, `boss`, `achievement`, `shop-item`, `event`, `season-track` (`specs/schemas/*.schema.json`) | Validate payload theo `config_type` | Có sẵn (delta riêng ở F074–F077) |
| `item`, `collection`, `event-template` | `items`, `collections`, `event-templates` | Cần thêm (F075, F077) |

## 6. Delta cần thực hiện

- OpenAPI public: thêm `createGameConfigVersion_game`, `cancelScheduledGameConfigPublish_game`; `GameConfigVersion` thêm `thumbnail_url`, `turntable_url` (`maxLength: 2048`, `format: uri`), `based_on_version`, `author_id`; `unpublishGameConfig_game` response thêm `usages[]`; `MediaKind` thêm `render_thumbnail`, `render_turntable`; `createGameConfig_game`/`transition…` khai báo `409`, `428`.
- `game.internal.v1.yaml`: thêm `applyTenantAssignment` (`[identity]`), `listAssignedGameActivities` (`[learning]`), schema `GameActivityCatalogEntry` khớp `GameActivityCatalogItem` của learning.v1.
- Payload event `content.version_*`, `content.rolled_back`, `content.unpublished`, `content.publish_scheduled`, `media.*` cho game (§4); khối `catalog` cho `content.published` của game (schema owner F071; tùy chọn, bắt buộc với `quest`/`minigame_config`). `content.manifest_updated` dùng schema của F026; `tenant_data.*` do module identity phát (F045).
- APISIX: route `/api/v1/game-admin/*`, `/api/v1/game-media*` chỉ trên gateway nội bộ (F004 cấu hình; F078 thêm test).
- Kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`, `make test-contract`.
