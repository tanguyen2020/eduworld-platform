# Implementation Plan: F030 — Activity Registry và Attempt API (`quiz`, `interactive_exercise`)

**Branch**: `030-activity-attempt-api` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Module `activity` trong module `learning`: interface `Plugin` + `Registry` (ACTIVITY_ENGINE §5), plugin `quiz` và `interactive_exercise` bọc scorer/render của Question Bank (F022). Aggregate `Attempt` (trạng thái, items ghim version, answers append-only, thời gian server) với command `StartAttempt`, `SubmitResponse`, `PauseAttempt`, `SubmitAttempt`, `ExpireAttempt`, `AbandonAttempt`; mỗi command ghi learning event (bảng `learning.learning_events`) + outbox trong cùng transaction. Ghim version qua `ReleaseResolver`/`VersionReader` của F026. Internal `createAttempt`/`getAttemptResult` cho module game. Worker `attempt-lifecycle` xử lý idle/abandon/expire/auto-submit.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen (server public + internal), nats.go, `santhosh-tekuri/jsonschema` (validate config theo `specs/schemas/activities/*`), `src/backend/pkg/{outbox,tenancy,entx,authz,problem,clock,ids,httpx}` |
| Storage | PostgreSQL `eduworld_learning` (`content.activities`, `content.activity_versions`, `assessment.*`, `learning.learning_events`, `infra.idempotency_records`); Valkey chỉ cache snapshot (F026) |
| Testing | go test + testcontainers (PostgreSQL, NATS, Valkey); contract test plugin; facade test với module game (game test với fake facade `learning`; learning test hiện thực facade theo ví dụ spec internal) |
| Target Platform | Linux container (module `learning` trong `core-api`, `worker-events`) |
| Project Type | backend module |
| Performance Goals | start p95 < 300 ms, response p95 < 150 ms, submit p95 < 300 ms [Proposed]; gọi liên module theo TC-PERF-013 |
| Constraints | R5, R6, R10, R11, R13, R14, R23, R26, R29, R31; ADR-0016, ADR-0043, ADR-0048, ADR-0059 |
| Scale/Scope | 10.000 CCU (NFR-SCAL); `answer_rate`, `attempt_start_rate` |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Contract attempt có sẵn; delta (`activity.abandoned` payload, `learning-event` `exercise.*` → `activity.*`, `ProblemCode`, `context_type`) ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | module game gọi qua facade `learning` (`src/backend/modules/learning/api`, `src/backend/pkg/modcall`); kết quả trả bằng event; plugin không truy cập DB module khác |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng `assessment.*` loại T/TA; `attempt_answers`, `learning_events` append-only; attempt chấm rồi không xóa |
| IV Server-authoritative | ✅ | Chấm, thời gian, điểm ở server; DTO không đáp án; start/response/submit có Idempotency-Key |
| V Xác thực, phân quyền | ✅ | `game_player` cho route học sinh; `portal_staff` + `learning_content.activity.*` cho CMS; `x-allowed-callers: [game]` |
| VI An toàn trẻ em | ✅ | Không log câu trả lời; learning event không có PII |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số theo tên CONFIG_DEFAULTS; enum từ `specs/schemas/common` |
| VIII UI tách logic, token | ✅ | Không UI |
| IX Test có bằng chứng | ✅ | Test trước mỗi story; tenant + authz mọi endpoint; contract test plugin |

## Project Structure

### Documentation (feature này)

```text
specs/030-activity-attempt-api/
├── spec.md · plan.md · data-model.md · quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
src/backend/modules/learning/
├── ent/schema/{activity.go,activity_version.go,attempt.go,attempt_item.go,attempt_answer.go,evaluation.go,learning_event.go}
├── internal/domain/activity/          # Plugin, Registry, Type, RenderPayload, Response, Evidence, CompletionRule
├── internal/domain/activity/plugins/quiz/, plugins/interactive/
├── internal/domain/attempt/           # Attempt aggregate, Item, Answer, Evaluation, scoring §7.2, FeedbackPolicy
├── internal/application/command/{start_attempt.go,start_ad_hoc_attempt.go,submit_response.go,pause_attempt.go,submit_attempt.go,expire_attempt.go,abandon_attempt.go,create_internal_attempt.go,create_activity.go,update_activity_draft.go}
├── internal/application/query/{get_attempt.go,list_my_attempts.go,get_attempt_review.go,get_attempt_result.go,list_activity_types.go,get_activity.go,list_cms_activities.go}
├── internal/application/port/{attempt_repo.go,learning_event_writer.go,question_scorer.go,question_selector.go}
├── internal/infrastructure/persistence/{attempt_repo.go,learning_event_writer.go}
├── internal/presentation/http/public/{activity_attempt_handler.go,cms_activity_handler.go}
├── internal/presentation/facade/attempt_facade.go     # hiện thực src/backend/modules/learning/api: createAttempt, getAttemptResult
├── internal/presentation/job/attempt_lifecycle.go
└── tests/{unit,integration,contract}/attempt/
src/backend/modules/game/internal/infrastructure/modules/learning.go      # adapter facade `learning` sau port của game (facade test với fake)
```

