# Tasks: F026 — Phát hành nội dung: publish/rollback/unpublish, snapshot, manifest, ghim version, nơi sử dụng

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo package `domain/release`, `domain/snapshot` và wiring composition root — file: `modules/learning/cmd/api/main.go`
- [ ] T002 [P] Cấu hình Valkey key builder `ew:learning:cv:*`, `ew:learning:current:*` và bucket CDN (`EW_S3_*`, `EW_CDN_SIGNING_KEY`) — file: `modules/learning/internal/infrastructure/cache/keys.go`
- [ ] T003 [P] Đăng ký durable consumer `learning.release_pointer_cache` và `game.manifest_on_learning_content` (FilterSubjects `ew.evt.learning.content.>`) — file: `modules/learning/cmd/worker/main.go`, `modules/game/cmd/worker/main.go`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta OpenAPI: `getContentVersion`, `getContentUsages`, `cancelScheduledPublication`; `ContentManifestItem.activity_type/min_client_version` — file: `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/modules/game.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=learning`, `make gen MOD=game`
- [ ] T005 [P] Payload event: sửa `content.published.v1`; thêm `content.rolled_back`, `content.unpublished`, `content.publish_scheduled`, `content.version_archived` v1 (learning) và `content.manifest_updated` v1 (game, owner F026) — file: `specs/events/payloads/learning/`, `specs/events/payloads/game/content.manifest_updated.v1.schema.json` — kiểm: `make specs-lint`
- [ ] T006 ent: mixin `ReleaseFields` cho 7 bảng item/version, schema `ScheduledPublication`; `ContentRelease` AppendOnly — file: `modules/learning/ent/schema/` — kiểm: `make migrate-diff MOD=learning name=f026_release`, `make migrate-lint MOD=learning`
- [ ] T007 Migration tay (review): trigger `forbid_published_update`, `check_release_target` cho mọi bảng item; GRANT chỉ INSERT/SELECT `content_releases` — file: `modules/learning/migrations/`
- [ ] T008 Cập nhật DATABASE_DESIGN §3.13 (`scheduled_publications`, `published_by`, unique một bản mở) — file: `docs/07-backend/DATABASE_DESIGN.md`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Xuất bản ngay một version đã duyệt (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [ ] T009 [P] [US1] Unit test `ContentItem.Publish` + giữ version cũ `published` (F026-AC-01, F026-AC-06) — file: `modules/learning/internal/domain/release/item_test.go` — phải FAIL trước khi code
- [ ] T010 [P] [US1] Integration test publish: guard, tham chiếu con chưa publish, trạng thái sai (F026-AC-02) — file: `modules/learning/tests/integration/release/publish_guard_test.go`
- [ ] T011 [P] [US1] Integration test Idempotency-Key + `CONTENT_VERSION_CONFLICT` (F026-AC-03) — file: `modules/learning/tests/integration/release/publish_idempotency_test.go`
- [ ] T012 [P] [US1] Integration test trigger + use case bất biến, draft `based_on`, một bản mở (F026-AC-04, F026-AC-05) — file: `modules/learning/tests/integration/release/immutability_test.go`
- [ ] T013 [P] [US1] Test tenant isolation + authorization `publishContentVersion` (F026-AC-07) — file: `modules/learning/tests/integration/release/publish_authz_test.go`

### Implementation cho User Story 1

- [ ] T014 [P] [US1] Domain `ContentItem` (Publish/Rollback/Unpublish/Archive), lỗi domain — file: `modules/learning/internal/domain/release/item.go`
- [ ] T015 [US1] `SnapshotBuilder` + `SnapshotStore` port (Valkey + CDN, `cdn_eligible`) — file: `modules/learning/internal/application/port/snapshot_store.go`
- [ ] T016 [US1] Command `PublishContentVersion` (guard F025, khóa hàng item, outbox, audit) — file: `modules/learning/internal/application/command/publish_content_version.go`
- [ ] T017 [US1] Repository release + adapter 7 loại item — file: `modules/learning/internal/infrastructure/persistence/release_repo.go`
- [ ] T018 [US1] Handler `publishContentVersion` — file: `modules/learning/internal/presentation/http/public/cms_release_handler.go`

## Phase 4: User Story 2 — Rollback và gỡ xuất bản (Priority: P1)

### Tests cho User Story 2

- [ ] T019 [P] [US2] Integration test rollback chỉ đổi con trỏ + `content_releases` + event (F026-AC-08) — file: `modules/learning/tests/integration/release/rollback_test.go`
- [ ] T020 [P] [US2] Test `ROLLBACK_TARGET_INVALID` (archived/khác item/con trỏ/chưa publish) ở use case và trigger (F026-AC-09) — file: `modules/learning/tests/integration/release/rollback_target_test.go`
- [ ] T021 [P] [US2] Integration test unpublish → lượt mới `CONTENT_NOT_PUBLISHED` (F026-AC-10) — file: `modules/learning/tests/integration/release/unpublish_test.go`
- [ ] T022 [P] [US2] Test authorization + tenant + `reason` bắt buộc/độ dài cho rollback/unpublish (F026-AC-11) — file: `modules/learning/tests/integration/release/rollback_authz_test.go`

### Implementation cho User Story 2

- [ ] T023 [US2] Command `RollbackContent`, `UnpublishContent` + handler — file: `modules/learning/internal/application/command/rollback_content.go`

## Phase 5: User Story 3 — Snapshot bất biến và đọc theo version đã ghim (Priority: P1)

### Tests cho User Story 3

- [ ] T024 [P] [US3] Contract test strip đáp án cho mọi `LearningContentType` (F026-AC-12) — file: `modules/learning/tests/contract/snapshot_strip_test.go` — phải FAIL trước khi code
- [ ] T025 [P] [US3] Integration test không có object CDN cho `exam_only`/tenant-private, signed URL cho private (F026-AC-13) — file: `modules/learning/tests/integration/release/snapshot_cdn_test.go`
- [ ] T026 [P] [US3] Integration test Valkey trống/dừng → đọc DB, nạp lại (F026-AC-14) — file: `modules/learning/tests/integration/release/snapshot_cache_test.go`
- [ ] T027 [P] [US3] Integration test `VersionReader`/`ReleaseResolver` qua publish/rollback/unpublish (F026-AC-15) — file: `modules/learning/tests/integration/release/pinning_test.go`
- [ ] T028 [P] [US3] Integration test consumer con trỏ cache idempotent, tenant được gán / `_platform` (F026-AC-16) — file: `modules/learning/tests/integration/release/pointer_cache_consumer_test.go`

### Implementation cho User Story 3

- [ ] T029 [P] [US3] `snapshot.Strip` theo loại nội dung — file: `modules/learning/internal/domain/snapshot/strip.go`
- [ ] T030 [US3] Port + adapter `ReleaseResolver`, `VersionReader` (cache → DB) — file: `modules/learning/internal/infrastructure/cache/snapshot_cache.go`
- [ ] T031 [US3] Consumer `learning.release_pointer_cache` — file: `modules/learning/internal/presentation/consumer/release_pointer_cache.go`

## Phase 6: User Story 4 — Hẹn giờ xuất bản (Priority: P2)

### Tests cho User Story 4

- [ ] T032 [P] [US4] Integration test tạo lịch `202`, `scheduled_publish_min_lead` (F026-AC-17) — file: `modules/learning/tests/integration/release/schedule_test.go`
- [ ] T033 [P] [US4] Integration test job áp dụng ±1 phút với clock giả, 2 worker không trùng, guard lỗi → `failed` (F026-AC-18) — file: `modules/learning/tests/integration/release/scheduled_job_test.go`
- [ ] T034 [P] [US4] Test hủy lịch + authz/tenant `cancelScheduledPublication` (F026-AC-19) — file: `modules/learning/tests/integration/release/schedule_cancel_test.go`

### Implementation cho User Story 4

- [ ] T035 [US4] Nhánh hẹn giờ trong `PublishContentVersion`, command `CancelScheduledPublication`, `ApplyScheduledPublication` — file: `modules/learning/internal/application/command/apply_scheduled_publication.go`
- [ ] T036 [US4] Job `content-scheduled-publish` (`FOR UPDATE SKIP LOCKED`) — file: `modules/learning/internal/presentation/job/scheduled_publish.go`

## Phase 7: User Story 5 — Lịch sử, xem version chỉ đọc, so sánh (Priority: P2)

### Tests cho User Story 5

- [ ] T037 [P] [US5] Integration test `listContentVersions`, `getContentVersion` (archived) + tenant/authz (F026-AC-20) — file: `modules/learning/tests/integration/release/history_test.go`
- [ ] T038 [P] [US5] Integration test diff ẩn pointer đáp án khi thiếu `learning_content.review` (F026-AC-21) — file: `modules/learning/tests/integration/release/diff_test.go`
- [ ] T039 [P] [US5] Integration test `listContentReleases` thứ tự + tenant isolation (F026-AC-22) — file: `modules/learning/tests/integration/release/releases_list_test.go`

### Implementation cho User Story 5

- [ ] T040 [US5] Query `ListContentVersions` (đếm `active_pinned_sessions` từ port), `GetContentVersion`, `DiffContentVersions` (JSON pointer diff), `ListContentReleases` + handler — file: `modules/learning/internal/application/query/diff_content_versions.go`

## Phase 8: User Story 6 — Content manifest theo tenant (Priority: P2)

### Tests cho User Story 6

- [ ] T041 [P] [US6] Integration test consumer game dựng manifest tenant sở hữu/được gán, ETag, 304 (F026-AC-23) — file: `modules/game/tests/integration/manifest/learning_entries_test.go`
- [ ] T042 [P] [US6] Test đo độ trễ publish → manifest (F026-AC-24) — file: `modules/game/tests/integration/manifest/publish_latency_test.go`
- [ ] T043 [P] [US6] Test mục activity có `activity_type`, `min_client_version` (F026-AC-25) — file: `modules/game/tests/integration/manifest/min_client_version_test.go`
- [ ] T044 [P] [US6] Test tenant isolation manifest: không có mục tenant khác/platform chưa gán (F026-AC-26) — file: `modules/game/tests/integration/manifest/tenant_isolation_test.go`

### Implementation cho User Story 6

- [ ] T045 [US6] Thêm `min_client_version`/`snapshot_url`/`cdn_eligible` vào payload `content.published` (bảng metadata loại tạm cho `quiz`/`interactive_exercise` tới F030) — file: `modules/learning/internal/application/command/publish_content_version.go`
- [ ] T046 [US6] Command `MergeLearningManifestEntries` + consumer `game.manifest_on_learning_content` — file: `modules/game/internal/presentation/consumer/manifest_on_learning_content.go`

## Phase 9: User Story 7 — Nơi sử dụng và lưu trữ version (Priority: P3)

### Tests cho User Story 7

- [ ] T047 [P] [US7] Integration test `getContentUsages` (question set, lesson, activity, bài giao, exam session) + tenant/authz (F026-AC-27) — file: `modules/learning/tests/integration/release/usages_test.go`
- [ ] T048 [P] [US7] Integration test archive con trỏ bị từ chối, archive version cũ + event (F026-AC-28) — file: `modules/learning/tests/integration/release/archive_test.go`

### Implementation cho User Story 7

- [ ] T049 [US7] Query `GetContentUsages` (read model SQL có `tenant_id` + `deleted_at IS NULL`), nhánh `archive` trong `transitionContentVersion` — file: `modules/learning/internal/application/query/get_content_usages.go`

## Phase 10: Polish & Cross-Cutting

- [ ] T050 [P] Metric `eduworld_content_publish_to_visible_seconds`, `eduworld_snapshot_cache_miss_total`, `eduworld_scheduled_publish_lag_seconds` — file: `modules/learning/internal/infrastructure/observability/release_metrics.go`
- [ ] T051 [P] Đăng ký `content_snapshot_cache_ttl` trong CONFIG_DEFAULTS; CHANGELOG — file: `docs/00-overview/CONFIG_DEFAULTS.md`
- [ ] T052 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/026-content-release/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2, US3 (song song) → US4, US5 → US6 (cần F014 Done) → US7 → Polish.
- T016 dùng `PublishReadinessGuard` của F025 (T029 của F025). T045 thay bảng metadata tạm bằng registry khi F030 xong.

## Definition of Done

- [ ] Mọi tiêu chí F026-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh cho module `learning` và module `game`
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
