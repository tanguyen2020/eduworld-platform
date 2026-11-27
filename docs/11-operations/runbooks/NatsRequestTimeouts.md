# NatsRequestTimeouts

> Runbook của alert `NatsRequestTimeouts` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/realtime.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _More than 2% of WS commands to module <module> time out on NATS_
- Điều kiện (MONITORING §5): `eduworld_nats_request_errors_total{reason="timeout"}` > 2% lệnh WS trong 5 phút

## Tác động

Lệnh WS (quest, party, chat) của học sinh bị timeout.

## Chẩn đoán

1. `eduworld_nats_request_errors_total` theo `module` và `reason` (`timeout`, `no_responders` = không có handler, `unavailable` = kết nối NATS đóng/đang nối lại — nếu mọi binary cùng `unavailable` xem [NatsClusterDown](NatsClusterDown.md))
2. `core-api` có subscribe `ew.cmd.<module>.>` không (log khởi động), CPU `core-api`
3. NATS: `nats_consumer_num_pending`, trạng thái cụm

## Xử lý

1. Scale `core-api`; nếu không có handler sau deploy: rollback `core-api`
2. NATS mất quorum: xem [ChatJetStreamUnavailable](ChatJetStreamUnavailable.md)

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không cần gọi ngoài giờ: tạo ticket cho đội sở hữu (thẻ `module`/`service`) và theo dõi trong giờ làm việc; leo thang như critical nếu alert critical liên quan cùng bắn.

## Liên quan

- [REALTIME_ARCHITECTURE](../../06-architecture/REALTIME_ARCHITECTURE.md)
- [EVENT_DESIGN](../../07-backend/EVENT_DESIGN.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
