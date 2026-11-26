# ChatSendLatencyHigh

> Runbook của alert `ChatSendLatencyHigh` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/chat.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _p95 chat send latency above 0.3 s for 10 minutes (NFR-PERF-003)_
- Điều kiện (MONITORING §5): p95 `eduworld_chat_send_latency_seconds` > 0,3 s trong 10'

## Tác động

Chat chậm > 0,3 s p95 (NFR-PERF-003).

## Chẩn đoán

1. Thời gian kiểm duyệt (`eduworld_chat_filter_duration_seconds`), JetStream ack

## Xử lý

1. Scale `core-api`/NATS; xem regex chậm (`eduworld_chat_re2_timeouts_total`)

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không cần gọi ngoài giờ: tạo ticket cho đội sở hữu (thẻ `module`/`service`) và theo dõi trong giờ làm việc; leo thang như critical nếu alert critical liên quan cùng bắn.

## Liên quan

- [CHAT_SYSTEM](../../03-social-multiplayer/CHAT_SYSTEM.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
