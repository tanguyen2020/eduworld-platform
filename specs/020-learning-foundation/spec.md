# Feature Specification: F020 — Nền module learning: projection, mô hình nội dung, gán platform→tenant, thùng rác tenant

**Feature Branch**: `020-learning-foundation` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `learning` · **Mốc**: M-C2 · **Phụ thuộc**: F003, F010
**Input**: Nền dữ liệu chung cho mọi nội dung học tập: projection dữ liệu identity, mô hình item/version có `owner_scope`, gán dữ liệu platform cho tenant, soft-delete + Thùng rác, khung clone. FR nguồn: FR-CNT-001, FR-CNT-002, FR-CNT-005, FR-CMS-014, FR-CMS-015.

## Clarifications

### Session 2026-11-16

- Q: Tenant ghi bản ghi gán vào đâu, ai phát `tenant_data.assigned`? → A: Bảng `infra.tenant_resource_assignments` nằm trong `eduworld_learning`; module identity điều phối và phát `tenant_data.assigned/unassigned`; module learning chỉ ghi qua facade `learning` do module identity gọi ([ADR-0029](../../docs/adr/ADR-0029-system-admin-gan-du-lieu-cho-tenant-va-clone-du-lieu-giua-te.md), [MULTITENANCY §10](../../docs/06-architecture/MULTITENANCY.md#10-gán-và-clone-dữ-liệu-giữa-tenant-adr-0029), [EVENT_DESIGN §4.1](../../docs/07-backend/EVENT_DESIGN.md#41-identity-tenancy-organization--sis)).
- Q: F020 chưa có loại nội dung cụ thể, kiểm Thùng rác/clone bằng gì? → A: F020 cung cấp registry `ResourceHandler`; loại cụ thể đăng ký ở F021 (curriculum), F022 (question), F024 (question set, lesson, blueprint). Test của F020 dùng entity fixture chỉ có trong `tests/integration/fixtures` (không biên dịch vào binary chạy thật, R17).
- Q: `owner_scope` gồm những giá trị nào? → A: `platform`/`tenant` theo [CONTENT_MODEL §4](../../docs/02-learning/CONTENT_MODEL.md#4-phạm-vi-sở-hữu-owner_scope--adr-0005-adr-0029) và DATABASE_DESIGN (chốt D8 của CODE_PLAN §3).

### Session 2026-11-17 (ADR-0089)

- Q: Mixin nào cho nội dung lai và RLS có lọc `deleted_at`? → A: `ContentItemMixin` dựa trên `entx.HybridContent` (tên cũ `PlatformBase`); hiển thị nội dung platform cho tenant chỉ qua `infra.tenant_resource_assignments`, không có cờ "public cho mọi tenant". Policy RLS loại H có điều kiện tenant + gán, loại T/TA chỉ điều kiện tenant, không policy nào có `deleted_at`; `deleted_at IS NULL` do interceptor và view `<table>_live` (cho `learning_readonly`) bảo đảm.
- Q: Thùng rác và clone bỏ filter bằng gì? → A: `tenancy.Bypass(ctx, "trash")` và `tenancy.Bypass(ctx, "tenant_clone")` (danh sách đóng), audit `cross_tenant.accessed` + metric `eduworld_tenant_filter_bypass_total{module,reason}`.

### Session 2026-11-19 (triển khai)

- Q: Bảng `ref.*` dùng mixin `entx` nào? → A: Không dùng mixin; bảng viết tay trong migration `20261122000001_f020_foundation.sql`, ghi bằng SQL tường minh (`persistence/write/ref_repo.go`), vì mọi mixin thêm cột audit/soft-delete trái DATABASE_DESIGN §2.17 quy tắc 2 và F020-AC-02 (chỉ cột tối thiểu). `ref.tenants` có thêm `is_qa` theo §2.17 [Confirmed].
- Q: Consumer kiểm "tenant của bản ghi projection được tham chiếu" thế nào khi RLS chỉ cho thấy tenant của phiên? → A: Hàm `ref.projection_tenant(table, id)` `SECURITY DEFINER` (chủ sở hữu bảng) trả tenant sở hữu dòng; bảng `ref.*` có `tenant_id` bật RLS điều kiện tenant nhưng **không** `FORCE` để hàm này đọc được. Lệch tenant → `outbox.Permanent` → `DLQ_LEARNING` + `eduworld_consumer_rejected_total{consumer,reason="tenant_mismatch"}`; thiếu tenant cha (FK `ref.tenants`) → lỗi tạm, NAK có delay rồi DLQ sau `MaxDeliver` [Confirmed].
- Q: Version để so khi payload không có `source_version` (`class.synced`, `province.updated`, `student.cleaned_up/restored`, `school.synced` do F011 phát)? → A: Theo schema payload: `occurred_at` của envelope (micro giây). `student.cleaned_up/restored` chỉ đổi `deleted_at` theo thời điểm event, không đụng `source_version`. Rủi ro: `school.synced` (không `source_version`) và `school.updated` (có) cùng ghi `ref.schools` — xem Câu hỏi mở F020-1 [Confirmed].
- Q: Consumer nhận những subject nào? → A: Đúng 10 subject: `tenant.created/updated`, `school.synced/updated` (`school.updated` theo DATABASE_DESIGN §2.17), `class.synced`, `student.synced/cleaned_up/restored`, `enrollment.changed`, `province.updated` (event platform: consumer `Platform: true`). `class.synced` không có `academic_year_id` → cột để NULL; `enrollment.changed` thiếu `start_date` (khóa) → DLQ `invalid_payload` [Confirmed].
- Q: Cột số version của `ContentVersionMixin`? → A: `version_no` (khớp ER CONTENT_MODEL §5), vì `version` là cột optimistic concurrency của mixin `entx` (ADR-0039). DATABASE_DESIGN §3.3 cập nhật [Confirmed].
- Q: Bảng version (H) hiển thị cho tenant theo assignment nào? → A: Theo assignment của **item**: `entx.HybridContent` thêm trường `ResourceIDColumn` (mặc định cột khóa) để interceptor so `resource_id` với cột item (`question_id`), đồng bộ với `tenancy.HybridResource(resource_type, id_column)` của RLS [Confirmed]. Đồng thời sửa lỗi `entx` sinh `SELECT "1"` trong subquery assignment.
- Q: Ghi `infra.tenant_resource_assignments` bằng role nào, scope nào? → A: Mixin `entx.Base` (thêm cột `created_*`, `updated_*`, `version`, `delete_reason`); không RLS (schema `infra`); `learning_app` chỉ `SELECT` (policy H và interceptor cần đọc), ghi bằng `learning_platform` dưới `CrossTenantScope` đúng các tenant yêu cầu (audit `cross_tenant.accessed`, mode `write`); kiểm tài nguyên bằng `PlatformScope` của System Admin (chỉ dòng `tenant_id IS NULL`, chưa xóa). View `infra.tenant_resource_assignments_live` cho `learning_readonly`.
- Q: Audit gán/gỡ gán/clone thuộc tenant nào? → A: Mỗi assignment một audit `tenant_data.assigned/unassigned` gắn tenant đó (envelope `tenant_id`, actor = Principal gốc); clone ghi `tenant_data.cloned` ở tenant nguồn và tenant đích (F020-AC-30); xóa/khôi phục ghi `content.deleted/restored` ở tenant sở hữu (platform khi nội dung platform) [Confirmed].
- Q: `restorable` của Thùng rác nằm ở đâu trong contract? → A: Thêm `restorable: boolean` (tùy chọn) vào `TrashItem` của `common/internal-common.v1.yaml`; `false` khi một bản ghi sống giữ khóa unique (khôi phục sẽ `409`) [Confirmed].
- Q: Idempotency của facade? → A: `infra.idempotency_records` với `method = FACADE`, `route_template = learning.<operationId>`, khóa = `Idempotency-Key` (UUIDv7) của Principal gốc; dùng lại cùng khóa với request khác → `422 IDEMPOTENCY_KEY_REUSED`. Bước clone idempotent theo `(job_id, resource_type)` (`user_id = idem_key = job_id`, ngày theo timestamp UUIDv7 của `job_id`), hiệu lực trong `idempotency_record_ttl` [Confirmed].
- Q: `revokeTenantAssignments` lưu lý do ở đâu? → A: `reason` 3–1000 ký tự (contract); `delete_reason varchar(64)` lưu `unassigned: <64 ký tự đầu>`, lý do đầy đủ trong audit [Confirmed].
- Q: Kiểm `RESOURCE_NOT_ASSIGNED` khi tham chiếu (F020-AC-18)? → A: Query `CheckReference`: thấy dưới scope tenant → hợp lệ; không thấy thì đọc tồn tại dưới `authz.SystemPlatformScope("learning-reference-check")` (chỉ dòng platform, trả mã lỗi, không trả dữ liệu) → `403 RESOURCE_NOT_ASSIGNED`, ngược lại `404` [Confirmed].
- Q: Tenant Admin dùng Thùng rác? → A: `listTrash`/`restoreTrashItem` nhận Principal tenant có `tenant.trash.read/restore` và luôn giới hạn ở tenant phiên (bỏ qua `tenant_ids`; item tenant khác → `404` trước mọi kiểm tra khác) — giữ [Confirmed] của Assumptions.
- Q: F020-AC-19 "`make arch-check` PHẢI fail khi module khác `identity` import facade platform-ops"? → A: `archgen` kiểm ở mức package: `game`/`competition` được import `modules/learning/api` vì có operation khác (`createAttempt`, `createQuestionPlan`). Cưỡng chế theo operation bằng `x-allowed-callers` + `learningapi.AllowedCallers` + `pkg/modcall` (`403 CALLER_NOT_ALLOWED` lúc chạy) và contract test; module không gọi operation nào của learning (`chat`, `realtime`) bị `depguard` chặn import. Xem Câu hỏi mở F020-2.
- Q: F020-AC-05 chặn ghi `ref.*` bằng role? → A: Consumer và use case cùng chạy `learning_app`, nên chặn bằng kiến trúc: test `tests/unit/arch_ref_write_test.go` (SQL ghi `ref.*` chỉ trong `ref_repo.go`; `ProjectionWriter`/`UpsertProjection` chỉ dùng ở consumer) [Confirmed].
- Q: T001 file `.golangci.yml`/`.go-arch-lint.yml` riêng của module? → A: Không tạo: luật tầng và ranh giới module nằm tập trung trong `src/backend/.golangci.arch.yml` do `tools/archgen` sinh (như module identity).
- Q: F020-AC-11/13 kiểm qua API nào khi F020 không có endpoint public? → A: Một route fixture (chỉ trong test) chạy qua chuỗi `pkg/httpx` thật + `scope.ForRead`; các endpoint nội dung của F021+ dùng cùng helper.


### Session 2026-11-20 (owner chốt — mục "Câu hỏi mở")

- F020-1 → **Chốt:** F011 phát `source_version` trong `school.synced`/`class.synced` (+ `academic_year_id`, `is_virtual` của lớp); projection learning so theo `source_version` khi có, `occurred_at` chỉ là dự phòng. Task ở F011 và tasks.md này.
- F020-2 → **Chốt:** `arch-check` theo operation (task ở F001).
- F020-3 → đã giải quyết (nối binary 2026-11-20).
- F020-4 → **Chốt:** lint cấm `UpdateOne*` (task ở F003).
- Triển khai T044: `source_version` (version aggregate, nhỏ) và `occurred_at` µs (lớn hơn 10¹²) cùng nằm trong cột `source_version`; quy tắc trộn: cùng loại chỉ áp bản mới hơn, `source_version` thay dòng đang xếp theo thời gian, event chỉ có `occurred_at` không thay dòng đã có `source_version` (tránh event cũ phát lại ghi đè) [Proposed]. Áp cho `ref.schools`, `ref.classes`; bảng `ref.*` khác giữ so sánh cũ.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Projection dữ liệu identity trong module learning (Priority: P1)

module learning cần biết tenant, trường, lớp, học sinh, enrollment, tỉnh để kiểm tenant hợp lệ và lọc dữ liệu mà không gọi đồng bộ module identity.

**Why this priority**: Mọi bảng tenant-scoped có FK tới `ref.tenants`; không có projection thì không ghi được dữ liệu nào.

**Independent Test**: Phát event identity mẫu lên NATS (testcontainers), kiểm bảng `ref.*` cập nhật đúng, event cũ hơn bị bỏ qua, event giao lại không xử lý hai lần.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F020-AC-01 | KHI nhận `tenant.created`/`tenant.updated`, module learning PHẢI upsert `ref.tenants` theo `source_version` và bỏ qua bản có `source_version` nhỏ hơn hoặc bằng bản đang lưu | [DATABASE_DESIGN §2.17](../../docs/07-backend/DATABASE_DESIGN.md#217-projection-ref-và-bảng-hạ-tầng-infra-mọi-database) | TC-SVC-003 |
| F020-AC-02 | KHI nhận `school.synced`, `class.synced`, `student.synced`, `student.cleaned_up`, `student.restored`, `enrollment.changed`, `province.updated`, module learning PHẢI cập nhật đúng bảng `ref.*` tương ứng chỉ với các cột tối thiểu liệt kê ở DATABASE_DESIGN §2.17 | NFR-PRIV-002 · [DATABASE_DESIGN §2.17](../../docs/07-backend/DATABASE_DESIGN.md#217-projection-ref-và-bảng-hạ-tầng-infra-mọi-database) | TC-SVC-008 |
| F020-AC-03 | KHI cùng một event được giao lại, consumer PHẢI xử lý đúng một lần nhờ `infra.processed_events` | NFR-MAINT-004 · ADR-0042 | TC-SVC-003 |
| F020-AC-04 | NẾU event mang `tenant_id` khác `tenant_id` của bản ghi projection mà nó tham chiếu, THÌ consumer PHẢI từ chối, chuyển `DLQ_LEARNING` và tăng metric `eduworld_consumer_rejected_total` | [MULTITENANCY §4.1](../../docs/06-architecture/MULTITENANCY.md#41-principal-và-tenantscope) | — |
| F020-AC-05 | HỆ THỐNG PHẢI chặn mọi use case nghiệp vụ ghi bảng `ref.*` (chỉ consumer projection được ghi) | [DATABASE_DESIGN §2.17](../../docs/07-backend/DATABASE_DESIGN.md#217-projection-ref-và-bảng-hạ-tầng-infra-mọi-database) | TC-TEN-017 |

---

### User Story 2 — Mô hình nội dung chung có `owner_scope` và cô lập tenant (Priority: P1)

Mọi loại nội dung học tập (question, question set, blueprint, lesson, learning material, learning path, activity, curriculum) dùng chung cách định danh item/version, metadata bắt buộc và quy tắc ai thấy gì.

**Why this priority**: F021–F027 xây trên mô hình này; sai ở đây là rò dữ liệu giữa tenant.

**Independent Test**: Với entity fixture, tạo bản ghi tenant A, tenant B, platform (gán/không gán); đọc bằng người dùng từng tenant và System Admin; kiểm kết quả và RLS.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F020-AC-06 | HỆ THỐNG PHẢI chỉ chấp nhận loại nội dung trong `LearningContentType` (cộng `curriculum` cho Thùng rác/gán/clone theo `ResourceType`); loại khác PHẢI bị từ chối `422 VALIDATION_FAILED` | FR-CNT-001 · [CONTENT_MODEL §2](../../docs/02-learning/CONTENT_MODEL.md#2-các-loại-nội-dung) | — |
| F020-AC-07 | KHI lưu một version nội dung, HỆ THỐNG PHẢI kiểm metadata bắt buộc theo loại (khối, môn, chủ đề, LO, độ khó, version, status, tác giả, người duyệt khi ≥ approved, `published_at` khi published, `content_language`, `curriculum_version_id`) và trả `422 VALIDATION_FAILED` liệt kê từng field thiếu | FR-CNT-002 · [CONTENT_MODEL §3](../../docs/02-learning/CONTENT_MODEL.md#3-metadata-bắt-buộc-confirmed--master-prompt-42) | — |
| F020-AC-08 | HỆ THỐNG PHẢI bảo đảm `(owner_scope = 'platform') = (tenant_id IS NULL)` ở domain và bằng CHECK trong DB | FR-CNT-005 · ADR-0029 | — |
| F020-AC-09 | TRONG KHI người dùng tenant đọc nội dung, HỆ THỐNG PHẢI chỉ trả bản ghi của tenant phiên hoặc bản ghi platform có assignment active cho tenant đó, luôn lọc `deleted_at IS NULL`; ID ngoài phạm vi PHẢI trả `404 NOT_FOUND` | AC-ORG-001 · AC-ORG-007 · [MULTITENANCY §4.2](../../docs/06-architecture/MULTITENANCY.md#42-đọc-nội-dung-lai-riêng-tenant--được-gán) | TC-TEN-001 |
| F020-AC-10 | NẾU repository chạy không có `TenantScope` trong context, THÌ HỆ THỐNG PHẢI trả `ErrMissingTenantScope`; NẾU query bỏ điều kiện tenant, THÌ RLS PHẢI trả 0 dòng | AC-ORG-005 · NFR-SEC-006 | TC-TEN-005, TC-TEN-021 |
| F020-AC-11 | NẾU người dùng tenant gửi `tenant_ids`, THÌ HỆ THỐNG PHẢI bỏ qua tham số; NẾU `X-Tenant-Id` khác tenant phiên, THÌ PHẢI trả `403 TENANT_MISMATCH` và ghi audit | AC-CMS-002 · ADR-0065 | TC-TEN-011, TC-CMS-TEN-01 |
| F020-AC-12 | KHI System Admin tạo nội dung với `owner_scope = tenant` mà không có tenant đích, HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` | AC-CMS-003 | TC-CMS-TEN-02 |
| F020-AC-13 | TRONG KHI System Admin đọc danh sách với `tenant_ids` (rỗng = mọi tenant active), HỆ THỐNG PHẢI dùng `CrossTenantScope`, trả `tenant_id` mỗi dòng và ghi audit `cross_tenant.accessed` với `reason`, `tenant_ids` và `mode = read` (ADR-0089 §3) | AC-ORG-004 · AC-CMS-003 | TC-TEN-018, TC-CMS-TEN-03 |
| F020-AC-32 | KHI ghi dòng `tenant_id IS NULL` (nội dung platform loại H, `content_releases`/`content_transitions` platform loại TA\*), module learning PHẢI dùng client `learning_platform` dưới `PlatformScope` (`authz.PlatformScope(p)` cho nhân sự platform, `authz.SystemPlatformScope(job)` cho job); NẾU `learning_app` ghi dòng `tenant_id IS NULL`, THÌ RLS PHẢI từ chối; phiên tenant PHẢI chỉ đọc dòng platform đã được gán | ADR-0089 §2–3 · ADR-0029 | — |
| F020-AC-14 | HỆ THỐNG PHẢI giới hạn độ dài mọi chuỗi (MaxLen ở DB, `maxLength` ở API); vượt PHẢI trả `422 VALIDATION_FAILED` chỉ rõ field | NFR-MAINT-005 · ADR-0041 | TC-TEN-024 |

---

### User Story 3 — Gán nội dung platform cho tenant (Priority: P1)

System Admin gán một nội dung platform cho tenant A (qua module identity, F045); nhân sự tenant A thấy và dùng ở chế độ chỉ đọc, tenant B không thấy.

**Why this priority**: Nội dung chuẩn EduWorld (curriculum GDPT, ngân hàng câu hỏi platform) chỉ đến được trường qua gán (FR-CMS-015 P2).

**Independent Test**: Gọi facade `learning` từ fake bên gọi identity (Principal gốc trong `context`); kiểm tenant A thấy, tenant B không thấy, gỡ gán thì A không thấy.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F020-AC-15 | KHI module `identity` gọi `applyTenantAssignments` kèm `Idempotency-Key`, module learning PHẢI tạo một assignment active cho mỗi cặp (tài nguyên, tenant) chưa có và trả danh sách assignment; gọi lại cùng key PHẢI trả kết quả cũ | FR-CMS-015 · ADR-0029 | TC-TEN-019 |
| F020-AC-16 | NẾU tài nguyên không phải `owner_scope = platform`, đã soft-delete, hoặc tenant không có trong `ref.tenants` với `status = active`, THÌ module learning PHẢI trả `422 VALIDATION_FAILED` liệt kê phần tử lỗi và không ghi assignment nào | ADR-0029 | — |
| F020-AC-17 | KHI module `identity` gọi `revokeTenantAssignments`, module learning PHẢI soft-delete assignment; tenant PHẢI không còn thấy tài nguyên ở lần đọc kế tiếp, còn attempt/lịch sử đã tham chiếu version giữ nguyên | FR-CMS-015 · [MULTITENANCY §10](../../docs/06-architecture/MULTITENANCY.md#10-gán-và-clone-dữ-liệu-giữa-tenant-adr-0029) | TC-TEN-019 |
| F020-AC-18 | TRONG KHI tài nguyên platform được gán cho tenant, HỆ THỐNG PHẢI coi nó là chỉ đọc với người dùng tenant (ghi → `403 PERMISSION_DENIED`); tham chiếu tài nguyên platform chưa gán PHẢI trả `403 RESOURCE_NOT_ASSIGNED` | AC-CMS-005 · [API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md#41-catalog-mã-lỗi) | TC-CMS-ASG-01, TC-CMS-ASG-02 |
| F020-AC-19 | NẾU một module khác `identity` import facade platform-ops của module learning (ngoài `x-allowed-callers`), THÌ `make arch-check` PHẢI fail; operation platform-ops PHẢI KHÔNG có route HTTP nào | NFR-MAINT-006 · ADR-0074 | TC-SVC-005, TC-SVC-006 |
| F020-AC-20 | KHI gán hoặc gỡ gán thành công, module learning PHẢI ghi `audit.recorded` (`tenant_data.assigned`/`tenant_data.unassigned`, actor = Principal gốc (on-behalf) trong `context`) qua outbox trong cùng transaction và xóa cache liên quan của tenant | ADR-0029 · R10 | — |

---

### User Story 4 — Soft-delete và Thùng rác cho nội dung học tập (Priority: P2)

Biên tập viên xóa nội dung: nội dung biến khỏi danh sách, vào Thùng rác; System Admin (hoặc Tenant Admin được cấp) xem và khôi phục qua module identity.

**Why this priority**: FR-CMS-014 là P2, bắt buộc trước khi mở CMS cho nhiều người dùng.

**Independent Test**: Xóa bản ghi fixture, kiểm không xuất hiện ở API đọc; gọi `listTrash`/`restoreTrashItem` qua facade như module `identity`; thử khôi phục khi đã có bản ghi mới trùng mã.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F020-AC-21 | KHI người dùng có quyền xóa một item nội dung, HỆ THỐNG PHẢI đặt `deleted_at`, `deleted_by`, `delete_reason`, ghi audit `content.deleted` và phát `<resource_type>.deleted` qua outbox trong cùng transaction | FR-CMS-014 · ADR-0028 | TC-TEN-015 |
| F020-AC-22 | NẾU code gọi `Delete()` trên entity ent, THÌ hook PHẢI chuyển thành soft-delete; NẾU role `learning_app`/`learning_platform` chạy `DELETE`/`TRUNCATE`, THÌ PostgreSQL PHẢI từ chối | AC-ORG-006 | TC-TEN-016, TC-TEN-022 |
| F020-AC-23 | HỆ THỐNG PHẢI không trả bản ghi đã soft-delete ở bất kỳ API đọc thường nào | AC-ORG-006 · AC-CMS-004 | TC-TEN-015, TC-CMS-DEL-01 |
| F020-AC-24 | KHI module `identity` gọi `listTrash`, module learning PHẢI trả bản ghi đã xóa thuộc các `resource_type` đã đăng ký (lọc `tenant_ids` nếu có, cờ `restorable`) qua `tenancy.Bypass(ctx, "trash")` có audit `cross_tenant.accessed` và metric `eduworld_tenant_filter_bypass_total{module,reason}` | ADR-0028 · [MULTITENANCY §9](../../docs/06-architecture/MULTITENANCY.md#9-soft-delete-adr-0028) | TC-CMS-DEL-01, TC-TEN-023 |
| F020-AC-25 | KHI module `identity` gọi `restoreTrashItem` hợp lệ, module learning PHẢI xóa `deleted_*`, ghi audit `content.restored`, phát `<resource_type>.restored`; gọi lại cùng `Idempotency-Key` PHẢI trả kết quả cũ | ADR-0028 | TC-CMS-DEL-01 |
| F020-AC-26 | NẾU khôi phục vi phạm partial unique, THÌ module learning PHẢI trả `409 CONFLICT` và không đổi dữ liệu | [ADMIN_PORTAL §3.8](../../docs/04-admin-portal/ADMIN_PORTAL.md#38-thùng-rác-adr-0028) | TC-CMS-DEL-02 |
| F020-AC-31 | Migration của F020 PHẢI tạo policy RLS loại T/TA chỉ có điều kiện tenant và loại H có điều kiện tenant + gán, không policy nào chứa `deleted_at`, cùng view `<table>_live` cho `learning_readonly` (không quyền trên bảng gốc); KHI xóa một item nội dung bằng role `learning_app` dưới RLS, lệnh soft-delete PHẢI thành công và item PHẢI biến mất khỏi đọc qua interceptor và view `_live` | ADR-0089 §1 · [DATABASE_DESIGN §4](../../docs/07-backend/DATABASE_DESIGN.md#4-row-level-security) | TC-TEN-015 |

---

### User Story 5 — Khung bước clone nội dung giữa tenant (Priority: P3)

Saga clone của module identity gọi module learning để sao sâu nội dung từ tenant nguồn sang tenant đích.

**Why this priority**: Clone là P3 (FR-CMS-015); cần khung sẵn để F021/F022/F024 đăng ký loại.

**Independent Test**: Gọi `executeTenantCloneStep` với `resource_type` fixture, kiểm bản sao, `id_map`, idempotency, không sao dữ liệu học sinh.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F020-AC-27 | KHI module `identity` gọi `executeTenantCloneStep` cho `resource_type` đã đăng ký, module learning PHẢI tạo bản sao sâu ID mới, `tenant_id` = tenant đích, version mới ở `draft`, ghi `cloned_from_id`, `cloned_from_tenant_id`, `cloned_at`, `cloned_by`, trả `id_map`; gọi lại cùng `(job_id, resource_type)` PHẢI trả kết quả cũ | FR-CMS-015 · ADR-0029 | TC-TEN-020, TC-CMS-CLN-01 |
| F020-AC-28 | HỆ THỐNG PHẢI không sao chép dữ liệu học sinh (attempt, câu trả lời, learning event, mastery, bài nộp) trong bất kỳ bước clone nào; tham chiếu tới dữ liệu platform PHẢI giữ nguyên | R22 · AC-CMS-005 | TC-CMS-CLN-01 |
| F020-AC-29 | NẾU `resource_type` chưa có cloner đăng ký hoặc một bản ghi nguồn không thuộc tenant nguồn, THÌ module learning PHẢI trả trạng thái `failed` cho phần tử đó kèm lý do và không ghi bản sao dở dang | ADR-0029 | — |
| F020-AC-30 | KHI hoàn tất bước clone, module learning PHẢI ghi audit ở cả tenant nguồn và tenant đích | [MULTITENANCY §10](../../docs/06-architecture/MULTITENANCY.md#10-gán-và-clone-dữ-liệu-giữa-tenant-adr-0029) | TC-CMS-CLN-01 |

### Edge Cases

- NẾU event projection đến trước `tenant.created` của tenant đó, THÌ consumer PHẢI NAK có delay và thử lại theo chính sách retry rồi mới DLQ (F020-AC-04).
- NẾU tenant chuyển `disabled`, THÌ gán mới cho tenant đó bị từ chối (F020-AC-16); assignment cũ giữ nguyên.
- NẾU gán lại cho tenant đã có assignment active, THÌ không tạo dòng mới (F020-AC-15).
- NẾU gỡ gán một tài nguyên đang được dùng trong bài giao/attempt đang mở, THÌ lượt đang chạy giữ version đã ghim (F020-AC-17).
- NẾU xóa item có version `published`, THÌ chỉ được khi item không phải con trỏ hiện hành; version published không bao giờ bị xóa ([CONTENT_MODEL §5](../../docs/02-learning/CONTENT_MODEL.md#5-version-release-pointer-và-draftpublish-adr-0015-adr-0043) quy tắc 10).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CNT-001 | Registry loại nội dung, ánh xạ `LearningContentType` ↔ `ResourceType` | [CONTENT_MODEL §2](../../docs/02-learning/CONTENT_MODEL.md#2-các-loại-nội-dung) |
| FR-CNT-002 | Value object `ContentMeta` + kiểm metadata bắt buộc | [CONTENT_MODEL §3](../../docs/02-learning/CONTENT_MODEL.md#3-metadata-bắt-buộc-confirmed--master-prompt-42) |
| FR-CNT-005 | `owner_scope`, đọc nội dung lai (riêng tenant + được gán), chỉ đọc với tenant | [CONTENT_MODEL §4](../../docs/02-learning/CONTENT_MODEL.md#4-phạm-vi-sở-hữu-owner_scope--adr-0005-adr-0029), [MULTITENANCY §3.1](../../docs/06-architecture/MULTITENANCY.md#31-nội-dung-sở-hữu-và-gán) |
| FR-CMS-014 | Soft-delete, `<entity>.deleted/restored`, facade Thùng rác | [ADMIN_PORTAL §3.8](../../docs/04-admin-portal/ADMIN_PORTAL.md#38-thùng-rác-adr-0028), ADR-0028 |
| FR-CMS-015 | Lưu assignment, facade gán/gỡ gán, khung bước clone | [ADMIN_PORTAL §3.7](../../docs/04-admin-portal/ADMIN_PORTAL.md#37-gán-dữ-liệu-cho-tenant-và-clone-giữa-tenant-adr-0029--chỉ-system-admin-workspace-d), ADR-0029 |

### Key Entities

- **ContentItem**: định danh một nội dung (loại, `owner_scope`, tenant sở hữu, con trỏ version hiện hành, số version mới nhất, nguồn gốc clone).
- **ContentVersion**: một bản nội dung bất biến sau publish, mang `ContentMeta` và trạng thái workflow.
- **ContentRelease**: lịch sử đổi con trỏ (append-only); bảng tạo ở F020, ghi bởi F021/F026.
- **ContentTransition**: lịch sử chuyển trạng thái workflow (append-only).
- **TenantResourceAssignment**: tài nguyên platform được gán cho một tenant, có soft-delete.
- **Projection `ref.*`**: bản sao chỉ đọc của tenant, trường, lớp, học sinh, enrollment, tỉnh.
- **TrashEntry** (read model): bản ghi đã soft-delete của một `resource_type`.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `idempotency_record_ttl` | Thời hạn bản ghi `Idempotency-Key` của facade assign/restore/clone |
| `tenant_header_required` | Quy tắc header `X-Tenant-Id` cho route tenant-scoped |
| `request_body_max` | Giới hạn kích thước body request |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| 100% endpoint/consumer tenant-scoped có test tenant isolation | NFR-SEC-006 · AGENTS R8 |
| Clean Architecture + CQRS tách write/read context (ADR-0087), outbox cùng transaction | NFR-MAINT-001 · NFR-MAINT-004 · ADR-0038 · ADR-0042 · ADR-0087 |
| Chỉ soft-delete, global filter ent | ADR-0028 · ADR-0039 · AGENTS R21, R27 |
| Mọi chuỗi có giới hạn | NFR-MAINT-005 · ADR-0041 |
| Gọi chéo module chỉ qua facade sinh từ spec internal (`src/backend/pkg/modcall`) | NFR-MAINT-006 · ADR-0074 · ADR-0040 |
| Không lưu dữ liệu học sinh ngoài mức cần | NFR-PRIV-002 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 0 trường hợp người dùng tenant đọc được bản ghi của tenant khác hoặc nội dung platform chưa gán trong bộ test isolation tự động (100% endpoint/consumer của feature).
- **SC-002**: Projection bắt kịp event identity trong ≤ 5 giây p95 ở môi trường integration.
- **SC-003**: Gán nội dung platform cho tenant có hiệu lực ở lần đọc kế tiếp; gỡ gán làm tenant mất quyền đọc ngay mà 100% attempt cũ vẫn xem được.
- **SC-004**: 100% thao tác xóa/khôi phục/gán/clone có bản ghi audit tương ứng.

## Ngoài phạm vi

- Saga điều phối gán/clone, API `/admin/*`, `/tenant/trash*`, `platform.tenant_default_assignments` — F045 (module identity); UI workspace D — F057.
- Cloner và handler Thùng rác của từng loại: curriculum — F021; question — F022; question set, lesson, learning material, blueprint — F024; media — F027.
- Workflow duyệt, publish/rollback, snapshot, manifest — F025, F026.
- Endpoint rebuild projection phía module identity (chỉ consumer phía learning ở đây) — F010.

## Assumptions

- `src/backend/pkg/entx` (`Base`, `TenantAppendOnly`, `TenantOptional`, `TenantOptionalAppendOnly`, `HybridContent`, `GlobalBase`, `AppendOnly`, interceptor theo mixin, hook — ADR-0089), `src/backend/pkg/tenancy` (`TenantScope`, `CrossTenantScope`, `PlatformScope` qua `authz.PlatformScope(p)`/`authz.SystemPlatformScope(job)`, `Bypass` danh sách đóng), `src/backend/pkg/outbox`, `src/backend/pkg/modcall` có từ F002/F003.
- ADR-0089 (làm rõ 2026-11-17): `content.content_releases` và `content.content_transitions` là bảng TA\*, mixin `TenantOptionalAppendOnly`; dòng `tenant_id` NULL (nội dung platform, release platform) chỉ ghi bằng `learning_platform` dưới `PlatformScope`.
- Payload schema của event identity (`tenant.*`, `school.synced`, `student.*`, `enrollment.changed`) do F010/F011 bổ sung; F020 tiêu thụ theo schema đó [Confirmed].
- Thư mục module `src/backend/modules/learning/` theo ADR-0074.
- Tenant Admin dùng Thùng rác khi tenant được cấp `tenant.trash.*` [Confirmed] — identity kiểm quyền (F045).

## Câu hỏi mở (đã chốt 2026-11-20 — xem Clarifications, Session owner chốt)

- **F020-1** (owner identity, F011): `school.synced` và `class.synced` chưa mang `source_version` nên projection dùng `occurred_at` (micro giây) — lớn hơn mọi `source_version` của `school.updated`; sau một `school.synced`, `school.updated` cũ hơn về số version sẽ bị bỏ. Đề xuất F011 phát `source_version` (= version của aggregate) trong `school.synced`/`class.synced` và thêm `academic_year_id`, `is_virtual` vào `class.synced`.
- **F020-2** (owner F001/archgen): `make arch-check` chỉ kiểm import theo package; đề xuất kiểm theo operation (lời gọi `modcall.Op{Operation: …}` khớp `x-allowed-callers`) để F020-AC-19 fail ngay ở CI khi `game` gọi operation platform-ops (hiện bị chặn lúc chạy bằng `CALLER_NOT_ALLOWED`).
- **F020-3** (composition root): facade (`presentation/facade`) và consumer `learning.projection_identity` chưa được nối vào `cmd/core-api`/`cmd/worker` (module identity cũng chưa nối facade/consumer); cần feature composition root đăng ký facade cho `modcall` và consumer cho profile `events`.
  - 2026-11-20 [Confirmed]: đã nối theo OQ F010-2 (specs/010-tenant-org-structure/spec.md, phiên "nối binary"): facade learning nối vào identity qua `identity.Imports`, consumer `learning.projection_identity`, `learning.qb_pool_invalidation`, `learning.media_worker` chạy ở worker `events`, job `import-jobs`/`translation-jobs` ở worker `jobs`; registry câu hỏi của binary là `question.Full()` (F023 Done, OQ F023-2).
- **F020-4** (pkg/entx): `UpdateOne`/`UpdateOneID` của ent tự mở transaction và panic với `entx.Driver` (driver gắn transaction của `context`); repository dùng `Update().Where(ID(...))`. Đề xuất `entx` hỗ trợ hoặc lint chặn `UpdateOne*`.
