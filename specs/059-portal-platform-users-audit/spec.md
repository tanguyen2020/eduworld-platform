# Feature Specification: F059 — Workspace D — người dùng/role platform, catalog permission, audit toàn hệ thống, login events liên tenant, feature flags, màn tích hợp

**Feature Branch**: `059-portal-platform-users-audit` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `src/cms/frontend/apps/portal` (workspace D) · **Mốc**: M-C3 · **Phụ thuộc**: F012, F015, F057
**Input**: Phần còn lại của workspace D (chỉ System Admin, gateway nội bộ + MFA) ngoài F057: Role platform/role dùng chung, UserRole platform và UserRole tenant ở tenant đích, người dùng mọi tenant, Permission catalog (chỉ xem), audit toàn hệ thống và nhật ký phân quyền, lịch sử đăng nhập game liên tenant, feature flag, màn Tích hợp (Keycloak/SIS/APISIX chỉ xem). FR nguồn: FR-AUD-001, FR-AUTH-032, FR-AUTH-016, FR-CMS-001, FR-CMS-012, FR-GAME-013, FR-ORG-007, FR-AUTH-010, FR-SIS-007.

## Clarifications

### Session 2026-11-16

- Q: `TenantFilter`, `TargetTenantField`, `MfaStepUp`, `VpnRequiredBanner`, guard chunk D có làm lại không? → A: Không; F057 cung cấp, F059 dùng lại.
- Q: Danh sách người dùng mọi tenant và tạo nhân sự platform có contract chưa? → A: Chưa; thêm `listPlatformUsers`, `createPlatformUser` (delta, owner F012). Tạo nhân sự trường ở tenant đích dùng `createTenantUser` có sẵn với `X-Tenant-Id` = tenant đích.
- Q: Mở khóa/vô hiệu hóa tài khoản? → A: `lockUser` có sẵn; `unlockUser`, `deactivateUser` là delta của F016. Nút hiện khi contract có.
- Q: Feature flag API? → A: `listFeatureFlags`, `putFeatureFlag` là delta của F015; F059 chỉ làm UI.
- Q: Màn Tích hợp lấy dữ liệu ở đâu? → A: [Proposed] một endpoint chỉ đọc `getIntegrationOverview` (module identity, giá trị không bí mật từ cấu hình + đếm SIS connection) và `listSisConnections`/`listSisSyncRuns` (delta F011). Integration Monitoring là [TBD], ngoài phạm vi.
- Q: Role tùy biến của tenant sửa ở đâu? → A: Workspace D chỉ xem (lọc tenant); sửa ở workspace C với tenant đích (F052).

### Session 2026-11-17

- Q: Tên thư mục workspace D? → A: `platform-admin` (ADMIN_PORTAL §1.2, RB-603); route `/portal/platform-admin/*`.

### Session 2026-11-20 (triển khai — Câu hỏi mở)

