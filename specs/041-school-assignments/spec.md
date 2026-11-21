# Feature Specification: F041 — Giao bài của nhà trường: assignment, `ResultRelease`, gia hạn/miễn, `my-assignments`

**Feature Branch**: `041-school-assignments` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `learning` (LMS) · **Mốc**: M-C3 · **Phụ thuộc**: F032 (F040 Removed — ADR-0095)
**Input**: School Admin/Academic Manager giao bài tự chấm, đề ôn tập, quest/mini-game đã publish cho một khối của trường (± học sinh thêm/bớt) hoặc danh sách học sinh của trường (ADR-0095); học sinh nhận bài trong game ở tab "Nhà trường giao"; nhà trường công bố kết quả, gia hạn, miễn, nhắc (ADR-0072). FR nguồn: FR-LMS-002, FR-LMS-006, FR-LMS-007, FR-LMS-012, FR-EXR-001.

## Clarifications

### Session 2026-11-16

- Q: Đối tượng `study_group` lấy danh sách thành viên ở đâu khi sở hữu study group chưa chốt (F040 US4)? → A: Qua port đọc thành viên nhóm do F040 cung cấp; F041 không phụ thuộc nơi lưu, chỉ cần snapshot thành viên lúc publish ([SCHOOL_LMS §2](../../docs/04-admin-portal/SCHOOL_LMS.md#2-mô-hình)).
- Q: Quest/mini-game để giao đọc từ đâu khi dữ liệu nằm ở module game? → A: Danh mục chỉ đọc dựng từ event của module game (F078) vào projection trong module learning; trước khi F078 Done danh mục rỗng, không có dữ liệu giả.
- Q: Bài `mode = graded` dùng assessment blueprint tạo gì? → A: Tạo ExamSession qua command của F042 trong cùng module ([SCHOOL_LMS §4.1](../../docs/04-admin-portal/SCHOOL_LMS.md#41-tạo-và-giao-bài) bước 5). Đổi ở Session 2026-11-17: phần tạo ExamSession chuyển sang F042.
- Q: Rule gợi ý R1 thuộc feature nào? → A: F041 đăng ký R1 vào rule engine F032.
- Q: Mã lỗi khi Academic Manager giao ngoài môn được gán? → A: `403 PERMISSION_DENIED` (đổi ở Session 2026-11-17).

### Session 2026-11-17

- Q: Mã lỗi khi vượt phạm vi trường/môn của UserRole? → A: `403 PERMISSION_DENIED`; `FORBIDDEN_SCOPE` không phát và sẽ bị gỡ khỏi `ProblemCode` ([API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md), RB-113).
- Q: Vòng phụ thuộc F041 ↔ F042 (bài `graded` + blueprint cần `CreateExamSession`) xử lý thế nào? → A: Technical Architect chốt thứ tự F040 → F041 → F042 ([CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý)). F041 sở hữu port `ExamSessionCreator` (application) và adapter mặc định "chưa khả dụng"; trong khi chưa có adapter thật, tạo bài `graded` + `assessment_blueprint` trả `422 VALIDATION_FAILED` (F041-AC-09). Tạo ExamSession khi publish, lưu `exam_session_id`, hủy ExamSession khi hủy bài và adapter gọi `CreateExamSession`/`CancelExamSession` thuộc F042 (F042-AC-31).

### Session 2026-11-20 (owner chốt — việc nhận từ OQ F062)

- OQ F062-1 → **Chốt:** `MyAssignment` thêm `subject_name` (`LearningName`, như `subject_name` ở schema lớp), điền từ danh mục môn ở server theo ngôn ngữ hiển thị; client không tự tra danh mục.
- OQ F062-2 → **Chốt:** bài `content_kind = question_set` được bắt đầu bằng `startAdHocAttempt` (`mode = assignment`, `source.kind = question_set`, `context_type = assignment`, `context_id` = assignment); F041 mở `mode = assignment` cho endpoint này với cùng kiểm tra F041-AC-13 (cửa sổ mở, số lần, thuộc lớp). Client mở bằng `activity_host` (F062, F063).

### Session 2026-11-20 (triển khai)

- Lưu trữ → **[Confirmed]** bảng `lms.assignments`, `lms.assignment_targets`, `lms.assignment_target_students`, `lms.assignment_submissions`, `lms.assignment_student_exceptions`, `lms.assignment_reminders` (TA) và projection `ref.game_activities` viết tay trong migration `20261216000001_f041_assignments.sql` với repository SQL tường minh (như F040), không qua ent; RLS T.
- Tham số → **[Confirmed]** `graded_assignment_release_default` (after_close/after_close), `late_window_hours` tối đa 168, `assignment_remind_limit` 1/ngày (ngày ICT) là `command.DefaultAssignmentConfig()` cho tới khi module learning đọc platform/tenant settings (cùng cách `attempt_start_rate`).
- `feedback_timing` so với `result_release` → **[Confirmed]** theo ACTIVITY_ENGINE §8.1: `result_release = immediate` nhận mọi `feedback_timing`; `after_close` và `manual` chỉ nhận `after_close` hoặc `never`; ngoài ra `422` tại `/feedback_timing`.
- Blueprint → **[Confirmed]** mọi bài `assessment_blueprint` (cả `practice`) giao bằng ExamSession nên bị `422` tại `/content_ref` khi `ExamSessionCreator` là adapter "chưa khả dụng" (F041-AC-09); hoạt động `written_response`/`speaking`/`project` không lưu được (CHECK `activity_versions_type_ck`) nên AC-03 kiểm ở unit test của use case.
- Đối tượng → **[Confirmed]** lớp bất kỳ của trường của course (không chỉ lớp của course), nhóm học tập của trường, học sinh active của trường; ngoài trường/tenant → `404`. Snapshot khi publish gồm học sinh active có enrollment active; một học sinh chỉ một submission.
- `created_by_role` → **[Confirmed]** `system_admin` khi principal có grant `system_admin` (ghi với `X-Tenant-Id`), `school_admin` khi có grant `school_admin` phủ trường, còn lại `academic_manager`. `tenant_id` trong body khác tenant của phiên → `422`.
- Sửa và publish → **[Confirmed]** `publishAssignment` cần `If-Match`; bài `scheduled` sửa như draft trừ `targets` (đã chụp snapshot); `open` chỉ `due_at`/`instructions`/`late_policy`. Hủy phát `assignment.cancelled` (outbox cùng transaction); consumer lịch F040 soft-delete mục `assignment_due` (owner chốt F041-1, 2026-11-20).
- Submission → **[Confirmed]** tính lại từ attempt của bài (idempotent): `attempts_used` = số attempt đã bắt đầu, điểm = evaluation hiệu lực của lượt giữ theo `score_policy` (chấm lại cập nhật theo), `submitted`/`late` theo lượt chấm đầu tiên so với hạn cá nhân, `rapid_guess_count` của lượt được giữ; `assignment.completed` phát một lần khi có lượt chấm đầu tiên.
- Gia hạn → **[Confirmed]** nhận ở `scheduled`/`open`/`closed`; bài `closed` vẫn cho học sinh được gia hạn làm trong hạn cá nhân, scheduler chưa đóng bài khi còn học sinh có gia hạn đang chạy. Khóa tài khoản (`user.access_revoked`) trước hạn → `excused` với lý do `account_locked`.
- Học sinh → **[Confirmed]** `MyAssignment.status`: `submitted`/`late` thành `graded` khi kết quả đã công bố, `missed` thành `overdue`; `content_kind`: `assessment_blueprint` → `assessment`, `minigame_config` → `activity`; tham số `status` lọc theo `MyAssignment.status`; `subject_name` là `LearningName` của danh mục môn (OQ F062-1); `due_at` là hạn cá nhân khi được gia hạn.
- Công bố → **[Confirmed]** trước khi công bố, `getActivityAttempt` không trả `evaluation`, `listMyActivityAttempts` không trả điểm, `getActivityAttemptReview` chỉ trả `attempt_id` (không câu, điểm, đáp án); sau khi công bố review mở cả với `feedback_timing = after_close`.
- Danh mục game → `ref.game_activities` có bảng; event `content.published`/`content.rolled_back`/`content.unpublished` của module game và `tenant_data.assigned/unassigned` cho `quest`/`minigame_config` chưa có schema (T005) nên chưa có consumer — danh mục rỗng, không dữ liệu giả; giao quest/mini-game không có trong danh mục → `403 RESOURCE_NOT_ASSIGNED`.

### Session 2026-11-21 (ADR-0095 — [Confirmed])

- Đối tượng → **[Confirmed]** `AssignmentTarget.target_type` ∈ {`grade`, `students`}: `grade` = mọi học sinh hiệu lực (`ref.students.status = active`, chưa xóa) của khối `grade_level` trong `school_id` của bài, cộng `include_student_ids` (học sinh khác khối **cùng trường**), trừ `exclude_student_ids`; **exclude thắng include**; `students` = danh sách học sinh hiệu lực của trường. Mỗi khối tối đa một target `grade`, tối đa một target `students`, tổng 1–9 target; include/exclude/danh sách tối đa 200. Học sinh khác trường hoặc khác tenant trong include/danh sách → `422 VALIDATION_FAILED` tại con trỏ của danh sách (thay `404` cũ của lớp/nhóm). Bỏ `class`, `study_group`, `target_id`.
- Course → **[Confirmed]** `AssignmentCreate` bỏ `course_id`, thêm `school_id` và `subject_id` bắt buộc (gắn trực tiếp); quyền `lms.assignment.write` xét theo trường × môn của bài (ADR-0072, không có scope khối); trường ngoài mọi grant → `404`; môn ngoài grant giới hạn môn → `403`; môn tenant không thấy → `422` tại `/subject_id`; nội dung có môn khác `subject_id` → `422` tại `/subject_id`. `lms.assignments.course_id` thành nullable, không ghi nữa (DROP ở WP-R).
- Snapshot → **[Confirmed]** khi publish chụp học sinh theo quy tắc trên, ghi `grade_level` của học sinh tại thời điểm giao trên `lms.assignment_target_students` (không suy ngược từ khối hiện tại); `class_id_at_assign` và `assignment_submissions.class_id` không ghi nữa. `student_count` của target (chỉ có sau publish): `grade` = học sinh snapshot có khối đó hoặc thuộc include, trừ exclude; `students` = học sinh snapshot trong danh sách.
- Học sinh mới vào khối → **[Confirmed]** consumer `learning.assignment_enrollment` nhận `student.placement_changed.v1` (producer identity ở WP-I1), và trong lúc chuyển đổi cả `student.synced` có `grade_level` và `enrollment.changed` (bỏ qua `class_id`, đọc trường/khối từ `ref.students`): học sinh hiệu lực vào (trường, khối) được thêm (`source = grade_added`) vào bài `scheduled`/`open` có target khối đó, trừ khi bị exclude; học sinh rời khối/trường giữ submission (miễn bằng `excuse`). Placement khác tenant của envelope → lỗi vĩnh viễn (DLQ).
- Bài cũ → **[Confirmed]** target `class`/`study_group` hiện có được migration `20261229000001_adr0095_assignment_grade_targets.sql` chuyển thành `students` với **danh sách đóng băng**: snapshot lúc publish (bài đã publish) hoặc thành viên hiện tại của lớp/nhóm (bài nháp); không mở rộng thành cả khối; `target_ref_id` giữ id cũ làm lịch sử. ExamSession theo lớp/nhóm: `target` thành danh sách học sinh từ participations.
- Event → **[Confirmed]** `assignment.created` và `assignment.published` phát **v2** (`schema_version = 2`, `subject_id`, target `grade`/`students` với số đếm include/exclude); consumer duy nhất trong repo (lịch F040) đã gỡ nên v1 bị xóa cùng thay đổi này. `assignment.cancelled` bỏ trường tùy chọn `course_id` (sửa tại chỗ v1). `listAssignments` lọc `school_id`, `subject_id`, `grade_level` (bỏ `class_id`, `course_id`); `listAssignmentSubmissions` bỏ lọc `class_id`; `AssignmentSubmission` bỏ `class_id`.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Tạo và giao bài (Priority: P1)

Academic Manager chọn bộ câu hỏi chương 3 đã publish, giao cho khối 4 của trường (thêm/bớt vài học sinh) hạn thứ Sáu, chọn số lần làm, chính sách điểm, thời điểm phản hồi và công bố kết quả (US-LMS-001).

**Why this priority**: Là vòng lặp School Loop của mốc M-C3.

**Independent Test**: `ad.truong.a` tạo và publish bài giao question set cho khối 3 của trường → bài `open`, event `assignment.published` có đủ đối tượng; `ad.truong.b` không thấy bài.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F041-AC-01 | KHI School Admin (trường mình) hoặc Academic Manager (trong scope) gọi `createAssignment` với nội dung đã publish khả dụng cho tenant và `Idempotency-Key`, module learning PHẢI tạo bài `draft` gắn tenant, trường, môn (`school_id`, `subject_id` — ADR-0095, không course), ghim version (mặc định version publish lúc giao) và phát `assignment.created` | FR-LMS-002 · US-LMS-001 · [SCHOOL_LMS §4.1](../../docs/04-admin-portal/SCHOOL_LMS.md#41-tạo-và-giao-bài) | TC-LMS-001 |
| F041-AC-02 | NẾU nội dung chưa publish, THÌ HỆ THỐNG PHẢI trả `409 CONTENT_NOT_PUBLISHED`; NẾU là nội dung platform chưa gán cho tenant, THÌ HỆ THỐNG PHẢI trả `403 RESOURCE_NOT_ASSIGNED` | FR-LMS-007 · ADR-0029 | TC-CMS-ASG-01 |
| F041-AC-03 | NẾU nội dung là activity có `activity_type` ngoài Release (`written_response`, `speaking`, `project`), THÌ HỆ THỐNG PHẢI trả `422 ACTIVITY_TYPE_NOT_SUPPORTED` và không tạo bài | AC-EXR-008 · [SCHOOL_LMS §6](../../docs/04-admin-portal/SCHOOL_LMS.md#6-chỉ-giao-hoạt-động-tự-chấm) | — |
| F041-AC-04 | NẾU Academic Manager giao bài thuộc môn ngoài scope UserRole, THÌ HỆ THỐNG PHẢI trả `403 PERMISSION_DENIED` và ghi audit `authz.denied` | AC-ORG-002 · [SCHOOL_LMS §13](../../docs/04-admin-portal/SCHOOL_LMS.md#13-yêu-cầu-và-nghiệm-thu) (AC-LMS-003) | TC-TEN-002 |
| F041-AC-05 | NẾU trường của bài nằm ngoài mọi grant của người giao, THÌ HỆ THỐNG PHẢI trả `404 NOT_FOUND` không lộ sự tồn tại; NẾU học sinh trong `include_student_ids` hoặc `student_ids` thuộc trường khác hoặc tenant khác, THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` tại danh sách đó và không tạo bài (ADR-0095) | AC-ORG-001 · ADR-0095 | TC-CMS-AUTHZ-08, TC-LMS-001 |
| F041-AC-06 | KHI tạo bài `mode = graded` không chỉ định, HỆ THỐNG PHẢI đặt `feedback_timing` và `result_release` theo `graded_assignment_release_default`; NẾU `mode = graded` kèm `follow_latest = true`, hoặc `feedback_timing` sớm hơn thời điểm công bố của `result_release`, THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` | [SCHOOL_LMS §4.1](../../docs/04-admin-portal/SCHOOL_LMS.md#41-tạo-và-giao-bài) · [ACTIVITY_ENGINE §8.1](../../docs/02-learning/ACTIVITY_ENGINE.md#81-feedbacktiming-và-resultrelease) | — |
| F041-AC-07 | NẾU `title` vượt 120 ký tự, `instructions` vượt 2000, `targets` vượt 9 (ADR-0095), `late_policy.late_window_hours` ngoài 0–`late_window_hours` tối đa, `max_attempts` ngoài 1–10 hoặc `due_at ≤ open_at`, THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` chỉ rõ trường | ADR-0041 | TC-TEN-024 |
| F041-AC-08 | KHI gọi `publishAssignment`, HỆ THỐNG PHẢI chụp snapshot học sinh đối tượng (theo F041-AC-32, kèm khối của học sinh lúc giao), chuyển `scheduled` (khi `open_at > now`) hoặc `open`, phát `assignment.published` và ghi audit | FR-LMS-002 · [SCHOOL_LMS §4](../../docs/04-admin-portal/SCHOOL_LMS.md#4-assignment) | TC-LMS-001 |
| F041-AC-09 | NẾU gọi `createAssignment` (hoặc `updateAssignment` ở `draft`) với `mode = graded` và nội dung `assessment_blueprint` trong khi port `ExamSessionCreator` chỉ có adapter mặc định "chưa khả dụng", THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` (`errors[].field = content_ref`) và không tạo/sửa bài; việc tạo ExamSession khi publish thuộc F042 (F042-AC-31) | [SCHOOL_LMS §4.1](../../docs/04-admin-portal/SCHOOL_LMS.md#41-tạo-và-giao-bài) · [CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý) | — |
| F041-AC-10 | KHI gọi `updateAssignment` với `If-Match` đúng, HỆ THỐNG PHẢI cho sửa mọi trường ở `draft`, chỉ `due_at`/`instructions`/`late_policy` ở `open`; NẾU sửa trường khác hoặc bài đã `closed`/`archived`/`cancelled`, THÌ HỆ THỐNG PHẢI trả `409 ASSIGNMENT_NOT_EDITABLE`; `If-Match` lệch → `412 VERSION_MISMATCH` | [API_DESIGN §9.14](../../docs/07-backend/API_DESIGN.md#914-lms--apiv1lms) | — |
| F041-AC-11 | KHI gọi `cancelAssignment` cho bài `scheduled` hoặc `open` chưa có lượt nộp, HỆ THỐNG PHẢI chuyển `cancelled`, ghi audit; NẾU đã có lượt nộp, THÌ HỆ THỐNG PHẢI trả `409 ASSIGNMENT_NOT_EDITABLE` | [SCHOOL_LMS §4](../../docs/04-admin-portal/SCHOOL_LMS.md#4-assignment) | — |

---

### User Story 2 — Học sinh nhận và làm bài trong game (Priority: P1)

Học sinh thấy bài trong tab "Nhà trường giao", bắt đầu làm trong hạn, kết quả tự chấm cập nhật trạng thái nộp.

**Why this priority**: Không có phía học sinh thì bài giao không có giá trị (FR-LMS-006).

**Independent Test**: `hs.an.3a` gọi `listMyAssignments` thấy bài của khối 3; học sinh tenant B không thấy; `hs.an.3a` làm và nộp → `listAssignmentSubmissions` hiện `submitted` với điểm tự chấm.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F041-AC-12 | KHI học sinh gọi `listMyAssignments`, HỆ THỐNG PHẢI chỉ trả bài `open`/`closed`/`results_released` có học sinh trong snapshot đối tượng, kèm trạng thái nộp, `attempt_id` đang dở và số lượt còn lại | FR-LMS-006 · [SCHOOL_LMS §13](../../docs/04-admin-portal/SCHOOL_LMS.md#13-yêu-cầu-và-nghiệm-thu) (AC-LMS-001) | TC-LMS-002 |
| F041-AC-13 | KHI học sinh bắt đầu attempt với `context.assignment_id`, HỆ THỐNG PHẢI tạo attempt `mode = assignment` dùng version đã ghim (hoặc version mới nhất khi `follow_latest`) và `feedback_timing` của bài; NẾU ngoài `open_at`–`due_at` cá nhân cộng cửa sổ nộp muộn, THÌ `403 ASSIGNMENT_NOT_OPEN`; NẾU hết `max_attempts`, THÌ `409 ASSIGNMENT_MAX_ATTEMPTS`; NẾU học sinh không thuộc đối tượng, THÌ `404 NOT_FOUND` | FR-EXR-001 · FR-CNT-011 · [ACTIVITY_ENGINE §9](../../docs/02-learning/ACTIVITY_ENGINE.md#9-api) | TC-LMS-002, TC-CNT-001 |
| F041-AC-14 | TRONG KHI bài giữ version lúc giao, publish hoặc rollback nội dung PHẢI không đổi version của lượt mới; TRONG KHI bài `follow_latest`, lượt mới PHẢI nhận version mới và lượt đang làm giữ version cũ | [SCHOOL_LMS §13](../../docs/04-admin-portal/SCHOOL_LMS.md#13-yêu-cầu-và-nghiệm-thu) (AC-LMS-002, AC-LMS-016) · ADR-0043 | TC-CNT-001 |
| F041-AC-15 | KHI nhận `activity.started`, `activity.submitted`, `activity.graded`, `exam.graded` hoặc `attempt.re_evaluated` có `assignment_id`, HỆ THỐNG PHẢI cập nhật `AssignmentSubmission` (trạng thái, `best_attempt_id` và điểm theo `score_policy`, `submitted_at`, `rapid_guess_count`, `late` khi nộp trong cửa sổ muộn) đúng một lần mỗi event và phát `assignment.completed` khi học sinh hoàn thành | [SCHOOL_LMS §4.2](../../docs/04-admin-portal/SCHOOL_LMS.md#42-theo-dõi) · [EVENT_DESIGN §4.3](../../docs/07-backend/EVENT_DESIGN.md#43-assessment-exercise--exam) | TC-LMS-002 |
| F041-AC-16 | KHI học sinh hiệu lực vào một khối của trường (`student.placement_changed`; trong đợt chuyển cả `student.synced`, `enrollment.changed`) trong lúc bài `scheduled`/`open` có target khối đó, HỆ THỐNG PHẢI thêm học sinh vào snapshot (`source = grade_added`) trừ khi bị exclude, giữ submission của học sinh rời khối/trường, và đặt `excused` cho học sinh bị khóa trước `due_at` | [SCHOOL_LMS §4.3](../../docs/04-admin-portal/SCHOOL_LMS.md#43-học-sinh-đổi-trườngkhối-khi-bài-đang-mở) · ADR-0095 | — |
| F041-AC-17 | KHI bài được publish, payload `assignment.published` (schema `assignment.created.v2`) PHẢI có đối tượng (khối/danh sách, số đếm), nội dung, hạn và trạng thái để module game/thông báo đưa bài vào tab "Nhà trường giao" | FR-LMS-006 · [EVENT_DESIGN §4.3](../../docs/07-backend/EVENT_DESIGN.md#43-assessment-exercise--exam) | — |

---

### User Story 3 — Đóng bài và công bố kết quả (Priority: P1)

Khi hết hạn, học sinh chưa nộp bị đánh `missed`; kết quả hiện cho học sinh theo `ResultRelease` (ngay, khi đóng, hoặc khi nhà trường bấm công bố).

**Why this priority**: Bảo vệ công bằng (không lộ đáp án cho bạn chưa làm) và là đầu vào dashboard F043.

**Independent Test**: Bài `manual` hết hạn → học sinh gọi review không thấy đáp án; `qlcm.truong.a` gọi `releaseAssignmentResults` → học sinh thấy kết quả.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F041-AC-18 | KHI qua `due_at` cộng cửa sổ nộp muộn, worker PHẢI chuyển bài `open` → `closed`, đánh `missed` cho học sinh chưa nộp và không `excused`, phát `assignment.closed` | FR-LMS-002 · [SCHOOL_LMS §13](../../docs/04-admin-portal/SCHOOL_LMS.md#13-yêu-cầu-và-nghiệm-thu) (AC-LMS-004) | — |
| F041-AC-19 | TRONG KHI `result_release = after_close` hoặc `manual` chưa công bố, HỆ THỐNG PHẢI không trả kết quả, đáp án, giải thích hay phản hồi phương án của bài cho học sinh (`getActivityAttemptReview` chỉ trả trạng thái đã nộp) | [ACTIVITY_ENGINE §8.1](../../docs/02-learning/ACTIVITY_ENGINE.md#81-feedbacktiming-và-resultrelease) · AGENTS R6 | — |
| F041-AC-20 | KHI bài `after_close` chuyển `closed`, HỆ THỐNG PHẢI tự công bố (`results_released`); KHI người có `lms.result.release` gọi `releaseAssignmentResults` cho bài `manual` đã đóng, HỆ THỐNG PHẢI chuyển `results_released`, phát `assignment.results_released` và ghi audit; bài `immediate` chuyển `archived` khi đóng | [SCHOOL_LMS §4.5](../../docs/04-admin-portal/SCHOOL_LMS.md#45-công-bố-kết-quả-và-chấm-lại) (AC-LMS-008) | TC-LMS-001 |
| F041-AC-21 | NẾU gọi `releaseAssignmentResults` cho bài chưa `closed` hoặc `result_release ≠ manual`, THÌ HỆ THỐNG PHẢI trả `409 CONFLICT`; NẾU người gọi thiếu `lms.result.release`, THÌ `403 PERMISSION_DENIED` | [API_DESIGN §9.14](../../docs/07-backend/API_DESIGN.md#914-lms--apiv1lms) | — |

---

### User Story 4 — Gia hạn, miễn, nhắc từng học sinh (Priority: P2)

Academic Manager theo dõi tiến độ, gia hạn cho học sinh ốm, miễn bài cho học sinh chuyển đến muộn, nhắc học sinh chưa làm (US-LMS-004).

**Why this priority**: Xử lý ngoại lệ hằng tuần, sau khi giao bài chạy được.

**Independent Test**: Gia hạn `hs.binh.3a` thêm 2 ngày → học sinh còn làm được sau hạn chung; miễn `hs.an.3a` → không bị `missed`; nhắc 2 lần trong ngày → lần 2 bị từ chối.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F041-AC-22 | KHI gọi `extendAssignmentForStudent` với `due_at` mới muộn hơn hạn hiện tại, lý do và `Idempotency-Key`, HỆ THỐNG PHẢI đặt hạn cá nhân cho học sinh đó, mở lại khả năng làm bài nếu học sinh đang `missed` và ghi audit | FR-LMS-012 · US-LMS-004 | — |
| F041-AC-23 | KHI gọi `excuseAssignmentForStudent` với lý do và `Idempotency-Key`, HỆ THỐNG PHẢI đặt `excused`, loại học sinh khỏi tỉ lệ hoàn thành và `missed`, ghi audit | FR-LMS-012 · [SCHOOL_LMS §13](../../docs/04-admin-portal/SCHOOL_LMS.md#13-yêu-cầu-và-nghiệm-thu) (AC-LMS-004) | — |
| F041-AC-24 | KHI gọi `remindAssignment` cho bài `open`, HỆ THỐNG PHẢI xếp thông báo trong game cho học sinh chưa nộp, không `excused`, trả `recipients` và ghi audit; NẾU vượt `assignment_remind_limit`, THÌ HỆ THỐNG PHẢI trả `429 RATE_LIMITED` có `Retry-After` | FR-LMS-012 · [SCHOOL_LMS §4.1](../../docs/04-admin-portal/SCHOOL_LMS.md#41-tạo-và-giao-bài) | — |
| F041-AC-25 | KHI người có `lms.assignment.read` trong phạm vi gọi `listAssignmentSubmissions`, HỆ THỐNG PHẢI trả từng học sinh với trạng thái, điểm tự chấm, số lần làm, thời điểm nộp và `rapid_guess_count`, phân trang cursor | US-LMS-004 · [SCHOOL_LMS §8.2](../../docs/04-admin-portal/SCHOOL_LMS.md#82-theo-dõi-bài-giao) | — |

---

### User Story 5 — Phân quyền và cô lập tenant (Priority: P1)

Mọi endpoint bài giao chỉ cho đúng vai trò, trường/môn, tenant; Tenant Admin chỉ đọc; Safety Moderator không truy cập.

**Why this priority**: Bắt buộc AGENTS R7, R8.

**Independent Test**: Bộ test sinh từ OpenAPI gọi mọi endpoint F041 bằng `ad.truong.b`, `ad.donvi`, `sm.antoan`, `hs.an.3a`, `sa.admin`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F041-AC-26 | NẾU `assignment_id` hoặc `student_id` thuộc trường khác cùng tenant hoặc tenant khác, THÌ mọi endpoint bài giao PHẢI trả `404 NOT_FOUND`; tham số `tenant_ids` của người dùng tenant PHẢI bị bỏ qua | AC-ORG-001 · AC-CMS-002 | TC-TEN-001, TC-TEN-003, TC-CMS-TEN-01 |
| F041-AC-27 | NẾU Tenant Admin gọi thao tác ghi bài giao, hoặc `safety_moderator`/role platform khác `system_admin` gọi bất kỳ endpoint `/lms/assignments*`, THÌ HỆ THỐNG PHẢI trả `403 PERMISSION_DENIED` và ghi audit `authz.denied`; học sinh gọi `/lms/*` PHẢI nhận `403 PORTAL_ACCESS_STAFF_ONLY` | AGENTS R7, R20 · [SCHOOL_LMS §13](../../docs/04-admin-portal/SCHOOL_LMS.md#13-yêu-cầu-và-nghiệm-thu) (AC-LMS-018) | TC-TEN-004 |
| F041-AC-28 | KHI System Admin tạo/sửa bài giao thay trường, HỆ THỐNG PHẢI yêu cầu `X-Tenant-Id` tenant đích (thiếu → `400 TENANT_HEADER_REQUIRED`), ghi `created_by_role = system_admin` và audit tenant đích | FR-ORG-008 · AC-ORG-004 | TC-TEN-004 |
| F041-AC-29 | KHI các thao tác ghi (`createAssignment`, `publishAssignment`, `cancelAssignment`, `releaseAssignmentResults`, `archiveAssignment`, `remindAssignment`, `extend`, `excuse`) được gửi lại cùng `Idempotency-Key`, HỆ THỐNG PHẢI trả cùng kết quả, không ghi lần hai; cùng key khác payload → `409 IDEMPOTENCY_KEY_REUSED` | AGENTS R10 · [API_DESIGN §6](../../docs/07-backend/API_DESIGN.md#6-idempotency) | — |

---

### User Story 6 — Giao quest/mini-game theo LO yếu và nhắc trong gợi ý (Priority: P3)

Academic Manager chọn quest/mini-game đã publish và được gán cho tenant để giao (không cấu hình game); bài gần hạn chưa làm xuất hiện đầu tiên trong gợi ý của Mây (US-LMS-011).

**Why this priority**: Phụ thuộc module game (F078) ở mốc sau; R1 có giá trị ngay khi có bài giao.

**Independent Test**: Khi projection có quest được gán, `listGameActivities` trả quest; giao quest chưa gán → `403`; học sinh có bài hạn < 48 giờ chưa nộp → gợi ý `R1` ưu tiên 1.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F041-AC-30 | KHI gọi `listGameActivities`, HỆ THỐNG PHẢI chỉ trả quest/mini-game đã publish và được gán cho tenant (theo dữ liệu module game phát), lọc theo `objective_ids`; HỆ THỐNG PHẢI không có endpoint sửa cấu hình game trong `/lms/*` | FR-LMS-007 · US-LMS-011 · ADR-0027 | TC-CMS-ASG-01 |
| F041-AC-31 | KHI học sinh có bài giao chưa nộp với hạn cá nhân còn dưới 48 giờ, rule R1 PHẢI tạo gợi ý "Hoàn thành bài nhà trường giao" với ưu tiên 1 qua rule engine | [LEARNING_ANALYTICS §6](../../docs/02-learning/LEARNING_ANALYTICS.md#6-recommendation-rules) | — |
| F041-AC-32 | KHI publish bài có target `grade`, HỆ THỐNG PHẢI giao cho mọi học sinh hiệu lực của khối trong trường của bài, cộng học sinh `include_student_ids` (khác khối, cùng trường) và trừ `exclude_student_ids` (exclude thắng include); NẾU một khối có hai target, `grade_level` ngoài 1–9, target `students` có include/exclude/khối hoặc có hai target `students`, THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` tại con trỏ của target | FR-LMS-002 · ADR-0095 mục 3 | TC-LMS-001 |
| F041-AC-33 | KHI ghép đối tượng của bài, HỆ THỐNG PHẢI tính mỗi học sinh một lần theo thứ tự target, không bao giờ đưa học sinh bị exclude vào snapshot, và `listAssignments` PHẢI lọc được theo `school_id`, `subject_id`, `grade_level` trong scope người đọc | ADR-0095 mục 3 | — |
| F041-AC-34 | KHI migration ADR-0095 chạy, mọi target `class`/`study_group` hiện có PHẢI thành target `students` với danh sách đóng băng (snapshot lúc publish, hoặc thành viên hiện tại với bài nháp), không mở rộng thành cả khối | ADR-0095 mục 3 | — |

### Edge Cases

- NẾU học sinh mở attempt trước `due_at` và nộp sau hạn nhưng trong cửa sổ muộn, THÌ submission PHẢI là `late`; ngoài cửa sổ → `ATTEMPT_EXPIRED`/không tính (F041-AC-13, F041-AC-15).
- NẾU nội dung đã giao bị soft-delete hoặc platform gỡ gán, THÌ bài đã giao PHẢI vẫn làm được trên version đã ghim; chỉ không giao mới được (ADMIN_PORTAL §9).
- NẾU re-evaluate đổi điểm, THÌ submission PHẢI cập nhật theo `score_policy` từ `attempt.re_evaluated` (F041-AC-15).
- NẾU cùng event attempt giao hai lần, THÌ submission PHẢI chỉ cập nhật một lần (`infra.processed_events`).
- NẾU học sinh thuộc cả khối đối tượng và danh sách học sinh (hoặc được include), THÌ PHẢI chỉ có một submission; học sinh bị exclude không có submission dù được include.
- NẾU projection quest/mini-game rỗng (module game chưa có), THÌ `listGameActivities` trả danh sách rỗng, không lỗi.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-LMS-002 | Tạo, publish, đóng, công bố kết quả bài giao; đối tượng khối của trường ± học sinh, hoặc danh sách học sinh (ADR-0095); open/due, số lần làm, `ResultRelease` | [SCHOOL_LMS §4](../../docs/04-admin-portal/SCHOOL_LMS.md#4-assignment) |
| FR-LMS-006 | `listMyAssignments` và event `assignment.published` cho tab "Nhà trường giao" (giao diện game ở F062) | [SCHOOL_LMS §4.1](../../docs/04-admin-portal/SCHOOL_LMS.md#41-tạo-và-giao-bài) |
| FR-LMS-007 | Chỉ chọn quest/mini-game đã publish được gán; danh mục chỉ đọc (thử thách/giải trường ở F107) | [SCHOOL_LMS §6–§7](../../docs/04-admin-portal/SCHOOL_LMS.md#6-chỉ-giao-hoạt-động-tự-chấm) |
| FR-LMS-012 | Gia hạn, miễn, nhắc từng học sinh, có audit | [SCHOOL_LMS §4.4](../../docs/04-admin-portal/SCHOOL_LMS.md#44-gia-hạn-miễn-nhắc) |
| FR-EXR-001 | Mode `assignment` (nhà trường giao) của attempt | [ACTIVITY_ENGINE §6.1](../../docs/02-learning/ACTIVITY_ENGINE.md#61-mode--hoạt-động-được-gọi-từ-đâu-trong-game) |

### Key Entities

- **Assignment**: bài giao của trường (trường, môn — không course, ADR-0095; người tạo, nội dung ghim version hoặc `follow_latest`, `mode` practice/graded, open/due, nộp muộn, số lần làm, chính sách điểm, `FeedbackTiming`, `ResultRelease`, trạng thái).
- **AssignmentTarget**: đối tượng khối của trường (+ include, − exclude) hoặc danh sách học sinh của trường; snapshot (kèm khối lúc giao) khi publish.
- **AssignmentSubmission**: trạng thái nộp của từng học sinh (`not_started`, `in_progress`, `submitted`, `late`, `missed`, `excused`), lượt tốt nhất, điểm, số câu "làm quá nhanh".
- **StudentException**: hạn cá nhân (gia hạn) hoặc miễn, có lý do và người thực hiện.
- **AssignmentReminder**: lần nhắc của bài trong ngày.
- **GameActivityCatalogEntry**: quest/mini-game đã publish được gán cho tenant (chỉ đọc).

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `graded_assignment_release_default` | Mặc định `feedback_timing`/`result_release` cho bài graded |
| `late_window_hours` | Cửa sổ nộp muộn (mặc định, tối đa) |
| `assignment_remind_limit` | Giới hạn nhắc mỗi ngày mỗi bài |
| `score_scale` | Thang điểm hiển thị |
| `attempt_resume_window` | Resume attempt bài giao |
| `idempotency_record_ttl` | Thời hạn Idempotency-Key |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Authn + authorization nghiệp vụ; phạm vi tenant/trường/môn, không theo lớp phân công | AGENTS R7 · ADR-0072 |
| Chỉ hoạt động server tự chấm; không đáp án trước công bố | AGENTS R5, R6 · ADR-0016, ADR-0072 |
| Ghim version nội dung khi giao | AGENTS R11, R31 · ADR-0043 |
| Thao tác ghi idempotent + audit | AGENTS R10 |
| Không cấu hình game từ LMS | AGENTS R20 · ADR-0027 |
| Kết quả và submission lưu PostgreSQL, không chỉ Valkey | AGENTS R9 · ADR-0004 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Trong kịch bản demo M-C3, 100% học sinh hiệu lực của khối đối tượng (trừ exclude, cộng include) thấy bài; 0 học sinh ngoài đối tượng thấy bài.
- **SC-002**: 100% học sinh chưa nộp, không được miễn có trạng thái `missed` sau khi bài đóng.
- **SC-003**: 0 trường hợp học sinh thấy đáp án/kết quả trước thời điểm công bố theo `ResultRelease` trong bộ test.
- **SC-004**: 100% endpoint F041 đạt bộ test phân quyền và cô lập tenant; 100% thao tác ghi có audit.

## Ngoài phạm vi

- Course, lộ trình, lịch, nhóm học tập — bỏ theo ADR-0095 (F040 Removed); ExamSession, làm đề, chấm đề, adapter `ExamSessionCreator` gọi `CreateExamSession`/`CancelExamSession` và tạo/hủy ExamSession của bài `graded` + blueprint — F042; School Learning Dashboard — F043; xuất báo cáo, bảng kết quả course — F044.
- Wizard giao bài, theo dõi nộp bài trên portal — F055; tab "Nhà trường giao", Bưu tá Nhím trong game — F062; thông báo trong game — F123.
- Thử thách lớp/giải trường từ template — F107; catalog quest/mini-game phía module game — F078.
- Bài nói, tự luận, dự án, hàng đợi chấm — deferred (ADR-0082).

## Assumptions

- [Confirmed] `follow_latest`, nhắc học sinh, xử lý enrollment thay đổi theo SCHOOL_LMS §4.3; mặc định graded và cửa sổ muộn là Confirmed trong CONFIG_DEFAULTS.
- F040 cung cấp port đọc thành viên study group; study group thuộc module learning (ADR-0084, đã chốt); tới khi port của F040 có, đối tượng `study_group` bị từ chối `422 VALIDATION_FAILED` (feature flag cấu hình, không mock).
- F041 sở hữu port `ExamSessionCreator` và đăng ký adapter mặc định "chưa khả dụng" (không phải mock: adapter thật, trả lỗi có kiểu) ở composition root; trước khi F042 `Done`, bài `graded` + `assessment_blueprint` bị từ chối `422 VALIDATION_FAILED` (F041-AC-09). F042 thay adapter bằng adapter gọi `CreateExamSession`/`CancelExamSession` (F042-AC-31).
- Event module game cho danh mục quest/mini-game do F078 định nghĩa; projection có sẵn bảng trước.

## Câu hỏi mở

- F041-1 — Hủy bài giao (`cancelAssignment`) chưa có event (`assignment.cancelled`): mục lịch `assignment_due` do F040 tạo từ `assignment.published` và thông báo đã gửi không được gỡ. Đề xuất: thêm event `assignment.cancelled` (owner F041) và để consumer lịch F040 soft-delete mục lịch. **Đã chốt 2026-11-20 (owner): làm theo đề xuất** — `cancelAssignment` ghi outbox `assignment.cancelled` (schema `assignment.cancelled.v1`); consumer `learning.lms_schedule` soft-delete mục `assignment_due` của bài (`delete_reason = cancelled`), phát `schedule_item.deleted` cho từng mục, giao lại không làm gì (T048, `TestAssignmentCancelRemovesSchedule`).
- F041-2 — Danh mục quest/mini-game (`ref.game_activities`) cần payload `content.published`/`content.rolled_back`/`content.unpublished` của module game (khối `catalog`, owner F071/F078) và `tenant_data.assigned/unassigned` cho `quest`/`minigame_config` (owner F045); chưa có schema nên consumer chưa làm (T005, T041 phần projection). **Đã chốt 2026-11-20 (owner): giữ nguyên** — consumer `ref.game_activities` làm khi F071/F078 định nghĩa event catalog (ghi phụ thuộc ở tasks F041 và F078 T034) — `listGameActivities` trả rỗng.
