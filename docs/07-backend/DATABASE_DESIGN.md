# Database Design — PostgreSQL (database riêng mỗi module)

> Trạng thái: Draft · Phiên bản: 0.6 · Owner: Backend Architect · Cập nhật: 2026-11-17 (`identity.external_identities`: soft-delete GlobalBase + unique `(issuer, subject)` partial, issuer `platform_common` cho `external_ref` — quyết định Technical Architect; ADR-0090: `platform.feature_flags` thêm `flag_kind`/`parent_key` + FK tới bảng đọc-chỉ `platform.portal_menu_catalog`; ADR-0091: `identity.portal_password_reset_tokens`; DDL §3.15) · 2026-11-17 (Áp ADR-0074: database theo module trên một cụm PostgreSQL, role `<module>_*`, rebuild projection qua facade; áp ADR-0089: RLS chỉ lọc tenant, view `_live`, bộ mixin theo loại bảng, role `_relay`/`_partition_admin`, retention bảng `infra` bằng gỡ partition, migration viết tay; làm rõ: mixin `TenantOptionalAppendOnly`, dòng tenant NULL ghi bằng `_platform`, khóa partition ổn định, mẫu ghi idempotency, dead letters, hàm `identity.memberships_for_user`) · 2026-11-16 (study group sang `eduworld_learning` `lms.*` — ADR-0084; phiên game giữ bằng refresh token — ADR-0086; khóa tài khoản nhiều nguồn)
> Liên quan: [ADR-0074](../adr/ADR-0074-modular-monolith-co-ranh-gioi-cuong-che.md) · [ADR-0037](../adr/ADR-0037-moi-service-mot-database-postgresql-thay-mot-phan-adr-0003.md) · [ADR-0089](../adr/ADR-0089-chot-tang-du-lieu-rls-mixin-outbox.md) · [ADR-0039](../adr/ADR-0039-orm-ent-cho-go-tuong-duong-entity-framework-migration-bang-a.md) · [ADR-0041](../adr/ADR-0041-moi-cot-truong-kieu-chuoi-phai-co-gioi-han-do-dai.md) · [ADR-0042](../adr/ADR-0042-domain-event-qua-transactional-outbox-nats-jetstream-thay-tr.md) · [ADR-0003](../adr/ADR-0003-postgresql-la-nguon-du-lieu-ben-vung-duy-nhat-schema-theo-mo.md) · [ADR-0005](../adr/ADR-0005-mo-hinh-multi-tenancy-shared-database-tenant-id-rls.md) · [ADR-0028](../adr/ADR-0028-chi-soft-delete-khong-hard-delete-mac-dinh-khong-doc-du-lieu.md) · [ADR-0029](../adr/ADR-0029-system-admin-gan-du-lieu-cho-tenant-va-clone-du-lieu-giua-te.md) · [ADR-0015](../adr/ADR-0015-content-versioning-bat-bien-sau-publish.md) · [DOMAIN_MODEL](../06-architecture/DOMAIN_MODEL.md) · [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [EVENT_DESIGN](EVENT_DESIGN.md) · [BACKUP_RECOVERY](../11-operations/BACKUP_RECOVERY.md)

**Mục đích & phạm vi.** Nguồn chuẩn duy nhất cho mô hình vật lý: database theo module, bảng, cột, kiểu, độ dài chuỗi, khóa, index, DDL, RLS SQL, partition, migration. Mô hình khái niệm ở DOMAIN_MODEL.

> **Thay đổi v0.6 (ADR-0074, thay ADR-0036):** backend là modular monolith; giữ nguyên 5 database, nay là database của 5 module domain, mặc định chung **một cụm** PostgreSQL (qua PgBouncer) ở mọi môi trường. `core-api` và `worker` mở một pool cho mỗi database của module mình nạp, bằng role của module đó; ranh giới không truy cập chéo database giữ nguyên.
>
> **Thay đổi v0.3:** tách theo 5 database của 5 service (ADR-0036/0037, nay đọc là 5 module theo ADR-0074); schema PostgreSQL giữ làm namespace trong database của module sở hữu; mọi cột chuỗi `varchar(n)` (ADR-0041); ent + Atlas thay sqlc + goose (ADR-0039); outbox/processed/idempotency chuyển sang schema `infra` của mỗi database và publish lên NATS JetStream (ADR-0042); thêm projection `ref` (§2.17).

## 1. Nguyên tắc

| # | Nguyên tắc | Nhãn |
|---|---|---|
| 1 | PostgreSQL ≥ 16 (khuyến nghị 17) là system of record | [Confirmed] |
| 2 | **Mỗi module domain một database** (`eduworld_identity`, `eduworld_learning`, `eduworld_game`, `eduworld_competition`, `eduworld_chat`) trên **một cụm** PostgreSQL chung (ADR-0074 mục 4); không module nào kết nối database của module khác (role chỉ có `CONNECT` trên database của module mình); không có FK/join/transaction chéo database (ADR-0037). Schema PostgreSQL bên trong mỗi database giữ vai trò namespace theo bounded context (§2.0a) | [Confirmed — ADR-0037/0074] |
| 3 | Dữ liệu tham chiếu của module khác cần đọc thường xuyên được lưu dạng **projection** trong schema `ref` của database dùng nó, cập nhật idempotent từ event NATS (§2.17) | [Confirmed — ADR-0037] |
| 4 | PK là `uuid` UUIDv7 sinh ở ứng dụng; bảng liên kết dùng PK ghép | [Confirmed] |
| 5 | `timestamptz` UTC cho mọi thời điểm; `created_at`, `updated_at` mặc định | [Confirmed] |
| 6 | Bảng tenant-scoped có `tenant_id uuid NOT NULL` + RLS theo `app.tenant_id` trong **mọi** database. Ở `eduworld_identity`, FK tới `platform.tenants`; ở database khác, FK tới projection `ref.tenants`. `school_id` là chiều bên trong tenant | [Confirmed — ADR-0005] |
| 7 | **Chỉ soft-delete**: bảng không append-only có `deleted_at timestamptz`, `deleted_by uuid`, `delete_reason varchar(64)`; role ứng dụng không có `DELETE`/`TRUNCATE`; query đọc lọc `deleted_at IS NULL` bằng ent interceptor (RLS chỉ lọc tenant; lớp thứ hai cho bản ghi đã xóa là view `<table>_live` của `<module>_readonly`, ADR-0089 §1); unique dùng partial index `WHERE deleted_at IS NULL`. Ngoại lệ duy nhất: purge chat gameplay trong `eduworld_chat` (ADR-0033) | [Confirmed — ADR-0028/0033] |
| 8 | Dữ liệu platform dùng chung có `tenant_id NULL` + `owner_scope = 'platform'`; tenant thấy khi có dòng active trong `infra.tenant_resource_assignments` **của database sở hữu tài nguyên đó**. Bảng có thể clone mang `cloned_from_id`, `cloned_from_tenant_id`, `cloned_at`, `cloned_by` | [Confirmed — ADR-0029] |
| 9 | **Mọi cột chuỗi là `varchar(n)` có `n` tường minh** theo bảng §1.4; không dùng `text` không giới hạn; nội dung dài có cấu trúc dùng `jsonb` + CHECK `octet_length(col::text) <= N` | [Confirmed — ADR-0041] |
| 10 | Enum lưu `varchar(64)` + `CHECK` (dễ migrate hơn `CREATE TYPE`) | [Confirmed] |
| 11 | JSONB chỉ cho dữ liệu có schema phiên bản hóa (render payload, rule, settings); field cần query/index đưa ra cột riêng | [Confirmed] |
| 12 | ORM **ent** (code-first, mixin chung, interceptor tenant + soft-delete, `version` cho optimistic concurrency); migration **Atlas** versioned; SQL tường minh chỉ ở tầng infrastructure cho query đọc phức tạp (ADR-0039) | [Confirmed — ADR-0039] |
| 13 | Mỗi database có schema `infra` chứa `outbox_events`, `processed_events`, `idempotency_records`, `dead_letters`, `tenant_resource_assignments` (nếu module có tài nguyên gán được) và hàm `infra.current_tenant_id()` (ADR-0042) | [Confirmed — ADR-0042] |
| 14 | Dữ liệu cá nhân nhạy cảm (ngày sinh) mã hóa cột bằng `pgcrypto` hoặc mã hóa ứng dụng (key trong secret manager) | [Confirmed] |

### 1.1. DB roles (mỗi database một bộ)

Mỗi database có bộ role riêng, đặt tên theo module (`identity_*`, `learning_*`, `game_*`, `competition_*`, `chat_*`; ADR-0074 mục 4). Trong DDL mẫu dưới đây, `<module>_` đại diện cho bộ role của database chứa bảng đó; ví dụ gắn với một database cụ thể dùng tên thật (`identity_app`, `learning_platform`…).

| Role | Dùng bởi | Quyền |
|---|---|---|
| `<module>_migrator` | job `migrate --module=<module>` (Atlas) | DDL; sở hữu schema trong database của module |
| `<module>_app` | pool của module trong `core-api`, consumer thường trong `worker` | `SELECT`, `INSERT`, `UPDATE` trên schema của module; **không `DELETE`/`TRUNCATE`**; **chịu RLS** |
| `<module>_platform` | job platform của worker, phiên System Admin liên tenant, bước saga platform, ghi dòng `tenant_id IS NULL` của bảng T\*/TA\*/H dưới `PlatformScope` | `SELECT`, `INSERT`, `UPDATE` + `BYPASSRLS`; `SELECT`, `UPDATE` cột xử lý của `infra.dead_letters`; **không `DELETE`/`TRUNCATE`**; query vẫn có `tenant_id` tường minh; mọi phiên ghi audit |
| `chat_purger` | **chỉ** job `gameplay-chat-purge` của module chat | Ngoại lệ duy nhất của ADR-0028 (ADR-0033): `SELECT`, `UPDATE`, `DELETE` chỉ trên bảng chat gameplay (§3.0) trong `eduworld_chat`; không có quyền ở database khác |
| `<module>_relay` | relay outbox của module (profile `worker-events`) | `SELECT`, `UPDATE (published_at, attempts)` trên `infra.outbox_events`; không quyền trên schema nghiệp vụ (ADR-0089 §4) |
| `<module>_partition_admin` | **chỉ** job `infra-partition-maintenance` | `CREATE`/`DETACH`/`DROP` partition trên schema `infra`; không quyền trên schema nghiệp vụ; không `DELETE`/`TRUNCATE` (ADR-0089 §5, §3.9) |
| `<module>_readonly` | BI, read model báo cáo trên replica | `SELECT` **chỉ** trên view `<schema>.<table>_live` (`WHERE deleted_at IS NULL`) và view reporting; không quyền trên bảng gốc; chịu RLS (ADR-0089 §1) |
| `<module>_dashboard` (`learning_dashboard`, `game_dashboard`) | API dashboard, export worker — **chỉ trên read replica** | `SELECT` chỉ trên bảng rollup, read model và view dashboard; không quyền trên bảng nghiệp vụ/thô; chịu RLS; `statement_timeout = 5s`, `idle_in_transaction_session_timeout = 10s`, `work_mem = 32MB` (ADR-0062, [DASHBOARD_DATA §7](DASHBOARD_DATA.md#7-bảo-vệ-database-khi-truy-vấn)) |
| `<module>_backup` | pgBackRest/WAL-G | replication |

Không user database nào có quyền trên database của module khác, dù cùng cụm. CI (`db-privileges`) kiểm tra: không role ứng dụng nào có `DELETE`/`TRUNCATE` (trừ `chat_purger` trên danh sách bảng cho phép) và không có `CONNECT` chéo database. Ánh xạ DSN: [DEPLOYMENT](../11-operations/DEPLOYMENT.md).

### 1.2. Database theo module

| Database | Module sở hữu | Schema (namespace) bên trong |
|---|---|---|
| `eduworld_identity` | module identity | `platform` (tenant, settings, cleanup, clone/trash job), `identity`, `org`, `catalog` (danh mục platform, §2.2a), `sis`, `audit`, `infra` |
| `eduworld_learning` | module learning | `curriculum`, `content`, `assessment`, `learning`, `lms`, `ref`, `infra` |
| `eduworld_game` | module game | `game`, `gamification`, `analytics` (Game Analytics, §2.8b), `ref`, `infra` |
| `eduworld_competition` | module competition | `multiplayer`, `competition`, `ref`, `infra` |
| `eduworld_chat` | module chat | `social`, `chat`, `ref`, `infra` |
| — | realtime | Không có database (chỉ Valkey) |

Cả 5 database nằm trên một cụm PostgreSQL (primary + replica, Patroni, PgBouncer); sizing và việc chuyển một database sang cụm riêng khi có số đo: [DEPLOYMENT §7](../11-operations/DEPLOYMENT.md#7-postgresql-adr-0037).

Các cột tham chiếu thực thể của module khác (ví dụ `student_id` trong `assessment.attempts`, `quest_version_id` trong `quest_progress` của party) là `uuid` **không có FK chéo database**; tính hợp lệ kiểm tra ở use case (qua projection `ref` hoặc facade của module sở hữu, gọi trong tiến trình) tại thời điểm ghi.

### 1.3. Audit tập trung

Mỗi module ghi sự kiện audit vào outbox (`audit.recorded`, payload không chứa PII/nội dung chat); worker của module identity tiêu thụ stream `EVT_*` subject `ew.evt.*.audit.recorded` và ghi `audit.audit_logs` trong `eduworld_identity` (§3.11) [Confirmed]. Như vậy audit được truy vấn tập trung mà không cần join chéo database.

### 1.4. Bảng độ dài chuỗi chuẩn (ADR-0041)

| Loại cột | Kiểu | Ví dụ cột |
|---|---|---|
| Mã, enum, trạng thái, loại, vai trò, locale, slug | `varchar(64)` | `code`, `status`, `kind`, `type`, `role`, `mode`, `event_type`, `emoji_code`, `delete_reason`, `idp_alias`, `keycloak_org_alias` |
| Mã ngắn có định dạng | `varchar(16)` | `login_code`, `method` HTTP |
| External ID, khóa idempotency, client id | `varchar(64)` | `student_external_id`, `idempotency_key`, `client_message_id` (khi là chuỗi), `keycloak_sid` |
| Biệt danh người chơi | `varchar(32)` | `nickname` (mọi bảng, projection và contract dùng cùng độ dài) |
| Tên ngắn | `varchar(128)` | `display_name`, `device_model`, tên lớp, tên party, `name_vi` của sticker pack |
| Tên / tiêu đề | `varchar(255)` | tên tenant/trường, tiêu đề bài học, `subject` OIDC, `term`, `phrase`, `route_template` |
| Email | `varchar(320)` | `work_email` |
| Khóa lưu trữ / issuer | `varchar(512)` | `storage_key`, `asset_key`, `preview_key`, `issuer` |
| URL | `varchar(2048)` | `media_url` |
| Mô tả ngắn, lý do, ghi chú, lỗi | `varchar(1000)` | `description`, `reason`, `note`, `error`, `alt_text_vi`, `change_summary` |
| Tin nhắn chat | `varchar(1000)` (policy hiển thị mặc định 300) | `body`, `body_display` |
| Nội dung dài | `varchar(20000)` hoặc `jsonb` + CHECK `octet_length(col::text) <= 65536` | nội dung bài học, giải thích, render payload |
| Hash dạng hex | `varchar(128)` | `content_sha256` (khi lưu hex); hash nhị phân dùng `bytea` có CHECK độ dài |
| Mảng chuỗi | `varchar(64)[]` + CHECK `cardinality(col) <= N` | `permissions`, `category_codes`, `variants` (`varchar(255)[]`) |

Thay đổi `n` là migration expand/contract; tăng `n` an toàn, giảm `n` cần kiểm tra dữ liệu và ADR. Lint `entlint` chặn `field.String` không có `MaxLen` ([BACKEND_DESIGN §5.3](BACKEND_DESIGN.md)).

## 2. Danh sách bảng theo database / schema

Ký hiệu: **T** = tenant-scoped (có `tenant_id NOT NULL` + `deleted_at`/`deleted_by` + RLS), **TA** = tenant-scoped append-only (có `tenant_id`, không có `deleted_at`, không xóa), **T\*** = tenant nullable (`tenant_id NULL` cho dòng platform, vd. `user_roles`, `account_locks`), **TA\*** = append-only, tenant nullable (`identity.login_events`, `content.content_releases`, `content.content_transitions`), **H** = nội dung lai (`tenant_id NULL` = platform), **G** = global/platform (có `deleted_at`), **GA** = global append-only (không `tenant_id`, không `deleted_at`), **P** = partitioned theo tháng.

**Loại bảng → mixin `src/backend/pkg/entx` (ADR-0089 §2).** Mỗi entity ent khai báo đúng một mixin; CI có test liệt kê mọi schema ent và từ chối schema không có mixin. Điều kiện interceptor của từng mixin: [MULTITENANCY §4.4](../06-architecture/MULTITENANCY.md#44-quy-tắc-interceptor-theo-mixin-ba-loại-scope-và-cổng-bypass-adr-0089). Dòng `tenant_id IS NULL` của bảng T\*/TA\*/H chỉ được ghi bằng role `<module>_platform` dưới `PlatformScope`; policy RLS áp cho `<module>_app` giữ điều kiện tenant thuần (ADR-0089 §2).

| Loại | Mixin | RLS (§4) |
|---|---|---|
| T | `Base` | chỉ điều kiện tenant |
| TA | `TenantAppendOnly` | chỉ điều kiện tenant |
| T\* | `TenantOptional` | chỉ điều kiện tenant; dòng `tenant_id IS NULL` chỉ đọc/ghi qua `PlatformScope` (`<module>_platform`) |
| TA\* | `TenantOptionalAppendOnly` | chỉ điều kiện tenant; dòng `tenant_id IS NULL` chỉ đọc/ghi qua `PlatformScope` (`<module>_platform`) |
| H | `HybridContent` (tên cũ `PlatformBase`) | tenant + gán qua `infra.tenant_resource_assignments` |
| G | `GlobalBase` | không bật |
| GA | `AppendOnly` | không bật |

**Quy ước cột chung (không liệt kê lặp lại ở từng bảng dưới đây):**

| Loại bảng | Cột bắt buộc | Ghi chú |
|---|---|---|
| T | `tenant_id uuid NOT NULL REFERENCES platform.tenants` (DB identity) hoặc `REFERENCES ref.tenants` (DB khác), `deleted_at timestamptz`, `deleted_by uuid`, `delete_reason varchar(64)`, `created_at`, `updated_at` | `tenant_id` là cột đầu tiên của mọi index phức hợp; mọi UNIQUE ghi trong tài liệu này là partial `WHERE deleted_at IS NULL` |
| TA | `tenant_id uuid NOT NULL` | Không cập nhật/xóa; sửa sai bằng bản ghi bù |
| H | `tenant_id uuid NULL`, `owner_scope varchar(64) CHECK (owner_scope IN ('platform','tenant'))`, `deleted_at`, `deleted_by`, `cloned_from_id`, `cloned_from_tenant_id`, `cloned_at`, `cloned_by` | CHECK `(owner_scope = 'platform') = (tenant_id IS NULL)` |
| G | `deleted_at`, `deleted_by` | Ghi chỉ qua `/api/v1/game-admin/*`, `/api/v1/admin/*` |

Cột `school_id` vẫn có ở các bảng gắn với trường (lớp, học sinh, enrollment, attempt, UserRole scope trường, báo cáo) để lọc/báo cáo theo trường bên trong tenant.

### 2.0. `platform` — Tenancy (ADR-0005, ADR-0026, ADR-0029) · DB `eduworld_identity`

| Bảng | Loại | Cột chính | Ràng buộc / index |
|---|---|---|---|
| `tenants` | G | `tenant_id`, `code`, `name`, `tenant_type` (`school`/`unit`/`common`), `status` (`active`/`disabled`), `keycloak_org_id uuid`, `keycloak_org_alias varchar(64)`, `login_code varchar(16)`, `settings jsonb`, `is_qa boolean NOT NULL DEFAULT false` (tenant kiểm thử, bị loại khỏi analytics và dashboard liên tenant; CHECK `NOT (is_qa AND tenant_type = 'common')`; chỉ System Admin đổi), `deleted_at`, `deleted_by` | UNIQUE `code`, `keycloak_org_id`, `keycloak_org_alias` (CHECK `^[a-z0-9-]{3,63}$`), `login_code` (CHECK 6 ký tự chữ hoa/số, không có 0/O/1/I) — đều partial `WHERE deleted_at IS NULL`; **UNIQUE `(tenant_type) WHERE tenant_type = 'common'`** (duy nhất một tenant chung). Tra cứu khi login: `keycloak_org_alias` → `tenant_id` (cache `ew:tenant:alias:<alias>` 5 phút) |
| _(ghi chú)_ | — | Bảng gán thực tế `infra.tenant_resource_assignments` nằm ở **database sở hữu tài nguyên** (§2.17); module identity điều phối qua saga. Dòng dưới mô tả cấu trúc chung | — |
| `infra.tenant_resource_assignments` (mỗi DB sở hữu tài nguyên) | G | `assignment_id`, `resource_type` (`curriculum`/`lesson`/`question`/`question_set`/`assessment_blueprint`/`challenge_template`/`tournament_template`/`quest`/`minigame_config`/`item`…), `resource_id`, `tenant_id`, `assigned_by`, `assigned_at`, `deleted_at`, `deleted_by` | UNIQUE `(resource_type, resource_id, tenant_id) WHERE deleted_at IS NULL`; idx `(tenant_id, resource_type, resource_id) WHERE deleted_at IS NULL`. Chỉ ghi được với resource có `tenant_id IS NULL` (kiểm tra ở use case) |
| `tenant_default_assignments` | G | `default_assignment_id` (PK), `resource_type`, `resource_id`, `owner_module`, `created_by`, `created_at`, `deleted_at`, `deleted_by`, `delete_reason` | "Gán cho mọi tenant mới": consumer `tenant.created` gán qua facade module sở hữu (F045) [Confirmed]; partial unique `(resource_type, resource_id) WHERE deleted_at IS NULL`; view `_live` |
| `tenant_clone_jobs` | G | `job_id`, `source_tenant_id`, `target_tenant_id`, `resource_type`, `resource_ids uuid[]`, `options jsonb`, `status` (`queued`/`running`/`succeeded`/`partially_failed`/`failed`), `id_map jsonb` (nguồn → bản sao, ≤ 1 MiB), `stats jsonb`, `errors jsonb` (lỗi từng bước), `saga_id` (FK `saga_runs`, F045), `requested_by`, `idempotency_key`, `created_at`, `updated_at`, `finished_at`, `version` | UNIQUE `(requested_by, idempotency_key)`, UNIQUE `saga_id`; CHECK `source_tenant_id <> target_tenant_id`; danh sách `resource_type` được phép clone nằm trong code (không gồm bảng dữ liệu học sinh) |

| `saga_runs` | G | `saga_id`, `saga_type` (`tenant_clone`/`trash_restore`/`student_cleanup`/`student_restore`/`user_merge`/`user_anonymize`), `subject jsonb`, `status` (`running`/`compensating`/`succeeded`/`failed`), `requested_by`, `idempotency_key varchar(64)`, `started_at`, `finished_at` | UNIQUE `(saga_type, idempotency_key)`; điều phối thao tác liên module (ADR-0074) |
| `saga_steps` | G | `saga_id`, `step_no`, `service varchar(64)`, `action varchar(64)`, `status`, `attempts`, `request jsonb`, `response jsonb` (id_map…), `error varchar(1000)`, `updated_at` | PK `(saga_id, step_no)`; mỗi bước gọi facade platform của module đích (trong tiến trình, ADR-0074), idempotent theo `saga_id`; cột `service` lưu tên module đích |
| `platform_settings` | G | `key` (`student_cleanup`, `chat_retention`, …), `value jsonb`, `version`, `updated_by`, `updated_at` | PK `key`; ghi chỉ System Admin, bắt buộc `reason` trong audit. `student_cleanup` = `{default_days: 180, notice_days: 14, allow_tenant_override: true, approval_threshold_percent: 10}`; `chat_retention` = `{gameplay_chat_retention_days: 7, minimum_days: 3, ephemeral_retention_hours: 24, allow_tenant_override: true}` [Confirmed] |
| `feature_flags` | G | `flag_key varchar(128)` (PK, CHECK `^[a-z0-9_.]{3,128}$`), `flag_kind varchar(16)` (`feature`/`menu`/`action`, mặc định `feature`), `parent_key varchar(128)` (action → menu cha), `description varchar(1000)`, `default_enabled boolean`, `tenant_overrides jsonb` (`{tenant_id: bool}`, ≤ 64 KB), `catalog_key` (cột sinh = `flag_key` khi `flag_kind <> 'feature'`), `version`, `updated_by`, `updated_at`, `deleted_at`, `deleted_by`, `delete_reason` | FK `catalog_key` và `parent_key` → `portal_menu_catalog.item_key` (flag `feature` không bị kiểm); CHECK `(flag_kind = 'action') = (parent_key IS NOT NULL)`. Không có dòng = bật; ghi chỉ System Admin (`platform.feature_flag.write`, `If-Match`, `reason`, audit, `platform_settings.changed`) — ADR-0090, DDL §3.15 |
| `portal_menu_catalog` | G (đọc-chỉ) | `item_key varchar(128)` (PK: workspace, `menu.<workspace>.<menu>`, `action.<workspace>.<menu>.<action>`), `item_kind` (`workspace`/`menu`/`action`), `workspace_key`, `parent_key`, `path varchar(256)`, `label_key varchar(128)`, `title varchar(200)`, `required_permissions varchar(64)[]` (any-of), `operation_ids varchar(128)[]`, `sort_order`, `can_disable boolean`, `catalog_version varchar(64)`, `synced_at`, `deleted_at`, `deleted_by` | **Bản sao đọc-chỉ** của `specs/portal/menu-catalog.yaml`, ghi bởi `src/backend/cmd/migrate` lúc deploy (cùng mẫu `identity.permissions`); `identity_app`/`identity_platform` chỉ `SELECT`; key bị gỡ khỏi catalog → soft-delete — ADR-0090, DDL §3.15 |
| `cleanup_exclusions` | T | `exclusion_id`, `tenant_id`, `student_id`, `until` (NULL = vô thời hạn), `reason`, `created_by`, `created_at`, `deleted_at`, `deleted_by` | UNIQUE `(tenant_id, student_id) WHERE deleted_at IS NULL` (ADR-0032) |
| `student_cleanup_runs` | G (append-only) | `run_id`, `tenant_id`, `mode` (`dry_run`/`apply`), `status` (`running`/`completed`/`awaiting_approval`/`approved`/`cancelled`/`failed`), `cutoff_days`, `candidates`, `cleaned`, `excluded`, `skipped_active`, `approved_by`, `started_at`, `finished_at` | idx `(tenant_id, started_at DESC)` |

Cài đặt theo tenant (`platform.tenants.settings`): `inactive_student_cleanup_days` (null/0 = tắt; vắng = mặc định platform), `gameplay_chat_retention_days` (≥ 3). Giá trị hiệu lực = tenant nếu platform `allow_tenant_override`, ngược lại = platform.

Tenant `common` có một School ảo (`org.schools.is_virtual = true`) và lớp ảo theo khối, được tạo bằng migration seed.

### 2.1. `identity` · DB `eduworld_identity`

| Bảng | Loại | Cột chính | Ràng buộc / index |
|---|---|---|---|
| `users` | G | `user_id`, `user_type`, `status` (`pending`/`active`/`locked`/`deactivated`/`anonymized`; `locked` ⇔ còn ít nhất một dòng `account_locks` chưa giải phóng), `username` (đã chuẩn hóa trim/NFC/chữ thường, bản sao từ Keycloak, ADR-0067), `email` (NULL được với học sinh, bắt buộc với nhân sự), `display_name` (NULL sau anonymize), `locale`, `primary_tenant_id`, `primary_school_id`, `created_at`, `updated_at`, `deactivated_at`, `anonymized_at`, `provisioned_at`, `last_login_at` (lần tạo game session gần nhất — ADR-0031), `deleted_at`, `deleted_by`, `delete_reason` (`inactive_cleanup`/…) | User là định danh toàn cục (một nhân sự có thể thuộc nhiều tenant); `user_type` ∈ `student`/`staff` (ADR-0081): nhân sự platform = `staff` có `primary_tenant_id IS NULL`, nhân sự trường = `staff` có `primary_tenant_id`; học sinh luôn có `primary_tenant_id` (CHECK); idx `(primary_tenant_id, user_type) WHERE deleted_at IS NULL`; `status` CHECK. **UNIQUE `lower(username)`** (mọi dòng, kể cả đã soft-delete — chỉ giải phóng khi ẩn danh hóa đổi username thành `anon.<user_id>`); **UNIQUE `lower(email)` WHERE `email IS NOT NULL AND deleted_at IS NULL`** [Confirmed: email của tài khoản đã soft-delete vẫn giữ chỗ đến khi ẩn danh hóa, kiểm tra trong use case]; CHECK nhân sự (`user_type = 'staff'`) có email (ADR-0067). Duy nhất toàn cục khớp Keycloak (một người một tài khoản SSO) nên trong cùng tenant không bao giờ trùng; bảng theo tenant có `tenant_id NOT NULL` (ADR-0069). Vi phạm → `409 USERNAME_TAKEN`/`EMAIL_TAKEN`; SIS/import → `reconciliation_issue` `duplicate_identifier`. Tenant membership suy ra từ `org.students`/`org.enrollments`/`identity.user_roles` (view `identity.tenant_memberships_v` `WITH (security_invoker = true)` để RLS áp theo role gọi, đúng một dòng mỗi `(tenant_id, user_id)`; F010 tạo phần học sinh, F012 thêm nhân sự) |
| `external_identities` | G | `external_identity_id`, `issuer`, `subject`, `user_id`, `idp_alias`, `status` (`active`/`superseded`), `superseded_at`, `superseded_by`, `last_login_at` | **UNIQUE `(issuer, subject) WHERE deleted_at IS NULL`** (`issuer = platform_common`: `external_ref` học sinh tenant chung, không dùng đăng nhập — ACCOUNT_PROVISIONING §3.4); idx `user_id`; UNIQUE `(user_id, issuer) WHERE status='active'`. Dòng `superseded` do job `identity-relink` để lại phục vụ truy vết ([IDENTITY_MAPPING §7.1](../09-identity/IDENTITY_MAPPING.md)); resolve `(iss, sub)` chỉ chấp nhận dòng `active` |
| `permissions` | G | `permission_code` (PK, vd. `lms.assignment.write`), `scope_kind` (`tenant`/`platform`), `group_code`, `name_vi`, `name_en`, `catalog_version`, `synced_at` | **Bản sao đọc-chỉ** của catalog permission trong code (`src/backend/pkg/authz/catalog.go`), ghi bởi `src/backend/cmd/migrate` lúc deploy; app role chỉ SELECT. Permission tiền tố `game.`, `competition.config.`, `platform.` luôn có `scope_kind = platform` (CHECK) |
| `roles` | G/T | `role_id`, `code` (vd. `school_admin`, `academic_manager`, `safety_moderator`), `name_vi`, `name_en`, `description`, `role_scope_kind` (`tenant`/`platform`), `is_system`, `tenant_id` (NULL = role dùng chung toàn platform; có giá trị = role tùy biến của tenant [Confirmed]), `version`, cột audit chuẩn, `deleted_at` | UNIQUE `(code) WHERE tenant_id IS NULL AND deleted_at IS NULL`; UNIQUE `(tenant_id, code) WHERE tenant_id IS NOT NULL AND deleted_at IS NULL`; CHECK `role_scope_kind = 'tenant' OR tenant_id IS NULL` (role tùy biến chỉ là role tenant); role `is_system` do migrate seed (id tất định theo mã), trigger `roles_system_readonly` chặn UPDATE bởi mọi role trừ `identity_migrator` (ADR-0066). Mixin T* (F012): RLS cho mọi tenant **đọc** dòng dùng chung (`tenant_id IS NULL`), ghi chỉ trong tenant của phiên; `name_en` mặc định bằng `name_vi` khi không nhập |
| `role_permissions` | G/T | `role_permission_id`, `role_id`, `permission_code` (FK `permissions`), `tenant_id` (bằng `roles.tenant_id`) | Khóa surrogate + UNIQUE `(role_id, permission_code) WHERE deleted_at IS NULL` (F012; gỡ permission = soft-delete); quyền của role hệ thống chỉ `identity_migrator` ghi; trigger `role_permissions_scope_chk`: role `tenant` không được nhận permission `scope_kind = platform` (ADR-0027, ADR-0066) — lớp 2 sau use case `CreateRole`/`UpdateRole` |
| `user_roles` | T* | `user_role_id`, `user_id`, `role_id`, `tenant_id` (NULL nếu `scope_type = platform`), `scope_type` (`platform`/`tenant`/`school`/`subject`; không có `class` — ADR-0072), `scope_id`, `school_id` (null nếu scope platform/tenant), `valid_from`, `valid_to`, `granted_by`, `source` (chỉ `manual`: SIS không cấp role — ADR-0072), `training_completed_at` (bắt buộc với role `safety_moderator`, [CHILD_SAFETY](../10-security/CHILD_SAFETY.md)), cột audit chuẩn, `deleted_at`, `deleted_by` | Thay `role_assignments` (ADR-0066). UNIQUE `(user_id, role_id, scope_type, scope_id) WHERE deleted_at IS NULL AND valid_to IS NULL`; idx `(user_id, tenant_id) WHERE deleted_at IS NULL`; idx `(tenant_id, role_id)`; trigger `user_roles_scope_chk` (§3.1): role `platform` chỉ `scope_type = platform`, role `tenant` không ở scope platform, role tùy biến chỉ gán trong tenant sở hữu, học sinh (`users.user_type = 'student'`) không nhận role có permission portal; **bất biến ADR-0081**: user có `primary_tenant_id IS NULL` chỉ nhận role `role_scope_kind = platform`, user có `primary_tenant_id` chỉ nhận role `tenant` (một người làm cả hai việc dùng hai tài khoản) |
| `sis_identity_links` | T | `user_id`, `tenant_id`, `school_id`, `sis_connection_id`, `student_external_id`, `status` (`active`/`superseded`), `linked_at`, `superseded_at`, `superseded_by` | UNIQUE một link active/user và một link active/khóa nguồn (ADR-0046, DDL §3.14); ghi lúc apply SIS cùng transaction với user `pending` (F011), chuyển SIS cùng tenant đặt link cũ `superseded` |
| `account_locks` | T* | `lock_id`, `user_id`, `tenant_id` (NULL chỉ khi khóa nhân sự platform, `source = platform_admin`), `source varchar(32)` (`school_admin`/`platform_admin`/`sis`/`moderation_sanction`), `source_ref_id` (`sanction_id` khi `moderation_sanction`), `case_id`, `reason varchar(1000)` (nội bộ, không trả học sinh), `created_by` (NULL = hệ thống), `created_at`, `released_at`, `released_by`, `release_reason varchar(1000)`, `source_event_id`, `deleted_at`, `deleted_by` | Khóa tài khoản **nhiều nguồn**: mỗi nguồn khóa/mở độc lập, nhà trường mở khóa không gỡ được khóa của Safety Team (`moderation_sanction` chỉ giải phóng bởi `sanction.lifted`). idx `(user_id) WHERE released_at IS NULL AND deleted_at IS NULL`; UNIQUE `(source, source_ref_id) WHERE source_ref_id IS NOT NULL AND deleted_at IS NULL` ([ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md), [MODERATION](../03-social-multiplayer/MODERATION.md)) |
| `game_auth_sessions` | T | `game_auth_session_id`, `tenant_id`, `user_id`, `keycloak_sid varchar(64)`, `device_id uuid`, `platform varchar(16)` (`windows`/`macos`/`linux`/`android`/`ios`/`web`), `refresh_token_hash char(64)` (SHA-256 hex của refresh token hiện hành, **không** lưu token thô), `previous_token_hashes jsonb` (≤ 20 hash đã xoay, phát hiện dùng lại), `created_at`, `last_refreshed_at`, `revoked_at`, `revoke_reason varchar(32)` (`logout`/`takeover`/`password_changed`/`account_locked`/`sanction`/`access_revoked`/`cleanup`/`refresh_reuse`/`admin`), `deleted_at`, `deleted_by` | Phiên đăng nhập game giữ bằng refresh token xoay vòng gắn thiết bị ([ADR-0086](../adr/ADR-0086-giu-phien-game-bang-refresh-token.md)). UNIQUE `keycloak_sid` và `refresh_token_hash` partial (phiên chưa thu hồi, `deleted_at IS NULL`); idx `(tenant_id, user_id) WHERE revoked_at IS NULL`, `(device_id) WHERE revoked_at IS NULL`; GIN `previous_token_hashes` |
| `email_deliveries` | T* | `delivery_id`, `tenant_id` (NULL cho email nhân sự platform), `source_event_id`, `template_code varchar(64)` (`school_safety_notification`/`school_safety_reminder`/`moderation_oncall_page`/`break_glass_request`), `recipient_user_id` (**không** lưu địa chỉ email), `params jsonb` (≤ 2 KB, chỉ ID và thời điểm), `status varchar(16)` (`pending`/`sent`/`failed`/`skipped`), `attempts smallint`, `next_attempt_at`, `sent_at`, `created_at`, `last_error_code varchar(64)` (mã SMTP, không thân lỗi) | Hàng đợi email gửi qua SMTP của module identity; UNIQUE `(source_event_id, recipient_user_id, template_code)`; idx `(status, next_attempt_at) WHERE status = 'pending'`; chỉ thêm và cập nhật trạng thái |
| `portal_password_reset_tokens` | G | `reset_token_id`, `user_id` (nhân sự), `token_hash char(64)` (SHA-256 hex, **không** lưu token thô), `locale`, `expires_at` (= tạo + `portal_password_reset_ttl`), `used_at`, `created_at`, `deleted_at`, `deleted_by`, `delete_reason` (`superseded`/`revoked`/`user_deactivated`) | UNIQUE `token_hash`; UNIQUE `(user_id) WHERE used_at IS NULL AND deleted_at IS NULL` (một token hiệu lực/user, cấp mới soft-delete token cũ); dùng một lần bằng `UPDATE … SET used_at` nguyên tử; không lưu IP thô — ADR-0091, DDL §3.15 |
| `login_events` | TA\*, P | Nhật ký **xác thực** (mọi client, kể cả thất bại — khác `game.login_events` chỉ ghi phiên game thành công, ADR-0031): PK `(login_event_id, occurred_at)`, `user_id`, `tenant_id` (NULL khi thất bại trước khi xác định tenant — chỉ `identity_platform` ghi được, RLS), `occurred_at`, `client`, `ip_hash` (HMAC), `sis_connection_id` (ADR-0046, nullable), `device_id` (nullable), `result` (CHECK: `success`/`invalid_credentials`/`account_not_provisioned`/`game_access_student_only`/`account_locked`/`account_deactivated`/`no_active_enrollment`/`tenant_not_selected`/`tenant_mismatch`/`rate_limited`/`queue_busy`/`client_version_unsupported`/`curfew_active`/`unavailable`), `tenant_resolution` (`claim`/`single_membership`/`common`/`selected`/`mismatch`); `REVOKE UPDATE, DELETE, TRUNCATE` (F013) | retention `login_events_retention` (90 ngày rồi ẩn danh hóa; partition giữ lại, ADR-0070) |

**Mở rộng sau (ADR-0063):** bảng `identity.login_credentials` (PIN, thẻ QR, mật khẩu hình ảnh cho đăng nhập nhanh — ADR-0045 mục 4) **không có trong Release 1.0**; Release 1.0 chỉ đăng nhập username + mật khẩu qua Keycloak (ADR-0063, ADR-0068). Khi đưa lại cần ADR mới; thiết kế cũ: `credential_id`, `user_id`, `tenant_id`, `kind` (`pin`/`qr_card`/`picture_password`), `secret_hash` (argon2id), `status`, đếm sai và khóa tạm.

\* `user_roles`: RLS áp dụng khi `school_id` không null; dòng scope platform/organization chỉ đọc qua role platform. `account_locks`, `email_deliveries`: dòng `tenant_id` NULL (nhân sự platform) chỉ đọc/ghi qua role platform.

**Permission model (ADR-0027, ADR-0066) — lựa chọn lưu trữ.** **Permission nằm trong code** (gắn chặt với handler, version cùng release); bảng `permissions` chỉ là bản sao đọc-chỉ để làm khóa ngoại và trigger. **Role và UserRole nằm trong DB** (`roles`, `role_permissions`, `user_roles`) để quản lý trên Admin Portal (ADR-0066): role hệ thống seed sẵn (`is_system`, không sửa/xóa), role tùy biến của tenant [Confirmed] chỉ gộp permission `scope_kind = tenant`. Bất biến "role tenant không chứa `game.*`/`competition.config.*`/`platform.*`" được giữ ở 3 lớp: `ValidateCatalog` lúc khởi động (role seed), use case `CreateRole`/`UpdateRole`, trigger `role_permissions_scope_chk`. Thay đổi role/UserRole phát `role.updated`/`user_role.changed` qua outbox để xóa cache Principal ([VALKEY_DESIGN](VALKEY_DESIGN.md)).

### 2.2. `org` · DB `eduworld_identity`

| Bảng | Loại | Cột chính | Ràng buộc / index |
|---|---|---|---|
| `schools` | T | `school_id`, `tenant_id`, `code`, `name`, `name_i18n jsonb` (`{vi,en}`, ADR-0056), `country_code varchar(2)` (FK `catalog.countries`), `province_id` (FK `catalog.provinces`, chỉ đơn vị `active` khi tạo/sửa), `moet_code varchar(32)` (mã trường ngành giáo dục, tùy chọn), `school_level varchar(64)` (`primary`/`lower_secondary`/`multi_level`), `grade_min smallint`, `grade_max smallint` (CHECK 1–9), `address varchar(500)`, `status` (`active`/`pending_review`/`disabled` — cùng giá trị với contract `School.status`, F010), `is_virtual boolean`, `login_code varchar(16)` (tùy chọn: mã trường để tìm tenant chứa trường), `settings jsonb` (ADR-0060) | UNIQUE `code`; UNIQUE `login_code`; UNIQUE `moet_code` WHERE NOT NULL; idx `(province_id, school_level)`; trường ảo của tenant `common` có `province_id` NULL; CHECK tenant `school` có đúng 1 school, tenant `common` chỉ có school `is_virtual` (kiểm tra ở use case + test). Bảng `org.organizations` trước đây được thay bằng `platform.tenants` loại `unit` (migration expand/contract: tạo tenant từ organization, gán `schools.tenant_id`, rồi bỏ `organization_id`) |
| `academic_years` | T | `academic_year_id`, `school_id`, `name`, `start_date`, `end_date`, `is_current` | UNIQUE `(school_id) WHERE is_current` |
| `classes` | T | `class_id`, `school_id`, `academic_year_id`, `grade_level` 1–9, `name`, `is_virtual boolean` (lớp ảo theo khối của tenant `common`), `status` | UNIQUE `(school_id, academic_year_id, name)`; **UNIQUE `(class_id, school_id)`** (đích của FK ghép từ `enrollments`); idx `(school_id, grade_level)` |
| `students` | T | `student_id` (= user_id), `school_id`, `grade_level`, `student_code`, `full_name` (chỉ hiển thị cho nhân sự nhà trường trong scope), `date_of_birth_enc bytea`, `national_id_hash` (HMAC, tùy chọn, chỉ để đối soát SIS, [DATA_PRIVACY](../10-security/DATA_PRIVACY.md)), `status` | UNIQUE `(school_id, student_code)`. Không có cột consent (pháp lý ngoài version này, ADR-0082) |
| `enrollments` | T | `enrollment_id`, `school_id`, `student_id`, `class_id`, `status`, `start_date`, `end_date` | **UNIQUE `(student_id) WHERE status = 'active'`** (đúng một trường, một lớp — ADR-0060); **FK ghép `(class_id, school_id)` → `classes(class_id, school_id)`** (lớp thuộc đúng trường của enrollment); idx `(class_id, status)` |

### 2.2a. `catalog` — danh mục platform (G, không theo tenant) · DB `eduworld_identity` (ADR-0060)

Dữ liệu seed: [specs/reference-data](../../specs/reference-data/) (kiểm tra bằng `node scripts/specs/validate-reference-data.mjs`). Chỉ System Admin sửa; các module khác nhận qua projection `ref.provinces` (game, learning) khi cần hiển thị.

| Bảng | Loại | Cột chính | Ràng buộc / index |
|---|---|---|---|
| `countries` | G | `country_code varchar(2)` (ISO 3166-1 alpha-2), `name_i18n jsonb` (`{vi,en}`), `default_locale varchar(16)`, `status varchar(64)` (`active`/`inactive`), cột audit chuẩn | PK `country_code`; Release 1.0 chỉ `VN` active |
| `provinces` | G | `province_id uuid`, `country_code`, `code varchar(64)` (mã nội bộ ổn định, vd. `vn.ha-noi`, `vn.old.ha-giang`), `official_code varchar(16)` (mã đơn vị hành chính chính thức — nhập từ danh mục do cơ quan nhà nước ban hành trước khi seed production, NULL tới khi nhập, không tự đặt [Confirmed — owner 2026-11-20]), `name_i18n jsonb`, `short_name_i18n jsonb`, `province_type varchar(64)` (`municipality`/`province`), `effective_from date` (NULL với đơn vị cũ không rõ ngày bắt đầu — F010), `effective_to date`, `status varchar(64)`, cột audit chuẩn | UNIQUE `code`; UNIQUE `(country_code, official_code)` WHERE NOT NULL; idx `(country_code, status)`; CHECK `status='inactive'` ⇔ `effective_to IS NOT NULL` |
| `province_successions` | G | `old_province_id`, `new_province_id`, `effective_date date`, `kind varchar(64)` (`merged`/`unchanged`/`renamed`/`split`) | PK `(old_province_id)` (mỗi đơn vị cũ một kế thừa); FK cả hai cột → `provinces`; dùng để chuyển dữ liệu cũ (vd. trường SIS gửi tỉnh cũ) sang đơn vị hiện hành |

Cấp xã/phường chưa có trong Release 1.0; mở rộng bằng bảng `wards` cùng mô hình hiệu lực + kế thừa [Confirmed].

### 2.3. `sis` · DB `eduworld_identity`

| Bảng | Loại | Cột chính | Ghi chú |
|---|---|---|---|
| `sis_connections` | T | `sis_connection_id`, `tenant_id` (connection thuộc một tenant; tenant `unit` có thể có một connection cho nhiều trường — danh sách trường nằm ở bảng nối `sis_connection_schools`, không có cột mảng), `provider_kind` (`contract_v1` \| `csv_import`), `contract_version` (vd. `1.0.0`), `config jsonb` (`base_url`, `auth_type`, `token_url`, `oauth_scope`, `schools[]` = `{school_external_id, school_id}` — không secret, **không field-mapping**), `secret_ref`, `webhook_secret_ref` (F017), `capabilities jsonb` (F017, mảng ≤ 4 từ `getHealth`), `last_health_at`, `last_success_at`, `consecutive_failed_runs` (circuit breaker `sis_circuit_breaker`), `status_reason` (`version_mismatch`/`violation_threshold`/`credential_rejected`/`secret_unavailable`/`circuit_open`/`manual`), `status` (`active`/`auth_error`/`contract_error`/`paused`), `delta_watermark` (chỉ ghi sau khi mọi lô apply thành công), `schedule` (cron 5 trường ICT) · index `(status, provider_kind)` cho scheduler | credential trong secret manager; DTO theo [EduWorld SIS Contract v1](../../specs/sis/eduworld-sis-contract.v1.yaml) (ADR-0021) |
| `sync_runs` | T | `sync_run_id`, `tenant_id`, `sis_connection_id`, `school_id` (run `csv_import` theo trường), `mode` (`full`/`delta`/`webhook`/`csv_import`), `trigger`, `dry_run`, `status` (`queued`/`running`/`succeeded`/`partially_failed`/`failed`), `failure_code`, `stats jsonb`, `started_at`, `finished_at`, `requested_by` (F017, người kích hoạt run thủ công), `delivery_id` (F017, chỉ `mode = webhook`) | T (F011): run thuộc connection của một tenant; System Admin đọc qua bypass `system_admin_cross_tenant` rồi chỉ đọc tenant đó. Tra cứu SIS khi đăng nhập áp kết quả bằng run một bản ghi `mode = delta` [Confirmed]. Payload staging của mọi run (CSV, pull, webhook) được ẩn danh sau `sis_staging_retention` (30 ngày, job `sis-staging-anonymizer`) |
| `staging_records` | G, P | `sync_run_id`, `entity_kind`, `external_id`, `row_no`, `payload jsonb` (đã chuẩn hóa, không cột ngoài contract), `raw_hash` (SHA-256 chuẩn, bỏ `updated_at`), `action` (create/update/noop/deactivate), `status` | staging append-only (chỉ REVOKE DELETE/TRUNCATE); job `sis-staging-anonymizer` đặt `payload = NULL` sau `sis_staging_retention` (30 ngày) |
| `sync_record_errors` | G, P | `sync_run_id`, `entity_kind`, `external_id`, `row_no`, `column_name`, `error_code` (vd. `SIS_CONTRACT_VIOLATION`), `message` (không chứa giá trị thô), `occurred_at` | retention 90 ngày |
| `sis_imports` | T | `import_id`, `school_id`, `requested_by`, `storage_key` (`sis-imports/<tenant_id>/<import_id>.csv`, bucket private `ew-sis-imports`), `file_sha256`, `size_bytes`, `entity_kinds jsonb` (một loại [Confirmed]), `status` (`awaiting_upload` → `queued`/`validating`/`applying` → `completed`/`completed_with_issues`/`failed`/`expired`), `dry_run` (mặc định `true`), `sync_run_id`, `stats jsonb` (`diff_*`, `rows_*`, `issues`), `created_at`, `idempotency_key` | Phục vụ `POST /api/v1/schools/{school_id}/sis-imports`; watcher HEAD object phát hiện upload; file xóa khỏi object storage sau 30 ngày [Confirmed] |
| `sis_links` | T | `sis_link_id`, `entity_type`, `entity_id`, `sis_connection_id`, `external_id`, `school_id`, `last_hash`, `last_synced_at` | PK surrogate; UNIQUE `(sis_connection_id, entity_type, external_id) WHERE deleted_at IS NULL`; dùng cho school/class/enrollment |
| `sis_connection_schools` | T | `sis_connection_id`, `school_id`, `tenant_id`, `active` | **UNIQUE một connection active/trường** (ADR-0046, DDL §3.14) |
| `synced_students` | T | `synced_student_id`, khóa nguồn `(sis_connection_id, student_external_id)`, `school_id`, `student_code`, `record_status`, `national_student_id_hash` (HMAC-SHA256 có khóa `EW_SIS_ID_HASH_KEY` [Confirmed]), `last_hash`, `last_synced_at`, `missing_runs` | PK surrogate + UNIQUE khóa nguồn `WHERE deleted_at IS NULL`; `student_external_id` chỉ duy nhất trong một SIS (ADR-0046); `missing_runs` đếm số import đầy đủ liên tiếp vắng mặt (`missing_in_source`) |
| `student_login_keys` | T | `sis_connection_id`, `login_type` (`username`/`idp_subject`), `idp_alias`, `login_key`, `student_external_id` | UNIQUE `(sis_connection_id, login_type, idp_alias, login_key)` (ADR-0046) |
| `reconciliation_issues` | T | `issue_id`, `school_id`, `sis_connection_id`, `sync_run_id` (NULL với issue của provisioning), `issue_type` (conflict matrix SIS_INTEGRATION §6.3), `entity_refs jsonb` (chỉ ID nội bộ/external), `suggested_action`, `status` (`open`/`resolved`/`ignored`), `resolution` (`link`/`same_person`/`ignore`/`fixed_in_source`), `resolution_note`, `resolved_by`, `resolved_at` | CHECK `status = 'open'` ⇔ `resolved_at IS NULL` |

Tên bảng/cột theo [SIS_INTEGRATION §6.1](../09-identity/SIS_INTEGRATION.md); ở DB, khóa ngoại thống nhất hậu tố đầy đủ (`sis_connection_id`, `sync_run_id`) thay cho `connection_id`, `run_id` trong tài liệu domain.

### 2.4. `curriculum` (H — nội dung có `owner_scope`; danh mục G) · DB `eduworld_learning`

DDL tạo ở F021 (`src/backend/modules/learning/migrations/20261124000001_f021_curriculum.sql`). Mọi bảng thuộc một CurriculumVersion mang thêm `curriculum_id` để hiển thị cho tenant theo assignment của **curriculum** (`entx.HybridContent{ResourceIDColumn: curriculum_id}`, policy RLS H `resource_type = 'curriculum'`). Mảng id lưu `jsonb` (CHECK độ dài) vì ent không có kiểu mảng PostgreSQL [Confirmed, F021 Clarifications 2026-11-19].

| Bảng | Loại | Cột chính | Ràng buộc / index |
|---|---|---|---|
| `curricula` | H | `curriculum_id`, `code varchar(64)`, `name jsonb` (`LearningName`), `framework varchar(64)`, `current_version_id`, `latest_version_no`, `cloned_*` (mixin `ContentItemMixin`) | CHECK `code ~ '^[A-Z][A-Z0-9_]{1,63}$'`, `octet_length(name) ≤ 2048`; UNIQUE `(coalesce(tenant_id, '0…0'), code) WHERE deleted_at IS NULL` |
| `curriculum_versions` | H | `curriculum_version_id`, `curriculum_id`, `version_no`, `version_label varchar(16)` (`MAJOR.MINOR`), `status`, `effective_from/to date`, `supersedes_version_id`, `based_on_version_id`, `author_id`, `reviewer_id`, `published_at`, `change_note varchar(1000)`, `content_language`, `cloned_*` | UNIQUE `(curriculum_id, version_no)`, `(curriculum_id, version_label) WHERE deleted_at IS NULL`; CHECK trạng thái, nhãn, `effective_to ≥ effective_from`; trigger `curriculum_versions_immutable` (`content.forbid_published_update()`) |
| `subjects` | G | `subject_id`, `code varchar(64)` (`lower_snake`), `name jsonb`, `grade_levels jsonb` (1–9 phần tử, giá trị 1–9), `icon_key` | UNIQUE `code WHERE deleted_at IS NULL`; chỉ `learning_platform` ghi |
| `subject_grades` | H | `subject_grade_id`, `curriculum_id`, `curriculum_version_id`, `subject_id`, `grade_level smallint`, `periods_per_year smallint` | UNIQUE `(curriculum_version_id, subject_id, grade_level) WHERE deleted_at IS NULL` |
| `topics` | H | `topic_id`, `curriculum_id`, `curriculum_version_id`, `subject_id`, `grade_level`, `parent_topic_id` (FK self), `code varchar(64)`, `name varchar(255)`, `sort_order` | UNIQUE `(curriculum_version_id, code) WHERE deleted_at IS NULL`; idx `(curriculum_version_id, subject_id, grade_level, sort_order)` |
| `learning_objectives` | H | `objective_id`, `curriculum_id`, `curriculum_version_id`, `topic_id`, `code varchar(64)`, `stable_key varchar(64)`, `description varchar(1000)`, `bloom_level`, `yccd_ref varchar(64) NULL`, `competency_ids jsonb`, `prerequisite_ids jsonb`, `sort_order` | UNIQUE `(curriculum_version_id, code) WHERE deleted_at IS NULL`; CHECK mã LO ([CURRICULUM_MODEL §4](../02-learning/CURRICULUM_MODEL.md)), mảng ≤ 20; GIN `prerequisite_ids jsonb_path_ops` |
| `competencies` | H | `competency_id`, `curriculum_id`, `curriculum_version_id`, `code`, `name jsonb`, `kind` (`general_competency`/`subject_competency`/`quality`), `subject_id` | UNIQUE `(curriculum_version_id, code) WHERE deleted_at IS NULL`; CHECK `(kind = 'subject_competency') = (subject_id IS NOT NULL)` |
| `lo_version_mappings`, `textbook_*` | | §2.16 | |

Bảng con của version (`subject_grades`, `topics`, `learning_objectives`, `competencies`, `lo_version_mappings`) có trigger `curriculum.forbid_child_change_when_published()`: INSERT/UPDATE chỉ khi version cha `draft` (`CONTENT_NOT_EDITABLE`). `learning_units`, `lessons`, `learning_activities` thuộc F024/F030.

### 2.5. `content` · DB `eduworld_learning`

| Bảng | Cột chính | Ràng buộc / index |
|---|---|---|
| `questions` | `question_id`, `owner_scope`, `tenant_id`, `current_version_id` (release pointer, ADR-0043), `latest_version_no`, `created_by` | idx `(owner_scope, tenant_id)`; trigger kiểm tra con trỏ trỏ version `published` của chính item (§3.13) |
| `question_versions` | `question_version_id`, `question_id`, `version_no`, `status`, `type`, `stem jsonb`, `render_payload jsonb` (không có `feedback`/`misconception_tag`), `answer_key jsonb`, `scoring_rule jsonb`, `explanation jsonb`, `option_feedback jsonb` (phần chỉ server tách khỏi `render_payload.options[]` khi lưu, F022), `hints jsonb` (mảng ≤ 5), `worked_example_ref jsonb` (`{content_item_id, block_id}` theo [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md), ≤ 1 KB), `min_response_ms int` (0–600000), `max_score numeric(7,2)` (> 0, mặc định 1), `math_assets jsonb` (≤ 50), `exam_only`, `efficacy_instrument`, `tags jsonb` (≤ 20), `difficulty` 1–5, `grade_levels jsonb` (mảng 1–9 phần tử, giá trị 1–9), `subject_id`, `topic_id` (NOT NULL), `media_refs jsonb` (mảng uuid ≤ 50), `content_hash bytea` (SHA-256), cột mixin version (F020) | H (assignment theo `question_id`); UNIQUE `(question_id, version_no)`; UNIQUE `(question_id) WHERE status IN ('draft','in_review') AND deleted_at IS NULL` (một nháp mở); idx `(tenant_id, subject_id, topic_id, difficulty) WHERE status='published'` + GIN `grade_levels jsonb_path_ops`; FK tới `curriculum.subjects/topics/curriculum_versions`; trigger `question_versions_immutable`; DDL §3.3 |
| `question_version_objectives` | `question_version_objective_id`, `tenant_id`, `owner_scope`, `question_id`, `question_version_id`, `objective_id` | H (assignment theo `question_id`); UNIQUE `(question_version_id, objective_id) WHERE deleted_at IS NULL`; idx `objective_id`; trigger chỉ cho ghi khi version là `draft` (F022) |
| `question_sets`, `question_set_versions`, `question_set_items` | H (F024): bảng version chung cột với lesson/material/blueprint — `title`, `summary` jsonb, `subject_id`, `curriculum_version_id`, `grade_levels`/`topic_ids`/`objective_ids`/`media_refs`/`worked_example_ids` jsonb, `body` jsonb (≤ 512 KB; set: `definition` {kind, purpose, items, selection_rules…} + `default_feedback_timing`), `search_tsv` sinh (`content.f_search_text` + `unaccent`, GIN); `question_set_items` khóa thay thế `question_set_item_id` + `question_id`, `question_version_id` (FK), `position` 1–200, `max_score`, chỉ sửa khi version `draft` | RLS H theo assignment `question_set`; trigger bất biến + con trỏ release |
| `assessment_blueprints`, `assessment_blueprint_versions` | H (F024): cột chung như trên; `body` = `definition` (duration_sec, sections, total_points, attempt_policy, result_release, variants + balance mặc định `variant_balance_max_mean_p_diff`, grading_scale…), `exam_type`, `feedback_timing` (≠ `immediate`), `result_release`, `efficacy_instrument` | thay `sections`/`scoring`/`shuffle_policy`/`result_visibility` riêng bằng `body` [Confirmed, OQ F024-1] |
| `lessons`, `lesson_versions`, `learning_materials`, `learning_material_versions` | H (F024): cột chung như trên; `body` = `blocks` (1–100 khối `rich_text`/`media`/`worked_example`/`activity` — lesson.schema.json) + `completion_rule`, `estimated_minutes` (lesson) hoặc `material_kind` (material); `worked_example_ids` (GIN) để kiểm `worked_example_ref` | RLS H theo assignment `lesson` / `learning_material` |
| `media_assets`, `media_usages` | metadata file trên object storage (ADR-0044): `kind`, `mime_type`, `size_bytes`, `sha256`, `bucket`, `variants jsonb`, `alt_text`, `license`, `status`, `quarantine_key`, `is_exam_material`, `upload_completed_at`, `processed_at` (F027); `media_usages` có `tenant_id`, `item_id`, `version_no` | H (F027): DDL §3.13; file **không** lưu trong DB; RLS theo assignment `resource_type = media` |
| `scheduled_publications` | `schedule_id` (trả về như `release_id` trong `202 ContentRelease`), `tenant_id` (NULL = platform), `item_type`, `item_id`, `version_id`, `publish_at`, `status` (`scheduled`/`applied`/`cancelled`/`failed`), `failure_code`, `applied_release_id`, `change_note varchar(500)`, `requested_by`, `cancelled_by`, `idempotency_key` | Lịch hẹn publish; worker áp đúng giờ và ghi `content_releases`. UNIQUE `(item_type, item_id) WHERE status = 'scheduled' AND deleted_at IS NULL` (một lịch chờ/item), UNIQUE `(item_type, item_id, idempotency_key)`; idx `(status, publish_at) WHERE status = 'scheduled'` |
| `import_jobs` | `import_id`, `tenant_id` (NULL = platform), `owner_scope`, `kind` (`curriculum_yaml`/`curriculum_csv`, F024 mở rộng), `asset_id`, `source_ref varchar(512)`, `dry_run`, `status` (`queued`/`running`/`succeeded`/`failed`), `created_count`, `rejected_count`, `errors jsonb` (≤ 500 dòng, ≤ 256 KB), `result_refs jsonb` (≤ 64 KB), `idempotency_key`, `finished_at` | T* (F021): UNIQUE `(created_by, idempotency_key)`; idx `(status, created_at)` |
| `content_releases` | lịch sử con trỏ: `action` publish/rollback/unpublish, `from/to_version_id`, `reason`, `released_by` | TA\* (append-only, `tenant_id` NULL = platform); DDL §3.13 |
| `learning.lesson_sessions` | `lesson_session_id`, `tenant_id`, `school_id`, `student_id`, `lesson_id`, `lesson_version_id` (ghim), `lesson_version`, `status` (`in_progress`/`completed`), `context_mode`, `assignment_id`, `learning_path_id`, `recommendation_id`, `completed_activity_ids uuid[]`, `completed_attempt_ids uuid[]` (≤ 50), `active_ms`, `started_at`, `ended_at`, `version` | ghim version bài học (ADR-0043); UNIQUE `(tenant_id, student_id, lesson_id) WHERE status = in_progress`, UNIQUE `(tenant_id, student_id, lesson_version_id) WHERE status = completed` (F031) |
| `review_tasks` | `task_id`, `tenant_id` (NULL ⇔ lane `platform_learning`), `review_lane varchar(64)` (`tenant`/`platform_learning`), `content_type`, `content_id`, `content_version_id`, `version_no`, `title varchar(255)`, `school_id`, `author_id`, `assignee_id`, `status` (`open`/`approved`/`rejected`/`cancelled`), `comments jsonb` (≤ 64 KB; nhận xét theo trường khi `rejected`, F025 T046), `submitted_at`, `due_at` (`content_review_sla_days`), `resolved_at`, `resolved_by` | T\* (mixin `TenantOptional`, F025 [Confirmed: T\* thay vì H — task không được gán cho tenant]): duyệt nội dung (workflow biên tập), không phải chấm bài; partial unique `(content_version_id) WHERE status = 'open'` |
| `content_transitions` | `transition_id`, `tenant_id` (NULL = platform), `content_type varchar(64)` (`LearningContentType` + `curriculum`), `content_version_id`, `from_status` (NULL khi tạo), `to_status varchar(64)`, `actor_id`, `at`, `note varchar(2000)`; F025 thêm `content_id`, `action varchar(64)` (`submit_review`/`approve`/`reject`/`revise`/`publish`/`archive`/`rollback`/`unpublish`), `idempotency_key varchar(64)` (NULL với dòng trước F025); F025 T046 thêm `field_comments jsonb` (chỉ `reject`, mảng 1–50 `{path, comment}`, ≤ 64 KB — OQ F054-1) | TA\* (append-only, audit trail CMS; mixin `TenantOptionalAppendOnly`, CHECK trạng thái, idx `content_version_id`, unique `(content_version_id, idempotency_key)` khi có key; tạo ở F020) |
| `translation_jobs` | `job_id`, `tenant_id` (NULL = platform), `owner_scope`, `kind` (`export`/`import`), `status` (`queued`/`running`/`succeeded`/`failed`), `format` (`xliff`/`csv`), `source_language`, `target_language`, `request jsonb`, `input_asset_id`, `result_storage_key varchar(512)` (`translations/<job_id>.<ext>` trong `ew-exports`), `report jsonb` (≤ 64 KB, `errors` ≤ 500), `requested_by`, `finished_at` | T\* (F025 US5, job `translation-jobs`) |
| `scheduled_publications` | `schedule_id`, `tenant_id` (NULL = platform), `item_type`, `item_id`, `version_id`, `version_no`, `publish_at`, `status` (`scheduled`/`applied`/`cancelled`/`failed`), `failure_code varchar(64)`, `applied_release_id`, `change_note varchar(500)`, `requested_by`, `cancelled_by`, `idempotency_key varchar(64)` | T\* (F026 US4, job `content-scheduled-publish`): unique `(item_type, item_id, idempotency_key)`, một lịch chờ mỗi item (partial unique `WHERE status = 'scheduled'`); `content_releases` chỉ ghi khi con trỏ đổi. F026 thêm cột `published_by` vào các bảng `*_versions` |
| Cột F025 của bảng `*_versions` nội dung học tập (`question_versions`, `question_set_versions`, `lesson_versions`, `learning_material_versions`, `assessment_blueprint_versions`; `activity_versions` ở F030) | `translation_status varchar(64)` (`complete`/`partial`/`outdated`, tính khi lưu), `translation_meta jsonb` (`{pointer: {source_sha256, translated_at}}`, ≤ 64 KB), `audio_exempt_reason varchar(255)`, `audio_exempt_approved_by` | Mixin `WorkflowFields` (CONTENT_MODEL §8, §8.1); cột audio có trên cả 5 bảng để repository sinh từ `doc_lesson.go` đồng nhất [Confirmed] |
| (mixin version, F020) | mọi bảng version nội dung dùng `ContentVersionMixin` của module learning: `version_no`, `status`, `author_id`, `reviewer_id`, `published_at`, `content_language varchar(64)` (`vi`/`en`, mặc định `vi`, ADR-0056), `curriculum_version_id` (NOT NULL trừ media), `change_note`, `publish_at`, `snapshot_key`, `snapshot_sha256`, `based_on_version_id`, `cloned_*`; item dùng `ContentItemMixin` (`current_version_id`, `latest_version_no`, `cloned_*`) | Bảng version là H, hiển thị cho tenant theo assignment của **item** (`entx.HybridContent.ResourceIDColumn`); trigger `content.forbid_published_update()` (tạo ở F020) gắn cho từng bảng version ở F021/F022/F024 |

`answer_key` **không bao giờ** được select bởi query phục vụ client học sinh: query delivery dùng view `content.question_delivery_v` loại bỏ cột này (ADR-0016).

### 2.6. `assessment` (T) · DB `eduworld_learning`

| Bảng | Cột chính | Ràng buộc / index |
|---|---|---|
| `attempts` | `attempt_id`, `school_id`, `student_id`, `mode`, `context_type`, `context_id`, `activity_id` (NULL với ad-hoc), `activity_type`, `source_kind` (`activity`/`question_set`/`selection`, F030 [Confirmed]), `source_version_id`, `status`, `seed`, `started_at`, `deadline_at`, `submitted_at`, `graded_at`, `paused_at`, `paused_ms`, `last_activity_at`, `feedback_timing`, `display_language`, `submit_reason`, `score numeric(7,2)`, `max_score`, `duration_ms`, `client_elapsed_ms`, `item_count`, `objective_ids jsonb` (≤ 50), `recommendation_id`, `idempotency_key varchar(64)`, `version` (F030) | idx `(student_id, started_at DESC)`; idx `(context_type, context_id)`; partial UNIQUE `(student_id, context_type, context_id) WHERE status='in_progress' AND mode IN ('exam','assignment')` (một attempt đang mở); UNIQUE `(student_id, idempotency_key)`; partial UNIQUE `(student_id, source_version_id) WHERE status='in_progress' AND mode='practice'` (F030-AC-04); idx `(status, deadline_at)`, `(status, last_activity_at)` WHERE `in_progress` (job `attempt-lifecycle`) |
| `attempt_items` | `attempt_item_id` (khóa thay thế, F030), `attempt_id`, `position`, `question_version_id`, `question_type`, `objective_ids jsonb` (snapshot ≤ 50), `presentation jsonb` (thứ tự đã xáo, tham số; ≤ 16 KB), `presented_payload_hash`, `max_score numeric(6,2)`, `answer_count`, `final_is_correct`, `final_score`, `hints_revealed` | UNIQUE `(attempt_id, position)` |
| `attempt_answers` (P) | `attempt_answer_id`, `attempt_id`, `position`, `school_id`, `question_version_id`, `response jsonb` (≤ 32 KB), `is_correct`, `score`, `answered_at`, `client_elapsed_ms`, `response_ms`, `attempt_no`, `is_scored`, `hints_used`, `rapid_guess`, `idempotency_key varchar(64)` | append-only (role chỉ INSERT/SELECT), partition tháng; idx `(attempt_id, position, answered_at DESC)`; DDL §3.5 |
| `exam_sessions` | `exam_session_id`, `school_id`, `blueprint_version_id` (ghim lúc tạo, ADR-0043), `title varchar(255)`, `subject_id`, `grade_level`, `exam_type` (`practice_test`/`mock`/`chapter`/`semester`/`subject`/`custom`, [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md)), `is_official`, `window_start`, `window_end`, `duration_sec int`, `status` (`scheduled`/`open`/`closed`/`results_released`/`cancelled`), `target jsonb` (≤ 64 KB), `created_by` | CHECK `window_end > window_start`, `duration_sec > 0` |
| `exam_papers` | `exam_paper_id`, `exam_session_id`, `seed`, `items jsonb` | đề đã sinh (phục vụ truy vết) |

### 2.6a. Activity Engine, video/audio/mô phỏng (ADR-0048, ADR-0052, ADR-0055) · DB `eduworld_learning`

Mọi bảng có cột chung `tenant_id`, `created_at/by`, `updated_at/by`, `deleted_at/by`, `version` (ADR-0028/0039); chuỗi là `varchar(n)` (ADR-0041). Release 1.0 chỉ có hoạt động server tự chấm (ADR-0072).

| Bảng (schema) | Loại | Cột chính | Ràng buộc / ghi chú |
|---|---|---|---|
| `content.activities` | H | `activity_id`, `owner_scope`, `current_version_id` (ADR-0043) — F030: loại nội dung ghép như F024 (bảng item + version cột chung); `activity_type` nằm trong `body` của version | assignment `resource_type = activity` [Confirmed] |
| `content.activity_versions` | H | cột chung bảng version F024 (`title`, `summary` = description, `subject_id`, `grade_levels`, `topic_ids`, `objective_ids` ≤ 20, `media_refs`, `body`) + `ContentVersionMixin`, `WorkflowFields`; `body` = `activity_type`, `instructions`, `difficulty`, `time_limit_seconds`, `reward_rule_id`, `config` (content/interaction/evaluation/feedback/completion theo [schema loại](../../specs/schemas/activities/activity-base.schema.json)), `math_assets` — CHECK ≤ 256 KB; cột sinh `activity_type varchar(32)` (CHECK `ActivityType`) | UNIQUE `(activity_id, version_no)`; trigger `forbid_published_update`, release target; F030 [Confirmed: H thay vì G — nội dung tenant và platform gán như lesson] |
| `content.simulation_templates` | G | `template_code varchar(64)`, `template_version`, `simulation_kind varchar(32)`, `parameter_schema jsonb`, `checker_available bool`, `status` | PK `(template_code, template_version)`; platform đăng ký |
| `content.math_svgs` | G | `sha256 char(64)` PK, `storage_key varchar(128)` (`math/<sha256>.svg`), `latex varchar(2000)`, `speech_text varchar(1000)`, `size_bytes`, `sanitized_at` | SVG do portal render (MathJax 4), server sanitize; bất biến (ADR-0055) |
| `assessment.activity_attempt_responses` | P | `response_id`, `attempt_id`, `part_id varchar(64)` (bước tương tác, nhánh, mục tiêu — không phải câu hỏi: câu hỏi chèn trong video/audio là item của `attempt_items` + `attempt_answers`, mỗi cue một item theo `at_ms`), `response jsonb` (≤ 32 KB), `result jsonb`, `answered_at`, `client_seq` | append-only; UNIQUE `(attempt_id, part_id, client_seq, answered_at)` |
| `assessment.evaluations` | T | `evaluation_id`, `attempt_id`, `source varchar(16)` (`system`/`re_evaluation`), `regrade_request_id uuid null`, `supersedes_evaluation_id uuid null`, `details jsonb` (điểm từng item sau chấm lại, ≤ 64 KB), `status varchar(16)`, `score numeric(7,2)`, `max_score`, `passed`, `feedback varchar(4000)`, `triggered_by uuid null` (người kích hoạt re-evaluate: `content_admin`/`academic_manager`), `reason varchar(500) null`, `effective bool` | UNIQUE partial `(attempt_id) WHERE effective AND deleted_at IS NULL`; re-evaluate tạo dòng mới, phát `attempt.re_evaluated`, có audit |
| `assessment.video_progress` | T | `attempt_id` PK, `school_id`, `position_ms bigint`, `watched_ranges jsonb` (≤ 64 khoảng), `watched_ratio numeric(5,4)`, `last_heartbeat_at`, `client_seq`, `milestones_emitted smallint` (bitmask), `processed_cue_ids varchar(64)[]` (≤ 30), `segment_id varchar(64)`, `reached_ending` | Dùng chung cho audio; migration F065 `20261215000001` |
| `assessment.video_branch_choices` | T | `choice_id` PK, `attempt_id`, `branch_point_id varchar(64)`, `option_id varchar(64)`, `target_segment_id varchar(64)`, `chosen_at` | partial UNIQUE `(attempt_id, branch_point_id) WHERE deleted_at IS NULL` (F065) |
| `assessment.simulation_runs` | T | `run_id`, `attempt_id`, `template_code varchar(64)`, `template_version`, `seed bigint`, `action_log jsonb` (CHECK ≤ 64 KB), `client_outcome jsonb`, `verified_outcome jsonb`, `goals_met jsonb`, `verification_status varchar(16)` | Idempotency-Key |

Tự luận, bài nói, dự án, rubric và hàng đợi chấm ngoài version này (ADR-0082): không có bảng.

### 2.7. `learning` (T) · DB `eduworld_learning`

| Bảng | Loại | Cột chính | Ghi chú |
|---|---|---|---|
| `learning_events` | **P** | xem DDL §3.6 (+ `actor_type` `student`/`system`/`staff`, F030) | LRS nội bộ; UNIQUE idempotency theo partition; ghi ở F030 (`activity.*`, `question.answered`) và F031 (`learning_objective.*`), kiểm `learning-event.schema.json` trước khi ghi; role app/platform chỉ `SELECT, INSERT`, kể cả trên partition (F031 thu hồi quyền trực tiếp trên partition); partition tháng `learning_events_YYYY_MM` do job `learning-events-partitions` (role `learning_partition_admin`, hàm `learning.ensure_monthly_partitions`) giữ tháng hiện tại + 3 tháng (F031-AC-04) |
| `mastery_model_configs` | G | `config_id`, `model_version varchar(64)` (UNIQUE live), `algorithm` (`weighted_evidence_ewma`), `params jsonb` (≤ 16 KB, tham số LEARNING_ANALYTICS §3.1), `status` (`draft`/`shadow`/`active`/`retired`), `activated_at`, `activated_by` | partial UNIQUE một dòng `active`; migration F031 seed `we-1.0` `active`; role app chỉ đọc (F031) |
| `objective_masteries` | T | `objective_mastery_id` PK, `tenant_id`, `school_id`, `student_id`, `objective_id`, `value numeric(4,3)` (giá trị gốc, chưa decay), `level` (lúc cập nhật), `evidence_count`, `distinct_days`, `last_evidence_at`, `last_evidence_date date`, `first_mastered_at` (phát `learning_objective.mastered` một lần), `model_version`, `prior_source` (`default`/`diagnostic`/`propagated`), `prior_value numeric(4,3)` (M0 của bài chẩn đoán, F032), `prior_diagnostic_attempt_id`, `version` (khóa lạc quan) | UNIQUE `(tenant_id, student_id, objective_id) WHERE deleted_at IS NULL` (khóa tự nhiên thay PK `(student_id, objective_id)` cũ, F031); CHECK `prior_source = default OR prior_value IS NOT NULL`; bài chẩn đoán chỉ ghi prior khi `evidence_count = 0` (F032) |
| `wrong_question_entries` | T | `entry_id`, `tenant_id`, `school_id`, `student_id`, `question_id`, `question_version_id`, `subject_id`, `objective_ids uuid[]` (≤ 20), `question_type`, `difficulty`, `first_wrong_attempt_id`, `wrong_count`, `last_wrong_at`, `stage` (0–2), `next_due_at`, `correct_streak` (0–2), `last_correct_on date`, `status` (`active`/`cleared`), `retry_attempt_id`, `retry_position`, `cleared_at`, `version` | danh sách ôn câu sai lượt đầu (lịch `retry_wrong_schedule`, LEARNING_EXPERIENCE §6.1); partial UNIQUE `(tenant_id, student_id, question_id) WHERE deleted_at IS NULL AND status = active`; rời danh sách = `cleared`, không xóa; ghi trong transaction trả lời câu (F032) |
| `student_merge_links` | T | `merge_id` (PK), `tenant_id`, `source_student_id`, `target_student_id` (khác nguồn), `reason` (3–1000), `created_by`, `created_at`, `version` | liên kết `student_id` gốc → user đích khi gộp tài khoản (IDENTITY_MAPPING §7): attempt, learning record, mastery giữ theo `student_id` gốc; partial UNIQUE `(tenant_id, source_student_id) WHERE deleted_at IS NULL`; ghi bởi bước `mergeUserData` (F030 T056) |
| `rm_student_recommendations` | T | `recommendation_id`, `tenant_id`, `school_id`, `student_id`, `rule_code` (`R1`…`R9`), `kind`, `target_type`, `target_id`, `subject_id`, `grade_level`, `objective_ids uuid[]` (≤ 10), `reason`, `reason_code`, `reason_params jsonb` (≤ 2 KB), `priority` (1–9), `status` (`active`/`started`/`completed`/`dismissed`/`expired`/`superseded`), `skip_count`, `started_attempt_id`, `generated_at`, `expires_at`, `closed_at`, `version` | read model rule engine (LEARNING_ANALYTICS §6); partial UNIQUE `(tenant_id, student_id, rule_code, target_type, target_id) WHERE deleted_at IS NULL AND status = active`; tính lại trong transaction consumer `learning.mastery_on_graded` và job `recommendation-nightly` (F032) |
| `mastery_history` | TA, P (tháng) | `history_id`, `tenant_id`, `student_id`, `objective_id`, `value_before`, `value_after`, `level_before`, `level_after`, `evidence_count`, `source_event_id`, `source_kind` (`evidence`/`re_evaluation`/`recompute`), `model_version`, `occurred_at` | append-only (role app/platform `SELECT, INSERT`); phục vụ biểu đồ tiến bộ (F031) |
| `mastery_recompute_runs` | G | `run_id`, `model_version`, `status` (`running`/`completed`/`failed`/`activated`), `report jsonb` (≤ 64 KB), `started_by`, `started_at`, `finished_at`, `activated_at`, `activated_by` | CLI vận hành F031 US5; role app không đọc |
| `objective_mastery_shadows` | T | `run_id`, `tenant_id`, `school_id`, `student_id`, `objective_id`, `value`, `level`, `current_level`, `evidence_count`, `distinct_days`, `last_evidence_at`, `first_mastered_at`, `model_version` | PK `(run_id, tenant_id, student_id, objective_id)`; không bao giờ đọc bởi sản phẩm (F031 US5) |
| `progress_daily` | T, P (tháng) | `student_id`, `class_id` (snapshot lúc event), `subject_id`, `school_id`, `bucket_date`, `metric_version`, `activities_completed`, `items_answered`, `items_correct`, `items_rapid_guess`, `items_with_hint`, `active_seconds`, `lessons_completed`, `is_final`, `computed_at` | cập nhật bởi consumer `learning.progress_daily` (F031: `activity.graded` → `activities_completed` khi mode ≠ exam, `items_*` theo lần trả lời có tính điểm đầu tiên, `active_seconds`; `bucket_date` theo giờ Việt Nam; `subject_id` = môn của activity, UUID 0 khi không có môn [Confirmed]); PK `(tenant_id, student_id, subject_id, bucket_date, metric_version)`; không `deleted_at` (dữ liệu dẫn xuất) |
| Rollup `rollup_class_*`, `rollup_class_objective_weekly`, `rollup_school_*`, `rollup_tenant_*`; read model `rm_class_objective_stats`, `rm_class_subject_summary`, `rm_school_grade_summary`, `rm_student_attention`, `mastery_snapshots_weekly` | T | Cột chung và khóa: [DASHBOARD_DATA](DASHBOARD_DATA.md) (sở hữu cấu trúc rollup) | School Learning Dashboard; dữ liệu dẫn xuất, dựng lại được |

### 2.8. `game` · DB `eduworld_game`

| Bảng | Loại | Cột chính | Ràng buộc |
|---|---|---|---|
| `game_profiles` | T | `student_id` PK, `school_id`, `nickname varchar(32)` (NULL tới khi chọn), `level`, `xp_total`, `curve_version int`, `title_code varchar(64)`, `avatar_config jsonb`, `avatar_created_at`, `last_area_id`, `settings jsonb`, `last_game_login_at` (ADR-0031) | UNIQUE `(tenant_id, lower(nickname)) WHERE deleted_at IS NULL AND nickname IS NOT NULL` (duy nhất trong tenant) |
| `item_definitions` | G | `item_id varchar(64)` (mã ổn định, không phải UUID; mọi bảng và contract tham chiếu item dùng mã này), `kind`, `rarity`, `metadata jsonb`, `status` | |
| `inventory_items` | T | `student_id`, `item_id varchar(64)`, `school_id`, `acquired_at`, `reward_grant_id` | PK `(student_id, item_id)` |
| `world_areas` | G | `area_id` (text), `scene_ref`, `capacity`, `access_policy jsonb` | |
| `quest_definitions`, `quest_versions` | G (owner_scope) | con trỏ `quest_definitions.current_version_id` (ADR-0043); xem DOMAIN_MODEL §5; `steps jsonb` theo `specs/schemas/quest.schema.json` | UNIQUE `(quest_id, version)` |
| `quest_progress` | T | `quest_progress_id` PK, `student_id`, `quest_id`, `quest_version_id`, `school_id`, `status`, `step_states jsonb`, `party_id`, `period_key varchar(16)` (quest lặp: `2026-11-15`, `2026-W46`), `started_at`, `expires_at`, `completed_at` | UNIQUE `(tenant_id, student_id, quest_id) WHERE status = 'active' AND deleted_at IS NULL` (một lượt đang chạy); UNIQUE `(tenant_id, student_id, quest_id, period_key) WHERE period_key IS NOT NULL AND deleted_at IS NULL` (quest lặp chơi lại mỗi kỳ) |
| `minigame_configs`, `minigame_config_versions` | G (owner_scope) | | |
| `level_definitions`, `level_versions` | G | gói level/zone: `bundle_id`, `bundle_url`, `bundle_sha256`, `platforms` | con trỏ `current_version_id` |
| `level_sessions` | T | ghim version theo màn (ADR-0043) | DDL §3.13; một màn active/học sinh |
| `content_releases`, `media_assets`, `media_usages` | G/T | như `eduworld_learning` | DDL §3.13 |
| `npc_dialogues`, `npc_dialogue_versions` | G | | |
| `game_sessions` | T | `game_session_id`, `tenant_id`, `student_id`, `school_id`, `device_id`, `device_model`, `client_platform`, `client_version`, `started_at`, `last_heartbeat_at`, `ended_at`, `end_reason` (`logout`/`replaced_by_other_device`/`timeout`/`account_locked`/`cleanup`/`client_crash`), `replaced_by_session_id`, `keycloak_sid` | **UNIQUE `(student_id) WHERE ended_at IS NULL AND deleted_at IS NULL`** — một session active/học sinh (ADR-0031); idx `(ended_at IS NULL, last_heartbeat_at)` cho job timeout 45 s. Xem DDL §3.12 |
| `login_events` | TA, P (tháng) | `login_event_id`, `tenant_id`, `student_id`, `game_session_id`, `device_id`, `platform`, `device_model`, `client_version`, `ip_hash` (HMAC, không lưu IP thô), `login_at` (giờ server), `logout_at`, `end_reason` | **Append-only** (chỉ `UPDATE` một lần để ghi `logout_at`/`end_reason`); idx `(tenant_id, student_id, login_at DESC)`; retention theo §5 |

### 2.8a. Progression, challenge/boss, kinh tế, collection, achievement · DB `eduworld_game`

Nguồn: [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md), [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md), [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md), ADR-0049. Mọi bảng T có `tenant_id NOT NULL`, cột audit chuẩn, `deleted_at` (trừ bảng ledger append-only); bảng G là nội dung platform có `*_versions` + con trỏ `current_version_id` (ADR-0043). Chuỗi theo §1.4. Schema: progression, khám phá, challenge, boss, preset avatar nằm trong `game`; tiền tệ, ledger, shop, collection, achievement nằm trong `gamification`.

| Bảng | Loại | Cột chính | Ràng buộc / index |
|---|---|---|---|
| `progression_curves`, `progression_curve_versions` | G | `code varchar(64)`, `max_level int`, `formula jsonb`, `level_overrides jsonb`, `titles jsonb`, `level_floor_protection bool` (payload theo `progression-curve.schema.json`) | UNIQUE `(curve_id, version)` |
| `progression_levels` | G | `curve_version_id`, `level int`, `xp_required int` (XP cần để lên level kế, NULL tại `max_level`; tên theo [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md)), `xp_cumulative bigint`, `title_code varchar(64)`, `rewards jsonb` | PK `(curve_version_id, level)` — sinh khi publish curve; config type `progression-levels` (không dùng `levels`, tránh trùng `game.level_definitions`) |
| `level_history` | T | `student_id`, `from_level`, `to_level`, `curve_version`, `trigger_xp_entry_id`, `reached_at` | idx `(tenant_id, student_id, reached_at DESC)`; append-only |
| `world_area_unlocks` | T | `student_id`, `area_id varchar(64)`, `rule_version int`, `unlocked_at` | UNIQUE `(student_id, area_id)` |
| `exploration_progress` | T | `student_id`, `area_id varchar(64)`, `discoveries varchar(64)[]`, `collectibles varchar(64)[]`, `completion_pct numeric(5,2)`, `updated_at` | PK `(student_id, area_id)`; CHECK `cardinality(discoveries) <= 500` |
| `world_areas`, `world_area_versions` | G | `area_id varchar(64)`, `region_code varchar(64)`, `name_i18n jsonb` (`{vi,en}`, ≤ 1 KB, ADR-0056), `scene_ref varchar(255)`, `capacity int`, `access_policy jsonb`, `unlock_rule jsonb` ([WORLD_DESIGN §3, §7](../01-game-design/WORLD_DESIGN.md)) | UNIQUE `(area_id, version)`; config type `world-areas` |
| `story_chapters`, `story_chapter_versions` | G | `chapter_code varchar(64)`, `region_code varchar(64)`, `title_i18n jsonb` (`{vi,en}`, ADR-0056), `order int`, `main_quest_ids uuid[]`, `boss_id uuid` | UNIQUE `(chapter_code, version)`; config type `story-chapters` |
| `story_progress` | T | `student_id`, `chapter_code varchar(64)`, `chapter_version_id`, `status varchar(32)` (`available/in_progress/completed`), `main_quests_completed int`, `completed_at` | PK `(student_id, chapter_code)`; suy từ quest event, phát `story.chapter_completed` một lần (outbox idempotent) |
| `challenges`, `challenge_versions` | G | `code varchar(64)`, `challenge_type varchar(64)`, `grade_levels int[]`, `subject_id`, `objective_ids uuid[]`, `area_id varchar(64)`, `config jsonb` (`challenge.schema.json`) | UNIQUE `(challenge_id, version)` |
| `challenge_attempts` | T | `challenge_attempt_id`, `student_id`, `challenge_version_id`, `difficulty varchar(64)`, `party_id`, `status varchar(64)`, `score int`, `lives_left int`, `started_at`, `deadline_at`, `ended_at`, `learning_attempt_ids uuid[]` | idx `(tenant_id, student_id, started_at DESC)`; partial idx `(student_id) WHERE status = 'in_progress'` |
| `boss_definitions`, `boss_versions` | G | `code varchar(64)`, `area_id varchar(64)`, `grade_levels int[]`, `mode varchar(64)`, `phases jsonb` (`boss.schema.json`), `rewards jsonb` | |
| `boss_encounters` | T | `encounter_id`, `boss_version_id`, `party_id`, `mode`, `status varchar(64)`, `current_phase varchar(64)`, `energy_remaining numeric(5,4)`, `checkpoint jsonb` (CHECK ≤ 64 KB), `owner_node varchar(128)`, `started_at`, `ended_at` | partial idx `(status) WHERE status IN ('lobby','phase_active')` |
| `boss_encounter_members` | T | `encounter_id`, `student_id`, `joined_at`, `left_at` | PK `(encounter_id, student_id)` |
| `boss_encounter_contributions` | T | `encounter_id`, `student_id`, `phase_id varchar(64)`, `correct_count int`, `activities_completed int`, `contribution_pct numeric(5,2)`, `eligible bool` | UNIQUE `(encounter_id, student_id, phase_id)` |
| `currencies` | G | `currency_code varchar(64)` PK (`xu`), `name_vi varchar(128)`, `daily_cap int`, `status` | |
| `wallets` | T | `student_id`, `currency_code`, `balance bigint`, `version int`, `updated_at` | PK `(student_id, currency_code)`; CHECK `balance >= 0` |
| `currency_ledger` | TA, P (tháng) | `entry_id`, `student_id`, `currency_code`, `delta bigint`, `balance_after bigint`, `reason_code varchar(64)`, `source_type varchar(64)`, `source_id uuid`, `idempotency_key varchar(64)`, `created_by uuid`, `created_at` | UNIQUE `(idempotency_key, created_at)` (giới hạn partition) + kiểm tra ứng dụng; append-only, không `UPDATE`/`DELETE` |
| `item_ledger` | TA, P (tháng) | `entry_id`, `student_id`, `item_id varchar(64)`, `delta int`, `reason_code varchar(64)`, `source_type varchar(64)`, `source_id uuid`, `idempotency_key varchar(64)`, `created_at` | Như `currency_ledger` |
| `shop_items`, `shop_item_versions` | G | `item_id varchar(64)`, `item_kind varchar(64)`, `price_amount int`, `currency_code varchar(64)`, `purchase_limit int`, `available_from`, `available_to`, `grade_levels int[]` (`shop-item.schema.json`) | CHECK `price_amount > 0` |
| `shop_purchases` | T | `purchase_id`, `student_id`, `shop_item_version_id`, `price_amount int`, `currency_ledger_entry_id`, `idempotency_key varchar(64)`, `created_at` | idx `(student_id, shop_item_version_id)` để kiểm tra `purchase_limit` |
| `collections`, `collection_versions`, `collection_items` | G | `code varchar(64)`, `collection_type varchar(64)`, `items jsonb`, `completion_rewards jsonb`, `visibility varchar(64)` | |
| `collection_progress` | T | `student_id`, `collection_id`, `owned_item_ids varchar(64)[]`, `owned_count int`, `completed_at` | PK `(student_id, collection_id)` |
| `achievements`, `achievement_versions` | G | `code varchar(64)`, `category varchar(64)`, `trigger jsonb`, `progress_metric jsonb`, `visibility varchar(64)`, `tiers jsonb` (`achievement.schema.json`) | |
| `student_achievements` | T | `student_id`, `achievement_id`, `tier varchar(64)`, `progress int`, `unlocked_at` | PK `(student_id, achievement_id, tier)` |
| `economy_adjustments` | T | `adjustment_id`, `student_id`, `kind varchar(64)` (`currency`/`item`/`xp`), `currency_code`, `item_id varchar(64)`, `delta bigint` (≠ 0), `reason varchar(1000)` (≥ 10 ký tự), `ticket_ref varchar(128)`, `status varchar(64)` (`pending_approval`/`applied`/`rejected`), `created_by`, `approved_by`, `ledger_entry_id`, `idempotency_key varchar(64)`, `created_at`, `applied_at` | Điều chỉnh kinh tế thủ công (Game Operations Console, permission `game.economy.adjust`): hai người (CHECK `approved_by IS NULL OR approved_by <> created_by`), áp dụng bằng bút toán ledger, không sửa số dư trực tiếp; UNIQUE partial `idempotency_key`; schema `gamification` |
| `preview_sessions` | G | `preview_session_id`, `token_hash varchar(128)` (SHA-256 hex của `preview_token`, không lưu token thô), `config_type varchar(64)`, `config_id`, `config_version_id`, `created_by`, `expires_at` (= `created_at` + `preview_token_ttl`), `created_at` | UNIQUE `token_hash`; xem trước nội dung game bằng dữ liệu mẫu, không tạo game session (ADR-0031) |
| `render_jobs` | G | `render_job_id`, `config_type varchar(64)`, `config_version_id`, `source_asset_id`, `kind varchar(64)` (`render_thumbnail`/`render_turntable`), `status varchar(16)` (`queued`/`running`/`done`/`failed`/`dead`), `attempts int`, `last_error varchar(1000)`, `result_asset_id`, `created_at`, `updated_at` | UNIQUE `(config_version_id, source_asset_id, kind)`; kết quả là `game.media_assets` loại `render_*` |
| `avatar_presets` | T | `student_id`, `slot smallint`, `avatar_config jsonb` | PK `(student_id, slot)`; CHECK `slot BETWEEN 1 AND 3` |

```sql
-- Ví dụ: ví và sổ cái tiền tệ (ADR-0049). Số dư là cache; nguồn sự thật là ledger.
CREATE TABLE gamification.wallets (
  tenant_id     uuid        NOT NULL,
  student_id    uuid        NOT NULL,
  currency_code varchar(64) NOT NULL REFERENCES gamification.currencies(currency_code),
  balance       bigint      NOT NULL DEFAULT 0 CHECK (balance >= 0),
  version       integer     NOT NULL DEFAULT 1,
  updated_at    timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (student_id, currency_code)
);

CREATE TABLE gamification.currency_ledger (
  entry_id        uuid         NOT NULL,
  tenant_id       uuid         NOT NULL,
  student_id      uuid         NOT NULL,
  currency_code   varchar(64)  NOT NULL,
  delta           bigint       NOT NULL CHECK (delta <> 0),
  balance_after   bigint       NOT NULL CHECK (balance_after >= 0),
  reason_code     varchar(64)  NOT NULL,
  source_type     varchar(64)  NOT NULL,
  source_id       uuid         NOT NULL,
  idempotency_key varchar(64)  NOT NULL,
  created_by      uuid,
  created_at      timestamptz  NOT NULL DEFAULT now(),
  PRIMARY KEY (entry_id, created_at),
  UNIQUE (idempotency_key, created_at)
) PARTITION BY RANGE (created_at);
-- Role ứng dụng chỉ INSERT/SELECT trên ledger (REVOKE UPDATE, DELETE, TRUNCATE).
```

Bảng `xp_ledger` (§2.13) bổ sung `source_code varchar(64)`, `rule_version int`, `capped bool`. Job đối soát hằng đêm so `wallets.balance` với tổng `currency_ledger.delta` và `game_profiles.xp_total` với tổng `xp_ledger.amount`; lệch → alert, không tự sửa.

### 2.8b. `analytics` (T) · DB `eduworld_game` — Game Analytics (ADR-0058)

Ghi bởi analytics consumer của module game; chỉ số định nghĩa ở [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md). **Không** có `student_id`, tên, nickname, nội dung chat; định danh là `player_key`/`session_key` varchar(64) (HMAC-SHA256 hex). Chiều chuẩn: `tenant_id`, `school_id`, `grade_level smallint`, `platform varchar(16)`, `locale varchar(8)`, `device_tier varchar(8)`, `client_version varchar(32)`.

| Bảng | Loại | Cột chính | Ràng buộc |
|---|---|---|---|
| `events_raw` | T, P (ngày theo `occurred_on`) | `event_id uuid`, `occurred_at`, `occurred_on date`, chiều chuẩn, `player_key`, `session_key`, `event_name varchar(64)`, `source varchar(8)` (`server`/`client`), `props jsonb`, `ingested_at` | PK `(occurred_on, event_id)`; CHECK `octet_length(props::text) <= 2048`; index `(occurred_on, event_name)`, `(occurred_on, tenant_id)` |
| `player_days` | T, P (tháng) | `activity_date`, `player_key`, chiều chuẩn, `sessions int`, `active_seconds int`, `first_active_date date`, `capped_xp bool`, `capped_currency bool` | PK `(activity_date, player_key)` |
| `rollup_daily` | T, P (tháng) | `bucket_date`, `metric_code varchar(64)`, `metric_version smallint`, `dims_hash varchar(64)`, `dims jsonb`, `value numeric`, `numerator bigint`, `denominator bigint`, `computed_at` | UNIQUE `(bucket_date, metric_code, metric_version, tenant_id, dims_hash)` |
| `rollup_weekly` | T, P (năm) | như `rollup_daily` với `iso_week varchar(8)` | UNIQUE tương tự |
| `cohort_retention` | T, P (năm) | `cohort_date`, `dims_hash`, `dims jsonb`, `day_n smallint`, `cohort_size int`, `retained int` | UNIQUE `(cohort_date, tenant_id, dims_hash, day_n)` |
| `funnel_daily` | T, P (tháng) | `bucket_date`, `funnel_code varchar(64)`, `step_no smallint`, `dims_hash`, `dims jsonb`, `count int` | UNIQUE `(bucket_date, funnel_code, step_no, tenant_id, dims_hash)` |
| `content_stats_weekly` | T, P (năm) | `iso_week`, `content_kind varchar(16)` (`quest`/`challenge`/`boss`), `content_version_id uuid`, `started`, `completed`, `abandoned`, `duration_p50_s` | UNIQUE `(iso_week, tenant_id, content_kind, content_version_id)` |
| `perf_daily` | T, P (tháng) | `bucket_date`, `platform`, `device_tier`, `preset varchar(8)`, `renderer varchar(16)`, `client_version`, `samples int`, `fps_p50`, `fps_p10`, `budget_pass_rate`, `load_cold_p50_ms`, `load_cold_p90_ms`, `load_area_p90_ms`, `crash_free_rate` | UNIQUE `(bucket_date, tenant_id, platform, device_tier, preset, renderer, client_version)` |

- Consumer idempotent qua `infra.processed_events` (ADR-0042); dashboard chỉ đọc bảng tổng hợp, không đọc `events_raw`.
- Truy vấn liên tenant chỉ qua role `game_platform` có audit (ADR-0005).

### 2.8c. Sự kiện, community goal, season track · DB `eduworld_game`

Nguồn: [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md). Bảng G (platform) có `tenant_id NULL`; sự kiện tenant (từ template) là T. Mọi bảng có cột audit chuẩn và `deleted_at` (trừ ledger append-only). Chuỗi theo §1.4.

| Bảng | Loại | Cột chính | Ràng buộc / index |
|---|---|---|---|
| `events`, `event_versions` | G/T | `event_id`, `code varchar(64)`, `owner_scope varchar(16)`, `event_kind varchar(32)`, `current_version_id`; version: `payload jsonb` (theo `event.schema.json`), `status varchar(16)`, `start_at`, `end_at` | UNIQUE `(tenant_id, code) WHERE deleted_at IS NULL`; UNIQUE `(event_id, version)` |
| `event_templates`, `event_template_versions` | G | `template_id`, `code varchar(64)`, `allowed_params jsonb`, `current_version_id` | Gán tenant qua `infra.tenant_resource_assignments` (ADR-0029) |
| `event_runs` | G/T | `event_run_id`, `event_id`, `event_version_id`, `status varchar(16)` (`scheduled/live/suspended/ended/cancelled/archived`), `starts_at`, `ends_at`, `claim_until`, `suspended_reason varchar(1000)`, `status_changed_by` | idx `(status, starts_at)`, `(status, ends_at)` cho scheduler |
| `event_participation` | T | `event_run_id`, `student_id`, `school_id`, `joined_at`, `event_points int`, `claimed_milestones varchar(64)[]` | PK `(event_run_id, student_id)` |
| `event_point_ledger` | TA, P (tháng) | `entry_id`, `event_run_id`, `student_id`, `delta int`, `source_code varchar(64)`, `source_event_id uuid`, `capped bool`, `created_at` | UNIQUE `(source_event_id, student_id, created_at)`; chỉ INSERT |
| `community_goals` | G/T | `goal_id`, `event_run_id`, `goal_code varchar(64)`, `metric varchar(32)`, `scope varchar(16)`, `target bigint`, `per_student_daily_cap int`, `milestones jsonb` | UNIQUE `(event_run_id, goal_code)` |
| `community_goal_contributions` | TA, P (ngày) | `goal_id`, `scope_id uuid`, `student_id`, `class_id`, `amount int`, `source_event_id uuid`, `contributed_on date`, `created_at` | UNIQUE `(goal_id, source_event_id, created_at)`; idx `(goal_id, student_id, contributed_on)` kiểm tra trần ngày; chỉ INSERT |
| `community_goal_progress` | T | `goal_id`, `scope_id`, `total bigint`, `milestones_reached smallint[]`, `recomputed_at` | PK `(goal_id, scope_id)`; job tính lại từ contributions (nguồn sự thật) |
| `season_tracks`, `season_track_versions` | G | `season_track_id`, `season_id`, `code varchar(64)`, `payload jsonb` (theo `season-track.schema.json`), `current_version_id` | UNIQUE `(season_id) WHERE deleted_at IS NULL` (một track/mùa) |
| `season_point_ledger` | TA, P (tháng) | `entry_id`, `season_id`, `student_id`, `delta int`, `source_code varchar(64)`, `source_event_id uuid`, `capped bool`, `created_at` | UNIQUE `(source_event_id, student_id, created_at)`; chỉ INSERT |
| `season_track_progress` | T | `season_id`, `student_id`, `school_id`, `points int`, `tier smallint`, `claimed_tiers smallint[]`, `auto_granted_at` | PK `(season_id, student_id)` |
| `ref.seasons` | projection | `season_id`, `starts_at`, `ends_at`, `theme_code varchar(64)` | Nạp từ `season.started`/`season.ended` (module competition sở hữu `competition.seasons`) |

- Phần thưởng sự kiện/mùa cấp qua `reward_grants` + `xp_ledger`/`currency_ledger`/`item_ledger` (§2.8a, ADR-0049), idempotent theo `(trigger_event_id, rule_version_id, student_id)`.
- Bộ đếm nhanh community goal ở Valkey (`ew:game:cgoal:*`) chỉ là cache; mất Valkey thì tính lại từ `community_goal_contributions`.

### 2.9. `multiplayer` (T) · DB `eduworld_competition`

| Bảng | Cột chính | Ràng buộc |
|---|---|---|
| `parties` | `party_id`, `school_id`, `name`, `leader_id`, `party_type`, `visibility`, `invitation_policy`, `max_members` (CHECK 2–8, mặc định 4), `status`, `current_activity_id`, `current_game_session_ref`, `created_at`, `updated_at`, `disbanded_at` | idx `(school_id, status)` |
| `party_members` | `party_id`, `student_id`, `school_id`, `role`, `joined_at`, `left_at`, `left_reason` | **UNIQUE `(student_id) WHERE left_at IS NULL`** (1 active party/học sinh); **UNIQUE `(party_id, student_id) WHERE left_at IS NULL`** (chống duplicate membership) |
| `party_invitations` | `invitation_id`, `party_id`, `school_id`, `inviter_id`, `invitee_id`, `status`, `expires_at`, `created_at`, `responded_at` | UNIQUE `(party_id, invitee_id) WHERE status='pending'` |
| `party_activities` | `party_activity_id`, `party_id`, `activity_type`, `activity_ref`, `room_id`, `started_at`, `ended_at`, `result_summary jsonb` | lịch sử |
| `rooms` | `room_id`, `room_type`, `area_id`, `school_id` (null cho match liên trường), `capacity`, `status`, `created_at`, `closed_at` | bản ghi tóm tắt; state live trong Valkey |
| `session_memberships` | `room_id`, `student_id`, `school_id`, `joined_at`, `left_at`, `disconnects` | |

### 2.10. `competition` · DB `eduworld_competition`

| Bảng | Loại | Cột chính | Ràng buộc |
|---|---|---|---|
| `seasons` | G | `season_id`, `name`, `starts_at`, `ends_at`, `rules jsonb` | |
| `matches` | G* | `match_id` (= `proposal_id` của matchmaking khi ghép từ hàng đợi), `mode`, `subject_id`, `grade_level`, `question_plan jsonb`, `status`, `tournament_id`, `started_at`, `ended_at` | *match có thể liên trường; truy cập qua `match_participants` |
| `match_participants` | T | `match_id`, `student_id`, `school_id`, `team_no`, `party_id`, `outcome`, `score`, `connected_ms` | PK `(match_id, student_id)` |
| `match_results` | G | `match_id` PK, `winner_team`, `summary jsonb`, `verified_at`, `recorded_by` | **PK = match_id → một kết quả/trận** |
| `rating_changes` | T | `match_id`, `student_id`, `school_id`, `before jsonb`, `after jsonb` | PK `(match_id, student_id)` → không tính trùng |
| `ratings` | T | `student_id`, `mode`, `subject_id`, `season_id`, `school_id`, `rating`, `rd`, `volatility`, `games_played`, `updated_at` | PK `(student_id, mode, subject_id, season_id)` |
| `competition_points` | T | `student_id`, `season_id`, `scope_key`, `points` | |
| `leaderboard_definitions` | G | `leaderboard_id`, `scope_type`, `scope_id`, `metric`, `period` | Mọi bảng, key Valkey và API dùng tên `leaderboard_id` |
| `leaderboard_contributions` | TA | `contribution_id`, `school_id`, `student_id`, `class_id`, `metric varchar(64)` (`effort_points`/`xp`/`quest_completed`/`team_points`), `amount int`, `source_type varchar(64)`, `source_id varchar(64)`, `day_key date`, `rule_version_id`, `occurred_at` | UNIQUE `(metric, source_type, source_id, student_id)`; idx `(tenant_id, metric, day_key, student_id)` |
| `season_results` | TA | `season_id`, `student_id`, `mode`, `subject_id`, `school_id`, `final_rating`, `final_rd numeric(9,4)`, `peak_tier_code varchar(64)`, `games_played`, `competition_points`, `created_at` | PK `(season_id, student_id, mode, subject_id)` |
| `competition_restrictions` | T | `restriction_id`, `school_id`, `student_id`, `kind varchar(64)` (`abandon_penalty`/`restrict_competition`), `reason varchar(1000)`, `starts_at`, `ends_at`, `created_by` (NULL = hệ thống) | idx `(tenant_id, student_id, ends_at) WHERE deleted_at IS NULL` |
| `game_configs`, `game_config_versions`, `content_releases` | G | `game_configs`: `config_id`, `config_type varchar(64)` (`matchmaking-configs`/`ranking-rules`/`seasons`/`tournament-templates`/`challenge-templates`/`leaderboard-definitions`/`competition-rewards`), `code`, `current_version_id`, `latest_version_no`, `owner_scope = platform`; version: `version`, `status`, `title`, `payload jsonb` (≤ 64 KB), `effective_from`, `change_note`, `published_at` | Cấu hình thi đấu có version (ADR-0043), ghi chỉ qua `/api/v1/game-admin/*`; UNIQUE `(config_type, code) WHERE deleted_at IS NULL`, UNIQUE `(config_id, version)`; `content_releases` append-only theo mẫu §3.13 |
| `challenges` | T | `challenge_id`, `school_id`, `template_config_id`, `template_version_id` (ghim), `kind varchar(64)` (`class_challenge`/`class_tournament`/`school_tournament`), `name varchar(128)`, `targets jsonb`, `participation varchar(64)` (`individual`/`study_group`), `question_set_version_id`, `params jsonb`, `starts_at`, `ends_at`, `status varchar(64)` (`scheduled`/`running`/`completed`/`cancelled`), `tournament_id`, `match_id`, `cancel_reason`, `created_by` | Thử thách lớp/giải trường do nhà trường tạo qua `/lms/challenges` (module competition); idx `(tenant_id, school_id, status, starts_at) WHERE deleted_at IS NULL` |
| `leaderboard_snapshots` | T/G, P | `leaderboard_id`, `period_key`, `rank`, `student_id`, `value` | snapshot cuối kỳ (Valkey là live) |
| `tournaments`, `tournament_entries`, `tournament_groups`, `bracket_nodes`, `tournament_results`, `tournament_rewards` | T (giải trong tenant, tạo từ template) / G (giải liên tenant do platform tạo, có `tenant_ids uuid[]`) | `tournaments.format` (`single_elim`/`double_elim`/`round_robin`/`groups_then_single_elim`), `owner_scope` (`class`/`school`/`tenant`/`platform`), `status`; `bracket_nodes.bracket` (`winners`/`losers`/`grand_final`/`group`); kết quả, phần thưởng append-only | UNIQUE `(tournament_id, student_id)` entry; UNIQUE `(tournament_id, bracket, round, position)` node |
| `duels` | T | `duel_id`, `challenger_id`, `opponent_id`, `subject_id`, `topic_id`, `status varchar(32)` (`pending/accepted/declined/expired/cancelled`), `match_id`, `expires_at` (60 s) | UNIQUE `(challenger_id, opponent_id) WHERE status='pending'`; không tạo khi hai bên block nhau (projection `ref.blocks`) — [MATCHMAKING §8](../03-social-multiplayer/MATCHMAKING.md) |

### 2.11. `social` (T) · DB `eduworld_chat`

Nguồn: [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md), ADR-0050, ADR-0054, ADR-0084. `social.clubs` chỉ chứa **club do học sinh/nhà trường lập theo SOCIAL_SYSTEM §6** (Team cố định là **loại club** `club_type`, không có bảng `teams` riêng). **Study group không phải club**: bảng `lms.study_groups`/`lms.study_group_members` thuộc `eduworld_learning` (§2.14, ADR-0084); module chat chỉ giữ bản chiếu thành viên trong `chat.channel_members` của channel `study_group`.

| Bảng | Loại | Cột chính | Ràng buộc / index |
|---|---|---|---|
| `friend_requests` | T | `request_id`, `requester_id`, `addressee_id`, `status varchar(32)`, `expires_at` (7 ngày) | UNIQUE `(requester_id, addressee_id) WHERE status='pending'` |
| `friendships` | T | `friendship_id` PK, `user_low_id`, `user_high_id` (chuẩn hóa thứ tự), `created_at`, `deleted_at`, `deleted_by` | UNIQUE `(user_low_id, user_high_id) WHERE deleted_at IS NULL` — PK riêng để hủy kết bạn bằng soft-delete rồi kết bạn lại được; chặn một người kết thúc tình bạn |
| `friend_codes` | T | `friend_code_id` PK, `code varchar(12)`, `owner_id`, `expires_at` (10 phút) | UNIQUE `code WHERE expires_at > now()` (kiểm trong app); dọn bằng TTL job, không phải dữ liệu nghiệp vụ |
| `blocks` | T | `block_id` PK, `blocker_id`, `blocked_id`, `created_at`, `deleted_at`, `deleted_by` | UNIQUE `(blocker_id, blocked_id) WHERE deleted_at IS NULL` |
| `mutes` | T | `mute_id` PK, `muter_id`, `muted_id`, `created_at`, `deleted_at`, `deleted_by` | UNIQUE `(muter_id, muted_id) WHERE deleted_at IS NULL` |
| `clubs` | T | `club_id`, `school_id`, `club_type varchar(32)`, `name varchar(64)`, `description varchar(500)`, `icon_id varchar(64)`, `subject_id`, `owner_id`, `status varchar(32)` (`pending_approval/active/locked/archived`), `max_members int` (mặc định 50), `chat_channel_id`, `version` | idx `(tenant_id, school_id, status)` |
| `club_members` | T | `club_id`, `user_id`, `role varchar(32)` (`owner/officer/member`; club do nhân sự trường tạo ghi người tạo ở `clubs.created_by`, không có vai trò giáo viên giám sát — ADR-0072), `joined_at`, `left_at` | UNIQUE `(club_id, user_id) WHERE left_at IS NULL AND deleted_at IS NULL`; kiểm ≤ 5 club active/học sinh trong transaction |
| `club_join_requests` | T | `request_id`, `club_id`, `requester_id`, `status varchar(32)`, `decided_by`, `decided_at` | UNIQUE `(club_id, requester_id) WHERE status='pending'` |
| `club_invitations` | T | `invitation_id`, `club_id`, `inviter_id`, `invitee_id`, `status varchar(32)`, `expires_at` | UNIQUE `(club_id, invitee_id) WHERE status='pending'` |
| `club_goals`, `club_goal_progress` | T | `goal_id`, `club_id`, `title varchar(128)`, `metric varchar(64)`, `subject_id`, `target int`, `week_start date`, `status`; progress: `goal_id`, `user_id`, `contribution int` | PK progress `(goal_id, user_id)`; đếm từ event học tập/game, không lưu điểm cá nhân |
| `notifications` | T, P (tháng theo `created_at`) | `notification_id`, `recipient_id`, `school_id`, `kind varchar(64)` (một enum dùng chung REST và WS), `message_code varchar(128)` (khóa `ui.notification.<code>`), `params jsonb` (≤ 4 KB, không PII ngoài nickname), `title jsonb`, `body jsonb`, `ref_type`, `ref_id`, `action jsonb` (deep link `{kind, id}`), `source_event_id`, `created_at`, `read_at` | PK `(notification_id, created_at)`; UNIQUE `(recipient_id, source_event_id, kind, created_at)` (idempotent theo event nguồn); idx `(tenant_id, recipient_id, created_at DESC)`. Thông báo trong game (không push); "Thông báo trường" chỉ gồm bài giao và kết quả bài giao |
| `club_events` | T | `event_id`, `club_id`, `title varchar(128)`, `description varchar(1000)`, `event_kind varchar(32)`, `tournament_template_id`, `starts_at`, `ends_at` | idx `(club_id, starts_at)` |.

### 2.12. `chat` (T) · DB `eduworld_chat`

| Bảng | Loại | Cột chính | Ràng buộc |
|---|---|---|---|
| `channels` | T | `channel_id`, `channel_type` (`world`/`area`/`match`/`party`/`team`/`class`/`study_group`/`direct`), `is_gameplay` (true cho `match`, `party`, `team` gắn trận — ADR-0020 §6), `scope_ref` (match_id/party_id/…), `tenant_id`, `tenant_ids uuid[]` (chỉ channel liên tenant do platform tạo), `school_id`, `policy_id`, `retention_days`, `last_seq bigint` (persister cập nhật bằng `GREATEST`; `seq` cấp bằng Valkey — ADR-0035), `opened_at`, `closed_at` (trận kết thúc / party giải tán), `purge_after` (= `closed_at` + `gameplay_chat_retention_days`) | CHECK `channel_type IN (...)`; CHECK `is_gameplay = (channel_type IN ('match','party') OR (channel_type = 'team' AND scope_ref->>'match_id' IS NOT NULL))`; idx `(purge_after) WHERE is_gameplay AND closed_at IS NOT NULL`. Channel gameplay **không tắt được** bằng policy trường/lớp |
| `gameplay_messages` | **P (ngày `created_on`)** | Tin nhắn của channel gameplay (ADR-0033), cùng cột tính năng với `messages`. Xem DDL §3.8b | Purge theo channel hết hạn (xóa lô) + `DROP PARTITION` khi mọi channel của ngày đó đã purge (role `chat_purger`) |
| `gameplay_partition_channels` | G | `partition_day`, `channel_id`, `tenant_id`, `purged_at` | PK `(partition_day, channel_id)`; quyết định khi nào drop partition |
| `gameplay_chat_daily_stats` | T | `tenant_id`, `day`, `channel_type`, `channels`, `messages`, `masked`, `blocked`, `reported` | PK `(tenant_id, day, channel_type)`; rollup **trước** khi purge, giữ lâu dài cho analytics |
| `gameplay_purge_runs` | G (append-only) | `run_id`, `started_at`, `finished_at`, `tenant_id`, `kind` (`gameplay`/`ephemeral`), `channels_purged`, `messages_purged`, `partitions_dropped`, `deferred_open_case`, `evidence_copied`, `status` | Bản ghi tổng hợp, không chứa nội dung |
| `channel_members` | T | `channel_id`, `user_id`, `school_id`, `role`, `muted_until`, `last_delivered_seq`, `last_read_seq`, `unread_mentions`, `show_read_receipts`, `notification_level` | PK `(channel_id, user_id)`; con trỏ chỉ tăng |
| `messages` | **P** | xem DDL §3.8 — gồm `kind` (text/emoji/quick/sticker/gif/system), `body`/`body_display`, `reply_to_message_id`, `quote`, `forwarded_from`, `mentions`, `revision`, `edited_at`, `recalled_at`, `search_tsv` | UNIQUE `(channel_id, seq, created_at)`; GIN `search_tsv` |
| `message_revisions` | T | `message_id`, `revision`, `body`, `body_display`, `decision`, `edited_at` | PK `(message_id, revision)`; bằng chứng khi sửa (ADR-0034) |
| `message_reactions` | T | `channel_id`, `message_id`, `user_id`, `emoji_code` | PK `(message_id, user_id, emoji_code)`; ≤ 3/người/tin (use case) |
| `pinned_messages` | T | `channel_id`, `message_id`, `pinned_by`, `pinned_at` | PK `(channel_id, message_id)`; ≤ 3 active/channel |
| `sticker_packs`, `stickers` | G (platform) | xem DDL §3.8a | `owner_scope = 'platform'`; tenant thấy qua `tenant_resource_assignments` |
| `gif_assets`, `gif_tags` | G (platform) | xem DDL §3.8a | Thư viện GIF nội bộ đã duyệt; không có nguồn bên thứ ba |
| `quick_messages` | G | `quick_message_id`, `category`, `text_vi`, `text_en`, `status` | |
| `chat_policies` | T* | `scope_type`, `scope_id`, `school_id`, `rules jsonb`, `version` | |
| `reports` | T | `report_id`, `school_id`, `reporter_id`, `target_type`, `target_id`, `reason`, `status`, `evidence jsonb` (snapshot tin nhắn) | retention 2 năm sau khi đóng case [Confirmed] ([DATA_PRIVACY](../10-security/DATA_PRIVACY.md)) |
| `moderation_cases`, `moderation_actions`, `sanctions` | T | `moderation_cases`: thêm `assignee_moderator_id` (user `safety_moderator`), `assigned_via varchar(16)` (`manual`/`rotation`), `oncall_page_level varchar(16)` (`primary`/`secondary`/`lead`/`system_admin`), `last_paged_at`, `school_notified_at`, `school_ack_at` (thời điểm trường xác nhận đã liên hệ phụ huynh). Enum trạng thái case, loại sanction và lý do theo [MODERATION](../03-social-multiplayer/MODERATION.md) | append-only cho actions; đọc liên tenant chỉ qua `tenancy.Bypass(ctx, "moderation_case")` của module moderation, có audit (ADR-0072) |
| `moderation_oncall_shifts` | G | `shift_id`, `moderator_user_id`, `oncall_role varchar(16)` (`primary`/`secondary`), `starts_at`, `ends_at`, `note varchar(500)`, `created_by`, `version`, `deleted_at`, `deleted_by` | Lịch trực Safety Team (bảng platform, không `tenant_id`); idx `(oncall_role, starts_at, ends_at) WHERE deleted_at IS NULL`; không chồng lấn cùng `oncall_role` (kiểm trong command dưới advisory lock) |
| `break_glass_grants` | T | `grant_id`, `scope_type varchar(16)` (`channel`/`user`), `scope_id`, `window_from`, `window_to`, `duration_minutes`, `reason varchar(1000)`, `status varchar(16)` (`pending`/`approved`/`rejected`/`expired`/`revoked`), `requested_by`, `requested_at`, `decided_by`, `decided_at`, `decision_note`, `expires_at`, `revoked_by`, `revoked_at`, `revoke_reason`, `messages_viewed_count`, `version` | Truy cập khẩn cấp hai người (CHECK `decided_by <> requested_by`); idx `(status, expires_at) WHERE status = 'approved'`, `(tenant_id, requested_at DESC)`; mọi lượt xem ghi audit |
| `sensitive_term_categories` | H | `category_id`, `code`, `name`, `description`, `severity` (`watch/low/medium/high/critical`), `default_action`, `enabled`, `allow_tenant_toggle`, `created_by`, `updated_at` | UNIQUE `(code) WHERE tenant_id IS NULL AND deleted_at IS NULL`; tenant không tạo category [Confirmed] → CHECK `owner_scope = 'platform'` |
| `tenant_category_settings` | T | `tenant_id`, `category_id`, `enabled`, `reason`, `updated_by`, `updated_at` | UNIQUE `(tenant_id, category_id) WHERE deleted_at IS NULL`; ghi chỉ khi category `allow_tenant_toggle` |
| `sensitive_terms` | H | `term_id`, `category_id`, `term`, `match_type`, `locale`, `variants varchar(64)[]`, `severity_override`, `action_override`, `enabled`, `status` (`draft/published`), `note`, `created_by`, `published_in_version` | UNIQUE `(coalesce(tenant_id, '00000000-0000-0000-0000-000000000000'), category_id, lower(term), match_type) WHERE deleted_at IS NULL`; nội dung chỉ đọc được bởi role có `chat.sensitive_terms.read` (view riêng, không qua API game) |
| `sensitive_allowlist` | H | `entry_id`, `phrase`, `locale`, `reason` | UNIQUE `(coalesce(tenant_id, …), lower(phrase)) WHERE deleted_at IS NULL` |
| `sensitive_catalog_versions` | H (append-only) | `catalog_version bigint`, `scope` (`platform/tenant`), `tenant_id?`, `published_at`, `published_by`, `change_summary`, `term_count`, `checksum`, `regression_recall`, `regression_fp_rate` | PK `(coalesce(tenant_id,…), catalog_version)`; không sửa/xóa |
| `moderation_switches` | H | `switch_id`, `scope` (`platform/tenant`), `tenant_id` (NULL = platform), `enabled_all`, `steps jsonb` (`sensitive_terms, pii, spam, bullying, async_analysis`), `reason`, `expires_at`, `updated_by`, `updated_at` | UNIQUE `(coalesce(tenant_id,…)) WHERE deleted_at IS NULL`; CHECK có bước tắt ⇒ `reason IS NOT NULL`; ghi chỉ qua `/api/v1/admin/chat/moderation-switches` |
| `moderation_switch_changes` | TA/G (append-only) | `change_id`, `tenant_id?`, `before jsonb`, `after jsonb`, `reason`, `expires_at`, `actor_id`, `changed_at`, `source` (`manual`/`auto_expire`) | Lịch sử bật/tắt (bổ sung cho audit log) |
| `moderation_decisions` | **P**, TA | `decision_id`, `tenant_id`, `channel_id`, `message_id?`, `sender_id`, `decision` (`allow/unchecked/mask/block/hold_for_review/block_and_alert`), `category_codes varchar(64)[]`, `steps_run varchar(64)[]`, `steps_skipped varchar(64)[]`, `catalog_version_platform`, `catalog_version_tenant`, `switch_snapshot_id`, `latency_ms`, `created_at` | Không lưu nội dung tin; partition theo tháng; retention theo DATA_PRIVACY (đề xuất 180 ngày, OQ-12; không xóa vật lý, ADR-0070) |

### 2.13. `gamification` (T) · DB `eduworld_game`

| Bảng | Cột chính | Ràng buộc |
|---|---|---|
| `xp_ledger` | `xp_entry_id`, `student_id`, `school_id`, `amount`, `source_type`, `source_id`, `rule_id`, `idempotency_key`, `created_at` | **UNIQUE `(student_id, source_type, source_id, rule_id)`**; partition theo tháng khi > 50M dòng [Confirmed] |
| `xp_daily_caps` | `student_id`, `date`, `bucket`, `granted` | PK ghép |
| `reward_rules`, `reward_rule_versions` | G | |
| `reward_grants` | `reward_grant_id`, `student_id`, `school_id`, `rule_version_id`, `trigger_event_id`, `grants jsonb` | **UNIQUE `(trigger_event_id, rule_version_id, student_id)`** |
| `achievements`, `student_achievements` | | Thay bằng định nghĩa ở §2.8a (`achievement_versions`, PK `(student_id, achievement_id, tier)`) |

### 2.14. `lms` (T) · DB `eduworld_learning`

`courses` (F040: `school_id`, `class_id`, `subject_id`, `grade_level`, `academic_year_id`, `academic_year`, `curriculum_version_id`, `textbook_series_id`, `status` `active`/`hidden`/`archived`; UNIQUE `(tenant_id, class_id, subject_id, academic_year_id)` live), `path_node_unlocks` (F040: `node_id`, `student_id`, `reason varchar(1000)`, `unlocked_by`; UNIQUE `(tenant_id, node_id, student_id)` live), `learning_paths` (**không** có version: sửa có audit, áp dụng tiếp cho học sinh, tiến độ giữ theo `node_id`; loại H — lộ trình mẫu platform `tenant_id` NULL), `path_nodes` (§2.16), `school_curriculum_settings` (§2.16), `assignments` (`open_at`, `due_at`, `max_attempts`, `feedback_timing` (`FeedbackTiming`), `result_release` (`ResultRelease`), `pinned_version_id`, `follow_latest` (ADR-0043), `version`), `assignment_targets` (`target_type` class/study_group/students), `assignment_submissions`, `schedule_items` (§2.16), `study_groups` + `study_group_members` (§2.16, ADR-0084 — study group thuộc module learning, không phải club).

### 2.15. `infra` (mọi DB), `platform` và `audit` (DB `eduworld_identity`)

Mỗi database: `infra.outbox_events`, `infra.processed_events`, `infra.idempotency_records`, `infra.dead_letters` (+ `infra.tenant_resource_assignments` nếu có tài nguyên gán được) — §2.17, DDL §3.9–3.10. Riêng `eduworld_identity`: `platform.feature_flags` và `platform.portal_menu_catalog` (§2.0, DDL §3.15), `platform.retention_policies`, `platform.erasure_log`, `platform.deletion_requests` (§2.16) và `audit.audit_logs` (P, nhận audit từ mọi module — §1.3, DDL §3.11).

### 2.16. Bảng bổ sung từ tài liệu domain

Các bảng dưới đây được các tài liệu domain đưa vào sau khi danh sách §2.1–2.15 được viết. Tài liệu nguồn là nơi mô tả nghiệp vụ; bảng này là nguồn chuẩn về tên, khóa và loại bảng. Mọi bảng đều [Confirmed].

| Bảng | Loại | Cột chính | Khóa / ràng buộc | Tài liệu nguồn |
|---|---|---|---|---|
| `identity.user_merges` | G | `merge_id`, `tenant_id` (tenant đích, RLS đọc theo tenant — F045), `from_user_id`, `into_user_id`, `merged_by`, `merged_at`, `reason`, `school_id`, `saga_id` (FK `platform.saga_runs`, `user_merge`), `reconciliation_issue_id` [Confirmed] | UNIQUE `from_user_id` (một user chỉ bị gộp một lần), UNIQUE `saga_id`; CHECK `from_user_id <> into_user_id`; append-only; user nguồn soft-delete `delete_reason = merged`; issue đối soát `resolution` thêm `merge_requested`/`merged` | [IDENTITY_MAPPING §7](../09-identity/IDENTITY_MAPPING.md) |
| `infra.dead_letters` (mọi DB) | G | `dead_letter_id uuid`, `stream varchar(64)`, `consumer varchar(128)`, `event_id uuid`, `event_type varchar(64)`, `payload jsonb` (CHECK ≤ 64 KB), `error varchar(1000)`, `delivery_count int`, `created_at timestamptz`, `resolved_at timestamptz`, `resolved_by uuid` | Bảng hạ tầng (không mixin, không RLS); partition theo ngày trên `created_at`, UNIQUE `(consumer, event_id, created_at)`; retention theo [EVENT_DESIGN §8](EVENT_DESIGN.md#8-retention) bằng gỡ partition, không gỡ partition còn dòng `resolved_at IS NULL` (§3.9) | [EVENT_DESIGN §7](EVENT_DESIGN.md) |
| `platform.erasure_log` | G | `erasure_id`, `user_id`, `kind` (`anonymize`/`erase`), `requested_by`, `legal_basis`, `executed_at`, `scope jsonb` | append-only; giữ lâu hơn vòng đời backup (dùng để áp lại sau restore) | [DATA_PRIVACY §4](../10-security/DATA_PRIVACY.md) |
| `platform.deletion_requests` | G | `request_id`, `subject_type` (`user`/`tenant`), `subject_id`, `tenant_id`, `requested_by`, `action` (`anonymize`; Release 1.0 không dùng `physical_erase`, ADR-0070), `status` (`pending`/`approved`/`executed`/`rejected`), `approved_by`, `due_at`, `executed_at`, `erasure_id` | idx `(status, due_at)`; job `retention-reapply` đọc sau mỗi lần restore | [BACKUP_RECOVERY §7](../11-operations/BACKUP_RECOVERY.md), [DATA_PRIVACY](../10-security/DATA_PRIVACY.md) |
| ~~`content.school_content_overrides`~~ | — | Bỏ: thay bằng `infra.tenant_resource_assignments` (tenant chỉ thấy nội dung platform được gán, ADR-0029) | — | [MULTITENANCY §3.1](../06-architecture/MULTITENANCY.md) |
| `curriculum.lo_version_mappings` | H | `mapping_id` PK, `curriculum_id` (assignment), `curriculum_version_id` (version mới), `from_lo_id`, `to_lo_id` (NULL khi `removed`), `relation` (`same`/`split`/`merge`/`replaces`/`removed`), `created_at` | CHECK `(relation = 'removed') = (to_lo_id IS NULL)`; UNIQUE `(curriculum_version_id, from_lo_id, coalesce(to_lo_id, '00000000-0000-0000-0000-000000000000')) WHERE deleted_at IS NULL` | [CURRICULUM_MODEL §5](../02-learning/CURRICULUM_MODEL.md) |
| `curriculum.textbook_series`, `curriculum.textbooks`, `curriculum.textbook_sections`, `curriculum.textbook_section_objectives` | G | series: `code`, `name`, `publisher`; textbook: `series_id`, `subject_id`, `grade_level`, `volume`; section: `textbook_id`, `chapter`, `lesson_no`, `title`, `page_range`; n–n section ↔ LO (`section_objective_id` PK, `section_id`, `objective_id`) | UNIQUE `series.code`; UNIQUE `(series_id, subject_id, grade_level, volume)`; UNIQUE `(section_id, objective_id) WHERE deleted_at IS NULL` (khóa thay thế để soft-delete, F021 Clarifications 2026-11-19); chỉ `learning_platform` ghi | [CURRICULUM_MODEL §6](../02-learning/CURRICULUM_MODEL.md) |
| `assessment.regrade_requests` | H | `regrade_request_id`, `tenant_id` (NULL = yêu cầu platform của `content_admin`), `school_id`, `subject_id` (phạm vi `academic_manager`), `scope` (`question_version`/`activity_version`/`exam_session`), `question_version_id`, `activity_version_id`, `exam_session_id`, `corrected_version_id` (bắt buộc khi `use_new_key`), `action` (`accept_all`/`use_new_key`/`drop_item`), `reason varchar(1000)`, `status` (`queued`/`running`/`completed`/`failed`), `affected_attempts`, `re_evaluated_attempts`, `requested_by`, `requested_role` (`content_admin`/`academic_manager`), `idempotency_key`, `created_at`, `started_at`, `finished_at` | CHECK cột đích theo `scope`; UNIQUE `(requested_by, idempotency_key)`; UNIQUE `(coalesce(question_version_id, activity_version_id, exam_session_id), coalesce(tenant_id, '00000000-0000-0000-0000-000000000000')) WHERE status IN ('queued','running')` | [EXAM_ENGINE §9](../02-learning/EXAM_ENGINE.md) — cột do F031 định nghĩa (migration `20261202000001_f031_regrade.sql`: thêm `last_error`, `updated_at`, soft-delete; RLS `tenant_id = current_tenant_id()`, hàng platform chỉ qua `learning_platform`; job `regrade-runner`); F042 chỉ dùng (`scope = exam_session`). `assessment.evaluations`: trigger `evaluations_superseded_immutable` chặn sửa evaluation `superseded`; `details.items` mang kết quả item sau chấm lại |
| `assessment.attempt_score_revisions` | T | `revision_id`, `attempt_id`, `school_id`, `regrade_request_id`, `old_score`, `new_score`, `reason`, `approved_by`, `created_at` | append-only; điểm gốc trong `attempts` giữ nguyên, điểm hiệu lực = revision mới nhất | [EXAM_ENGINE §9](../02-learning/EXAM_ENGINE.md) |
| `assessment.integrity_signals` | T, P | `signal_id`, `school_id`, `student_id`, `attempt_id`, `signal_type`, `score`, `evidence jsonb`, `created_at` | idx `(tenant_id, student_id, created_at DESC)`; không tự động phạt; append-only; tạo ở F065 (`video_progress_implausible`), chưa partition [Proposed] — các định dạng sau chỉ thêm `signal_type` | [ANTI_CHEAT §6](../10-security/ANTI_CHEAT.md) |
| `competition.integrity_signals` | T, P | như trên với `match_id` thay `attempt_id` | như trên | [ANTI_CHEAT §6](../10-security/ANTI_CHEAT.md) |
| `competition.competition_point_ledger` | T | `entry_id`, `student_id`, `school_id`, `season_id`, `mode`, `subject_id`, `amount`, `source_type`, `source_id`, `created_at` | **UNIQUE `(source_type, source_id, student_id)`**; append-only; thay cho bảng tổng `competition_points` (giữ `competition_points` làm read model tổng theo `scope_key`) | [RANKING §6](../01-game-design/COMPETITION_SYSTEM.md) |
| `multiplayer.party_policies` | T* | `scope_type` (platform/school/class), `scope_id`, `school_id`, `rules jsonb` (`allowed_party_types`, `cross_class`, `cross_grade_band`, `max_members_cap`), `version` | UNIQUE `(scope_type, scope_id)`; cấp dưới chỉ siết chặt | [PARTY_SYSTEM §8](../03-social-multiplayer/PARTY_SYSTEM.md) |
| `multiplayer.party_histories` | T, P | `party_id`, `school_id`, `event_type`, `actor_id`, `data jsonb`, `occurred_at` | append-only (Party History) | [PARTY_SYSTEM §12](../03-social-multiplayer/PARTY_SYSTEM.md) |
| `chat.ephemeral_messages` | T, P (theo ngày) | `message_id`, `channel_id`, `school_id`, `sender_id`, `kind`, `body` (đã kiểm tra hợp lệ/che), `quick_message_id`, `moderation_state`, `created_at` | Tin `world`/`area`; ứng dụng chỉ đọc tin trong 24 h (lọc `created_at`); **ADR-0033:** partition ngày được job `gameplay-chat-purge` (role `chat_purger`) `DETACH` + `DROP` sau `ephemeral_retention_hours` (24 h), sau khi sao chép tin thuộc report/case sang `moderation_evidence`. Nguồn cho report | [CHAT_SYSTEM §5–6](../03-social-multiplayer/CHAT_SYSTEM.md) |
| `chat.moderation_evidence` | T | `evidence_id`, `case_id`, `school_id`, `snapshot jsonb` (tin liên quan ± 10 tin ngữ cảnh theo [MODERATION](../03-social-multiplayer/MODERATION.md), metadata, kết quả filter), `sha256`, `created_at` | Bất biến; không bị retention chat xóa cho tới khi case đóng + thời hạn giữ case; mọi lượt xem ghi audit | [CHILD_PROTECTION §5](../10-security/CHILD_SAFETY.md) |
| `game.client_manifests` | G | `client_version`, `platform`, `manifest jsonb` (`area_id`, `npc_id`, `scene_ref`, `interactable_id` có trong build), `published_at` | PK `(client_version, platform)` — CMS dùng để chặn nội dung trỏ tới scene không tồn tại | [ADMIN_PORTAL §3.3](../04-admin-portal/ADMIN_PORTAL.md) |
| `game.content_manifests` | T | `tenant_id`, `manifest_version`, `etag`, `entries jsonb` (item_type, item_id, version_id, snapshot_url, sha256; bundles; areas), `generated_at`, `cause_release_id` | PK `(tenant_id, manifest_version)`; phục vụ `GET /game/content-manifest` (ADR-0043, DDL §3.13) | [ADMIN_PORTAL §6](../04-admin-portal/ADMIN_PORTAL.md) |
| `lms.assignment_submissions` | T | `assignment_id`, `student_id`, `school_id`, `status` (`not_started`/`in_progress`/`submitted`/`late`/`missed`/`excused`), `best_attempt_id`, `score`, `submitted_at`, `updated_at` | PK `(assignment_id, student_id)`; read model cập nhật từ event | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) (School Learning Dashboard) |
| `lms.path_nodes` | H (như lộ trình) | `node_id`, `tenant_id` (NULL với mẫu), `path_id`, `school_id`, `position`, `content_type`, `content_id`, `content_version_id` (ghim, F040 thay `content_ref jsonb`), `title`, `objective_ids uuid[]` (≤ 50), `unlock_rule jsonb` | UNIQUE `(path_id, position) WHERE deleted_at IS NULL`; thay cho `learning_path_steps` ở §2.14; tiến độ học sinh gắn với `node_id` nên sửa lộ trình không làm mất tiến độ | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) |
| `lms.school_curriculum_settings` | T | `setting_id`, `school_id`, `academic_year_id`, `grade_level smallint` (NULL = mọi khối), `curriculum_version_id` (version đã publish), `set_by`, `deleted_at`, `deleted_by` | UNIQUE `(tenant_id, school_id, academic_year_id, coalesce(grade_level, 0)) WHERE deleted_at IS NULL`; nơi lưu chương trình áp dụng của trường (thay khóa `school.settings.curriculum_version_id`) | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) |
| `lms.schedule_items` | T | `schedule_item_id`, `course_id`, `school_id`, `date`, `period`, `kind` (`lesson`/`assignment_due`/`exam`/`event`), `ref_type`, `ref_id` (F040 thay `ref jsonb`), `note varchar(500)`, `source_event_id`, `created_by` | idx `(tenant_id, course_id, date)` live; UNIQUE `(tenant_id, source_event_id)` live (mục sinh từ event một lần, F040); thay cho `schedules` ở §2.14 | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) |
| `lms.study_groups` | T (soft-delete) | `study_group_id` (UUIDv7), `tenant_id`, `school_id`, `class_id uuid NULL` (null = nhóm theo khối), `grade_level smallint NOT NULL`, `name varchar(64) NOT NULL`, `member_count int NOT NULL`, `version int`, `created_by`, `updated_by`, `created_at`, `updated_at`, `deleted_at`, `deleted_by` | UNIQUE `(tenant_id, school_id, lower(name)) WHERE deleted_at IS NULL` (trùng → `409 CONFLICT`); idx `(tenant_id, school_id, class_id) WHERE deleted_at IS NULL`; ghi qua command handler + outbox `study_group.*` | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md), [ADR-0084](../adr/ADR-0084-nhom-hoc-tap-thuoc-learning-service.md) |
| `lms.study_group_members` | T (soft-delete) | `study_group_id`, `student_id`, `tenant_id`, `school_id`, `added_by`, `added_at`, `deleted_at`, `deleted_by` | UNIQUE `(study_group_id, student_id) WHERE deleted_at IS NULL`; idx `(tenant_id, student_id) WHERE deleted_at IS NULL`; 1–100 thành viên/nhóm, là học sinh active của trường (`ref.students`; có `class_id` thì `ref.enrollments` của lớp) | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md), [ADR-0084](../adr/ADR-0084-nhom-hoc-tap-thuoc-learning-service.md) |
| `sis.sis_imports`, `sis.staging_records`, `sis.sync_record_errors` | xem §2.3 | | | [SIS_INTEGRATION §6.1](../09-identity/SIS_INTEGRATION.md) |

