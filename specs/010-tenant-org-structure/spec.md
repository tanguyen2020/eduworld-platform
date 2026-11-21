# Feature Specification: F010 — Tenant, danh mục quốc gia/tỉnh/trường, tổ chức trường–lớp–enrollment

**Feature Branch**: `010-tenant-org-structure` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `identity` · **Mốc**: M-C1 · **Phụ thuộc**: F003, F004
**Input**: Mô hình tenant (`school`/`unit`/`common`) gắn một Keycloak Organization, danh mục quốc gia/tỉnh/trường, năm học–lớp–học sinh–enrollment có lịch sử, tenant chung do platform tạo. FR nguồn: FR-ORG-001, FR-ORG-002, FR-ORG-003, FR-ORG-005, FR-ORG-009, FR-ORG-010, FR-ORG-015, FR-ORG-016, FR-ORG-017, FR-AUTH-001, FR-AUTH-021.

## Clarifications

### Session 2026-11-16

- Q: Ai tạo Keycloak Organization và user của học sinh tenant chung? → A: module identity (worker `keycloak-membership`/`keycloak-provisioner`) là module duy nhất gọi Keycloak Admin API ([KEYCLOAK_SSO §2](../../docs/09-identity/KEYCLOAK_SSO.md)); adapter Keycloak do F010 tạo, F011/F013/F016 dùng lại.
- Q: API tạo/sửa tenant, trường thuộc tenant, tạo học sinh tenant chung chưa có trong spec? → A: Thêm ở Phase 2 (contract-first) theo [API_DESIGN §9.16](../../docs/07-backend/API_DESIGN.md); xem [contracts](contracts/README.md) §6.
- Q: Số lớp ảo tenant chung? → A: `common-g1` … `common-g9`, School ảo duy nhất, seed bằng migration ([MULTITENANCY §8.3](../../docs/06-architecture/MULTITENANCY.md)) [Confirmed].

### Session 2026-11-17

- Q: Mã lỗi khi vượt phạm vi trường/môn của UserRole? → A: `403 PERMISSION_DENIED`; `FORBIDDEN_SCOPE` không phát và sẽ bị gỡ khỏi `ProblemCode` ([API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md), RB-113).
- Q: Operation facade phân trang để rebuild projection `ref.*` (`listProjectionSnapshot`, F020 yêu cầu) thuộc feature nào? → A: F010 sở hữu (module identity là nguồn tenant/trường/lớp/học sinh/enrollment; [DATABASE_DESIGN §2.17](../../docs/07-backend/DATABASE_DESIGN.md) mục 3, BACKUP_RECOVERY §3); khai báo trong `specs/openapi/modules/identity.internal.v1.yaml`, `x-allowed-callers` `learning`, `game`, `competition`, `chat`; bên gọi (F020, F014, F091, F095) chỉ hiện thực consumer/job rebuild.
- Q: Mật khẩu ban đầu, quy tắc username và lô một phần của `createCommonTenantStudents`? → A (Technical Architect 2026-11-17): username bắt buộc (chuẩn hóa ADR-0067, không `@`); module identity sinh mật khẩu ngẫu nhiên, trả một lần (`no-store`), không lưu/log; khối 6–9 đổi mật khẩu lần đầu, khối 1–5 không [Confirmed]; lô ≤ `common_student_batch_max`, từng dòng độc lập (`USERNAME_TAKEN` theo dòng); replay không có mật khẩu (`password_redacted`). Nguồn: [ACCOUNT_PROVISIONING §3.4](../../docs/09-identity/ACCOUNT_PROVISIONING.md) (F010-AC-28, F010-AC-29).

### Session 2026-11-19 (khi triển khai)

