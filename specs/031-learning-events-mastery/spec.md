# Feature Specification: F031 — Learning events, mastery v1, evaluation và re-evaluate

**Feature Branch**: `031-learning-events-mastery` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `learning` · **Mốc**: M-C2 · **Phụ thuộc**: F030
**Input**: LRS nội bộ `learning.learning_events` (append-only, xAPI-like), mastery theo `(student, LO)` bằng mô hình Weighted Evidence có version (ADR-0017), tiến độ học sinh, re-evaluate do hệ thống khi nội dung lỗi (ADR-0072) và tính lại mastery. FR nguồn: FR-ANL-001, FR-ANL-002, FR-ANL-004, FR-PRG-001…003, FR-PRG-009, FR-EXR-008, NFR-AVAIL-003.

## Clarifications

### Session 2026-11-16

- Q: Mastery lấy evidence từ đâu? → A: Consumer nội bộ nhận `activity.graded` (và `attempt.re_evaluated`), đọc các learning event `question.answered` của attempt trong cùng database (EVENT_DESIGN §3.2, §5.4). `exam.graded` thêm ở F042, `match.*` ở F105.
- Q: Giá trị `source` của evaluation chấm lại? → A: `re_evaluation` (OpenAPI `Evaluation.source`, ACTIVITY_ENGINE §7.3); DATABASE_DESIGN §2.6a ghi `system_reevaluation` sẽ được sửa.
- Q: F031 có gắn cờ `rapid_guess`/xử lý bài chẩn đoán? → A: Không. F031 chỉ tôn trọng cờ `rapid_guess`/`is_scored` có trong event (evidence bị loại); gắn cờ và đặt `M0` chẩn đoán thuộc F032.
- Q: Re-evaluate nội dung platform ảnh hưởng nhiều tenant? → A: `content_admin` kích hoạt → job chạy qua mọi tenant bằng `tenancy.Bypass(ctx, "re_evaluation")` có audit từng tenant; `academic_manager` chỉ tác động attempt của tenant và môn trong scope.

### Session 2026-11-17

- Q: Mã lỗi khi vượt phạm vi trường/môn của UserRole? → A: `403 PERMISSION_DENIED`; `FORBIDDEN_SCOPE` không phát và sẽ bị gỡ khỏi `ProblemCode` ([API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md), RB-113).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Ghi learning event làm bằng chứng học tập (Priority: P1)

Mọi kết quả học tập do server sinh được ghi bền vững, không trùng, không sửa, không chứa dữ liệu ngoài mục đích công bố.

**Why this priority**: Nguồn sự thật cho mastery, báo cáo nhà trường (F043) và đánh giá hiệu quả học tập.

**Independent Test**: Nộp một attempt (F030) → các learning event có đủ trường envelope, hợp lệ với `learning-event.schema.json`; phát lại event → không thêm dòng.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F031-AC-01 | HỆ THỐNG PHẢI ghi mỗi learning event với `event_id` UUIDv7, `occurred_at` theo server, `student_id`, `tenant_id`, `school_id`, `class_id` (snapshot lúc xảy ra), `session_id`, `activity_id`, `content_id`, `content_version_id` đã ghim, `context.mode`, `objective_ids`, `idempotency_key`, `actor.type` ∈ {`student`,`system`,`staff`} và hợp lệ với `learning-event.schema.json`. | FR-ANL-001 · [EVENT_DESIGN §5.1](../../docs/07-backend/EVENT_DESIGN.md) | TC-LRN-004 |
| F031-AC-02 | KHI một learning event có `idempotency_key` đã tồn tại được ghi lại, module learning PHẢI không tạo dòng mới; consumer nhận cùng domain event hai lần PHẢI xử lý đúng một lần (`processed_events`). | FR-ANL-001 · [EVENT_DESIGN §5.4](../../docs/07-backend/EVENT_DESIGN.md) | TC-SVC-003 |
| F031-AC-03 | HỆ THỐNG PHẢI giữ `learning.learning_events` append-only: role ứng dụng chỉ `INSERT`/`SELECT`; sửa sai chỉ bằng event bù trừ (`attempt.re_evaluated`). | FR-ANL-002 · R21 · ADR-0070 | TC-TEN-016 |
| F031-AC-04 | HỆ THỐNG PHẢI có partition tháng của `learning_events` cho tháng hiện tại và ít nhất hai tháng kế tiếp (job hằng ngày); NẾU job thất bại, THÌ PHẢI cảnh báo trước khi hết partition. | FR-ANL-002 · [DATABASE_DESIGN §5](../../docs/07-backend/DATABASE_DESIGN.md) | — |
| F031-AC-05 | HỆ THỐNG PHẢI không ghi vào learning event tên, email, ngày sinh, vị trí/đường đi trong game hay nội dung chat; payload ngoài schema PHẢI bị từ chối khi ghi. | FR-ANL-004 · [LEARNING_ANALYTICS §9](../../docs/02-learning/LEARNING_ANALYTICS.md) · R14 | — |
| F031-AC-06 | TRONG KHI Valkey không khả dụng, module learning PHẢI tiếp tục ghi learning event, evaluation và cập nhật mastery vào PostgreSQL không mất dữ liệu. | NFR-AVAIL-003 · ADR-0004 · R9 | TC-PERF-008 |
| F031-AC-07 | HỆ THỐNG PHẢI cô lập learning event và mastery theo tenant (RLS + `TenantScope`); truy vấn thiếu scope PHẢI trả 0 dòng. | R8 · ADR-0005 | TC-TEN-005 |

