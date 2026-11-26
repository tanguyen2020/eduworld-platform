# ModuleDatabaseUnavailable

> Runbook của alert `ModuleDatabaseUnavailable` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/api.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _Module <module> has no connection to its database <database> (<service>)_
- Điều kiện (MONITORING §5): Pool database của một module trong `core-api` không kết nối được > 2 phút (label `module`)

## Tác động

Route của module đó trả `503`; module khác trong `core-api` vẫn chạy (DEPLOYMENT §14).

## Chẩn đoán

1. `/readyz` của `core-api` (check `db:<module>`), `eduworld_db_pool_connections{module}`
2. PgBouncer (`pgbouncer`), role DB của module, Patroni primary

## Xử lý

1. Khôi phục PgBouncer/primary; kiểm tra secret `<module>_db_dsn` sau xoay vòng
2. Failover PostgreSQL: theo BACKUP_RECOVERY

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [DEPLOYMENT](../DEPLOYMENT.md)
- [DATABASE_DESIGN](../../07-backend/DATABASE_DESIGN.md)
- [BACKUP_RECOVERY](../BACKUP_RECOVERY.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
