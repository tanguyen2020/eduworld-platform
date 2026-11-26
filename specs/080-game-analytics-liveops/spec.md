# Feature Specification: F080 — Game Analytics và Live Ops dashboard (workspace E)

**Feature Branch**: `080-game-analytics-liveops` · **Created**: 2026-11-16 · **Status**: Done · **Module**: module `game` + `src/cms/frontend/apps/portal` (workspace E `live-ops`; màn "Hoạt động game" ở workspace C `school-lms`) · **Mốc**: M4b · **Phụ thuộc**: F078, F079, F052, F056 (và F014 telemetry endpoint, F077 event run)
**Input**: Thu telemetry client không PII; analytics consumer ghi `analytics.events_raw` từ domain event NATS với định danh giả `player_key`; rollup ngày/tuần/tháng, cohort, funnel, hiệu năng thiết bị; API Game Analytics cho platform và báo cáo tổng hợp tenant (ẩn ô < 5) kèm màn "Hoạt động game" cho nhà trường ở workspace C (chuyển từ F081); Live dashboard thời gian thực từ Prometheus; giám sát/dừng khẩn cấp sự kiện; UI workspace E. FR nguồn: FR-ANL-006…013, FR-CMS-016, FR-GAME-011, FR-OPS-003, NFR-OBS-003.

## Clarifications

### Session 2026-11-16

- Q: Định nghĩa chỉ số ở đâu? → A: Chỉ ở [GAME_ANALYTICS §4](../../docs/11-operations/GAME_ANALYTICS.md) (`metric_code`, `metric_version`); spec này không định nghĩa lại.
- Q: Bảng rollup theo DATABASE_DESIGN (`dims_hash` + `dims` jsonb) hay GAME_ANALYTICS §6 (cột chiều tường minh + `grouping_set`)? → A: Theo DATABASE_DESIGN §2.8b (nguồn sự thật bảng); thêm `rollup_monthly` (Mới). Mâu thuẫn báo cáo.
- Q: Live dashboard lấy số liệu từ đâu? → A: Prometheus (DASHBOARD_DATA §2: không qua database). Portal không gọi Prometheus trực tiếp; module game cung cấp `getLiveOpsOverview` (delta) đọc Prometheus HTTP API qua adapter, URL từ cấu hình.
- Q: Tenant tắt telemetry sản phẩm lưu ở đâu? → A: Cài đặt tenant (identity, F015) — delta `TenantSettings.product_telemetry_enabled`; module game giữ projection từ event tenant và trả cờ cho client qua `getClientConfig` (F014).
- Q: Chỉ số chơi lành mạnh lấy lời nhắc nghỉ từ đâu (server không biết lúc nhắc)? → A: Không phải domain event (Technical Architect 2026-11-17). Game client (F062-AC-24) gửi telemetry `break_reminder_shown` (`reminder_no`, `minutes_played`) qua `postTelemetry`; F080 nhận, ghi `events_raw` (`source = client`), cộng `player_days.reminders_shown` và tính `sessions_over_reminder`, `sessions_over_second_reminder` theo `session_key` ([GAME_ANALYTICS §3.2, §4.6](../../docs/11-operations/GAME_ANALYTICS.md)).
- Q: operationId Game Analytics? → A: Theo API_DESIGN §9: `getGameMetrics`, `getGameRetention`, `getGameFunnel`, `getGameContentStats`, `getGamePerformance`, `createGameAnalyticsExport`, `getTenantGameSummary`; thêm `getGameAnalyticsExport` (trạng thái job). Chưa có trong OpenAPI → delta.

### Session 2026-11-17

- Q: Component hiển thị ô bị ẩn < 5? → A: Dùng lại `SuppressedCell` của F056 trong `ops-ui` (DESIGN_SYSTEM §4, RB-619); workspace E là thư mục `live-ops`.
- Q: UI báo cáo game tổng hợp cho nhà trường (workspace C) thuộc feature nào? → A: Technical Architect chốt tách F081 ([CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý)): User Story 4 cũ của F081 (tiêu chí 13–18 cũ) chuyển vào US5 của feature này thành F080-AC-27…32 (M4b); màn đặt trong chunk `school-lms` (F052), dùng component dashboard của F056; mở rộng `TenantGameSummary` theo đủ [GAME_ANALYTICS §7.2](../../docs/11-operations/GAME_ANALYTICS.md) cũng ở đây.
- Q: Academic Manager có xem báo cáo game không? GAME_ANALYTICS §8 ghi "không", nhưng ADMIN_PORTAL §2.2 và SECURITY_ARCHITECTURE cấp `analytics.school.read` cho `academic_manager`. → A: [Confirmed] Theo permission catalog (ADMIN_PORTAL §2.2 sở hữu catalog): có `analytics.tenant.read`/`analytics.school.read` ở scope tenant/trường thì xem được; UserRole chỉ scope `subject` → `403 PERMISSION_DENIED` (dữ liệu game không có chiều môn). Mâu thuẫn GAME_ANALYTICS §8 ghi báo cáo (chuyển từ F081).
- Q: Xuất CSV báo cáo game tenant? → A: Không trong GAME_ANALYTICS §7.2; ngoài phạm vi.

### Session 2026-11-21 (owner bỏ MFA — [Confirmed])

- Q: Owner bỏ MFA/TOTP cho đăng nhập Admin Portal với mọi tài khoản, kể cả role platform (2026-11-21, [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md)) — phần nào của feature này đổi? → A: F080-AC-11, AC-14, AC-25: bỏ điều kiện MFA, mã `403 MFA_REQUIRED` và step-up MFA; giữ gateway nội bộ, `azp`, permission `game.analytics.read`/`game.liveops.read`. T029, T046, quickstart, contracts nhắc MFA là lịch sử. Thay thế chung: route platform (`/admin/*`, `/game-admin/*`) chỉ còn gateway nội bộ + `azp = eduworld-portal` + `user_type = staff` + role platform (guard `platform_role`, thiếu → `403 PERMISSION_DENIED` + `authz.denied`); `CrossTenantScope` giữ `platform.cross_tenant.read` + client portal + `reason` + audit; phiên `platform_staff` bị từ chối `refreshPortalToken` sau 8 giờ từ lúc nhập mật khẩu (`EW_OIDC_PLATFORM_MAX_AUTH_AGE`, `401 REFRESH_TOKEN_INVALID`).

