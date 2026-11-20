# Feature Specification: F044 — Báo cáo, xuất dữ liệu bất đồng bộ, item analysis, chất lượng đề

**Feature Branch**: `044-reports-item-analysis` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `learning` · **Mốc**: M-C3 · **Phụ thuộc**: F042, F043
**Input**: Nhà trường xuất báo cáo kết quả (bài giao, course, tham khảo TT27/TT22, bản đồ kỹ năng) bằng job nền có link hết hạn và audit; xem bảng kết quả course theo điểm tự chấm; đội nội dung thấy thống kê chất lượng câu hỏi và đề xuất hiệu chỉnh độ khó có người duyệt; người tạo đề nhận đề cân bằng giữa biến thể và báo cáo độ tin cậy. FR nguồn: FR-ANL-014, FR-ANL-015, FR-LMS-013, FR-QB-010, FR-QB-014, FR-EXM-010.

## Clarifications

### Session 2026-11-16

- Q: Cân bằng độ khó giữa biến thể (FR-EXM-010) chạy trong F042 hay F044? → A: F042 sinh đề và định nghĩa interface `VariantBalancer`; F044 hiện thực bộ cân bằng vì cần `p_value` từ item analysis do F044 tính ([EXAM_ENGINE §4.1](../../docs/02-learning/EXAM_ENGINE.md)).
- Q: Phân tích đề/bài cho nhà trường (US-LMS-007) thuộc feature nào? → A: F044, trong báo cáo đề `GET /exams/sessions/{id}/report` (EXAM_ENGINE §8.1, §11), đọc kết quả đã chấm và thống kê câu hỏi.
- Q: Ngưỡng ≥ 30 lượt cho item analysis lấy ở đâu? → A: Do [QUESTION_BANK §9](../../docs/02-learning/QUESTION_BANK.md) sở hữu (chưa có trong CONFIG_DEFAULTS); ngưỡng hiệu chỉnh độ khó dùng `difficulty_calibration`.

### Session 2026-11-17

- Q: Mã lỗi khi vượt phạm vi trường/môn của UserRole? → A: `403 PERMISSION_DENIED`; `FORBIDDEN_SCOPE` không phát và sẽ bị gỡ khỏi `ProblemCode` ([API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md), RB-113).

### Session 2026-11-21 (triển khai)

- Q: Bảng mới viết bằng ent schema (T006) hay SQL tay? → A: SQL tay trong migration `20261226000001_f044_reports_quality` và repository SQL tường minh, như F040–F043 (bảng lms/assessment/rollup không có entity ent); mọi chuỗi `varchar(n)`, jsonb có CHECK `octet_length` [Confirmed theo tiền lệ F041–F043].
- Q: Work-queue `LEARNING_REPORT_JOBS` (NATS) hay bảng job? → A: Job `report-jobs` (worker profile `jobs`, 10 giây) lấy job `queued` bằng `FOR UPDATE SKIP LOCKED` trên `learning.report_jobs`, như `import-jobs`/`translation-jobs`; job `running` quá 30 phút được chạy lại; `report.completed` qua outbox [Proposed].
- Q: Ai có `report.export`? → A: Theo catalog role hiện hành (school_admin, academic_manager, tenant_admin, system_admin); đọc job: người tạo, đúng route (`/lms`, `/tenant`, `/admin`), cùng tenant — khác → `404`; không có `report.export` → `403`.
- Q: `report_type` `game_summary` (F080) và `efficacy_dataset` (F035) của `ReportRequest`? → A: Trả `422 VALIDATION_FAILED` (`report_type`) cho tới khi feature sở hữu giao; PDF chỉ cho báo cáo LMS [Proposed].
- Q: Bảng ánh xạ cột/khối của item analysis? → A: Mỗi version ghi một dòng/khối (khối của học sinh trong `ref.students`) và một dòng gộp `grade_level = 0`; `p_value` cho bộ cân bằng lấy dòng gộp; đề xuất hiệu chỉnh dùng khối có nhiều lượt nhất (≥ 100 lượt trả lời đầu không gợi ý, đã loại `rapid_guess`); job đêm tính lại version có câu trả lời trong 48 giờ (toàn bộ lịch sử của version) [Proposed].
- Q: Ngưỡng cờ "nên viết phản hồi cho phương án"? → A: Phương án sai được ≥ 25% lượt hợp lệ chọn; cờ "độ phân biệt thấp" khi discrimination < 0,1 (EXAM_ENGINE §8.1) [Proposed].
- Q: Lịch kỳ đánh giá khi chưa có lịch LMS? → A: Năm học bắt đầu 1/9; giữa kỳ I tới 15/11, cuối kỳ I tới 15/1, giữa kỳ II tới 31/3, cuối kỳ II tới 31/8; phạm vi LO = 25/50/75/100% LO của khối × môn theo thứ tự curriculum (ASSESSMENT_REGULATION_PROFILE §4, trường hợp không có lịch LMS); phạm vi theo lịch LMS là việc tiếp theo [Proposed].
- Q: Điểm đề trong báo cáo đề/độ tin cậy dùng lượt nào? → A: Lượt đã chấm cuối cùng của học sinh (`exam_participations.last_attempt_id`); tỉ lệ đạt = điểm thang 10 ≥ 5 [Proposed].
- Q: Bộ cân bằng (F044-AC-22) — "seed cố định"? → A: Thuật toán tham lam xác định (thứ tự cố định theo version id, không ngẫu nhiên): hoán đổi cùng vị trí rule giữa biến thể dễ nhất/khó nhất hoặc rút lại vị trí đó từ pool trong rule; cùng đầu vào → cùng đề (tái lập). Kết quả trả ở `createExamSession.balance` và `ExamPaper.balance_warning`.
- Q: Payload `report.completed`? → A: `report_id`, `scope_level`, `report_type`, `format`, `status` (`succeeded`/`failed`), `requested_by`, `row_count`; EVENT_DESIGN §4 đã đồng bộ.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Xuất báo cáo bất đồng bộ (Priority: P1)

