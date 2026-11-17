# Data Model: F131 — Monitoring, alerting, dashboard vận hành, runbook

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — F131 không tạo/sửa bảng trong database module. Dữ liệu quan sát (TSDB Prometheus, chunk Loki/Tempo, DB nội bộ của Grafana/GlitchTip) là dữ liệu vận hành trên volume node `ops`, không phải nguồn sự thật nghiệp vụ (ADR-0004, ADR-0037). Hợp đồng nội bộ của feature:

### Alert rule — `deploy/observability/prometheus/rules/<nhóm>.yaml`

| Trường | Ràng buộc |
|---|---|
| `alert` | Trùng tên cột Alert ở MONITORING §5 (PascalCase, ≤ 64 ký tự) |
| `labels.severity` | `critical` \| `warning` |
| `labels.service` / `labels.module` / `labels.profile` | `OTEL_SERVICE_NAME` (binary `core-api`, `realtime`, `worker`), module và profile worker khi alert theo binary/module (MONITORING §2) |
| `annotations.summary` | ≤ 200 ký tự, không PII |
| `annotations.runbook_url` | Đường dẫn repo `docs/11-operations/runbooks/<alert>.md` (render qua base URL Git nội bộ) |
| `annotations.source` | Link mục MONITORING sở hữu ngưỡng |

### Runbook — `docs/11-operations/runbooks/<AlertName>.md` (Mới)

Mục bắt buộc theo thứ tự: `## Triệu chứng`, `## Tác động`, `## Chẩn đoán`, `## Xử lý`, `## Leo thang`, `## Liên quan` (link MONITORING, BACKUP_RECOVERY RB-xx, DEPLOYMENT).

### Báo cáo diễn tập alert — `docs/11-operations/alert-drills/<YYYY-MM>.md` (Mới)

Trường: alert, thời điểm kích hoạt, thời điểm nhận ở kênh trực, người nhận, kết quả, vấn đề.

### Metric mới (bổ sung MONITORING §3)

| Metric | Loại | Label |
|---|---|---|
| `eduworld_synthetic_probe_success` | Gauge (0/1) | `probe` (`client_config`, `student_login`), `result` |
| `eduworld_synthetic_probe_duration_seconds` | Histogram | `probe` |
| `swarm_service_replicas_desired`, `swarm_service_replicas_running`, `swarm_task_restarts_total`, `swarm_node_ready` | Gauge/Counter | `service`, `node` (đã có tên ở MONITORING §1) |

## 2. Vòng đời / trạng thái

Alert: `inactive` → `pending` (`for`) → `firing` → `resolved`; silence có `createdBy` + `comment` bắt buộc.

## 3. Quy tắc kiểm tra dữ liệu

- Mọi alert MONITORING §5 ↔ rule ↔ runbook là song ánh (F131-AC-08, F131-AC-13).
- Label cấm theo MONITORING §2 (F131-AC-06).

## 4. Event / outbox

| Event | Khi nào | Payload schema |
|---|---|---|
| — | Không phát event nghiệp vụ | — |
