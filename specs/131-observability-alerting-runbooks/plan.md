# Implementation Plan: F131 — Monitoring, alerting, dashboard vận hành, runbook

**Branch**: `131-observability-alerting-runbooks` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Thêm stack quan sát vào `deploy/swarm/<env>/infra.stack.yml` (node `ops`): OTel Collector (redaction, head/tail sampling), Prometheus (`dockerswarm_sd_configs`, rule), Alertmanager (định tuyến, inhibit, Watchdog), Loki, Tempo (metrics generator service graph), Grafana (provision dashboard JSON, đăng nhập Keycloak, không MFA — ADR-0094 mục 2b), exporter (node, cAdvisor global; postgres, redis, nats; `swarm-exporter` Go chỉ đọc), GlitchTip. Ba script kiểm CI đọc thẳng bảng trong MONITORING (nguồn duy nhất): catalog metric, rule ↔ bảng alert ↔ runbook, label policy. Runbook mỗi alert trong `docs/11-operations/runbooks/`. Probe synthetic k6 chạy trong service `synthetic-probe` (k6 + supercronic, remote write về Prometheus) trên node `ops`.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | YAML (Prometheus/Alertmanager/Collector/Loki/Tempo), JSON (Grafana), Node 22 (`scripts/obs/*.mjs`), Go ≥ 1.24 (`src/backend/tools/swarm-exporter`), k6 (JS) |
| Primary Dependencies | otel-collector-contrib (`redaction`, `tail_sampling`), Prometheus 2.5x + `promtool`, Alertmanager + `amtool`, Loki 3, Tempo 2 (`metrics_generator`), Grafana 11 (generic OAuth Keycloak), GlitchTip, k6 |
| Storage | TSDB Prometheus, chunk Loki/Tempo trên volume node `ops` (không dữ liệu nghiệp vụ); GlitchTip dùng PostgreSQL riêng `glitchtip` trên node `ops` |
| Testing | `promtool test rules`, `amtool check-config`, `node --test scripts/obs/*.test.mjs`, `go -C src/backend test ./tools/swarm-exporter/...`, kịch bản staging |
| Target Platform | Docker Swarm node `ops` (2 node, DEPLOYMENT §4.2) |
| Project Type | Hạ tầng quan sát |
| Performance Goals | Alert critical tới kênh trực ≤ 2 phút; scrape interval 15 s |
| Constraints | ADR-0023, ADR-0024, AGENTS R14, R20, R33 |
| Scale/Scope | 5 Swarm service backend (`core-api`, `realtime`, `worker-events`, `worker-chat`, `worker-jobs`) × replica đỉnh DEPLOYMENT §4.2, ~35 node |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Không có contract API mới; không chạm deferred |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Không sửa code module; `swarm-exporter` là tool trong `src/backend/tools/`, không import `src/backend/modules/` |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Không có label `school_id`/user; không dữ liệu tenant trong Prometheus; probe dùng tenant QA |
| IV Server-authoritative | ✅ | Không áp dụng |
| V Xác thực, phân quyền | ✅ | Công cụ chỉ qua network `admin` + đăng nhập Keycloak có role vận hành platform (không MFA — ADR-0094 mục 2b); credential kênh trực qua Docker secret |
| VI An toàn trẻ em | ✅ | Redaction ở Collector, label policy, dashboard Chat Safety không nội dung |
| VII Một nguồn sự thật, cấu hình | ✅ | Script đọc bảng MONITORING §3/§5; ngưỡng theo `alert_thresholds` và MONITORING; alert mới bổ sung vào MONITORING (task) |
| VIII UI tách logic, token | ✅ | Không áp dụng (Grafana là công cụ vận hành, không phải Admin Portal) |
| IX Test có bằng chứng | ✅ | `promtool test rules` từng alert, test script kiểm, diễn tập alert có báo cáo |

## Project Structure

### Documentation (feature này)

