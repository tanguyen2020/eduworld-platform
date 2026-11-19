# Quickstart: F053 — Workspace B — cây curriculum, editor nội dung, công thức, song ngữ, trình soạn câu hỏi

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F021, F022, F027, F051 (F023 cho 3 dạng nâng cao); delta contract T005–T009 có backend.
- Lệnh ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)): `make up`, `make seed`, `make up-app`; `cd src/cms/frontend && pnpm gen:api && pnpm gen:schemas && pnpm dev`; `pnpm storybook`.
- Tài khoản demo: `cm.noidung` (content_admin), `cm.duyet` (content_reviewer), `qlcm.truong.a` (academic_manager `th-demo-a`), `ad.truong.b` (tenant khác), `sa.admin`.
- Ảnh mẫu PNG có alt text; file âm thanh đề bài cho khối 3.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `cm.noidung` mở "Tạo câu hỏi", duyệt qua 11 dạng | Mỗi dạng có form riêng | F053-AC-01 |
| 2 | Tạo `single_choice` 4 phương án, chọn đáp án, xóa phương án đang là đáp án | ID dạng `o_xxxx`; lưu bị chặn đến khi chọn lại đáp án | F053-AC-02 |
| 3 | Thêm feedback + `misconception_tag` cho phương án sai, 3 gợi ý; mở "Như học sinh thấy" | Preview không có feedback/gợi ý/đáp án | F053-AC-03 |
| 4 | Đặt khối 3, không có audio đề → Gửi duyệt | Nút khóa, liệt kê field thiếu; thêm audio → mở | F053-AC-04 |
| 5 | Chèn ảnh mới (upload) | Bắt buộc license + alt text; trạng thái tới `ready` | F053-AC-05 |
| 6 | Gõ liên tục 30 giây | "Đang lưu…" → "Đã lưu lúc HH:mm"; request có `If-Match` | F053-AC-06 |
| 7 | Mở cùng câu ở tab thứ hai, sửa và lưu; quay lại tab đầu sửa | Banner xung đột, không ghi đè | F053-AC-07 |
| 8 | `cm.noidung` Gửi duyệt; `cm.duyet` Duyệt, Xuất bản ngay (bấm hai lần) | Một release; thanh trạng thái "Đã xuất bản v1"; tác giả không thấy nút Duyệt | F053-AC-09, F053-AC-10 |
| 9 | Mở lại câu v1 | Chỉ đọc + "Tạo bản nháp từ v1" → v2 draft | F053-AC-08 |
| 10 | `qlcm.truong.a` mở câu platform đã gán cho `th-demo-a` | Nhãn "Từ EduWorld", chỉ đọc | F053-AC-11 |
| 11 | `sa.admin` tạo câu không chọn phạm vi | Bắt buộc chọn Platform hoặc tenant đích | F053-AC-12 |
| 12 | `cm.duyet` mở editor | Không có nút tạo/sửa; có Duyệt | F053-AC-13 |
| 13 | Dán chuỗi dài hơn giới hạn vào phương án, `change_note` | Bị chặn + bộ đếm | F053-AC-14 |
| 14 | Thu nhỏ < 1024 px | "Tác vụ này cần màn hình lớn hơn", xem chỉ đọc | F053-AC-15 |
| 15 | Nhập `\frac{3}{4}` bằng bàn phím ảo, `\ce{2H2 + O2 -> 2H2O}`; lưu | Request có `math_assets` (SVG `currentColor` + speech) | F053-AC-16, F053-AC-17 |
| 16 | Nhập `\href{x}{y}` / công thức sai cú pháp | Chặn ở client; nếu gửi thẳng API → `422 MATH_RENDER_FAILED`, công thức tô đỏ | F053-AC-17, F053-AC-18 |
| 17 | Xem bộ công thức mẫu theme sáng/tối | Khớp ảnh chuẩn | F053-AC-19 |
| 18 | Câu có công thức chọn dạng `short_answer` | Cảnh báo ADR-0055 | F053-AC-20 |
| 19 | Soạn đề vi/en, xóa bản `vi` (gốc) | Hai cột, nhãn "Gốc"; huy hiệu Thiếu; Gửi duyệt khóa | F053-AC-21, F053-AC-22, F053-AC-23 |
| 20 | Câu `vi` không có `en` → xuất bản | Thành công, cảnh báo `partial` | F053-AC-24 |
| 21 | Xuất XLIFF, sửa `en`, nhập lại | Job xong, Draft mới, báo field không khớp | F053-AC-25 |
| 22 | Mở "Chương trình học" Toán 3, kéo chủ đề | Cây đúng; `updateTopic` kèm `If-Match`; lỗi → hoàn tác | F053-AC-26, F053-AC-27 |
| 23 | Thêm LO tiên quyết tạo chu trình | Lỗi hiển thị | F053-AC-28 |
| 24 | `qlcm.truong.a` xem curriculum platform; tạo curriculum riêng tenant | Chỉ đọc với platform; tạo riêng được | F053-AC-29 |
| 25 | `cm.noidung` gửi duyệt curriculum version thiếu `yccd_ref` | Danh sách LO thiếu | F053-AC-30 |
| 26 | Thêm khối `worked_example` 3 bước; gắn vào câu hỏi | Lưu đúng; câu có `worked_example_ref` | F053-AC-31, F053-AC-32 |
| 27 | `ad.truong.b` mở URL câu hỏi của `th-demo-a` | "Không tìm thấy" | F053-AC-13 |
| 28 | `sa.admin` tắt `action.content_studio.question_bank.publish` cho `th-demo-a`; `cm.duyet` mở câu hỏi đã duyệt | Nút "Xuất bản" ẩn/khóa với tooltip "Tính năng đang tắt"; gọi thẳng `publishContentVersion` → `403 FEATURE_DISABLED` (`feature_key`) | F053-AC-33 |

