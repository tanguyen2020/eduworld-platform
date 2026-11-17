# Feature Specification: F035 — Nghiên cứu hiệu quả học tập

**Feature Branch**: `035-learning-efficacy-study` · **Created**: 2026-11-17 · **Status**: Ready · **Module**: module `learning` + `web/apps/portal` · **Mốc**: M-C3 · **Phụ thuộc**: F031, F032, F042
**Input**: Đo hiệu quả học tập bằng bài kiểm tra độc lập trước/sau có lớp so sánh, kiểm tra mastery có dự báo đúng năng lực thật, hạ tầng A/B cho cấu hình mastery và luật gợi ý, xuất dữ liệu giả danh có kiểm soát; là exit criterion của pilot ([ADR-0080](../../docs/adr/ADR-0080-do-hieu-qua-hoc-tap-la-exit-criterion-pilot.md)). Thiết kế nghiên cứu do [LEARNING_EFFICACY_PLAN](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) sở hữu. PRD **chưa có FR riêng** cho nghiên cứu hiệu quả: feature dùng FR gần nhất (mục Functional Requirements) và ghi Câu hỏi mở.

## Clarifications

### Session 2026-11-17

- Q: ADR-0080 đã là quyết định hiện hành? → A: Có, Accepted ngày 2026-11-17 (file ADR và [chỉ mục ADR](../../docs/adr/README.md) đã cập nhật). PRD cấp FR-ANL-017…023 cho feature này.
- Q: Bài kiểm tra trước/sau chạy bằng gì? → A: ExamSession của F042 (EXAM_ENGINE) với cài đặt cố định, do System Admin lập lịch theo lớp; evidence **không** vào mastery ([LEARNING_EFFICACY_PLAN §3.2](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md)).
- Q: Ai vận hành nghiên cứu khi chưa có role EdTech? → A: `system_admin` (role platform duy nhất đọc liên tenant, MFA, gateway nội bộ) qua permission mới `platform.efficacy_study.manage`, `platform.efficacy_study.export`, `platform.learning_experiment.manage` [Proposed]; nhóm `platform.*` tự động cấm cho role tenant (R20).
- Q: "Rút khỏi nghiên cứu theo yêu cầu trường" do ai thao tác? → A: Trường gửi yêu cầu ngoài hệ thống; System Admin thực hiện với tenant đích và ghi mã yêu cầu ([LEARNING_EFFICACY_PLAN §8](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md)). Mô hình consent ngoài version này (ADR-0082).
- Q: Loại A/B nào trong F035? → A: `mastery_model` và `recommendation_rule`; `selection_rule` ngoài phạm vi. Nhánh thử nghiệm chỉ đổi trải nghiệm phía học sinh (tiến độ hiển thị, gợi ý, độ khó luyện tập); báo cáo nhà trường và XP vẫn theo mô hình `active` (rào chắn §6).
- Q: Hồi quy đa mức chạy trong hệ thống? → A: Không. Báo cáo trong hệ thống tính thống kê mô tả, Hedges' g có hiệu chỉnh cụm (design effect) và predictive validity; mô hình đa mức do chuyên gia thống kê chạy trên bộ dữ liệu giả danh xuất ra.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Câu và đề đo hiệu quả độc lập với luyện tập (Priority: P1)

`cm.noidung` soạn câu và hai dạng đề A/B có cờ `efficacy_instrument`, người thứ hai duyệt; các câu này không bao giờ xuất hiện ở luyện tập, quest, minigame, trận đấu, gợi ý hay đề thường.

**Why this priority**: Không có công cụ đo độc lập thì mọi kết quả nghiên cứu là vòng tròn (đo bằng chính mastery).

**Independent Test**: Publish 30 câu `efficacy_instrument` cùng LO với câu luyện tập → 1.000 lần `startAdHocAttempt`, `createQuestionPlan`, sinh đề blueprint thường không lần nào chọn trúng; `createExamSession` với blueprint dạng A → `422`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F035-AC-01 | KHI người có `learning_content.write` phạm vi platform lưu câu hỏi hoặc blueprint với `efficacy_instrument = true`, module learning PHẢI chấp nhận chỉ khi nội dung là `owner_scope = platform`, câu có `exam_only = true` và không tham số hóa, blueprint có `variants.strategy = fixed` và mọi rule dùng `fixed_question_version_ids` trỏ câu `efficacy_instrument`; ngược lại PHẢI trả `422 VALIDATION_FAILED`; cờ chỉ đổi bằng version mới. | FR-QB-004 · [LEARNING_EFFICACY_PLAN §3.2](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) · R11 | — |
| F035-AC-02 | HỆ THỐNG PHẢI loại mọi version `efficacy_instrument` khỏi mọi lần chọn câu (practice, `retry_wrong`, `diagnostic`, question set động, `createQuestionPlan`, sinh đề blueprint thường kể cả `prefer_exam_only`/`include_exam_only`, gợi ý ôn tập). | AC-EFF-001 · FR-QB-004 · [QUESTION_BANK §8](../../docs/02-learning/QUESTION_BANK.md) | — |
| F035-AC-03 | NẾU question set tĩnh, lesson, activity hoặc blueprint không phải `efficacy_instrument` tham chiếu câu `efficacy_instrument`, THÌ module learning PHẢI trả `422 VALIDATION_FAILED` khi lưu. | AC-EFF-001 | — |
| F035-AC-04 | NẾU `createExamSession` dùng blueprint `efficacy_instrument` hoặc `applyTenantAssignments` gán tài nguyên `efficacy_instrument` cho tenant, THÌ module learning PHẢI từ chối `422 VALIDATION_FAILED`; ExamSession đo hiệu quả chỉ do F035 tạo. | [LEARNING_EFFICACY_PLAN §3.2](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) · ADR-0029 | TC-TEN-019 |
| F035-AC-05 | HỆ THỐNG PHẢI không đưa snapshot câu/đề `efficacy_instrument` lên CDN công khai và không trả `answer_key`, `scoring_rule`, `explanation` trong render payload. | R6 · R31 · ADR-0044 | TC-CNT-006, TC-LRN-007 |

