# ADR-0089 — Chốt tầng dữ liệu: RLS chỉ lọc tenant, bộ mixin ent, outbox và partition hạ tầng

> **Trạng thái:** Accepted · **Ngày:** 2026-11-17 · **OQ:** — · **Bổ sung:** ADR-0005, ADR-0028, ADR-0039, ADR-0042, ADR-0070 · **Tài liệu:** [DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md) · [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [BACKEND_DESIGN](../07-backend/BACKEND_DESIGN.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Review kiến trúc ngày 2026-11-17 tìm thấy các điểm trong tầng dữ liệu chưa chạy được hoặc chưa được định nghĩa, cần chốt trước khi viết F002/F003/F013/F020:

1. Policy RLS `FOR SELECT` có `deleted_at IS NULL` (DATABASE_DESIGN §3, ví dụ `assessment.attempts`). Với PostgreSQL, `UPDATE` có `WHERE` hoặc `RETURNING` buộc dòng mới phải thỏa policy SELECT. Soft-delete (`SET deleted_at = now()`) vì vậy bị từ chối với lỗi "new row violates row-level security policy".
2. `pkg/entx` chỉ có mixin `Base` (tenant bắt buộc), `PlatformBase`, `AppendOnly`. DATABASE_DESIGN dùng các loại bảng **T**, **TA**, **T\*** (tenant nullable), **H** (nội dung lai), **G** (global), nhưng chưa nói interceptor xử lý từng loại thế nào.
3. MULTITENANCY ghi `infra.outbox_events` chỉ role `<module>_platform` được ghi, trong khi `UnitOfWork` ghi outbox cùng transaction bằng role `<module>_app`.
4. Outbox và `processed_events` được dọn bằng gỡ partition, trái ADR-0070 (Release 1.0 không gỡ partition).
5. Đường đăng nhập phải đọc membership của user trước khi biết tenant, nhưng chưa có cơ chế bypass được đặt tên.
6. Interceptor trả `ErrMissingTenantScope` khi context không có scope, nhưng job platform, consumer event và bảng global chưa có quy tắc.

## Quyết định

### 1. RLS chỉ lọc tenant, không lọc `deleted_at`

- Policy RLS của bảng **T/TA** chỉ có điều kiện tenant: `USING (tenant_id = infra.current_tenant_id())`, cho `SELECT`, `INSERT` (`WITH CHECK`) và `UPDATE` (`USING` + `WITH CHECK`). Không có policy `DELETE`.
- Lọc `deleted_at IS NULL` do ent interceptor làm (R27); SQL tường minh ở read side tự thêm điều kiện và có test (R27).
- Lớp phòng thủ thứ hai cho bản ghi đã xóa: view `<schema>.<table>_live` (`WHERE deleted_at IS NULL`) cho `<module>_readonly` và BI; role `<module>_readonly` không có quyền trên bảng gốc.
- Policy của bảng **H** (nội dung lai) giữ điều kiện tenant + gán qua `infra.tenant_resource_assignments`, bỏ `deleted_at`.

### 2. Bộ mixin ent và quy tắc interceptor

| Loại bảng | Mixin `pkg/entx` | Điều kiện interceptor thêm vào | Không có scope trong context |
|---|---|---|---|
| **T** tenant-scoped | `Base` | `tenant_id IN (scope) AND deleted_at IS NULL` | `ErrMissingTenantScope` |
| **TA** append-only theo tenant | `TenantAppendOnly` | `tenant_id IN (scope)` | `ErrMissingTenantScope` |
| **T\*** tenant nullable (`user_roles`, `account_locks`) | `TenantOptional` | Phiên tenant: `tenant_id IN (scope) AND deleted_at IS NULL`; dòng `tenant_id IS NULL` chỉ đọc được qua `PlatformScope` | `ErrMissingTenantScope` |
| **H** nội dung lai (`tenant_id NULL` = platform) | `HybridContent` | `deleted_at IS NULL AND (tenant_id IN (scope) OR (tenant_id IS NULL AND được gán cho tenant trong scope))`; phiên platform (`content_admin`…): `tenant_id IS NULL` | `ErrMissingTenantScope` |
| **G** global có soft-delete | `GlobalBase` | `deleted_at IS NULL` | Không cần scope |
| **GA** global append-only | `AppendOnly` | Không | Không cần scope |
| **TA\*** append-only, tenant nullable (`identity.login_events`, `content.content_releases`, `content.content_transitions`) | `TenantOptionalAppendOnly` | Phiên tenant: `tenant_id IN (scope)`; dòng `tenant_id IS NULL` chỉ đọc qua `PlatformScope` | `ErrMissingTenantScope` |

- `PlatformBase` đổi tên thành `HybridContent`. Hiển thị nội dung platform cho tenant chỉ qua `infra.tenant_resource_assignments` (ADR-0029), không có cờ "public cho mọi tenant".
- Mỗi entity khai báo đúng một mixin trong danh sách trên. CI có test liệt kê mọi schema ent và từ chối schema không có mixin.
- Dòng có `tenant_id IS NULL` của bảng **T\*/TA\*/H** chỉ được ghi bằng role `<module>_platform` dưới `PlatformScope` (vd. đăng nhập thất bại trước khi biết tenant, role platform, nội dung platform). Policy RLS áp cho `<module>_app` giữ điều kiện tenant thuần.

### 3. Ba loại scope và một cổng bypass

- `TenantScope` (`authz.ScopeFor(p)`): phiên người dùng tenant, consumer event (lấy `tenant_id` từ envelope), lệnh realtime.
- `CrossTenantScope` (`authz.CrossTenantScope(p, ids, reason)`): System Admin, danh sách `tenant_ids` tường minh, có audit (ADR-0027).
- `PlatformScope`: `authz.PlatformScope(p)` cho nhân sự platform và `authz.SystemPlatformScope(component)` cho job platform và thành phần hệ thống chưa có Principal (vd. ghi `login_events` của lần đăng nhập thất bại trước khi biết tenant); chỉ thao tác dòng `tenant_id IS NULL` (nội dung platform, role platform). Không đọc được dòng có `tenant_id` khác NULL.
- Bỏ filter chỉ qua `tenancy.Bypass(ctx, reason)`. `reason` thuộc danh sách đóng, khai báo là hằng số kiểu `tenancy.BypassReason`:

  | Reason | Dùng cho | Quyền DB |
  |---|---|---|
  | `system_admin_cross_tenant` | System Admin đọc/ghi liên tenant | `<module>_platform` |
  | `moderation_case` | `safety_moderator` trong module moderation (ADR-0072) | `chat_platform` |
  | `break_glass` | Truy cập khẩn cấp nội dung chat đã được hai System Admin duyệt (F124) | `chat_platform` |
  | `trash` | Thùng rác (ADR-0028) | `<module>_platform` |
  | `auth_login` | Đọc membership trên đường đăng nhập | Hàm `SECURITY DEFINER` (dưới đây) |
  | `tenant_clone` | Saga clone/gán (ADR-0029) | `<module>_platform` |
  | `inactive_cleanup` | Dọn học sinh không đăng nhập (ADR-0032) | `<module>_platform` |
  | `re_evaluation` | Chấm lại attempt sau khi sửa nội dung platform (F031) | `learning_platform` |
  | `efficacy_study` | Nghiên cứu hiệu quả học tập liên tenant (F035, ADR-0080) | `learning_platform` |
  | `platform_job` | Job platform khác: rollup liên tenant, đồng bộ SIS theo lịch | `<module>_platform` |
| `platform_analytics` | Dashboard analytics/live-ops của workspace platform: chỉ đọc số liệu gộp liên tenant đã áp k-anonymity, không đọc dòng của từng học sinh (F080) | `<module>_platform`, chỉ trên bảng rollup |

  Thêm reason mới cần sửa ADR này. Linter cấm literal ngoài danh sách. Mỗi lần gọi ghi audit action `cross_tenant.accessed` (trường `reason`, `tenant_ids`, `mode` = `read`/`write`) và tăng metric `eduworld_tenant_filter_bypass_total{module,reason}` ([MONITORING](../11-operations/MONITORING.md)). **Ngoại lệ `platform_job` (làm rõ 2026-11-20):** chỉ ghi audit khi lần chạy job có đọc/ghi ít nhất một dòng nghiệp vụ (job báo số dòng trước khi kết thúc; audit ghi một lần mỗi lần chạy kèm số dòng); metric vẫn tăng mỗi lần chạy. Reason khác giữ audit mỗi lần dùng.
- **Đường đăng nhập:** module `identity` gọi hàm `identity.memberships_for_user(user_id)` (`SECURITY DEFINER`, `SET search_path` cố định, chỉ trả các cột membership của đúng user đó; `EXECUTE` cấp cho `identity_app`). Owner của hàm là role `identity_membership_reader`: `NOLOGIN`, `BYPASSRLS` (vì bảng nguồn dùng `FORCE ROW LEVEL SECURITY`), chỉ có `SELECT` trên các bảng nguồn membership và không sở hữu đối tượng nào khác bên trong `tenancy.Bypass(ctx, "auth_login")`. Chỉ route `/auth/session`, `/auth/game/*` và `login-credentials:verify` được gọi; test kiến trúc chặn nơi khác. Role ứng dụng của đường đăng nhập không có `BYPASSRLS`.
- API công khai của `pkg/tenancy` và `pkg/authz` (`TenantScope`, `CrossTenantScope`, `PlatformScope`, `SystemPlatformScope`, `Bypass`, `BypassReason`, `Principal`) được khóa từ F002; đổi chữ ký cần ADR.

### 4. Outbox, processed events, idempotency, dead letters

- `infra.outbox_events`, `infra.processed_events`, `infra.idempotency_records`, `infra.dead_letters` là bảng hạ tầng, **không bật RLS**; `tenant_id` nằm trong envelope để consumer dựng `TenantScope`.
- Quyền:
  - `<module>_app`: `INSERT` outbox (cùng transaction trong `UnitOfWork`); `INSERT`, `SELECT` processed events và idempotency; `INSERT` dead letters.
  - `<module>_relay`: `SELECT` outbox và `UPDATE` các cột trạng thái publish (`published_at`, `attempts`).
  - `<module>_platform`: `SELECT` và `UPDATE` cột xử lý của dead letters (màn xử lý event lỗi); và các quyền ghi như `<module>_app` (`INSERT` outbox, `INSERT`/`SELECT` processed events và idempotency, `INSERT` dead letters) vì `UnitOfWork` chạy transaction `PlatformScope`/`CrossTenantScope`/consumer platform bằng role này.
  - Không role ứng dụng nào có `DELETE`/`TRUNCATE` trên `infra.*`.
- **Khóa partition ổn định để chống trùng xuyên ngày:**
  - `outbox_events`: partition theo `created_at`, PK `(event_id, created_at)`; `event_id` là UUIDv7 do server sinh.
  - `processed_events`: partition theo `event_occurred_at` lấy từ envelope (bất biến với mỗi event), PK `(consumer, event_id, event_occurred_at)`. Event gửi lại luôn rơi vào cùng partition, nên PK chặn trùng ở mọi thời điểm.
  - `idempotency_records`: `Idempotency-Key` bắt buộc là UUIDv7; cột partition `key_issued_on` tính từ timestamp trong key; PK `(user_id, route_template, idem_key, key_issued_on)`. Key không phải UUIDv7 hoặc lệch quá `idempotency_record_ttl` so với giờ server → `400 BAD_REQUEST` (mã có sẵn cho header sai định dạng, [API_DESIGN](../07-backend/API_DESIGN.md)).
- **Mẫu ghi idempotency:** bản ghi được `INSERT` một lần, trong cùng transaction `UnitOfWork` với thay đổi nghiệp vụ, kèm response đã tuần tự hóa. Request trùng chạy đồng thời bị chặn ở unique index cho tới khi transaction đầu commit, rồi đọc và trả lại response đã lưu; chờ quá `lock_timeout` → `409 IDEMPOTENCY_IN_PROGRESS`. Không có trạng thái `in_progress`. Chỉ response thành công được lưu: lỗi 4xx/5xx làm transaction rollback nên không để lại bản ghi, request gửi lại được chạy lại (lỗi 4xx là kết quả xác định nên chạy lại an toàn).

### 5. Retention của bảng hạ tầng: ngoại lệ có giới hạn của ADR-0070

- Bốn bảng ở mục 4 được partition theo ngày và được **gỡ partition** sau thời hạn retention lấy từ [CONFIG_DEFAULTS](../00-overview/CONFIG_DEFAULTS.md): `outbox_retention` (outbox đã publish, processed events), `idempotency_record_ttl` (idempotency), retention dead letter của [EVENT_DESIGN](../07-backend/EVENT_DESIGN.md). Payload event không chứa PII (R14).
- Job `infra-partition-maintenance` (profile `worker-jobs`, ADR-0074) chạy với role `<module>_partition_admin` (chỉ `CREATE`/`DETACH`/`DROP` partition trên schema `infra`). Job từ chối gỡ partition còn dòng outbox chưa publish hoặc dead letter chưa xử lý, và phát cảnh báo.
- Ngoại lệ chỉ áp cho bốn bảng trên và chat gameplay theo ADR-0033. Mọi bảng nghiệp vụ partition (`learning_events`, `attempt_answers`, `audit_logs`, ledger) vẫn theo ADR-0070: không gỡ partition trong Release 1.0.

### 6. Migration viết tay

- Migration quản lý bằng Atlas (thư mục `modules/<module>/migrations/`, `atlas.sum`, `atlas migrate lint`); không auto-migrate khi khởi động. ent sinh phần bảng/cột/index. Phần ent không biểu diễn được viết tay trong cùng thư mục và review như migration sinh tự động: RLS policy, trigger, `GRANT`/`REVOKE`, view `_live`, hàm `SECURITY DEFINER`, DDL partition (thư mục con `partitions/`).
- Partition của kỳ tiếp theo do job `infra-partition-maintenance` tạo trước tối thiểu 7 ngày; thiếu partition là cảnh báo P2.

## Hệ quả

(+) Soft-delete chạy được với RLS; mỗi loại bảng có mixin và quy tắc interceptor rõ ràng; outbox ghi được trong transaction của `_app`; đường đăng nhập có bypass được đặt tên và audit. (−) Mất lớp RLS cho `deleted_at`, nên phải dựa vào interceptor, test và view `_live`. Thêm role `<module>_relay`, `<module>_partition_admin`, `identity_membership_reader` và hàm `identity.memberships_for_user`. Client phải sinh `Idempotency-Key` dạng UUIDv7. Phải sửa DATABASE_DESIGN, MULTITENANCY, BACKEND_DESIGN, AGENTS R27 và các feature F002, F003, F013, F020.

## Phương án đã loại

- **Giữ `deleted_at` trong policy SELECT và soft-delete bằng hàm `SECURITY DEFINER`:** mọi lệnh xóa phải đi qua hàm riêng, ent hook không dùng được, và hàm chạy quyền cao.
- **Dọn outbox bằng `DELETE` theo lô:** trái ADR-0028 và tạo bloat; gỡ partition rẻ và không cần quyền `DELETE`.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-17 | Tạo ADR (Accepted) từ review kiến trúc | Technical Architect |
| 2026-11-17 | Làm rõ sau khi áp vào tài liệu: mixin `TenantOptionalAppendOnly`; dòng tenant NULL ghi bằng `_platform`; danh sách reason đầy đủ kèm quyền DB; audit `cross_tenant.accessed`; metric `eduworld_tenant_filter_bypass_total`; hàm `identity.memberships_for_user`; khóa partition ổn định cho processed/idempotency; mẫu ghi idempotency; retention dead letter; phạm vi migration viết tay; tên `<module>` theo ADR-0074 | Technical Architect |
| 2026-11-17 | Chốt điểm còn treo: reason `platform_analytics`; `SystemPlatformScope` thay `JobPlatformScope` (gồm thành phần hệ thống chưa có Principal); owner hàm membership là role `identity_membership_reader` có `BYPASSRLS`; key sai → `400 BAD_REQUEST`; chỉ lưu response thành công | Technical Architect |
| 2026-11-19 | Làm rõ (F010, phát hiện khi chạy trên PostgreSQL): `<module>_platform` cũng ghi outbox, processed events, idempotency và dead letters như `<module>_app` — System Admin ghi (`PlatformScope`/`CrossTenantScope`) và consumer platform phải phát event cùng transaction. Migration mới `<version>_infra_platform_grants` (tools/infragen), không sửa migration infra đã áp | Technical Architect (F010) |
| 2026-11-20 | Làm rõ §3 (OQ F010-3, smoke cho thấy job ghi `cross_tenant.accessed` mỗi chu kỳ kể cả khi không xử lý dòng nào): với reason `platform_job`, `tenancy.Bypass` chỉ ghi audit khi lần chạy đọc/ghi ≥ 1 dòng nghiệp vụ, một bản ghi mỗi lần chạy kèm số dòng; metric `eduworld_tenant_filter_bypass_total` vẫn đếm mọi lần chạy; reason khác không đổi. Task hiện thực ở F003 | Owner ([F010](../../specs/010-tenant-org-structure/spec.md), [F003](../../specs/003-platform-kit-data/spec.md)) |
