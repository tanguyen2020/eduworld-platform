# DayCloseNotDone

> Runbook của alert `DayCloseNotDone` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/dashboard.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _<job> has not succeeded for yesterday by 05:00 ICT_
- Điều kiện (MONITORING §5): Job `dashboard-day-close` chưa thành công cho ngày hôm qua lúc 05:00 ICT

## Tác động

Dashboard hiển thị "Đang tính" cho ngày hôm qua.

## Chẩn đoán

1. Log job `dashboard-day-close*`

## Xử lý

1. Chạy lại job

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [DASHBOARD_DATA](../../07-backend/DASHBOARD_DATA.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
