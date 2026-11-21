# Feature Specification: F131 — Monitoring, alerting, dashboard vận hành, runbook

**Feature Branch**: `131-observability-alerting-runbooks` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: hạ tầng observability (`deploy/observability`, `deploy/swarm/<env>/infra.stack.yml`, `scripts/obs`, `docs/11-operations/runbooks`) · **Mốc**: M6 · **Phụ thuộc**: F002, F130
**Input**: Dựng stack quan sát staging/production (ADR-0023) trên Docker Swarm, kiểm metric bắt buộc, alert có runbook và định tuyến người trực, dashboard as code, SLO burn rate, error tracking, synthetic monitoring — FR-OPS-003, NFR-OBS-002.

## Clarifications

### Session 2026-11-16

- Q: Ai đánh giá rule và định tuyến alert — Grafana Alerting hay Prometheus + Alertmanager? → A: Prometheus đánh giá rule trong `deploy/observability/prometheus/rules/*.yaml` ([MONITORING §5](../../docs/11-operations/MONITORING.md)), Alertmanager định tuyến/khử trùng/im lặng; Grafana chỉ hiển thị. Chi tiết ở [research.md](research.md) [Confirmed].
- Q: Runbook theo alert đặt ở đâu (MONITORING chỉ nói "mục runbook trong repo")? → A: `docs/11-operations/runbooks/<AlertName>.md`, mỗi file một alert, khung mục cố định; RB-01…05 vẫn do [BACKUP_RECOVERY §9](../../docs/11-operations/BACKUP_RECOVERY.md) sở hữu, runbook alert chỉ link [Confirmed].
- Q: Ai phát metric nghiệp vụ (CCU, matchmaking, chat…)? → A: Module sở hữu (feature nền của module) phát qua `src/backend/pkg/otel` (F002) với nhãn `service` (= binary `core-api`, `realtime`, `worker`), `module` và `profile` (worker) theo [MONITORING §2](../../docs/11-operations/MONITORING.md) (ADR-0074); F131 kiểm **có mặt** đúng tên/label theo [MONITORING §3](../../docs/11-operations/MONITORING.md), dựng dashboard/alert. Metric client từ telemetry do F080 chuyển đổi.
- Q: Synthetic monitoring dùng tài khoản nào? → A: Tenant QA riêng (seed như tenant demo, không học sinh thật) theo [TEST_STRATEGY §4](../../docs/12-project/TEST_STRATEGY.md); chỉ luồng đọc + đăng nhập học sinh QA [Confirmed].

### Session 2026-11-21 (owner bỏ MFA — [Confirmed])

- Q: Owner bỏ MFA/TOTP cho đăng nhập Admin Portal với mọi tài khoản, kể cả role platform (2026-11-21, [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md)) — phần nào của feature này đổi? → A: ADR-0094 bỏ OTP và mapper AMR/LoA khỏi realm `eduworld`, nên yêu cầu "đăng nhập Keycloak có MFA" của F131-AC-07 (Grafana/GlitchTip qua client `eduworld-ops-tools`, T005, T012, plan) không còn đạt được bằng realm này. [Confirmed] Giữ chặn bằng network `admin` (VPN) + role vận hành platform, bỏ yêu cầu `acr` MFA. **Owner chốt 2026-11-21:** không giữ MFA riêng cho công cụ vận hành (Grafana, GlitchTip, Prometheus, Alertmanager, Loki, Tempo) — cùng phạm vi ADR-0094; F131-AC-07 viết lại theo đó.

### Session 2026-11-22 (ADR-0094 — [Confirmed])

- Q: Văn bản còn mô tả MFA/`MFA_REQUIRED`/step-up như hành vi hiện hành? → A: [Confirmed] Viết lại theo [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md), giữ nguyên mã AC: Constitution, plan, contracts (công cụ quan sát không MFA — ADR-0094 mục 2b). Route platform chỉ còn gateway nội bộ + `azp = eduworld-portal` + `user_type = staff` + role platform; thiếu → `403 PERMISSION_DENIED` + `authz.denied`; không còn `403 MFA_REQUIRED` hay hộp đăng nhập lại (step-up). Câu trả lời Clarifications cũ nhắc MFA là lịch sử.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Thu thập metric, log, trace xuyên binary và module trên Swarm (Priority: P1)

