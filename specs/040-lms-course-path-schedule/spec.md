# Feature Specification: F040 — Course, learning path, lịch, nhóm học tập

**Feature Branch**: `040-lms-course-path-schedule` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `learning` (LMS) · **Mốc**: M-C3 · **Phụ thuộc**: F031
**Input**: Nhà trường (School Admin, Academic Manager) có course tự tạo theo lớp × môn × năm học, dựng lộ trình học (learning path) cho học sinh tự học, lập lịch và tạo nhóm học tập để giao bài (ADR-0072). FR nguồn: FR-LMS-001, FR-LMS-004, FR-SOC-004, FR-CUR-008.

## Clarifications

### Session 2026-11-16

- Q: Curriculum version mặc định theo năm học của trường (`school.settings.curriculum_version_id`, CURRICULUM_MODEL §5) lưu ở đâu khi chưa có endpoint cài đặt trường nào chứa trường này? → A: [Confirmed] module learning (sở hữu curriculum) lưu cài đặt theo trường × năm học × khối; course đọc trực tiếp, không gọi module identity.
- Q: Rule gợi ý R6 (topic kế tiếp theo lịch LMS) thuộc feature nào? → A: F040 đăng ký R6 vào rule engine của F032 ([LEARNING_ANALYTICS §6](../../docs/02-learning/LEARNING_ANALYTICS.md#6-recommendation-rules)).
- Q: Học sinh đọc lộ trình và lịch trong game qua API nào? → A: Route học sinh nằm dưới `/learning/*` (`/lms/*` chỉ cho nhân sự, API_DESIGN §9.3); hạn nộp lấy từ `listMyAssignments` (F041); lộ trình và lịch bài học cần endpoint mới (contracts §6).
- Q: Study group thuộc module nào? → A: Đã chốt — [ADR-0084](../../docs/adr/ADR-0084-nhom-hoc-tap-thuoc-learning-service.md): module learning sở hữu (`lms.study_groups`, `lms.study_group_members`), phát `study_group.created/updated/member_changed/deleted` qua outbox; module chat (M5, F120) tạo channel `study_group` từ event; club do học sinh tạo vẫn thuộc module chat.
- Q: Sửa thành viên nhóm qua endpoint nào? → A: `updateStudyGroup` với `member_ids` thay toàn bộ danh sách (merge-patch, `If-Match`); server tính phần thêm/bớt để phát `study_group.member_changed`.

### Session 2026-11-17

- Q: Mã lỗi khi vượt phạm vi trường/môn của UserRole? → A: `403 PERMISSION_DENIED`; `FORBIDDEN_SCOPE` không phát và sẽ bị gỡ khỏi `ProblemCode` ([API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md), RB-113).
- Q: Learning path có version/workflow phát hành không? → A: Không (RB-216, [SCHOOL_LMS §3](../../docs/04-admin-portal/SCHOOL_LMS.md#3-course-và-learning-path)): sửa trực tiếp với `If-Match` + audit, áp dụng tiếp cho học sinh, tiến độ giữ theo `node_id`; cột `version` chỉ là khóa lạc quan.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Course tự tạo theo lớp × môn × năm học (Priority: P1)

Khi trường có lớp mới trong năm học (SIS hoặc CSV), mỗi môn của khối theo curriculum version trường đang dùng có sẵn một course; School Admin ẩn course trường không dạy và chọn curriculum version mặc định cho năm học.

**Why this priority**: Course là gốc của lộ trình, lịch, bài giao (F041) và bảng kết quả (F044).

**Independent Test**: Đồng bộ lớp 3A của TH Demo A → `listCourses?class_id=` trả một course cho mỗi môn khối 3; `ad.truong.a` ẩn course Âm nhạc → `status = hidden`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F040-AC-01 | KHI projection lớp nhận lớp mới (event `class.synced`), module learning PHẢI tạo đúng một course cho mỗi cặp (lớp, môn của khối trong curriculum version hiệu lực, năm học) và không tạo trùng khi event giao lại | FR-LMS-001 · [SCHOOL_LMS §3](../../docs/04-admin-portal/SCHOOL_LMS.md#3-course-và-learning-path) | — |
| F040-AC-02 | KHI tạo course, HỆ THỐNG PHẢI ghi curriculum version theo cài đặt của trường cho năm học và khối đó; NẾU trường chưa cài đặt, THÌ HỆ THỐNG PHẢI dùng version published mới nhất của curriculum mà tenant sở hữu hoặc được gán | FR-CUR-008 · [CURRICULUM_MODEL §5](../../docs/02-learning/CURRICULUM_MODEL.md#5-đa-chương-trình--di-chuyển-phiên-bản) | — |
| F040-AC-03 | KHI School Admin đặt curriculum version cho trường × năm học × khối, HỆ THỐNG PHẢI chỉ nhận version đã publish của curriculum thuộc tenant hoặc platform được gán (chưa publish → `409 CONTENT_NOT_PUBLISHED`; chưa gán → `403 RESOURCE_NOT_ASSIGNED`), ghi audit và chỉ áp cho course tạo sau thời điểm đổi | FR-CUR-008 · ADR-0029 | TC-CMS-ASG-01 |
| F040-AC-04 | KHI người dùng gọi `listCourses`, HỆ THỐNG PHẢI trả course trong phạm vi: School Admin trường mình, Academic Manager theo scope (tenant/trường/môn), Tenant Admin toàn tenant chỉ đọc | FR-LMS-001 · AC-ORG-002 · [SCHOOL_LMS §9](../../docs/04-admin-portal/SCHOOL_LMS.md#9-phạm-vi-dữ-liệu-theo-vai-trò) | TC-TEN-002, TC-TEN-003 |
| F040-AC-05 | KHI người có `lms.course.write` trong phạm vi gọi `updateCourse` với `If-Match` đúng, HỆ THỐNG PHẢI đổi `status` (`active`/`hidden`/`archived`) và ghi audit; thiếu `If-Match` → `428 PRECONDITION_REQUIRED`, lệch → `412 VERSION_MISMATCH` | FR-LMS-001 · [API_DESIGN §7](../../docs/07-backend/API_DESIGN.md#7-optimistic-concurrency-cmslms) | — |

---

### User Story 2 — Learning path cho học sinh tự học (Priority: P1)

Academic Manager dựng lộ trình từ thư viện nội dung đã publish; course chưa có lộ trình riêng dùng lộ trình mẫu theo bộ sách; lộ trình `mastery_gate` mở node khi LO trước đạt mức `developing`, nhà trường mở khóa thủ công khi cần.

**Why this priority**: Lộ trình là con đường học chính cùng gợi ý tự động (ADR-0072).

**Independent Test**: `qlcm.truong.a` tạo path `mastery_gate` 3 node cho course Toán 3A; `hs.an.3a` thấy node 2 khóa cho tới khi LO node 1 đạt `developing`; mở khóa thủ công có lý do → node mở, có audit.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F040-AC-06 | KHI người có `lms.learning_path.write` gọi `createLearningPath`/`updateLearningPath`, HỆ THỐNG PHẢI lưu `unlock_mode` và các node theo `position`, mỗi node ghim `content_version_id` của nội dung đã publish khả dụng cho tenant; learning path PHẢI không có version nội dung hay workflow duyệt/phát hành: `updateLearningPath` lưu trực tiếp với `If-Match`, ghi audit, áp dụng cho học sinh ở lần mở tiếp theo và giữ tiến độ/mở khóa theo `node_id` | FR-LMS-001 · [SCHOOL_LMS §3](../../docs/04-admin-portal/SCHOOL_LMS.md#3-course-và-learning-path) · ADR-0043 | — |
| F040-AC-07 | NẾU node tham chiếu nội dung chưa publish, THÌ HỆ THỐNG PHẢI trả `409 CONTENT_NOT_PUBLISHED`; NẾU tham chiếu nội dung platform chưa gán cho tenant, THÌ HỆ THỐNG PHẢI trả `403 RESOURCE_NOT_ASSIGNED` | ADR-0029 | TC-CMS-ASG-01 |
| F040-AC-08 | TRONG KHI course chưa có lộ trình riêng, HỆ THỐNG PHẢI trả lộ trình mẫu theo bộ sách của course cho cả nhân sự và học sinh | [SCHOOL_LMS §3](../../docs/04-admin-portal/SCHOOL_LMS.md#3-course-và-learning-path) | — |
| F040-AC-09 | TRONG KHI lộ trình ở chế độ `mastery_gate`, HỆ THỐNG PHẢI chỉ mở node cho học sinh khi mọi LO của node trước đạt ≥ `developing` hoặc học sinh đã được mở khóa thủ công; chế độ `sequential` mở theo hoàn thành node trước, `free` mở tất cả | [SCHOOL_LMS §3](../../docs/04-admin-portal/SCHOOL_LMS.md#3-course-và-learning-path) | — |
| F040-AC-10 | KHI gọi `unlockPathNodeForStudent` có lý do và `Idempotency-Key`, HỆ THỐNG PHẢI mở node cho đúng học sinh đó, ghi audit (người, lý do); NẾU thiếu lý do, THÌ `422 VALIDATION_FAILED`; NẾU học sinh không thuộc lớp của course, THÌ `404 NOT_FOUND` | [SCHOOL_LMS §3](../../docs/04-admin-portal/SCHOOL_LMS.md#3-course-và-learning-path) | — |
| F040-AC-11 | KHI học sinh gọi API lộ trình của mình, HỆ THỐNG PHẢI trả các course đang `active` của lớp học sinh với node và trạng thái khóa/mở, không trả course `hidden`/`archived` | FR-LMS-001 | TC-TEN-001 |
| F040-AC-12 | KHI học sinh chưa có evidence ở topic kế tiếp trong lịch LMS của course, rule R6 PHẢI tạo gợi ý bài học/quest tương ứng với ưu tiên 4 qua rule engine | [LEARNING_ANALYTICS §6](../../docs/02-learning/LEARNING_ANALYTICS.md#6-recommendation-rules) | — |

---

### User Story 3 — Lịch học, hạn nộp, đề (Priority: P2)

Nhà trường lập lịch bài học theo course; hạn nộp bài giao và lịch đề tự xuất hiện trên lịch; học sinh xem lịch trong game.

**Why this priority**: Lịch giúp học sinh biết việc sắp tới và là đầu vào cho R6, nhưng không chặn giao bài.

**Independent Test**: `ad.truong.a` tạo lịch bài học tuần sau cho course Toán 3A; `hs.an.3a` thấy mục lịch; xóa mục → không còn trong danh sách, có trong Thùng rác.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F040-AC-13 | KHI người có `lms.schedule.write` trong phạm vi gọi `createScheduleItem`/`updateScheduleItem`, HỆ THỐNG PHẢI lưu mục lịch (`kind` `lesson`/`assignment_due`/`exam`/`event`, ngày, tiết, `ref`) cho course trong phạm vi | FR-LMS-001 · [SCHOOL_LMS §2](../../docs/04-admin-portal/SCHOOL_LMS.md#2-mô-hình) | — |
| F040-AC-14 | KHI gọi `deleteScheduleItem`, HỆ THỐNG PHẢI soft-delete mục lịch (đặt `deleted_at`, `deleted_by`), ghi audit, phát `schedule_item.deleted` và loại khỏi mọi API đọc | ADR-0028 · AGENTS R21 | TC-TEN-015 |
| F040-AC-15 | KHI có event `assignment.published` hoặc `exam.opened` gắn course, HỆ THỐNG PHẢI tạo mục lịch `assignment_due`/`exam` tương ứng một lần duy nhất | [SCHOOL_LMS §2](../../docs/04-admin-portal/SCHOOL_LMS.md#2-mô-hình) | — |
| F040-AC-16 | KHI học sinh gọi API lịch của mình với khoảng ngày, HỆ THỐNG PHẢI trả mục lịch của các course `active` thuộc lớp học sinh trong khoảng đó | FR-LMS-001 | TC-TEN-001 |

---

### User Story 4 — Nhóm học tập (Priority: P3)

School Admin hoặc Academic Manager tạo nhóm học tập trong lớp hoặc khối để giao bài cho nhóm (học sinh không tự tạo nhóm dùng làm đối tượng giao bài). module learning sở hữu dữ liệu nhóm và thành viên (ADR-0084); mỗi thay đổi phát event để module chat (M5) dựng channel `study_group` sau này.

**Why this priority**: Đối tượng giao bài `class`/`students` đã đủ cho vòng M-C3 tối thiểu; nhóm học tập bổ sung đối tượng `study_group` cho F041.

**Independent Test**: `ad.truong.a` tạo nhóm "Ôn chia có dư" gồm 5 học sinh 3A, 3B → `listStudyGroups` có nhóm, outbox có `study_group.created`; thêm học sinh trường B → `422`; tạo trùng tên → `409`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F040-AC-17 | KHI người có `lms.study_group.write` trong phạm vi gọi `createStudyGroup` với `Idempotency-Key`, module learning PHẢI tạo nhóm theo lớp (có `class_id`) hoặc theo khối (`grade_level`), lưu thành viên, ghi `created_by`, audit `study_group.created` và trả `201` kèm `study_group_id` | FR-LMS-004 · FR-SOC-004 · US-LMS-008 · ADR-0084 | TC-SOC-008 |
| F040-AC-18 | KHI người có `lms.class.read` gọi `listStudyGroups` (lọc `class_id`, `grade_level`, phân trang cursor) hoặc `getStudyGroup`, HỆ THỐNG PHẢI chỉ trả nhóm chưa xóa của trường trong phạm vi, `getStudyGroup` kèm `member_ids` và `ETag` | FR-LMS-004 · [SCHOOL_LMS §9](../../docs/04-admin-portal/SCHOOL_LMS.md#9-phạm-vi-dữ-liệu-theo-vai-trò) | — |
| F040-AC-19 | KHI gọi `updateStudyGroup` với `If-Match` đúng, HỆ THỐNG PHẢI đổi tên/lớp/khối và thay toàn bộ danh sách thành viên theo `member_ids` (thành viên bị bỏ được soft-delete), tăng `version`, ghi audit `study_group.updated`; thiếu `If-Match` → `428 PRECONDITION_REQUIRED`, lệch → `412 VERSION_MISMATCH` | FR-LMS-004 · [API_DESIGN §7](../../docs/07-backend/API_DESIGN.md#7-optimistic-concurrency-cmslms) | TC-SOC-008 |
| F040-AC-20 | KHI gọi `deleteStudyGroup`, HỆ THỐNG PHẢI soft-delete nhóm và thành viên (`deleted_at`, `deleted_by`), ghi audit, loại khỏi mọi API đọc; bài giao đã gửi cho nhóm giữ snapshot đối tượng | ADR-0028 · AGENTS R21 · [SCHOOL_LMS §4.3](../../docs/04-admin-portal/SCHOOL_LMS.md#43-enrollment-thay-đổi-khi-bài-đang-mở) | TC-SOC-008, TC-TEN-015 |
| F040-AC-21 | NẾU tên nhóm trùng (không phân biệt hoa thường) với nhóm chưa xóa của cùng trường, THÌ HỆ THỐNG PHẢI trả `409 CONFLICT` và không ghi | ADR-0084 · AGENTS R21 | — |
| F040-AC-22 | NẾU `member_ids` rỗng, vượt 100, chứa học sinh không active của trường hoặc (khi có `class_id`) không học lớp đó, hoặc `name` rỗng/vượt 64 ký tự, THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` chỉ rõ trường, không lộ học sinh trường khác | ADR-0041 · AGENTS R16 | TC-TEN-024 |
| F040-AC-23 | KHI nhóm được tạo, sửa thông tin, đổi thành viên hoặc xóa, module learning PHẢI ghi outbox cùng transaction event `study_group.created`/`study_group.updated`/`study_group.member_changed` (chỉ ID thêm/bớt, `member_count`)/`study_group.deleted` theo payload `specs/events/payloads/learning/study_group.*.v1`, không gọi đồng bộ module chat và không chứa tên học sinh | ADR-0084 · ADR-0042 · AGENTS R13, R14 | — |

---

### User Story 5 — Phân quyền và cô lập tenant cho mọi API LMS của feature (Priority: P1)

Mọi endpoint của F040 chỉ cho đúng vai trò, đúng trường/môn, đúng tenant.

**Why this priority**: Bắt buộc theo AGENTS R7, R8.

**Independent Test**: Bộ test sinh từ OpenAPI gọi mọi endpoint F040 bằng `ad.truong.b`, `qlcm.truong.a` (ngoài môn), `sm.antoan`, `hs.an.3a` và `sa.admin`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F040-AC-24 | NẾU tài nguyên (course, path, mục lịch, nhóm) thuộc trường khác cùng tenant hoặc tenant khác, THÌ HỆ THỐNG PHẢI trả `404 NOT_FOUND` không lộ sự tồn tại; tham số `tenant_ids` của người dùng tenant PHẢI bị bỏ qua | AC-ORG-001 · AC-CMS-002 | TC-TEN-001, TC-TEN-003, TC-CMS-TEN-01 |
| F040-AC-25 | NẾU Academic Manager thao tác course/path/lịch/nhóm của môn ngoài scope, THÌ HỆ THỐNG PHẢI trả `403 PERMISSION_DENIED` | AC-ORG-002 | TC-TEN-002 |
| F040-AC-26 | NẾU người gọi là học sinh trên route `/lms/*`, role platform không phải `system_admin` (vd. `safety_moderator`, `content_admin`) hoặc Tenant Admin gọi thao tác ghi, THÌ HỆ THỐNG PHẢI trả `403` (`PORTAL_ACCESS_STAFF_ONLY` cho học sinh, `PERMISSION_DENIED` cho trường hợp còn lại) và ghi audit `authz.denied` | AGENTS R7, R20 · [ADMIN_PORTAL §2.3](../../docs/04-admin-portal/ADMIN_PORTAL.md#23-role--permission) | TC-TEN-004 |
| F040-AC-27 | KHI System Admin ghi dữ liệu LMS của một tenant, HỆ THỐNG PHẢI yêu cầu `X-Tenant-Id` tenant đích (thiếu → `400 TENANT_HEADER_REQUIRED`) và ghi audit có tenant đích | FR-ORG-008 · AC-ORG-004 | TC-TEN-004 |
| F040-AC-28 | KHI thao tác ghi (tạo path, mở khóa node, tạo/xóa lịch, đặt curriculum, tạo nhóm) được gửi lại cùng `Idempotency-Key`, HỆ THỐNG PHẢI trả cùng kết quả và không ghi lần hai; mọi thao tác ghi PHẢI có audit | AGENTS R10 · [API_DESIGN §6](../../docs/07-backend/API_DESIGN.md#6-idempotency) | — |
| F040-AC-29 | NẾU chuỗi đầu vào vượt `maxLength` hoặc enum sai, THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` chỉ rõ trường | ADR-0041 | TC-TEN-024 |

### Edge Cases

- NẾU lớp bị soft-delete ở identity (`class.deleted`), THÌ course PHẢI chuyển `archived`, không xóa (F040-AC-01, ADR-0028).
- NẾU học sinh chuyển lớp trong năm học, THÌ API lộ trình/lịch của học sinh PHẢI theo lớp active mới; mở khóa thủ công ở course cũ giữ trong lịch sử (ADR-0060).
- NẾU nội dung trong node được publish version mới, THÌ node vẫn dùng `content_version_id` đã ghim cho tới khi nhà trường cập nhật lộ trình (F040-AC-06).
- NẾU nội dung platform bị gỡ gán khỏi tenant, THÌ node hiện có vẫn hiển thị cho học sinh đã bắt đầu nhưng không thêm mới được (ADMIN_PORTAL §9).
- NẾU học sinh trong nhóm học tập rời trường (`student.cleaned_up`, enrollment kết thúc), THÌ nhóm PHẢI giữ thành viên cho tới khi nhà trường sửa nhóm; đối tượng giao bài lấy theo snapshot khi publish (F041).
- NẾU curriculum mới không còn một môn của khối, THÌ course cũ của môn đó giữ nguyên, không tạo course mới cho môn đó.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-LMS-001 | Module Course (tự tạo), Learning Path, Schedule, Enrollment (view); Assignment ở F041, dashboard ở F043 | [SCHOOL_LMS §2–§3](../../docs/04-admin-portal/SCHOOL_LMS.md#2-mô-hình) |
| FR-LMS-004 | Phần nhóm học tập do `school_admin`/`academic_manager` tạo (thử thách lớp thuộc F107) | [SCHOOL_LMS §2](../../docs/04-admin-portal/SCHOOL_LMS.md#2-mô-hình) |
| FR-SOC-004 | Study Group do nhà trường tạo, module learning sở hữu (ADR-0084); club/team học sinh và channel chat `study_group` thuộc F120 | [SCHOOL_LMS §2](../../docs/04-admin-portal/SCHOOL_LMS.md#2-mô-hình) · ADR-0084 |
| FR-CUR-008 | Curriculum version mặc định theo năm học của trường, dùng khi tạo course | [CURRICULUM_MODEL §5](../../docs/02-learning/CURRICULUM_MODEL.md#5-đa-chương-trình--di-chuyển-phiên-bản) |

### Key Entities

- **Course**: lớp × môn × năm học, có curriculum version, bộ sách, trạng thái `active`/`hidden`/`archived`.
- **SchoolCurriculumSetting**: curriculum version mặc định của trường theo năm học (và khối tùy chọn).
- **LearningPath / PathNode**: lộ trình của course (hoặc mẫu theo bộ sách), node có vị trí, nội dung ghim version, luật mở khóa.
- **PathNodeUnlock**: mở khóa thủ công một node cho một học sinh, có lý do.
- **ScheduleItem**: mục lịch của course (bài học, hạn nộp, đề, sự kiện).
- **StudyGroup**: nhóm học sinh trong lớp/khối do nhà trường tạo, dùng làm đối tượng giao bài; module learning sở hữu (ADR-0084).
- **StudyGroupMember**: học sinh thuộc nhóm (1–100), cùng trường, soft-delete khi bị bỏ khỏi nhóm.
- **Enrollment (view)**: dẫn xuất từ enrollment SIS, mỗi học sinh đúng một lớp active.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `lesson_completion_heartbeat` | Điều kiện hoàn thành node bài học (chế độ `sequential`) |
| `idempotency_record_ttl` | Thời hạn kết quả Idempotency-Key |
| `principal_cache_ttl` | Thời gian quyền thu hồi có hiệu lực ở phạm vi LMS |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Authn + authorization nghiệp vụ mọi endpoint, phạm vi tenant/trường/môn, không theo lớp phân công | AGENTS R7 · ADR-0072 · ADR-0066 |
| `TenantScope` + `deleted_at IS NULL`, chỉ soft-delete | AGENTS R8, R21, R27 · ADR-0005, ADR-0028 |
| Node ghim version nội dung đã publish | AGENTS R11, R31 · ADR-0043 |
| Không đọc DB module khác; dữ liệu lớp từ projection `ref` | AGENTS R13 · ADR-0037 |
| Chuỗi có giới hạn độ dài | AGENTS R28 · ADR-0041 |
| Không hiển thị tên thật học sinh ngoài phạm vi | AGENTS R16 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% lớp active của trường demo có đủ course cho mọi môn của khối sau một lần đồng bộ, 0 course trùng khi chạy lại đồng bộ.
- **SC-002**: 100% endpoint F040 qua bộ test phân quyền và cô lập tenant sinh từ OpenAPI (đúng mã 403/404).
- **SC-003**: Học sinh trong course `mastery_gate` chỉ thấy node mở đúng điều kiện ở 100% trường hợp trong bộ test.
- **SC-004**: Mọi thao tác ghi của feature có bản ghi audit tương ứng (đối soát 100% trong integration test).

## Ngoài phạm vi

- Assignment, `my-assignments`, rule R1 — F041; ExamSession và lịch đề sinh từ exam — F042; School Learning Dashboard — F043; bảng kết quả course — F044.
- Màn hình portal course/lộ trình/lịch/nhóm — F055; hiển thị lộ trình, lịch trong game — F062.
- Thử thách lớp/giải trường từ template — F107; club/team do học sinh tạo và channel chat `study_group` (tiêu thụ `study_group.*`) — F120.
- Đồng bộ enrollment/lớp từ SIS — F011.

## Assumptions

- [Confirmed] Course tự tạo, lộ trình mẫu theo bộ sách, `mastery_gate` mở khi LO trước ≥ `developing` (SCHOOL_LMS §3).
- [Confirmed] Cài đặt curriculum của trường lưu ở module learning; đổi cài đặt không đổi course đã tạo của năm học hiện tại.
- Projection `ref.classes`, `ref.enrollments`, `ref.students` đã có từ F020 (event `class.synced`, `enrollment.changed`).
- Seed GDPT Toán lớp 1–5 có bộ sách và topic để dựng lộ trình mẫu (OQ-08 [Confirmed]).
