# PostgresDiskHigh

> Runbook của alert `PostgresDiskHigh` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/datastore.yaml` · Mức: **warning / critical**.

## Triệu chứng

- Thông báo: _Disk of PostgreSQL node <node> above 80%_
- Điều kiện (MONITORING §5): > 80%

## Tác động

Đĩa PostgreSQL đầy dần (> 90% là nguy hiểm: ghi sẽ dừng).

## Chẩn đoán

1. node_exporter filesystem của node `data-pg`, WAL tích lũy (archive pgBackRest lỗi?)

## Xử lý

1. Sửa archive WAL ([BackupFailed](BackupFailed.md)); mở rộng đĩa

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [BACKUP_RECOVERY](../BACKUP_RECOVERY.md)
- [DATABASE_DESIGN](../../07-backend/DATABASE_DESIGN.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
