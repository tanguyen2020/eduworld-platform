# PrometheusTargetSampleLimit

> Runbook của alert `PrometheusTargetSampleLimit` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/meta.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Scrapes of <job> exceed sample_limit (label cardinality too high)_
- Điều kiện (MONITORING §5): `prometheus_target_scrapes_exceeded_sample_limit_total` tăng

## Tác động

Scrape của target bị cắt vì vượt `sample_limit`: mất metric của target đó.

## Chẩn đoán

1. Metric mới có nhãn cardinality cao (`topk(10, count by (__name__) ({job="<job>"}))`)

## Xử lý

1. Sửa code phát metric (bỏ nhãn cardinality cao); không nâng `sample_limit` thay cho sửa

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Ticket cho đội sở hữu module.

## Liên quan

- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