Cột bổ sung trên bảng có sẵn: `identity.users.status` thêm `anonymized` (+ `anonymized_at`); `identity.external_identities.status` (`active`/`superseded`); `identity.user_roles.training_completed_at` (trước đây `role_assignments`, ADR-0066); `org.students.national_id_hash`; `platform.tenants.keycloak_org_id`/`keycloak_org_alias` (một Keycloak Organization cho mỗi tenant, ADR-0026, [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md)); `org.organizations` bỏ, thay bằng tenant loại `unit`; `multiplayer.parties.grade_band` (grade band thấp nhất của party, [PARTY_SYSTEM §12](../03-social-multiplayer/PARTY_SYSTEM.md)).

### 2.17. Projection `ref` và bảng hạ tầng `infra` (mọi database)

**Bảng `infra` có trong mỗi database** (DDL §3.9–3.10):

| Bảng | Mô tả |
|---|---|
| `infra.outbox_events` | Event chờ publish lên NATS JetStream `ew.evt.<module>.<event_type>` (ADR-0042) |
| `infra.processed_events` | Dedupe consumer: PK `(consumer, event_id)` |
| `infra.idempotency_records` | Kết quả request có `Idempotency-Key` |
| `infra.dead_letters` | Event consumer xử lý lỗi quá số lần, chờ xử lý tay |
| `infra.tenant_resource_assignments` | Gán tài nguyên platform cho tenant (ADR-0029) — chỉ ở database có tài nguyên gán được: learning (curriculum, lesson, question, question set, blueprint, challenge template), game (quest, mini-game, item), competition (tournament template), chat (sticker pack, GIF, sensitive catalog) |

