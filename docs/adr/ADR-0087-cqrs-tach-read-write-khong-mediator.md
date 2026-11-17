# ADR-0087 — CQRS tách rõ read/write context, không dùng mediator

> **Trạng thái:** Accepted · **Ngày:** 2026-11-16 · **OQ:** — · **Thay một phần:** ADR-0038 (mediator `pkg/mediator` và pipeline behavior) · **Tài liệu:** [BACKEND_DESIGN](../07-backend/BACKEND_DESIGN.md) · [AGENTS.md](../../AGENTS.md) R26 · [Chỉ mục ADR](README.md)

## Bối cảnh

[ADR-0038](ADR-0038-cqrs-clean-architecture-trong-moi-service.md) yêu cầu mọi command/query đi qua mediator `pkg/mediator` (kiểu MediatR) với pipeline logging, tracing, validation, authorization, idempotency, transaction. Review công nghệ ([EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) §4.5) đánh giá mediator dùng `any` là tư duy .NET, mất kiểu và khó đọc trace trong Go. Technical Architect chốt: CQRS **không cần mediator**, chỉ cần **chia rõ read/write context**.

## Quyết định

1. **Write context** (thay đổi trạng thái):
   - Handler ở `internal/application/command/`, mỗi use case một kiểu có phụ thuộc tường minh (repository của aggregate, `UnitOfWork`, clock, port gọi service khác).
   - Quy trình giữ nguyên R26: nạp aggregate → gọi phương thức domain → lưu bằng ent trong transaction → ghi outbox cùng transaction → trả ID/version.
   - Port ghi ở `internal/application/port/write/`; hiện thực ở `internal/infrastructure/persistence/write/` (primary DB).
2. **Read context** (chỉ đọc):
   - Handler ở `internal/application/query/`, đọc read model/projection qua port ở `internal/application/port/read/`; hiện thực ở `internal/infrastructure/persistence/read/` (được dùng replica, cache).
   - Không ghi DB, không phát event, không dùng `UnitOfWork`.
3. **Presentation** gọi handler trực tiếp qua interface có kiểu; không có bus/mediator chung, không dispatch theo `any`.
4. **Cross-cutting không qua mediator:**
   - HTTP middleware (`pkg/httpx`): xác thực, principal, tenant, idempotency (`Idempotency-Key`), Problem Details, recover, log, tracing.
   - Authorization: handler gọi tường minh `authz.Require(ctx, p, perm, resource)` (`pkg/authz`) ở đầu use case.
   - Validation: trong DTO/handler (validator sinh từ OpenAPI + invariant domain).
   - Transaction: `UnitOfWork` (`pkg/entx`) chỉ dùng trong command handler.
   - Tracing: span theo use case qua helper `pkg/observability`.
   - Được dùng decorator có kiểu (generic) cho từng handler khi thấy lặp, nhưng không bắt buộc và không phải bus chung.
5. **Cưỡng chế:** `go-arch-lint`/`depguard`: package `query` không import `port/write`, `UnitOfWork` hay publisher; `domain` không import application/infrastructure.
6. Bỏ `pkg/mediator` khỏi platform kit; các phần còn lại của ADR-0038 (Clean Architecture, CQRS, outbox, không event sourcing) giữ nguyên.

## Hệ quả

(+) Code Go idiomatic, có kiểu, trace dễ đọc; ranh giới read/write rõ và kiểm bằng linter. (−) Mỗi handler tự gọi authz/validation tường minh (có thể quên — giảm bằng test authorization bắt buộc và linter); phải sửa AGENTS R26, constitution điều II, BACKEND_DESIGN và mọi feature spec nhắc `pkg/mediator`.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-16 | Tạo ADR | Technical Architect |
| 2026-11-16 | Chuẩn hóa chữ ký `authz.Require(ctx, p, perm, resource)` theo F002 | Technical Architect |
