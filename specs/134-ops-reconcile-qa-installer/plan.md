# Implementation Plan: F134 — Công cụ vận hành bổ sung: lệnh `reconcile-<module>`, loại tenant QA khỏi analytics, trang tải bộ cài cho trường

**Branch**: `134-ops-reconcile-qa-installer` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

(1) **Đối soát**: package `src/backend/pkg/reconcile` (so khớp, phân loại, báo cáo) + `src/backend/pkg/messaging` công bố registry consumer → nguồn/event type; binary `src/backend/cmd/worker` thêm lệnh con `reconcile --module=<module>` (job Swarm một lần `reconcile-<module>`, ADR-0074); mỗi module có DB hiện thực hai operation facade chỉ đọc `listOutboxEventIds`, `listProcessedEventIds` (spec internal → `src/backend/modules/<module>/api`) qua `src/backend/pkg/outbox` store (không qua ent nghiệp vụ). (2) **Tenant QA**: cột `platform.tenants.is_qa` → `tenant.updated` → `ref.tenants.is_qa` ở game/learning; analytics consumer/telemetry bỏ sự kiện tenant QA; query catalog Game Analytics và tổng hợp học tập platform anti-join tenant QA; portal workspace D có công tắc. (3) **Bộ cài**: CI `build-kit.sh` phát chỉ mục `school-kit/<channel>.json` có chữ ký; module game `listSchoolClientInstallers` tải + kiểm chữ ký + cache, lọc theo khối của trường; portal workspace C trang "Tải bộ cài".

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 / TypeScript (React, Vite) / Bash (CI) |
| Primary Dependencies | `src/backend/pkg/outbox`, `src/backend/pkg/messaging`, `src/backend/pkg/modcall`, `src/backend/pkg/entx` (`UnitOfWork`), `src/backend/pkg/authz`, oapi-codegen, ent + Atlas; Ant Design Pro, TanStack Query (`apiClient`, `withTenantScope` của F050/F051); `openssl` (ký chỉ mục) |
| Storage | PostgreSQL mọi database module (`infra.*` chỉ đọc); `eduworld_identity` (`platform.tenants.is_qa`), `eduworld_game`/`eduworld_learning` (`ref.tenants.is_qa`); Valkey `ew:game:school_kit_index:<channel>` (cache 5 phút) |
| Testing | go test + testcontainers (PG, NATS); test khôi phục DB về T (TC-SVC-014); Vitest + Playwright cho portal |
| Target Platform | Linux container (job Swarm một lần dùng image `worker`), trình duyệt |
| Project Type | platform kit + backend module + web app + CI |
| Performance Goals | SC-001 (≤ 10 phút cho 30 ngày một module); `listProcessedEventIds` p95 ≤ 500 ms/trang 5000 |
| Constraints | AGENTS R1, R3, R7, R13, R14, R20, R28, R29, R31, R32; ADR-0037, ADR-0074, ADR-0042, ADR-0044, ADR-0058, ADR-0062 |
| Scale/Scope | 5 module có DB; `outbox_retention`; mọi tenant |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | 2 operation facade × 5 module, `listSchoolClientInstallers`, 2 JSON Schema, payload `tenant.updated` ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Đối soát chỉ qua facade trong tiến trình (`src/backend/pkg/modcall`); operation đối soát là query handler chỉ đọc; chỉ import `src/backend/modules/<khác>/api` theo `x-allowed-callers` (`src/backend/pkg/reconcile` dùng chung) |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng `infra.*` là hạ tầng không thuộc tenant (đọc như outbox relay, chỉ trả ID, không payload); `listSchoolClientInstallers` kiểm scope trường; dashboard platform vẫn qua `CrossTenantScope` hiện có; không ghi/xóa |
| IV Server-authoritative | ✅ | Lọc QA, lọc khối, kiểm chữ ký ở server |
| V Xác thực, phân quyền | ✅ | Internal: `x-allowed-callers`; `platform.tenant.manage` cho cờ QA; `school.installer.read` [Confirmed] |
| VI An toàn trẻ em | ✅ | Báo cáo/endpoint không PII; tenant QA không chứa học sinh thật |
| VII Một nguồn sự thật, cấu hình | ✅ | `outbox_retention` theo tên; `reconcile_settle_minutes` [Confirmed] đăng ký CONFIG_DEFAULTS; URL CDN/khóa công khai qua env |
| VIII UI tách logic, token | ✅ | Trang bộ cài/công tắc QA chỉ gọi API, theme `design/admin-tokens/` |
| IX Test có bằng chứng | ✅ | Mỗi F134-AC có test viết trước; TC-SVC-014 chạy trong CI nightly |

