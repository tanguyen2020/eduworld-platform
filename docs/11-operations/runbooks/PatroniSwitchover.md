# PatroniSwitchover

> Runbook của alert `PatroniSwitchover` (obsfix sau F132). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/datastore.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Patroni cluster <scope> changed its primary in the last 10 minutes (switchover or failover)_
- Điều kiện (MONITORING §5): timeline Patroni (`patroni_postgres_timeline`, job `patroni` scrape REST `/metrics` cổng 8008 của `pg-N`) tăng so với 10 phút trước — một node khác đã được promote

## Tác động

Ghi bị gián đoạn trong lúc chuyển primary (mục tiêu < 60 s, F132-AC-11); `pg-router`/PgBouncer tự trỏ sang primary mới, không task backend nào phải khởi động lại. Nếu là **failover ngoài kế hoạch** (không có diễn tập/bảo trì) thì primary cũ đã hỏng — có nguy cơ RPO nếu replica đang trễ.

## Chẩn đoán

1. Có kế hoạch không? Đối chiếu lịch bảo trì/diễn tập (`scripts/ops/patroni-switchover.sh`, chaos `pg-switchover`). Có → chỉ xác nhận cụm khỏe.
2. `patronictl -c /etc/patroni/patroni.yml list` trong một task `pg-N`: leader mới, replica còn streaming, lag.
3. `PostgresReplicationLag`, `ModuleDatabaseUnavailable`, tỷ lệ 5xx (`ModuleHighErrorRate`) trong khoảng chuyển.
4. Ngoài kế hoạch: log `docker service logs eduworld_pg-<N>` của primary cũ (OOM, đĩa, mất etcd quorum `etcd-pg-*`).

## Xử lý

1. Có kế hoạch và cụm khỏe (1 leader, replica streaming, lag < 30 s): không cần làm gì; alert tự hết sau 10 phút.
2. Ngoài kế hoạch: xử lý theo [BACKUP_RECOVERY RB-02](../BACKUP_RECOVERY.md#rb-02--mất-primary-postgresql-phần-cứng) — đưa node cũ về làm replica (`pg_rewind`), kiểm tra không mất giao dịch đã commit (đối soát outbox/attempt).
3. Không có leader (`patroni_primary` tổng = 0) hoặc lặp switchover nhiều lần: kiểm tra etcd `etcd-pg-*`, tạm `patronictl pause` nếu flapping, leo thang.

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Failover ngoài kế hoạch, mất dữ liệu nghi ngờ hoặc không có leader quá 5 phút: gọi người trực secondary, sau đó DBA/Technical Lead hạ tầng. Ghi diễn biến vào kênh sự cố.

## Liên quan

- [BACKUP_RECOVERY §9 RB-02](../BACKUP_RECOVERY.md#rb-02--mất-primary-postgresql-phần-cứng)
- [DEPLOYMENT](../DEPLOYMENT.md)
- [PostgresReplicationLag](PostgresReplicationLag.md)
- Chaos `tests/chaos/scenarios/pg-switchover.yaml` (TC-SVC-013)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