---

### User Story 2 — Mastery theo LO từ nhiều bằng chứng (Priority: P1)

Sau mỗi bài đã chấm, hệ thống cập nhật mức thành thạo của học sinh với từng LO theo mô hình cấu hình được; một bài điểm cao không đủ để "Làm chủ".

**Why this priority**: Là tín hiệu chính cho gợi ý tự học (F032) và dashboard nhà trường (F043).

**Independent Test**: Phát chuỗi evidence của bảng golden LEARNING_ANALYTICS §3.3 → giá trị và level khớp từng dòng; `learning_objective.mastered` phát đúng một lần.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F031-AC-08 | KHI nhận `activity.graded`, consumer mastery PHẢI lấy các `question.answered` của attempt có `is_scored = true` và `rapid_guess = false`, áp mô hình đang `active` cho từng `(student, LO)` theo thứ tự `occurred_at`, cập nhật `value`, `evidence_count`, `distinct_days`, `last_evidence_at`, `model_version` và ghi lịch sử mastery. | FR-PRG-002 · [LEARNING_ANALYTICS §3.1, §3.2](../../docs/02-learning/LEARNING_ANALYTICS.md) | TC-LRN-010 |
| F031-AC-09 | HỆ THỐNG PHẢI tính đúng mọi dòng của bảng ví dụ và ví dụ bổ sung tại [LEARNING_ANALYTICS §3.3](../../docs/02-learning/LEARNING_ANALYTICS.md) (golden test, sai số ≤ 0,0005). | FR-PRG-003 · ADR-0017 | TC-LRN-010 |
| F031-AC-10 | NẾU một LO chưa đủ `n_min` evidence trên `d_min` ngày khác nhau, THÌ module learning PHẢI không xếp level `mastered` dù giá trị ≥ ngưỡng (kể cả attempt điểm tuyệt đối). | AC-PRG-001 · ADR-0017 | TC-LRN-010 |
| F031-AC-11 | KHI đọc mastery, module learning PHẢI tính decay và level từ giá trị lưu (`Effective`) mà không ghi đè giá trị gốc. | FR-PRG-002 · [LEARNING_ANALYTICS §3.2](../../docs/02-learning/LEARNING_ANALYTICS.md) | TC-LRN-010 |
| F031-AC-12 | HỆ THỐNG PHẢI lưu tham số mô hình trong cấu hình có version, đúng một version `active` tại một thời điểm, và ghi `model_version` vào mỗi bản ghi mastery. | FR-PRG-003 · ADR-0017 | — |
| F031-AC-13 | KHI level của `(student, LO)` lần đầu thành `mastered`, module learning PHẢI phát `learning_objective.mastered` (learning event + outbox) đúng một lần; mỗi thay đổi giá trị PHẢI phát `learning_objective.progressed`; giảm rồi đạt lại do decay/recompute/re-evaluate PHẢI không phát lại `mastered`. | [LEARNING_ANALYTICS §3.4](../../docs/02-learning/LEARNING_ANALYTICS.md) | TC-LRN-010 |
| F031-AC-14 | KHI hai evidence của cùng học sinh được xử lý đồng thời, module learning PHẢI cập nhật tuần tự (khóa lạc quan theo `version`, thử lại) để kết quả bằng xử lý tuần tự. | [LEARNING_ANALYTICS §3.4](../../docs/02-learning/LEARNING_ANALYTICS.md) | TC-SVC-003 |