## Project Structure

### Documentation (feature này)

```text
specs/134-ops-reconcile-qa-installer/
├── spec.md
├── plan.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
src/backend/pkg/reconcile/{window.go,classify.go,report.go,runner.go}       # thuần + port PeerClient, OutboxReader, ProcessedReader
src/backend/pkg/messaging/registry.go                                      # Consumer{Name, SourceService, EventTypes}
src/backend/pkg/outbox/reader.go                                           # ListByWindow (chỉ đọc infra.*)
src/backend/cmd/worker/reconcile.go                                        # `worker reconcile --module=<module> --restore-point --from --to --settle --out`
src/backend/modules/<module>/                                              # module ∈ identity, learning, game, competition, chat
├── api/                                                       # facade sinh từ spec internal (listOutboxEventIds, listProcessedEventIds)
├── internal/application/query/ops/{list_outbox_event_ids.go,list_processed_event_ids.go}
├── internal/infrastructure/modules/reconcile_peers.go          # adapter facade 4 module còn lại (src/backend/pkg/modcall)
└── internal/presentation/facade/ops_reconcile.go              # hiện thực interface facade
scripts/ops/reconcile.sh                                       # `docker service create --mode replicated-job --restart-condition none … ghcr.io/<org>/eduworld/worker:<tag> reconcile --module=<module>`
src/backend/modules/identity/ent/schema/tenant.go                 # is_qa + use case UpdateTenant (F010)
src/backend/modules/game/
├── ent/schema/ref/tenant.go                                   # is_qa
├── internal/application/command/analytics/{ingest_domain_event.go,ingest_telemetry.go}   # bỏ tenant QA
├── internal/application/query/analytics/query_catalog.go      # anti-join ref.tenants.is_qa
├── internal/application/query/installer/list_school_installers.go
├── internal/application/port/school_kit_index.go
├── internal/infrastructure/release/{school_kit_index_http.go,signature.go}
└── internal/presentation/http/public/client_installers.go
src/backend/modules/learning/{ent/schema/ref/tenant.go,internal/application/query/analytics/platform_summary.go}
deploy/installer/school-kit/{build-kit.sh,publish-index.sh}
src/cms/frontend/apps/portal/src/workspaces/school-lms/installers/{InstallerListPage.tsx,DeviceConfigGuide.tsx,useSchoolInstallers.ts}
src/cms/frontend/apps/portal/src/workspaces/platform-admin/tenants/TenantForm.tsx  # công tắc is_qa
src/cms/frontend/packages/ops-ui/src/tenant/TenantFilter.tsx                 # nhãn QA
```

**Structure Decision**: Logic đối soát chung ở `src/backend/pkg/reconcile` để 5 module không lặp code và không import `internal/` của nhau (R13); lệnh con nằm ở composition root `src/backend/cmd/worker`, mỗi module chỉ thêm query + hiện thực facade. Trang bộ cài là module mới của workspace C theo sitemap [ADMIN_PORTAL §10](../../docs/04-admin-portal/ADMIN_PORTAL.md); công tắc QA nằm trong form tenant hiện có của F057.

## Phase 0: Research

Không có. Cơ chế đối soát theo [BACKUP_RECOVERY §3.5](../../docs/11-operations/BACKUP_RECOVERY.md); lọc cửa sổ theo khoảng UUIDv7 dùng PK `(consumer, event_id)` sẵn có ([DATABASE_DESIGN §3.9](../../docs/07-backend/DATABASE_DESIGN.md)); ký chỉ mục dùng cùng cơ chế RSA-SHA256 của F133.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `src/backend/pkg/reconcile.Classify(event, peerState, restorePoint, settle, now)`; game `installer.FilterByGradeBands`, `GradeLevel→GradeBand` (GLOSSARY); identity `Tenant.SetQA()` (cấm `common`) | Hàm thuần, unit test bảng |
| application | Query `ListOutboxEventIds`, `ListProcessedEventIds` (mọi module) · Command `UpdateTenant` (+`is_qa`, F010) · Command `IngestDomainEvent`/`IngestTelemetry` (bỏ QA) · Query `ListSchoolInstallers`, query catalog analytics, `PlatformLearningSummary` (anti-join QA) | authz `authz.Require` tường minh đầu handler; tracing `src/backend/pkg/observability` |
| infrastructure | `src/backend/pkg/outbox.Reader`; adapter facade `<peer>` qua `src/backend/pkg/modcall` (span, deadline, không retry); `SchoolKitIndexHttp` (`EW_CLIENT_RELEASES_BASE_URL`, `EW_CLIENT_RELEASE_PUBKEY_FILE`) + cache Valkey; projection consumer `tenant.updated` | |
| presentation | Hiện thực facade `listOutboxEventIds`, `listProcessedEventIds`; public `listSchoolClientInstallers`; `worker reconcile`; portal `InstallerListPage`, `TenantForm` | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F003 (`src/backend/pkg/outbox`, `src/backend/pkg/modcall`, `src/backend/pkg/messaging`), F010 (`updateTenant`), F080 (analytics consumer, query catalog), F043/F044 (tổng hợp học tập platform), F057 (form tenant, `TenantFilter`), F131 (tenant QA, probe), F133 (khóa ký, bộ cài trường), F130 (Swarm, runbook RB-01).
- Facade: module được đối soát → 4 module còn lại (`listOutboxEventIds`, `listProcessedEventIds`), trong tiến trình `worker`. Event: `tenant.updated` (identity → game, learning).

