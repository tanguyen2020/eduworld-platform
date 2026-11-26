# BackupFailed

> Runbook của alert `BackupFailed` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/backup.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _Last <type> run of backup target <target> failed_
- Điều kiện (MONITORING §5): Job backup thất bại hoặc không có backup thành công trong 26h

## Tác động

Lần chạy backup gần nhất thất bại: RPO bị đe dọa nếu lặp lại.

## Chẩn đoán

1. Log backup-tools (supercronic JSON), `eduworld_backup_last_status{target,type}`

## Xử lý

1. Sửa nguyên nhân rồi chạy lại job theo BACKUP_RECOVERY §3

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [BACKUP_RECOVERY](../BACKUP_RECOVERY.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