## 3. Lệnh test tự động

```bash
make web-lint
make web-test
cd src/cms/frontend && pnpm --filter portal test:e2e -- --grep @F053
make storybook
```

## 4. Bằng chứng cần nộp

- Kết quả test (số pass/fail); ảnh visual regression công thức (sáng/tối); HAR kịch bản 15; `trace_id` kịch bản 8.

## 5. Bằng chứng (2026-11-19, local — Windows, Node 20.19, pnpm 10.34.9; Vitest 3 worker theo giới hạn CPU)

Backend module learning chưa nối route vào binary (OQ F020-3) nên các kịch bản §2 được kiểm tự động bằng **MOCK backend hợp đồng**: `apps/portal/src/test/mock-backend.ts` (F051) + `apps/portal/src/test/mock-content.ts` (câu hỏi/version có ETag theo row version, `412`/`428`, transition/publish với phát lại Idempotency-Key, preview, media, dịch, curriculum). **Không phải tích hợp thật**; MathLive là MOCK module trong jsdom (component thật chạy trong Storybook). E2E Playwright `@F053` với backend thật, HAR, `trace_id`, ảnh visual công thức chờ OQ F051-9.

| Kịch bản / tiêu chí | Bằng chứng tự động | Kết quả |
|---|---|---|
| 1, 2 · AC-01, AC-02 | `ops-ui/question-editor/mappers/mappers.test.ts` (11 dạng: bản nháp hoàn chỉnh hợp lệ theo `question.schema.json` bằng ajv, round-trip, không khóa đáp án ở gốc `render_payload`; bản nháp trống báo thiếu), `itemId.test.ts` (pattern, ngẫu nhiên, không trùng, đáp án trỏ id đã xóa → chặn lưu) | 25/25, 4/4 pass |
| 3 · AC-03 | `feedbackHints.test.tsx` (feedback + `misconception_tag` chỉ ở phương án sai, ≤ 64/≤ 500, ≤ 5 gợi ý, preview học sinh không có feedback/tag/đáp án), `mappers.test.ts` › student view | 3/3 pass |
| 4 · AC-04 | `QuestionEditorPage.test.tsx` › AC-04 (khối 3 thiếu audio → Gửi duyệt khóa + liệt kê, lý do miễn → mở), `shared/gates.test.ts` (map `422 AUDIO_REQUIRED` `/stem/audio_ref`) | pass, 7/7 |
| 5 · AC-05 | `QuestionEditorPage.test.tsx` › AC-05 (bắt buộc license + alt, presign → PUT → complete → poll tới "Sẵn sàng"), `gates.test.ts` › media gate | pass |
| 6, 7 · AC-06, AC-07 | `ops-ui/editor/useAutosave.test.ts` (debounce, ép lưu theo `content_autosave_interval`, một request mỗi lúc, ETag mới nhất, `409`/`412` dừng, lỗi mạng thử lại), `QuestionEditorPage.test.tsx` › AC-06/AC-07 (PATCH merge-patch + `If-Match: W/"1"`, "Đã lưu lúc HH:mm", `409 CONTENT_VERSION_CONFLICT` → banner, không PATCH thêm, giữ văn bản) | 6/6, pass |
| 8, 9 · AC-08…AC-10 | `ItemStatusBar.test.tsx` (nút theo trạng thái/quyền, tác giả không thấy Duyệt/Từ chối, published chỉ "Tạo bản nháp từ v{n}"), `QuestionEditorPage.test.tsx` › AC-08 (`createQuestionVersion {from_version: 1}` + Idempotency-Key), AC-09/AC-10 (approve `If-Match` + key; publish lỗi rồi bấm lại **cùng** Idempotency-Key, 1 release, "Đã xuất bản v1") | 8/8, pass |
| 10, 11 · AC-11, AC-12 | `QuestionEditorPage.test.tsx` › AC-11 (`qlcm.truong.a`: "Từ EduWorld", không thao tác), AC-12 (`sa.admin` Lưu khi chưa chọn → "Chọn một tenant đích.", không POST) | pass |
| 12 · AC-13, AC-33 | `QuestionEditorPage.test.tsx` › AC-13/AC-33 (quyền chỉ review: không Lưu/Gửi duyệt; action publish tắt → nút khóa; `403 PERMISSION_DENIED` → thông điệp) | pass |
| 13 · AC-14 | `shared/schemaValidator.test.ts` (map lỗi ajv về field path, `maxLength` 1000/500/64, `alt` bắt buộc, ≤ 5 gợi ý; bản sao schema = specs), bộ đếm + `maxlength` trong `BilingualField.test.tsx`, `feedbackHints.test.tsx` | 5/5 pass |
| 14 · AC-15 | `QuestionEditorPage.test.tsx` › AC-12/AC-15 (800 px → "Tác vụ này cần màn hình lớn hơn", không nút ghi) | pass |
| 15, 16 · AC-16…AC-18 | `ops-ui/math/renderMath.test.ts` (SVG `currentColor`, speech vi/en, `math_id` sha256, `\ce`, chặn `\href`/`\require`/`\html…`/macro/màu, `latex_max_length`/`latex_max_depth`), `MathInput.test.tsx` (tab Toán/Lý/Hóa, `\( \)`/`\[ \]`, tô đỏ lỗi cục bộ và `MATH_RENDER_FAILED`, nút chèn) | 7/7, 8/8 pass |
| 17 · AC-19 | story `MathText` sáng/tối/compact (TC-UI-011); `MathInput.test.tsx` › preview `role="img"` + speech | story có, visual chờ baseline |
| 18 · AC-20 | `MathInput.test.tsx` › F053-AC-20 (short_answer có công thức → cảnh báo, single_choice → không), `mappers.test.ts` | pass |
| 19, 20 · AC-21…AC-24 | `ops-ui/i18n-field/BilingualField.test.tsx` (cột gốc trước + "Gốc", tab VI/EN, huy hiệu Đủ/Thiếu/Lỗi thời/Thiếu bản gốc), `shared/gates.test.ts` (thiếu bản gốc → chặn, `TRANSLATION_INCOMPLETE` → field, thiếu bản dịch → chỉ cảnh báo `partial`) | 6/6, pass |
| 21 · AC-25 | `QuestionEditorPage.test.tsx` › AC-25 (`exportLearningTranslations` có Idempotency-Key, job được poll, link tải) | pass |
| 22, 23 · AC-26…AC-28 | `curriculum/treeModel.test.ts` (Môn → khối → chủ đề → LO, drop → `parent_topic_id`/`sort_order`, chặn khác khối/vào cây con), `TopicTree.test.tsx` (hiển thị ngay, server từ chối → hoàn tác + lỗi, `If-Match: W/"<row_version>"`), `CurriculumTreePage.test.tsx` (5 operation đọc; ObjectiveForm `PREREQUISITE_CYCLE`, tối đa 20) | 4/4, 3/3, 4/4 pass |
| 24, 25 · AC-29, AC-30 | `CurriculumTreePage.test.tsx` › AC-29 (curriculum platform chỉ đọc với tenant, có "Tạo chương trình riêng"), › AC-30 (mã LO thiếu `yccd_ref` tách từ 422) | pass |
| 26 · AC-31, AC-32 | `ops-ui/rich-content/WorkedExampleBlock.test.tsx` (1–8 bước, audio khối 1–3, id duy nhất), `questions/workedExampleRef.test.tsx` (lọc lesson theo LO, khối `worked_example`) | 3/3, 2/2 pass |
| 27 | Cô lập tenant do server (404); portal hiển thị "Không tìm thấy câu hỏi" khi `getCmsQuestion` 404 | E2E chờ OQ F051-9 |
| 28 · AC-33 | như kịch bản 12 | pass |

