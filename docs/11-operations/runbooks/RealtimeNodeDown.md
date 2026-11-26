# RealtimeNodeDown

> Runbook của alert `RealtimeNodeDown` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/realtime.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _A realtime replica (<instance>) cannot be scraped for 2 minutes_
- Điều kiện (MONITORING §5): Replica `realtime` không scrape được 2 phút

## Tác động

Một replica realtime không scrape được: người chơi trên replica đó bị ngắt và kết nối lại.

## Chẩn đoán

1. `docker service ps eduworld_<swarm-service> --no-trunc` và `docker service logs eduworld_<swarm-service> --since 15m` (từ node manager, qua VPN) với `realtime`
2. Node chứa task: `swarm_node_ready`

## Xử lý

1. Swarm tự khởi động lại task; nếu node hỏng xem [SwarmNodeDown](SwarmNodeDown.md)
2. Restart lặp: [SwarmTaskRestartLoop](SwarmTaskRestartLoop.md)

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [REALTIME_ARCHITECTURE](../../06-architecture/REALTIME_ARCHITECTURE.md)
- [DEPLOYMENT](../DEPLOYMENT.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
