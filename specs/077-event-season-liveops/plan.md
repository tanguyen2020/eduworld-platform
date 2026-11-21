# Implementation Plan: F077 — Event & Season live-ops: vòng đời sự kiện, community goal, season track, sự kiện trường

**Branch**: `077-event-season-liveops` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Thêm bounded context `liveops` (event, run, participation, community goal) và `season` vào module game. Module config `events`, `event-templates`, `season-tracks` (validate + `OnPublished` tạo `event_runs`). Worker job `event-scheduler` (khóa singleton `ew:lock:job:event-scheduler`, chạy mỗi 15 s) chuyển trạng thái run theo giờ server, phát event + dựng lại manifest tenant. Consumer `game.liveops_progress` nhận event nguồn (`quest.completed`, `activity.graded`, `world.discovery_found`, `boss.*`, `match.completed`…) → điểm sự kiện, đóng góp community goal, điểm mùa (mỗi loại một ledger append-only idempotent theo `source_event_id`). Phần thưởng mốc/cấp/community goal cấp qua Reward Engine (F074) với applier của F075. Consumer `game.season_projection` nhận `season.started/ended`. API tenant `/lms/school-events*` (route ngoại lệ tới module game) tạo sự kiện tenant từ template, kiểm văn bản qua facade `chat`.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen (server game + client `chat.internal`), `src/backend/pkg/modcall`, `src/backend/pkg/entx` (`UnitOfWork`), `src/backend/pkg/idempotency`, `src/backend/pkg/outbox`, `src/backend/pkg/clock`, Valkey client |
| Storage | PostgreSQL `eduworld_game`: `game.events`/`event_versions` G/T, `event_templates`/`event_template_versions` G, `event_runs` G/T, `event_participation` T, `event_point_ledger` TA P, `community_goals` G/T, `community_goal_contributions` TA P ngày, `community_goal_progress` T, `season_tracks`/`season_track_versions` G, `season_point_ledger` TA P, `season_track_progress` T, `ref.seasons`; Valkey `ew:game:cgoal:<goal_id>:<scope_id>`, `ew:lock:job:event-scheduler` |
| Testing | go test + testcontainers (PostgreSQL, NATS, Valkey); fake clock; facade test game ↔ chat |
| Target Platform | Linux container (`core-api` + `worker`, module `game`) |
| Project Type | backend module |
| Performance Goals | Chuyển trạng thái ±1 phút (AC-EVT-002); consumer ≥ 500 event/s/instance; dừng khẩn cấp hiệu lực ≤ 30 s (SC-002) |
| Constraints | AGENTS R5, R8, R9, R10, R15, R19, R20, R21, R26, R29; ADR-0027, ADR-0029, ADR-0043, ADR-0049, ADR-0056, ADR-0057 |
| Scale/Scope | Community goal scope `platform` gom mọi tenant: chỉ tổng hợp số đếm, không dữ liệu học sinh liên tenant |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta endpoint kiểm văn bản (chat internal), payload event live-ops, schema `achievement` category `event`, AsyncAPI flush ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Lịch mùa qua event; kiểm văn bản qua facade `chat`; scheduler là job gọi command |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng T/TA + RLS; sự kiện tenant `tenant_id NOT NULL`; goal `platform` chỉ trả tổng; test isolation |
| IV Server-authoritative | ✅ | Điểm/đóng góp chỉ từ event server; claim có `Idempotency-Key` + audit; vận hành có audit |
| V Xác thực, phân quyền | ✅ | `game_player`; `lms.school_event.write` (tenant); `game.event.operate`, `game.liveops.read` (platform, gateway nội bộ) |
| VI An toàn trẻ em | ✅ | Không streak/push/track trả phí; kiểm từ nhạy cảm, không trả từ khớp (R15); không xếp hạng cá nhân |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số theo CONFIG_DEFAULTS + version sự kiện |
| VIII UI tách logic, token | ✅ | Không có UI |
| IX Test có bằng chứng | ✅ | Test mỗi F077-AC-*, scheduler đa instance, Valkey mất |

## Project Structure

### Documentation (feature này)

