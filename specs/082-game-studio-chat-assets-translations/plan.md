# Implementation Plan: F082 — Game Studio — quản lý sticker/GIF và dịch nội dung game (`/game-admin/translations*`)

**Branch**: `082-game-studio-chat-assets-translations` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Frontend (chunk `game-ops`): module `chat-assets/` (Sticker pack editor, thư viện GIF) trên `ContentEditorShell` của F079, gọi `/api/v1/game-admin/{sticker-packs|gifs}` (APISIX → `core-api`, handler của module chat F118) và presign asset chat; module `translations/` (lọc trạng thái dịch, dialog xuất/nhập, theo dõi job) và mở rộng editor game bằng `BilingualField` + huy hiệu trạng thái. Backend (module game): bounded context `translation` — tính `translation_status` khi lưu draft (hook vào command của F078), lọc `listGameConfigs` theo trạng thái, command `ExportGameTranslations`/`ImportGameTranslations`, query `GetGameTranslationJob`, job runner trong `worker` (profile `jobs`) sinh/đọc XLIFF 2.1/CSV. Quyết định kỹ thuật ở [research.md](research.md).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | TypeScript strict (React 18, Vite) · Go ≥ 1.24 |
| Primary Dependencies | Ant Design 5 + Pro, TanStack Query, `apiClient` (F050), `ContentEditorShell`/`SchemaForm` (F079), `BilingualField` (F053), Web Crypto (sha256) · ent, Atlas, oapi-codegen, `src/backend/pkg/entx` (`UnitOfWork`), `src/backend/pkg/idempotency`, `src/backend/pkg/outbox`, S3 SDK (floci, ADR-0071), `encoding/xml`, `encoding/csv`, `golang.org/x/text/unicode/norm` |
| Storage | PostgreSQL `eduworld_game`: `game.config_translation_states` (Mới), `game.translation_jobs` (Mới), đọc bảng item/version của F071–F078; object storage bucket `ew-exports` (file xuất), `media/<sha256>/…` (file nhập qua presign game-media) |
| Testing | Vitest + Testing Library + MSW (chỉ test, nhãn `MOCK`), Playwright E2E · go test + testcontainers (PostgreSQL, floci) |
| Target Platform | Trình duyệt desktop (editor), tablet xem · Linux container (`core-api` + `worker`, module `game`) |
| Project Type | web app (workspace A) + mở rộng backend module |
| Performance Goals | Lưu draft thêm ≤ 20 ms p95 cho tính trạng thái dịch [Proposed]; job xuất 500 item ≤ 2 phút [Proposed] |
| Constraints | AGENTS R1, R7, R10, R11, R20, R27, R28, R31, R32; ADR-0027, ADR-0034, ADR-0043, ADR-0044, ADR-0056 |
| Scale/Scope | `content_ids` ≤ 500, `content_types` ≤ 20, `report.errors` ≤ 500 dòng (`TranslationJob`) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta `GameConfigVersion.translation_*`, `listGameConfigs.translation_status`, `presignChatAssetUpload`, schema `sticker-pack`/`gif` ở Phase 2 (T004–T007) |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Dịch thuộc module game (chủ dữ liệu); sticker/GIF chỉ gọi API module chat qua gateway; nhập dịch dùng lại command handler F078 qua interface có kiểu |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Chỉ dữ liệu platform; bảng mới G có `deleted_at`; bản ghi tenant bị loại; không hard-delete |
| IV Server-authoritative | ✅ | Trạng thái dịch, giới hạn tệp, duyệt ≠ tác giả ở server; nhập dịch không sửa version published; Idempotency-Key + audit |
| V Xác thực, phân quyền | ✅ | `game.content.translate`, `game.chat_assets.write` platform-only; `platform_staff` (azp + MFA + role platform); gateway nội bộ |
| VI An toàn trẻ em | ✅ | Không upload học sinh, không GIF bên thứ ba; alt text bắt buộc |
| VII Một nguồn sự thật, cấu hình | ✅ | Trường cần dịch suy từ JSON Schema (research R1); tham số theo tên; `MaxLen` cho cột mới |
| VIII UI tách logic, token | ✅ | Admin/Ops theme; game UI chỉ trong `GamePreviewFrame` |
| IX Test có bằng chứng | ✅ | Mỗi F082-AC có test viết trước |

