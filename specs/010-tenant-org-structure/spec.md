# Feature Specification: F010 — Tenant, danh mục quốc gia/tỉnh/trường, tổ chức trường–lớp–enrollment

**Feature Branch**: `010-tenant-org-structure` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `identity` · **Mốc**: M-C1 · **Phụ thuộc**: F003, F004
**Input**: Mô hình tenant (`school`/`unit`/`common`) gắn một Keycloak Organization, danh mục quốc gia/tỉnh/trường, năm học–lớp–học sinh–enrollment có lịch sử, tenant chung do platform tạo. FR nguồn: FR-ORG-001, FR-ORG-002, FR-ORG-003, FR-ORG-005, FR-ORG-009, FR-ORG-010, FR-ORG-015, FR-ORG-016, FR-ORG-017, FR-AUTH-001, FR-AUTH-021.

## Clarifications

### Session 2026-11-16

- Q: Ai tạo Keycloak Organization và user của học sinh tenant chung? → A: module identity (worker `keycloak-membership`/`keycloak-provisioner`) là module duy nhất gọi Keycloak Admin API ([KEYCLOAK_SSO §2](../../docs/09-identity/KEYCLOAK_SSO.md)); adapter Keycloak do F010 tạo, F011/F013/F016 dùng lại.
- Q: API tạo/sửa tenant, trường thuộc tenant, tạo học sinh tenant chung chưa có trong spec? → A: Thêm ở Phase 2 (contract-first) theo [API_DESIGN §9.16](../../docs/07-backend/API_DESIGN.md); xem [contracts](contracts/README.md) §6.
- Q: Số lớp ảo tenant chung? → A: `common-g1` … `common-g9`, School ảo duy nhất, seed bằng migration ([MULTITENANCY §8.3](../../docs/06-architecture/MULTITENANCY.md)) [Proposed].

### Session 2026-11-17

- Q: Mã lỗi khi vượt phạm vi trường/môn của UserRole? → A: `403 PERMISSION_DENIED`; `FORBIDDEN_SCOPE` không phát và sẽ bị gỡ khỏi `ProblemCode` ([API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md), RB-113).
- Q: Operation facade phân trang để rebuild projection `ref.*` (`listProjectionSnapshot`, F020 yêu cầu) thuộc feature nào? → A: F010 sở hữu (module identity là nguồn tenant/trường/lớp/học sinh/enrollment; [DATABASE_DESIGN §2.17](../../docs/07-backend/DATABASE_DESIGN.md) mục 3, BACKUP_RECOVERY §3); khai báo trong `specs/openapi/modules/identity.internal.v1.yaml`, `x-allowed-callers` `learning`, `game`, `competition`, `chat`; bên gọi (F020, F014, F091, F095) chỉ hiện thực consumer/job rebuild.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — System Admin tạo và quản lý tenant (Priority: P1)

System Admin (`sa.admin`) tạo tenant loại trường độc lập, đơn vị nhiều trường hoặc tenant chung, bật/tắt tenant; hệ thống tạo/cập nhật Keycloak Organization tương ứng.

**Why this priority**: Tenant là đơn vị cô lập dữ liệu và đơn vị đăng nhập; mọi feature khác cần tenant tồn tại.

**Independent Test**: Gọi API tạo tenant `th-demo-x` loại `school`, kiểm tra bản ghi `platform.tenants`, Organization trong Keycloak có alias và attribute `tenant_id`; tắt tenant → Organization `enabled=false`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F010-AC-01 | KHI System Admin tạo tenant, module identity PHẢI lưu tenant với `tenant_type ∈ {school, unit, common}`, `keycloak_org_alias` duy nhất khớp mẫu `^[a-z0-9-]{3,63}$`, phát `tenant.created` và worker PHẢI tạo đúng một Keycloak Organization có attribute `tenant_id`, `tenant_type` | FR-ORG-001 · FR-ORG-009 · [KEYCLOAK_SSO §2.1](../../docs/09-identity/KEYCLOAK_SSO.md) | — |
| F010-AC-02 | NẾU đã tồn tại một tenant `common`, THÌ module identity PHẢI từ chối tạo tenant `common` thứ hai với `409 CONFLICT` | FR-ORG-001 · [DATABASE_DESIGN §2.0](../../docs/07-backend/DATABASE_DESIGN.md) | — |
| F010-AC-03 | KHI tenant chuyển `status = disabled`, worker PHẢI đặt Organization `enabled=false` và mọi thành viên PHẢI nhận `403 TENANT_ACCESS_DISABLED` ở lần resolve kế tiếp | FR-ORG-009 · AC-AUTH-010 | TC-TEN-007 |
| F010-AC-04 | HỆ THỐNG PHẢI chỉ cho người có permission `platform.tenant.manage` (token `eduworld-portal` + MFA, gateway nội bộ) tạo/sửa tenant; người dùng tenant nhận `403 PERMISSION_DENIED` | FR-ORG-001 · ADR-0027 | TC-CMS-WS-02 |
| F010-AC-05 | KHI tạo/sửa tenant hoặc trường, module identity PHẢI yêu cầu `Idempotency-Key`, ghi audit `tenant.created`/`tenant.updated` và từ chối chuỗi vượt `maxLength` với `422 VALIDATION_FAILED` chỉ rõ trường | FR-ORG-001 · ADR-0041 | TC-TEN-024 |

