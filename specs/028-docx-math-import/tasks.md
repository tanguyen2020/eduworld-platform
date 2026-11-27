# Tasks: F028 — Import Word/MathType (`lessons_docx`) qua Pandoc

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Spike: stage `media-tools` (ffmpeg + `pandoc-cli` 3.6.4) của image worker, bộ fixture `.docx` sinh bằng script, golden JSON AST, bảng OMML → LaTeX — file: `deploy/docker/worker.Dockerfile`, `src/backend/modules/learning/internal/infrastructure/docximport/docxfixture/`, `…/docximport/testdata/`, `research.md`
- [x] T002 Media `document` nhận MIME `.docx` (họ `application/zip`, đuôi `.docx`) — file: `src/backend/pkg/media/magic.go`, `src/backend/modules/learning/internal/domain/media/asset.go`, `…/application/command/media.go`

## Phase 2: Foundational

- [x] T003 Contract: `listContentImportItems`, `createDraftFromImportItem`, `ContentImportItem*`, `ContentImportJob.result.items_*`, mô tả `lessons_docx`; `split-public.mjs`; portal `gen:api` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: check-maxlength, check-portal-catalog, oasdiff
- [x] T004 ent `ImportItem` + migration `content.import_items` (RLS, view `_live`, CHECK) + CHECK kind `lessons_docx`; `atlas.sum` — file: `src/backend/modules/learning/ent/schema/curriculum.go`, `…/migrations/20270202000028_f028_import_items.sql` — kiểm: `TestMigrationMatchesEnt`, `atlas-sum.mjs --check`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Phân tích file Word (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [x] T010 [P] [US1] Test domain tách câu/mẫu/đáp án/metadata/vấn đề (F028-AC-02, AC-05) và chuẩn hóa + allowlist LaTeX Pandoc (F028-AC-03) — file: `…/internal/domain/docximport/docximport_test.go`
- [x] T011 [P] [US1] Test infrastructure: AST golden → mô hình đoạn, quét zip (macro, OLE MathType, entries, uncompressed), Pandoc thật qua `EW_PANDOC_CMD` (timeout, file hỏng) (F028-AC-01, AC-04, AC-06) — file: `…/internal/infrastructure/docximport/*_test.go`
- [x] T012 [P] [US1] Integration: job `lessons_docx` → mục câu, không tạo câu hỏi kể cả `dry_run=false`, báo cáo job, audit, lỗi cấp file, list phân trang + lọc (F028-AC-01, AC-06, AC-07, AC-08) — file: `…/tests/integration/content_import_docx_test.go`

### Implementation cho User Story 1

- [x] T013 [US1] Domain `docximport` (mô hình, `Split`, `Build`, `NormalizeLatex`, mã vấn đề) — file: `…/internal/domain/docximport/`
- [x] T014 [US1] Adapter Pandoc (exec, `--sandbox`, timeout, RTS heap, giới hạn stdout), AST → mô hình, `Inspect` zip — file: `…/internal/infrastructure/docximport/`
- [x] T015 [US1] Command `DocxImport.Run` trong `RunImportJob`, `CreateImport` nhận `lessons_docx`, repository `ImportItemRepo`; query `ImportItems.List` + read model — file: `…/application/command/import_docx.go`, `…/application/query/import_items.go`, `…/persistence/{write,read}/import_items.go`
- [x] T016 [US1] Handler `listContentImportItems`, routes, nối `module.go` (`EW_PANDOC_BIN`) — file: `…/presentation/http/cms_imports_docx.go`, `module.go`

## Phase 4: User Story 2 — Tạo Draft từng câu (Priority: P1)

### Tests cho User Story 2

- [x] T020 [P] [US2] Integration: tạo Draft (version 1 `draft`, audit, idempotent replay), thiếu/thừa SVG `MATH_RENDER_FAILED`, `409` câu lỗi/đã tạo, `draft_failed` + tạo lại, ảnh vào media pipeline (F028-AC-09…13) — file: `…/tests/integration/content_import_docx_test.go`
- [x] T021 [P] [US2] Integration: authorization + tenant isolation ba endpoint (F028-AC-14, TC-TEN-001); route contract — file: `…/tests/integration/content_import_docx_test.go`, `…/tests/contract/curriculum_routes_test.go`

### Implementation cho User Story 2

- [x] T022 [US2] Command `DocxImport.CreateDraft` (kiểm tập công thức, ghép SVG, thay `media_ref`, `createDraft` + metadata import, trạng thái câu) — file: `…/application/command/import_docx.go`
- [x] T023 [US2] Ảnh nhúng → media pipeline (`Media.IngestFile`) — file: `…/application/command/media_ingest.go`
- [x] T024 [US2] Handler `createDraftFromImportItem` + route — file: `…/presentation/http/cms_imports_docx.go`

## Phase 5: User Story 3 — Màn Import Word (Priority: P2)

- [x] T030 [P] [US3] Test portal: nguồn Word, upload + job + bảng phân trang + lọc, tạo Draft từng câu (Idempotency intent), lỗi render không gọi API, ActionGate (F028-AC-15); MathJax render LaTeX Pandoc (research R2) — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/import/docx/*.test.ts(x)`
- [x] T031 [US3] `DocxImportPanel`, `useDocxImport`, chuyển nguồn trong `ImportWizardPage`, i18n vi/en — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/import/`

## Phase N: Polish & Cross-Cutting

- [x] T040 [P] Tài liệu: CONTENT_PRODUCTION_PLAN §8, QUESTION_BANK §10, CONFIG_DEFAULTS, DATABASE_DESIGN, ADMIN_PORTAL, DEPLOYMENT, CHANGELOG, CODE_PLAN — file: `docs/…`
- [x] T041 Gates: gofmt, vet, golangci-lint (+arch), specs checks, oasdiff, portal lint/test, docs checks — ghi số liệu
- [ ] T042 Đo bộ mẫu thật Toán/Lý/Hóa (quickstart §2.6, F028-AC-16) — cần dữ liệu thật ngoài repo

## Bằng chứng (2026-11-28, local, sau rebase origin/main)

- Domain `go test ./modules/learning/internal/domain/docximport`: ok (TestBuildSingleChoice, TestBuildMultipleChoiceAndOverrides, TestBuildIssues 17 trường hợp, TestBuildFormulasAndImages, TestPandocLatexAllowlist 22 công thức, TestSplitHeaderVariants) — F028-AC-02, AC-03, AC-05.
- Infrastructure `go test ./modules/learning/internal/infrastructure/docximport/...` với `EW_PANDOC_CMD="docker run --rm -i --network none eduworld/media-tools:local pandoc"`: 6/6 pass (TestGoldenSample, TestGoldenObjects, TestInspect, TestPandocProcess — timeout/exit/thiếu chương trình, TestPandocReal — 50 câu ready, 22 công thức, OLE, zip hỏng) — F028-AC-01, AC-04, AC-06.
- Integration (testcontainers, Pandoc thật qua docker): TestDocxImportAnalyse, TestDocxImportFileErrors, TestDocxImportCreateDraft, TestDocxImportAccess pass (F028-AC-01…14, TC-TEN-001, TC-MATH-002); hồi quy TestQuestionImport, TestCurriculumImportSeedExport, TestMathFormulas, TestMediaUploadPipeline, TestCurriculumRouteMatrix pass.
- Contract/unit learning (`./modules/learning/internal/...`, `tests/contract`, `ent` TestMigrationMatchesEnt, `pkg/media`): pass; `routematrix check` 603 operation, 0 problems.
- Portal Vitest: DocxImportPanel.test.tsx 3/3, pandocLatex.test.ts 22/22, builders.test.tsx 8/8 (F054 import không hồi quy), parseImport 4/4 — F028-AC-15; `tsc` 0 lỗi; `pnpm lint` (eslint + stylelint) 0; `gen:check` khớp.
- golangci-lint learning + pkg/media: 0 issues (sau sửa QF1002); arch config: 0 issues. Specs: check-features 0 lỗi, check-cross-features 0 lỗi, check-portal-catalog 0, check-problem-codes 0, check-maxlength 0, check-client-authoritative 0, check-enums 0, validate-schemas 0, split-public --check, spectral-functions 6/6; oasdiff 0 breaking. Docs: check-links 0 hỏng, check-mermaid 0, build-rtm khớp.
- Mở: T042 / F028-AC-16 (bộ mẫu thật ngoài repo) → feature `In progress`.

## Dependencies & Execution Order

- Setup → Foundational (T003, T004) → US1 → US2 (cần mục câu của US1) → US3 (cần API US1–US2) → Polish.

## Definition of Done

- [ ] Mọi tiêu chí F028-AC-* có test pass (ghi mã test/TC) — AC-01…15 có; AC-16 cần dữ liệu thật
- [x] gen-check, lint, test, test-integration, web-test xanh
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