---

### User Story 2 — Thỏa thuận, nghiên cứu và lớp tham gia/so sánh (Priority: P1)

`sa.admin` ghi thỏa thuận pilot của `th-demo-a` và `pgd-demo`, tạo nghiên cứu Toán lớp 3 dùng dạng A/B, thêm 3A (can thiệp) – 3B (so sánh) thành một cặp, kích hoạt.

**Why this priority**: Thiết kế bán thực nghiệm chia theo lớp là nền của mọi phân tích ([LEARNING_EFFICACY_PLAN §3.1](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md)).

**Independent Test**: Tạo study 2 cặp lớp, một cặp để trống `arm` → `activateEfficacyStudy` gán ngẫu nhiên tái lập được, cặp 1 `a_then_b`, cặp 2 `b_then_a`; sửa arm sau kích hoạt → `409`; `ad.truong.a` gọi API → `403`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F035-AC-06 | KHI System Admin gọi `createResearchAgreement` với `X-Tenant-Id` tenant đích, `agreement_ref` (≤ 128) và `covers` ⊂ {`efficacy_study`, `experiment`}, module learning PHẢI lưu thỏa thuận `active` và ghi audit `research_agreement.created`; thiếu header PHẢI trả `400 TENANT_HEADER_REQUIRED`. | [LEARNING_EFFICACY_PLAN §8](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) · AC-ORG-004 | TC-TEN-011c |
| F035-AC-07 | KHI `createEfficacyStudy` hợp lệ (`code` ≤ 64 duy nhất, `title` ≤ 255, môn, khối, 1–100 `objective_ids`, hai blueprint version `efficacy_instrument` đã publish khác nhau cùng môn/khối phủ đúng tập LO với số câu tối thiểu mỗi LO mỗi dạng theo LEARNING_EFFICACY_PLAN §3.2, `min_intervention_weeks` ≥ 1), module learning PHẢI tạo study `draft`, idempotent theo `Idempotency-Key`, ghi audit `efficacy_study.created`; vi phạm PHẢI trả `422 VALIDATION_FAILED` chỉ rõ trường. | [LEARNING_EFFICACY_PLAN §3.2](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) · R10 | TC-TEN-024 |
| F035-AC-08 | KHI `addEfficacyStudyClass` (tenant đích) thêm lớp với `pair_no` và `arm` tùy chọn ∈ {`intervention`, `comparison`, `validation`}, module learning PHẢI kiểm lớp thuộc tenant đích (khác → `404 NOT_FOUND`), đúng khối, tenant có thỏa thuận `active` phủ `efficacy_study` (thiếu → `422 VALIDATION_FAILED`), lớp chưa ở study `draft`/`active` khác (→ `409 CONFLICT`), study còn `draft` (→ `409 INVALID_STATE_TRANSITION`). | [LEARNING_EFFICACY_PLAN §3.1](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) | TC-TEN-001 |
| F035-AC-09 | KHI `activateEfficacyStudy`, module learning PHẢI kiểm mỗi `pair_no` có đúng 2 lớp, gán arm còn trống bằng hash ổn định `(study_id, pair_no)` (`assignment_method = random`), bảo đảm mỗi cặp một `intervention` một `comparison`, đặt `form_order` xen kẽ theo cặp (lẻ `a_then_b`, chẵn `b_then_a`) nếu chưa đặt, chuyển `active`, ghi audit; sau đó mọi thay đổi arm/cặp/dạng đề PHẢI trả `409 INVALID_STATE_TRANSITION`. | [LEARNING_EFFICACY_PLAN §3.1, §3.2](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) | — |
| F035-AC-10 | HỆ THỐNG PHẢI chỉ cho principal có `platform.efficacy_study.manage` [Proposed] qua gateway nội bộ (`azp = eduworld-portal`, MFA) gọi `/admin/learning/research-agreements*` và `/admin/learning/efficacy-studies*`; role tenant, `content_admin`, `safety_moderator` PHẢI nhận `403 PERMISSION_DENIED`; token thiếu MFA PHẢI nhận `403 MFA_REQUIRED`. | R7 · R20 · ADR-0027 | TC-TEN-004 |
| F035-AC-11 | KHI System Admin liệt kê thỏa thuận, study, lớp, module learning PHẢI dùng `CrossTenantScope` theo `tenant_ids[]`, trả `tenant_id` mỗi dòng và ghi audit `cross_tenant.accessed`. | R8 · AC-ORG-004 | TC-TEN-018 |