**Structure Decision**: Registry ở domain (interface thuần), plugin là package con; aggregate `Attempt` không biết loại activity cụ thể — gọi plugin qua interface (ADR-0048).

## Phase 0: Research

Không có — ACTIVITY_ENGINE, QUESTION_BANK, ADR-0016/0043/0048/0059 đủ.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `activity.Plugin` (Type, ConfigSchema, ValidateConfig, BuildRenderPayload, Respond, Progress, Evaluate, CompletionRule, EmitEvidence, MinClientVersion), `Registry`; `attempt.Attempt` (Start, Respond, Pause, Submit, Expire, Abandon), `FeedbackPolicy` (mode → `FeedbackTiming`, §8.1), `Score` (§7.2) | Không I/O |
| application | Command: `StartActivityAttempt`, `StartAdHocAttempt`, `SubmitActivityResponse`, `PauseActivityAttempt`, `SubmitActivityAttempt`, `ExpireAttempt`, `AbandonAttempt`, `CreateInternalAttempt`, `CreateCmsActivity`, `UpdateCmsActivityDraft` · Query: `GetActivityAttempt`, `ListMyActivityAttempts`, `GetActivityAttemptReview`, `GetAttemptResult`, `GetLearningActivity`, `ListActivityTypes`, `GetCmsActivity`, `ListCmsActivities` | Học sinh: owner check `student_id = principal`; CMS: `learning_content.activity.write`, `learning_content.read` |
| infrastructure | ent repo (`UPDATE … WHERE status='in_progress' AND version=$v`), `LearningEventWriter` (`ON CONFLICT DO NOTHING` theo `idempotency_key`), adapter `QuestionScorer`/`QuestionSelector` (F022), `ReleaseResolver`/`VersionReader` (F026), outbox | |
| presentation | Handler: `startActivityAttempt`, `startAdHocAttempt`, `listMyActivityAttempts`, `getActivityAttempt`, `submitActivityResponse`, `pauseActivityAttempt`, `submitActivityAttempt`, `getActivityAttemptReview`, `getLearningActivity`, `createCmsActivity`, `updateCmsActivityDraft`, `getCmsActivity`, `listCmsActivities`, `listActivityTypes`; internal `createAttempt`, `getAttemptResult`; job `attempt-lifecycle` (mỗi phút) | readiness probe kiểm registry |

### Tích hợp và phụ thuộc

- Phụ thuộc: F026 (resolver/reader, snapshot), F025 (`i18ntext.Resolve` cho `display_language`), F022 (scorer, selector, render câu hỏi), F027 (`math_ref` trong render).
- Event phát: domain `activity.started` (EVENT_DESIGN §4.3, consumer LMS F041, gợi ý F032), `activity.submitted`, `activity.graded`, `activity.abandoned`, `audit.recorded` (CMS); feature này là owner payload schema mọi domain event `activity.*` (không phát `exercise.*`, ACTIVITY_ENGINE §9); learning event `activity.started`, `question.answered`, `activity.submitted`, `activity.graded`.
- Gọi vào: module game → `createAttempt`/`getAttemptResult` (facade `learning`, `src/backend/pkg/modcall`, facade test).

### Bảo mật, tenant, idempotency, audit

- `game_player` (azp `eduworld-game-backend`, `user_type = student`) cho `/learning/activities/*`, `/learning/activity-attempts/*`; `portal_staff` cho `/cms/learning/activities*`; facade: chỉ module `game` (`x-allowed-callers`, `make arch-check`), `TenantScope` và Principal gốc (on-behalf) qua `context` (`src/backend/pkg/modcall`).
- Idempotency-Key: start, response, pause, submit (public) và `createAttempt` (facade); unique DB `(attempt_id, idempotency_key, answered_at)`.
- Audit: `content.version_created` (activity), `authz.denied`; attempt của học sinh không audit từng thao tác (learning event là bằng chứng).

### Quan sát

- Metric `eduworld_attempt_started_total{activity_type,mode}`, `eduworld_attempt_submitted_total{result}`, `eduworld_attempt_response_latency_seconds`, `eduworld_attempt_expired_total`, `eduworld_activity_registry_plugins`; span `attempt.start`, `attempt.respond`, `plugin.evaluate`.
- Log không chứa `response`; chỉ `attempt_id`, `position`, `is_correct`.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
