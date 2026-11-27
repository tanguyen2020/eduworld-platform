# Tasks: F131 — Monitoring, alerting, dashboard vận hành, runbook

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Target Makefile `obs-check`, `obs-rules`, `obs-runbooks`, `obs-metric-catalog`, `alert-drill` — file: `Makefile` — Makefile `obs-check`, `obs-rules`, `obs-runbooks`, `obs-dashboards`, `obs-label-policy`, `obs-alertmanager`, `obs-redaction`, `obs-metric-catalog`, `alert-drill`, `test-obs`
- [x] T002 [P] Parser bảng MONITORING §3/§4/§5 (metric, alert, dashboard) dùng chung cho script kiểm — file: `scripts/obs/lib-monitoring-md.mjs` — `scripts/obs/lib-monitoring-md.mjs` (bảng §3 chính + bổ sung, §4, §5; `|` trong backtick không tách ô)
- [x] T003 [P] Khung runbook (mục bắt buộc) và mẫu báo cáo diễn tập — file: `docs/11-operations/runbooks/README.md` — `docs/11-operations/runbooks/README.md` + `docs/11-operations/alert-drills/README.md`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Không có delta contract `specs/` (xem contracts §6); xác nhận `make specs-lint` không đổi — file: `specs/131-observability-alerting-runbooks/contracts/README.md` — không delta `specs/openapi`; `contracts/README.md` thêm `endGameSession` (có sẵn) cho dọn phiên probe
- [x] T005 Phối hợp F004: client Keycloak `eduworld-ops-tools` (confidential, không MFA — ADR-0094, mapper role platform) cho Grafana/GlitchTip — file: `deploy/keycloak/realm-eduworld.json` — client `eduworld-ops-tools` (confidential, chỉ Authorization Code + PKCE S256, redirect URI từ env, client role `admin`/`editor`; không MFA — ADR-0094) + luật `check-realm.mjs` (10/10 test) + KEYCLOAK_SSO §3; Grafana `role_attribute_strict` chỉ `platform_staff`
- [x] T006 Secret kênh trực `ops_alert_chat_webhook`, `ops_alert_sms`, `ops_alert_email` trong SOPS (F130) — file: `deploy/secrets/production/ops.sops.yaml` — secret `ops_alert_*`, `ops_grafana_*`, `ops_probe_password`… trong policy + `deploy/secrets/secrets.template.yaml` (layout SOPS `ops.sops.yaml`); giá trị thật do vận hành tạo

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Thu thập metric, log, trace xuyên binary và module trên Swarm (Priority: P1) 🎯 MVP

**Goal**: Stack quan sát chạy trên node `ops`, dữ liệu đầy đủ, không PII.

**Independent Test**: Như spec.md User Story 1.

### Tests cho User Story 1

- [x] T007 [P] [US1] Test F131-AC-02: `check-metric-catalog` với `/metrics` fixture thiếu metric/thiếu label `service` hoặc `module` → exit ≠ 0, liệt kê đúng — file: `scripts/obs/check-metric-catalog.test.mjs` — phải FAIL trước khi code — `scripts/obs/check-metric-catalog.test.mjs` 7/7 (thiếu metric, thiếu nhãn `service`/`module`, sai kiểu, wildcard, quy tắc hậu tố)
- [x] T008 [P] [US1] Test F131-AC-06: `check-label-policy` phát hiện label cấm trong rule/dashboard/fixture metric — file: `scripts/obs/check-label-policy.test.mjs` — `scripts/obs/check-label-policy.test.mjs` 6/6
- [x] T009 [P] [US1] Go test F131-AC-01: `swarm-exporter` với Docker API giả xuất đúng 4 metric — file: `src/backend/tools/swarm-exporter/internal/collector/collector_test.go` — `src/backend/tools/swarm-exporter/internal/collector/collector_test.go` 7 test + 2 cmd test (Docker API giả, MOCK)
- [x] T010 [P] [US1] Test F131-AC-05 (TC-AUTH-060): chạy Collector với cấu hình production trên log/span fixture chứa JWT/email/SĐT/mật khẩu mẫu → đầu ra đã che — file: `deploy/observability/otel-collector/tests/redaction_test.sh` — `deploy/observability/otel-collector/tests/redaction_test.sh` 10/10 trên otelcol-contrib 0.120.0 dùng một lần (JWT, bearer, email, SĐT, mật khẩu bị che; `trace_id` giữ)
- [ ] T011 [P] [US1] Kịch bản staging F131-AC-01, F131-AC-03, F131-AC-04: target `up`, trace xuyên `core-api` → facade → NATS → `worker` + log cùng `trace_id`, tỷ lệ sampling — file: `tests/obs/staging/trace_e2e.sh` — `tests/obs/staging/trace_e2e.sh` viết xong — **chờ môi trường staging**
- [ ] T012 [P] [US1] Kịch bản staging F131-AC-07: `ad.truong.a` (nhân sự trường) bị Grafana từ chối, nhân sự vận hành platform vào được; truy vấn Loki có audit — file: `tests/obs/staging/access_control.sh` — `tests/obs/staging/access_control.sh` viết xong — **chờ môi trường staging**