**Projection `ref`** (chỉ đọc với use case; chỉ consumer cập nhật; nguồn sự thật ở module gốc):

| Bảng projection | Có trong database | Nguồn (module · event) | Cột (tối thiểu cần dùng) |
|---|---|---|---|
| `ref.tenants` | learning, game, competition, chat | identity · `tenant.created/updated`, `tenant.status_changed` | `tenant_id`, `code varchar(64)`, `tenant_type varchar(64)`, `status varchar(64)`, `is_qa boolean` (idx `WHERE is_qa`, dashboard/analytics loại bằng anti-join), `settings_subset jsonb`, `source_version bigint` |
| `ref.schools` | learning, game, competition, chat | identity · `school.synced`, `school.updated` | `school_id`, `tenant_id`, `name varchar(255)`, `name_i18n jsonb`, `province_code varchar(64)`, `province_name_i18n jsonb` (phi chuẩn hóa để hiển thị), `school_level varchar(64)`, `is_virtual`, `status`, `settings_subset jsonb` (cài đặt trường cần cho module nhận, vd. competition: `access_hours`, `disabled_leaderboard_definitions`, `allow_cross_school_listing`; identity phát trong `school.updated`) |
| `ref.classes` | learning, **game**, competition, chat | identity · `class.synced` | `class_id`, `tenant_id`, `school_id`, `grade_level`, `name varchar(128)`, `academic_year_id`, `is_virtual` |
| `ref.provinces` | learning, game | identity · `province.updated` | `code varchar(64)`, `country_code varchar(2)`, `name_i18n jsonb`, `status varchar(64)` (báo cáo theo tỉnh) |
| `ref.students` | learning, game, competition, chat | identity · `student.synced`, `student.cleaned_up`, `student.restored`, `user.status_changed` | `student_id`, `tenant_id`, `school_id`, `grade_level`, `status`, `deleted_at` |
| `ref.enrollments` | learning, **game**, competition, chat | identity · `enrollment.changed` | `student_id`, `class_id`, `tenant_id`, `status`, `start_date`, `end_date` |
| `ref.player_profiles` | competition, chat | game · `game_profile.updated` | `student_id`, `tenant_id`, `nickname varchar(32)`, `avatar_thumb_key varchar(512)`, `level` |
| `ref.parties` | chat | competition · `party.created/disbanded`, `party.member_joined/left` | `party_id`, `tenant_id`, `member_ids uuid[]`, `status` |
| `ref.mastery_summary` | game | learning · `learning_objective.progressed` | `student_id`, `tenant_id`, `objective_id`, `level varchar(64)`, `value numeric(5,4)`, `source_version bigint`; PK `(student_id, objective_id)` (mở khóa khu/quest theo mastery) |
| `ref.friendships` | competition | chat · `friend.added`, `friend.removed` | `user_low_id`, `user_high_id` (chuẩn hóa thứ tự như `social.friendships`), `tenant_id`, `active boolean`, `source_version bigint`; PK `(user_low_id, user_high_id)` (ưu tiên bạn bè trong hub, mời party) — tạo ở F091, F100/F104 chỉ đọc |
| `ref.blocks` | competition | chat · `user.blocked`, `user.unblocked` | `blocker_id`, `blocked_id`, `tenant_id`, `active boolean`, `source_version bigint`; PK `(blocker_id, blocked_id)` (chặn mời party, ghép trận, duel) — tạo ở F091, F100/F104 chỉ đọc |
| `ref.login_activity` | learning | game · `student.game_logged_in/out` | `student_id`, `tenant_id`, `last_game_login_at`, `sessions_7d`, `minutes_7d` (read model dashboard) |
| `ref.platform_settings` | game | identity · `platform_settings.changed` (`scope = platform`; hiện chỉ `key = platform_curfew`) | `key varchar(128)` (PK), `value jsonb` (≤ 16 KB; `platform_curfew` = `{start, end}` giờ ICT), `source_version bigint` (`occurred_at` µs), `updated_at`; dữ liệu platform, không `tenant_id`/RLS; chỉ consumer `game.platform_settings` (role `game_platform`) ghi, `game_app` chỉ đọc. Chưa có dòng → mặc định khởi động `EW_PLATFORM_CURFEW` (OQ F014-2) [Confirmed] |

