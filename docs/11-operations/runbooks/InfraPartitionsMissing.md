# InfraPartitionsMissing

> Runbook của alert `InfraPartitionsMissing` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/async.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Less than 7 days of partitions ahead for <table> (<module>)_
- Điều kiện (MONITORING §5): `eduworld_infra_partition_days_ahead < 7`

## Tác động

Còn < 7 ngày partition phía trước; ghi rơi vào `*_default` nếu không sửa.

## Chẩn đoán

1. Log job `infra-partition-maintenance` (worker jobs)

## Xử lý

1. Chạy lại job; sửa lỗi quyền `*_partition_admin_dsn`

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [DATABASE_DESIGN](../../07-backend/DATABASE_DESIGN.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