### Bảo mật, tenant, idempotency, audit

- Facade: `x-allowed-callers` gồm 5 module (cưỡng chế bằng `make arch-check`); không có route HTTP nên không có đường cho token người dùng (TC-SVC-005); ngữ cảnh `SystemPlatformScope("reconcile")` + `tenancy.Bypass(ctx, "platform_job")` có audit (ADR-0089); trả ID/metadata, không `envelope`.
- `listSchoolClientInstallers`: `TenantScope` + scope trường của UserRole; trường ngoài scope `404`.
- `updateTenant`: `If-Match` + `Idempotency-Key`; audit `tenant.qa_flag_changed`.
- `worker reconcile`: audit `ops.reconcile.completed` qua outbox của module được đối soát (actor `system:reconcile`, người chạy ghi trong `reason`); không ghi bảng nghiệp vụ (DB user chỉ cần SELECT trên `infra.*` + INSERT outbox).

### Quan sát

- Lệnh đối soát là job một lần: không xuất metric Prometheus; kết quả ở báo cáo JSON + log JSON (`run_id`, `trace_id`, số lệch theo loại) + audit.
- Metric: `eduworld_analytics_events_dropped_total{reason="qa_tenant"}`, `eduworld_school_kit_index_fetch_total{result}`, `eduworld_modcall_requests_total{caller,callee,operation,outcome}` cho hai operation đối soát.
- Alert: `SchoolKitIndexInvalid` (fetch lỗi chữ ký > 0 trong 15 phút). Runbook RB-01 bước 7 trỏ lệnh `scripts/ops/reconcile.sh`.


### Danh mục menu/action portal (ADR-0090)

Menu và nút của feature lấy từ [`specs/portal/menu-catalog.yaml`](../portal/menu-catalog.yaml) qua registry sinh (`make portal-catalog`, F051) — **không** đăng ký menu tay trong TSX. Nút thao tác bọc `<ActionGate actionKey=…>`; menu/action trong `disabled_menus`/`disabled_actions` của `getMyPermissions` bị ẩn; `403 FEATURE_DISABLED` → thông báo "Tính năng đang tạm tắt" và tải lại quyền. Operation gọi tới đều có `x-portal-action` khớp các khóa dưới (kiểm `node scripts/specs/check-portal-catalog.mjs`).

| Loại | Khóa |
|---|---|
| Menu feature hiện thực | `menu.school_lms.installers` |
| Action dùng | `action.platform_admin.tenants.view`, `action.live_ops.game_analytics.view`, `action.live_ops.game_analytics.export`, `action.school_lms.reports.view`, `action.platform_admin.reports.view`, `action.platform_admin.reports.export`, `action.school_lms.installers.view`, `action.platform_admin.tenants.edit` |

Khoảng trống catalog/contract (không sửa trong feature này; báo owner `specs/portal/menu-catalog.yaml` + OpenAPI):

- ~~Catalog chưa có menu "Tải bộ cài"~~ — đã đóng: `menu.school_lms.installers`, action `action.school_lms.installers.view` (UI-only; gắn `x-portal-action` cho `listSchoolClientInstallers` khi thêm vào OpenAPI)
- `updateTenant` (cờ QA, Cần thêm trong OpenAPI) → gắn `x-portal-action: action.platform_admin.tenants.edit` khi thêm (action đã có trong catalog)

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
