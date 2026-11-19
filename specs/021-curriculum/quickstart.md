# Quickstart: F021 — Curriculum: cấu trúc, LO, DAG tiên quyết, version, mapping, import/export, seed GDPT

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F020 (và F015 cho bước 10; F027 nếu import qua portal).
- Lệnh: `make up`, `make migrate`, `make seed` (gồm `go -C src/backend run ./modules/learning/cmd/seed`), `make run-core-api`, `make run-worker PROFILE=events`.
- Tài khoản demo: `cm.noidung` (content_admin), `cm.duyet` (content_reviewer — kiểm 403 khi không có `curriculum.publish` nếu role không được cấp), `sa.admin`, `qlcm.truong.a` (academic_manager, `th-demo-a`), `ad.truong.a` (school_admin), `ad.truong.b` (school_admin, `pgd-demo`), `hs.an.3a` (học sinh lớp 3).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `make seed` hai lần, rồi `cm.noidung` mở `listCmsCurricula` | Một curriculum `GDPT2018_SAMPLE`, 16 LO mẫu, không trùng | F021-AC-22, F021-AC-23 |
| 2 | `cm.noidung` tạo curriculum `TEST_MATH` + version `1.0`, môn × khối Toán 3, topic, LO `MATH.G3.NUM.MUL_TABLE.2` | Tạo thành công, version `draft` | F021-AC-01, F021-AC-02 |
| 3 | Tạo LO mã `math-g3-x` và LO `yccd_ref` 65 ký tự | `422 VALIDATION_FAILED` chỉ rõ field | F021-AC-03, F021-AC-04 |
| 4 | Đặt tiên quyết A→B, B→C rồi C→A | `422` với `PREREQUISITE_CYCLE`, liệt kê A, B, C | F021-AC-11, F021-AC-12 |
| 5 | `ad.truong.a` tạo topic; `qlcm.truong.a` tạo môn học | `403 PERMISSION_DENIED` cả hai | F021-AC-06 |
| 6 | `cm.noidung` submit_review; `cm.noidung` tự approve | `422 INVALID_STATE_TRANSITION` (reviewer = author); `sa.admin` approve → `approved` | F021-AC-13, F021-AC-18 |
| 7 | Publish với `Idempotency-Key` hai lần | Một `content_releases`, event `curriculum.version_published` một lần | F021-AC-15 |
| 8 | PATCH LO của version đã publish | `409 CONTENT_NOT_EDITABLE` | F021-AC-07, F021-AC-16 |
| 9 | Tạo version mới từ version 1; tạo thêm lần nữa khi bản nháp còn mở | Bản nháp sao sâu, giữ mã LO; lần hai `409 CONFLICT` | F021-AC-17 |
| 10 | `sa.admin` gán `GDPT2018_SAMPLE` cho `th-demo-a`; `hs.an.3a` gọi `GET /api/v1/learning/curricula`, `/learning/topics?subject_id=…&grade_level=3` | Thấy curriculum và version áp dụng; không có field biên tập; `ad.truong.b` không thấy | F021-AC-08, F021-AC-19, F021-AC-20 |
| 11 | Tạo curriculum `GDPT2018` (thiếu `yccd_ref`) và publish | `422` liệt kê LO thiếu | F021-AC-14 |
| 12 | Export version (`format=yaml`), import lại ở `dry_run` rồi thật | Dry-run không ghi; import tạo cấu trúc giống | F021-AC-22, F021-AC-24, F021-AC-25 |
| 13 | Version 2 thay version 1, thiếu mapping một LO → publish | `422` liệt kê LO; bổ sung mapping → publish, payload có `mappings` | F021-AC-26, F021-AC-27 |
| 14 | Tra `GET /learning/objectives/{id}` của LO version 1 sau khi version 2 publish | Trả đúng LO cũ | F021-AC-21 |
| 15 | Tạo bộ sách, bài 12 gắn 2 LO; `listCmsObjectives?textbook_section_id=` | Trả đúng 2 LO | F021-AC-28, F021-AC-29 |
| 16 | Xóa topic ở version draft; mở Thùng rác bằng `sa.admin`; If-Match sai khi PATCH | Topic vào Thùng rác; `412 VERSION_MISMATCH` | F021-AC-10, F021-AC-09 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=learning
make test MOD=learning
make test-integration MOD=learning
make migrate-lint MOD=learning
make specs-lint
```

## 4. Bằng chứng cần nộp

- Kết quả test theo mã F021-AC-*, trace_id bước 7 và 13, payload event `curriculum.version_published`, file export bước 12.

## 5. Bằng chứng (2026-11-19, local, testcontainers)

Kịch bản §2 chạy bằng test tự động qua chuỗi `pkg/httpx` thật (xác thực token giả lập IdP, X-Tenant-Id, guard portal, validate OpenAPI, idempotency) trên PostgreSQL đã migrate; tài khoản demo là principal giả lập cùng tên (`cm.noidung`, `sa.admin`, `cm.duyet`, `qlcm.truong.a`, `ad.truong.a`, `ad.truong.b`, `hs.an.3a`).

| # | Test | Tiêu chí |
|---|---|---|
| 1 | `TestCurriculumImportSeedExport` (seed 2 lần: 3 môn, 16 LO, lần 2 `Skipped`, publish một lần) | F021-AC-22, AC-23 |
| 2–3 | `TestCurriculumAuthoring` (`TEST_MATH` + version `1.0` draft, replay idempotent, mã LO sai/`G4` sai khối → `code:INVALID`, `yccd_ref` 65 → `yccd_ref:MAX_LENGTH`, competency version khác → 422) | AC-01…05 |
| 4 | `TestCurriculumAuthoring` (A→B→C rồi C→A → `PREREQUISITE_CYCLE`, `cycle` = 3 mã, không lưu; tiên quyết version khác, 21 phần tử, tự tham chiếu → 422) | AC-11, AC-12 |
| 5 | `TestCurriculumAuthzIsolation` (school_admin tạo topic, academic_manager tạo môn học/ghi curriculum platform được gán → 403, `authz.denied` +3) | AC-06 |
| 6 | `TestCurriculumWorkflow` (tự duyệt → 422 `INVALID_STATE_TRANSITION`, `sa.admin` duyệt; `cm.duyet` → 403) | AC-13, AC-18 |
| 7 | `TestCurriculumWorkflow` (publish 2 lần cùng key: 1 `content_releases`, 1 `curriculum.version_published`, `Idempotent-Replayed: true`) | AC-15 |
| 8 | `TestCurriculumEditability`, `TestCurriculumWorkflow` (PATCH LO/version không draft → 409 `CONTENT_NOT_EDITABLE`; trigger chặn UPDATE trực tiếp) | AC-07, AC-16 |
| 9 | `TestCurriculumWorkflow` (version 2 sao sâu, giữ `code`/`stable_key`, tiên quyết ánh xạ lại; lần hai 409) | AC-17, AC-02 |
| 10 | `TestCurriculumPublicRead` (`hs.an.3a` thấy version đang hiệu lực; với dòng `lms.school_curriculum_settings` (fixture) thấy version cài đặt; không field biên tập; `ad.truong.b` không thấy) | AC-08, AC-19, AC-20 |
| 11 | `TestCurriculumWorkflow` (`GDPT2018` thiếu `yccd_ref` → 422 liệt kê `MATH.G1.NUM.COUNT_10`) | AC-14 |
| 12 | `TestCurriculumImportSeedExport` (export YAML/CSV, dry-run không ghi, import thật cho tenant A/B cùng cấu trúc; file lỗi/quá lớn/hỏng → `failed`) | AC-22, AC-24, AC-25 |
| 13 | `TestCurriculumLoMappings` (thiếu mapping → 422 liệt kê `MATH.G3.NUM.DIV`; đủ mapping → payload 3 dòng có `removed`) | AC-26, AC-27 |
| 14 | `TestCurriculumPublicRead` (LO version 1 đọc được sau khi version 2 publish) | AC-21 |
| 15 | `TestCurriculumTextbooks` (bộ sách Cánh Diều, bài 12 gắn 2 LO; lọc theo bài: platform 2, tenant A 2, tenant B 0) | AC-28, AC-29 |
| 16 | `TestCurriculumEditability` (xóa topic → Thùng rác `resource_type = curriculum`; If-Match sai → 412, thiếu → 428) | AC-09, AC-10 |

Ma trận quyền: `TestCurriculumRouteMatrix` phủ 43 operation (role_denied + cross_tenant); `go run ./tools/routematrix check` → 0 problems. Unit: `go test ./modules/learning/internal/domain/curriculum` (11 test). Contract: `TestCurriculumOperationsMatchSpec`.

Lệnh (Git Bash, không có `make`, giới hạn CPU):

```bash
cd src/backend
GOMAXPROCS=2 go test -p 2 ./modules/learning/... -count=1
GOMAXPROCS=2 TESTCONTAINERS_RYUK_DISABLED=true go test -p 2 -tags integration -timeout 30m ./modules/learning/tests/integration -count=1  # 28/28 pass (F020 + F021); gói vượt timeout mặc định 10 phút
```

Chưa chạy được trên máy này: `make seed` với stack local (không có `make`; seed được kiểm bằng `job.Seed` trong test), `make migrate-lint` (Atlas container) — migration được áp trong mọi test integration.
