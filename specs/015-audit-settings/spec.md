# Feature Specification: F015 — Audit log tập trung và cài đặt platform/tenant/trường

**Feature Branch**: `015-audit-settings` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `identity` · **Mốc**: M-C1 · **Phụ thuộc**: F012
**Input**: Mọi module ghi sự kiện audit qua outbox; module identity gom vào một nhật ký append-only phân vùng theo tháng, cho nhà trường và System Admin truy vấn trong phạm vi; quản lý cài đặt platform (dọn học sinh, thời hạn chat gameplay), cài đặt tenant, cài đặt trường (khung giờ truy cập, chính sách trường), feature flag và bật/tắt menu/action của Admin Portal theo danh mục `specs/portal/menu-catalog.yaml` (ADR-0090). FR nguồn: FR-AUD-001, FR-AUD-002, FR-ORG-007, FR-CMS-018, NFR-SCAL-004.

## Clarifications

### Session 2026-11-16

- Q: Audit ghi trực tiếp vào DB identity từ module khác? → A: Không; mỗi module phát `audit.recorded` qua outbox, worker identity tiêu thụ `ew.evt.*.audit.recorded` và ghi `audit.audit_logs` ([DATABASE_DESIGN §1.3](../../docs/07-backend/DATABASE_DESIGN.md)).
- Q: Permission truy vấn audit platform là `platform.audit.read` hay `platform.audit.read_all`? → A: `platform.audit.read` (API_DESIGN §9.16, ADMIN_PORTAL §2.2 dòng `audit.*`); `platform.audit.read_all` trong dòng `platform.*` coi là trùng, đề xuất gỡ.
- Q: Chính sách chat cấp trường lưu ở đâu? → A: Ngoài phạm vi feature này (module chat, F122); F015 chỉ lưu khung giờ truy cập và chính sách trường không thuộc chat. JIT đã bỏ (ADR-0063).
- Q: Retention audit? → A: Theo DATA_PRIVACY §2 (Audit log) — hết hạn ẩn danh hóa, giữ partition (ADR-0070); chưa có tên tham số trong CONFIG_DEFAULTS (task đăng ký `audit_log_retention`).

### Session 2026-11-17

- Q: Bật/tắt menu và action của Admin Portal lưu ở đâu (ADR-0090)? → A: Dùng chung `platform.feature_flags` (không bảng riêng — R33) với cột mới `flag_kind` (`feature`/`menu`/`action`) và `parent_key` (action → menu); khóa menu/action phải có trong bảng đọc-chỉ `platform.portal_menu_catalog` (đồng bộ từ `specs/portal/menu-catalog.yaml` bởi `src/backend/cmd/migrate`, `identity_app` chỉ đọc). Không có dòng = bật; action hiệu lực = flag action AND flag menu cha; flag = ghi đè tenant nếu có, ngược lại `default_enabled` (US5).
- Q: API System Admin? → A: Mở rộng operation đã dự kiến của F015, không tạo bộ thứ hai: `listFeatureFlags` (`GET /admin/feature-flags?flag_kind=&tenant_id=`), `putFeatureFlag` (`PUT /admin/feature-flags/{flag_key}`, body `{scope: platform|tenant, state: enabled|disabled|inherit, reason}`, `If-Match`, `X-Tenant-Id` = tenant đích khi `scope = tenant`) và thêm `listPortalMenuCatalog` (`GET /admin/portal-menu-catalog?tenant_id=`); chỉ `platform.feature_flag.write`, gateway nội bộ + MFA. Mục `can_disable = false` → `422 FEATURE_NOT_DISABLEABLE`.
- Q: Middleware `featuregate` thuộc F015? → A: Không — F012 sở hữu middleware và port `featuregate.Source`; F015 hiện thực `Source` trong module identity (đọc DB/Valkey của chính module) và cache `ew:fg:{tenant_id|platform}` (TTL `feature_toggle_cache_ttl`, làm mới khi `platform_settings.changed` scope `feature_flag`). Màn "Bật/tắt menu & action" → F059.