## Project Structure

### Documentation (feature này)

```text
specs/082-game-studio-chat-assets-translations/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
src/backend/modules/game/
├── ent/schema/{configtranslationstate.go,translationjob.go}
├── internal/domain/i18ntext/{pointers.go,status.go,key.go}          # R1, R3; khóa <config_type>/<id>/<version>/<pointer>
├── internal/application/command/translation/{recompute_translation_state.go,export_game_translations.go,
│                                             import_game_translations.go,run_translation_job.go}
├── internal/application/query/translation/get_game_translation_job.go
├── internal/application/query/list_game_configs.go                    # thêm lọc translation_status (F078)
├── internal/application/port/{translation_codec.go,export_storage.go}
├── internal/infrastructure/translation/{xliff21_codec.go,csv_codec.go}
├── internal/infrastructure/persistence/translation/{state_repo.go,job_repo.go}
├── internal/presentation/http/public/game_translations.go            # exportGameTranslations, importGameTranslations, getGameTranslationJob
├── internal/presentation/job/translation_jobs.go                     # worker profile jobs
└── tests/{unit,integration,contract}/translation/
src/cms/frontend/packages/ops-ui/src/translations/{TranslationExportDialog.tsx,TranslationImportDialog.tsx,TranslationJobProgress.tsx,TranslationStatusBadge.tsx}
src/cms/frontend/packages/ops-ui/src/upload/{useChecksumUpload.ts,UploadQueue.tsx}
src/cms/frontend/apps/portal/src/workspaces/game-ops/
├── chat-assets/{routes.tsx,StickerPackListPage.tsx,StickerPackEditor.tsx,StickerGridEditor.tsx,UnlockRulePicker.tsx,
│                GifLibraryPage.tsx,GifUploadDialog.tsx,GifReviewPanel.tsx,ChatAssetPreview.tsx}
├── chat-assets/api/{chatAssets.ts,presignChatAsset.ts}
├── translations/{routes.tsx,TranslationsPage.tsx,BilingualConfigFields.tsx,OutdatedDiffDrawer.tsx}
├── translations/api/{gameTranslations.ts,gameMediaPresign.ts}
└── registry/configTypes.ts                                           # thêm sticker-packs, gifs (permission game.chat_assets.write)
src/cms/frontend/apps/portal/src/i18n/locales/{vi,en}/portal.game_ops.json
src/cms/frontend/apps/portal/tests/{unit,e2e}/game-ops/{chat-assets,translations}/
```

**Structure Decision**: Dịch là bounded context riêng trong module game để không chạm schema của module từng loại; hook tính trạng thái gắn vào command lưu draft chung của F078. UI dùng lại khung editor F079 và dialog dịch tách từ F053 sang `ops-ui` (ADMIN_PORTAL §1.4: một khung editor dùng chung).

## Phase 0: Research

