# ValkeyDown

> Runbook của alert `ValkeyDown` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/datastore.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _Valkey <instance> does not answer for 1 minute (realtime degraded)_
- Điều kiện (MONITORING §5): không phản hồi 1 phút

## Tác động

Realtime suy giảm, rate limit/presence lỗi (dữ liệu quan trọng không chỉ ở Valkey, ADR-0004).

## Chẩn đoán

1. Sentinel failover đã xảy ra? `redis_up`

## Xử lý

1. Khôi phục instance; Sentinel chuyển master

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [VALKEY_DESIGN](../../07-backend/VALKEY_DESIGN.md)
- [DEPLOYMENT](../DEPLOYMENT.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
