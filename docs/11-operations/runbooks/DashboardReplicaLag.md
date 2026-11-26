# DashboardReplicaLag

> Runbook của alert `DashboardReplicaLag` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/dashboard.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Replica <instance> serving the dashboards lags more than 60 s_
- Điều kiện (MONITORING §5): Replica dành cho dashboard lag > 60 s trong 10 phút

## Tác động

Số liệu dashboard bị trễ (> `dashboard_replica_lag_alert_seconds`).

## Chẩn đoán

1. Replica lag PostgreSQL

## Xử lý

1. Không chuyển truy vấn về primary; xem [PostgresReplicationLag](PostgresReplicationLag.md)

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [DASHBOARD_DATA](../../07-backend/DASHBOARD_DATA.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
