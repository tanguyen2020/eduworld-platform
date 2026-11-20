# Feature Specification: F045 — Platform Administration backend: saga gán/clone, thùng rác mọi tenant, đọc liên tenant

**Feature Branch**: `045-platform-administration` · **Created**: 2026-11-16 · **Status**: Done · **Module**: module `identity` · **Mốc**: M-C3 · **Phụ thuộc**: F015, F020
**Input**: System Admin là người duy nhất xem dữ liệu liên tenant, gán dữ liệu platform cho tenant, clone cấu hình/nội dung giữa tenant (không bao giờ clone dữ liệu học sinh) và xem/khôi phục Thùng rác mọi tenant; module identity điều phối, module sở hữu dữ liệu thực hiện qua internal API. FR nguồn: FR-ORG-008, FR-ORG-012, FR-ORG-013, FR-CMS-013, FR-CMS-014, FR-CMS-015; hợp nhất tài khoản `/admin/users:merge` theo IDENTITY_MAPPING §7 (PRD chưa có mã FR riêng).

## Clarifications

### Session 2026-11-16

- Q: Bảng gán nằm ở đâu, ai ghi? → A: `infra.tenant_resource_assignments` ở database của module sở hữu tài nguyên; module identity định tuyến theo `resource_type` và điều phối bằng saga ([MULTITENANCY §10](../../docs/06-architecture/MULTITENANCY.md#10-gán-và-clone-dữ-liệu-giữa-tenant-adr-0029)).
- Q: Phần callee (list/restore/clone/gán trong từng module) thuộc feature nào? → A: Mỗi module tự hiện thực interface platform ops chung: module learning ở F020; game, competition, chat ở feature nền của các module đó. F045 làm phía điều phối, phần identity tự sở hữu và contract test với module learning.
- Q: Ai được hợp nhất tài khoản (IDENTITY_MAPPING §8: "school_admin + platform review")? → A: [Confirmed] School Admin đề nghị gộp khi xử lý issue `duplicate_identifier`/`possible_duplicate` trong scope trường (F011, `resolveSisReconciliationIssue` với `resolution = merge_requested`); chỉ System Admin có `platform.user.merge` (permission mới, chờ catalog ADMIN_PORTAL §2.2) gọi `mergeUsers` với tenant đích. Hai user phải cùng tenant đích; khác tenant mà cùng một người thì thêm membership (AC-AUTH-174, F011), không gộp.
- Q: Thùng rác của Tenant Admin bật thế nào? → A: Bằng permission `tenant.trash.read`/`tenant.trash.restore` gán qua role (ADMIN_PORTAL §2.3, [Confirmed]); không có cờ riêng.

### Session 2026-11-19 (triển khai)

- Q: Operation internal gán tài nguyên (contracts §6 `assignTenantResources`…)? → A: F020 đã thêm vào `common/internal-common.v1.yaml` với tên `applyTenantAssignments`, `revokeTenantAssignments`, `listResourceAssignments` (`x-allowed-callers: [identity]`); F045 dùng đúng các operation đó, không thêm operation mới (T004 xong ở F020).
- Q: Gán/gỡ gán/khôi phục có cần saga `tenant_assign`/`tenant_unassign`/`trash_restore` không? → A: Không. Mỗi thao tác là **một** lời gọi facade nguyên tử của module sở hữu (mọi tenant trong một transaction của callee) với `Idempotency-Key` của request; module identity ghi event + audit + bản ghi idempotency sau khi callee trả về; gửi lại cùng key thì callee phát lại kết quả đã lưu [Confirmed]. Saga (engine dùng chung với F016) chỉ dùng cho clone (`tenant_clone`) và gộp tài khoản (`user_merge`); engine thêm `Enqueue` (tạo run trong transaction của bên gọi, job `platform-saga` chạy) và `Reopen` (chạy lại saga `blocked`/`failed`, bước `done` không chạy lại) — tương thích ngược.
- Q: Module nào có adapter facade? → A: Chỉ module learning (F020) có platform-ops; game, competition, chat chưa hiện thực facade platform-ops và **không module nào** hiện thực `mergeUserData`. `OwnerGateway` trả `503 UNAVAILABLE` cho module chưa nối (không bao giờ cho qua); saga `user_merge` luôn gồm bước của cả 4 module nên sẽ `failed` sau số lần thử của engine cho tới khi các module hiện thực `mergeUserData` và binary được nối (OQ F010-2) — **OQ F045-3**.
  - 2026-11-20 [Confirmed]: binary đã nối (OQ F010-2): facade learning được nối vào `modules.Gateway`; `LearningMerge` để trống nên bước `mergeUserData` vẫn `503 UNAVAILABLE`; game/competition/chat chưa có platform-ops.
- Q: Loại do identity sở hữu trong Thùng rác/clone? → A: `user` (tài khoản đã gộp, `restorable = false`, khôi phục → `409`) và `student` (học sinh bị dọn `delete_reason = inactive_cleanup`, khôi phục qua port `StudentRestorer` do F016 hiện thực — saga `student_restore`, F016-AC-19; chưa nối → `503`) được thêm vào enum `ResourceType` của API public [Confirmed]. `class_template`, `school_settings`, `tenant_settings` chưa có bảng (DATABASE_DESIGN): Thùng rác trả rỗng, khôi phục `404`, clone định tuyến về identity và bước clone trả `503` → job `failed` — **OQ F045-4**. `class`, `party` (contracts §6) không thêm.
- Q: Bước saga bất đồng bộ dùng Principal nào khi gọi facade? → A: Principal platform dựng lại cho System Admin đã yêu cầu (`requested_by`/`merged_by`/`created_by` của gán mặc định), chỉ mang đúng permission bước cần (`platform.tenant_data.clone`, `platform.user.merge`, `platform.tenant_data.assign`), kênh portal + MFA; không rộng hơn request gốc [Confirmed] — **OQ F045-5** (thay bằng phân giải lại Principal khi có cơ chế chung).
- Q: Chuyển `external_identities` khi user đích đã có định danh cùng issuer (vd. cùng realm Keycloak)? → A: Định danh khác issuer chuyển nguyên trạng `active` (đăng nhập bằng nó vào user đích, AC-33); định danh trùng issuer chuyển sang user đích ở trạng thái `superseded` (`superseded_by` = định danh của user đích) vì unique `(user_id, issuer)` — đăng nhập bằng nó không còn vào được (IDENTITY_MAPPING §5 mục 9). `sis_identity_links` tương tự (một link `active` mỗi user). Membership/`org.students`/enrollment của user nguồn chưa được hợp nhất — **OQ F045-6** [Confirmed].
- Q: Trạng thái issue "đề nghị gộp"? → A: `status = open` + `resolution = merge_requested`; gộp xong → `resolved` + `resolution = merged` (CHECK DB mở rộng, enum response `SisReconciliationIssue.resolution` thêm hai giá trị). Phía School Admin đặt `merge_requested` qua `resolveSisReconciliationIssue` thuộc F011 (chưa có) — **OQ F045-7**.
- Q: `identity.user_merges` ghi khi nào? → A: Lúc nhận `mergeUsers` (cùng transaction với saga, có `tenant_id` đích + RLS đọc theo tenant), `merged_at` = thời điểm duyệt; UNIQUE `from_user_id` chặn gộp lần hai (`409`); gọi lại `mergeUsers` cùng cặp khi saga `blocked`/`failed` → mở lại saga (`Reopen`), saga `succeeded` → `409` [Confirmed]. Audit `user.merge_requested` lúc nhận, `user.merged` ở bước identity (chỉ ID + lý do).
- Q: Audit gán/clone/khôi phục bị ghi hai lần (module sở hữu + identity)? → A: Giữ cả hai: module sở hữu ghi thay đổi dữ liệu (F020), identity ghi thao tác của bên điều phối (`tenant_data.assigned/unassigned` từng tenant, `tenant_data.cloned` ở nguồn và đích, `<resource_type>.restored` có lý do) [Confirmed] — **OQ F045-8**.
- Q: `TenantCloneJob.errors[]` dạng object (contracts §6)? → A: Giữ mảng chuỗi (`maxLength 255`, không breaking change) dạng `<service> step <n>: <message>`; `id_map` lưu ở `platform.tenant_clone_jobs` (tối đa 1 MiB) vì `saga_steps.response` giới hạn 16 KiB [Confirmed].
- Q: Gỡ gán mặc định (`apply_to_new_tenants`)? → A: Không có API gỡ trong Release 1.0 (contract không có) — **OQ F045-9**.
- Q: Vị trí và phạm vi test? → A: Integration ở `src/backend/modules/identity/tests/integration/platform_admin_*_integration_test.go` (dùng chung `TestMain`/helper của package), test hợp đồng facade phía bên gọi ở `tests/contract/owner_gateway_test.go` với fake facade learning (phía bên nhận đã kiểm ở F020). `RESOURCE_NOT_ASSIGNED` và chỉ đọc do module learning cưỡng chế (F020-AC-16, AC-18); test module identity không import được `modules/learning/internal` nên dùng TEST DOUBLE có cùng luật [Confirmed].

### Session 2026-11-20 (bổ sung sau F057 — [Confirmed], owner chốt 2026-11-20)

- Q: `learning_material` (F024 thêm vào enum `ResourceType`) chưa có trong bảng định tuyến `resource_type → module` nên lọc/khôi phục Thùng rác theo loại này lỗi? → A: [Confirmed] Thêm `learning_material → learning` vào `internal/domain/platformadmin/routing.go`, cùng cờ với `lesson`: gán được, nằm trong allowlist clone, khôi phục được qua Thùng rác (facade learning chịu trách nhiệm từng loại). Unit test kiểm mọi giá trị enum `ResourceType` của spec gộp đều có module sở hữu.

### Session 2026-11-20 (owner chốt)

- OQ F045-3 → **Chốt:** facade vòng đời/merge của từng module do feature tạo dữ liệu làm: learning F030, game F070 + F074, competition F100, chat F115; platform-ops (gán/clone/Thùng rác) của game ở F078, competition ở F108, chat (sticker/GIF) ở F082. Tới khi có, bước tương ứng vẫn `503 UNAVAILABLE` (không bao giờ cho qua).
- OQ F045-4 → **Chốt:** clone `tenant_settings`, `school_settings` đọc/ghi bảng cài đặt của F015 (đã có); `class_template` không có trong Release 1.0: tạo job clone với loại này trả `422 VALIDATION_FAILED` ngay (thay vì job `failed` do `503`), Thùng rác trả rỗng.
- OQ F045-5, OQ F045-8 → giữ [Confirmed] (không làm cơ chế phân giải lại Principal chung trong Release 1.0).
- OQ F045-6 → **Chốt:** gộp chỉ trong cùng tenant (OQ F011-1); bước identity chuyển enrollment active của user nguồn sang user đích nếu user đích chưa có enrollment active (có thì giữ của user đích), soft-delete `org.students` của user nguồn (`delete_reason = merged`), gỡ membership của user nguồn qua `tenant_membership.changed`.
- OQ F045-7 → **Chốt:** School Admin đề nghị gộp bằng `resolveSchoolReconciliationIssue` `resolution = merge_requested` (task ở F011, nút ở F052).
- OQ F045-9 → **Chốt:** `unassignFromTenants` thêm `remove_default` (boolean, mặc định `false`): `true` thì soft-delete luôn quy tắc gán mặc định (`apply_to_new_tenants`) của tài nguyên, có audit; không thêm endpoint mới.
- OQ F057-1 → **Chốt:** thêm `previewTenantClone` (`POST /admin/tenant-clones:preview`, cùng body `TenantCloneRequest`, chỉ đọc, không `Idempotency-Key`, `platform.tenant_data.clone` + MFA) trả đồ thị phụ thuộc (`TenantClonePreview`: nút, cạnh, hành động `clone`/`keep_reference`/`conflict`) theo ADMIN_PORTAL bước 3; identity gom qua operation facade mới `previewTenantDataClone` của module sở hữu (learning trước, `x-allowed-callers` identity).
- `activity` trong `ResourceType` (OQ F030) → **Chốt:** thêm vào bảng định tuyến (`activity → learning`), test enum đầy đủ.
- Mọi mục [Confirmed] của Session bổ sung sau F057 được owner chốt 2026-11-20.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — System Admin đọc dữ liệu liên tenant có kiểm soát (Priority: P1)

`sa.admin` mở màn hình có dữ liệu theo tenant: mặc định "Tất cả tenant", chọn nhiều tenant qua dropdown tìm kiếm, mỗi dòng có cột Tenant; mọi lần xem được audit. Người dùng tenant không bao giờ đọc được tenant khác.

**Why this priority**: Nền cho mọi màn hình workspace D (F057) và mọi thao tác ghi platform.

**Independent Test**: `sa.admin` gọi `listTenants?q=demo`, rồi `listTrash?resource_type=question&tenant_ids=<A>&tenant_ids=<B>` → chỉ dữ liệu A, B, có `tenant_id`; audit `cross_tenant.accessed`. `ad.donvi` gửi `tenant_ids` của tenant khác → bị bỏ qua.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F045-AC-01 | KHI System Admin gọi endpoint danh sách của F045 không kèm `X-Tenant-Id` và có `tenant_ids[]` (rỗng = mọi tenant `active`), module identity PHẢI dùng phạm vi liên tenant với danh sách tenant tường minh, trả `tenant_id` trên mỗi dòng và ghi audit `cross_tenant.accessed` (actor, `tenant_ids`, endpoint) | FR-ORG-008 · FR-CMS-013 · AC-ORG-004 | TC-TEN-018, TC-CMS-TEN-02 |
| F045-AC-02 | NẾU người dùng tenant gửi `tenant_ids`, THÌ HỆ THỐNG PHẢI bỏ qua tham số và chỉ trả dữ liệu tenant của phiên | AC-CMS-002 | TC-CMS-TEN-01, TC-TEN-011 |
| F045-AC-03 | NẾU role platform khác `system_admin` (vd. `content_admin`, `safety_moderator`) gọi API dữ liệu tenant của F045, THÌ HỆ THỐNG PHẢI trả `403 PERMISSION_DENIED` | [ADMIN_PORTAL §2.1](../../docs/04-admin-portal/ADMIN_PORTAL.md#21-nguyên-tắc) | TC-CMS-TEN-04, TC-TEN-004 |
| F045-AC-04 | HỆ THỐNG PHẢI chỉ phục vụ route `/api/v1/admin/*` của F045 qua gateway nội bộ với `azp = eduworld-portal`, MFA và role platform; thiếu MFA PHẢI nhận `403 MFA_REQUIRED`; token role tenant PHẢI nhận `403 PERMISSION_DENIED` kèm audit `authz.denied`; gọi qua gateway public PHẢI không có route | AC-CMS-001 · ADR-0027 | TC-CMS-WS-02, TC-CMS-WS-03 |
| F045-AC-05 | KHI System Admin gọi `listTenants` với `q`, HỆ THỐNG PHẢI trả tenant khớp mã hoặc tên (không phân biệt hoa thường), kèm `tenant_type`, phân trang cursor; NẾU `q` vượt `maxLength`, THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` | FR-CMS-013 · ADR-0041 | TC-TEN-024 |
| F045-AC-06 | HỆ THỐNG PHẢI chỉ bỏ global filter tenant/soft-delete trong API liên tenant và Thùng rác qua `tenancy.Bypass(ctx, reason)` có audit và tăng `eduworld_tenant_filter_bypass_total`; linter PHẢI chặn bỏ filter ở chỗ khác | AGENTS R27 · ADR-0039 | TC-TEN-023 |

---

### User Story 2 — Gán dữ liệu platform cho tenant (Priority: P1)

System Admin chọn một question set platform, gán cho tenant A (hoặc mọi tenant mới); nhân sự tenant A thấy chỉ đọc, tenant B không thấy; gỡ gán không ảnh hưởng bài đã làm.

**Why this priority**: FR-CMS-015 phần gán là P2; nội dung platform phải gán được trước khi nhà trường giao bài (F041).

**Independent Test**: `sa.admin` gọi `assignToTenants` cho question set platform với tenant `th-demo-a` → `ad.truong.a` thấy (chỉ đọc), `ad.truong.b` không thấy; `unassignFromTenants` → A không còn thấy, attempt cũ vẫn xem được.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F045-AC-07 | KHI System Admin gọi `assignToTenants` `{resource_type, resource_id, tenant_ids[]}`, module identity PHẢI định tuyến tới module sở hữu theo `resource_type`, module đó PHẢI tạo một assignment `active` cho mỗi tenant (gọi lại không tạo trùng), và module identity PHẢI phát `tenant_data.assigned` và ghi audit `tenant_data.assigned` cho từng tenant | FR-ORG-012 · FR-CMS-015 · AC-CMS-005 | TC-TEN-019 |
| F045-AC-08 | NẾU tài nguyên không phải dữ liệu platform (`tenant_id` khác NULL), THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED`; NẾU tài nguyên không tồn tại hoặc đã soft-delete, THÌ HỆ THỐNG PHẢI trả `404 NOT_FOUND` | [DATABASE_DESIGN §2.0](../../docs/07-backend/DATABASE_DESIGN.md#20-platform--tenancy-adr-0005-adr-0026-adr-0029--db-eduworld_identity) | — |
| F045-AC-09 | TRONG KHI assignment `active`, HỆ THỐNG PHẢI cho tenant được gán đọc tài nguyên ở chế độ chỉ đọc (ghi → `403`) và tenant chưa gán tham chiếu tài nguyên PHẢI nhận `403 RESOURCE_NOT_ASSIGNED` | FR-ORG-012 · AC-ORG-007 | TC-CMS-ASG-01, TC-CMS-ASG-02 |
| F045-AC-10 | KHI System Admin gọi `unassignFromTenants`, HỆ THỐNG PHẢI soft-delete assignment, phát `tenant_data.unassigned`, ghi audit; tenant không còn thấy tài nguyên nhưng attempt và bài giao cũ vẫn xem lại được | [ADMIN_PORTAL §9](../../docs/04-admin-portal/ADMIN_PORTAL.md#9-bảo-vệ-lịch-sử-học-tập) | TC-TEN-019 |
| F045-AC-11 | Ở NƠI `apply_to_new_tenants = true`, HỆ THỐNG PHẢI lưu gán mặc định và tự gán tài nguyên cho mỗi tenant mới khi nhận `tenant.created` | [MULTITENANCY §10](../../docs/06-architecture/MULTITENANCY.md#10-gán-và-clone-dữ-liệu-giữa-tenant-adr-0029) | — |
| F045-AC-12 | KHI gửi lại thao tác gán/gỡ gán cùng `Idempotency-Key` và cùng payload, HỆ THỐNG PHẢI trả kết quả cũ; NẾU cùng key khác payload, THÌ HỆ THỐNG PHẢI trả `409 IDEMPOTENCY_KEY_REUSED` | AGENTS R10 · [API_DESIGN §6](../../docs/07-backend/API_DESIGN.md#6-idempotency) | — |

---

### User Story 3 — Clone cấu hình/nội dung giữa tenant (Priority: P2)

System Admin chọn tenant nguồn, loại tài nguyên và bản ghi, chọn **một** tenant đích, xem tiến độ job; bản sao là bản nháp riêng của tenant đích, không mang dữ liệu học sinh.

**Why this priority**: FR-CMS-015 phần clone là P3; cần US2 và luồng saga.

**Independent Test**: `sa.admin` clone một question set từ `th-demo-a` sang `pgd-demo` → `getTenantClone` `succeeded`, bản sao ID mới, `cloned_from_*`, `draft`; không có attempt/học sinh nào ở `pgd-demo` được tạo.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F045-AC-13 | KHI System Admin gọi `createTenantClone` với một tenant đích, module identity PHẢI trả `202` kèm `job_id`, lưu job và saga, rồi worker PHẢI gọi `executeTenantCloneStep` của từng module theo đồ thị phụ thuộc, truyền `id_map` của bước trước | FR-ORG-013 · FR-CMS-015 | TC-TEN-020 |
| F045-AC-14 | HỆ THỐNG PHẢI tạo bản sao có ID mới, `cloned_from_id`, `cloned_from_tenant_id`, `cloned_at`, `cloned_by`, trạng thái `draft` với nội dung có workflow, tham chiếu nội bộ trỏ tới bản sao và phụ thuộc là dữ liệu platform giữ nguyên tham chiếu | AC-CMS-005 · [ADMIN_PORTAL §3.7](../../docs/04-admin-portal/ADMIN_PORTAL.md#37-gán-dữ-liệu-cho-tenant-và-clone-giữa-tenant-adr-0029--chỉ-system-admin-workspace-d) | TC-TEN-020, TC-CMS-CLN-01 |
| F045-AC-15 | HỆ THỐNG PHẢI không clone học sinh, enrollment, attempt, kết quả, chat, mastery, XP; NẾU `resource_type` ngoài danh sách clone cho phép, THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` | AGENTS R22 · ADR-0029 | TC-CMS-CLN-01 |
| F045-AC-16 | NẾU tenant nguồn trùng tenant đích, tenant không tồn tại/`disabled` hoặc `resource_ids` vượt 500 phần tử, THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` và không tạo job | AC-CMS-003 | TC-CMS-TEN-03 |
| F045-AC-17 | KHI một bước saga lỗi tạm thời, HỆ THỐNG PHẢI thử lại theo lịch backoff của engine saga với cùng `Idempotency-Key` (không bước nào có hiệu lực hai lần) và kết thúc job `partially_failed`/`failed` kèm lỗi từng bước; `getTenantClone` PHẢI trả trạng thái, `id_map`, số bản ghi và lỗi | ADR-0036 · ADR-0040 | TC-SVC-004 |
| F045-AC-18 | KHI job clone kết thúc, HỆ THỐNG PHẢI ghi audit `tenant_data.cloned` ở cả tenant nguồn và tenant đích và phát `tenant_data.cloned` | [EVENT_DESIGN §4.1](../../docs/07-backend/EVENT_DESIGN.md#41-identity-tenancy-organization--sis) | TC-CMS-CLN-01 |

---

### User Story 4 — Thùng rác và khôi phục (Priority: P2)

System Admin xem bản ghi đã xóa của mọi tenant (và dữ liệu platform) theo loại tài nguyên, khôi phục có lý do; Tenant Admin được cấp quyền xem/khôi phục trong tenant mình. Không có xóa vĩnh viễn.

**Why this priority**: FR-CMS-014 (P2); cần US1 cho phạm vi liên tenant.

**Independent Test**: Xóa một câu hỏi tenant A → `listTrash?resource_type=question` thấy; `restoreTrashItem` có `reason` → câu hỏi xuất hiện lại; khôi phục khi đã có câu cùng mã → `409`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F045-AC-19 | KHI System Admin gọi `listTrash` với `resource_type` và `tenant_ids[]`, module identity PHẢI lấy danh sách từ module sở hữu loại đó (hoặc từ chính identity với loại do identity sở hữu), trả `deleted_at`, `deleted_by`, `delete_reason`, `restorable` và ghi audit | FR-CMS-014 · AC-CMS-004 | TC-TEN-015, TC-CMS-DEL-01 |
| F045-AC-20 | KHI System Admin gọi `restoreTrashItem` với `reason` (≥ 3 ký tự) và `X-Tenant-Id` = tenant của bản ghi (vắng với dữ liệu platform), HỆ THỐNG PHẢI khôi phục ở module sở hữu, phát `<entity>.restored`, ghi audit có lý do, và bản ghi xuất hiện lại ở API đọc thường | FR-CMS-014 · ADR-0028 | TC-TEN-015 |
| F045-AC-21 | NẾU khôi phục vi phạm unique (đã có bản ghi mới cùng mã), THÌ HỆ THỐNG PHẢI trả `409 CONFLICT` và không thay đổi dữ liệu | [ADMIN_PORTAL §3.8](../../docs/04-admin-portal/ADMIN_PORTAL.md#38-thùng-rác-adr-0028) | TC-CMS-DEL-02 |
| F045-AC-22 | KHI Tenant Admin có `tenant.trash.read`/`tenant.trash.restore` gọi `listTenantTrash`/`restoreTenantTrashItem`, HỆ THỐNG PHẢI chỉ trả và khôi phục bản ghi của tenant phiên; bản ghi tenant khác PHẢI trả `404 NOT_FOUND`; thiếu permission PHẢI trả `403 PERMISSION_DENIED` | FR-CMS-014 · AC-ORG-001 | TC-TEN-001 |
| F045-AC-23 | HỆ THỐNG PHẢI không có API xóa vĩnh viễn và API đọc thường PHẢI không trả bản ghi đã soft-delete | AC-ORG-006 · ADR-0070 | TC-TEN-022 |

---

### User Story 5 — Gọi module khác an toàn khi điều phối (Priority: P2)

module identity gọi facade của module sở hữu trong tiến trình qua `src/backend/pkg/modcall`; khi module đích lỗi hoặc chưa được nối thì báo lỗi có kiểm soát, không bao giờ cho qua.

**Why this priority**: Ràng buộc kiến trúc cho mọi luồng US2–US4.

**Independent Test**: Tắt module learning rồi gọi `listTrash?resource_type=question` → `503 UNAVAILABLE`; gọi internal API learning bằng token người dùng → `401`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F045-AC-24 | HỆ THỐNG PHẢI gọi module khác chỉ qua facade sinh từ spec internal, bằng adapter sau port `OwnerServiceGateway` dùng `src/backend/pkg/modcall` truyền trong `context` span tracing, deadline, `TenantScope`, Principal gốc (on-behalf) và `Idempotency-Key` cho thao tác ghi; NẾU module không có trong `x-allowed-callers` import facade platform ops, THÌ `make arch-check` PHẢI fail | AGENTS R29 · ADR-0074 | TC-SVC-006 |
| F045-AC-25 | NẾU facade của module sở hữu `resource_type` trả lỗi hạ tầng, vượt deadline hoặc module chưa được nối trong binary, THÌ HỆ THỐNG PHẢI trả `503 UNAVAILABLE` cho thao tác đọc/ghi đồng bộ và không suy ra kết quả từ cache | ADR-0074 · [API_DESIGN §12.3](../../docs/07-backend/API_DESIGN.md#123-resilience-mặc-định-confirmed) | TC-SVC-004 |
| F045-AC-26 | NẾU body chứa chuỗi vượt `maxLength` (`reason`, `q`) hoặc mảng vượt giới hạn (`tenant_ids` > 100), THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` chỉ rõ trường | ADR-0041 | TC-TEN-024 |

---

### User Story 6 — Hợp nhất hai tài khoản của cùng một người (Priority: P3)

Một học sinh có hai tài khoản (phát hiện qua issue `duplicate_identifier`/`possible_duplicate` của F011). School Admin đề nghị gộp khi xử lý issue trong scope trường; System Admin duyệt và gọi `POST /api/v1/admin/users:merge`; module identity chạy saga `user_merge`, các service chuyển dữ liệu sang tài khoản đích; tài khoản nguồn bị soft-delete.

**Why this priority**: Trường hợp hiếm, xử lý thủ công được tạm thời; cần trước khi vận hành nhiều trường (IDENTITY_MAPPING §7: "school_admin + platform review").

**Independent Test**: Tạo hai tài khoản `hs.trung.a1`, `hs.trung.a2` cùng tenant `th-demo-a` + issue `duplicate_identifier` đã được `ad.truong.a` đề nghị gộp; `sa.admin` gọi `mergeUsers` (from = a2, into = a1) → `202` + `merge_id`; saga `succeeded`; `identity.user_merges` có một dòng; đăng nhập bằng định danh của a2 vào a1; attempt/XP của a2 thấy ở a1; a2 không còn ở API đọc thường.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F045-AC-27 | KHI System Admin gọi `mergeUsers` (`POST /admin/users:merge`, `X-Tenant-Id` = tenant đích, `Idempotency-Key`) với `from_user_id`, `into_user_id`, `reason`, `reconciliation_issue_id` của issue đã được School Admin đề nghị gộp, module identity PHẢI trả `202` kèm `merge_id`, tạo saga `user_merge` và ở bước identity chuyển `external_identities` + `sis_identity_links` sang user đích, ghi `identity.user_merges`, soft-delete user nguồn (`delete_reason = merged`) và đóng issue | [IDENTITY_MAPPING §7](../../docs/09-identity/IDENTITY_MAPPING.md#7-liên-kết-tài-khoản-account-linking) · AC-AUTH-026 · ADR-0067 | TC-AUTH-080 |
| F045-AC-28 | KHI bước identity xong, worker PHẢI gọi `mergeUserData` của facade learning, game, competition, chat với `Idempotency-Key = merge_id`; module game PHẢI giữ một GameProfile và gộp ledger XP bằng bản ghi bù, module learning PHẢI giữ attempt theo `student_id` gốc và thêm liên kết sang user đích; không bước nào xóa vật lý | [IDENTITY_MAPPING §7](../../docs/09-identity/IDENTITY_MAPPING.md#7-liên-kết-tài-khoản-account-linking) · [ACCOUNT_PROVISIONING §7b](../../docs/09-identity/ACCOUNT_PROVISIONING.md#7b-facade-platform-mà-mỗi-module-phải-cung-cấp-saga-adr-0074) · ADR-0028 | TC-SVC-004 |
| F045-AC-29 | NẾU một module trả `blocked` hoặc lỗi quá số lần thử của engine saga, THÌ saga PHẢI dừng ở `failed` kèm lỗi từng bước, không chạy lại bước đã `done`, phát cảnh báo `SagaStuck`; `getUserMerge` PHẢI trả trạng thái và lỗi từng bước | [ACCOUNT_PROVISIONING §7b](../../docs/09-identity/ACCOUNT_PROVISIONING.md#7b-facade-platform-mà-mỗi-module-phải-cung-cấp-saga-adr-0074) · ADR-0074 | TC-SVC-004 |
| F045-AC-30 | NẾU người gọi không phải `system_admin` có `platform.user.merge` (kể cả School Admin gọi trực tiếp), THÌ HỆ THỐNG PHẢI trả `403 PERMISSION_DENIED` và audit `authz.denied`; NẾU một trong hai user không có membership ở tenant đích, THÌ HỆ THỐNG PHẢI trả `404 NOT_FOUND` | AGENTS R20 · ADR-0027 · [IDENTITY_MAPPING §7](../../docs/09-identity/IDENTITY_MAPPING.md#7-liên-kết-tài-khoản-account-linking) | TC-TEN-004, TC-TEN-001 |
| F045-AC-31 | NẾU `from_user_id = into_user_id`, hai user khác `user_type`, user nguồn đã bị gộp (UNIQUE `from_user_id`) hoặc đã `anonymized`, hoặc issue không ở trạng thái đề nghị gộp, THÌ HỆ THỐNG PHẢI trả `409 CONFLICT` và không tạo saga; NẾU `reason` ngắn hơn 3 hoặc dài hơn 1000 ký tự, THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` | [IDENTITY_MAPPING §7](../../docs/09-identity/IDENTITY_MAPPING.md#7-liên-kết-tài-khoản-account-linking) · ADR-0041 | TC-TEN-024 |
| F045-AC-32 | KHI gửi lại `mergeUsers` cùng `Idempotency-Key` và cùng payload, HỆ THỐNG PHẢI trả cùng `merge_id`; cùng key khác payload PHẢI trả `409 IDEMPOTENCY_KEY_REUSED`; mỗi lần gộp PHẢI ghi audit `user.merged` ở tenant đích (from, into, `merge_id`, lý do) không chứa PII | AGENTS R10, R14 · [API_DESIGN §6](../../docs/07-backend/API_DESIGN.md#6-idempotency) | — |
| F045-AC-33 | KHI saga `succeeded`, đăng nhập bằng định danh ngoài cũ của user nguồn PHẢI vào user đích; user nguồn PHẢI không xuất hiện ở API đọc thường và hiện trong Thùng rác với `restorable = false` | [IDENTITY_MAPPING §7](../../docs/09-identity/IDENTITY_MAPPING.md#7-liên-kết-tài-khoản-account-linking) · ADR-0028 | TC-AUTH-080 |

### Edge Cases

- NẾU System Admin gán cho tenant `disabled` hoặc đã soft-delete, THÌ HỆ THỐNG PHẢI từ chối tenant đó bằng `422` và không gán phần còn lại một nửa (F045-AC-07, F045-AC-16).
- NẾU tài nguyên được gỡ gán trong khi học sinh đang làm bài dùng nó, THÌ lượt đang làm giữ version đã ghim (ADR-0043); chỉ không giao mới được (F045-AC-10).
- NẾU clone tham chiếu chéo module (quest → question set), THÌ bước của module phụ thuộc chạy trước và `id_map` truyền sang bước sau (F045-AC-13).
- NẾU tenant đích đã có bản ghi trùng mã khi clone, THÌ module đích PHẢI đặt mã mới cho bản sao hoặc báo lỗi bước, không ghi đè bản ghi có sẵn.
- NẾU học sinh bị dọn (ADR-0032) xuất hiện ở Thùng rác, THÌ khôi phục học sinh đi theo saga của F016, không qua luồng khôi phục chung.
- NẾU job clone đang chạy và System Admin gọi lại cùng `Idempotency-Key`, THÌ HỆ THỐNG PHẢI trả job cũ, không tạo job mới.
- NẾU user nguồn đang có game session active hoặc bài kiểm tra đang làm khi gộp, THÌ module tương ứng trả `blocked` (F045-AC-29); System Admin chạy lại sau.
- NẾU user nguồn có membership ở tenant khác tenant đích, THÌ chỉ dữ liệu trong tenant đích được gộp; membership tenant khác chuyển sang user đích ở bước identity.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-ORG-008 | Phạm vi liên tenant của System Admin (`tenant_ids[]`, cột tenant, audit), bỏ qua `tenant_ids` của người dùng tenant | [ADMIN_PORTAL §2.5](../../docs/04-admin-portal/ADMIN_PORTAL.md#25-chế-độ-liên-tenant-của-system-admin-adr-0027-6) |
| FR-ORG-012 | Saga gán/gỡ gán dữ liệu platform cho tenant, gán mặc định cho tenant mới | [MULTITENANCY §10](../../docs/06-architecture/MULTITENANCY.md#10-gán-và-clone-dữ-liệu-giữa-tenant-adr-0029) |
| FR-ORG-013 | Saga clone giữa tenant, không clone dữ liệu học sinh | [ADMIN_PORTAL §3.7](../../docs/04-admin-portal/ADMIN_PORTAL.md#37-gán-dữ-liệu-cho-tenant-và-clone-giữa-tenant-adr-0029--chỉ-system-admin-workspace-d) |
| FR-CMS-013 | Backend dropdown tenant (`listTenants?q`), đọc liên tenant, ghi luôn có tenant đích | [ADMIN_PORTAL §2.5](../../docs/04-admin-portal/ADMIN_PORTAL.md#25-chế-độ-liên-tenant-của-system-admin-adr-0027-6) |
| FR-CMS-014 | Thùng rác System Admin (mọi tenant + platform) và Tenant Admin (tenant mình), khôi phục có lý do | [ADMIN_PORTAL §3.8](../../docs/04-admin-portal/ADMIN_PORTAL.md#38-thùng-rác-adr-0028) |
| — (IDENTITY_MAPPING §7) | Saga hợp nhất tài khoản `/admin/users:merge` (PRD chưa có mã FR) | [IDENTITY_MAPPING §7](../../docs/09-identity/IDENTITY_MAPPING.md#7-liên-kết-tài-khoản-account-linking) |
| FR-CMS-015 | API gán và clone của System Admin | [ADMIN_PORTAL §3.7](../../docs/04-admin-portal/ADMIN_PORTAL.md#37-gán-dữ-liệu-cho-tenant-và-clone-giữa-tenant-adr-0029--chỉ-system-admin-workspace-d) |

### Key Entities

- **TenantResourceAssignment**: liên kết tài nguyên platform ↔ tenant được dùng chỉ đọc; nằm ở module sở hữu tài nguyên.
- **TenantDefaultAssignment**: tài nguyên tự gán cho mọi tenant mới.
- **TenantCloneJob**: yêu cầu clone (tenant nguồn, một tenant đích, loại, danh sách bản ghi), trạng thái, ánh xạ ID, thống kê.
- **SagaRun / SagaStep**: tiến trình điều phối nhiều module (clone, khôi phục), từng bước idempotent theo saga.
- **TrashItem**: bản ghi đã soft-delete hiển thị trong Thùng rác (loại, id, tenant, người xóa, lý do, có khôi phục được không).
- **UserMerge**: một lần gộp tài khoản nguồn vào tài khoản đích (append-only), gắn saga `user_merge` và issue đối soát.
- **CrossTenantScope**: phạm vi đọc nhiều tenant chỉ System Admin có, luôn là danh sách tenant tường minh và được audit.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `module_call_deadline` | Deadline bước saga (5 s) đặt vào `context` khi gọi facade; thử lại do engine saga với cùng `Idempotency-Key` (không retry/circuit breaker/bulkhead trong tiến trình, ADR-0074) |
| `idempotency_record_ttl` | Thời gian giữ kết quả theo `Idempotency-Key` |
| `portal_session_sso` | Phiên portal và `auth_time` của role platform (MFA) |
| `tenant_header_required` | Quy tắc header `X-Tenant-Id` (đọc liên tenant không gửi, ghi gửi tenant đích) |
| `principal_cache_ttl` | Hiệu lực thu hồi quyền platform |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Chỉ `system_admin` có `platform.cross_tenant.read`; role tenant không có `platform.*` | AGENTS R20 · ADR-0027 · ADR-0066 |
| Chỉ soft-delete; không xóa vật lý; khôi phục qua Thùng rác | AGENTS R21 · ADR-0028 · ADR-0070 |
| Gán/clone chỉ System Admin; clone không mang dữ liệu học sinh | AGENTS R22 · ADR-0029 |
| Không đọc/ghi DB module khác; chỉ facade + event | AGENTS R13, R29 · ADR-0037, ADR-0040, ADR-0074 |
| Bỏ filter chỉ qua `tenancy.Bypass` có audit | AGENTS R27 · ADR-0039 |
| Thao tác ghi có `Idempotency-Key` + audit | AGENTS R10 |
| Mọi chuỗi có giới hạn độ dài | AGENTS R28 · ADR-0041 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% endpoint của F045 có test authorization (System Admin / role platform khác / role tenant / thiếu MFA) và test tenant isolation pass.
- **SC-002**: 100% lần đọc liên tenant và thao tác gán, gỡ gán, clone, khôi phục có bản ghi audit tương ứng (kiểm bằng truy vấn đối soát trong test).
- **SC-003**: Trong bộ test clone, 0 bản ghi học sinh, enrollment, attempt, kết quả, mastery được tạo ở tenant đích.
- **SC-004**: Chạy lại bất kỳ bước saga nào hai lần cho cùng job không làm số bản sao thay đổi.
- **SC-005**: Kịch bản quickstart chạy hết với `sa.admin`, `ad.donvi`, `ad.truong.a`, `ad.truong.b`, `cm.noidung`; mọi `F045-AC-*` có test pass.
- **SC-006**: Sau mỗi kịch bản gộp tài khoản, 0 dữ liệu học tập/game của tài khoản nguồn bị mất (đối soát số attempt, XP trước/sau) và 0 dòng bị xóa vật lý.

## Ngoài phạm vi

- Phía callee platform ops trong module learning (list/restore/clone, ghi assignment) — F020; trong game/competition/chat — feature nền của các module đó.
- Kiểm tra chỉ đọc và `RESOURCE_NOT_ASSIGNED` khi tenant dùng nội dung — module sở hữu (F020, F026, F041, F078); F045 chỉ kiểm end-to-end bằng facade test.
- Quản lý tenant/trường (tạo, sửa, Keycloak Organization) — F010; audit log tập trung và cài đặt — F015; khôi phục học sinh bị dọn — F016.
- Giao diện workspace D (dropdown tenant, Thùng rác, hộp thoại gán/clone) — F057; dropdown tenant trên mọi màn hình portal — F051.
- Gán tài nguyên chat (sticker pack, GIF, danh mục từ nhạy cảm) — feature chat (F116, F118).
- Phát hiện trùng và School Admin đề nghị gộp qua reconciliation issue — F011; màn duyệt gộp tài khoản trên portal — F057; phía callee `mergeUserData` của learning/game/competition/chat — feature nền của từng module (F020, F014, F100, F115).
- Xóa vật lý, gỡ partition — không có trong Release 1.0 (ADR-0070).

## Assumptions

- Danh sách `resource_type` được gán/clone/khôi phục và module sở hữu nằm trong một bảng định tuyến trong code module identity, theo [DATABASE_DESIGN §2.17](../../docs/07-backend/DATABASE_DESIGN.md#217-projection-ref-và-bảng-hạ-tầng-infra-mọi-database); loại thuộc service chưa triển khai trả `503`.
- Gán cho "mọi tenant hiện tại" (`apply_to_all_tenants`) và gán cho tenant mới là [Confirmed].
- Thùng rác của Tenant Admin là [Confirmed] (ADMIN_PORTAL §3.8); quyền cấp qua role.
- Permission `platform.user.merge`, trạng thái issue `merge_requested`, endpoint `getUserMerge` và cột `identity.user_merges.saga_id` là [Confirmed] (chưa có trong ADMIN_PORTAL §2.2, SIS_INTEGRATION, spec, DATABASE_DESIGN).
- Loại `class_template` (lớp mẫu không có học sinh) do identity sở hữu, cấu trúc chi tiết [Confirmed] vì DATABASE_DESIGN chưa định nghĩa bảng.