### Implementation cho User Story 1

- [x] T013 [US1] Collector staging/production: receiver OTLP + filelog, processor `redaction`, `tail_sampling` (lỗi, > 1 s), `probabilistic_sampler` 10%, exporter Tempo/Loki/Prometheus — file: `deploy/observability/otel-collector/collector.production.yaml` — `collector.swarm.yaml` (redaction + transform, tail sampling lỗi/> 1 s/10%) + `agent.swarm.yaml` (global, log json-file); `otelcol validate` OK
- [x] T014 [US1] Prometheus `dockerswarm_sd_configs` (role tasks, label `prometheus.scrape`), `sample_limit`, scrape exporter — file: `deploy/observability/prometheus/prometheus.production.yml` — `prometheus.swarm.yml` (dockerswarm_sd qua `docker-api-proxy-ro`, nhãn `service`/`profile` từ tên Swarm service, `sample_limit`, đổi tên `jetstream_*` → `nats_*`); `promtool check config` OK; HAProxy RO `haproxy -c` OK
- [x] T015 [P] [US1] Loki (`app_log_retention`, tenant `audit-ops`), Tempo (`metrics_generator` service graph) — file: `deploy/observability/loki/loki.yaml` — `loki/loki.swarm.yaml` (retention 336 h = `app_log_retention`, log mọi truy vấn; `-verify-config` OK), `tempo/tempo.swarm.yaml` (metrics generator service graph; khởi động thử OK)
- [x] T016 [P] [US1] `swarm-exporter` (socket proxy chỉ `GET`) + Dockerfile non-root — file: `src/backend/tools/swarm-exporter/cmd/swarm-exporter/main.go` — `src/backend/tools/swarm-exporter` + `deploy/docker/swarm-exporter.Dockerfile` (distroless non-root)
- [x] T017 [US1] Thêm service observability, exporter global, Grafana (OAuth Keycloak, `role_attribute_strict`), GlitchTip vào stack; label `prometheus.scrape=true` cho `core-api`, `realtime`, `worker-*` — file: `deploy/swarm/production/infra.stack.yml` — `deploy/swarm/{staging,production}/obs.stack.yml` + `obs.env`, `glitchtip.env`, `blackbox-*.yml`; label `prometheus.*` trên `core-api`, `realtime`, `worker-*`, APISIX, Keycloak, NATS exporter, `ops-scheduler`, SeaweedFS; policy (stack, network, secret, `global_services`, `docker_socket`); `make swarm-validate` OK
- [x] T018 [US1] `check-metric-catalog.mjs`, `check-label-policy.mjs` + job `obs-label-policy` (workflow `obs.yml`) và `obs-metric-catalog` (sau deploy staging trong `release.yml`) — file: `.github/workflows/obs.yml` — `check-metric-catalog.mjs`, `check-label-policy.mjs`; job `obs-label-policy` (`obs.yml`) và `obs-metric-catalog` (sau `deploy-staging`, `release.yml`) — follow-up 2026-11-27: `--static=src/backend --strict` 0 metric bắt buộc thiếu (trước đó 19)

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Alert có runbook, đến đúng người trực (Priority: P1)

**Goal**: Rule đủ bảng MONITORING §5, Alertmanager định tuyến, runbook, diễn tập.

**Independent Test**: Như spec.md User Story 2.

### Tests cho User Story 2

