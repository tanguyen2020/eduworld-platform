# Implementation Plan: F026 — Phát hành nội dung: publish/rollback/unpublish, snapshot, manifest, ghim version, nơi sử dụng

**Branch**: `026-content-release` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Aggregate `ContentItem` (con trỏ `current_version_id`, `latest_version_no`) dùng chung cho 7 `LearningContentType` qua adapter. Command `PublishContentVersion` → guard (F025) → `SnapshotBuilder` (render payload lọc đáp án theo plugin loại, ghi Valkey `ew:learning:cv:*`, CDN nếu được phép) → transaction đổi con trỏ + `content_releases` + outbox. Hẹn giờ bằng bảng `scheduled_publications` và job `content-scheduled-publish` (khóa `FOR UPDATE SKIP LOCKED`). `ReleaseResolver`/`VersionReader` là port cho các module ghim version. module game có consumer gộp mục học tập vào manifest tenant.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, nats.go, valkey-go, AWS SDK v2 S3, `src/backend/pkg/{outbox,tenancy,entx,authz,problem,clock}` |
| Storage | PostgreSQL `eduworld_learning` (`content.*`), `eduworld_game` (`game.content_manifests`); Valkey (cache); floci S3 `ew-media-public`/`ew-media-private` |
| Testing | go test + testcontainers (PostgreSQL, NATS, Valkey); clock giả cho hẹn giờ |
| Target Platform | Linux container: `core-api` (module `learning`), `worker-events` (consumer của module `learning` và `game`) |
| Project Type | backend module |
| Performance Goals | `publish_to_visible_max` (CONFIG_DEFAULTS); publish p95 < 2 s gồm dựng snapshot |
| Constraints | R9, R11, R13, R21, R26, R27, R31; ADR-0015, ADR-0043, ADR-0044 |
| Scale/Scope | Manifest ≤ 20.000 mục (`ContentManifest.items.maxItems`); `assets_per_version_max` |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta: `getContentVersion`, `getContentUsages`, `cancelScheduledPublication`, payload event, `ContentManifestItem` — Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | learning ↔ game chỉ qua event `content.*`; consumer game không đọc DB learning |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `content_releases` append-only; bảng mới có `tenant_id`/`deleted_at`; không hard-delete |
| IV Server-authoritative | ✅ | Snapshot lọc đáp án; version bất biến; Idempotency-Key + audit cho publish/rollback/unpublish |
| V Xác thực, phân quyền | ✅ | `learning_content.publish`, `content.release.rollback`, `learning_content.read/review` |
| VI An toàn trẻ em | ✅ | Không dữ liệu học sinh (chỉ đếm lượt ghim) |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số theo tên; TTL snapshot đăng ký mới (Polish) |
| VIII UI tách logic, token | ✅ | Không UI |
| IX Test có bằng chứng | ✅ | Test trước mỗi story; test tenant + authz mọi endpoint |

## Project Structure

### Documentation (feature này)

```text
specs/026-content-release/
├── spec.md · plan.md · data-model.md · quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
src/backend/modules/learning/
├── ent/schema/{content_release.go,scheduled_publication.go}
├── ent/schema/mixin/release_fields.go            # current_version_id, latest_version_no, snapshot_key, snapshot_sha256, based_on_version_id, change_note, publish_at
├── migrations/<ts>_f026_release.sql              # trigger forbid_published_update, check_release_target cho 7 bảng item
├── internal/domain/release/                      # ContentItem, Release, ScheduledPublication, rules
├── internal/domain/snapshot/                     # Snapshot, AnswerFieldStripper
├── internal/application/command/{publish_content_version.go,rollback_content.go,unpublish_content.go,cancel_scheduled_publication.go,archive_content_version.go,apply_scheduled_publication.go}
├── internal/application/query/{list_content_versions.go,get_content_version.go,diff_content_versions.go,list_content_releases.go,get_content_usages.go}
├── internal/application/port/{release_resolver.go,version_reader.go,snapshot_store.go}
├── internal/infrastructure/{persistence/release_repo.go,cache/snapshot_cache.go,storage/snapshot_cdn.go}
├── internal/presentation/http/public/cms_release_handler.go
├── internal/presentation/consumer/release_pointer_cache.go     # learning.release_pointer_cache
└── internal/presentation/job/scheduled_publish.go              # content-scheduled-publish
src/backend/modules/game/
├── internal/application/command/merge_learning_manifest_entries.go
└── internal/presentation/consumer/manifest_on_learning_content.go  # game.manifest_on_learning_content
```

**Structure Decision**: Một mixin release cho mọi bảng item để dùng chung trigger và adapter; consumer manifest đặt ở module game vì module đó sở hữu `game.content_manifests` (DATABASE_DESIGN §3.13).

## Phase 0: Research

Không có — ADR-0043, CONTENT_MODEL §5 và DATABASE_DESIGN §3.13 đủ quyết định.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `release.ContentItem.Publish/Rollback/Unpublish/Archive`, `ScheduledPublication`, `snapshot.Strip(payload, contentType)` | Bất biến: con trỏ chỉ trỏ version `published` của chính item |
| application | Command `PublishContentVersion`, `RollbackContent`, `UnpublishContent`, `CancelScheduledPublication`, `ApplyScheduledPublication`, `ArchiveContentVersion` (nhánh `archive` của `transitionContentVersion`) · Query `ListContentVersions`, `GetContentVersion`, `DiffContentVersions`, `ListContentReleases`, `GetContentUsages` | Permission: publish/unpublish/archive/cancel `learning_content.publish`; rollback `content.release.rollback`; đọc `learning_content.read`; diff đáp án `learning_content.review` |
| infrastructure | ent repo (khóa hàng item), `SnapshotCache` (Valkey `ew:learning:cv:<type>:<id>:<v>`, `ew:learning:current:<tenant>:<type>:<id>`), `SnapshotCDN` (S3 `content/<type>/<id>/<v>/<sha256>.json`), outbox | Cache miss → DB |
| presentation | `publishContentVersion`, `rollbackContent`, `unpublishContent`, `listContentVersions`, `getContentVersion`, `diffContentVersions`, `listContentReleases`, `getContentUsages`, `cancelScheduledPublication`; consumer `learning.release_pointer_cache`; job `content-scheduled-publish` (mỗi phút) | |

### Tích hợp và phụ thuộc

- Phụ thuộc: F025 (`PublishReadinessGuard`), F020 (assignment platform→tenant để tính tenant đích), F014 (manifest game).
- Event phát: `content.published`, `content.rolled_back`, `content.unpublished`, `content.publish_scheduled`, `content.version_archived`, `audit.recorded`.
- Event tiêu thụ: learning `content.*` (consumer con trỏ cache); game `content.*` từ `EVT_LEARNING` (consumer manifest). Không gọi đồng bộ (R29).

### Bảo mật, tenant, idempotency, audit

- `portal_staff`; `TenantScope`; platform chỉ role platform; System Admin ghi có `X-Tenant-Id` đích.
- Idempotency-Key bắt buộc cho publish/rollback/unpublish/cancel; `content_releases.idempotency_key` UNIQUE.
- Audit: `content.published`, `content.publish_scheduled`, `content.rolled_back`, `content.unpublished`, `content.archived`, `authz.denied`.

### Quan sát

- Metric `eduworld_content_publish_to_visible_seconds` (histogram, label `item_type`), `eduworld_content_releases_total{action}`, `eduworld_snapshot_cache_miss_total`, `eduworld_scheduled_publish_lag_seconds`.
- Span `release.publish`, `snapshot.build`, `manifest.merge`; log không chứa nội dung.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
