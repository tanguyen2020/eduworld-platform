# Feature Specification: F053 — Workspace B — cây curriculum, editor nội dung, công thức, song ngữ, trình soạn câu hỏi

**Feature Branch**: `053-content-studio-editor` · **Created**: 2026-11-16 · **Status**: Done · **Hoàn tất**: 2026-11-19 · **Module**: `src/cms/frontend/apps/portal` (workspace `content_studio`) + `src/cms/frontend/packages/ops-ui` · **Mốc**: M-C2 · **Phụ thuộc**: F021, F022, F027, F051
**Input**: Phần soạn thảo của Learning Content Studio: cây curriculum (môn, curriculum, chủ đề, LO), khung editor dùng chung `ContentEditorShell` (autosave, khóa lạc quan, thanh trạng thái, thao tác workflow cơ bản), công thức (MathLive nhập, MathJax 4 hiển thị, SVG render trên trình duyệt), biên tập song ngữ, trình soạn 11 dạng câu hỏi (phản hồi theo phương án, gợi ý nhiều cấp, lồng tiếng `g1_3`), khối `worked_example`. FR nguồn: FR-CMS-002, FR-CMS-003, FR-CNT-017, FR-CNT-018, FR-CNT-020, FR-QB-001, FR-QB-013.

## Clarifications

### Session 2026-11-16

- Q: M-C2 cần "soạn → duyệt → publish" trên portal nhưng hàng đợi duyệt, lịch sử version, rollback thuộc F054 (M-C3)? → A: F053 đưa vào thanh trạng thái của editor các thao tác trên **một item**: Gửi duyệt, Duyệt, Từ chối, Xuất bản ngay, Tạo bản nháp từ version đã publish. Hàng đợi duyệt, hẹn giờ, lịch sử/diff, rollback, gỡ xuất bản, archive thuộc F054.
- Q: Gửi SVG công thức cho câu hỏi/bài học bằng trường nào? → A: `ActivityDefinitionInput.math_assets` đã có; `QuestionAuthoringInput`, `LessonAuthoringInput` chưa có → "Cần sửa" ([contracts](contracts/README.md) §6).
- Q: `hints[]`, `worked_example_ref`, `audio_exempt_reason` trong `QuestionAuthoringInput`? → A: Theo `specs/schemas/question.schema.json`; contract authoring cần khai báo tường minh → "Cần sửa".
- Q: Lỗi thiếu lồng tiếng `g1_3`? → A: [CONTENT_MODEL §8.1](../../docs/02-learning/CONTENT_MODEL.md) dùng `422 AUDIO_REQUIRED`; enum `ProblemCode` chưa có → "Cần thêm".
- Q: Curriculum version, gửi duyệt/xuất bản curriculum, import YAML/CSV? → A: Contract CMS chưa có (CURRICULUM_MODEL §9) → "Cần thêm", backend F021.

### Session 2026-11-19 (triển khai — câu hỏi mở và quyết định [Confirmed])