- OQ F059-1 — `listPlatformUsers`, `createPlatformUser` (T004, owner F012) chưa có trong contract và backend? → A: [Proposed] Không tự thêm contract thay owner khi chưa có backend (cùng cách OQ F057-1). Tab Người dùng ghi rõ "Danh sách tài khoản mọi tenant có khi API listPlatformUsers được bổ sung" và dùng phần contract có: tài khoản theo một tenant (`listTenantUsers`, `X-Tenant-Id` = tenant đó, lọc `q`/loại, nhãn SIS, học sinh chỉ đọc), tra cứu theo mã người dùng (ngăn chi tiết + UserRole mọi tenant), "Tạo nhân sự trường" (`createTenantUser` với tenant đích). Lựa chọn "Nhân sự platform" hiện nhưng khóa. Cờ `PLATFORM_USERS_AVAILABLE` trong `api/users.ts` bật khi owner áp delta.
- OQ F059-2 — `getIntegrationOverview` (T009, owner identity/F012) chưa có? → A: [Proposed] Tab "Keycloak · APISIX" chỉ ghi rõ API chưa có và cấu hình quản lý bằng realm-as-code/route-as-code; tab SIS dùng `listSisConnections`/`listSisConnectionSchools`/`listSisSyncRuns` (đã có). Cờ `INTEGRATION_OVERVIEW_AVAILABLE` trong `api/integrations.ts`.
- OQ F059-3 — `listPlatformAuditLogs` chỉ khớp `action` chính xác (không có lọc tiền tố, T007 owner F015)? → A: [Proposed] Nhật ký phân quyền chọn một trong các action `role.created/updated/deleted`, `user_role.granted/revoked/expired` (danh sách action của F012 plan) mỗi lần tra cứu (mặc định `role.created`); mọi request của tab chỉ mang action `role.*`/`user_role.*`. Đề xuất owner F015 thêm `action_prefix[]` (maxItems 10) để xem chung một danh sách. `x-internal` đã có.
- OQ F059-4 — T005: `UserRoleGrantRequest.training_completed_at` đã có (owner F012 áp), nhưng `TenantUserCreateRequest.initial_user_roles[].scope_type` vẫn liệt kê `class`? → A: [Proposed] Portal chỉ cho chọn `tenant`/`school`/`subject` (ADR-0072); đề xuất owner F012 bỏ `class` khỏi enum (cần `make specs-breaking`).
- OQ F059-5 — `UserRole` không có username/họ tên và `User` không có membership (`tenant_ids[]`) → cột Người dùng hiện mã người dùng (mở ngăn chi tiết), `TargetTenantField` không lọc được theo membership (edge case)? → A: [Proposed] Hộp gán cho nhân sự có tenant gốc chọn tenant đích rồi chọn người trong `listTenantUsers` của tenant đó (chỉ ra người thuộc tenant); server vẫn kiểm (`422`). Đề xuất owner F012 thêm tên hiển thị/`tenant_ids[]` khi làm `listPlatformUsers`.
- OQ F059-6 — Picker role và cột "Role đang chứa" của catalog dùng trang đầu `listPlatformRoles` (`limit` = 100)? → A: [Proposed] Đủ cho số role hiện có; danh sách Role chính vẫn phân trang cursor.
- OQ F059-7 — Contract feature flag của owner F015 khác bản đề xuất ở contracts §6 (`FeatureFlagWrite {scope, state, reason}`, `tenant_override` theo `tenant_id` của truy vấn, không có "số tenant ghi đè")? → A: [Proposed] Theo contract owner: màn liệt kê mặc định + ghi đè của tenant đang xem (chọn tenant), không hiển thị số tenant ghi đè; sửa bằng `putFeatureFlag` với `If-Match` = `version` của dòng, `X-Tenant-Id` = tenant đích khi ghi đè; mục `can_disable = false` không tắt được. Màn cây "Bật/tắt menu & action" theo `listPortalMenuCatalog` không thuộc spec F059 (không làm).
- OQ F059-8 — `unlockUser`, `deactivateUser` (T006, owner F016)? → A: Đã có trong contract (owner áp); nút Mở khóa/Vô hiệu hóa dùng ngay.
- OQ F059-9 — MSW và Playwright E2E với backend thật? → A: [Proposed] Như OQ F057-9/F057-10: backend MOCK trong tiến trình (`src/test/platform-access-backend.ts`, nhãn MOCK) thay MSW; tiêu chí kiểm ở mức integration (portal đầy đủ + MOCK theo contract); E2E Playwright, axe/visual sáng-tối chờ môi trường compose.
- OQ F059-10 — Route thật? → A: [Proposed] Path lấy từ danh mục (ADR-0090) thay cho danh sách route ở Assumptions: `/platform-admin/access-control?tab=users|roles|permissions|user-roles`, `/platform-admin/audit-logs?tab=system|permissions`, `/platform-admin/student-activity`, `/platform-admin/feature-toggles`, `/platform-admin/integrations`.
- OQ F059-11 — Permission đọc audit `platform.audit.read` (OpenAPI, catalog menu) hay `platform.audit.read_all` (ADMIN_PORTAL §2.2)? → A: [Proposed] Dùng `platform.audit.read` theo contract và `specs/portal/menu-catalog.yaml`; báo F012 chốt (T054), không sửa ADMIN_PORTAL khi chưa chốt.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Role platform và Permission catalog (Priority: P1) 🎯 MVP