### Session 2026-11-22 (ADR-0094 — [Confirmed])

- Q: Văn bản còn mô tả MFA/`MFA_REQUIRED`/step-up như hành vi hiện hành? → A: [Confirmed] Viết lại theo [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md), giữ nguyên mã AC: F080-AC-11, F080-AC-14, F080-AC-25. Route platform chỉ còn gateway nội bộ + `azp = eduworld-portal` + `user_type = staff` + role platform; thiếu → `403 PERMISSION_DENIED` + `authz.denied`; không còn `403 MFA_REQUIRED` hay hộp đăng nhập lại (step-up). Câu trả lời Clarifications cũ nhắc MFA là lịch sử.

### Session 2026-11-26 (triển khai — owner "chốt hết", [Confirmed])

- Q: Bảng `analytics.*` dùng ent hay viết tay, có partition không? → A: [Confirmed] Viết tay như `game.login_events` (khóa ghép, không mixin, SQL tường minh có `tenancyguard:ok`), **chưa partition** (index theo ngày; partition tháng/năm khi chạm cỡ DASHBOARD_DATA §11 — cùng quyết định với ledger F077). Thêm cột `dim_set` (tập chiều), `players` (số người chơi của ô — dùng ẩn ô), `hist`, `is_final`; bảng tuần dùng `metric_date` = thứ Hai ISO thay `iso_week`; `player_days` khóa thay thế `(activity_date, day_id)` + UNIQUE `(activity_date, player_key)` vì `player_key` NULL sau ẩn danh, thêm `reminders_shown`, `late_night`, `social`, `matches`, `purchases`, `level`; thêm `analytics.rollup_days` (trạng thái chốt ngày) và `analytics.export_jobs`. DATABASE_DESIGN §2.8b, GAME_ANALYTICS §6 cập nhật.
- Q: Tập chiều cố định và bộ lọc nhiều chiều? → A: [Confirmed] 6 tập GAME_ANALYTICS §6 + `(trường, khối)`, `(trường, nền tảng)`, `(trường, hạng thiết bị)` cho báo cáo School Admin; API platform chỉ lọc **một** chiều (hai chiều → `422 VALIDATION_FAILED`); `currency_sources/sinks` thêm khóa `reason_code`, `event_participation_rate` thêm `event_id` (cấp tenant).
- Q: Ẩn ô < `analytics_min_cell_size` tính trên gì, có áp cho view platform? → A: [Confirmed] Theo `players` của ô (người chơi phân biệt của quần thể chỉ số); ẩn bổ sung (ô nhỏ nhất còn lại khi chỉ một ô bị ẩn) trên **toàn bộ giá trị của chiều** trong kỳ (không chỉ giá trị được lọc) để hai bộ lọc khác nhau không trừ được nhau; tổng theo khoảng của báo cáo tenant bị ẩn nếu bất kỳ kỳ con nào bị ẩn (chống trừ theo khoảng ngày); áp cho cả API platform (DASHBOARD_DATA §8 quy tắc 7). Tenant không bị ẩn theo tập `tenant_ids` (nhân sự platform nội bộ).
- Q: Số liệu "hôm nay" bằng HyperLogLog Valkey? → A: [Confirmed] Không cần: job hằng giờ tính chính xác từ `player_days` (dòng `is_final = false`); Valkey chỉ còn cache theo phiên bản tenant và bộ đếm đồng thời.
- Q: Chỉ số §4 nào có trong catalog? → A: [Confirmed] Catalog `domain/analytics.Catalog` (30 mã): đủ §4.1, §4.5, §4.6, §4.7 (bảng `perf_daily`), `chapter0_completion`, `challenge_success_rate`, `boss_defeat_rate`, `abandon_rate`, `level_distribution`, `xp_cap_hit_rate`, §4.4 trừ chỉ số cần trạng thái tích lũy theo người; thêm `quest_completion_rate` (GAME_ANALYTICS §4.2). `region_unlock_rate`, `map_completion_p50`, `days_to_level_p50`, `chapter_completion_rate` cần trạng thái mở vùng/chương theo người (chưa có nguồn) → chưa vào catalog, UI hiển thị "Chưa có số liệu".
- Q: `active_seconds` và cờ đêm khuya lấy từ đâu? → A: [Confirmed] `game_session.ended.duration_ms / 1000`; `late_night` tính ở consumer theo `analytics_late_night_window` từ `ended_at - duration_ms`.
- Q: Funnel onboarding? → A: [Confirmed] Cohort = ngày hoạt động đầu tiên (`first_active_date`); bước: `first_login`, `avatar_created`, `tutorial_completed` (≥ 1 `tutorial_step` `result = done`), `first_quest_accepted`, `first_quest_completed`, `chapter0_completed` (`story.chapter_completed` có `order = 0` hoặc `chapter_code` `chapter.0.*`), trong 7 ngày.
- Q: Cờ telemetry sản phẩm của tenant? → A: [Confirmed] Module game đọc `ref.tenants.settings_subset.analytics_product_telemetry_enabled` (projection `tenant.updated`, vắng mặt = bật — không thêm cột). Phía phát (thêm khóa vào `TenantSettings` và `TenantSubset` của identity) là delta của F015 — consumer-first bằng fixture như CODE_PLAN §4.1. `getClientConfig` nhận `tenant_id` tùy chọn để trả cờ hiệu lực của tenant (endpoint công khai, không có tenant khác).
- Q: Tenant QA? → A: [Confirmed] Theo GAME_ANALYTICS A-8 với `ref.tenants.is_qa` sẵn có: telemetry/consumer bỏ sự kiện (metric `qa_tenant`), rollup bỏ ngày của tenant QA, truy vấn platform loại tenant QA (chỉ định toàn tenant QA → rỗng). Trường `excluded_tenant_ids` của phản hồi thuộc F134.
- Q: Live dashboard đọc gì? → A: [Confirmed] Catalog PromQL cố định `query.KPICatalog` (18 KPI); ngưỡng `liveops_kpi_thresholds` (CONFIG_DEFAULTS); `reports_pending` = báo cáo mới trong 1 giờ (gauge hàng chờ thuộc F122/F131); thời lượng trận = histogram `eduworld_match_duration_seconds` do consumer analytics ghi từ `match_start` → `match.completed`; hiệu năng DB = `go_sql_*` của pool game; `tenant_ids` không lọc (metric kỹ thuật không có nhãn tenant). Test dùng HTTP Prometheus giả (test double R17) thay testcontainer.
- Q: Giám sát sự kiện lấy danh sách run ở đâu? → A: [Confirmed] Thêm `listEventRuns` (`GET /game-admin/event-runs`, `game.liveops.read`, `action.game_ops.events.stats`, keyset) — chỉ run sự kiện platform.
- Q: Export? → A: [Confirmed] Hàng đợi là bảng `export_jobs` + job `analytics-export` mỗi 15 s (`FOR UPDATE SKIP LOCKED`), không NATS work-queue; chỉ export platform (`tenant_id` NULL); CSV qua cùng query catalog (đã ẩn ô) lên bucket `ew-exports`, link ký GET; `ContentStatsItem.title` không trả (portal mở version hiện hành qua `listGameConfigVersions`).
- Q: Phiên dashboard và đọc liên tenant? → A: [Confirmed] Đọc tenant bằng `game_dashboard`; đọc platform bằng `tenancy.Bypass(platform_analytics)` có audit trên pool `game_platform` của phiên dashboard (`EW_DB_GAME_DASHBOARD_PLATFORM_DSN`, replica) — giống `learning_platform` của F043. Không có DSN → `503`.
- Q: Khóa HMAC và xoay khóa? → A: [Confirmed] `EW_ANALYTICS_PLAYER_KEYS` = `version:secret,…` (secret store); khóa mới nhất dùng cho dòng mới, `key_version` lưu theo dòng, ẩn danh tính `player_key` với mọi phiên bản. Bảng ánh xạ phiên bản cho retention qua mốc xoay: chưa làm (lần xoay đầu sau 24 tháng). Không có khóa → consumer analytics không chạy, telemetry `503`.
- Q: Báo cáo tenant: `active_students`, phân bố thiết bị? → A: [Confirmed] `active_students` = `mau` tại ngày cuối khoảng (≤ hôm nay); chuỗi `active_students_series` = dau/wau/mau theo `grain`; phân bố nền tảng/thiết bị không có khi lọc khối; School Admin chỉ đọc trường của grant (không lọc → trường của grant).
- Q: Rate limit telemetry? → A: [Confirmed] Trong use case (bộ đếm Valkey `ew:game:rl:telemetry:<user>`), `EW_TELEMETRY_RATE_PER_MINUTE` = 30.
- Q: Catalog menu cho School Admin? → A: [Confirmed] `action.school_lms.reports.view` và `menu.school_lms.reports` thêm `analytics.school.read`.
- Q: Portal workspace E? → A: [Confirmed] Thẻ KPI dùng `DashboardWidget` (dự án không có `@ant-design/pro-components`); không thêm `GrainLabel`/`DataAsOf` vì `DashboardWidget` đã hiển thị độ hạt, "Cập nhật lúc", "Tạm tính"; dropdown tenant chỉ hiện cho System Admin (`platform.cross_tenant.read`), role `liveops` xem tổng mọi tenant không QA.

