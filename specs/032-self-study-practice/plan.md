# Implementation Plan: F032 — Chế độ luyện tập, hỗ trợ tự học (gợi ý, chẩn đoán, `rapid_guess`), gợi ý ôn tập tự động

**Branch**: `032-self-study-practice` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Mở rộng Activity & Attempt Engine (F030) và learning_record (F031) trong module `learning` để học sinh tự học: (1) attempt ad-hoc `practice`/`retry_wrong`/`diagnostic` qua `startAdHocAttempt` với bộ chọn câu theo mastery, câu anh em và luyện trộn; (2) `revealNextHint` + `hints_used`; (3) cờ `rapid_guess` tính ở server; (4) `MasteryModel` version mới có `w_hint`, `w_guess`, loại `rapid_guess`, đặt `M0` từ bài chẩn đoán; (5) danh sách ôn câu sai; (6) rule engine gợi ý ôn tập (strategy registry) chạy trong worker, đọc qua `getMyRecommendations`; (7) API tiến độ cho học sinh. Mọi thay đổi trạng thái qua command handler + outbox (ADR-0038, ADR-0042).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, nats.go (JetStream), `src/backend/pkg/authz`, `src/backend/pkg/tenancy`, `src/backend/pkg/entx`, `src/backend/pkg/outbox`, `src/backend/pkg/idempotency` |
| Storage | PostgreSQL `eduworld_learning` (schema `assessment`, `learning`, `content` đọc); Valkey chỉ cache pool câu (`ew:learning:qb:pool:*`) |
| Testing | go test + testcontainers (PostgreSQL, NATS, Valkey); contract test render payload |
| Target Platform | Linux container (module `learning` api + worker) |
| Project Type | backend module |
| Performance Goals | API học tập theo [PERFORMANCE_BUDGET](../../docs/08-game-client/PERFORMANCE_BUDGET.md) (`load_time` mở activity); worker gợi ý không chặn đường trả lời câu |
| Constraints | AGENTS R5, R6, R8, R10, R11, R14, R19, R26, R27, R28, R31 · ADR-0016, ADR-0017, ADR-0043, ADR-0072 |
| Scale/Scope | `answer_rate`, `attempt_start_rate` ([CONFIG_DEFAULTS §17](../../docs/00-overview/CONFIG_DEFAULTS.md)); 10.000 CCU (NFR-SCAL theo PRD) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | spec.md Ready; delta contract (RecommendationList, `getMyMastery`, payload event) ở Phase 2 tasks; không chạm nội dung deferred |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ module `learning`; không gọi module khác đồng bộ; phản ứng phụ qua event; command (write context) và query (read context) tách (ADR-0087) |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng mới có `tenant_id` + `deleted_at` + RLS; test tenant isolation cho mọi endpoint; không hard-delete (danh sách ôn "rời" = `status = cleared`) |
| IV Server-authoritative | ✅ | Chấm, `response_ms`, `rapid_guess`, mastery tính ở server; gợi ý chỉ trả từng cấp; start/hint có `Idempotency-Key` |
| V Xác thực, phân quyền | ✅ | Route học sinh: principal `user_type = student`, self-only; nhân sự → `403 PERMISSION_DENIED` |
| VI An toàn trẻ em | ✅ | Không log câu trả lời/gợi ý (R14); không hiện cờ `rapid_guess` cho học sinh (R19); không so sánh với bạn (R16) |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số đọc từ cấu hình platform theo tên CONFIG_DEFAULTS; chuỗi mới có `MaxLen`/`maxLength` |
| VIII UI tách logic, token | ✅ | Không có UI trong feature; lý do gợi ý là mã + bản dịch, client chỉ hiển thị |
| IX Test có bằng chứng | ✅ | Mỗi `F032-AC-*` có task test viết trước; golden test mastery; test tenant isolation + authorization |

## Project Structure

### Documentation (feature này)

```text
specs/032-self-study-practice/
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
│   ├── wrong_question_entry.go          # mới: learning.wrong_question_entries
│   ├── student_recommendation.go        # mới: learning.rm_student_recommendations
│   ├── attempt.go                       # thêm recommendation_id
│   ├── attempt_item.go                  # thêm hints_revealed
│   └── objective_mastery.go             # thêm prior_value, prior_source, prior_diagnostic_attempt_id
├── internal/domain/
│   ├── selection/                       # adaptive_difficulty.go, sibling.go, interleave.go (thuần, seed PRNG)
│   ├── attempt/                         # hint.go (HintPolicy), rapid_guess.go (RapidGuessDetector)
│   ├── mastery/                         # evidence_weight.go (w_hint, w_guess, loại rapid_guess), diagnostic_prior.go
│   ├── retrywrong/                      # entry.go (lịch 1–3–7, rời danh sách)
│   └── recommendation/                  # rule.go (interface Rule), engine.go, r2_weak.go, r3_prereq.go, r4_decay.go, r7_diagnostic.go, r8_interleave.go, r9_retry_due.go
├── internal/application/
│   ├── command/practice/                # start_adhoc_attempt.go, reveal_next_hint.go
│   ├── command/mastery/                 # apply_diagnostic_prior.go
│   ├── command/retrywrong/              # record_wrong_answer.go, record_retry_result.go
│   ├── command/recommendation/          # recompute_student_recommendations.go, mark_recommendation_started.go
│   ├── query/practice/                  # list_my_wrong_questions.go
│   ├── query/recommendation/            # get_my_recommendations.go
│   ├── query/progress/                  # get_my_progress.go, get_my_mastery.go
│   └── port/                            # QuestionPoolReader, MasteryReader, WrongQuestionRepository, RecommendationRepository, RuleRegistry, ContentAvailabilityReader, LearningConfigReader
├── internal/infrastructure/
│   ├── persistence/                     # ent repo + read SQL (pool candidate, sibling query) có scope tenant + deleted_at
│   ├── cache/                           # question_pool_cache.go
│   └── messaging/                       # outbox writer cho learning.recommendation_generated
├── internal/presentation/
│   ├── http/public/                     # handler startAdHocAttempt, revealNextHint, listMyWrongQuestions, getMyRecommendations, getMyProgress, getMyMastery
│   ├── consumer/                        # wrong_question_consumer.go, recommendation_consumer.go, diagnostic_prior_consumer.go
│   └── job/                             # recommendation_nightly.go (R4 decay, R7 đầu năm)
└── tests/{unit,integration,contract}/practice|recommendation|mastery/
```