`sa.admin` xem catalog permission theo nhóm, tạo role platform `liveops_lead` từ một phần quyền của `liveops`, nhân bản role hệ thống, xem role tùy biến của các tenant (FR-AUTH-032, FR-AUTH-016, FR-CMS-001, FR-CMS-012).

**Why this priority**: Không có role thì không cấp được quyền cho đội vận hành platform mới.

**Independent Test**: Mở Permission catalog; tạo role platform; thử tạo role `tenant` dùng chung chứa `game.quest.write` (gọi API trực tiếp) → lỗi hiển thị theo `code`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F059-AC-01 | KHI System Admin mở Permission catalog, Portal PHẢI hiển thị `listAllPermissions` nhóm theo `group_code`, lọc `scope_kind`, tìm theo mã, hiện `catalog_version` và với mỗi permission các role đang chứa nó; HỆ THỐNG PHẢI không có thao tác thêm/sửa/xóa permission | FR-CMS-012 · FR-AUTH-016 · [ADMIN_PORTAL §2.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F059-AC-02 | KHI System Admin mở Role, Portal PHẢI hiển thị `listPlatformRoles` lọc `role_scope_kind` và `tenant_ids[]` (cột Tenant: "Dùng chung" khi `tenant_id = null`), nhãn "Hệ thống" cho `is_system`, số người `active_user_role_count`; role tùy biến của tenant chỉ xem | FR-AUTH-032 · [ADMIN_PORTAL §3.17](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-TEN-004 |
| F059-AC-03 | KHI System Admin có `platform.role.write` tạo/sửa role, Portal PHẢI gọi `createPlatformRole` (`Idempotency-Key`) hoặc `updatePlatformRole` (`If-Match`, `reason` bắt buộc) và picker PHẢI chỉ liệt kê permission có `scope_kind` khớp `role_scope_kind` (role `tenant` không thấy `platform.*`, `game.*`, `competition.config.*`) | FR-AUTH-032 · FR-CMS-012 · ADR-0066 | TC-AUTH-076 |
| F059-AC-04 | NẾU server trả `422 VALIDATION_FAILED` với `errors[].code = PERMISSION_NOT_ALLOWED_FOR_SCOPE`, `412`, `409` (xóa role còn người dùng) hoặc `403` (role `is_system`), THÌ Portal PHẢI hiển thị thông điệp theo `code` và không mất dữ liệu form; role `is_system` PHẢI chỉ có nút "Nhân bản thành role tùy biến" thay cho Sửa/Xóa | FR-AUTH-032 · ADR-0066 | TC-CMS-RBAC-01 |
| F059-AC-05 | KHI System Admin xóa role, Portal PHẢI yêu cầu xác nhận + `reason` rồi gọi `deletePlatformRole` (`If-Match`, `reason`); KHI lưu role thành công, Portal PHẢI hiển thị "thay đổi áp dụng trong ≤ `principal_cache_ttl`" kèm số người bị ảnh hưởng | FR-AUTH-032 · AGENTS R21 | TC-CMS-RBAC-04 |

---

### User Story 2 — UserRole platform và UserRole tenant đích (Priority: P1)

System Admin gán role `content_reviewer` (scope platform) cho `cm.duyet`, gán `tenant_admin` cho một nhân sự `pgd-demo`, thu hồi UserRole có lý do (FR-AUTH-032, FR-AUTH-016).

**Why this priority**: Cấp/thu hồi quyền platform chỉ làm được ở workspace D.

**Independent Test**: Gán → người được gán thấy menu mới sau ≤ `principal_cache_ttl`; thử gán role platform cho nhân sự có tenant gốc → `422`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F059-AC-06 | KHI System Admin mở UserRole, Portal PHẢI hiển thị `listAllUserRoles` lọc `user_id`, `role_id`, `tenant_ids[]` với cột người dùng, role, phạm vi, tenant, hiệu lực (`valid_from`–`valid_to`), `source`, người gán | FR-AUTH-032 · FR-AUTH-016 | TC-TEN-004 |
| F059-AC-07 | KHI System Admin gán role cho nhân sự platform (`primary_tenant_id = null`), hộp gán PHẢI chỉ cho chọn role `platform` với `scope_type = platform` và gọi `grantPlatformUserRole` không có `X-Tenant-Id`; KHI người được gán có tenant gốc, hộp PHẢI chỉ cho role `tenant`, bắt buộc tenant đích (`TargetTenantField`), `scope_type ∈ {tenant, school, subject}` có `scope_id` tương ứng, gửi `X-Tenant-Id` = tenant đích | FR-AUTH-016 · ADR-0081 · ADR-0072 · AGENTS R20 | TC-TEN-011c |
| F059-AC-08 | HỆ THỐNG PHẢI gửi `grantPlatformUserRole` với `Idempotency-Key`, `reason` bắt buộc, `valid_from`/`valid_to` tùy chọn (`valid_to` > `valid_from`); NẾU server trả `422` (role platform ở scope tenant, role portal cho học sinh, scope `class`, `ROLE_NOT_FOUND`) hoặc `409`, THÌ Portal PHẢI hiển thị theo `code` | FR-AUTH-016 · ADR-0066 | TC-CMS-AUTHZ-03 |
| F059-AC-09 | KHI System Admin thu hồi UserRole, Portal PHẢI yêu cầu xác nhận + `reason` rồi gọi `revokePlatformUserRole` (`X-Tenant-Id` = tenant của UserRole khi có) và cập nhật danh sách | FR-AUTH-032 · FR-AUTH-016 | TC-CMS-RBAC-03 |
| F059-AC-10 | KHI role được chọn là `safety_moderator`, hộp gán PHẢI bắt buộc ngày hoàn thành đào tạo và gửi trong request (sau delta `UserRoleGrantRequest.training_completed_at`) | FR-AUTH-016 · ADR-0072 · [CHILD_SAFETY](../../docs/10-security/CHILD_SAFETY.md) | — |

---

### User Story 3 — Người dùng mọi tenant (Priority: P2)

System Admin tìm tài khoản theo username/email trên mọi tenant, xem UserRole của người đó, tạo nhân sự platform mới, khóa tài khoản bị lộ mật khẩu (FR-AUTH-032, FR-AUTH-010).

**Why this priority**: Cần cho vận hành nhưng tài khoản platform ban đầu do seed.

**Independent Test**: Tìm `qlcm.truong.a` thấy 2 tenant; tạo nhân sự platform trùng email → `409 EMAIL_TAKEN`; khóa `hs.locked`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F059-AC-11 | KHI System Admin mở Người dùng, Portal PHẢI gọi `listPlatformUsers` (delta) lọc `q` (trong `maxLength`), `user_type`, `status`, `tenant_ids[]`, "chỉ nhân sự platform", hiển thị họ tên, username, email, tenant gốc, trạng thái, nhãn "SIS" với `source = sis`; học sinh chỉ đọc, không có nút gán role portal | FR-AUTH-032 · FR-ORG-003 · [ADMIN_PORTAL §3.17](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-RBAC-02 |
| F059-AC-12 | KHI System Admin có `platform.user.manage` tạo nhân sự, Portal PHẢI cho chọn "Nhân sự platform" (`createPlatformUser`, role platform ban đầu) hoặc "Nhân sự trường" (`createTenantUser` với tenant đích); email bắt buộc; `Idempotency-Key`; NẾU `409 USERNAME_TAKEN`/`409 EMAIL_TAKEN`, THÌ Portal PHẢI hiển thị gợi ý "gán role cho tài khoản sẵn có" | FR-AUTH-031 · FR-AUTH-032 · ADR-0069 · ADR-0081 | TC-CMS-LOGIN-04 |
| F059-AC-13 | KHI System Admin khóa tài khoản, Portal PHẢI yêu cầu `reason` và gọi `lockUser` (`Idempotency-Key`, `X-Tenant-Id` = tenant gốc khi có); nút Mở khóa/Vô hiệu hóa PHẢI dùng `unlockUser`/`deactivateUser` sau delta F016 | FR-AUTH-010 · ADR-0028 | — |
| F059-AC-14 | KHI mở chi tiết người dùng, Portal PHẢI hiển thị UserRole qua `listAllUserRoles?user_id=` theo từng tenant và lần đăng nhập cuối; Portal PHẢI không hiển thị mật khẩu, token, IP | FR-AUTH-032 · AGENTS R14 · ADR-0031 | — |

---

### User Story 4 — Audit toàn hệ thống và nhật ký phân quyền (Priority: P1)

System Admin tra cứu ai đã đổi công tắc kiểm duyệt tuần trước, xem mọi thay đổi `role.*`/`user_role.*` của tháng (FR-AUD-001).

**Why this priority**: FR-AUD-001 P1+; điều tra sự cố và kiểm toán bắt buộc.

**Independent Test**: Gán role ở US2 → nhật ký phân quyền có dòng mới với lý do; lọc audit theo 2 tenant.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F059-AC-15 | KHI System Admin mở Nhật ký toàn hệ thống, Portal PHẢI gọi `listPlatformAuditLogs` với `from`/`to` bắt buộc (mặc định 24 giờ gần nhất), lọc `tenant_ids[]`, `actor_id`, `action`, `resource_type`, `resource_id`, hiển thị thời điểm, người làm, `actor_type`, tenant, action, tài nguyên, `reason`, `trace_id` và `changes` trong ngăn chi tiết | FR-AUD-001 · [ADMIN_PORTAL §7](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-TEN-02 |
| F059-AC-16 | KHI System Admin mở Nhật ký phân quyền, Portal PHẢI hiển thị `listPlatformAuditLogs` chỉ với action thuộc `role.*` và `user_role.*` (ai, khi nào, trước/sau, lý do, tenant) | FR-AUTH-032 · FR-AUD-001 | TC-CMS-RBAC-04 |
| F059-AC-17 | NẾU server trả `422 VALIDATION_FAILED` vì khoảng `from`–`to` vượt giới hạn, THÌ Portal PHẢI hiển thị giới hạn theo thông điệp server và giữ bộ lọc; Portal PHẢI không có thao tác sửa/xóa bản ghi audit | FR-AUD-001 · FR-AUD-002 | — |

---

### User Story 5 — Lịch sử đăng nhập game liên tenant (Priority: P2)

System Admin xem phiên game của học sinh ở nhiều tenant khi hỗ trợ sự cố "bị đăng xuất liên tục" (FR-GAME-013).

**Why this priority**: Hỗ trợ vận hành; workspace C (F052) đã phục vụ nhà trường.

**Independent Test**: Lọc 2 tenant, 7 ngày; lọc một học sinh thấy `end_reason = replaced_by_other_device`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F059-AC-18 | KHI System Admin có `platform.cross_tenant.read` mở Đăng nhập game, Portal PHẢI gọi `listLoginEventsCrossTenant` lọc `tenant_ids[]`, `student_id`, `from`, `to` (mặc định 7 ngày), hiển thị tenant, học sinh, nền tảng, `device_label`, `client_version`, vào/ra, `end_reason` (nhãn tiếng Việt) | FR-GAME-013 · FR-CMS-013 · ADR-0031 | TC-TEN-018 |
| F059-AC-19 | HỆ THỐNG PHẢI không hiển thị hay yêu cầu IP ở màn đăng nhập game; danh sách PHẢI phân trang cursor, không tải hết | FR-GAME-013 · ADR-0031 · AGENTS R23 | — |

---

### User Story 6 — Feature flag (Priority: P3)

System Admin bật flag cho `th-demo-a` để thử màn mới (FR-ORG-007).

**Why this priority**: Phục vụ ẩn màn chưa sẵn sàng (CODE_PLAN §1.2); không chặn luồng chính.

**Independent Test**: Đặt ghi đè tenant, lưu có lý do; mở lại thấy version tăng.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F059-AC-20 | KHI System Admin mở Feature flag, Portal PHẢI hiển thị `listFeatureFlags` (mã, mô tả, mặc định, số tenant ghi đè, cập nhật lúc/bởi) và cho sửa mặc định + ghi đè theo tenant (chọn tenant qua `listTenants`) bằng `putFeatureFlag` với `If-Match` và `reason` bắt buộc | FR-ORG-007 · FR-AUD-001 | — |
| F059-AC-21 | NẾU `putFeatureFlag` trả `412`, THÌ Portal PHẢI tải lại giá trị mới và báo "Đã có người sửa"; Portal PHẢI không tạo/xóa flag (flag khai báo trong code) | FR-ORG-007 · AGENTS R3 | — |

---

### User Story 7 — Tích hợp: Keycloak, SIS, APISIX (chỉ xem) (Priority: P3)

System Admin xem issuer/realm/client đang dùng, danh sách SIS connection và lần sync gần nhất, nhóm route public/nội bộ, để đối chiếu khi hỗ trợ trường.

**Why this priority**: Tham chiếu; mọi thay đổi cấu hình làm bằng realm-as-code/route-as-code (F004).

**Independent Test**: Mở màn, kiểm không có ô nhập, không có secret trong response/DOM.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F059-AC-22 | KHI System Admin mở Tích hợp, Portal PHẢI hiển thị `getIntegrationOverview` (Keycloak: issuer, realm, client id + mục đích, MFA cho role platform; APISIX: base URL public/nội bộ, nhóm prefix → service) và tab SIS: `listSisConnections` lọc `tenant_ids[]`, `listSisConnectionSchools`, `listSisSyncRuns` (trạng thái, thời điểm, số bản ghi lỗi) | FR-SIS-007 · FR-AUTH-012 · [ADMIN_PORTAL §10](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F059-AC-23 | HỆ THỐNG PHẢI không có thao tác ghi ở màn Tích hợp và không hiển thị secret, credential SIS hay token (chỉ `credential_ref` dạng che); NẾU response chứa trường ngoài schema, THÌ Portal PHẢI bỏ qua | FR-SIS-007 · AGENTS R3, R4 | — |

---

### User Story 8 — Bảo vệ màn và đầu vào (Priority: P1)

**Why this priority**: Constitution điều V; mọi màn D platform-only.

**Independent Test**: `ad.truong.a`, `cm.noidung`, `sm.antoan` mở URL các màn F059; `sa.admin` không MFA.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F059-AC-24 | KHI người dùng không có role `system_admin` mở URL bất kỳ màn F059, Portal PHẢI hiển thị trang 403 và không tải chunk workspace D; mục menu từng màn PHẢI ẩn khi thiếu permission tương ứng (`platform.role.read`, `platform.user_role.write`, `platform.user.manage`, `platform.audit.read`, `platform.cross_tenant.read`, `platform.feature_flag.write`, `platform.sis_connection.manage`) | FR-CMS-011 · FR-CMS-012 · ADR-0027 · AGENTS R20 | TC-CMS-AUTHZ-05 |
| F059-AC-25 | NẾU API trả `403 MFA_REQUIRED` hoặc lỗi kết nối gateway nội bộ, THÌ Portal PHẢI dùng `MfaStepUp`/`VpnRequiredBanner` của F057; NẾU trả `403 PERMISSION_DENIED`, THÌ Portal PHẢI hiển thị "Không có quyền" kèm `trace_id` | FR-CMS-011 · ADR-0066 | TC-CMS-WS-02 |
| F059-AC-26 | HỆ THỐNG PHẢI gửi `Idempotency-Key` cho mọi POST ghi của F059 (tạo role, gán UserRole, tạo người dùng, khóa) và giữ nguyên khi thử lại; `If-Match` cho mọi PATCH/PUT/DELETE có ETag | AGENTS R10 · ADR-0010 | — |
| F059-AC-27 | HỆ THỐNG PHẢI giới hạn mọi ô nhập (mã role, tên, mô tả, lý do, username, email, tìm kiếm, action) theo `maxLength`/`pattern` của schema OpenAPI và hiển thị bộ đếm ký tự | ADR-0041 · AGENTS R28 | TC-TEN-024 |
| F059-AC-28 | KHI menu/action của feature (`menu.platform_admin.access_control`, `menu.platform_admin.audit_logs`, `menu.platform_admin.student_activity`, `menu.platform_admin.feature_toggles`, `menu.platform_admin.integrations` và action con) đang tắt cho phạm vi phiên (`disabled_menus`/`disabled_actions` của `getMyPermissions`), Portal PHẢI ẩn menu và ẩn/khóa nút tương ứng (`<ActionGate actionKey>`); NẾU API trả `403 FEATURE_DISABLED`, Portal PHẢI hiện thông báo tính năng tạm tắt và tải lại quyền | FR-CMS-018 · ADR-0090 | TC-CMS-MENU-02 |

### Edge Cases

- NẾU người được gán UserRole thuộc nhiều tenant, THÌ `TargetTenantField` PHẢI chỉ cho chọn tenant có membership của người đó (F059-AC-07).
- NẾU role bị xóa trong lúc mở form sửa, THÌ Portal PHẢI hiển thị `404` và quay về danh sách.
- NẾU `listAllPermissions` trả `catalog_version` khác bản đã cache, THÌ Portal PHẢI tải lại picker.
- NẾU audit `changes` có khóa lạ, THÌ Portal hiển thị dạng JSON chỉ đọc đã escape, không render HTML.
- NẾU mở trên điện thoại, THÌ form role/UserRole/người dùng hiển thị "Tác vụ này cần màn hình lớn hơn"; danh sách vẫn xem được ([ADMIN_PORTAL §1.5](../../docs/04-admin-portal/ADMIN_PORTAL.md)).
- NẾU contract delta chưa có (`listPlatformUsers`, `listFeatureFlags`, `getIntegrationOverview`…), THÌ mục menu tương ứng ẩn sau feature flag và không dùng mock trong đường chạy thật (R17).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-AUD-001 | Tra cứu audit toàn hệ thống, nhật ký phân quyền | [ADMIN_PORTAL §7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-AUD-002 | Audit chỉ đọc, không sửa/xóa trên UI | [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md) |
| FR-AUTH-032 | Role platform, UserRole platform + tenant đích, nhật ký phân quyền | [ADMIN_PORTAL §3.17](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-AUTH-016 | Ràng buộc scope khi gán UserRole (hiển thị, server kiểm) | [ADMIN_PORTAL §2.1](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-001 | Danh mục role hệ thống tenant/platform hiển thị chỉ đọc | [ADMIN_PORTAL §2.3](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-012 | Permission catalog chỉ xem | [ADMIN_PORTAL §2.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-011 | Màn F059 chỉ System Admin, gateway nội bộ + MFA | [ADMIN_PORTAL §1.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-AUTH-031 | Tạo nhân sự, xử lý trùng username/email | [ADMIN_PORTAL §3.17](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-AUTH-010 | Khóa/mở/vô hiệu hóa tài khoản từ portal | [ACCOUNT_PROVISIONING](../../docs/09-identity/ACCOUNT_PROVISIONING.md) |
| FR-GAME-013 | Lịch sử đăng nhập game liên tenant, không IP | [ADMIN_PORTAL §3.10](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-ORG-007 | Feature flag platform + ghi đè tenant | [API_DESIGN §9.16](../../docs/07-backend/API_DESIGN.md) |
| FR-SIS-007 | Màn Tích hợp không lộ credential SIS | [SIS_INTEGRATION](../../docs/09-identity/SIS_INTEGRATION.md) |
| FR-AUTH-012 | Hiển thị cấu hình Keycloak đang dùng (lấy từ env server, chỉ xem) | [KEYCLOAK_SSO](../../docs/09-identity/KEYCLOAK_SSO.md) |
| FR-CMS-018 | Menu/action của feature theo danh mục `specs/portal/menu-catalog.yaml`, ẩn khi tắt, xử lý `403 FEATURE_DISABLED` | [ADMIN_PORTAL §2.7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |

### Key Entities

- **Permission**: mã, `scope_kind`, nhóm, tên song ngữ; định nghĩa trong code.
- **Role**: mã, tên, `role_scope_kind`, `is_system`, tenant (null = dùng chung), danh sách permission, version.
- **UserRole**: người dùng, role, phạm vi (`platform`/`tenant`/`school`/`subject`), tenant, hiệu lực, nguồn, người gán.
- **User**: tài khoản SSO, `user_type`, tenant gốc, trạng thái, nguồn.
- **AuditLogEntry**: người làm, tenant, action, tài nguyên, lý do, `changes`, `trace_id`, thời điểm.
- **LoginEvent**: phiên game của học sinh (vào/ra, nền tảng, thiết bị rút gọn, lý do kết thúc).
- **FeatureFlag**: mã, mặc định, ghi đè theo tenant, version.
- **IntegrationOverview / SisConnection / SyncRun**: tham chiếu cấu hình tích hợp, không có secret.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `principal_cache_ttl` | Thông điệp thời gian quyền mới có hiệu lực |
| `portal_session_sso` | Step-up MFA, `auth_time` route platform |
| `identifier_uniqueness` | Thông điệp trùng username/email |
| `sis_sync_schedule` | Hiển thị lịch sync mặc định ở tab SIS |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Chỉ gateway nội bộ + MFA; chunk lazy-load không phải lớp bảo mật | AGENTS R20 · ADR-0027 |
| Không hardcode URL/realm/client id; lấy từ cấu hình | AGENTS R3 · FR-AUTH-012 |
| Không hiển thị PII thừa, token, IP; log client không chứa lý do nhập tay | AGENTS R14 · ADR-0031 |
| Admin/Ops design system, không component game | AGENTS R32 · ADR-0047 |
| Chuỗi theo `maxLength` của schema | AGENTS R28 · ADR-0041 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: System Admin cấp một role platform cho nhân sự mới trong ≤ 2 phút, quyền có hiệu lực ≤ 60 giây.
- **SC-002**: Tìm được bản ghi audit của một thao tác cụ thể (theo người làm + action + 7 ngày) trong ≤ 3 thao tác lọc.
- **SC-003**: 0 secret/credential xuất hiện trong response hay DOM màn Tích hợp (quét tự động trong E2E).
- **SC-004**: 0 người dùng không phải System Admin mở được màn F059 hoặc tải chunk D trong bộ E2E.

## Ngoài phạm vi

- Backend role/UserRole/permission, tạo nhân sự: F012; audit sink, truy vấn audit, feature flag backend: F015; khóa/mở/vô hiệu hóa: F016; login events backend: F014; SIS connection backend: F011/F017.
- `TenantFilter`, tenant, gán/clone, Thùng rác, danh mục, cấu hình dọn học sinh/chat retention, guard chunk D: F057.
- Người dùng/role/UserRole/audit trong một tenant (workspace C), sửa role tùy biến của tenant: F052.
- Danh mục từ nhạy cảm, công tắc kiểm duyệt: F122. Integration Monitoring, dashboard sức khỏe tích hợp: [TBD] (F080/F131).
- Sửa cấu hình Keycloak/APISIX: realm-as-code, route-as-code (F004). Xuất audit ra file: chưa có contract, không làm.
- Yêu cầu dữ liệu cá nhân (`/admin/data-requests`): chưa thuộc feature này.

## Assumptions

- [Proposed] Route `/portal/platform-admin/{roles,permissions,user-roles,users,audit,audit/permissions,login-events,feature-flags,integrations}` trong chunk `src/workspaces/platform-admin/` của F057.
- [Proposed] Hình dạng `getIntegrationOverview` theo contracts §6; giá trị lấy từ cấu hình server, không có secret.
- [Proposed] Permission đọc audit là `platform.audit.read` (theo OpenAPI); catalog ADMIN_PORTAL §2.2 còn ghi thêm `platform.audit.read_all` — báo mâu thuẫn để chốt ở F012.
- Mặc định khoảng thời gian (audit 24 giờ, đăng nhập game 7 ngày) là lựa chọn UI, không phải tham số nghiệp vụ.
