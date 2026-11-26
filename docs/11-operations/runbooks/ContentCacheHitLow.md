# ContentCacheHitLow

> Runbook của alert `ContentCacheHitLow` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/content.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Valkey content cache hit ratio below 90% for 30 minutes in school hours_
- Điều kiện (MONITORING §5): Hit ratio Valkey nội dung < 90% trong 30 phút (giờ học)

## Tác động

Nhiều request nội dung đi xuống DB: chậm và tốn tài nguyên.

## Chẩn đoán

1. TTL, eviction Valkey

## Xử lý

1. Sửa TTL; tăng bộ nhớ Valkey

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [CONTENT_MODEL](../../02-learning/CONTENT_MODEL.md)
- [VALKEY_DESIGN](../../07-backend/VALKEY_DESIGN.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
