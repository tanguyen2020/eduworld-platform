# Feature Specification: F058 — Workspace B — Learning Activity Builder (quiz, interactive_exercise, video_lesson), UI re-evaluate

**Feature Branch**: `058-portal-activity-builder` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `web/apps/portal` (workspace `content-studio`) + `web/packages/ops-ui` · **Mốc**: M-C3 · **Phụ thuộc**: F030, F031, F053, F065
**Input**: Learning Activity Builder của Learning Content Studio cho `quiz`, `interactive_exercise` (nguồn câu hỏi, chấm, phản hồi, hoàn thành) và Video Manager cho `video_lesson` (video, chương, phụ đề, câu hỏi chèn, nhánh, quy tắc hoàn thành), preview "như học sinh thấy", thao tác workflow trên một activity, và màn **Chấm lại hệ thống** (re-evaluate) từ trang version câu hỏi/activity. FR nguồn: FR-CNT-013, FR-EXR-008, FR-CMS-003, FR-CMS-006, FR-CNT-014, FR-CNT-010, FR-QB-003.

## Clarifications

### Session 2026-11-16

- Q: Permission của builder là `learning_content.write` (ADMIN_PORTAL §3.13) hay `learning_content.activity.write`? → A: Theo [ACTIVITY_ENGINE §13](../../docs/02-learning/ACTIVITY_ENGINE.md) và OpenAPI `createCmsActivity`: `learning_content.activity.write`; duyệt `learning_content.activity.review`; xuất bản `learning_content.publish`. Lệch ở ADMIN_PORTAL §3.13 ghi vào báo cáo.
- Q: Editor video thuộc F054 (theo Ngoài phạm vi của F065) hay F058? → A: F058 theo danh mục CODE_PLAN §4.
- Q: "Tạo bản nháp từ v{n}" cho activity dùng API nào? → A: Chưa có; thêm `createCmsActivityVersion` (delta, owner F030), cùng mẫu `createQuestionVersion`.
- Q: Xem số attempt bị ảnh hưởng trước khi chấm lại, theo dõi và lịch sử yêu cầu? → A: Chỉ có `createRegradeRequest`; thêm `previewRegradeRequest`, `getRegradeRequest` (đã ghi ở F031), `listRegradeRequests` (delta, owner F031).
- Q: Upload phụ đề WebVTT dùng `MediaKind` nào? → A: `MediaKind` chưa có loại phụ đề; [Proposed] thêm `subtitle` (delta, owner F027).
- Q: Chấm lại đề (`scope = exam_session`) có trong F058? → A: F058 làm hộp thoại dùng chung nhận `scope`; chỉ gắn vào trang version câu hỏi/activity. Gắn vào màn đề/kết quả đề ngoài phạm vi (xem Ngoài phạm vi).

### Session 2026-11-17

- Q: Mã lỗi khi vượt phạm vi trường/môn của UserRole? → A: `403 PERMISSION_DENIED`; `FORBIDDEN_SCOPE` không phát và sẽ bị gỡ khỏi `ProblemCode` ([API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md), RB-113).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Soạn activity `quiz` và `interactive_exercise` (Priority: P1) 🎯 MVP

`cm.noidung` chọn loại `quiz` từ Activity Registry, nhập metadata (khối, môn, chủ đề, LO, độ khó), chọn nguồn câu hỏi (danh sách câu, bộ câu hỏi hoặc quy tắc chọn), cấu hình chấm, `FeedbackTiming`, quy tắc hoàn thành; bản nháp tự lưu. Sửa activity đã xuất bản tạo bản nháp mới.

**Why this priority**: Quest, bài giao, thử thách chỉ tham chiếu `activity_id` + version (FR-CNT-013); không có builder thì nhà trường không có hoạt động để giao (exit M-C3).