---

### User Story 2 — Danh mục quốc gia, tỉnh/thành và trường (Priority: P1)

System Admin quản lý danh mục quốc gia/tỉnh (hiệu lực, kế thừa) và import trường vào tenant; Tenant Admin xem/sửa thông tin hiển thị và duyệt trường `pending_review`.

**Why this priority**: Trường là chiều bên trong tenant; SIS và báo cáo cần danh mục chuẩn.

**Independent Test**: Seed `specs/reference-data`, `GET /catalog/provinces?country=VN` trả 34 đơn vị `active`; import CSV trường có mã tỉnh cũ → ánh xạ sang đơn vị kế thừa.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F010-AC-06 | HỆ THỐNG PHẢI seed danh mục từ `specs/reference-data` gồm đúng 34 tỉnh/thành Việt Nam `active`, 63 đơn vị cũ `inactive`, mỗi đơn vị cũ có đúng một kế thừa tới đơn vị `active` | FR-ORG-015 · ADR-0060 | TC-ORG-101 |
| F010-AC-07 | NẾU tạo/sửa trường với tỉnh/thành `inactive`, THÌ module identity PHẢI trả `422 VALIDATION_FAILED` | FR-ORG-016 | TC-ORG-102 |
| F010-AC-08 | KHI import trường có `province_code` của đơn vị cũ, module identity PHẢI ánh xạ sang đơn vị hiện hành qua `catalog.province_successions` và ghi cảnh báo `province_outdated`; thiếu mã tỉnh PHẢI đặt trường `pending_review` | FR-ORG-016 · [SIS_INTEGRATION §4.6](../../docs/09-identity/SIS_INTEGRATION.md) | TC-ORG-103 |
| F010-AC-09 | KHI Tenant Admin duyệt trường `pending_review` có chọn tỉnh `active`, module identity PHẢI chuyển trường sang `active`, ghi audit và phát `school.updated` | FR-ORG-016 | TC-ORG-104 |
| F010-AC-10 | HỆ THỐNG PHẢI chỉ trả trường thuộc tenant của phiên cho người dùng tenant; System Admin lọc bằng `tenant_ids` và mọi lần xem liên tenant PHẢI ghi audit `cross_tenant.accessed` | FR-ORG-016 · FR-ORG-001 | TC-TEN-018 |

---

### User Story 3 — Năm học, lớp, enrollment và chuyển lớp/chuyển trường (Priority: P1)

Hệ thống giữ cấu trúc trường–năm học–lớp–học sinh; mỗi học sinh đúng một enrollment active; chuyển lớp/trường đóng enrollment cũ và mở enrollment mới trong cùng transaction, giữ lịch sử.

**Why this priority**: Đăng nhập game (F013) yêu cầu học sinh có một trường, một lớp.

