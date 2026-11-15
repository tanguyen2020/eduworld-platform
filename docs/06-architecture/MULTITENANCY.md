# Multi-tenancy

> Trạng thái: Draft · Phiên bản: 0.4 · Owner: Solution Architect · Cập nhật: 2026-11-15
> Liên quan: [ADR-0005](../adr/ADR-0005-mo-hinh-multi-tenancy-shared-database-tenant-id-rls.md) · [ADR-0026](../adr/ADR-0026-dang-nhap-theo-tenant-xac-dinh-tenant-kiem-tra-cheo-mac-dinh.md) · [ADR-0027](../adr/ADR-0027-cms-tach-vung-tenant-chi-thao-tac-learning-management-cau-hi.md) · [ADR-0028](../adr/ADR-0028-chi-soft-delete-khong-hard-delete-mac-dinh-khong-doc-du-lieu.md) · [ADR-0029](../adr/ADR-0029-system-admin-gan-du-lieu-cho-tenant-va-clone-du-lieu-giua-te.md) · [DOMAIN_MODEL](DOMAIN_MODEL.md) · [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) · [DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md)

**Mục đích & phạm vi.** Mô hình tenant (trường / đơn vị / tenant chung), cách cô lập dữ liệu ở tầng ứng dụng và RLS, tenant chung, soft-delete, gán/clone giữa tenant. SQL RLS chuẩn ở DATABASE_DESIGN §4; thuật toán xác định tenant khi đăng nhập ở IDENTITY_MAPPING §3.

> **Thay đổi v0.4:** áp dụng cho 6 microservice và 5 database (ADR-0036/0037): mỗi database tự áp RLS `app.tenant_id` và có `infra.current_tenant_id()`; lớp ứng dụng dùng **ent interceptor** (global filter tenant + soft-delete, ADR-0039) thay cho quy ước sqlc; tenant/membership là nguồn sự thật ở identity-service, service khác dùng projection `ref.tenants`/`ref.students` và `GET /internal/v1/principals` ([DATABASE_DESIGN §2.17](../07-backend/DATABASE_DESIGN.md)).

## 1. Mục tiêu

| Mã | Yêu cầu | Nhãn |
|---|---|---|
| NFR-SEC-010 | Người dùng tenant A không đọc/ghi được dữ liệu tenant B qua bất kỳ API/WS nào | [Confirmed] |
| NFR-SEC-011 | Giáo viên chỉ truy cập lớp/môn được phân công (`TeachingAssignment`) trong tenant của phiên | [Confirmed] |
| NFR-SEC-012 | Tenant Admin chỉ quản trị trong tenant của mình; School Admin chỉ trong trường được gán bên trong tenant | [Confirmed] |
| NFR-SEC-013 | Chỉ System Admin xem/thao tác liên tenant; mọi truy cập liên tenant có điều kiện `tenant_id` tường minh và được audit | [Confirmed] ADR-0027 |
| NFR-SEC-014 | Không người dùng nào truy cập toàn bộ chat của tất cả tenant | [Confirmed] |
| NFR-SEC-015 | Hai lớp bảo vệ độc lập trong **mỗi** service: ứng dụng (ent interceptor, bắt buộc) + RLS của database service (defense-in-depth) | [Confirmed] ADR-0005, ADR-0039 |
| NFR-SEC-019 | Lời gọi service-to-service và event giữ đúng tenant: `X-Tenant-Id`/envelope `tenant_id` bị bên nhận đối chiếu với dữ liệu của mình; không service nào đọc database của service khác | [Confirmed] ADR-0037, ADR-0040 |
| NFR-SEC-016 | Khi đăng nhập phải xác định đúng tenant của người dùng; không ai mở được phiên trong ngữ cảnh tenant mình không thuộc về | [Confirmed] ADR-0026 |
| NFR-SEC-017 | Mọi truy vấn lọc theo `tenant_id` **và** `deleted_at IS NULL`; ứng dụng không bao giờ hard-delete | [Confirmed] ADR-0005, ADR-0028 |
| NFR-SEC-018 | Tài khoản chỉ đến từ tenant SIS (provisioning) hoặc tenant chung `common` do platform tạo có chủ đích; không thuộc tenant chung và không thuộc tenant SIS nào → không cho đăng nhập (`ACCOUNT_NOT_PROVISIONED`), **không** tự đưa vào tenant chung | [Confirmed] ADR-0063 |

## 2. Mô hình tenant

```mermaid
flowchart TD
  P[Platform — dữ liệu dùng chung tenant_id = NULL] --> T1[Tenant type=school\n'TH Nghĩa Đô A']
  P --> T2[Tenant type=unit\n'Phòng GD&ĐT Cầu Giấy']
  P --> T3[Tenant type=common\n'EduWorld chung']
  T1 --> S1[School: TH Nghĩa Đô A]
  T2 --> S2[School: TH Dịch Vọng]
  T2 --> S3[School: THCS Yên Hòa]
  T3 --> S4[School ảo: EduWorld chung]
  S2 --> AY[AcademicYear 2026–2027]
  AY --> C1[Class 3A]
  C1 --> E1[Enrollment → Student]
```

