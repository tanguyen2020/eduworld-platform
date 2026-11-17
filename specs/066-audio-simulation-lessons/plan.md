# Implementation Plan: F066 — Bài nghe và mô phỏng: plugin `audio_lesson`, `simulation` + checker Go, template Godot

**Branch**: `066-audio-simulation-lessons` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Backend (module learning): hai plugin mới hiện thực `activity.Plugin` (ACTIVITY_ENGINE §5) và đăng ký vào Registry. `audiolesson.Plugin` dùng lại domain tiến độ media của F065 (`ranges`, `plausibility`) và bảng `assessment.video_progress` (thêm `plays_used`), thêm quy tắc lượt nghe (research R1). `simulation.Plugin` kiểm cấu hình theo `parameter_schema` của template, cấp seed, nhận run qua command `SubmitSimulationRun` (handler `submitSimulationRun` có sẵn trong spec), gọi `CheckerRegistry` để kiểm chứng xác định, lưu `assessment.simulation_runs`, ghi integrity signal, learning event `simulation.goal_met`, outbox `simulation.run_verified`/`run_rejected`. Danh mục `content.simulation_templates` đồng bộ từ descriptor trong mã (research R2). Godot: `AudioLessonRenderer` + `AudioLessonPlayer`; `SimulationRenderer` + `SimulationHost` + `SimulationActionLog`; template tham chiếu `physics.lever` v1 với cùng mô hình toán như checker và vector chung (research R4). Portal: editor `audio_lesson`, `simulation` cắm vào editor registry của F058.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 / GDScript (Godot 4.x ghim, static typing) / TypeScript strict (React 18, Vite) |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, `src/backend/pkg/entx`, `src/backend/pkg/outbox`, `src/backend/pkg/tenancy`, `src/backend/pkg/problem`, `santhosh-tekuri/jsonschema` (validate `parameter_schema`) / GdUnit4, `AudioStreamPlayer`, `ProjectSettings.load_resource_pack` / Ant Design 5 + Pro, `@rjsf/antd` (form từ JSON Schema), TanStack Query, ajv |
| Storage | PostgreSQL `eduworld_learning` (schema `content`, `assessment`); floci S3 cho media (F027) |
| Testing | go test (unit domain/checker), testcontainers (integration, global filter), contract test plugin chung (TC-ACT-001), vector chung / GdUnit4 unit + scene + integration headless / Vitest + Testing Library + MSW (nhãn `MOCK`), Playwright |
| Target Platform | Linux container / Windows, Android, Web (Compatibility renderer) / trình duyệt desktop |
| Project Type | backend module + game client + web editor |
| Performance Goals | Checker p95 và template (FPS, kích thước, thời gian nạp) theo [SIMULATION_ENGINE §6](../../docs/02-learning/SIMULATION_ENGINE.md); [PERFORMANCE_BUDGET §3](../../docs/08-game-client/PERFORMANCE_BUDGET.md) |
| Constraints | AGENTS R1, R5, R6, R8, R10, R13, R14, R21, R26–R28, R31, R32 · ADR-0016, ADR-0037–0044, ADR-0074, ADR-0048, ADR-0052, ADR-0057, ADR-0059, ADR-0072 |
| Scale/Scope | ≤ 30 câu, ≤ 50 đoạn/bài nghe; ≤ 500 hành động/64 KB mỗi run; ≤ 50 run/attempt (`simulation.schema.json`) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | spec Ready; delta OpenAPI/schema/event ở Phase 2 (T004–T007); không chạm nội dung deferred (bài nói chỉ ghi "ngoài Release") |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Cả hai plugin trong module learning; checker là domain thuần; command `SubmitSimulationRun`, `ReportProgress` là command handler (write context); không gọi module khác |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `simulation_runs` có `tenant_id`, `deleted_at`, RLS; `content.simulation_templates` là bảng G (platform, không tenant) chỉ đọc; không hard-delete (template gỡ → `retired`) |
| IV Server-authoritative | ✅ | Checker tính lại từ tham số ghim + seed server + action log; payload không có `check`/đáp án; run/submit có `Idempotency-Key`, audit, outbox |
| V Xác thực, phân quyền | ✅ | Route người chơi qua `game_player` + owner; CMS `learning_content.activity.write`, `listSimulationTemplates` `learning_content.read`; không quyền platform cho role tenant |
| VI An toàn trẻ em | ✅ | Không ghi âm, không xin micro (TC-AUD-004); không log câu trả lời/action log ở INFO; integrity signal không tự phạt |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số theo tên CONFIG_DEFAULTS; giới hạn log theo SIMULATION_ENGINE §5 (đề xuất đăng ký T049); mọi chuỗi `varchar(n)`/`maxLength` |
| VIII UI tách logic, token | ✅ | Renderer/editor chỉ gọi service/API; style từ `game-tokens`/`admin-tokens` |
| IX Test có bằng chứng | ✅ | Mỗi F066-AC có task test viết trước; vector chung chạy cả Go và GdUnit4 |

