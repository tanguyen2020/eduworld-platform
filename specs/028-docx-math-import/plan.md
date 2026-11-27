# Implementation Plan: F028 — Import Word/MathType (`lessons_docx`) qua Pandoc

**Branch**: `028-docx-math-import` · **Date**: 2026-11-27 · **Spec**: [spec.md](spec.md)

## Summary

`lessons_docx` thành kind import được hỗ trợ của `createContentImport` (khung job F021/F024). Job worker (profile `jobs`) kiểm zip bằng Go, chạy Pandoc (`--sandbox`, timeout, giới hạn heap) ra JSON AST, chuyển AST thành mô hình đoạn trung lập, domain `docximport` tách câu theo mẫu và dựng nội dung `single_choice`/`multiple_choice`, công thức qua `mathx.ValidateLatex`; mỗi câu lưu một dòng `content.import_items` với trạng thái và vấn đề. Portal đọc `listContentImportItems`, render SVG công thức bằng MathJax 4 và gọi `createDraftFromImportItem` từng câu; server ghép SVG + ảnh (media pipeline) vào nội dung đã phân tích và tạo Draft bằng lõi `createQuestion` (research R5).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go 1.27 / TypeScript (React 19, Vite) |
| Primary Dependencies | ent, chi (`pkg/httpx`), Pandoc 3.6.4 (tiến trình ngoài, image `eduworld/media-tools`), `archive/zip` + `encoding/xml`; Ant Design 5, TanStack Query, MathJax 4 (ops-ui) |
| Storage | PostgreSQL `eduworld_learning` (`content.import_jobs`, mới `content.import_items`); object storage (file `.docx` là media `document` ở `ew-media-private`; ảnh nhúng vào `ew-media-quarantine` rồi pipeline F027) |
| Testing | go test (unit domain/infrastructure, integration testcontainers), Pandoc thật qua `EW_PANDOC_CMD` (docker `eduworld/media-tools`); Vitest portal với backend MOCK hợp đồng |
| Target Platform | Linux container (`core-api`, `worker` profile `jobs`), trình duyệt ≥ 1024 px |
| Project Type | backend module `src/backend/modules/learning` + web app `src/cms/frontend/apps/portal` |
| Performance Goals | SC-003: 200 câu một lượt; Pandoc ~1,5 s cho 50 câu (research R1) |
| Constraints | R7, R8, R10, R11, R14, R28, R31, R32; ADR-0055 §4 (SVG chỉ render trên trình duyệt), ADR-0044 |
| Scale/Scope | `docx_import_max_questions`, `csv_import_max` (CONFIG_DEFAULTS) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | spec + research spike trước code; CODE_PLAN cập nhật |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Domain `docximport` thuần (không I/O); Pandoc/zip ở `infrastructure/docximport` sau port `DocxConverter`; command ghi qua `UnitOfWork`, query `listContentImportItems` chỉ đọc read model; không gọi module khác |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `import_items` mixin `TenantOptional` như `import_jobs`; job chạy trong scope chủ job; soft-delete; không xóa vật lý; file ở object storage |
| IV Server-authoritative | ✅ | Nội dung câu (đề, đáp án) do server phân tích và giữ; client chỉ gửi SVG của đúng công thức server liệt kê; SVG sanitize phía server |
| V Xác thực, phân quyền | ✅ | `authz.Require(learning_content.import)` đầu mỗi use case; 404 tenant khác; Idempotency-Key bắt buộc cho hai lệnh ghi |
| VI An toàn trẻ em | ✅ | Không chạm chat/hồ sơ học sinh; không dữ liệu học sinh trong file import |
| VII Một nguồn sự thật, cấu hình | ✅ | Quy tắc mẫu ở CONTENT_PRODUCTION_PLAN §8; tham số mới đăng ký CONFIG_DEFAULTS; allowlist công thức dùng chung `mathx` |
| VIII UI tách logic, token | ✅ | Portal chỉ điều phối (upload, render MathJax theo ADR-0055, gọi API); Ant Design + token, không hardcode màu; chuỗi qua i18n |
| IX Test có bằng chứng | ✅ | AC-01…15 có test; AC-16 cần dữ liệu thật — để mở, feature `In progress` |

## Project Structure

