# Feature Specification: F055 — Workspace C — LMS: course, learning path, wizard giao bài, theo dõi nộp bài, lịch, nhóm học tập

**Feature Branch**: `055-portal-lms-assignments` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `src/cms/frontend/apps/portal` (workspace C) · **Mốc**: M-C3 · **Phụ thuộc**: F040, F041, F042, F052
**Input**: Màn LMS của workspace C cho `school_admin`/`academic_manager`: course, learning path, wizard giao bài, theo dõi nộp bài + extend/excuse/remind/release, lịch, nhóm học tập, gia hạn thời gian thi cá nhân. FR nguồn: FR-LMS-001, FR-LMS-002, FR-LMS-004, FR-LMS-012, FR-EXM-009.

## Clarifications

### Session 2026-11-16

- Q: Ai giao bài? → A: `school_admin` (trường mình), `academic_manager` (scope tenant/trường/môn), System Admin thay trên tenant đích; `tenant_admin` chỉ đọc ([SCHOOL_LMS §4.1, §9](../../docs/04-admin-portal/SCHOOL_LMS.md), ADR-0072).
- Q: Giá trị mặc định `feedback_timing`/`result_release` cho bài `graded` lấy ở đâu? → A: Server áp `graded_assignment_release_default` khi client không gửi; Portal không tự đặt giá trị.
- Q: Thử thách lớp/giải trường (FR-LMS-004, phần P4) có trong F055? → A: Không; F055 chỉ làm nhóm học tập. Thử thách thuộc F107 (backend) — xem Ngoài phạm vi.
- Q: Gia hạn thời gian thi cá nhân (`accommodations[]`) dùng endpoint nào? → A: Chưa có contract; F055 thêm delta `ExamSession.accommodations[]` + endpoint mới ở Phase 2 ([contracts §6](contracts/README.md)).

### Session 2026-11-17

- Q: Mã lỗi khi vượt phạm vi trường/môn của UserRole? → A: `403 PERMISSION_DENIED`; `FORBIDDEN_SCOPE` không phát và sẽ bị gỡ khỏi `ProblemCode` ([API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md), RB-113).
- Q: Dialog chấm lại đề thi trên portal thuộc feature nào? → A: F055 (RB-222): nút "Chấm lại đề" ở bài giao có `exam_session_id`, dùng `RegradeDialog` của F058 với `createRegradeRequest` `scope = exam_session` (F055-AC-23). Thư mục workspace C là `school-lms` (RB-619).

### Session 2026-11-21 (triển khai — [Confirmed], owner chốt 2026-11-20)

- Q: `updateCourse`/`updateScheduleItem` bắt buộc `If-Match` nhưng không có `getCourse`/`getScheduleItem` và danh sách không trả version — Portal lấy ETag ở đâu? → A: [Confirmed] Delta **D1**: thêm `version` (readOnly) vào `Course` và `ScheduleItem`; Portal gửi `If-Match: W/"<version>"` từ dòng danh sách. Backend learning chỉ đổi JSON tag của view (không đổi bảng); test `lms_test.go`, `lms_group_test.go`.
- Q: Delta accommodations (T004) có hình dạng nào? → A: [Confirmed] Delta **D2**: `PUT /exams/sessions/{exam_session_id}/accommodations` (`putExamAccommodations`, `x-portal-action: action.school_lms.assignments.manage_students`), body `ExamAccommodationsUpdate {accommodations[] ≤ 200, reason 3–500}` (thay cả danh sách; lý do chung cho lần lưu, không lý do từng học sinh vì `ExamAccommodation` có `additionalProperties: false`), `If-Match` + `Idempotency-Key`, chỉ khi session `scheduled`/`open` (409), học sinh ngoài đối tượng → 422; lượt đang làm giữ `deadline_at`. Hiện thực ở module learning (`PutExamAccommodations`, test `exam_accommodations_test.go`).
- Q: Chấm lại đề (`scope = exam_session`) cần gì thêm? → A: Backend (F042-AC-28) bắt buộc `question_version_id` của một câu trong đề đã khóa; dialog chọn câu từ `listExamSessionPapers`. [Confirmed] Với `scope = exam_session` dialog chỉ đưa `accept_all`/`drop_item`: `use_new_key` cần danh sách version đã sửa của câu mà API hiện chỉ tra được theo `content_id` (câu hỏi mở F055-1 — chốt: Release 1 giữ `accept_all`/`drop_item` cho `scope = exam_session`; `use_new_key` vẫn làm ở `scope = question`).
- Q: System Admin xem danh sách LMS thế nào? → A: [Confirmed] Mọi màn LMS của F055 làm việc trên **một** tenant đích chọn ở bộ lọc topbar (`SingleTenantGate`, như F052) cho cả đọc lẫn ghi; không gửi `tenant_ids[]`, không cột Tenant.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Wizard giao bài (Priority: P1) 🎯 MVP

