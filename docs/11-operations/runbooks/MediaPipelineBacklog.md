# MediaPipelineBacklog

> Runbook của alert `MediaPipelineBacklog` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/content.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _More than 200 media jobs pending for 15 minutes_
- Điều kiện (MONITORING §5): `eduworld_media_queue_pending` > 200 trong 15 phút

## Tác động

Media mới tải lên chưa sẵn sàng.

## Chẩn đoán

1. `eduworld_media_queue_pending`, lỗi job media

## Xử lý

1. Tăng replica `worker-events` (hoặc tách profile media)

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