`qlcm.truong.a` bấm "Xuất" kết quả bài giao "Phép nhân" của 3A; hệ thống tạo job nền, khi xong cho link tải ký có hạn; file có watermark và chỉ chứa dữ liệu trong quyền.

**Why this priority**: Nhà trường cần mang kết quả vào họp chuyên môn (US-LMS-010); xuất lớn luôn bất đồng bộ để không làm chậm hệ thống (ADR-0062).

**Independent Test**: `createLmsReport` (`report_type = assignment_results`, `format = xlsx`) → `202 report_id`; `getLmsReport` → `succeeded` + URL; tải được trong hạn, quá hạn → 403; audit `report.exported`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F044-AC-01 | KHI người có `report.export` gọi `createLmsReport` hoặc `createTenantReport` với `Idempotency-Key`, module learning PHẢI tạo job, trả `202` kèm `report_id` và xử lý ở worker đọc read replica theo lô; gửi lại cùng key PHẢI trả cùng `report_id` | FR-ANL-014 · FR-LMS-013 · [DASHBOARD_DATA §10](../../docs/07-backend/DASHBOARD_DATA.md) | TC-DASH-008 |
| F044-AC-02 | KHI job xong, HỆ THỐNG PHẢI lưu file CSV/XLSX/PDF trên object storage `ew-exports`, trả link ký hết hạn sau `dashboard_export_link_ttl_minutes` chỉ cho người tạo, giữ file `dashboard_export_file_retention_days`; NẾU link quá hạn, THÌ PHẢI trả `403` | FR-ANL-014 | TC-DASH-008 |
| F044-AC-03 | HỆ THỐNG PHẢI chèn watermark (người xuất, thời điểm), chỉ xuất dữ liệu trong phạm vi quyền (trường/môn của UserRole), không xuất nội dung chat, và ghi audit `report.exported` | FR-LMS-013 · [SCHOOL_LMS §10](../../docs/04-admin-portal/SCHOOL_LMS.md) (AC-LMS-013) | TC-DASH-008 |
| F044-AC-04 | NẾU người dùng đã có `dashboard_export_max_concurrent_per_user` job đang chạy, THÌ HỆ THỐNG PHẢI trả `429 EXPORT_LIMIT_REACHED` có `Retry-After`; NẾU phạm vi vượt `dashboard_export_max_rows` dòng, THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` yêu cầu thu hẹp | [DASHBOARD_DATA §10](../../docs/07-backend/DASHBOARD_DATA.md) | TC-DASH-008 |
| F044-AC-05 | NẾU người gọi `getLmsReport`/`getTenantReport` không phải người tạo hoặc khác tenant, THÌ HỆ THỐNG PHẢI trả `404 NOT_FOUND`; NẾU thiếu `report.export`, THÌ PHẢI trả `403 PERMISSION_DENIED` | AC-ORG-001 · AGENTS R7 | TC-TEN-001 |
| F044-AC-06 | KHI System Admin gọi `createPlatformReport` với `tenant_ids[]`, HỆ THỐNG PHẢI xuất dữ liệu gộp có cột tenant, ẩn ô < `analytics_min_cell_size` học sinh và ghi audit `cross_tenant.accessed` + `report.exported`; role tenant gọi route `/admin/reports` PHẢI nhận `403 PERMISSION_DENIED` | FR-ANL-014 · FR-ANL-016 · AC-ORG-004 | TC-TEN-004, TC-TEN-018 |
| F044-AC-07 | NẾU body tạo báo cáo có enum sai, chuỗi vượt `maxLength` hoặc khoảng ngày > `dashboard_max_range_days`, THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` hoặc `422 DASHBOARD_RANGE_TOO_LARGE` | ADR-0041 · FR-ANL-013 | TC-TEN-024, TC-DASH-002 |