---

### User Story 3 — Đợt kiểm tra trước/sau không ảnh hưởng học tập (Priority: P1)

`sa.admin` lập lịch đợt `pre` tuần 0 và `post` sau ≥ `min_intervention_weeks`; mỗi lớp nhận một ExamSession đúng dạng đề; học sinh làm trong Exam Zone, không thấy điểm, không được XP, mastery không đổi.

**Why this priority**: Dữ liệu trước/sau là nguồn của effect size và predictive validity.

**Independent Test**: Lập đợt `pre` cho 3A (`a_then_b`) và 3B → 2 ExamSession `feedback_timing = never`, `result_release = manual`, dạng A; `hs.an.3a` làm bài → mastery, XP, `progress_daily` không đổi, không có `exam.graded` trong outbox, `getExamResult` → `403 EXAM_RESULTS_NOT_RELEASED`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F035-AC-12 | KHI `scheduleEfficacyWave` với `kind` ∈ {`validation`, `pre`, `post`} và cửa sổ hợp lệ, module learning PHẢI tạo cho mỗi lớp đích một ExamSession trong tenant của lớp với `exam_type = custom`, `is_official = false`, `feedback_timing = never`, `result_release = manual`, ẩn đáp án/giải thích, `max_attempts = 1`, `efficacy_wave_id`, dạng đề theo `form_order` (`pre` dạng đầu, `post` dạng sau; `validation` theo `form` của request), idempotent, ghi audit `efficacy_wave.scheduled`. | AC-EFF-002 · [LEARNING_EFFICACY_PLAN §3.2](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) · FR-EXM-004 | TC-EXM-005 |
| F035-AC-13 | NẾU study đã có đợt cùng `kind` (`pre`/`post`) chưa hủy, đợt `post` mở sớm hơn `window_end` của `pre` cộng `min_intervention_weeks`, hoặc đợt `pre`/`post` trên study chưa `active`, THÌ module learning PHẢI từ chối (`409 INVALID_STATE_TRANSITION` hoặc `422 VALIDATION_FAILED`). | [LEARNING_EFFICACY_PLAN §3.1](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) | — |
| F035-AC-14 | KHI đợt `pre` mở, module learning PHẢI chụp danh sách học sinh đang ghi danh của mỗi lớp thành participant (arm, `form_order` của lớp), loại học sinh `processing_restricted` hoặc đã ẩn danh; ExamSession đợt sau chỉ nhắm participant `active` của lớp. | [LEARNING_EFFICACY_PLAN §3.1, §8](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) · NFR-PRIV-002 | — |
| F035-AC-15 | KHI attempt của ExamSession có `efficacy_wave_id` được chấm, module learning PHẢI không tạo evidence mastery, không cập nhật `progress_daily`, không đăng ký gợi ý R5, không phát `exam.*` ra outbox (nên không XP, không thông báo, không vào dashboard), và ghi learning event với `context.efficacy_wave_id`; recompute mastery PHẢI bỏ qua các event này. | AC-EFF-002 · FR-ANL-001 · [EXAM_ENGINE §10](../../docs/02-learning/EXAM_ENGINE.md) | TC-LRN-010 |
| F035-AC-16 | TRONG KHI ExamSession có `efficacy_wave_id`, module learning PHẢI trả `403 EXAM_RESULTS_NOT_RELEASED` cho `getExamResult`/`getExamAttemptAnalysis` của học sinh, `409 INVALID_STATE_TRANSITION` cho `releaseExamResults`, và `403 PERMISSION_DENIED` cho `listExamSessionPapers`, `getExamSessionReport`, `createRegradeRequest` (`scope = exam_session`) của role tenant. | AC-EFF-002 · [LEARNING_EFFICACY_PLAN §8](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) | TC-EXM-005 |
| F035-AC-17 | HỆ THỐNG PHẢI không trả `efficacy_wave_id`, study, arm hay `form_order` trong response cho học sinh; nhân sự trường thấy ExamSession đo hiệu quả ở `listExamSessions`/`getExamSession` với `is_efficacy = true` chỉ đọc; ExamSession của tenant khác PHẢI trả `404 NOT_FOUND`. | [LEARNING_EFFICACY_PLAN §6](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) · R8 | TC-TEN-001 |

---

### User Story 4 — Rút khỏi nghiên cứu theo yêu cầu trường (Priority: P2)

Trường yêu cầu loại 2 học sinh 3A và sau đó rút cả trường; dữ liệu của họ bị loại khỏi mọi phân tích từ thời điểm rút, học sinh vẫn học bình thường.

**Why this priority**: Điều kiện đạo đức của nghiên cứu; cần trước khi chạy đợt `post`.

