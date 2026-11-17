---
description: Tạo hoặc cập nhật spec.md cho một feature (chuẩn Spec Kit + EARS) từ PRD và danh mục feature.
---

Đầu vào: `$ARGUMENTS` — mã feature (vd `F021`) hoặc mô tả tính năng mới.

1. Đọc `.specify/memory/constitution.md`, `AGENTS.md`, ADR hiện hành (`docs/adr/README.md`) và danh mục feature trong `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md`.
2. Xác định feature: mã `F<NNN>`, thư mục `specs/<NNN>-<slug>/`, FR chính. Feature mới chưa có trong danh mục → đề xuất mã và dòng danh mục, dừng chờ người duyệt.
3. Tạo/ghi `specs/<NNN>-<slug>/spec.md` từ `.specify/templates/spec-template.md`:
   - User story có ưu tiên P1/P2/P3, mỗi story có Independent Test và bảng tiêu chí EARS `F<NNN>-AC-<NN>` (Nguồn: mã FR/AC + mục tài liệu sở hữu; TC: mã có thật trong `docs/12-project/TEST_STRATEGY.md` hoặc `—`).
   - Mã FR/US chỉ từ `docs/00-overview/PRODUCT_REQUIREMENTS.md`; tham số theo tên `docs/00-overview/CONFIG_DEFAULTS.md`.
   - Điểm chưa rõ đánh dấu `[NEEDS CLARIFICATION: …]` (tối đa 3 điểm quan trọng nhất).
4. Tạo `checklists/requirements.md` từ `.specify/templates/checklist-template.md` và tự đánh giá; sửa spec tới khi đạt (trừ marker cần người trả lời).
5. Không đọc/triển khai `docs/99-deferred/`, `specs/_deferred/`.
6. Chạy `node scripts/specs/check-features.mjs --verbose`; báo cáo đường dẫn, số tiêu chí, marker còn lại.
