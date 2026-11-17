# Implementation Plan: F080 — Game Analytics và Live Ops dashboard (workspace E)

**Branch**: `080-game-analytics-liveops` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Backend (module game, module `analytics`): mở rộng `postTelemetry` (schema đóng, khử trùng, lọc theo cờ tenant); durable consumer `analytics` ánh xạ domain event → `analytics.events_raw` với `player_key` HMAC; job rollup hằng giờ + chốt ngày + backfill trễ ≤ 48 h ghi bảng tổng hợp (DATABASE_DESIGN §2.8b + `rollup_monthly`); query catalog cố định cho API `/admin/analytics/game/*` và `getTenantGameSummary` trên replica (role `game_dashboard`, cache Valkey có phiên bản); export CSV bất đồng bộ; `getLiveOpsOverview` đọc Prometheus. Frontend (workspace `live-ops`): Live dashboard, 11 panel Game Analytics, Giám sát sự kiện. Delta contract ở Phase 2.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 · TypeScript strict (React 18, Vite) |
| Primary Dependencies | chi, ent + Atlas, oapi-codegen, nats.go, go-redis, `pkg/entx` (`UnitOfWork`), `pkg/outbox`, `pkg/tenancy`, Prometheus HTTP API client · Ant Design 5 + Pro (StatisticCard), chart theo DESIGN_SYSTEM §4.4, TanStack Query |
| Storage | PostgreSQL `eduworld_game` schema `analytics` (primary ghi, replica đọc); Valkey cache/HLL (tạm); object storage `ew-exports` |
| Testing | go test + testcontainers (PG, NATS, Valkey); golden dataset; EXPLAIN catalog; Vitest + Playwright; k6 tải dashboard |
| Target Platform | Linux container (`core-api` + `worker` profile `events`/`jobs`, module `game`); trình duyệt desktop/tablet |
| Project Type | backend module + web app |
| Performance Goals | `dashboard_query_p95_target`; job rollup ngày < 2 h (GAME_ANALYTICS §10); telemetry p95 ≤ 100 ms |
| Constraints | AGENTS R8, R14, R19–R21, R23, R26–R28; ADR-0042, ADR-0057, ADR-0058, ADR-0062, ADR-0070 |
| Scale/Scope | 10.000 CCU (TC-ANL-009); raw 90 ngày |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta 8 operationId analytics, `getLiveOpsOverview`, schema đóng telemetry, cờ telemetry tenant: task Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Nguồn dữ liệu chỉ qua event NATS; cờ tenant qua event projection; query handler chỉ đọc replica |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Mọi bảng `tenant_id` + RLS; liên tenant chỉ System Admin/platform qua Bypass có audit; ẩn danh hóa thay xóa |
| IV Server-authoritative | ✅ | Chỉ số từ event server; telemetry client chỉ là mẫu hiệu năng/UI, không ảnh hưởng thưởng |
| V Xác thực, phân quyền | ✅ | `game.analytics.read`, `game.liveops.read`, `game.event.operate`, `analytics.tenant.read`, `analytics.school.read`; `game_player` cho telemetry |
| VI An toàn trẻ em | ✅ | Không PII; ô < 5 ẩn; chỉ số chơi lành mạnh, không mục tiêu tăng thời gian chơi |
| VII Một nguồn sự thật, cấu hình | ✅ | Định nghĩa chỉ số ở GAME_ANALYTICS; tham số theo tên; `MaxLen` |
| VIII UI tách logic, token | ✅ | Portal chỉ hiển thị số server trả; palette chart từ `admin-tokens` |
| IX Test có bằng chứng | ✅ | Mỗi F080-AC có test trước; golden dataset |

## Project Structure

### Documentation (feature này)

