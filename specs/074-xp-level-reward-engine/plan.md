# Implementation Plan: F074 — XP ledger, level, đường cong XP, danh hiệu, Reward Engine có cap

**Branch**: `074-xp-level-reward-engine` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Thêm bounded context `reward` + `progression` vào module game. Consumer durable `game.reward_engine` (JetStream, FilterSubjects các trigger) chuyển event thành command `AwardFromEvent`: nạp reward rule published (cache con trỏ), tính XP thuần trong domain (`reward.Engine`: điều kiện → số XP gốc → hệ số khối → giảm dần theo content version → cap ngày), rồi trong một transaction `eduworld_game`: khóa `xp_daily_caps`, insert `xp_ledger` (unique idempotent), cập nhật `game_profiles.xp_total/level/title_code`, ghi `level_history` + `reward_grants` khi lên cấp, outbox `xp.awarded`, `level.reached`, `reward.granted`. Sau commit publish Valkey `ew:rt:user:<id>` (`progression.xp.updated`, `progression.level.reached`). Query handler đọc tiến trình. Module config `progression-curves`, `reward-rules` đăng ký vào `ConfigTypeRegistry` (validate + `OnPublished` sinh `progression_levels`, kích hoạt job `curve-rollout`). Consumer `game.grade_transition` áp FR-GAM-010.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen, `src/backend/pkg/outbox`, `src/backend/pkg/natsx`, `src/backend/pkg/entx`, `src/backend/pkg/tenancy`, `src/backend/pkg/clock`, santhosh-tekuri/jsonschema (validate payload) |
| Storage | PostgreSQL `eduworld_game` (`gamification.xp_ledger` TA, `gamification.xp_daily_caps` T, `gamification.reward_rules`/`reward_rule_versions` G, `gamification.reward_grants` T, `game.progression_curves`/`progression_curve_versions`/`progression_levels` G, `game.level_history` TA, `game.game_profiles` T); Valkey cache `ew:gam:cap:*`, `ew:game:current:_platform:*` |
| Testing | go test + testcontainers (PostgreSQL, NATS, Valkey); property test cho `reward.Engine` |
| Target Platform | Linux container (`core-api` + `worker`, module `game`) |
| Project Type | backend module |
| Performance Goals | XP tới client ≤ 2 s (AC-GAM-004); consumer ≥ 500 event/s/instance |
| Constraints | AGENTS R5, R8, R10, R13, R21, R26, R27, R28; ADR-0016, ADR-0018, ADR-0042, ADR-0043, ADR-0049 |
| Scale/Scope | 10.000 CCU (NFR-SCAL-003); `xp_ledger` partition khi > 50M dòng [Confirmed] |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta `reward-rule.schema.json`, payload event game, `activity.graded` ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ tiêu thụ event; không gọi module learning; Engine thuần domain; command (write context) và query (read context) tách (ADR-0087) |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng T/TA có `tenant_id` + RLS; ledger chỉ INSERT; test isolation consumer + API |
| IV Server-authoritative | ✅ | XP tính từ event server; unique ledger + `processed_events`; không nhận XP từ client |
| V Xác thực, phân quyền | ✅ | `game_player` cho API học sinh; config qua F078 (`game.progression.write`, `game.reward_rule.write`) |
| VI An toàn trẻ em | ✅ | Không streak/thưởng đăng nhập (NFR-SAFE-003); log không PII |
| VII Một nguồn sự thật, cấu hình | ✅ | Số XP/cap từ reward rule + CONFIG_DEFAULTS, không hardcode |
| VIII UI tách logic, token | ✅ | Không có UI |
| IX Test có bằng chứng | ✅ | Test mỗi F074-AC-*, test RLS, test race cap |

## Project Structure

### Documentation (feature này)