---

### User Story 2 — Bảng kết quả course theo điểm tự chấm (Priority: P1)

School Admin xem bảng kết quả course 3A × Toán: mỗi cột là một bài `graded`, giá trị là điểm tự chấm theo chính sách điểm, thang 10; không có cột XP, rating hay điểm thi đấu.

**Why this priority**: Thay sổ điểm chấm tay (ADR-0072), là nguồn cho báo cáo xuất.

**Independent Test**: Sau kịch bản giao đề F042, `getCourseResults` trả học sinh × bài graded với điểm đúng `score_policy`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F044-AC-08 | KHI người có `lms.report.read` trong phạm vi gọi `getCourseResults`, HỆ THỐNG PHẢI trả học sinh × bài `mode = graded` với điểm tự chấm theo `score_policy` của bài, quy đổi theo `score_scale`, trạng thái nộp, cờ "làm quá nhanh", và điểm hiệu lực sau re-evaluate | FR-LMS-013 · [SCHOOL_LMS §10](../../docs/04-admin-portal/SCHOOL_LMS.md) | — |
| F044-AC-09 | HỆ THỐNG PHẢI không trả cột XP, Game Rating, competition points hay xếp hạng học sinh trong bảng kết quả và file xuất | FR-LMS-013 · ADR-0018 | — |
| F044-AC-10 | NẾU course thuộc trường/môn ngoài phạm vi người gọi, THÌ HỆ THỐNG PHẢI trả `404 NOT_FOUND` (khác trường/tenant) hoặc `403 PERMISSION_DENIED` (môn ngoài scope `academic_manager`) | AC-ORG-002 · AC-ORG-003 | TC-TEN-002, TC-TEN-003 |

---

### User Story 3 — Báo cáo tham khảo theo TT27/2020 và TT22/2021 (Priority: P2)

Academic Manager xem và xuất mức tham khảo theo môn × kỳ cho trường → lớp → học sinh, luôn kèm dòng "chỉ tham khảo, không thay đánh giá chính thức".

**Why this priority**: Giúp nhà trường đọc kết quả bằng ngôn ngữ quy định (FR-ANL-015), cần mastery và kết quả đề.

