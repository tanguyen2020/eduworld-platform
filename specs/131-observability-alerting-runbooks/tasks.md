# Tasks: F131 — Monitoring, alerting, dashboard vận hành, runbook

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Target Makefile `obs-check`, `obs-rules`, `obs-runbooks`, `obs-metric-catalog`, `alert-drill` — file: `Makefile`
- [ ] T002 [P] Parser bảng MONITORING §3/§4/§5 (metric, alert, dashboard) dùng chung cho script kiểm — file: `scripts/obs/lib-monitoring-md.mjs`
- [ ] T003 [P] Khung runbook (mục bắt buộc) và mẫu báo cáo diễn tập — file: `docs/11-operations/runbooks/README.md`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Không có delta contract `specs/` (xem contracts §6); xác nhận `make specs-lint` không đổi — file: `specs/131-observability-alerting-runbooks/contracts/README.md`
- [ ] T005 Phối hợp F004: client Keycloak `eduworld-ops-tools` (confidential, MFA `acr`, mapper role platform) cho Grafana/GlitchTip — file: `deploy/keycloak/realm-eduworld.json`
- [ ] T006 Secret kênh trực `ops_alert_chat_webhook`, `ops_alert_sms`, `ops_alert_email` trong SOPS (F130) — file: `deploy/secrets/production/ops.sops.yaml`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Thu thập metric, log, trace xuyên binary và module trên Swarm (Priority: P1) 🎯 MVP

**Goal**: Stack quan sát chạy trên node `ops`, dữ liệu đầy đủ, không PII.

**Independent Test**: Như spec.md User Story 1.

### Tests cho User Story 1

- [ ] T007 [P] [US1] Test F131-AC-02: `check-metric-catalog` với `/metrics` fixture thiếu metric/thiếu label `service` hoặc `module` → exit ≠ 0, liệt kê đúng — file: `scripts/obs/check-metric-catalog.test.mjs` — phải FAIL trước khi code
- [ ] T008 [P] [US1] Test F131-AC-06: `check-label-policy` phát hiện label cấm trong rule/dashboard/fixture metric — file: `scripts/obs/check-label-policy.test.mjs`
- [ ] T009 [P] [US1] Go test F131-AC-01: `swarm-exporter` với Docker API giả xuất đúng 4 metric — file: `tools/swarm-exporter/internal/collector/collector_test.go`
- [ ] T010 [P] [US1] Test F131-AC-05 (TC-AUTH-060): chạy Collector với cấu hình production trên log/span fixture chứa JWT/email/SĐT/mật khẩu mẫu → đầu ra đã che — file: `deploy/observability/otel-collector/tests/redaction_test.sh`
- [ ] T011 [P] [US1] Kịch bản staging F131-AC-01, F131-AC-03, F131-AC-04: target `up`, trace xuyên `core-api` → facade → NATS → `worker` + log cùng `trace_id`, tỷ lệ sampling — file: `tests/obs/staging/trace_e2e.sh`
- [ ] T012 [P] [US1] Kịch bản staging F131-AC-07: `ad.truong.a` và nhân sự không MFA bị Grafana từ chối; truy vấn Loki có audit — file: `tests/obs/staging/access_control.sh`

### Implementation cho User Story 1

- [ ] T013 [US1] Collector staging/production: receiver OTLP + filelog, processor `redaction`, `tail_sampling` (lỗi, > 1 s), `probabilistic_sampler` 10%, exporter Tempo/Loki/Prometheus — file: `deploy/observability/otel-collector/collector.production.yaml`
- [ ] T014 [US1] Prometheus `dockerswarm_sd_configs` (role tasks, label `prometheus.scrape`), `sample_limit`, scrape exporter — file: `deploy/observability/prometheus/prometheus.production.yml`
- [ ] T015 [P] [US1] Loki (`app_log_retention`, tenant `audit-ops`), Tempo (`metrics_generator` service graph) — file: `deploy/observability/loki/loki.yaml`
- [ ] T016 [P] [US1] `swarm-exporter` (socket proxy chỉ `GET`) + Dockerfile non-root — file: `tools/swarm-exporter/cmd/swarm-exporter/main.go`
- [ ] T017 [US1] Thêm service observability, exporter global, Grafana (OAuth Keycloak, `role_attribute_strict`), GlitchTip vào stack; label `prometheus.scrape=true` cho `core-api`, `realtime`, `worker-*` — file: `deploy/swarm/production/infra.stack.yml`
- [ ] T018 [US1] `check-metric-catalog.mjs`, `check-label-policy.mjs` + job `obs-label-policy` (workflow `obs.yml`) và `obs-metric-catalog` (sau deploy staging trong `release.yml`) — file: `.github/workflows/obs.yml`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Alert có runbook, đến đúng người trực (Priority: P1)

