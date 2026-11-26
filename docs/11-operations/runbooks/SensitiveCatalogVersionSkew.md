# SensitiveCatalogVersionSkew

> Runbook của alert `SensitiveCatalogVersionSkew` (F131). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/chat.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _Nodes run different sensitive-word catalog versions (<scope>) for 5 minutes_
- Điều kiện (MONITORING §5): Các node chạy `catalog_version` khác nhau > 5 phút

## Tác động

Các node dùng danh mục từ nhạy cảm khác version: kiểm duyệt không nhất quán.

## Chẩn đoán

1. Pub/sub `ew:chat:sensitive:version`, lỗi reload

## Xử lý

1. Buộc reload; khởi động lại node lệch

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [CHAT_SYSTEM](../../03-social-multiplayer/CHAT_SYSTEM.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
