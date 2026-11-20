# ADR-0044 — Lưu trữ hình ảnh và media: object storage S3-compatible + CDN

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** OQ-10, OQ-12, OQ-21 · **Tài liệu:** [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md), [DEPLOYMENT](../11-operations/DEPLOYMENT.md), [LOCAL_DEVELOPMENT](../11-operations/LOCAL_DEVELOPMENT.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Không áp dụng ở version này ([ADR-0082](ADR-0082-spec-driven-development-va-noi-dung-deferred.md)):** bucket `ew-student-submissions` và ngoại lệ cho học sinh upload (ghi âm speaking, sản phẩm dự án, ảnh bài làm) — các hoạt động cần người chấm đã ngoài phạm vi (ADR-0072). **Còn hiệu lực:** toàn bộ phần còn lại.

> [!WARNING]
> **Mục 1 (staging/production) đã được thay bởi [ADR-0071](ADR-0071-object-storage-production-dung-floci.md)** (2026-11-15): floci là object storage ở mọi môi trường, kể cả staging/production; câu "floci không dùng cho staging/production" không còn hiệu lực. Các mục còn lại vẫn hiệu lực.

## Bối cảnh

Stakeholder hỏi nội dung có hình ảnh, media thì lưu ở đâu; sau đó chỉ định dùng floci cho S3.

## Quyết định

1. **Không lưu file trong PostgreSQL.** File ở **object storage tương thích S3**; PostgreSQL chỉ lưu **metadata** (`media_assets`).
   - **Staging/production:** dịch vụ S3-compatible của cloud tại Việt Nam hoặc cụm MinIO tự vận hành [TBD OQ-21].
   - **Local dev, CI, test tự động:** [floci](https://github.com/floci-io/floci) (trình giả lập AWS, MIT; image `floci/floci`, cổng 4566, `FLOCI_STORAGE_MODE=persistent`, `FLOCI_SERVICES_S3_ENFORCE_AUTH=true`). floci **không** dùng cho staging/production (theo tài liệu của dự án: dành cho dev/test/CI, không cam kết HA, mặc định không kiểm tra credential). Lifecycle rule và bucket policy test trên staging.
   - Code chỉ dùng AWS SDK S3 chuẩn với endpoint cấu hình (`EW_S3_ENDPOINT`, `EW_S3_FORCE_PATH_STYLE`), đổi môi trường không sửa code.
2. **Bucket:**

   | Bucket | Nội dung | Truy cập |
   |---|---|---|
   | `ew-media-quarantine` | File vừa upload, chờ quét/xử lý | Private, chỉ media worker |
   | `ew-media-public` | Media đã xử lý của nội dung dùng chung, gói nội dung game không nhạy cảm, SVG công thức | Qua CDN, URL có version/hash |
   | `ew-media-private` | Media nội dung riêng tenant, media đề kiểm tra | CDN với **signed URL** 5–15 phút |
   | `ew-game-bundles` | Bản build Godot, gói PCK, level package | Qua CDN |
   | `ew-student-submissions` | Ghi âm speaking, file sản phẩm dự án, ảnh bài làm toán | Private, quét virus, chỉ người chấm xem bằng signed URL, xóa theo hạn DATA_PRIVACY |
   | `ew-exports` | File xuất báo cáo, import SIS | Private, presigned URL, xóa theo hạn |

3. **Upload:**
   - **Từ Admin Portal:** presigned PUT (giới hạn loại/dung lượng) → `ew-media-quarantine` → event → **media worker** (trong worker của service sở hữu nội dung, dùng chung `pkg/media`): quét virus (ClamAV), kiểm tra magic bytes, xóa EXIF/GPS, chuyển đổi (ảnh → WebP + PNG 256/1024/2048 px; âm thanh → Ogg Vorbis cho Godot + MP3; video → Ogg Theora cho Godot 4 + HLS/MP4 cho web; mô hình 3D → glTF/GLB theo WORLD_RENDERING), lưu khóa **content-addressed** `media/<sha256>/<variant>.<ext>`.
   - **Học sinh chỉ upload trong ngoại lệ có kiểm soát:** ghi âm speaking, sản phẩm dự án, ảnh bài làm toán (`allow_photo_submission`, **tắt mặc định**), vào `ew-student-submissions`; giữ ghi âm 90 ngày sau khi chấm, sản phẩm 1 năm học [Confirmed].
4. **Bất biến:** media không sửa tại chỗ; thay ảnh = asset mới. Version nội dung cũ vẫn trỏ asset cũ (ADR-0043). Asset còn được tham chiếu không bị xóa (`media_usages`); asset mồ côi được đánh dấu để dọn theo quy trình.
5. **Metadata** `media_assets` (DB của service sở hữu): `asset_id`, `tenant_id`/`owner_scope`, `kind`, `original_filename` varchar(255), `mime_type` varchar(127), `size_bytes`, `width`, `height`, `duration_ms`, `sha256` varchar(64), `variants` jsonb, `alt_text` varchar(500), `license` varchar(255), `status` (`quarantined|processing|ready|rejected`), cột audit chuẩn.
6. **Giới hạn [Confirmed]:** ảnh ≤ 10 MB, âm thanh ≤ 20 MB, video ≤ 300 MB, mô hình 3D ≤ 50 MB; tối đa 50 asset mỗi version nội dung.
7. **Client:** Godot tải media qua CDN, cache `user://cache/media/<sha256>`; preload theo manifest trước khi vào màn.

## Hệ quả

(+) DB nhẹ, tải nhanh qua CDN, cache vĩnh viễn nhờ khóa bất biến, nội dung cũ không hỏng khi thay media. (−) Vận hành object storage + CDN + media worker + ClamAV; chi phí băng thông; signed URL cho nội dung riêng tenant; floci có thể khác S3 thật ở vài hành vi (RISKS).

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Accepted (local dev dùng MinIO) | Stakeholder |
| 2026-11-15 | floci cho local/dev/CI; production object storage chờ OQ-21 | Stakeholder |
| 2026-11-15 | Ngoại lệ upload của học sinh, bucket `ew-student-submissions` | ADR-0048, ADR-0055 |
| 2026-11-16 | Ghi chú phần không áp dụng theo ADR-0082 | Technical Architect |
