# Contracts: F[NNN] — [Tên tính năng]

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

<!--
contracts/README.md KHÔNG chép contract. Contract nguồn ở specs/openapi, specs/asyncapi, specs/schemas, specs/events.
Liệt kê những gì feature dùng và phần cần thêm/sửa (delta). Delta phải có task ở Phase 2 của tasks.md.
-->

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `…` | `GET /api/v1/…` | `specs/openapi/eduworld-api.v1.yaml` | `…` | Có sẵn / Cần thêm / Cần sửa |

## 2. Facade giữa module (spec internal)

<!-- Gọi trong tiến trình (ADR-0074). `x-allowed-callers` ghi tên module (identity, learning, game, competition, chat), không dùng svc-*. -->

| operationId | Module gọi → module nhận | Spec | `x-allowed-callers` · `x-context-required` | Trạng thái |
|---|---|---|---|---|

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|

## 6. Delta cần thực hiện

- [Mô tả thay đổi contract; "Không có" nếu contract đã đủ]