```text
specs/131-observability-alerting-runbooks/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
deploy/observability/
├── otel-collector/collector.{staging,production}.yaml   # redaction, tail_sampling, exporters
├── prometheus/prometheus.{staging,production}.yml       # dockerswarm_sd, sample_limit, rule_files
├── prometheus/rules/{slo,api,realtime,identity,learning,content,media,storage,swarm,async,competition,chat,dashboard,backup,client,synthetic,meta}.yaml
├── prometheus/tests/*.test.yaml                         # promtool test rules
├── alertmanager/alertmanager.yml                        # route, inhibit, receivers (*_file từ secret)
├── loki/loki.yaml   tempo/tempo.yaml                    # retention, metrics_generator
├── grafana/provisioning/{datasources,dashboards}/*.yaml  grafana/dashboards/*.json
└── glitchtip/glitchtip.env (không secret)
deploy/swarm/{staging,production}/infra.stack.yml        # thêm service observability (F130 tạo file)
src/backend/tools/swarm-exporter/{cmd/swarm-exporter/main.go,internal/collector/}   # thuộc Go module src/backend (go.mod duy nhất, ADR-0074, ADR-0092)
src/backend/tools/glitchtip-bridge/main.go                           # webhook GlitchTip → GitHub Issues
src/backend/pkg/otel/errortracking.go                                # sentry-go, BeforeSend (bổ sung vào gói của F002)
deploy/seed/qa-tenant/                                   # CSV tenant QA cho staging/production
tests/synthetic/k6/{client_config.js,student_login.js}
scripts/obs/{lib-monitoring-md.mjs,check-metric-catalog.mjs,check-alerts.mjs,check-label-policy.mjs,check-dashboards.mjs,alert-drill.sh}
docs/11-operations/runbooks/{README.md,<AlertName>.md}
docs/11-operations/alert-drills/README.md
.github/workflows/obs.yml                                # obs-rules, obs-runbooks, obs-label-policy, obs-dashboards (path filter deploy/observability/**, docs/11-operations/runbooks/**, scripts/obs/**); obs-metric-catalog chạy sau deploy staging trong release.yml
```

**Structure Decision**: Cấu hình theo `deploy/observability/` ([MONITORING §4–§5](../../docs/11-operations/MONITORING.md), F001 đã tạo bản local). Script kiểm parse Markdown MONITORING (R33) thay vì nhân bản danh sách. GlitchTip và Grafana mỗi thứ một database riêng trên node `ops`, không dùng cụm PostgreSQL nghiệp vụ (ADR-0037).

## Phase 0: Research

→ [research.md](research.md): R1 Alertmanager vs Grafana Alerting, R2 exporter Swarm tự viết, R3 GlitchTip, R4 probe synthetic.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md) (không có bảng nghiệp vụ)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `scripts/obs/lib-monitoring-md.mjs` | Parse bảng MONITORING §3 (metric, loại, label, nguồn), §5 (alert, mức), §4 (dashboard) |
| application | `check-metric-catalog`, `check-alerts`, `check-label-policy`, `check-dashboards` | Mỗi script trả danh sách vi phạm + exit code |
| infrastructure | `swarm-exporter` (Docker API chỉ đọc qua socket proxy: `GET /services`, `/tasks`, `/nodes`) | Xuất `swarm_service_replicas_desired`, `swarm_service_replicas_running`, `swarm_task_restarts_total`, `swarm_node_ready` |
| presentation | Stack service trong `infra.stack.yml`; workflow `.github/workflows/obs.yml` | Label `prometheus.scrape=true`, `prometheus.port=9090` trên Swarm service backend (`core-api`, `realtime`, `worker-*`) |

### Tích hợp và phụ thuộc

- F002: `src/backend/pkg/otel` xuất OTLP tới `otel-collector:4317`, log JSON; F130: network `admin`, node `ops`, Docker secret, metric backup/scale; F080: metric client từ telemetry; F004: plugin APISIX `prometheus`/`opentelemetry`, realm Keycloak (client `eduworld-ops-tools` cho Grafana/GlitchTip — task phối hợp F004).
- Alert routing: receiver `oncall-critical` (webhook chat nhóm + SMS gateway + email), `ops-warning` (chat); URL/token trong Docker secret `ops_alert_*`.

### Bảo mật, tenant, idempotency, audit

- Grafana OAuth Keycloak (không MFA — ADR-0094 mục 2b), role map: `platform_ops` → Editor, `system_admin` → Admin, còn lại Viewer chỉ khi có `platform_staff`; tài khoản tenant bị từ chối (`role_attribute_strict`).
- Loki production: bật query audit log của Grafana (`[log] filters` + audit) và lưu trong Loki tenant riêng `audit-ops`.
- Không idempotency nghiệp vụ; ticket GlitchTip khử trùng theo `issue_id`.

### Quan sát

- Meta-monitoring: `Watchdog`, `PrometheusTargetDown`, `PrometheusTargetSampleLimit`, `PrometheusRuleFailures`, `AlertmanagerNotificationsFailing`, `SyntheticProbeFailing` — bổ sung vào MONITORING §5 cùng PR (task T044).

### Rủi ro ADR

- ADR-0074 Accepted: đã áp — nhãn `service` = binary, thêm `module`/`profile`; alert theo Swarm service/module (`ServiceDown`, `ModuleDatabaseUnavailable`, `ModuleHighErrorRate`, `PanicRecoveredSpike` — MONITORING §5). ADR-0075 Accepted: alert `ObjectStorage*` theo backend object storage đã chọn (MONITORING §5).

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