Người trực vận hành mở Grafana thấy mọi Swarm service (`core-api`, `realtime`, `worker-events`, `worker-chat`, `worker-jobs`), module, node, datastore; từ một lỗi 5xx tra được trace xuyên APISIX → `core-api` (span facade sang module khác qua `src/backend/pkg/modcall`) → NATS → `worker` → PostgreSQL và log cùng `trace_id`, không có PII.

**Why this priority**: NFR-OBS-002 là điều kiện để mọi alert/SLO có dữ liệu; thiếu thì load/chaos test (F132) không có số đo.

**Independent Test**: Trên staging, gửi request mẫu tạo lỗi có chủ đích, tìm trace trong Tempo, log trong Loki cùng `trace_id`; chạy `check-metric-catalog` đạt 100%.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F131-AC-01 | HỆ THỐNG PHẢI chạy trên node `ops`: OTel Collector, Prometheus, Alertmanager, Loki, Tempo (bật service graph), Grafana; và dạng `mode: global`: node_exporter, cAdvisor; cùng postgres_exporter, redis_exporter (Valkey), prometheus-nats-exporter, metric Keycloak, plugin `prometheus` + `opentelemetry` của APISIX, exporter Swarm (chỉ đọc); KHI target có label `prometheus.scrape=true`, Prometheus PHẢI phát hiện qua `dockerswarm_sd_configs` và `up == 1` | NFR-OBS-002 · [MONITORING §1](../../docs/11-operations/MONITORING.md) · ADR-0023 · ADR-0024 | — |
| F131-AC-02 | Job `obs-metric-catalog` (chạy sau deploy staging trong workflow `release.yml`) PHẢI scrape `/metrics` từng binary (`core-api`, `realtime`, `worker` theo từng profile) trên staging và đối chiếu bảng [MONITORING §3](../../docs/11-operations/MONITORING.md); NẾU thiếu metric bắt buộc của binary/module, sai đơn vị/hậu tố hoặc thiếu label `service`/`module`, THÌ job PHẢI fail và liệt kê metric thiếu | FR-OPS-003 · NFR-OBS-002 | — |
| F131-AC-03 | KHI một request đi qua APISIX tới `core-api`, gọi facade của module khác qua `src/backend/pkg/modcall` và phát event NATS được `worker` tiêu thụ, HỆ THỐNG PHẢI lưu một trace có cùng `trace_id` gồm span gateway, server, facade (`<caller>→<callee>`), producer/consumer và truy vấn DB; log JSON của các binary liên quan PHẢI có cùng `trace_id` | NFR-OBS-001 · NFR-OBS-002 · [MONITORING §7](../../docs/11-operations/MONITORING.md) | — |
| F131-AC-04 | Ở NƠI môi trường là staging/production, Collector PHẢI head sampling 10% và tail sampling giữ 100% trace lỗi và trace > 1 s; local/dev PHẢI giữ 100% | [MONITORING §7](../../docs/11-operations/MONITORING.md) | — |
| F131-AC-05 | Loki PHẢI giữ log theo `app_log_retention`; Collector PHẢI có processor `redaction` che token/JWT/email/SĐT; NẾU log hoặc thuộc tính span chứa mẫu cấm ([MONITORING §8](../../docs/11-operations/MONITORING.md)), THÌ giá trị PHẢI bị che trước khi lưu | NFR-OBS-002 · AGENTS R14 · [MONITORING §8](../../docs/11-operations/MONITORING.md) | TC-AUTH-060 |
| F131-AC-06 | NẾU rule Prometheus, dashboard hoặc metric scrape được có label thuộc danh sách cấm (`user_id`, `student_id`, `school_id`, `ip`, `email`, `username`, `message`, nội dung) hoặc label `route` không phải template, THÌ job `obs-label-policy` PHẢI fail | [MONITORING §2](../../docs/11-operations/MONITORING.md) · AGENTS R14 | — |
| F131-AC-07 | Grafana, Prometheus, Alertmanager, Loki, Tempo, GlitchTip PHẢI chỉ truy cập qua network `admin` (VPN) với đăng nhập Keycloak bằng tài khoản platform (không MFA, ADR-0094); chỉ nhân sự có role vận hành platform được vào; NẾU tài khoản role tenant hoặc học sinh đăng nhập, THÌ HỆ THỐNG PHẢI từ chối; mọi truy vấn Loki production PHẢI được ghi nhật ký truy cập | [MONITORING §8](../../docs/11-operations/MONITORING.md) · [SECURITY_ARCHITECTURE §8](../../docs/10-security/SECURITY_ARCHITECTURE.md) · AGENTS R20 | — |