**Independent Test**: Áp chuyển lớp cho `hs.an.3a` (3A → 3B) và chuyển trường `hs.em.4a` trong tenant `pgd-demo`; truy vấn ở mọi thời điểm thấy đúng một lớp, tenant không đổi.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F010-AC-11 | HỆ THỐNG PHẢI bảo đảm mỗi học sinh có tối đa một enrollment `active`; hai lệnh áp đồng thời PHẢI chỉ một thành công | FR-ORG-017 · FR-ORG-002 | TC-ORG-105 |
| F010-AC-12 | NẾU enrollment trỏ lớp thuộc trường khác với trường của enrollment, THÌ HỆ THỐNG PHẢI từ chối (FK ghép `(class_id, school_id)`) | FR-ORG-017 | TC-ORG-106 |
| F010-AC-13 | KHI học sinh chuyển lớp, module identity PHẢI đóng enrollment cũ (`transferred`, `end_date`) và mở enrollment mới trong cùng transaction, phát `enrollment.changed` | FR-ORG-005 | TC-ORG-107 |
| F010-AC-14 | KHI học sinh chuyển trường trong cùng tenant `unit`, module identity PHẢI giữ `tenant_id`, membership Keycloak và phiên; chỉ đổi `school_id` và enrollment | FR-ORG-005 · FR-ORG-010 | TC-TEN-009 |
| F010-AC-15 | KHI học sinh chuyển sang trường thuộc tenant khác, module identity PHẢI giữ `user_id`, phát `tenant_membership.changed` (removed rồi added) và dữ liệu lịch sử PHẢI giữ `tenant_id` cũ | FR-ORG-005 · AC-AUTH-013 | TC-AUTH-008 |
| F010-AC-16 | HỆ THỐNG PHẢI chỉ chấp nhận `user_type ∈ {student, staff}`; học sinh luôn có `primary_tenant_id` | FR-ORG-003 · ADR-0081 | — |
| F010-AC-17 | NẾU người dùng tenant A gọi API lớp/học sinh/trường bằng ID của tenant B, THÌ HỆ THỐNG PHẢI trả `404` (không tiết lộ tồn tại) | FR-ORG-001 · NFR-SEC-006 | TC-TEN-001 |
| F010-AC-18 | KHI School Admin trường A gọi danh sách lớp của trường B cùng tenant, HỆ THỐNG PHẢI trả `403 PERMISSION_DENIED` hoặc `404` | FR-ORG-001 | TC-TEN-003 |
| F010-AC-19 | KHI học sinh xem hồ sơ của mình (`getMyStudentProfile`), HỆ THỐNG PHẢI trả trường/lớp/tỉnh song ngữ; người chơi khác không nhận các trường này | FR-ORG-016 · AGENTS R16 | TC-ORG-109 |
| F010-AC-27 | KHI module có trong `x-allowed-callers` gọi facade `listProjectionSnapshot` (`entity_type` ∈ `tenant`, `school`, `class`, `student`, `enrollment`; `tenant_id` hoặc phạm vi platform cho job; `cursor`, `limit` ≤ 1000), module identity PHẢI trả trang trạng thái hiện hành gồm đúng các trường của payload event projection tương ứng kèm `source_version` (bản ghi soft-delete trả cờ `deleted`), đọc read model không ghi DB, không có PII ngoài trường của payload event; module không có trong `x-allowed-callers` PHẢI bị `make arch-check` chặn | [DATABASE_DESIGN §2.17](../../docs/07-backend/DATABASE_DESIGN.md) · ADR-0074 · ADR-0042 · AGENTS R13 | — |

---

### User Story 4 — Membership Keycloak Organization đồng bộ tự động (Priority: P1)

Worker đồng bộ membership từ dữ liệu EduWorld: học sinh member đúng một tenant, nhân sự member các tenant có UserRole hiệu lực; reconciliation hằng đêm gỡ member thêm tay.

**Why this priority**: Membership là điều kiện cho chọn tenant (portal) và phát hiện cấu hình sai.

**Independent Test**: Phát `tenant_membership.changed` cho một học sinh; kiểm Keycloak gỡ org cũ trước, thêm org mới sau, gọi logout user. Thêm tay một member → job reconciliation gỡ và tạo issue.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F010-AC-20 | KHI nhận `tenant_membership.changed`, worker PHẢI gỡ member khỏi Organization cũ, sau đó thêm vào Organization mới, rồi thu hồi phiên Keycloak của user (thứ tự bắt buộc) và phát `user.access_revoked` | FR-ORG-010 · [ACCOUNT_PROVISIONING §2.1](../../docs/09-identity/ACCOUNT_PROVISIONING.md) | TC-TEN-009 |
| F010-AC-21 | KHI nhận `user_role.changed`, worker PHẢI thêm nhân sự vào mọi Organization nơi có UserRole hiệu lực, gỡ khỏi tenant không còn, gán/gỡ realm role `tenant_staff`/`platform_staff` | FR-ORG-010 · ADR-0066 | — |
| F010-AC-22 | KHI job reconciliation phát hiện member Organization không có membership trong DB, worker PHẢI gỡ member và tạo `reconciliation_issue` `unexpected_org_member`, ghi audit | FR-ORG-010 | TC-TEN-010 |
| F010-AC-23 | NẾU Keycloak Admin API lỗi, THÌ consumer PHẢI thử lại có backoff (không ack), sau số lần tối đa chuyển `DLQ_IDENTITY`, và không ghi trạng thái giả vào DB | FR-ORG-010 · ADR-0042 | TC-SVC-003 |