- Q: Sinh migration từ ent khi `make migrate-diff` (Atlas `ent://`) không chạy được (image Atlas không có Go, máy dev không có `make`)? → A: Phần bảng/index của migration được render từ ent schema bằng `entx.DDL` giữa hai dấu `-- BEGIN entddl`/`-- END entddl`, golden test `modules/identity/ent/ddl_test.go` chặn lệch; CHECK, FK ghép, index biểu thức, RLS, view viết tay bên dưới (R27). `atlas.sum` tính bằng `scripts/db/atlas-sum.mjs` (cùng thuật toán `atlas migrate hash`, khớp 5 thư mục hiện có); Atlas vẫn kiểm sum khi áp. Mixin `entx` có `PK` để đặt tên cột khóa theo DATABASE_DESIGN.
- Q: Trạng thái trường lưu `inactive` (data-model cũ) hay `disabled` (contract `School.status`)? → A: `disabled` ở DB, API và event `school.updated`/`school.synced` (sửa enum schema event — chưa có producer/consumer nào).
- Q: `catalog.provinces.effective_from` bắt buộc? → A: Không: 63 đơn vị cũ trong `specs/reference-data` không có ngày bắt đầu (contract `Province.effective_from` đã cho phép null).
- Q: Scope của System Admin khi ghi vào một tenant đích? → A: `appscope.TargetTenant`: có `X-Tenant-Id` (principal được resolve vào tenant đó) → `TenantScope` của phiên (RLS áp); không có header → `CrossTenantScope` đúng tenant đích (ADR-0089 §3, audit `cross_tenant.accessed`). Không thêm API scope mới vào `pkg/authz` (API khóa theo ADR-0089).
- Q: `<module>_platform` không có quyền ghi outbox (ADR-0089 §4 chỉ cấp `_app`) nên transaction `PlatformScope` không phát được event? → A: Migration mới `20261119000010_infra_platform_grants` (mọi module, sinh bằng `tools/infragen`) cấp cho `_platform` cùng quyền ghi như `_app`; ADR-0089 ghi làm rõ 2026-11-19.
- Q: Thứ tự gỡ → thêm → thu hồi phiên khi `tenant_membership.changed` đến thành hai event (removed, added)? → A: Worker đối soát toàn bộ membership của user mỗi lần (đọc `tenant_memberships_v` liên tenant bằng bypass `platform_job`), gỡ trước, thêm sau, logout khi có gỡ — không phụ thuộc thứ tự giao event.
- Q: Issue `unexpected_org_member` (F010-AC-22) lưu ở đâu khi bảng `sis.reconciliation_issues` thuộc F011? → A: F010 gỡ member + audit `org_member.removed` và `reconciliation_issue.raised`; F011 lưu issue vào bảng của mình.
- Q: Mật khẩu ban đầu học sinh tenant chung? → A: 10 ký tự ngẫu nhiên (crypto/rand) từ bảng chữ không nhầm lẫn (bỏ `0/o/1/l/i`) để in thẻ đăng nhập [Confirmed]; kết quả lớn hơn giới hạn bản ghi idempotency (64 KiB) không được lưu — replay khi đó chạy lại và mọi dòng `USERNAME_TAKEN` (không tạo trùng).
- **OQ F010-1 (đã chốt 2026-11-20, Session owner chốt):** `importSchools` bất đồng bộ (`202` + `getSchoolImport`, lưu file, `IDENTITY_SIS_JOBS`) cần kho job và lưu file chưa có trong data-model F010 (bảng `sis.sis_imports`, adapter object storage là của F011). F010 hiện thực use case `ImportSchools` theo lô (dry-run, ánh xạ tỉnh cũ, `pending_review`); endpoint chờ F011 [Confirmed: F011 sở hữu].
- **OQ F010-2 (đã giải quyết 2026-11-20):** chưa có feature nào sở hữu phần nối binary (core-api mount `httpx.Server` với resolver Principal — cần `resolvePrincipal` của F012; worker chạy relay outbox, consumer `identity.keycloak_membership`, job `org-membership-reconcile`). F010 cung cấp `presentation/http.Register`, consumer và use case job; [Confirmed: F012 nối core-api, F011 nối worker identity].
- Q: Audit lần đọc hồ sơ học sinh (`getStudentProfile`)? → A: Chưa làm: read context không ghi outbox (ADR-0087) [đã chốt 2026-11-20: audit qua `audit.Recorder`, Session owner chốt].

