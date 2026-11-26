# ObjectStorageNodeDown

> Runbook của alert `ObjectStorageNodeDown` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/storage.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Fewer than 3 healthy object storage nodes for 5 minutes_
- Điều kiện (MONITORING §5): Cụm tự host (ADR-0075): < 3 node `storage` khỏe trong 5 phút

## Tác động

Cụm tự host còn < 3 node khỏe: giảm dư thừa.

## Chẩn đoán

1. Volume server nào down (`up{job="seaweedfs-volume"}`)

## Xử lý

1. Thay node, theo dõi rebalance (RB-05 bước 1)

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không cần gọi ngoài giờ: tạo ticket cho đội sở hữu (thẻ `module`/`service`) và theo dõi trong giờ làm việc; leo thang như critical nếu alert critical liên quan cùng bắn.

## Liên quan

- [BACKUP_RECOVERY](../BACKUP_RECOVERY.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
