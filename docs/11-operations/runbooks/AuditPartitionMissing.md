# AuditPartitionMissing

> Runbook của alert `AuditPartitionMissing` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/identity.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _Next month partition of <table> is missing_
- Điều kiện (MONITORING §5): `increase(eduworld_partition_missing_total[1d])` > 0 (tháng kế tiếp chưa có partition, kiểm trước ≥ 7 ngày)

## Tác động

Ghi audit sẽ lỗi khi sang tháng mới (không có partition DEFAULT).

## Chẩn đoán

1. `eduworld_partition_missing_total` theo `table`
2. Log job `partition-maintainer` (worker jobs)

## Xử lý

1. Chạy job `partition-maintainer` (`make ops-job ...`) và kiểm tra partition tháng tới đã có

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [DATABASE_DESIGN](../../07-backend/DATABASE_DESIGN.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
