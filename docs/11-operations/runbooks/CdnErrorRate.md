# CdnErrorRate

> Runbook của alert `CdnErrorRate` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/edge.yaml` · Mức: **critical**.

## Triệu chứng

- Thông báo: _CDN probe <instance> fails more than 1% for 10 minutes_
- Điều kiện (MONITORING §5): CDN 5xx > 1% trong 10 phút — **tạm thời** đo bằng tỷ lệ lỗi probe blackbox một object CDN công khai (job `blackbox-cdn`) cho tới khi tích hợp metric của nhà cung cấp CDN (stand-in, AGENTS R17)

## Tác động

Học sinh không tải được nội dung/media/bản client.

## Chẩn đoán

1. Probe `blackbox-cdn` (stand-in cho metric nhà cung cấp CDN)
2. Origin object storage (`ObjectStorageDown`)

## Xử lý

1. Sửa origin; liên hệ nhà cung cấp CDN

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không khắc phục được trong 15 phút hoặc ảnh hưởng nhiều trường: gọi người trực secondary, sau đó Technical Lead của binary/module (thẻ `service`/`module` của alert). Ghi diễn biến vào kênh sự cố.

## Liên quan

- [DEPLOYMENT](../DEPLOYMENT.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