---

### User Story 5 — Tenant chung và tài khoản học sinh do platform tạo (Priority: P2)

System Admin tạo/import có chủ đích học sinh vào tenant `common` với khối 1–9; học sinh vào lớp ảo theo khối; không có tự đăng ký.

**Why this priority**: Cần cho học sinh ngoài trường ký hợp đồng; không chặn luồng chính.

**Independent Test**: Tạo `hs.tudo` khối 4 trong `common` → `org.students` thuộc School ảo, enrollment lớp `common-g4`, Keycloak user member Organization `common`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F010-AC-24 | KHI System Admin tạo học sinh tenant chung với khối n, module identity PHẢI tạo `identity.users` (`student`, `primary_tenant_id = common`), `org.students` thuộc School ảo, enrollment lớp ảo `common-g<n>`, Keycloak local user member Organization `common`, audit `user.provisioned` với `source = platform_common` | FR-AUTH-021 · AC-AUTH-011 | TC-ORG-108 |
| F010-AC-25 | NẾU yêu cầu tạo học sinh tenant chung thiếu khối hoặc khối ngoài 1–9, THÌ module identity PHẢI trả `422 VALIDATION_FAILED` | FR-AUTH-021 · FR-ORG-017 | — |
| F010-AC-26 | HỆ THỐNG PHẢI không có endpoint nào cho phép tự đăng ký hoặc tự tạo tài khoản học sinh từ client game; không bao giờ tự đưa người dùng không khớp tenant nào vào `common` | FR-AUTH-001 · FR-AUTH-021 · ADR-0063 | TC-TEN-014 |

### Edge Cases

