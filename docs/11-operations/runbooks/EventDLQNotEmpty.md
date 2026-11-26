# EventDLQNotEmpty

> Runbook của alert `EventDLQNotEmpty` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/async.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Dead letter stream <stream> holds messages_
- Điều kiện (MONITORING §5): `nats_stream_total_messages{stream=~"DLQ_.*"} > 0`

## Tác động

Có event không xử lý được sau MaxDeliver.

## Chẩn đoán

1. Xem message trong stream `DLQ_<SERVICE>` (nats-box qua `make ops-job`), không chép payload chứa dữ liệu cá nhân ra ngoài

## Xử lý

1. Sửa nguyên nhân rồi replay theo EVENT_DESIGN

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [EVENT_DESIGN](../../07-backend/EVENT_DESIGN.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
