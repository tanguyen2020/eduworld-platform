# ObjectStorageDown

> Runbook của alert `ObjectStorageDown` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/storage.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _Object storage unhealthy: S3 gateway down or more than 5% S3 errors from the modules_
- Điều kiện (MONITORING §5): Health check backend lỗi hoặc tỷ lệ lỗi S3 từ adapter S3 của các module > 5% trong 2 phút

## Tác động

Media/bài nộp không đọc/ghi được.

## Chẩn đoán

1. Gateway S3 (`storage-s3`), lỗi S3 theo module
2. Node `storage`

## Xử lý

1. RB-05 (BACKUP_RECOVERY); kiểm tra nhà cung cấp hoặc node `storage`

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [BACKUP_RECOVERY](../BACKUP_RECOVERY.md)
- [DEPLOYMENT](../DEPLOYMENT.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