Quy tắc projection:
1. Mỗi dòng có `source_version bigint` (version của aggregate nguồn hoặc `occurred_at`); consumer chỉ áp bản mới hơn (chống event đến lệch thứ tự). `ref.schools`/`ref.classes` của learning (F020 T044): version aggregate luôn thay giá trị `occurred_at` (µs, > 10¹²), giá trị `occurred_at` không thay version aggregate [Confirmed].
2. Projection không có `deleted_at` riêng: phản ánh trạng thái nguồn (gồm `deleted_at` nguồn nếu cần lọc).
3. Có job **rebuild** projection qua facade của module nguồn (operation liệt kê theo trang trong spec internal, gọi trong tiến trình) khi tạo database mới hoặc phát hiện lệch [Confirmed].
4. Không ghi projection từ use case nghiệp vụ; sai lệch được sửa ở nguồn.
5. Bảng `ref.*` viết tay, không dùng mixin `entx` (không có cột audit/soft-delete riêng, F020). Bảng có `tenant_id` bật RLS điều kiện tenant **không** `FORCE`, để hàm `ref.projection_tenant(table, id)` (`SECURITY DEFINER`, chủ sở hữu bảng) trả tenant sở hữu một dòng cho consumer kiểm event lệch tenant (F020-AC-04); role ứng dụng không phải chủ sở hữu nên vẫn chịu RLS. Ở `eduworld_learning`, consumer `learning.projection_identity` cũng nhận `school.updated` cho `ref.schools`; `ref.classes.academic_year_id` để NULL đến khi `class.synced` mang trường này [Confirmed].

