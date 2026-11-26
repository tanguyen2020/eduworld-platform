# ModuleHighErrorRate

> Runbook của alert `ModuleHighErrorRate` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/api.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _Module <module> answers more than 2% 5xx (<service>)_
- Điều kiện (MONITORING §5): 5xx của một module (label `module`) > 2% trong 5 phút

## Tác động

Route của module trả 5xx; `core-api` chạy cả 5 module nên lỗi có thể do một module.

## Chẩn đoán

1. Dashboard Grafana **Module** với biến `module` — route và handler lỗi
2. Panic gần nhất: `eduworld_panics_recovered_total{module}` và issue GlitchTip theo thẻ `module`
3. Grafana → Explore (Loki) `{service="<binary>"} | json | level="error"`, lọc theo `trace_id` lấy từ Tempo

## Xử lý

1. Tắt tính năng lỗi bằng feature flag nếu có
2. Bản deploy mới (kể cả canary `core-api-canary` — canary tự hủy khi alert này pending): `make deploy-rollback ENV=<env> SERVICE=<swarm-service>` nếu bắt đầu sau một bản deploy ([DEPLOYMENT §11](../DEPLOYMENT.md)) (lùi cả 5 module)
3. Database module lỗi: xem [ModuleDatabaseUnavailable](ModuleDatabaseUnavailable.md)

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [DEPLOYMENT](../DEPLOYMENT.md)
- [runbooks/README](README.md)
