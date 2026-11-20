# ADR-0032 — Worker dọn tài khoản học sinh không đăng nhập game

> **Trạng thái:** Accepted · một phần superseded by ADR-0085 · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md), [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần đã được thay bởi [ADR-0085](ADR-0085-don-hoc-sinh-giu-user-keycloak.md):** không vô hiệu hóa user Keycloak khi dọn (mục 3); chỉ gỡ membership, thu hồi phiên, soft-delete hồ sơ. **Còn hiệu lực:** toàn bộ phần còn lại.

## Bối cảnh

Stakeholder yêu cầu worker dọn học sinh không login vào game, số ngày cấu hình được, dựa trên last_login + số ngày.

## Quyết định

1. **Cấu hình:** `inactive_student_cleanup_days` cấp platform (mặc định) và cấp tenant (ghi đè); `null`/`0` = tắt. System Admin sửa cấp platform; Tenant Admin sửa cấp tenant nếu được cấp quyền [Confirmed]; có audit. Mặc định đề xuất 180 ngày [Confirmed].
2. **Điều kiện dọn:** học sinh chưa bị xóa thỏa `COALESCE(last_login_at, provisioned_at) + cleanup_days < now()`. Không dọn học sinh đang có game session active, đang làm bài kiểm tra/giải đấu chưa kết thúc, hoặc có moderation case mở.
3. **"Clean" = soft-delete** (ADR-0028): đặt `deleted_at` cho User, Student, GameProfile và membership liên quan, `delete_reason = inactive_cleanup`; gỡ khỏi Keycloak Organization và vô hiệu hóa user Keycloak; thu hồi session; rời party; event `student.cleaned_up`; audit. Chạy như **saga** do identity-service điều phối qua internal platform API của từng service.
4. **Báo trước:** trước N ngày (mặc định 14 [Confirmed]), tạo danh sách "sắp bị dọn" cho Tenant Admin/School Admin trong portal; tenant có thể **loại trừ** hoặc gia hạn.
5. **Quay lại:** học sinh đăng nhập lại qua SSO và vẫn hợp lệ trong SIS → **khôi phục** (bỏ `deleted_at`, thêm lại membership) kèm dữ liệu cũ, audit `student.restored`. SIS sync **không** tự khôi phục học sinh đã bị dọn khi họ chưa đăng nhập lại.
6. **Vận hành:** chạy hằng ngày 02:00 ICT [Confirmed], lô 500, khóa singleton, idempotent, có dry-run, metric `eduworld_student_cleanup_total{tenant,result}`, báo cáo mỗi lần chạy.

## Hệ quả

(+) Danh sách học sinh gọn, tối thiểu hóa dữ liệu. (−) Cấu hình quá ngắn (vd nghỉ hè) có thể dọn nhầm hàng loạt; biện pháp: báo trước, loại trừ, khôi phục khi đăng nhập lại, cảnh báo và yêu cầu xác nhận khi một lần chạy dọn > 10% học sinh của tenant [Confirmed].

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Accepted | Stakeholder |
| 2026-11-15 | Thực thi bằng saga qua internal platform API | ADR-0036 |
| 2026-11-16 | Một phần superseded by ADR-0085 | Technical Architect |