**Independent Test**: Học sinh lớp 4 có 85% LO ≥ `proficient`, 45% `mastered`, độ phủ 90% → "Hoàn thành tốt"; độ phủ 50% → "Chưa đủ dữ liệu".

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F044-AC-11 | KHI job hằng đêm chạy, HỆ THỐNG PHẢI tính mức tham khảo học sinh × môn × kỳ theo hồ sơ `primary_tt27`/`lower_secondary_tt22` và ngưỡng `assessment_reference_thresholds`, từ `M_eff` cuối kỳ và điểm đề `is_official`; độ phủ dưới ngưỡng PHẢI cho "Chưa đủ dữ liệu" | FR-ANL-015 · [ASSESSMENT_REGULATION_PROFILE §4](../../docs/02-learning/ASSESSMENT_REGULATION_PROFILE.md) (AC-ARP-001, AC-ARP-002) | TC-LRN-018 |
| F044-AC-12 | HỆ THỐNG PHẢI kèm `disclaimer` trên mọi response và ở đầu/chân mọi file xuất báo cáo tham khảo, ghi audit `learning.reference_report_exported` khi xuất | FR-ANL-015 · (AC-ARP-003) | TC-LRN-018 |
| F044-AC-13 | HỆ THỐNG PHẢI không trả nhãn mức thông tư qua bất kỳ API học sinh nào | (AC-ARP-004) · [ASSESSMENT_REGULATION_PROFILE §1](../../docs/02-learning/ASSESSMENT_REGULATION_PROFILE.md) | — |
| F044-AC-14 | KHI ngưỡng thay đổi, HỆ THỐNG PHẢI tạo version hồ sơ mới và giữ kết quả các kỳ trước theo version đã dùng | (AC-ARP-005) | — |
| F044-AC-15 | NẾU `academic_manager` xem môn ngoài scope, THÌ HỆ THỐNG PHẢI trả `403 PERMISSION_DENIED`; báo cáo PHẢI hiển thị tỉ lệ "làm quá nhanh" và đánh dấu đề có cảnh báo độ tin cậy thấp | (AC-ARP-006) · [ASSESSMENT_REGULATION_PROFILE §4.3](../../docs/02-learning/ASSESSMENT_REGULATION_PROFILE.md) | TC-TEN-002 |

---

### User Story 4 — Thống kê chất lượng câu hỏi (item analysis) (Priority: P2)

Biên tập viên mở một câu hỏi trong CMS và thấy `p_value`, độ phân biệt, tỉ lệ chọn từng phương án nhiễu, thời gian trung bình, tỉ lệ đoán bừa và dùng gợi ý; hệ thống không tự sửa version đã publish.

**Why this priority**: Phát hiện câu lỗi để rollback hoặc re-evaluate, và là đầu vào cho hiệu chỉnh độ khó, cân bằng đề.

**Independent Test**: Câu có ≥ 30 lần trả lời đầu (sau khi loại `rapid_guess`) → worker ghi thống kê; `getQuestionStats` trả số liệu theo version.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F044-AC-16 | KHI job item analysis hằng đêm chạy, HỆ THỐNG PHẢI tính cho mỗi question version có đủ lượt theo QUESTION_BANK §9: `p_value`, `discrimination` (point-biserial), `distractor_rate` từng phương án, `avg_time_ms`, `rapid_guess_rate`, `hint_usage_rate`, chỉ dùng lần trả lời đầu có tính điểm, loại `rapid_guess`, tách lần có dùng gợi ý | FR-QB-010 · [QUESTION_BANK §9](../../docs/02-learning/QUESTION_BANK.md) | — |
| F044-AC-17 | KHI biên tập viên có `learning_content.read` gọi thống kê câu hỏi, HỆ THỐNG PHẢI trả số liệu theo từng version và cờ "cần hiệu chỉnh độ khó", "phương án không ai chọn", "nên viết phản hồi cho phương án"; thống kê PHẢI không làm đổi version đã publish | FR-QB-010 · AGENTS R11 | — |
| F044-AC-18 | HỆ THỐNG PHẢI chỉ trả thống kê của câu thuộc tenant người gọi hoặc platform được gán (chỉ đọc); câu tenant khác PHẢI trả `404 NOT_FOUND` | ADR-0029 · AC-ORG-001 | TC-TEN-001 |

---

### User Story 5 — Đề xuất hiệu chỉnh độ khó có người duyệt (Priority: P3)

Hằng tuần hệ thống so độ khó khai báo với độ khó suy từ `p_value`; lệch ≥ 1 mức tạo đề xuất trong hàng đợi CMS; người duyệt chấp nhận thì hệ thống tạo version mới chỉ đổi độ khó.

**Why this priority**: Cải thiện chất lượng ngân hàng câu theo dữ liệu thật, cần US4.