### Documentation (feature này)

```text
specs/028-docx-math-import/
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
deploy/docker/worker.Dockerfile                     # stage media-tools (ffmpeg + pandoc-cli) = eduworld/media-tools
specs/openapi/eduworld-api.v1.yaml                  # listContentImportItems, createDraftFromImportItem, ContentImportItem…
src/backend/pkg/media/magic.go                      # MIME .docx trong họ application/zip
src/backend/modules/learning/
├── ent/schema/curriculum.go                        # ImportItem — content.import_items
├── migrations/20270102000028_f028_import_items.sql
├── internal/domain/docximport/                     # mô hình đoạn, tách câu theo mẫu, dựng nội dung, chuẩn hóa LaTeX
├── internal/domain/media/asset.go                  # MIME .docx cho kind document
├── internal/application/command/import_docx.go     # DocxImport: Run (phân tích), CreateDraft (giai đoạn 2)
├── internal/application/command/media_ingest.go    # ảnh nhúng → media pipeline
├── internal/application/query/import_items.go      # ListImportItems
├── internal/application/port/{import_docx.go,write,read}
├── internal/infrastructure/docximport/             # Pandoc runner, AST → mô hình, quét zip/OLE, đọc ảnh; docxfixture (TEST FIXTURE)
├── internal/infrastructure/persistence/{write,read}/import_items.go
├── internal/presentation/http/cms_imports_docx.go
├── module.go                                       # nối DocxImport vào Curriculum.Docx, handler, EW_PANDOC_BIN
└── tests/integration/content_import_docx_test.go
src/cms/frontend/apps/portal/src/workspaces/content-studio/import/docx/   # DocxImportPanel, useDocxImport, test
```

**Structure Decision**: Theo cấu trúc sẵn có của module `learning` (F021/F024 import, F027 media); không tạo module/service mới.

## Phase 0: Research

[research.md](research.md): R1 công cụ và sandbox, R2 bảng OMML → LaTeX (22/22), R3 đầu vào bị từ chối, R4 mẫu tài liệu, R5 hai giai đoạn, R6 ảnh nhúng, R7 tiêu chí bộ mẫu thật.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** (cách chạy và kiểm) → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `docximport.Document`/`Block`/`Inline`, `Split`, `Build`, `NormalizeLatex`, mã vấn đề | Thuần Go; dùng `mathx.ValidateLatex` |
| application | Command `DocxImport.Run` (gọi từ `Curriculum.RunImportJob`), `DocxImport.CreateDraft` · Query `ImportItems.List` | Permission `learning_content.import` |
| infrastructure | `docximport.Pandoc` (exec, timeout, RTS), `docximport.Inspect` (zip, macro, OLE, entries), `ImportItemRepo`, `ImportItemModel`, `storage.AssetReader` | |
| presentation | Handler `listContentImportItems`, `createDraftFromImportItem`; job `import-jobs` (có sẵn) | `core-api`; job ở `worker` profile `jobs` |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F024 (khung import, `createDraft`), F027 (media, `MathAssets`), F054 (màn Import).
- Facade gọi: không có. Event phát: `media.uploaded` (F027, ảnh nhúng). Event tiêu thụ: không mới.

### Bảo mật, tenant, idempotency, audit

- Permission `learning_content.import`; TenantScope của phiên, job chạy trong scope chủ job (`jobScope`).
- Idempotency-Key bắt buộc: `createContentImport` (có sẵn), `createDraftFromImportItem` (mới).
- Audit: `content.import_requested`, `content.imported` (có sẵn), `content.version_created` metadata `source = import`, `import_id`, `item_no`; `media.uploaded` cho ảnh.
- Pandoc: `--sandbox`, env rỗng, stdin/stdout, timeout giết tiến trình, giới hạn đọc stdout; file kiểm zip trước.

### Quan sát

- Span `learning.command.RunDocxImport`, `learning.command.CreateDraftFromImportItem`, `learning.query.ListContentImportItems`; metric có sẵn `eduworld_import_jobs_total{kind,status}`, `eduworld_import_rows_rejected_total{kind}` qua `ImportFinished`; log chỉ id job, số câu, thời gian, mã lỗi — không nội dung.

## Complexity Tracking

> Không có vi phạm.