## 3. DDL mẫu (bảng then chốt)

### 3.0. Tenancy và quy ước soft-delete

```sql
CREATE TABLE platform.tenants (
  tenant_id          uuid PRIMARY KEY,
  code               varchar(64) NOT NULL,
  name               varchar(255) NOT NULL,
  tenant_type        varchar(64) NOT NULL CHECK (tenant_type IN ('school','unit','common')),
  status             varchar(64) NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
  keycloak_org_id    uuid,
  keycloak_org_alias varchar(64) CHECK (keycloak_org_alias ~ '^[a-z0-9-]{3,63}$'),
  login_code         varchar(16) CHECK (login_code ~ '^[A-HJ-NP-Z2-9]{6}$'),
  settings           jsonb NOT NULL DEFAULT '{}',
  is_qa              boolean NOT NULL DEFAULT false,  -- tenant QA: loại khỏi analytics/dashboard liên tenant
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  deleted_at         timestamptz,
  deleted_by         uuid,
  CHECK (NOT (is_qa AND tenant_type = 'common'))
);
CREATE UNIQUE INDEX tenants_code_uq      ON platform.tenants (code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX tenants_kc_alias_uq  ON platform.tenants (keycloak_org_alias) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX tenants_kc_org_uq    ON platform.tenants (keycloak_org_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX tenants_login_code_uq ON platform.tenants (login_code) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX tenants_single_common_uq ON platform.tenants (tenant_type) WHERE tenant_type = 'common';

-- Có trong mỗi database sở hữu tài nguyên gán được (learning, game, competition, chat); tenant_id FK tới ref.tenants
CREATE TABLE infra.tenant_resource_assignments (
  assignment_id uuid PRIMARY KEY,
  resource_type varchar(64) NOT NULL,
  resource_id   uuid NOT NULL,
  tenant_id     uuid NOT NULL REFERENCES ref.tenants(tenant_id),
  assigned_by   uuid NOT NULL,
  assigned_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz,
  deleted_by    uuid,
  delete_reason varchar(64)              -- F020: cột soft-delete chuẩn; module learning dùng mixin entx.Base (thêm created_*, updated_*, version)
);
CREATE UNIQUE INDEX tra_active_uq ON infra.tenant_resource_assignments (resource_type, resource_id, tenant_id)
  WHERE deleted_at IS NULL;
CREATE INDEX tra_tenant_idx ON infra.tenant_resource_assignments (tenant_id, resource_type, resource_id)
  WHERE deleted_at IS NULL;

-- ADR-0028: ứng dụng không bao giờ xóa vật lý. Migration nền của MỖI database chạy (thay schema list theo DB, §1.2):
-- eduworld_identity: platform, identity, org, sis, audit, infra   | eduworld_learning: curriculum, content, assessment, learning, lms, ref, infra
-- eduworld_game: game, gamification, ref, infra | eduworld_competition: multiplayer, competition, ref, infra | eduworld_chat: social, chat, ref, infra
REVOKE DELETE, TRUNCATE ON ALL TABLES IN SCHEMA <schemas_of_db> FROM <module>_app, <module>_platform;
ALTER DEFAULT PRIVILEGES FOR ROLE <module>_migrator REVOKE DELETE, TRUNCATE ON TABLES FROM <module>_app, <module>_platform;
REVOKE CONNECT ON DATABASE <db_of_other_modules> FROM <module>_app, <module>_platform;  -- không truy cập chéo database (cùng cụm)

-- ADR-0033 (chỉ trong eduworld_chat): ngoại lệ DUY NHẤT — role chat_purger chỉ được xóa dữ liệu chat gameplay.
GRANT SELECT, UPDATE, DELETE ON chat.gameplay_messages, chat.channels, chat.channel_members, chat.ephemeral_messages,
  chat.message_revisions, chat.message_reactions, chat.pinned_messages, chat.gameplay_partition_channels TO chat_purger;
-- purger chỉ xóa revision/reaction/pin của tin thuộc channel gameplay đã hết hạn (điều kiện trong query + RLS theo channel)
GRANT INSERT ON chat.moderation_evidence, chat.gameplay_chat_daily_stats, chat.gameplay_purge_runs, infra.outbox_events TO chat_purger;  -- audit gửi qua outbox (§1.3)
REVOKE TRUNCATE ON ALL TABLES IN SCHEMA chat FROM chat_purger;
-- RLS cho purger: chỉ thấy/xóa channel gameplay đã đóng
CREATE POLICY purger_gameplay_only ON chat.channels FOR ALL TO chat_purger
  USING (is_gameplay AND closed_at IS NOT NULL);
-- CI kiểm tra: ngoài các GRANT trên, không role ứng dụng nào có DELETE/TRUNCATE (test `db-privileges`).
```

