# ADR-0090 — Danh mục menu/action của Admin Portal và bật/tắt lưu trong database

> **Trạng thái:** Accepted · **Ngày:** 2026-11-17 · **OQ:** — · **Bổ sung:** ADR-0027, ADR-0066 · **Tài liệu:** [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) · [DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md) · [API_DESIGN](../07-backend/API_DESIGN.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Menu và workspace của Admin Portal được khai báo trong code frontend (`menuRegistry`, `workspaceRegistry` của F051) và lọc theo permission từ `getMyPermissions`. Permission nằm trong code (`pkg/authz/catalog.go`), DB chỉ có bản sao đọc-chỉ `identity.permissions`. `platform.feature_flags` (F015) có mặc định platform và ghi đè theo tenant nhưng không gắn với menu hay action.

Hệ quả: muốn ngừng dùng một menu phải sửa code và deploy; gỡ permission khỏi role là phân quyền theo người, không phải tắt tính năng; ẩn menu không chặn API (ADR-0027 mục 7, R1). Technical Architect yêu cầu danh mục menu và action có trong database để bật/tắt dễ dàng, chỉ System Admin thao tác, chỉ áp cho Admin Portal.

## Quyết định

### 1. Danh mục menu/action là contract, có bản sao trong DB

- Nguồn duy nhất: `specs/portal/menu-catalog.yaml`, kiểm tra bằng `specs/schemas/portal-menu-catalog.schema.json`. Cấu trúc phân cấp **workspace → menu → action**:
  - workspace: `key` (`school_lms`, `content_studio`, `game_ops`, `live_ops`, `platform_admin`), `label_key`;
  - menu: `key` dạng `menu.<workspace>.<menu>`, `path`, `label_key`, `required_permissions[]`, `order`, `can_disable`;
  - action: `key` dạng `action.<workspace>.<menu>.<action>`, `label_key`, `required_permissions[]`, `operation_ids[]` (operation của OpenAPI public mà action gọi), `can_disable`.
- Từ file này sinh `menuRegistry`/`workspaceRegistry` cho `web/apps/portal` (không khai báo menu tay trong TSX) và bảng đọc-chỉ `platform.portal_menu_catalog` trong `eduworld_identity` (ghi bởi `cmd/migrate` lúc deploy, có `catalog_version`; `identity_app` không có quyền ghi), cùng mẫu với `identity.permissions`.
- Thêm menu hay action mới vẫn cần code (màn hình, API); database chỉ giữ danh mục và trạng thái bật/tắt.

### 2. Trạng thái bật/tắt dùng `platform.feature_flags`

- Thêm cột `flag_kind` (`feature`/`menu`/`action`) và `parent_key` (action → menu). Key của flag loại `menu`/`action` phải tồn tại trong `platform.portal_menu_catalog` (kiểm tra ở use case và khóa ngoại tới catalog).
- Không có dòng flag nghĩa là **bật**. Giá trị hiệu lực của một action = flag của action AND flag của menu cha; mỗi flag = ghi đè của tenant (nếu có) nếu không thì `default_enabled`.
- **Chỉ System Admin** (`platform.feature_flag.write`, gateway nội bộ, MFA) đặt mặc định platform hoặc ghi đè cho một tenant đích. Bắt buộc `reason`, `If-Match`; ghi audit, phát `platform_settings.changed`.
- Mục `can_disable = false` không tắt được (`422`): Người dùng & phân quyền, Tenant, Bật/tắt menu, Nhật ký toàn hệ thống, Thùng rác, hàng đợi case kiểm duyệt và sanction, thông báo an toàn của trường, và mọi action thuộc các menu này.

### 3. Backend chặn, không chỉ ẩn

- Mỗi operation OpenAPI được Admin Portal gọi khai báo `x-portal-action: <action key>`. Script `check-portal-catalog` bảo đảm hai chiều: mọi `operation_ids` trong catalog tồn tại và có đúng `x-portal-action`; mọi operation có `x-portal-action` thuộc một action trong catalog.
- Middleware `featuregate` (`pkg/httpx`) trong `core-api`, chỉ áp với token `azp = eduworld-portal`: tra trạng thái hiệu lực của action theo tenant của phiên (cache Valkey, làm mới khi nhận `platform_settings.changed`, TTL tối đa `principal_cache_ttl`). Action hoặc menu cha đang tắt → `403 FEATURE_DISABLED` (Problem Details có `feature_key`). Phiên System Admin liên tenant không gửi `X-Tenant-Id` dùng giá trị mặc định platform.
- `featuregate` chạy sau xác thực và trước `authz.Require`; không thay authorization.
- `getMyPermissions` trả thêm `disabled_menus[]` và `disabled_actions[]` cho phạm vi phiên; Portal ẩn menu tắt và ẩn/khóa nút của action tắt.

### 4. Ngoài phạm vi

Menu trong game client (Godot) không dùng cơ chế này; tính năng game vẫn dùng feature flag hiện có qua bootstrap.

## Hệ quả

(+) Ngừng dùng một menu hay action không cần deploy; trạng thái có audit; tắt menu là tắt thật ở API. (−) Thêm một contract (`menu-catalog.yaml`) phải giữ đồng bộ với OpenAPI (có script kiểm tra); thêm một lần tra cache cho mỗi request của portal; phải sửa F012, F015, F051, các feature portal (F052–F059, F079, F108, F122…), DATABASE_DESIGN, ADMIN_PORTAL, API_DESIGN.

## Phương án đã loại

- **Menu định nghĩa hoàn toàn trong DB (kể cả path, nhãn):** menu không có màn hình tương ứng trong code thì vô nghĩa; tách nguồn khiến code và DB lệch nhau.
- **Bảng bật/tắt riêng thay vì `platform.feature_flags`:** hai cơ chế cho cùng một việc (R33).
- **Chỉ ẩn menu ở frontend:** API vẫn gọi được, không phải tắt thật.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-17 | Tạo ADR (Accepted): chỉ System Admin bật/tắt, chỉ Admin Portal | Technical Architect |
