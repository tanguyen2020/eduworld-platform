# Tasks: F028 — Import Word/MathType (`lessons_docx`) qua Pandoc

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Spike: stage `media-tools` (ffmpeg + `pandoc-cli` 3.6.4) của image worker, bộ fixture `.docx` sinh bằng script, golden JSON AST, bảng OMML → LaTeX — file: `deploy/docker/worker.Dockerfile`, `src/backend/modules/learning/internal/infrastructure/docximport/docxfixture/`, `…/docximport/testdata/`, `research.md`
- [ ] T002 Media `document` nhận MIME `.docx` (họ `application/zip`, đuôi `.docx`) — file: `src/backend/pkg/media/magic.go`, `src/backend/modules/learning/internal/domain/media/asset.go`, `…/application/command/media.go`

## Phase 2: Foundational

- [ ] T003 Contract: `listContentImportItems`, `createDraftFromImportItem`, `ContentImportItem*`, `ContentImportJob.result.items_*`, mô tả `lessons_docx`; `split-public.mjs`; portal `gen:api` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: check-maxlength, check-portal-catalog, oasdiff
- [ ] T004 ent `ImportItem` + migration `content.import_items` (RLS, view `_live`, CHECK) + CHECK kind `lessons_docx`; `atlas.sum` — file: `src/backend/modules/learning/ent/schema/curriculum.go`, `…/migrations/20270102000028_f028_import_items.sql` — kiểm: `TestMigrationMatchesEnt`, `atlas-sum.mjs --check`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Phân tích file Word (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [ ] T010 [P] [US1] Test domain tách câu/mẫu/đáp án/metadata/vấn đề (F028-AC-02, AC-05) và chuẩn hóa + allowlist LaTeX Pandoc (F028-AC-03) — file: `…/internal/domain/docximport/docximport_test.go`
- [ ] T011 [P] [US1] Test infrastructure: AST golden → mô hình đoạn, quét zip (macro, OLE MathType, entries, uncompressed), Pandoc thật qua `EW_PANDOC_CMD` (timeout, file hỏng) (F028-AC-01, AC-04, AC-06) — file: `…/internal/infrastructure/docximport/*_test.go`
- [ ] T012 [P] [US1] Integration: job `lessons_docx` → mục câu, không tạo câu hỏi kể cả `dry_run=false`, báo cáo job, audit, lỗi cấp file, list phân trang + lọc (F028-AC-01, AC-06, AC-07, AC-08) — file: `…/tests/integration/content_import_docx_test.go`

### Implementation cho User Story 1

- [ ] T013 [US1] Domain `docximport` (mô hình, `Split`, `Build`, `NormalizeLatex`, mã vấn đề) — file: `…/internal/domain/docximport/`
- [ ] T014 [US1] Adapter Pandoc (exec, `--sandbox`, timeout, RTS heap, giới hạn stdout), AST → mô hình, `Inspect` zip — file: `…/internal/infrastructure/docximport/`
- [ ] T015 [US1] Command `DocxImport.Run` trong `RunImportJob`, `CreateImport` nhận `lessons_docx`, repository `ImportItemRepo`; query `ImportItems.List` + read model — file: `…/application/command/import_docx.go`, `…/application/query/import_items.go`, `…/persistence/{write,read}/import_items.go`
- [ ] T016 [US1] Handler `listContentImportItems`, routes, nối `module.go` (`EW_PANDOC_BIN`) — file: `…/presentation/http/cms_imports_docx.go`, `module.go`

## Phase 4: User Story 2 — Tạo Draft từng câu (Priority: P1)

### Tests cho User Story 2

- [ ] T020 [P] [US2] Integration: tạo Draft (version 1 `draft`, audit, idempotent replay), thiếu/thừa SVG `MATH_RENDER_FAILED`, `409` câu lỗi/đã tạo, `draft_failed` + tạo lại, ảnh vào media pipeline (F028-AC-09…13) — file: `…/tests/integration/content_import_docx_test.go`
- [ ] T021 [P] [US2] Integration: authorization + tenant isolation ba endpoint (F028-AC-14, TC-TEN-001); route contract — file: `…/tests/integration/content_import_docx_test.go`, `…/tests/contract/curriculum_routes_test.go`

### Implementation cho User Story 2

- [ ] T022 [US2] Command `DocxImport.CreateDraft` (kiểm tập công thức, ghép SVG, thay `media_ref`, `createDraft` + metadata import, trạng thái câu) — file: `…/application/command/import_docx.go`
- [ ] T023 [US2] Ảnh nhúng → media pipeline (`Media.IngestFile`) — file: `…/application/command/media_ingest.go`
- [ ] T024 [US2] Handler `createDraftFromImportItem` + route — file: `…/presentation/http/cms_imports_docx.go`

## Phase 5: User Story 3 — Màn Import Word (Priority: P2)

- [ ] T030 [P] [US3] Test portal: nguồn Word, upload + job + bảng phân trang + lọc, tạo Draft từng câu (Idempotency intent), lỗi render không gọi API, ActionGate (F028-AC-15); MathJax render LaTeX Pandoc (research R2) — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/import/docx/*.test.ts(x)`
- [ ] T031 [US3] `DocxImportPanel`, `useDocxImport`, chuyển nguồn trong `ImportWizardPage`, i18n vi/en — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/import/`

## Phase N: Polish & Cross-Cutting

- [ ] T040 [P] Tài liệu: CONTENT_PRODUCTION_PLAN §8, QUESTION_BANK §10, CONFIG_DEFAULTS, DATABASE_DESIGN, ADMIN_PORTAL, DEPLOYMENT, CHANGELOG, CODE_PLAN — file: `docs/…`
- [ ] T041 Gates: gofmt, vet, golangci-lint (+arch), specs checks, oasdiff, portal lint/test, docs checks — ghi số liệu
- [ ] T042 Đo bộ mẫu thật Toán/Lý/Hóa (quickstart §2.6, F028-AC-16) — cần dữ liệu thật ngoài repo

## Dependencies & Execution Order

- Setup → Foundational (T003, T004) → US1 → US2 (cần mục câu của US1) → US3 (cần API US1–US2) → Polish.

## Definition of Done

- [ ] Mọi tiêu chí F028-AC-* có test pass (ghi mã test/TC) — AC-16 cần dữ liệu thật
- [ ] gen-check, lint, test, test-integration, web-test xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
