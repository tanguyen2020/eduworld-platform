# Data Model: F080 — Game Analytics và Live Ops dashboard (workspace E)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

DB `eduworld_game`, schema `analytics` (T: `tenant_id` + RLS). Chiều chuẩn: `tenant_id`, `school_id`, `grade_level smallint`, `platform varchar(16)`, `locale varchar(8)`, `device_tier varchar(8)`, `client_version varchar(32)`. Không có `student_id`/tên/nickname/nội dung chat. Bảng append/aggregate không soft-delete (ẩn danh hóa thay xóa, ADR-0070).

## 1. Entity

### events_raw — `analytics.events_raw` (Có sẵn §2.8b)

| Trường | Kiểu (ent / SQL) | Ràng buộc | Ghi chú |
|---|---|---|---|
| `occurred_on` | date | PK (1/2), partition ngày | |
| `event_id` | uuid | PK (2/2) | `event_id` domain hoặc `client_event_id` |
| `occurred_at`, `ingested_at` | timestamptz | | |
| chiều chuẩn | như trên | | |
| `player_key`, `session_key` | `varchar(64)` (`MaxLen(64)`) | nullable sau ẩn danh | HMAC-SHA256 hex |
| `event_name` | `varchar(64)` | | danh mục GAME_ANALYTICS §3 |
| `source` | `varchar(8)` | CHECK `server/client` | |
| `props` | jsonb | CHECK `octet_length(props::text) <= 2048` | chỉ khóa danh mục |
| `key_version` | smallint | **Thêm cột** | phiên bản khóa HMAC (xoay khóa) |

**Index**: `(occurred_on, event_name)`, `(occurred_on, tenant_id)`.

### player_days — `analytics.player_days` (Có sẵn)

`activity_date`, `player_key`, chiều chuẩn, `sessions int`, `active_seconds int`, `first_active_date date`, `capped_xp bool`, `capped_currency bool`, `reminders_shown smallint` (**Thêm cột**), `late_night bool` (**Thêm cột**); PK `(activity_date, player_key)`, partition tháng.

### rollup_daily / rollup_weekly — (Có sẵn) · rollup_monthly — `analytics.rollup_monthly` (Mới)

`metric_date` (`iso_week varchar(8)` tuần · `month date` tháng), `metric_code varchar(64)`, `metric_version smallint`, `dims_hash varchar(64)`, `dims jsonb`, `value numeric`, `numerator bigint`, `denominator bigint`, `hist jsonb` (**Thêm cột**, percentile), `is_final bool` (**Thêm cột**), `computed_at`; UNIQUE `(metric_date, metric_code, metric_version, tenant_id, dims_hash)`. `rollup_monthly` không partition (GAME_ANALYTICS §6).

### cohort_retention, funnel_daily, content_stats_weekly, perf_daily — (Có sẵn §2.8b)

Cột và unique giữ nguyên DATABASE_DESIGN; `content_stats_weekly.content_kind varchar(16)`, `perf_daily.preset varchar(8)`, `renderer varchar(16)`.

### export_jobs — `analytics.export_jobs` (Mới)

| Trường | Kiểu | Ràng buộc | Ghi chú |
|---|---|---|---|
| `export_id` | uuid | PK | |
| `tenant_id` | uuid | NULL = liên tenant platform | |
| `requested_by` | uuid | NOT NULL | |
| `query_id` | `varchar(64)` | | id trong query catalog |
| `params` | jsonb | CHECK ≤ 8 KB | bộ lọc đã validate |
| `status` | `varchar(16)` | CHECK `queued/running/succeeded/failed/expired` | |
| `row_count` | int | ≤ `dashboard_export_max_rows` | |
| `object_key` | `varchar(512)` | | `ew-exports/…` |
| `error_code` | `varchar(64)` | | |
| `created_at`, `finished_at`, `expires_at` | timestamptz | | |

### Projection — `ref.tenants` (Có sẵn §2.17 · Thêm cột)

`product_telemetry_enabled bool NOT NULL DEFAULT true` từ event `tenant.updated` (delta F015).

Cập nhật DATABASE_DESIGN §2.8b (`rollup_monthly`, `export_jobs`, cột mới) — task T052.

## 2. Vòng đời / trạng thái

| Entity | Chuyển trạng thái |
|---|---|
| Rollup ngày | `is_final = false` (hằng giờ) → `true` (chốt ngày) → tính lại khi event trễ ≤ 48 h |
| Raw / player_days | bình thường → ẩn danh (`player_key = NULL`) khi hết hạn hoặc có yêu cầu |
| Export job | `queued → running → succeeded/failed` → `expired` sau `dashboard_export_file_retention_days` |

## 3. Quy tắc kiểm tra dữ liệu

- `props` chỉ chứa khóa của `event_name` trong danh mục; khóa lạ bị bỏ khi ingest.
- Ô báo cáo tenant < `analytics_min_cell_size` → null (suppression ở query, không ở bảng).
- DAU/WAU/MAU, retention, percentile tính từ `player_days` khi chốt ngày, không cộng từ bảng ngày (DASHBOARD_DATA §3.2).

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| — | Module analytics chỉ tiêu thụ; export hoàn tất thông báo qua notification portal (không event domain mới) | — |

## 5. Read model `TenantGameSummary` mở rộng (query `GetTenantSummary`, chuyển từ F081)

| Field mới | `metric_code` (GAME_ANALYTICS §4) | Bảng |
|---|---|---|
| `active_students_series[]` {`t`, `value`} | `dau` (grain day) / `wau` (week) / `mau` (month) | `analytics.rollup_daily/weekly/monthly` |
| `active_days_3plus_rate` | `active_days_per_week` (KPI ≥ 3 ngày) | `rollup_weekly` |
| `session_duration_p50_seconds` | `session_duration_p50` | `rollup_daily` |
| `chapter0_completion_rate` | `chapter0_completion` | `funnel_daily`/`rollup_daily` |
| `social_play_rate` | `social_play_rate` | `rollup_weekly` |
| `sessions_over_second_reminder_rate`, `late_night_play_rate` | `sessions_over_second_reminder`, `late_night_play_rate` | `rollup_daily` |
| `platform_distribution[]`, `device_tier_distribution[]` {`key`, `share`} | `preset_distribution`/chiều `platform`, `device_tier` | `rollup_daily` (dims), `perf_daily` |
| `by_grade_band[]` {`grade_band`, `active_students`, `quest_completion_rate`} | như trên theo chiều khối | `rollup_*` |

Mọi giá trị số có thể `null` khi ô < `analytics_min_cell_size` (tăng `suppressed_cells`).