Tổng: `pnpm lint` 0 lỗi, `pnpm typecheck` 0 lỗi, `pnpm gen:check` khớp (OpenAPI + danh mục + 2 schema), `vitest run` (3 worker) trước rebase **51 file / 335 test pass** (398 s), sau rebase lên main có F023/F024/F057 **60 file / 411 test pass** (1 602 s); trong đó F053: 73 test ops-ui + 34 test portal. Một lượt chạy riêng có 1 test F051 (`catalogMenus` › 403 FEATURE_DISABLED) fail do tải CPU, chạy lại pass 8/8. `make specs-breaking` không chạy được ở máy local: oasdiff v1.11.7 (docker) không đọc được cả spec gốc trên main (lỗi `items` kiểu bool), mọi file kể cả file không đổi đều báo lỗi — delta F053 chỉ thêm trường tùy chọn/readOnly, `pnpm build` OK (5 chunk workspace, MathJax trong chunk `vendor-mathjax` + font theo range, MathLive chunk riêng, không chunk nào trong closure của entry), `check-modes` 17 component có story Light/Dark/Compact. Liên repo: `check-features` 0 lỗi, `check-portal-catalog` 0 lỗi, `check-problem-codes` 0 lỗi, `check-maxlength` 0 vi phạm, `check-enums`/`validate-schemas` 0 lỗi, `split-public --check`, `sync-platform-ops --check` khớp.

Chưa làm/không tự động: E2E Playwright `@F053` và axe toàn trang (T004, T024–T026, T035, T042, T049, T056, T060), baseline visual cho 14 story mới (`make web-visual`, baseline Linux do CI tạo), HAR kịch bản 15 và `trace_id` kịch bản 8 — chờ backend learning nối vào binary (OQ F020-3, OQ F051-9).
