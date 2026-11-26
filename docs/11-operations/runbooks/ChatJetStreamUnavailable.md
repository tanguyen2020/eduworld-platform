# ChatJetStreamUnavailable

> Runbook của alert `ChatJetStreamUnavailable` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/chat.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _Chat cannot publish to JetStream (errors above 5% or NATS quorum lost)_
- Điều kiện (MONITORING §5): `rate(eduworld_chat_jetstream_publish_errors_total[5m]) > 0.05 * rate(eduworld_chat_messages_total[5m])` trong 5' hoặc NATS cluster mất quorum

## Tác động

Chat không gửi được (`CHAT_TEMPORARILY_UNAVAILABLE`).

## Chẩn đoán

1. Cụm NATS (3 node), `nats_*`, lỗi publish theo `reason`

## Xử lý

1. Khôi phục node NATS; quorum JetStream

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [CHAT_SYSTEM](../../03-social-multiplayer/CHAT_SYSTEM.md)
- [DEPLOYMENT](../DEPLOYMENT.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