### Session 2026-11-20 (bổ sung sau F057 — [Confirmed], owner chốt 2026-11-20)

- Q: Portal (F057) không biết version để gửi `If-Match` khi sửa tenant/quốc gia/tỉnh chưa ghi trong phiên (OQ F057-3)? → A: [Confirmed] Thêm field tùy chọn `version` (integer ≥ 1, chính là giá trị `If-Match` mong đợi) vào `TenantSummary` (nên cả `TenantDetail`), `Country`, `Province`; `listTenants`, `listCountries`, `listProvinces` và kết quả create/update trả field này (thêm field, không breaking). Không thêm GET từng bản ghi.
- Q: `listCatalogSchools` cho System Admin đọc liên tenant và lọc trạng thái (OQ F057-4)? → A: [Confirmed] `X-Tenant-Id` đổi sang `TenantIdHeaderOptional` (route `TenantOptional`). Người dùng tenant vẫn khóa vào tenant của phiên (`tenant_ids` bị bỏ qua; không có tenant phiên và không có quyền platform → `403`). Principal platform không gửi `X-Tenant-Id` phải có `platform.cross_tenant.read` + MFA và gửi `tenant_ids` (CrossTenantScope, audit `cross_tenant.accessed`); chỉ có PlatformScope → `403 PERMISSION_DENIED` (không có truy vấn "mọi trường của mọi tenant", R8). Thêm tham số tùy chọn `status` ∈ {`pending_review`, `active`, `disabled`} (đúng `schools_status_chk`).
- Q: Backend của `listTenantClassStudents` (contract do F052 thêm, Câu hỏi mở F052-1)? → A: Việc tiếp theo của F010, đã làm 2026-11-20: `query.Org.ListTenantClassStudents` + `OrgModel.ClassStudents` (học sinh có enrollment `active` của lớp; lớp phải thuộc trường trong path và tenant phiên, nếu không → `404`). [Confirmed] Quyền: `tenant.class.read` **hoặc** `lms.class.read` (đúng hai quyền của `action.school_lms.classes.view` trong menu catalog) trong scope trường — role hệ thống `school_admin` không có `tenant.class.read` nên contract ghi "School Admin trường mình" chỉ đạt được qua `lms.class.read`; grant chỉ ở trường khác → `404` (không lộ trường, vẫn audit `authz.denied` `outside_grant_scope`), không có quyền nào → `403`. Chỉ trả đúng field `ClassStudent` (R16): `display_name` = tên hiển thị tài khoản (rỗng thì họ tên SIS), `source` theo cùng quy tắc `listTenantUsers` (link SIS active → `sis`, ngược lại `manual`), `days_since_last_login` tính ở server từ `users.last_login_at` (đăng nhập game, F014); bỏ tài khoản soft-delete/ẩn danh hóa; `q` tìm không phân biệt hoa thường theo tên hiển thị, username, mã học sinh (ký tự `%`/`_` được escape); phân trang keyset `(display_name, student_id)`. Test: `TestListTenantClassStudentsScope` (unit), `TestSchoolScopedRoutes` (integration HTTP).

### Session 2026-11-20 (nối binary — trả lời OQ F010-2, [Confirmed])

