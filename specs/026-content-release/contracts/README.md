# Contracts: F026 — Phát hành nội dung: publish/rollback/unpublish, snapshot, manifest, ghim version, nơi sử dụng

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/modules/game.v1.yaml` (gộp trong `specs/openapi/eduworld-api.v1.yaml`). Prefix chung `/api/v1/cms/learning/content/{content_type}/{content_id}`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `publishContentVersion` | `POST …/versions/{version}:publish` | learning.v1 | `learning_content.publish` | Có sẵn |
| `rollbackContent` | `POST …:rollback` | learning.v1 | `content.release.rollback` | Có sẵn |
| `unpublishContent` | `POST …:unpublish` | learning.v1 | `learning_content.publish` | Có sẵn |
| `listContentVersions` | `GET …/versions` | learning.v1 | `learning_content.read` | Có sẵn |
| `diffContentVersions` | `GET …/versions/{version}:diff?with=` | learning.v1 | `learning_content.read` (+ `learning_content.review` để thấy đáp án) | Có sẵn |
| `listContentReleases` | `GET …/releases` | learning.v1 | `learning_content.read` | Có sẵn |
| `transitionContentVersion` (action `archive`) | `POST …/versions/{version}:transition` | learning.v1 | `learning_content.publish` | Có sẵn |
| `getContentVersion` | `GET …/versions/{version}` | learning.v1 | `learning_content.read` (đáp án chỉ khi `learning_content.review`) | Cần thêm |
| `getContentUsages` | `GET …/usages` | learning.v1 | `learning_content.read` | Cần thêm |
| `cancelScheduledPublication` | `POST …/releases/{release_id}:cancel` | learning.v1 | `learning_content.publish` | Cần thêm |
| `getContentManifest` | `GET /api/v1/game/content-manifest` | game.v1 | học sinh (`game_player`) | Có sẵn (F014) — `ContentManifestItem` Cần sửa |

Schema dùng: `PublishRequest`, `RollbackRequest`, `ReasonRequest`, `ContentRelease` (`status` scheduled/applied/cancelled), `ContentVersionHistory`, `ContentVersionSummary`, `ContentVersionDiff`, `ContentManifest`, `ContentManifestItem`. ProblemCode: `CONTENT_NOT_EDITABLE`, `CONTENT_VERSION_CONFLICT`, `CONTENT_NOT_PUBLISHED`, `ROLLBACK_TARGET_INVALID`, `INVALID_STATE_TRANSITION`, `VALIDATION_FAILED`, `PERMISSION_DENIED`, `RESOURCE_NOT_ASSIGNED`.

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có (manifest dựng từ event, R29) | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `content.manifest.updated` | server → client | Có sẵn (phát bởi F090/F014, ngoài phạm vi) |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `content.published` | learning / learning (cache), game (manifest), LMS, AUD | `specs/events/payloads/learning/content.published.v1.schema.json` | Cần sửa (thêm `min_client_version`, `snapshot_url`, `snapshot_sha256`, `cdn_eligible`) |
| `content.rolled_back` | learning / như trên | `…/content.rolled_back.v1.schema.json` | Cần thêm |
| `content.unpublished` | learning / như trên | `…/content.unpublished.v1.schema.json` | Cần thêm |
| `content.publish_scheduled` | learning / AUD | `…/content.publish_scheduled.v1.schema.json` | Cần thêm |
| `content.version_archived` | learning / cache, LMS | `…/content.version_archived.v1.schema.json` | Cần thêm |
| `content.manifest_updated` | game (consumer manifest của F026) → realtime (WS `content.manifest.updated`) | `specs/events/payloads/game/content.manifest_updated.v1.schema.json` | Cần thêm |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/question.schema.json` | Xác định trường đáp án cần loại khỏi snapshot | Có sẵn |
| `specs/schemas/question-set.schema.json`, `assessment-blueprint.schema.json`, `activities/*.schema.json` | Strip theo loại | Có sẵn |

## 6. Delta cần thực hiện

- Thêm `getContentVersion`, `getContentUsages` (schema `ContentUsage {ref_type, ref_id, ref_version_id?, title ≤255, pinned_sessions}`), `cancelScheduledPublication` vào `learning.v1.yaml` + `eduworld-api.v1.yaml` (task T004).
- `ContentManifestItem` (game.v1): thêm `activity_type` (`ActivityType`), `min_client_version` (`maxLength 32`) (task T004).
- `content.published.v1`: thêm `min_client_version`, `snapshot_url` (uri, `maxLength 2048`), `snapshot_sha256` (`maxLength 64`), `cdn_eligible` (boolean) (task T005).
- Payload mới: `content.rolled_back`, `content.unpublished`, `content.publish_scheduled`, `content.version_archived` v1 (task T005).