- [x] T019 [P] [US2] Unit test rule F131-AC-08, F131-AC-12: mỗi alert một ca bắn + một ca không bắn (`promtool test rules`) — file: `deploy/observability/prometheus/tests/alerts.test.yaml` — phải FAIL trước khi code — `deploy/observability/prometheus/tests/*.test.yaml` (19 file) — `promtool test rules` 19/19 SUCCESS, mỗi alert một ca bắn + một ca không bắn (trừ `Watchdog`)
- [x] T020 [P] [US2] Test F131-AC-08, F131-AC-13: `check-alerts` báo alert thiếu so với MONITORING §5, thiếu `runbook_url`/`source`, runbook thiếu mục — file: `scripts/obs/check-alerts.test.mjs` — `scripts/obs/check-alerts.test.mjs` 7/7
- [x] T021 [P] [US2] Test F131-AC-09, F131-AC-10: `amtool config routes test` (critical → `oncall-critical`, warning → `ops-warning`, inhibit theo `service`/`module`, `Watchdog` → heartbeat) — file: `deploy/observability/alertmanager/tests/routes_test.sh` — `deploy/observability/alertmanager/tests/routes_test.sh --live` 14/14 (amtool check-config, 7 route, 6 inhibit trên Alertmanager v0.28.1 dùng một lần)
- [x] T022 [P] [US2] Kịch bản F131-AC-11: `alert-drill.sh` kích hoạt từng alert critical trên staging, đo thời gian tới kênh trực — file: `scripts/obs/alert-drill.sh` — `scripts/obs/alert-drill.sh` + `alert-drill.test.mjs` 2/2 (curl giả, MOCK); diễn tập thật **chờ môi trường staging**

### Implementation cho User Story 2

- [x] T023 [US2] Rule theo nhóm (api, realtime, identity, learning, content, media, storage, swarm, async, competition, chat, dashboard, backup, client) từ MONITORING §5, annotation `runbook_url`, `source` — file: `deploy/observability/prometheus/rules/api.yaml` — `deploy/observability/prometheus/rules/*.yaml` (19 nhóm, 102 alert/107 rule, chuyển từ `prometheus/alerts/`); `promtool check rules` OK
- [x] T024 [US2] Rule meta (`Watchdog`, `PrometheusTargetDown`, `PrometheusTargetSampleLimit`, `PrometheusRuleFailures`, `AlertmanagerNotificationsFailing`) — file: `deploy/observability/prometheus/rules/meta.yaml` — `rules/meta.yaml` (`Watchdog`, `PrometheusTargetDown`, `PrometheusTargetSampleLimit`, `PrometheusRuleFailures`, `AlertmanagerNotificationsFailing`, recording `eduworld:school_hours`)
- [x] T025 [US2] Alertmanager: route, group_by, inhibit `ServiceDown`, receiver `*_file` từ secret — file: `deploy/observability/alertmanager/alertmanager.yml` — `alertmanager/alertmanager.yml` (critical → `oncall-critical`, warning → `ops-warning`, `team=safety` → `safety-oncall`, `Watchdog` → heartbeat; inhibit; `url_file` từ secret) + `alertmanager-entrypoint.sh` + template
- [x] T026 [US2] Runbook cho mọi alert MONITORING §5 + alert meta (link RB-xx khi liên quan) — file: `docs/11-operations/runbooks/ServiceDown.md` — 102 runbook `docs/11-operations/runbooks/<AlertName>.md` (khung 6 mục)
- [x] T027 [US2] `check-alerts.mjs` + job `obs-rules`, `obs-runbooks` (workflow `obs.yml`) — file: `scripts/obs/check-alerts.mjs` — `scripts/obs/check-alerts.mjs` (0 vi phạm: 102 alert §5 ↔ 107 rule ↔ runbook ↔ test) + job `obs-rules`, `obs-runbooks`

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — Dashboard vận hành và SLO as code (Priority: P1)

**Goal**: 13 dashboard provision từ Git, SLO + burn rate.

**Independent Test**: Như spec.md User Story 3.

### Tests cho User Story 3

- [x] T028 [P] [US3] Test F131-AC-14, F131-AC-16: `check-dashboards` phát hiện datasource UID sai, metric không có ở MONITORING §3, panel Chat Safety truy vấn label nội dung — file: `scripts/obs/check-dashboards.test.mjs` — phải FAIL trước khi code — `scripts/obs/check-dashboards.test.mjs` 7/7
- [x] T029 [P] [US3] Unit test F131-AC-15: recording rule SLI và `SLOBurnRateFast`/`Slow` (loại 429, chỉ giờ học) — file: `deploy/observability/prometheus/tests/slo.test.yaml` — `tests/slo.test.yaml` (SLI loại 429, burn rate nhanh/chậm, im lặng ngoài giờ học)

### Implementation cho User Story 3