**Goal**: Rule đủ bảng MONITORING §5, Alertmanager định tuyến, runbook, diễn tập.

**Independent Test**: Như spec.md User Story 2.

### Tests cho User Story 2

- [ ] T019 [P] [US2] Unit test rule F131-AC-08, F131-AC-12: mỗi alert một ca bắn + một ca không bắn (`promtool test rules`) — file: `deploy/observability/prometheus/tests/alerts.test.yaml` — phải FAIL trước khi code
- [ ] T020 [P] [US2] Test F131-AC-08, F131-AC-13: `check-alerts` báo alert thiếu so với MONITORING §5, thiếu `runbook_url`/`source`, runbook thiếu mục — file: `scripts/obs/check-alerts.test.mjs`
- [ ] T021 [P] [US2] Test F131-AC-09, F131-AC-10: `amtool config routes test` (critical → `oncall-critical`, warning → `ops-warning`, inhibit theo `service`/`module`, `Watchdog` → heartbeat) — file: `deploy/observability/alertmanager/tests/routes_test.sh`
- [ ] T022 [P] [US2] Kịch bản F131-AC-11: `alert-drill.sh` kích hoạt từng alert critical trên staging, đo thời gian tới kênh trực — file: `scripts/obs/alert-drill.sh`

### Implementation cho User Story 2

- [ ] T023 [US2] Rule theo nhóm (api, realtime, identity, learning, content, media, storage, swarm, async, competition, chat, dashboard, backup, client) từ MONITORING §5, annotation `runbook_url`, `source` — file: `deploy/observability/prometheus/rules/api.yaml`
- [ ] T024 [US2] Rule meta (`Watchdog`, `PrometheusTargetDown`, `PrometheusTargetSampleLimit`, `PrometheusRuleFailures`, `AlertmanagerNotificationsFailing`) — file: `deploy/observability/prometheus/rules/meta.yaml`
- [ ] T025 [US2] Alertmanager: route, group_by, inhibit `ServiceDown`, receiver `*_file` từ secret — file: `deploy/observability/alertmanager/alertmanager.yml`
- [ ] T026 [US2] Runbook cho mọi alert MONITORING §5 + alert meta (link RB-xx khi liên quan) — file: `docs/11-operations/runbooks/ServiceDown.md`
- [ ] T027 [US2] `check-alerts.mjs` + job `obs-rules`, `obs-runbooks` (workflow `obs.yml`) — file: `scripts/obs/check-alerts.mjs`

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — Dashboard vận hành và SLO as code (Priority: P1)

**Goal**: 13 dashboard provision từ Git, SLO + burn rate.

**Independent Test**: Như spec.md User Story 3.

### Tests cho User Story 3

- [ ] T028 [P] [US3] Test F131-AC-14, F131-AC-16: `check-dashboards` phát hiện datasource UID sai, metric không có ở MONITORING §3, panel Chat Safety truy vấn label nội dung — file: `scripts/obs/check-dashboards.test.mjs` — phải FAIL trước khi code
- [ ] T029 [P] [US3] Unit test F131-AC-15: recording rule SLI và `SLOBurnRateFast`/`Slow` (loại 429, chỉ giờ học) — file: `deploy/observability/prometheus/tests/slo.test.yaml`

### Implementation cho User Story 3

