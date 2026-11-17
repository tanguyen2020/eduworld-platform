# Quickstart: F027 — Media pipeline, công thức SVG, preview session

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F020 (và F022 cho bước "thử làm").
- Lệnh: `make up` (gồm `floci`, `s3-init`, `cdn`, `clamav` — profile `media`), `make migrate`, `make seed`, `make run-core-api`, `make run-worker PROFILE=events` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)); CI bật quét virus (không đặt `EW_MEDIA_SCAN=disabled`).
- Tài khoản demo: `cm.noidung` (content_admin), `cm.duyet` (content_reviewer), `qlcm.truong.a` (academic_manager), `ad.truong.b` (school_admin tenant khác), `hs.an.3a` (học sinh), `sa.admin`.
- File mẫu: `photo-gps.jpg` (có EXIF GPS), `eicar.txt` đổi tên `.png`, `fake.png` (nội dung PDF), `read-aloud.mp3`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `cm.noidung` presign `photo-gps.jpg` (có `alt_text`) → PUT → `completeMediaUpload` | `quarantined` → `processing` → `ready`; outbox `media.uploaded`, `media.processed` | F027-AC-01, F027-AC-03, F027-AC-05 |
| 2 | Tải biến thể `webp_1024`, đọc EXIF | Không có EXIF/GPS; header `Cache-Control: immutable`; key `media/<sha256>/…` | F027-AC-05 |
| 3 | Presign lại cùng file | `duplicate_of` = asset bước 1, không có `upload_url` mới | F027-AC-01 |
| 4 | Presign `mime_type=application/x-msdownload`; ảnh 11 MB; ảnh không `alt_text` | `415`; `413`; `422` | F027-AC-02 |
| 5 | Upload `eicar.txt` khai `image/png`; upload `fake.png` | `rejected` `virus`; `rejected` `type_mismatch` | F027-AC-04 |
| 6 | Complete với `sha256` sai | `rejected` | F027-AC-03 |
| 7 | `qlcm.truong.a` upload ảnh tenant → `getMediaAsset` | `bucket = ew-media-private`, URL signed; mở URL sau khi hết hạn → bị từ chối | F027-AC-06 |
| 8 | `ad.truong.b` đọc asset bước 7; `qlcm.truong.a` presign `owner_scope=platform`; token game của `hs.an.3a` gọi presign | `404`; `403`; `403 PERMISSION_DENIED` | F027-AC-07 |
| 9 | Câu hỏi v1 dùng ảnh A; v2 thay bằng ảnh B; xem v1 và v2 | v1 vẫn ảnh A, v2 ảnh B | F027-AC-08 |
| 10 | `listMediaAssets?kind=image&status=rejected&q=eicar`; `getMediaAsset` ảnh A | Lọc đúng; `used_in` liệt kê version | F027-AC-09, F027-AC-10 |
| 11 | Lưu version tham chiếu `assets_per_version_max`+1 asset | `422 VALIDATION_FAILED` | F027-AC-11 |
| 12 | Lưu câu hỏi `\frac{3}{4}` + SVG MathJax 4; lưu `\href{…}`; SVG có `<script>`; SVG > `math_svg_max` | `math/<sha256>.svg` được lưu, đã sanitize; 3 lần `422 MATH_RENDER_FAILED` có pointer | F027-AC-12, F027-AC-13 |
| 13 | `hs.an.3a` mở activity có công thức (qua F030) | Payload có `math_ref` + `speech_text`, không cần MathJax | F027-AC-14 |
| 14 | Publish v1 có công thức, nâng MathJax, lưu v2 cùng LaTeX | SVG của v1 không đổi | F027-AC-15 |
| 15 | `cm.duyet` preview draft v2 (`display_language=en`), thử làm đúng/sai | Render không đáp án; kết quả có giải thích/phản hồi; không có dòng `assessment.attempts`, `learning.learning_events`, outbox học tập mới | F027-AC-16, F027-AC-17, F027-AC-18 |
| 16 | Thử làm sau khi phiên hết hạn; `ad.truong.b` preview nội dung tenant khác | `404`; `404` | F027-AC-18, F027-AC-19 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=learning
make test MOD=learning
make test-integration MOD=learning   # floci + clamav + PostgreSQL + Valkey
make migrate-lint MOD=learning
make specs-lint
```

## 4. Bằng chứng cần nộp

- Số test pass/fail theo F027-AC-*, trace_id bước 1 và 12, kết quả quét EXIF (SC-001) và quét SVG (SC-004).
