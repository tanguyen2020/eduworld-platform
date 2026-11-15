# ADR-0025 — Thư viện nền tảng cho Go backend

> **Trạng thái:** Proposed · một phần superseded by [ADR-0039](ADR-0039-orm-ent-cho-go-tuong-duong-entity-framework-migration-bang-a.md) và [ADR-0040](ADR-0040-goi-service-to-service-bang-typed-rest-client-sinh-tu-openap.md) · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [BACKEND_DESIGN](../07-backend/BACKEND_DESIGN.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần đã được thay.** Không còn hiệu lực: "pgx + sqlc, không dùng ORM, migration goose" → nay **ent + Atlas** (ADR-0039); gọi service-to-service bằng typed client sinh từ OpenAPI (ADR-0040). **Còn hiệu lực:** các thư viện còn lại dưới đây.

## Bối cảnh

Cần bộ thư viện thống nhất cho 6 service Go.

## Quyết định

- Go stable mới nhất (≥ 1.24); HTTP router `chi` (tương thích `net/http`); OpenAPI `oapi-codegen` (strict server + client); config env + `koanf`; log `log/slog`; WebSocket `coder/websocket`; Valkey client `valkey-io/valkey-go`; NATS `nats.go`; JWT/OIDC `coreos/go-oidc` + `go-jose`; test `testing` + `testify` + `testcontainers-go`; lint `golangci-lint` (gồm `depguard`).
- ORM `ent` + driver `pgx/v5`, migration Atlas (ADR-0039); resilience `failsafe-go` (ADR-0040).
- *(Gốc, không còn hiệu lực)* `sqlc` + `goose`, "không dùng ORM".

## Hệ quả

(+) Thư viện phổ biến, tương thích `net/http`. (−) Phụ thuộc vòng đời nhiều thư viện bên ngoài; ghim phiên bản trong `go.mod`.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Phase 0 |
| 2026-11-15 | ORM/migration thay bởi ent + Atlas; typed client | ADR-0039, ADR-0040 |
