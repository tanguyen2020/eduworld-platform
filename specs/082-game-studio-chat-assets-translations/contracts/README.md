# Contracts: F082 — Game Studio — quản lý sticker/GIF và dịch nội dung game (`/game-admin/translations*`)

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/eduworld-api.v1.yaml` (`x-internal: true`, `keycloakAdminBearer`, gateway nội bộ + MFA); `config_type` `sticker-packs`/`gifs` định tuyến tới module chat (`specs/openapi/modules/chat.v1.yaml`, operationId `*_chat`), loại còn lại tới module game (`game.v1.yaml`).

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listGameConfigs` · `createGameConfig` | `GET/POST /api/v1/game-admin/{config_type}` | eduworld-api (game, chat) | `game.chat_assets.write` / `game.<loại>.write` | Có sẵn (Cần sửa: lọc `translation_status`) |
| `getGameConfigVersion` · `updateGameConfigDraft` | `GET/PATCH …/{config_id}/versions/{version}` (`If-Match`) | eduworld-api | như trên | Có sẵn (Cần sửa `GameConfigVersion`) |
| `transitionGameConfigVersion` · `publishGameConfigVersion` | `POST …/versions/{version}:transition` · `:publish` | eduworld-api | ghi loại / `game.content.review` / `game.content.publish` | Có sẵn |
| `listGameConfigVersions` · `diffGameConfigVersions` · `listGameConfigReleases` | `GET …/versions`, `…:diff`, `…/releases` | eduworld-api | ghi loại | Có sẵn |
| `rollbackGameConfig` · `unpublishGameConfig` | `POST …/{config_id}:rollback` · `:unpublish` | eduworld-api | `content.release.rollback` / `game.content.publish` | Có sẵn |
| `presignChatAssetUpload` | `POST /api/v1/game-admin/{config_type}/{config_id}/assets:presign` (`sticker-packs`/`gifs`) | chat.v1.yaml → eduworld-api | `game.chat_assets.write` | Cần thêm (owner F118) |
| `presignGameMediaUpload` · `getGameMediaAsset` | `POST /api/v1/game-media:presign-upload` · `GET /api/v1/game-media/{asset_id}` | eduworld-api | `game.content.translate` (kind `document`) / ghi loại | Có sẵn (Cần sửa mô tả: chấp nhận `document` XLIFF/CSV cho dịch) |
| `exportGameTranslations` | `POST /api/v1/game-admin/translations:export` (`Idempotency-Key`) | eduworld-api (game) | `game.content.translate` | Có sẵn (Cần sửa: khai báo `409`, `428`) |
| `importGameTranslations` | `POST /api/v1/game-admin/translations:import` (`Idempotency-Key`) | eduworld-api (game) | `game.content.translate` | Có sẵn (Cần sửa: `409`, `428`) |
| `getGameTranslationJob` | `GET /api/v1/game-admin/translations/jobs/{job_id}` | eduworld-api (game) | `game.content.translate` | Có sẵn |
| `getMyPermissions` | `GET /api/v1/me/permissions` | eduworld-api | đã đăng nhập | Có sẵn |

Schema dùng: `GameConfigType`, `GameConfigInput`, `GameConfigVersion`, `ContentStatus`, `PublishRequest`, `RollbackRequest`, `MediaPresignRequest`, `MediaPresignResponse`, `MediaKind`, `TranslationExportRequest`, `TranslationImportRequest`, `TranslationJob`, `ContentLanguage`, `LocalizedName`, `Problem` (`VALIDATION_FAILED`, `TRANSLATION_INCOMPLETE`, `MEDIA_TOO_LARGE`, `MEDIA_TYPE_NOT_ALLOWED`, `CONTENT_VERSION_CONFLICT`, `VERSION_MISMATCH`, `PERMISSION_DENIED`, `MFA_REQUIRED`, `NOT_FOUND`, `IDEMPOTENCY_KEY_REUSED`, `PRECONDITION_REQUIRED`).

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có (portal gọi qua APISIX nội bộ) | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng (portal poll job) | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `audit.recorded` | game → identity (audit) | theo F015 | Có sẵn (F015) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `quest`, `npc-dialogue`, `minigame-config`, `achievement`, `challenge`, `boss`, `progression-curve`, `event`, `season-track`, `shop-item`, `reward-rule` (`specs/schemas/*.schema.json`, `$defs/localized_text`) | Xác định JSON pointer văn bản game (research R1) | Có sẵn |
| `specs/schemas/sticker-pack.schema.json` | Form + validate payload `sticker-packs` | Cần thêm (owner F118) |
| `specs/schemas/gif.schema.json` | Form + validate payload `gifs` | Cần thêm (owner F118) |

## 6. Delta cần thực hiện

Task Phase 2: T004–T007 (owner trong ngoặc; F082 tạo contract nếu owner chưa làm).

- **Cần sửa** `GameConfigVersion` (module game; module chat trả `null`): thêm `translation_status` ($ref `TranslationStatus`, readOnly), `missing_fields[]` (string maxLength 512, maxItems 500), `outdated_fields[]` maxItems 500 {`pointer` string maxLength 512, `translated_in_version` integer ≥ 1}. `listGameConfigs` thêm query `translation_status` ($ref `TranslationStatus`) (F082).
- **Cần sửa** `exportGameTranslations`/`importGameTranslations`: khai báo `409 IDEMPOTENCY_KEY_REUSED`, `428 PRECONDITION_REQUIRED`; `TranslationExportRequest.content_types` mô tả "chỉ `config_type` của module game" (F082). `TranslationJob.report.errors[].message` giữ maxLength 500; thêm `code` enum `key_not_found|stale_version|version_in_review|unsupported_type|too_long|invalid_language` (F082).
- **Cần sửa** mô tả `presignGameMediaUpload`: cho `kind = document` với `mime_type` `application/xliff+xml`/`text/csv` khi người gọi có `game.content.translate`; file không gắn `media_usages`, không lên CDN (F082 + F078).
- **Cần thêm** `presignChatAssetUpload_chat` (`POST /game-admin/{config_type}/{config_id}/assets:presign`, `config_type` enum `sticker-packs|gifs`, `Idempotency-Key`, body `MediaPresignRequest` (`kind = image`)) → `MediaPresignResponse`; `413`, `415`, `422` (owner F118).
- **Cần sửa** response version `sticker-packs`/`gifs`: payload trả `stickers[].preview_url`, `preview_url`/`url` (URL ký ngắn hạn, format uri maxLength 2048) để portal xem Draft (owner F118).
- **Cần thêm** `sticker-pack.schema.json` {`code` maxLength 64, `name` LocalizedName (maxLength 128 theo DDL), `unlock_rule` {`kind` enum `default|reward_rule|item`, `ref_id` uuid}, `stickers[]` maxItems 48 {`sticker_id`, `asset_id`, `alt_text` {`vi`,`en`} maxLength 1000, `sort_order`}} và `gif.schema.json` {`asset_id`, `tags[]` maxItems 20 {`tag` maxLength 64, `locale` enum `vi|en`}} (owner F118; maxItems [Confirmed]).
- **Tài liệu**: DATABASE_DESIGN thêm `game.config_translation_states`, `game.translation_jobs`; CONFIG_DEFAULTS đề xuất tham số TTL link/thời gian giữ file dịch; API_DESIGN ghi `presignChatAssetUpload_chat`.
- Kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`, `make gen MOD=chat`, `pnpm gen:api`.
