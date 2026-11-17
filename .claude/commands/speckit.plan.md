---
description: Lập plan.md và artefact thiết kế (research, data-model, contracts, quickstart) cho một feature đã có spec Ready.
---

Đầu vào: `$ARGUMENTS` — mã feature.

1. Điều kiện: `spec.md` không còn `[NEEDS CLARIFICATION]`, checklist đạt. Nếu chưa → dừng, đề nghị `/speckit.clarify`.
2. Đọc constitution, AGENTS.md, ADR hiện hành (không theo ADR Proposed), tài liệu sở hữu và contract được spec link (`specs/openapi`, `specs/asyncapi`, `specs/schemas`, `specs/events`), `docs/07-backend/BACKEND_DESIGN.md`, `docs/07-backend/DATABASE_DESIGN.md`.
3. Từ `.specify/templates/` tạo trong thư mục feature: `plan.md` (Technical Context, **Constitution Check** đủ 9 điều, Project Structure với đường dẫn thật (`src/backend/modules/<module>/…`), Module & tầng (gồm binary/profile chạy và facade gọi), bảo mật/tenant/idempotency/audit, quan sát), `research.md` (chỉ khi có quyết định cần nghiên cứu), `data-model.md`, `contracts/README.md` (liệt kê operationId/event/schema có sẵn và delta cần thêm — không chép contract), `quickstart.md`.
4. Constitution Check có ⚠️ ở điều III–VI → dừng và báo; ⚠️ khác → ghi Complexity Tracking.
5. Không sửa `specs/` contract ở bước này; delta sẽ thành task ở `/speckit.tasks`.