### Session 2026-11-26 (follow-up — owner hoãn)

- Q: Phía phát cờ `analytics_product_telemetry_enabled` của tenant (khóa trong `TenantSettings`/`settings_subset` của identity, delta F015)? → A: **Hoãn** (owner 2026-11-26: "cài đặt telemetry để sau vì chỉ xem log"). Module game giữ hành vi hiện tại: đọc `ref.tenants.settings_subset.analytics_product_telemetry_enabled`, vắng mặt = bật theo giá trị platform (CONFIG_DEFAULTS); identity chưa phát khóa này nên mọi tenant hiện dùng mặc định. Không có thay đổi contract/settings/migration nào cho cờ này.
- Q: `excluded_tenant_ids` (loại tenant QA khỏi analytics, F134-AC-09)? → A: **Hoãn** cùng đợt, làm trong F134; F080 tiếp tục loại tenant QA theo `ref.tenants.is_qa` như đã triển khai.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Telemetry client không PII (Priority: P1) 🎯 MVP

Game client gửi lô `perf_sample`, `load_time`, `screen_view`, `crash`; trường tắt telemetry sản phẩm thì chỉ còn hiệu năng/crash.

**Why this priority**: Nguồn dữ liệu hiệu năng thiết bị và funnel client (FR-GAME-011); phải đúng quyền riêng tư ngay từ đầu.

