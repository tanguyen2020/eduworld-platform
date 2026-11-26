# ChatPersistLagHigh

> Runbook của alert `ChatPersistLagHigh` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/chat.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _chat-persister is behind (p95 lag above 5 s or more than 10 000 pending)_
- Điều kiện (MONITORING §5): `histogram_quantile(0.95, eduworld_chat_persist_lag_seconds) > 5` trong 10' hoặc `nats_consumer_num_pending{consumer="persister"} > 10000`

## Tác động

Tin chat lưu PostgreSQL chậm (JetStream giữ 72 h nên còn thời gian).

## Chẩn đoán

1. `chat-persister` (worker-chat), PostgreSQL

## Xử lý

1. Scale `worker-chat`; sửa DB

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không cần gọi ngoài giờ: tạo ticket cho đội sở hữu (thẻ `module`/`service`) và theo dõi trong giờ làm việc; leo thang như critical nếu alert critical liên quan cùng bắn.

## Liên quan

- [CHAT_SYSTEM](../../03-social-multiplayer/CHAT_SYSTEM.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