---

### User Story 2 — Alert có runbook, đến đúng người trực (Priority: P1)

Khi `ServiceDown` hay `BackupMissing` xảy ra, người trực nhận thông báo trong vài phút, mở link runbook và làm theo.

**Why this priority**: DEPLOYMENT §15 và exit criteria M6 yêu cầu "alert chính đã kích hoạt thử và đến đúng người trực".

**Independent Test**: Chạy `make alert-drill ENV=staging ALERT=ServiceDown`; xác nhận thông báo tới kênh trực và runbook mở được.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F131-AC-08 | Mỗi alert trong [MONITORING §5](../../docs/11-operations/MONITORING.md) PHẢI có rule cùng tên trong `deploy/observability/prometheus/rules/`, label `severity` (`critical`/`warning`) và `service`/`module` (khi theo binary/module), annotation `summary` và `runbook_url`; job `obs-rules` PHẢI chạy `promtool check rules` và `promtool test rules` (mỗi alert ít nhất một ca kích hoạt và một ca không kích hoạt) và fail khi thiếu alert so với bảng | FR-OPS-003 · [MONITORING §5](../../docs/11-operations/MONITORING.md) | — |
| F131-AC-09 | Alertmanager PHẢI định tuyến `critical` tới kênh trực (chat nhóm vận hành + SMS/email), `warning` tới kênh chat; PHẢI gom nhóm theo `alertname`, `service`, `module`; KHI `ServiceDown` của một Swarm service đang bắn, alert con cùng `service` PHẢI bị inhibit; silence PHẢI có người tạo và lý do | [MONITORING §1, §5](../../docs/11-operations/MONITORING.md) | — |
| F131-AC-10 | HỆ THỐNG PHẢI có alert `Watchdog` luôn bắn gửi tới kênh ngoài (heartbeat); NẾU kênh ngoài không nhận heartbeat quá 10 phút, THÌ người trực PHẢI được báo (phát hiện Prometheus/Alertmanager chết) | [MONITORING §5](../../docs/11-operations/MONITORING.md) | — |
| F131-AC-11 | KHI chạy `make alert-drill ENV=staging`, mỗi alert `critical` PHẢI được kích hoạt giả lập (rule test qua `amtool` hoặc điều kiện thật trên staging) và thông báo PHẢI tới kênh trực trong ≤ 2 phút; kết quả ghi `docs/11-operations/alert-drills/<YYYY-MM>.md` | [DEPLOYMENT §15](../../docs/11-operations/DEPLOYMENT.md) | — |
| F131-AC-12 | Ngưỡng alert PHẢI lấy theo `alert_thresholds` và bảng MONITORING §5 (tài liệu sở hữu); rule file PHẢI ghi link mục MONITORING tương ứng, không định nghĩa ngưỡng ở nơi khác | AGENTS R33 · [MONITORING §5](../../docs/11-operations/MONITORING.md) | — |
| F131-AC-13 | Mỗi `runbook_url` PHẢI trỏ tới file tồn tại `docs/11-operations/runbooks/<AlertName>.md` có đủ mục Triệu chứng, Tác động, Chẩn đoán, Xử lý, Leo thang, Liên quan; NẾU thiếu, THÌ job `obs-runbooks` PHẢI fail | [MONITORING §5](../../docs/11-operations/MONITORING.md) | — |

---

### User Story 3 — Dashboard vận hành và SLO as code (Priority: P1)

Trực vận hành, đội backend, Safety Team có dashboard đúng [MONITORING §4](../../docs/11-operations/MONITORING.md); SLO có burn rate để quyết định đóng băng tính năng.

**Why this priority**: FR-OPS-003 yêu cầu theo dõi các nhóm metric; SLO là căn cứ go-live.