**Independent Test**: Câu khai báo độ khó 2 có `p_value` 0,40 trên ≥ 100 lượt → đề xuất độ khó 4; chấp nhận → version mới `change_note = "difficulty_calibration"` ở trạng thái chờ duyệt/publish.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F044-AC-19 | KHI job hằng tuần chạy, HỆ THỐNG PHẢI tạo đề xuất hiệu chỉnh cho câu có số lượt và bảng ánh xạ theo `difficulty_calibration` mà độ khó đề xuất lệch ≥ 1 mức, kèm `p_value`, số lượt, phân bố theo khối; câu bị bỏ qua 2 lần liên tiếp PHẢI không được đề xuất lại trong 90 ngày | FR-QB-014 · [QUESTION_BANK §9.1](../../docs/02-learning/QUESTION_BANK.md) | — |
| F044-AC-20 | KHI người có `learning_content.review` đúng `owner_scope` chấp nhận đề xuất (có `Idempotency-Key`), HỆ THỐNG PHẢI tạo question version mới chỉ đổi `difficulty` với `change_note = "difficulty_calibration"` đi qua workflow duyệt/publish thường, không sửa version đã publish, và ghi audit | FR-QB-014 · AGENTS R10, R11 · ADR-0043 | — |
| F044-AC-21 | NẾU người gọi thiếu `learning_content.review`, là tác giả khi reviewer ≠ author bắt buộc, hoặc đề xuất thuộc câu platform mà người gọi là role tenant, THÌ HỆ THỐNG PHẢI trả `403 PERMISSION_DENIED` | ADR-0027 · ADR-0029 | TC-CMS-ASG-02 |

---

### User Story 6 — Chất lượng đề: cân bằng biến thể, độ tin cậy, phân tích đề (Priority: P2)

Người tạo đề "Phép nhân" 4 biến thể nhận các biến thể tương đương độ khó; sau khi đề chính thức đóng, báo cáo đề có phổ điểm, p-value theo câu, lựa chọn sai phổ biến, độ tin cậy KR-20/alpha và SEM từng biến thể.

**Why this priority**: Công bằng giữa học sinh và cho biết khi nào nên dùng kết quả thận trọng (FR-EXM-010, US-LMS-007).

**Independent Test**: Blueprint 4 biến thể `independent_draw` có `balance.enabled` → chênh `expected_p` ≤ ngưỡng hoặc cảnh báo kèm số liệu; session official 40 học sinh đóng → `getExamSessionReport` có KR-20/alpha, SEM, cảnh báo khi < ngưỡng.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F044-AC-22 | KHI F042 sinh đề `independent_draw` có `balance.enabled`, bộ cân bằng PHẢI tính `P_v` từ `p_value` (hoặc `p̂` suy từ `difficulty`), hoán đổi/rút lại với seed cố định tối đa 200 lần để chênh ≤ `variant_balance_max_mean_p_diff`, lưu `expected_p` từng biến thể; NẾU không đạt, THÌ PHẢI vẫn sinh đề và trả cảnh báo kèm số liệu từng biến thể | FR-EXM-010 · [EXAM_ENGINE §4.1](../../docs/02-learning/EXAM_ENGINE.md) | TC-EXM-001 |
| F044-AC-23 | KHI session `is_official` chuyển `closed`, worker PHẢI tính KR-20 (câu 0/1) hoặc Cronbach alpha và SEM cho từng biến thể có đủ học sinh theo `exam_reliability_warn`, so điểm trung bình giữa biến thể, gắn cờ câu có vấn đề; reliability dưới ngưỡng PHẢI hiển thị cảnh báo và không tự sửa điểm | FR-EXM-010 · [EXAM_ENGINE §8.1](../../docs/02-learning/EXAM_ENGINE.md) | — |
| F044-AC-24 | KHI người có `lms.report.read` trong phạm vi gọi `getExamSessionReport`, HỆ THỐNG PHẢI trả phổ điểm, trung vị, tỉ lệ đạt, p-value theo câu của lớp/trường (loại `rapid_guess`), lựa chọn sai phổ biến theo `misconception_tag`, thời gian trung bình, chỉ số độ tin cậy | US-LMS-007 · FR-EXM-010 · [SCHOOL_LMS §8.6](../../docs/04-admin-portal/SCHOOL_LMS.md) (AC-LMS-017) | TC-EXM-004 |
| F044-AC-25 | NẾU session thuộc trường/tenant khác, THÌ HỆ THỐNG PHẢI trả `404 NOT_FOUND`; NẾU người gọi là học sinh hoặc thiếu quyền, THÌ PHẢI trả `403 PERMISSION_DENIED`; báo cáo PHẢI không chứa đáp án của từng học sinh theo tên ngoài phạm vi | AC-ORG-001 · AGENTS R6, R16 | TC-TEN-001 |