**Independent Test**: `postTelemetry` lô 100 sự kiện → 202; thuộc tính lạ → 422; tenant tắt telemetry sản phẩm → `screen_view` bị bỏ, `perf_sample` vào raw.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F080-AC-01 | KHI học sinh (middleware `game_player`) gọi `postTelemetry` với `TelemetryBatch` (≤ 100 sự kiện), module game PHẢI validate schema đóng (thuộc tính ngoài danh mục → `422 VALIDATION_FAILED`), khử trùng theo `client_event_id`, trả `202` | FR-GAME-011 · NFR-OBS-003 · [GAME_ANALYTICS §3.2](../../docs/11-operations/GAME_ANALYTICS.md) · AGENTS R23 | TC-ANL-003 |
| F080-AC-02 | NẾU vượt rate limit của endpoint (30 req/phút/người theo spec), THÌ module game PHẢI trả `429 RATE_LIMITED` kèm `Retry-After`; token không phải học sinh PHẢI nhận `403 GAME_ACCESS_STUDENT_ONLY` | FR-GAME-011 · AGENTS R23 | TC-ANL-003 |
| F080-AC-03 | TRONG KHI tenant có `analytics_product_telemetry_enabled = false`, module game PHẢI bỏ `screen_view`/`tutorial_step` và vẫn nhận `perf_sample`, `load_time`, `client_error`, `crash`; `getClientConfig` PHẢI trả cờ để client không gửi | FR-ANL-011 · AC-ANL-107 | TC-ANL-007 |
| F080-AC-04 | HỆ THỐNG PHẢI tách `client_error`/`crash` sang error tracking (log có `crash_signature`, không PII) và chỉ ghi vào analytics các trường danh mục; log INFO PHẢI không chứa token, `student_id`, tên | NFR-OBS-003 · AGENTS R14 · [MONITORING](../../docs/11-operations/MONITORING.md) | TC-ANL-003 |
| F080-AC-34 | KHI `postTelemetry` chứa sự kiện `break_reminder_shown`, module game PHẢI kiểm `reminder_no` (1–20), `minutes_played` (0–1.440) và chỉ hai thuộc tính này (khác → `422 VALIDATION_FAILED`), gắn `session_key` từ game session của token (không nhận từ client), khử trùng `client_event_id`, ghi `events_raw` (`event_name = break_reminder_shown`, `source = client`) kể cả khi tenant tắt telemetry sản phẩm; giá trị PHẢI không dùng để chặn chơi hay tính thưởng | FR-ANL-007 · FR-ANL-010 · [GAME_ANALYTICS §3.2](../../docs/11-operations/GAME_ANALYTICS.md) · ADR-0016 | TC-ANL-003 |

---

### User Story 2 — Analytics consumer và dữ liệu thô giả danh (Priority: P1)

Domain event của game/competition/chat/learning được ánh xạ thành sự kiện analytics với `player_key`, không có định danh thật.

**Why this priority**: FR-ANL-007, FR-ANL-008 — nền tảng mọi chỉ số.

**Independent Test**: Phát `game_session.started`, `quest.completed`, `match.completed` → `analytics.events_raw` có dòng `player_key`, chiều chuẩn; giao lại cùng `event_id` → không trùng.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F080-AC-05 | KHI durable consumer `analytics` nhận domain event trong danh mục [GAME_ANALYTICS §3.1](../../docs/11-operations/GAME_ANALYTICS.md), module game PHẢI ánh xạ sang `event_name` analytics, tính `player_key`/`session_key` bằng HMAC-SHA256 với khóa bí mật từ secret store, gắn chiều chuẩn, bỏ thuộc tính ngoài danh mục, ghi `analytics.events_raw` idempotent theo `event_id` | FR-ANL-007 · FR-ANL-008 · ADR-0058 | TC-ANL-002 |
| F080-AC-06 | HỆ THỐNG PHẢI không có cột/khóa `student_id`, tên, nickname, nội dung chat trong schema `analytics` và `props` (≤ 2 KB); kiểm tự động trên schema và payload | FR-ANL-008 · AC-ANL-103 · AGENTS R14 | TC-ANL-003 |
| F080-AC-07 | KHI có yêu cầu ẩn danh một học sinh, module game PHẢI tính `player_key` và đặt NULL `player_key`/`session_key` ở raw và `player_days`; bảng tổng hợp không đổi; không xóa vật lý | FR-ANL-008 · ADR-0070 · AGENTS R21 | TC-ANL-010 |

---

### User Story 3 — Rollup chỉ số (Priority: P1)

Job hằng giờ và job chốt ngày tính DAU/WAU/MAU, retention, funnel, nội dung, hiệu năng, chơi lành mạnh.

**Why this priority**: FR-ANL-006, FR-ANL-010; dashboard chỉ đọc bảng tổng hợp (ADR-0062).

**Independent Test**: Golden dataset → `rollup_daily`, `cohort_retention`, `funnel_daily` khớp tính tay; event trễ 30 giờ tính lại đúng ngày.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F080-AC-08 | KHI job rollup hằng giờ và job chốt ngày (`dashboard_day_close_time`) chạy, module game PHẢI cập nhật `player_days`, `rollup_daily/weekly/monthly`, `cohort_retention`, `funnel_daily`, `content_stats_weekly`, `perf_daily` theo đúng công thức §4 (phiên < `analytics_active_session_min_seconds` không tính hoạt động), đánh `is_final` sau chốt | FR-ANL-006 · AC-ANL-101 · AC-ANL-102 · [DASHBOARD_DATA §4](../../docs/07-backend/DASHBOARD_DATA.md) | TC-ANL-001 |
| F080-AC-09 | KHI event đến trễ ≤ `analytics_late_event_window`, job PHẢI tính lại bucket ngày cũ và tăng phiên bản cache tenant; trễ hơn PHẢI chỉ đếm metric, không sửa số đã chốt | FR-ANL-006 · AC-ANL-106 | TC-DASH-005 |
| F080-AC-10 | HỆ THỐNG PHẢI tính chỉ số chơi lành mạnh (`sessions_over_reminder`, `sessions_over_second_reminder`, `late_night_play_rate` theo `analytics_late_night_window`, `daily_playtime_p90`) và PHẢI không có chỉ số "tổng thời gian chơi" làm mục tiêu tăng | FR-ANL-010 · AC-ANL-109 · AGENTS R19 | TC-ANL-001 |
| F080-AC-35 | KHI job rollup xử lý `break_reminder_shown`, module game PHẢI cộng `player_days.reminders_shown`, tính `sessions_over_reminder` = số `session_key` có ≥ 1 sự kiện / số phiên và `sessions_over_second_reminder` = số `session_key` có sự kiện `reminder_no` ≥ 2 / số phiên trong ngày, theo tập chiều chuẩn; sự kiện trùng `client_event_id` PHẢI chỉ tính một lần | FR-ANL-010 · AC-ANL-109 · [GAME_ANALYTICS §4.6](../../docs/11-operations/GAME_ANALYTICS.md) | TC-ANL-001 |