```text
specs/074-xp-level-reward-engine/
├── spec.md · plan.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
src/backend/modules/game/
├── ent/schema/        # xpledger.go, xpdailycap.go, rewardrule.go, rewardruleversion.go, rewardgrant.go,
│                      # progressioncurve.go, progressioncurveversion.go, progressionlevel.go, levelhistory.go
├── migrations/
├── internal/domain/reward/        # Rule, Engine (Evaluate), CapPolicy, RepeatPolicy, GradePolicy, Grant, GrantApplier
├── internal/domain/progression/   # Curve, LevelTable, Title, LevelUp (nhiều cấp), FloorProtection
├── internal/application/command/  # award_from_event.go, apply_grade_transition.go, rollout_curve.go, reconcile_xp.go
├── internal/application/query/    # get_progression.go, list_xp_history.go, list_progression_levels.go, get_xp_status.go
├── internal/application/port/     # XpLedgerRepository, RewardRuleReader, CurveReader, GrantApplierRegistry, RealtimePublisher
├── internal/infrastructure/persistence/   # xp_ledger_repo.go, reward_rule_reader.go (cache con trỏ), curve_repo.go
├── internal/infrastructure/gameconfig/    # progression_curve_module.go, reward_rule_module.go (validate + OnPublished)
├── internal/presentation/consumer/        # reward_engine.go, grade_transition.go
├── internal/presentation/http/public/     # progression.go
├── internal/presentation/job/             # curve_rollout.go, xp_reconcile.go
└── tests/{unit,integration,contract}/
```

**Structure Decision**: Hai package domain (`reward` dùng chung cho F075–F077, `progression` riêng level) để F075 chỉ thêm applier; consumer một durable cho mọi trigger để giữ thứ tự theo học sinh không cần thiết (idempotent theo ledger).

## Phase 0: Research

Không có — PROGRESSION_SYSTEM, ADR-0049 và DATABASE_DESIGN §2.8a/§2.13 đủ. Hai giả định [Confirmed] (ranh giới ngày cap, một curve) ghi ở spec Assumptions.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `reward.Engine.Evaluate(trigger, rules, history, caps, student) → []XpAward`; `progression.LevelTable.Apply(xpTotal, currentLevel)` | Thứ tự tính: điều kiện → XP gốc (`amount` hoặc `per_correct_answer × first_try_correct_count`) → hệ số khối (miễn khi `recommendation_id`/`assignment`) → hệ số lặp → cắt cap; làm tròn xuống |
| application | Command `AwardFromEvent`, `ApplyGradeTransition`, `RolloutCurve`, `ReconcileXp` · Query `GetProgression`, `ListXpHistory`, `ListProgressionLevels`, `GetXpStatus` | Query chỉ đọc, dùng replica được |
| infrastructure | ent repo + `SELECT … FOR UPDATE` trên `xp_daily_caps`; reader rule có cache Valkey `ew:game:current:_platform:reward_rule:*`; `GrantApplierRegistry` (applier `xp`, `title`) | |
| presentation | Handler `getProgression`, `listXpHistory`, `listProgressionLevels`, `getXpStatus` · consumer `game.reward_engine`, `game.grade_transition` · job `curve-rollout`, `xp-reconcile` (hằng đêm) | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F072 (quest event, `QuestProgress.ExpireForGradeChange`), F071 (world event, kernel config), F070 (`game_profiles`), F030/F031 (event `activity.graded`, `learning_objective.mastered`, `lesson.completed`), F010/F011 (`student.synced`, `enrollment.changed`).
- Event tiêu thụ: `activity.graded`, `lesson.completed`, `quest.completed`, `minigame.completed`, `learning_objective.mastered`, `world.discovery_found`, `world.area_completed`, `student.synced`, `enrollment.changed`, `content.published` (item_type `progression_curve`).
- Event phát: `xp.awarded`, `level.reached`, `reward.granted`. Realtime: `progression.xp.updated`, `progression.level.reached`.
- Không facade mới (R29: phản ứng phụ qua event).

### Bảo mật, tenant, idempotency, audit

- API học sinh: `game_player` + `tenant` middleware; `TenantScope` từ Principal; chỉ dữ liệu `student_id = principal.user_id`.
- Consumer: `TenantScope` từ envelope `tenant_id`, kiểm tra khớp `ref.students` (F074-AC-11).
- Idempotency: `infra.processed_events (consumer, event_id)` + UNIQUE `(student_id, source_type, source_id, rule_id)` + `reward_grants.idempotency_key`.
- Audit: thay đổi config do F078 ghi; cấp XP tự động không ghi audit từng bút toán (ledger là vết), chỉ `reconcile` lệch ghi `audit.recorded` mức cảnh báo.

### Quan sát

- Metric `eduworld_game_xp_awarded_total{source_code,capped}`, `eduworld_game_level_reached_total`, `eduworld_game_reward_engine_duration_seconds`, `eduworld_game_ledger_mismatch_total{ledger="xp"}`, `eduworld_game_consumer_dlq_total{consumer}`; span `reward.evaluate`, `reward.persist`; log không chứa nickname/tên.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