---

### User Story 3 — Tiến độ cho học sinh và nhà trường (Priority: P2)

Học sinh xem tiến độ và mức thành thạo của mình; `school_admin`/`academic_manager` xem hồ sơ học tập học sinh trong phạm vi, có audit.

**Why this priority**: Hiển thị kết quả của Content Loop; dashboard tổng hợp làm ở F043.

**Independent Test**: Sau 2 attempt, `getMyProgress` trả `items_answered`, `accuracy`, level từng LO; `ad.truong.b` xem học sinh `th-demo-a` → `404`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F031-AC-15 | KHI học sinh gọi `getMyProgress`, module learning PHẢI trả tổng `lessons_completed`, `items_answered`, `accuracy` (lần trả lời đầu), `active_minutes` và danh sách LO với `level` (sau decay), `evidence_count`, `last_evidence_at`, lọc theo môn; chỉ dữ liệu của chính học sinh. | FR-PRG-001 · [LEARNING_ANALYTICS §4](../../docs/02-learning/LEARNING_ANALYTICS.md) | — |
| F031-AC-16 | KHI `school_admin` (trường mình) hoặc `academic_manager` (môn trong scope) gọi `getStudentProgress`, module learning PHẢI trả hồ sơ và ghi audit `learning.profile_viewed`; trường khác → `404`; môn ngoài scope → `403 PERMISSION_DENIED`. | FR-PRG-001 · [LEARNING_ANALYTICS §7, §9](../../docs/02-learning/LEARNING_ANALYTICS.md) | TC-TEN-002 |
| F031-AC-17 | KHI nhận `activity.submitted`/`activity.graded`, consumer PHẢI cập nhật bộ đếm `progress_daily` theo `(student, ngày, môn)` idempotent. | FR-PRG-001 | TC-SVC-003 |

---

### User Story 4 — Chấm lại khi nội dung có lỗi (Priority: P2)

Phát hiện câu hỏi sai đáp án, `content_admin` hoặc `academic_manager` sửa bằng version mới rồi yêu cầu hệ thống chấm lại các attempt bị ảnh hưởng; không có chấm tay.

**Why this priority**: Bảo vệ kết quả học tập và mastery khi nội dung lỗi (FR-EXR-008); cần trước pilot.