### Edge Cases

- NẾU job export thất bại giữa chừng, THÌ trạng thái `failed` có `error` (không PII), file dở không có link; người dùng tạo lại được (F044-AC-01).
- NẾU re-evaluate đổi điểm sau khi xuất, THÌ bảng kết quả mới phản ánh điểm hiệu lực; file cũ giữ nguyên tới hết hạn (F044-AC-08).
- NẾU câu có nhiều version, THÌ thống kê tách theo version, đề xuất hiệu chỉnh dựa trên version hiện hành (F044-AC-17, F044-AC-19).
- NẾU câu chưa đủ lượt, THÌ không có thống kê và bộ cân bằng dùng `p̂` suy từ độ khó (F044-AC-22).
- NẾU biến thể có < số học sinh tối thiểu, THÌ không tính độ tin cậy, hiển thị "chưa đủ dữ liệu" (F044-AC-23).
- NẾU trường không có lịch LMS, THÌ phạm vi LO của kỳ theo thứ tự curriculum (ASSESSMENT_REGULATION_PROFILE §4).
- NẾU học sinh bị soft-delete, THÌ không xuất hiện trong báo cáo mới.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-ANL-014 | Export bất đồng bộ `/lms/reports`, `/tenant/reports`, `/admin/reports`, link có hạn, audit | [DASHBOARD_DATA §10](../../docs/07-backend/DASHBOARD_DATA.md) |
| FR-ANL-015 | Báo cáo tham khảo TT27/TT22 | [ASSESSMENT_REGULATION_PROFILE](../../docs/02-learning/ASSESSMENT_REGULATION_PROFILE.md) |
| FR-LMS-013 | Bảng kết quả course tự chấm, xuất XLSX/CSV có watermark | [SCHOOL_LMS §10](../../docs/04-admin-portal/SCHOOL_LMS.md) |
| FR-QB-010 | Item analysis hằng đêm | [QUESTION_BANK §9](../../docs/02-learning/QUESTION_BANK.md) |
| FR-QB-014 | Đề xuất hiệu chỉnh độ khó, người duyệt chấp nhận tạo version mới | [QUESTION_BANK §9.1](../../docs/02-learning/QUESTION_BANK.md) |
| FR-EXM-010 | Cân bằng biến thể, độ tin cậy KR-20/alpha, SEM | [EXAM_ENGINE §4.1, §8.1](../../docs/02-learning/EXAM_ENGINE.md) |

### Key Entities