- Q: Mã lỗi khi vượt phạm vi trường/môn của UserRole? → A: `403 PERMISSION_DENIED`; `FORBIDDEN_SCOPE` không phát và sẽ bị gỡ khỏi `ProblemCode` ([API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md), RB-113).
- Q: Cài đặt giờ giới nghiêm `platform_curfew` và khung giới nghiêm của trường (ADR-0078) chưa có tiêu chí — owner nào? → A: Bổ sung theo [CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý) (lát cắt M-P1); tham số theo tên CONFIG_DEFAULTS, mã lỗi theo [API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md).

### Session 2026-11-19 (triển khai)

- Q: Cột `settings_version` cho `platform.tenants`, `org.schools`? → A: [Proposed] Dùng cột `version` (optimistic concurrency) sẵn có của dòng tenant/trường làm ETag cài đặt (thay đổi tên/trạng thái cũng tăng version) — không đổi bảng ent của F010 — OQ F015-1. Khóa `platform_settings` chưa cấu hình có ETag `W/"0"`.
- Q: Role `identity_retention`? → A: [Proposed] Thay bằng hàm `SECURITY DEFINER` `audit.anonymize_expired` (và `audit.ensure_monthly_partitions`, `audit.monthly_partitions_ahead`) chạy bằng `identity_partition_admin`: `identity_app`/`identity_platform` không có quyền UPDATE nào trên nhật ký — OQ F015-2.
- Q: F015-AC-05 so `tenant_id` của envelope với payload — payload `audit.recorded` v1 không có `tenant_id`? → A: Sink kiểm `school_id` của payload thuộc đúng tenant của envelope; khác → lỗi vĩnh viễn (DLQ) + cảnh báo bảo mật trong log + metric `eduworld_audit_records_total{result="tenant_mismatch"}` — OQ F015-3.
- Q: Quyền đọc cài đặt platform/feature flag? → A: Catalog chỉ có `platform.settings.write`/`platform.feature_flag.write`; đọc dùng chính quyền ghi [Proposed] — OQ F015-4.
- Q: `X-Tenant-Id` (tùy chọn) ở `putStudentCleanupSettings`/`putChatRetentionSettings`? → A: [Proposed] Bỏ qua; ghi đè theo tenant đi qua `updateTenantSettings` — OQ F015-5.
- Q: `getTenant` (facade internal, F015-AC-14)? → A: Chưa hiện thực (facade identity mới có nhóm Principal); module khác nhận thay đổi qua `tenant.updated`/`platform_settings.changed` — OQ F015-6.
- Q: Job `partition-maintainer` cho `sis.staging_records`, `sis.sync_record_errors`? → A: F015 tạo partition tháng cho `audit.audit_logs` (không DEFAULT) và `identity.login_events` (tháng đã có dòng trong DEFAULT của F013 bị bỏ qua có cảnh báo); bảng `sis.*` giữ partition DEFAULT của F011 — OQ F015-7.
- Q: Nối binary? → A: Consumer `identity.audit_sink` (mỗi stream `EVT_*`), `identity.feature_toggle_cache`, job `partition-maintainer`/`audit-retention`, `RegisterSettings` và `featuretoggle.Source` cho `featuregate` chưa nối vào `core-api`/`worker` (cùng OQ F010-2).
- Q: Ghi đè flag theo tenant của System Admin? → A: `X-Tenant-Id` là tenant đích (R20); Principal của System Admin mang tenant đó vẫn được ghi.

### Session 2026-11-20 (follow-up F059, Workspace D)

- Q: Nhật ký phân quyền cần xem chung `role.*` và `user_role.*` (OQ F059-3, T007 của F059)? → A: Thêm tham số `action_prefix` (mảng, maxItems 10, mỗi giá trị 2–64 ký tự, pattern `^[a-z][a-z0-9_]*(.[a-z][a-z0-9_]*)*$`) cho cả `listPlatformAuditLogs` và `listTenantAuditLogs`: giá trị khớp action bằng chính nó hoặc bắt đầu bằng `<giá trị>.` (không khớp chuỗi con — `role` không khớp `role_x.*`; SQL dùng `starts_with`, không `LIKE`); nhiều giá trị là OR, dùng cùng `action` là AND; giá trị sai → `422 VALIDATION_FAILED` (`action_prefix`). Phạm vi tenant/platform/CrossTenantScope không đổi. `listPlatformAuditLogs` thêm `security: keycloakAdminBearer`.
- Q: `platform.audit.read` hay `platform.audit.read_all` (OQ F059-11)? → A: Giữ quyết định Session 2026-11-16: `platform.audit.read` (catalog permission trong code, ADMIN_PORTAL §2.2 đã gỡ `read_all`).

