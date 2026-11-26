# PrometheusTargetDown

> Runbook của alert `PrometheusTargetDown` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/meta.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Scrape target <job> (<instance>) is down for 5 minutes_
- Điều kiện (MONITORING §5): `up == 0` > 5 phút

## Tác động

Một target không scrape được: alert dựa trên target đó bị mù.

## Chẩn đoán

1. Prometheus → Status → Targets: lỗi scrape
2. Task/network `admin`/`app`

## Xử lý

1. Khởi động lại task; sửa network/label `prometheus.scrape`

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ trừ khi là binary backend.

## Liên quan

- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