| Cấp | Vai trò trong cô lập | Ví dụ dữ liệu |
|---|---|---|
| Platform | Không phải tenant. Dữ liệu `tenant_id = NULL`, `owner_scope = platform`; tenant chỉ thấy khi được gán (ADR-0029) | Curriculum chuẩn, question bank platform, cấu hình game, item definitions, ChatPolicy gốc |
| **Tenant** | **Đơn vị cô lập dữ liệu (`tenant_id`) và đơn vị đăng nhập** (một Keycloak Organization: `keycloak_org_id`, `keycloak_org_alias`, mã nhập `login_code`) | Mọi dữ liệu học sinh, lớp, attempt, learning event, chat, party, moderation, nội dung riêng của tenant |
| School | Chiều phân loại **bên trong** tenant (`school_id`): tenant `school` có 1 trường, tenant `unit` có nhiều trường, tenant `common` có 1 trường ảo | Lớp, enrollment, báo cáo theo trường, phạm vi của School Admin |
| Class | Phạm vi quyền bên trong trường | Assignment, channel lớp, dashboard giáo viên |

### 2.1. Loại tenant

| `tenant_type` | Số School | Ví dụ | Ghi chú |
|---|---|---|---|
| `school` | Đúng 1 | Trường độc lập ký hợp đồng trực tiếp | Tenant Admin ≈ quản trị trường |
| `unit` | ≥ 1 | Sở/Phòng GD&ĐT, hệ thống trường | Dữ liệu mọi trường trong đơn vị cùng `tenant_id`; học sinh chuyển trường **trong** đơn vị không đổi tenant |
| `common` | 1 (School ảo "EduWorld chung") | Học sinh do platform (System Admin) tạo/import có chủ đích (ADR-0063) | Duy nhất (`code = 'common'`); không có SIS; nội dung do System Admin gán [Proposed] |

## 3. Phân loại bảng

| Loại | Cột bắt buộc | RLS | Ví dụ |
|---|---|---|---|
| **Tenant-scoped** | `tenant_id uuid NOT NULL`, `deleted_at`, `deleted_by` (+ `school_id` nếu dữ liệu gắn trường) | Bật | `org.classes`, `org.enrollments`, `assessment.attempts`, `learning.objective_masteries`, `game.quest_progress`, `multiplayer.parties`, `chat.messages`, `lms.assignments` |
| **Tenant-scoped append-only** | `tenant_id NOT NULL` (không có `deleted_at`, không xóa) | Bật | `learning.learning_events`, `assessment.attempt_answers`, `gamification.xp_ledger`, `audit.audit_logs` (đọc qua API có scope) |
| **Nội dung lai** | `tenant_id uuid NULL` (NULL = platform), `owner_scope ∈ {platform, tenant}`, `deleted_at` | Bật (policy có nhánh assignment) | `content.questions`, `content.question_sets`, `content.lessons`, `lms.challenge_templates` |
| **Global/platform** | `deleted_at` | Tắt; chỉ ghi qua `/api/v1/game-admin/*`, `/api/v1/admin/*` | `curriculum.subjects`, `game.item_definitions`, `game.quest_definitions`, `platform.feature_flags`, **danh mục `catalog.countries`, `catalog.provinces`, `catalog.province_successions`** (ADR-0060: dùng chung mọi tenant, chỉ System Admin sửa, mọi người dùng đã đăng nhập được đọc) |
| **Cross-tenant có kiểm soát** | `tenant_ids uuid[]` hoặc bảng participants có `tenant_id` | Bật với policy đặc biệt | `competition.matches` (liên tenant), `competition.tournaments` do platform tạo |
| **Platform internal** | — | Chỉ role `<svc>_platform` | `infra.outbox_events` (mọi DB), `platform.tenant_clone_jobs` (DB identity) |

**Trường (School) là dữ liệu tenant-scoped:** `org.schools` có `tenant_id NOT NULL` — mỗi trường thuộc đúng một tenant và tham chiếu một tỉnh/thành trong danh mục platform. System Admin xem danh mục trường liên tenant với bộ lọc tenant/tỉnh/cấp học (ADR-0027 mục 6); Tenant Admin chỉ thấy trường của tenant mình. Học sinh có đúng một enrollment active (một trường, một lớp) trong tenant của mình (ADR-0060).

### 3.1. Nội dung: sở hữu và gán

| Nguồn | `tenant_id` | Ai thấy (khi `published`) | Ai sửa |
|---|---|---|---|
| Dữ liệu platform | `NULL` | Tenant có **assignment active** trong `infra.tenant_resource_assignments` của database sở hữu tài nguyên (ADR-0029); System Admin | System Admin / Content Admin platform |
| Dữ liệu riêng của tenant | `= tenant` | Chỉ tenant đó (+ System Admin) | Người có quyền biên tập trong tenant |
| Bản clone | `= tenant đích` | Như dữ liệu riêng của tenant đích | Như trên; giữ `cloned_from_*` để truy vết |

Draft/in_review chỉ người có quyền biên tập/duyệt trong phạm vi thấy. Attempt luôn mang `tenant_id` của **học sinh**, không phải của nội dung.

## 4. Lớp 1 — Ứng dụng (bắt buộc)

### 4.1. Principal và TenantScope

