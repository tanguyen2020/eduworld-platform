# Watchdog

> Runbook của alert `Watchdog` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/meta.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Heartbeat of the alerting pipeline (always firing)_
- Điều kiện (MONITORING §5): Luôn bắn (`vector(1)`) — gửi tới heartbeat ngoài mỗi phút

## Tác động

Không có tác động: đây là heartbeat của đường cảnh báo. Khi nó **ngừng**, Prometheus hoặc Alertmanager đã chết và mọi alert khác cũng mù.

## Chẩn đoán

1. Dịch vụ heartbeat ngoài báo mất tín hiệu > 10 phút
2. Trạng thái service `prometheus`, `alertmanager` (obs stack) và node `ops`

## Xử lý

1. Khôi phục Prometheus/Alertmanager (`docker service ps eduworld_prometheus`, `eduworld_alertmanager`)
2. Node `ops` hỏng: dời service theo DEPLOYMENT; metric của khoảng thời gian đó bị mất

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Mất heartbeat: gọi người trực vận hành ngay (kênh ngoài báo trực tiếp).

## Liên quan

- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [DEPLOYMENT](../DEPLOYMENT.md)
- [runbooks/README](README.md)
