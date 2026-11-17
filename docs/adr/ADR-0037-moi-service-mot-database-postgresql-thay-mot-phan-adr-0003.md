# ADR-0037 — Mỗi service một database PostgreSQL

> **Trạng thái:** Accepted · một phần superseded by [ADR-0074](ADR-0074-modular-monolith-co-ranh-gioi-cuong-che.md) · supersedes phần DB/schema của [ADR-0003](ADR-0003-postgresql-la-nguon-du-lieu-ben-vung-duy-nhat-schema-theo-mo.md) · **Ngày:** 2026-11-15 · **OQ:** OQ-10 · **Tài liệu:** [DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md), [BACKUP_RECOVERY](../11-operations/BACKUP_RECOVERY.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần được thay bởi ADR-0074.** Không còn hiệu lực: database gắn với "service" triển khai độc lập. **Còn hiệu lực:** mỗi module một database (cùng tên), mặc định chung một cụm PostgreSQL; role DB theo module; không truy cập chéo database; projection `ref.*`; backup theo database.

## Bối cảnh

Stakeholder chọn database riêng cho mỗi service; PostgreSQL vẫn là system of record.

## Quyết định

- Mỗi service có database riêng (`eduworld_identity`, `eduworld_learning`, `eduworld_game`, `eduworld_competition`, `eduworld_chat`), có thể chung cluster ở môi trường nhỏ, tách cluster khi cần; role DB riêng theo service (`<svc>_app`, `<svc>_platform`, `<svc>_migrator`, và `chat_purger` chỉ ở `eduworld_chat`). Không cấp quyền chéo database.
- Trong mỗi database: `tenant_id` + RLS (ADR-0005), soft-delete (ADR-0028), outbox/processed_events (ADR-0042), giới hạn độ dài chuỗi (ADR-0041); purge chỉ ở `eduworld_chat` (ADR-0033).
- Projection dữ liệu tham chiếu nhận từ event, lưu bảng `ref.*` (vd `ref_students`, `ref_tenants`), chỉ trường cần thiết, cập nhật idempotent.
- Báo cáo liên service xây bằng read model trong service sở hữu báo cáo (vd learning-service cho dashboard trường), nạp từ event, không join chéo DB.
- Backup/restore theo từng database; RPO/RTO giữ nguyên NFR-AVAIL-002; khôi phục nhất quán liên service theo BACKUP_RECOVERY.

## Hệ quả

(+) Ranh giới dữ liệu rõ, service độc lập. (−) Dữ liệu tham chiếu nhất quán sau (eventual consistency); báo cáo liên domain cần read model.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Accepted, supersede phần DB của ADR-0003 | Stakeholder |
| 2026-11-15 | Đặt tên role DB theo service | DEPLOYMENT/DATABASE_DESIGN |
| 2026-11-15 | Cập nhật tham chiếu theo ADR-0072 (bỏ vai trò giáo viên), không đổi ý nghĩa quyết định | Technical Architect |
| 2026-11-17 | Một phần superseded by ADR-0074: database theo module, mặc định một cụm | Technical Architect |
