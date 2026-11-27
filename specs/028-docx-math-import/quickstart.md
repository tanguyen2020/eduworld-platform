# Quickstart: F028 — Import Word/MathType (`lessons_docx`) qua Pandoc

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F024, F027, F054.
- Image công cụ: `docker build --target media-tools -t eduworld/media-tools:local -f deploy/docker/worker.Dockerfile .` (Pandoc 3.6.4 + ffmpeg); image `worker` dựng trên stage này nên `pandoc` có trên `PATH` của worker.
- Chạy local theo [LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md) (`make up`, `make migrate`, `make seed`, `make dev`); worker ngoài container đặt `EW_PANDOC_BIN` (mặc định `pandoc`).
- Tài khoản demo: `bt.noidung.a` (tenant_content_editor, có `learning_content.import`), `staff.norole.a` (không quyền), `qlcm.truong.b` (tenant B).
- File mẫu sinh bằng script (không nội dung sách giáo khoa): `EW_DOCX_FIXTURE_DIR=<dir> go test ./modules/learning/internal/infrastructure/docximport/docxfixture -run TestWriteSpikeFixtures` → `sample_50.docx`, `objects.docx`, `formulas.docx`, `formula_*.docx`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 2.1 | Portal → Content Studio → Import → nguồn **Word (.docx)**, chọn `sample_50.docx`, giấy phép, **Phân tích** | Job `succeeded`; tổng kết 50 câu nhập được; bảng 20 câu/trang, lọc trạng thái | F028-AC-01, AC-02, AC-07, AC-08, AC-15 |
| 2.2 | Chọn `objects.docx` | Job `failed` `MACRO_NOT_ALLOWED` (file có `vbaProject.bin`); bỏ macro rồi thử lại → câu MathType `FORMULA_NEEDS_REWRITE`, ảnh GIF `IMAGE_TYPE_NOT_ALLOWED`, câu link `warning` | AC-04, AC-05, AC-06 |
| 2.3 | Bấm **Tạo bản nháp** | 50 câu `created`, mỗi câu một version 1 `draft`; công thức hiển thị SVG trong editor; không có version published | AC-09, AC-13 |
| 2.4 | Gọi lại `createDraftFromImportItem` cùng `Idempotency-Key` | `Idempotent-Replayed: true`, không tạo thêm; khóa khác → `409 CONFLICT IMPORT_ITEM_ALREADY_CREATED` | AC-09, AC-11 |
| 2.5 | `qlcm.truong.b` gọi `GET /cms/learning/imports/{id}/items` của job tenant A; `staff.norole.a` gọi | `404`; `403` | AC-14 |
| 2.6 | **Bộ mẫu thật** (ngoài repo): EdTech đưa ≥ 30 file Toán/Lý/Hóa lớp 6–9; import từng file, so công thức SVG với bản Word và đáp án | ≥ 95% công thức đúng + render được, 0 câu sai đáp án; ghi kết quả vào bảng dưới trước khi mở cho người soạn | AC-16 |

## 3. Lệnh test tự động

```bash
# domain + infrastructure (Pandoc thật qua docker khi có EW_PANDOC_CMD)
EW_PANDOC_CMD="docker run --rm -i --network none eduworld/media-tools:local pandoc" go test ./modules/learning/internal/domain/docximport/... ./modules/learning/internal/infrastructure/docximport/...
# integration (testcontainers)
EW_PANDOC_CMD="docker run --rm -i --network none eduworld/media-tools:local pandoc" go test -tags integration -run 'TestDocxImport' ./modules/learning/tests/integration/
# portal
cd src/cms/frontend && npx vitest run apps/portal/src/workspaces/content-studio/import/docx
```

## 4. Bằng chứng cần nộp

- Kết quả test (số pass/fail) ghi trong tasks.md; bảng §2.6 (số file, số công thức, % đúng, số câu sai đáp án, ngày, người đo).

| Ngày | Người đo | Số file | Số công thức | % đúng | Câu sai đáp án |
|---|---|---|---|---|---|
| — | — | — | — | — | — |
