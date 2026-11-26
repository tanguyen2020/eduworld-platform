# ChatDLQNotEmpty

> Runbook của alert `ChatDLQNotEmpty` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/chat.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _DLQ_CHAT holds messages_
- Điều kiện (MONITORING §5): `nats_stream_total_messages{stream="DLQ_CHAT"} > 0`

## Tác động

Có tin chat không lưu được.

## Chẩn đoán

1. Message trong `DLQ_CHAT` (không chép nội dung ra ngoài)

## Xử lý

1. Sửa nguyên nhân và replay

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [CHAT_SYSTEM](../../03-social-multiplayer/CHAT_SYSTEM.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
