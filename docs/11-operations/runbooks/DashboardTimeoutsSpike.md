# DashboardTimeoutsSpike

> Runbook của alert `DashboardTimeoutsSpike` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/dashboard.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _More than 1% of dashboard queries time out (statement_timeout)_
- Điều kiện (MONITORING §5): `eduworld_dashboard_rejected_total{reason="timeout"}` > 1% truy vấn trong 15 phút

## Tác động

Truy vấn dashboard vượt `statement_timeout` 5 s.

## Chẩn đoán

1. Index/partition pruning của query

## Xử lý

1. Sửa query/index

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [DASHBOARD_DATA](../../07-backend/DASHBOARD_DATA.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