```go
// pkg/authz — Principal dựng từ identity-service (/internal/v1/principals), cache 60 s
type Principal struct {
    UserID      uuid.UUID
    UserType    UserType         // student, teacher, staff, admin
    TenantID    *uuid.UUID       // tenant hoạt động của phiên (ADR-0026); nil với System Admin
    TenantType  TenantType       // school | unit | common
    SchoolIDs   []uuid.UUID      // các trường trong tenant mà user có phạm vi (School Admin, giáo viên); học sinh = [school_id]
    Permissions PermissionSet    // hợp permission của các UserRole hiệu lực trong tenant (ADR-0066)
    IsSystemAdmin bool           // chế độ liên tenant: role system_admin + token eduworld-portal có MFA
    GradeLevel  *int
    SessionID   string
}

// TenantScope bắt buộc truyền vào mọi repository tenant-scoped.
type TenantScope struct {
    tenantIDs []uuid.UUID // đúng 1 với người dùng tenant; 1..n (đã chọn ở dropdown) với System Admin
    crossTenant bool      // chỉ authz.CrossTenantScope(p, tenantIDs, reason) tạo được
    reason      string    // bắt buộc khi crossTenant → ghi audit
}
```

Quy tắc:

1. `TenantScope` **chỉ** được tạo bởi package `authz`: `authz.ScopeFor(p)` cho người dùng tenant (luôn `[p.TenantID]`), `authz.CrossTenantScope(p, ids, reason)` cho System Admin. Linter cấm literal `TenantScope{}` ngoài `authz`.
2. `TenantScope` được đặt vào `context` bởi behavior Authorization của mediator; **ent interceptor** (`pkg/tenancy`) tự thêm `tenant_id IN (scope) AND deleted_at IS NULL` cho mọi query trên entity có mixin `Base`, và trả lỗi `ErrMissingTenantScope` nếu context không có scope (bảng append-only dùng mixin `AppendOnly`, không có điều kiện `deleted_at`). SQL tường minh cho read model phải nhận `scope` làm tham số và được job `tenancy-guard` kiểm tra (§11).
3. Command/query handler (qua behavior Authorization) gọi `authz.Require(ctx, p, permission, resource)` **trước** khi gọi repository; resource chứa `tenant_id`/`school_id`/`class_id` nạp từ DB, không lấy từ request body.
4. ID trong URL không đủ để truy cập: `GET /attempts/{id}` → load với scope → không thấy (khác tenant hoặc đã soft-delete) = `404`.
5. **Header `X-Tenant-Id` bắt buộc (ADR-0065):** mọi request REST tenant-scoped của game và portal mang `X-Tenant-Id` (giá trị lấy từ PlayerBootstrap/`GET /me/tenants` sau đăng nhập). APISIX từ chối thiếu/sai định dạng (`400 TENANT_HEADER_REQUIRED`); middleware `tenant` so header với tenant của phiên (claim `organization` + membership) — lệch → `403 TENANT_MISMATCH` + audit. `Principal.TenantID` = kết quả kiểm tra, **không** lấy thẳng từ header. Người dùng tenant gửi `tenant_ids` → bị bỏ qua. Miễn header: `POST /auth/session`, `GET /game/client-config`, health, `GET /me/tenants`. WebSocket: tenant cố định khi cấp realtime ticket.
6. System Admin: thao tác ghi gửi `X-Tenant-Id` = tenant đích; danh sách nhận `tenant_ids[]` từ dropdown (không gửi header) (rỗng = "tất cả tenant" → `CrossTenantScope` với toàn bộ tenant active, vẫn là danh sách tường minh); thao tác ghi bắt buộc **một** `tenant_id` đích hoặc phạm vi platform. Mỗi lần dùng `CrossTenantScope` (`tenancy.Bypass`) phát `audit.recorded` với `action = cross_tenant_access`.
7. **Giữa service:** service gọi internal API truyền `X-Tenant-Id` của request gốc; bên nhận tạo `TenantScope` từ header **sau khi** xác thực token service (allowlist `azp`) và kiểm tra tenant tồn tại/hoạt động trong `ref.tenants`. Consumer event tạo `TenantScope` từ `tenant_id` của envelope. Event platform (`tenant_id = null`) chỉ consumer platform xử lý.

### 4.2. Đọc nội dung lai (riêng tenant + được gán)

```sql
-- learning-service: read model query (SQL tường minh trong infrastructure/persistence, scope truyền vào)
SELECT qs.*
FROM content.question_sets qs
WHERE qs.deleted_at IS NULL
  AND qs.status = 'published'
  AND (
        qs.tenant_id = ANY(@tenant_ids::uuid[])
     OR (qs.tenant_id IS NULL AND EXISTS (
          SELECT 1 FROM infra.tenant_resource_assignments a   -- cùng database eduworld_learning
          WHERE a.resource_type = 'question_set'
            AND a.resource_id = qs.question_set_id
            AND a.tenant_id = ANY(@tenant_ids::uuid[])
            AND a.deleted_at IS NULL))
  )
ORDER BY qs.updated_at DESC
LIMIT @limit;
```

### 4.3. Middleware chuỗi

```mermaid
flowchart LR
  A[APISIX: verify JWT] --> B[Service: authn\nverify iss/aud/exp/azp]
  B --> C[Resolve Principal\nidentity-service /internal/v1/principals:resolve\ncache theo user+tenant]
  C --> T[Tenant middleware\nclaim organization ↔ membership DB\nTENANT_NOT_SELECTED / TENANT_MISMATCH / common]
  T --> D[Handler / Use case]
  D --> E[Mediator behavior Authorization\nauthz.Require permission+resource]
  E --> F[ent interceptor\ntenant_id + deleted_at IS NULL]
  F --> G[(Database của service\nSET LOCAL app.tenant_id\nRLS)]
```

