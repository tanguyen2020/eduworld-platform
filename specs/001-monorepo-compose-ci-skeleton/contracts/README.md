# Contracts: F001 — Monorepo, Docker Compose local, CI GitHub Actions theo path, khung modular monolith

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| — | `/livez`, `/readyz`, `/healthz`, `/metrics`, `/version` trên cổng quản trị của `core-api`, `realtime`, `worker` | Không thuộc OpenAPI: cổng quản trị không qua APISIX ([API_DESIGN §9.17](../../../docs/07-backend/API_DESIGN.md), [DEPLOYMENT §14](../../../docs/11-operations/DEPLOYMENT.md)) | Không xác thực; chỉ mạng nội bộ | Không áp dụng |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có lời gọi facade giữa module trong F001. `tools/archgen` chỉ **đọc** `x-allowed-callers` của mọi operation trong `specs/openapi/modules/*.internal.v1.yaml` để sinh rule `make arch-check` (F001-AC-22) | `specs/openapi/modules/*.internal.v1.yaml` | Đọc, không sửa | Có sẵn |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Không phát/tiêu thụ; F001 chỉ khởi động NATS JetStream trong compose | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không dùng | — |

## 6. Delta cần thực hiện

- Không có delta contract. CI của F001 chạy các kiểm tra contract đã có (`scripts/specs/*.mjs`, `scripts/openapi/split-public.mjs --check`, `sync-platform-ops.mjs --check`) trong workflow `specs.yml`.
