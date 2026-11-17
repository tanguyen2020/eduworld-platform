# src/ — mã nguồn EduWorld

Bố cục theo [ADR-0092](../docs/adr/ADR-0092-to-chuc-ma-nguon-trong-src.md):

| Thư mục | Nội dung | Feature tạo khung |
|---|---|---|
| [`backend/`](backend/) | Go module **duy nhất** của backend (ADR-0074), dùng chung cho CMS và game: `cmd/` (binary `core-api`, `realtime`, `worker`, job `migrate`), `modules/` (`identity`, `learning`, `game`, `competition`, `chat`, `realtime`), `pkg/` (platform kit), `tools/`, `test/` | F001 |
| [`cms/frontend/`](cms/frontend/) | Admin Portal (React + TypeScript + Vite, 5 workspace): `apps/portal`, `packages/ops-ui` | F050 |
| [`game/`](game/) | Game client Godot 4.x (`project.godot`, `scenes/`, `scripts/`, `resources/`) | F060 |

`worker` không có thư mục riêng: đó là một binary (`cmd/worker --profile=events|chat|jobs`) build từ cùng mã Go với `core-api`, nên nằm trong `backend/`.

Ngoài `src/`: `deploy/` (compose, Dockerfile, init DB), `specs/` (contract + feature spec), `docs/`, `scripts/`, `design/` (design tokens), `tools/versions.mk` (phiên bản công cụ).