---

### User Story 4 — API Game Analytics cho platform (Priority: P1)

Live-ops truy vấn chuỗi `dau`, retention cohort, funnel onboarding, top quest bị bỏ, hiệu năng theo hạng thiết bị; xuất CSV.

**Why this priority**: FR-ANL-009 — đầu vào của UI.

**Independent Test**: Gọi từng operationId với khoảng 7/90/730 ngày → `grain` đúng; 731 ngày → 422; người dùng thứ 5 đồng thời → 429.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F080-AC-11 | KHI người có `game.analytics.read` (token `eduworld-portal` + role platform, gateway nội bộ) gọi `getGameMetrics`/`getGameRetention`/`getGameFunnel`/`getGameContentStats`/`getGamePerformance` với `from`/`to`, module game PHẢI đọc chỉ bảng tổng hợp trên read replica bằng role `game_dashboard`, lọc `tenant_ids` (audit `cross_tenant.accessed`), trả `grain`, `data_as_of`, `is_partial`, ≤ 120 điểm | FR-ANL-009 · ADR-0062 · [DASHBOARD_DATA §7–§8](../../docs/07-backend/DASHBOARD_DATA.md) | TC-DASH-003 |
| F080-AC-12 | NẾU thiếu `from`/`to` hoặc khoảng > `dashboard_max_range_days`, THÌ module game PHẢI trả `422 DASHBOARD_RANGE_TOO_LARGE`/`VALIDATION_FAILED`; vượt `dashboard_max_concurrent_per_user` → `429 DASHBOARD_BUSY`; quá `dashboard_statement_timeout` → `504 DASHBOARD_QUERY_TIMEOUT` | FR-ANL-009 · AC-DASH-002 · AC-DASH-004 · AC-DASH-005 | TC-DASH-002 |
| F080-AC-13 | KHI gọi `createGameAnalyticsExport` với `Idempotency-Key`, module game PHẢI tạo job `202`, worker ghi CSV lên object storage `ew-exports`, `getGameAnalyticsExport` trả link ký hạn `dashboard_export_link_ttl_minutes`; vượt `dashboard_export_max_concurrent_per_user` → `429 EXPORT_LIMIT_REACHED`; audit `report.exported` | FR-ANL-009 · [DASHBOARD_DATA §10](../../docs/07-backend/DASHBOARD_DATA.md) | TC-DASH-008 |
| F080-AC-14 | NẾU role tenant gọi `/admin/analytics/game/*`, THÌ gateway public PHẢI không có route và backend trả `403 PERMISSION_DENIED` (không MFA — ADR-0094) | FR-ANL-009 · AC-ANL-105 · AGENTS R20 | TC-ANL-006 |

---

### User Story 5 — Báo cáo game tổng hợp của tenant (Priority: P2)

`ad.donvi` mở "Hoạt động game" (workspace C) của `pgd-demo`, lọc 30 ngày, theo trường và khối: học sinh hoạt động, số ngày chơi/tuần, hoàn thành chương 0 và quest, chơi cùng bạn, chơi lành mạnh, nền tảng/thiết bị; ô ít học sinh hiển thị "< 5".

**Why this priority**: FR-ANL-009 phần tenant; API và màn workspace C (chuyển từ F081 US4).

**Independent Test**: `getTenantGameSummary` với tenant/trường khác → 404/403; nhóm < 5 → null + `suppressed_cells`; seed rollup có nhóm 3 học sinh → ô "< 5" trên màn; khoảng 731 ngày → gợi ý thu hẹp; `ad.truong.a` lọc trường B → 403.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F080-AC-15 | KHI Tenant Admin (`analytics.tenant.read`) hoặc School Admin (`analytics.school.read`, chỉ trường mình) gọi `getTenantGameSummary`, module game PHẢI chỉ trả dữ liệu tenant của phiên (`X-Tenant-Id`), lọc `school_ids` trong scope (ngoài scope → `403`/`404`), không có số liệu kinh tế chi tiết hay danh sách học sinh | FR-ANL-009 · AC-ANL-104 · AGENTS R8 | TC-ANL-004 |
| F080-AC-16 | NẾU một ô có < `analytics_min_cell_size` học sinh, THÌ giá trị PHẢI là `null`, `suppressed_cells` tăng, và tổng không suy ra được ô bị ẩn | FR-ANL-009 · AC-DASH-011 | TC-ANL-005 |
| F080-AC-27 | KHI người có `analytics.tenant.read` hoặc `analytics.school.read` mở "Hoạt động game", Portal PHẢI gọi `getTenantGameSummary` với `from`/`to` (preset + tùy chọn ≤ `dashboard_max_range_days`), `school_ids` (chỉ hiện với tenant `unit` và trường trong scope), `grade_bands`; mỗi nhóm chỉ số là một widget tải độc lập | FR-ANL-009 · FR-ANL-012 · [GAME_ANALYTICS §7.2](../../docs/11-operations/GAME_ANALYTICS.md) | TC-DASH-003 |
| F080-AC-28 | Portal PHẢI hiển thị đúng bộ chỉ số [GAME_ANALYTICS §7.2](../../docs/11-operations/GAME_ANALYTICS.md) theo nhóm Tổng quan, Tiến trình, Xã hội, Chơi lành mạnh, Thiết bị; chỉ số thời lượng PHẢI chỉ ở dạng trung vị và nằm trong Tổng quan/Chơi lành mạnh; PHẢI không có số liệu kinh tế chi tiết, danh sách học sinh hay chỉ số "tổng thời gian chơi"; trên điện thoại màn PHẢI chỉ hiển thị thẻ Tổng quan/Chơi lành mạnh ở chế độ xem | FR-ANL-009 · FR-ANL-010 · AC-ANL-109 · AGENTS R19 | — |
| F080-AC-29 | NẾU một giá trị là `null` do ẩn ô, THÌ Portal PHẢI hiển thị "< 5" (component dùng chung) kèm chú thích số ô bị ẩn (`suppressed_cells`), và PHẢI không tự cộng/trừ các ô để hiển thị tổng | FR-ANL-009 · AC-ANL-104 | TC-ANL-005 |
| F080-AC-30 | Portal PHẢI áp quy tắc dashboard [ADMIN_PORTAL §3.14.1](../../docs/04-admin-portal/ADMIN_PORTAL.md): nhãn độ hạt theo `grain`, "Cập nhật lúc" theo `data_as_of`, "Tạm tính" khi `is_partial`; `422 DASHBOARD_RANGE_TOO_LARGE` → gợi ý thu hẹp; `429 DASHBOARD_BUSY` → tự thử lại 2–5 s tối đa 2 lần; `504 DASHBOARD_QUERY_TIMEOUT` → gợi ý thu hẹp phạm vi | FR-ANL-013 · ADR-0062 | TC-DASH-002 |
| F080-AC-31 | KHI trả `TenantGameSummary` mở rộng (field ở [data-model §5](data-model.md)), module game PHẢI tính mọi field mới chỉ từ bảng tổng hợp `analytics.*` trên read replica (không đọc `events_raw`/`player_days`), áp ẩn ô theo `analytics_min_cell_size` cho cả field mới, và đặt `is_partial`/`data_as_of` đúng | FR-ANL-009 · FR-ANL-013 · ADR-0062 · [DASHBOARD_DATA §8](../../docs/07-backend/DASHBOARD_DATA.md) | TC-DASH-001 |
| F080-AC-32 | NẾU Tenant Admin gọi với tenant khác, School Admin lọc trường ngoài scope, hoặc UserRole chỉ scope `subject`, THÌ module game PHẢI trả `403 TENANT_MISMATCH` / `403 PERMISSION_DENIED`; người không có `analytics.*.read` PHẢI nhận `403 PERMISSION_DENIED` và Portal PHẢI không hiện mục menu | FR-ANL-009 · AC-ANL-104 · AGENTS R8 | TC-ANL-004 |

