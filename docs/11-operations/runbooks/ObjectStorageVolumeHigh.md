# ObjectStorageVolumeHigh

> Runbook của alert `ObjectStorageVolumeHigh` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/storage.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Object storage node <node> uses more than 80% of its disk_
- Điều kiện (MONITORING §5): Dung lượng cụm/hạn mức bucket > 80%

## Tác động

Dung lượng storage > 80%.

## Chẩn đoán

1. Dung lượng theo bucket, object mồ côi

## Xử lý

1. Mở rộng dung lượng, rà soát object mồ côi

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [BACKUP_RECOVERY](../BACKUP_RECOVERY.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
