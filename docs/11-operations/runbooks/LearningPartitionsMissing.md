# LearningPartitionsMissing

> Runbook của alert `LearningPartitionsMissing` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/learning.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _Fewer than 2 months of partitions ahead for <table> (no DEFAULT partition)_
- Điều kiện (MONITORING §5): `eduworld_learning_events_partition_months_ahead < 2` (bảng `learning_events`, `attempt_answers`, `mastery_history`, `progress_daily`)

## Tác động

Bảng học tập không có partition DEFAULT: ghi sẽ lỗi khi hết tháng.

## Chẩn đoán

1. Log job `learning-events-partitions` (worker jobs)

## Xử lý

1. Chạy lại job ngay

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [DATABASE_DESIGN](../../07-backend/DATABASE_DESIGN.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