---

### User Story 6 — Live dashboard và giám sát sự kiện (Priority: P1)

Live-ops mở Live dashboard: CCU, phiên, trận, hàng chờ ghép trận, đăng nhập/hàng chờ, tin bị chặn, báo cáo chờ, lỗi theo service, publish/rollback gần đây, SIS sync; mở sự kiện đang chạy, tạm dừng khẩn cấp có lý do.

**Why this priority**: FR-CMS-016, FR-OPS-003 — vận hành giờ học cần thấy sự cố ngay.

**Independent Test**: Tải giả lập CCU → số liệu đổi trong ≤ 30 s; ngưỡng vượt → thẻ cảnh báo; dừng sự kiện không lý do → bị chặn.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F080-AC-17 | KHI người có `game.liveops.read` gọi `getLiveOpsOverview` (`tenant_ids` tùy chọn), module game PHẢI trả các KPI ở [DESIGN_SYSTEM §4.3](../../docs/05-ux/DESIGN_SYSTEM.md) đọc từ Prometheus (truy vấn định nghĩa sẵn, không truy vấn tự do), kèm `as_of` và trạng thái ngưỡng `normal`/`warning`/`incident`; Prometheus không phản hồi → `503` có `code`, không trả số 0 giả | FR-CMS-016 · FR-OPS-003 · [DASHBOARD_DATA §2](../../docs/07-backend/DASHBOARD_DATA.md) | TC-UI-004 |
| F080-AC-18 | HỆ THỐNG PHẢI có metric cho mọi KPI của FR-OPS-003 (online players, active sessions, matchmaking queue, match duration, API latency, error rate, chat rate, learning completion, exam submission, DB performance) với nhãn không chứa user id/nội dung | FR-OPS-003 · [MONITORING](../../docs/11-operations/MONITORING.md) · AGENTS R14 | — |
| F080-AC-19 | KHI mở Giám sát sự kiện, Portal PHẢI hiển thị `getEventRunStats` (tham gia, tiến độ community goal, tỷ lệ nhận thưởng, lỗi); KHI người có `game.event.operate` bấm Tạm dừng/Tiếp tục/Hủy, Portal PHẢI bắt buộc lý do, xác nhận 2 bước và gọi `suspendEventRun`/`resumeEventRun`/`cancelEventRun` với `Idempotency-Key` | FR-CMS-016 · FR-EVT-003 · [ADMIN_PORTAL §3.15](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-EVT-003 |
| F080-AC-20 | Portal PHẢI tự làm mới Live dashboard 15–30 giây, hiển thị "Cập nhật lúc HH:mm:ss", và cho chuyển nhanh sang module Kiểm duyệt khi KPI báo cáo chờ vượt ngưỡng (nếu người dùng có quyền) | FR-CMS-016 · [ADMIN_PORTAL §3.14](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-UI-004 |

---

### User Story 7 — UI Game Analytics (Priority: P1)

Game designer xem 11 panel, lọc khối/nền tảng/hạng thiết bị, so sánh kỳ, mở quest bị bỏ nhiều trong workspace A để chỉnh.

**Why this priority**: FR-ANL-006, FR-ANL-009 — giá trị nhìn thấy được của analytics.

**Independent Test**: Mở từng panel với dữ liệu seed; widget tải độc lập; `DASHBOARD_BUSY` tự thử lại tối đa 2 lần.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F080-AC-21 | KHI người có `game.analytics.read` mở Game Analytics, Portal PHẢI hiển thị 11 panel theo [GAME_ANALYTICS §7.1](../../docs/11-operations/GAME_ANALYTICS.md) với bộ lọc khoảng ngày, tenant (nhiều), trường, khối/grade band, nền tảng, ngôn ngữ, hạng thiết bị, phiên bản client, so sánh kỳ; mỗi widget tải độc lập, ≤ 4 yêu cầu song song | FR-ANL-006 · FR-ANL-009 · [ADMIN_PORTAL §3.14](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-ANL-008 |
| F080-AC-22 | Portal PHẢI áp quy tắc UI dashboard [ADMIN_PORTAL §3.14.1](../../docs/04-admin-portal/ADMIN_PORTAL.md): preset khoảng ≤ 2 năm, nhãn độ hạt, "Cập nhật lúc", "Tạm tính", "Đang tính số liệu…", "Chưa có số liệu trước ngày …", xử lý `DASHBOARD_RANGE_TOO_LARGE`/`DASHBOARD_BUSY` (thử lại 2–5 s, tối đa 2 lần)/`DASHBOARD_QUERY_TIMEOUT` | FR-ANL-009 · ADR-0062 | TC-DASH-003 |
| F080-AC-23 | KHI bấm một quest/challenge/boss trong panel nội dung, Portal PHẢI mở item tương ứng trong workspace A (nếu có quyền); nút Xuất PHẢI tạo job bất đồng bộ và báo khi sẵn sàng; chỉ số thời lượng PHẢI chỉ ở panel Chơi lành mạnh và dạng trung vị ở Tổng quan | FR-ANL-010 · AC-ANL-109 | TC-DASH-008 |

---

### User Story 8 — Bảo vệ workspace E (Priority: P1)

**Why this priority**: Constitution điều V; Safety Moderator chỉ thấy Kiểm duyệt.

**Independent Test**: `sm.antoan` mở Live dashboard/Game Analytics → 403; `ad.donvi` mở `/live-ops` → 403, không tải chunk.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F080-AC-24 | KHI người dùng không có `game.liveops.read`/`game.analytics.read` (kể cả Safety Moderator) mở URL Live dashboard/Game Analytics, Portal PHẢI hiển thị trang 403; role tenant PHẢI không tải được chunk workspace E | FR-ANL-009 · ADR-0027 · AGENTS R20 | TC-CMS-WS-01 |
| F080-AC-25 | NẾU API trả `403 PERMISSION_DENIED`, THÌ Portal PHẢI hiển thị trang 403 kèm `trace_id` (không có step-up MFA — ADR-0094); NẾU gateway nội bộ không kết nối được, THÌ hiển thị "Cần kết nối mạng quản trị (VPN)" | ADR-0027 · [ADMIN_PORTAL §1.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-WS-02 |
| F080-AC-26 | HỆ THỐNG PHẢI giới hạn mọi tham số chuỗi (`metric_codes`, `client_versions`, lý do dừng sự kiện…) theo `maxLength`/`maxItems` của schema và trả `422 VALIDATION_FAILED` khi vượt | ADR-0041 · AGENTS R28 | TC-TEN-024 |
| F080-AC-33 | KHI menu/action của feature (`menu.live_ops.game_analytics`, `menu.live_ops.live_dashboard`, `menu.school_lms.reports` và action con) đang tắt cho phạm vi phiên (`disabled_menus`/`disabled_actions` của `getMyPermissions`), Portal PHẢI ẩn menu và ẩn/khóa nút tương ứng (`<ActionGate actionKey>`); NẾU API trả `403 FEATURE_DISABLED`, Portal PHẢI hiện thông báo tính năng tạm tắt và tải lại quyền | FR-CMS-018 · ADR-0090 | TC-CMS-MENU-02 |

### Edge Cases

- NẾU khóa HMAC `player_key` xoay (sau `analytics_player_key_rotation_months`), THÌ retention qua mốc xoay tính theo bảng ánh xạ phiên bản khóa [Confirmed]; không lưu `student_id`.
- NẾU Valkey mất, THÌ API vẫn trả đúng số (chậm hơn, không cache) (AC-DASH-012).
- NẾU chưa có dữ liệu cho chỉ số mới, THÌ trả `available_from`; UI (workspace E và màn "Hoạt động game") hiển thị "Chưa có số liệu trước ngày …" thay vì 0.
- NẾU tenant tắt telemetry sản phẩm, THÌ chỉ số từ telemetry client (phân bố thiết bị) trên màn "Hoạt động game" có thể thiếu → hiển thị "Không có dữ liệu", không lỗi.
- NẾU client gửi `occurred_at` tương lai quá 5 phút hoặc cũ hơn cửa sổ trễ, THÌ bỏ sự kiện và đếm metric, không lỗi lô.
- NẾU sự kiện đang `Suspended` mà bấm Tạm dừng lần nữa, THÌ server trả `409`; UI tải lại trạng thái.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-ANL-006 | Game Analytics tách riêng; rollup và panel chỉ số | [GAME_ANALYTICS §1, §4](../../docs/11-operations/GAME_ANALYTICS.md) |
| FR-ANL-007 | Consumer domain event + telemetry, schema `analytics` | [GAME_ANALYTICS §3, §5](../../docs/11-operations/GAME_ANALYTICS.md) · ADR-0058 |
| FR-ANL-008 | `player_key` giả danh, không PII | [GAME_ANALYTICS §2](../../docs/11-operations/GAME_ANALYTICS.md) |
| FR-ANL-009 | Dashboard workspace E; báo cáo tenant ẩn ô < 5 | [GAME_ANALYTICS §7–§8](../../docs/11-operations/GAME_ANALYTICS.md) |
| FR-ANL-010 | Chỉ số chơi lành mạnh; không dùng thời gian chơi làm thước đo thành công | [GAME_ANALYTICS §4.6](../../docs/11-operations/GAME_ANALYTICS.md) |
| FR-ANL-011 | Tenant tắt telemetry sản phẩm (cơ chế kỹ thuật) | [GAME_ANALYTICS §3.2](../../docs/11-operations/GAME_ANALYTICS.md) |
| FR-ANL-012 | Trang dashboard cấp tenant cho dữ liệu game (màn "Hoạt động game" workspace C) | [DASHBOARD_DATA §2](../../docs/07-backend/DASHBOARD_DATA.md) |
| FR-ANL-013 | Quy tắc UI dashboard (khoảng ≤ 2 năm, độ hạt, thời điểm, tạm tính) cho màn workspace C | [ADMIN_PORTAL §3.14.1](../../docs/04-admin-portal/ADMIN_PORTAL.md) · ADR-0062 |
| FR-CMS-016 | Live dashboard KPI live-ops | [DESIGN_SYSTEM §4.3](../../docs/05-ux/DESIGN_SYSTEM.md) · [ADMIN_PORTAL §3.14](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-GAME-011 | Telemetry client không PII, rate limit | [GAME_ANALYTICS §3.2](../../docs/11-operations/GAME_ANALYTICS.md) |
| FR-OPS-003 | Metric vận hành hiển thị trên Live dashboard | [MONITORING](../../docs/11-operations/MONITORING.md) |
| NFR-OBS-003 | Telemetry không PII | [GAME_ANALYTICS §2](../../docs/11-operations/GAME_ANALYTICS.md) |
| FR-EVT-003 | Thao tác dừng/tiếp tục/hủy sự kiện từ workspace E | [EVENT_SEASON_SYSTEM §5](../../docs/01-game-design/EVENT_SEASON_SYSTEM.md) |
| FR-CMS-018 | Menu/action của feature theo danh mục `specs/portal/menu-catalog.yaml`, ẩn khi tắt, xử lý `403 FEATURE_DISABLED` | [ADMIN_PORTAL §2.7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |

### Key Entities

- **AnalyticsEvent (raw)**: sự kiện đã giả danh, chiều chuẩn, thuộc tính danh mục.
- **PlayerDay**: hoạt động một người chơi giả danh trong ngày.
- **Rollup (ngày/tuần/tháng)**, **CohortRetention**, **FunnelDaily**, **ContentStatsWeekly**, **PerfDaily**: số liệu tổng hợp.
- **AnalyticsExportJob**: job xuất CSV, link ký.
- **LiveOpsOverview**: ảnh chụp KPI thời gian thực, không lưu.
- **TenantGameSummary**: bộ chỉ số game tổng hợp của tenant/trường theo khoảng ngày, có ô bị ẩn (mở rộng đủ GAME_ANALYTICS §7.2).

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `analytics_raw_retention_days` · `analytics_player_days_retention_days` · `analytics_rollup_retention_years` | Ẩn danh hóa khi hết hạn (không xóa) |
| `analytics_player_key_rotation_months` | Xoay khóa HMAC |
| `analytics_min_cell_size` | Ẩn ô nhỏ (API và màn "Hoạt động game") |
| `analytics_active_session_min_seconds` | Định nghĩa "hoạt động" |
| `analytics_late_night_window` · `analytics_late_event_window` · `analytics_perf_sample_interval` · `analytics_product_telemetry_enabled` | Chỉ số, cửa sổ trễ, mẫu perf, tắt telemetry |
| `dashboard_max_range_days` · `dashboard_statement_timeout` · `dashboard_max_concurrent_per_user` · `dashboard_max_concurrent_per_tenant` · `dashboard_cache_ttl_today` · `dashboard_cache_ttl_closed` · `dashboard_day_close_time` · `dashboard_page_size_max` | Bảo vệ truy vấn, cache, chốt ngày |
| `dashboard_export_max_rows` · `dashboard_export_max_concurrent_per_user` · `dashboard_export_link_ttl_minutes` · `dashboard_export_file_retention_days` | Xuất CSV |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Dashboard chỉ đọc rollup trên replica, `statement_timeout` | ADR-0062 · NFR-PERF-017 |
| Không PII trong analytics/telemetry; không lưu IP | NFR-OBS-003 · AGENTS R14 |
| Raw không purge (ẩn danh hóa) | ADR-0070 · AGENTS R21 |
| `game.analytics.read` không bao giờ cấp cho role tenant | AGENTS R20 |
| Consumer idempotent, outbox | ADR-0042 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Truy vấn dashboard p95 ≤ `dashboard_query_p95_target` trên dữ liệu 2 năm (kịch bản tải DASHBOARD_DATA §15).
- **SC-002**: 0 trường định danh thật trong schema/payload analytics (quét tự động).
- **SC-003**: Live dashboard phản ánh thay đổi CCU trong ≤ 30 giây.
- **SC-004**: 100% ô báo cáo tenant có < 5 học sinh bị ẩn và hiển thị "< 5" trên màn "Hoạt động game".
- **SC-006**: Trang "Hoạt động game" hiển thị widget đầu tiên trong ≤ 3 giây p95 trên dữ liệu 2 năm (kịch bản tải DASHBOARD_DATA §15).
- **SC-005**: Số liệu golden dataset khớp tuyệt đối sau chốt ngày.

## Ngoài phạm vi

- Sự kiện trường từ template và tiến độ sự kiện trường (workspace C): F081.
- Endpoint `/game/telemetry` lát mỏng ban đầu và `getClientConfig`: F014 (feature này mở rộng). Cài đặt tenant `product_telemetry_enabled`: F015.
- Alert, dashboard Grafana, runbook hạ tầng: F131. Builder sự kiện (workspace A): F079. Module Kiểm duyệt (E): F122.
- Kho phân tích riêng (ClickHouse): chỉ khi chạm ngưỡng `analytics_store_migration_threshold`, cần ADR mới.

## Assumptions

- [Confirmed] Live dashboard đọc Prometheus HTTP API qua adapter trong module game, URL `EW_PROMETHEUS_URL` từ env (R3); query catalog cố định trong code.
- [Confirmed] `getGameAnalyticsExport` là operationId mới (API_DESIGN chỉ có `createGameAnalyticsExport`).
- [Confirmed] UI Game Analytics đặt `src/cms/frontend/apps/portal/src/workspaces/live-ops/{dashboard,analytics,events}/` (F051 slug `live-ops`).
- [Confirmed] Màn "Hoạt động game" route `/school-lms/reports/game-activity` trong chunk `school-lms` (F052), menu "Tiến độ & báo cáo → Hoạt động game"; dùng `DashboardRangePicker`, `DashboardWidget`, `SuppressedCell`, `useDashboardQueue` của F056 trong `ops-ui`.
- ADR-0058 ở trạng thái Proposed nhưng được liệt kê "đang dùng làm mặc định" trong chỉ mục ADR.