```text
specs/080-game-analytics-liveops/
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
modules/game/
├── ent/schema/analytics/{events_raw.go,player_days.go,rollup_daily.go,rollup_weekly.go,rollup_monthly.go,
│                         cohort_retention.go,funnel_daily.go,content_stats_weekly.go,perf_daily.go,export_job.go}
├── internal/domain/analytics/{event_mapping.go,player_key.go,metric_catalog.go,suppression.go,grain.go}
├── internal/application/
│   ├── command/analytics/{ingest_telemetry.go,ingest_domain_event.go,run_hourly_rollup.go,close_day.go,
│   │                      reprocess_late.go,create_export.go,anonymize_player.go}
│   ├── query/analytics/{get_metrics.go,get_retention.go,get_funnel.go,get_content_stats.go,get_performance.go,
│   │                    get_tenant_summary.go,get_export.go,query_catalog.go}
│   ├── query/liveops/get_overview.go
│   └── port/{metrics_source.go,export_storage.go,dashboard_cache.go,hmac_key_provider.go}
├── internal/infrastructure/{persistence/analytics/*,prometheus/metrics_source.go,storage/export_writer.go,cache/dashboard_cache.go}
├── internal/presentation/
│   ├── http/public/{telemetry.go,tenant_analytics.go}
│   ├── http/admin/{analytics.go,liveops.go}
│   ├── consumer/analytics_consumer.go                  # durable `analytics`
│   └── job/{rollup_hourly.go,day_close.go,export_worker.go,analytics_retention.go}
└── tests/{unit,integration,contract,perf}/analytics/
web/apps/portal/src/workspaces/live-ops/
├── dashboard/{LiveDashboardPage.tsx,KpiCards.tsx}
├── analytics/{GameAnalyticsPage.tsx,filters/AnalyticsFilterBar.tsx,panels/*.tsx}   # 11 panel
├── events/{EventMonitorPage.tsx,EventRunActions.tsx}
├── api/{liveops.ts,analytics.ts,eventRuns.ts}
└── __tests__/
web/apps/portal/src/workspaces/school-lms/game-report/   # màn "Hoạt động game" (chuyển từ F081)
├── GameActivityReportPage.tsx         # /school-lms/reports/game-activity
├── widgets/{OverviewWidget,ProgressionWidget,SocialWidget,WellbeingWidget,DeviceWidget}.tsx
└── api/tenantGameSummary.ts
web/packages/ops-ui/src/dashboard/{GrainLabel.tsx,DataAsOf.tsx}   # DashboardWidget, SuppressedCell dùng lại của F056
deploy/observability/prometheus/rules/analytics.yaml
```

**Structure Decision**: Module `analytics` trong module game (ADR-0058); handler admin tách `http/admin` (gateway nội bộ). Thành phần dashboard chung (ADR-0062 §3.14.1) ở `ops-ui` để F056 dùng lại.

## Phase 0: Research

Không có. Chỉ số ở [GAME_ANALYTICS](../../docs/11-operations/GAME_ANALYTICS.md), cơ chế rollup/bảo vệ DB ở [DASHBOARD_DATA](../../docs/07-backend/DASHBOARD_DATA.md), KPI live-ops ở [DESIGN_SYSTEM §4.3](../../docs/05-ux/DESIGN_SYSTEM.md).

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `EventMapping` (bảng domain event → analytics), `PlayerKey(hmacKey, student_id)`, `MetricCatalog` (`metric_code`, `metric_version`), `Suppress(cell, min)`, `GrainFor(from,to)` | Không import hạ tầng |
| application | Command `IngestTelemetry`, `IngestDomainEvent`, `RunHourlyRollup`, `CloseDay`, `ReprocessLate`, `CreateExport`, `AnonymizePlayer` · Query `GetMetrics`, `GetRetention`, `GetFunnel`, `GetContentStats`, `GetPerformance`, `GetTenantSummary`, `GetExport`, `GetLiveOpsOverview` | Query đọc replica, giới hạn đồng thời |
| infrastructure | Repository ent; `MetricsSource` (Prometheus, URL `EW_PROMETHEUS_URL`); `ExportStorage` (`ew-exports`, URL ký); `DashboardCache` (`ew:game:dash:*`); `HmacKeyProvider` (secret) | |
| presentation | Handler `postTelemetry`, `getTenantGameSummary`, `getGameMetrics`, `getGameRetention`, `getGameFunnel`, `getGameContentStats`, `getGamePerformance`, `createGameAnalyticsExport`, `getGameAnalyticsExport`, `getLiveOpsOverview` · consumer `analytics` · job | Portal gọi thêm `getEventRunStats`, `suspendEventRun`, `resumeEventRun`, `cancelEventRun` (F077) |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F014 (telemetry, client-config), F015 (cờ telemetry tenant), F077 (event run API), F078/F079 (link sang workspace A), F071–F075, F105, F117 (nguồn event), F051 (shell), F052 (chunk `school-lms` cho màn "Hoạt động game"), F056 (component dashboard `ops-ui`).
- Event tiêu thụ: theo GAME_ANALYTICS §3.1 (`game_session.*`, `world.*`, `story.chapter_completed`, `avatar.created`, `quest.*`, `challenge.*`, `boss.*`, `xp.awarded`, `level.reached`, `currency.*`, `shop.item_purchased`, `achievement.unlocked`, `collection.completed`, `party.*`, `match.completed`, `duel.*`, `event.*`, `activity.submitted` (owner F030), `chat.daily_stats`; `wellbeing.break_reminder_shown` chưa có producer — EVENT_DESIGN §4.5) + `tenant.updated` (cờ telemetry).

