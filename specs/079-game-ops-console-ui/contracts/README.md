# Contracts: F079 — Workspace A — Game Operations Console UI (world, NPC, quest builder, reward, item, event, season)

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/eduworld-api.v1.yaml` (`x-internal: true`, `keycloakAdminBearer`, gateway nội bộ + role platform). Permission theo `config_type` ([API_DESIGN §9.13b](../../../docs/07-backend/API_DESIGN.md)).

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listGameConfigs` · `createGameConfig` | `GET/POST /api/v1/game-admin/{config_type}` | eduworld-api (game) | `game.<loại>.write` | Có sẵn (`listGameConfigs` Cần sửa) |
| `getGameConfigVersion` · `updateGameConfigDraft` | `GET/PATCH /api/v1/game-admin/{config_type}/{config_id}/versions/{version}` | eduworld-api | `game.<loại>.write` | Có sẵn (Cần sửa `GameConfigVersion`) |
| `transitionGameConfigVersion` | `POST …/versions/{version}:transition` | eduworld-api | `game.<loại>.write` / `game.content.review` | Có sẵn |
| `listGameConfigVersions` · `diffGameConfigVersions` | `GET …/{config_id}/versions` · `GET …/versions/{version}:diff` | eduworld-api | `game.<loại>.write` | Có sẵn |
| `publishGameConfigVersion` | `POST …/versions/{version}:publish` | eduworld-api | `game.content.publish` | Có sẵn |
| `rollbackGameConfig` · `unpublishGameConfig` · `listGameConfigReleases` | `POST …/{config_id}:rollback|:unpublish` · `GET …/releases` | eduworld-api | `content.release.rollback` / `game.content.publish` | Có sẵn |
| `createGamePreviewSession` | `POST /api/v1/game-admin/preview/sessions` | eduworld-api | `game.<loại>.write` | Có sẵn |
| `presignGameMediaUpload` · `getGameMediaAsset` | `POST /api/v1/game-media:presign-upload` · `GET /api/v1/game-media/{asset_id}` | eduworld-api | `game.<loại>.write` | Có sẵn |
| `simulateGameConfig` | `POST /api/v1/game-admin/{config_type}/{config_id}/versions/{version}:simulate` | eduworld-api | `game.progression.write` / `game.season_track.write` | Cần thêm |
| `getGameClientManifest` | `GET /api/v1/game-admin/client-manifests/current` | eduworld-api | bất kỳ `game.*.write` | Cần thêm |
| `getGameConfigSchema` | `GET /api/v1/game-admin/{config_type}/schema` | eduworld-api | bất kỳ `game.*.write` | Cần thêm |
| `getMyPermissions` | `GET /api/v1/me/permissions` | eduworld-api | đã đăng nhập | Có sẵn |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có (frontend) | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Portal không phát/tiêu thụ | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `quest`, `npc-dialogue`, `minigame-config`, `reward-rule`, `progression-curve`, `achievement`, `challenge`, `boss`, `shop-item`, `event`, `season-track` (`specs/schemas/*.schema.json`) | Sinh form + validate client | Có sẵn |
| `world-areas`, `story-chapters`, `items`, `collections`, `currencies`, `event-templates`, `progression-levels` | Sinh form | Cần thêm (owner F071/F074/F075/F077 — lấy qua `getGameConfigSchema`) |

## 6. Delta cần thực hiện

Task Phase 2: T004–T006 (owner backend ghi trong ngoặc; F079 tạo contract nếu owner chưa làm).

- **Cần sửa** `GameConfigVersion` thêm `media` {`thumbnail_url` format uri maxLength 2048, `turntable_url` format uri maxLength 2048, `render_status` enum `pending|ready|failed|not_applicable`, `rendered_at`} (owner F078, FR-CMS-017); `listGameConfigs` thêm query `q` (maxLength 100), `from`/`to` (date, cho `events`), `sort` enum `updated_at|title`.
- **Cần thêm** `simulateGameConfig` body `SimulationRequest` {`minutes_per_day` integer 1–240, `days_per_week` 1–7, `weeks` 1–52} → `SimulationResult` {`points[]` {`week`, `level` hoặc `tier`, `xp_total`}, maxItems 52}; chỉ cho `progression-curves`, `season-tracks` (khác → `422 VALIDATION_FAILED`) (owner F074/F077).
- **Cần thêm** `getGameClientManifest` → `GameClientManifest` {`client_version` maxLength 32, `areas[]` {`area_id` maxLength 64, `name` LocalizedName}, `npcs[]` {`npc_id` maxLength 64}, `scene_refs[]` maxLength 128, `interactables[]` maxLength 64} (owner F078, `game.client_manifests`).
- **Cần thêm** `getGameConfigSchema` → JSON Schema của payload theo `config_type` (`$id`, version) (owner F078).
- **Tài liệu**: đã thống nhất (RB-309) — `game.progression.write` (`progression-curves`, `progression-levels`), `game.shop.write` (`shop-items`), `game.collection.write` (`collections`) có trong catalog ADMIN_PORTAL §2.2; catalog code do F078 task T039.
