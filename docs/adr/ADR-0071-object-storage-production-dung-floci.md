# ADR-0071 — Object storage cho staging/production dùng floci

> **Trạng thái:** Accepted (rủi ro đã được người duyệt chấp nhận) · Superseded by ADR-0075 · **Ngày:** 2026-11-15 · **OQ:** OQ-21 · **Thay thế một phần:** ADR-0044 mục 1 (staging/production; câu "floci không dùng cho staging/production") · **Tài liệu:** [DEPLOYMENT](../11-operations/DEPLOYMENT.md) · [BACKUP_RECOVERY](../11-operations/BACKUP_RECOVERY.md) · [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Đã được thay bởi [ADR-0075](ADR-0075-object-storage-production-khong-dung-floci.md):** staging/production không dùng floci; floci chỉ dùng ở local và CI.

## Bối cảnh

ADR-0044 dùng floci làm S3 cho local/dev/CI và để ngỏ object storage staging/production (OQ-21). Người duyệt quyết định: **staging và production cũng lưu trữ trên floci**.

Theo tài liệu của dự án floci, đây là trình giả lập AWS hướng tới dev/test/CI: không cam kết tính sẵn sàng cao (HA), không có sao chép dữ liệu giữa nhiều node, mặc định không kiểm tra credential. Object storage giữ media học tập, media riêng tenant và file export, nên phần quyết định dưới đây kèm các biện pháp bắt buộc để bù rủi ro.

## Quyết định

1. **floci là object storage của mọi môi trường** (local, CI, staging, production) [Confirmed].
2. **Cấu hình bắt buộc ở staging/production:**
   - `FLOCI_STORAGE_MODE=persistent`, dữ liệu trên volume riêng của node lưu trữ (Docker Swarm, ràng buộc `node.labels.storage == true`), ổ có RAID hoặc tương đương.
   - `FLOCI_SERVICES_S3_ENFORCE_AUTH=true`; credential riêng cho từng service, lấy từ Docker secret (R3, R4); không mở cổng floci ra Internet. Chỉ mạng nội bộ Swarm truy cập; người dùng tải media qua CDN/APISIX và signed URL như ADR-0044.
   - Phiên bản image floci ghim theo digest; nâng cấp có kiểm thử trên staging trước.
3. **Bảo vệ dữ liệu:**
   - Backup object storage riêng: đồng bộ định kỳ volume/bucket sang vị trí lưu trữ khác máy (ít nhất hàng ngày, giữ theo chính sách trong BACKUP_RECOVERY) và diễn tập khôi phục định kỳ.
   - Khóa media bất biến `media/<sha256>/…` (ADR-0044) cho phép kiểm tra toàn vẹn khi khôi phục bằng checksum.
   - Theo dõi: health check, dung lượng volume, tỷ lệ lỗi và độ trễ S3 từ phía service (metric của adapter S3).
4. **Giữ khả năng chuyển đổi:** code chỉ dùng AWS SDK S3 chuẩn với endpoint cấu hình (`EW_S3_ENDPOINT`, `EW_S3_FORCE_PATH_STYLE`), không dùng tính năng riêng của floci. Chuyển sang S3-compatible khác (cloud tại Việt Nam, MinIO cụm) chỉ là đổi cấu hình và di chuyển dữ liệu, cần ADR mới.
5. **Điều kiện xem xét lại** (ghi ở RISKS): mất dữ liệu hoặc gián đoạn object storage ở staging/production; độ trễ hoặc lỗi S3 vượt ngưỡng giám sát; dung lượng vượt khả năng một node; floci thay đổi giấy phép hoặc ngừng phát triển.

## Hệ quả

(+) Một công cụ duy nhất cho mọi môi trường, cấu hình đơn giản, chi phí thấp, hành vi local giống production. (−) Một điểm lỗi đơn (single point of failure) cho media; khôi phục phụ thuộc backup; floci không được thiết kế cho production nên rủi ro về độ bền dữ liệu và bảo mật cao hơn dịch vụ object storage chuyên dụng — rủi ro đã được người duyệt chấp nhận, theo dõi trong RISKS.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR; đóng OQ-21 | Stakeholder (người duyệt) |
| 2026-11-17 | Superseded by ADR-0075 | Technical Architect |
