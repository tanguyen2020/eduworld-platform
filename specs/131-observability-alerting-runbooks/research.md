# Research: F131 — Monitoring, alerting, dashboard vận hành, runbook

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Đánh giá rule và định tuyến alert

- **Decision**: Prometheus đánh giá rule từ `deploy/observability/prometheus/rules/*.yaml`; Alertmanager định tuyến, gom nhóm, inhibit, silence; Grafana chỉ hiển thị (datasource Alertmanager để xem alert).
- **Rationale**: MONITORING §5 đã đặt rule ở `prometheus/rules`; `promtool test rules` cho unit test từng alert (F131-AC-08); `amtool` kiểm cấu hình định tuyến; không phụ thuộc DB của Grafana để alert hoạt động.
- **Alternatives**: Grafana Unified Alerting (rule lưu trong DB Grafana, khó review/test trong Git); Grafana data-source-managed rules (vẫn cần Prometheus ruler + Alertmanager).

## R2. Metric trạng thái Swarm

- **Decision**: Viết `tools/swarm-exporter` (Go, ~200 dòng) đọc Docker API qua socket proxy chỉ `GET`, xuất 4 metric MONITORING §1 đặt tên.
- **Rationale**: Docker Engine `metrics-addr` không có replica desired/running theo service; exporter cộng đồng thường cần socket đầy đủ.
- **Alternatives**: `docker_state_exporter`/`swarm-exporter` bên thứ ba (quyền socket rộng, tên metric khác MONITORING).

## R3. Error tracking

- **Decision**: GlitchTip tự host (API tương thích Sentry SDK), 1 replica trên node `ops`, PostgreSQL riêng `glitchtip` + Valkey nhỏ riêng; SDK `sentry-go` trong `pkg/otel` (F002) với `SendDefaultPII=false`, `BeforeSend` xóa request body/header; tích hợp ticket bằng webhook GlitchTip → GitHub Issues (label `module:<name>` hoặc `service:<binary>`).
- **Rationale**: ADR-0023 đề xuất GlitchTip/Sentry tự host; GlitchTip nhẹ hơn Sentry self-hosted (không Kafka/ClickHouse).
- **Alternatives**: Sentry self-hosted (≥ 16 GB RAM, nhiều thành phần); chỉ dùng Loki (không gom nhóm issue).

## R4. Synthetic monitoring

- **Decision**: Service `synthetic-probe` (image `grafana/k6` + supercronic) chạy 2 script mỗi 5 phút; xuất metric qua Prometheus remote write (`k6 run -o experimental-prometheus-rw`) với tên `eduworld_synthetic_probe_success{probe}` và `eduworld_synthetic_probe_duration_seconds{probe}` (mapping trong script); credential QA qua Docker secret.
- **Rationale**: Dùng lại k6 của F132; kiểm luồng có xác thực (blackbox exporter không làm được).
- **Alternatives**: blackbox_exporter (chỉ HTTP đơn, không đăng nhập); dịch vụ SaaS (dữ liệu ra ngoài vùng, MONITORING §9).
