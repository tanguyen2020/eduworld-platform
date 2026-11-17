# ADR-0038 — CQRS + Clean Architecture trong mỗi service

> **Trạng thái:** Accepted · một phần superseded by ADR-0087 · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [BACKEND_DESIGN](../07-backend/BACKEND_DESIGN.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần đã được thay bởi [ADR-0087](ADR-0087-cqrs-tach-read-write-khong-mediator.md):** không dùng mediator `pkg/mediator`/pipeline behavior; CQRS tách read/write context, cross-cutting qua HTTP middleware và gọi tường minh. **Còn hiệu lực:** Clean Architecture, CQRS, outbox, không event sourcing.

## Bối cảnh

Stakeholder yêu cầu mô hình CQRS và Clean Architecture.

## Quyết định

1. **Clean Architecture** — mỗi service gồm 4 tầng, phụ thuộc chỉ hướng vào trong:

   | Tầng | Thư mục | Chứa | Phụ thuộc vào |
   |---|---|---|---|
   | Domain | `internal/domain` | Entity, value object, aggregate, domain event, quy tắc nghiệp vụ thuần | Không gì |
   | Application | `internal/application/{command,query}` | Command/Query + handler, port (repository, client, publisher), DTO | Domain |
   | Infrastructure | `internal/infrastructure` | ent repository, NATS/Valkey adapter, typed HTTP client (ADR-0040), Keycloak adapter, storage S3 | Application, Domain |
   | Presentation | `internal/presentation/{http,ws,consumer}` | HTTP handler (oapi-codegen), consumer NATS, job worker | Application |

   Ranh giới kiểm tra bằng linter (`depguard`/`go-arch-lint`) trong CI.
2. **CQRS:**
   - **Command** đi qua aggregate, ghi bằng ent trong transaction, cùng transaction ghi outbox; trả về ID/version.
   - **Query** chỉ đọc, không qua aggregate, đọc read model/projection tối ưu cho màn hình (ent query hoặc SQL tường minh), có thể dùng replica và cache Valkey.
   - **Mediator nội bộ** nhẹ (`CommandHandler[C, R]`, `QueryHandler[Q, R]`) với pipeline behavior: logging, tracing, validation, authorization (permission + tenant), idempotency, transaction.
   - Read model cập nhật đồng bộ khi đơn giản hoặc bất đồng bộ qua event cho dashboard/leaderboard/analytics (eventual consistency, ghi rõ trong tài liệu màn hình).
   - **Không** dùng event sourcing.

## Hệ quả

(+) Tách rõ ghi/đọc, dễ test theo tầng. (−) Nhiều lớp code hơn cho nghiệp vụ đơn giản.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Accepted | Stakeholder |
| 2026-11-16 | Một phần superseded by ADR-0087 | Technical Architect |
