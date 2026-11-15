# ADR-0002 — Monorepo

> **Trạng thái:** Proposed · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [SYSTEM_ARCHITECTURE](../06-architecture/SYSTEM_ARCHITECTURE.md), [CONTRIBUTING](../../CONTRIBUTING.md) · [Chỉ mục ADR](README.md)

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
