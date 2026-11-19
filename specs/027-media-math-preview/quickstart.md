# Quickstart: F027 — Media pipeline, công thức SVG, preview session

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F020 (và F022 cho bước "thử làm").
- Cấu hình (OQ F027-10, chưa nối binary — OQ F010-2/F020-3): `EW_S3_ENDPOINT`, `EW_S3_REGION`, `EW_S3_ACCESS_KEY`, `EW_S3_SECRET_KEY` (adapter `storage.S3`), `EW_CDN_BASE_URL`, `EW_CDN_SIGNING_KEY` (`storage.CdnSigner`, cùng khóa với service `cdn` của compose), `EW_CLAMAV_ADDR` (`mediaproc.New`), `EW_MEDIA_SCAN=disabled` chỉ local (`command.Media.ScanDisabled`).
- Quyền `media.upload` hiện chỉ có ở `tenant_content_editor` và `system_admin` (OQ F027-2): bước 1 dùng `sa.admin` (platform) hoặc một biên tập viên tenant.
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

## 5. Bằng chứng (2026-11-19, local, worktree `f027-learning`)

Môi trường: Windows + Docker Desktop; PostgreSQL/NATS/Valkey/floci 2.5.0 qua testcontainers (`TESTCONTAINERS_RYUK_DISABLED=true`, `GOMAXPROCS=2`); clamd là TEST DOUBLE (nhận EICAR, OQ F027-9); máy không có ffmpeg nên `TestFFmpegAudio` bỏ qua.

| Lệnh | Kết quả |
|---|---|
| `go test ./pkg/media/... ./modules/learning/...` (unit + contract) | ok — gồm `TestSniffAndCompatible`, `TestProcessImageStripsMetadata` (SC-001), `TestClamd`, `TestUploadValidate`, `TestAssetStateMachine`, `TestValidateLatex`, `TestSanitizeSVG` (SC-004), `TestTransform`, `TestLocalize`, `TestCdnSigner`, `TestPresignPutSignsSizeAndType`, `TestMathRenderPayload`, `TestMigrationMatchesEnt` |
| `go test -tags integration -run 'TestMedia|TestMath|TestPreview' ./modules/learning/tests/integration/` | 10/10 PASS: `TestMediaUploadPipeline`, `TestMediaRejections`, `TestMediaPresignRules`, `TestMediaAuthorizationAndTenants`, `TestMediaLibrary`, `TestMediaUsagesOnSave`, `TestMediaReplaceKeepsOldVersion`, `TestMathFormulas`, `TestPreviewSession`, `TestPreviewAuthorization` |
| `go test -tags integration -run TestQuestion …` (F022 hồi quy) | PASS |
| `go run ./tools/routematrix check` · `archgen --check` · `tenancyguard` · `entlint` · `ewlint` · golangci-lint (2 cấu hình) | 0 lỗi |
| `check-maxlength` · `check-enums` · `validate-schemas` · `check-cross-features` · `check-portal-catalog` · `check-features` | 0 lỗi |

Ánh xạ tiêu chí: AC-01/03/05/06 `TestMediaUploadPipeline`; AC-02 `TestMediaPresignRules`; AC-03/04 `TestMediaRejections`; AC-07 `TestMediaAuthorizationAndTenants`; AC-08 `TestMediaReplaceKeepsOldVersion`; AC-09/10 `TestMediaLibrary`; AC-11 `TestMediaUsagesOnSave`; AC-12/13/15 `TestMathFormulas` + `TestValidateLatex`/`TestSanitizeSVG`; AC-14 `TestMathRenderPayload`; AC-16/17/18 `TestPreviewSession`; AC-19 `TestPreviewAuthorization`.

Chưa chạy (cần môi trường compose đầy đủ): bước 2 tải biến thể qua CDN thật, bước 5 với ClamAV thật, bước 7 URL hết hạn qua nginx `secure_link`, bước 13 qua F030 (chưa có), trace_id (binary chưa nối). SC-003 (30 giây) chưa đo.
