# ADR-0065 — `tenant_id` trên header mọi request, bắt buộc và luôn được kiểm tra chéo

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** — · **Bổ sung:** ADR-0005, ADR-0026 · **Tài liệu:** [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [API_DESIGN](../07-backend/API_DESIGN.md) · [BACKEND_DESIGN](../07-backend/BACKEND_DESIGN.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Người duyệt yêu cầu: dữ liệu theo học sinh **phải phân biệt theo `tenant_id` (bắt buộc)**, và **`tenant_id` được gắn trên header**. Trước đây tài liệu chỉ lấy tenant từ token và cấm client gửi tenant qua header.

## Quyết định

1. **Header `X-Tenant-Id` (UUID) bắt buộc** trên mọi request REST tới dữ liệu tenant-scoped của game và portal (sau bước xác định tenant khi đăng nhập). Client lấy giá trị từ kết quả `POST /auth/session` (PlayerBootstrap) hoặc từ lựa chọn tenant sau đăng nhập (ADR-0063), và gửi ở mọi request tiếp theo.
2. **Không bao giờ tin header một mình.** Gateway và backend kiểm tra theo thứ tự:
   - APISIX: thiếu header ở route tenant-scoped → `400 TENANT_HEADER_REQUIRED`; định dạng sai → `400`. Dùng header cho rate limit theo tenant và ghi log/trace.
   - Backend (middleware tenant, mọi service): header phải **khớp** tenant của phiên (claim `organization` đã kiểm tra + membership trong DB, ADR-0026). Không khớp → `403 TENANT_MISMATCH`, ghi audit/security event. `Principal.tenant_id` lấy từ kết quả kiểm tra, không lấy thẳng từ header.
   - Repository, ent interceptor và RLS (`SET LOCAL app.tenant_id`) dùng `Principal.tenant_id` (ADR-0005, ADR-0039).
3. **Mọi bảng dữ liệu theo học sinh có `tenant_id NOT NULL`** và mọi truy vấn lọc theo `tenant_id` [Confirmed, nhắc lại ADR-0005].
4. **Ngoại lệ không cần header:** `POST /auth/session` (chưa có tenant), `GET /game/client-config`, health. **System Admin** (liên tenant, ADR-0027): thao tác ghi bắt buộc `X-Tenant-Id` là tenant đích; thao tác đọc liên tenant dùng tham số `tenant_ids` và không gửi header (hoặc gửi giá trị đặc biệt bị từ chối) — chỉ với permission `platform.cross_tenant.read`.
5. **Service-to-service** giữ `X-Tenant-Id` như ADR-0040; bên nhận vẫn kiểm tra quyền. **WebSocket:** tenant cố định khi cấp realtime ticket; không đổi trong kết nối.
6. **Đổi tenant** (người thuộc nhiều tenant): lấy token mới với scope tổ chức mới rồi đổi header; header khác token → bị từ chối.

## Hệ quả

(+) Mọi request mang tenant rõ ràng, dễ định tuyến, giới hạn và truy vết theo tenant ở gateway, log và trace. (−) Client phải luôn gửi header; thêm một bước kiểm tra ở gateway và middleware (chi phí nhỏ, có cache membership 60 giây).

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR; thay quy tắc "cấm X-Tenant từ client" trong BACKEND_DESIGN | Stakeholder (người duyệt) |
| 2026-11-16 | Đọc mục 1 và 4 theo ADR-0068/ADR-0083: với game, `bootstrap.tenant_id` lấy từ `POST /auth/game/login` hoặc `POST /auth/game/tenant:select` (hai endpoint này không gửi header); `POST /auth/session` chỉ còn là bootstrap của portal. Đóng CODE_PLAN D16 | Technical Architect |
