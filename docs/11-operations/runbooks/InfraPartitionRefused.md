# InfraPartitionRefused

> Runbook của alert `InfraPartitionRefused` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/async.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Expired partition of <table> kept: unpublished outbox or unhandled dead letters_
- Điều kiện (MONITORING §5): `increase(eduworld_infra_partition_refused_total[1d]) > 0`

## Tác động

Partition hết hạn được giữ lại vì còn outbox chưa publish/dead letter chưa xử lý.

## Chẩn đoán

1. Relay outbox và DLQ của module

## Xử lý

1. Xử lý outbox/DLQ; job sẽ gỡ ở lượt sau

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [DATABASE_DESIGN](../../07-backend/DATABASE_DESIGN.md)
- [EVENT_DESIGN](../../07-backend/EVENT_DESIGN.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