- [x] T030 [US3] Recording rule SLI cho các SLO MONITORING §6 + alert burn rate multiwindow (`availability`) — file: `deploy/observability/prometheus/rules/slo.yaml` — `rules/slo.yaml` (SLI recording rule cho §6, `eduworld:slo_availability_target` = `availability`)
- [x] T031 [US3] 13 dashboard JSON theo MONITORING §4 (dashboard binary/module dùng biến `service`, `module`; Service Map từ Tempo) + provisioning — file: `deploy/observability/grafana/dashboards/eduworld-overview.json` — 14 dashboard (MONITORING §4 có 14 dòng) sinh bởi `scripts/obs/build-dashboards.mjs` + provisioning `dashboards.yaml`, datasource Alertmanager
- [x] T032 [US3] `check-dashboards.mjs` + job `obs-dashboards` — file: `scripts/obs/check-dashboards.mjs` — `scripts/obs/check-dashboards.mjs` + job `obs-dashboards` (`build-dashboards --check`)

**Checkpoint**: User Story 3 chạy và kiểm độc lập được.

## Phase 6: User Story 4 — Error tracking và synthetic monitoring (Priority: P2)

**Goal**: GlitchTip + ticket; probe định kỳ tenant QA.

**Independent Test**: Như spec.md User Story 4.

### Tests cho User Story 4

- [x] T033 [P] [US4] Go test F131-AC-17: `BeforeSend` của `src/backend/pkg/otel` xóa body/header nhạy cảm, gắn `trace_id`/`service`/`module`/`version` — file: `src/backend/pkg/otel/errortracking_test.go` — phải FAIL trước khi code — `src/backend/pkg/observability/errortracking_test.go` (gói F002 thực tế là `pkg/observability`, không phải `pkg/otel`) + `pkg/httpx/errortracking_test.go`, `pkg/bootstrap/errortracking_test.go`
- [x] T034 [P] [US4] Test F131-AC-18: webhook GlitchTip → GitHub Issue khử trùng theo `issue_id` (mock HTTP có nhãn MOCK) — file: `src/backend/tools/glitchtip-bridge/bridge_test.go` — `src/backend/tools/glitchtip-bridge/internal/bridge/bridge_test.go` 10 test + 2 cmd test (GlitchTip/GitHub giả, MOCK)
- [x] T035 [P] [US4] Test F131-AC-19, F131-AC-20 (TC-AUTH-044): k6 script với server giả trả 5xx/429 + `Retry-After` → metric `result` đúng, không retry dồn, không gọi endpoint attempt — file: `tests/synthetic/k6/student_login.test.js` — `tests/synthetic/k6/student_login.test.js` 7/7 (logic `lib/probe.js`: 429 → `rate_limited` + `Retry-After`, không retry, không endpoint học tập)

### Implementation cho User Story 4

- [x] T036 [US4] Tích hợp `sentry-go` trong `src/backend/pkg/otel` (`SendDefaultPII=false`, `BeforeSend`), DSN qua `EW_ERROR_TRACKING_DSN_FILE` — file: `src/backend/pkg/otel/errortracking.go` — `src/backend/pkg/observability/errortracking.go` (`sentry-go` v0.50, thu thập dữ liệu người dùng tắt hết, `BeforeSend` xóa request/user/breadcrumb data), `EW_ERROR_TRACKING_DSN_FILE`; nối vào recover của httpx, realtime, modcall, outbox consumer, saga; metric `eduworld_panics_recovered_total{module,entrypoint}`
- [x] T037 [P] [US4] `glitchtip-bridge` (webhook → GitHub Issues, label `module:<name>`/`service:<binary>`) — file: `src/backend/tools/glitchtip-bridge/main.go` — `src/backend/tools/glitchtip-bridge` + `deploy/docker/glitchtip-bridge.Dockerfile`
- [x] T038 [US4] Script k6 `client_config.js`, `student_login.js` (login → session → logout), service `synthetic-probe` + rule `SyntheticProbeFailing` — file: `tests/synthetic/k6/student_login.js` — `tests/synthetic/k6/{client_config,student_login}.js` + `deploy/docker/synthetic-probe.Dockerfile` + `probe.crontab`; kết quả là textfile node_exporter (thay remote write — research R4 cập nhật) + rule `SyntheticProbeFailing` (2 chu kỳ hoặc probe ngừng > 15 phút)
- [x] T039 [US4] Seed tenant QA + `hs.qa.probe` cho staging/production qua CSV import (F011), loại trừ khỏi `student-inactive-cleanup` — file: `deploy/seed/qa-tenant/README.md` — `deploy/seed/qa-tenant/README.md` (tạo tenant QA, loại trừ dọn học sinh); tạo thật **chờ môi trường staging** — `createTenant` chưa nhận `is_qa` [Proposed]

