# APIHighErrorRate

> Runbook của alert `APIHighErrorRate` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/api.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _More than 2% 5xx on the public gateway for 5 minutes_
- Điều kiện (MONITORING §5): 5xx > 2% trong 5 phút (route public)

## Tác động

Học sinh/nhà trường gặp lỗi khi gọi REST qua gateway công khai; error budget availability bị đốt.

## Chẩn đoán

1. Dashboard Grafana **EduWorld Overview**, panel "5xx theo module" để biết module nào lỗi
2. Tempo: tìm trace có `status=error` 15 phút gần nhất; Grafana → Explore (Loki) `{service="<binary>"} | json | level="error"`, lọc theo `trace_id` lấy từ Tempo
3. APISIX: upstream nào trả 5xx (`apisix_http_status` theo `route`)

## Xử lý

1. Lỗi tập trung một module: xử lý theo [ModuleHighErrorRate](ModuleHighErrorRate.md)
2. Sau deploy: `make deploy-rollback ENV=<env> SERVICE=<swarm-service>` nếu bắt đầu sau một bản deploy ([DEPLOYMENT §11](../DEPLOYMENT.md))
3. Upstream không khỏe (`core-api` 0 replica): xem [ServiceDown](ServiceDown.md)

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [DEPLOYMENT](../DEPLOYMENT.md)
- [runbooks/README](README.md)
