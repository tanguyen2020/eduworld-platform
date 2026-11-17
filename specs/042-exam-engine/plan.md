# Implementation Plan: F042 — Exam Engine: 7 hình thức đề, random theo blueprint, thi có giờ, chấm lại đề

**Branch**: `042-exam-engine` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Module `exam` trong module `learning` (schema `assessment`): aggregate `ExamSession` ghim blueprint version, `PaperGenerator` sinh `ExamPaper` bất biến theo `variants.strategy` với seed tái lập và điểm mở rộng `VariantBalancer` (mặc định không cân bằng, F044 thay thế), attempt `mode = exam` dùng chung entity Attempt (F030) với `deadline_at` server, autosave append-only, chấm trì hoãn trong worker, công bố theo `ResultRelease`, job `exam.autosubmit` và `exam-window`. Chấm lại đề chạy theo `RegradeRequest` `scope = exam_session`. Mọi ghi qua command handler + outbox (ADR-0038, ADR-0042); kết quả chỉ ở PostgreSQL (ADR-0004).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, nats.go (JetStream), `src/backend/pkg/authz`, `src/backend/pkg/tenancy`, `src/backend/pkg/entx`, `src/backend/pkg/outbox`, `src/backend/pkg/idempotency`, `src/backend/pkg/clock` |
| Storage | PostgreSQL `eduworld_learning` (schema `assessment`, đọc `content`); object storage bucket private cho media đề (signed URL, ADR-0044); Valkey không dùng cho kết quả |
| Testing | go test + testcontainers (PostgreSQL, NATS, Valkey); đồng hồ giả `src/backend/pkg/clock` cho deadline/job |
| Target Platform | Linux container (module `learning` api + worker) |
| Project Type | backend module |
| Performance Goals | Start/save answer theo budget API học tập; auto-submit ≤ 1 phút sau hạn ([EXAM_ENGINE §7.2](../../docs/02-learning/EXAM_ENGINE.md#72-tự-động-nộp-khi-hết-giờ)) |
| Constraints | AGENTS R5, R6, R7, R8, R9, R10, R11, R14, R26, R27, R28, R31 · ADR-0004, ADR-0016, ADR-0043, ADR-0044, ADR-0072 |
| Scale/Scope | `answer_rate`, `attempt_start_rate` ([CONFIG_DEFAULTS §17](../../docs/00-overview/CONFIG_DEFAULTS.md)); lớp 45 học sinh bắt đầu cùng lúc |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | spec.md Ready; delta contract (`listExamLibrary`, `listExamSessionPapers`, `ExamSessionCreate`, payload event exam) ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ module `learning`; F042 cung cấp adapter cho port `ExamSessionCreator` của F041 (cùng module) gọi `CreateExamSession`/`CancelExamSession`; không gọi module khác đồng bộ |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng mới có `tenant_id` + soft-delete + RLS; attempt/answer append-only; không hard-delete |
| IV Server-authoritative | ✅ | Deadline, chấm, công bố ở server; payload không đáp án; submit/release/regrade có Idempotency-Key + audit |
| V Xác thực, phân quyền | ✅ | Permission từng endpoint (plan §Bảo mật); scope trường/môn; học sinh self-only |
| VI An toàn trẻ em | ✅ | Không xếp hạng với bạn; không log câu trả lời; cờ tốc độ không tự phạt |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số theo tên CONFIG_DEFAULTS; chuỗi mới `MaxLen`/`maxLength` |
| VIII UI tách logic, token | ✅ | Không có UI trong feature |
| IX Test có bằng chứng | ✅ | Mỗi `F042-AC-*` có task test viết trước; test tenant isolation + authorization mọi endpoint |

## Project Structure

### Documentation (feature này)

```text
specs/042-exam-engine/
├── spec.md
├── plan.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
src/backend/modules/learning/
├── ent/schema/
│   ├── exam_session.go                  # thêm cột (status, accommodations, release policy…)
│   ├── exam_paper.go                    # thêm variant_no, expected_p, student_id
│   ├── exam_participation.go            # mới
│   ├── attempt.go                       # thêm exam_paper_id, submit_reason, active_device_token, client_platform, game_session_id
│   └── regrade_request.go               # thêm scope, reason, corrected_version_id (nếu F031 chưa thêm)
├── internal/domain/
│   ├── exam/                            # session.go (aggregate, trạng thái), deadline.go, accommodation.go, release_policy.go
│   ├── exam/paper/                      # generator.go, variant_balancer.go (interface + NoopBalancer), shuffle.go
│   ├── exam/regrade/                    # regrade.go (accept_all, use_new_key, drop_item)
│   └── recommendation/r5_exam_followup.go
├── internal/application/
│   ├── command/exam/                    # create_exam_session.go, start_exam_attempt.go, save_exam_answer.go, submit_exam_attempt.go, auto_submit_expired.go, transition_exam_windows.go, grade_exam_attempt.go, release_exam_results.go, cancel_exam_session.go, switch_exam_device.go, run_exam_regrade.go
│   ├── query/exam/                      # list_exam_sessions.go, get_exam_session.go, list_exam_library.go, list_exam_session_papers.go, get_exam_result.go
│   └── port/                            # ExamSessionRepository, BlueprintReader, QuestionPoolReader, Grader, MediaSigner, AudienceResolver
├── internal/infrastructure/
│   ├── persistence/                     # ent repo exam_session/paper/participation; SQL đọc pool theo rule
│   └── storage/                         # signed URL media private
├── internal/presentation/
│   ├── http/public/exam_handler.go      # handler 11 operationId (contracts §1)
│   ├── consumer/                        # exam_grade_consumer.go (exam.submitted nội bộ), regrade_consumer.go
│   └── job/                             # exam_autosubmit.go (15 s), exam_window.go
└── tests/{unit,integration,contract}/exam/
```

**Structure Decision**: Exam dùng chung Attempt và grader của module learning (EXAM_ENGINE §1) nên ở cùng module; package `exam/paper` thuần để F044 cung cấp `VariantBalancer` thật qua composition root.

## Phase 0: Research

Không có. Thuật toán sinh đề, deadline, auto-submit, regrade đã định ở EXAM_ENGINE §3–§9.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** (cách chạy và kiểm) → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `exam.Session` (trạng thái, cửa sổ, chính sách công bố), `exam.Deadline` (late start, accommodation), `paper.Generator` + `paper.VariantBalancer`, `regrade.Plan` | Invariant: `feedback_timing ≠ immediate`; công bố không sớm hơn `result_release` |
| application | Command `CreateExamSession`, `StartExamAttempt`, `SaveExamAnswer`, `SubmitExamAttempt`, `AutoSubmitExpired`, `TransitionExamWindows`, `GradeExamAttempt`, `ReleaseExamResults`, `CancelExamSession` (qua adapter `ExamSessionCreator`), `SwitchExamDevice`, `RunExamRegrade` · Query `ListExamSessions`, `GetExamSession`, `ListExamLibrary`, `ListExamSessionPapers`, `GetExamResult` | Permission ở plan §Bảo mật |
| infrastructure | Adapter `ExamSessionCreator` (port của F041) gọi `CreateExamSession`/`CancelExamSession`; repository ent; SQL pool câu published khả dụng cho tenant; signer media private; outbox | `FOR UPDATE SKIP LOCKED` cho auto-submit |
| presentation | Handler `createExamSession`, `listExamSessions`, `getExamSession`, `startExamAttempt`, `saveExamAnswer`, `submitExamAttempt`, `getExamResult`, `getExamAttemptAnalysis`, `releaseExamResults`, `listExamLibrary`, `listExamSessionPapers`, nhánh `scope = exam_session` của `createRegradeRequest` · consumer chấm, regrade · job `exam.autosubmit`, `exam-window` | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F024 (blueprint, question set), F041 (Done trước; sở hữu port `ExamSessionCreator`, F042 cung cấp adapter để bài `graded` + blueprint tạo/hủy ExamSession — F042-AC-31), F030 (Attempt, grader), F031 (mastery tiêu thụ `exam.graded`; `createRegradeRequest`), F032 (`RuleRegistry` cho R5).
- Facade: không gọi facade module khác.
- Event phát: `exam.started`, `exam.submitted`, `exam.graded`, `exam.results_released`, `exam.regraded`, `attempt.re_evaluated`, learning event `question.answered` (khi chấm). Tiêu thụ: `content.published`/`content.rolled_back` (không ảnh hưởng session đã ghim, chỉ cache pool).
- F044 thay `VariantBalancer`, đọc `exam_papers.expected_p`, điểm attempt để làm báo cáo đề.

### Bảo mật, tenant, idempotency, audit

- `createExamSession`, `listExamSessionPapers`: `lms.assignment.write` [Proposed]; `listExamSessions`/`getExamSession` (nhân sự): `lms.assignment.read`; `releaseExamResults`: `lms.result.release`; regrade: `learning_content.re_evaluate`; route làm bài: học sinh self + thuộc đối tượng (`AudienceResolver` từ `ref.enrollments`, target snapshot).
- Scope: `school_admin` trường mình; `academic_manager` trường × môn (`PERMISSION_DENIED`); System Admin ghi với `X-Tenant-Id` đích.
- `Idempotency-Key` bắt buộc: `createExamSession`, `startExamAttempt`, `submitExamAttempt`, `releaseExamResults`, `createRegradeRequest`.
- Audit: `exam_session.created`, `exam_session.results_released`, `exam_session.cancelled`, `exam_attempt.device_switched`, `content.re_evaluated`.

### Quan sát

- Log không chứa `response`/đáp án (R14); chỉ `exam_session_id`, `attempt_id`, `position`.
- Metric: `eduworld_exam_attempts_started_total`, `eduworld_exam_autosubmit_total{reason}`, `eduworld_exam_answer_rejected_total{code}`, `eduworld_exam_grade_duration_seconds`, `eduworld_exam_regrade_attempts_total{action}`.
- Span: `CreateExamSession`, `paper.generate`, `StartExamAttempt`, `GradeExamAttempt`, `RunExamRegrade`.

Ước lượng: 2–3 sprint (backend 2 dev) [Proposed].

## Complexity Tracking

> Chỉ điền khi Constitution Check có ⚠️.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
| — | — | — |
