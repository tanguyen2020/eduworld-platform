# NotificationConsumerBacklog

> Runbook của alert `NotificationConsumerBacklog` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/chat.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Notification consumer <consumer> has more than 1000 pending events for 10 minutes_
- Điều kiện (MONITORING §5): `eduworld_consumer_pending{consumer=~"chat.notify_.*"} > 1000` trong 10 phút (F123)

## Tác động

Học sinh nhận thông báo trong game muộn.

## Chẩn đoán

1. Consumer `chat.notify_*`, `DLQ_CHAT`, lỗi modcall chat→learning (`listAssignmentRecipients`)

## Xử lý

1. Sửa module learning/consumer; scale `worker-chat`

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [CHAT_SYSTEM](../../03-social-multiplayer/CHAT_SYSTEM.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