- Q: Ai nối binary và nối thế nào (OQ F010-2, cùng OQ F014-7, F016-7, F020-3, F023-2, F045-3)? → A: [Confirmed] Một lần nối chung cho mọi feature Done thay vì chia F012 (core-api) / F011 (worker): mỗi module tự nối trong `modules/<m>/module.go` (identity thêm `wire.go`) và khai báo điểm vào trên `pkg/module.Registry` — `HTTP` (chỉ `core-api`), `Relay` + `Consumer` (worker `events`/`all`), `Job` (worker `jobs`/`all`); `cmd/internal/wiring.Compose` chỉ ghép module: learning, game đăng ký trước, identity `Wire(Imports{Learning, Game})` sau (facade gọi trong tiến trình qua `pkg/modcall`, chỉ import package `api`). `core-api` dựng **một** `httpx.Server` (`wiring.NewPublicServer`): JWKS của `EW_OIDC_ISSUERS` (readiness `jwks`), resolver Principal của F012 (`resolvePrincipal`) sau cache L1 15 s + Valkey 60 s (invalidate bằng `natsx.PrincipalInvalidator`), OpenAPI validator trên `EW_OPENAPI_SPEC`, kho idempotency **theo database của module** (`httpx.Deps.IdempotencyStores`, theo `Operation.Module`), featuregate (ADR-0090) từ nguồn của identity; một operation mount hai lần làm `core-api` không khởi động.
- Q: Hook audit của `tenancy.Bypass`/`authz.denied` khi một binary nạp nhiều module? → A: [Confirmed] `Registry.AuditRecorder()` ghi vào outbox của module trong `ctx` (`tenancy.ModuleFrom`); không biết module → từ chối, không ghi chéo database.
- Q: Consumer identity dùng `Platform` nào? → A: [Confirmed] `Platform: true` cho mọi consumer identity (event `tenant_id = null` như `platform_settings.changed`, `audit.recorded` của System Admin vẫn được nhận; event có tenant vẫn chạy trong `TenantScope` của event). `identity.audit_sink` có một durable mỗi stream `EVT_*` (cùng tên consumer). `identity.feature_toggle_cache` chạy durable trong worker `events` (cache dùng chung trong Valkey, featuregate trong `core-api` giữ cache tiến trình ≤ `feature_toggle_cache_ttl`) thay vì consumer tạm thời mỗi replica `core-api`.
- Q: Lịch job chưa có hằng số? → A: [Confirmed] `org-membership-reconcile` 24 h, `user-role-expiry` 5 phút, `retention-anonymizer` 24 h, learning `import-jobs`/`translation-jobs` 30 s, `content-scheduled-publish` 1 phút (F026 T036); còn lại theo hằng số của `presentation/job` (audit partition 6 h, audit retention 24 h, student cleanup 24 h, saga-retry 1 phút, platform-saga 10 s, sis-import-watch 15 s, sis-staging-anonymizer 1 h, sis-scheduler 30 s, game session-timeout `EW_GAME_SESSION_TIMEOUT_SCAN`, curfew closer 1 phút). Khóa leader (DEPLOYMENT §7) chưa làm: các job an toàn khi chạy song song (kiểm `FOR UPDATE`, saga idempotent); local chạy một replica.
- Q: Kho secret SIS (`port.SecretStore`) chưa có adapter? → A: [Confirmed, OQ F017-3] `contractv1client.FileSecrets`: đọc `EW_SIS_SECRETS_DIR/<secret_ref>`, cache 5 phút, chặn đường dẫn ra ngoài thư mục; compose mount `deploy/compose/sis-secrets.example` (đổi bằng `EW_SIS_SECRETS_HOST_DIR`).
- Q: Issuer của token trong container? → A: [Confirmed] Token chỉ cấp qua backend (ADR-0068, ADR-0091) nên container dùng `EW_OIDC_ISSUERS=http://keycloak:8180/realms/eduworld`; token lấy trực tiếp từ `localhost:8180` không dùng được với `core-api` trong container (chạy native thì đặt issuer `localhost`).
- **OQ F010-3 (đã chốt 2026-11-20, Session owner chốt):** mỗi lần chạy job nền có `tenancy.Bypass(ctx, "platform_job")` ghi một `cross_tenant.accessed` dù không xử lý dòng nào: smoke 2 phút sinh 427 bản ghi audit (learning 282, identity 150) — ước tính hàng chục nghìn dòng/ngày với chu kỳ hiện tại. [Confirmed] chỉ audit bypass của job khi có dòng bị đọc/ghi (hoặc gộp theo giờ); owner chốt 2026-11-20 (làm rõ ADR-0089 §3). Tạm thời chu kỳ `import-jobs`/`translation-jobs` đặt 30 s.
- Q: Facade chưa hiện thực đủ `StrictServerInterface`? → A: `Module.Facade()` trả interface con đúng các operation đã hiện thực (identity: principal + snapshot; learning: platform-ops + vòng đời học sinh; game: vòng đời học sinh); identity nhận đúng phần nó gọi, `mergeUserData` và platform-ops của game/competition/chat vẫn trả `503 UNAVAILABLE` (OQ F045-3), `getTenant` chưa có bên gọi.

