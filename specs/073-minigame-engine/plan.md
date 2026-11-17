# Implementation Plan: F073 — Mini-game Engine: `MiniGameConfig`, plugin `minigame`, scene Godot 11 loại

**Branch**: `073-minigame-engine` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Ba phần. (1) **module game**: item `minigame_config` trên lõi phát hành F071 (payload `minigame-config.schema.json`), `MiniGamePublishValidator` (bảng tương thích, gọi `validateMinigameQuestionSource` của learning qua facade `learning`), outbox `content.published` (`item_type = minigame_config`) có snapshot; `StartMiniGame` (ghim version, gán tenant, khối, `createAttempt`), bộ xử lý bước quest `minigame` đăng ký vào registry F072; bảng `game.minigame_plays`; consumer `game.minigame_on_activity_graded` phát `minigame.completed`. (2) **module learning**: projection `ref.minigame_config_versions` (consumer), plugin `minigame` trong Activity Registry dùng Exercise Engine chấm từng câu và `MinigameScorer` tính game points theo giờ server; `activity.graded` thêm khối `minigame`; internal `validateMinigameQuestionSource`. (3) **Godot**: renderer `minigame` (`ActivityHost` của F063) → `MiniGameHost` + `MiniGameRegistry` + `MiniGameBase` + 11 scene.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 · GDScript (Godot 4.x) |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, `pkg/entx` (`UnitOfWork`), `pkg/outbox`, `pkg/idempotency`, `pkg/modcall`, nats.go, JSON Schema validator · GdUnit4 |
| Storage | `eduworld_game`: `game.minigame_configs`, `game.minigame_config_versions`, `game.minigame_plays`; `eduworld_learning`: `ref.minigame_config_versions`, cột `assessment.attempts.game_score` |
| Testing | go test + testcontainers; facade test game ↔ learning (`createAttempt`, `validateMinigameQuestionSource`), contract test event; test hợp đồng plugin chung (TC-ACT-001); GdUnit4 |
| Target Platform | Linux container (`core-api` + `worker`; module game, learning) · Windows, Android, iOS, Web |
| Project Type | backend module (2) + game client |
| Performance Goals | Chấm một câu p95 theo round-trip chấm của GAME_DESIGN §7.1; hiệu ứng ≤ 100 ms sau kết quả; scene trong ngân sách [PERFORMANCE_BUDGET](../../docs/08-game-client/PERFORMANCE_BUDGET.md) |
| Constraints | AGENTS R1, R2, R5, R6, R8, R13, R19, R26, R29, R31, R32; ADR-0016, ADR-0029, ADR-0040, ADR-0042, ADR-0043, ADR-0047, ADR-0048, ADR-0059 |
| Scale/Scope | 11 `minigame_type`; `question_count` ≤ 50 |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Internal `validateMinigameQuestionSource`, `ActivityRef` `minigame_config`, payload event, `ActivityResponseResult`, `MiniGameSession` sửa ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | game sở hữu cấu hình; learning đọc projection từ event; game gọi learning qua facade `learning`; không đọc DB chéo |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `minigame_plays` T + RLS; cấu hình platform qua gán tenant; test isolation |
| IV Server-authoritative | ✅ | Chấm + game score ở learning; payload không đáp án; `Idempotency-Key` start/respond/submit; version ghim |
| V Xác thực, phân quyền | ✅ | `game_player`; internal `x-allowed-callers: [game]` |
| VI An toàn trẻ em | ✅ | Không thưởng tốc độ lớp 1–5; chơi lại không bị chặn, chỉ không thưởng |
| VII Một nguồn sự thật, cấu hình | ✅ | Bảng tương thích một nơi (MINIGAME_ENGINE §2 → code `compat.go` sinh test từ bảng); tham số CONFIG_DEFAULTS |
| VIII UI tách logic, token | ✅ | Scene chỉ phát `response_ready`; Theme token |
| IX Test có bằng chứng | ✅ | Mỗi F073-AC có task test |

## Project Structure

### Documentation (feature này)

