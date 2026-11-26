# SLOBurnRateSlow

> Runbook của alert `SLOBurnRateSlow` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/slo.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Availability error budget burns more than 6× (6 h and 30 m windows)_
- Điều kiện (MONITORING §5): Burn rate > 6× trong 6h

## Tác động

Error budget đang bị đốt chậm nhưng liên tục (6× trong 6 giờ).

## Chẩn đoán

1. So sánh error rate theo module/route trong 6 giờ qua (Dashboard Grafana **API**)
2. Lỗi lặp lại có cùng `code` (Problem Details)?

## Xử lý

1. Tạo ticket cho đội sở hữu module lỗi nhiều nhất
2. Cân nhắc đóng băng release cho tới khi burn rate < 1×

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Trong giờ làm việc: Technical Lead backend; không cần gọi ngoài giờ.

## Liên quan

- [MONITORING §6](../MONITORING.md#6-slo--confirmed-oq-11)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
