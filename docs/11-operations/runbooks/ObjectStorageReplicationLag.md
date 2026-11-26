# ObjectStorageReplicationLag

> Runbook của alert `ObjectStorageReplicationLag` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/storage.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _Replication of bucket <bucket> is more than 5 minutes behind (RPO)_
- Điều kiện (MONITORING §5): Độ trễ replication bucket riêng tư > 5 phút trong 10 phút

## Tác động

Bucket riêng tư trễ replication > 5 phút: vi phạm RPO.

## Chẩn đoán

1. `eduworld_object_storage_replication_lag_seconds` theo bucket, `storage-filer-sync`

## Xử lý

1. Khôi phục replication; xem BACKUP_RECOVERY §1

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [BACKUP_RECOVERY](../BACKUP_RECOVERY.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
