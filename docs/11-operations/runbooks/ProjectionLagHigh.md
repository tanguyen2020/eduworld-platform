# ProjectionLagHigh

> Runbook của alert `ProjectionLagHigh` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/async.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Projection <projection> of module <module> is more than 60 s behind_
- Điều kiện (MONITORING §5): `eduworld_projection_lag_seconds` > 60 s trong 10 phút

## Tác động

Dữ liệu tham chiếu (tenant/học sinh) ở module nhận bị cũ > 60 s.

## Chẩn đoán

1. `eduworld_projection_lag_seconds` theo projection
2. Consumer của projection

## Xử lý

1. Như [EventConsumerLag](EventConsumerLag.md)

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không cần gọi ngoài giờ: tạo ticket cho đội sở hữu (thẻ `module`/`service`) và theo dõi trong giờ làm việc; leo thang như critical nếu alert critical liên quan cùng bắn.

## Liên quan

- [EVENT_DESIGN](../../07-backend/EVENT_DESIGN.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