### Bảo mật, tenant, idempotency, audit

- Liên tenant: `tenancy.Bypass(ctx, "platform_analytics")` chỉ trong query admin analytics/liveops, audit `cross_tenant.accessed` + metric; tenant summary dùng `TenantScope`.
- Idempotency-Key: `createGameAnalyticsExport`, thao tác event run; telemetry khử trùng `client_event_id`; consumer `processed_events`.
- Audit: `report.exported`, `event.run_suspended/resumed/cancelled` (F077), `analytics.player_anonymized`.

### Quan sát

- Metric: `eduworld_analytics_raw_rows`, `eduworld_analytics_rollup_duration_seconds`, `eduworld_analytics_query_duration_seconds`, `eduworld_rollup_late_events_total`, `eduworld_telemetry_events_total{type,accepted}`, `eduworld_dashboard_rejected_total{reason}`. Alert `AnalyticsRawRowsHigh`, `RollupLagHigh`, `DayCloseNotDone` (MONITORING). Log không PII.


### Danh mục menu/action portal (ADR-0090)

Menu và nút của feature lấy từ [`specs/portal/menu-catalog.yaml`](../portal/menu-catalog.yaml) qua registry sinh (`make portal-catalog`, F051) — **không** đăng ký menu tay trong TSX. Nút thao tác bọc `<ActionGate actionKey=…>`; menu/action trong `disabled_menus`/`disabled_actions` của `getMyPermissions` bị ẩn; `403 FEATURE_DISABLED` → thông báo "Tính năng đang tạm tắt" và tải lại quyền. Operation gọi tới đều có `x-portal-action` khớp các khóa dưới (kiểm `node scripts/specs/check-portal-catalog.mjs`).

| Loại | Khóa |
|---|---|
| Menu feature hiện thực | `menu.live_ops.game_analytics`, `menu.live_ops.live_dashboard`, `menu.school_lms.reports` |
| Action dùng | `action.live_ops.game_analytics.view`, `action.live_ops.game_analytics.export`, `action.live_ops.live_dashboard.view`, `action.game_ops.events.stats`, `action.game_ops.events.operate`, `action.school_lms.reports.view` |

Khoảng trống catalog/contract (không sửa trong feature này; báo owner `specs/portal/menu-catalog.yaml` + OpenAPI):

- Màn "Hoạt động game" (workspace C) không có menu riêng trong catalog: `getTenantGameSummary` thuộc `action.school_lms.reports.view` — hiển thị trong `menu.school_lms.reports` hoặc bổ sung menu
- `getLiveOpsOverview`, `getGameAnalyticsExport` (Cần thêm) → đề xuất `action.live_ops.live_dashboard.view`, `action.live_ops.game_analytics.export`

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
