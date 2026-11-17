# Data Model: F[NNN] — [Tên tính năng]

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

<!--
Mô tả entity và bảng feature tạo/sửa. Tên bảng, cột, kiểu, MaxLen phải khớp DATABASE_DESIGN; nếu feature thêm mới thì ghi "Mới" và có task cập nhật DATABASE_DESIGN.
Nếu feature không có dữ liệu bền vững: ghi "Không áp dụng — <lý do>" ở mục 1 và bỏ các mục còn lại.
-->

## 1. Entity

### [Entity] — `schema.table` ([Có sẵn | Mới | Thêm cột])

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `id` | UUIDv7 | PK | mixin `Base` |
| `tenant_id` | uuid | NOT NULL, RLS | R8 |
| `…` | `varchar(n)` (`MaxLen(n)`) | | ADR-0041 |
| `deleted_at`, `deleted_by` | | partial unique `WHERE deleted_at IS NULL` | R21 |

**Quan hệ**: …
**Index**: …

## 2. Vòng đời / trạng thái

[Sơ đồ hoặc bảng chuyển trạng thái nếu có]

## 3. Quy tắc kiểm tra dữ liệu

- [Invariant domain, link tài liệu sở hữu]

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| `<entity>.<past_verb>` | | `specs/events/…` |
