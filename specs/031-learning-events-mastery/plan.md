# Implementation Plan: F031 — Learning events, mastery v1, evaluation và re-evaluate

**Branch**: `031-learning-events-mastery` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Module `learning_record` trong module `learning`: `LearningEventRecorder` chuẩn hóa envelope + kiểm schema cho mọi event (F030 dùng lại), job partition. Domain `mastery` hiện thực `MasteryModel` Weighted Evidence (ADR-0017) với golden test; consumer `learning.mastery_on_graded` (nhận `activity.graded`, `attempt.re_evaluated`) áp evidence, ghi `objective_masteries`, `mastery_history`, phát `learning_objective.progressed/mastered`. Consumer `learning.progress_daily` cập nhật bộ đếm. Command `CreateRegradeRequest` + job `regrade-runner` tạo evaluation mới. Job `mastery-recompute` (shadow → kích hoạt). Hoàn thành bài học (US6): ghim version bài trong `learning.lesson_sessions` khi bắt đầu attempt đầu tiên ngữ cảnh `lesson`, command `CompleteLesson` (consumer nội bộ `activity.graded` + `completeLesson`) phát `lesson.completed` (learning event + outbox, payload `lesson.completed.v1`) đúng một lần mỗi học sinh × version bài.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, nats.go (JetStream durable consumer), `santhosh-tekuri/jsonschema` (kiểm `learning-event.schema.json`), `src/backend/pkg/{outbox,tenancy,entx,authz,problem,clock,ids}` |
| Storage | PostgreSQL `eduworld_learning` (`learning.*`, `assessment.evaluations`, `assessment.regrade_requests`) |
| Testing | go test (golden test), testcontainers (PostgreSQL, NATS, Valkey — dừng Valkey cho NFR-AVAIL-003) |
| Target Platform | Linux container (module `learning` trong `core-api`, `worker-events`) |
| Project Type | backend module |
| Performance Goals | SC-002; consumer gom lô theo [DASHBOARD_DATA §4](../../docs/07-backend/DASHBOARD_DATA.md) |
| Constraints | R8, R9, R14, R21, R26, R27; ADR-0004, ADR-0017, ADR-0042, ADR-0070, ADR-0072 |
| Scale/Scope | `learning_events` partition tháng (DATABASE_DESIGN §5); 10.000 CCU |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta payload `attempt.re_evaluated`, `learning_objective.*`, `lesson.completed` (đã có, T045) và cột `regrade_requests`, `lesson_sessions` ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Consumer nội bộ module learning; event ra ngoài qua outbox; domain mastery thuần |
| III Cô lập tenant, an toàn dữ liệu | ✅ | RLS, append-only, Bypass có audit chỉ cho job re-evaluate platform; không hard-delete |
| IV Server-authoritative | ✅ | Mastery/evaluation/hoàn thành bài do server; bài ghim version khi bắt đầu (R31); regrade, `completeLesson` có Idempotency-Key |
| V Xác thực, phân quyền | ✅ | `learning_content.re_evaluate` (content_admin, academic_manager scope); progress: owner hoặc `analytics.school.read` trong scope |
| VI An toàn trẻ em | ✅ | Purpose limitation, không PII trong event, không so sánh học sinh |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số lấy từ `mastery_model_configs` theo tên CONFIG_DEFAULTS |
| VIII UI tách logic, token | ✅ | Không UI |
| IX Test có bằng chứng | ✅ | Golden test viết trước; tenant + authz cho endpoint mới |

## Project Structure

### Documentation (feature này)

```text
specs/031-learning-events-mastery/
├── spec.md · plan.md · data-model.md · quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
src/backend/modules/learning/
├── ent/schema/{mastery_model_config.go,objective_mastery.go,mastery_history.go,progress_daily.go,regrade_request.go,objective_mastery_shadow.go,mastery_recompute_run.go,lesson_session.go}
├── internal/domain/learningrecord/      # Envelope, Validate, IdempotencyKey builders
├── internal/domain/mastery/             # MasteryModel, WeightedEvidence, Evidence, State, Level, Effective
├── internal/domain/evaluation/          # Evaluation, Supersede, RegradeAction (accept_all, use_new_key, drop_item)
├── internal/domain/lesson/              # Session, CompletionRule, IdempotencyKey (US6)
├── internal/application/command/{apply_mastery_evidence.go,create_regrade_request.go,run_regrade_request.go,recompute_mastery.go,activate_mastery_model.go,upsert_progress_daily.go,pin_lesson_session.go,complete_lesson.go}
├── internal/application/query/{get_my_progress.go,get_student_progress.go}
├── internal/application/port/{learning_event_reader.go,mastery_repo.go,schema_validator.go}
├── internal/infrastructure/persistence/{mastery_repo.go,regrade_repo.go,learning_event_reader.go}
├── internal/presentation/http/public/{progress_handler.go,cms_regrade_handler.go,lesson_handler.go}
├── internal/presentation/consumer/{mastery_on_graded.go,progress_daily.go,lesson_completion.go}
└── internal/presentation/job/{learning_events_partitions.go,regrade_runner.go,mastery_recompute.go}
```