- **ReportJob**: yêu cầu xuất (loại, định dạng, phạm vi, người tạo, trạng thái, số dòng, khóa file, hạn link).
- **CourseResultTable**: học sinh × bài graded với điểm tự chấm hiệu lực (dẫn xuất, không lưu riêng).
- **AssessmentProfile**: hồ sơ ánh xạ mức tham khảo có version, phạm vi khối/môn, kỳ, disclaimer.
- **ReferenceLevel**: mức tham khảo học sinh × môn × kỳ theo version hồ sơ, độ phủ, tỉ lệ "làm quá nhanh".
- **QuestionStats**: thống kê chất lượng theo question version.
- **DifficultyCalibrationProposal**: đề xuất đổi độ khó (độ khó hiện tại/đề xuất, dữ liệu, trạng thái, người quyết định).
- **ExamQualityReport**: chỉ số độ tin cậy, SEM, chênh biến thể, câu có vấn đề, phân tích đề của một exam session.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `dashboard_export_max_rows`, `dashboard_export_max_concurrent_per_user` | Giới hạn export |
| `dashboard_export_link_ttl_minutes`, `report_link_ttl` | Hạn link tải |
| `dashboard_export_file_retention_days` | Thời gian giữ file |
| `dashboard_max_range_days`, `analytics_min_cell_size` | Khoảng ngày, ẩn ô nhỏ |
| `score_scale` | Thang điểm bảng kết quả |
| `assessment_reference_thresholds` | Ngưỡng mức tham khảo |
| `difficulty_calibration` | Ngưỡng lượt + bảng `p_value` → độ khó |
| `variant_balance_max_mean_p_diff` | Chênh tối đa giữa biến thể |
| `exam_reliability_warn` | Ngưỡng cảnh báo độ tin cậy, số học sinh tối thiểu |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Export luôn bất đồng bộ, đọc replica theo lô, không làm chậm dashboard | NFR-PERF-018 · ADR-0062 |
| Không sửa version đã publish; hiệu chỉnh = version mới | AGENTS R11, R31 · ADR-0043 |
| Chỉ dữ liệu trong quyền, không xuất chat, audit mọi export | AGENTS R7, R14 · ADR-0027 |
| File chỉ trên object storage, khóa bất biến | AGENTS R31 · ADR-0044 |
| Báo cáo chỉ tham khảo, không thay đánh giá chính thức | ADR-0072 |
| Mọi chuỗi có giới hạn độ dài | AGENTS R28 · ADR-0041 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Export 500.000 dòng hoàn tất bất đồng bộ, link hết hạn đúng hạn, có audit; 20 export đồng thời không làm chậm dashboard > 10%.
- **SC-002**: 100% file xuất có watermark; 100% báo cáo tham khảo có disclaimer; 0 dòng ngoài phạm vi quyền trong bộ kiểm thử.
- **SC-003**: Thống kê item analysis khớp tính tay trên bộ dữ liệu mẫu; 0 version đã publish bị thay đổi bởi job thống kê/hiệu chỉnh.
- **SC-004**: Đề 4 biến thể có `balance.enabled` đạt chênh `expected_p` ≤ ngưỡng hoặc có cảnh báo kèm số liệu trong 100% lần sinh.
- **SC-005**: Kịch bản quickstart chạy hết với tài khoản demo, mọi `F044-AC-*` có test pass.

## Ngoài phạm vi

- Giao diện báo cáo, dashboard và màn hình thống kê trên portal — F054 (CMS), F056 (School Learning Dashboard).
- Rollup và API dashboard — F043; sinh đề, chấm, regrade — F042; workflow duyệt/publish version — F025, F026.
- Export Game Analytics — F080; đồng bộ ngược điểm sang SIS — ngoài Release (OQ-01).
- Học bạ, sổ điểm chính thức, xét lên lớp — ngoài phạm vi ASSESSMENT_REGULATION_PROFILE §1; kỹ năng viết/nói — deferred (ADR-0082).

## Assumptions

- Nội dung và tên mức TT27/TT22 cần pháp chế/EdTech xác nhận trước pilot (ASSESSMENT_REGULATION_PROFILE §6); ngưỡng mặc định [Confirmed], phần còn lại [Confirmed].
- Ngưỡng ≥ 30 lượt và chu kỳ hằng đêm/hằng tuần của item analysis theo QUESTION_BANK §9 [Confirmed]; nên đăng ký vào CONFIG_DEFAULTS (ghi trong báo cáo).
- PDF dùng thư viện render phía server trong worker [Confirmed]; bucket `ew-exports` theo ADR-0044.
- Trọng số `exam_type` cho điểm tham khảo THCS do tenant cấu hình (ASSESSMENT_REGULATION_PROFILE §4.2); mặc định trung bình đều [Confirmed] — bản này dùng trung bình đều (chưa có cấu hình tenant).
- Tham số (CONFIG_DEFAULTS) đọc từ giá trị mặc định trong code (`reporting.DefaultConfig`, `command.DefaultQualityConfig`, `regulation.DefaultRules`) vì chưa có bộ đọc platform settings — như F043.
- PDF dùng `github.com/go-pdf/fpdf` v0.9.0 với font Be Vietnam Pro (OFL) nhúng từ `design/fonts` [Confirmed thư viện server-side].