**Independent Test**: Provision Grafana từ Git trên staging, mọi panel có dữ liệu sau 10 phút tải mẫu; recording rule SLO trả giá trị.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F131-AC-14 | HỆ THỐNG PHẢI provision mọi dashboard trong [MONITORING §4](../../docs/11-operations/MONITORING.md) từ JSON trong `deploy/observability/grafana/dashboards/`; dashboard theo binary/module PHẢI dùng biến `service` và `module`; NẾU JSON tham chiếu datasource UID không tồn tại hoặc panel truy vấn metric không có trong MONITORING §3, THÌ job `obs-dashboards` PHẢI fail | FR-OPS-003 · [MONITORING §4](../../docs/11-operations/MONITORING.md) | — |
| F131-AC-15 | HỆ THỐNG PHẢI có recording rule SLI cho mọi SLO ở [MONITORING §6](../../docs/11-operations/MONITORING.md) (availability tính trong giờ học, loại `429`), và alert `SLOBurnRateFast`/`SLOBurnRateSlow` dạng multiwindow; mục tiêu availability lấy từ `availability` | FR-OPS-003 · [MONITORING §6](../../docs/11-operations/MONITORING.md) | — |
| F131-AC-16 | Dashboard Chat Safety PHẢI chỉ hiển thị số đếm/tỷ lệ theo nhóm, không nội dung tin nhắn, không định danh học sinh | [MONITORING §4](../../docs/11-operations/MONITORING.md) · AGENTS R14, R16 | — |

---

### User Story 4 — Error tracking và synthetic monitoring (Priority: P2)

Đội backend sở hữu module nhận issue tự động khi production có panic/5xx bất ngờ; người trực biết khi luồng chính hỏng trước khi trường báo.

**Why this priority**: Rút ngắn thời gian phát hiện; không chặn go-live như US1–US3.

**Independent Test**: Gây panic có recover trên staging → issue GlitchTip có `trace_id`, tag `service`, `module`; tắt route client-config → probe báo lỗi trong 2 chu kỳ.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F131-AC-17 | KHI binary gặp panic đã recover hoặc 5xx không mong đợi, HỆ THỐNG PHẢI gửi sự kiện tới GlitchTip tự host kèm `trace_id`, tag `service`, `module`, `version`; PHẢI KHÔNG gửi request body, header nhạy cảm hay PII (`send_default_pii=false`) | [MONITORING §9](../../docs/11-operations/MONITORING.md) · ADR-0023 · AGENTS R14 | — |
| F131-AC-18 | KHI GlitchTip có issue mới ở production, HỆ THỐNG PHẢI tạo ticket (GitHub Issue) cho đội sở hữu theo tag `module` (hoặc `service` khi lỗi nằm ngoài module) (một ticket mỗi issue, không trùng khi sự kiện lặp) | [MONITORING §9](../../docs/11-operations/MONITORING.md) | — |
| F131-AC-19 | HỆ THỐNG PHẢI chạy probe định kỳ trên production bằng tài khoản tenant QA: `getClientConfig` (không xác thực) và chuỗi `gameLogin` → `createGameSession` → `gameLogout` của học sinh QA; kết quả PHẢI thành metric `eduworld_synthetic_probe_success{probe}` và `eduworld_synthetic_probe_duration_seconds{probe}`; NẾU probe thất bại 2 chu kỳ liên tiếp, THÌ alert PHẢI bắn | [TEST_STRATEGY §4](../../docs/12-project/TEST_STRATEGY.md) · [MONITORING §6](../../docs/11-operations/MONITORING.md) | — |
| F131-AC-20 | Probe synthetic PHẢI KHÔNG ghi dữ liệu học tập (không start attempt), PHẢI dùng tenant QA riêng và credential qua Docker secret; NẾU probe gặp `429 LOGIN_QUEUE_BUSY` hoặc `RATE_LIMITED`, THÌ PHẢI ghi `result="rate_limited"` và tôn trọng `Retry-After`, không thử lại dồn dập | FR-OPS-003 · AGENTS R3, R4 · FR-AUTH-027 | TC-AUTH-044 |

### Edge Cases

- NẾU node `ops` chứa Prometheus mất, THÌ `Watchdog` ngừng heartbeat và kênh ngoài báo người trực (F131-AC-10); các binary vẫn chạy (metric bị mất khoảng thời gian đó).
- NẾU một binary/module phát label cardinality cao bất thường, THÌ `obs-label-policy` chặn ở CI; ở runtime Prometheus `sample_limit` theo target cắt scrape và alert `PrometheusTargetSampleLimit` (warning).
- NẾU Loki đầy đĩa, THÌ Collector giữ hàng đợi có giới hạn và bỏ log debug trước; alert disk của node `ops`.
- NẾU tenant QA bị dọn bởi `student-inactive-cleanup` (ADR-0032), THÌ tài khoản QA PHẢI nằm trong danh sách loại trừ của job (cấu hình tenant) — giả định ở Assumptions.
- Tenant isolation, idempotency nghiệp vụ, Problem Details: không áp dụng — F131 không tạo endpoint nghiệp vụ; công cụ quan sát chỉ cho nhân sự vận hành platform (F131-AC-07); dữ liệu theo trường không đưa vào Prometheus (F131-AC-06). Giới hạn độ dài: label/annotation do rule file kiểm bằng `promtool` + policy.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-OPS-003 | Kiểm có mặt metric bắt buộc, dashboard, alert, SLO cho online players, sessions, matchmaking, match, latency, error rate, chat, learning, exam, DB | MONITORING §3–§6 |
| NFR-OBS-002 | Stack OTel/Prometheus/Loki/Tempo/Grafana trên Swarm, redaction, sampling, nhãn `service`/`module`/`profile` | MONITORING §1, §7, §8 · ADR-0023 |