**Checkpoint**: User Story 4 chạy và kiểm độc lập được.

## Phase N: Polish & Cross-Cutting

- [x] T040 [P] MONITORING: thêm Alertmanager vào §1, metric `eduworld_synthetic_probe_*` vào §3, alert meta + `SyntheticProbeFailing` vào §5, vị trí runbook `docs/11-operations/runbooks/`; thống nhất `eduworld_ratelimit_rejections_total` vs `eduworld_ratelimit_rejected_total` (SECURITY_ARCHITECTURE §14) và `CHAT_DLQ` vs `DLQ_<SERVICE>` (DEPLOYMENT §8b) — file: `docs/11-operations/MONITORING.md` — MONITORING §1 (Alertmanager, stack), §2 (nhãn khi scrape, nhãn cấm), §3 (synthetic, swarm, `facade`, purge, client sessions), §4, §5 (alert meta + alert module đã có, `MatchmakingWaitHigh` 60 s, `OutboxLagHigh` theo tên mã), §6 (SLI), §7, §8, §9
- [x] T041 [P] LOCAL_DEVELOPMENT §5 ghi tenant QA (chỉ staging/production) — file: `docs/11-operations/LOCAL_DEVELOPMENT.md` — LOCAL_DEVELOPMENT §3.3, §4 (biến F131), §7 (target obs-*)
- [x] T042 [P] CHANGELOG mục F131 — file: `CHANGELOG.md` — CHANGELOG mục F131
- [ ] T043 Chạy [quickstart.md](quickstart.md) trên staging và ghi bằng chứng — file: `specs/131-observability-alerting-runbooks/quickstart.md` — kịch bản quickstart.md trên staging — **chờ môi trường staging**

### Follow-up 2026-11-27 — lỗ hổng do chaos/anti-cheat F132 phát hiện (obsfix)

- [x] T044 [P] Phát `eduworld_external_client_circuit_state{module,integration}` từ breaker của adapter ngoài — file: `src/backend/pkg/observability/external.go` — `ObserveCircuit` gắn vào 7 breaker (S3 ×4 module, Keycloak Admin, Keycloak token, SIS lookup), gauge = trạng thái xấu nhất mỗi cặp; test `pkg/observability/external_test.go` (3 test)
- [x] T045 [P] Phát `eduworld_nats_request_duration_seconds`/`eduworld_nats_request_errors_total` — file: `src/backend/pkg/natsx/request.go` — helper `natsx.Request` dùng trong `modules/realtime/internal/router`, lý do `timeout`/`no_responders`/`unavailable`/`error`, series 0 từ khởi động; test `pkg/natsx/request_test.go` (3 test, server NATS giả MOCK trong test), `router_test.go`
- [x] T046 Readiness `nats` thăm dò server (PING/PONG, `nats_readiness_probe_timeout` 1 s) + gauge `eduworld_nats_connected` — file: `src/backend/pkg/health/checks.go` — test `pkg/health/nats_probe_test.go` (4 test: server bị pause vẫn CONNECTED → `fail/timeout` trong ≤ 1 s)
- [x] T047 Alert `NatsClusterDown`, `PatroniSwitchover` (MONITORING §5) + runbook + ca `promtool` bắn/không bắn; deploy label job `patroni` cho `pg-N` (staging, production) — file: `deploy/observability/prometheus/rules/{async,datastore}.yaml`
- [x] T048 Anti-cheat ANTI_CHEAT §10: `eduworld_answers_too_fast_ratio{mode}` (learning, khi học sinh nộp attempt) và `eduworld_matches_voided_total{mode}` (competition, series 0; use case vô hiệu trận thuộc feature xử lý vi phạm sau); sửa Spectral `array-items` của `MatchingResponse.pairs` — file: `src/backend/modules/learning/internal/infrastructure/observability/attempt_metrics.go` — `TestAnswersTooFastRatio`, `TestMatchesVoidedMetric`, integration `TestSelfStudyPracticeHintsRapidGuess` (1 rapid / 3 câu tính điểm → +1 quan sát, tổng 1/3); Spectral 0 lỗi, oasdiff 0 breaking

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