### Session 2026-11-20 (giới nghiêm platform ở module game, OQ F014-2)

- Q: Module game đọc `platform_curfew` từ `EW_PLATFORM_CURFEW` lúc khởi động nên lệch với module identity sau `putPlatformCurfewSettings` (smoke seed dev, [quickstart F010 §6.1](../010-tenant-org-structure/quickstart.md)) — module game lấy giá trị hiệu lực thế nào? → A: [Proposed] Qua event, không gọi đồng bộ (R13, R29): consumer `game.platform_settings` của module game chiếu `platform_settings.changed` (`key = platform_curfew`, `scope = platform`) vào `ref.platform_settings` của `eduworld_game`, dùng cho `createGameSession` và job `curfew-session-closer` ngay khi nhận (không restart). **Không đổi payload schema** (owner F015): `curfew_window` (F015 T032) đã mang giá trị sau thay đổi; `after` cùng giá trị là dự phòng. Thứ tự theo `occurred_at` của envelope (event cũ đến muộn không ghi đè). Chi tiết và giới hạn (stream giữ 7 ngày; env chỉ là mặc định trước event đầu tiên): [F014 Clarifications OQ F014-2](../014-game-session-thin/spec.md).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Audit log tập trung, append-only (Priority: P1)

Mọi thao tác nhạy cảm (đăng nhập quản trị, đổi quyền, truy cập dữ liệu học sinh ngoài phạm vi thường, publish nội dung, kết quả thi/trận, kiểm duyệt, SIS sync) được ghi lại một chỗ, không sửa/xóa được.

**Why this priority**: Trách nhiệm giải trình là yêu cầu P1; nhiều feature M-C1 (F010–F014) phát audit.

**Independent Test**: Phát `audit.recorded` từ identity và module game → đúng một dòng mỗi event trong partition tháng tương ứng; UPDATE/DELETE bằng role ứng dụng bị từ chối.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F015-AC-01 | KHI nhận `audit.recorded` từ bất kỳ stream `EVT_*`, module identity PHẢI ghi đúng một dòng audit (idempotent theo `event_id`) với `actor_id`, `actor_type`, `tenant_id`, `school_id`, `action`, `target_type`, `target_id`, `result`, `reason`, `metadata`, `trace_id`, `ip_hash`, `occurred_at` | FR-AUD-001 · [DATABASE_DESIGN §3.11](../../docs/07-backend/DATABASE_DESIGN.md) | TC-SVC-003 |
| F015-AC-02 | HỆ THỐNG PHẢI giữ nhật ký audit append-only: role ứng dụng và role platform không có quyền UPDATE/DELETE/TRUNCATE | FR-AUD-002 · ADR-0028 | TC-TEN-016 |
| F015-AC-03 | HỆ THỐNG PHẢI phân vùng nhật ký audit theo tháng và job PHẢI tạo trước partition cho các tháng sắp tới; không có ghi nào thất bại vì thiếu partition | FR-AUD-002 · NFR-SCAL-004 | — |
| F015-AC-04 | NẾU `metadata` chứa khóa bị cấm (mật khẩu, token, nội dung chat, họ tên, ngày sinh) hoặc vượt giới hạn kích thước, THÌ sink PHẢI lọc bỏ trường đó, ghi phần còn lại và tăng metric vi phạm | FR-AUD-001 · AGENTS R14 | — |
| F015-AC-05 | NẾU `tenant_id` của envelope khác `tenant_id` trong payload, THÌ sink PHẢI chuyển event vào `DLQ_IDENTITY` và cảnh báo bảo mật | FR-AUD-001 · ADR-0042 | — |
| F015-AC-06 | KHI bản ghi audit quá thời hạn lưu giữ theo DATA_PRIVACY §2, job retention PHẢI ẩn danh hóa (`actor_id`, `ip_hash`, PII trong `metadata`) và giữ partition, không xóa vật lý | FR-AUD-002 · NFR-SCAL-004 · ADR-0070 | — |