### Key Entities

- **Metric bắt buộc**: tên, loại, label, binary/module phát (MONITORING §3).
- **Alert rule**: tên, điều kiện, mức, binary/module, runbook.
- **Runbook**: tài liệu xử lý cho một alert (Triệu chứng → Leo thang).
- **Dashboard**: tập panel theo người dùng (MONITORING §4).
- **SLO/SLI**: mục tiêu, cửa sổ, recording rule, burn rate.
- **Probe synthetic**: luồng kiểm định kỳ bằng tài khoản QA.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `app_log_retention` | Retention Loki |
| `alert_thresholds` | Ngưỡng alert chính |
| `availability` | Mục tiêu SLO availability |
| `login_slo` | SLO đăng nhập |
| `publish_to_visible_max` | SLO publish nội dung |
| `dashboard_replica_lag_alert_seconds` | Alert `DashboardReplicaLag` |
| `p1_unclaimed_alert_minutes` | Alert case moderation p1 chưa nhận (rule cho profile `worker-chat`) |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không PII/token trong log, label, span | AGENTS R14 · MONITORING §2, §7, §8 |
| Công cụ quan sát chỉ trong mạng admin + đăng nhập Keycloak có role vận hành platform (không MFA — ADR-0094 mục 2b) | SECURITY_ARCHITECTURE §8 |
| Dữ liệu quan sát lưu cùng vùng với production | MONITORING §9 |
| Không Kubernetes metrics/KEDA | ADR-0024 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% metric bắt buộc của MONITORING §3 có mặt trên staging trước go-live.
- **SC-002**: 100% alert của MONITORING §5 có rule, unit test và runbook; 100% alert critical tới kênh trực trong ≤ 2 phút ở lần diễn tập gần nhất.
- **SC-003**: Từ một lỗi 5xx, người trực tìm được trace và log liên quan trong ≤ 5 phút.
- **SC-004**: 0 mẫu PII/token trong log mẫu staging (quét tự động hằng tuần).
- **SC-005**: Lỗi luồng đăng nhập production được phát hiện bởi probe trong ≤ 2 chu kỳ probe.

## Ngoài phạm vi

- Instrument trong code (`src/backend/pkg/otel`, `src/backend/pkg/health`, middleware log) → F001, F002; metric nghiệp vụ từng module → feature của module đó.
- Telemetry client và dashboard Game Analytics/hiệu năng theo hạng thiết bị → F080 ([GAME_ANALYTICS](../../docs/11-operations/GAME_ANALYTICS.md)).
- Metric backup/scale (phát) và stack Swarm → F130; load/chaos test → F132.
- Lịch trực, hợp đồng SMS/email: quy trình tổ chức, không phải code.

## Assumptions

- Alertmanager được thêm vào sơ đồ MONITORING §1 (task tài liệu) [Confirmed].
- Tenant QA được loại khỏi analytics/dashboard nhà trường và khỏi job dọn học sinh bằng cấu hình tenant; cơ chế loại trừ analytics chưa có trong tài liệu — cần bổ sung ở F080/F043 (ghi báo cáo).
- OQ-11 đã trả lời 2026-11-20: mục tiêu SLO dùng `availability` (99,5 %/tháng trong giờ học, CONFIG_DEFAULTS); đổi giá trị chỉ sửa CONFIG_DEFAULTS + MONITORING.
- ADR-0074 Accepted (2026-11-17): nhãn `service` = binary (`core-api`, `realtime`, `worker`) kèm `module` và `profile` ([MONITORING §2](../../docs/11-operations/MONITORING.md)); dashboard/alert theo 5 Swarm service và 5 module domain.
- CI là GitHub Actions (ADR-0088): kiểm tra cấu hình quan sát chạy trong workflow `obs.yml` theo path filter [Confirmed]; ticket lỗi tạo trên GitHub Issues.