**Independent Test**: Rút `hs.binh.3a` → participant `withdrawn`, snapshot mastery của em bị soft-delete, báo cáo và bộ dữ liệu xuất sau đó không có em; `revokeResearchAgreement` của `th-demo-a` → đợt chưa mở của 3A bị hủy.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F035-AC-18 | KHI System Admin gọi `withdrawEfficacyParticipants` (tenant đích, `scope` ∈ {`tenant`, `school`, `class`, `students`}, ≤ 500 `student_ids`, `reason` 10–1.000 ký tự, `school_request_ref` ≤ 128, `Idempotency-Key`), module learning PHẢI chuyển participant phạm vi đó sang `withdrawn` với `withdrawn_at`, loại họ khỏi đối tượng ExamSession đo hiệu quả chưa mở, hủy ExamSession chưa mở của lớp bị rút toàn bộ, ghi audit `efficacy.participants_withdrawn`; gửi lại cùng key PHẢI trả kết quả cũ. | [LEARNING_EFFICACY_PLAN §8](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) · R10 | — |
| F035-AC-19 | KHI participant bị rút hoặc bị ẩn danh (`anonymizeUserData`), module learning PHẢI loại họ khỏi mọi báo cáo và bộ dữ liệu xuất tạo sau thời điểm đó và soft-delete dữ liệu dẫn xuất của nghiên cứu (snapshot mastery) với lý do; learning record gốc giữ nguyên; không xóa vật lý. | [LEARNING_EFFICACY_PLAN §8](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) · R21 · ADR-0070 | TC-TEN-022 |
| F035-AC-20 | KHI `revokeResearchAgreement`, module learning PHẢI rút toàn bộ participant của tenant khỏi mọi study đang chạy (như F035-AC-18) và đưa học sinh của tenant ra khỏi mọi thử nghiệm A/B (trải nghiệm trở về nhánh control từ request kế tiếp), ghi audit. | [LEARNING_EFFICACY_PLAN §6, §8](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) | — |

---

### User Story 5 — Thử nghiệm A/B cấu hình mastery và luật gợi ý (Priority: P2)

`sa.admin` tạo thử nghiệm `mastery-we-1-1` (control `we-1.0`, nhánh `we-1.1`, đơn vị lớp), chạy shadow 2 tuần rồi bật live; hệ thống tự dừng nếu nhánh làm giảm độ chính xác.

**Why this priority**: Cơ sở cải tiến mô hình mastery (Q4, ADR-0017), không chặn đo hiệu quả chính.

**Independent Test**: Thử nghiệm 50/50 trên 4 lớp → `hs.an.3a` và `hs.binh.3a` cùng arm; shadow không đổi `getMyMastery`; live đổi mức hiển thị cho arm thử nhưng `getStudentProgress` của nhà trường không đổi; event mang `experiment_arms`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F035-AC-21 | KHI `createLearningExperiment` hợp lệ (`code` ≤ 64 duy nhất, `kind` ∈ {`mastery_model`, `recommendation_rule`}, `unit` ∈ {`class`, `student`} mặc định `class`, 2–5 arm đúng một `is_control`, tổng `allocation_pct` = 100, arm mastery trỏ `model_version` có trong `mastery_model_configs` (control = `active`), arm luật gợi ý chỉ chứa tham số trong danh mục cho phép, mọi tenant có thỏa thuận phủ `experiment`, không chứa lớp `comparison`/`validation` của study đang chạy), module learning PHẢI tạo `draft` có `revision`, ghi audit; ngược lại `422 VALIDATION_FAILED`; sửa ngoài `draft` PHẢI trả `409 INVALID_STATE_TRANSITION`. | [LEARNING_EFFICACY_PLAN §6](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) · FR-PRG-003 | — |
| F035-AC-22 | HỆ THỐNG PHẢI gán arm bằng `hash(experiment_id, unit_id) mod 100` theo `allocation_pct` cộng dồn, lưu ở lần tiếp xúc đầu và giữ nguyên khi học sinh chuyển lớp; với `unit = class` hai học sinh cùng lớp PHẢI luôn cùng arm. | AC-EFF-004 | — |
| F035-AC-23 | TRONG KHI thử nghiệm `mastery_model` ở `shadow`, consumer mastery PHẢI tính song song mô hình của mọi arm không phải control cho học sinh thuộc đơn vị tham gia từ cùng evidence, không đổi bất kỳ giá trị hiển thị hay gợi ý nào. | FR-PRG-009 · [LEARNING_EFFICACY_PLAN §6](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) | TC-LRN-010 |
| F035-AC-24 | TRONG KHI thử nghiệm ở `live`, module learning PHẢI dùng trạng thái/tham số của arm cho trải nghiệm học sinh thuộc arm (`getMyProgress`, `getMyMastery`, `getMyRecommendations`, độ khó luyện tập) và PHẢI giữ mô hình `active` cho `getStudentProgress`, dashboard/báo cáo nhà trường, mức tham khảo và `learning_objective.mastered`; thử nghiệm PHẢI không tác động attempt `mode = exam`. | [LEARNING_EFFICACY_PLAN §6](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) · ADR-0017 · R19 | — |
| F035-AC-25 | TRONG KHI học sinh thuộc thử nghiệm đang `shadow`/`live`, learning event của học sinh PHẢI mang `context.experiment_arms` (`<code>:<arm>`); không response nào cho học sinh chứa arm. | AC-EFF-004 · FR-ANL-001 | — |
| F035-AC-26 | KHI job rào chắn hằng ngày thấy chỉ số chính của một arm thấp hơn control từ ngưỡng điểm phần trăm của LEARNING_EFFICACY_PLAN §6 trở lên, module learning PHẢI chuyển thử nghiệm sang `stopped` (`stop_reason = guardrail_breach`), trả học sinh về control, phát alert và audit; `stopLearningExperiment`/hết `end_at` PHẢI làm tương tự với lý do tương ứng mà không phát lại `learning_objective.mastered`. | [LEARNING_EFFICACY_PLAN §6](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) · R19 | — |
| F035-AC-27 | KHI gọi `getLearningExperimentResults`, module learning PHẢI trả theo arm: số đơn vị/học sinh, chỉ số chính, log-loss dự đoán câu kế tiếp, thời gian học, với ô < `analytics_min_cell_size` bị ẩn; chỉ principal có `platform.learning_experiment.manage` [Proposed] (MFA) được gọi API thử nghiệm, khác → `403 PERMISSION_DENIED`. | FR-ANL-016 · R20 | TC-TEN-004, TC-ANL-005 |

