# ADR-0006 — Keycloak: một Realm dùng Organizations + Identity Brokering

> **Trạng thái:** Proposed · một phần superseded by [ADR-0026](ADR-0026-dang-nhap-theo-tenant-xac-dinh-tenant-kiem-tra-cheo-mac-dinh.md) và [ADR-0027](ADR-0027-cms-tach-vung-tenant-chi-thao-tac-learning-management-cau-hi.md) · **Ngày:** 2026-11-15 · **OQ:** OQ-02 · **Tài liệu:** [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần đã được thay.** Không còn hiệu lực: "mỗi Organization (Sở/Phòng) là một Keycloak Organization, không tạo cho từng School" → nay **mỗi Tenant là một Keycloak Organization** (ADR-0026); client `eduworld-cms`/`eduworld-admin` → một client `eduworld-portal` (ADR-0027). **Còn hiệu lực:** một realm, IdP brokering, phạm vi quyền không nằm trong token, định danh `(issuer, subject)`.

## Bối cảnh

OQ-02 (một hay nhiều realm) chưa được trả lời; cần mặc định để làm Phase 1.

## Quyết định

- Một realm `eduworld` (dev/staging/prod là các deployment riêng, không phải realm riêng).
- **Mỗi Tenant là một Keycloak Organization** (Keycloak ≥ 26), theo ADR-0026. Trường/đơn vị có IdP riêng (SIS SSO, Google Workspace for Education, Microsoft Entra ID) được cấu hình là Identity Provider gắn với Organization; trường không có IdP dùng tài khoản local do provisioning từ SIS.
- Realm role thô dùng làm cổng (`student`, `tenant_staff`, `platform_staff`); permission chi tiết do backend quyết định (ADR-0027). **Phạm vi (tenant/school/class/subject) không nằm trong token**; backend tra từ DB, token chỉ là gợi ý.
- Client: `eduworld-game`, `eduworld-portal` (public, PKCE), `eduworld-api` (resource/audience), `svc-*` cho service-to-service (ADR-0040), `eduworld-sis-sync` (confidential, cho provisioning) và `eduworld-apisix` (tùy chọn, chỉ khi APISIX dùng introspection). Chi tiết: KEYCLOAK_SSO.

## Hệ quả

(+) Một realm, SSO dễ cho nhân sự làm việc ở nhiều tenant. (−) Số Organization lớn cần kiểm thử tải.

**Khi nào chuyển sang multi-realm:** nếu một tổ chức yêu cầu cô lập hoàn toàn cấu hình đăng nhập/brand/chính sách mật khẩu mà Organizations không đáp ứng → ADR mới; backend đã hỗ trợ nhiều issuer vì định danh là `(issuer, subject)` (ADR-0008).

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR (Organization theo Sở/Phòng) | Phase 0 |
| 2026-11-15 | Ánh xạ Organization đổi sang mỗi Tenant | ADR-0026 |
| 2026-11-15 | Client portal hợp nhất `eduworld-portal`; thêm `svc-*` | ADR-0027, ADR-0040 |
| 2026-11-15 | Cập nhật tham chiếu theo ADR-0072 (bỏ vai trò giáo viên), không đổi ý nghĩa quyết định | Technical Architect |
