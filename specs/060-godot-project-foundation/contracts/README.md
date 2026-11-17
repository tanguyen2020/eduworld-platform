# Contracts: F060 — Khung project Godot: autoload, theme sinh từ token, i18n vi/en, CI, Prism mock

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| mọi operation (mock) | Prism phục vụ toàn bộ `/api/v1/*` bằng `examples` | `specs/openapi/eduworld-api.v1.yaml` | — (mock) | Có sẵn |
| — (component) | `ProblemCode` → khóa dịch `ui.error.<CODE>` | `specs/openapi/eduworld-api.v1.yaml` | — | Có sẵn |
| `getClientConfig` | `GET /api/v1/game/client-config` | `specs/openapi/eduworld-api.v1.yaml` | Không token | Có sẵn — fixture contract test (gọi thật ở F061) |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Game không gọi internal API | — | — | Không áp dụng |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Chưa dùng (F092) | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Game không kết nối NATS (AGENTS R25) | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/*.schema.json` + `specs/examples/*.json` | Fixture contract test parse model GDScript (F060-AC-22) | Có sẵn |

## 6. Delta cần thực hiện

- Không có delta contract API. Hợp đồng nội bộ client (không phải OpenAPI): interface `SecureStorage` (`is_available`/`save`/`load`/`erase`) dùng chung với F061, F133 — mô tả ở [plan.md](../plan.md) và [GODOT_ARCHITECTURE §5.2](../../../docs/08-game-client/GODOT_ARCHITECTURE.md#52-lưu-phiên-bằng-securestorage-adr-0086) (ADR-0086).
