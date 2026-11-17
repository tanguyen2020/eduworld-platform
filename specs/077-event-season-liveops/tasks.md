# Tasks: F077 — Event & Season live-ops: vòng đời sự kiện, community goal, season track, sự kiện trường

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Tạo package `internal/domain/liveops`, `internal/domain/season` + depguard — file: `src/backend/modules/game/internal/domain/liveops/doc.go`
- [ ] T002 [P] Cấu hình tham số live-ops (`event_*`, `community_goal_*`, `season_*`, `school_event*`) từ platform settings — file: `src/backend/modules/game/internal/config/liveops.go`, `src/backend/modules/game/.env.example`
- [ ] T003 [P] Seed config: lễ hội khối 3–5, season track 30 cấp, template sự kiện trường gán `th-demo-a`; fixture `season.started`/`season.ended` — file: `deploy/seed/game-config/events/`, `deploy/seed/game-config/season-tracks/`, `src/backend/modules/game/tests/fixtures/events/season.started.json`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Phối hợp module chat thêm `screenText` (`x-allowed-callers: [game]`, không trả từ khớp) + facade test — file: `specs/openapi/modules/chat.internal.v1.yaml`, `src/backend/modules/game/tests/contract/chat_screen_text_test.go` — kiểm: `make specs-lint`, `make test-contract`
- [ ] T005 [P] OpenAPI: `createSchoolEvent` `409`, `503`; `428` cho `joinGameEvent`, `createSchoolEvent`, `updateSchoolEvent`; AsyncAPI nhịp `event.community_goal.updated` — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`
- [ ] T006 [P] Thêm `event-template.schema.json`; sửa `achievement.schema.json` (category `event`, trigger sự kiện/mùa); payload event live-ops ở `specs/events/payloads/game/` — file: `specs/schemas/`, `specs/events/payloads/game/` — kiểm: `make specs-lint`
- [ ] T007 Adapter facade `chat` qua `src/backend/pkg/modcall` (deadline 800 ms, không fail-open) — file: `src/backend/modules/game/internal/infrastructure/modules/chat.go`
- [ ] T008 ent schema theo data-model (`event_runs.status_version`, `suspended_reason varchar(1000)`, `event_versions.effective_from`, `event_participation.auto_granted_milestones`, `ref.seasons.status`) — file: `src/backend/modules/game/ent/schema/` — kiểm: `make migrate-diff MOD=game name=f077_liveops`, `make migrate-lint MOD=game`
- [ ] T009 Migration: partition (ledger tháng, contributions ngày), unique idempotent, RLS bảng T/TA, REVOKE UPDATE/DELETE ledger — file: `src/backend/modules/game/migrations/`
- [ ] T010 [P] Test RLS + append-only cho bảng live-ops — file: `src/backend/modules/game/tests/integration/liveops_rls_test.go`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Vòng đời chạy sự kiện và vận hành khẩn cấp (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [ ] T011 [P] [US1] Unit test `EventRun` state machine (chuyển hợp lệ/không hợp lệ, lý do bắt buộc) — file: `src/backend/modules/game/internal/domain/liveops/event_run_test.go` — phải FAIL trước khi code
- [ ] T012 [P] [US1] Integration test F077-AC-01, F077-AC-02 với fake clock và 2 instance scheduler: ±1 phút, không trùng, manifest dựng lại, `event.status.changed` — file: `src/backend/modules/game/tests/integration/event_scheduler_test.go`
- [ ] T013 [P] [US1] Integration test F077-AC-03, F077-AC-04: suspend/resume/cancel có audit + lý do, chuyển sai 409, quest đang dở hoàn thành theo version ghim, không thưởng mới khi suspended — file: `src/backend/modules/game/tests/integration/event_operate_test.go`
- [ ] T014 [P] [US1] Integration test F077-AC-05: quá `claim_until`/hủy → tự cấp mốc đạt chưa nhận một lần, `archived` — file: `src/backend/modules/game/tests/integration/event_archive_test.go`
- [ ] T015 [P] [US1] Test authorization F077-AC-06: token tenant, role platform thiếu quyền, thiếu MFA, `azp` sai → 403 + `authz.denied` — file: `src/backend/modules/game/tests/integration/event_ops_authz_test.go`

### Implementation cho User Story 1

- [ ] T016 [P] [US1] Domain `liveops.EventRun`, `Audience`, `HealthRules` — file: `src/backend/modules/game/internal/domain/liveops/`
- [ ] T017 [US1] Module config `events` (`OnPublished` tạo `event_runs`) — file: `src/backend/modules/game/internal/infrastructure/gameconfig/event_module.go`
- [ ] T018 [US1] Command `TransitionEventRuns`, `ArchiveEventRun`, job `event-scheduler` (khóa singleton) — file: `src/backend/modules/game/internal/application/command/transition_event_runs.go`, `src/backend/modules/game/internal/presentation/job/event_scheduler.go`
- [ ] T019 [US1] Command `OperateEventRun` + query `GetEventRunStats` + handler `suspendEventRun`, `resumeEventRun`, `cancelEventRun`, `getEventRunStats` — file: `src/backend/modules/game/internal/application/command/operate_event_run.go`, `src/backend/modules/game/internal/presentation/http/public/event_ops_admin.go`

## Phase 4: User Story 2 — Học sinh tham gia sự kiện, điểm và mốc thưởng (Priority: P1)

### Tests cho User Story 2

- [ ] T020 [P] [US2] Integration test F077-AC-07, F077-AC-08: audience theo khối/trường/tenant, upcoming theo `event_upcoming_visible_days`, `EVENT_NOT_AVAILABLE`, `EVENT_ENDED`, tham gia một lần — file: `src/backend/modules/game/tests/integration/event_player_test.go` — phải FAIL trước khi code
- [ ] T021 [P] [US2] Integration test F077-AC-09, F077-AC-10, F077-AC-11: điểm idempotent + trần, claim một lần + audit, mốc chưa đạt 409, thiếu key 428, version mới không giảm tiến độ và áp từ `event_rule_change_effective` — file: `src/backend/modules/game/tests/integration/event_points_claim_test.go`
- [ ] T022 [P] [US2] Test authorization + tenant isolation F077-AC-12: 8 route học sinh × token nhân sự → 403; header tenant khác → `TENANT_MISMATCH` — file: `src/backend/modules/game/tests/integration/liveops_player_authz_test.go`

### Implementation cho User Story 2

- [ ] T023 [P] [US2] Domain `PointsPolicy`, `MilestonePolicy`, `RuleChangePolicy` — file: `src/backend/modules/game/internal/domain/liveops/points.go`
- [ ] T024 [US2] Command `JoinEvent`, `RecordLiveopsProgress` (phần điểm sự kiện), `ClaimEventMilestone`; consumer `game.liveops_progress`; query + handler `listGameEvents`, `getGameEvent`, `joinGameEvent`, `claimEventMilestone` — file: `src/backend/modules/game/internal/application/command/record_liveops_progress.go`, `src/backend/modules/game/internal/presentation/consumer/liveops_progress.go`, `src/backend/modules/game/internal/presentation/http/public/events.go`

## Phase 5: User Story 3 — Community goal toàn trường (Priority: P2)

### Tests cho User Story 3

- [ ] T025 [P] [US3] Integration test F077-AC-13, F077-AC-14: khử trùng `source_event_id`, trần/học sinh/ngày, loại `suspicious_speed`, xóa Valkey → dựng lại đúng — file: `src/backend/modules/game/tests/integration/community_goal_test.go` — phải FAIL trước khi code
- [ ] T026 [P] [US3] Integration test F077-AC-15, F077-AC-16: mốc phát một lần, chỉ người có đóng góp nhận thưởng, `getCommunityGoal` không có xếp hạng/danh tính, nhịp flush — file: `src/backend/modules/game/tests/integration/community_goal_milestone_test.go`

### Implementation cho User Story 3

- [ ] T027 [US3] Domain `CommunityGoal`; mở rộng `RecordLiveopsProgress` (đóng góp); cache `goal_counter`; job `goal-recompute`; query + handler `getCommunityGoal`; publish `event.community_goal.updated` gộp theo nhịp — file: `src/backend/modules/game/internal/domain/liveops/community_goal.go`, `src/backend/modules/game/internal/infrastructure/cache/goal_counter.go`, `src/backend/modules/game/internal/presentation/job/goal_recompute.go`

## Phase 6: User Story 4 — Season track miễn phí (Priority: P2)

### Tests cho User Story 4

- [ ] T028 [P] [US4] Integration test F077-AC-17, F077-AC-18, F077-AC-19, F077-AC-20 với fixture `season.started/ended`: projection idempotent, điểm mùa trần ngày, claim một lần, tự cấp khi hết mùa, lịch sử giữ; module từ chối `paid_track = true` — file: `src/backend/modules/game/tests/integration/season_track_test.go` — phải FAIL trước khi code

### Implementation cho User Story 4

- [ ] T029 [US4] Domain `season`; consumer `game.season_projection`; command `ClaimSeasonTier`, `CloseSeason`; module `season-tracks`; query + handler `getSeasonTrack`, `claimSeasonTier`, `listSeasonTrackHistory` — file: `src/backend/modules/game/internal/domain/season/`, `src/backend/modules/game/internal/presentation/consumer/season_projection.go`, `src/backend/modules/game/internal/presentation/http/public/season_track.go`

## Phase 7: User Story 5 — Sự kiện của trường từ template (Priority: P2)

### Tests cho User Story 5

- [ ] T030 [P] [US5] Integration test F077-AC-21, F077-AC-22, F077-AC-24: template theo assignment, tham số ngoài `allowed_params` 422, thời lượng vượt, quá số sự kiện đồng thời 409, hủy có audit — file: `src/backend/modules/game/tests/integration/school_event_test.go` — phải FAIL trước khi code
- [ ] T031 [P] [US5] Integration test F077-AC-23 (fixture từ placeholder, không từ thật — R15): 422 không lộ từ khớp; module chat tắt → 503 không lưu; chuỗi quá dài 422 — file: `src/backend/modules/game/tests/integration/school_event_screening_test.go`
- [ ] T032 [P] [US5] Test authorization + tenant isolation F077-AC-25: `ad.truong.b` → 404 sự kiện trường A; `academic_manager` → 403; token học sinh → `PORTAL_ACCESS_STAFF_ONLY` — file: `src/backend/modules/game/tests/integration/school_event_authz_test.go`

### Implementation cho User Story 5

- [ ] T033 [US5] Domain `TemplateParams`; module `event-templates`; command `CreateSchoolEvent`, `UpdateSchoolEvent`, `CancelSchoolEvent` (port `TextScreeningClient`); query + handler `listEventTemplates`, `listSchoolEvents`, `getSchoolEvent` — file: `src/backend/modules/game/internal/application/command/create_school_event.go`, `src/backend/modules/game/internal/presentation/http/public/school_events.go`, `src/backend/modules/game/internal/infrastructure/gameconfig/event_template_module.go`

## Phase 8: User Story 6 — Quy tắc lành mạnh, song ngữ, ngân sách trang trí (Priority: P3)

### Tests cho User Story 6

- [ ] T034 [P] [US6] Unit test module `events` F077-AC-26, F077-AC-27, F077-AC-28: thiếu `en`, lễ hội ngắn, thưởng điểm danh/chuỗi, eligibility level cao cho sự kiện học tập, quest thêm vượt, bundle vượt ngân sách/thiếu bản Thấp, chồng trang trí vùng — file: `src/backend/modules/game/tests/unit/event_module_test.go` — phải FAIL trước khi code
- [ ] T035 [P] [US6] Static test F077-AC-29: không có producer push ngoài game; ledger không có bút toán âm do bỏ lỡ ngày — file: `src/backend/modules/game/tests/contract/liveops_health_rules_test.go`

### Implementation cho User Story 6

- [ ] T036 [US6] Hoàn thiện `HealthRules` + kiểm song ngữ/bundle trong `event_module`; trả văn bản theo `Accept-Language` — file: `src/backend/modules/game/internal/domain/liveops/health_rules.go`, `src/backend/modules/game/internal/infrastructure/gameconfig/event_module.go`

## Phase 9: Polish & Cross-Cutting

- [ ] T037 [P] Cập nhật DATABASE_DESIGN §2.8c (cột thêm), EVENT_SEASON_SYSTEM §14 (`screenText`, quyền), EVENT_DESIGN §4.5, CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/01-game-design/EVENT_SEASON_SYSTEM.md`, `docs/07-backend/EVENT_DESIGN.md`, `CHANGELOG.md`
- [ ] T038 [P] Metric `eduworld_game_event_*`, `eduworld_game_cgoal_*`, `eduworld_game_season_*` — file: `src/backend/modules/game/internal/infrastructure/observability/metrics.go`
- [ ] T039 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational (T004 chặn US5) → US1 → US2 → US3, US4 (song song) → US5 → US6 → Polish.
- Phụ thuộc: F075 (applier), F074 (Reward Engine), F072 (quest sự kiện), F071/F078 (publish, manifest), F116 (`screenText`), F106 (`season.*` cho chạy thật).

## Definition of Done

- [ ] Mọi tiêu chí F077-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh cho module `game`; `make test-contract` xanh với module chat
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