### Session 2026-11-20 (seed identity cho local — [Confirmed], owner chốt 2026-11-20)

- Q: Ai tạo System Admin đầu tiên (đăng nhập portal `sa.admin` → `403 PORTAL_ACCESS_STAFF_ONLY`; `createPlatformUser` của F059 cần một System Admin sẵn có)? → A: [Confirmed] Use case `command.PlatformStaffEnsure.Ensure` (không có route HTTP): không có Principal = bootstrap của thành phần hệ thống `identity-bootstrap`, chỉ khi chưa có grant `system_admin` hiệu lực nào và chỉ cấp `system_admin` (người cấp = chính tài khoản mới); có Principal = cần `platform.user_role.write` + nhân sự platform, role cấp qua `UserRoles.Grant` route platform. Tài khoản `user_type = staff`, `primary_tenant_id = NULL`, `external_identities` = subject Keycloak sẵn có của issuer cục bộ; audit `user.created` + `user_role.granted`, event `user.provisioned` (`source = manual`) + `user_role.changed`. Nhân sự platform tiếp theo ở production đi qua `createPlatformUser` (F059); seed dùng `Ensure` vì gắn subject sẵn có của realm import và cấp `safety_moderator` kèm ngày đào tạo (`createPlatformUser` không nhận role này làm role ban đầu).
- Q: Seed dev của module identity (LOCAL_DEVELOPMENT §5) chạy thế nào mà không SQL tay? → A: [Confirmed] `modules/identity/cmd/seed` (`make seed-identity`, chỉ `EW_ENV=local`) đọc `modules/identity/seed/dev.yaml` + CSV `sis/<mã trường>/`: bootstrap `sa.admin` → thay mặt `sa.admin` (Principal resolve qua `resolvePrincipal`, kênh portal + MFA như `onBehalf` của F045) gọi `createTenant`, `createAdminTenantSchool`, `createSisConnection` + gán trường, import CSV qua `SisRun.RunImportSystem` (đúng pipeline của job import, bỏ bước upload/quét vì file nằm trong repository), `ProvisionStudentAccount`, `createTenantUser`, `grantTenantUserRole`, `lockTenantUser`. Tài khoản Keycloak không tạo mới: `keycloak.DevAdopter` (chỉ seed local) trả user cùng username của realm import, không đặt mật khẩu, không disable, không gửi email. Mỗi bước tra trước (`command.SeedLookup`, đọc theo scope của dòng) nên chạy lại không ghi gì (test tích hợp `TestDevSeedIsIdempotent`: outbox và số dòng không đổi). `make sis-sync` bỏ: import CSV chạy đồng bộ trong seed; `make seed-check` chờ `ref.students` của learning, game đủ học sinh đã seed.
- Q: Bộ tài khoản/mã demo không khớp giữa tài liệu và realm import? → A: [Confirmed] Giữ một bộ theo realm import (`deploy/keycloak/realm-eduworld.dev.json`, test `devseed` kiểm realm role + Organization): tenant `th-demo-a` (loại `school`, code = alias), `pgd-demo` (`unit`); `DEMOA1` là **mã trường** TH Demo A (khớp ví dụ SIS Contract), hai trường của `pgd-demo` là `DEMOB1` (THCS Demo B, 6–9), `DEMOC1` (TH Demo C, 1–5); `PGDDM2` (mã đăng nhập tenant, đã bỏ theo ADR-0063) không dùng. `sa.admin` có OTP seed sẵn trong realm import (LOCAL_DEVELOPMENT ghi "MFA tắt ở local" là sai, đã sửa); nhân sự platform khác chưa có OTP nên lần đăng nhập portal đầu đi luồng thiết lập OTP. Chưa seed: `qa-probe`/`hs.qa.probe` (`createTenant` chưa đặt được `is_qa`), `hs.tudo` (`createCommonStudents` sinh mật khẩu mới, ghi đè mật khẩu demo), `sa.moi`; `hs.nosis` cố ý không có tài khoản; `qlcm.truong.a` giữ `academic_manager` toàn tenant ở hai tenant (scope môn cần mã môn của module learning); `sm.truongca` chưa có `moderation.case.lead` (chưa có role).

