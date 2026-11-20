# ADR-0002 — Monorepo

> **Trạng thái:** Accepted · cấu trúc thư mục superseded by [ADR-0074](ADR-0074-modular-monolith-co-ranh-gioi-cuong-che.md) và [ADR-0092](ADR-0092-to-chuc-ma-nguon-trong-src.md) · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [SYSTEM_ARCHITECTURE](../06-architecture/SYSTEM_ARCHITECTURE.md), [CONTRIBUTING](../../CONTRIBUTING.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Cấu trúc thư mục đã được thay** bởi [ADR-0074](ADR-0074-modular-monolith-co-ranh-gioi-cuong-che.md) (modular monolith, bỏ `services/` và `go.work`) và [ADR-0092](ADR-0092-to-chuc-ma-nguon-trong-src.md) (mã nguồn trong `src/backend`, `src/cms/frontend`, `src/game`). **Còn hiệu lực:** một repository chứa mã, contract, design token, deploy và tài liệu; `specs/` là nguồn sự thật để sinh code.

## Bối cảnh

Contract (specs), backend, game client và portal thay đổi cùng nhau; cần một nguồn sự thật chung và một quy trình review.

## Quyết định

- Một repository chứa: `services/` (6 microservice Go, ADR-0036) và `pkg/` (platform kit dùng chung) trong một `go.work`; `godot/`; `web/` (Admin Portal, ADR-0022); `design/` (token giao diện, ADR-0047); `specs/`; `deploy/`; `scripts/`; `docs/`.
- `specs/` là nguồn sự thật để sinh code cho Go (oapi-codegen), Admin Portal (openapi-typescript) và kiểm tra contract của Godot.

## Hệ quả

(+) Thay đổi contract + server + client trong một merge request. (−) CI cần job theo đường dẫn (path-based) để không build Godot/portal khi chỉ sửa một service Go; release và tag theo từng service.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Phase 0 |
| 2026-11-15 | Cập nhật cấu trúc thư mục `backend/` → `services/` + `pkg/` | ADR-0036 |
| 2026-11-20 | Owner chốt (Accepted); danh sách thư mục (`services/`, `go.work`, `godot/`, `web/`) ghi nhận đã thay bởi ADR-0074 và ADR-0092, không đổi ý nghĩa quyết định monorepo | Owner |
