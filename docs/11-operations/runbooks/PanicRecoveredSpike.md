# PanicRecoveredSpike

> Runbook của alert `PanicRecoveredSpike` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/api.yaml` · Mức: **warning / critical**.

## Triệu chứng

- Thông báo: _More than 10 recovered panics in 5 minutes in module <module> (<service>)_
- Điều kiện (MONITORING §5): `increase(eduworld_panics_recovered_total[5m]) > 10` cho một `module`

## Tác động

Lỗi lập trình trong module; mức critical khi đi kèm vòng restart `core-api` (mọi module bị ảnh hưởng).

## Chẩn đoán

1. Issue GlitchTip theo thẻ `module`/`entrypoint` (có `trace_id`)
2. Trace trong Tempo và log cùng `trace_id`

## Xử lý

1. Tắt tính năng bằng feature flag hoặc `make deploy-rollback ENV=<env> SERVICE=<swarm-service>` nếu bắt đầu sau một bản deploy ([DEPLOYMENT §11](../DEPLOYMENT.md))
2. Ticket GitHub tự tạo bởi `glitchtip-bridge` cho đội sở hữu module

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [DEPLOYMENT](../DEPLOYMENT.md)
- [runbooks/README](README.md)
