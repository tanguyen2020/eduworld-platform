# ADR-0046 — Nhận biết SIS nguồn của học sinh khi đăng nhập

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** OQ-01 · **Tài liệu:** [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md), [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md), [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Nhánh "đăng nhập lần đầu qua IdP của trường" không áp dụng cho Release 1.0** theo ADR-0063 (chỉ username/password). Các nhánh còn lại còn hiệu lực.

## Bối cảnh

Stakeholder hỏi: khi tích hợp nhiều SIS khác nhau, lúc login làm sao biết học sinh thuộc SIS nào để verify. Bổ sung ADR-0021, ADR-0026.

## Quyết định

**Nguyên tắc:** không đoán SIS từ username, email hay định dạng mã; SIS nguồn xác định qua chuỗi liên kết trong DB của identity-service `Tenant → School → SISConnection → bản ghi SIS`. Không gọi SIS trên đường đăng nhập thông thường; verify bằng dữ liệu SIS **đã đồng bộ**; chỉ gọi SIS ở ngoại lệ (mục 4).

1. **Mỗi SISConnection gắn một tenant và một tập trường** (`sis_connections.tenant_id`, `sis_connection_schools`); mỗi trường chỉ có **một** connection hoạt động.
2. **Khóa nguồn** của học sinh: `(sis_connection_id, student_external_id)` (mã chỉ duy nhất trong một SIS). **Khóa đăng nhập** từ DTO contract (`login.username`, `login.idp_alias` + `login.idp_subject`) lưu `student_login_keys`, unique theo `(sis_connection_id, login_type, idp_alias, login_key)`.
3. **Provisioning** ghi liên kết bền vững `identity.sis_identity_links(user_id, tenant_id, school_id, sis_connection_id, student_external_id, status)` và gắn thuộc tính Keycloak chỉ để tra cứu nhanh (`ew_tenant`, `ew_sis_connection`, `ew_external_id`); DB EduWorld là nguồn quyết định.
4. **Thứ tự nhận biết:**

   | Trường hợp | Cách xác định SIS nguồn |
   |---|---|
   | Đã từng đăng nhập | `(iss, sub)` → User → `sis_identity_links` active → `sis_connection_id` + `student_external_id`; verify bản ghi SIS active, enrollment active, trường thuộc tenant trong token |
   | Lần đầu, tài khoản do EduWorld cấp từ SIS | Link đã có từ lúc provisioning; xử lý như trên |
   | Lần đầu qua IdP trường/đơn vị (JIT) | Tenant từ Organization của token (hoặc tenant đã chọn) → connection của các trường trong tenant → tìm `student_login_keys` theo `(idp_alias, idp_subject)`, rồi theo `username`. **1 kết quả** → tạo link + user; **0** → mục 5; **> 1** → không tự liên kết, tạo reconciliation issue, `409 IDENTITY_CONFLICT` |
   | Không có trong dữ liệu đã đồng bộ | Nếu connection có capability `student_lookup` → gọi lookup của **đúng SIS đó** (timeout 3 s, song song các connection của tenant); kết quả qua validator + reconciliation; không thấy → mục 5 |

5. **Không thuộc SIS nào:** người dùng không chọn tenant và không thuộc tenant nào → **tenant chung** (ADR-0026); đã chọn tenant cụ thể mà không khớp → `ACCOUNT_NOT_PROVISIONED` (không âm thầm đẩy sang tenant chung).
6. **Chuyển trường giữa hai SIS:** SIS mới đồng bộ học sinh có `national_student_id` (hash) trùng → reconciliation đề xuất **chuyển link** sang connection mới; link cũ `superseded`; dữ liệu học tập/game giữ nguyên. Cùng tenant đơn vị: tự động theo policy; khác tenant: cần xác nhận.
7. **Kiểm tra lại mỗi lần tạo game session:** link còn active và bản ghi SIS chưa `inactive/suspended/transferred_out` (cache 60 s); SIS báo nghỉ học → phiên sau bị chặn, phiên hiện tại bị thu hồi (`user.access_revoked`).
8. **Audit:** ghi `sis_connection_id` đã dùng vào `login_events` (chỉ ID).

## Hệ quả

(+) Biết chính xác SIS nguồn; đăng nhập không phụ thuộc SIS bên ngoài; không nhầm học sinh giữa hai SIS dù trùng mã. (−) Phụ thuộc chất lượng dữ liệu đồng bộ; trường hợp khớp nhiều kết quả cần người xử lý.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Accepted | Stakeholder |
