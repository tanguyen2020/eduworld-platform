# Tasks: F035 — Nghiên cứu hiệu quả học tập

**Input**: [spec.md](spec.md), [plan.md](plan.md), [research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo package `domain/efficacy`, `domain/efficacy/stats`, `domain/experiment`, `application/{command,query}/{efficacy,experiment}`; wiring handler admin — file: `src/backend/modules/learning/cmd/api/main.go`
- [ ] T002 [P] Đăng ký job `efficacy-wave-monitor`, `experiment-guardrail`, `experiment-lifecycle`; handler `efficacy_dataset` trong report worker F044 — file: `src/backend/modules/learning/cmd/worker/main.go`
- [ ] T003 [P] Secret `EW_EFFICACY_PSEUDONYM_KEY_FILE` (compose dev + `PseudonymKeyProvider`) — file: `deploy/compose/module learning.yaml`, `src/backend/modules/learning/internal/infrastructure/secret/pseudonym_key.go`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta OpenAPI: 24 operationId `/admin/learning/*` (có `x-portal-action`), `ReportRequest.report_type = efficacy_dataset`, `efficacy_instrument` trong input câu/blueprint, `ExamSession.is_efficacy`, mô tả chặn (contracts §6); menu `menu.platform_admin.efficacy_studies`, `menu.platform_admin.learning_experiments` trong `specs/portal/menu-catalog.yaml`; API_DESIGN §2.1, §9.16, §10 — file: `specs/openapi/modules/learning.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=learning`
- [x] T005 [P] Delta JSON Schema: `learning-event.schema.json` `$defs/context` (`efficacy_wave_id`, `experiment_arms`), `question.schema.json`, `assessment-blueprint.schema.json` (`efficacy_instrument`) — file: `specs/schemas/` — kiểm: `make specs-lint`
- [ ] T006 [P] Permission `platform.efficacy_study.manage`, `platform.efficacy_study.export`, `platform.learning_experiment.manage` (scope platform, seed role `system_admin`; ADMIN_PORTAL §2.2 đã có) — file: `src/backend/pkg/authz/catalog.go`, `deploy/seed/identity/roles.json`
- [ ] T007 [P] Đăng ký tham số mới (số câu/LO/dạng, ngưỡng rời, số lớp khuyến nghị, ngưỡng ρ/hiệu chuẩn/AUC, ngưỡng dừng sớm, độ trễ đánh giá) — file: `docs/00-overview/CONFIG_DEFAULTS.md`, `docs/02-learning/LEARNING_EFFICACY_PLAN.md`
- [ ] T008 Bổ sung ADR-0080 (chuyển Accepted, mục hạ tầng A/B theo LEARNING_EFFICACY_PLAN §6) và chỉ mục ADR — file: `docs/adr/ADR-0080-do-hieu-qua-hoc-tap-la-exit-criterion-pilot.md`, `docs/adr/README.md`
- [ ] T009 Cập nhật DATABASE_DESIGN (§2.5, §2.6, §2.7 bảng/cột mới) và EVENT_DESIGN (không phát `exam.*` cho session đo hiệu quả; context mới) — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/07-backend/EVENT_DESIGN.md`
- [ ] T010 ent schema 13 bảng mới + cột `efficacy_instrument`, `efficacy_wave_id`; view `question_delivery_v`; RLS bảng T (mọi chuỗi `MaxLen`) — file: `src/backend/modules/learning/ent/schema/` — kiểm: `make migrate-diff MOD=learning name=f035_efficacy`, `make migrate-lint MOD=learning`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Câu và đề đo hiệu quả độc lập với luyện tập (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [ ] T011 [P] [US1] Unit test `EfficacyGuard` lưu câu/blueprint (platform, `exam_only`, không tham số, `fixed`) (F035-AC-01) — file: `src/backend/modules/learning/internal/domain/question/efficacy_guard_test.go` — phải FAIL trước khi code
- [ ] T012 [P] [US1] Test thống kê 10.000 lần `Select` qua practice/retry_wrong/diagnostic/question set/`createQuestionPlan`/blueprint thường (F035-AC-02, SC-001) — file: `src/backend/modules/learning/tests/integration/efficacy/instrument_selection_test.go`
- [ ] T013 [P] [US1] Integration test tham chiếu câu đo hiệu quả từ set/lesson/activity/blueprint thường; `createExamSession` + `applyTenantAssignments` → 422 (F035-AC-03, F035-AC-04) — file: `src/backend/modules/learning/tests/integration/efficacy/instrument_reference_test.go`
- [ ] T014 [P] [US1] Integration test không object CDN, payload không đáp án (F035-AC-05) — file: `src/backend/modules/learning/tests/integration/efficacy/instrument_cdn_test.go`

### Implementation cho User Story 1

- [ ] T015 [US1] `question.EfficacyGuard` + gắn vào lưu câu/blueprint/set/lesson/activity và `selection.Select` — file: `src/backend/modules/learning/internal/domain/question/efficacy_guard.go`
- [ ] T016 [US1] Chặn ở `CreateExamSession` (đường tenant), `applyTenantAssignments`, snapshot CDN — file: `src/backend/modules/learning/internal/application/command/exam/create_exam_session.go`, `src/backend/modules/learning/internal/presentation/http/internal/tenant_assignments_handler.go`

## Phase 4: User Story 2 — Thỏa thuận, nghiên cứu và lớp tham gia/so sánh (Priority: P1)

### Tests cho User Story 2

- [ ] T017 [P] [US2] Integration test thỏa thuận: header bắt buộc, audit (F035-AC-06) — file: `src/backend/modules/learning/tests/integration/efficacy/agreement_test.go` — phải FAIL trước khi code
- [ ] T018 [P] [US2] Unit test `efficacy.Study` kiểm dạng đề, phủ LO, độ dài (F035-AC-07) — file: `src/backend/modules/learning/internal/domain/efficacy/study_test.go`
- [ ] T019 [P] [US2] Integration test thêm lớp: tenant khác 404, thiếu thỏa thuận 422, trùng study 409 (F035-AC-08) — file: `src/backend/modules/learning/tests/integration/efficacy/study_class_test.go`
- [ ] T020 [P] [US2] Unit test `Pairing` hash tái lập, xen kẽ `form_order`, khóa sau kích hoạt (F035-AC-09) — file: `src/backend/modules/learning/internal/domain/efficacy/pairing_test.go`
- [ ] T021 [P] [US2] Test authorization mọi route `/admin/learning/research-agreements*`, `/admin/learning/efficacy-studies*` (role tenant, `content_admin`, `safety_moderator`, thiếu MFA) (F035-AC-10) — file: `src/backend/modules/learning/tests/integration/efficacy/authz_test.go`
- [ ] T022 [P] [US2] Test tenant isolation + `CrossTenantScope` + audit `cross_tenant.accessed` cho list (F035-AC-11) — file: `src/backend/modules/learning/tests/integration/efficacy/tenant_isolation_test.go`

### Implementation cho User Story 2

- [ ] T023 [P] [US2] Domain `efficacy.Study`, `Pairing` — file: `src/backend/modules/learning/internal/domain/efficacy/study.go`, `src/backend/modules/learning/internal/domain/efficacy/pairing.go`
- [ ] T024 [US2] Command agreement/study/class/activate + repo write — file: `src/backend/modules/learning/internal/application/command/efficacy/`, `src/backend/modules/learning/internal/infrastructure/persistence/write/efficacy_repo.go`
- [ ] T025 [US2] Query list/get + read model SQL — file: `src/backend/modules/learning/internal/application/query/efficacy/`, `src/backend/modules/learning/internal/infrastructure/persistence/read/efficacy_read.go`
- [ ] T026 [US2] Handler admin agreement/study/class — file: `src/backend/modules/learning/internal/presentation/http/public/admin/efficacy_handler.go`

## Phase 5: User Story 3 — Đợt kiểm tra trước/sau không ảnh hưởng học tập (Priority: P1)

### Tests cho User Story 3

- [ ] T027 [P] [US3] Integration test `scheduleEfficacyWave` tạo ExamSession cài đặt cố định, dạng theo `form_order`, idempotent (F035-AC-12) — file: `src/backend/modules/learning/tests/integration/efficacy/wave_schedule_test.go` — phải FAIL trước khi code
- [ ] T028 [P] [US3] Unit test `efficacy.Wave` thứ tự pre/post, khoảng cách tuần (F035-AC-13) — file: `src/backend/modules/learning/internal/domain/efficacy/wave_test.go`
- [ ] T029 [P] [US3] Integration test chụp participant khi mở đợt, loại `processing_restricted` (F035-AC-14) — file: `src/backend/modules/learning/tests/integration/efficacy/participant_snapshot_test.go`
- [ ] T030 [P] [US3] Integration test attempt đo hiệu quả: mastery/progress không đổi, không outbox `exam.*`, không R5, event có `efficacy_wave_id`, recompute bỏ qua (F035-AC-15, SC-002) — file: `src/backend/modules/learning/tests/integration/efficacy/no_evidence_test.go`
- [ ] T031 [P] [US3] Test authorization/chặn kết quả, papers, report, regrade của session đo hiệu quả (F035-AC-16) — file: `src/backend/modules/learning/tests/integration/efficacy/exam_lock_test.go`
- [ ] T032 [P] [US3] Contract test response học sinh không có study/arm; tenant isolation session (F035-AC-17) — file: `src/backend/modules/learning/tests/contract/efficacy_student_payload_test.go`

### Implementation cho User Story 3

- [ ] T033 [P] [US3] Domain `efficacy.Wave`, `Participant` — file: `src/backend/modules/learning/internal/domain/efficacy/wave.go`, `src/backend/modules/learning/internal/domain/efficacy/participant.go`
- [ ] T034 [US3] Command `ScheduleEfficacyWave`, `OpenEfficacyWave` + `CreateExamSession` nhận `EfficacySettings` — file: `src/backend/modules/learning/internal/application/command/efficacy/schedule_wave.go`, `src/backend/modules/learning/internal/application/command/exam/create_exam_session.go`
- [ ] T035 [US3] F042: bỏ outbox `exam.*`, bỏ R5, chặn release/papers/result/analysis khi `efficacy_wave_id`; F044 chặn `getExamSessionReport`; F031 bỏ evidence + recompute — file: `src/backend/modules/learning/internal/application/command/exam/grade_exam_attempt.go`, `src/backend/modules/learning/internal/application/command/apply_mastery_evidence.go`
- [ ] T036 [US3] Job `efficacy-wave-monitor` (`tenancy.Bypass(ctx, "efficacy_study")`) — file: `src/backend/modules/learning/internal/presentation/job/efficacy_wave_monitor.go`

## Phase 6: User Story 4 — Rút khỏi nghiên cứu theo yêu cầu trường (Priority: P2)

### Tests cho User Story 4

- [ ] T037 [P] [US4] Integration test rút theo 4 phạm vi, loại khỏi session chưa mở, hủy session lớp, idempotent, audit (F035-AC-18) — file: `src/backend/modules/learning/tests/integration/efficacy/withdraw_test.go` — phải FAIL trước khi code
- [ ] T038 [P] [US4] Integration test loại khỏi báo cáo/xuất sau rút, soft-delete snapshot, `anonymizeUserData` (F035-AC-19) — file: `src/backend/modules/learning/tests/integration/efficacy/withdraw_data_test.go`
- [ ] T039 [P] [US4] Integration test thu hồi thỏa thuận → rút study + rời thử nghiệm (F035-AC-20) — file: `src/backend/modules/learning/tests/integration/efficacy/agreement_revoke_test.go`

### Implementation cho User Story 4

- [ ] T040 [US4] Command `WithdrawEfficacyParticipants`, `RevokeResearchAgreement` (gọi `CancelExamSession`, `ExcludeExamSessionStudents`) — file: `src/backend/modules/learning/internal/application/command/efficacy/withdraw_participants.go`
- [ ] T041 [US4] Mở rộng `anonymizeUserData` cho bảng efficacy/experiment — file: `src/backend/modules/learning/internal/application/command/anonymize_user_data.go`

## Phase 7: User Story 5 — Thử nghiệm A/B cấu hình mastery và luật gợi ý (Priority: P2)

### Tests cho User Story 5

- [ ] T042 [P] [US5] Unit test `experiment.Experiment` validate arm/allocation/tham số/lớp so sánh, `revision` (F035-AC-21) — file: `src/backend/modules/learning/internal/domain/experiment/experiment_test.go` — phải FAIL trước khi code
- [ ] T043 [P] [US5] Unit test `Assign` hash ổn định, cùng lớp cùng arm, giữ khi chuyển lớp (F035-AC-22, SC-004) — file: `src/backend/modules/learning/internal/domain/experiment/assignment_test.go`
- [ ] T044 [P] [US5] Integration test shadow tính song song, hiển thị không đổi (F035-AC-23) — file: `src/backend/modules/learning/tests/integration/experiment/shadow_test.go`
- [ ] T045 [P] [US5] Integration test live: đọc phía học sinh theo arm, nhà trường/XP theo `active`, exam không bị tác động (F035-AC-24) — file: `src/backend/modules/learning/tests/integration/experiment/live_test.go`
- [ ] T046 [P] [US5] Contract test learning event có `experiment_arms`, response học sinh không có arm (F035-AC-25) — file: `src/backend/modules/learning/tests/contract/experiment_arms_test.go`
- [ ] T047 [P] [US5] Integration test rào chắn dừng sớm, dừng tay, hết hạn, không phát lại `mastered` (F035-AC-26) — file: `src/backend/modules/learning/tests/integration/experiment/guardrail_test.go`
- [ ] T048 [P] [US5] Test kết quả ẩn ô nhỏ + authorization/tenant isolation route `/admin/learning/experiments*` (F035-AC-27) — file: `src/backend/modules/learning/tests/integration/experiment/results_authz_test.go`

### Implementation cho User Story 5

- [ ] T049 [P] [US5] Domain `experiment` (Experiment, Assign, ArmParams, Guardrail) — file: `src/backend/modules/learning/internal/domain/experiment/`
- [ ] T050 [US5] Command/query experiment + handler admin — file: `src/backend/modules/learning/internal/application/command/experiment/`, `src/backend/modules/learning/internal/presentation/http/public/admin/experiment_handler.go`
- [ ] T051 [US5] `ModelResolver` trong `ApplyMasteryEvidence`, `ExperienceMasteryReader` + `ArmResolver` cho `getMyProgress`/`getMyMastery`/rule engine/`selection.Policy`; `experiment_arms` trong `LearningEventRecorder` — file: `src/backend/modules/learning/internal/application/port/read/experience_mastery_reader.go`, `src/backend/modules/learning/internal/domain/learningrecord/recorder.go`
- [ ] T052 [US5] Job `experiment-guardrail`, `experiment-lifecycle` — file: `src/backend/modules/learning/internal/presentation/job/experiment_guardrail.go`

## Phase 8: User Story 6 — Báo cáo hiệu quả và độ tin cậy dự báo của mastery (Priority: P2)

### Tests cho User Story 6

- [ ] T053 [P] [US6] Integration test chụp `M_eff` khi mở đợt `post` (F035-AC-28) — file: `src/backend/modules/learning/tests/integration/efficacy/mastery_snapshot_test.go` — phải FAIL trước khi code
- [ ] T054 [P] [US6] Golden test `stats` gain/Hedges' g/ICC/CI/KR-20/rời nghiên cứu, tái lập theo `inputs_hash` (F035-AC-29, SC-003) — file: `src/backend/modules/learning/internal/domain/efficacy/stats/golden_test.go`
- [ ] T055 [P] [US6] Golden test Spearman, hiệu chuẩn, AUC, gợi ý (F035-AC-30) — file: `src/backend/modules/learning/internal/domain/efficacy/stats/validity_test.go`
- [ ] T056 [P] [US6] Integration test ẩn ô nhỏ, không định danh, `descriptive_only` (F035-AC-31) — file: `src/backend/modules/learning/tests/integration/efficacy/report_privacy_test.go`

### Implementation cho User Story 6

- [ ] T057 [P] [US6] Domain `efficacy/stats` theo research R2 — file: `src/backend/modules/learning/internal/domain/efficacy/stats/`
- [ ] T058 [US6] Command `GenerateEfficacyReport` (đọc replica, dùng `examquality.KR20`), query `GetEfficacyStudyReport` — file: `src/backend/modules/learning/internal/application/command/efficacy/generate_report.go`

## Phase 9: User Story 7 — Xuất dữ liệu định danh giả có kiểm soát (Priority: P2)

### Tests cho User Story 7

- [ ] T059 [P] [US7] Integration test CSV chỉ cột giả danh, không participant rút (F035-AC-32, SC-005) — file: `src/backend/modules/learning/tests/integration/efficacy/dataset_export_test.go` — phải FAIL trước khi code
- [ ] T060 [P] [US7] Unit test `Pseudonymizer` ổn định trong study, khác giữa study; thiếu khóa → lỗi (F035-AC-33) — file: `src/backend/modules/learning/internal/domain/efficacy/pseudonym_test.go`
- [ ] T061 [P] [US7] Integration test audit, link TTL, giới hạn đồng thời, authorization export (F035-AC-34) — file: `src/backend/modules/learning/tests/integration/efficacy/dataset_export_authz_test.go`

### Implementation cho User Story 7

- [ ] T062 [P] [US7] `efficacy.Pseudonymizer` (research R3) — file: `src/backend/modules/learning/internal/domain/efficacy/pseudonym.go`
- [ ] T063 [US7] Command `ExportEfficacyDataset` (handler `report_type` của report worker) — file: `src/backend/modules/learning/internal/application/command/efficacy/export_dataset.go`

## Phase 10: User Story 8 — Màn quản trị nghiên cứu trên Portal (Priority: P3)

### Tests cho User Story 8

- [ ] T064 [P] [US8] Playwright luồng study/thử nghiệm của `sa.admin`, ẩn menu với `ad.truong.a`, axe + lint token (F035-AC-35) — file: `src/cms/frontend/apps/portal/e2e/platform/research.spec.ts` — phải FAIL trước khi code
- [ ] T065 [P] [US8] Vitest ô `efficacy_instrument` chỉ hiện ở phạm vi platform (F035-AC-36) — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/__tests__/EfficacyInstrumentField.test.tsx`

### Implementation cho User Story 8

- [ ] T066 [P] [US8] Trang workspace D `research/*` + API client — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/research/`
- [ ] T067 [P] [US8] `EfficacyInstrumentField` trong editor câu/blueprint — file: `src/cms/frontend/apps/portal/src/workspaces/content-studio/questions/EfficacyInstrumentField.tsx`

## Phase 11: Polish & Cross-Cutting

- [ ] T068 [P] Metric, span, alert `ExperimentGuardrailBreached` — file: `deploy/observability/alerts/learning.yaml`
- [ ] T069 [P] Cập nhật LEARNING_EFFICACY_PLAN (hiện thực §6, §10), LEARNING_ANALYTICS §3.4, CHANGELOG; trạng thái F035 trong CODE_PLAN — file: `docs/02-learning/LEARNING_EFFICACY_PLAN.md`, `CHANGELOG.md`, `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md`
- [ ] T070 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/035-learning-efficacy-study/quickstart.md`
- [ ] T071 [P] Test F035-AC-37: tắt `menu.platform_admin.efficacy_studies`/`menu.platform_admin.learning_experiments` → menu/nút ẩn, `403 FEATURE_DISABLED` hiện thông báo và tải lại `getMyPermissions` (TC-CMS-MENU-02) — file: `src/cms/frontend/apps/portal/e2e/platform/research-feature-toggle.spec.ts`
- [ ] T072 Dùng khóa catalog, không đăng ký menu tay: route/menu lấy từ registry sinh từ `specs/portal/menu-catalog.yaml` (F051), nút thao tác bọc `<ActionGate actionKey=…>` theo bảng khóa trong [plan.md](plan.md) (F035-AC-37) — file: `src/cms/frontend/apps/portal/src/workspaces/platform-admin/research/routes.tsx`

## Dependencies & Execution Order

- Setup → Foundational (T004–T010; T008 phải xong trước Phase 7) → US1 → US2 → US3 (cần US1, US2) → US4, US6 (cần US3) → US7 (cần US6) → US5 (độc lập sau Phase 2, cần F032 rule engine) → US8 (cần API các story) → Polish.
- T035 sửa code F031/F042/F044 trong cùng module: chạy lại test của các feature đó.

## Definition of Done

- [ ] Mọi tiêu chí F035-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make web-test` xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, DATABASE_DESIGN, EVENT_DESIGN, ADMIN_PORTAL, CHANGELOG cập nhật
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