---

### User Story 6 — Báo cáo hiệu quả và độ tin cậy dự báo của mastery (Priority: P2)

Sau đợt `post`, `sa.admin` tạo báo cáo: effect size kèm khoảng tin cậy, gain theo LO, tỉ lệ rời nghiên cứu, độ tin cậy đề, đối chiếu mastery với bài kiểm tra độc lập.

**Why this priority**: Là bằng chứng của exit criterion pilot (ADR-0080 mục 1, 3).

**Independent Test**: Dataset golden 2 nhóm × 4 lớp → `createEfficacyStudyReport` trả Hedges' g, CI, ρ, AUC khớp giá trị tính tay (sai số ≤ 0,001); ô < 5 học sinh bị ẩn.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F035-AC-28 | KHI đợt `post` mở, module learning PHẢI chụp `M_eff` và level (mô hình `active`, sau decay) của mỗi participant `intervention` cho từng LO của study. | [LEARNING_EFFICACY_PLAN §5](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) · FR-PRG-003 | — |
| F035-AC-29 | KHI `createEfficacyStudyReport` (`Idempotency-Key`, tùy chọn `excluded_question_version_ids`), job PHẢI tính theo arm: số participant, tỉ lệ rời (thiếu pre/post, rút), gain, normalized gain, Hedges' g của chênh lệch gain với CI 95% hiệu chỉnh design effect, gain theo LO, KR-20 từng dạng đề từng đợt (cảnh báo theo `exam_reliability_warn`), mức sử dụng (thời gian học, số ngày học, tỉ lệ `rapid_guess`) và tỉ lệ lớp so sánh có dùng luyện tập; cùng đầu vào PHẢI cho cùng kết quả. | AC-EFF-005 · FR-EXM-010 · [LEARNING_EFFICACY_PLAN §4](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) | — |
| F035-AC-30 | Báo cáo PHẢI có predictive validity cho nhóm can thiệp: Spearman ρ theo LO giữa `M_eff` và tỉ lệ đúng bài sau, tỉ lệ hiệu chuẩn (`mastered` ≥ 80%, `beginning` < 50%), AUC dự đoán "đạt ≥ 2/3 câu của LO", tỉ lệ làm theo gợi ý và gain LO được gợi ý so với LO không được gợi ý; LO dưới ngưỡng tham chiếu PHẢI được đánh dấu. | AC-EFF-005 · [LEARNING_EFFICACY_PLAN §5](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) | — |
| F035-AC-31 | Báo cáo PHẢI chỉ chứa số liệu gộp, ẩn mọi ô < `analytics_min_cell_size` sao cho tổng không suy ra được ô ẩn, không có tên/lớp thật, không xếp hạng, và gắn nhãn `descriptive_only` khi số lớp mỗi nhóm dưới khuyến nghị cỡ mẫu của LEARNING_EFFICACY_PLAN §4. | AC-EFF-003 · FR-ANL-016 · R16 | TC-DASH-009 |

---

### User Story 7 — Xuất dữ liệu định danh giả có kiểm soát (Priority: P2)

`sa.admin` xuất bộ dữ liệu CSV giả danh cho chuyên gia thống kê chạy mô hình đa mức.

**Why this priority**: Phân tích chính thức và phản biện chạy ngoài hệ thống.

