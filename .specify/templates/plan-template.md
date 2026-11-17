# Implementation Plan: F[NNN] — [Tên tính năng]

**Branch**: `[NNN-slug]` · **Date**: [YYYY-MM-DD] · **Spec**: [spec.md](spec.md)

<!--
Chuẩn Spec Kit (ADR-0082). plan.md trả lời LÀM THẾ NÀO; tạo kèm research.md (nếu có quyết định cần nghiên cứu), data-model.md, contracts/README.md, quickstart.md.
Không định nghĩa lại quy tắc/tham số; link tài liệu sở hữu. Theo ADR hiện hành, không theo ADR Proposed.
-->

## Summary

[Yêu cầu chính + cách tiếp cận kỹ thuật trong 3–6 câu.]

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 / TypeScript (React, Vite) / GDScript (Godot 4.x) |
| Primary Dependencies | [chi, ent, oapi-codegen, nats.go… / Ant Design Pro, TanStack Query… / GdUnit4…] |
| Storage | [PostgreSQL `eduworld_<module>` / Valkey (tạm) / floci S3 / không có] |
| Testing | [go test + testcontainers / Vitest + Playwright / GdUnit4] |
| Target Platform | [Linux container / trình duyệt / Windows, Android] |
| Project Type | [backend module (`modules/<module>`, binary `core-api`/`realtime`/`worker` profile …) / web app / game client / platform kit] |
| Performance Goals | [link NFR/PERFORMANCE_BUDGET] |
| Constraints | [link AGENTS R…, ADR…] |
| Scale/Scope | [link CONFIG_DEFAULTS / NFR-SCAL] |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ / ⚠️ | |
| II Ranh giới module, Clean Architecture + CQRS | | |
| III Cô lập tenant, an toàn dữ liệu | | |
| IV Server-authoritative | | |
| V Xác thực, phân quyền | | |
| VI An toàn trẻ em | | |
| VII Một nguồn sự thật, cấu hình | | |
| VIII UI tách logic, token | | |
| IX Test có bằng chứng | | |

## Project Structure

### Documentation (feature này)

```text
specs/[NNN-slug]/
├── spec.md
├── plan.md              # file này
├── research.md          # Phase 0 (nếu có)
├── data-model.md        # Phase 1
├── quickstart.md        # Phase 1
├── contracts/README.md  # Phase 1
├── checklists/requirements.md
└── tasks.md             # /speckit.tasks
```

### Source Code (repository)

```text
[Cây thư mục thật sẽ tạo/sửa, ví dụ:]
modules/<module>/
├── module.go                              # Register: route, consumer, job, facade (BACKEND_DESIGN §3.3)
├── api/                                   # facade sinh từ specs/openapi/modules/<module>.internal.v1.yaml (không sửa tay)
├── ent/schema/
├── migrations/
├── internal/domain/<aggregate>/
├── internal/application/command/          # write context
├── internal/application/query/            # read context
├── internal/application/port/{write,read}/
├── internal/infrastructure/persistence/{write,read}/
├── internal/infrastructure/{modules,messaging}/   # modules/: adapter gọi facade module khác qua pkg/modcall
├── internal/presentation/{http,facade,natscmd,consumer,job}/
└── tests/{unit,integration,facade}/
```

**Structure Decision**: [Vì sao chọn cấu trúc trên]

## Phase 0: Research

[Quyết định kỹ thuật cần làm rõ → research.md (Decision / Rationale / Alternatives). "Không có" nếu ADR/tài liệu đã đủ.]

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** (cách chạy và kiểm) → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `…` | Aggregate, invariant |
| application | Command `…` · Query `…` | Permission `…` |
| infrastructure | Repository, adapter facade module khác, publisher, adapter tích hợp ngoài | |
| presentation | Handler `operationId` · facade · consumer · job | Binary/profile chạy: `core-api` / `worker-events` / `worker-chat` / `worker-jobs` / `realtime` |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F… · Facade gọi: `<module>.<operationId>` (R29, `x-allowed-callers`) · Event phát/tiêu thụ: `…`

### Bảo mật, tenant, idempotency, audit

- Permission `…` · TenantScope/CrossTenantScope · Idempotency-Key · audit action `…`

### Quan sát

- Log (không PII), metric `eduworld_…`, span use case `<module>.command.<Name>` và span facade …

## Complexity Tracking

> Chỉ điền khi Constitution Check có ⚠️.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