- Q: Delta contract T005–T009 khi triển khai? → A: `hints`, `worked_example_ref`, `math_assets` (QuestionAuthoringInput), `AUDIO_REQUIRED`, `listCurriculumVersions`/`transitionCurriculumVersion`/`publishCurriculumVersion` đã có sẵn trên main (F021/F022/F027). F053 thêm: `QuestionAuthoringInput.audio_exempt_reason` (≤ 255), `LessonAuthoringInput.math_assets[]`, `ContentVersion.outdated_fields[]` (readOnly, ≤ 500 × 256). [Confirmed] **Không** thay `stem`/`render_payload`/`answer_key` bằng `$ref` ngoài tới `question.schema.json` (công cụ tách spec/sinh type không xử lý `$ref` ngoài file); portal kiểm từng phần bằng ajv trên bản sao sinh của schema (research R2), server kiểm lại (F022). Backend điền `outdated_fields`, nhận `audio_exempt_reason` thuộc F025; `LessonAuthoringInput.math_assets` thuộc F024.
- Q: `CurriculumVersion.missing_yccd_objective_ids[]` (contracts §6)? → A: [Confirmed] không thêm: F021 đã trả `422 VALIDATION_FAILED` với `errors[].field = yccd_ref` và danh sách mã LO trong `message` (F021-AC-14); portal tách mã LO từ đó. Trường có cấu trúc để lại cho F021 nếu cần.
- Q: Kéo-thả chủ đề cần `If-Match` nhưng danh sách topic/LO không có ETag từng dòng? → A: Delta mới (Cần thêm, backend F021): `Topic.row_version`, `LearningObjective.row_version` (readOnly) — portal gửi `If-Match: W/"<row_version>"`. Tới khi F021 trả trường này, server trả `428` và cây hoàn tác + báo lỗi (đúng F053-AC-27).
- Q: ETag nào cho `updateQuestionDraft`? → A: ETag của **version** (`getCmsQuestionVersion`); ETag của `getCmsQuestion` chỉ dùng cho `deleteQuestion` (hiện thực F022). Portal đọc question rồi version.
- Q: Speech text tiếng Việt cho công thức? → A: [Confirmed] speech-rule-engine của MathJax 4 không có locale `vi`; portal sinh `speech_text` bằng bộ đọc quy tắc nhỏ (vi/en) trên cây MathML của MathJax 4 (phân số, căn, lũy thừa, chỉ số, quan hệ, mũi tên mhchem). Đổi engine khi có TTS/SRE tiếng Việt.
- Q: "feedback/misconception lưu ở phần đáp án" so với contract? → A: Theo contract (`QuestionAuthoringInput`, `question.schema.json#/$defs/option`), `feedback`/`misconception_tag` nằm trong `render_payload.options[]` khi gửi lên, server tách khỏi DTO học sinh; preview "như học sinh thấy" không bao giờ hiển thị chúng.
- Q: `question.schema.json` khai báo `worked_example_ref` là uuid, OpenAPI là `{content_item_id, block_id}`? → A: F024 (trên main) đã đổi schema thành `{content_item_id, block_id}` với `block_id` `^we_[a-z0-9_]{1,40}$`; portal theo đúng shape này, id khối `worked_example` sinh theo cùng pattern và đọc từ `block.id` (lesson.schema.json).
- Q: Kéo-thả? → A: [Confirmed] dùng Tree kéo-thả của Ant Design thay `@dnd-kit` (cùng kết quả, không thêm phụ thuộc); khối nội dung chuyển bằng nút Lên/Xuống (dùng được bàn phím).
- Q: `assets_per_version_max` là platform setting không đọc được từ portal? → A: dùng 50 = `media_refs.maxItems` của contract; server quyết định.
- Q: Màn danh sách/tìm kiếm câu hỏi? → A: thuộc F054; màn "Ngân hàng câu hỏi" của F053 có "Tạo câu hỏi" và "Mở câu hỏi theo mã".
- Q: E2E Playwright với backend thật (T004, T024–T026, T035, T042, T049, T056, T060, T064)? → A: Như F051 (OQ F051-9), backend chưa nối route vào binary (OQ F020-3): mọi F053-AC-* được kiểm ở mức unit/integration với MOCK backend hợp đồng (`src/test/mock-content.ts`); E2E backend thật và baseline visual cho story mới chờ chạy ở CI/khi backend nối xong.

### Session 2026-11-20 (owner chốt)

- Delta backend F053 cần (`Topic/LearningObjective.row_version` F021, `outdated_fields`/`audio_exempt_reason` F025, `LessonAuthoringInput.math_assets` F024) → đã áp 2026-11-20 (Session "follow-up OQ F053" của ba feature).
- E2E backend thật, axe toàn trang và baseline visual story mới (T004, T024–T026, T035, T042, T049, T056, T060) → **Chốt:** chạy khi có môi trường compose/CI có Chromium (OQ F051-9) — không phải câu hỏi mở.

### Session 2026-11-21 (sửa lỗi E2E portal trên stack local)

