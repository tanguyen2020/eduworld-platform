# Contracts: F050 — Khung web: pnpm workspace, `ops-ui`, theme từ token, `apiClient`, Storybook

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| mọi operation | Sinh type + metadata cho toàn bộ `/api/v1/*` | `specs/openapi/eduworld-api.v1.yaml` | Theo từng operation (backend kiểm) | Có sẵn |
| — (component) | `Problem`, `ProblemCode`, `TenantIdHeader`, `TenantIdHeaderOptional`, `IdempotencyKey`, `IfMatch`, `AcceptLanguage` | `specs/openapi/eduworld-api.v1.yaml` | — | Có sẵn |

F050 không gọi operation nghiệp vụ nào trong runtime; F051 trở đi gọi qua apiClient.

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Portal chỉ gọi API public, không gọi facade nội bộ giữa module (ADR-0074) | — | — | Không áp dụng |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Không dùng | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Helper rule form đọc schema OpenAPI component (không dùng `specs/schemas` ở F050) | — |

## 6. Delta cần thực hiện

- Không có delta contract. Việc bổ sung `x-internal: true` cho 20 operation `/admin/*` do F004 (T005) thực hiện; apiClient chọn gateway theo prefix nên không phụ thuộc delta đó.
