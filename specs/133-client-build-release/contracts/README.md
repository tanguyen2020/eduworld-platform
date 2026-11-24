# Contracts: F133 — Build và phát hành client Web, Android, iOS

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `getClientConfig` | `GET /api/v1/game/client-config` | `specs/openapi/eduworld-api.v1.yaml` | Không xác thực | Có sẵn (`min_client_version`, `latest_client_version`, `update_required`, `device_profiles`) |
| `gameLogin` | `POST /api/v1/auth/game/login` | như trên | Không xác thực | Có sẵn (`426 CLIENT_VERSION_UNSUPPORTED`) — smoke |
| `createGameSession` | `POST /api/v1/game/sessions` | như trên | `game_player` | Có sẵn — smoke |
| `getWorldManifest` · `getContentManifest` | `GET /api/v1/game/world/manifest` · `GET /api/v1/game/content-manifest` | như trên | `game_player` | Có sẵn — nguồn gói/media khi có phiên |
| `postTelemetry` | `POST /api/v1/game/telemetry` | như trên | `game_player` | **Cần sửa** — `TelemetryEvent.type` thêm `content_download` |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Client không gọi internal API | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không thay đổi (smoke chỉ vào Hub qua F092) | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Không | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/client-release-manifest.schema.json` | Manifest phát hành theo nền tảng/kênh (data-model §1) | Cần thêm |
| `specs/schemas/bundle-index.schema.json` | Chỉ mục gói vùng theo `client_version` (data-model §1) | Cần thêm |

## 6. Delta cần thực hiện

- **Cần sửa** `TelemetryEvent` trong `specs/openapi/eduworld-api.v1.yaml` (rồi `split-public.mjs`): thêm `content_download` vào enum `type`; thuộc tính `source` (enum `preinstalled`, `lan_cache`, `cdn`), `bytes` (integer ≥ 0), `bundle_id` (string, maxLength 128) — task T004; bên ingest F080 ánh xạ sang `eduworld_client_content_download_bytes_total{source}` ([MONITORING §3](../../../docs/11-operations/MONITORING.md)).
- **Cần thêm** `client-release-manifest.schema.json`, `bundle-index.schema.json` (mọi chuỗi có `maxLength`/`pattern`, ADR-0041) — task T005.
- ADR-0086: không delta API; hợp đồng nội bộ client `SecureStorage` (F060) — F133 thêm adapter iOS Keychain hiện thực cùng interface.
