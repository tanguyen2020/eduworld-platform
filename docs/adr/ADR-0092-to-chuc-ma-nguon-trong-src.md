# ADR-0092 — Tổ chức mã nguồn trong `src/`: backend dùng chung, cms/frontend, game

> **Trạng thái:** Accepted · một phần supersedes ADR-0074 (mục 1, đường dẫn mã) · **Ngày:** 2026-11-17 · **OQ:** — · **Tài liệu:** [BACKEND_DESIGN](../07-backend/BACKEND_DESIGN.md) · [SYSTEM_ARCHITECTURE](../06-architecture/SYSTEM_ARCHITECTURE.md) · [LOCAL_DEVELOPMENT](../11-operations/LOCAL_DEVELOPMENT.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

ADR-0074 đặt Go module ở gốc repository (`go.mod`, `cmd/`, `modules/`, `pkg/`) cạnh `web/` (Admin Portal) và `godot/` (game). Technical Architect yêu cầu mã nguồn nằm trong `src/`, tách rõ theo cấp thư mục giữa CMS và game. Backend Go là một modular monolith phục vụ chung CMS lẫn game (cùng `core-api`, `worker`, database), và `worker` là binary build từ cùng mã với `core-api`.

## Quyết định

1. Mã nguồn nằm trong `src/`:

   | Thư mục | Nội dung |
   |---|---|
   | `src/backend/` | Go module **duy nhất** (`go.mod`, module path `github.com/tanguyen2020/eduworld-platform/src/backend`): `cmd/` (`core-api`, `realtime`, `worker`, `migrate`), `modules/<module>/` (cấu trúc của ADR-0074 mục 1 giữ nguyên), `pkg/`, `tools/` (công cụ Go của repo, vd. `archgen`), `test/` (integration test không thuộc module nào) |
   | `src/cms/frontend/` | Admin Portal (thay `web/`): `apps/portal`, `packages/ops-ui` |
   | `src/game/` | Godot client (thay `godot/`) |

2. `worker` không có thư mục riêng: là binary `src/backend/cmd/worker` (profile `events`/`chat`/`jobs`). Backend không tách theo CMS/game; ranh giới vẫn là module domain (ADR-0074).
3. Ở gốc giữ nguyên: `deploy/`, `specs/`, `docs/`, `scripts/`, `design/`, `.github/`, `Makefile`, `tools/versions.mk` (phiên bản công cụ dùng chung), và `tests/` cho bộ test không phải Go chạy trên hệ thống đã triển khai (k6, chaos, ZAP, synthetic). Test Go nằm trong `src/backend` (cạnh mã, hoặc `src/backend/test/` khi không thuộc module nào); công cụ Godot nằm trong `src/game/tools/`.
4. Lệnh Go chạy trong `src/backend` (`go -C src/backend …`; Makefile dùng biến `BACKEND`). Docker build context vẫn là gốc repository, Dockerfile chép từ `src/backend/`.
5. Đường dẫn trong tài liệu và feature spec: `modules/…`, `cmd/…`, `pkg/…` → `src/backend/…`; `web/…` → `src/cms/frontend/…`; `godot/…` → `src/game/…`. ADR cũ giữ đường dẫn cũ như bản ghi lịch sử; đọc theo bảng trên.

## Hệ quả

(+) Mã nguồn tách khỏi tài liệu, contract và hạ tầng; ranh giới CMS / game / backend dùng chung nhìn thấy ngay ở cấp thư mục. (−) Module path Go dài hơn; công cụ phải chạy với `-C src/backend` hoặc `cd`; mọi đường dẫn trong tài liệu và CI đổi một lần.

## Phương án đã loại

- **`src/cms/{frontend,backend,worker}` và backend game riêng:** phần backend của game (module `game`, `competition`, `chat`, `realtime`) sẽ nằm dưới `cms/` hoặc phải tách Go module, trái ADR-0074.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-17 | Tạo ADR (Accepted) | Technical Architect |
| 2026-11-17 | Làm rõ: `tests/` ở gốc cho test không phải Go; công cụ Godot trong `src/game/tools/` | Technical Architect |
