# ContentPublishSlow

> Runbook của alert `ContentPublishSlow` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/content.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _p95 publish → visible above 30 s for 15 minutes (publish_to_visible_max)_
- Điều kiện (MONITORING §5): p95 publish → visible > 30 s trong 15 phút

## Tác động

Publish/rollback nội dung hiển thị chậm > 30 s (`publish_to_visible_max`).

## Chẩn đoán

1. Snapshot builder (`eduworld_content_snapshot_build_seconds`), consumer lag

## Xử lý

1. Scale `worker-events`; sửa consumer

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [CONTENT_MODEL](../../02-learning/CONTENT_MODEL.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
