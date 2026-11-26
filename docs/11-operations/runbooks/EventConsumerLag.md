# EventConsumerLag

> Runbook của alert `EventConsumerLag` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/async.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Consumer <consumer> of <stream> falls behind_
- Điều kiện (MONITORING §5): `nats_consumer_num_pending` của consumer `EVT_*` > 10.000 hoặc tăng liên tục 10 phút

## Tác động

Consumer event `EVT_*` chậm: dữ liệu phản ứng (XP, projection) bị trễ.

## Chẩn đoán

1. `nats_consumer_num_pending` theo stream/consumer, `eduworld_events_consumed_total{result}`
2. Lỗi lặp trong consumer (log, DLQ)

## Xử lý

1. Scale `worker-events`; sửa consumer lỗi rồi để nó bắt kịp

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không cần gọi ngoài giờ: tạo ticket cho đội sở hữu (thẻ `module`/`service`) và theo dõi trong giờ làm việc; leo thang như critical nếu alert critical liên quan cùng bắn.

## Liên quan

- [EVENT_DESIGN](../../07-backend/EVENT_DESIGN.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