```text
specs/077-event-season-liveops/
├── spec.md · plan.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
src/backend/modules/game/
├── ent/schema/        # event.go, eventversion.go, eventtemplate*.go, eventrun.go, eventparticipation.go, eventpointledger.go,
│                      # communitygoal.go, communitygoalcontribution.go, communitygoalprogress.go,
│                      # seasontrack.go, seasontrackversion.go, seasonpointledger.go, seasontrackprogress.go, refseason.go
├── internal/domain/liveops/       # EventRun (state machine), Audience, PointsPolicy, MilestonePolicy, RuleChangePolicy,
│                                  # CommunityGoal (contribution, milestone), TemplateParams (allowed_params), HealthRules
├── internal/domain/season/        # SeasonTrack, TierPolicy, AutoGrant
├── internal/application/command/  # transition_event_runs.go, operate_event_run.go, join_event.go, record_liveops_progress.go,
│                                  # claim_event_milestone.go, claim_season_tier.go, close_season.go, archive_event_run.go,
│                                  # create_school_event.go, update_school_event.go, cancel_school_event.go, recompute_goals.go
├── internal/application/query/    # list_game_events.go, get_game_event.go, get_community_goal.go, get_season_track.go,
│                                  # list_season_history.go, get_event_run_stats.go, list_event_templates.go, list_school_events.go
├── internal/application/port/     # TextScreeningClient, ManifestRebuilder, GoalCounterCache
├── internal/infrastructure/modules/chat.go          # adapter facade chat (kiểm văn bản)
├── internal/infrastructure/gameconfig/             # event_module.go, event_template_module.go, season_track_module.go
├── internal/infrastructure/cache/goal_counter.go   # Valkey INCRBY + rebuild
├── internal/presentation/http/public/              # events.go, season_track.go, school_events.go, event_ops_admin.go
├── internal/presentation/consumer/                 # liveops_progress.go, season_projection.go
├── internal/presentation/job/                      # event_scheduler.go, goal_recompute.go
└── tests/{unit,integration,contract}/
```

**Structure Decision**: Một consumer `game.liveops_progress` cho điểm sự kiện, đóng góp goal và điểm mùa vì cùng nguồn event và cùng khóa idempotent `source_event_id`; ba ledger riêng giữ nguồn sự thật tách biệt.

## Phase 0: Research

Không có — EVENT_SEASON_SYSTEM, DATABASE_DESIGN §2.8c, ADR-0043/0049 đủ. Điểm cần chốt ghi ở spec Clarifications/Assumptions.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `EventRun.Transition(now)`, `Suspend/Resume/Cancel(reason)`, `Audience.Includes(student)`, `PointsPolicy` (nguồn, trần ngày), `RuleChangePolicy` (hiệu lực `event_rule_change_effective`, không giảm), `CommunityGoal.Contribute`, `TemplateParams.Validate`, `HealthRules.Validate`, `SeasonTrack.TierFor(points)` | Thuần, Clock inject |
| application | Command như cây trên (permission: học sinh self; `game.event.operate`; `lms.school_event.write`) · Query như cây trên (`game.liveops.read` cho stats) | authz `authz.Require` tường minh đầu handler; idempotency middleware `src/backend/pkg/httpx` |
| infrastructure | ent repo; adapter facade `chat` (deadline 800 ms từ context, không fail-open); goal counter Valkey; manifest rebuilder (kernel F071/F078); module config | |
| presentation | `listGameEvents`, `getGameEvent`, `joinGameEvent`, `claimEventMilestone`, `getCommunityGoal`, `getSeasonTrack`, `claimSeasonTier`, `listSeasonTrackHistory`, `suspendEventRun`, `resumeEventRun`, `cancelEventRun`, `getEventRunStats`, `listEventTemplates`, `listSchoolEvents`, `createSchoolEvent`, `getSchoolEvent`, `updateSchoolEvent`, `cancelSchoolEvent` · consumer · job | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F075 (applier Xu/vật phẩm/huy hiệu), F074 (Reward Engine), F072 (quest sự kiện, event `quest.completed`), F071/F078 (kernel publish, manifest), F106 (`season.started/ended` — M4b), F116 (danh mục từ nhạy cảm của module chat).
- Facade: `chat` `screenText` (Cần thêm).
- Event phát: `event.scheduled/started/suspended/resumed/ended/cancelled`, `event.participation_started`, `event.milestone_claimed`, `community_goal.milestone_reached/completed`, `season_track.tier_reached`, `season_track.reward_claimed`, `audit.recorded`. Tiêu thụ: event nguồn điểm, `season.started/ended`, `content.published` (events).
- Realtime: `event.status.changed`, `event.progress.updated`, `event.community_goal.updated`, `season.track.updated`.

### Bảo mật, tenant, idempotency, audit

- Học sinh: `game_player`, lọc `audience` theo `ref.students` (`school_id`, `grade_level`; projection cập nhật từ `student.placement_changed` — ADR-0095, không `ref.enrollments`).
- Tenant: `portal_staff` + `lms.school_event.write`, scope trường; sự kiện tenant `owner_scope = tenant`, `tenant_id` của phiên; template chỉ khi có assignment.
- Platform: `/game-admin/events/*` gateway nội bộ, role platform, `game.event.operate`/`game.liveops.read`.
- `Idempotency-Key`: `joinGameEvent`, `claimEventMilestone`, `claimSeasonTier`, `suspend/resume/cancelEventRun`, `create/update/cancelSchoolEvent`. Audit: thao tác vận hành, sự kiện trường, claim, tự cấp.

### Quan sát

- Metric `eduworld_game_event_run_transition_total{to}`, `eduworld_game_event_scheduler_lag_seconds`, `eduworld_game_cgoal_contribution_total{result}`, `eduworld_game_cgoal_cache_rebuild_total`, `eduworld_game_season_points_total{capped}`, `eduworld_game_event_claim_total{kind}`.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
