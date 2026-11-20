# Feature Specification: F042 — Exam Engine: 7 hình thức đề, random theo blueprint, thi có giờ, chấm lại đề

**Feature Branch**: `042-exam-engine` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `learning` · **Mốc**: M-C3 · **Phụ thuộc**: F024, F041
**Input**: Nhà trường tạo đề kiểm tra/thi thử từ Assessment Blueprint, học sinh làm bài có giờ theo server, server chấm và công bố kết quả theo chính sách, hệ thống chấm lại khi câu hỏi lỗi. FR nguồn: FR-EXM-001…006, FR-EXM-008, FR-EXM-009, FR-QB-005.

## Clarifications

### Session 2026-11-16

- Q: Cân bằng độ khó giữa biến thể và báo cáo độ tin cậy đề thuộc F042? → A: Không. F042 sinh đề theo blueprint (không cân bằng) và định nghĩa điểm mở rộng cân bằng; cân bằng theo `p_value` và báo cáo đề thuộc F044 (FR-EXM-010).
- Q: Phân tích câu sai và đề xuất ôn tập trả ở endpoint nào? → A: Đổi ở Session 2026-11-17: route riêng `getExamAttemptAnalysis`; `getExamResult` giữ điểm tổng quan.
- Q: Ai tạo ExamSession cho bài giao? → A: F041 gọi command tạo ExamSession khi assignment `mode = graded` với `assessment_blueprint`; nhà trường cũng gọi trực tiếp `createExamSession`. Đổi ở Session 2026-11-17: F042 cung cấp adapter cho port `ExamSessionCreator` của F041 (F042-AC-31).
- Q: Regrade đề dùng API nào? → A: `createRegradeRequest` dùng chung với re-evaluate (F031), `scope = exam_session`; F042 thực thi chấm lại attempt đề ([EXAM_ENGINE §9](../../docs/02-learning/EXAM_ENGINE.md#9-chấm-lại-regrade)).

### Session 2026-11-17

- Q: Mã lỗi khi vượt phạm vi trường/môn của UserRole? → A: `403 PERMISSION_DENIED`; `FORBIDDEN_SCOPE` không phát và sẽ bị gỡ khỏi `ProblemCode` ([API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md), RB-113).
- Q: Giữ route `/exams/attempts/{id}/analysis` và `/exams/sessions/{id}/report` không? → A: Có (RB-213 "Thêm", API_DESIGN §9.5): F042 thêm `getExamAttemptAnalysis`; `/report` là `getExamSessionReport` của F044. Cột `assessment.exam_sessions` là `exam_type`, `duration_sec` (EXAM_ENGINE §5, DATABASE_DESIGN, RB-212); field API `ExamSession.type`/`duration_seconds` ánh xạ ở DTO cho tới khi delta contract đổi tên [Confirmed].

- Q: Vòng phụ thuộc F041 ↔ F042 xử lý thế nào? → A: Technical Architect chốt thứ tự F040 → F041 → F042 ([CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý)). F041 sở hữu port `ExamSessionCreator` và trả `422 VALIDATION_FAILED` cho bài `graded` + blueprint khi chưa có adapter thật (F041-AC-09). F042 nhận phần còn lại của F041-AC-09 cũ: adapter gọi `CreateExamSession`/`CancelExamSession`, tạo ExamSession khi publish bài và lưu `exam_session_id`, hủy ExamSession khi hủy bài (F042-AC-31), cùng integration test tương ứng.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Tạo đề từ blueprint (Priority: P1)

`qlcm.truong.a` chọn blueprint "Phép nhân" đã publish, đặt loại đề, cửa sổ thời gian, đối tượng 3A, gia hạn thời gian cho một học sinh; hệ thống sinh và khóa các biến thể đề để xem trước.

**Why this priority**: Không có đề thì không có luồng thi; là bước đầu của kịch bản demo M-C3.

**Independent Test**: Gọi `createExamSession` với blueprint 4 biến thể `independent_draw` → session `scheduled`, 4 `ExamPaper` đúng ma trận LO × độ khó, seed lưu; gọi lại cùng `Idempotency-Key` → cùng session.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F042-AC-01 | HỆ THỐNG PHẢI hỗ trợ `exam_type` `practice_test`, `mock`, `chapter`, `semester`, `subject`, `custom` và thuộc tính có giờ `duration_sec`, với giá trị mặc định `is_official`, số lượt theo bảng loại đề | FR-EXM-001 · FR-EXM-004 · [EXAM_ENGINE §2](../../docs/02-learning/EXAM_ENGINE.md#2-loại-đề-confirmed--master-prompt-45) | TC-EXM-005 |
| F042-AC-02 | KHI `createExamSession` hợp lệ, module learning PHẢI ghim `blueprint_version_id` đã publish, sinh `ExamPaper` bất biến theo `variants.strategy` (`fixed`, `independent_draw`) đúng ma trận objective × độ khó × loại câu, lưu seed để tái lập và đặt session `scheduled` (hoặc `open` khi `window_start ≤ now`) | FR-EXM-002 · FR-QB-005 · [EXAM_ENGINE §4](../../docs/02-learning/EXAM_ENGINE.md#4-sinh-đề) | TC-EXM-001 |
| F042-AC-03 | NẾU blueprint có `feedback_timing = immediate`, tổng điểm lệch `total_points`, pool một rule thiếu câu, câu trùng giữa các rule hoặc loại câu không tự chấm, THÌ HỆ THỐNG PHẢI từ chối bằng `422 VALIDATION_FAILED` chỉ rõ rule; pool thiếu khi sinh đề PHẢI trả `422 QUESTION_POOL_INSUFFICIENT` | [EXAM_ENGINE §3](../../docs/02-learning/EXAM_ENGINE.md#3-assessment-blueprint) · [EXAM_ENGINE §13](../../docs/02-learning/EXAM_ENGINE.md#13-yêu-cầu--acceptance) | — |
| F042-AC-04 | KHI người tạo đề gọi `listExamSessionPapers`, HỆ THỐNG PHẢI trả các biến thể đã khóa (câu, điểm) chỉ cho người có quyền tạo đề trong phạm vi; học sinh gọi PHẢI nhận `403 PERMISSION_DENIED` | [EXAM_ENGINE §4](../../docs/02-learning/EXAM_ENGINE.md#4-sinh-đề) | — |
| F042-AC-05 | Ở NƠI session có `accommodations[]`, HỆ THỐNG PHẢI cộng phần trăm gia hạn vào `duration_sec` khi tính `deadline_at` cho đúng học sinh đó, và chỉ chấp nhận học sinh thuộc đối tượng của session | FR-EXM-009 · [EXAM_ENGINE §5](../../docs/02-learning/EXAM_ENGINE.md#5-examsession--cửa-sổ-thời-gian) | — |
| F042-AC-06 | HỆ THỐNG PHẢI chỉ cho `school_admin` (trường mình) và `academic_manager` (trường × môn được gán) tạo session; `academic_manager` tạo đề môn ngoài phạm vi PHẢI nhận `403 PERMISSION_DENIED`; đối tượng thuộc trường/tenant khác PHẢI nhận `404 NOT_FOUND`; System Admin tạo PHẢI gửi `X-Tenant-Id` tenant đích | AC-ORG-002 · AC-ORG-001 · [SCHOOL_LMS §9](../../docs/04-admin-portal/SCHOOL_LMS.md#9-phạm-vi-dữ-liệu-theo-vai-trò) | TC-TEN-002, TC-TEN-003 |
| F042-AC-07 | KHI `createExamSession` được gửi lại cùng `Idempotency-Key`, HỆ THỐNG PHẢI trả cùng session; mọi lần tạo session PHẢI ghi audit `exam_session.created` (actor, tenant, trường, blueprint version) | AGENTS R10 · [API_DESIGN §6](../../docs/07-backend/API_DESIGN.md#6-idempotency) | — |
| F042-AC-31 | KHI F042 đăng ký adapter `ExamSessionCreator` (port của F041) và người có quyền publish bài giao `mode = graded` với nội dung `assessment_blueprint`, HỆ THỐNG PHẢI gọi `CreateExamSession` trong cùng transaction publish để tạo ExamSession ghim blueprint version cho cùng đối tượng và cửa sổ thời gian (mặc định công bố theo `graded_assignment_release_default`), lưu `exam_session_id` vào bài giao; tạo bài `graded` + blueprint PHẢI không còn bị từ chối `422`; KHI bài đó bị hủy, HỆ THỐNG PHẢI gọi `CancelExamSession` chuyển session `cancelled` | [SCHOOL_LMS §4.1](../../docs/04-admin-portal/SCHOOL_LMS.md#41-tạo-và-giao-bài) · [EXAM_ENGINE §5](../../docs/02-learning/EXAM_ENGINE.md#5-examsession--cửa-sổ-thời-gian) · [CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý) | — |
| F042-AC-08 | NẾU `title` vượt `maxLength`, `window_end ≤ window_start`, `max_attempts` ngoài giới hạn hoặc phần trăm gia hạn ngoài miền cho phép, THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` | ADR-0041 | TC-TEN-024 |

---

### User Story 2 — Làm bài có giờ theo server (Priority: P1)

Học sinh 3A vào Exam Zone, bắt đầu đề 15 phút; đồng hồ theo giờ server, câu trả lời được lưu dần, rớt mạng vẫn làm tiếp trong thời gian còn lại, hết giờ thì server tự nộp.

**Why this priority**: Là trải nghiệm chính của học sinh và điều kiện công bằng của đề kiểm tra.

**Independent Test**: `hs.an.3a` gọi `startExamAttempt`, `saveExamAnswer` vài câu, thoát, gọi lại start → cùng attempt, `deadline_at` không đổi; để quá hạn → attempt `graded` với `submit_reason = auto_timeout`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F042-AC-09 | KHI học sinh thuộc đối tượng gọi `startExamAttempt` trong cửa sổ, HỆ THỐNG PHẢI tạo attempt `mode = exam` với `started_at` server, `deadline_at = min(started_at + duration hiệu lực, window_end)` theo `late_start_policy`, chọn biến thể theo `hash(student_id, exam_session_id) mod N`, xáo câu/đáp án theo seed attempt và trả `server_time` | FR-EXM-002 · FR-EXM-005 · [EXAM_ENGINE §6](../../docs/02-learning/EXAM_ENGINE.md#6-làm-bài--deadline-server) | TC-EXM-001 |
| F042-AC-10 | HỆ THỐNG PHẢI trả render payload câu hỏi đề không chứa `answer_key`, `scoring_rule`, `explanation`, `options[].feedback`, `hints`; media đề chỉ qua signed URL ngắn hạn của bucket private, không qua CDN công khai | AC-EXR-001 · AGENTS R6, R31 · ADR-0043, ADR-0044 | TC-LRN-007 |
| F042-AC-11 | NẾU học sinh không thuộc đối tượng, THÌ HỆ THỐNG PHẢI trả `403 EXAM_NOT_ASSIGNED`; NẾU trước `window_start`, THÌ `403 EXAM_NOT_OPEN`; NẾU sau `window_end`, THÌ `403 EXAM_WINDOW_CLOSED`; NẾU hết lượt, THÌ `409 EXAM_MAX_ATTEMPTS` | [EXAM_ENGINE §11](../../docs/02-learning/EXAM_ENGINE.md#11-api-tóm-tắt) | — |
| F042-AC-12 | KHI học sinh gọi `saveExamAnswer`, HỆ THỐNG PHẢI lưu câu trả lời append-only (bản mới nhất có hiệu lực), không trả `is_correct`, giải thích hay điểm, và từ chối gợi ý trong attempt đề | FR-EXM-003 · [EXAM_ENGINE §6](../../docs/02-learning/EXAM_ENGINE.md#6-làm-bài--deadline-server) | TC-LRN-007 |
| F042-AC-13 | KHI học sinh gọi lại `startExamAttempt` khi đã có attempt `in_progress`, HỆ THỐNG PHẢI trả attempt đó với câu trả lời đã lưu và `deadline_at` không đổi (thời gian vẫn chạy khi mất kết nối) | US-EXM-001 · [EXAM_ENGINE §13](../../docs/02-learning/EXAM_ENGINE.md#13-yêu-cầu--acceptance) | TC-LRN-009 |
| F042-AC-14 | NẾU attempt đang active trên thiết bị khác, THÌ HỆ THỐNG PHẢI trả `409 EXAM_ACTIVE_ON_OTHER_DEVICE`; KHI học sinh chọn chuyển sang thiết bị mới, HỆ THỐNG PHẢI vô hiệu thiết bị cũ và ghi audit `exam_attempt.device_switched` | [EXAM_ENGINE §12](../../docs/02-learning/EXAM_ENGINE.md#12-toàn-vẹn--chống-gian-lận) | — |
| F042-AC-15 | NẾU câu trả lời đến sau `deadline_at` + thời gian ân hạn đồng bộ, THÌ HỆ THỐNG PHẢI từ chối bằng `422 EXAM_TIME_OVER` | AC-EXM-001 | TC-EXM-002 |
| F042-AC-16 | KHI attempt `in_progress` quá `deadline_at` + ân hạn, job `exam.autosubmit` PHẢI chuyển attempt sang nộp với `submit_reason = auto_timeout` và đưa vào chấm; KHI cửa sổ đóng, HỆ THỐNG PHẢI auto-submit mọi attempt còn mở và ghi học sinh chưa bắt đầu là `absent` | AC-EXM-001 · FR-EXM-005 · [EXAM_ENGINE §7.2](../../docs/02-learning/EXAM_ENGINE.md#72-tự-động-nộp-khi-hết-giờ) | TC-EXM-002 |
| F042-AC-17 | HỆ THỐNG PHẢI chỉ cho principal `user_type = student` gọi các route làm bài trên attempt của chính mình trong tenant phiên; attempt của học sinh khác/tenant khác PHẢI trả `404 NOT_FOUND`; header `X-Tenant-Id` khác tenant phiên PHẢI trả `403 TENANT_MISMATCH` | AC-ORG-001 · AC-AUTH-024 | TC-TEN-001, TC-TEN-011 |

---

### User Story 3 — Nộp bài, chấm và công bố kết quả (Priority: P1)

Học sinh nộp bài một lần; server chấm trong worker; kết quả và đáp án hiện theo chính sách công bố; sau khi công bố học sinh thấy phân tích theo LO, câu sai có giải thích và đề xuất ôn tập.

**Why this priority**: Kết quả đúng, không trùng, không phụ thuộc client là yêu cầu cốt lõi của đề kiểm tra.

**Independent Test**: Nộp hai lần cùng key → một kết quả; session `after_close` → học sinh nộp sớm gọi `getExamResult` nhận `EXAM_RESULTS_NOT_RELEASED` tới khi cửa sổ đóng.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F042-AC-18 | KHI học sinh gọi `submitExamAttempt` với `Idempotency-Key`, HỆ THỐNG PHẢI chuyển attempt `in_progress → submitted` có điều kiện; gửi lại cùng key PHẢI trả cùng kết quả; khác key sau khi đã nộp hoặc sau auto-submit PHẢI trả `409 EXAM_ALREADY_SUBMITTED`; chỉ một kết quả tồn tại | FR-EXM-005 · AC-EXR-003 | TC-EXM-003, TC-LRN-008 |
| F042-AC-19 | KHI attempt đề `submitted`, worker `exam.grade` PHẢI chấm từng câu bằng grader server theo version đã ghim, ghi điểm câu, điểm attempt, `status = graded` và phát `exam.graded`; mọi trường điểm do client gửi PHẢI bị bỏ qua | FR-EXM-003 · AC-EXR-002 · ADR-0016 | TC-LRN-007 |
| F042-AC-20 | TRONG KHI kết quả chưa đến thời điểm công bố theo `result_release` (`immediate`, `after_close`, `manual`), HỆ THỐNG PHẢI trả `403 EXAM_RESULTS_NOT_RELEASED` cho học sinh và không trả đáp án, giải thích hay phản hồi phương án; `feedback_timing` không bao giờ sớm hơn công bố | FR-EXM-003 · FR-EXM-004 · [EXAM_ENGINE §8](../../docs/02-learning/EXAM_ENGINE.md#8-chấm--công-bố-kết-quả) | TC-EXM-005 |
| F042-AC-21 | KHI session có `result_release = manual` và người có `lms.result.release` gọi `releaseExamResults`, HỆ THỐNG PHẢI chuyển `results_released`, phát `exam.results_released`, ghi audit; gọi lại cùng `Idempotency-Key` PHẢI không phát lại event; người không có quyền PHẢI nhận `403 PERMISSION_DENIED` | FR-EXM-003 · AGENTS R10 | — |
| F042-AC-22 | KHI kết quả đã công bố, `getExamResult` PHẢI trả điểm, thang điểm theo `score_scale`, điểm theo section, và `getExamAttemptAnalysis` (`GET /exams/attempts/{attempt_id}/analysis`; học sinh chủ bài, `school_admin`/`academic_manager` trong phạm vi) PHẢI trả đúng/tổng theo LO, câu sai kèm đáp án và giải thích theo `show_correct_answers`/`show_explanations`, 1–3 LO yếu nhất kèm đề xuất ôn tập; cả hai không xếp hạng với bạn | FR-EXM-003 · [EXAM_ENGINE §10](../../docs/02-learning/EXAM_ENGINE.md#10-phân-tích-câu-sai--đề-xuất-ôn-tập) | TC-EXM-004 |
| F042-AC-23 | KHI exam được chấm, HỆ THỐNG PHẢI đăng ký gợi ý rule R5 (LO đúng < 60% và mastery < `proficient`) vào bộ gợi ý tự động của học sinh | FR-EXM-003 · [LEARNING_ANALYTICS §6](../../docs/02-learning/LEARNING_ANALYTICS.md#6-recommendation-rules) | TC-EXM-004 |
| F042-AC-24 | HỆ THỐNG PHẢI lưu attempt, câu trả lời, điểm, trạng thái session và tham gia trong PostgreSQL; kết quả PHẢI còn nguyên sau khi xóa hoặc khởi động lại Valkey | FR-EXM-006 · AC-EXM-002 · ADR-0004 | — |
| F042-AC-25 | KHI một attempt đề đúng 100% với thời gian dưới ngưỡng `suspicious_speed_flag`, HỆ THỐNG PHẢI gắn cờ `suspicious_speed` vào integrity signal và không tự hủy hay sửa kết quả | FR-EXM-005 · [ACTIVITY_ENGINE §11](../../docs/02-learning/ACTIVITY_ENGINE.md#11-toàn-vẹn-và-chống-gian-lận) | TC-EXM-006 |

---

### User Story 4 — Luyện đề tự do (Priority: P2)

Học sinh lớp 9 chọn đề thi thử trong thư viện, mỗi lần làm nhận bộ câu riêng, xem kết quả ngay sau khi nộp.

**Why this priority**: Giá trị luyện tập (US-EXM-001) nhưng không chặn luồng đề kiểm tra nhà trường giao.

**Independent Test**: `hs.dung.7a` gọi `listExamLibrary` → thấy đề `practice_test` của khối; làm hai lượt → hai bộ câu khác nhau (seed theo attempt), kết quả hiện ngay.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F042-AC-26 | KHI học sinh gọi `listExamLibrary`, HỆ THỐNG PHẢI chỉ trả session `practice_test`/`mock` không chính thức đang mở, đúng khối và thuộc đối tượng của học sinh trong tenant | US-EXM-001 · FR-EXM-001 | — |
| F042-AC-27 | Ở NƠI session dùng `variants.strategy = per_student`, HỆ THỐNG PHẢI rút đề riêng khi start với seed từ session, học sinh và số lượt, cho làm lại theo giới hạn lượt của loại đề, `result_release = immediate`, `feedback_timing = after_submit` | FR-EXM-002 · FR-EXM-004 · [EXAM_ENGINE §4](../../docs/02-learning/EXAM_ENGINE.md#4-sinh-đề) | TC-EXM-005 |

---

### User Story 5 — Chấm lại đề khi câu hỏi lỗi (Priority: P2)

Phát hiện câu sai đáp án sau kỳ kiểm tra, `qlcm.truong.a` tạo yêu cầu chấm lại với hành động loại câu; hệ thống chấm lại mọi attempt bị ảnh hưởng, lưu lịch sử điểm cũ.

**Why this priority**: Cần để sửa sai công bằng, nhưng chỉ xảy ra sau khi có kết quả (US3).

**Independent Test**: Session đã chấm, tạo `RegradeRequest` `scope = exam_session`, `action = drop_item` → điểm mới tính trên thang đã loại câu, `attempt_score_revisions` có điểm cũ; `ad.truong.a` gọi → 403.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F042-AC-28 | KHI người có `learning_content.re_evaluate` trong phạm vi tạo `createRegradeRequest` với `scope = exam_session`, `action ∈ {accept_all, use_new_key, drop_item}` và lý do, worker PHẢI chấm lại mọi attempt bị ảnh hưởng, ghi `attempt_score_revisions` (điểm cũ, điểm mới, lý do, người kích hoạt), giữ điểm gốc, phát `exam.regraded` và `attempt.re_evaluated` cho từng attempt | FR-EXM-008 · [EXAM_ENGINE §9](../../docs/02-learning/EXAM_ENGINE.md#9-chấm-lại-regrade) | TC-LRN-015 |
| F042-AC-29 | NẾU `school_admin` hoặc người không có `learning_content.re_evaluate` gọi, THÌ HỆ THỐNG PHẢI trả `403 PERMISSION_DENIED`; NẾU đã có yêu cầu `pending`/`running` cho cùng session và câu, THÌ PHẢI trả `409 CONFLICT`; gửi lại cùng `Idempotency-Key` PHẢI không chấm bù hai lần; mọi yêu cầu PHẢI ghi audit `content.re_evaluated` | FR-EXM-008 · AGENTS R10 | TC-LRN-015 |
| F042-AC-30 | HỆ THỐNG PHẢI không sửa question version đã publish khi chấm lại; `use_new_key` PHẢI tham chiếu version sửa lỗi đã publish | AGENTS R11 · ADR-0043 | TC-LRN-015 |

### Edge Cases

- NẾU học sinh bắt đầu muộn, THÌ `deadline_at` PHẢI theo `late_start_policy` (mặc định không vượt `window_end`) (F042-AC-09).
- NẾU blueprint hoặc câu hỏi được publish/rollback trong giờ thi, THÌ mọi học sinh vẫn làm trên version đã ghim của session (F042-AC-02, ADR-0043).
- NẾU học sinh bị khóa tài khoản hoặc chuyển lớp trong cửa sổ, THÌ attempt đang mở vẫn được auto-submit khi hết giờ; học sinh không còn thuộc đối tượng không start mới được.
- NẾU câu trả lời offline đến trễ nhưng `answered_at_client` trước `deadline_at` và server nhận trong ân hạn đồng bộ, THÌ được chấp nhận; ngược lại `EXAM_TIME_OVER` (F042-AC-15).
- NẾU học sinh không trả lời câu nào khi hết giờ, THÌ attempt vẫn chấm (0 điểm) với `submit_reason = auto_timeout`.
- NẾU job `exam.autosubmit` chạy trên nhiều instance, THÌ mỗi attempt chỉ được nộp một lần (`FOR UPDATE SKIP LOCKED`).
- NẾU regrade `drop_item` làm `max_score` = 0 ở một section, THÌ section đó bị loại khỏi tổng, không chia cho 0.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-EXM-001 | 6 `exam_type` + thuộc tính có giờ; thư viện đề luyện | [EXAM_ENGINE §2](../../docs/02-learning/EXAM_ENGINE.md#2-loại-đề-confirmed--master-prompt-45) |
| FR-EXM-002 | Sinh đề, random đề/câu, xáo đáp án, thời gian, thang điểm | [EXAM_ENGINE §3–§5](../../docs/02-learning/EXAM_ENGINE.md#3-assessment-blueprint) |
| FR-EXM-003 | Nộp, chấm, công bố theo `result_release`, phân tích câu sai, đề xuất ôn tập | [EXAM_ENGINE §7, §8, §10](../../docs/02-learning/EXAM_ENGINE.md#8-chấm--công-bố-kết-quả) |
| FR-EXM-004 | Phân biệt luyện tập và kiểm tra (`is_official`) | [EXAM_ENGINE §1, §2](../../docs/02-learning/EXAM_ENGINE.md#2-loại-đề-confirmed--master-prompt-45) |
| FR-EXM-005 | Chống nộp trùng, chống sửa điểm, tự nộp khi hết giờ, một thiết bị | [EXAM_ENGINE §7, §12](../../docs/02-learning/EXAM_ENGINE.md#12-toàn-vẹn--chống-gian-lận) |
| FR-EXM-006 | Kết quả bền vững trong PostgreSQL | [EXAM_ENGINE §12](../../docs/02-learning/EXAM_ENGINE.md#12-toàn-vẹn--chống-gian-lận) |
| FR-EXM-008 | Chấm lại đề do hệ thống, lịch sử điểm | [EXAM_ENGINE §9](../../docs/02-learning/EXAM_ENGINE.md#9-chấm-lại-regrade) |
| FR-EXM-009 | Gia hạn thời gian cá nhân `accommodations[]` | [EXAM_ENGINE §5](../../docs/02-learning/EXAM_ENGINE.md#5-examsession--cửa-sổ-thời-gian) |
| FR-QB-005 | Tạo đề từ Question Bank theo blueprint | [QUESTION_BANK §7](../../docs/02-learning/QUESTION_BANK.md#7-tạo-đề-từ-question-bank) |

### Key Entities

- **ExamSession**: một lần tổ chức đề cho đối tượng (lớp/nhóm/danh sách học sinh), ghim blueprint version, loại đề, cửa sổ, thời lượng, chính sách công bố, gia hạn cá nhân, trạng thái `scheduled → open → closed → results_released` (hoặc `cancelled`).
- **ExamPaper**: biến thể đề bất biến (danh sách question version theo section + điểm), seed sinh đề.
- **ExamParticipation**: trạng thái tham gia của từng học sinh trong session (gồm `absent`), số lượt đã dùng.
- **Attempt (mode exam)**: lượt làm bài, biến thể được chọn, `deadline_at`, lý do nộp, thiết bị đang active.
- **RegradeRequest / AttemptScoreRevision**: yêu cầu chấm lại do hệ thống chạy và lịch sử điểm của từng attempt.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `score_scale` | Thang hiển thị điểm kết quả |
| `suspicious_speed_flag` | Cờ tốc độ bất thường |
| `rapid_guess_min_response_ms` | Cờ đoán bừa trên câu trả lời đề (loại khỏi mastery) |
| `graded_assignment_release_default` | Mặc định `feedback_timing`/`result_release` khi session tạo từ bài giao `graded` |
| `answer_rate`, `attempt_start_rate` | Rate limit lưu câu/start |
| `idempotency_record_ttl` | Hạn bản ghi Idempotency-Key |

Ân hạn đồng bộ offline và chu kỳ job auto-submit theo [EXAM_ENGINE §6, §7.2](../../docs/02-learning/EXAM_ENGINE.md#72-tự-động-nộp-khi-hết-giờ) (chưa có tên trong CONFIG_DEFAULTS).

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Điểm, thời gian, nộp bài tính ở server; không gửi đáp án xuống client | AGENTS R5, R6 · ADR-0016 |
| Kết quả không phụ thuộc Valkey | AGENTS R9 · ADR-0004 |
| Nộp, công bố, chấm lại idempotent + audit | AGENTS R10 |
| Session ghim version; không sửa version đã publish | AGENTS R11, R31 · ADR-0043 |
| Câu hỏi/media đề không lên CDN công khai | AGENTS R31 · ADR-0044 |
| Phạm vi `school_admin`/`academic_manager` theo trường/môn | AGENTS R7, R8 · ADR-0072 |
| Không log câu trả lời học sinh ở mức INFO | AGENTS R14 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Với cùng seed, sinh lại đề cho ra đúng cùng danh sách câu và thứ tự trong 100% lần thử; mọi biến thể khớp ma trận blueprint.
- **SC-002**: Trong kịch bản 3 học sinh (1 nộp muộn, 1 rớt mạng rồi tiếp tục), 100% attempt có đúng một kết quả, attempt hết giờ được tự nộp trong vòng 1 phút sau hạn.
- **SC-003**: 0 trường đáp án/giải thích xuất hiện trong mọi response trước thời điểm công bố (contract test).
- **SC-004**: 100% kết quả còn nguyên sau khi xóa toàn bộ dữ liệu Valkey.
- **SC-005**: Chấm lại `drop_item` cập nhật điểm mọi attempt bị ảnh hưởng và truy xuất được điểm cũ của từng attempt.

## Ngoài phạm vi

- Cân bằng độ khó biến thể theo `p_value`, báo cáo độ tin cậy KR-20/alpha, phổ điểm, item analysis của đề — F044.
- Tạo blueprint và question set trong CMS — F024; giao đề qua assignment, port `ExamSessionCreator`, gia hạn/miễn bài giao — F041 (F042 chỉ cung cấp adapter cho port đó, F042-AC-31).
- Exam Zone, đồng hồ, tự nộp, resume trong Godot — F064; giao diện tạo đề và theo dõi — F055.
- Re-evaluate attempt ngoài đề (activity) — F031.
- Thi có giá trị pháp lý, proctoring (OQ-18) — ngoài Release.

## Assumptions

- [Confirmed, OQ-18] Chỉ luyện đề, thi thử và kiểm tra do nhà trường giao; `is_official` chỉ là "kết quả được ghi nhận tham khảo".
- [Confirmed] Thư viện đề luyện là ExamSession `practice_test`/`mock` không chính thức do nhà trường tạo cho khối/lớp với cửa sổ dài (từ blueprint của tenant hoặc blueprint platform được gán); học sinh không tự tạo session.
- [Confirmed] Permission: tạo/xem trước đề `lms.assignment.write`, xem session `lms.assignment.read`, công bố `lms.result.release`, chấm lại `learning_content.re_evaluate` ([ADMIN_PORTAL §2.2](../../docs/04-admin-portal/ADMIN_PORTAL.md#22-permission-catalog)).
- Ân hạn đồng bộ offline 30 giây và job auto-submit mỗi 15 giây theo EXAM_ENGINE là [Confirmed].
