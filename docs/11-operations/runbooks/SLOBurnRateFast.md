# SLOBurnRateFast

> Runbook của alert `SLOBurnRateFast` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/slo.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _Availability error budget burns more than 14.4× (1 h and 5 m windows)_
- Điều kiện (MONITORING §5): Burn rate availability > 14.4× trong 1h (multiwindow)

## Tác động

Error budget tháng (99,5% giờ học) sẽ hết trong vài giờ nếu tiếp tục.

## Chẩn đoán

1. Dashboard Grafana **EduWorld Overview**: error rate, module lỗi
2. Alert đi kèm: `APIHighErrorRate`, `ModuleHighErrorRate`, `ServiceDown`

## Xử lý

1. Xử lý theo alert gốc (thường là `ModuleHighErrorRate` hoặc `ServiceDown`)
2. Sau deploy: `make deploy-rollback ENV=<env> SERVICE=<swarm-service>` nếu bắt đầu sau một bản deploy ([DEPLOYMENT §11](../DEPLOYMENT.md))
3. Ghi sự cố; error budget hết → đóng băng tính năng mới (MONITORING §6)

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [MONITORING §6](../MONITORING.md#6-slo--confirmed-oq-11)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
