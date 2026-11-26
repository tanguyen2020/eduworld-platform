# NotificationDeliverySlow

> Runbook của alert `NotificationDeliverySlow` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/chat.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _p95 from the source event to notification.item.created above 2 s (SC-001 of F123)_
- Điều kiện (MONITORING §5): p95 `eduworld_notification_delivery_latency_seconds` > 2 s trong 15 phút (SC-001 của F123)

## Tác động

p95 thông báo > 2 s (SC-001 F123).

## Chẩn đoán

1. Độ trễ consumer `chat.notify_*`, publish Valkey `ew:rt:user:<id>`

## Xử lý

1. Như [NotificationConsumerBacklog](NotificationConsumerBacklog.md)

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [CHAT_SYSTEM](../../03-social-multiplayer/CHAT_SYSTEM.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