**Independent Test**: `createPlatformReport` `report_type = efficacy_dataset` → CSV chỉ có mã giả danh; xuất lại cho study khác → mã khác; role không có quyền → `403`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F035-AC-32 | KHI principal có `report.export` và `platform.efficacy_study.export` [Proposed] gọi `createPlatformReport` với `report_type = efficacy_dataset`, `params.study_id`, worker PHẢI xuất CSV mỗi dòng một participant `active` gồm `participant_code` = HMAC-SHA256 theo khóa bí mật và `study_id`, `class_code`, `site_code`, `pair_no`, `arm`, `form_order`, khối, đúng/tổng theo đợt × LO, `M_eff`/level theo LO, mức sử dụng; PHẢI không có `student_id`, tên, mã/ tên lớp, trường, tenant, ngày sinh. | AC-EFF-003 · FR-ANL-014 · NFR-PRIV-002 | — |
| F035-AC-33 | `participant_code` PHẢI ổn định trong một study và khác giữa các study; khóa PHẢI chỉ đọc từ secret cấu hình, không log, không trả về; NẾU thiếu khóa, THÌ job PHẢI `failed` mà không xuất. | R3 · R14 · [DATA_PRIVACY §4](../../docs/10-security/DATA_PRIVACY.md) | — |
| F035-AC-34 | Export đo hiệu quả PHẢI ghi audit `report.exported` và `efficacy.dataset_exported` (study, số dòng, không PII), link hết hạn theo `dashboard_export_link_ttl_minutes`, file giữ theo `dashboard_export_file_retention_days`, giới hạn đồng thời theo `dashboard_export_max_concurrent_per_user`; thiếu permission PHẢI trả `403 PERMISSION_DENIED`. | FR-ANL-014 · FR-AUD-001 | TC-DASH-008, TC-TEN-004 |

---

### User Story 8 — Màn quản trị nghiên cứu trên Portal (Priority: P3)

System Admin thao tác toàn bộ luồng trên workspace D; Content Admin đặt cờ `efficacy_instrument` trong Content Studio.

**Why this priority**: Vận hành được bằng API (quickstart); UI giảm lỗi thao tác.

