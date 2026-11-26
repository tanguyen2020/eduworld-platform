# PostgresConnectionsHigh

> Runbook của alert `PostgresConnectionsHigh` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/datastore.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _PostgreSQL <instance> uses more than 85% of max_connections_
- Điều kiện (MONITORING §5): > 85% `max_connections`

## Tác động

Gần `max_connections`: kết nối mới sẽ bị từ chối.

## Chẩn đoán

1. `pg_stat_database_numbackends` theo database; PgBouncer pool

## Xử lý

1. Kiểm tra pool của binary (`eduworld_db_pool_connections`), giảm pool hoặc tăng qua PgBouncer

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không cần gọi ngoài giờ: tạo ticket cho đội sở hữu (thẻ `module`/`service`) và theo dõi trong giờ làm việc; leo thang như critical nếu alert critical liên quan cùng bắn.

## Liên quan

- [DATABASE_DESIGN](../../07-backend/DATABASE_DESIGN.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
