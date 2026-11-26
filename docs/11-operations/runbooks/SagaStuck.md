# SagaStuck

> Runbook của alert `SagaStuck` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/identity.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Saga <saga> stuck for 30 minutes_
- Điều kiện (MONITORING §5): `eduworld_saga_stuck > 0` quá 30 phút

## Tác động

Saga (clone tenant, dọn học sinh, khôi phục thùng rác) kẹt > 30 phút.

## Chẩn đoán

1. Bước lỗi của saga (log module identity, `saga.id`)

## Xử lý

1. Chạy lại bước hoặc bù trừ theo thiết kế saga

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