## Project Structure

### Documentation (feature này)

```text
specs/066-audio-simulation-lessons/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
src/backend/modules/learning/
├── ent/schema/{simulation_template.go,simulation_run.go}            # + thêm field plays_used vào video_progress.go (F065)
├── migrations/<ts>_audio_simulation.sql                              # Atlas diff + RLS viết tay
├── internal/domain/activity/audiolesson/{config.go,plays.go,completion.go}
├── internal/domain/activity/simulation/
│   ├── config.go                 # SimulationConfig + Validate (schema loại + parameter_schema)
│   ├── checker.go                # interface Checker, Action, Outcome, CheckerRegistry
│   ├── actionlog.go              # ValidateLog (≤ 500, ≤ 64 KB, t_ms tăng, action ∈ template)
│   ├── goals.go                  # EvaluateGoals(check, outcome) → goals_met
│   ├── seed.go                   # FixedSeed(versionID), NewSeed()
│   └── templates/physics.lever/v1/{descriptor.json,checker.go}
├── internal/application/activityplugin/{audiolesson,simulation}/plugin.go
├── internal/application/command/submit_simulation_run.go
├── internal/application/port/{simulation_run_repository.go,simulation_template_repository.go}
├── internal/infrastructure/persistence/{simulation_run_repo.go,simulation_template_repo.go}
├── internal/presentation/http/public/simulation_run_handler.go      # submitSimulationRun
├── internal/presentation/job/simtemplates_sync.go                    # upsert danh mục lúc khởi động
├── cmd/api/main.go                                                               # Registry.Register(audiolesson, simulation)
└── tests/{unit,integration,contract/plugins}/{audiolesson,simulation}/
specs/examples/simulation/physics.lever/v1/vectors.json               # vector chung Go + GdUnit4
src/game/
├── scenes/learning/activity_renderers/{audio_lesson/AudioLessonRenderer.tscn,simulation/SimulationRenderer.tscn}
├── scenes/learning/audio/AudioLessonPlayer.tscn (+ .gd)
├── scenes/learning/simulation/SimulationHost.tscn (+ .gd)
├── scenes/learning/simulation/templates/physics_lever/v1/{LeverTemplate.tscn,lever_template.gd,lever_model.gd,lever_lite.tscn}
├── scripts/learning/{simulation_action_log.gd,simulation_template_loader.gd,audio_play_tracker.gd}
├── locale/{vi.po,en.po}                                              # ui.audio.*, ui.simulation.*
└── tests/{unit,scene,integration,perf}/{audio,simulation}/
src/cms/frontend/apps/portal/src/workspaces/content-studio/activities/editors/
├── audio_lesson/{AudioLessonEditor.tsx,SegmentTable.tsx,TimedQuestionList.tsx}
├── simulation/{SimulationEditor.tsx,TemplatePicker.tsx,GoalEditor.tsx}
└── __tests__/
```

**Structure Decision**: Theo [BACKEND_DESIGN §3](../../docs/07-backend/BACKEND_DESIGN.md) (`src/backend/modules/learning/`, Bước 0 D1); checker và mô hình toán ở domain (thuần, test bằng vector), plugin ở application vì cần port. Godot theo [GODOT_ARCHITECTURE §3](../../docs/08-game-client/GODOT_ARCHITECTURE.md) (`scenes/learning/audio`, `scenes/learning/simulation`, `scripts/learning/simulation_action_log.gd`). Editor trong `content-studio` cùng workspace B (F053/F054/F058).

## Phase 0: Research