→ [research.md](research.md): R1 trường cần dịch từ JSON Schema + bảng trạng thái riêng, R2 quy tắc nhập theo version, R3 phát hiện lỗi thời bằng hash, R4 định dạng XLIFF/CSV và job DB.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `i18ntext.Pointers`, `ComputeStatus(prevMeta, payload, required_languages)`, `TranslationKey` parse/format, `ImportDecision` (R2) | Thuần, test unit |
| application | Command `RecomputeTranslationState` (gọi trong transaction lưu draft của F078), `ExportGameTranslations`, `ImportGameTranslations`, `RunTranslationJob` · Query `GetGameTranslationJob`, mở rộng `ListGameConfigs` | Permission `game.content.translate`; authz tường minh đầu handler, idempotency middleware `src/backend/pkg/httpx`, tracing `src/backend/pkg/observability` |
| infrastructure | `XLIFF21Codec`, `CSVCodec` (chặn DTD/entity), `ExportStorage` (S3 `ew-exports`, presign GET theo `report_link_ttl`), repo ent | |
| presentation | Handler `exportGameTranslations`, `importGameTranslations`, `getGameTranslationJob`; job `translation-jobs` (`worker` profile `jobs`) · Portal gọi `listGameConfigs`, `createGameConfig`, `getGameConfigVersion`, `updateGameConfigDraft`, `transitionGameConfigVersion`, `publishGameConfigVersion`, `rollbackGameConfig`, `unpublishGameConfig`, `listGameConfigVersions`, `diffGameConfigVersions`, `presignChatAssetUpload`, `presignGameMediaUpload`, `getGameMediaAsset`, `exportGameTranslations`, `importGameTranslations`, `getGameTranslationJob`, `getMyPermissions` | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F078 (workflow, `ConfigTypeRegistry`, presign game-media), F079 (`ContentEditorShell`, registry loại, route `game-ops`), F118 (backend sticker/GIF), F053 (`BilingualField`), F057 (hộp gán tenant), F051 (403, step-up, VPN).
- Facade: không (sticker/GIF do portal gọi API public của module chat qua APISIX → `core-api`). Event phát: `audit.recorded` (outbox). Tiêu thụ: không.

### Bảo mật, tenant, idempotency, audit

- Permission: `game.content.translate` (dịch), `game.chat_assets.write` (sticker/GIF); middleware `platform_staff`; route chỉ trên gateway nội bộ (`/api/v1/game-admin/translations*` đã có trong bảng route API_DESIGN).
- Không `X-Tenant-Id`; truy vấn module dùng `owner_scope = platform`/`tenant_id IS NULL`; bản ghi tenant bị loại (F082-AC-17).
- `Idempotency-Key`: `exportGameTranslations`, `importGameTranslations`, `presignChatAssetUpload`, `presignGameMediaUpload`, các thao tác workflow. Audit: `game_translation.exported`, `game_translation.imported` (actor, số item, job_id), không ghi nội dung văn bản.

### Quan sát

- Metric `eduworld_game_translation_jobs_total{kind,status}`, `eduworld_game_translation_rows_total{result}`; span `translation.job.run`; log không chứa văn bản dịch (R14 không áp vì không phải dữ liệu học sinh, nhưng giữ log gọn).


### Danh mục menu/action portal (ADR-0090)

Menu và nút của feature lấy từ [`specs/portal/menu-catalog.yaml`](../portal/menu-catalog.yaml) qua registry sinh (`make portal-catalog`, F051) — **không** đăng ký menu tay trong TSX. Nút thao tác bọc `<ActionGate actionKey=…>`; menu/action trong `disabled_menus`/`disabled_actions` của `getMyPermissions` bị ẩn; `403 FEATURE_DISABLED` → thông báo "Tính năng đang tạm tắt" và tải lại quyền. Operation gọi tới đều có `x-portal-action` khớp các khóa dưới (kiểm `node scripts/specs/check-portal-catalog.mjs`).

| Loại | Khóa |
|---|---|
| Menu feature hiện thực | `menu.game_ops.chat_assets`, `menu.game_ops.translations`, `menu.game_ops.media` |
| Action dùng | `action.game_ops.chat_assets.view`, `action.game_ops.chat_assets.edit`, `action.game_ops.chat_assets.workflow`, `action.game_ops.chat_assets.publish`, `action.game_ops.chat_assets.rollback`, `action.game_ops.translations.translate`, `action.game_ops.media.view`, `action.game_ops.media.upload` |

Khoảng trống catalog/contract (không sửa trong feature này; báo owner `specs/portal/menu-catalog.yaml` + OpenAPI):

- `presignChatAssetUpload` (Cần thêm) → đề xuất `action.game_ops.chat_assets.edit`

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
