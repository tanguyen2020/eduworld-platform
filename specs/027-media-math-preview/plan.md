# Implementation Plan: F027 — Media pipeline, công thức SVG, preview session

**Branch**: `027-media-math-preview` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Module `media` trong module `learning`: presign PUT vào `ew-media-quarantine`, xác nhận upload (`completeMediaUpload`), outbox `media.uploaded` → media worker (consumer module `learning` trong profile `worker-events`) dùng `src/backend/pkg/media` (ClamAV, magic bytes, xóa EXIF, libvips/ffmpeg) → biến thể content-addressed ở `ew-media-public`/`ew-media-private`, `media.processed`/`media.rejected`. Module `mathx` kiểm tra LaTeX allowlist + sanitize SVG từ portal và lưu `math/<sha256>.svg`, được gọi từ pipeline validate nội dung khi lưu draft. Module `preview` tạo phiên ngắn hạn trong Valkey, dựng render bằng builder của F022/F030 và chấm thử không ghi DB.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, AWS SDK v2 (s3 + presign), libvips (binding Go), ffmpeg (process), clamd client, valkey-go, `src/backend/pkg/{media,outbox,tenancy,entx,authz,problem}` |
| Storage | PostgreSQL `eduworld_learning` (`content.media_assets`, `media_usages`, `math_svgs`); floci S3 (`ew-media-quarantine`, `ew-media-public`, `ew-media-private`); Valkey (phiên preview) |
| Testing | go test + testcontainers (PostgreSQL, NATS, Valkey, floci, clamav); file mẫu EICAR, ảnh có EXIF GPS |
| Target Platform | Linux container (module `learning` trong `core-api`; image `worker` có libvips + ffmpeg cho profile `worker-events`) |
| Project Type | backend module + platform kit |
| Performance Goals | SC-003; preview p95 < 500 ms |
| Constraints | R3, R9, R23, R28, R31; ADR-0044, ADR-0055, ADR-0071 |
| Scale/Scope | `assets_per_version_max`; giới hạn dung lượng theo CONTENT_MODEL §7 |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta `completeMediaUpload`, `submitPreviewResponse`, payload `media.*` ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | `src/backend/pkg/media` không chứa nghiệp vụ; `domain/mathx` thuần; không gọi module khác |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `media_assets` có `tenant_id`/`deleted_at`; preview chỉ Valkey tạm (không dữ liệu quan trọng, R9) |
| IV Server-authoritative | ✅ | Preview chấm server-side, render không đáp án; presign/complete có Idempotency-Key + audit `media.uploaded` |
| V Xác thực, phân quyền | ✅ | `media.upload`, `learning_content.read`; token game bị chặn; endpoint S3 qua env |
| VI An toàn trẻ em | ✅ | Học sinh không upload; xóa EXIF/GPS |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số công thức theo tên; giới hạn media đăng ký mới (Polish) |
| VIII UI tách logic, token | ✅ | Không UI |
| IX Test có bằng chứng | ✅ | Test trước mỗi story; tenant + authz cho mọi endpoint mới |

## Project Structure

### Documentation (feature này)

```text
specs/027-media-math-preview/
├── spec.md · plan.md · research.md · data-model.md · quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
src/backend/pkg/media/{scanner.go,magic.go,exif.go,transcode.go,keys.go}
src/backend/modules/learning/
├── ent/schema/{media_asset.go,media_usage.go,math_svg.go}
├── internal/domain/media/                 # MediaAsset, Status, Kind, allowlist mime, Variant
├── internal/domain/mathx/                 # LatexValidator, SvgSanitizer, MathBlock
├── internal/domain/preview/               # PreviewSession
├── internal/application/command/{presign_media_upload.go,complete_media_upload.go,process_media.go,validate_math_assets.go,create_preview_session.go,submit_preview_response.go}
├── internal/application/query/{list_media_assets.go,get_media_asset.go}
├── internal/application/port/{object_storage.go,media_processor.go,preview_store.go,scorer.go}
├── internal/infrastructure/storage/{s3_storage.go,cdn_signer.go}
├── internal/infrastructure/cache/preview_store.go
├── internal/presentation/http/public/{cms_media_handler.go,cms_preview_handler.go}
└── internal/presentation/consumer/media_on_uploaded.go   # learning.media_worker
```

**Structure Decision**: Xử lý file nặng nằm trong `src/backend/pkg/media` (ADR-0044 mục 3: dùng chung cho game/module chat sau này); quyết định nghiệp vụ (trạng thái, bucket, quyền) ở module `learning`. Chi tiết công cụ: [research.md](research.md).

## Phase 0: Research

→ [research.md](research.md): R1 công cụ media, R2 sanitize SVG, R3 kiểm tra LaTeX.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `media.MediaAsset` (Quarantine → Processing → Ready/Rejected), `media.AllowedMime(kind)`, `mathx.ValidateLatex`, `mathx.SanitizeSvg`, `preview.Session` | Không I/O |
| application | Command `PresignMediaUpload`, `CompleteMediaUpload`, `ProcessMedia`, `ValidateMathAssets` (gọi từ hook lưu draft), `CreateLearningPreviewSession`, `SubmitPreviewResponse` · Query `ListMediaAssets`, `GetMediaAsset` | Permission: `media.upload`; đọc/preview `learning_content.read` |
| infrastructure | `S3Storage` (presign PUT/GET, HEAD, PUT biến thể với `Cache-Control: immutable`), `CdnSigner` (`EW_CDN_SIGNING_KEY`), `MediaProcessor` (src/backend/pkg/media), `PreviewStore` (Valkey `ew:learning:preview:<id>`), `Scorer` (port của F022) | |
| presentation | `presignMediaUpload`, `completeMediaUpload`, `listMediaAssets`, `getMediaAsset`, `createLearningPreviewSession`, `submitPreviewResponse`; consumer `learning.media_worker` | |

### Tích hợp và phụ thuộc

- Phụ thuộc: F020 (owner_scope, tenant); F022 (scorer, render builder) cho preview "thử làm".
- Event phát: `media.uploaded`, `media.processed`, `media.rejected`, `audit.recorded`. Tiêu thụ nội bộ: `media.uploaded`.
- F025 đọc `media_assets.status` (`MEDIA_NOT_READY`); F026 dùng `CdnSigner` cho snapshot private.

### Bảo mật, tenant, idempotency, audit

- `portal_staff`; `TenantScope`; platform asset chỉ role platform; System Admin cần `X-Tenant-Id`.
- Idempotency-Key cho presign, complete, preview; asset trùng `sha256` trong phạm vi → `duplicate_of`.
- Audit: `media.uploaded` (complete), `authz.denied`. Không log tên file đầy đủ ở INFO (chỉ `asset_id`).

### Quan sát

- Metric `eduworld_media_pipeline_duration_seconds{kind,result}`, `eduworld_media_rejected_total{reason}`, `eduworld_math_render_failed_total{reason}`; span `media.scan`, `media.transcode`, `math.validate`.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
