# Feature Specification: F052 — Workspace C — quản trị trường: trường/lớp/học sinh, SIS, người dùng và phân quyền, cài đặt, audit

**Feature Branch**: `052-school-admin-workspace` · **Created**: 2026-11-16 · **Status**: Done · **Module**: `src/cms/frontend/apps/portal` (workspace `school_lms`) · **Mốc**: M-C3 · **Phụ thuộc**: F011, F015, F016, F051
**Input**: Phần quản trị của workspace C (School & Learning Management) cho Tenant Admin, School Admin, Academic Manager và System Admin (một tenant đích): trường, lớp, học sinh SIS (chỉ đọc), nhập CSV SIS và xung đột đồng bộ, người dùng/Role/UserRole, cài đặt tenant/trường, dọn học sinh không đăng nhập, hoạt động đăng nhập game, nhật ký audit. FR nguồn: FR-AUTH-024, FR-AUTH-032, FR-ORG-007, FR-ORG-016, FR-SIS-005, FR-AUD-001, FR-GAME-013.

## Clarifications

### Session 2026-11-16

- Q: Màn "Xung đột đồng bộ" gọi API nào khi `/api/v1/sis/*` chỉ có ở gateway nội bộ ([SIS_INTEGRATION §9–§10](../../docs/09-identity/SIS_INTEGRATION.md)) mà School Admin chỉ dùng gateway public? → A: Dùng route public theo trường giống `createSisImport`; contract "Cần thêm" `listSchoolReconciliationIssues`, `resolveSchoolReconciliationIssue` ([contracts](contracts/README.md) §6). `missing_class` đã có qua `listUnassignedStudents`.
- Q: Danh sách học sinh của lớp? → A: Contract chưa có; "Cần thêm" `listTenantClassStudents` (chỉ đọc, kèm `last_login_at`).
- Q: Cài đặt trường (`FR-ORG-007`)? → A: `getSchoolSettings`/`updateSchoolSettings` (`GET`/`PATCH /schools/{school_id}/settings`, backend F015) đã có trong spec gộp (2026-11-17). Chat policy trường thuộc F122.
- Q: Màn cài đặt giờ giới nghiêm cho nhà trường (ADR-0078, RB-803) thuộc feature nào? → A: Khung giới nghiêm **cấp trường** (School Admin, chỉ mở rộng) thuộc F052 trong màn Cài đặt trường (`menu.school_lms.settings`, action `action.school_lms.settings.edit_school`); không có khung giới nghiêm cấp tenant ([CHILD_SAFETY §4.1](../../docs/10-security/CHILD_SAFETY.md#41-giờ-giới-nghiêm-cấp-platform-adr-0078)); `platform_curfew` do System Admin đặt ở F057. Kiểm "chỉ siết thêm" ở server (F015-AC-22, `422 VALIDATION_FAILED`); portal chỉ kiểm trước để báo sớm.
- Q: UserRole có scope `class`? → A: Không (ADR-0072); portal chỉ cho Đơn vị/Trường/Môn dù enum contract còn `class` (mâu thuẫn D18, ghi ở contracts §6).

### Session 2026-11-17

- Q: Mã lỗi khi vượt phạm vi trường/môn của UserRole? → A: `403 PERMISSION_DENIED`; `FORBIDDEN_SCOPE` không phát và sẽ bị gỡ khỏi `ProblemCode` ([API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md), RB-113).

### Session 2026-11-19 (triển khai)

- Q: Schema của xung đột đồng bộ theo trường? → A: [Confirmed] Dùng lại `SisReconciliationIssue`/`SisReconciliationIssuePage` của F011 (một nguồn, R33) thay cho schema mới `ReconciliationIssue`; body xử lý là `SchoolReconciliationResolve { resolution: fixed_in_source | ignore, reason (3–500) }` (portal trường không gộp tài khoản). `ClassStudent.source` dùng lại enum `User.source` (`sis`/`manual`/`import`).
- Q: Ba operation contract-only (`listTenantClassStudents`, `listSchoolReconciliationIssues`, `resolveSchoolReconciliationIssue`) đã có backend chưa? → A: Chưa — F052 chỉ thêm contract (T004–T005) và màn hình; backend thuộc owner F010/F011 (Ngoài phạm vi). Test tenant isolation/authorization phía backend của ba operation làm cùng backend (`routematrix` đã có dòng ma trận). **Câu hỏi mở F052-1.** — **Đã giải quyết 2026-11-20:** backend ba operation có trong module `identity` (owner F010: `listTenantClassStudents`; owner F011: `listSchoolReconciliationIssues`, `resolveSchoolReconciliationIssue` dùng lại use case `ResolveSisIssue` của F011), xem Clarifications 2026-11-20 của [F010](../010-tenant-org-structure/spec.md) và [F011](../011-sis-sync-provisioning/spec.md); test `TestSchoolScopedRoutes` (HTTP thật: tenant A/B → `404`, School Admin trường khác → `404`, không quyền → `403`, `Idempotency-Key` replay, audit). E2E Playwright với backend thật vẫn chờ OQ F051-9.
- Q: "Nhật ký phân quyền" lọc `role.*`, `user_role.*` thế nào khi `listTenantAuditLogs.action` so khớp tuyệt đối? → A: [Confirmed] Chọn một trong sáu action module identity ghi (`role.created/updated/deleted`, `user_role.granted/revoked/expired`), khoảng 90 ngày (API bắt buộc `from`/`to`); lọc theo tiền tố ở server là **OQ F052-2** (đã chốt 2026-11-20: dùng `action_prefix`, Session owner chốt).
- Q: Đối tượng của phạm vi "Môn"? → A: [Confirmed] Danh sách môn lấy từ `listSubjects` (API public chỉ đọc của module learning); UserRole vẫn chỉ gọi API identity.
- Q: Lý do chờ duyệt khi `School` không có trường lý do? → A: [Confirmed] Portal suy ra: thiếu tỉnh/thành (SIS không gửi) + thông điệp chung "trường mới từ SIS/CSV cần xác nhận". Trường `source = sis`: tên, cấp học, khối, địa chỉ chỉ đọc; tỉnh/thành sửa được khi SIS chưa gửi (ADMIN_PORTAL §3.16).
- Q: `sis_sync_schedule` hiển thị thế nào khi API public không trả? → A: [Confirmed] Màn nhập CSV chỉ ghi chú lịch đồng bộ do EduWorld cấu hình theo kết nối (không hardcode giá trị, R33). `dry_run` mặc định bật theo contract.
- Q: `ResponsiveTable` (plan ghi thuộc ops-ui) chưa có trong ops-ui? → A: [Confirmed] Đặt ở `workspaces/school-lms/common/ResponsiveTable.tsx`; chuyển sang ops-ui khi workspace thứ hai cần.
- Q: Test và E2E đặt ở đâu? → A: Theo tiền lệ F051: test đặt cạnh mã trong `src/workspaces/school-lms/**` (Vitest chỉ chạy `src/**/*.test.*`), dựng toàn portal với **MOCK backend** (`src/test/school-lms-mock.ts`); E2E Playwright `@F052` với backend thật chờ backend local chạy được (OQ F051-9) và ba endpoint ở trên.

### Session 2026-11-20 (owner chốt)

- OQ F052-2 → **Chốt:** backend đã có `action_prefix[]` (F015, OQ F059-3); "Nhật ký phân quyền" chuyển sang một truy vấn `action_prefix=role&action_prefix=user_role`, ô "Hành động" chỉ để thu hẹp. Task bổ sung.
- OQ F045-7 → **Chốt:** màn "Xung đột đồng bộ" có hành động "Đề nghị gộp" cho issue trùng định danh (gửi `resolveSchoolReconciliationIssue` `resolution = merge_requested` kèm lý do, quyền `tenant.sis.import`); School Admin không tự gộp — System Admin gộp ở workspace D. Bổ sung cho quyết định Session 2026-11-19 "portal trường không gộp tài khoản".
- OQ-17 giới hạn chơi/ngày (owner chốt 2026-11-20) → **Chốt:** Cài đặt trường thêm ô "Giới hạn chơi mỗi ngày" (tắt mặc định); thêm F052-AC-32.
- OQ F051-9 → E2E backend thật chạy khi có môi trường compose (không phải câu hỏi mở).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Trường, lớp, học sinh và hoạt động đăng nhập (Priority: P1)

Tenant Admin/School Admin xem trường trong tenant, sửa thông tin hiển thị, duyệt trường `pending_review`; xem lớp và sĩ số, học sinh (chỉ đọc, từ SIS), học sinh chưa có lớp; xem lịch sử đăng nhập game và số ngày chưa vào game.

**Why this priority**: Là dữ liệu nền nhà trường cần thấy ngay sau khi SIS đồng bộ (exit M-C1/M-C3).

**Independent Test**: `ad.truong.a` mở Trường → Lớp 3A → học sinh `hs.an.3a` → Hoạt động đăng nhập; `ad.truong.b` mở URL trường TH Demo A → "Không tìm thấy".

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F052-AC-01 | KHI người có `tenant.school.read` mở "Trường", Portal PHẢI liệt kê trường bằng `listTenantSchools` (lọc trạng thái `active`/`pending_review`/`inactive`, tên theo locale) và chỉ hiển thị trường API trả về. | FR-ORG-016 | — |
| F052-AC-02 | KHI người có `tenant.school.write` sửa thông tin trường, Portal PHẢI gửi `updateTenantSchool` kèm `If-Match`, chỉ cho chọn tỉnh/thành `active` (`listProvinces`) và hiển thị chỉ đọc các trường dữ liệu do SIS sở hữu. | FR-ORG-016 · ADR-0060 | TC-ORG-102 |
| F052-AC-03 | KHI duyệt trường `pending_review`, Portal PHẢI bắt buộc có tỉnh/thành trước khi gọi `approveTenantSchool` (kèm `If-Match`) và hiển thị lý do chờ duyệt. | FR-ORG-016, FR-SIS-005 | TC-ORG-104 |
| F052-AC-04 | KHI người có `tenant.class.read` mở lớp, Portal PHẢI hiển thị lớp theo năm học/khối với sĩ số (`listTenantSchoolClasses`) và danh sách học sinh chỉ đọc có nhãn "SIS", không có thao tác sửa định danh hay gán lớp tạm. | FR-ORG-016 · ADR-0060 | — |
| F052-AC-05 | KHI mở "Học sinh chưa có lớp", Portal PHẢI liệt kê `listUnassignedStudents` kèm hướng dẫn bổ sung lớp trong SIS/CSV và không có nút gán lớp. | FR-SIS-005 · ADR-0060 | TC-SIS-103 |
| F052-AC-06 | KHI người có `lms.student_activity.read` mở hoạt động đăng nhập của học sinh, Portal PHẢI hiển thị `listTenantStudentLoginEvents` (vào/ra, nền tảng, thiết bị rút gọn, lý do kết thúc) và số ngày chưa vào game, không hiển thị IP, `ip_hash` hay mã thiết bị thô. | FR-GAME-013, AC-AUTH-017 · ADR-0031 | — |
| F052-AC-07 | NẾU API trả 404 hoặc 403 cho trường/lớp/học sinh ngoài scope (ví dụ `ad.truong.b` mở trường TH Demo A), THÌ Portal PHẢI hiện trang "Không tìm thấy" và không tiết lộ tài nguyên tồn tại. | AC-ORG-001, AC-ORG-003 | TC-TEN-001, TC-TEN-003 |
| F052-AC-31 | KHI người có `lms.student.password_reset` bấm "Đặt lại mật khẩu" ở danh sách học sinh của lớp, Portal PHẢI yêu cầu `reason`, gọi `resetStudentPassword` với một `Idempotency-Key` cho mỗi lần xác nhận (dùng lại khi thử lại), hiển thị `initial_password` đúng một lần kèm nút in thẻ đăng nhập, không lưu mật khẩu vào bộ nhớ trình duyệt/log; NẾU response có `password_redacted = true` THÌ PHẢI báo mật khẩu không xem lại được và gợi ý đặt lại lần nữa; nút PHẢI ẩn khi thiếu permission hoặc action bị tắt | FR-AUTH-017 · AC-AUTH-028 · [ACCOUNT_PROVISIONING §6d](../../docs/09-identity/ACCOUNT_PROVISIONING.md#6d-đặt-lại-mật-khẩu-học-sinh) | TC-AUTH-087 |

---

### User Story 2 — Nhập CSV SIS và xử lý xung đột đồng bộ (Priority: P1)

School Admin tải file CSV theo EduWorld SIS Contract, theo dõi tiến trình, xem số dòng lỗi và các xung đột đồng bộ cần xử lý.

**Why this priority**: Nguồn dữ liệu học sinh/lớp duy nhất ở giai đoạn này là `csv_import` (OQ-01).

**Independent Test**: `ad.truong.a` tải CSV mẫu có một học sinh trùng username → import `completed_with_issues`, có issue `duplicate_identifier`, đánh dấu đã xử lý có lý do.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F052-AC-08 | KHI người có `tenant.sis.import` chọn file, Portal PHẢI kiểm tra đuôi `.csv` và kích thước ≤ `csv_import_max`, tính sha256 trên trình duyệt, gọi `createSisImport` (Idempotency-Key) rồi tải file lên `upload_url` trước `upload_expires_at`. | FR-SIS-005, US-SIS-001 · [SIS_INTEGRATION §6](../../docs/09-identity/SIS_INTEGRATION.md) | — |
| F052-AC-09 | TRONG KHI import ở trạng thái `awaiting_upload`/`queued`/`validating`/`applying`, Portal PHẢI theo dõi `getSisImport` và hiển thị `rows_total`, `rows_applied`, `rows_rejected`, `reconciliation_issue_count`; trạng thái `failed`/`expired` PHẢI có thông điệp và cho tải lại. | FR-SIS-005 | — |
| F052-AC-10 | KHI người có `tenant.sis.read` mở "Xung đột đồng bộ", Portal PHẢI liệt kê issue theo loại (`missing_class`, `duplicate_identifier`, `school_province_missing`, `school_duplicate`…) với hành động gợi ý, và cho người có `tenant.sis.import` đánh dấu đã xử lý kèm lý do bắt buộc. | FR-SIS-005, US-SIS-002 | TC-AUTH-074 |
| F052-AC-11 | NẾU người dùng gửi lại cùng một lần import hay cùng một thao tác xử lý issue (bấm hai lần, thử lại sau lỗi mạng), THÌ Portal PHẢI dùng lại cùng Idempotency-Key để server không tạo bản ghi thứ hai. | NFR-SEC-004 · AGENTS R10 | — |

---

### User Story 3 — Người dùng, Role, UserRole và nhật ký phân quyền (Priority: P2)

Tenant Admin quản lý nhân sự (tạo thủ công), role tùy biến [Confirmed] và gán/thu hồi UserRole; School Admin gán UserRole trong trường mình.

**Why this priority**: Cần trước khi trường tự vận hành; bootstrap ban đầu có thể do seed/System Admin.

**Independent Test**: `ad.donvi` tạo nhân sự trùng email (khác hoa thường) → lỗi 409 kèm gợi ý; gán role `academic_manager` scope Môn; thu hồi → nhật ký phân quyền có dòng `user_role.*`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F052-AC-12 | KHI người có `tenant.user.read` mở "Người dùng", Portal PHẢI liệt kê `listTenantUsers` (lọc `q`, `user_type`, `school_id`, `role_code`); học sinh hiển thị chỉ đọc có nhãn "SIS" và không có thao tác gán role portal. | FR-AUTH-032 · ADR-0066 | TC-CMS-RBAC-02 |
| F052-AC-13 | KHI người có `tenant.user.write` thêm nhân sự, Portal PHẢI bắt buộc Họ tên, Username, Email, ít nhất một role ban đầu kèm phạm vi, gọi `createTenantUser` (Idempotency-Key); NẾU nhận `409 USERNAME_TAKEN`/`409 EMAIL_TAKEN` THÌ hiện lỗi tại trường kèm gợi ý gán role cho tài khoản sẵn có. | FR-AUTH-032, AC-AUTH-026 · ADR-0069 | TC-AUTH-073, TC-CMS-LOGIN-04 |
| F052-AC-14 | Form role PHẢI chỉ liệt kê permission từ `listTenantPermissions` nhóm theo workspace; NẾU server trả `422 VALIDATION_FAILED` có `errors[].code = PERMISSION_NOT_ALLOWED_FOR_SCOPE` THÌ Portal PHẢI đánh dấu lỗi tại permission đó. | FR-AUTH-032 · ADR-0027 | TC-CMS-RBAC-01, TC-AUTH-076 |
| F052-AC-15 | Portal PHẢI hiển thị role `is_system` chỉ đọc với nút "Nhân bản thành role tùy biến" thay cho Sửa/Xóa; lưu role PHẢI có lý do và `If-Match`; NẾU xóa role còn người dùng trả `409 CONFLICT` THÌ hiện gợi ý thu hồi UserRole trước. | FR-AUTH-032 | TC-CMS-RBAC-05 |
| F052-AC-16 | KHI gán UserRole, Portal PHẢI chỉ cho chọn role người gán được phép cấp, phạm vi Đơn vị/Trường/Môn (không có Lớp), hiệu lực từ/đến tùy chọn, lý do bắt buộc; Tenant Admin gọi `grantTenantUserRole`, School Admin gọi `grantSchoolUserRole` (Idempotency-Key); NẾU nhận `403 PERMISSION_DENIED` hoặc `422 ROLE_NOT_FOUND` THÌ hiện lỗi tương ứng. | FR-AUTH-032 · ADR-0072 | TC-CMS-RBAC-06, TC-AUTH-077 |
| F052-AC-17 | KHI thu hồi UserRole, Portal PHẢI yêu cầu lý do và xác nhận, gọi `revokeTenantUserRole` và thông báo thay đổi có hiệu lực trong `principal_cache_ttl`. | FR-AUTH-032 | TC-CMS-RBAC-03, TC-AUTH-078 |
| F052-AC-18 | "Nhật ký phân quyền" PHẢI liệt kê `listTenantAuditLogs` lọc `action` thuộc `role.*` và `user_role.*`, hiển thị người làm, thời điểm, trước/sau, lý do. | FR-AUTH-032, FR-AUD-001 | — |

---

### User Story 4 — Cài đặt tenant/trường và dọn học sinh không đăng nhập (Priority: P2)

Tenant Admin đặt khung giờ truy cập, ngôn ngữ mặc định, số ngày dọn (khi platform cho phép), thời hạn chat gameplay (khi được phép); xem danh sách học sinh sắp bị dọn, loại trừ/gia hạn. School Admin đặt chính sách trường, kể cả mở rộng khung giờ giới nghiêm của trường (không nới quá giờ giới nghiêm platform).

**Why this priority**: Cấu hình vận hành; có giá trị mặc định platform nên không chặn US1–US2.

**Independent Test**: `ad.donvi` loại trừ `hs.sapbidon` khỏi đợt dọn có lý do; đặt `gameplay_chat_retention_days` dưới mức tối thiểu → lỗi tại trường.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F052-AC-19 | KHI người có `tenant.settings.write` lưu cài đặt tenant, Portal PHẢI gửi `updateTenantSettings` (JSON Merge Patch, `If-Match`), hiển thị giá trị hiệu lực `effective_cleanup_days`, và cảnh báo khi đặt `inactive_student_cleanup_days` dưới ngưỡng cảnh báo của [ADMIN_PORTAL §3.10](../../docs/04-admin-portal/ADMIN_PORTAL.md). | FR-AUTH-024 · ADR-0032 | — |
| F052-AC-20 | NẾU server từ chối ghi đè số ngày dọn (`403 PERMISSION_DENIED`) hoặc trả `422 RETENTION_BELOW_MINIMUM` cho `gameplay_chat_retention_days`, THÌ Portal PHẢI hiển thị lỗi tại đúng trường và giữ giá trị cũ. | FR-AUTH-024 · ADR-0033 | — |
| F052-AC-21 | KHI người có `tenant.student_cleanup.read` mở "Học sinh sắp bị dọn", Portal PHẢI liệt kê `previewTenantStudentCleanup` (ngày dự kiến, lần đăng nhập cuối) và cho người có `tenant.student_cleanup.exclude` chọn nhiều (≤ `maxItems` của `CleanupExclusionRequest`) để Loại trừ hoặc Gia hạn đến ngày, lý do bắt buộc, gọi `createStudentCleanupExclusions` (Idempotency-Key). | FR-AUTH-024, AC-AUTH-018 | — |
| F052-AC-22 | KHI người có `school.settings.write` lưu chính sách trường (khung giờ truy cập, bật/tắt bảng xếp hạng lớp/trường, `student_club_creation`), Portal PHẢI gọi `updateSchoolSettings` kèm `If-Match` và hiển thị lỗi theo trường. | FR-ORG-007 | — |
| F052-AC-29 | KHI người có `school.settings.read` mở Cài đặt trường, Portal PHẢI gọi `getSchoolSettings` và hiển thị khối "Giờ giới nghiêm" gồm `platform_curfew` (chỉ đọc, ghi chú "do EduWorld đặt, áp cho game và chat"), khung trường `curfew_window` (trống = chỉ áp giờ platform) và `effective_curfew` (giờ ICT, khung qua nửa đêm hiển thị "đến … hôm sau"); ô sửa PHẢI chỉ hiện với action `action.school_lms.settings.edit_school`; trường thuộc tenant `common` PHẢI chỉ hiển thị giờ platform; màn PHẢI không có lựa chọn nào theo khối/lớp | FR-ORG-007 · NFR-SAFE-005 · [CHILD_SAFETY §4.1](../../docs/10-security/CHILD_SAFETY.md#41-giờ-giới-nghiêm-cấp-platform-adr-0078) · ADR-0078 · AGENTS R15 | TC-AUTH-092 |
| F052-AC-30 | KHI người có `school.settings.write` lưu khung giới nghiêm trường, Portal PHẢI kiểm trước khi gửi rằng khung chứa trọn `platform_curfew` (không thì báo lỗi tại ô và khóa nút Lưu), bắt buộc `reason`, gửi `updateSchoolSettings` (JSON Merge Patch `curfew_window`, `If-Match`) và cho "Bỏ khung trường" gửi `curfew_window: null`; NẾU server trả `422 VALIDATION_FAILED` (`errors[]` trỏ `curfew_window.*`) hoặc `412 VERSION_MISMATCH`, THÌ Portal PHẢI hiển thị lỗi tại đúng ô (412: tải lại giá trị mới) và giữ giá trị đã lưu | FR-ORG-007 · AC-AUTH-032 · [CHILD_SAFETY §4.1](../../docs/10-security/CHILD_SAFETY.md#41-giờ-giới-nghiêm-cấp-platform-adr-0078) · ADR-0078 | TC-AUTH-092 |
| F052-AC-32 | KHI người có `school.settings.write` bật, sửa hoặc tắt "Giới hạn chơi mỗi ngày" trong Cài đặt trường, Portal PHẢI gửi `updateSchoolSettings` (`daily_play_limit_minutes`, `null` = tắt) kèm `reason` và `If-Match`, hiển thị trạng thái mặc định "Tắt", và NẾU server trả `422 VALIDATION_FAILED`/`412 VERSION_MISMATCH`, THÌ PHẢI báo lỗi tại ô như F052-AC-30 | NFR-SAFE-005 · [CHILD_SAFETY §4](../../docs/10-security/CHILD_SAFETY.md) · ADR-0078 | — |

---

### User Story 5 — Nhật ký audit của tenant (Priority: P3)

Tenant Admin (School Admin trong trường mình) tra cứu audit: ai làm gì, khi nào, trên tài nguyên nào.

**Why this priority**: Phục vụ truy vết; không chặn vận hành hằng ngày.

**Independent Test**: `ad.donvi` lọc audit theo action `user_role.changed` trong 7 ngày.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F052-AC-23 | KHI người có `tenant.audit.read` mở "Nhật ký", Portal PHẢI liệt kê `listTenantAuditLogs` lọc trường/người làm/action/khoảng ngày, hiển thị `changes` đã lọc PII và `trace_id`; không có mục của module moderation. | FR-AUD-001 | — |

---

### Yêu cầu chung cho mọi story

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F052-AC-24 | TRONG KHI System Admin dùng workspace C, Portal PHẢI yêu cầu chọn đúng một tenant cho các màn dùng `X-Tenant-Id` và gửi tenant đó ở mọi thao tác đọc/ghi (theo F051). | AC-ORG-004 · ADR-0027 | TC-TEN-004 |
| F052-AC-25 | Portal PHẢI ẩn hành động ghi khi người dùng thiếu permission tương ứng (bảng permission ở plan.md) và vẫn xử lý `403 PERMISSION_DENIED` từ server bằng thông điệp không có quyền. | AC-ORG-003 · AGENTS R20 | TC-TEN-003 |
| F052-AC-26 | Portal PHẢI chặn nhập vượt `maxLength`/`pattern` của OpenAPI cho mọi trường form (username, email, tên, địa chỉ, lý do…), hiển thị bộ đếm ký tự và ánh xạ `422 VALIDATION_FAILED` về đúng trường. | ADR-0041 | TC-TEN-024 |
| F052-AC-27 | Ở NƠI độ rộng < 768 px, Portal PHẢI hiển thị danh sách dạng thẻ (`ResponsiveTable`), cho xem trường/lớp/học sinh/học sinh sắp bị dọn; form tạo role, import CSV hiển thị "Tác vụ này cần màn hình lớn hơn". | [ADMIN_PORTAL §1.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F052-AC-28 | KHI menu/action của feature (`menu.school_lms.schools`, `menu.school_lms.classes`, `menu.school_lms.sis_sync`, `menu.school_lms.student_activity`, `menu.school_lms.access_control`, `menu.school_lms.settings`, `menu.school_lms.audit_logs` và action con) đang tắt cho phạm vi phiên (`disabled_menus`/`disabled_actions` của `getMyPermissions`), Portal PHẢI ẩn menu và ẩn/khóa nút tương ứng (`<ActionGate actionKey>`); NẾU API trả `403 FEATURE_DISABLED`, Portal PHẢI hiện thông báo tính năng tạm tắt và tải lại quyền | FR-CMS-018 · ADR-0090 | TC-CMS-MENU-02 |

### Edge Cases

- NẾU `upload_url` hết hạn trước khi tải xong, THÌ Portal PHẢI báo hết hạn và tạo import mới với Idempotency-Key mới (AC-09).
- NẾU tenant `unit` có nhiều trường, THÌ School Admin chỉ thấy trường của mình; Tenant Admin có bộ lọc Trường (AC-01).
- NẾU tài khoản đang xem bị soft-delete bởi đợt dọn, THÌ hồ sơ hiển thị trạng thái và không có thao tác sửa; khôi phục do đăng nhập lại (ADR-0032).
- NẾU `listTenantPermissions` không có permission nào cho một workspace, THÌ nhóm đó không hiển thị trong picker (AC-14).
- NẾU School Admin cố gán role vượt quyền mình, THÌ role đó không có trong dropdown; server vẫn trả lỗi nếu gọi thẳng (AC-16).
- Không có thao tác "Xóa vĩnh viễn" ở bất kỳ màn nào (R21).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-AUTH-024 | Màn cài đặt dọn học sinh (tenant), danh sách sắp bị dọn, loại trừ/gia hạn | [ACCOUNT_PROVISIONING](../../docs/09-identity/ACCOUNT_PROVISIONING.md), [ADMIN_PORTAL §3.10](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-AUTH-032 | Người dùng, Role, UserRole, nhật ký phân quyền trong tenant | [ADMIN_PORTAL §3.17](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-ORG-007 | Cài đặt trường (khung giờ, bảng xếp hạng, club) và tenant | [ADMIN_PORTAL §3.9](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-ORG-016 | Trường của tenant: xem, sửa hiển thị, duyệt `pending_review`; lớp | [ADMIN_PORTAL §3.16](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-SIS-005 | Nhập CSV, theo dõi import, xung đột đồng bộ, học sinh chưa có lớp | [SIS_INTEGRATION](../../docs/09-identity/SIS_INTEGRATION.md) |
| FR-AUD-001 | Tra cứu audit tenant, nhật ký phân quyền | [ADMIN_PORTAL §7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-GAME-013 | Xem lịch sử đăng nhập game, số ngày chưa vào game | ADR-0031 · [ADMIN_PORTAL §3.10](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-018 | Menu/action của feature theo danh mục `specs/portal/menu-catalog.yaml`, ẩn khi tắt, xử lý `403 FEATURE_DISABLED` | [ADMIN_PORTAL §2.7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |

### Key Entities

- **Trường**: thuộc một tenant và một tỉnh/thành; trạng thái `active`/`pending_review`/`inactive`; dữ liệu SIS chỉ đọc.
- **Lớp, học sinh**: từ SIS; học sinh thuộc đúng một trường, một lớp.
- **Lần nhập SIS, xung đột đồng bộ**: tiến trình import CSV và các bản ghi cần người xử lý.
- **Nhân sự, Role, UserRole**: tài khoản nhân sự; role là tập permission; UserRole gán role theo phạm vi Đơn vị/Trường/Môn, có hiệu lực.
- **Cài đặt tenant/trường, loại trừ dọn học sinh, sự kiện đăng nhập, bản ghi audit**.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `csv_import_max` | Giới hạn kích thước file CSV kiểm trước khi upload |
| `inactive_student_cleanup_days` | Hiển thị/chỉnh số ngày dọn và giá trị hiệu lực |
| `principal_cache_ttl` | Thông báo thời gian hiệu lực khi gán/thu hồi |
| `identifier_uniqueness` | Thông điệp và gợi ý khi trùng username/email |
| `sis_sync_schedule` | Hiển thị lịch đồng bộ ở màn SIS |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không hiển thị IP, `ip_hash`, mã thiết bị thô | ADR-0031 · AGENTS R23 |
| Không đưa chat thô, dữ liệu moderation vào workspace C (F052) | AGENTS R14, R16 · ADR-0072 |
| Chỉ soft-delete, không "xóa vĩnh viễn" | AGENTS R21 · ADR-0028 |
| Theme token, WCAG 2.1 AA | NFR-MAINT-007, NFR-A11Y-003 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: School Admin nhập CSV 500 học sinh và thấy kết quả (số dòng hợp lệ/lỗi, số xung đột) mà không cần hỗ trợ kỹ thuật.
- **SC-002**: 100% màn của F052 trả "Không tìm thấy" khi tài khoản trường khác mở URL tài nguyên ngoài scope.
- **SC-003**: Gán và thu hồi một UserRole mất ≤ 4 thao tác; người được gán thấy menu đổi trong ≤ `principal_cache_ttl` + một chu kỳ làm mới.
- **SC-004**: 0 lần hiển thị IP hay mã thiết bị thô trong mọi màn hoạt động đăng nhập.
- **SC-005**: 0 lỗi axe mức serious/critical trên các màn chính của F052.

## Ngoài phạm vi

- Shell, bộ lọc tenant, đổi đơn vị → F051.
- Course, learning path, giao bài, lịch, nhóm học tập → F055; School Learning Dashboard, báo cáo → F056.
- Chat policy trường, nhóm từ của tenant (chỉ xem), thông báo vụ việc an toàn → F122.
- Quản lý tenant, danh mục quốc gia/tỉnh, import trường liên tenant, role platform, audit liên tenant, Thùng rác mọi tenant → F057.
- Khóa/mở khóa tài khoản học sinh (US-AUTH-003, backend F016) — màn hình đi kèm F057 hoặc bổ sung sau; F052 chỉ hiển thị trạng thái.
- Backend các endpoint "Cần thêm" → owner F010/F011/F015 (contract delta ở Phase 2 của F052).

## Assumptions

- Ngưỡng cảnh báo số ngày dọn và bố cục màn theo [ADMIN_PORTAL §3.10, §3.16, §3.17](../../docs/04-admin-portal/ADMIN_PORTAL.md) [Confirmed].
- Role tùy biến của tenant là [Confirmed] (FR-AUTH-032); màn vẫn làm, chỉ hiện khi người dùng có `tenant.role.write`.
- `listTenantPermissions` trả nhãn nhóm theo workspace; nếu thiếu, portal nhóm theo tiền tố permission.
