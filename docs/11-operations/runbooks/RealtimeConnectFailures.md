# RealtimeConnectFailures

> Runbook của alert `RealtimeConnectFailures` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/realtime.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _More than 10% of WebSocket connects fail for 5 minutes_
- Điều kiện (MONITORING §5): `ws_connect_total{result!="ok"}` > 10% trong 5 phút

## Tác động

Học sinh không vào được hub/trận (WebSocket), có thể kẹt ở màn hình kết nối.

## Chẩn đoán

1. `eduworld_ws_connect_total` theo `result` (Dashboard Grafana **Realtime**): `ticket_invalid` = vé thiếu/sai/hết hạn/đã dùng (lỗi phát ticket ở `core-api` module game hoặc client dùng lại vé; lý do chi tiết ở `eduworld_rt_ticket_rejected_total{reason}`); `rejected` = realtime từ chối (draining, Valkey, Origin/subprotocol, sức chứa)
2. APISIX route `/realtime/v1` và upstream realtime
3. Grafana → Explore (Loki) `{service="<binary>"} | json | level="error"`, lọc theo `trace_id` lấy từ Tempo

## Xử lý

1. Ticket lỗi: kiểm tra `core-api` (module game, `issueRealtimeTicket`) và Valkey
2. Realtime quá tải: scale `realtime` (drain có `stop_grace_period`)
3. Sau deploy realtime: `make deploy-rollback ENV=<env> SERVICE=<swarm-service>` nếu bắt đầu sau một bản deploy ([DEPLOYMENT §11](../DEPLOYMENT.md))

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [REALTIME_ARCHITECTURE](../../06-architecture/REALTIME_ARCHITECTURE.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
