# ADR-0075 — Object storage staging/production không dùng floci

> **Trạng thái:** Accepted · supersedes ADR-0071 · mục 2 và mục 4 superseded by [ADR-0093](ADR-0093-object-storage-tu-host-seaweedfs-truoc-chuyen-s3-managed-bang-cau-hinh.md) · **Ngày:** 2026-11-15 · **OQ:** OQ-10, OQ-21 · **Nếu Accepted sẽ thay:** ADR-0071 · **Tài liệu:** [DEPLOYMENT](../11-operations/DEPLOYMENT.md) · [BACKUP_RECOVERY](../11-operations/BACKUP_RECOVERY.md) · [EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

[ADR-0071](ADR-0071-object-storage-production-dung-floci.md) dùng floci (công cụ giả lập AWS) làm object storage ở mọi môi trường, kể cả production, và tự ghi nhận là điểm lỗi đơn cho media. RPO media 24 giờ, trong khi bucket `ew-student-submissions` (bài nộp, ghi âm của học sinh) cũng nằm trên đó. Theo hiểu biết bên ngoài, floci định vị cho phát triển, kiểm thử và CI [Ngoài — cần xác minh]. Ứng dụng đã thiết kế để đổi storage chỉ bằng `EW_S3_ENDPOINT` (ADR-0071 mục 4).

## Quyết định

1. Local và CI giữ floci (ADR-0044).
2. Staging/production dùng một trong hai; owner chốt 2026-11-20 phương án **ưu tiên** [Confirmed]:
   > **Mục 2 và mục 4 đã được thay bởi [ADR-0093](ADR-0093-object-storage-tu-host-seaweedfs-truoc-chuyen-s3-managed-bang-cau-hinh.md)** (owner 2026-11-20): staging/production dùng SeaweedFS tự host, chuyển S3 managed bằng cấu hình khi có ngân sách.
   - **Ưu tiên (đã chọn sáng 2026-11-20, sau đó thay bởi ADR-0093):** dịch vụ S3-compatible managed của nhà cung cấp cloud tại Việt Nam, có replication, SLA và dữ liệu lưu tại Việt Nam. Nhà cung cấp chọn khi ký hợp đồng.
   - **Tự host (ADR-0093 chọn SeaweedFS):** cụm Garage hoặc SeaweedFS tối thiểu 3 node, có replication và backup ngoài máy — chỉ quay lại bằng ADR mới nếu không nhà cung cấp managed nào pass spike.
3. Bucket chứa dữ liệu học sinh (`ew-student-submissions`, `ew-media-private`) có RPO bằng dữ liệu học tập (BACKUP_RECOVERY).
4. Spike trước pilot (nghiệm thu nhà cung cấp managed): tương thích presigned URL, versioning, lifecycle, bucket policy; đo hiệu năng; diễn tập khôi phục.

## Hệ quả

(+) Bỏ điểm lỗi đơn cho media và dữ liệu học sinh; RPO nhất quán. (−) Chi phí dịch vụ hoặc thêm node; local khác production về triển khai (cùng giao diện S3).

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR (Proposed) từ EDU_FIT_REVIEW | Technical Architect |
| 2026-11-17 | Accepted | Technical Architect |
| 2026-11-20 | Chọn phương án ưu tiên (S3 managed tại Việt Nam) theo cơ chế mục 2; spike thành nghiệm thu nhà cung cấp (OQ-21) | Owner |
| 2026-11-20 | Mục 2, mục 4 superseded by ADR-0093: tự host SeaweedFS do chưa có ngân sách S3 managed | Owner |