→ [research.md](research.md): R1 đếm lượt nghe; R2 danh mục template từ descriptor; R3 không gửi `check`; R4 mô hình toán + vector chung; R5 seed.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `audiolesson.Config`, `Plays.Accept(prev, playStarted, max)`, `simulation.Config`, `Checker`, `CheckerRegistry`, `ValidateLog`, `EvaluateGoals`, `lever.Checker` | Không I/O; invariant giới hạn log, seed, mục tiêu |
| application | Plugin `audiolesson.Plugin`, `simulation.Plugin`; command engine F030 gọi plugin (`StartAttempt`, `ReportProgress`, `SubmitResponse`, `SubmitAttempt`); command mới `SubmitSimulationRun` | Người chơi: `game_player` + owner; CMS: `learning_content.activity.write` |
| infrastructure | `SimulationRunRepository`, `SimulationTemplateRepository` (ent), `VideoProgressRepository` (F065, thêm `plays_used`), `IntegritySignalWriter`, `MediaVariantResolver` (F027) | Ghi run + outbox + integrity trong cùng transaction |
| presentation | Handler có sẵn: `startActivityAttempt`, `getActivityAttempt`, `reportActivityProgress`, `submitActivityResponse`, `submitSimulationRun`, `submitActivityAttempt`, `listActivityTypes`, `listSimulationTemplates`, `createCmsActivity`, `updateCmsActivityDraft`; job `simtemplates.Sync` | Không thêm route ngoài spec |

### Tích hợp và phụ thuộc

- F030 (Plugin/Registry, bảng attempt, command/query handler, outbox, audit), F065 (domain tiến độ media, `video_progress`), F031 (learning event, evidence), F027 (biến thể Ogg Vorbis, signed URL), F063 (`ActivityRendererBase`, renderer câu hỏi, `PendingWrites`, `ActivityHost`), F062 (`QualityManager`, `suspend_world`), F058 (editor registry, Activity Builder).
- Event phát: learning event `activity.started`, `question.answered`, `simulation.goal_met`, `activity.completed`; domain event (outbox) `simulation.run_verified`, `simulation.run_rejected`, và qua engine F030 `activity.submitted`, `activity.graded`.

### Bảo mật, tenant, idempotency, audit

- `TenantScope` mọi truy vấn `simulation_runs`/`video_progress`; RLS; owner `attempt.student_id = principal.user_id`; `content.simulation_templates` đọc không cần tenant (bảng G).
- Idempotency: tiến độ theo `client_seq`; `submitSimulationRun`, responses, submit theo `Idempotency-Key` (`infra.idempotency_records`).
- Audit: `attempt.submitted` do engine; `simulation.run_rejected` tiêu thụ bởi AUD (EVENT_DESIGN §4). Integrity signal `simulation_log_invalid`, `simulation_outcome_mismatch`, `audio_progress_implausible` vào `assessment.integrity_signals`.

### Quan sát

- Metric `eduworld_simulation_runs_total{template_code,status}`, `eduworld_simulation_checker_duration_seconds{template_code}` (histogram), `eduworld_audio_play_limit_rejected_total`; span `simulation.Verify`, `audiolesson.Progress`; log không chứa action log/câu trả lời (R14), chỉ `run_id`, `attempt_id`, `trace_id`.


### Danh mục menu/action portal (ADR-0090)

Menu và nút của feature lấy từ [`specs/portal/menu-catalog.yaml`](../portal/menu-catalog.yaml) qua registry sinh (`make portal-catalog`, F051) — **không** đăng ký menu tay trong TSX. Nút thao tác bọc `<ActionGate actionKey=…>`; menu/action trong `disabled_menus`/`disabled_actions` của `getMyPermissions` bị ẩn; `403 FEATURE_DISABLED` → thông báo "Tính năng đang tạm tắt" và tải lại quyền. Operation gọi tới đều có `x-portal-action` khớp các khóa dưới (kiểm `node scripts/specs/check-portal-catalog.mjs`).

| Loại | Khóa |
|---|---|
| Menu feature hiện thực | `menu.content_studio.activities`, `menu.content_studio.simulations` |
| Action dùng | `action.content_studio.activities.view`, `action.content_studio.activities.edit`, `action.content_studio.simulations.view`, `action.content_studio.media.view`, `action.content_studio.media.upload` |

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
| — | — | — |
