# ADR-0093 — Object storage tự host SeaweedFS trước, chuyển S3 managed bằng cấu hình

> **Trạng thái:** Accepted · một phần supersedes ADR-0075 (mục 2) · **Ngày:** 2026-11-20 · **OQ:** OQ-10, OQ-21 · **Tài liệu:** [DEPLOYMENT §9b](../11-operations/DEPLOYMENT.md#9b-object-storage-và-media-pipeline-adr-0044-adr-0043) · [BACKUP_RECOVERY §6](../11-operations/BACKUP_RECOVERY.md#6-object-storage) · [CONFIG_DEFAULTS](../00-overview/CONFIG_DEFAULTS.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

[ADR-0075](ADR-0075-object-storage-production-khong-dung-floci.md) mục 2 cho staging/production hai phương án: S3-compatible managed tại Việt Nam hoặc cụm Garage/SeaweedFS tự host ≥ 3 node. Sáng 2026-11-20 owner chọn S3 managed. Cùng ngày owner đổi quyết định: ngân sách hiện tại chưa có khoản chi cho dịch vụ S3 managed, cần chạy trên storage tự host và chuyển sang S3 managed khi có ngân sách.

Mã nguồn đã sẵn sàng cho việc đổi backend: adapter storage của module `identity` và `learning` ký request bằng AWS Signature V4, URL kiểu path-style, chỉ dùng thao tác S3 chuẩn (PUT/GET/HEAD, presigned PUT/GET), endpoint và credential đọc từ `EW_S3_ENDPOINT`, `EW_S3_REGION`, `EW_S3_ACCESS_KEY`, `EW_S3_SECRET_KEY` (R3). Khóa media là `media/<sha256>/…` bất biến (ADR-0044, R31), nên sao chép sang backend khác có thể làm tăng dần và kiểm tra bằng checksum.

## Quyết định

1. Local và CI giữ floci (ADR-0044, ADR-0075 mục 1).
2. **Staging/production dùng SeaweedFS tự host** (S3 gateway của SeaweedFS), giấy phép Apache-2.0:
   - Production: cụm **3 node `storage`**, replication 3 bản đặt trên 3 node khác nhau (mất 1 node không mất dữ liệu, không gián đoạn); master chạy 3 instance (quorum) cùng 3 node đó.
   - Staging: 1 node, không replication (dữ liệu staging dựng lại được).
   - Bản sao **ngoài cụm, khác site, trong lãnh thổ Việt Nam** hằng ngày; bucket riêng tư replication bất đồng bộ sang site thứ hai với độ trễ ≤ 5 phút (giữ yêu cầu RPO của ADR-0075 mục 3).
   - Chọn SeaweedFS thay Garage vì SeaweedFS hỗ trợ đầy đủ hơn các tính năng S3 mà [BACKUP_RECOVERY §6](../11-operations/BACKUP_RECOVERY.md#6-object-storage) yêu cầu (versioning, lifecycle hết hạn, replication giữa cụm). Garage không có versioning và object lock [Ngoài — cần xác minh trong spike].
3. **Chuyển sang S3 managed chỉ bằng cấu hình + di chuyển dữ liệu**, không sửa mã:
   - Code không dùng API riêng của SeaweedFS; mọi tính năng mới của storage phải dùng API S3 chuẩn mà cả SeaweedFS và nhà cung cấp S3 managed tại Việt Nam đều hỗ trợ (kiểm bằng floci ở CI).
   - Quy trình chuyển (runbook RB-05b ở BACKUP_RECOVERY): tạo bucket + policy + lifecycle ở đích → `rclone sync` tăng dần từ SeaweedFS sang đích, kiểm checksum khóa `media/<sha256>/…` → trong `maintenance_window`, dừng ghi (portal upload, export), sync lần cuối → đổi `EW_S3_*` và origin của CDN → redeploy `core-api`, `worker` → kiểm tra presigned PUT/GET và tải media qua CDN → giữ cụm SeaweedFS chỉ đọc 30 ngày rồi mới gỡ.
   - Nếu nhà cung cấp đích không hỗ trợ URL path-style, thêm cấu hình chọn virtual-hosted style trong adapter (thay đổi nhỏ, có test) trước khi chuyển.
4. **Spike nghiệm thu SeaweedFS trước pilot** (thay spike nhà cung cấp managed của ADR-0075 mục 4): presigned PUT/GET với SigV4 (kể cả ký `Content-Length`/`Content-Type` như adapter đang làm), multipart, versioning, lifecycle (quarantine 7 ngày), bucket policy, signed URL qua CDN, replication sang site thứ hai, đo hiệu năng upload/tải, diễn tập khôi phục RB-05. Tính năng nào SeaweedFS không đạt thì ghi biện pháp bù ở BACKUP_RECOVERY (ví dụ: không có object lock → bản sao ngoài site giữ theo chính sách retention, quyền ghi tách khỏi quyền xóa).
5. Khi có ngân sách cho S3 managed tại Việt Nam, quyết định chuyển do owner chốt và ghi vào lịch sử ADR này; không cần ADR mới vì cách chuyển đã định ở mục 3.

## Hệ quả

(+) Không phát sinh chi phí dịch vụ managed ở giai đoạn đầu; dữ liệu nằm tại Việt Nam; đổi sang S3 managed không sửa mã.
(−) Đội vận hành tự chịu HA, nâng cấp, giám sát và backup của cụm SeaweedFS (RISK-38); cần 3 node `storage` và một vị trí lưu bản sao khác site; tính năng object lock có thể không có, phải bù bằng quy trình backup.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-20 | Tạo và Accepted: tự host SeaweedFS do chưa có ngân sách S3 managed; chuyển S3 managed bằng cấu hình (thay ADR-0075 mục 2, mục 4) | Owner |
