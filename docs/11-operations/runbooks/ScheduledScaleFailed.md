# ScheduledScaleFailed

> Runbook của alert `ScheduledScaleFailed` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/swarm.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _ops-scheduler failed to scale <service> on the timetable_
- Điều kiện (MONITORING §5): `eduworld_ops_scale_actions_total{result="error"}` tăng hoặc replica chưa đạt mức đỉnh lúc bắt đầu khung cao điểm

## Tác động

Không đạt mức replica đỉnh theo lịch: giờ cao điểm có thể quá tải.

## Chẩn đoán

1. Log `ops-scheduler`, `eduworld_ops_scale_actions_total{result}`
2. `docker-api-proxy` khỏe? năng lực node đủ?

## Xử lý

1. Chạy `docker service scale` thủ công cho service lỗi

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [DEPLOYMENT](../DEPLOYMENT.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