**Independent Test**: 3 attempt đã chấm dùng question version lỗi → `createRegradeRequest` (`use_new_key`) → 3 evaluation mới `re_evaluation`, evaluation cũ `superseded`, 3 `attempt.re_evaluated`, mastery điều chỉnh, attempt gốc không đổi.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F031-AC-18 | KHI người có `learning_content.re_evaluate` gọi `createRegradeRequest` với `scope ∈ {question_version, activity_version}`, `action` và `reason` (10–1.000 ký tự), module learning PHẢI tạo yêu cầu `queued`, trả `202 RegradeRequest`, ghi audit `content.re_evaluated`; `use_new_key` PHẢI có `corrected_version_id` là version `published` của cùng item. | FR-EXR-008 · AC-ACT-003 | TC-LRN-015 |
| F031-AC-19 | NẾU người gọi là `school_admin` hoặc role khác không có `learning_content.re_evaluate`, THÌ module learning PHẢI trả `403 PERMISSION_DENIED`; `academic_manager` PHẢI chỉ tác động attempt của tenant và môn trong scope (ngoài scope → `403 PERMISSION_DENIED`). | FR-EXR-008 · ADR-0072 | TC-TEN-002 |
| F031-AC-20 | KHI job chạy yêu cầu, module learning PHẢI với mỗi attempt `graded` (mode ≠ `exam`) bị ảnh hưởng tạo evaluation mới `source = re_evaluation`, `effective = true`, chuyển evaluation cũ `superseded`, giữ nguyên attempt và câu trả lời gốc, phát `attempt.re_evaluated` (learning event `actor.type = staff` + outbox), cập nhật `affected_attempts`/`re_evaluated_attempts`. | FR-EXR-008 · [ACTIVITY_ENGINE §7.3](../../docs/02-learning/ACTIVITY_ENGINE.md) | TC-LRN-015 |
| F031-AC-21 | KHI nhận `attempt.re_evaluated`, consumer mastery PHẢI tính lại các `(student, LO)` bị ảnh hưởng từ learning event (dùng kết quả chấm lại thay cho kết quả cũ của item) mà không xóa event cũ. | FR-PRG-009 · [LEARNING_ANALYTICS §3.4](../../docs/02-learning/LEARNING_ANALYTICS.md) | TC-LRN-015 |
| F031-AC-22 | KHI gửi lại cùng `Idempotency-Key`, module learning PHẢI trả yêu cầu cũ; NẾU đã có yêu cầu `queued`/`running` cho cùng version, THÌ PHẢI trả `409 CONFLICT`; mọi thao tác ghi trên evaluation `superseded` PHẢI bị từ chối `409 EVALUATION_NOT_EDITABLE`. | R10 · FR-EXR-008 | TC-LRN-015 |

---

### User Story 5 — Tính lại mastery khi đổi mô hình (Priority: P3)

Khi đổi tham số/mô hình mastery, vận hành chạy tính lại ở chế độ shadow, so sánh rồi mới chuyển sang dùng.

**Why this priority**: Cần trước khi tinh chỉnh tham số sau pilot; không chặn M-C2.

**Independent Test**: Tạo `model_version` mới → chạy recompute shadow → báo cáo chênh lệch phân bố level; mastery hiện hành không đổi; kích hoạt → bản ghi mang `model_version` mới, không phát lại `mastered`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F031-AC-23 | KHI chạy job recompute shadow cho một `model_version`, module learning PHẢI tính lại mọi `(student, LO)` từ `learning_events` vào vùng shadow, xuất báo cáo chênh lệch (phân bố level, số LO đổi level) và không thay đổi mastery đang dùng. | FR-PRG-009 · FR-PRG-003 | — |
| F031-AC-24 | KHI kích hoạt `model_version` đã chạy shadow (System Admin, có audit), module learning PHẢI chuyển cấu hình `active`, áp kết quả shadow theo lô, và không phát lại `learning_objective.mastered` cho LO đã từng đạt. | FR-PRG-009 · ADR-0017 | — |

### Edge Cases

- Item gắn nhiều LO: mỗi LO nhận một evidence (F031-AC-08).
- Event đến lệch thứ tự giữa hai attempt của cùng học sinh: cập nhật theo `occurred_at` của evidence; evidence cũ hơn `last_evidence_at` được áp lại bằng tính lại LO đó.
- Học sinh chuyển lớp: event giữ `class_id` lúc xảy ra (F031-AC-01).
- Re-evaluate `drop_item`: item không còn trong `max_score`, evidence của item bị loại khỏi mastery.
- Re-evaluate attempt `abandoned`/`in_progress`: bỏ qua (chỉ `graded`).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-ANL-001 | Envelope, idempotency, kiểm schema mọi learning event; F030 ghi event attempt | [EVENT_DESIGN §5](../../docs/07-backend/EVENT_DESIGN.md) |
| FR-ANL-002 | LRS `learning.learning_events` append-only, partition, job partition | [LEARNING_ANALYTICS §2](../../docs/02-learning/LEARNING_ANALYTICS.md) |
| FR-ANL-004 | Purpose limitation ở tầng ghi event | [LEARNING_ANALYTICS §9](../../docs/02-learning/LEARNING_ANALYTICS.md) |
| FR-PRG-001 | Tiến độ cá nhân (`getMyProgress`, `getStudentProgress`, `progress_daily`); rollup lớp/trường ở F043 | [LEARNING_ANALYTICS §4](../../docs/02-learning/LEARNING_ANALYTICS.md) |
| FR-PRG-002 | Mastery `(student, LO)` từ evidence | [LEARNING_ANALYTICS §3.1](../../docs/02-learning/LEARNING_ANALYTICS.md) |
| FR-PRG-003 | `MasteryModel` cắm được, tham số có version, `model_version` | [LEARNING_ANALYTICS §3.1, §3.4](../../docs/02-learning/LEARNING_ANALYTICS.md) · ADR-0017 |
| FR-PRG-009 | Tính lại khi re-evaluate và khi đổi `model_version` (shadow) | [LEARNING_ANALYTICS §3.4](../../docs/02-learning/LEARNING_ANALYTICS.md) |
| FR-EXR-008 | Evaluation tách attempt, một evaluation hiệu lực, `RegradeRequest` do hệ thống chạy (non-exam) | [ACTIVITY_ENGINE §7](../../docs/02-learning/ACTIVITY_ENGINE.md) |
| NFR-AVAIL-003 | Dữ liệu học tập chỉ ở PostgreSQL; Valkey chỉ cache | ADR-0004 |