---

### User Story 2 — Truy vấn audit trong phạm vi (Priority: P1)

Tenant Admin xem audit của tenant, School Admin chỉ trường mình; System Admin xem liên tenant bằng bộ lọc tenant và mỗi lần xem được ghi lại.

**Why this priority**: Nhà trường và đội vận hành cần tra cứu thay đổi quyền, đồng bộ SIS.

**Independent Test**: `ad.truong.a` lọc `action = user_role.granted` → chỉ trường TH Demo A; `sa.admin` lọc hai tenant → có cột tenant và audit `cross_tenant.accessed`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F015-AC-07 | KHI người có `tenant.audit.read` gọi `listTenantAuditLogs`, module identity PHẢI trả audit của tenant phiên (School Admin chỉ trường được gán), lọc theo `school_id`, `actor_id`, `action`, `from`, `to`, phân trang cursor, không gồm nhật ký module kiểm duyệt | FR-AUD-001 | TC-TEN-003 |
| F015-AC-08 | KHI System Admin gọi `listPlatformAuditLogs` với `tenant_ids`, module identity PHẢI trả dữ liệu có `tenant_id` và ghi audit `cross_tenant.accessed`; role khác PHẢI nhận `403 PERMISSION_DENIED` | FR-AUD-001 · ADR-0027 | TC-TEN-004 |
| F015-AC-09 | NẾU người dùng tenant gửi `tenant_ids` hoặc lọc tenant khác, THÌ HỆ THỐNG PHẢI bỏ qua tham số và chỉ trả tenant phiên | FR-AUD-001 · NFR-SEC-006 | TC-CMS-TEN-01 |
| F015-AC-10 | NẾU khoảng `from`–`to` vượt giới hạn truy vấn hoặc thiếu, THÌ module identity PHẢI trả `422 VALIDATION_FAILED`; response PHẢI không chứa PII/nội dung chat | FR-AUD-001 · AGENTS R14 | — |

---

### User Story 3 — Cài đặt platform và cài đặt tenant (Priority: P2)

System Admin đặt số ngày dọn học sinh, số ngày báo trước, ngưỡng cần duyệt, thời hạn chat gameplay; Tenant Admin đặt cài đặt tenant trong giới hạn platform.

**Why this priority**: Cần trước F016 (dọn học sinh) và chat gameplay; không chặn đăng nhập.

