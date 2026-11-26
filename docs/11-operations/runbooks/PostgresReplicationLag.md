# PostgresReplicationLag

> Runbook của alert `PostgresReplicationLag` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/datastore.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _PostgreSQL replica <instance> lags more than 30 s (RPO at risk)_
- Điều kiện (MONITORING §5): > 30s

## Tác động

Replica PostgreSQL trễ > 30 s: nguy cơ RPO và số liệu dashboard cũ.

## Chẩn đoán

1. Dashboard Grafana **Data stores**: replication lag, IO, WAL
2. Patroni (`patronictl list`)

## Xử lý

1. Giảm tải ghi bất thường; kiểm tra mạng/đĩa replica
2. Replica hỏng: dựng lại theo BACKUP_RECOVERY

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [BACKUP_RECOVERY](../BACKUP_RECOVERY.md)
- [DATABASE_DESIGN](../../07-backend/DATABASE_DESIGN.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