Cache Principal (`ew:id:principal:<user_id>:<tenant_id>`, ở mỗi service) bị xóa khi nhận event `user.access_revoked`, `enrollment.changed`, `user_role.changed`, `role.updated`, `tenant_membership.changed`, `tenant.updated`.

## 5. Lớp 2 — Row-Level Security (defense-in-depth)

### 5.1. Role DB

Mỗi database có bộ role riêng theo service (`identity_*`, `learning_*`, `game_*`, `competition_*`, `chat_*`); bảng dưới dùng `<svc>` đại diện.

| Role | Dùng bởi | RLS | Quyền |
|---|---|---|---|
| `<svc>_migrator` | job `cmd/migrate` (Atlas) của service | Owner bảng | DDL trong database của service |
| `<svc>_app` | api + consumer thường của worker của service | **Áp dụng** (`NOBYPASSRLS`) | `SELECT, INSERT, UPDATE` — **không có `DELETE`, `TRUNCATE`** (ADR-0028) |
| `<svc>_platform` | Phiên System Admin liên tenant, job platform (SIS sync, rollup, bước saga clone/cleanup) | `BYPASSRLS` | `SELECT, INSERT, UPDATE` — **không có `DELETE`, `TRUNCATE`**; truy vấn vẫn có `tenant_id` tường minh; audit |
| `chat_purger` | Chỉ job `gameplay-chat-purge` của chat-service (ADR-0033) | Policy riêng: chỉ channel gameplay đã đóng | `DELETE` **chỉ** trên bảng chat gameplay trong `eduworld_chat` — ngoại lệ duy nhất của ADR-0028 ([DATABASE_DESIGN §1.1](../07-backend/DATABASE_DESIGN.md)) |
| `<svc>_readonly` | Reporting/BI | Áp dụng | `SELECT` trên view tổng hợp |
| `<svc>_backup` | Backup | — | Theo [BACKUP_RECOVERY](../11-operations/BACKUP_RECOVERY.md) |

Bảng do `<svc>_migrator` sở hữu và bật `FORCE ROW LEVEL SECURITY`. Không role nào có `CONNECT` vào database của service khác.

### 5.2. Set ngữ cảnh mỗi transaction

```go
// pkg/tenancy — unit of work dùng chung cho ent (mọi service)
func (u *UnitOfWork) InTenantTx(ctx context.Context, scope authz.TenantScope, fn func(ctx context.Context, tx *ent.Tx) error) error {
    return u.withTx(ctx, u.clientFor(scope), func(ctx context.Context, tx *ent.Tx) error {
        if _, err := tx.Exec(ctx,
            `SELECT set_config('app.tenant_id', $1, true)`,
            scope.SessionTenantID()); err != nil { // '' với CrossTenantScope (client <svc>_platform, BYPASSRLS)
            return fmt.Errorf("set tenant context: %w", err)
        }
        return fn(authz.WithScope(ctx, scope), tx) // interceptor đọc scope từ ctx
    })
}
```

`set_config(..., true)` tương đương `SET LOCAL`. `u.clientFor(scope)` chọn ent client của `<svc>_app` (người dùng tenant) hoặc `<svc>_platform` (CrossTenantScope). Behavior Transaction của mediator gọi `InTenantTx`; **cấm** truy vấn tenant-scoped ngoài unit of work.

### 5.3. Policy RLS

