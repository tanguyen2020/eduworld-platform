# Tasks: F027 — Media pipeline, công thức SVG, preview session

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo module Go `src/backend/pkg/media` (Scanner, MagicSniffer, ExifStripper, Transcoder, Keys) — file: `src/backend/pkg/media/media.go`
- [ ] T002 [P] Image `worker` (`src/backend/cmd/worker`, profile `events`) có libvips + ffmpeg; compose profile `media` (clamav) cho integration test — file: `deploy/docker/worker.Dockerfile`, `deploy/compose/docker-compose.yml`
- [ ] T003 [P] Cấu hình S3/CDN (`EW_S3_ENDPOINT`, `EW_S3_FORCE_PATH_STYLE`, `EW_S3_BUCKET_MEDIA`, `EW_CDN_SIGNING_KEY`, `EW_MEDIA_SCAN`) trong `.env.example` — file: `src/backend/modules/learning/.env.example`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta OpenAPI: `completeMediaUpload`, `submitPreviewResponse`, `alt_text`/`is_exam_material` trong `MediaPresignRequest`, `license` `maxLength` 255 (D17) — file: `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=learning`
- [ ] T005 [P] Payload `media.uploaded/processed/rejected.v1` — file: `specs/events/payloads/learning/` — kiểm: `make specs-lint`
- [ ] T006 ent schema `MediaAsset` (cột mới), `MediaUsage` (+`tenant_id`), `MathSvg` — file: `src/backend/modules/learning/ent/schema/` — kiểm: `make migrate-diff MOD=learning name=f027_media`, `make migrate-lint MOD=learning`
- [ ] T007 Cập nhật DATABASE_DESIGN §3.13 (`quarantine_key`, `is_exam_material`, `upload_completed_at`, `processed_at`, `media_usages.tenant_id`) — file: `docs/07-backend/DATABASE_DESIGN.md`
- [ ] T008 Port `ObjectStorage`, `MediaProcessor`, `PreviewStore`, `Scorer` + adapter S3 — file: `src/backend/modules/learning/internal/infrastructure/storage/s3_storage.go`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Upload media an toàn (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [ ] T009 [P] [US1] Unit test allowlist mime/size/alt_text + state machine asset (F027-AC-01, F027-AC-02) — file: `src/backend/modules/learning/internal/domain/media/asset_test.go` — phải FAIL trước khi code
- [ ] T010 [P] [US1] Integration test presign + dedupe `sha256` + complete khớp/không khớp (F027-AC-01, F027-AC-03) — file: `src/backend/modules/learning/tests/integration/media/presign_complete_test.go`
- [ ] T011 [P] [US1] Integration test worker: EICAR, sai magic bytes, lỗi transcode → `rejected` (F027-AC-04) — file: `src/backend/modules/learning/tests/integration/media/worker_reject_test.go`
- [ ] T012 [P] [US1] Integration test xóa EXIF/GPS, biến thể, khóa content-addressed, `immutable` (F027-AC-05) — file: `src/backend/modules/learning/tests/integration/media/worker_variants_test.go`
- [ ] T013 [P] [US1] Integration test bucket public/private + signed URL hết hạn (F027-AC-06) — file: `src/backend/modules/learning/tests/integration/media/signed_url_test.go`
- [ ] T014 [P] [US1] Test tenant isolation + authorization presign/complete/get (tenant khác, platform scope, token game) (F027-AC-07) — file: `src/backend/modules/learning/tests/integration/media/authz_tenant_test.go`
- [ ] T015 [P] [US1] Integration test thay media = asset mới, version cũ giữ asset cũ (F027-AC-08) — file: `src/backend/modules/learning/tests/integration/media/immutability_test.go`

### Implementation cho User Story 1

- [ ] T016 [P] [US1] Domain `media.MediaAsset`, `AllowedMime`, `BucketFor` — file: `src/backend/modules/learning/internal/domain/media/asset.go`
- [ ] T017 [P] [US1] `src/backend/pkg/media`: clamd INSTREAM, magic bytes, libvips (WebP/PNG 256/1024/2048, strip metadata), ffmpeg (Vorbis/MP3, Theora/HLS/MP4) — file: `src/backend/pkg/media/transcode.go`
- [ ] T018 [US1] Command `PresignMediaUpload`, `CompleteMediaUpload` (outbox `media.uploaded`, audit) — file: `src/backend/modules/learning/internal/application/command/complete_media_upload.go`
- [ ] T019 [US1] Command `ProcessMedia` + consumer `learning.media_worker` (retry/backoff, DLQ) — file: `src/backend/modules/learning/internal/presentation/consumer/media_on_uploaded.go`
- [ ] T020 [US1] `CdnSigner` + handler `presignMediaUpload`, `completeMediaUpload` — file: `src/backend/modules/learning/internal/presentation/http/public/cms_media_handler.go`

## Phase 4: User Story 2 — Thư viện media và nơi sử dụng (Priority: P2)

### Tests cho User Story 2

- [ ] T021 [P] [US2] Integration test `listMediaAssets` lọc + tenant/`tenant_ids` System Admin (F027-AC-09) — file: `src/backend/modules/learning/tests/integration/media/library_test.go`
- [ ] T022 [P] [US2] Integration test `getMediaAsset` biến thể + `used_in` (F027-AC-10) — file: `src/backend/modules/learning/tests/integration/media/usage_test.go`
- [ ] T023 [P] [US2] Integration test cập nhật `media_usages` khi lưu version + `assets_per_version_max` (F027-AC-11) — file: `src/backend/modules/learning/tests/integration/media/usage_limit_test.go`

### Implementation cho User Story 2

- [ ] T024 [US2] Query `ListMediaAssets`, `GetMediaAsset`; hook `MediaUsageRecorder` trong pipeline lưu draft — file: `src/backend/modules/learning/internal/application/query/list_media_assets.go`

## Phase 5: User Story 3 — Công thức Toán/Lý/Hóa hiển thị bằng SVG (Priority: P1)

### Tests cho User Story 3

- [ ] T025 [P] [US3] Unit test `ValidateLatex` (lệnh cấm, macro, độ dài, độ sâu) (F027-AC-12) — file: `src/backend/modules/learning/internal/domain/mathx/latex_test.go` — phải FAIL trước khi code
- [ ] T026 [P] [US3] Unit test `SanitizeSvg` (`<script>`, `on*`, tham chiếu ngoài, kích thước) (F027-AC-13) — file: `src/backend/modules/learning/internal/domain/mathx/svg_test.go`
- [ ] T027 [P] [US3] Contract test render payload/snapshot có `math_ref` + `speech_text` (F027-AC-14) — file: `src/backend/modules/learning/tests/contract/math_render_payload_test.go`
- [ ] T028 [P] [US3] Integration test SVG bất biến theo sha256, không render lại version đã publish (F027-AC-15) — file: `src/backend/modules/learning/tests/integration/math/immutability_test.go`

### Implementation cho User Story 3

- [ ] T029 [P] [US3] Domain `mathx` (LatexValidator, SvgSanitizer, MathBlock) — file: `src/backend/modules/learning/internal/domain/mathx/mathx.go`
- [ ] T030 [US3] Command `ValidateMathAssets` (lưu `math/<sha256>.svg`, `content.math_svgs`) gắn vào pipeline validate khi lưu draft — file: `src/backend/modules/learning/internal/application/command/validate_math_assets.go`
- [ ] T031 [US3] Biến đổi LaTeX → `{math_ref, speech_text, latex}` trong builder render/snapshot — file: `src/backend/modules/learning/internal/domain/mathx/render_transform.go`

## Phase 6: User Story 4 — Preview nội dung theo version (Priority: P2)

### Tests cho User Story 4

- [ ] T032 [P] [US4] Integration test tạo phiên preview draft/published, `display_language`, không đáp án (F027-AC-16) — file: `src/backend/modules/learning/tests/integration/preview/create_session_test.go`
- [ ] T033 [P] [US4] Integration test không tạo attempt/learning event/outbox học tập/game session (F027-AC-17) — file: `src/backend/modules/learning/tests/integration/preview/no_side_effects_test.go`
- [ ] T034 [P] [US4] Integration test `submitPreviewResponse` chấm + hết hạn → 404 (F027-AC-18) — file: `src/backend/modules/learning/tests/integration/preview/try_out_test.go`
- [ ] T035 [P] [US4] Test tenant isolation + authorization preview (F027-AC-19) — file: `src/backend/modules/learning/tests/integration/preview/authz_tenant_test.go`

### Implementation cho User Story 4

- [ ] T036 [US4] Command `CreateLearningPreviewSession`, `SubmitPreviewResponse` (Valkey `PreviewStore`, scorer F022) + handler — file: `src/backend/modules/learning/internal/presentation/http/public/cms_preview_handler.go`

## Phase 7: Polish & Cross-Cutting

- [ ] T037 [P] Metric `eduworld_media_pipeline_duration_seconds`, `eduworld_media_rejected_total`, `eduworld_math_render_failed_total` — file: `src/backend/modules/learning/internal/infrastructure/observability/media_metrics.go`
- [ ] T038 [P] Đăng ký `media_max_bytes_by_kind`, `media_presign_ttl`, `media_signed_url_ttl`, `preview_token_ttl` trong CONFIG_DEFAULTS; CHANGELOG — file: `docs/00-overview/CONFIG_DEFAULTS.md`
- [ ] T039 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/027-media-math-preview/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational → US1 → US3 (song song với US2) → US4 (cần F022 scorer) → Polish.
- T030 cần pipeline validate lưu draft (F020/F022); F025 dùng `media_assets.status` từ T006.

## Definition of Done

- [ ] Mọi tiêu chí F027-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh cho module `learning` và `src/backend/pkg/media`
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