```text
specs/073-minigame-engine/
├── spec.md · plan.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
modules/game/
├── ent/schema/                        # minigameconfig.go, minigameconfigversion.go, minigameplay.go
├── internal/domain/minigame/          # MiniGameConfig (parse), Compatibility (minigame_type ↔ QuestionType), PublishValidator, Play
├── internal/domain/quest/handlers/minigame.go
├── internal/application/command/       # start_minigame.go, record_minigame_result.go
├── internal/infrastructure/client/learningclient/   # + validateMinigameQuestionSource
├── internal/presentation/http/public/minigames.go
├── internal/presentation/consumer/minigame_on_activity_graded.go
└── seed/minigames/                    # 2 cấu hình + quest mẫu Gấu Nâu
modules/learning/
├── ent/schema/refminigameconfigversion.go
├── internal/domain/activity/plugins/minigame/   # plugin.go, scorer.go (game points), render.go
├── internal/application/query/validate_minigame_question_source.go
├── internal/presentation/http/internal/minigame_source.go
└── internal/presentation/consumer/minigame_config_projection.go
godot/
├── scenes/learning/activity_renderers/minigame/MinigameRenderer.tscn
├── scenes/minigames/{MiniGameHost.tscn, types/<type>/<Type>.tscn}
├── scripts/learning/{minigame_base.gd, minigame_registry.gd, minigame_host.gd}
└── tests/unit/minigames/, tests/fixtures/minigames/
```

**Structure Decision**: Theo [MINIGAME_ENGINE §4](../../docs/08-game-client/MINIGAME_ENGINE.md#4-kiến-trúc-client-godot-plugin) và [ACTIVITY_ENGINE §5](../../docs/02-learning/ACTIVITY_ENGINE.md#5-interface-plugin-go); plugin đặt trong module learning vì chấm điểm là của Exercise Engine (M-2, M-6).

## Phase 0: Research

Không có — quyết định sở hữu cấu hình, đường bắt đầu và nơi tính game score ghi ở Clarifications của spec.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain (game) | `Compatibility.Allowed(minigame_type) []QuestionType`; `PublishValidator`; `MiniGamePlay.Record(result, playsToday, limit)` | Thuần |
| domain (learning) | Plugin `minigame`: `BuildRenderPayload` (câu từ `question_source`, không đáp án), `Respond` (scorer câu + `MinigameScorer.Points(served_at, answered_at, streak, grade)`), `Evaluate` (`learning_score`, `game_score`, `completion_met`), `EmitEvidence` (chỉ câu trả lời) | `time_bonus = 0`, `wrong_penalty = 0` khi khối ≤ 5 |
| application | game: Command `StartMiniGame`, `RecordMiniGameResult`; learning: Query `ValidateMinigameQuestionSource`, Command `UpsertMiniGameConfigSnapshot` | Command/query tách, gọi qua interface có kiểu (ADR-0087) |
| infrastructure | learningclient (createAttempt, validate…); consumer projection; publisher outbox | |
| presentation | game: `startMiniGame`, consumer `game.minigame_on_activity_graded`; learning: internal `validateMinigameQuestionSource`, consumer `learning.minigame_config_projection` | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F022/F023 (scorer, render 11 loại), F030 (Activity Registry, `createAttempt`, `/learning/activity-attempts`), F071 (lõi phát hành), F072 (registry bước), F063 (`ActivityHost`).
- Event: phát `content.published` (`item_type = minigame_config`), `minigame.completed`; tiêu thụ `activity.graded` (khối `minigame`).

### Bảo mật, tenant, idempotency, audit

- `game_player` cho `startMiniGame`; facade `validateMinigameQuestionSource` chỉ cho caller `game` (`x-allowed-callers`, `make arch-check`); `TenantScope` cho `minigame_plays`.
- `Idempotency-Key`: `startMiniGame` (gửi tiếp xuống `createAttempt`), response/submit (F030); audit `minigame.completed` không cần riêng (phần thưởng audit ở F074).

### Quan sát

- Metric `eduworld_game_minigame_start_total{type,result}`, `eduworld_game_minigame_completed_total{type}`, `eduworld_learning_minigame_score_seconds`; span `minigame.start` có `traceparent` sang learning.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
