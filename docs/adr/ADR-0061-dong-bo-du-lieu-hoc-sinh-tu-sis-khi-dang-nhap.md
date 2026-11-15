# ADR-0061 — Đồng bộ dữ liệu học sinh từ SIS khi đăng nhập

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** OQ-01 · **Tài liệu:** [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) · [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Stakeholder yêu cầu: khi đăng nhập, nếu học sinh thuộc SIS thì **phải đồng bộ sang database của game** để hiển thị lớp, trường, thông tin học sinh… theo các trường dữ liệu game đã định nghĩa (EduWorld SIS Contract, ADR-0021). ADR-0046 trước đây chọn không gọi SIS trên đường đăng nhập thông thường để giữ tốc độ ở 10.000 CCU (ADR-0045). Cần dung hòa.

## Quyết định

1. **Đồng bộ khi đăng nhập ("login sync") bổ sung cho sync định kỳ**, không thay thế:
   - Ở `POST /api/v1/auth/session`, identity-service kiểm tra bản ghi SIS đã đồng bộ của học sinh (`synced_students`, enrollment, trường, lớp).
   - **Thiếu dữ liệu bắt buộc** (lần đầu đăng nhập, chưa có trường/lớp) → **đồng bộ đồng bộ (blocking)**: gọi `students/lookup` của đúng SIS connection (ADR-0046), timeout 3 giây, rồi validate contract → reconciliation → apply. Thất bại hoặc SIS không hỗ trợ lookup → dùng dữ liệu từ lần full/delta sync gần nhất; vẫn không có lớp → `NO_ACTIVE_ENROLLMENT` (ADR-0060).
   - **Đã có dữ liệu nhưng cũ hơn ngưỡng** (`sis_login_refresh_after_hours`, mặc định 24 giờ [Proposed]) → **đồng bộ nền (non-blocking)**: cho đăng nhập ngay bằng dữ liệu hiện có, đặt job làm mới (NATS work-queue `IDENTITY_SIS_JOBS`, gộp trùng theo học sinh) chạy sau.
   - Dữ liệu còn mới → không gọi SIS.
2. **Phạm vi dữ liệu đồng bộ** đúng theo DTO của EduWorld SIS Contract (ADR-0021): học sinh (mã, họ tên, ngày sinh nếu có, khối, trạng thái, login keys), trường, lớp, năm học, enrollment, giáo viên chủ nhiệm. Không lấy dữ liệu ngoài contract (tối thiểu hóa dữ liệu).
3. **Lan sang các service game:** sau khi apply, identity-service phát event (`student.synced`, `enrollment.changed`, `school.synced`) qua outbox → NATS; game-service, learning-service, competition-service, chat-service cập nhật projection `ref.students`, `ref.schools`, `ref.classes` để hiển thị trong game (hồ sơ, Hub, bạn cùng lớp, kênh lớp). Bootstrap trả về trường/lớp từ identity-service nên **lần đăng nhập đầu hiển thị đúng ngay**, không chờ projection.
4. **Tải ở giờ cao điểm:** login sync blocking chỉ xảy ra cho học sinh thiếu dữ liệu; giới hạn đồng thời theo SIS connection (`sis_login_sync_concurrency`, mặc định 20 [Proposed]) và circuit breaker theo connection; vượt giới hạn → dùng dữ liệu cũ + job nền. Có metric `eduworld_sis_login_sync_total{result}` và cảnh báo khi tỷ lệ timeout cao.
5. Ghi `sis_connection_id` và kết quả sync vào `login_events` (ADR-0046 mục 8).

## Hệ quả

(+) Học sinh luôn thấy đúng trường/lớp; dữ liệu thay đổi trong SIS phản ánh nhanh khi đăng nhập. (−) Phụ thuộc chất lượng API lookup của SIS; thêm job nền và giới hạn đồng thời cần vận hành. ADR-0046 mục "không gọi SIS trên đường đăng nhập thông thường" được bổ sung bởi ngoại lệ trên.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Stakeholder (người duyệt) |