`qlcm.truong.a` giao bộ câu hỏi chương 3 cho lớp 3A, hạn thứ Sáu, chế độ luyện tập, giữ phiên bản lúc giao (US-LMS-001, US-LMS-011).

**Why this priority**: Giao bài là vòng lặp School Loop của mốc M-C3; mọi màn khác (theo dõi, dashboard) cần bài đã giao.

**Independent Test**: Đăng nhập `ad.truong.a`, tạo bài qua wizard 5 bước, publish; `hs.an.3a` thấy bài trong `listMyAssignments`, `hs.chi.3b` không thấy.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F055-AC-01 | KHI người dùng có `lms.assignment.write` mở bước "Nội dung" của wizard, Portal PHẢI chỉ liệt kê nội dung đã publish thuộc loại server tự chấm (activity theo `listActivityTypes` trong Release, question set, assessment blueprint, lesson) và quest/mini-game từ `listGameActivities`; không có `written_response`/`speaking`/`project` | FR-LMS-002 · AC-LMS-007 · [SCHOOL_LMS §6](../../docs/04-admin-portal/SCHOOL_LMS.md) | TC-LMS-001 |
| F055-AC-02 | KHI người dùng chọn đối tượng, Portal PHẢI cho chọn `target_type` `class` (một hoặc nhiều lớp), `study_group` hoặc `students` chỉ từ lớp/nhóm/học sinh trong scope trả về bởi API (trường, môn của UserRole) | FR-LMS-002 · FR-LMS-005 · [SCHOOL_LMS §4.1](../../docs/04-admin-portal/SCHOOL_LMS.md) | TC-LMS-001 |
| F055-AC-03 | KHI người dùng chọn `mode = graded`, Portal PHẢI vô hiệu tùy chọn `follow_latest` và không gửi `feedback_timing`/`result_release` nếu người dùng không đổi, để server áp `graded_assignment_release_default` | FR-LMS-002 · FR-CNT-011 · [CONTENT_MODEL §5.2](../../docs/02-learning/CONTENT_MODEL.md) | TC-LMS-001 |
| F055-AC-04 | HỆ THỐNG PHẢI kiểm tra form theo giới hạn sinh từ schema `AssignmentCreate` (`title`, `instructions` `maxLength`, `late_window_hours` trong khoảng `late_window_hours`, `max_attempts`, `targets` `maxItems`, `due_at > open_at`) và hiển thị bộ đếm ký tự trước khi gửi | ADR-0041 · [DESIGN_SYSTEM §4.3](../../docs/05-ux/DESIGN_SYSTEM.md) | TC-TEN-024 |
| F055-AC-05 | KHI người dùng bấm "Giao bài", Portal PHẢI gọi `createAssignment` rồi `publishAssignment` (kèm `If-Match` từ `ETag`), mỗi lời gọi có `Idempotency-Key` sinh một lần cho thao tác và giữ nguyên khi thử lại, nên bấm hai lần chỉ tạo một bài | FR-LMS-002 · AGENTS R10 · ADR-0010 | TC-LMS-001 |
| F055-AC-06 | NẾU server trả `422 VALIDATION_FAILED`, `422 CONTENT_NOT_PUBLISHED`, `403 RESOURCE_NOT_ASSIGNED`, `403 PERMISSION_DENIED`, THÌ Portal PHẢI ánh xạ `errors[]` về từng trường hoặc hiển thị thông điệp tiếng Việt theo `code` kèm `trace_id`, không hiển thị `detail` thô | ADR-0010 · [ADMIN_PORTAL §10.1](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-ASG-01 |
| F055-AC-07 | Ở NƠI người dùng là System Admin, wizard PHẢI bắt buộc chọn một tenant đích (gửi `tenant_id` trong body và `X-Tenant-Id` = tenant đích); không có lựa chọn "mọi tenant" | FR-CMS-013 · [ADMIN_PORTAL §2.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-TEN-03 |

---

### User Story 2 — Theo dõi nộp bài và thao tác cá nhân (Priority: P1)

Academic Manager theo dõi bài theo lớp (đã nộp, chưa bắt đầu, `missed`), gia hạn hoặc miễn cho từng học sinh, nhắc học sinh chưa làm, công bố kết quả bài `manual` (US-LMS-004).

**Why this priority**: Không có theo dõi thì giao bài không khép vòng; extend/excuse/remind là FR-LMS-012.

**Independent Test**: Với bài đã publish ở US1, mở chi tiết bài, gia hạn `hs.an.3a`, miễn `hs.binh.3a`, nhắc; kiểm tra trạng thái submission và lỗi giới hạn nhắc.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F055-AC-08 | KHI người dùng mở chi tiết bài, Portal PHẢI hiển thị bảng `listAssignmentSubmissions` (cursor) gồm học sinh, trạng thái (`not_started`…`excused`), điểm tự chấm, số lần làm, thời điểm nộp, nhãn "làm quá nhanh" khi `rapid_guess_count > 0`; không có cột XP, rating, nhận xét thủ công | FR-LMS-003 · [SCHOOL_LMS §8.2](../../docs/04-admin-portal/SCHOOL_LMS.md) | TC-LMS-001 |
| F055-AC-09 | KHI người dùng gia hạn hoặc miễn một học sinh, Portal PHẢI yêu cầu lý do trong giới hạn `ReasonRequest`/`AssignmentExtendRequest`, gọi `extendAssignmentForStudent`/`excuseAssignmentForStudent` với `Idempotency-Key`, rồi cập nhật dòng từ phản hồi `AssignmentSubmission` | FR-LMS-012 · AGENTS R10 | — |
| F055-AC-10 | KHI người dùng bấm "Nhắc học sinh chưa làm", Portal PHẢI gọi `remindAssignment`; NẾU server trả `429 RATE_LIMITED` (giới hạn `assignment_remind_limit`) THÌ Portal PHẢI hiển thị "Bài này đã được nhắc hôm nay" và khóa nút tới hết ngày theo `Retry-After` | FR-LMS-012 · [SCHOOL_LMS §4.1](../../docs/04-admin-portal/SCHOOL_LMS.md) | — |
| F055-AC-11 | Ở NƠI bài có `result_release = manual` và `status = closed`, Portal PHẢI hiện nút "Công bố kết quả" chỉ cho người có `lms.result.release` và gọi `releaseAssignmentResults`; nút "Hủy bài" gọi `cancelAssignment` với lý do và hiển thị `409 CONFLICT`/`422 INVALID_STATE_TRANSITION` khi bài đã có submission | FR-LMS-002 · AC-LMS-008 · [SCHOOL_LMS §4, §4.5](../../docs/04-admin-portal/SCHOOL_LMS.md) | — |
| F055-AC-12 | KHI người dùng sửa bài, Portal PHẢI gọi `updateAssignment` với `If-Match`; TRONG KHI bài ở `open`, form PHẢI chỉ cho sửa `due_at`, `instructions`, `late_policy`; NẾU server trả `412`/`409 VERSION_MISMATCH` hoặc `ASSIGNMENT_NOT_EDITABLE` THÌ Portal PHẢI báo và tải lại bản mới, không ghi đè | ADR-0010 · [SCHOOL_LMS §4](../../docs/04-admin-portal/SCHOOL_LMS.md) | — |

---

### User Story 3 — Course và learning path (Priority: P2)

School Admin ẩn course trường không dạy; Academic Manager dựng learning path cho course Toán 3A từ thư viện nội dung đã publish (FR-LMS-001).

**Why this priority**: Course tự sinh từ lớp × môn (F040) đã cho học sinh path mẫu; builder là cải tiến sau giao bài.

**Independent Test**: Mở danh sách course của TH Demo A, ẩn một course, dựng path 3 node `mastery_gate`, mở khóa thủ công một node cho một học sinh.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F055-AC-13 | KHI người dùng mở "Khóa học", Portal PHẢI hiển thị `listCourses` (lọc trường/lớp/môn) và chỉ cho người có `lms.course.write` đổi `status` `active`/`hidden` qua `updateCourse` | FR-LMS-001 · [SCHOOL_LMS §3](../../docs/04-admin-portal/SCHOOL_LMS.md) | — |
| F055-AC-14 | KHI người dùng có `lms.learning_path.write` lưu path, Portal PHẢI gửi `createLearningPath`/`updateLearningPath` với node tham chiếu `content_version_id` đã publish, `unlock_mode`, số node trong `maxItems` của `LearningPathInput`; path mẫu (`is_template`) chỉ đọc | FR-LMS-001 · [SCHOOL_LMS §3](../../docs/04-admin-portal/SCHOOL_LMS.md) | — |
| F055-AC-15 | KHI người dùng mở khóa node cho một học sinh, Portal PHẢI yêu cầu lý do và gọi `unlockPathNodeForStudent` với `Idempotency-Key` | FR-LMS-001 · [SCHOOL_LMS §3](../../docs/04-admin-portal/SCHOOL_LMS.md) | — |

---

### User Story 4 — Lịch và nhóm học tập (Priority: P2)

School Admin tạo nhóm học tập "Ôn Nhân 3A" và lên lịch hạn nộp/kiểm tra cho course (US-LMS-008).

**Why this priority**: Nhóm học tập là đối tượng giao bài `study_group`; lịch hiển thị cho học sinh trong game.

**Independent Test**: Tạo nhóm 5 học sinh 3A, giao bài cho nhóm (US1), tạo/sửa/xóa mục lịch tuần.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F055-AC-16 | KHI người dùng có `lms.study_group.write` tạo/sửa nhóm, Portal PHẢI chỉ cho chọn học sinh trong trường/lớp của scope, kiểm tra `name` và `member_ids` theo giới hạn `StudyGroupInput`, gọi `createStudyGroup`/`updateStudyGroup`; xóa gọi `deleteStudyGroup` (soft-delete) sau xác nhận | FR-LMS-004 · ADR-0028 · [SCHOOL_LMS §2](../../docs/04-admin-portal/SCHOOL_LMS.md) | — |
| F055-AC-17 | KHI người dùng có `lms.schedule.write` thao tác lịch, Portal PHẢI hiển thị lịch tháng/tuần từ `listScheduleItems` và tạo/sửa/xóa qua `createScheduleItem`/`updateScheduleItem`/`deleteScheduleItem` với `note` trong `maxLength` | FR-LMS-001 · [SCHOOL_LMS §2](../../docs/04-admin-portal/SCHOOL_LMS.md) | — |

---

### User Story 5 — Gia hạn thời gian thi cho học sinh cụ thể (Priority: P3)

Academic Manager cộng thêm thời gian làm đề kiểm tra cho một học sinh cần hỗ trợ (FR-EXM-009).

**Why this priority**: [Confirmed] trong PRD; cần delta contract trước.

**Independent Test**: Với bài `mode=graded` có `exam_session_id`, thêm accommodation +25% cho `hs.binh.3a`; `deadline_at` của học sinh dài hơn bạn cùng lớp.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F055-AC-18 | Ở NƠI bài giao có `exam_session_id`, Portal PHẢI có tab "Hỗ trợ thời gian" cho người có `lms.assignment.write` chọn học sinh trong đối tượng bài và mức gia hạn theo schema `ExamAccommodation`, lưu qua endpoint accommodations (delta Phase 2) với `Idempotency-Key` và lý do | FR-EXM-009 · [EXAM_ENGINE §5](../../docs/02-learning/EXAM_ENGINE.md) | — |
| F055-AC-23 | TRONG KHI người dùng có `learning_content.re_evaluate` trong phạm vi xem bài giao có `exam_session_id` đã chấm, Portal PHẢI hiển thị nút "Chấm lại đề" mở `RegradeDialog` dùng chung (F058) với `scope = exam_session`, `action` (`accept_all`/`use_new_key`/`drop_item`), `reason` 10–1.000 ký tự và số attempt bị ảnh hưởng từ `previewRegradeRequest`; KHI xác nhận PHẢI gọi `createRegradeRequest` với `Idempotency-Key` giữ nguyên khi thử lại và theo dõi `getRegradeRequest`; thiếu permission PHẢI ẩn nút, không có ô sửa điểm thủ công | FR-EXM-008 · [EXAM_ENGINE §9](../../docs/02-learning/EXAM_ENGINE.md#9-chấm-lại-regrade) | TC-LRN-015 |

---

### User Story 6 — Phân quyền và cô lập tenant trên mọi màn LMS (Priority: P1)

Mọi màn của F055 chỉ hiện thao tác người dùng được phép và chỉ dữ liệu của tenant phiên.

**Why this priority**: Constitution điều V, III; chặn thật ở backend, Portal không được lộ hoặc trộn dữ liệu.

**Independent Test**: Lần lượt đăng nhập `ad.donvi`, `qlcm.truong.a`, `ad.truong.b`, `sm.antoan`, `sa.admin` và kiểm menu, nút, request header.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F055-AC-19 | HỆ THỐNG PHẢI hiển thị menu và nút ghi LMS chỉ khi `getMyPermissions` trả permission tương ứng (`lms.assignment.write`, `lms.course.write`, `lms.learning_path.write`, `lms.schedule.write`, `lms.study_group.write`, `lms.result.release`); `tenant_admin` chỉ xem; NẾU backend trả `403` THÌ Portal PHẢI giữ nguyên dữ liệu và báo không đủ quyền | FR-LMS-005 · AC-LMS-003 · AGENTS R7 | TC-TEN-002 |
| F055-AC-20 | KHI người dùng không có permission `lms.*` (ví dụ Safety Moderator) mở URL màn LMS, Portal PHẢI hiển thị trang 403 của portal và không tải chunk workspace C | AC-LMS-018 · [ADMIN_PORTAL §1.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-WS-01 |
| F055-AC-21 | HỆ THỐNG PHẢI gửi `X-Tenant-Id` = tenant phiên ở mọi request LMS của người dùng tenant và không gửi `tenant_ids`; KHI nhận `403 TENANT_MISMATCH` Portal PHẢI xóa cache truy vấn và đưa về màn chọn đơn vị; KHI nhận `404` cho tài nguyên khác tenant/trường Portal PHẢI hiện "Không tìm thấy" | FR-ORG-008 · ADR-0065 · AGENTS R8 | TC-CMS-AUTHZ-08 |
| F055-AC-22 | HỆ THỐNG PHẢI không có màn hay nút sửa cấu hình quest/mini-game trong workspace C; quest/mini-game chỉ được chọn từ `listGameActivities` | FR-LMS-007 · AC-LMS-014 · ADR-0027 | TC-CMS-AUTHZ-05 |
| F055-AC-24 | KHI menu/action của feature (`menu.school_lms.assignments`, `menu.school_lms.courses`, `menu.school_lms.schedules`, `menu.school_lms.study_groups`, `menu.school_lms.game_activities` và action con) đang tắt cho phạm vi phiên (`disabled_menus`/`disabled_actions` của `getMyPermissions`), Portal PHẢI ẩn menu và ẩn/khóa nút tương ứng (`<ActionGate actionKey>`); NẾU API trả `403 FEATURE_DISABLED`, Portal PHẢI hiện thông báo tính năng tạm tắt và tải lại quyền | FR-CMS-018 · ADR-0090 | TC-CMS-MENU-02 |

### Edge Cases

- NẾU nội dung đã chọn bị gỡ gán hoặc unpublish giữa lúc mở wizard và lúc publish, THÌ Portal PHẢI hiển thị lỗi theo `code` (`RESOURCE_NOT_ASSIGNED`/`CONTENT_NOT_PUBLISHED`) và quay lại bước chọn nội dung (F055-AC-06).
- NẾU mạng lỗi sau khi gửi `createAssignment`, THÌ thử lại PHẢI dùng cùng `Idempotency-Key` (F055-AC-05).
- NẾU người dùng đổi tenant khi wizard có dữ liệu chưa lưu, THÌ Portal PHẢI cảnh báo trước khi đổi (shell F051).
- NẾU lớp có hơn `page_size` học sinh, THÌ bảng submission PHẢI phân trang cursor, không tải hết.
- NẾU `academic_manager` chọn course môn ngoài scope, THÌ server trả `403` và Portal báo, không tạo bài (F055-AC-19).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-LMS-001 | UI Course, Learning Path, Schedule (module LMS) | [SCHOOL_LMS §2–§3](../../docs/04-admin-portal/SCHOOL_LMS.md) |
| FR-LMS-002 | Wizard giao bài, publish, hủy, công bố kết quả | [SCHOOL_LMS §4](../../docs/04-admin-portal/SCHOOL_LMS.md) |
| FR-LMS-003 | Bảng theo dõi nộp bài của từng bài (dashboard tổng thuộc F056) | [SCHOOL_LMS §8.2](../../docs/04-admin-portal/SCHOOL_LMS.md) |
| FR-LMS-004 | Nhóm học tập (phần P3); thử thách ngoài phạm vi | [SCHOOL_LMS §2, §7](../../docs/04-admin-portal/SCHOOL_LMS.md) |
| FR-LMS-005 | Hiển thị theo scope trường/môn; không scope lớp | [SCHOOL_LMS §9](../../docs/04-admin-portal/SCHOOL_LMS.md) |
| FR-LMS-007 | Chỉ chọn quest/mini-game đã publish, được gán | [ADMIN_PORTAL §3.6](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-LMS-012 | Extend, excuse, remind | [SCHOOL_LMS §4.4](../../docs/04-admin-portal/SCHOOL_LMS.md) |
| FR-EXM-009 | Tab gia hạn thời gian thi cá nhân | [EXAM_ENGINE §5](../../docs/02-learning/EXAM_ENGINE.md) |
| FR-CNT-011 | Lựa chọn giữ phiên bản / `follow_latest` | [CONTENT_MODEL §5.2](../../docs/02-learning/CONTENT_MODEL.md) |
| FR-CMS-013 | Chọn tenant đích khi System Admin ghi | [ADMIN_PORTAL §2.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-018 | Menu/action của feature theo danh mục `specs/portal/menu-catalog.yaml`, ẩn khi tắt, xử lý `403 FEATURE_DISABLED` | [ADMIN_PORTAL §2.7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |

### Key Entities

- **Course**: lớp × môn × năm học, trạng thái `active`/`hidden`/`archived`; tự sinh ở backend.
- **LearningPath / PathNode**: chuỗi nội dung đã publish có điều kiện mở khóa.
- **Assignment**: bài giao với nội dung ghim version, đối tượng, thời gian, chính sách điểm, `FeedbackTiming`, `ResultRelease`, trạng thái.
- **AssignmentSubmission**: trạng thái nộp và điểm tự chấm của từng học sinh (read model).
- **ScheduleItem**: mục lịch của course.
- **StudyGroup**: nhóm học tập trong lớp/khối, là đối tượng giao bài.
- **ExamAccommodation**: gia hạn thời gian của một học sinh trong một exam session.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `late_window_hours` | Giới hạn ô cửa sổ nộp muộn (giá trị lấy từ schema/server) |
| `assignment_remind_limit` | Thông điệp khi nhắc vượt giới hạn |
| `graded_assignment_release_default` | Mặc định server áp cho bài `graded` |
| `score_scale` | Hiển thị điểm tự chấm |
| `portal_session_sso` | Phiên portal hết hạn → giữ URL, đăng nhập lại (shell F051) |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không business logic trong component; gọi API qua hook/service | AGENTS R1 · ADR-0022 |
| Style chỉ từ theme `admin-tokens`; không hardcode màu/khoảng cách | AGENTS R32 · ADR-0047 |
| WCAG 2.1 AA, điều hướng bàn phím | NFR-A11Y-003 · [DESIGN_SYSTEM §4.5](../../docs/05-ux/DESIGN_SYSTEM.md) |
| Responsive: wizard chỉ desktop/tablet; theo dõi, nhắc, gia hạn chạy trên mobile | [ADMIN_PORTAL §1.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: School Admin giao một bài cho một lớp từ màn tổng quan trong ≤ 2 phút (5 bước wizard) ở kịch bản quickstart.
- **SC-002**: 100% thao tác ghi (tạo, publish, extend, excuse, remind, release, cancel, unlock) có `Idempotency-Key`; bấm lặp không tạo bản ghi trùng (kiểm bằng test E2E).
- **SC-003**: 0 nút ghi hiển thị cho `tenant_admin`, Safety Moderator và người thiếu permission trong ma trận test quyền.
- **SC-004**: 0 lỗi axe mức serious/critical trên wizard, chi tiết bài, lịch, nhóm học tập.

## Ngoài phạm vi

- Backend LMS (course, path, assignment, schedule, study group): F040, F041; Exam Engine: F042.
- School Learning Dashboard, hồ sơ học sinh, bảng kết quả course, xuất báo cáo: F056.
- Thử thách lớp/giải trường từ template (FR-LMS-004 phần P4): backend F107; màn workspace C chưa có feature trong danh mục — đề xuất bổ sung CODE_PLAN.
- Shell, đăng nhập, chọn tenant, bộ lọc tenant: F051, F057; `apiClient`, `ops-ui`: F050.
- Nội dung `written_response`, `speaking`, `project`: deferred — ADR-0082.

## Assumptions

- Thư mục workspace C là `src/cms/frontend/apps/portal/src/workspaces/school-lms/`, route gốc `/portal/school-lms/...` (ADMIN_PORTAL §1.2, F051).
- `RegradeDialog` (ops-ui) do F058 tạo (cùng mốc M-C3); F055 chỉ gắn vào màn bài giao có đề kiểm tra.
- [Confirmed] Client TS sinh bằng `pnpm gen:api` (openapi-typescript) từ `specs/openapi/eduworld-api.v1.yaml`; `apiClient` (F050) tự gắn `Authorization`, `X-Tenant-Id`, `Idempotency-Key`, `If-Match`.
- Server là nguồn quyết định quyền, scope và trạng thái; Portal chỉ ẩn/hiện theo `getMyPermissions` (ADR-0027 mục 3).
- [Confirmed] Permission cho accommodations là `lms.assignment.write` cho tới khi contract chốt (F042).
