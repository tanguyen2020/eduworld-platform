# Feature Specification: F057 — Workspace D — Platform Administration: tenant, gán/clone, thùng rác, danh mục, cấu hình hệ thống

**Feature Branch**: `057-portal-platform-admin` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `src/cms/frontend/apps/portal` (workspace D) · **Mốc**: M-C3 · **Phụ thuộc**: F045, F051
**Input**: Workspace D chỉ cho System Admin (gateway nội bộ + MFA): bộ lọc tenant liên tenant dùng chung cho mọi workspace, quản lý tenant, gán dữ liệu platform cho tenant, clone giữa tenant, Thùng rác mọi tenant, danh mục quốc gia/tỉnh/trường, cấu hình hệ thống (dọn học sinh, thời hạn chat gameplay). FR nguồn: FR-AUTH-024, FR-CMS-013, FR-CMS-015, FR-ORG-012, FR-ORG-013, FR-ORG-015.

## Clarifications

### Session 2026-11-16

- Q: Bộ lọc tenant liên tenant do F051 hay F057 làm? → A: F051 làm shell và bộ chọn đơn vị cho nhân sự nhiều tenant; F057 làm `TenantFilter` của System Admin (FR-CMS-013) và cung cấp cho mọi workspace qua shell.
- Q: Tạo/sửa tenant có contract chưa? → A: Chỉ có `listTenants`; `createTenant`/`updateTenant` là delta Phase 2 (CODE_PLAN §3, owner F045/F010).
- Q: Bước "xem trước đồ thị phụ thuộc" khi clone dùng API nào? → A: Chưa có; thêm `previewTenantClone` (delta, owner F045). Trước khi có, wizard bỏ bước 3 và ghi rõ trong UI.
- Q: Hủy lần dọn `awaiting_approval` có API? → A: Chỉ có `approveStudentCleanupRun`; thêm `cancelStudentCleanupRun` (delta, owner F016).
- Q: Danh mục từ nhạy cảm, công tắc kiểm duyệt, người dùng/role platform, audit toàn hệ thống, feature flag có trong F057? → A: Không (xem Ngoài phạm vi).

### Session 2026-11-17

- Q: Tên thư mục workspace D? → A: `platform-admin` (ADMIN_PORTAL §1.2, RB-603); route `/portal/platform-admin/*`.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Bộ lọc tenant liên tenant của System Admin (Priority: P1) 🎯 MVP

`sa.admin` mở bất kỳ danh sách có dữ liệu theo tenant, chọn 2 tenant trong dropdown tìm kiếm (mặc định "Tất cả tenant"), thấy cột Tenant; khi ghi phải chọn một tenant đích (FR-CMS-013, FR-ORG-008).

**Why this priority**: Mọi màn của workspace D và chế độ liên tenant ở B/C phụ thuộc component này.