**Independent Test**: Tạo quiz 5 câu `single_choice` đã xuất bản cho LO "Bảng nhân 3", lưu → `getCmsActivity` trả config đúng schema; thêm câu `numeric` → bị chặn.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F058-AC-01 | KHI người dùng tạo activity, Portal PHẢI liệt kê loại từ `listActivityTypes` chỉ gồm bản ghi `status` ≠ `disabled` có editor trong bản portal hiện hành (`quiz`, `interactive_exercise`, `video_lesson`); HỆ THỐNG PHẢI không bao giờ hiển thị `written_response`, `speaking`, `project` | FR-CNT-013 · AC-EXR-008 · [ACTIVITY_ENGINE §12](../../docs/02-learning/ACTIVITY_ENGINE.md) | — |
| F058-AC-02 | Portal PHẢI sinh form cấu hình từ JSON Schema của loại (`schema_url`, `specs/schemas/activities/<type>.schema.json`), validate tức thời `required`/`maxLength`/`enum`/`minimum`/`maximum`, bắt buộc metadata `title`, `grade_levels`, `subject_id`, `objective_ids` (1–20), `difficulty` (1–5), `content_language`, rồi tạo bằng `createCmsActivity` kèm `Idempotency-Key` | FR-CNT-002 · FR-CMS-003 · [ACTIVITY_ENGINE §4](../../docs/02-learning/ACTIVITY_ENGINE.md) | — |
| F058-AC-03 | KHI chọn nguồn câu hỏi, Portal PHẢI hỗ trợ đủ `questionSource.kind`: `questions` (chọn câu `published` qua `searchQuestions`, tối đa `maxItems`), `question_set` (`listCmsQuestionSets`, ghim version hoặc "version hiện hành"), `selection` (chủ đề, LO, khoảng độ khó, loại câu, `count`); và chặn chọn câu có loại ngoài `allowed_question_types` của loại activity | FR-CNT-013 · [ACTIVITY_ENGINE §3](../../docs/02-learning/ACTIVITY_ENGINE.md) | TC-ACT-003 |
| F058-AC-04 | Portal PHẢI cho chọn `evaluation.strategies` chỉ trong `ActivityTypeInfo.evaluation_strategies`, nhập `max_score`, `pass_threshold`, `feedback.timing` theo enum `FeedbackTiming`, `completion_rule` và hiển thị ghi chú "Hoàn thành khác Đạt"; quiz tính giờ PHẢI nằm trong khoảng `timed.total_seconds` của schema | FR-EXR-008 · [ACTIVITY_ENGINE §7, §8.1](../../docs/02-learning/ACTIVITY_ENGINE.md) | — |
| F058-AC-05 | TRONG KHI soạn bản nháp, Portal PHẢI autosave bằng `updateCmsActivityDraft` kèm `If-Match` theo chu kỳ của `ContentEditorShell` (F053); NẾU nhận `412`/`409 CONTENT_VERSION_CONFLICT`, THÌ Portal PHẢI dừng autosave, giữ thay đổi cục bộ và cho tải bản mới, không ghi đè | FR-CMS-006 · ADR-0043 | TC-CNT-003 |
| F058-AC-06 | NẾU server trả `422 ACTIVITY_CONFIG_INVALID` hoặc `VALIDATION_FAILED`, THÌ Portal PHẢI ánh xạ `errors[].field` (JSON Pointer) về đúng ô và khóa Gửi duyệt tới khi hết lỗi | FR-CNT-013 · ADR-0048 | TC-ACT-003 |
| F058-AC-07 | KHI mở activity có version `published`, Portal PHẢI hiển thị chỉ đọc, không có nút Sửa, chỉ có "Tạo bản nháp từ v{n}" gọi `createCmsActivityVersion` (delta); NẾU lưu nhận `CONTENT_NOT_EDITABLE`, THÌ Portal PHẢI chuyển sang chỉ đọc | FR-CNT-004 · ADR-0015 · ADR-0043 | TC-ACT-005 |
| F058-AC-08 | Portal PHẢI dùng `MathInput`/`MathText` (F053) cho `instructions`, `description` và nhãn có công thức, gửi SVG + speech text trong `math_assets`; NẾU nhận `422 MATH_RENDER_FAILED`, THÌ Portal PHẢI tô đỏ đúng công thức lỗi | FR-CNT-017 · ADR-0055 | TC-UI-013 |
| F058-AC-09 | Portal PHẢI soạn `title`, `instructions` bằng `BilingualField` theo `content_language`; Ở NƠI activity có khối lớp 1–3, Portal PHẢI bắt buộc audio đọc hướng dẫn, khóa Gửi duyệt khi thiếu và hiển thị `422 AUDIO_REQUIRED` liệt kê trường thiếu | FR-CNT-018 · [CONTENT_MODEL §8.1](../../docs/02-learning/CONTENT_MODEL.md) | TC-CNT-010 |

