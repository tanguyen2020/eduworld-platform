# Tasks: F027 — Media pipeline, công thức SVG, preview session

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Tạo module Go `src/backend/pkg/media` (Scanner, MagicSniffer, ExifStripper, Transcoder, Keys) — file: `src/backend/pkg/media/media.go` — ✅ 2026-11-19: `pkg/media/{media,scanner,magic,image,ffmpeg}.go` + `mediatest` (TEST FIXTURE); `go test ./pkg/media/...` ok (TestSniffAndCompatible, TestProcessImageStripsMetadata, TestClamd, TestFFmpegJobs; TestFFmpegAudio skip khi không có ffmpeg)
- [x] T002 [P] Image `worker` (`src/backend/cmd/worker`, profile `events`) có libvips + ffmpeg; compose profile `media` (clamav) cho integration test — file: `deploy/docker/worker.Dockerfile`, `deploy/compose/docker-compose.yml` — ✅ 2026-11-19: `deploy/docker/worker.Dockerfile` runtime Alpine + ffmpeg (ảnh xử lý Go thuần, OQ F027-1); compose profile `media` (clamav) có sẵn; CDN `secure_link` cho `ew-media-private` (`deploy/cdn/nginx.conf`, template envsubst)
- [ ] T003 [P] Cấu hình S3/CDN (`EW_S3_ENDPOINT`, `EW_S3_FORCE_PATH_STYLE`, `EW_S3_BUCKET_MEDIA`, `EW_CDN_SIGNING_KEY`, `EW_MEDIA_SCAN`) trong `.env.example` — file: `src/backend/modules/learning/.env.example` — ✅ 2026-11-19: KHÔNG làm — `.env*` bị chặn với agent (OQ F027-10); biến cấu hình ghi ở quickstart §1

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta OpenAPI: `completeMediaUpload`, `submitPreviewResponse`, `alt_text`/`is_exam_material` trong `MediaPresignRequest`, `license` `maxLength` 255 (D17) — file: `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=learning` — ✅ 2026-11-19: `completeMediaUpload`, `submitPreviewResponse` (+ `PreviewResponseRequest/Result`), `is_exam_material`, `license` 255, `duplicate_of` không URL, `math_assets[].svg`; `split-public.mjs`, `check-maxlength` 0 vi phạm, `check-portal-catalog` 0 lỗi, portal `gen:api`
- [x] T005 [P] Payload `media.uploaded/processed/rejected.v1` — file: `specs/events/payloads/learning/` — kiểm: `make specs-lint` — ✅ 2026-11-19: `specs/events/payloads/learning/media.{uploaded,processed,rejected}.v1.schema.json`; `validate-schemas` 0 lỗi, `check-cross-features` 0 lỗi
- [x] T006 ent schema `MediaAsset` (cột mới), `MediaUsage` (+`tenant_id`), `MathSvg` — file: `src/backend/modules/learning/ent/schema/` — kiểm: `make migrate-diff MOD=learning name=f027_media`, `make migrate-lint MOD=learning` — ✅ 2026-11-19: `ent/schema/media.go` (MediaAsset, MediaUsage, MathSvg) + migration `20261126000001_f027_media.sql` (CHECK, unique sha256 theo phạm vi, RLS H, `_live`, REVOKE UPDATE math_svgs), golden `TestMigrationMatchesEnt` ok, `atlas.sum` sinh lại
- [x] T007 Cập nhật DATABASE_DESIGN §3.13 (`quarantine_key`, `is_exam_material`, `upload_completed_at`, `processed_at`, `media_usages.tenant_id`) — file: `docs/07-backend/DATABASE_DESIGN.md` — ✅ 2026-11-19: DATABASE_DESIGN §2 bảng + §3.13 DDL cập nhật
- [x] T008 Port `ObjectStorage`, `MediaProcessor`, `PreviewStore`, `Scorer` + adapter S3 — file: `src/backend/modules/learning/internal/infrastructure/storage/s3_storage.go` — ✅ 2026-11-19: `port/media.go` (ObjectStorage, URLSigner, MediaProcessor, PreviewStore), `storage/s3_storage.go` (SigV4, retry, circuit breaker, `EnsureBucket`, `Stat`), `TestPresignPutSignsSizeAndType`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Upload media an toàn (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [x] T009 [P] [US1] Unit test allowlist mime/size/alt_text + state machine asset (F027-AC-01, F027-AC-02) — file: `src/backend/modules/learning/internal/domain/media/asset_test.go` — phải FAIL trước khi code — ✅ 2026-11-19: `TestUploadValidate`, `TestBucketFor`, `TestAssetStateMachine` ok
- [x] T010 [P] [US1] Integration test presign + dedupe `sha256` + complete khớp/không khớp (F027-AC-01, F027-AC-03) — file: `src/backend/modules/learning/tests/integration/media/presign_complete_test.go` — ✅ 2026-11-19: `TestMediaUploadPipeline`, `TestMediaRejections` (sha sai, chưa PUT → 422) ok — file `tests/integration/media_upload_test.go` (OQ F027-14)
- [x] T011 [P] [US1] Integration test worker: EICAR, sai magic bytes, lỗi transcode → `rejected` (F027-AC-04) — file: `src/backend/modules/learning/tests/integration/media/worker_reject_test.go` — ✅ 2026-11-19: `TestMediaRejections` (EICAR → virus, PDF khai PNG → type_mismatch, PNG hỏng → transcode_failed, clamd down → giữ processing) ok
- [x] T012 [P] [US1] Integration test xóa EXIF/GPS, biến thể, khóa content-addressed, `immutable` (F027-AC-05) — file: `src/backend/modules/learning/tests/integration/media/worker_variants_test.go` — ✅ 2026-11-19: `TestMediaUploadPipeline` (6 biến thể không EXIF/GPS, khóa `media/<sha256>/…`, `Cache-Control: immutable`) ok
- [x] T013 [P] [US1] Integration test bucket public/private + signed URL hết hạn (F027-AC-06) — file: `src/backend/modules/learning/tests/integration/media/signed_url_test.go` — ✅ 2026-11-19: `TestMediaUploadPipeline` (private signed URL, hết hạn bị `Verify` từ chối; platform → public) + `TestCdnSigner` ok
- [x] T014 [P] [US1] Test tenant isolation + authorization presign/complete/get (tenant khác, platform scope, token game) (F027-AC-07) — file: `src/backend/modules/learning/tests/integration/media/authz_tenant_test.go` — ✅ 2026-11-19: `TestMediaAuthorizationAndTenants` (+ `authztest.Cover` presign/complete/get/list) ok
- [x] T015 [P] [US1] Integration test thay media = asset mới, version cũ giữ asset cũ (F027-AC-08) — file: `src/backend/modules/learning/tests/integration/media/immutability_test.go` — ✅ 2026-11-19: `TestMediaReplaceKeepsOldVersion` ok

### Implementation cho User Story 1

- [x] T016 [P] [US1] Domain `media.MediaAsset`, `AllowedMime`, `BucketFor` — file: `src/backend/modules/learning/internal/domain/media/asset.go` — ✅ 2026-11-19: `internal/domain/media/asset.go`
- [x] T017 [P] [US1] `src/backend/pkg/media`: clamd INSTREAM, magic bytes, libvips (WebP/PNG 256/1024/2048, strip metadata), ffmpeg (Vorbis/MP3, Theora/HLS/MP4) — file: `src/backend/pkg/media/transcode.go` — ✅ 2026-11-19: `pkg/media/{scanner,magic,image,ffmpeg}.go` (libvips thay bằng Go thuần, OQ F027-1)
- [x] T018 [US1] Command `PresignMediaUpload`, `CompleteMediaUpload` (outbox `media.uploaded`, audit) — file: `src/backend/modules/learning/internal/application/command/complete_media_upload.go` — ✅ 2026-11-19: `command/media.go` PresignMediaUpload, CompleteMediaUpload (outbox `media.uploaded`, audit `media.uploaded`)
- [x] T019 [US1] Command `ProcessMedia` + consumer `learning.media_worker` (retry/backoff, DLQ) — file: `src/backend/modules/learning/internal/presentation/consumer/media_on_uploaded.go` — ✅ 2026-11-19: `command.Media.ProcessMedia` + `consumer/media_on_uploaded.go` (`learning.media_worker`, backoff, DLQ sau MaxDeliver 8)
- [x] T020 [US1] `CdnSigner` + handler `presignMediaUpload`, `completeMediaUpload` — file: `src/backend/modules/learning/internal/presentation/http/public/cms_media_handler.go` — ✅ 2026-11-19: `storage/cdn_signer.go` + `presentation/http/cms_media.go`

## Phase 4: User Story 2 — Thư viện media và nơi sử dụng (Priority: P2)

### Tests cho User Story 2

- [x] T021 [P] [US2] Integration test `listMediaAssets` lọc + tenant/`tenant_ids` System Admin (F027-AC-09) — file: `src/backend/modules/learning/tests/integration/media/library_test.go` — ✅ 2026-11-19: `TestMediaLibrary` (kind/status/q, cursor, tenant B không thấy, System Admin `tenant_ids`) ok
- [x] T022 [P] [US2] Integration test `getMediaAsset` biến thể + `used_in` (F027-AC-10) — file: `src/backend/modules/learning/tests/integration/media/usage_test.go` — ✅ 2026-11-19: `TestMediaLibrary` (`used_in` 2 version) ok
- [x] T023 [P] [US2] Integration test cập nhật `media_usages` khi lưu version + `assets_per_version_max` (F027-AC-11) — file: `src/backend/modules/learning/tests/integration/media/usage_limit_test.go` — ✅ 2026-11-19: `TestMediaUsagesOnSave` (ghi/soft-delete usage, asset lạ/tenant khác → 422, > 50 → 422) ok

### Implementation cho User Story 2

- [x] T024 [US2] Query `ListMediaAssets`, `GetMediaAsset`; hook `MediaUsageRecorder` trong pipeline lưu draft — file: `src/backend/modules/learning/internal/application/query/list_media_assets.go` — ✅ 2026-11-19: `query/media.go` (List keyset, Get + used_in), `command/media_refs.go` (MediaRefs, checkAssets, recordUsages trong command câu hỏi)

## Phase 5: User Story 3 — Công thức Toán/Lý/Hóa hiển thị bằng SVG (Priority: P1)

### Tests cho User Story 3

- [x] T025 [P] [US3] Unit test `ValidateLatex` (lệnh cấm, macro, độ dài, độ sâu) (F027-AC-12) — file: `src/backend/modules/learning/internal/domain/mathx/latex_test.go` — phải FAIL trước khi code — ✅ 2026-11-19: `TestValidateLatex` ok
- [x] T026 [P] [US3] Unit test `SanitizeSvg` (`<script>`, `on*`, tham chiếu ngoài, kích thước) (F027-AC-13) — file: `src/backend/modules/learning/internal/domain/mathx/svg_test.go` — ✅ 2026-11-19: `TestSanitizeSVG` ok
- [x] T027 [P] [US3] Contract test render payload/snapshot có `math_ref` + `speech_text` (F027-AC-14) — file: `src/backend/modules/learning/tests/contract/math_render_payload_test.go` — ✅ 2026-11-19: `tests/contract/math_render_payload_test.go` `TestMathRenderPayload` ok (LaTeX không gửi xuống client, OQ F027-3)
- [x] T028 [P] [US3] Integration test SVG bất biến theo sha256, không render lại version đã publish (F027-AC-15) — file: `src/backend/modules/learning/tests/integration/math/immutability_test.go` — ✅ 2026-11-19: `TestMathFormulas` (`tests/integration/math_test.go`) ok

### Implementation cho User Story 3

- [x] T029 [P] [US3] Domain `mathx` (LatexValidator, SvgSanitizer, MathBlock) — file: `src/backend/modules/learning/internal/domain/mathx/mathx.go` — ✅ 2026-11-19: `internal/domain/mathx/{latex,svg,render_transform}.go`
- [x] T030 [US3] Command `ValidateMathAssets` (lưu `math/<sha256>.svg`, `content.math_svgs`) gắn vào pipeline validate khi lưu draft — file: `src/backend/modules/learning/internal/application/command/validate_math_assets.go` — ✅ 2026-11-19: `command/validate_math_assets.go` gắn vào `Questions.prepare`
- [x] T031 [US3] Biến đổi LaTeX → `{math_ref, speech_text, latex}` trong builder render/snapshot — file: `src/backend/modules/learning/internal/domain/mathx/render_transform.go` — ✅ 2026-11-19: `mathx.Transform`/`Block` dùng trong `question.renderRich` và preview

## Phase 6: User Story 4 — Preview nội dung theo version (Priority: P2)

### Tests cho User Story 4

- [x] T032 [P] [US4] Integration test tạo phiên preview draft/published, `display_language`, không đáp án (F027-AC-16) — file: `src/backend/modules/learning/tests/integration/preview/create_session_test.go` — ✅ 2026-11-19: `TestPreviewSession` (draft/published, `en-US`, không đáp án) ok
- [x] T033 [P] [US4] Integration test không tạo attempt/learning event/outbox học tập/game session (F027-AC-17) — file: `src/backend/modules/learning/tests/integration/preview/no_side_effects_test.go` — ✅ 2026-11-19: `TestPreviewSession` (không outbox/version/usage mới) ok
- [x] T034 [P] [US4] Integration test `submitPreviewResponse` chấm + hết hạn → 404 (F027-AC-18) — file: `src/backend/modules/learning/tests/integration/preview/try_out_test.go` — ✅ 2026-11-19: `TestPreviewSession` (đúng/sai, giải thích, feedback, hint_levels, 422 response sai, hết hạn → 404) ok
- [x] T035 [P] [US4] Test tenant isolation + authorization preview (F027-AC-19) — file: `src/backend/modules/learning/tests/integration/preview/authz_tenant_test.go` — ✅ 2026-11-19: `TestPreviewAuthorization` (+ `authztest.Cover`) ok

### Implementation cho User Story 4

- [x] T036 [US4] Command `CreateLearningPreviewSession`, `SubmitPreviewResponse` (Valkey `PreviewStore`, scorer F022) + handler — file: `src/backend/modules/learning/internal/presentation/http/public/cms_preview_handler.go` — ✅ 2026-11-19: `command/preview.go` (Valkey `cache/preview_store.go`, scorer F022) + handler trong `cms_media.go`

## Phase 7: Polish & Cross-Cutting

- [x] T037 [P] Metric `eduworld_media_pipeline_duration_seconds`, `eduworld_media_rejected_total`, `eduworld_math_render_failed_total` — file: `src/backend/modules/learning/internal/infrastructure/observability/media_metrics.go` — ✅ 2026-11-19: `infrastructure/observability/media_metrics.go`
- [x] T038 [P] Đăng ký `media_max_bytes_by_kind`, `media_presign_ttl`, `media_signed_url_ttl`, `preview_token_ttl` trong CONFIG_DEFAULTS; CHANGELOG — file: `docs/00-overview/CONFIG_DEFAULTS.md` — ✅ 2026-11-19: tham số đã có trong CONFIG_DEFAULTS (đăng ký 2026-11-16); dùng hằng mặc định (OQ F027-11); CHANGELOG cập nhật
- [x] T039 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/027-media-math-preview/quickstart.md` — ✅ 2026-11-19: quickstart §5 bằng chứng
- [x] T040 [P] Follow-up F065 (OQ): pipeline nhận phụ đề WebVTT: `MediaKind` thêm `subtitle` (F058 Clarifications [Confirmed]), MIME `text/vtt`, ≤ 10 MB = giới hạn `document`/`csv_import_max` [Proposed]; migration `20261221000001` nới CHECK `kind`, magic bytes kiểm dòng `WEBVTT`, lưu biến thể `vtt` dưới khóa bất biến `media/<sha256>/vtt.vtt` (R31) — file: `src/backend/modules/learning/internal/domain/media/asset.go`, `src/backend/pkg/media/magic.go`, `src/backend/modules/learning/internal/application/command/media.go`, `docs/02-learning/CONTENT_MODEL.md` §7 — ✅ `pkg/media/media_test.go::TestSniffAndCompatible` (VTT có/không BOM, SRT và `WEBVTTX` khai `text/vtt` → lệch, VTT khai CSV → lệch) + `tests/integration/media_upload_test.go::TestMediaWebVTT` (ready, khóa `media/<sha256>/vtt.vtt`, `Cache-Control` immutable, SRT → `type_mismatch`, > 10 MB → `413 MEDIA_TOO_LARGE`, `subtitle` + `text/plain` → `MEDIA_TYPE_NOT_ALLOWED`; gói TestMedia testcontainers xanh)

## Dependencies & Execution Order

- Setup → Foundational → US1 → US3 (song song với US2) → US4 (cần F022 scorer) → Polish.
- T030 cần pipeline validate lưu draft (F020/F022); F025 dùng `media_assets.status` từ T006.

## Definition of Done

- [x] Mọi tiêu chí F027-AC-* có test pass (ghi mã test/TC)
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh cho module `learning` và `src/backend/pkg/media`
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