**Independent Test**: Đăng nhập `sa.admin`, lọc Thùng rác và danh mục trường theo 2 tenant; đăng nhập `ad.truong.a` kiểm không có dropdown và không gửi `tenant_ids`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F057-AC-01 | TRONG KHI người dùng có `platform.cross_tenant.read`, Portal PHẢI hiển thị `TenantFilter` trên topbar và trong QueryFilter của mọi màn có dữ liệu theo tenant: tìm theo mã/tên qua `listTenants` (`q` trong `maxLength`), mặc định "Tất cả tenant", chọn nhiều, lưu lựa chọn theo phiên làm việc của người xem | FR-CMS-013 · FR-ORG-008 · [ADMIN_PORTAL §2.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-TEN-018 |
| F057-AC-02 | KHI System Admin xem danh sách liên tenant, Portal PHẢI không gửi `X-Tenant-Id`, gửi `tenant_ids[]` khi đã chọn tenant, và hiển thị cột Tenant (tên + loại `school`/`unit`/`common`, thêm cột Trường với tenant `unit`) | FR-CMS-013 · ADR-0065 | TC-CMS-TEN-02 |
| F057-AC-03 | KHI System Admin ghi dữ liệu thuộc tenant, form PHẢI bắt buộc chọn đúng một tenant đích hoặc phạm vi "Platform" và gửi `X-Tenant-Id` = tenant đích; HỆ THỐNG PHẢI không có thao tác ghi "tất cả tenant" ngầm định | FR-CMS-013 · [ADMIN_PORTAL §2.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-TEN-03 |
| F057-AC-04 | TRONG KHI người dùng không có `platform.cross_tenant.read`, Portal PHẢI không hiển thị `TenantFilter`, không hiển thị cột Tenant và không bao giờ gửi `tenant_ids` | FR-ORG-008 · AGENTS R8, R20 | TC-CMS-TEN-01 |

---

### User Story 2 — Gán dữ liệu platform cho tenant (Priority: P1)

System Admin gán bộ câu hỏi platform "Bảng nhân" cho TH Demo A và `pgd-demo`, xem tab "Tenant được gán", gỡ gán một tenant (FR-ORG-012, FR-CMS-015).

**Why this priority**: Tenant chỉ dùng được nội dung platform khi đã gán; chặn giao bài M-C3.

**Independent Test**: Gán → đăng nhập `ad.truong.a` thấy bộ câu hỏi (chỉ đọc) ở bộ chọn nội dung; gỡ gán → không còn.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F057-AC-05 | KHI System Admin bấm "Gán cho tenant" trên bản ghi platform, Portal PHẢI mở hộp chọn nhiều tenant (`listTenants`) và tùy chọn [Proposed] "mọi tenant hiện tại"/"tenant tạo sau", rồi gọi `assignToTenants` với `Idempotency-Key`; tab "Tenant được gán" PHẢI hiển thị `listTenantAssignments` | FR-ORG-012 · FR-CMS-015 · [ADMIN_PORTAL §3.7](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-TEN-019 |
| F057-AC-06 | KHI System Admin gỡ gán, Portal PHẢI hiển thị xác nhận nêu rõ "lịch sử học tập và bài đã giao giữ nguyên" và gọi `unassignFromTenants` với `Idempotency-Key` | FR-ORG-012 · [ADMIN_PORTAL §9](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-TEN-019 |

---

### User Story 3 — Clone dữ liệu giữa tenant (Priority: P2)

System Admin clone question set + câu hỏi từ TH Demo A sang `pgd-demo` (FR-ORG-013).

**Why this priority**: [Confirmed] nhưng P3 trong PRD; không chặn vòng lặp chính.

**Independent Test**: Chạy wizard clone, theo dõi job tới `succeeded`, kiểm bản sao `draft` ở tenant đích và `id_map`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F057-AC-07 | KHI System Admin chạy wizard clone, Portal PHẢI yêu cầu tenant nguồn, loại tài nguyên trong enum `ResourceType`, các bản ghi (không vượt `maxItems` của `TenantCloneRequest`), đúng một tenant đích khác tenant nguồn, rồi gọi `createTenantClone` với `Idempotency-Key` và theo dõi `getTenantClone` tới khi `succeeded`/`partially_failed`/`failed`, hiển thị `id_map` và `errors` | FR-ORG-013 · [ADMIN_PORTAL §3.7](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-CLN-01 |
| F057-AC-08 | HỆ THỐNG PHẢI không cho chọn "mọi tenant" làm tenant đích và không có loại tài nguyên dữ liệu học sinh (học sinh, enrollment, attempt, kết quả, chat, mastery) trong wizard; KHI có `previewTenantClone` Portal PHẢI hiển thị đồ thị phụ thuộc trước khi chạy (phụ thuộc platform giữ tham chiếu) | FR-ORG-013 · AGENTS R22 · ADR-0029 | TC-TEN-020 |

---

### User Story 4 — Thùng rác mọi tenant (Priority: P2)

System Admin tìm câu hỏi bị xóa nhầm của TH Demo A và khôi phục (FR-CMS-014).

**Why this priority**: Khôi phục dữ liệu là nhu cầu vận hành; không chặn M-C3.

**Independent Test**: Xóa mềm câu hỏi ở workspace B, mở Thùng rác D, khôi phục; thử khôi phục khi mã đã bị dùng lại.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F057-AC-09 | KHI System Admin mở Thùng rác, Portal PHẢI hiển thị `listTrash` lọc `resource_type` và `tenant_ids[]` (gồm dữ liệu platform), cột Tenant, người xóa, thời điểm, lý do; nút Khôi phục gọi `restoreTrashItem` với `Idempotency-Key` sau xác nhận có lý do | FR-CMS-014 · ADR-0028 · [ADMIN_PORTAL §3.8](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-DEL-01 |
| F057-AC-10 | NẾU `restorable = false` hoặc server trả `409 CONFLICT` khi khôi phục, THÌ Portal PHẢI vô hiệu/báo "Đã có bản ghi mới cùng mã"; HỆ THỐNG PHẢI không có nút "Xóa vĩnh viễn" ở bất kỳ màn nào | FR-CMS-014 · AGENTS R21 | TC-CMS-DEL-02 |

---

### User Story 5 — Danh mục quốc gia, tỉnh/thành, trường (Priority: P2)

System Admin nhập `official_code` cho tỉnh sau khi đối chiếu văn bản, ghi kế thừa đơn vị cũ → mới, import CSV trường cho một tenant (FR-ORG-015, FR-ORG-016).

**Why this priority**: Seed đã có; màn quản trị cần trước production.

**Independent Test**: Sửa tên tỉnh, tạo kế thừa, import CSV dry-run có dòng lỗi rồi áp dụng.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F057-AC-11 | KHI System Admin có `platform.catalog.write` mở Quốc gia, Portal PHẢI hiển thị `listCountries` và chỉ cho bật/tắt `status`, sửa tên song ngữ qua `updateCountry` kèm lý do; không có thao tác xóa | FR-ORG-015 · [ADMIN_PORTAL §3.16](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F057-AC-12 | KHI System Admin mở Tỉnh/thành, Portal PHẢI hiển thị `listProvinces` (`status=all`) có lọc trạng thái, tìm theo tên/mã, cột hiệu lực và `successor_code`; sửa tên/`official_code` qua `updateProvince`; thay đổi địa giới PHẢI tạo đơn vị mới (`createProvince`) và kế thừa (`createProvinceSuccession`), không sửa đè đơn vị cũ | FR-ORG-015 · ADR-0060 | TC-ORG-101 |
| F057-AC-13 | KHI System Admin import CSV trường cho một tenant đích, Portal PHẢI gọi `importSchools` (dry-run trước, `Idempotency-Key`), theo dõi `getSchoolImport`, hiển thị bảng lỗi theo dòng (`row`, `field`, `code`) và chỉ cho áp dụng khi người dùng xác nhận; danh mục trường `listCatalogSchools` lọc tenant, tỉnh, cấp học, trạng thái | FR-ORG-016 · [ADMIN_PORTAL §3.16](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-ORG-103 |

---

### User Story 6 — Cấu hình hệ thống: dọn học sinh, thời hạn chat gameplay, giờ giới nghiêm platform (Priority: P2)

System Admin đặt số ngày dọn học sinh không đăng nhập, duyệt lần dọn vượt ngưỡng, đặt thời hạn lưu chat gameplay (FR-AUTH-024) và giờ giới nghiêm platform `platform_curfew` (ADR-0078).

**Why this priority**: Worker dọn (F016) cần cấu hình và người duyệt; nếu không lần chạy vượt ngưỡng bị treo.

**Independent Test**: Đổi cài đặt có lý do; giả lập lần chạy `awaiting_approval` và duyệt.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F057-AC-14 | KHI System Admin có `platform.settings.write` lưu cài đặt dọn học sinh, Portal PHẢI gửi `putStudentCleanupSettings` gồm `default_days` (cho phép tắt), `notice_days`, `allow_tenant_override`, `approval_threshold_percent` và `reason` bắt buộc trong giới hạn schema, hiển thị cảnh báo khi số ngày thấp hơn ngưỡng cảnh báo của [ADMIN_PORTAL §3.10](../../docs/04-admin-portal/ADMIN_PORTAL.md) | FR-AUTH-024 · ADR-0032 | TC-AUTH-020 |
| F057-AC-15 | KHI System Admin có `platform.student_cleanup.approve` mở "Lần dọn chờ xác nhận", Portal PHẢI liệt kê `listStudentCleanupRuns` theo tenant với số ứng viên/đã dọn/loại trừ/bỏ qua, và nút Xác nhận gọi `approveStudentCleanupRun` (`Idempotency-Key`); nút Hủy dùng `cancelStudentCleanupRun` sau delta | FR-AUTH-024 · ADR-0032 | TC-AUTH-021 |
| F057-AC-16 | KHI System Admin lưu thời hạn chat gameplay, Portal PHẢI gửi `putChatRetentionSettings` với `reason`, chặn giá trị dưới `minimum_days` do server trả và hiển thị `422 RETENTION_BELOW_MINIMUM` theo `code` | ADR-0033 · [ADMIN_PORTAL §3.10](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F057-AC-23 | KHI System Admin có `platform.settings.write` mở Cấu hình hệ thống, Portal PHẢI hiển thị khối "Giờ giới nghiêm platform" từ `getPlatformCurfewSettings` (giờ ICT, nhãn "mặc định" khi `is_default`, `warning_minutes` chỉ đọc); khi lưu PHẢI kiểm `start`/`end` dạng HH:MM (khung qua nửa đêm hiển thị "đến … hôm sau"), bắt buộc `reason`, hiện hộp xác nhận nêu tác động (chặn đăng nhập game, game session và chat của mọi học sinh mọi tenant trong khung) rồi gửi `putPlatformCurfewSettings` với `If-Match`; `412`/`422` PHẢI hiển thị theo `code` tại đúng ô | NFR-SAFE-005 · [CHILD_SAFETY §4.1](../../docs/10-security/CHILD_SAFETY.md#41-giờ-giới-nghiêm-cấp-platform-adr-0078) · ADR-0078 | TC-AUTH-092 |

---

### User Story 7 — Quản lý tenant (Priority: P3)

System Admin tạo tenant trường mới, tắt tenant ngừng hợp đồng.

**Why this priority**: Tenant ban đầu do seed/provisioning; màn tạo/sửa cần delta contract.

**Independent Test**: Danh sách tenant lọc loại; tạo tenant trùng mã → lỗi.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F057-AC-17 | KHI System Admin có `platform.tenant.manage` mở Tenant, Portal PHẢI hiển thị `listTenants` (lọc `q`, `tenant_type`) và cho tạo/sửa/kích hoạt/tắt qua `createTenant`/`updateTenant` (delta) với mã ≤ `maxLength` của schema, xác nhận khi tắt, hiển thị `409 CONFLICT` khi trùng mã | FR-ORG-001 · [ADMIN_PORTAL §10.1](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |

---

### User Story 8 — Bảo vệ workspace D (Priority: P1)

**Why this priority**: Workspace platform-only; constitution điều V.

**Independent Test**: `ad.truong.a`, `cm.noidung` mở URL workspace D; `sa.admin` không MFA/ngoài VPN.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F057-AC-18 | KHI người dùng không có role `system_admin` (theo `getMyPermissions`) mở URL workspace D, Portal PHẢI hiển thị trang 403 và không tải chunk workspace D | FR-CMS-011 · ADR-0027 · AGENTS R20 | TC-CMS-AUTHZ-05 |
| F057-AC-19 | NẾU API trả `403 MFA_REQUIRED`, THÌ Portal PHẢI chuyển người dùng sang đăng nhập lại có MFA (step-up) và quay về URL cũ; NẾU request tới gateway nội bộ lỗi kết nối, THÌ Portal PHẢI hiển thị "Cần kết nối mạng quản trị (VPN)" thay cho lỗi chung | FR-CMS-011 · [ADMIN_PORTAL §1.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-WS-02 |
| F057-AC-20 | HỆ THỐNG PHẢI gửi `Idempotency-Key` cho mọi thao tác ghi của workspace D (gán, gỡ gán, clone, khôi phục, import, cài đặt, duyệt) và giữ nguyên khi thử lại; thông điệp lỗi theo `code` kèm `trace_id` | AGENTS R10 · ADR-0010 | TC-CMS-ASG-01 |
| F057-AC-21 | HỆ THỐNG PHẢI giới hạn mọi ô nhập (lý do, mã, tên, tìm kiếm) theo `maxLength` của schema OpenAPI và hiển thị bộ đếm ký tự | ADR-0041 | TC-TEN-024 |
| F057-AC-22 | KHI menu/action của feature (`menu.platform_admin.tenants`, `menu.platform_admin.tenant_data`, `menu.platform_admin.trash`, `menu.platform_admin.catalog`, `menu.platform_admin.system_settings` và action con) đang tắt cho phạm vi phiên (`disabled_menus`/`disabled_actions` của `getMyPermissions`), Portal PHẢI ẩn menu và ẩn/khóa nút tương ứng (`<ActionGate actionKey>`); NẾU API trả `403 FEATURE_DISABLED`, Portal PHẢI hiện thông báo tính năng tạm tắt và tải lại quyền | FR-CMS-018 · ADR-0090 | TC-CMS-MENU-02 |

### Edge Cases

- NẾU `listTenants` trả nhiều trang, THÌ `TenantFilter` PHẢI tải tiếp theo cursor khi cuộn, không tải hết.
- NẾU tenant đang được chọn trong bộ lọc bị tắt, THÌ vẫn hiển thị trong lựa chọn với nhãn "đã tắt".
- NẾU job clone `partially_failed`, THÌ Portal PHẢI hiện danh sách lỗi và không tự chạy lại (F057-AC-07).
- NẾU người dùng mở workspace D trên điện thoại, THÌ màn gán/clone/tenant hiển thị "Tác vụ này cần màn hình lớn hơn" và chỉ đọc ([ADMIN_PORTAL §1.5](../../docs/04-admin-portal/ADMIN_PORTAL.md)).
- NẾU khôi phục bản ghi có quan hệ cha đã xóa, THÌ hiển thị lỗi theo `code` server trả.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CMS-013 | `TenantFilter`, cột Tenant, chọn tenant đích khi ghi | [ADMIN_PORTAL §2.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-ORG-008 | Một tenant mỗi phiên với người dùng tenant; System Admin xem liên tenant | [ADMIN_PORTAL §2.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-015 | UI gán và clone | [ADMIN_PORTAL §3.7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-ORG-012 | Gán dữ liệu platform cho tenant, gỡ gán | [ADMIN_PORTAL §3.7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-ORG-013 | Clone giữa tenant, không dữ liệu học sinh | [ADMIN_PORTAL §3.7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-014 | Thùng rác mọi tenant + platform | [ADMIN_PORTAL §3.8](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-ORG-015 | Danh mục quốc gia, tỉnh/thành, kế thừa | [ADMIN_PORTAL §3.16](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-ORG-016 | Danh mục trường liên tenant, import CSV | [ADMIN_PORTAL §3.16](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-AUTH-024 | Cấu hình và duyệt dọn học sinh (phía platform) | [ADMIN_PORTAL §3.10](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-ORG-001 | Quản lý tenant `school`/`unit`/`common` | [MULTITENANCY](../../docs/06-architecture/MULTITENANCY.md) |
| FR-CMS-018 | Menu/action của feature theo danh mục `specs/portal/menu-catalog.yaml`, ẩn khi tắt, xử lý `403 FEATURE_DISABLED` | [ADMIN_PORTAL §2.7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |

### Key Entities

- **Tenant**: mã, tên, loại, trạng thái, số trường.
- **TenantAssignment**: bản ghi platform được gán cho một tenant (chỉ đọc với tenant).
- **TenantCloneJob**: job clone bất đồng bộ, ánh xạ ID nguồn → bản sao, lỗi.
- **TrashItem**: bản ghi đã xóa mềm, có `restorable`.
- **Country / Province / ProvinceSuccession**: danh mục hành chính có hiệu lực và kế thừa.
- **SchoolImportJob**: job import CSV trường (dry-run/áp dụng).
- **StudentCleanupSettings / StudentCleanupRun**: cấu hình và lần chạy dọn học sinh.
- **ChatRetentionSettings**: thời hạn lưu chat gameplay.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `inactive_student_cleanup_days` | Giá trị mặc định hiển thị/sửa ở cài đặt dọn học sinh |
| `gameplay_chat_retention_days` | Thời hạn chat gameplay (tối thiểu do server trả) |
| `portal_session_sso` | Phiên platform, step-up MFA |
| `identifier_uniqueness` | Thông điệp trùng mã khi tạo tenant |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Workspace D chỉ gọi gateway nội bộ, token MFA; chunk lazy-load không phải lớp bảo mật | AGENTS R20 · ADR-0027 |
| Không hardcode URL gateway; lấy từ cấu hình môi trường | AGENTS R3 |
| UI theo Admin/Ops design system, không component game | AGENTS R32 · ADR-0047 |
| Gán/clone chỉ System Admin; clone không dữ liệu học sinh | AGENTS R22 · ADR-0029 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: System Admin lọc dữ liệu theo tenant bất kỳ trong ≤ 3 thao tác trên mọi màn liên tenant.
- **SC-002**: Gán một bản ghi platform cho 10 tenant trong một thao tác; tenant thấy dữ liệu ngay khi tải lại.
- **SC-003**: 0 request có `tenant_ids` từ người dùng không phải System Admin trong bộ E2E.
- **SC-004**: 0 người dùng không phải System Admin tải được chunk workspace D (kiểm network log).

## Ngoài phạm vi

- Backend gán/clone/thùng rác/đọc liên tenant: F045; catalog/tenant API: F010; dọn học sinh worker: F016; purge chat: F117.
- Danh mục từ nhạy cảm, công tắc kiểm duyệt (workspace D → An toàn chat): F122.
- Người dùng, Role platform, UserRole mọi tenant, Permission catalog, audit toàn hệ thống, login events liên tenant, feature flag, tích hợp Keycloak/SIS/APISIX (workspace D): chưa có feature trong danh mục — đề xuất bổ sung CODE_PLAN (F052 làm phần tenant ở workspace C).
- Học sinh sắp bị dọn và loại trừ ở cấp tenant (workspace C): F052.
- Shell, bộ chọn đơn vị nhân sự nhiều tenant: F051.

## Assumptions

- Thư mục `src/cms/frontend/apps/portal/src/workspaces/platform-admin/` (ADMIN_PORTAL §1.2), route `/portal/platform-admin/*`; `TenantFilter` đặt ở `src/cms/frontend/packages/ops-ui/src/tenant/` và gắn vào topbar của F051.
- [Proposed] Lựa chọn bộ lọc tenant lưu `sessionStorage` (theo phiên làm việc, per-viewer — ADMIN_PORTAL §2.5).
- [Proposed] Cảnh báo số ngày dọn thấp theo ngưỡng ADMIN_PORTAL §3.10 (chưa đăng ký CONFIG_DEFAULTS) — đề xuất đăng ký tên tham số.
- Màn danh sách dữ liệu platform để bấm "Gán cho tenant" là màn của workspace B/A (F053/F054, F079); F057 cung cấp hộp thoại gán và tab "Tenant được gán" dùng chung.