- Q: `sa.admin` mở "Chương trình học" nhận `422 VALIDATION_FAILED` — bên nào sai contract? → A: Portal: `listCmsTopics`/`listCmsObjectives` gửi `limit=200` trong khi `components/parameters/Limit` là 1–100 (kiểu bị che bằng `as never`). Sửa portal: `limit ≤ 100` (`LIST_LIMIT_MAX`), query có kiểu theo `schema.gen.d.ts`; `listCmsTopics` chỉ gọi khi có `curriculum_version_id` (bắt buộc trong contract). Backend đúng contract, không đổi.
- **OQ F053-9 (owner chốt 2026-11-21 — [Confirmed], việc T066):** bộ chọn chủ đề của `DocMetaFields` và `QuestionActivityEditor` chỉ biết môn (± khối), không có `curriculum_version_id` nên `listCmsTopics` không gọi được theo contract (trước đây nhận `422`, nay không gửi request, danh sách chủ đề trống). Cần chốt: contract cho phép `subject_id` thay `curriculum_version_id` (chủ đề của version hiện hành), hay portal tự suy version hiện hành từ `listCmsCurricula`. [Proposed] portal suy version hiện hành theo môn × khối — chưa làm.

### Session 2026-11-22 (triển khai — [Confirmed], owner chốt 2026-11-20)

- Q: T066 (OQ F053-9) — portal suy `curriculum_version_id` hiện hành thế nào? → A: [Confirmed] Không đổi contract. Portal đọc `listCmsCurricula` (curriculum tenant thấy: của tenant và platform được gán), với mỗi curriculum có `current_version` (con trỏ version đang hiệu lực, CURRICULUM_MODEL §2) gọi `getCurriculumVersion` để lấy `subject_grades`, chọn version có cặp môn × khối (chỉ một khối được chọn → khối đó; nhiều hoặc chưa chọn khối → chỉ theo môn), rồi gọi `listCmsTopics` với `curriculum_version_id` + `subject_id` (+ `grade_level`). Nhiều curriculum khớp → ưu tiên curriculum riêng của tenant, rồi theo thứ tự danh sách (bảo thủ: dùng đúng chương trình tenant tự soạn, không trộn chủ đề nhiều chương trình). Version nháp/đang duyệt không bao giờ dùng. Ô chọn hiện dòng "Chủ đề theo chương trình …, phiên bản …" để người soạn thấy nguồn. Không có version khớp → không gửi `listCmsTopics`, ô chọn hiện giải thích "Chưa có chương trình học đã xuất bản cho môn và khối này…". Cài đặt curriculum mặc định của trường (`lms.school_curriculum_settings`, SCHOOL_LMS §3) không dùng ở đây vì nội dung Content Studio không có chiều trường. LO giữ nguyên cách lọc hiện có (ngoài phạm vi T066).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Soạn câu hỏi 11 dạng và đưa qua workflow (Priority: P1)

Biên tập viên (Tenant Content Editor, Academic Manager, Content Admin) tạo câu hỏi theo dạng, nhập đề/phương án/đáp án/giải thích, phản hồi theo phương án sai, gợi ý nhiều cấp, ảnh/âm thanh; bản nháp tự lưu; gửi duyệt; reviewer duyệt và xuất bản ngay. Sửa câu đã xuất bản tạo bản nháp mới. (US-CMS-001)

**Why this priority**: Câu hỏi là nội dung lõi của Content Loop (exit M-C2).

