# Quickstart: F022 — Question Bank 8 loại cơ bản: validator, scorer server-side, render payload không đáp án

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F020, F021 (seed `GDPT2018_SAMPLE` đã publish và gán cho `th-demo-a`).
- Lệnh: `make up`, `make migrate`, `make seed`, `make run-core-api`, `make run-worker PROFILE=events`.
- Tài khoản demo: `cm.noidung` (content_admin, câu platform), `ad.truong.a` (school_admin `th-demo-a`, có `learning_content.write`), `ad.truong.b` (`pgd-demo`), `qlcm.truong.a` (academic_manager), `sa.admin`.
- Fixture: `src/backend/modules/learning/tests/integration/question/fixtures/qb_examples/*.json` (8 ví dụ QUESTION_BANK §3).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `ad.truong.a` `POST /api/v1/cms/learning/questions` với từng ví dụ 8 loại (header `X-Tenant-Id`, `Idempotency-Key`) | `201`, version 1 `draft`, có `ETag`; gửi lại cùng key → cùng response | F022-AC-01 |
| 2 | Tạo câu `short_answer` | `422`, `errors[].code = TYPE_NOT_ENABLED` (trước F023) | F022-AC-02 |
| 3 | Gửi `single_choice` có 2 đáp án đúng; `ordering` 2 mục; `matching` cặp tham chiếu `r_9`; option id `A` | `422` kèm JSON pointer từng lỗi | F022-AC-03 |
| 4 | `ordering` với `partial_method = right_minus_wrong` | `422` | F022-AC-04 |
| 5 | `GET /cms/learning/questions/{id}` | Có `answer_key`, `options[].feedback`, `hints`; bản lưu `render_payload` không có `feedback` | F022-AC-06, F022-AC-10 |
| 6 | PATCH nháp không `If-Match` / `If-Match` cũ | `428` / `412` | F022-AC-08 |
| 7 | `ad.truong.b` GET câu của `th-demo-a`; `ad.truong.a` PATCH câu platform được gán | `404` / `403` | F022-AC-09 |
| 8 | `searchQuestions?subject_id=…&grade_level=3&type=numeric&source=own` | Chỉ câu khớp của `th-demo-a` | F022-AC-13 |
| 9 | `ad.truong.a` xóa câu nháp; `sa.admin` xem Thùng rác và khôi phục | Biến mất khỏi danh sách, có trong Thùng rác, khôi phục được | F022-AC-11 |
| 10 | Gửi đáp án chấp nhận 300 ký tự | `422` chỉ rõ field | F022-AC-12 |
| 11 | `make test MOD=learning` (gói `question/...`) | Bảng chấm 8 loại, ≥ 200 cặp chuẩn hóa, property shuffle pass | F022-AC-14 … F022-AC-21 |
| 12 | `make test-contract` (render DTO, view delivery) | Không trường đáp án trong DTO; `ordering` khác thứ tự đúng; `math_ref` thay LaTeX | F022-AC-22 … F022-AC-25 |
| 13 | `make test-integration MOD=learning` (selection) | Cùng seed cùng kết quả; pool thiếu → `QUESTION_POOL_INSUFFICIENT`; cache bị xóa sau `content.published` | F022-AC-26 … F022-AC-28 |
| 14 | Tạo version mới từ version đang `in_review` khi đã có nháp mở; PATCH version không draft | `409 CONFLICT` / `409 CONTENT_NOT_EDITABLE` | F022-AC-07 |
| 15 | Lưu câu có ký tự tổ hợp NFD, khối `worked_example`, 51 media | NFC khi đọc lại; `worked_example` và vượt media → `422` | F022-AC-05 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=learning
make lint MOD=learning
make test MOD=learning
make test-integration MOD=learning
make test-contract
```

## 4. Bằng chứng cần nộp

- Kết quả test theo mã F022-AC-* (số case bảng chấm, số cặp chuẩn hóa), báo cáo contract test DTO, trace_id bước 1 và 9.

## 5. Bằng chứng (2026-11-19)

Kịch bản §2 chạy bằng test tự động qua chuỗi `pkg/httpx` thật (token IdP giả lập, `X-Tenant-Id`, guard portal, validate OpenAPI, idempotency) trên PostgreSQL đã migrate (testcontainers) với tài khoản demo `qlcm.truong.a`/`qlcm.truong.b` (academic_manager) thay `ad.truong.a`/`ad.truong.b` (cùng `learning_content.write`), `cm.noidung`, `cm.duyet`, `sa.admin`. Trạng thái `published` + con trỏ release đặt bằng helper TEST `publishQuestion` (luồng publish thật ở F025/F026 — OQ F022-13).

| # | Test | Tiêu chí |
|---|---|---|
| 1 | `TestQuestionAuthoring` (8 ví dụ QUESTION_BANK §3 → 201 `draft` version 1 + `ETag`; gửi lại cùng key → `Idempotent-Replayed: true`, cùng `content_id`; 8 dòng `content.questions`) | F022-AC-01 |
| 2 | `TestQuestionAuthoring` (`short_answer` → 422 `/type:TYPE_NOT_ENABLED`) | AC-02 |
| 3–4 | `TestQuestionAuthoring` (key `o_9999`, cặp `r_9`, option id `A`, `ordering` + `right_minus_wrong` → 422 kèm JSON pointer); `TestValidate` (ID trùng, 2 đáp án đúng, `ordering` 2 mục, `partial_method` theo loại, `max_score`) | AC-03, AC-04 |
| 5 | `TestQuestionAuthoring` (stem NFD lưu NFC; khối `worked_example` → 422; 51 media → `/media_refs:INVALID`) | AC-05 |
| 6, 10 | `TestQuestionAuthoring` (cột `render_payload` không có `feedback`/`misconception`, `option_feedback` giữ `off_by_one_factor`; GET CMS có `correct_option_id`, feedback, hints) | AC-06, AC-10 |
| 7, 14 | `TestQuestionVersioning` (PATCH thiếu/sai `If-Match` → 428/412; PATCH version published → 409 `CONTENT_NOT_EDITABLE`; trigger chặn UPDATE trực tiếp; version 2 sao từ 1 có `based_on_version_id`; nháp mở thứ hai → 409 `CONFLICT`; con trỏ release tới nháp → `RELEASE_TARGET_INVALID`) | AC-07, AC-08 |
| 7 (quyền), 8 | `TestQuestionAuthzSearchDelete` (tenant B → 404; không role → 403; tenant tạo câu platform → 403; câu platform được gán: đọc được `assigned: true`, PATCH → 403; tenant B không thấy câu platform chưa gán) + `TestQuestionRouteMatrix` (7 operation × role_denied/cross_tenant) | AC-09 |
| 9 | `TestQuestionAuthzSearchDelete` (xóa thiếu `If-Match` → 428; xóa → 204, biến mất, có trong Thùng rác `question`, audit `content.deleted`, event `question.deleted`, version còn; `sa.admin` khôi phục → đọc lại được; câu đã phát hành → 409) | AC-11 |
| 10 | `TestQuestionAuthoring` (đáp án chấp nhận 300 ký tự → `/answer_key/blanks/b_1/accepted/0:INVALID`) | AC-12 |
| 8 (lọc) | `TestQuestionAuthzSearchDelete` (11 tổ hợp `source`/`type`/`difficulty`/`objective_id`/`topic_id`/`subject_id`/`grade_level`/`status`) | AC-13 |
| 11 | `TestGradeTable` (8 loại, điểm trong `[min_score, max_score]`, công thức `multiple_choice`, partial `ordering`/`matching`/`drag_drop`/`true_false`/`fill_in_blank`), `TestNormalizationPairs` (314 cặp) + `TestNormalizationRules`, `TestParse`/`TestWithin` (`,`/`.`, ngăn nghìn, phân số, 3 kiểu dung sai), `TestGradeInvalidResponses` (sai shape → `ErrInvalidResponse`, không panic), `TestGradeIndependentOfShuffle`; `Delivery.Grade` chỉ nhận response | AC-14 … AC-21 |
| 12 | `TestRenderDTOHasNoAnswerData` (contract, 8 loại: không khóa đáp án/feedback/hints/LaTeX, giữ mọi id, `math_ref` + `speech_text`), `TestRender` (trộn tái lập theo seed + version, `ordering` khác thứ tự đúng) | AC-22, AC-23, AC-25 |
| 12 (view) | `TestQuestionDelivery` (`learning_readonly` không select được `answer_key` từ view lẫn bảng; view không có 6 cột đáp án; render qua view không lộ đáp án; tenant B không render được câu của A) | AC-24 |
| 13 | `TestSelect`, `TestRelaxation` (unit); `TestQuestionDelivery` (pool chỉ con trỏ hiện hành của tenant A — loại nháp, `exam_only` ngoài đề thi, platform chưa gán, efficacy instrument được gán; cùng seed cùng kết quả; thiếu pool → `QUESTION_POOL_INSUFFICIENT`; tenant B pool rỗng; version 2 thay version 1 sau release; consumer xóa cache Valkey theo tenant, bỏ qua `item_type` khác, xóa toàn bộ với event platform; metric `eduworld_qb_grade_total` tăng) | AC-26, AC-27, AC-28 |

Lệnh (Git Bash, không có `make`, giới hạn CPU):

```bash
cd src/backend
GOMAXPROCS=2 go test -p 2 ./modules/learning/... -count=1   # 12 package ok; question 90%, normalize 100%, numeric 95,5%, selection 79,8% coverage
GOMAXPROCS=2 TESTCONTAINERS_RYUK_DISABLED=true go test -p 2 -tags integration -timeout 40m ./modules/learning/tests/integration -count=1   # 33/33 pass (F020 + F021 + F022)
golangci-lint run --allow-parallel-runners --concurrency 2 ./modules/learning/...            # 0 issues (cả .golangci.arch.yml)
go run ./tools/routematrix check   # 509 operation, 0 problems
```

Chưa chạy được trên máy này: `make migrate-lint` (Atlas container), `make specs-breaking` (oasdiff) — thay đổi OpenAPI là thêm operation `getCmsQuestionVersion` và viết lại `QuestionAuthoringInput` của feature chưa phát hành; trace_id bước 1 và 9 xem log `usecase=learning.command.CreateQuestion` / `DeleteQuestion` của test (không chạy stack local).