Soft-delete chuẩn (mọi bảng T/G/H): `UPDATE <table> SET deleted_at = now(), deleted_by = $actor, delete_reason = $reason WHERE <pk> = $id AND tenant_id = $tenant AND deleted_at IS NULL`. Khôi phục: `SET deleted_at = NULL, deleted_by = NULL` sau khi kiểm tra partial unique, chỉ qua Thùng rác (ADR-0028). Job CI `tenancy-guard` kiểm tra grant và quy ước cột ([MULTITENANCY §11](../06-architecture/MULTITENANCY.md#11-test-bắt-buộc)).

### 3.1. Identity

```sql
CREATE TABLE identity.users (
  user_id            uuid PRIMARY KEY,
  user_type          varchar(64) NOT NULL CHECK (user_type IN ('student','staff')),  -- ADR-0081: không có teacher/admin
  status             varchar(64) NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending','active','locked','deactivated','anonymized')),
  username           varchar(128) NOT NULL,                -- đã chuẩn hóa trim/NFC/chữ thường; bản sao Keycloak (ADR-0067)
  email              varchar(320),                         -- bắt buộc với nhân sự; NULL được với học sinh
  display_name       varchar(128),                         -- NULL sau anonymize (DATA_PRIVACY §4)
  locale             varchar(64) NOT NULL DEFAULT 'vi-VN',
  primary_tenant_id  uuid REFERENCES platform.tenants(tenant_id),
  primary_school_id  uuid,
  created_at         timestamptz NOT NULL DEFAULT now(),
  updated_at         timestamptz NOT NULL DEFAULT now(),
  deactivated_at     timestamptz,
  anonymized_at      timestamptz,
  provisioned_at     timestamptz NOT NULL DEFAULT now(), -- mốc cho worker dọn khi chưa từng đăng nhập (ADR-0032)
  last_login_at      timestamptz,                  -- cập nhật khi tạo game session (ADR-0031)
  deleted_at         timestamptz,
  deleted_by         uuid,
  delete_reason      varchar(64),                         -- 'inactive_cleanup' | 'manual' | ...
  CHECK (status = 'anonymized' OR display_name IS NOT NULL),
  CHECK (username = lower(btrim(username))),
  CHECK (user_type = 'student' OR status = 'anonymized' OR email IS NOT NULL),  -- nhân sự bắt buộc email (ADR-0067)
  CHECK (user_type = 'staff' OR primary_tenant_id IS NOT NULL)  -- học sinh luôn có tenant gốc; staff không có tenant gốc = nhân sự platform (ADR-0081)
);
-- Định danh đăng nhập duy nhất trên SSO/toàn hệ thống (ADR-0067, ADR-0069) => cũng không trùng trong cùng tenant;
-- username đã soft-delete vẫn giữ chỗ đến khi ẩn danh hóa
CREATE UNIQUE INDEX users_username_uq ON identity.users (lower(username));
CREATE UNIQUE INDEX users_email_uq ON identity.users (lower(email))
  WHERE email IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX users_tenant_type_idx ON identity.users (primary_tenant_id, user_type) WHERE deleted_at IS NULL;
-- Worker dọn học sinh (ADR-0032): tìm ứng viên theo tenant
CREATE INDEX users_cleanup_idx ON identity.users (primary_tenant_id, (COALESCE(last_login_at, provisioned_at)))
  WHERE user_type = 'student' AND deleted_at IS NULL;

CREATE TABLE identity.external_identities (
  external_identity_id uuid PRIMARY KEY,
  issuer        varchar(512) NOT NULL,
  subject       varchar(255) NOT NULL,
  user_id       uuid NOT NULL REFERENCES identity.users(user_id),
  idp_alias     varchar(64),
  status        varchar(64) NOT NULL DEFAULT 'active' CHECK (status IN ('active','superseded')),
  superseded_at timestamptz,
  superseded_by uuid REFERENCES identity.external_identities(external_identity_id),
  created_at    timestamptz NOT NULL DEFAULT now(),
  last_login_at timestamptz,
  deleted_at    timestamptz,                  -- GlobalBase (ADR-0089), soft-delete ADR-0028
  deleted_by    uuid,
  CHECK ((status = 'superseded') = (superseded_at IS NOT NULL))
);
-- issuer = OIDC issuer đăng nhập (allowlist) hoặc nguồn tham chiếu không đăng nhập `platform_common` (external_ref học sinh tenant chung, ACCOUNT_PROVISIONING §3.4)
CREATE UNIQUE INDEX external_identities_iss_sub_uq
  ON identity.external_identities (issuer, subject) WHERE deleted_at IS NULL;
CREATE INDEX external_identities_user_idx ON identity.external_identities (user_id);
-- Một identity active cho mỗi (user, issuer); dòng superseded do job identity-relink giữ lại để truy vết
CREATE UNIQUE INDEX external_identities_active_uq
  ON identity.external_identities (user_id, issuer) WHERE status = 'active' AND deleted_at IS NULL;

-- Permission model (ADR-0027, ADR-0066): permission trong code, bản sao đọc-chỉ ghi bởi src/backend/cmd/migrate khi deploy.
CREATE TABLE identity.permissions (
  permission_code  varchar(64) PRIMARY KEY,
  scope_kind       varchar(64) NOT NULL CHECK (scope_kind IN ('tenant','platform')),
  group_code       varchar(64) NOT NULL,
  name_vi          varchar(255) NOT NULL,
  name_en          varchar(255) NOT NULL,
  catalog_version  varchar(64) NOT NULL,
  synced_at        timestamptz NOT NULL DEFAULT now(),
  -- Tiền tố platform-only luôn là permission platform
  CHECK (scope_kind = 'platform' OR NOT (permission_code LIKE 'game.%'
         OR permission_code LIKE 'competition.config.%' OR permission_code LIKE 'platform.%'))
);
REVOKE INSERT, UPDATE, DELETE ON identity.permissions FROM identity_app;  -- chỉ identity_migrator ghi

-- Role: seed hệ thống (is_system) + role tùy biến của tenant [Confirmed]
CREATE TABLE identity.roles (
  role_id          uuid PRIMARY KEY,
  code             varchar(64) NOT NULL,
  name_vi          varchar(255) NOT NULL,
  name_en          varchar(255) NOT NULL,
  description      varchar(1000),
  role_scope_kind  varchar(64) NOT NULL CHECK (role_scope_kind IN ('tenant','platform')),
  is_system        boolean NOT NULL DEFAULT false,
  tenant_id        uuid REFERENCES platform.tenants(tenant_id),   -- NULL = dùng chung
  version          integer NOT NULL DEFAULT 1,
  created_at       timestamptz NOT NULL DEFAULT now(),
  created_by       uuid,
  updated_at       timestamptz NOT NULL DEFAULT now(),
  updated_by       uuid,
  deleted_at       timestamptz,
  deleted_by       uuid,
  CHECK (role_scope_kind = 'tenant' OR tenant_id IS NULL),       -- role tùy biến chỉ là role tenant
  CHECK (NOT is_system OR tenant_id IS NULL)
);
CREATE UNIQUE INDEX roles_shared_code_uq ON identity.roles (code)
  WHERE tenant_id IS NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX roles_tenant_code_uq ON identity.roles (tenant_id, code)
  WHERE tenant_id IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE identity.role_permissions (
  role_id          uuid NOT NULL REFERENCES identity.roles(role_id),
  permission_code  varchar(64) NOT NULL REFERENCES identity.permissions(permission_code),
  tenant_id        uuid,                                           -- = roles.tenant_id (RLS)
  PRIMARY KEY (role_id, permission_code)
);

-- Lớp 2 sau use case CreateRole/UpdateRole: role tenant không nhận permission platform
CREATE FUNCTION identity.enforce_role_permission_scope() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE rkind text; pkind text;
BEGIN
  SELECT role_scope_kind INTO rkind FROM identity.roles WHERE role_id = NEW.role_id;
  SELECT scope_kind INTO pkind FROM identity.permissions WHERE permission_code = NEW.permission_code;
  IF rkind = 'tenant' AND pkind = 'platform' THEN
    RAISE EXCEPTION 'tenant role % must not hold platform permission %', NEW.role_id, NEW.permission_code
      USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER role_permissions_scope_chk
  BEFORE INSERT OR UPDATE ON identity.role_permissions
  FOR EACH ROW EXECUTE FUNCTION identity.enforce_role_permission_scope();

-- UserRole (thay role_assignments)
CREATE TABLE identity.user_roles (
  user_role_id           uuid PRIMARY KEY,
  user_id                uuid NOT NULL REFERENCES identity.users(user_id),
  role_id                uuid NOT NULL REFERENCES identity.roles(role_id),
  tenant_id              uuid REFERENCES platform.tenants(tenant_id), -- NULL khi scope_type = platform
  scope_type             varchar(64) NOT NULL
                         CHECK (scope_type IN ('platform','tenant','school','subject')),  -- ADR-0072: bỏ class
  scope_id               uuid,
  school_id              uuid,
  valid_from             timestamptz NOT NULL DEFAULT now(),
  valid_to               timestamptz,
  granted_by             uuid NOT NULL,
  source                 varchar(64) NOT NULL DEFAULT 'manual' CHECK (source IN ('manual')),  -- SIS không cấp role (ADR-0072)
  training_completed_at  timestamptz,
  created_at             timestamptz NOT NULL DEFAULT now(),
  deleted_at             timestamptz,
  deleted_by             uuid,
  CHECK ((scope_type = 'platform') = (tenant_id IS NULL)),
  CHECK (valid_to IS NULL OR valid_to > valid_from)
);
CREATE UNIQUE INDEX user_roles_active_uq ON identity.user_roles (user_id, role_id, scope_type, scope_id)
  WHERE deleted_at IS NULL AND valid_to IS NULL;
CREATE INDEX user_roles_user_tenant_idx ON identity.user_roles (user_id, tenant_id) WHERE deleted_at IS NULL;
CREATE INDEX user_roles_tenant_role_idx ON identity.user_roles (tenant_id, role_id) WHERE deleted_at IS NULL;

-- Lớp 2 sau use case GrantUserRole
CREATE FUNCTION identity.enforce_user_role_scope() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE r identity.roles%ROWTYPE; utype text; uprimary uuid;
BEGIN
  SELECT * INTO r FROM identity.roles WHERE role_id = NEW.role_id AND deleted_at IS NULL;
  IF NOT FOUND THEN
    RAISE EXCEPTION 'unknown role %', NEW.role_id USING ERRCODE = 'check_violation';
  END IF;
  IF r.role_scope_kind = 'platform' AND NEW.scope_type <> 'platform' THEN
    RAISE EXCEPTION 'platform role % cannot be assigned at scope %', r.code, NEW.scope_type
      USING ERRCODE = 'check_violation';
  END IF;
  IF r.role_scope_kind = 'tenant' AND NEW.scope_type = 'platform' THEN
    RAISE EXCEPTION 'tenant role % cannot be assigned at platform scope', r.code
      USING ERRCODE = 'check_violation';
  END IF;
  IF r.tenant_id IS NOT NULL AND r.tenant_id IS DISTINCT FROM NEW.tenant_id THEN
    RAISE EXCEPTION 'custom role % belongs to another tenant', r.code USING ERRCODE = 'check_violation';
  END IF;
  SELECT user_type, primary_tenant_id INTO utype, uprimary FROM identity.users WHERE user_id = NEW.user_id;
  IF utype = 'student' AND r.code <> 'student' THEN   -- học sinh không nhận role portal (ADR-0066)
    RAISE EXCEPTION 'student cannot hold portal role %', r.code USING ERRCODE = 'check_violation';
  END IF;
  -- ADR-0081: không có tenant gốc => chỉ role platform; có tenant gốc => chỉ role tenant
  IF uprimary IS NULL AND r.role_scope_kind <> 'platform' THEN
    RAISE EXCEPTION 'platform staff cannot hold tenant role %', r.code USING ERRCODE = 'check_violation';
  END IF;
  IF uprimary IS NOT NULL AND r.role_scope_kind = 'platform' THEN
    RAISE EXCEPTION 'tenant user cannot hold platform role %', r.code USING ERRCODE = 'check_violation';
  END IF;
  RETURN NEW;
END $$;

CREATE TRIGGER user_roles_scope_chk
  BEFORE INSERT OR UPDATE OF role_id, scope_type, tenant_id, user_id ON identity.user_roles
  FOR EACH ROW EXECUTE FUNCTION identity.enforce_user_role_scope();
```

### 3.2. Enrollment

```sql
CREATE TABLE org.enrollments (
  enrollment_id uuid PRIMARY KEY,
  tenant_id     uuid NOT NULL REFERENCES platform.tenants(tenant_id),
  school_id     uuid NOT NULL REFERENCES org.schools(school_id),
  student_id    uuid NOT NULL REFERENCES org.students(student_id),
  class_id      uuid NOT NULL REFERENCES org.classes(class_id),
  status        varchar(64) NOT NULL CHECK (status IN ('active','transferred','ended')),
  start_date    date NOT NULL,
  end_date      date,
  source        varchar(64) NOT NULL DEFAULT 'sis' CHECK (source IN ('sis','manual','platform_common')),  -- platform_common: học sinh tenant chung (F010)
  created_at    timestamptz NOT NULL DEFAULT now(),
  updated_at    timestamptz NOT NULL DEFAULT now(),
  deleted_at    timestamptz,
  deleted_by    uuid,
  delete_reason varchar(64),
  CHECK (end_date IS NULL OR end_date >= start_date)
);
-- Lớp phải thuộc đúng trường của enrollment (ADR-0060): FK ghép tới UNIQUE (class_id, school_id) của org.classes
ALTER TABLE org.enrollments
  ADD CONSTRAINT enrollments_class_school_fk FOREIGN KEY (class_id, school_id)
  REFERENCES org.classes (class_id, school_id);
-- Đúng một enrollment active (một trường, một lớp) cho mỗi học sinh
CREATE UNIQUE INDEX enrollments_one_active_uq ON org.enrollments (student_id)
  WHERE status = 'active' AND deleted_at IS NULL;
CREATE INDEX enrollments_class_idx ON org.enrollments (tenant_id, class_id, status) WHERE deleted_at IS NULL;
```

Chuyển lớp/trường: đóng enrollment cũ (`status='transferred'`, `end_date`) và tạo enrollment mới trong cùng transaction; lịch sử học tập giữ nguyên `tenant_id`/`school_id` tại thời điểm phát sinh. Chuyển trường trong cùng tenant `unit` giữ `tenant_id`. Chi tiết: [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md).

### 3.3. Question version (bất biến sau publish)

```sql
CREATE TABLE content.question_versions (
  question_version_id uuid PRIMARY KEY,
  question_id     uuid NOT NULL REFERENCES content.questions(question_id),
  tenant_id       uuid,                    -- NULL = nội dung platform (chỉ hiện với tenant được gán, ADR-0029)
  owner_scope     varchar(64) NOT NULL CHECK (owner_scope IN ('platform','tenant')),
  version_no      int  NOT NULL,           -- số version (F020: `version` là cột optimistic concurrency của mixin entx, ADR-0039)
  status          varchar(64) NOT NULL CHECK (status IN ('draft','in_review','approved','published','archived','rejected')),
  type            varchar(64) NOT NULL CHECK (type IN ('single_choice','multiple_choice','true_false','fill_in_blank',
                     'short_answer','numeric','matching','ordering','drag_drop','image_based','interactive')),
  stem            jsonb NOT NULL,
  render_payload  jsonb NOT NULL,          -- gửi client, KHÔNG có đáp án
  answer_key      jsonb NOT NULL,          -- chỉ server
  scoring_rule    jsonb NOT NULL,
  explanation     jsonb,
  difficulty      smallint NOT NULL CHECK (difficulty BETWEEN 1 AND 5),
  grade_levels    smallint[] NOT NULL CHECK (cardinality(grade_levels) BETWEEN 1 AND 9
                     AND grade_levels <@ ARRAY[1,2,3,4,5,6,7,8,9]::smallint[]),
  subject_id      uuid NOT NULL,
  topic_id        uuid NOT NULL,                       -- bắt buộc (CONTENT_MODEL)
  worked_example_ref jsonb CHECK (octet_length(worked_example_ref::text) <= 1024),  -- {content_item_id, block_id}
  media_refs      uuid[] NOT NULL DEFAULT '{}',
  author_id       uuid NOT NULL,
  reviewer_id     uuid,
  published_at    timestamptz,
  content_hash    bytea NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  deleted_at      timestamptz,             -- soft-delete chỉ cho draft/rejected; published dùng archived
  deleted_by      uuid,
  cloned_from_id  uuid, cloned_from_tenant_id uuid, cloned_at timestamptz, cloned_by uuid,
  CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  UNIQUE (question_id, version_no)
);
CREATE INDEX question_versions_delivery_idx
  ON content.question_versions (tenant_id, subject_id, topic_id, difficulty)
  WHERE status = 'published' AND deleted_at IS NULL;
CREATE INDEX question_versions_grades_gin ON content.question_versions USING gin (grade_levels);

-- Bất biến: không sửa nội dung version đã publish/archive (chỉ cho phép published -> archived)
CREATE FUNCTION content.forbid_published_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.status IN ('published','archived') AND
     NOT (OLD.status = 'published' AND NEW.status = 'archived'
          AND (NEW.stem, NEW.render_payload, NEW.answer_key, NEW.scoring_rule)
            IS NOT DISTINCT FROM (OLD.stem, OLD.render_payload, OLD.answer_key, OLD.scoring_rule)) THEN
    RAISE EXCEPTION 'CONTENT_NOT_EDITABLE' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER question_versions_immutable BEFORE UPDATE ON content.question_versions
  FOR EACH ROW EXECUTE FUNCTION content.forbid_published_update();

CREATE VIEW content.question_delivery_v WITH (security_barrier = true) AS
  SELECT question_version_id, question_id, tenant_id, owner_scope, version_no AS version, type, stem, render_payload, math_assets,
         max_score, difficulty, grade_levels, subject_id, topic_id, media_refs, content_language, exam_only, efficacy_instrument,
         worked_example_ref, tags, curriculum_version_id
  FROM content.question_versions WHERE status = 'published' AND deleted_at IS NULL;
-- Không có answer_key, scoring_rule, explanation, option_feedback, hints, min_response_ms (ADR-0016).
-- Lọc tenant (riêng tenant + platform được gán) theo MULTITENANCY §4.2: view chạy quyền owner + FORCE RLS của bảng gốc.
GRANT SELECT ON content.question_delivery_v TO learning_app, learning_platform, learning_readonly;
```

Bản triển khai F022 (migration `20261125000001_f022_question_bank.sql`): DDL ở trên là dạng rút gọn; bảng thật dùng mixin `entx.HybridContent` + `ContentVersionMixin` (cột `version` là optimistic concurrency, số version là `version_no`), các mảng `grade_levels`/`tags`/`media_refs` là `jsonb` (CHECK `jsonb_typeof = 'array'` + giới hạn phần tử) thay cho `smallint[]`/`uuid[]`, trigger dùng hàm chung `content.forbid_published_update()` của F020 (so toàn bộ dòng trừ cột bookkeeping) và `content.questions_release_target()` (§3.13). Cột nullable kiểu JSON lưu SQL `NULL` khi vắng (không lưu JSON `null`).

### 3.4. Attempts

```sql
CREATE TABLE assessment.attempts (
  attempt_id        uuid PRIMARY KEY,
  tenant_id         uuid NOT NULL,
  school_id         uuid NOT NULL,
  student_id        uuid NOT NULL,
  mode              varchar(64) NOT NULL CHECK (mode IN ('practice','assignment','quest','minigame','challenge','exam','competition','retry_wrong','diagnostic')),
  context_type      varchar(64) NOT NULL CHECK (context_type IN ('free_play','lesson','learning_path','recommendation','question_set',
                      'selection','assignment','quest_step','minigame_session','exam_session','match','challenge_attempt','boss_encounter')),  -- theo OpenAPI learning (+ mode challenge [Confirmed], F030)
  context_id        uuid NOT NULL,
  source_version_id uuid NOT NULL,          -- version nội dung nguồn
  status            varchar(64) NOT NULL CHECK (status IN ('in_progress','submitted','graded','expired','abandoned')),
  seed              bigint NOT NULL,
  started_at        timestamptz NOT NULL,
  deadline_at       timestamptz,
  submitted_at      timestamptz,
  graded_at         timestamptz,
  score             numeric(7,2),
  max_score         numeric(7,2),
  duration_ms       bigint,
  version           bigint NOT NULL DEFAULT 1,
  deleted_at        timestamptz,            -- chỉ dùng khi hủy attempt tạo nhầm (hiếm, có audit); kết quả đã chấm không xóa
  deleted_by        uuid,
  CHECK (status <> 'graded' OR (score IS NOT NULL AND graded_at IS NOT NULL)),
  CHECK (deleted_at IS NULL OR status NOT IN ('submitted','graded'))
);
CREATE INDEX attempts_student_idx ON assessment.attempts (tenant_id, student_id, started_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX attempts_context_idx ON assessment.attempts (tenant_id, context_type, context_id) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX attempts_one_open_uq ON assessment.attempts (student_id, context_type, context_id)
  WHERE status = 'in_progress' AND mode IN ('exam','assignment') AND deleted_at IS NULL;
```

Chống nộp trùng: `UPDATE assessment.attempts SET status='submitted', submitted_at=$now, version=version+1 WHERE attempt_id=$1 AND tenant_id=$tenant AND deleted_at IS NULL AND status='in_progress' AND version=$v` — 0 dòng ảnh hưởng → `ATTEMPT_ALREADY_SUBMITTED`.

### 3.5. Attempt answers (partitioned)

```sql
CREATE TABLE assessment.attempt_answers (
  attempt_answer_id uuid NOT NULL,
  attempt_id        uuid NOT NULL,
  position          int  NOT NULL,
  tenant_id         uuid NOT NULL,           -- append-only (TA): không có deleted_at
  school_id         uuid NOT NULL,
  response          jsonb NOT NULL,
  is_correct        boolean,               -- null khi chưa chấm
  score             numeric(6,2),
  answered_at       timestamptz NOT NULL,  -- thời gian server
  client_elapsed_ms int,
  response_ms       int,                   -- thời gian trả lời theo server (từ lúc hiển thị câu)
  hints_used        smallint NOT NULL DEFAULT 0 CHECK (hints_used >= 0),
  rapid_guess       boolean NOT NULL DEFAULT false,  -- response_ms < ngưỡng tối thiểu của item [Confirmed]
  idempotency_key   uuid NOT NULL,
  PRIMARY KEY (attempt_answer_id, answered_at),
  UNIQUE (attempt_id, idempotency_key, answered_at)
) PARTITION BY RANGE (answered_at);
CREATE INDEX attempt_answers_attempt_idx ON assessment.attempt_answers (attempt_id, position, answered_at DESC);
```

### 3.6. Learning events (partitioned)

```sql
CREATE TABLE learning.learning_events (
  event_id           uuid NOT NULL,          -- UUIDv7
  occurred_at        timestamptz NOT NULL,
  received_at        timestamptz NOT NULL DEFAULT now(),
  event_type         varchar(64) NOT NULL,          -- activity.submitted, question.answered, ...
  student_id         uuid NOT NULL,
  tenant_id          uuid NOT NULL,          -- append-only (TA)
  school_id          uuid NOT NULL,
  class_id           uuid,
  game_session_id    uuid,
  activity_id        uuid,
  content_id         uuid,
  content_version_id uuid,
  objective_ids      uuid[] NOT NULL DEFAULT '{}',
  context            varchar(64) NOT NULL,          -- practice|assignment|quest|exam|competition
  payload            jsonb NOT NULL,
  idempotency_key    varchar(64) NOT NULL,
  schema_version     smallint NOT NULL DEFAULT 1,
  PRIMARY KEY (event_id, occurred_at),
  UNIQUE (idempotency_key, occurred_at)
) PARTITION BY RANGE (occurred_at);

CREATE INDEX learning_events_student_idx ON learning.learning_events (tenant_id, student_id, occurred_at DESC);
CREATE INDEX learning_events_school_type_idx ON learning.learning_events (tenant_id, school_id, event_type, occurred_at);
CREATE INDEX learning_events_objectives_gin ON learning.learning_events USING gin (objective_ids);

CREATE TABLE learning.learning_events_2026_11 PARTITION OF learning.learning_events
  FOR VALUES FROM ('2026-11-01') TO ('2026-12-01');
```

`occurred_at` nằm trong key vì PostgreSQL yêu cầu cột partition trong mọi unique constraint; idempotency toàn cục được đảm bảo bổ sung bằng `infra.processed_events` / idempotency record ở tầng ứng dụng. Schema payload theo [EVENT_DESIGN](EVENT_DESIGN.md).

### 3.7. Party

```sql
CREATE TABLE multiplayer.parties (
  party_id                 uuid PRIMARY KEY,
  tenant_id                uuid NOT NULL,
  school_id                uuid NOT NULL,
  name                     varchar(128) NOT NULL CHECK (char_length(name) BETWEEN 1 AND 40),
  leader_id                uuid NOT NULL,
  party_type               varchar(64) NOT NULL CHECK (party_type IN ('friend','study_group','quest','competition_team','custom')),
  visibility               varchar(64) NOT NULL DEFAULT 'private' CHECK (visibility IN ('private','friends','class')),
  invitation_policy        varchar(64) NOT NULL DEFAULT 'leader_only' CHECK (invitation_policy IN ('leader_only','members')),
  max_members              smallint NOT NULL DEFAULT 4 CHECK (max_members BETWEEN 2 AND 8),
  status                   varchar(64) NOT NULL DEFAULT 'open' CHECK (status IN ('open','in_activity','disbanded')),
  current_activity_id      uuid,
  current_game_session_ref uuid,
  created_at               timestamptz NOT NULL DEFAULT now(),
  updated_at               timestamptz NOT NULL DEFAULT now(),
  disbanded_at             timestamptz,
  deleted_at               timestamptz,   -- giải tán dùng status/disbanded_at; deleted_at chỉ cho dữ liệu tạo nhầm
  deleted_by               uuid
);
CREATE INDEX parties_tenant_status_idx ON multiplayer.parties (tenant_id, status) WHERE deleted_at IS NULL;

CREATE TABLE multiplayer.party_members (
  party_id    uuid NOT NULL REFERENCES multiplayer.parties(party_id),
  student_id  uuid NOT NULL,
  tenant_id   uuid NOT NULL,
  school_id   uuid NOT NULL,
  role        varchar(64) NOT NULL CHECK (role IN ('leader','member')),
  joined_at   timestamptz NOT NULL DEFAULT now(),
  left_at     timestamptz,
  left_reason varchar(64) CHECK (left_reason IN ('left','kicked','disbanded','timeout','policy')),
  deleted_at  timestamptz,
  deleted_by  uuid,
  PRIMARY KEY (party_id, student_id, joined_at)
);
-- Một học sinh chỉ có một active party (Party Addendum §6, áp dụng Release 1.0)
CREATE UNIQUE INDEX party_members_one_active_party_uq ON multiplayer.party_members (student_id)
  WHERE left_at IS NULL AND deleted_at IS NULL;
-- Một leader active mỗi party
CREATE UNIQUE INDEX party_members_one_leader_uq ON multiplayer.party_members (party_id)
  WHERE left_at IS NULL AND role = 'leader' AND deleted_at IS NULL;
```

Kiểm tra `max_members` thực hiện trong transaction với `SELECT ... FROM multiplayer.parties WHERE party_id=$1 FOR UPDATE` rồi đếm member active.

### 3.8. Chat messages (partitioned) — ghi bởi `chat-persister` từ NATS JetStream (ADR-0035)

```sql
CREATE EXTENSION IF NOT EXISTS unaccent;
-- unaccent không IMMUTABLE nên bọc lại để dùng trong cột generated / index
CREATE FUNCTION chat.f_unaccent(text) RETURNS text
  LANGUAGE sql IMMUTABLE PARALLEL SAFE STRICT
  AS $$ SELECT public.unaccent('public.unaccent'::regdictionary, $1) $$;

CREATE TABLE chat.messages (
  message_id          uuid NOT NULL,             -- UUIDv7, cấp ở realtime, = Nats-Msg-Id
  channel_id          uuid NOT NULL,
  seq                 bigint NOT NULL,           -- cấp bằng Valkey INCR trước khi publish
  tenant_id           uuid NOT NULL,
  school_id           uuid,
  sender_id           uuid NOT NULL,
  client_message_id   uuid,
  kind                varchar(64) NOT NULL CHECK (kind IN ('text','emoji','quick','sticker','gif','system')),
  body                varchar(1000),                      -- nội dung gốc (bằng chứng; không trả cho API người dùng khi masked/recalled)
  body_display        varchar(1000),                      -- nội dung hiển thị (đã che)
  quick_message_id    uuid,
  sticker_id          uuid,
  gif_id              uuid,
  reply_to_message_id uuid,
  quote               jsonb,                     -- {message_id, excerpt} — excerpt cắt từ body_display
  forwarded_from      jsonb,                     -- {message_id, channel_type, sender_nickname}
  mentions            uuid[] NOT NULL DEFAULT '{}',
  revision            int NOT NULL DEFAULT 1,
  edit_count          int NOT NULL DEFAULT 0,
  edited_at           timestamptz,
  recalled_at         timestamptz,
  recalled_by         uuid,
  moderation_state    varchar(64) NOT NULL DEFAULT 'allowed'
                      CHECK (moderation_state IN ('allowed','masked','unchecked','held','removed')),
  filter_verdict      jsonb,                     -- category_codes, catalog_version, steps_skipped (không nội dung)
  search_tsv          tsvector GENERATED ALWAYS AS
                      (to_tsvector('simple', chat.f_unaccent(lower(coalesce(body_display, ''))))) STORED,
  created_at          timestamptz NOT NULL,
  deleted_at          timestamptz,               -- moderator gỡ = removed + soft-delete
  deleted_by          uuid,
  PRIMARY KEY (message_id, created_at),
  UNIQUE (channel_id, seq, created_at),
  CHECK (cardinality(mentions) <= 5)
) PARTITION BY RANGE (created_at);
CREATE INDEX messages_channel_seq_idx ON chat.messages (channel_id, seq DESC) WHERE deleted_at IS NULL;
CREATE INDEX messages_sender_idx      ON chat.messages (sender_id, created_at DESC);
CREATE INDEX messages_search_idx      ON chat.messages USING gin (search_tsv)
  WHERE deleted_at IS NULL AND recalled_at IS NULL AND moderation_state IN ('allowed','masked','unchecked');
CREATE INDEX messages_mentions_idx    ON chat.messages USING gin (mentions);

CREATE TABLE chat.message_revisions (           -- bản cũ khi sửa (ADR-0034), bằng chứng cho moderator
  tenant_id   uuid NOT NULL,
  message_id  uuid NOT NULL,
  revision    int  NOT NULL,
  body        varchar(1000),
  body_display varchar(1000),
  decision    varchar(64) NOT NULL,
  edited_at   timestamptz NOT NULL,
  PRIMARY KEY (message_id, revision)
);

CREATE TABLE chat.message_reactions (
  tenant_id   uuid NOT NULL,
  channel_id  uuid NOT NULL,
  message_id  uuid NOT NULL,
  user_id     uuid NOT NULL,
  emoji_code  varchar(64) NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,                       -- bỏ reaction = soft-delete
  PRIMARY KEY (message_id, user_id, emoji_code)
);
-- tối đa 3 reaction/người/tin kiểm tra ở use case

CREATE TABLE chat.pinned_messages (
  tenant_id   uuid NOT NULL,
  channel_id  uuid NOT NULL,
  message_id  uuid NOT NULL,
  pinned_by   uuid NOT NULL,
  pinned_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at  timestamptz,                       -- bỏ ghim = soft-delete
  deleted_by  uuid,
  PRIMARY KEY (channel_id, message_id)
);
-- ≤ 3 pin active/channel: kiểm tra trong use case + trigger đếm WHERE deleted_at IS NULL

ALTER TABLE chat.channel_members
  ADD COLUMN last_delivered_seq bigint NOT NULL DEFAULT 0,
  ADD COLUMN last_read_seq      bigint NOT NULL DEFAULT 0,   -- đã có, giữ
  ADD COLUMN unread_mentions    int    NOT NULL DEFAULT 0,
  ADD COLUMN show_read_receipts boolean NOT NULL DEFAULT true; -- [Confirmed] cho phép tắt "đã xem" ở DM
```

- Persister cập nhật con trỏ bằng `GREATEST(...)` (không lùi); `channels.last_seq = GREATEST(last_seq, $seq)`.
- Tin `block` không được publish nên không có dòng; tin `held` có dòng với `moderation_state = 'held'`.
- Tin thu hồi giữ `body` làm bằng chứng đến hết retention; API người dùng trả placeholder.
- `chat.messages` chỉ chứa tin của channel **bền vững** (class, study_group, team cố định, direct). Tin của channel gameplay nằm ở `chat.gameplay_messages`.

### 3.8a. Sticker và GIF (platform-only, Game Studio — ADR-0027/0034)

```sql
CREATE TABLE chat.sticker_packs (
  pack_id      uuid PRIMARY KEY,
  tenant_id    uuid,                         -- luôn NULL (platform); tenant thấy qua tenant_resource_assignments
  owner_scope  varchar(64) NOT NULL DEFAULT 'platform' CHECK (owner_scope = 'platform'),
  code         varchar(64) NOT NULL,
  name_vi      varchar(128) NOT NULL,
  name_en      varchar(128),
  unlock_rule  jsonb,                        -- NULL = mặc định cho mọi người; hoặc {reward_rule_id} / {item_id}
  status       varchar(64) NOT NULL CHECK (status IN ('draft','in_review','published','archived')),
  created_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz, deleted_by uuid
);
CREATE UNIQUE INDEX sticker_packs_code_uq ON chat.sticker_packs (code) WHERE deleted_at IS NULL;

CREATE TABLE chat.stickers (
  sticker_id   uuid PRIMARY KEY,
  pack_id      uuid NOT NULL REFERENCES chat.sticker_packs,
  asset_key    varchar(512) NOT NULL,                -- object storage / CDN (webp/png, ≤ 512 KB)
  alt_text_vi  varchar(1000) NOT NULL,                -- hỗ trợ đọc màn hình + hiển thị khi không tải được
  sort_order   int NOT NULL DEFAULT 0,
  deleted_at   timestamptz, deleted_by uuid
);

CREATE TABLE chat.gif_assets (
  gif_id       uuid PRIMARY KEY,
  tenant_id    uuid,                         -- luôn NULL (platform)
  owner_scope  varchar(64) NOT NULL DEFAULT 'platform' CHECK (owner_scope = 'platform'),
  asset_key    varchar(512) NOT NULL,                -- ≤ 2 MB, ≤ 480 px, có bản tĩnh preview
  preview_key  varchar(512) NOT NULL,
  width        int NOT NULL, height int NOT NULL,
  status       varchar(64) NOT NULL CHECK (status IN ('draft','in_review','published','archived')),
  reviewed_by  uuid, reviewed_at timestamptz,
  created_at   timestamptz NOT NULL DEFAULT now(),
  deleted_at   timestamptz, deleted_by uuid
);
CREATE TABLE chat.gif_tags (
  gif_id  uuid NOT NULL REFERENCES chat.gif_assets,
  tag     varchar(64) NOT NULL,                      -- chữ thường, không dấu (vd. 'co vu', 'cam on')
  locale  varchar(64) NOT NULL DEFAULT 'vi',
  PRIMARY KEY (gif_id, tag, locale)
);
CREATE INDEX gif_tags_tag_idx ON chat.gif_tags (tag);
```

### 3.8b. Gameplay messages (ADR-0033 — purge vật lý)

```sql
CREATE TABLE chat.gameplay_messages (
  LIKE chat.messages INCLUDING DEFAULTS INCLUDING GENERATED,   -- cùng cột tính năng với chat.messages
  created_on  date NOT NULL,                                    -- = created_at::date (persister điền)
  PRIMARY KEY (message_id, created_on),
  UNIQUE (channel_id, seq, created_on)
) PARTITION BY RANGE (created_on);
-- partition ngày: chat.gameplay_messages_YYYYMMDD, tạo trước 7 ngày bởi job partition-maintainer
CREATE INDEX gameplay_messages_channel_idx ON chat.gameplay_messages (channel_id, seq DESC) WHERE deleted_at IS NULL;
CREATE INDEX gameplay_messages_search_idx  ON chat.gameplay_messages USING gin (search_tsv)
  WHERE deleted_at IS NULL AND recalled_at IS NULL;

-- Reaction, revision, pin của channel gameplay dùng chung bảng chat.message_reactions /
-- chat.message_revisions / chat.pinned_messages; purge xóa theo channel_id/message_id.

CREATE TABLE chat.gameplay_partition_channels (   -- channel nào có tin trong partition ngày nào
  partition_day date NOT NULL,
  channel_id    uuid NOT NULL,
  tenant_id     uuid NOT NULL,
  purged_at     timestamptz,
  PRIMARY KEY (partition_day, channel_id)
);
```

**Lý do đổi từ partition theo `closed_on` sang theo ngày tạo** [Confirmed]: tin được ghi khi channel còn mở nên chưa biết ngày đóng. Partition theo `closed_on` buộc phải chuyển dòng giữa partition khi channel đóng (row movement, tốn I/O và cần quyền DELETE ngầm). Partition theo ngày tạo ghi một lần, không di chuyển.

Vòng đời:
1. Persister ghi tin vào partition `created_on` của ngày tạo và `INSERT ... ON CONFLICT DO NOTHING` vào `gameplay_partition_channels(created_on, channel_id)`.
2. Trận kết thúc / party giải tán: use case chat đặt `chat.channels.closed_at`, `purge_after = closed_at + gameplay_chat_retention_days` (theo tenant), phát `chat.channel_closed`.
3. Job `gameplay-chat-purge` (hằng giờ [Confirmed], role `chat_purger`) với mỗi channel có `purge_after < now()` và không bị hoãn (case mở): rollup `gameplay_chat_daily_stats` → sao chép tin + revision thuộc report/case/sanction sang `moderation_evidence` → xóa lô 1.000 dòng trong `gameplay_messages` (theo `channel_id`), `message_revisions`, `message_reactions`, `pinned_messages`, `channel_members`, `channels` → đánh dấu `gameplay_partition_channels.purged_at`.
4. Partition ngày `d` được `DETACH PARTITION ... CONCURRENTLY` + `DROP` khi **mọi** dòng `gameplay_partition_channels(partition_day = d)` đã `purged_at IS NOT NULL` (trận ngắn: thường sau `retention + 1` ngày; party sống lâu giữ partition đến khi party đó purge).
5. Ghi `gameplay_purge_runs` + audit tổng hợp; phát `chat.gameplay_purged`.

### 3.12. Game sessions & login events (ADR-0031)

```sql
CREATE TABLE game.game_sessions (
  game_session_id        uuid PRIMARY KEY,
  tenant_id              uuid NOT NULL REFERENCES ref.tenants(tenant_id),
  student_id             uuid NOT NULL,          -- tham chiếu ref.students (không FK chéo DB)
  school_id              uuid,
  device_id              uuid NOT NULL,
  device_model           varchar(128),
  client_platform        varchar(64) NOT NULL,
  client_version         varchar(64) NOT NULL,
  keycloak_sid           varchar(64),                -- để thu hồi Keycloak session khi bị takeover
  started_at             timestamptz NOT NULL DEFAULT now(),
  last_heartbeat_at      timestamptz NOT NULL DEFAULT now(),
  ended_at               timestamptz,
  end_reason             varchar(64) CHECK (end_reason IN ('logout','replaced_by_other_device','timeout','account_locked','access_revoked','cleanup','client_crash','curfew')),
  replaced_by_session_id uuid REFERENCES game.game_sessions(game_session_id),
  deleted_at             timestamptz,
  deleted_by             uuid,
  CHECK ((ended_at IS NULL) = (end_reason IS NULL))
);
-- Một thiết bị cho mỗi tài khoản: tối đa 1 session active/học sinh
CREATE UNIQUE INDEX game_sessions_one_active_uq ON game.game_sessions (student_id)
  WHERE ended_at IS NULL AND deleted_at IS NULL;
CREATE INDEX game_sessions_heartbeat_idx ON game.game_sessions (last_heartbeat_at) WHERE ended_at IS NULL;

CREATE TABLE game.login_events (
  login_event_id   uuid NOT NULL,
  tenant_id        uuid NOT NULL,
  student_id       uuid NOT NULL,
  game_session_id  uuid NOT NULL,
  device_id        uuid NOT NULL,
  platform         varchar(64) NOT NULL,
  device_model     varchar(128),
  client_version   varchar(64),
  ip_hash          bytea,                     -- HMAC-SHA256(IP, khóa xoay vòng); không lưu IP thô
  login_at         timestamptz NOT NULL,
  logout_at        timestamptz,
  end_reason       varchar(64),
  sis_connection_id uuid,                     -- SIS đã dùng để verify (ADR-0046); NULL với tenant chung
  PRIMARY KEY (login_event_id, login_at)
) PARTITION BY RANGE (login_at);
CREATE INDEX login_events_student_idx ON game.login_events (tenant_id, student_id, login_at DESC);
REVOKE DELETE, TRUNCATE ON game.login_events FROM game_app, game_platform;
```

Tạo session (một transaction trong `eduworld_game`, `SELECT pg_advisory_xact_lock(hashtextextended(student_id::text, 0))` để tuần tự hóa hai thiết bị đăng nhập cùng lúc):
1. Kiểm tra Principal (từ module identity, ADR-0040) có `user_type = 'student'` và `ref.students` còn hoạt động (khác → `GAME_ACCESS_STUDENT_ONLY`).
2. Tìm session active: có và `device_id` khác và `last_heartbeat_at` > now() − 45 s → nếu không `takeover` thì trả `409 ACTIVE_SESSION_ON_OTHER_DEVICE`; nếu `takeover` thì `UPDATE ... SET ended_at = now(), end_reason = 'replaced_by_other_device', replaced_by_session_id = $new`. Session cũ quá 45 s không heartbeat → kết thúc `timeout` rồi tạo mới không cần hỏi. Cùng `device_id` → kết thúc session cũ (`logout`) và tạo mới.
3. `INSERT game_sessions`, `INSERT login_events`, `UPDATE game.game_profiles SET last_game_login_at = now()`, outbox `student.game_logged_in` (+ `game_session.replaced` nếu takeover). module identity cập nhật `identity.users.last_login_at` khi tiêu thụ `student.game_logged_in` (bất đồng bộ, đủ cho job dọn ADR-0032).

### 3.13. Phát hành nội dung, ghim version, media (ADR-0043, ADR-0044)

Mẫu chung dùng cho **mọi** item có version trong `eduworld_learning` (`content.*`, `curriculum.*`; `lms.learning_paths` không có version, §2.14), `eduworld_game` (`game.quest_*`, `game.minigame_config_*`, `game.npc_dialogue_*`, `game.level_*`, `game.item_*`, `gamification.reward_rule_*`) và `eduworld_chat` (`chat.sticker_packs`, `chat.gif_*`). Mỗi item giữ con trỏ `current_version_id`; lịch sử con trỏ nằm ở bảng `content_releases` của schema tương ứng (ví dụ `content.content_releases`, `game.content_releases`).

```sql
-- Con trỏ release trên bảng định danh (ví dụ questions; các bảng item khác giống hệt).
-- Thay cột cũ current_published_version.
ALTER TABLE content.questions
  ADD COLUMN current_version_id uuid REFERENCES content.question_versions(question_version_id),  -- NULL = chưa publish / đã unpublish
  ADD COLUMN latest_version_no  int NOT NULL DEFAULT 0;

ALTER TABLE content.question_versions
  ADD COLUMN change_note         varchar(500),
  ADD COLUMN publish_at          timestamptz,          -- hẹn giờ (NULL = publish ngay khi gọi)
  ADD COLUMN snapshot_key        varchar(512),         -- khóa snapshot đã dựng (Valkey/CDN); NULL khi chưa publish
  ADD COLUMN snapshot_sha256     varchar(64),
  ADD COLUMN based_on_version_id uuid;                 -- draft tạo từ version nào

CREATE TABLE content.content_releases (
  release_id      uuid PRIMARY KEY,                    -- UUIDv7
  tenant_id       uuid,                                -- NULL = item platform
  item_type       varchar(64) NOT NULL,                -- question | question_set | lesson | assessment_blueprint | activity | curriculum
  item_id         uuid NOT NULL,
  action          varchar(16) NOT NULL CHECK (action IN ('publish','rollback','unpublish')),
  from_version_id uuid,
  to_version_id   uuid,                                -- NULL khi unpublish
  reason          varchar(500),
  scheduled_for   timestamptz,
  released_by     uuid NOT NULL,
  released_at     timestamptz NOT NULL DEFAULT now(),
  idempotency_key varchar(64) NOT NULL,
  CHECK (action = 'publish' OR reason IS NOT NULL),    -- rollback/unpublish bắt buộc lý do
  CHECK ((action = 'unpublish') = (to_version_id IS NULL)),
  UNIQUE (item_type, item_id, idempotency_key)
);  -- append-only: role app chỉ INSERT/SELECT
CREATE INDEX content_releases_item_idx ON content.content_releases (item_type, item_id, released_at DESC);

-- Con trỏ chỉ được trỏ tới version 'published' của chính item (kiểm tra trong use case + trigger)
CREATE FUNCTION content.check_release_target() RETURNS trigger LANGUAGE plpgsql AS $fn$
BEGIN
  IF NEW.current_version_id IS NOT NULL AND NOT EXISTS (
       SELECT 1 FROM content.question_versions v
       WHERE v.question_version_id = NEW.current_version_id
         AND v.question_id = NEW.question_id AND v.status = 'published' AND v.deleted_at IS NULL) THEN
    RAISE EXCEPTION 'ROLLBACK_TARGET_INVALID' USING ERRCODE = 'P0001';
  END IF;
  RETURN NEW;
END $fn$;
CREATE TRIGGER questions_release_target BEFORE UPDATE OF current_version_id ON content.questions
  FOR EACH ROW EXECUTE FUNCTION content.check_release_target();

-- Ghim version cho lượt chơi màn (eduworld_game)
CREATE TABLE game.level_sessions (
  level_session_id uuid PRIMARY KEY,
  tenant_id        uuid NOT NULL,
  student_id       uuid NOT NULL,
  game_session_id  uuid NOT NULL,
  level_id         uuid NOT NULL,
  level_version_id uuid NOT NULL,
  manifest_version bigint NOT NULL,
  content_versions jsonb NOT NULL,                     -- {"quest:<id>": "<version_id>", ...}
  status           varchar(16) NOT NULL CHECK (status IN ('active','completed','abandoned','expired')),
  started_at       timestamptz NOT NULL DEFAULT now(),
  ended_at         timestamptz,
  deleted_at timestamptz, deleted_by uuid,
  CHECK (octet_length(content_versions::text) <= 65536)
);
CREATE UNIQUE INDEX level_sessions_one_active ON game.level_sessions (student_id)
  WHERE status = 'active' AND deleted_at IS NULL;      -- một màn đang chơi / học sinh

-- Content manifest theo tenant (eduworld_game)
CREATE TABLE game.content_manifests (
  tenant_id        uuid NOT NULL,
  manifest_version bigint NOT NULL,
  etag             varchar(64) NOT NULL,
  entries          jsonb NOT NULL,                     -- item_type, item_id, version_id, snapshot_url, sha256; bundles; areas
  generated_at     timestamptz NOT NULL DEFAULT now(),
  cause_release_id uuid,
  PRIMARY KEY (tenant_id, manifest_version),
  CHECK (octet_length(entries::text) <= 4194304)
);

-- Metadata media (mỗi DB sở hữu nội dung: content.media_assets, game.media_assets; chat dùng stickers/gif_assets)
CREATE TABLE content.media_assets (
  asset_id          uuid PRIMARY KEY,
  tenant_id         uuid,
  owner_scope       varchar(64) NOT NULL CHECK (owner_scope IN ('platform','tenant')),
  kind              varchar(16) NOT NULL CHECK (kind IN ('image','audio','video','model3d','document')),
                    -- game.media_assets thêm 'render_thumbnail','render_turntable' (ảnh/turntable do render_jobs sinh)
  original_filename varchar(255) NOT NULL,
  mime_type         varchar(127) NOT NULL,
  size_bytes        bigint NOT NULL CHECK (size_bytes > 0),
  width int, height int, duration_ms int,
  sha256            varchar(64) NOT NULL,
  bucket            varchar(64) NOT NULL CHECK (bucket IN ('ew-media-public','ew-media-private')),
  variants          jsonb NOT NULL DEFAULT '{}',       -- {"webp_1024": {"key": "media/<sha>/webp_1024.webp", "bytes": 1}}
  alt_text          varchar(500),
  license           varchar(255) NOT NULL,
  status            varchar(16) NOT NULL CHECK (status IN ('quarantined','processing','ready','rejected')),
  reject_reason     varchar(255),
  quarantine_key    varchar(512) NOT NULL,             -- F027: quarantine/<asset_id> trong ew-media-quarantine
  is_exam_material  boolean NOT NULL DEFAULT false,    -- F027: media đề kiểm tra → luôn ew-media-private
  upload_completed_at timestamptz, processed_at timestamptz,  -- F027
  uploaded_by       uuid NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz, deleted_by uuid,
  CHECK ((owner_scope = 'platform') = (tenant_id IS NULL)),
  CHECK (octet_length(variants::text) <= 16384)
);
CREATE UNIQUE INDEX media_assets_sha_scope_uq
  ON content.media_assets (coalesce(tenant_id, '00000000-0000-0000-0000-000000000000'::uuid), sha256)
  WHERE deleted_at IS NULL;                            -- cùng file trong cùng phạm vi → dùng lại asset
CREATE TABLE content.media_usages (                    -- "đang dùng ở version nào"; chặn soft-delete asset còn được tham chiếu
  media_usage_id uuid PRIMARY KEY,                     -- F027: khóa thay thế (soft-delete khi bản nháp bỏ tham chiếu)
  tenant_id  uuid,                                     -- F027: chủ sở hữu version (NULL = platform), RLS H theo assignment của asset
  owner_scope varchar(64) NOT NULL,
  asset_id   uuid NOT NULL REFERENCES content.media_assets(asset_id),
  item_type  varchar(64) NOT NULL,
  item_id    uuid NOT NULL,                            -- F027
  version_id uuid NOT NULL,
  version_no int NOT NULL,                             -- F027 (used_in trả số version)
  -- cột chuẩn ADR-0039 (created_*, updated_*, deleted_*, version)
  CHECK ((owner_scope = 'platform') = (tenant_id IS NULL))
);
CREATE UNIQUE INDEX media_usages_uq ON content.media_usages (asset_id, item_type, version_id) WHERE deleted_at IS NULL;
```

Ghim version ở các bảng khác: `game.quest_progress.quest_version_id`, `assessment.attempts.source_version_id`, `assessment.exam_sessions.blueprint_version_id`, `learning.lesson_sessions.lesson_version_id`, `lms.assignments.pinned_version_id` + `follow_latest boolean NOT NULL DEFAULT false`, `competition.matches.question_plan_version` (§2.6, §2.8, §2.10, §2.14).

### 3.14. Nguồn SIS của học sinh (ADR-0046) · DB `eduworld_identity`

```sql
-- Một trường chỉ có một connection hoạt động
CREATE TABLE sis.sis_connection_schools (
  sis_connection_id uuid NOT NULL REFERENCES sis.sis_connections(sis_connection_id),
  school_id         uuid NOT NULL,
  tenant_id         uuid NOT NULL,
  active            boolean NOT NULL DEFAULT true,
  assigned_by       uuid NOT NULL,
  assigned_at       timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz, deleted_by uuid,
  PRIMARY KEY (sis_connection_id, school_id)
);
CREATE UNIQUE INDEX sis_connection_schools_one_active
  ON sis.sis_connection_schools (school_id) WHERE active AND deleted_at IS NULL;   -- vi phạm → SIS_SCHOOL_ALREADY_CONNECTED

-- Bản ghi học sinh đồng bộ: khóa nguồn (sis_connection_id, student_external_id)
CREATE TABLE sis.synced_students (
  sis_connection_id   uuid NOT NULL,
  student_external_id varchar(64) NOT NULL,
  tenant_id           uuid NOT NULL,
  school_id           uuid NOT NULL,
  student_code        varchar(32) NOT NULL,
  record_status       varchar(32) NOT NULL CHECK (record_status IN ('active','inactive','transferred_out','graduated','suspended')),
  national_student_id_hash varchar(64),
  last_hash           varchar(64) NOT NULL,
  last_synced_at      timestamptz NOT NULL,
  deleted_at timestamptz, deleted_by uuid,
  PRIMARY KEY (sis_connection_id, student_external_id)
);

-- Khóa đăng nhập lấy từ DTO contract (login.username, login.idp_alias + login.idp_subject)
CREATE TABLE sis.student_login_keys (
  sis_connection_id   uuid NOT NULL,
  login_type          varchar(16) NOT NULL CHECK (login_type IN ('username','idp_subject')),
  idp_alias           varchar(64),                       -- NULL khi login_type = username
  login_key           varchar(255) NOT NULL,
  student_external_id varchar(64) NOT NULL,
  tenant_id           uuid NOT NULL,
  updated_at          timestamptz NOT NULL DEFAULT now(),
  deleted_at timestamptz,
  FOREIGN KEY (sis_connection_id, student_external_id) REFERENCES sis.synced_students (sis_connection_id, student_external_id)
);
CREATE UNIQUE INDEX student_login_keys_uq
  ON sis.student_login_keys (sis_connection_id, login_type, coalesce(idp_alias, ''), login_key) WHERE deleted_at IS NULL;

-- Liên kết bền vững User ↔ bản ghi SIS
CREATE TABLE identity.sis_identity_links (
  link_id             uuid PRIMARY KEY,
  user_id             uuid NOT NULL REFERENCES identity.users(user_id),
  tenant_id           uuid NOT NULL,
  school_id           uuid NOT NULL,
  sis_connection_id   uuid NOT NULL,
  student_external_id varchar(64) NOT NULL,
  status              varchar(16) NOT NULL CHECK (status IN ('active','superseded')),
  linked_at           timestamptz NOT NULL DEFAULT now(),
  superseded_at       timestamptz,
  superseded_by       uuid
);
CREATE UNIQUE INDEX sis_identity_links_one_active ON identity.sis_identity_links (user_id) WHERE status = 'active';
CREATE UNIQUE INDEX sis_identity_links_source_active
  ON identity.sis_identity_links (sis_connection_id, student_external_id) WHERE status = 'active';
```

`identity.login_events` và `game.login_events` có thêm cột `sis_connection_id uuid NULL` (chỉ ID connection đã dùng để verify; module game nhận qua kết quả `principals:resolve`/projection). Bảng `sis.sis_links` giữ cho các entity khác (school, class, enrollment); học sinh dùng `sis.synced_students` + `identity.sis_identity_links`.

### 3.15. Feature flag, danh mục menu portal, token đặt lại mật khẩu (ADR-0090, ADR-0091) · DB `eduworld_identity`

```sql
-- Bản sao đọc-chỉ của specs/portal/menu-catalog.yaml (ADR-0090). Chỉ src/backend/cmd/migrate ghi lúc deploy (role `identity_migrator`);
-- key bị gỡ khỏi catalog → soft-delete (deleted_at), không xóa vật lý để giữ FK của feature_flags và lịch sử audit.
CREATE TABLE platform.portal_menu_catalog (
  item_key             varchar(128) PRIMARY KEY
                       CHECK (item_key ~ '^(school_lms|content_studio|game_ops|live_ops|platform_admin)$'
                           OR item_key ~ '^menu\.[a-z_]+\.[a-z0-9_]+$'
                           OR item_key ~ '^action\.[a-z_]+\.[a-z0-9_]+\.[a-z0-9_]+$'),
  item_kind            varchar(16)  NOT NULL CHECK (item_kind IN ('workspace','menu','action')),
  workspace_key        varchar(32)  NOT NULL CHECK (workspace_key IN ('school_lms','content_studio','game_ops','live_ops','platform_admin')),
  parent_key           varchar(128) REFERENCES platform.portal_menu_catalog(item_key),  -- menu → workspace, action → menu
  path                 varchar(256),                       -- chỉ menu
  label_key            varchar(128) NOT NULL,
  title                varchar(200),
  required_permissions varchar(64)[] NOT NULL DEFAULT '{}',   -- any-of; mỗi phần tử có trong identity.permissions (kiểm ở src/backend/cmd/migrate)
  operation_ids        varchar(128)[] NOT NULL DEFAULT '{}',  -- chỉ action; dạng operationId hoặc operationId[param=value]
  sort_order           integer NOT NULL DEFAULT 0,
  can_disable          boolean NOT NULL DEFAULT true,
  catalog_version      varchar(64) NOT NULL,                -- sha256 của file catalog lúc đồng bộ
  synced_at            timestamptz NOT NULL DEFAULT now(),
  deleted_at           timestamptz,
  deleted_by           uuid,
  delete_reason        varchar(64),
  CHECK ((item_kind = 'workspace') = (parent_key IS NULL)),
  CHECK (item_kind = 'action' OR cardinality(operation_ids) = 0),
  CHECK (item_kind = 'menu' OR path IS NULL)
);
CREATE INDEX portal_menu_catalog_parent_idx ON platform.portal_menu_catalog (parent_key) WHERE deleted_at IS NULL;
REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON platform.portal_menu_catalog FROM identity_app, identity_platform;
GRANT SELECT ON platform.portal_menu_catalog TO identity_app, identity_platform;  -- chỉ identity_migrator ghi

-- Feature flag (F015) mở rộng cho menu/action portal (ADR-0090). Không có dòng = bật.
CREATE TABLE platform.feature_flags (
  flag_key          varchar(128) PRIMARY KEY CHECK (flag_key ~ '^[a-z0-9_.]{3,128}$'),
  flag_kind         varchar(16)  NOT NULL DEFAULT 'feature' CHECK (flag_kind IN ('feature','menu','action')),
  parent_key        varchar(128),                               -- action → menu cha; NULL với feature/menu
  description       varchar(1000),
  default_enabled   boolean NOT NULL DEFAULT true,
  tenant_overrides  jsonb NOT NULL DEFAULT '{}'                 -- {"<tenant_id>": true|false}
                    CHECK (jsonb_typeof(tenant_overrides) = 'object' AND octet_length(tenant_overrides::text) <= 65536),
  catalog_key       varchar(128) GENERATED ALWAYS AS (CASE WHEN flag_kind <> 'feature' THEN flag_key END) STORED,
  version           integer NOT NULL DEFAULT 1,                 -- If-Match
  created_at        timestamptz NOT NULL DEFAULT now(),
  updated_at        timestamptz NOT NULL DEFAULT now(),
  updated_by        uuid,
  deleted_at        timestamptz,
  deleted_by        uuid,
  delete_reason     varchar(64),
  CHECK ((flag_kind = 'action') = (parent_key IS NOT NULL)),
  -- FK chỉ có hiệu lực với menu/action: catalog_key NULL với flag_kind = feature nên không bị kiểm.
  FOREIGN KEY (catalog_key) REFERENCES platform.portal_menu_catalog(item_key),
  FOREIGN KEY (parent_key)  REFERENCES platform.portal_menu_catalog(item_key)
);
CREATE INDEX feature_flags_kind_idx ON platform.feature_flags (flag_kind) WHERE deleted_at IS NULL;

-- Token đặt lại mật khẩu portal (ADR-0091 mục 3). Chỉ nhân sự; không lưu token thô, không lưu IP thô.
CREATE TABLE identity.portal_password_reset_tokens (
  reset_token_id    uuid PRIMARY KEY,                                   -- UUIDv7
  user_id           uuid NOT NULL REFERENCES identity.users(user_id),
  token_hash        char(64) NOT NULL,                                  -- SHA-256 hex của token gửi trong email
  locale            varchar(8) NOT NULL DEFAULT 'vi',
  expires_at        timestamptz NOT NULL,                               -- created_at + portal_password_reset_ttl
  used_at           timestamptz,
  created_at        timestamptz NOT NULL DEFAULT now(),
  deleted_at        timestamptz,                                        -- token bị thay bởi token mới hoặc thu hồi
  deleted_by        uuid,
  delete_reason     varchar(64) CHECK (delete_reason IN ('superseded','revoked','user_deactivated')),
  CHECK (expires_at > created_at)
);
CREATE UNIQUE INDEX portal_password_reset_tokens_hash_uq ON identity.portal_password_reset_tokens (token_hash);
-- Một token còn hiệu lực cho mỗi user: cấp token mới phải soft-delete token cũ (delete_reason = 'superseded') trong cùng transaction.
CREATE UNIQUE INDEX portal_password_reset_tokens_active_uq
  ON identity.portal_password_reset_tokens (user_id) WHERE used_at IS NULL AND deleted_at IS NULL;

-- Dùng một lần, nguyên tử (password:reset):
-- UPDATE identity.portal_password_reset_tokens SET used_at = now()
--  WHERE token_hash = $1 AND used_at IS NULL AND deleted_at IS NULL AND expires_at > now()
--  RETURNING user_id;     -- 0 dòng → 401 PASSWORD_RESET_TOKEN_INVALID
```

- `portal_menu_catalog` cùng mẫu với `identity.permissions` (§2.1): nguồn là contract, `src/backend/cmd/migrate` upsert theo `item_key`, đặt `catalog_version`, soft-delete key không còn trong file; nếu còn flag trỏ tới key bị gỡ, migrate cảnh báo và giữ dòng catalog (đã soft-delete) để FK hợp lệ. `identity_app` chỉ `SELECT`.
- `feature_flags`: use case `PutFeatureFlag` suy `flag_kind`/`parent_key` từ catalog (key menu/action không có trong catalog → `404`; `can_disable = false` → `422 FEATURE_NOT_DISABLEABLE`), bắt buộc `If-Match` + `reason`, ghi audit trước/sau và outbox `platform_settings.changed` (scope `feature_flag`) trong cùng transaction. Giá trị hiệu lực của action = flag action AND flag menu cha; mỗi flag = `tenant_overrides[tenant]` nếu có, ngược lại `default_enabled`.
- `portal_password_reset_tokens`: loại G (`GlobalBase`, không RLS) vì nhân sự platform không có tenant; chỉ module identity đọc/ghi. Token hết hạn/đã dùng giữ nguyên dòng (soft-delete theo ADR-0028, không xóa vật lý); retention theo `login_events_retention` [Confirmed] bằng ẩn danh hóa `token_hash`.

### 3.9. Outbox & processed events

```sql
CREATE TABLE infra.outbox_events (
  event_id        uuid NOT NULL,                   -- UUIDv7, cũng là Nats-Msg-Id
  event_type      varchar(64) NOT NULL,
  subject         varchar(128) NOT NULL,           -- 'ew.evt.learning.activity.submitted'
  tenant_id       uuid,                            -- NULL cho event platform
  partition_key   varchar(128) NOT NULL,           -- vd. student_id, party_id — ordering
  envelope        jsonb NOT NULL CHECK (octet_length(envelope::text) <= 65536), -- envelope chuẩn EVENT_DESIGN §2 (gồm traceparent, schema_version)
  created_at      timestamptz NOT NULL DEFAULT now(),
  published_at    timestamptz,
  attempts        int NOT NULL DEFAULT 0,
  PRIMARY KEY (event_id, created_at)               -- cột partition phải nằm trong PK
) PARTITION BY RANGE (created_at);                 -- partition theo ngày (ADR-0089 §5)
CREATE INDEX outbox_unpublished_idx ON infra.outbox_events (event_id) WHERE published_at IS NULL;

CREATE TABLE infra.processed_events (
  consumer          varchar(128) NOT NULL,         -- '<module>.<handler>'
  event_id          uuid NOT NULL,
  event_occurred_at timestamptz NOT NULL,          -- lấy từ envelope.occurred_at, bất biến với mỗi event
  processed_at      timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (consumer, event_id, event_occurred_at)
) PARTITION BY RANGE (event_occurred_at);          -- partition theo ngày; event giao lại luôn rơi vào cùng partition (ADR-0089 §4)

CREATE TABLE infra.dead_letters (
  dead_letter_id  uuid NOT NULL,                   -- UUIDv7
  stream          varchar(64) NOT NULL,
  consumer        varchar(128) NOT NULL,
  event_id        uuid NOT NULL,
  event_type      varchar(64) NOT NULL,
  payload         jsonb NOT NULL CHECK (octet_length(payload::text) <= 65536),
  error           varchar(1000) NOT NULL,          -- không PII
  delivery_count  int NOT NULL,
  created_at      timestamptz NOT NULL DEFAULT now(),
  resolved_at     timestamptz,
  resolved_by     uuid,
  PRIMARY KEY (dead_letter_id, created_at),
  UNIQUE (consumer, event_id, created_at)
) PARTITION BY RANGE (created_at);                 -- partition theo ngày (ADR-0089 §4–5)

-- Bảng hạ tầng: KHÔNG bật RLS (ADR-0089 §4). tenant_id nằm trong envelope để consumer dựng TenantScope.
GRANT INSERT ON infra.outbox_events TO <module>_app;                       -- ghi cùng transaction trong UnitOfWork
GRANT INSERT ON infra.outbox_events TO <module>_platform;                  -- F010: transaction PlatformScope/consumer platform cũng phát event
-- <module>_platform cũng INSERT/SELECT processed_events, idempotency_records và INSERT dead_letters (migration <version>_infra_platform_grants)
GRANT INSERT, SELECT ON infra.processed_events, infra.idempotency_records TO <module>_app;
GRANT INSERT ON infra.dead_letters TO <module>_app;
GRANT SELECT, UPDATE (published_at, attempts) ON infra.outbox_events TO <module>_relay;
GRANT SELECT, UPDATE (resolved_at, resolved_by) ON infra.dead_letters TO <module>_platform;  -- màn xử lý event lỗi
-- <module>_partition_admin: chỉ CREATE/DETACH/DROP partition trên schema infra; không role nào có DELETE/TRUNCATE trên infra.*
```

Mỗi database có bộ bảng này. Relay của module (role `<module>_relay`, profile `worker-events`) publish lên NATS JetStream `ew.evt.<module>.<event_type>` (ADR-0042) và chỉ cập nhật `published_at`, `attempts`. Consumer ghi `processed_events` với `event_occurred_at` của envelope (không dùng giờ xử lý), nên PK chặn trùng cả khi event được giao lại sang ngày khác.

**Retention bằng gỡ partition (ngoại lệ có giới hạn của ADR-0070, ADR-0089 §5).** Bốn bảng `infra.outbox_events`, `infra.processed_events`, `infra.idempotency_records`, `infra.dead_letters` partition theo ngày. Job `infra-partition-maintenance` (role `<module>_partition_admin`) gỡ partition (`DETACH` + `DROP`) sau thời hạn retention: `outbox_retention` (30 ngày) cho outbox đã publish và `processed_events`, `idempotency_record_ttl` (24 giờ) cho idempotency ([CONFIG_DEFAULTS](../00-overview/CONFIG_DEFAULTS.md)), retention dead letter của [EVENT_DESIGN §8](EVENT_DESIGN.md#8-retention). Job từ chối gỡ partition còn dòng outbox chưa publish hoặc dead letter chưa xử lý (`resolved_at IS NULL`) và phát cảnh báo. Bảng này không chứa dữ liệu học sinh ngoài định danh trong envelope và payload không chứa PII (R14). Ngoại lệ chỉ áp cho bốn bảng trên (và chat gameplay theo ADR-0033). Mọi bảng nghiệp vụ partition vẫn theo ADR-0070 (§5). Job cũng tạo partition ngày tiếp theo trước tối thiểu 7 ngày; thiếu partition là cảnh báo P2. DDL partition viết tay theo §7.1.

### 3.10. Idempotency records

```sql
CREATE TABLE infra.idempotency_records (
  user_id         uuid NOT NULL,
  method          varchar(16) NOT NULL,
  route_template  varchar(255) NOT NULL,
  idem_key        uuid NOT NULL,                   -- Idempotency-Key, bắt buộc UUIDv7
  key_issued_on   date NOT NULL,                   -- ngày (UTC) lấy từ timestamp trong UUIDv7 của key
  request_hash    bytea NOT NULL,
  response_status smallint NOT NULL,
  response_body   jsonb CHECK (response_body IS NULL OR octet_length(response_body::text) <= 65536),
  created_at      timestamptz NOT NULL DEFAULT now(),
  expires_at      timestamptz NOT NULL,
  PRIMARY KEY (user_id, route_template, idem_key, key_issued_on)
) PARTITION BY RANGE (key_issued_on);              -- partition theo ngày; retry luôn rơi vào cùng partition (ADR-0089 §4)
CREATE INDEX idempotency_expires_idx ON infra.idempotency_records (expires_at);
-- Không bật RLS. INSERT một lần trong cùng transaction UnitOfWork với thay đổi nghiệp vụ, kèm response đã tuần tự hóa;
-- không có trạng thái in_progress, không UPDATE. Request trùng đồng thời chờ ở unique index tới khi transaction đầu commit
-- rồi đọc và trả lại response đã lưu. Key không phải UUIDv7 hoặc lệch quá idempotency_record_ttl so với giờ server → 400 VALIDATION_ERROR.
-- Bản ghi hết hạn bị bỏ qua khi đọc (expires_at < now()); partition quá idempotency_record_ttl được job infra-partition-maintenance gỡ (§3.9).
```

### 3.11. Audit logs (partitioned) · DB `eduworld_identity`

```sql
CREATE TABLE audit.audit_logs (
  audit_id      uuid NOT NULL,
  occurred_at   timestamptz NOT NULL,
  actor_id      uuid,                         -- null = hệ thống
  actor_type    varchar(64) NOT NULL,                -- user|service|system
  tenant_id     uuid,                         -- NULL cho thao tác platform; cross_tenant.accessed ghi tenant bị truy cập
  school_id     uuid,
  action        varchar(64) NOT NULL,                -- vd. party.member_kicked, chat.case_actioned, role.assigned
  target_type   varchar(64) NOT NULL,
  target_id     varchar(128) NOT NULL,
  result        varchar(64) NOT NULL CHECK (result IN ('success','denied','error')),
  reason        varchar(1000),
  metadata      jsonb NOT NULL DEFAULT '{}',  -- không chứa PII/nội dung chat
  trace_id      varchar(64),
  ip_hash       bytea,
  producer      varchar(64) NOT NULL,          -- <module>/<thành phần> của envelope (F015)
  PRIMARY KEY (audit_id, occurred_at)
) PARTITION BY RANGE (occurred_at);              -- partition tháng, KHÔNG có partition DEFAULT (F015-AC-03)
CREATE INDEX audit_tenant_time_idx ON audit.audit_logs (tenant_id, occurred_at DESC);
CREATE INDEX audit_target_idx ON audit.audit_logs (target_type, target_id);

REVOKE UPDATE, DELETE ON audit.audit_logs FROM identity_app, identity_platform; -- append-only (audit nằm trong eduworld_identity)
-- F015: RLS (identity_app đọc tenant của mình; sink và System Admin qua identity_platform: SELECT, INSERT). Partition tháng và
-- ẩn danh hóa theo retention chạy bằng hàm SECURITY DEFINER audit.ensure_monthly_partitions, audit.monthly_partitions_ahead,
-- audit.anonymize_expired (EXECUTE cho identity_partition_admin) — không role nào khác có UPDATE trên bảng.
```

## 4. Row-Level Security

Mẫu áp cho mọi bảng **T**/**TA**/**H** trong **mọi database** (sinh tự động trong migration bằng hàm helper). Mô hình khái niệm: [MULTITENANCY §5](../06-architecture/MULTITENANCY.md).

```sql
CREATE OR REPLACE FUNCTION infra.current_tenant_id() RETURNS uuid
LANGUAGE sql STABLE AS $$
  SELECT NULLIF(current_setting('app.tenant_id', true), '')::uuid
$$;

-- Bảng tenant-scoped (T/TA/T*): policy CHỈ có điều kiện tenant (ADR-0089 §1)
ALTER TABLE assessment.attempts ENABLE ROW LEVEL SECURITY;
ALTER TABLE assessment.attempts FORCE ROW LEVEL SECURITY;

CREATE POLICY tenant_read ON assessment.attempts FOR SELECT
  USING (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_insert ON assessment.attempts FOR INSERT
  WITH CHECK (tenant_id = infra.current_tenant_id());
CREATE POLICY tenant_update ON assessment.attempts FOR UPDATE
  USING (tenant_id = infra.current_tenant_id())
  WITH CHECK (tenant_id = infra.current_tenant_id());
-- Không tạo policy DELETE; role ứng dụng không có quyền DELETE.

-- Nội dung lai (H: platform + gán + riêng tenant): tenant + gán, không có deleted_at
CREATE POLICY content_read ON content.question_versions FOR SELECT
  USING (
    tenant_id = infra.current_tenant_id()
    OR (tenant_id IS NULL AND EXISTS (
          SELECT 1 FROM infra.tenant_resource_assignments a
          WHERE a.resource_type = 'question' AND a.resource_id = question_versions.question_id
            AND a.tenant_id = infra.current_tenant_id() AND a.deleted_at IS NULL))
  );

-- Lớp phòng thủ thứ hai cho bản ghi đã xóa: view _live cho BI/replica (ADR-0089 §1)
CREATE VIEW assessment.attempts_live WITH (security_invoker = true) AS
  SELECT * FROM assessment.attempts WHERE deleted_at IS NULL;
GRANT SELECT ON assessment.attempts_live TO learning_readonly;     -- learning_readonly không có quyền trên bảng gốc

-- Đường đăng nhập (ADR-0089 §3): đọc membership trước khi biết tenant; role ứng dụng không có BYPASSRLS
CREATE ROLE identity_membership_reader NOLOGIN BYPASSRLS;   -- chỉ làm owner của hàm dưới, không đăng nhập được
GRANT USAGE ON SCHEMA identity, org TO identity_membership_reader;
GRANT SELECT ON identity.tenant_memberships_v TO identity_membership_reader;  -- và SELECT trên các bảng nguồn của view
CREATE FUNCTION identity.memberships_for_user(p_user_id uuid)
RETURNS SETOF identity.tenant_memberships_v         -- chỉ các cột membership của view, không bảng gốc
LANGUAGE sql STABLE SECURITY DEFINER SET search_path = identity, pg_temp AS $
  SELECT m.*
  FROM identity.tenant_memberships_v m
  WHERE m.user_id = p_user_id
$;                                                  -- chỉ trả membership của đúng user đó
ALTER FUNCTION identity.memberships_for_user(uuid) OWNER TO identity_membership_reader;  -- bảng nguồn FORCE RLS nên owner cần BYPASSRLS
REVOKE ALL ON FUNCTION identity.memberships_for_user(uuid) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION identity.memberships_for_user(uuid) TO identity_app;  -- gọi trong tenancy.Bypass(ctx, "auth_login")

REVOKE DELETE, TRUNCATE ON ALL TABLES IN SCHEMA assessment FROM learning_app, learning_platform;  -- ví dụ trong eduworld_learning; mỗi DB áp cho <module>_app, <module>_platform
```

- **Vì sao RLS không lọc `deleted_at`** (ADR-0089 §1): với PostgreSQL, `UPDATE` có `WHERE` hoặc `RETURNING` buộc dòng mới cũng phải thỏa policy `SELECT`. Nếu policy `SELECT` có `deleted_at IS NULL`, lệnh soft-delete (`SET deleted_at = now()`) bị từ chối ("new row violates row-level security policy"). Vì vậy RLS chỉ lọc tenant; `deleted_at IS NULL` do ent interceptor thêm, SQL tường minh ở read side tự thêm điều kiện và có test (R27), và view `<schema>.<table>_live` là lớp thứ hai cho `<module>_readonly`/BI.
- Unit of work (ent) chạy `SET LOCAL app.tenant_id = '<uuid>'` ở đầu mỗi transaction ([BACKEND_DESIGN §5.1](BACKEND_DESIGN.md#51-interceptor-global-filter--soft-delete)); ent interceptor thêm `tenant_id`/`deleted_at` vào query (lớp 1), RLS lọc tenant là lớp 2. Không set → `infra.current_tenant_id()` trả null → không thấy dòng nào (fail-closed).
- Bảng `infra.outbox_events`, `infra.processed_events`, `infra.idempotency_records`, `infra.dead_letters` không bật RLS (§3.9, ADR-0089 §4); bảng G/GA không bật RLS.
- Dòng `tenant_id IS NULL` của bảng T\*/TA\*/H (vd. đăng nhập thất bại trước khi biết tenant, role platform, nội dung platform) chỉ được ghi bằng `<module>_platform` dưới `PlatformScope`; policy cho `<module>_app` giữ điều kiện tenant thuần nên `<module>_app` không ghi được dòng NULL.
- Hàm `SECURITY DEFINER` chỉ dùng khi ADR cho phép (hiện có `identity.memberships_for_user`); cột trả về cố định, có `SET search_path`, `REVOKE ... FROM PUBLIC`.
- Phiên System Admin liên tenant dùng role `<module>_platform` (BYPASSRLS); mọi query của phiên này có `tenant_id = ANY(@tenant_ids)` tường minh từ dropdown và ghi audit `cross_tenant.accessed` (ADR-0027 §6).
- Bảng liên tenant (`competition.matches`, `match_results` trong `eduworld_competition`) có `tenant_ids uuid[]`; truy cập qua join `match_participants` (có RLS theo `tenant_id`) trong use case.
- Test bắt buộc: mỗi bảng T có test "tenant A không đọc/ghi được dòng tenant B", "soft-delete (`UPDATE ... SET deleted_at`) chạy thành công dưới RLS" và "dòng đã soft-delete không đọc được qua interceptor và view `_live`" (Master Prompt §22 Multi-tenancy, ADR-0028, ADR-0089).
- Chi tiết mô hình: [MULTITENANCY](../06-architecture/MULTITENANCY.md).

## 5. Partitioning & retention

| Bảng | Partition | Retention mặc định | Nhãn |
|---|---|---|---|
| `learning.learning_events` | tháng theo `occurred_at` | Suốt thời gian học + 1 năm; sau đó ẩn danh hóa/tổng hợp | [Confirmed] |
| `assessment.attempt_answers` | tháng | như learning events | [Confirmed] |
| `chat.messages` | tháng | 180 ngày hiển thị; sau đó ẩn danh hóa nội dung, partition giữ lại (ADR-0070) | [Confirmed] |
| `audit.audit_logs` | tháng | 2 năm | [Confirmed] |
| `identity.login_events` | tháng | `login_events_retention` (90 ngày) rồi ẩn danh hóa | [Confirmed — Technical Architect 2026-11-16] |
| `game.game_sessions` | không partition (bảng nhỏ, chỉ phiên) | 1 năm, sau đó ẩn danh hóa `device_id`/`device_model` | [Confirmed] |
| `game.login_events` | tháng | 1 năm; sau đó ẩn danh hóa (`ip_hash`, `device_model` = NULL) — báo cáo hoạt động dùng `users.last_login_at` | [Confirmed] |
| `chat.gameplay_messages` | ngày theo `created_on` (ngày tạo; drop partition khi mọi channel trong đó đã purge) | `gameplay_chat_retention_days` sau khi channel đóng (mặc định 7, tối thiểu 3) → **purge vật lý** (ADR-0033) | [Confirmed] |
| `chat.ephemeral_messages` | ngày | 24 h → **purge vật lý** (ADR-0033) | [Confirmed] |
| `sis.staging_records` | tháng | `sis_staging_retention` (30 ngày) rồi ẩn danh hóa payload | [Confirmed — Technical Architect 2026-11-16] |
| `sis.sync_record_errors` | tháng | 90 ngày | [Confirmed] |
| `learning.mastery_history`, `competition.leaderboard_snapshots` | tháng | 3 năm | [Confirmed] |
| `analytics.events_raw` (game) | ngày theo `occurred_on` | 90 ngày → ẩn danh hóa `player_key`/`session_key`, partition đủ điều kiện gỡ theo quy trình vận hành (ADR-0028, ADR-0058) | [Confirmed] |
| `analytics.player_days` (game) | tháng | 400 ngày → ẩn danh hóa như trên | [Confirmed] |
| `analytics.cohort_retention`, `funnel_daily`, `content_stats_weekly`, `perf_daily` (game) | tháng/năm | 2 năm (không chứa định danh) | [Confirmed] |
| Rollup dashboard `*_daily` (learning `rollup_class_daily`, `rollup_school_daily`, `rollup_tenant_daily`; game `analytics.rollup_daily`) | tháng theo `bucket_date` | 2 năm | [Confirmed] |
| Rollup `*_weekly` / `*_monthly` | năm / không partition | 3 năm / 5 năm | [Confirmed] |

**Rollup dashboard (ADR-0062).** Cấu trúc cột chung (khóa chiều tường minh, `bucket_date`, `metric_version`, cột đo cộng dồn được, `numerator`/`denominator`, `hist`, `is_final`, `computed_at`), cách upsert gia tăng, job chốt ngày, backfill và sizing: [DASHBOARD_DATA](DASHBOARD_DATA.md). Index: PK B-tree `(tenant_id, <scope_id>, bucket_date, metric_version)` + BRIN `(bucket_date)` trên bảng ngày; partition tạo trước bằng `pg_partman` [Confirmed]. Bảng rollup là dữ liệu dẫn xuất, dựng lại được, nên không có `deleted_at`; gỡ partition hết hạn theo quy trình vận hành như mục trên. Dashboard đọc trên **read replica** của `eduworld_learning` và `eduworld_game` bằng role `<module>_dashboard`.

- Partition tạo trước 3 tháng bằng job worker (hoặc `pg_partman` [Confirmed]); có partition `DEFAULT` để không mất dữ liệu, cảnh báo nếu có dòng rơi vào DEFAULT. Partition ngày của bốn bảng `infra` do job `infra-partition-maintenance` tạo trước tối thiểu 7 ngày (§3.9).
- Hết hạn lưu trữ (ADR-0028): job ứng dụng chỉ **ẩn danh hóa** (ghi đè cột PII/nội dung, giữ dòng); partition **được giữ lại**: Release 1.0 không detach/drop partition dữ liệu nghiệp vụ và không xóa vật lý (ADR-0070; ngoại lệ: chat gameplay theo ADR-0033 và bốn bảng hạ tầng `infra.outbox_events`, `infra.processed_events`, `infra.idempotency_records`, `infra.dead_letters` theo ADR-0089 §5, §3.9). Dung lượng được theo dõi theo [RISKS](../12-project/RISKS.md) RISK-46. Role ứng dụng không có quyền DDL/DELETE.
- Yêu cầu xóa theo luật (privacy request): ẩn danh hóa theo `student_id` trên mọi schema qua quy trình [DATA_PRIVACY](../10-security/DATA_PRIVACY.md); Release 1.0 không xóa vật lý (ADR-0070).

## 6. Index strategy

1. Mọi FK có index (trừ bảng rất nhỏ).
2. Query hot path có index composite bắt đầu bằng `tenant_id`, đúng thứ tự filter → sort: `(tenant_id, student_id, started_at DESC)`, `(channel_id, seq DESC)`, `(tenant_id, status)`.
3. Partial index cho bản ghi chưa xóa và trạng thái hoạt động (`WHERE deleted_at IS NULL`, `WHERE status='published' AND deleted_at IS NULL`, `WHERE left_at IS NULL`, `WHERE published_at IS NULL`).
4. GIN cho mảng/JSONB chỉ khi có query thật (`objective_ids`).
5. Full-text CMS: cột `search_tsv tsvector` sinh từ stem (dùng `unaccent`) + GIN.
6. Mỗi migration thêm index trên bảng lớn dùng `CREATE INDEX CONCURRENTLY` (migration riêng, không trong transaction).
7. `pg_stat_statements` bật ở mọi môi trường; review top query mỗi sprint.

## 7. Migration policy

| Quy tắc | Chi tiết |
|---|---|
| Công cụ | **Atlas** versioned migrations (thư mục `src/backend/modules/<module>/migrations/`, `atlas.sum`, `atlas migrate lint` trong CI — ADR-0039); ent sinh phần bảng/cột/index (`atlas migrate diff`); phần ent không biểu diễn được viết tay trong cùng thư mục (§7.1) |
| Chạy | Image `migrate` chạy job `migrate --module=<module>` (Atlas apply) cho từng database trước rollout `core-api`/`worker`, bằng role `<module>_migrator` ([DEPLOYMENT §12](../11-operations/DEPLOYMENT.md#12-migration-trong-deploy-adr-0039)); binary kiểm tra revision tối thiểu của từng module trong `/readyz`. Không auto-migrate khi khởi động |
| Expand / contract | Thay đổi breaking chia ≥ 2 release: (1) expand — thêm cột/bảng mới, ghi kép; (2) backfill bằng job; (3) contract — bỏ đọc cột cũ; (4) xóa cột cũ ở release sau khi xác nhận |
| Không mất dữ liệu | Migration không chứa `DELETE`/`TRUNCATE` dữ liệu nghiệp vụ (ADR-0028). Cấm `DROP TABLE`/`DROP COLUMN` trên dữ liệu học tập, kết quả, audit trong migration thường; cần ADR hoặc phê duyệt Data Owner + backup xác nhận [Confirmed — Master Prompt §21.19] |
| Down migration | Viết cho thay đổi schema thuần; với thay đổi dữ liệu ghi rõ "irreversible" và kế hoạch phục hồi từ backup |
| Lock an toàn | `SET lock_timeout = '5s'`; thêm cột `NOT NULL` có default dùng cách không rewrite; FK mới thêm `NOT VALID` rồi `VALIDATE CONSTRAINT` |
| Review | Migration chạm bảng > 1M dòng phải có ước lượng thời gian + kế hoạch rollback trong MR |
| CI | Chạy migration của từng module trên database trống + snapshot staging ẩn danh (nightly); kiểm tra `varchar(n)` (không có cột `text`), REVOKE DELETE, RLS bật trên bảng T/TA/T\*/H, policy RLS không chứa `deleted_at`, mỗi schema ent có đúng một mixin (ADR-0089) |

### 7.1. Migration viết tay (ADR-0089 §6)

- Phần ent không biểu diễn được viết tay trong cùng thư mục migration do Atlas quản lý và được `atlas migrate lint` kiểm tra, review trong MR như migration sinh tự động: RLS policy, trigger, `GRANT`/`REVOKE`, view `<table>_live`, hàm `SECURITY DEFINER`, DDL partition (`PARTITION BY`, tạo/`ATTACH`/`DETACH` partition — đặt trong thư mục con `partitions/`).
- Partition của ngày/tháng tiếp theo do job `infra-partition-maintenance` tạo trước tối thiểu 7 ngày; thiếu partition là cảnh báo P2.

## 8. Sizing ước tính [Confirmed — OQ-03]

Giả định năm 1: 50.000 học sinh, 60% hoạt động hằng tuần, mỗi học sinh hoạt động trung bình 40 câu trả lời/ngày học, 200 ngày học/năm.

| Bảng | Dòng/năm | Kích thước ước tính (dữ liệu + index) |
|---|---|---|
| `attempt_answers` | 50k × 0.6 × 40 × 200 ≈ 240M | ~120 GB |
| `learning_events` | ~3× answers ≈ 700M | ~280 GB |
| `chat.messages` | 30k HS × 20 tin/ngày × 200 ≈ 120M (giữ 180 ngày ≈ 60M) | ~25 GB |
| `xp_ledger` | ~50M | ~8 GB |
| `audit_logs` | ~20M | ~10 GB |
| Còn lại | | < 20 GB |

Phân bố theo database: `eduworld_learning` chiếm phần lớn (attempt_answers, learning_events ≈ 400 GB), `eduworld_chat` ≈ 30 GB, `eduworld_game` ≈ 15 GB (xp_ledger, login_events), `eduworld_identity` ≈ 15 GB (audit), `eduworld_competition` < 10 GB. Kết luận sơ bộ: mọi môi trường đặt 5 database trên một cụm PostgreSQL (ADR-0074 mục 4; pilot primary + 1 replica, sizing ở [DEPLOYMENT §4](../11-operations/DEPLOYMENT.md#4-topology)); chỉ chuyển `eduworld_learning` sang cụm riêng khi có số đo vượt ~60% tài nguyên (ADR-0074 mục 7, không đổi mã) [Confirmed]; partition + retention giữ bảng nóng nhỏ. Cần benchmark thật trước khi chốt (Master Prompt §22 Performance).

## 9. Backup & recovery (tham chiếu)

- Backup và PITR cho cả cụm (pgBackRest), khôi phục được **từng database**; khi khôi phục một database, các module khác giữ nguyên và projection `ref` được đồng bộ lại từ event/rebuild (§2.17).
- PITR: base backup hằng ngày + WAL archiving liên tục (pgBackRest hoặc WAL-G) vào object storage mã hóa, đặt tại Việt Nam [Confirmed]; mục tiêu RPO ≤ 5 phút, RTO ≤ 4 giờ (NFR-AVAIL-002).
- Diễn tập khôi phục hằng tháng trên staging.
- Chi tiết quy trình, lịch giữ backup và script: [BACKUP_RECOVERY](../11-operations/BACKUP_RECOVERY.md).