**Independent Test**: `cm.noidung` tạo câu `single_choice` có ảnh, `cm.duyet` duyệt và xuất bản; học sinh nhận version mới (kiểm bằng render payload, không có đáp án).

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F053-AC-01 | Portal PHẢI cung cấp form soạn riêng cho từng giá trị của enum `QuestionType` (11 dạng) với trường theo [QUESTION_BANK §3](../../docs/02-learning/QUESTION_BANK.md). | FR-QB-001 | — |
| F053-AC-02 | Portal PHẢI sinh ID phương án/mục dạng `itemId` ngẫu nhiên ổn định trong version (không A/B/C/D, không chỉ số) và chặn lưu khi `answer_key` tham chiếu ID không tồn tại hoặc thiếu đáp án theo dạng. | FR-QB-001 · [QUESTION_BANK §2](../../docs/02-learning/QUESTION_BANK.md) | — |
| F053-AC-03 | KHI soạn `single_choice`/`multiple_choice`, Portal PHẢI cho nhập `misconception_tag` và `feedback` song ngữ cho từng phương án sai và tối đa 5 cấp `hints[]`, lưu ở phần đáp án; chế độ "như học sinh thấy" KHÔNG hiển thị feedback/gợi ý trước khi chấm. | FR-QB-013 · [QUESTION_BANK §2](../../docs/02-learning/QUESTION_BANK.md) | — |
| F053-AC-04 | Ở NƠI `grade_levels` của câu hỏi giao {1, 2, 3}, Portal PHẢI đánh dấu `stem.audio_ref` bắt buộc, khóa Gửi duyệt/Xuất bản khi thiếu (trừ khi có `audio_exempt_reason`) và hiển thị `422 AUDIO_REQUIRED` liệt kê field thiếu. | [CONTENT_MODEL §8.1](../../docs/02-learning/CONTENT_MODEL.md) | TC-CNT-010 |
| F053-AC-05 | KHI chèn ảnh/âm thanh, Portal PHẢI cho chọn asset có sẵn (`listMediaAssets`) hoặc upload mới (`presignMediaUpload` + PUT), bắt buộc `license` và `alt_text` với ảnh, và hiển thị trạng thái pipeline đến `ready`/`rejected`. | FR-CMS-003 · [CONTENT_MODEL §7](../../docs/02-learning/CONTENT_MODEL.md) | TC-CNT-008 |
| F053-AC-06 | TRONG KHI soạn bản nháp, Portal PHẢI autosave theo chu kỳ của [ADMIN_PORTAL §1.4](../../docs/04-admin-portal/ADMIN_PORTAL.md) bằng `updateQuestionDraft` kèm `If-Match` và hiển thị "Đang lưu…" / "Đã lưu lúc HH:mm" / "Lỗi lưu, thử lại". | FR-CMS-003 | — |
| F053-AC-07 | NẾU lưu nhận `412`/`409 CONTENT_VERSION_CONFLICT`, THÌ Portal PHẢI dừng autosave, giữ thay đổi cục bộ, báo người khác đã sửa và cho tải bản mới; không ghi đè. | FR-CMS-003 · ADR-0043 | TC-CNT-003 |
| F053-AC-08 | KHI mở câu hỏi có version `published`, Portal PHẢI hiển thị chỉ đọc, không có nút Sửa, chỉ có "Tạo bản nháp từ v{n}" gọi `createQuestionVersion`. | ADR-0015, ADR-0043 · [CONTENT_MODEL §5](../../docs/02-learning/CONTENT_MODEL.md) | TC-CMS-001 |
| F053-AC-09 | KHI người dùng bấm Gửi duyệt / Duyệt / Từ chối trên thanh trạng thái, Portal PHẢI gọi `transitionContentVersion` (`submit_review`/`approve`/`reject`, `If-Match`, Idempotency-Key), bắt buộc lý do khi từ chối, và ẩn Duyệt/Từ chối khi người dùng là tác giả của version. | FR-CMS-003, US-CMS-001 · [CONTENT_MODEL §6](../../docs/02-learning/CONTENT_MODEL.md) | TC-CMS-001 |
| F053-AC-10 | KHI người có `learning_content.publish` bấm "Xuất bản ngay" trên version `approved`, Portal PHẢI gọi `publishContentVersion` (Idempotency-Key, `change_note`) và cập nhật thanh trạng thái thành "Đã xuất bản v{n}"; bấm lại không tạo release thứ hai. | FR-CMS-003 · ADR-0043 | TC-CNT-003 |
| F053-AC-11 | TRONG KHI nội dung là dữ liệu platform được gán cho tenant (`assigned = true`), Portal PHẢI hiển thị nhãn "Từ EduWorld" và chế độ chỉ đọc. | AC-CMS-005 · ADR-0029 | TC-CMS-ASG-02 |
| F053-AC-12 | KHI System Admin tạo nội dung, Portal PHẢI bắt buộc chọn tenant đích hoặc phạm vi Platform (`owner_scope`) và gửi `X-Tenant-Id`/`tenant_id` tương ứng. | AC-CMS-003 · ADR-0027 | TC-CMS-TEN-03 |
| F053-AC-13 | Portal PHẢI ẩn tạo/sửa khi thiếu `learning_content.write`, ẩn Duyệt/Từ chối khi thiếu `learning_content.review`, ẩn Xuất bản khi thiếu `learning_content.publish`, và hiển thị thông điệp không có quyền khi server trả `403 PERMISSION_DENIED`. | AC-CMS-001 · [ADMIN_PORTAL §2.3](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F053-AC-14 | Portal PHẢI giới hạn mọi trường văn bản (đề, phương án, giải thích, feedback, gợi ý, `change_note`, `alt_text`) theo `maxLength` của OpenAPI/`question.schema.json`, hiển thị bộ đếm và ánh xạ `422 VALIDATION_FAILED` về đúng field path. | ADR-0041 | TC-TEN-024 |
| F053-AC-15 | Ở NƠI độ rộng < 1024 px, Portal PHẢI hiển thị "Tác vụ này cần màn hình lớn hơn" cho Question Editor và chỉ cho xem chỉ đọc. | [ADMIN_PORTAL §1.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |

---

### User Story 2 — Công thức Toán/Lý/Hóa (Priority: P1)

Biên tập viên nhập công thức trực quan bằng bàn phím ảo Toán/Lý/Hóa; portal hiển thị bằng MathJax 4 và render sẵn SVG cho game.

**Why this priority**: Exit M-C2 yêu cầu "câu hỏi có công thức + ảnh".

**Independent Test**: Nhập `\frac{3}{4}` và `\ce{H2O}` trong đề, lưu → request có `math_assets` (SVG + speech text); nhập `\href{...}` → bị chặn.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F053-AC-16 | Portal PHẢI cho nhập công thức bằng trình nhập trực quan có bàn phím ảo tab Toán/Lý/Hóa ở mọi trường rich text của editor câu hỏi và khối nội dung, lưu LaTeX inline `\( \)` hoặc block `\[ \]`. | FR-CNT-017 · ADR-0055 | TC-UI-012 |
| F053-AC-17 | KHI lưu hoặc gửi duyệt nội dung có công thức, Portal PHẢI render mỗi công thức trên trình duyệt thành SVG đơn sắc (`currentColor`) + speech text và gửi kèm trong `math_assets`; NẾU LaTeX lỗi cú pháp hoặc vượt `latex_max_length`, THÌ chặn lưu và tô đỏ đúng công thức. | FR-CNT-017, AC-CNT-008 | TC-UI-012, TC-UI-013 |
| F053-AC-18 | NẾU server trả `422 MATH_RENDER_FAILED` kèm vị trí, THÌ Portal PHẢI tô đỏ đúng công thức, hiện thông điệp và khóa Gửi duyệt cho tới khi sửa. | FR-CNT-017, AC-CNT-008 | TC-UI-013, TC-MATH-002 |
| F053-AC-19 | Portal PHẢI hiển thị công thức ở editor, preview "như học sinh thấy" và danh sách bằng MathJax 4 (có `mhchem`), màu theo token theme sáng/tối, có speech/aria cho trình đọc màn hình. | FR-CNT-017 · [DESIGN_SYSTEM §4.6](../../docs/05-ux/DESIGN_SYSTEM.md) | TC-UI-011, TC-MATH-001 |
| F053-AC-20 | Ở NƠI câu hỏi có công thức dùng dạng đòi học sinh nhập biểu thức, Portal PHẢI hiển thị cảnh báo theo ADR-0055 mục 6 (học sinh không gõ biểu thức trong game). | FR-CNT-017 · ADR-0055 | — |

---

### User Story 3 — Biên tập song ngữ (Priority: P2)

Biên tập viên soạn trường văn bản hai cột vi/en, thấy trạng thái bản dịch, xuất/nhập file dịch.

**Why this priority**: Nội dung học tập chỉ bắt buộc bản `content_language`; bản dịch là bổ sung.

**Independent Test**: Câu `content_language = vi` không có bản `en` vẫn xuất bản được với `translation_status = partial`; thiếu bản `vi` bị khóa Gửi duyệt.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F053-AC-21 | Portal PHẢI hiển thị mọi trường văn bản học tập song ngữ thành hai cột vi/en (màn hẹp: tab VI / EN), cột `content_language` đứng trước có nhãn "Gốc". | FR-CNT-018 · ADR-0056 | — |
| F053-AC-22 | Portal PHẢI hiển thị huy hiệu Đủ/Thiếu/Lỗi thời theo `translation_status` ở từng trường và ở version. | FR-CNT-018 · [CONTENT_MODEL §8](../../docs/02-learning/CONTENT_MODEL.md) | — |
| F053-AC-23 | NẾU thiếu bản theo `content_language` ở trường bắt buộc, THÌ Portal PHẢI khóa Gửi duyệt/Xuất bản kèm danh sách field thiếu bấm để nhảy tới; NẾU server trả `422 TRANSLATION_INCOMPLETE`, THÌ hiển thị cùng cách. | FR-CNT-018, NFR-I18N-002 | — |
| F053-AC-24 | KHI chỉ thiếu bản dịch tùy chọn, Portal PHẢI cho xuất bản kèm cảnh báo `translation_status = partial`. | FR-CNT-018 · `learning_translation_required` | TC-I18N-002 |
| F053-AC-25 | KHI người có `learning_content.translate` xuất file dịch, Portal PHẢI gọi `exportLearningTranslations` (`xliff`/`csv`, Idempotency-Key), theo dõi `getLearningTranslationJob` và cho tải file; KHI nhập lại qua `importLearningTranslations`, Portal PHẢI báo field không khớp và Draft mới được tạo. | FR-CNT-018 · [ADMIN_PORTAL §1.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |

---

### User Story 4 — Cây curriculum (Priority: P2)

Content Admin (platform) và Academic Manager (curriculum riêng của tenant) quản lý môn, curriculum theo môn × khối, cây chủ đề kéo-thả, LO có mã `yccd_ref` và LO tiên quyết.

**Why this priority**: Câu hỏi gắn LO; curriculum chuẩn được seed sẵn (F021) nên editor câu hỏi có thể làm trước.

**Independent Test**: `cm.noidung` thêm chủ đề con, kéo-thả đổi thứ tự, thêm LO có tiên quyết; tạo chu trình tiên quyết → lỗi.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F053-AC-26 | KHI người có `curriculum.read` mở "Chương trình học", Portal PHẢI hiển thị cây Môn → Curriculum (môn × khối) → Chủ đề (lồng nhau) → LO từ `listCmsSubjects`, `listCmsCurricula`, `listCmsTopics`, `listCmsObjectives`. | FR-CMS-002 · [CURRICULUM_MODEL §2](../../docs/02-learning/CURRICULUM_MODEL.md) | — |
| F053-AC-27 | KHI người có `curriculum.write` kéo-thả chủ đề, Portal PHẢI gọi `updateTopic` với `parent_topic_id`/`position` mới kèm `If-Match` và hoàn tác hiển thị nếu server từ chối. | FR-CMS-002 | — |
| F053-AC-28 | KHI tạo/sửa LO, Portal PHẢI cho chọn LO tiên quyết (≤ `maxItems` của `prerequisite_ids`) và hiển thị lỗi khi server từ chối vì tạo chu trình. | FR-CMS-002 · [CURRICULUM_MODEL §3.3](../../docs/02-learning/CURRICULUM_MODEL.md) | — |
| F053-AC-29 | TRONG KHI người dùng tenant xem curriculum platform được gán, Portal PHẢI hiển thị chỉ đọc; chỉ người có `curriculum.write` ở scope tenant (Academic Manager) thấy thao tác tạo curriculum riêng của tenant. | FR-CMS-002, AC-CMS-005 · [ADMIN_PORTAL §2.4](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-ASG-02 |
| F053-AC-30 | KHI người có `curriculum.publish` gửi duyệt hoặc xuất bản một curriculum version, Portal PHẢI gọi API workflow curriculum (Cần thêm) và hiển thị danh sách LO thiếu `yccd_ref` khi server từ chối. | FR-CMS-002 · [CURRICULUM_MODEL §10](../../docs/02-learning/CURRICULUM_MODEL.md) | — |

---

### User Story 5 — Khối `worked_example` trong nội dung (Priority: P3)

Biên tập viên soạn ví dụ mẫu từng bước trong bài học/học liệu, và gắn câu hỏi với ví dụ mẫu cùng LO.

**Why this priority**: [Confirmed] trong PRD; dùng lại khi Lesson Builder (F054) có.

**Independent Test**: Trong Storybook và trong Question Editor: thêm khối 3 bước, đặt `worked_example_ref` cho câu hỏi.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F053-AC-31 | Portal PHẢI cung cấp khối `worked_example` trong trình soạn rich content gồm đề bài, 1–8 bước (mỗi bước rich content + `audio_ref` tùy chọn, bắt buộc với khối lớp 1–3), câu tự giải thích tùy chọn và `id` duy nhất trong version. | FR-CNT-020 · [CONTENT_MODEL §9.2](../../docs/02-learning/CONTENT_MODEL.md) | — |
| F053-AC-32 | KHI soạn câu hỏi, Portal PHẢI cho chọn `worked_example_ref` từ khối `worked_example` của lesson cùng LO. | FR-CNT-020, FR-QB-013 | — |
| F053-AC-33 | KHI menu/action của feature (`menu.content_studio.question_bank`, `menu.content_studio.curriculum` và action con) đang tắt cho phạm vi phiên (`disabled_menus`/`disabled_actions` của `getMyPermissions`), Portal PHẢI ẩn menu và ẩn/khóa nút tương ứng (`<ActionGate actionKey>`); NẾU API trả `403 FEATURE_DISABLED`, Portal PHẢI hiện thông báo tính năng tạm tắt và tải lại quyền | FR-CMS-018 · ADR-0090 | TC-CMS-MENU-02 |

### Edge Cases

- NẾU mất mạng khi autosave, THÌ Portal PHẢI giữ bản nháp trong bộ nhớ, hiện "Lỗi lưu, thử lại" và thử lại khi có mạng; đóng tab có thay đổi chưa lưu → cảnh báo (AC-06).
- NẾU media còn `processing` khi Gửi duyệt, THÌ Portal PHẢI khóa nút và hiển thị `422 MEDIA_NOT_READY` nếu server trả (AC-05).
- NẾU số asset trong version vượt `assets_per_version_max`, THÌ Portal PHẢI chặn chèn thêm.
- NẾU đổi `QuestionType` khi đã có đáp án, THÌ Portal PHẢI xác nhận và xóa phần đáp án không tương thích.
- NẾU version đang `in_review`, THÌ editor chỉ đọc với tác giả (CONTENT_MODEL §5 quy tắc 2).
- NẾU nội dung `CONTENT_NOT_EDITABLE`, THÌ Portal PHẢI chuyển chỉ đọc và gợi ý "Tạo bản nháp" (AC-08).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CMS-002 | Cây curriculum: môn, curriculum, chủ đề kéo-thả, LO + tiên quyết; workflow curriculum version | [CURRICULUM_MODEL](../../docs/02-learning/CURRICULUM_MODEL.md), [ADMIN_PORTAL §3.1](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-003 | `ContentEditorShell`, Question Editor, chèn media, thao tác workflow một item | [ADMIN_PORTAL §1.4, §3.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CNT-017 | `MathInput`/`MathText`, render SVG + speech text trên trình duyệt | [CONTENT_MODEL §9.1](../../docs/02-learning/CONTENT_MODEL.md), ADR-0055 |
| FR-CNT-018 | `BilingualField`, trạng thái bản dịch, chặn thiếu bản gốc, xuất/nhập file dịch | [CONTENT_MODEL §8](../../docs/02-learning/CONTENT_MODEL.md), ADR-0056 |
| FR-CNT-020 | Khối `worked_example`, chọn `worked_example_ref` | [CONTENT_MODEL §9.2](../../docs/02-learning/CONTENT_MODEL.md) |
| FR-QB-001 | Form soạn 11 dạng câu hỏi | [QUESTION_BANK §3](../../docs/02-learning/QUESTION_BANK.md) |
| FR-QB-013 | Phản hồi theo phương án, gợi ý nhiều cấp, `misconception_tag` | [QUESTION_BANK §2](../../docs/02-learning/QUESTION_BANK.md) |
| FR-CMS-018 | Menu/action của feature theo danh mục `specs/portal/menu-catalog.yaml`, ẩn khi tắt, xử lý `403 FEATURE_DISABLED` | [ADMIN_PORTAL §2.7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |

### Key Entities

- **Câu hỏi và version câu hỏi**: dạng, đề, phần hiển thị, đáp án, cách chấm, giải thích, gợi ý, phản hồi theo phương án, metadata (khối, môn, chủ đề, LO, độ khó, ngôn ngữ gốc); version `published` bất biến.
- **Môn, curriculum, chủ đề, LO**: cây chương trình; LO có mã, `yccd_ref`, LO tiên quyết.
- **Công thức**: LaTeX + SVG + speech text gắn với version.
- **Media asset**: file đã qua pipeline, có license, alt text, trạng thái.
- **Ví dụ mẫu (`worked_example`)**: khối nội dung từng bước trong bài học.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `latex_max_length` / `latex_max_depth` / `math_svg_max` | Chặn công thức quá dài/sâu, SVG quá lớn trước khi gửi |
| `assets_per_version_max` | Giới hạn số asset chèn vào một version |
| `learning_translation_required` | Bản dịch nội dung học tập bắt buộc hay tùy chọn |
| `game_text_required_languages` | Không dùng cho nội dung học tập (chỉ nhắc để không áp nhầm) |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không hardcode câu hỏi/đáp án trong client; đáp án chỉ hiển thị cho người có quyền CMS | AGENTS R2, R6 |
| Không sửa version published; sửa = version mới | AGENTS R11, R31 · ADR-0043 |
| MathJax 4, MathLive ghim phiên bản cố định | ADR-0055 · [DESIGN_SYSTEM §4.6](../../docs/05-ux/DESIGN_SYSTEM.md) |
| Theme token, WCAG 2.1 AA, i18n UI | NFR-MAINT-007, NFR-A11Y-003, NFR-I18N-003 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Biên tập viên tạo một câu `single_choice` có công thức và ảnh, gửi duyệt trong ≤ 5 phút mà không cần hỗ trợ.
- **SC-002**: 100% công thức trong bộ mẫu Toán/Lý/Hóa hiển thị khớp ảnh chuẩn ở theme sáng/tối.
- **SC-003**: 0 trường hợp ghi đè bản nháp của người khác trong kịch bản hai người sửa đồng thời.
- **SC-004**: 0 version `published` sửa được từ portal.
- **SC-005**: Đủ 11 dạng câu hỏi soạn và lưu được (8 dạng cơ bản khi F022 Done, 3 dạng nâng cao khi F023 Done).

## Ngoài phạm vi

- Hàng đợi duyệt, hẹn giờ xuất bản, lịch sử/diff version, rollback, gỡ xuất bản, archive, thư viện media (màn danh sách), bộ câu hỏi, Lesson Builder, ma trận đề, preview "thử làm", import hàng loạt, thùng rác, tìm kiếm → F054.
- Learning Activity Builder theo `activity_type`, Video/Audio Manager, Simulation Configuration ([ADMIN_PORTAL §3.13](../../docs/04-admin-portal/ADMIN_PORTAL.md)) → chưa có feature portal sở hữu trong danh mục (ghi trong báo cáo); plugin backend F030, F065, F066.
- Chấm lại hệ thống (re-evaluate) từ trang version → feature LMS/learning (F031 backend), UI chưa có feature sở hữu.
- So sánh curriculum version + bảng mapping LO, mapping bộ sách → F021 (backend), UI bổ sung sau.
- Tự luận, rubric, bài nói → deferred (ADR-0082).

## Assumptions

- Chu kỳ autosave và luật khóa lạc quan theo [ADMIN_PORTAL §1.4](../../docs/04-admin-portal/ADMIN_PORTAL.md) (chưa đăng ký trong CONFIG_DEFAULTS — task T061).
- Lồng tiếng `g1_3` bắt buộc là [Confirmed] (PED-07, OQ-23); tắt/bật theo kết quả OQ thì chỉ đổi điều kiện hiển thị, server vẫn quyết định.
- F027 cung cấp pipeline media; F022/F023 cung cấp validator theo dạng câu hỏi.
