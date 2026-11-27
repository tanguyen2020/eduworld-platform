# NatsClusterDown

> Runbook của alert `NatsClusterDown` (obsfix sau F132). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/async.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _No backend binary reaches any NATS node for 1 minute (NATS cluster lost)_
- Điều kiện (MONITORING §5): `max(eduworld_nats_connected) == 0` trong 1 phút — không binary backend nào (`core-api`, `realtime`, `worker-*`) nhận được PONG của bất kỳ node NATS nào
- Đi kèm thường thấy: `/readyz` của mọi binary báo `nats: fail` (`error_code` `timeout`/`not_connected`), `ChatJetStreamUnavailable`, `OutboxLagHigh`, `NatsRequestTimeouts` (lý do `unavailable`/`timeout`)

## Tác động

Mất toàn bộ cụm NATS (một node chết không gây alert này — client tự nối sang node khác):

- Lệnh WebSocket từ `realtime` tới module trong `core-api` không có phản hồi (hub, battle, party); client nhận thông báo lỗi và tự thử lại.
- Chat trả `CHAT_TEMPORARILY_UNAVAILABLE` (không phát tin chưa được JetStream ack — ADR-0035).
- Event outbox dồn lại trong PostgreSQL (không mất), XP/quest/mastery/projection bị trễ; REST ngoài chat vẫn chạy.
- Binary bị APISIX rút khỏi pool vì readiness fail; Swarm **không** khởi động lại task (healthcheck dùng `/livez` — DEPLOYMENT §14).

## Chẩn đoán

1. Trạng thái node NATS: `docker service ps eduworld_nats-1 eduworld_nats-2 eduworld_nats-3`; log `docker service logs eduworld_nats-<N>`.
2. `eduworld_nats_connected` theo `service`/`instance` (Grafana Explore): tất cả 0 = cụm mất; chỉ vài binary 0 = sự cố mạng/overlay của node chạy binary đó.
3. Node data-nats còn `Ready`? (`swarm_node_ready{role="data-nats"}`, `SwarmNodeDown`); đĩa JetStream đầy?
4. Nếu node chạy nhưng không trả lời (bị pause/treo): `/healthz` của `nats-<N>` (`:8222/healthz`), CPU/IO của node.

## Xử lý

1. Khởi động lại node NATS hỏng theo thứ tự (`docker service update --force eduworld_nats-<N>`), ưu tiên đưa về ≥ 2 node để JetStream có quorum.
2. Khi node chạy lại: client NATS tự reconnect (không cần restart binary); kiểm tra `eduworld_nats_connected` về 1 và `/readyz` ok.
3. Theo dõi outbox xả hết (`eduworld_outbox_lag_seconds` < 60 s) và consumer bắt kịp (`nats_consumer_num_pending`); relay dùng `Nats-Msg-Id` nên không trùng event.
4. Nếu dữ liệu JetStream hỏng: khôi phục theo [BACKUP_RECOVERY §5b](../BACKUP_RECOVERY.md#5b-nats-jetstream); outbox trong PostgreSQL là nguồn phát lại.

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead hạ tầng. Ghi diễn biến vào kênh sự cố.

## Liên quan

- [DEPLOYMENT §14](../DEPLOYMENT.md#14-health-readiness-liveness) (readiness NATS, `nats_readiness_probe_timeout`)
- [REALTIME_ARCHITECTURE](../../06-architecture/REALTIME_ARCHITECTURE.md)
- [EVENT_DESIGN](../../07-backend/EVENT_DESIGN.md)
- [ChatJetStreamUnavailable](ChatJetStreamUnavailable.md), [OutboxLagHigh](OutboxLagHigh.md), [NatsRequestTimeouts](NatsRequestTimeouts.md)
- Chaos `tests/chaos/scenarios/nats-down.yaml` (TC-SVC-012)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