Ba loại policy, SQL chuẩn ở [DATABASE_DESIGN §4](../07-backend/DATABASE_DESIGN.md#4-row-level-security):

| Loại bảng | Điều kiện đọc | Ghi |
|---|---|---|
| T (tenant-scoped, có soft-delete) | `tenant_id = infra.current_tenant_id() AND deleted_at IS NULL` | INSERT/UPDATE chỉ với đúng tenant; **không có policy DELETE** |
| TA (append-only) | `tenant_id = infra.current_tenant_id()` | Chỉ INSERT |
| H (nội dung lai platform + gán + riêng tenant) | Dòng của tenant **hoặc** dòng platform (`tenant_id IS NULL`) có assignment active cho tenant (ADR-0029) | Theo quyền Game Studio/CMS |

Không set `app.tenant_id` → `NULL` → không trả dòng nào. Phiên System Admin dùng client `<svc>_platform` (BYPASSRLS) nên bộ lọc tenant nằm ở điều kiện `tenant_id = ANY(@tenant_ids)` tường minh trong query (§4.1). Thùng rác (ADR-0028) đọc dữ liệu đã xóa qua view/hàm riêng `SECURITY DEFINER` chỉ dùng cho System Admin, có audit.

### 5.4. Hiệu năng

- Index phức hợp bắt đầu bằng `tenant_id` trên mọi bảng tenant-scoped lớn, dạng partial `WHERE deleted_at IS NULL`: `(tenant_id, student_id, created_at) WHERE deleted_at IS NULL`.
- Index assignment `(tenant_id, resource_type, resource_id) WHERE deleted_at IS NULL`.
- `infra.current_tenant_id()` là `STABLE`.
- `EXPLAIN` trong integration test cho truy vấn nóng (attempt, learning_events, chat.messages).

## 6. Valkey và realtime

- Key chứa dữ liệu tenant có `tenant_id` trong key ([VALKEY_DESIGN](../07-backend/VALKEY_DESIGN.md)). Gateway không tự quyết định subscribe: service sở hữu (chat/competition) kiểm tra `tenant_id` của channel/room so với `Ew-Tenant-Id` của lệnh rồi mới trả `subscribe` cho gateway.
- Hub instance cấp theo `(tenant_id, school_id, grade_band)` [Proposed]; tenant chung có instance riêng. Instance/match liên tenant chỉ tồn tại cho sự kiện do platform tạo, chat theo cùng ChatPolicy (ADR-0020), chỉ hiển thị nickname.
- Pub/sub không mang dữ liệu vượt tenant: message của channel tenant A chỉ publish trên `ew:chat:fanout:<channel_id>`, chỉ node có subscriber hợp lệ nhận.
- **NATS JetStream (ADR-0035):** subject chat chứa tenant: `ew.chat.<tenant_id>.{msg|evt|cursor}.<channel_id>`. Persister và mọi consumer của chat-service đối chiếu `tenant_id` của envelope với channel trong `eduworld_chat`; ghi qua unit of work có `TenantScope` + RLS `app.tenant_id` của envelope. Domain event `EVT_*` cũng mang `tenant_id` trong envelope; consumer của mọi service áp cùng quy tắc. Envelope sai tenant → DLQ + alert bảo mật.

## 7. Các tình huống đặc biệt

| Tình huống | Xử lý | Nhãn |
|---|---|---|
| **Chuyển lớp** trong cùng trường | Enrollment cũ `status=ended/transferred`, tạo enrollment mới; dữ liệu học tập giữ nguyên | [Proposed] |
| **Chuyển trường trong cùng tenant `unit`** | Đổi `school_id`, enrollment mới; **không** đổi tenant, không đổi membership Keycloak; lịch sử vẫn trong tenant | [Confirmed] |
| **Chuyển sang trường thuộc tenant khác** | User giữ `user_id`; Student đổi `tenant_id`/`school_id` hiện tại; bản ghi lịch sử **giữ `tenant_id` cũ**. Tenant mới không tự thấy lịch sử cũ. Mastery & Game Profile đi theo học sinh. Chia sẻ hồ sơ học tập cần quyết định pháp lý (OQ-12) | [Proposed] |
| **Học sinh không thuộc tenant SIS nào** | Chỉ vào tenant `common` nếu platform đã tạo tài khoản ở đó; nếu không → `ACCOUNT_NOT_PROVISIONED`, không cho đăng nhập; xem [§8.3](#83-tenant-chung) | [Confirmed] ADR-0063 |
| **Thi đấu liên tenant** | `competition.matches.tenant_ids[]`; mỗi `match_participants` có `tenant_id`; đối thủ chỉ thấy nickname/avatar/tier. Attempt/learning event ghi theo `tenant_id` của từng học sinh | [Proposed] |
| **Giải đấu trong tenant `unit`** | Tạo từ template platform trong tenant; đăng ký chỉ học sinh của tenant | [Proposed] |
| **Nội dung platform dùng trong tenant** | Chỉ đọc khi được gán; muốn sửa → System Admin clone sang tenant (ADR-0029) | [Confirmed] |
| **Báo cáo tổng hợp tenant `unit`** | Tenant Admin xem báo cáo theo trường trong tenant; báo cáo vượt tenant chỉ System Admin, dạng aggregate k-anonymity ≥ 5 [Proposed] | [Proposed] |
| **Moderation** | Moderator theo tenant (có thể giới hạn theo trường); tin nhắn liên tenant bị report → case thuộc tenant của người bị report; escalation lên platform trust & safety | [Proposed] |
| **System Admin hỗ trợ** | `CrossTenantScope(reason)`, dropdown chọn tenant, audit | [Confirmed] |

## 8. Xác định tenant khi đăng nhập và ở mỗi request (ADR-0026)

### 8.1. Nguyên tắc

| # | Nguyên tắc | Nhãn |
|---|---|---|
| 1 | Mỗi **Tenant** là một Keycloak Organization; `platform.tenants.keycloak_org_id` + `keycloak_org_alias` (duy nhất) là khóa nối. Không tạo Organization cho từng School trong tenant `unit` | [Confirmed] |
| 2 | **Học sinh đăng nhập username + password, không chọn trường** (ADR-0063): lần đăng nhập đầu không kèm tenant; client chỉ gửi scope `organization:{tenant_alias}` sau bước chọn tenant khi thuộc nhiều tenant. Không có ô mã trường, QR, gợi ý tenant trước đăng nhập hay nhận diện theo tên miền email trong Release 1.0 ([KEYCLOAK_SSO §8](../09-identity/KEYCLOAK_SSO.md#8-trải-nghiệm-đăng-nhập-cho-học-sinh)) | [Confirmed] |
| 3 | Backend **luôn** xác định tenant từ membership trong DB: claim là điều kiện cần khi có, DB là điều kiện đủ | [Confirmed] |
| 4 | Sau đăng nhập (ADR-0063): đúng 1 tenant (tenant SIS hoặc tenant chung) → vào thẳng; nhiều tenant → `TENANT_NOT_SELECTED` kèm danh sách, chọn **sau** khi đăng nhập; không thuộc tenant chung và không thuộc tenant SIS nào → `403 ACCOUNT_NOT_PROVISIONED`, audit, thu hồi phiên Keycloak | [Confirmed] |
| 5 | Membership Keycloak Organization do worker đồng bộ từ SIS/UserRole ([ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md)) | [Confirmed] |
| 6 | Một tenant hoạt động cho mỗi phiên. Username, email, mã học sinh **không** dùng để suy ra tenant | [Confirmed] |
| 7 | **Portal cũng không chọn tenant trước đăng nhập** (ADR-0067): role platform → phạm vi platform; nhân sự 1 tenant → vào thẳng; nhiều tenant → chọn sau, đổi trên topbar; không có role portal nào → `403 PORTAL_ACCESS_STAFF_ONLY`; học sinh → chặn (ADR-0066). Không có URL `/t/{tenant_alias}` | [Confirmed] |
| 8 | **Username và email duy nhất toàn hệ thống** (không phân biệt hoa thường) để đăng nhập không chọn tenant luôn xác định đúng một tài khoản; một người nhiều tenant = một tài khoản nhiều membership; do đó không trùng trong cùng tenant; SSO xác thực trước, identity-service kiểm tra tenant sau (ADR-0067, ADR-0069) | [Confirmed] |

### 8.2. Kiểm tra chéo khi tạo phiên (`POST /api/v1/auth/session`)

Tóm tắt: có claim `organization` thì tenant đó phải thuộc membership của user (không → `TENANT_MISMATCH`); không có claim thì 1 membership → dùng, nhiều → `TENANT_NOT_SELECTED` (kèm danh sách), 0 → `ACCOUNT_NOT_PROVISIONED` (thu hồi phiên Keycloak, không tự đưa vào tenant chung). **Thuật toán đầy đủ, sơ đồ và thứ tự kiểm tra mã lỗi** chỉ ở [IDENTITY_MAPPING §3 và §4.1](../09-identity/IDENTITY_MAPPING.md#41-mã-lỗi-khi-resolve-thứ-tự-kiểm-tra); luồng OIDC ở [KEYCLOAK_SSO §6](../09-identity/KEYCLOAK_SSO.md).

### 8.3. Tenant chung

| Hạng mục | Quy tắc | Nhãn |
|---|---|---|
| Ai vào | Chỉ tài khoản do platform (System Admin) **tạo/import có chủ đích** vào tenant chung, kèm khối và lớp ảo theo khối (ADR-0063, ADR-0060). Không có JIT tự đưa người dùng vào tenant chung; người không thuộc tenant chung và không thuộc tenant SIS nào bị từ chối `ACCOUNT_NOT_PROVISIONED` | [Confirmed] |
| Cấu trúc | Tenant `common`, School ảo "EduWorld chung"; lớp ảo theo khối (`common-g1` … `common-g9`) để mọi chức năng cần lớp vẫn hoạt động; khối lớp do System Admin đặt khi tạo/import tài khoản | [Proposed] |
| Nội dung | Chỉ dữ liệu platform được System Admin gán cho `common` | [Proposed] |
| Chuyển sang tenant thật | Khi SIS của một tenant đồng bộ bản ghi khớp user → membership tenant đó → lần đăng nhập sau dùng tenant mới; lịch sử ở `common` giữ `tenant_id = common` | [Proposed] |
| An toàn | ChatPolicy, moderation, giới hạn thời gian chơi áp dụng như mọi tenant; moderator của `common` là đội platform trust & safety | [Proposed] |

### 8.4. Mỗi request sau khi có phiên

- Client gửi `X-Tenant-Id` ở mọi request (ADR-0065). APISIX: thiếu/sai → `400 TENANT_HEADER_REQUIRED`. Tenant middleware so header với claim `organization` và membership (cache 60 s) → lệch → `403 TENANT_MISMATCH` + audit; `Principal.TenantID` = kết quả kiểm tra.
- Mỗi service: `authz.ScopeFor(p)` → `TenantScope{[TenantID]}`; unit of work đặt `app.tenant_id = TenantID` trong database của service.
- Realtime: ticket WebSocket (game-service cấp) gắn `tenant_id`; gateway gắn `Ew-Tenant-Id` vào mọi lệnh NATS; chat-service/competition-service từ chối join room/channel của tenant khác (`PERMISSION_DENIED`).

### 8.5. Tình huống đăng nhập đặc biệt

| Tình huống | Xử lý | Nhãn |
|---|---|---|
| Giáo viên dạy ở nhiều tenant | Member của mọi tenant có phân công. Đăng nhập (không chọn trước) → `TENANT_NOT_SELECTED` + danh sách, chọn sau đăng nhập (ADR-0063 mục 5); đổi tenant = token mới với `organization:{alias khác}` (SSO) | [Confirmed] |
| Giáo viên dạy nhiều trường trong cùng tenant `unit` | Một phiên tenant; `Principal.SchoolIDs` gồm các trường có phân công; màn hình CMS lọc theo trường | [Confirmed] |
| Học sinh chuyển sang tenant khác | Worker gỡ membership tenant cũ, thêm tenant mới, thu hồi session gắn tenant cũ, phát `user.access_revoked`. Lần đăng nhập sau backend tự xác định tenant mới; token còn mang tenant cũ → `TENANT_MISMATCH`, client đăng nhập lại | [Confirmed] |
| Thiết bị phòng máy dùng chung | Tenant cố định trong file cấu hình thiết bị ([GODOT_ARCHITECTURE](../08-game-client/GODOT_ARCHITECTURE.md)) | [Proposed] |
| Nhân sự (giáo viên, TA, System Admin) mở game | Không được vào game: `403 GAME_ACCESS_STUDENT_ONLY` (ADR-0031); tenant không thay đổi điều này | [Confirmed] |
| Học sinh đăng nhập thiết bị thứ hai | Tenant của phiên mới phải khớp như bình thường; takeover chỉ kết thúc session cũ, không đổi tenant (ADR-0031) | [Confirmed] |
| Học sinh bị dọn do không đăng nhập (ADR-0032) | Soft-delete + gỡ membership tenant; đăng nhập lại và còn hợp lệ trong SIS của **cùng tenant** → khôi phục; SIS tenant khác → xử lý như chuyển tenant | [Confirmed] |
| System Admin | Đăng nhập Admin Portal (`eduworld-portal`, MFA bắt buộc với role platform), không chọn tenant, không gắn tenant; dropdown chọn tenant ở mỗi màn hình (ADR-0027 §6); thao tác ghi gửi `X-Tenant-Id` tenant đích; `CrossTenantScope` + audit | [Confirmed] |
| Nhân sự tenant đăng nhập portal | Không chọn tenant (ADR-0067); 1 tenant → vào thẳng; nhiều → hộp chọn sau đăng nhập + bộ chọn trên topbar; không có UserRole portal hiệu lực → `403 PORTAL_ACCESS_STAFF_ONLY` | [Confirmed] |
| Học sinh mở portal | Bị chặn ở Keycloak (realm role gate) và backend (`403 PORTAL_ACCESS_STAFF_ONLY`) (ADR-0066) | [Confirmed] |
| Thi đấu/giải liên tenant | Mỗi học sinh vẫn ở phiên tenant mình; match room cross-tenant có kiểm soát | [Confirmed] |

## 9. Soft-delete (ADR-0028)

| Quy tắc | Cách thực thi |
|---|---|
| Không hard-delete | `REVOKE DELETE, TRUNCATE` cho `<svc>_app`/`<svc>_platform` ở mọi database; ent hook chuyển delete → update `deleted_at`; lint `sqlguard` chặn `DELETE FROM` trong SQL tường minh của `services/**` (trừ purge của chat-service); CI kiểm tra grant |
| "Xóa" = `UPDATE ... SET deleted_at = now(), deleted_by = $actor` | ent `DeleteOne` qua hook soft-delete (`pkg/tenancy`); command phát event `<entity>.deleted` qua outbox; audit |
| Đọc mặc định loại dữ liệu đã xóa | ent interceptor + RLS SELECT `deleted_at IS NULL` |
| Unique sau khi xóa | Partial unique index `WHERE deleted_at IS NULL` |
| Khôi phục | Chỉ qua Thùng rác (System Admin; Tenant Admin trong tenant nếu được cấp [Proposed]): identity-service điều phối, service sở hữu thực hiện qua `POST /internal/v1/platform/trash/{resource_type}/{id}:restore`; event `<entity>.restored`; audit |
| Append-only | Không có thao tác xóa; sửa sai bằng bản ghi bù |
| Nghĩa vụ xóa theo luật | Ẩn danh hóa; Release 1.0 không xóa vật lý (ADR-0070) |

## 10. Gán và clone dữ liệu giữa tenant (ADR-0029)

```mermaid
flowchart LR
  SA[System Admin\neduworld-portal + MFA] -->|tạo| PD[(Dữ liệu platform\ntenant_id = NULL)]
  SA -->|gán| AS[(tenant_resource_assignments)]
  AS --> TA[Tenant A thấy chỉ đọc]
  AS --> TB[Tenant B thấy chỉ đọc]
  SA -->|clone job| CJ[(tenant_clone_jobs)]
  CJ -->|bản sao sâu, ID mới,\ncloned_from_*| TC[(Dữ liệu riêng tenant C)]
```

- Gán: `infra.tenant_resource_assignments(resource_type, resource_id, tenant_id, assigned_by, assigned_at, deleted_at)` nằm trong **database của service sở hữu tài nguyên**; System Admin thao tác qua `/api/v1/admin/tenant-assignments` (identity-service định tuyến tới service sở hữu theo `resource_type`); tùy chọn `apply_to_new_tenants` lưu ở `platform.tenant_default_assignments` (identity) và được áp khi có `tenant.created` [Proposed].
- Clone: saga do identity-service điều phối (`platform.tenant_clone_jobs`), từng service thực hiện phần của mình qua `POST /internal/v1/platform/tenant-clones/{job_id}/steps` (idempotent theo `job_id`), đồ thị phụ thuộc theo loại tài nguyên; phụ thuộc chéo service (vd. quest tham chiếu question set) được ánh xạ bằng `id_map` trả về của bước trước; **không clone** học sinh, enrollment, attempt, kết quả, chat, mastery, XP. Audit ghi ở cả tenant nguồn và đích.

## 11. Test bắt buộc

| Mã | Kịch bản | Loại test |
|---|---|---|
| AC-ORG-001 | Người dùng tenant A gọi mọi endpoint với ID tài nguyên của tenant B → 404, không rò dữ liệu | API test sinh từ OpenAPI (mọi path có `{id}`) |
| AC-ORG-002 | Giáo viên lớp 3A xem kết quả lớp 3B cùng trường → từ chối | API |
| AC-ORG-003 | Tenant Admin tenant `unit` liệt kê học sinh → mọi trường trong tenant, không có tenant khác; School Admin → chỉ trường được gán | API |
| AC-ORG-004 | System Admin lọc dropdown tenant A+B → chỉ dữ liệu A, B; có audit `cross_tenant_access` | API + audit assert |
| AC-ORG-005 | `app.tenant_id` = A, truy vấn bảng có dữ liệu B **không** thêm WHERE → RLS trả 0 dòng | Integration — chạy cho mọi bảng tenant-scoped của **mọi database** (liệt kê tự động) |
| AC-ORG-006 | Không set `app.tenant_id` → mọi truy vấn tenant-scoped trả 0 dòng | Integration |
| AC-ORG-007 | WS: subscribe channel/room tenant B → `PERMISSION_DENIED` | Realtime integration |
| AC-ORG-008 | Chuyển sang tenant khác: tenant mới không thấy attempt cũ; học sinh vẫn thấy lịch sử của mình | Integration |
| AC-ORG-009 | Nội dung riêng tenant A không xuất hiện ở tenant B; nội dung platform chưa gán không xuất hiện ở tenant nào | Integration |
| AC-ORG-010 | Cache Principal bị vô hiệu sau `enrollment.changed` ≤ 60 s | Integration |
| AC-ORG-011 | Học sinh tenant A đăng nhập với `organization:{alias B}` (đúng mật khẩu) → `TENANT_MISMATCH` + audit | E2E Keycloak + API |
| AC-ORG-012 | Học sinh thuộc 1 tenant đăng nhập (màn đăng nhập không có bước chọn trường) → vào đúng tenant đó | API, E2E |
| AC-ORG-013 | Giáo viên 2 tenant đăng nhập không chọn → `TENANT_NOT_SELECTED` + danh sách 2 tenant; token tenant A gọi tài nguyên B → 404 | API |
| AC-ORG-014 | (a) Tài khoản platform tạo trong tenant `common` → phiên trong `common`, bootstrap `is_common_tenant = true`; (b) tài khoản không thuộc tenant chung và không thuộc tenant SIS nào → `403 ACCOUNT_NOT_PROVISIONED`, phiên Keycloak bị thu hồi, không tạo user/membership trong `common` | API |
| AC-ORG-015 | Tenant bị tắt → `TENANT_ACCESS_DISABLED` cho mọi thành viên | API |
| AC-ORG-016 | Soft-delete một lớp → không xuất hiện ở mọi API đọc; tạo lại lớp cùng tên được (partial unique); Thùng rác thấy và khôi phục được | API + Integration |
| AC-ORG-017 | Role `<svc>_app`/`<svc>_platform` của mọi database chạy `DELETE` → lỗi quyền; role của service A không `CONNECT` được database service B | Integration |
| AC-ORG-020 | ent query không có `TenantScope` trong context → `ErrMissingTenantScope`, không trả dữ liệu | Unit + Integration |
| AC-ORG-021 | game-service gọi `POST /internal/v1/attempts` với `X-Tenant-Id` = A cho học sinh tenant B → learning-service từ chối (404/403) | Contract + Integration |
| AC-ORG-023 | Request tenant-scoped thiếu `X-Tenant-Id` → `400 TENANT_HEADER_REQUIRED` (gateway); header khác tenant của token/membership → `403 TENANT_MISMATCH` + audit (ADR-0065) | API + Security |
| AC-ORG-024 | System Admin ghi dữ liệu không gửi `X-Tenant-Id` tenant đích → `400 TENANT_HEADER_REQUIRED`; đọc liên tenant bằng `tenant_ids` có audit | API |
| AC-ORG-022 | Event mang `tenant_id` = A nhưng tham chiếu channel của tenant B → consumer từ chối, chuyển DLQ, alert | Integration |
| AC-ORG-018 | Clone question set A → C: bản sao có ID mới, `cloned_from_*`, trạng thái `draft`; không có dữ liệu học sinh nào được sao | Integration |
| AC-ORG-019 | Gỡ gán nội dung platform khỏi tenant → tenant không còn thấy; attempt cũ vẫn xem được kết quả | Integration |

CI có job `tenancy-guard`: (1) mọi bảng có cột `tenant_id` đều có `ENABLE`+`FORCE ROW LEVEL SECURITY` và policy; (2) mọi bảng nghiệp vụ không append-only có `deleted_at`, `deleted_by`; (3) mọi entity ent trên bảng tenant-scoped dùng mixin `Base`/`AppendOnly` (có interceptor) và mọi SQL tường minh có tham số scope + `deleted_at IS NULL`; (4) không có `DELETE FROM`/`TRUNCATE` trong query ứng dụng; (5) grant của role ứng dụng không chứa `DELETE`/`TRUNCATE`.