### Key Entities

- **LearningEvent**: bằng chứng học tập bất biến (ai, nội dung version nào, ngữ cảnh, LO, kết quả).
- **MasteryModelConfig**: tham số mô hình có version và trạng thái (draft/shadow/active/retired).
- **ObjectiveMastery**: trạng thái mastery hiện tại của `(student, LO)`; **MasteryHistory**: lịch sử thay đổi.
- **ProgressDaily**: bộ đếm tiến độ theo học sinh × ngày × môn.
- **RegradeRequest**: lệnh chấm lại do hệ thống chạy; **Evaluation**: kết quả chấm (system / re_evaluation, final / superseded).

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `mastery_model` | Mô hình và tham số mặc định (version hóa) |
| `w_hint` | Trọng số evidence theo số gợi ý |
| `guess_coefficient` | Hệ số đoán trúng theo loại câu |
| `w_ctx` (bổ sung) | Trọng số ngữ cảnh `challenge`, `timed_challenge` |
| `event_stream_retention` / `outbox_retention` | Thời gian giữ stream/outbox |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không xóa vật lý learning event; hết hạn thì ẩn danh hóa | ADR-0070 · R21 |
| Mastery khác Game Rating/XP; không nhận event game rating | ADR-0018 |
| Bỏ tenant filter chỉ qua `tenancy.Bypass` có audit (job re-evaluate platform) | R27 · ADR-0039 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% dòng golden test mastery khớp giá trị tham chiếu.
- **SC-002**: Mastery được cập nhật trong ≤ 5 giây p95 sau khi attempt được chấm [Proposed].
- **SC-003**: 0 learning event trùng `idempotency_key`; 0 learning event bị UPDATE/DELETE (kiểm quyền DB).
- **SC-004**: Mọi attempt bị ảnh hưởng của một yêu cầu chấm lại có đúng một evaluation hiệu lực sau khi job xong.

## Ngoài phạm vi

- Gắn cờ `rapid_guess`, gợi ý `hints:next`, bài chẩn đoán đặt `M0`, gợi ý ôn tập → F032.
- Rollup lớp/trường/tenant, School Learning Dashboard → F043; báo cáo, item analysis → F044.
- Evidence từ `exam.graded`, `exam.regraded` (scope `exam_session`) → F042; từ trận đấu → F105; từ `quest.completed` → F072.
- Export xAPI ra ngoài (chức năng tương lai, EVENT_DESIGN §5.3).

## Assumptions

- `class_id` snapshot lấy từ projection `ref.enrollments` (F020) tại thời điểm ghi.
- Kích hoạt `model_version` qua lệnh vận hành (`worker learning mastery-model activate`) có audit, không có endpoint public [Proposed].
- Bảng `assessment.regrade_requests` cho phép `tenant_id NULL` với yêu cầu của `content_admin` trên nội dung platform (đổi loại bảng T → H, task cập nhật DATABASE_DESIGN).