**Independent Test**: `sa.admin` tạo study → thêm lớp → kích hoạt → lập đợt → xem báo cáo trên portal; `ad.truong.a` không thấy menu; ô cờ không hiện ở phạm vi tenant.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F035-AC-35 | Portal PHẢI hiện mục "Nghiên cứu hiệu quả" (thỏa thuận, study, lớp/cặp, đợt, rút, báo cáo, xuất, thử nghiệm A/B) trong workspace D chỉ khi `GET /me/permissions` có permission tương ứng, chỉ gọi API (không tính thống kê ở client), style từ token. | R1 · R32 · [ADMIN_PORTAL §2.1](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-UI-003, TC-UI-006 |
| F035-AC-36 | Content Studio PHẢI hiện ô `efficacy_instrument` và nhãn "Đo hiệu quả" chỉ ở phạm vi platform cho người có `learning_content.write` platform, không bao giờ ở phạm vi tenant. | [ADMIN_PORTAL §3.13](../../docs/04-admin-portal/ADMIN_PORTAL.md) · R20 | — |
| F035-AC-37 | KHI menu/action của feature đang tắt cho phạm vi phiên (`disabled_menus`/`disabled_actions` của `getMyPermissions`) — khóa: `action.content_studio.question_bank.view`, `action.content_studio.question_bank.edit`, `action.content_studio.assessments.view`, `action.content_studio.assessments.edit`, `action.platform_admin.reports.export` và menu quản trị riêng của feature (chờ bổ sung vào catalog, xem plan.md), Portal PHẢI ẩn menu và ẩn/khóa nút tương ứng; NẾU API trả `403 FEATURE_DISABLED`, Portal PHẢI hiện thông báo tính năng tạm tắt và tải lại quyền | FR-CMS-018 · ADR-0090 | TC-CMS-MENU-02 |

### Edge Cases

- Học sinh vào lớp sau khi đợt `pre` mở: không là participant, không trong đối tượng đợt sau (F035-AC-14).
- Học sinh chuyển lớp giữa nghiên cứu: giữ arm và `form_order` lúc chụp; nếu rời mọi lớp tham gia vẫn được mời đợt `post` qua danh sách participant (F035-AC-14).
- Học sinh vắng một đợt: tính vào tỉ lệ rời, không ước lượng thay (F035-AC-29).
- Lớp so sánh dùng luyện tập trong thời gian can thiệp: hệ thống không chặn, chỉ báo cáo tỉ lệ (F035-AC-29).
- Câu đề đo hiệu quả bị lỗi sau khi làm: loại câu bằng `excluded_question_version_ids` khi tạo báo cáo; không regrade ExamSession đo hiệu quả (F035-AC-16, F035-AC-29).
- Học sinh có attempt ở cả hai dạng đề cùng đợt (do tạo lại đợt): chỉ attempt của ExamSession đợt chưa hủy được tính.
- Thử nghiệm và nghiên cứu cùng lúc: lớp can thiệp được tham gia thử nghiệm; lớp so sánh/validation thì không (F035-AC-21).
- Tenant QA (`is_qa = true`) không được thêm vào study/thử nghiệm (`422 VALIDATION_FAILED`).

## Requirements *(bắt buộc)*

### Functional Requirements

FR chính là FR-ANL-017…023 (PRD cấp ngày 2026-11-17); các dòng sau là FR liên quan mà feature đóng góp.

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-ANL-017 | US1: ngân hàng `efficacy_instrument`, loại khỏi mọi lần chọn câu | [LEARNING_EFFICACY_PLAN](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) |
| FR-ANL-018 | US3: đợt pre/post/validation bằng ExamSession, không evidence, không XP, không công bố | [LEARNING_EFFICACY_PLAN](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) |
| FR-ANL-019 | US2: thỏa thuận tham gia của tenant | [LEARNING_EFFICACY_PLAN](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) |
| FR-ANL-020 | US5: thử nghiệm A/B mastery/gợi ý, `experiment_arms`, dừng sớm | [LEARNING_EFFICACY_PLAN](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) |
| FR-ANL-021 | US6: báo cáo effect size, tỉ lệ rời, độ tin cậy dự báo | [LEARNING_EFFICACY_PLAN](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) |
| FR-ANL-022 | US7: xuất dữ liệu định danh giả | [LEARNING_EFFICACY_PLAN](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) |
| FR-ANL-023 | US4: rút lớp/học sinh khỏi phân tích | [LEARNING_EFFICACY_PLAN](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) |
| FR-QB-004 | Loại câu `efficacy_instrument` khỏi mọi lần chọn ngẫu nhiên | [QUESTION_BANK §6, §8](../../docs/02-learning/QUESTION_BANK.md) · [LEARNING_EFFICACY_PLAN §3.2](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) |
| FR-EXM-004 | ExamSession đo hiệu quả: không chính thức, không công bố, không feedback | [EXAM_ENGINE](../../docs/02-learning/EXAM_ENGINE.md) · [LEARNING_EFFICACY_PLAN §3.2](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md) |
| FR-EXM-010 | KR-20 của dạng đề đo hiệu quả (kiểm định và từng đợt) | [EXAM_ENGINE §8.1](../../docs/02-learning/EXAM_ENGINE.md) |
| FR-PRG-003 | Arm thử nghiệm trỏ `model_version`; snapshot `M_eff` | [LEARNING_ANALYTICS §3.4](../../docs/02-learning/LEARNING_ANALYTICS.md) |
| FR-PRG-008 | Thử nghiệm luật gợi ý; tỉ lệ làm theo gợi ý | [LEARNING_ANALYTICS §6](../../docs/02-learning/LEARNING_ANALYTICS.md) |
| FR-PRG-009 | Tính shadow mô hình arm song song | [LEARNING_ANALYTICS §3.4](../../docs/02-learning/LEARNING_ANALYTICS.md) |
| FR-ANL-001 | Learning event mang `efficacy_wave_id`, `experiment_arms` | [EVENT_DESIGN §5](../../docs/07-backend/EVENT_DESIGN.md) |
| FR-ANL-004 | Dữ liệu nghiên cứu chỉ cho mục đích đánh giá hiệu quả, gộp/giả danh | [LEARNING_ANALYTICS §9](../../docs/02-learning/LEARNING_ANALYTICS.md) |
| FR-ANL-014 | Xuất bộ dữ liệu giả danh bất đồng bộ, link có hạn, audit | [DASHBOARD_DATA §10](../../docs/07-backend/DASHBOARD_DATA.md) |
| FR-ANL-016 | Ẩn ô nhỏ trong báo cáo hiệu quả và kết quả thử nghiệm | [LEARNING_ANALYTICS §5](../../docs/02-learning/LEARNING_ANALYTICS.md) |
| FR-AUD-001 | Audit mọi thao tác nghiên cứu, xuất dữ liệu, xem liên tenant | [ADMIN_PORTAL §7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| NFR-PRIV-002 | Tối thiểu hóa: chỉ chụp dữ liệu cần cho phân tích, mã giả danh | [DATA_PRIVACY](../../docs/10-security/DATA_PRIVACY.md) |
| FR-CMS-018 | Menu/action của feature theo danh mục `specs/portal/menu-catalog.yaml`, ẩn khi tắt, xử lý `403 FEATURE_DISABLED` | [ADMIN_PORTAL §2.7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |

### Key Entities

- **ResearchAgreement**: thỏa thuận pilot của một tenant (mã tham chiếu văn bản, phạm vi `efficacy_study`/`experiment`, trạng thái).
- **EfficacyStudy**: một nghiên cứu (môn, khối, tập LO, hai dạng đề, số tuần can thiệp tối thiểu, trạng thái `draft → active → completed | cancelled`).
- **EfficacyStudyClass**: lớp tham gia (cặp, arm, thứ tự dạng đề, cách gán). **EfficacyParticipant**: học sinh được chụp khi mở đợt `pre` (arm, trạng thái `active/withdrawn/excluded`).
- **EfficacyWave**: đợt `validation`/`pre`/`post` và các ExamSession theo lớp. **EfficacyWithdrawal**: yêu cầu rút (phạm vi, lý do, mã yêu cầu của trường).
- **EfficacyMasterySnapshot**: `M_eff`/level của participant × LO lúc mở đợt `post`. **EfficacyReport**: kết quả phân tích đã tính.
- **LearningExperiment**: thử nghiệm A/B (loại, đơn vị, arm, tenant, `shadow/live`), **ExperimentAssignment** (học sinh → arm), **ExperimentMasteryState** (mastery theo mô hình arm), **ExperimentResult**.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `mastery_model` | Mô hình `active` (control) và version cho arm |
| `analytics_min_cell_size` | Ẩn ô nhỏ trong báo cáo hiệu quả, kết quả thử nghiệm |
| `exam_reliability_warn` | Cảnh báo KR-20 dạng đề |
| `dashboard_export_link_ttl_minutes`, `dashboard_export_file_retention_days`, `dashboard_export_max_concurrent_per_user`, `dashboard_export_max_rows` | Kiểm soát export |

Chưa có tên trong CONFIG_DEFAULTS (giá trị ở [LEARNING_EFFICACY_PLAN](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md), task Phase 2 đăng ký): số câu tối thiểu mỗi LO mỗi dạng (§3.2), ngưỡng tỉ lệ rời (§4), khuyến nghị số lớp mỗi nhóm (§4), ngưỡng predictive validity ρ/hiệu chuẩn/AUC (§5), ngưỡng dừng sớm điểm phần trăm và độ trễ đánh giá ngày của chỉ số chính (§6).

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Điểm đo hiệu quả do server chấm; không đáp án xuống client | R5 · R6 · ADR-0016 |
| Mọi bảng mới có `tenant_id` (T) hoặc là dữ liệu platform (G); chỉ soft-delete | R8 · R21 · ADR-0005 · ADR-0028 |
| Đọc liên tenant chỉ System Admin, `CrossTenantScope` + audit | R20 · R27 · ADR-0027 |
| Thao tác ghi quan trọng có `Idempotency-Key` + audit | R10 |
| Không tạo áp lực/so sánh học sinh; không thưởng theo điểm đo hiệu quả | R16 · R19 |
| Không log PII, đáp án, khóa giả danh | R14 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 0 lần câu `efficacy_instrument` xuất hiện trong ≥ 10.000 lần chọn câu ngoài ExamSession đo hiệu quả (test thống kê).
- **SC-002**: 0 thay đổi mastery, XP, dashboard nhà trường do attempt đo hiệu quả.
- **SC-003**: Báo cáo trên dataset golden khớp giá trị tính tay của mọi chỉ số (sai số ≤ 0,001).
- **SC-004**: 100% học sinh cùng lớp cùng arm khi `unit = class`; 0 response cho học sinh chứa arm/study.
- **SC-005**: Bộ dữ liệu xuất có 0 cột định danh trực tiếp; participant đã rút không xuất hiện trong mọi báo cáo/xuất tạo sau thời điểm rút.

## Ngoài phạm vi

- Soạn nội dung câu/đề đo hiệu quả, lồng tiếng lớp 1–3 — đội EdTech/nội dung (dùng F053, F025).
- Hồi quy đa mức, phản biện thống kê, ngưỡng đạt exit criterion — chuyên gia thống kê, PO (ADR-0080 mục 2).
- Thử nghiệm `selection_rule` — chưa xếp feature [Proposed].
- Mô hình consent, nhận định pháp lý về nghiên cứu trên trẻ em — deferred (ADR-0082).
- Exam Zone trên Godot — F064 (dùng nguyên luồng ExamSession); giao diện giao bài/đề thường — F055.
- Thu điểm kiểm tra định kỳ của nhà trường làm tiêu chí ngoài — chưa xếp feature.

## Assumptions

- [Proposed] Permission `platform.efficacy_study.manage`, `platform.efficacy_study.export`, `platform.learning_experiment.manage` cấp cho role `system_admin`; chưa có role EdTech riêng.
- [Proposed] Chỉ số chính của thử nghiệm khi chưa có bài kiểm tra độc lập: độ chính xác lần đầu trên câu mới cùng LO sau độ trễ đánh giá ([LEARNING_EFFICACY_PLAN §6](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md)).
- [Proposed] Khóa giả danh lấy từ Docker secret `EW_EFFICACY_PSEUDONYM_KEY_FILE` (đăng ký ở LOCAL_DEVELOPMENT §4 qua task).
- ExamSession đo hiệu quả không bao giờ công bố kết quả cho học sinh, kể cả sau khi study `completed` ([LEARNING_EFFICACY_PLAN §8](../../docs/02-learning/LEARNING_EFFICACY_PLAN.md)).

### Câu hỏi mở (không chặn triển khai)

1. PRD cần FR riêng cho nghiên cứu hiệu quả (công cụ đo độc lập, lịch đợt, A/B, báo cáo, xuất giả danh, rút khỏi nghiên cứu); khi PO cấp mã, thay bảng FR gần nhất ở trên.
2. LEARNING_EFFICACY_PLAN §6 yêu cầu ADR riêng hoặc bổ sung ADR-0080 cho hạ tầng A/B (chạm schema learning event) — task Phase 2 viết bổ sung trước khi code US5.
3. Có cần role platform riêng cho EdTech lead (thay vì `system_admin`) để thu hẹp quyền xem khóa ánh xạ/xuất dữ liệu?
