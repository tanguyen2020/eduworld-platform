# ADR-0040 — Gọi service-to-service bằng typed REST client sinh từ OpenAPI (tương đương Refit)

> **Trạng thái:** Accepted · một phần superseded by [ADR-0074](ADR-0074-modular-monolith-co-ranh-gioi-cuong-che.md) · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [API_DESIGN](../07-backend/API_DESIGN.md), [BACKEND_DESIGN](../07-backend/BACKEND_DESIGN.md), [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần được thay bởi ADR-0074.** Không còn hiệu lực: HTTP nội bộ `/internal/v1`, client `oapi-codegen` gọi qua mạng, token `svc-*` (trừ `svc-identity` gọi Keycloak Admin), resilience và contract test HTTP giữa module. **Còn hiệu lực:** spec internal là hợp đồng facade (đổi thư mục sang `specs/openapi/modules/`), `x-allowed-callers`, sinh code có kiểu, adapter sau port, chỉ gọi đồng bộ khi cần kết quả ngay, phản ứng phụ qua event.

## Bối cảnh

Stakeholder yêu cầu dùng Refit khi gọi service to service; vì giữ Golang nên dùng cơ chế tương đương.

## Quyết định

- Mỗi service công bố **internal API** bằng OpenAPI (`specs/openapi/services/<service>.internal.v1.yaml`, prefix `/internal/v1/...`, chỉ mạng private, không qua APISIX public). Mỗi operation có `x-allowed-callers`.
- Bên gọi dùng **client type-safe sinh bằng `oapi-codegen`**, bọc trong adapter ở tầng infrastructure, expose qua port của application.
- **Resilience chuẩn:** timeout theo endpoint, retry có backoff + jitter chỉ cho thao tác idempotent, circuit breaker, bulkhead bằng `failsafe-go` [Proposed; tham số trong API_DESIGN].
- **Xác thực:** mỗi service một Keycloak confidential client (`svc-identity`, `svc-learning`, `svc-game`, `svc-competition`, `svc-chat`, `svc-realtime`), token client credentials (audience `svc-<callee>-api`, scope `internal-<callee>`), cache đến gần hết hạn; bên nhận kiểm tra `azp` theo allowlist. mTLS tầng mạng [Proposed].
- **Truyền ngữ cảnh:** `traceparent`, `X-Tenant-Id` (bên nhận vẫn kiểm tra quyền), `X-Request-Id`, `Idempotency-Key`, `X-On-Behalf-Of` (user gốc) khi cần audit.
- Chỉ gọi đồng bộ khi cần kết quả ngay (vd game → learning bắt đầu attempt; competition → learning lấy câu hỏi/chấm; mọi service → identity `principals:resolve`). Phản ứng phụ dùng event NATS (ADR-0042).
- Contract test consumer-driven giữa các cặp service trong CI.

## Hệ quả

(+) Gọi service an toàn kiểu, chính sách lỗi thống nhất. (−) Thêm spec internal phải duy trì.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Accepted | Stakeholder |
| 2026-11-15 | Thêm `x-allowed-callers`, audience/scope token | API_DESIGN, KEYCLOAK_SSO |
| 2026-11-17 | Một phần superseded by ADR-0074: gọi trong tiến trình qua facade sinh từ spec internal | Technical Architect |