**Independent Test**: `sa.admin` đổi `allow_tenant_override = false` → `ad.donvi` đổi `inactive_student_cleanup_days` nhận `403`; đặt thời hạn chat 2 ngày → `422 RETENTION_BELOW_MINIMUM`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F015-AC-11 | KHI System Admin (`platform.settings.write`) cập nhật cài đặt dọn học sinh hoặc thời hạn chat gameplay với `If-Match` và `reason`, module identity PHẢI lưu version mới, ghi audit trước/sau, phát `platform_settings.changed` | FR-ORG-007 · ADR-0032 · ADR-0033 | — |
| F015-AC-12 | NẾU thời hạn chat gameplay nhỏ hơn mức tối thiểu của `gameplay_chat_retention_days`, THÌ HỆ THỐNG PHẢI trả `422 RETENTION_BELOW_MINIMUM` | FR-ORG-007 · ADR-0033 | — |
| F015-AC-13 | NẾU Tenant Admin đổi `inactive_student_cleanup_days` khi platform không cho ghi đè hoặc thiếu `tenant.student_cleanup.configure`, THÌ HỆ THỐNG PHẢI trả `403 PERMISSION_DENIED` | FR-ORG-007 · ADR-0032 | — |
| F015-AC-14 | KHI đọc cài đặt tenant hoặc `getTenant` (internal), module identity PHẢI trả giá trị hiệu lực (tenant nếu được ghi đè, ngược lại platform) và mọi module PHẢI nhận thay đổi qua `tenant.updated`/`platform_settings.changed` | FR-ORG-007 | — |
| F015-AC-15 | NẾU `If-Match` không khớp version hiện tại, THÌ HỆ THỐNG PHẢI trả `412 VERSION_MISMATCH`; người dùng tenant khác PHẢI không đọc/ghi được cài đặt tenant này (`404`) | FR-ORG-007 · NFR-SEC-006 | TC-TEN-001 |
| F015-AC-21 | KHI System Admin (`platform.settings.write`) cập nhật `platform_curfew` với `reason` và `If-Match`, module identity PHẢI lưu khung giờ (ICT, bắt đầu/kết thúc, cho phép qua nửa đêm), ghi audit before/after, phát `platform_settings.changed`; role tenant PHẢI không ghi được (`403 PERMISSION_DENIED`); giá trị mặc định khi chưa cấu hình PHẢI lấy từ `platform_curfew` của CONFIG_DEFAULTS | NFR-SAFE-005 · FR-ORG-007 · [CHILD_SAFETY §4.1](../../docs/10-security/CHILD_SAFETY.md#41-giờ-giới-nghiêm-cấp-platform-adr-0078) · ADR-0078 | TC-AUTH-092 |

---

### User Story 4 — Cài đặt trường (Priority: P2)

School Admin đặt khung giờ truy cập game và chính sách trường (tạo club của học sinh, tắt bảng xếp hạng lớp/trường) cho trường mình; học sinh đăng nhập ngoài khung giờ bị từ chối.

**Why this priority**: FR-ORG-007 P1–P5; khung giờ là cài đặt an toàn của trường.

**Independent Test**: `ad.truong.a` đặt khung giờ 07:00–17:00; `hs.an.3a` đăng nhập 20:00 (đồng hồ giả) → `403 ACCESS_OUTSIDE_ALLOWED_HOURS`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F015-AC-16 | KHI School Admin (`school.settings.write`, scope trường) cập nhật cài đặt trường với `If-Match` và `reason`, module identity PHẢI kiểm schema (khung giờ hợp lệ theo múi giờ tenant, giá trị chính sách trong danh sách cho phép), lưu, ghi audit và phát `school.updated` | FR-ORG-007 | — |
| F015-AC-17 | NẾU School Admin cập nhật cài đặt trường khác (cùng hoặc khác tenant), THÌ HỆ THỐNG PHẢI trả `403 PERMISSION_DENIED` hoặc `404` | FR-ORG-007 · NFR-SEC-006 | TC-TEN-003 |
| F015-AC-18 | TRONG KHI thời điểm hiện tại nằm ngoài khung giờ truy cập của trường học sinh, module identity PHẢI từ chối `gameLogin` với `403 ACCESS_OUTSIDE_ALLOWED_HOURS` (thứ tự kiểm tra IDENTITY_MAPPING §4.1) | FR-ORG-007 · [IDENTITY_MAPPING §4.1](../../docs/09-identity/IDENTITY_MAPPING.md) | — |
| F015-AC-19 | HỆ THỐNG PHẢI không nhận cài đặt trường làm nới lỏng giới hạn platform (cấp dưới chỉ siết chặt) | FR-ORG-007 · ADR-0020 | — |
| F015-AC-22 | KHI School Admin cập nhật khung giới nghiêm của trường, module identity PHẢI chỉ nhận khung bao trùm `platform_curfew` (siết thêm); NẾU khung trường không chứa trọn `platform_curfew`, THÌ PHẢI trả `422 VALIDATION_FAILED` chỉ rõ trường; giới nghiêm hiệu lực của học sinh PHẢI là hợp của `platform_curfew` và khung trường (tenant `common` luôn áp `platform_curfew`, không có khung trường), cung cấp qua `PlayerBootstrap.policies.play_time` và event `school.updated`/`platform_settings.changed` cho module game, chat | FR-ORG-007 · NFR-SAFE-005 · AC-AUTH-032 · [CHILD_SAFETY §4.1](../../docs/10-security/CHILD_SAFETY.md#41-giờ-giới-nghiêm-cấp-platform-adr-0078) | TC-AUTH-092 |

---

### User Story 5 — Feature flag platform và bật/tắt menu/action của Admin Portal (Priority: P2)

System Admin bật/tắt feature flag toàn platform hoặc theo tenant; bootstrap game và portal trả flag hiệu lực. System Admin bật/tắt từng menu và action của Admin Portal (mặc định platform hoặc ghi đè cho một tenant), trừ các mục được bảo vệ; trạng thái lưu trong DB, có audit, không cần deploy (ADR-0090).

**Why this priority**: Cho phép ẩn màn chưa sẵn sàng (CODE_PLAN §1.2) và ngừng dùng menu/action mà không deploy; `featuregate` (F012 US9) và menu sinh từ catalog (F051) cần dữ liệu này trước các workspace F052+.

**Independent Test**: Bật flag `preview_game_frame` cho `th-demo-a` → bootstrap `ad.truong.a`/`hs.an.3a` có flag; tenant khác không. `sa.admin` tắt `menu.school_lms.assignments` cho `th-demo-a` (có `reason`, `If-Match`) → `listPortalMenuCatalog?tenant_id=th-demo-a` báo tắt, `th-demo-b` vẫn bật; tắt `menu.platform_admin.feature_toggles` → `422 FEATURE_NOT_DISABLEABLE`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F015-AC-20 | KHI System Admin (`platform.feature_flag.write`) đặt flag loại `feature` (mặc định platform + ghi đè theo tenant) với `reason`, module identity PHẢI lưu, audit, phát `platform_settings.changed` và bootstrap PHẢI phản ánh trong ≤ `principal_cache_ttl` | FR-ORG-007 · FR-AUD-001 | — |

| F015-AC-23 | KHI `src/backend/cmd/migrate` chạy lúc deploy, module identity PHẢI đồng bộ `specs/portal/menu-catalog.yaml` (đã kiểm bằng `portal-menu-catalog.schema.json`) vào bảng đọc-chỉ `platform.portal_menu_catalog` theo `item_key` kèm `catalog_version` (mục bị gỡ khỏi file → soft-delete, idempotent khi chạy lại); role `identity_app` PHẢI chỉ có `SELECT` và mọi lệnh ghi bằng role này PHẢI bị từ chối | FR-CMS-018 · ADR-0090 mục 1 | TC-CMS-MENU-01 |
| F015-AC-24 | `platform.feature_flags` PHẢI có `flag_kind` (`feature`/`menu`/`action`, mặc định `feature`) và `parent_key`; NẾU `flag_kind` là `menu`/`action` THÌ `flag_key` PHẢI tồn tại trong `platform.portal_menu_catalog` (kiểm ở use case và khóa ngoại), và flag `action` PHẢI có `parent_key` = menu cha theo catalog; ghi thẳng flag menu/action với khóa không có trong catalog PHẢI bị lớp DB từ chối | FR-CMS-018 · ADR-0090 mục 2 | TC-CMS-MENU-03 |
| F015-AC-25 | KHI System Admin gọi `putFeatureFlag` với `scope` (`platform`/`tenant`), `state` (`enabled`/`disabled`/`inherit`), `reason`, `If-Match` (và `X-Tenant-Id` = tenant đích khi `scope = tenant`), module identity PHẢI suy `flag_kind`/`parent_key` từ catalog, lưu mặc định platform hoặc ghi đè tenant (`inherit` gỡ ghi đè), tăng `version`, ghi audit `feature_flag.updated` (trước/sau, `reason`, tenant) và phát `platform_settings.changed` scope `feature_flag` trong cùng transaction | FR-CMS-018 · FR-AUD-001 · ADR-0090 mục 2 | TC-CMS-MENU-02, TC-CMS-MENU-03 |
| F015-AC-26 | NẾU `putFeatureFlag` tắt mục `can_disable = false` (menu được bảo vệ hoặc action của menu đó), THÌ HỆ THỐNG PHẢI trả `422 FEATURE_NOT_DISABLEABLE`; NẾU thiếu `reason` hoặc `inherit` với `scope = platform` THÌ `422 VALIDATION_FAILED`; thiếu `If-Match` → `428 PRECONDITION_REQUIRED`; lệch version → `412 VERSION_MISMATCH`; khóa menu/action không có trong catalog → `404 NOT_FOUND`; người không có `platform.feature_flag.write` → `403 PERMISSION_DENIED`, token thiếu MFA → `403 MFA_REQUIRED` | FR-CMS-018 · ADR-0090 mục 2 · AGENTS R20 | TC-CMS-MENU-03 |
| F015-AC-27 | Module identity PHẢI hiện thực port `featuregate.Source` (F012) theo quy tắc: không có dòng = bật; giá trị flag = ghi đè của tenant nếu có, ngược lại `default_enabled`; action hiệu lực = flag action AND flag menu cha; phạm vi platform (System Admin không `X-Tenant-Id`) dùng mặc định platform; cache Valkey `ew:fg:<tenant_id>` (phạm vi platform: `ew:fg:platform`) với TTL `feature_toggle_cache_ttl` và xóa khi nhận `platform_settings.changed` scope `feature_flag`, để thay đổi có hiệu lực ở `featuregate` và `getMyPermissions` trong ≤ `feature_toggle_cache_ttl` và không ảnh hưởng tenant khác | FR-CMS-018 · ADR-0090 mục 2, 3 | TC-CMS-MENU-02, TC-CMS-MENU-06 |
| F015-AC-28 | KHI System Admin gọi `listPortalMenuCatalog` (tùy chọn `tenant_id`) hoặc `listFeatureFlags` (lọc `flag_kind`, `tenant_id`), module identity PHẢI trả cây workspace → menu → action (hoặc danh sách flag) gồm `can_disable`, `required_permissions`, mặc định platform, ghi đè tenant, giá trị hiệu lực, `version` (ETag) và `catalog_version`; đọc theo tenant PHẢI ghi audit `cross_tenant.accessed` | FR-CMS-018 · ADR-0090 · ADR-0027 §6 | TC-CMS-MENU-02 |
| F015-AC-29 | Flag loại `menu`/`action` PHẢI chỉ ảnh hưởng Admin Portal (`featuregate` với `azp = eduworld-portal`, `getMyPermissions`) và PHẢI KHÔNG xuất hiện trong `PlayerBootstrap.feature_flags`; flag loại `feature` vẫn trả qua bootstrap như F015-AC-20 | FR-CMS-018 · ADR-0090 mục 4 | TC-CMS-MENU-04 |

### Edge Cases

- NẾU deploy mới gỡ một menu/action khỏi catalog trong khi còn flag trỏ tới, THÌ `src/backend/cmd/migrate` soft-delete mục catalog và soft-delete flag tương ứng (audit `feature_flag.retired`), không xóa vật lý; FK chỉ tham chiếu dòng còn hiệu lực theo kiểm tra use case.
- NẾU System Admin tắt menu cha rồi bật riêng một action con, THÌ action vẫn tắt (AND với menu cha); màn F059 hiển thị "tắt theo menu cha".
- NẾU hai System Admin cùng sửa một flag, THÌ người sau nhận `412 VERSION_MISMATCH`.
- NẾU NATS tạm dừng, THÌ audit tích lũy trong outbox của module nguồn và được ghi khi relay chạy lại (không mất bản ghi).
- NẾU cùng `event_id` giao hai lần, THÌ chỉ một dòng audit (`processed_events`).
- NẾU partition tháng kế tiếp chưa tạo vì job lỗi, THÌ cảnh báo trước ≥ 7 ngày; ghi vào partition `default` không được phép.
- NẾU `inactive_student_cleanup_days` = `0`/`null` ở tenant, THÌ dọn học sinh tắt cho tenant đó (F016).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-AUD-001 | Audit sink tập trung, truy vấn tenant/platform, lọc PII | [ADMIN_PORTAL §7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-AUD-002 | Append-only, partition tháng, retention theo data inventory (ẩn danh hóa) | [DATA_PRIVACY §2](../../docs/10-security/DATA_PRIVACY.md) |
| FR-ORG-007 | Cài đặt platform/tenant/trường (khung giờ, chính sách trường), feature flag; không JIT | [API_DESIGN §9.12, §9.16](../../docs/07-backend/API_DESIGN.md) |
| FR-CMS-018 | Dữ liệu và API bật/tắt menu/action (catalog đọc-chỉ, `flag_kind`/`parent_key`, `putFeatureFlag`, `listPortalMenuCatalog`, hiện thực `featuregate.Source`); middleware ở F012, UI ở F051/F059 | [ADMIN_PORTAL](../../docs/04-admin-portal/ADMIN_PORTAL.md) · [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md) · [API_DESIGN](../../docs/07-backend/API_DESIGN.md) |
| NFR-SCAL-004 | Partition tháng cho audit logs (và bảng log append-only của identity) | [DATABASE_DESIGN §5](../../docs/07-backend/DATABASE_DESIGN.md) |

### Key Entities

- **AuditLog**: bản ghi bất biến: ai, làm gì, trên đối tượng nào, tenant/trường, kết quả, lý do, metadata đã lọc.
- **PlatformSetting**: cấu hình platform theo khóa (`student_cleanup`, `chat_retention`) có version.
- **TenantSettings / SchoolSettings**: cài đặt theo tenant/trường trong giới hạn platform.
- **FeatureFlag**: cờ bật/tắt mặc định platform, ghi đè theo tenant; loại `feature` (tính năng, bootstrap), `menu`, `action` (Admin Portal, ADR-0090).
- **PortalMenuCatalogItem**: bản sao đọc-chỉ của một workspace/menu/action trong `specs/portal/menu-catalog.yaml` (`can_disable`, `required_permissions`, `operation_ids`, `catalog_version`).

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `inactive_student_cleanup_days` | Giá trị mặc định/ghi đè dọn học sinh |
| `gameplay_chat_retention_days`, `ephemeral_chat_retention` | Thời hạn chat gameplay và tối thiểu |
| `principal_cache_ttl` | Độ trễ áp flag/cài đặt vào bootstrap |
| `feature_toggle_cache_ttl` | Cache trạng thái bật/tắt menu/action (≤ `principal_cache_ttl`) |
| `event_stream_retention`, `outbox_retention` | Đệm audit khi sink chậm |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không PII/nội dung chat/token trong audit | AGENTS R14 · NFR-PRIV-002 |
| Append-only, không xóa vật lý | AGENTS R21 · ADR-0028 · ADR-0070 |
| Cài đặt đọc từ cấu hình hiệu lực, không hardcode | AGENTS R33 |
| Mọi chuỗi có giới hạn | AGENTS R28 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% event `audit.recorded` hợp lệ được ghi đúng một lần (kiểm bằng đối soát số event phát/ghi trong test tải).
- **SC-002**: 0 lệnh UPDATE/DELETE thành công trên nhật ký audit bằng role ứng dụng.
- **SC-003**: Truy vấn audit 30 ngày của một tenant trả trang đầu trong ≤ 2 giây ở dữ liệu thử 12 tháng.
- **SC-004**: Thay đổi cài đặt/flag có hiệu lực ở mọi module trong ≤ 60 giây.
- **SC-005**: 100% mục `can_disable = false` của catalog bị từ chối khi tắt (test sinh từ catalog); 0 lệnh ghi thành công vào `platform.portal_menu_catalog` bằng `identity_app`.

## Ngoài phạm vi

- Phát `audit.recorded` của từng nghiệp vụ: feature sở hữu nghiệp vụ (F010–F014, F016, learning, chat…).
- Chính sách chat cấp tenant/trường → F120/F122; công tắc kiểm duyệt → F116/F122.
- Worker dọn học sinh dùng cài đặt → F016; màn cài đặt/audit trên portal → F052, F057; màn "Bật/tắt menu & action" và feature flag → F059.
- Middleware `featuregate`, `disabled_menus`/`disabled_actions` trong `getMyPermissions` → F012 (US9); menu portal sinh từ catalog → F051; nội dung `specs/portal/menu-catalog.yaml` và script `check-portal-catalog` → contract chung (ADR-0090).
- Rà soát thời hạn lưu theo luật — deferred (ADR-0082).

## Assumptions

- `audit_log_retention` (2 năm theo DATA_PRIVACY §2) chưa có trong CONFIG_DEFAULTS — task đăng ký.
- Giới hạn khoảng truy vấn audit [Proposed] 93 ngày mỗi lần, đăng ký CONFIG_DEFAULTS cùng task.
- `platform.feature_flags` chưa có cột trong DATABASE_DESIGN — data-model đề xuất "Mới"; cột `flag_kind`, `parent_key`, khóa ngoại tới `platform.portal_menu_catalog` theo ADR-0090 (DATABASE_DESIGN là nguồn sự thật DDL).
- Payload `platform_settings.changed` có `scope = feature_flag` — owner schema event là đợt cập nhật event riêng (không sửa trong feature này).