### Session 2026-11-20 (owner chốt)

- OQ F010-1 → **Chốt:** F011 sở hữu `importSchools` bất đồng bộ (`202` + `getSchoolImport`): dùng lại bảng `sis.sis_imports`, bucket `ew-sis-imports`, watcher và job `IDENTITY_SIS_JOBS` của F011, gọi use case `ImportSchools` của F010 theo lô. Task ở F011.
- OQ F010-2 → đã giải quyết (nối binary 2026-11-20).
- OQ F010-3 → **Chốt:** audit bypass của job chỉ khi có dòng được xử lý (task ở F003); chu kỳ `import-jobs`/`translation-jobs` giữ 30 s. Làm rõ ADR-0089 §3 do agent docs ghi.
- Audit lần đọc hồ sơ học sinh ([TBD] Session 2026-11-19) → **Chốt:** `getStudentProfile` ghi audit `student_profile.viewed` (actor, `student_id`, `school_id`, không PII) qua `audit.Recorder` của module — cùng cơ chế `cross_tenant.accessed` ở read context (chỉ ghi outbox audit trong transaction riêng, không ghi dữ liệu nghiệp vụ, không phát domain event), đúng SCHOOL_LMS "xem hồ sơ học sinh ghi audit" [Confirmed]. Ghi chú ngoại lệ audit đọc vào ADR-0087 do agent docs làm.
- T034 `org-membership-reconcile` → **Chốt:** lịch 24 h đã nối (OQ F010-2); còn lệnh chạy một lần (`worker --job org-membership-reconcile --once`) và ghi issue `unexpected_org_member` thẳng vào `sis.reconciliation_issues` (cùng module identity, qua repository của F011) thay vì chỉ audit, để School Admin/System Admin xử lý trên màn xung đột.

### Session 2026-11-21 (triển khai — [Confirmed], owner chốt 2026-11-20)

- Q: T047 `listSchoolAcademicYears` — quyền, phạm vi và thứ tự? → A: [Confirmed] `GET /schools/{school_id}/academic-years` (guard `portal_staff`, `X-Tenant-Id` bắt buộc), quyền `analytics.school.read` hoặc `analytics.class.read` trên trường của path (đúng hai quyền của `action.school_lms.school_dashboard.view`; Tenant Admin không có → 403); trường ngoài phạm vi grant, khác tenant hoặc đã xóa mềm → 404 (cùng helper `schoolScoped` của F052-1); chỉ năm học `deleted_at IS NULL`, mới nhất trước (`start_date` giảm dần), tối đa 100; trả `academic_year_id`, `name`, `start_date`, `end_date`, `is_current`, `terms`.
- Q: T047 — `terms[]` lấy từ đâu? → A: [Confirmed] Module identity chưa lưu học kỳ (`org.academic_years` chỉ có khoảng ngày năm học, SIS chưa gửi học kỳ); không tự chia học kỳ theo ngày (sẽ là dữ liệu bịa). `terms` luôn `[]` → preset "Học kỳ này" của F056 vẫn vô hiệu, "Năm học này" bật được. **Câu hỏi mở F010-4:** nguồn học kỳ của trường (SIS `contract_v1`, nhập tay ở Workspace C hay cấu hình tenant) — cần feature riêng, không mở rộng phạm vi T047. **Chốt 2026-11-21:** Release 1 giữ preset "Học kỳ này" vô hiệu (không bịa ngày học kỳ); nguồn học kỳ (SIS hoặc nhập tay) là việc sau Release 1.

