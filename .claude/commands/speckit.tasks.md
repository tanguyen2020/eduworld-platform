---
description: Sinh tasks.md theo user story từ spec.md, plan.md và các artefact thiết kế.
---

Đầu vào: `$ARGUMENTS` — mã feature.

1. Đọc `spec.md`, `plan.md`, `data-model.md`, `contracts/README.md`, `quickstart.md`.
2. Tạo `tasks.md` từ `.specify/templates/tasks-template.md`: Phase 1 Setup → Phase 2 Foundational (delta contract, ent schema + migration) → mỗi user story một phase theo ưu tiên (test trước, phải FAIL trước khi code) → Polish.
3. Định dạng `- [ ] T001 [P] [US1] Mô tả — file: <đường dẫn>`; mỗi tiêu chí `F<NNN>-AC-*` có ít nhất một task test; mỗi endpoint mới có task test tenant isolation + authorization.
4. Ghi Dependencies & Execution Order và Definition of Done.
