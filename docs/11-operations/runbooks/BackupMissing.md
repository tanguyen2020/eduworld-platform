# BackupMissing

> Runbook của alert `BackupMissing` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/backup.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _No successful backup of <target> for 26 hours_
- Điều kiện (MONITORING §5): Job backup thất bại hoặc không có backup thành công trong 26h

## Tác động

Không có backup thành công của một target trong 26 giờ (`alert_thresholds`).

## Chẩn đoán

1. `eduworld_backup_last_success_timestamp` theo target/type; lịch `ops-jobs.crontab`

## Xử lý

1. Chạy backup thủ công; sửa lịch/credential

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [BACKUP_RECOVERY](../BACKUP_RECOVERY.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