---

### User Story 2 — Preview và workflow trên một activity (Priority: P1)

`cm.noidung` xem trước activity như học sinh thấy (vi/en), gửi duyệt; `cm.duyet` duyệt và xuất bản ngay (US-CMS-002).

**Why this priority**: Không có preview và đường xuất bản thì activity không tới được học sinh.

**Independent Test**: Preview quiz → network không có `answer_key`/`scoring_rule`; `cm.noidung` không thấy nút Duyệt trên activity của mình; `cm.duyet` duyệt + xuất bản → `current_published_version` = 1.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F058-AC-10 | KHI người dùng bấm Xem trước, Portal PHẢI gọi `createLearningPreviewSession` (`content_type = activity`, `version`, `display_language`) và hiển thị `render` + `items` bằng renderer web "như học sinh thấy", có công tắc vi/en; preview PHẢI không tạo attempt, learning event hay XP | FR-CNT-010 · US-CMS-002 · [ADMIN_PORTAL §5](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F058-AC-11 | Chế độ "như học sinh thấy" PHẢI chỉ dùng dữ liệu của `LearningPreviewSession` (không có đáp án); đáp án, `options[].feedback`, gợi ý chỉ hiển thị trong panel tác giả đọc từ `getCmsActivity`/câu hỏi, không trong khung học sinh | FR-QB-003 · ADR-0016 | TC-LRN-012 |
| F058-AC-12 | KHI người dùng bấm Gửi duyệt / Duyệt / Từ chối / Xuất bản ngay, Portal PHẢI gọi `transitionContentVersion` (`content_type = activity`, `If-Match`, `Idempotency-Key`; lý do bắt buộc khi từ chối) hoặc `publishContentVersion` (`Idempotency-Key`, `change_note`); Portal PHẢI ẩn Duyệt/Từ chối khi thiếu `learning_content.activity.review` hoặc người dùng là tác giả version | FR-CMS-006 · FR-CNT-003 · [CONTENT_MODEL §6](../../docs/02-learning/CONTENT_MODEL.md) | TC-CMS-001…004 |
| F058-AC-13 | TRONG KHI loại activity có `status = preview` trong registry, Portal PHẢI cho lưu bản nháp nhưng khóa Xuất bản kèm cảnh báo "Loại chưa có renderer trên client hiện hành" và hiển thị `min_client_version`; NẾU server trả `422 ACTIVITY_TYPE_NOT_SUPPORTED`, THÌ Portal PHẢI hiển thị theo `code` | FR-EXR-015 · [ACTIVITY_ENGINE §12](../../docs/02-learning/ACTIVITY_ENGINE.md) | TC-ACT-004 |
| F058-AC-14 | KHI xuất bản activity đã có version publish, hộp thoại PHẢI nêu "học sinh đang làm giữ version cũ tới khi xong lượt; lượt mới nhận version mới"; bấm lại không tạo release thứ hai | FR-CNT-011 · ADR-0043 | TC-ACT-005 |

---

### User Story 3 — Video Manager cho `video_lesson` (Priority: P2)

Biên tập viên upload video, chia chương, gắn phụ đề vi/en, chèn câu hỏi tại 01:30, cấu hình tua và quy tắc hoàn thành, xem trước bằng trình phát web.

**Why this priority**: FR-CNT-014 ở P4 trong PRD nhưng F065 (M-C3) cần nội dung video do portal soạn.

**Independent Test**: Tạo video 4 phút có 2 chương, phụ đề `vi`, câu chèn bắt buộc tại 90 000 ms; preview dừng đúng mốc; đặt câu chèn ở 300 000 ms → bị chặn.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F058-AC-15 | KHI chọn video, Portal PHẢI cho chọn asset `kind = video` qua `listMediaAssets` hoặc upload mới (`presignMediaUpload`, cần `media.upload`), theo dõi `getMediaAsset` tới `ready`/`rejected`, lấy `duration_ms` từ asset (chỉ đọc); TRONG KHI asset chưa `ready`, Portal PHẢI khóa Gửi duyệt và hiển thị `VIDEO_NOT_READY`/`MEDIA_NOT_READY` theo `code` | FR-CNT-014 · FR-CNT-006 · [VIDEO_LEARNING §3](../../docs/02-learning/VIDEO_LEARNING.md) | TC-CNT-008 |
| F058-AC-16 | Portal PHẢI cho soạn `chapters[]` (tối đa `maxItems`) trên timeline với `start_ms < end_ms ≤ duration_ms`, không chồng lấn, tiêu đề song ngữ trong `maxLength`, gắn LO tùy chọn | FR-CNT-014 · [VIDEO_LEARNING §3](../../docs/02-learning/VIDEO_LEARNING.md) | — |
| F058-AC-17 | Portal PHẢI cho gắn `subtitles[]` (WebVTT, `language` theo pattern schema, đúng một `is_default`); Ở NƠI video được đánh dấu "có lời", Portal PHẢI bắt buộc phụ đề `vi` trước khi Gửi duyệt [Proposed] | FR-CNT-014 · [ADMIN_PORTAL §3.13](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-VID-004 |
| F058-AC-18 | Portal PHẢI cho chèn `embedded_questions[]` (tối đa `maxItems`) với `cue_id` duy nhất, `at_ms` trong `[0, duration_ms]`, câu hỏi chọn từ câu `published` (`content_version_id`), cờ `required`, `allow_skip_after_wrong`; NẾU mốc ngoài thời lượng, THÌ Portal PHẢI chặn lưu và báo đúng cue | FR-CNT-014 · AC-CNT-006 · [VIDEO_LEARNING §5](../../docs/02-learning/VIDEO_LEARNING.md) | TC-VID-001 |
| F058-AC-19 | KHI soạn video có nhánh, Portal PHẢI yêu cầu mỗi `branches[].options[]` (2–4 lựa chọn) trỏ `target_segment_id` tồn tại trong `segments[]` và ít nhất một segment `is_ending`; segment PHẢI dùng asset video `ready` | FR-CNT-014 · [VIDEO_LEARNING §6](../../docs/02-learning/VIDEO_LEARNING.md) | — |
| F058-AC-20 | Portal PHẢI điền sẵn `interaction.allow_seek = watched_only` và `completion_rule` (`mode = min_watched_ratio`, `min_watched_ratio`, `require_required_cues`) theo tham số `video_completion`, và cảnh báo (không chặn) khi thời lượng vượt `video_recommended_length` theo khối | FR-CNT-014 · [VIDEO_LEARNING §7](../../docs/02-learning/VIDEO_LEARNING.md) | TC-VID-002 |
| F058-AC-21 | KHI xem trước video, Portal PHẢI phát biến thể HLS (MP4 dự phòng) với danh sách chương, phụ đề bật/tắt, điểm đánh dấu cue và tạm dừng tại cue hiển thị câu hỏi từ preview (không đáp án) | FR-CNT-010 · [VIDEO_LEARNING §2](../../docs/02-learning/VIDEO_LEARNING.md) | TC-VID-004 |

---

### User Story 4 — Chấm lại hệ thống khi nội dung lỗi (Priority: P2)

`qlcm.truong.a` phát hiện đáp án sai ở câu hỏi v2 đã có học sinh làm; tạo v3 đã sửa và xuất bản, mở v2 → "Chấm lại hệ thống", xem số bài bị ảnh hưởng, chọn "Dùng đáp án mới" + lý do, theo dõi tới khi hoàn tất (FR-EXR-008).

**Why this priority**: Bảo đảm điểm đúng sau khi sửa lỗi nội dung; không chặn vòng soạn → giao bài.

**Independent Test**: 3 attempt dùng question version lỗi → tạo yêu cầu `use_new_key` → trang trạng thái `completed`, `re_evaluated_attempts = 3`; `ad.truong.a` không thấy nút.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F058-AC-22 | TRONG KHI người dùng có `learning_content.re_evaluate` xem một version câu hỏi/activity đã xuất bản (hoặc đã thay thế), Portal PHẢI hiển thị "Chấm lại hệ thống"; hộp thoại PHẢI yêu cầu `action` (`accept_all`/`use_new_key`/`drop_item`), `corrected_version_id` (chọn version `published` của cùng item) khi `use_new_key`, `reason` 10–1.000 ký tự, hiển thị số attempt bị ảnh hưởng từ `previewRegradeRequest` (delta) trước khi xác nhận | FR-EXR-008 · [ACTIVITY_ENGINE §7.3](../../docs/02-learning/ACTIVITY_ENGINE.md) | TC-LRN-015 |
| F058-AC-23 | KHI xác nhận, Portal PHẢI gọi `createRegradeRequest` (`scope = question_version`/`activity_version`, `Idempotency-Key` giữ nguyên khi thử lại), nhận `202 RegradeRequest`, theo dõi `getRegradeRequest` (delta) tới `completed`/`failed` hiển thị `affected_attempts`/`re_evaluated_attempts`, và liệt kê lịch sử bằng `listRegradeRequests` (delta) | FR-EXR-008 · AGENTS R10 | TC-LRN-015 |
| F058-AC-24 | TRONG KHI người dùng thiếu `learning_content.re_evaluate` (ví dụ `school_admin`, `tenant_content_editor`), Portal PHẢI ẩn nút; NẾU server trả `403 PERMISSION_DENIED`, THÌ Portal PHẢI hiển thị thông điệp theo `code`; HỆ THỐNG PHẢI không có ô sửa điểm/evaluation thủ công ở bất kỳ màn nào | FR-EXR-008 · ADR-0072 | TC-LRN-015 |
| F058-AC-25 | Hộp thoại PHẢI nêu rõ "bài làm gốc và evaluation cũ được giữ (đánh dấu thay thế), điểm mới do hệ thống chấm, có ghi audit" | FR-CMS-007 · [ADMIN_PORTAL §9](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |

---

### User Story 5 — Phân quyền, tenant, idempotency, giới hạn nhập (Priority: P1)

**Why this priority**: Constitution điều III, V; nội dung tenant và platform tách biệt.

**Independent Test**: `ad.truong.a` mở builder; nội dung platform được gán chỉ đọc; `sa.admin` tạo activity không chọn tenant đích → bị chặn.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F058-AC-26 | TRONG KHI người dùng thiếu `learning_content.activity.write`, Portal PHẢI ẩn Tạo/Sửa activity và chỉ cho xem (khi có `learning_content.read`); NẾU server trả `403 PERMISSION_DENIED`, THÌ Portal PHẢI hiển thị trang/thông điệp không có quyền | AC-CMS-001 · [ACTIVITY_ENGINE §13](../../docs/02-learning/ACTIVITY_ENGINE.md) | — |
| F058-AC-27 | TRONG KHI activity là dữ liệu platform được gán (`source = assigned`), Portal PHẢI hiển thị nhãn "Từ EduWorld" và chỉ đọc; KHI System Admin tạo activity, Portal PHẢI bắt buộc chọn tenant đích (`tenant_id`, `X-Tenant-Id`) hoặc phạm vi Platform | FR-CNT-005 · AC-CMS-003 · ADR-0029 | TC-CMS-TEN-03 |
| F058-AC-28 | HỆ THỐNG PHẢI gửi `Idempotency-Key` cho mọi thao tác ghi của feature (tạo, tạo version, chuyển trạng thái, xuất bản, chấm lại), giữ nguyên khi thử lại; thông điệp lỗi theo `code` kèm `trace_id` | AGENTS R10 · ADR-0010 | TC-CNT-003 |
| F058-AC-29 | Portal PHẢI giới hạn mọi ô nhập (tiêu đề, hướng dẫn, tiêu đề chương, prompt nhánh, lý do, `change_note`) theo `maxLength` của OpenAPI/JSON Schema, có bộ đếm ký tự | ADR-0041 | TC-TEN-024 |
| F058-AC-30 | Ở NƠI độ rộng < 1024 px, Portal PHẢI hiển thị "Tác vụ này cần màn hình lớn hơn" cho builder và hộp thoại chấm lại, chỉ cho xem | [ADMIN_PORTAL §1.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F058-AC-31 | KHI menu/action của feature (`menu.content_studio.activities` và action con) đang tắt cho phạm vi phiên (`disabled_menus`/`disabled_actions` của `getMyPermissions`), Portal PHẢI ẩn menu và ẩn/khóa nút tương ứng (`<ActionGate actionKey>`); NẾU API trả `403 FEATURE_DISABLED`, Portal PHẢI hiện thông báo tính năng tạm tắt và tải lại quyền | FR-CMS-018 · ADR-0090 | TC-CMS-MENU-02 |

### Edge Cases

- NẾU `listActivityTypes` trả loại có editor nhưng `schema_version` lớn hơn bản form đã sinh, THÌ Portal PHẢI dùng schema tải từ `schema_url` và cảnh báo "Form có thể thiếu trường mới".
- NẾU câu hỏi trong `questions` bị gỡ xuất bản sau khi chọn, THÌ Portal PHẢI đánh dấu câu đó và khóa Gửi duyệt (server vẫn quyết định).
- NẾU video được thay bằng asset ngắn hơn, THÌ Portal PHẢI đánh dấu mọi chương/cue/nhánh vượt `duration_ms` mới (F058-AC-16, F058-AC-18).
- NẾU `previewRegradeRequest` trả `affected_attempts = 0`, THÌ Portal PHẢI cảnh báo và vẫn cho gửi khi người dùng xác nhận.
- NẾU yêu cầu chấm lại `failed`, THÌ Portal PHẢI hiển thị trạng thái và không tự gửi lại (F058-AC-23).
- NẾU preview session hết hạn (`expires_at`), THÌ Portal PHẢI tạo phiên mới khi người dùng bấm Xem trước lại.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CNT-013 | Builder theo Activity Registry, form sinh từ schema của loại, chỉ loại trong Release | [ACTIVITY_ENGINE §3, §12](../../docs/02-learning/ACTIVITY_ENGINE.md) |
| FR-EXR-008 | UI chấm lại hệ thống, không chấm tay | [ACTIVITY_ENGINE §7](../../docs/02-learning/ACTIVITY_ENGINE.md) |
| FR-CMS-003 | Content Management: learning activity | [ADMIN_PORTAL §3.13](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-006 | Workflow, preview, publish trên activity | [ADMIN_PORTAL §1.4, §3.11](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CNT-014 | Video Manager: chương, phụ đề, câu chèn, nhánh, hoàn thành | [VIDEO_LEARNING](../../docs/02-learning/VIDEO_LEARNING.md) |
| FR-CNT-010 | Preview activity như học sinh thấy | [ADMIN_PORTAL §5](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-QB-003 | Khung học sinh không hiển thị đáp án/scoring | [QUESTION_BANK](../../docs/02-learning/QUESTION_BANK.md) |
| FR-CMS-018 | Menu/action của feature theo danh mục `specs/portal/menu-catalog.yaml`, ẩn khi tắt, xử lý `403 FEATURE_DISABLED` | [ADMIN_PORTAL §2.7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |

### Key Entities

- **ActivityTypeInfo**: loại activity trong registry, schema cấu hình, chiến lược chấm hợp lệ, trạng thái, phiên bản client tối thiểu.
- **ActivityDefinition**: activity có version (metadata, `config` theo loại, trạng thái, version publish hiện hành).
- **QuestionSource**: nguồn câu hỏi của quiz/bài tập tương tác (danh sách, bộ câu hỏi, quy tắc chọn).
- **VideoLessonContent**: video, chương, phụ đề, câu chèn, nhánh/segment, tương tác, quy tắc hoàn thành.
- **LearningPreviewSession**: phiên xem trước ngắn hạn, render không đáp án.
- **RegradeRequest**: yêu cầu hệ thống chấm lại các attempt bị ảnh hưởng bởi version lỗi.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `video_completion` | Giá trị điền sẵn của `completion_rule` video |
| `video_recommended_length` | Cảnh báo thời lượng video theo khối |
| `latex_max_length` / `latex_max_depth` / `math_svg_max` | Giới hạn công thức trong editor |
| `assets_per_version_max` | Số asset tối đa mỗi version (video, segment, phụ đề) |
| `learning_translation_required` | Bản dịch bắt buộc hay tùy chọn khi Gửi duyệt |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không business logic chấm/validate nghiệp vụ trong UI; server quyết định | AGENTS R1, R5 · ADR-0016 |
| Không hiển thị đáp án trong khung học sinh | AGENTS R6 |
| UI theo Admin/Ops design system, style từ token | AGENTS R32 · ADR-0047 |
| Không hardcode URL API; cấu hình môi trường | AGENTS R3 |
| Chuỗi theo `maxLength` của contract | AGENTS R28 · ADR-0041 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Biên tập viên tạo một quiz 10 câu từ câu hỏi có sẵn và gửi duyệt trong ≤ 5 phút.
- **SC-002**: 100% activity được lưu từ builder qua validate schema phía server ở lần lưu đầu (không lỗi `ACTIVITY_CONFIG_INVALID` do form sinh sai) trong bộ E2E.
- **SC-003**: 0 phản hồi preview hiển thị trong khung học sinh chứa đáp án hoặc `feedback` theo phương án (quét network E2E).
- **SC-004**: Biên tập viên tạo video bài học có 3 câu chèn và phụ đề vi trong ≤ 10 phút (video đã `ready`).
- **SC-005**: Yêu cầu chấm lại được tạo trong ≤ 3 thao tác từ trang version; 0 yêu cầu trùng khi bấm lặp.

## Ngoài phạm vi

- Plugin/validate server `quiz`, `interactive_exercise`: F030; `video_lesson` và renderer Godot: F065; thực thi re-evaluate: F031 (đề: F042).
- Editor `audio_lesson`, `simulation`: F066; `minigame` (MiniGameConfig): F073/F079.
- Hàng đợi duyệt, hẹn giờ xuất bản, lịch sử/diff version, rollback, gỡ xuất bản, thư viện media, "Nơi sử dụng", thùng rác: F054 (dùng chung cho `content_type = activity`).
- Khung editor, trình soạn câu hỏi, công thức, song ngữ, `MediaPicker`: F053.
- Gắn hộp thoại chấm lại vào màn đề/kết quả đề (`scope = exam_session`): chưa có feature portal sở hữu (F055 không bao gồm) — ghi trong báo cáo.
- Upload/transcode video, sinh biến thể HLS/Theora: F027.
- Tự luận, rubric, bài nói, hàng đợi chấm: deferred (ADR-0082).

## Assumptions

- [Proposed] Thư mục `web/apps/portal/src/workspaces/content-studio/activities/`, route `/portal/content-studio/activities/*`; component editor tái sử dụng ở `web/packages/ops-ui/src/activity-editor/`.
- [Proposed] Phụ đề upload với `kind = subtitle` sau delta F027; trước delta, mục phụ đề hiển thị nhưng khóa upload mới (chọn asset có sẵn).
- [Proposed] Cờ "video có lời" là trạng thái form (không lưu); điều kiện phụ đề `vi` bắt buộc theo ADMIN_PORTAL §3.13 [Proposed], server vẫn quyết định.
- [Proposed] Lồng tiếng hướng dẫn khối 1–3 theo PED-07 (OQ-23); bật/tắt theo kết quả OQ chỉ đổi điều kiện hiển thị.
- F054 cung cấp `VersionHistoryTab`/`PublishDialog` tổng quát theo `content_type`; nếu F054 chưa xong, F058 dùng thanh trạng thái của F053 cho thao tác trên một item.