- [ ] T030 [US3] Recording rule SLI cho các SLO MONITORING §6 + alert burn rate multiwindow (`availability`) — file: `deploy/observability/prometheus/rules/slo.yaml`
- [ ] T031 [US3] 13 dashboard JSON theo MONITORING §4 (dashboard binary/module dùng biến `service`, `module`; Service Map từ Tempo) + provisioning — file: `deploy/observability/grafana/dashboards/eduworld-overview.json`
- [ ] T032 [US3] `check-dashboards.mjs` + job `obs-dashboards` — file: `scripts/obs/check-dashboards.mjs`

**Checkpoint**: User Story 3 chạy và kiểm độc lập được.

## Phase 6: User Story 4 — Error tracking và synthetic monitoring (Priority: P2)

**Goal**: GlitchTip + ticket; probe định kỳ tenant QA.

**Independent Test**: Như spec.md User Story 4.

### Tests cho User Story 4

- [ ] T033 [P] [US4] Go test F131-AC-17: `BeforeSend` của `pkg/otel` xóa body/header nhạy cảm, gắn `trace_id`/`service`/`module`/`version` — file: `pkg/otel/errortracking_test.go` — phải FAIL trước khi code
- [ ] T034 [P] [US4] Test F131-AC-18: webhook GlitchTip → GitHub Issue khử trùng theo `issue_id` (mock HTTP có nhãn MOCK) — file: `tools/glitchtip-bridge/bridge_test.go`
- [ ] T035 [P] [US4] Test F131-AC-19, F131-AC-20 (TC-AUTH-044): k6 script với server giả trả 5xx/429 + `Retry-After` → metric `result` đúng, không retry dồn, không gọi endpoint attempt — file: `tests/synthetic/k6/student_login.test.js`

### Implementation cho User Story 4

- [ ] T036 [US4] Tích hợp `sentry-go` trong `pkg/otel` (`SendDefaultPII=false`, `BeforeSend`), DSN qua `EW_ERROR_TRACKING_DSN_FILE` — file: `pkg/otel/errortracking.go`
- [ ] T037 [P] [US4] `glitchtip-bridge` (webhook → GitHub Issues, label `module:<name>`/`service:<binary>`) — file: `tools/glitchtip-bridge/main.go`
- [ ] T038 [US4] Script k6 `client_config.js`, `student_login.js` (login → session → logout), service `synthetic-probe` + rule `SyntheticProbeFailing` — file: `tests/synthetic/k6/student_login.js`
- [ ] T039 [US4] Seed tenant QA + `hs.qa.probe` cho staging/production qua CSV import (F011), loại trừ khỏi `student-inactive-cleanup` — file: `deploy/seed/qa-tenant/README.md`

**Checkpoint**: User Story 4 chạy và kiểm độc lập được.

## Phase N: Polish & Cross-Cutting

- [ ] T040 [P] MONITORING: thêm Alertmanager vào §1, metric `eduworld_synthetic_probe_*` vào §3, alert meta + `SyntheticProbeFailing` vào §5, vị trí runbook `docs/11-operations/runbooks/`; thống nhất `eduworld_ratelimit_rejections_total` vs `eduworld_ratelimit_rejected_total` (SECURITY_ARCHITECTURE §14) và `CHAT_DLQ` vs `DLQ_<SERVICE>` (DEPLOYMENT §8b) — file: `docs/11-operations/MONITORING.md`
- [ ] T041 [P] LOCAL_DEVELOPMENT §5 ghi tenant QA (chỉ staging/production) — file: `docs/11-operations/LOCAL_DEVELOPMENT.md`
- [ ] T042 [P] CHANGELOG mục F131 — file: `CHANGELOG.md`
- [ ] T043 Chạy [quickstart.md](quickstart.md) trên staging và ghi bằng chứng — file: `specs/131-observability-alerting-runbooks/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 (cần Prometheus/Alertmanager của US1) → US3 → US4 → Polish.
- T002 trước T018, T027, T032; T013–T016 trước T017.
- F132 dùng dashboard/alert của US1–US3 để đo load/chaos.

## Definition of Done

- [ ] Mọi tiêu chí F131-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