- NẾU tenant `school` được thêm trường thứ hai, THÌ HỆ THỐNG PHẢI từ chối `422 VALIDATION_FAILED` (tenant `school` có đúng một trường).
- NẾU `moet_code` trùng trường khác, THÌ HỆ THỐNG PHẢI không tự gộp và tạo issue `school_duplicate`.
- NẾU học sinh rời trường cũ (khác tenant) trước khi trường mới đồng bộ, THÌ học sinh không còn enrollment active; đăng nhập xử lý ở F013 (`NO_ACTIVE_ENROLLMENT`/`ACCOUNT_NOT_PROVISIONED`).
- NẾU xóa lớp (soft-delete), THÌ lớp không xuất hiện ở API đọc và tạo lại cùng tên được (partial unique) — TC-TEN-015.
- NẾU ent query thiếu `TenantScope`, THÌ trả `ErrMissingTenantScope`, không trả dữ liệu (TC-TEN-021).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-ORG-001 | Phân cấp Platform → Tenant → School → Class; CRUD tenant, cô lập theo tenant | [MULTITENANCY §2](../../docs/06-architecture/MULTITENANCY.md) |
| FR-ORG-002 | AcademicYear, Class (grade 1–9), Enrollment có lịch sử | [DATABASE_DESIGN §2.2](../../docs/07-backend/DATABASE_DESIGN.md) |
| FR-ORG-003 | Thực thể User/Student/Staff/School/Class/Enrollment/ExternalIdentity; không có thực thể giáo viên | [IDENTITY_MAPPING §2](../../docs/09-identity/IDENTITY_MAPPING.md) |
| FR-ORG-005 | Chuyển lớp, chuyển trường (cùng tenant `unit` / khác tenant) | [IDENTITY_MAPPING §6](../../docs/09-identity/IDENTITY_MAPPING.md) |
| FR-ORG-009 | Mỗi tenant là một Keycloak Organization | [KEYCLOAK_SSO §2.1](../../docs/09-identity/KEYCLOAK_SSO.md) |
| FR-ORG-010 | Đồng bộ membership Organization + reconciliation | [ACCOUNT_PROVISIONING §2.1](../../docs/09-identity/ACCOUNT_PROVISIONING.md) |
| FR-ORG-015 | Danh mục quốc gia, tỉnh/thành có hiệu lực và kế thừa | [ADMIN_PORTAL §3.16](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-ORG-016 | Danh mục trường, import CSV, duyệt `pending_review` | [ADMIN_PORTAL §3.16](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-ORG-017 | Một trường, một lớp, một enrollment active; lớp ảo tenant chung | [ADR-0060](../../docs/adr/ADR-0060-danh-muc-quoc-gia-tinh-thanh-truong-hoc-sinh-thuoc-mot-truong-mot-lop.md) |
| FR-AUTH-001 | Không có self-registration; tài khoản do nhà trường/platform cấp | [KEYCLOAK_SSO §1](../../docs/09-identity/KEYCLOAK_SSO.md) |
| FR-AUTH-021 | Tài khoản tenant chung chỉ do System Admin tạo/import, kèm khối và lớp ảo | [ACCOUNT_PROVISIONING §3.4](../../docs/09-identity/ACCOUNT_PROVISIONING.md) |

### Key Entities

- **Tenant**: đơn vị cô lập dữ liệu và đăng nhập; loại `school`/`unit`/`common`; trạng thái `active`/`disabled`; khóa nối Keycloak Organization.
- **Country, Province, ProvinceSuccession**: danh mục platform có hiệu lực, kế thừa đơn vị cũ → mới.
- **School**: thuộc đúng một tenant và một tỉnh; trạng thái `active`/`pending_review`/`inactive`; có School ảo cho tenant chung.
- **AcademicYear, Class**: lớp theo năm học, `grade_level` 1–9, lớp ảo theo khối cho tenant chung.
- **User, Student**: User toàn cục (`student`/`staff`); Student = User học sinh gắn tenant, trường hiện tại.
- **Enrollment**: học sinh ↔ lớp, có lịch sử, tối đa một `active`.
- **TenantMembership**: suy ra từ enrollment active (học sinh) và UserRole hiệu lực (nhân sự).

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `principal_cache` | TTL cache tenant/membership khi invalidate theo `tenant.updated`, `tenant_membership.changed` |
| `supported_locales`, `tenant_default_locale` | Tên song ngữ trường/tỉnh, locale mặc định của tenant |
| `csv_import_max` | Giới hạn file import trường |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Mọi bảng tenant-scoped có `tenant_id` + RLS + `deleted_at`; chỉ soft-delete | NFR-SEC-006 · AGENTS R8, R21, R27 · ADR-0005, ADR-0028, ADR-0039 |
| Mọi chuỗi có `MaxLen`/`maxLength` | AGENTS R28 · ADR-0041 |
| Chỉ module identity gọi Keycloak Admin API (client `svc-identity`) | ADR-0074 · AGENTS R13 |
| Thao tác ghi quan trọng có `Idempotency-Key` + audit | NFR-SEC-004 · AGENTS R10 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% endpoint tenant-scoped mới của feature qua test tenant isolation (tenant A không đọc/ghi tenant B).
- **SC-002**: Tạo tenant mới hiển thị Organization tương ứng trong Keycloak trong ≤ 60 giây sau commit.
- **SC-003**: 0 trường hợp học sinh có hai enrollment active dưới tải đồng thời trong integration test.
- **SC-004**: Seed danh mục đạt `validate-reference-data` (34/63/kế thừa đầy đủ) ở mọi môi trường.

## Ngoài phạm vi

- SIS sync, provisioning học sinh từ SIS, reconciliation dữ liệu SIS → F011.
- Role/UserRole, `/me/*`, `principals:resolve` → F012; đăng nhập game và xác định tenant khi đăng nhập → F013.
- Cài đặt trường/tenant (`settings`) → F015; khóa/dọn/ẩn danh hóa tài khoản → F016.
- Gán/clone dữ liệu giữa tenant, Thùng rác liên tenant → F045. Màn hình portal → F052, F057.
- Cấp xã/phường; đăng nhập theo mã trường (`login_code`) — mở rộng sau (ADR-0063).

## Assumptions

- F003 cung cấp mixin `Base`/`TenantOptional`/`HybridContent`/`GlobalBase` (ADR-0089), interceptor theo mixin, outbox; F004 cung cấp realm và client `eduworld-sis-sync` cho Admin API.
- Mã tỉnh chính thức (`official_code`) [TBD] đến khi đối chiếu văn bản; seed dùng `code` nội bộ.
- Lớp ảo `common-g1…g9` và School ảo [Proposed] theo MULTITENANCY §8.3.
