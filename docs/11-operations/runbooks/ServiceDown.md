# ServiceDown

> Runbook của alert `ServiceDown` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/swarm.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _No replica of <swarm_service> is running for 2 minutes_
- Điều kiện (MONITORING §5): Mọi replica của một Swarm service (`core-api`, `realtime`, `worker-*`) không ready > 2 phút (label `service`, `profile`)

## Tác động

`core-api` down = mọi REST của 5 module down; `realtime` down = mất hub/trận; `worker-*` down = event/chat/job dừng (các binary khác chạy chế độ suy giảm).

## Chẩn đoán

1. `docker service ps eduworld_<swarm-service> --no-trunc` và `docker service logs eduworld_<swarm-service> --since 15m` (từ node manager, qua VPN)
2. Panic/trace gần nhất trước khi task chết (GlitchTip, Loki)
3. Node và tài nguyên (`swarm_node_ready`, `NodeCapacityHigh`)

## Xử lý

1. Sau deploy: `make deploy-rollback ENV=<env> SERVICE=<swarm-service>` nếu bắt đầu sau một bản deploy ([DEPLOYMENT §11](../DEPLOYMENT.md))
2. Thiếu tài nguyên/placement: giải phóng node hoặc nới constraint theo DEPLOYMENT §4
3. Alert con cùng `service`/`profile` bị inhibit — xử lý alert này trước

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [DEPLOYMENT](../DEPLOYMENT.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
