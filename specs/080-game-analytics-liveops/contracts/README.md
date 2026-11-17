# Contracts: F080 — Game Analytics và Live Ops dashboard (workspace E)

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/eduworld-api.v1.yaml` (`x-service: game`). `/admin/*` chỉ gateway nội bộ + MFA.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `postTelemetry` | `POST /api/v1/game/telemetry` | eduworld-api | học sinh (`game_player`) | Có sẵn (Cần sửa schema đóng) |
| `getClientConfig` | `GET /api/v1/game/client-config` | eduworld-api | game client | Có sẵn (Cần sửa thêm cờ telemetry) |
| `getTenantGameSummary` | `GET /api/v1/tenant/analytics/game/summary` | eduworld-api | `analytics.tenant.read` / `analytics.school.read` | Có sẵn (Cần sửa `TenantGameSummary`, task T055) |
| `getGameMetrics` | `GET /api/v1/admin/analytics/game/metrics` | eduworld-api | `game.analytics.read` | Cần thêm |
| `getGameRetention` | `GET /api/v1/admin/analytics/game/retention` | eduworld-api | `game.analytics.read` | Cần thêm |
| `getGameFunnel` | `GET /api/v1/admin/analytics/game/funnels/{funnel_code}` | eduworld-api | `game.analytics.read` | Cần thêm |
| `getGameContentStats` | `GET /api/v1/admin/analytics/game/content-stats` | eduworld-api | `game.analytics.read` | Cần thêm |
| `getGamePerformance` | `GET /api/v1/admin/analytics/game/performance` | eduworld-api | `game.analytics.read` | Cần thêm |
| `createGameAnalyticsExport` · `getGameAnalyticsExport` | `POST /api/v1/admin/analytics/game/exports` · `GET …/exports/{export_id}` | eduworld-api | `game.analytics.read` | Cần thêm |
| `getLiveOpsOverview` | `GET /api/v1/admin/liveops/overview` | eduworld-api | `game.liveops.read` | Cần thêm |
| `getEventRunStats` · `suspendEventRun` · `resumeEventRun` · `cancelEventRun` | `/api/v1/game-admin/events/{event_id}/runs/{run_id}[/stats|:suspend|:resume|:cancel]` | eduworld-api | `game.liveops.read` / `game.event.operate` | Có sẵn |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không (nguồn dữ liệu qua event; Prometheus là hạ tầng, không phải service) | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng (portal poll 15–30 s) | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| Danh mục [GAME_ANALYTICS §3.1](../../../docs/11-operations/GAME_ANALYTICS.md) (`game_session.*`, `world.*`, `quest.*`, `challenge.*`, `boss.*`, `xp.awarded`, `level.reached`, `currency.*`, `shop.item_purchased`, `achievement.unlocked`, `collection.completed`, `party.*`, `match.completed`, `duel.*`, `event.*`, `activity.submitted`, `chat.daily_stats`, `avatar.created`, `story.chapter_completed`) | Tiêu thụ | `specs/events/payloads/<module>/*.v1.schema.json` | Cần thêm (owner feature phát: F014, F070–F077, F100, F105, F117, F031) |
| `tenant.updated` (`product_telemetry_enabled`) | Tiêu thụ | `specs/events/payloads/identity/tenant.updated.v1.schema.json` | Cần thêm (owner F010; F015 phát khi đổi cài đặt) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không | — |

## 6. Delta cần thực hiện

Task Phase 2: T004–T008.

- **Cần thêm** 7 operation analytics (`x-service: game`, `x-internal: true`, tag `admin`): tham số chung `from`, `to` ($ref `DashboardFrom/To`), `grain` ($ref `DashboardGrain`), `tenant_ids` ($ref `TenantIds`), `school_ids` (maxItems 100), `grade_bands` (maxItems 4), `platforms` (enum, maxItems 4), `locales` (maxItems 2), `device_tiers` (maxItems 3), `client_versions` (items maxLength 32, maxItems 20); `metric_codes` (items maxLength 64, maxItems 10). Response `GameMetricSeries` {`metric_code`, `metric_version`, `grain`, `data_as_of`, `is_partial`, `available_from`, `series[]` {`dims`, `points[]` {`t`, `value`, `numerator`, `denominator`} maxItems 120}}; `GameRetentionMatrix`, `GameFunnel` {`steps[]`}, `GameContentStatsPage` (cursor, ≤ `dashboard_page_size_max`), `GamePerformanceReport`; `GameAnalyticsExportCreate` {`query_id` maxLength 64, `params`} → `202` `GameAnalyticsExport` {`export_id`, `status`, `download_url` format uri maxLength 2048, `expires_at`}. Lỗi: `422 DASHBOARD_RANGE_TOO_LARGE`, `429 DASHBOARD_BUSY`, `429 EXPORT_LIMIT_REACHED`, `504 DASHBOARD_QUERY_TIMEOUT`.
- **Cần thêm** `getLiveOpsOverview` → `LiveOpsOverview` {`as_of`, `kpis[]` {`code` maxLength 64, `value` number nullable, `unit` maxLength 16, `status` enum `normal|warning|incident`, `breakdown[]` {`label` maxLength 64, `value`} maxItems 50} maxItems 30}; `503` khi nguồn metric lỗi.
- **Cần sửa** `TelemetryBatch` và `TelemetryEvent` thêm `additionalProperties: false`; mô tả rate limit giữ "30 req/phút".
- **Cần sửa** `ClientConfig` thêm `product_telemetry_enabled` boolean (owner F014); `TenantSettings` thêm `product_telemetry_enabled` (owner F015).
- **Tài liệu**: GAME_ANALYTICS §6 (cột chiều tường minh) lệch DATABASE_DESIGN §2.8b (`dims_hash`) — thống nhất; thêm `getGameAnalyticsExport`, `getLiveOpsOverview` vào API_DESIGN §9.
- **Cần sửa** `TenantGameSummary` (chuyển từ F081, task T055): thêm (mọi số `type: [number|integer, 'null']`) `active_students_series[]` maxItems 120 {`t` date, `value`}, `active_days_3plus_rate`, `session_duration_p50_seconds`, `chapter0_completion_rate`, `social_play_rate`, `sessions_over_second_reminder_rate`, `late_night_play_rate`, `platform_distribution[]` maxItems 6 {`key` enum `windows|android|ios|web|macos|linux`, `share`}, `device_tier_distribution[]` maxItems 3 {`key` string maxLength 8, `share`}, `by_grade_band[]` maxItems 4 {`grade_band` $ref `GradeBand`, `active_students`, `quest_completion_rate`}, `available_from` date (owner F080).
- **Cần sửa** `getTenantGameSummary`: khai báo `404` (trường ngoài tenant).
- **Tài liệu**: GAME_ANALYTICS §8 (Academic Manager không có quyền) mâu thuẫn ADMIN_PORTAL §2.2/SECURITY_ARCHITECTURE (`academic_manager` có `analytics.school.read`) — chuyển từ F081, task T061.
