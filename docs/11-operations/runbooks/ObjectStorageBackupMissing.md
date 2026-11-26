# ObjectStorageBackupMissing

> Runbook của alert `ObjectStorageBackupMissing` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/storage.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _No successful off-cluster copy of object storage for 26 hours_
- Điều kiện (MONITORING §5): Không có bản sao object storage ngoài cụm/ngoài nhà cung cấp thành công trong 26 giờ

## Tác động

Không có bản sao object storage ngoài cụm trong 26 giờ.

## Chẩn đoán

1. Log job sao chép (backup-tools), `eduworld_backup_last_status{target="object_storage"}`

## Xử lý

1. Chạy lại job sao chép, kiểm tra đích lưu trữ

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [BACKUP_RECOVERY](../BACKUP_RECOVERY.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