**Structure Decision**: `mastery` là domain thuần để golden test không cần DB; consumer đọc learning event cùng database (không gọi module khác).

## Phase 0: Research

Không có — công thức, tham số và ví dụ tính ở LEARNING_ANALYTICS §3; quy tắc event ở EVENT_DESIGN §5.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `mastery.WeightedEvidence.Apply/Effective` (§3.2: `w = w_ctx·w_diff·w_rep·w_hint·w_guess`, `α = max(min(α_max, α_base·w), 1/(n+3))`, decay khi đọc), `Level` (§3.2 bảng), `evaluation.Regrade(action)` | Tham số truyền vào, không đọc config trong domain |
| application | Command `ApplyMasteryEvidence`, `UpsertProgressDaily`, `PinLessonSession`, `CompleteLesson`, `CreateRegradeRequest`, `RunRegradeRequest`, `RecomputeMastery`, `ActivateMasteryModel` · Query `GetMyProgress`, `GetStudentProgress` | Permission: regrade `learning_content.re_evaluate`; `GetStudentProgress` `analytics.school.read` + scope trường/môn [Confirmed] |
| infrastructure | ent repo (UPDATE `WHERE version=$v`), `LearningEventReader` (SQL tường minh có `tenant_id`), `SchemaValidator`, outbox | |
| presentation | `getMyProgress`, `getStudentProgress`, `createRegradeRequest`, `getLesson`, `completeLesson`; consumer `learning.mastery_on_graded`, `learning.progress_daily`, `learning.lesson_completion`; job `learning-events-partitions` (hằng ngày), `regrade-runner`, `mastery-recompute` (CLI) | |

### Tích hợp và phụ thuộc

- Phụ thuộc: F030 (attempt, evaluation, learning event attempt; transaction start attempt gọi `PinLessonSession`), F024 (`content.lesson_versions`, `completion_rule`), F021 (LO, tiên quyết), F020 (`ref.enrollments` cho `class_id`).
- Event tiêu thụ (nội bộ): `activity.submitted`, `activity.graded`, `attempt.re_evaluated`. Event phát: `learning_objective.progressed`, `learning_objective.mastered`, `attempt.re_evaluated`, `lesson.completed`, `audit.recorded`. Consumer bên ngoài: module game (XP `learning_objective.mastered`, `lesson.completed` — F074), F043 rollup.

### Bảo mật, tenant, idempotency, audit

- `game_player` cho `getMyProgress`; học sinh (self, nhân sự `403 PERMISSION_DENIED` như F032-AC-09) cho `getLesson`, `completeLesson`; `portal_staff` cho `getStudentProgress`, `createRegradeRequest`.
- Job re-evaluate nội dung platform: `tenancy.Bypass(ctx, "re_evaluation")` (reason trong danh sách đóng ADR-0089 §3, client `learning_platform`), metric `eduworld_tenant_filter_bypass_total{module,reason}`, audit `cross_tenant.accessed` theo tenant (`mode = write`).
- Idempotency: learning event `idempotency_key`; consumer `processed_events`; `createRegradeRequest` Idempotency-Key.
- Audit: `content.re_evaluated`, `learning.profile_viewed`, `mastery_model.activated` [Confirmed].

### Quan sát

- Metric `eduworld_mastery_updates_total{result}`, `eduworld_mastery_update_lag_seconds`, `eduworld_learning_events_written_total{event_type}`, `eduworld_regrade_attempts_total{action}`, `eduworld_learning_events_partition_months_ahead`.
- Span `mastery.apply`, `regrade.run`; log không chứa `response` của học sinh.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
