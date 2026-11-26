# PrometheusRuleFailures

> Runbook của alert `PrometheusRuleFailures` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/meta.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _Rule group <rule_group> fails to evaluate (alerts may be blind)_
- Điều kiện (MONITORING §5): `prometheus_rule_evaluation_failures_total` tăng

## Tác động

Rule không đánh giá được: alert có thể mù.

## Chẩn đoán

1. Prometheus → Rules: lỗi của group
2. Rule mới được deploy?

## Xử lý

1. Sửa rule (`promtool check rules`, `promtool test rules`) và deploy lại cấu hình

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
