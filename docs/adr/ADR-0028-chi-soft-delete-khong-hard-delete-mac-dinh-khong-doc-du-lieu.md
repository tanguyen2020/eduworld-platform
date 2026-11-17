# ADR-0028 — Chỉ soft-delete, không hard-delete; mặc định không đọc dữ liệu đã xóa

> **Trạng thái:** Accepted · một phần superseded by [ADR-0089](ADR-0089-chot-tang-du-lieu-rls-mixin-outbox.md) · **Ngày:** 2026-11-15 · **OQ:** OQ-20 · **Tài liệu:** [DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md), [DATA_PRIVACY](../10-security/DATA_PRIVACY.md) · [Chỉ mục ADR](README.md)

> [!NOTE]
> **Mục 8 được chốt bởi [ADR-0070](ADR-0070-release-1-khong-voice-chat-khong-xoa-vat-ly-khong-purge-analytics.md)** (2026-11-15): Release 1.0 không có quy trình xóa vật lý hay gỡ partition; nghĩa vụ xóa đáp ứng bằng ẩn danh hóa; ngoại lệ duy nhất vẫn là ADR-0033. OQ-20 đã đóng.

> [!WARNING]
> **Một phần được thay bởi ADR-0089.** Không còn hiệu lực: điều kiện `deleted_at IS NULL` trong policy RLS SELECT. **Còn hiệu lực:** chỉ soft-delete, mặc định không đọc dữ liệu đã xóa (do ent interceptor và view `_live` bảo đảm), Thùng rác.

## Bối cảnh

Stakeholder yêu cầu: ứng dụng chỉ soft-delete, không hard-delete; khi lấy dữ liệu không lấy dữ liệu đã soft-delete.

## Quyết định

1. Mọi bảng nghiệp vụ có `deleted_at`, `deleted_by` (và `delete_reason` tùy chọn). "Xóa" = `UPDATE ... SET deleted_at = now(), deleted_by = $actor`, ghi audit và phát event `<entity>.deleted`.
2. **Ứng dụng không bao giờ chạy `DELETE`.** Role DB ứng dụng (`<svc>_app`, `<svc>_platform`) **không có quyền `DELETE`/`TRUNCATE`** trên bảng nghiệp vụ (REVOKE + kiểm tra trong CI). Endpoint `DELETE` của API mang nghĩa soft-delete. Hook của ent đổi delete thành soft-delete (ADR-0039).
3. **Mọi truy vấn đọc mặc định loại dữ liệu đã xóa:** global filter `deleted_at IS NULL` cùng `tenant_id` (ADR-0005, ADR-0039); RLS SELECT cũng có điều kiện này; unique constraint dùng partial index `WHERE deleted_at IS NULL`.
4. **Xem/khôi phục** chỉ qua "Thùng rác" dành cho System Admin (Tenant Admin trong tenant của mình nếu được cấp quyền [Proposed]), có audit; khôi phục kiểm tra không vi phạm unique.
5. **Quan hệ cha–con:** soft-delete cha không xóa cứng con; con được ẩn theo quy tắc từng aggregate (DATABASE_DESIGN).
6. **Bảng append-only** (learning events, attempt answers, audit logs, ledger XP/tiền tệ, outbox) không có thao tác xóa; sửa sai bằng bản ghi bù.
7. **Dữ liệu tạm trong Valkey** hết hạn bằng TTL, không thuộc phạm vi ADR này.
8. **Nghĩa vụ xóa theo luật/thời hạn lưu trữ:** ứng dụng xử lý bằng **ẩn danh hóa** (ghi đè PII, giữ bản ghi). Xóa vật lý (nếu luật bắt buộc hoặc gỡ partition hết hạn) chỉ qua quy trình vận hành có phê duyệt, ngoài ứng dụng, có biên bản [TBD OQ-20].
9. **Ngoại lệ duy nhất:** chat gameplay ngắn hạn được purge vật lý bởi job riêng theo ADR-0033. Mọi ngoại lệ khác cần ADR mới.

## Hệ quả

(+) Không mất dữ liệu do thao tác nhầm, khôi phục được, truy vết đầy đủ. (−) Bảng lớn dần, cần partial index và kế hoạch lưu trữ; truy vấn viết tay phải có điều kiện `deleted_at IS NULL` và được test tự động phát hiện thiếu.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Accepted | Stakeholder |
| 2026-11-15 | Thêm ngoại lệ purge chat gameplay | ADR-0033 |
| 2026-11-15 | Role DB theo service; hook ent | ADR-0037, ADR-0039 |
| 2026-11-17 | Một phần superseded by ADR-0089 (RLS chỉ lọc tenant) | Technical Architect |