### Session 2026-11-21 (owner bỏ MFA — [Confirmed])

- Q: Owner bỏ MFA/TOTP cho đăng nhập Admin Portal với mọi tài khoản, kể cả role platform (2026-11-21, [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md)) — phần nào của feature này đổi? → A: F010-AC-04, AC-05: điều kiện "token `eduworld-portal` + MFA" đọc là "token `eduworld-portal` + role platform qua gateway nội bộ"; kiểm thử thiếu MFA → `MFA_REQUIRED` (T012, quickstart) thay bằng người không có role platform → `403 PERMISSION_DENIED`. Thay thế chung: route platform (`/admin/*`, `/game-admin/*`) chỉ còn gateway nội bộ + `azp = eduworld-portal` + `user_type = staff` + role platform (guard `platform_role`, thiếu → `403 PERMISSION_DENIED` + `authz.denied`); `CrossTenantScope` giữ `platform.cross_tenant.read` + client portal + `reason` + audit; phiên `platform_staff` bị từ chối `refreshPortalToken` sau 8 giờ từ lúc nhập mật khẩu (`EW_OIDC_PLATFORM_MAX_AUTH_AGE`, `401 REFRESH_TOKEN_INVALID`).

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
| F010-AC-28 | KHI System Admin gọi `createCommonTenantStudents` (`Idempotency-Key`, `X-Tenant-Id` = tenant `common`, ≤ `common_student_batch_max` dòng, mỗi dòng `username` bắt buộc đã chuẩn hóa trim/NFC/chữ thường, `display_name`, `grade_level`, `external_ref` tùy chọn), module identity PHẢI xử lý từng dòng độc lập và trả `200` với `results[]` (`row_no`, `status` `created`/`failed`, `user_id`, `username`, `initial_password`, `error_code`) cùng số đếm; dòng trùng username với tài khoản khác PHẢI `failed` với `USERNAME_TAKEN` và không tạo reconciliation issue; `external_ref` PHẢI lưu thành ExternalIdentity `issuer = platform_common` (không thêm cột học sinh), trùng với bản ghi chưa xóa PHẢI `failed` với `IDENTITY_CONFLICT` | FR-AUTH-021 · ADR-0069 · [ACCOUNT_PROVISIONING §3.4](../../docs/09-identity/ACCOUNT_PROVISIONING.md) | — |
| F010-AC-29 | KHI tạo được một học sinh tenant chung, module identity PHẢI sinh ngẫu nhiên mật khẩu ban đầu, đặt trong Keycloak (`temporary = true` với khối 6–9, `false` với khối 1–5 [Confirmed]) và trả đúng một lần trong `initial_password` với `Cache-Control: no-store`, không lưu và không log mật khẩu; replay cùng `Idempotency-Key` PHẢI trả kết quả đã lưu không có `initial_password` và `password_redacted = true` | FR-AUTH-021 · AGENTS R14 · [ACCOUNT_PROVISIONING §3.4](../../docs/09-identity/ACCOUNT_PROVISIONING.md) | — |

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
| `common_student_batch_max` | Số dòng tối đa mỗi request `createCommonTenantStudents` |

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
- Mã tỉnh chính thức (`official_code`): owner chốt 2026-11-20 (ADR-0060) — nhập từ danh mục chính thức của nhà nước trước khi seed production, `NULL` tới khi nhập; seed dùng `code` nội bộ.
- Lớp ảo `common-g1…g9` và School ảo [Confirmed] theo MULTITENANCY §8.3.