**Structure Decision**: Giữ trong module `learning` vì attempt, mastery, nội dung đều thuộc module này (ADR-0074); rule engine là package domain thuần để F040/F041/F042 đăng ký rule R6/R1/R5 bằng `RuleRegistry` lúc composition root (`src/backend/cmd/worker/main.go`).

## Phase 0: Research

Không có. Thuật toán và tham số đã có ở LEARNING_ANALYTICS §3, §6, QUESTION_BANK §6, ACTIVITY_ENGINE §8.3, §11.1.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** (cách chạy và kiểm) → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `selection.Policy`, `attempt.HintPolicy`, `attempt.RapidGuessDetector`, `mastery.EvidenceWeight`, `mastery.DiagnosticPrior`, `retrywrong.Entry`, `recommendation.Engine` + `Rule` | Thuần, không I/O; seed PRNG từ `attempts.seed` |
| application | Command `StartAdHocAttempt`, `RevealNextHint`, `ApplyDiagnosticPrior`, `RecordWrongAnswer`, `RecordRetryResult`, `RecomputeStudentRecommendations`, `MarkRecommendationStarted` · Query `ListMyWrongQuestions`, `GetMyRecommendations`, `GetMyProgress`, `GetMyMastery` | Route học sinh: `authz.RequireStudentSelf`; idempotency middleware `src/backend/pkg/httpx` cho start/hint |
| infrastructure | Repository ent cho attempt, wrong entry, recommendation; SQL đọc pool/sibling; cache pool Valkey; outbox | SQL tường minh nhận `TenantScope` + `deleted_at IS NULL` (R27) |
| presentation | Handler `startAdHocAttempt`, `revealNextHint`, `listMyWrongQuestions`, `getMyRecommendations`, `getMyProgress`, `getMyMastery` · consumer `question.answered`, `activity.submitted`, `activity.graded`, `attempt.re_evaluated` · job `recommendation-nightly` | Consumer idempotent qua `infra.processed_events` |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F031 (mastery v1, learning events), F030 (Attempt, `submitActivityResponse`, `getActivityAttempt`).
- Facade: không gọi facade module khác (R29).
- Event phát: `learning.recommendation_generated`, `activity.started` (mang `recommendation_id`; phát bởi engine F030, F032 chỉ truyền id), learning event `question.answered` (mang `hints_used`, `response_ms`, `rapid_guess`). Event tiêu thụ (nội bộ module): `question.answered`, `activity.submitted`, `activity.graded`, `attempt.re_evaluated`, `content.published` (vô hiệu cache pool).
- Feature dùng lại: F040 (R6), F041 (R1), F042 (R5) đăng ký rule; F043 đọc `rapid_guess`, `hints_used`, `recommendation_id` cho dashboard.

### Bảo mật, tenant, idempotency, audit

- Route học sinh, self-only; staff → `403 PERMISSION_DENIED`; tenant từ `X-Tenant-Id` đã kiểm (`403 TENANT_MISMATCH`).
- `TenantScope` cho mọi query; RLS trên bảng mới; nội dung platform chỉ khi có `infra.tenant_resource_assignments` active.
- `Idempotency-Key` bắt buộc: `startAdHocAttempt`, `revealNextHint` (`infra.idempotency_records`, `idempotency_record_ttl`).
- Audit: không có thao tác quản trị; không ghi audit cho thao tác học tập của học sinh (dấu vết là learning event).

### Quan sát

- Log `slog` không chứa `response`, nội dung gợi ý, đáp án (R14); chỉ `attempt_id`, `position`, `level`.
- Metric: `eduworld_learning_hints_revealed_total{mode}`, `eduworld_learning_rapid_guess_total{question_type}`, `eduworld_learning_recommendations_generated_total{rule_code}`, `eduworld_learning_question_pool_insufficient_total`.
- Span: `StartAdHocAttempt`, `selection.pick`, `RevealNextHint`, `RecomputeStudentRecommendations`.

Ước lượng: 2–3 sprint (backend 2 dev) [Proposed].

## Complexity Tracking

> Chỉ điền khi Constitution Check có ⚠️.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
| — | — | — |
