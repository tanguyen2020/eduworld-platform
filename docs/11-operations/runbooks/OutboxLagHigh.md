# OutboxLagHigh

> Runbook của alert `OutboxLagHigh` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/async.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _Outbox of module <module> is more than 60 s behind_
- Điều kiện (MONITORING §5): `eduworld_outbox_lag_seconds{module}` > 60s trong 5 phút

## Tác động

XP, quest, mastery, projection bị trễ > 60 s.

## Chẩn đoán

1. `eduworld_outbox_lag_seconds`, `eduworld_outbox_unpublished_count` theo module
2. Relay trong `worker-events` sống? NATS khỏe?

## Xử lý

1. Khởi động lại/scale `worker-events`; sửa NATS nếu lỗi

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [EVENT_DESIGN](../../07-backend/EVENT_DESIGN.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
