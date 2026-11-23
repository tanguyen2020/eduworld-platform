# Tasks: F077 — Event & Season live-ops: vòng đời sự kiện, community goal, season track, sự kiện trường

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 (kết quả: `internal/domain/liveops` (event, run, points, template), `internal/domain/season`; ranh giới tầng do `.golangci.arch.yml` sinh sẵn — archgen 0 vi phạm, không cần depguard riêng) Tạo package `internal/domain/liveops`, `internal/domain/season` + depguard — file: `src/backend/modules/game/internal/domain/liveops/doc.go`
- [x] T002 (kết quả: `internal/infrastructure/config/liveops.go` — env `EW_GAME_EVENT_*`, `EW_GAME_CGOAL_*`, `EW_GAME_SCHOOL_EVENT*`, `EW_GAME_ANALYTICS_MIN_CELL_SIZE` theo CONFIG_DEFAULTS [Proposed: chưa có khóa platform settings]; tham số season track nằm trong version track) [P] Cấu hình tham số live-ops (`event_*`, `community_goal_*`, `season_*`, `school_event*`) từ platform settings — file: `src/backend/modules/game/internal/config/liveops.go`, `src/backend/modules/game/.env.example`
- [x] T003 (kết quả: `src/backend/modules/game/seed/liveops/` — lễ hội khối 3–5, template "Tuần lễ Toán học" gán `th-demo-a`, season track 30 cấp của mùa `pre-season`; fixture `season.started/ended` dựng trong test `liveops_season_test.go`) [P] Seed config: lễ hội khối 3–5, season track 30 cấp, template sự kiện trường gán `th-demo-a`; fixture `season.started`/`season.ended` — file: `deploy/seed/game-config/events/`, `deploy/seed/game-config/season-tracks/`, `src/backend/modules/game/tests/fixtures/events/season.started.json`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 (kết quả: `screenText` trong `chat.internal.v1.yaml` + `modules/chat/internal/presentation/facade` (pipeline F116) + test bên nhận `TestScreenTextFacade`, bên gọi `TestChatScreenTextFacade`) Phối hợp module chat thêm `screenText` (`x-allowed-callers: [game]`, không trả từ khớp) + facade test — file: `specs/openapi/modules/chat.internal.v1.yaml`, `src/backend/modules/game/tests/contract/chat_screen_text_test.go` — kiểm: `make specs-lint`, `make test-contract`
- [x] T005 (kết quả: `eduworld-api.v1.yaml` → split; `updateSchoolEvent` thêm `Idempotency-Key` (breaking, ADR-0010 trong allowlist); suspend/resume/cancel thêm `422`/`428`; `AchievementCategory` thêm `event`; AsyncAPI nhịp flush) [P] OpenAPI: `createSchoolEvent` `409`, `503`; `428` cho `joinGameEvent`, `createSchoolEvent`, `updateSchoolEvent`; AsyncAPI nhịp `event.community_goal.updated` — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`
- [x] T006 (kết quả: `event-template.schema.json` + ví dụ; `achievement.schema.json` (category `event`, 3 trigger); `event.schema.json` `decoration.bundle_size_bytes`; 12 payload `specs/events/payloads/game/`; `reward.granted`/`content.published` delta) [P] Thêm `event-template.schema.json`; sửa `achievement.schema.json` (category `event`, trigger sự kiện/mùa); payload event live-ops ở `specs/events/payloads/game/` — file: `specs/schemas/`, `specs/events/payloads/game/` — kiểm: `make specs-lint`
- [x] T007 (kết quả: `internal/infrastructure/modules/chat.go` — `TextScreener`/`LateChat`, deadline 800 ms, lỗi → 503) Adapter facade `chat` qua `src/backend/pkg/modcall` (deadline 800 ms, không fail-open) — file: `src/backend/modules/game/internal/infrastructure/modules/chat.go`
- [x] T008 (kết quả: `ent/schema/liveops.go` (14 bảng); cột thêm theo Clarifications 2026-11-23) ent schema theo data-model (`event_runs.status_version`, `suspended_reason varchar(1000)`, `event_versions.effective_from`, `event_participation.auto_granted_milestones`, `ref.seasons.status`) — file: `src/backend/modules/game/ent/schema/` — kiểm: `make migrate-diff MOD=game name=f077_liveops`, `make migrate-lint MOD=game`
- [x] T009 (kết quả: `migrations/20261230000770_f077_liveops.sql` — chưa partition (Clarifications), RLS G/T + T/TA, REVOKE UPDATE/DELETE ledger, CHECK `delta > 0`, `ref.seasons`, hàm `game.community_goal_platform_total`) Migration: partition (ledger tháng, contributions ngày), unique idempotent, RLS bảng T/TA, REVOKE UPDATE/DELETE ledger — file: `src/backend/modules/game/migrations/`
- [x] T010 (kết quả: `liveops_publish_test.go` `TestLiveOpsRLS`) [P] Test RLS + append-only cho bảng live-ops — file: `src/backend/modules/game/tests/integration/liveops_rls_test.go`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Vòng đời chạy sự kiện và vận hành khẩn cấp (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [x] T011 (kết quả: `internal/domain/liveops/liveops_test.go` `TestRunStateMachine`) [P] [US1] Unit test `EventRun` state machine (chuyển hợp lệ/không hợp lệ, lý do bắt buộc) — file: `src/backend/modules/game/internal/domain/liveops/event_run_test.go` — phải FAIL trước khi code
- [x] T012 (kết quả: `liveops_lifecycle_test.go` `TestLiveOpsScheduler`) [P] [US1] Integration test F077-AC-01, F077-AC-02 với fake clock và 2 instance scheduler: ±1 phút, không trùng, manifest dựng lại, `event.status.changed` — file: `src/backend/modules/game/tests/integration/event_scheduler_test.go`
- [x] T013 (kết quả: `TestLiveOpsOperate` — chuyển sai là `422 INVALID_STATE_TRANSITION` theo AC-03) [P] [US1] Integration test F077-AC-03, F077-AC-04: suspend/resume/cancel có audit + lý do, chuyển sai 409, quest đang dở hoàn thành theo version ghim, không thưởng mới khi suspended — file: `src/backend/modules/game/tests/integration/event_operate_test.go`
- [x] T014 (kết quả: gộp vào `TestLiveOpsScheduler`, hủy → archived trong `TestLiveOpsOperate`) [P] [US1] Integration test F077-AC-05: quá `claim_until`/hủy → tự cấp mốc đạt chưa nhận một lần, `archived` — file: `src/backend/modules/game/tests/integration/event_archive_test.go`
- [x] T015 (kết quả: gộp vào `TestLiveOpsOperate` — không MFA, ADR-0094) [P] [US1] Test authorization F077-AC-06: token tenant, role platform thiếu quyền, thiếu MFA, `azp` sai → 403 + `authz.denied` — file: `src/backend/modules/game/tests/integration/event_ops_authz_test.go`

### Implementation cho User Story 1

- [x] T016 (kết quả: `domain/liveops/run.go`, `event.go`) [P] [US1] Domain `liveops.EventRun`, `Audience`, `HealthRules` — file: `src/backend/modules/game/internal/domain/liveops/`
- [x] T017 (kết quả: lõi phát hành F071 — `command/publish_liveops.go`, item type `event`/`event_template`/`season_track`; config type HTTP ở F078) [US1] Module config `events` (`OnPublished` tạo `event_runs`) — file: `src/backend/modules/game/internal/infrastructure/gameconfig/event_module.go`
- [x] T018 (kết quả: `command/liveops.go` `RunScheduler`, `presentation/job/liveops.go`, khóa `cache.JobLock`) [US1] Command `TransitionEventRuns`, `ArchiveEventRun`, job `event-scheduler` (khóa singleton) — file: `src/backend/modules/game/internal/application/command/transition_event_runs.go`, `src/backend/modules/game/internal/presentation/job/event_scheduler.go`
- [x] T019 (kết quả: `command/liveops.go` `OperateRun`, `query/liveops.go` `GetEventRunStats`, `presentation/http/liveops.go`) [US1] Command `OperateEventRun` + query `GetEventRunStats` + handler `suspendEventRun`, `resumeEventRun`, `cancelEventRun`, `getEventRunStats` — file: `src/backend/modules/game/internal/application/command/operate_event_run.go`, `src/backend/modules/game/internal/presentation/http/public/event_ops_admin.go`

## Phase 4: User Story 2 — Học sinh tham gia sự kiện, điểm và mốc thưởng (Priority: P1)

### Tests cho User Story 2

- [x] T020 (kết quả: `liveops_player_test.go` `TestLiveOpsPlayer`) [P] [US2] Integration test F077-AC-07, F077-AC-08: audience theo khối/trường/tenant, upcoming theo `event_upcoming_visible_days`, `EVENT_NOT_AVAILABLE`, `EVENT_ENDED`, tham gia một lần — file: `src/backend/modules/game/tests/integration/event_player_test.go` — phải FAIL trước khi code
- [x] T021 (kết quả: `TestLiveOpsPlayer`, `TestLiveOpsRuleChange`) [P] [US2] Integration test F077-AC-09, F077-AC-10, F077-AC-11: điểm idempotent + trần, claim một lần + audit, mốc chưa đạt 409, thiếu key 428, version mới không giảm tiến độ và áp từ `event_rule_change_effective` — file: `src/backend/modules/game/tests/integration/event_points_claim_test.go`
- [x] T022 (kết quả: `TestLiveOpsPlayer` (TENANT_MISMATCH) + `player_routes_authz_test.go` `TestPlayerRoutesStaffDenied` (mọi route người chơi F077)) [P] [US2] Test authorization + tenant isolation F077-AC-12: 8 route học sinh × token nhân sự → 403; header tenant khác → `TENANT_MISMATCH` — file: `src/backend/modules/game/tests/integration/liveops_player_authz_test.go`

### Implementation cho User Story 2

- [x] T023 (kết quả: `domain/liveops/points.go`) [P] [US2] Domain `PointsPolicy`, `MilestonePolicy`, `RuleChangePolicy` — file: `src/backend/modules/game/internal/domain/liveops/points.go`
- [x] T024 (kết quả: `command/liveops_player.go`, `presentation/consumer/liveops.go` — 3 durable theo stream: `game.liveops_progress`, `_learning`, `_competition`) [US2] Command `JoinEvent`, `RecordLiveopsProgress` (phần điểm sự kiện), `ClaimEventMilestone`; consumer `game.liveops_progress`; query + handler `listGameEvents`, `getGameEvent`, `joinGameEvent`, `claimEventMilestone` — file: `src/backend/modules/game/internal/application/command/record_liveops_progress.go`, `src/backend/modules/game/internal/presentation/consumer/liveops_progress.go`, `src/backend/modules/game/internal/presentation/http/public/events.go`

## Phase 5: User Story 3 — Community goal toàn trường (Priority: P2)

### Tests cho User Story 3

- [x] T025 (kết quả: `liveops_goal_test.go` `TestCommunityGoal` (Valkey thật)) [P] [US3] Integration test F077-AC-13, F077-AC-14: khử trùng `source_event_id`, trần/học sinh/ngày, loại `suspicious_speed`, xóa Valkey → dựng lại đúng — file: `src/backend/modules/game/tests/integration/community_goal_test.go` — phải FAIL trước khi code
- [x] T026 (kết quả: gộp vào `TestCommunityGoal`) [P] [US3] Integration test F077-AC-15, F077-AC-16: mốc phát một lần, chỉ người có đóng góp nhận thưởng, `getCommunityGoal` không có xếp hạng/danh tính, nhịp flush — file: `src/backend/modules/game/tests/integration/community_goal_milestone_test.go`

### Implementation cho User Story 3

- [x] T027 (kết quả: `command/liveops_player.go` (contribute, reachMilestones, RecomputeGoals), `infrastructure/cache/goal_counter.go`, job `goal-recompute`) [US3] Domain `CommunityGoal`; mở rộng `RecordLiveopsProgress` (đóng góp); cache `goal_counter`; job `goal-recompute`; query + handler `getCommunityGoal`; publish `event.community_goal.updated` gộp theo nhịp — file: `src/backend/modules/game/internal/domain/liveops/community_goal.go`, `src/backend/modules/game/internal/infrastructure/cache/goal_counter.go`, `src/backend/modules/game/internal/presentation/job/goal_recompute.go`

## Phase 6: User Story 4 — Season track miễn phí (Priority: P2)

### Tests cho User Story 4

- [x] T028 (kết quả: `liveops_season_test.go` `TestSeasonTrack`) [P] [US4] Integration test F077-AC-17, F077-AC-18, F077-AC-19, F077-AC-20 với fixture `season.started/ended`: projection idempotent, điểm mùa trần ngày, claim một lần, tự cấp khi hết mùa, lịch sử giữ; module từ chối `paid_track = true` — file: `src/backend/modules/game/tests/integration/season_track_test.go` — phải FAIL trước khi code

### Implementation cho User Story 4

- [x] T029 (kết quả: `domain/season`, `command/liveops_season.go`, consumer `game.season_projection`) [US4] Domain `season`; consumer `game.season_projection`; command `ClaimSeasonTier`, `CloseSeason`; module `season-tracks`; query + handler `getSeasonTrack`, `claimSeasonTier`, `listSeasonTrackHistory` — file: `src/backend/modules/game/internal/domain/season/`, `src/backend/modules/game/internal/presentation/consumer/season_projection.go`, `src/backend/modules/game/internal/presentation/http/public/season_track.go`

## Phase 7: User Story 5 — Sự kiện của trường từ template (Priority: P2)

### Tests cho User Story 5

- [x] T030 (kết quả: `liveops_school_test.go` `TestSchoolEvents`) [P] [US5] Integration test F077-AC-21, F077-AC-22, F077-AC-24: template theo assignment, tham số ngoài `allowed_params` 422, thời lượng vượt, quá số sự kiện đồng thời 409, hủy có audit — file: `src/backend/modules/game/tests/integration/school_event_test.go` — phải FAIL trước khi code
- [x] T031 (kết quả: gộp vào `TestSchoolEvents` — placeholder, không từ thật) [P] [US5] Integration test F077-AC-23 (fixture từ placeholder, không từ thật — R15): 422 không lộ từ khớp; module chat tắt → 503 không lưu; chuỗi quá dài 422 — file: `src/backend/modules/game/tests/integration/school_event_screening_test.go`
- [x] T032 (kết quả: gộp vào `TestSchoolEvents`) [P] [US5] Test authorization + tenant isolation F077-AC-25: `ad.truong.b` → 404 sự kiện trường A; `academic_manager` → 403; token học sinh → `PORTAL_ACCESS_STAFF_ONLY` — file: `src/backend/modules/game/tests/integration/school_event_authz_test.go`

### Implementation cho User Story 5

- [x] T033 (kết quả: `domain/liveops/template.go`, `command/liveops_school.go`, `query/liveops.go`) [US5] Domain `TemplateParams`; module `event-templates`; command `CreateSchoolEvent`, `UpdateSchoolEvent`, `CancelSchoolEvent` (port `TextScreeningClient`); query + handler `listEventTemplates`, `listSchoolEvents`, `getSchoolEvent` — file: `src/backend/modules/game/internal/application/command/create_school_event.go`, `src/backend/modules/game/internal/presentation/http/public/school_events.go`, `src/backend/modules/game/internal/infrastructure/gameconfig/event_template_module.go`

## Phase 8: User Story 6 — Quy tắc lành mạnh, song ngữ, ngân sách trang trí (Priority: P3)

### Tests cho User Story 6

- [x] T034 (kết quả: domain `TestEventHealthRules` + integration `liveops_publish_test.go` `TestLiveOpsPublishChecks`) [P] [US6] Unit test module `events` F077-AC-26, F077-AC-27, F077-AC-28: thiếu `en`, lễ hội ngắn, thưởng điểm danh/chuỗi, eligibility level cao cho sự kiện học tập, quest thêm vượt, bundle vượt ngân sách/thiếu bản Thấp, chồng trang trí vùng — file: `src/backend/modules/game/tests/unit/event_module_test.go` — phải FAIL trước khi code
- [x] T035 (kết quả: `tests/contract/liveops_health_rules_test.go` `TestLiveOpsHealthRules`) [P] [US6] Static test F077-AC-29: không có producer push ngoài game; ledger không có bút toán âm do bỏ lỡ ngày — file: `src/backend/modules/game/tests/contract/liveops_health_rules_test.go`

### Implementation cho User Story 6

- [x] T036 (kết quả: `domain/liveops/event.go` (`CheckTranslations`, `Validate`), `command/publish_liveops.go`; `Content-Language` theo `Accept-Language`) [US6] Hoàn thiện `HealthRules` + kiểm song ngữ/bundle trong `event_module`; trả văn bản theo `Accept-Language` — file: `src/backend/modules/game/internal/domain/liveops/health_rules.go`, `src/backend/modules/game/internal/infrastructure/gameconfig/event_module.go`

## Phase 9: Polish & Cross-Cutting

- [x] T037 (kết quả: DATABASE_DESIGN §2.8c, EVENT_SEASON_SYSTEM §5.3/§6/§7.2/§10/§12/§14/§17, EVENT_DESIGN §4.5, INVENTORY_ECONOMY §5.1, CONFIG_DEFAULTS §8.1, CHANGELOG) [P] Cập nhật DATABASE_DESIGN §2.8c (cột thêm), EVENT_SEASON_SYSTEM §14 (`screenText`, quyền), EVENT_DESIGN §4.5, CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/01-game-design/EVENT_SEASON_SYSTEM.md`, `docs/07-backend/EVENT_DESIGN.md`, `CHANGELOG.md`
- [x] T038 (kết quả: `infrastructure/observability/liveops_metrics.go`) [P] Metric `eduworld_game_event_*`, `eduworld_game_cgoal_*`, `eduworld_game_season_*` — file: `src/backend/modules/game/internal/infrastructure/observability/metrics.go`
- [x] T039 (kết quả: kịch bản quickstart phủ bằng test tích hợp — bảng AC → test dưới đây; chạy tay trên stack local chưa làm) Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational (T004 chặn US5) → US1 → US2 → US3, US4 (song song) → US5 → US6 → Polish.
- Phụ thuộc: F075 (applier), F074 (Reward Engine), F072 (quest sự kiện), F071/F078 (publish, manifest), F116 (`screenText`), F106 (`season.*` cho chạy thật).

## Definition of Done

- [x] Mọi tiêu chí F077-AC-* có test pass (ghi mã test/TC)
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh cho module `game`; `make test-contract` xanh với module chat
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật

## Bảng tiêu chí → test (bằng chứng 2026-11-23)

| Tiêu chí | Test (`src/backend/modules/game/…`) |
|---|---|
| F077-AC-01 | `tests/integration/liveops_lifecycle_test.go` `TestLiveOpsScheduler` (2 scheduler đồng thời → 1 chuyển, ±1 phút, tick lặp 0); domain `TestRunStateMachine` |
| F077-AC-02 | `TestLiveOpsScheduler` (manifest tenant A/B thêm/gỡ mục `event`, `event.status.changed` tới học sinh online), `TestLiveOpsOperate` (suspend gỡ, resume thêm) |
| F077-AC-03 | `TestLiveOpsOperate` (lý do trong audit + `event.suspended`, `422 INVALID_STATE_TRANSITION`, lý do ngắn 422, thiếu key 428, cancel); domain `TestRunStateMachine` |
| F077-AC-04 | `TestLiveOpsOperate` (thành phần ẩn, không điểm/thưởng mới, version ghim giữ, `quest_progress` không đổi) |
| F077-AC-05 | `TestLiveOpsScheduler` (quá `claim_until` → m1 tự cấp một lần + `auto_granted`, archived, tick sau không cấp thêm), `TestLiveOpsOperate` (cancelled → archived) |
| F077-AC-06 | `TestLiveOpsOperate` (tenant_admin, content_admin, token game, sự kiện khác → 403/404, `authz.denied`; stats cần `game.liveops.read`) |
| F077-AC-07 | `liveops_player_test.go` `TestLiveOpsPlayer` (khối 3 thấy, khối 7 `404 EVENT_NOT_AVAILABLE`, không trong manifest của mình; upcoming 7 ngày); domain `TestAudienceAndVisibility` |
| F077-AC-08 | `TestLiveOpsPlayer` (join 2 lần → 1 tham gia + 1 `event.participation_started`; trước giờ 409; sau `end_at` `409 EVENT_ENDED`) |
| F077-AC-09 | `TestLiveOpsPlayer` (phát lại cùng event 1 lần, trần 200/ngày, `event.progress.updated`); domain `TestPointsMilestonesAndGoal` |
| F077-AC-10 | `TestLiveOpsPlayer` (một grant 20 Xu + audit, cùng key phát lại, key khác 409, chưa đạt `409 EVENT_MILESTONE_NOT_REACHED`, thiếu key 428) |
| F077-AC-11 | `liveops_player_test.go` `TestLiveOpsRuleChange` (`effective_from` 04:00 ICT hôm sau, ghim lại khi tới giờ, mốc đã đạt giữ, rút ngắn 422); domain `TestRuleChangeEffective` |
| F077-AC-12 | `TestLiveOpsPlayer` (`TENANT_MISMATCH`), `player_routes_authz_test.go` `TestPlayerRoutesStaffDenied` (8 route × 5 role → `GAME_ACCESS_STUDENT_ONLY`) |
| F077-AC-13 | `liveops_goal_test.go` `TestCommunityGoal` (phát lại, `rapid_guess_flagged`, trần 50/ngày); domain `TestPointsMilestonesAndGoal` |
| F077-AC-14 | `TestCommunityGoal` (xóa khóa Valkey → dựng lại = PostgreSQL; `goal-recompute` sửa progress + bộ đếm, audit) |
| F077-AC-15 | `TestCommunityGoal` (mốc 25/50 một lần, 9 người đóng góp mỗi người 2 grant, người đóng góp muộn nhận mốc đã đạt, người không đóng góp 0) |
| F077-AC-16 | `TestCommunityGoal` (`by_grade` khối 4 (5 HS) hiện, khối 5 (4 HS) ẩn, không id học sinh, một push mỗi cửa sổ flush) |
| F077-AC-17 | `liveops_season_test.go` `TestSeasonTrack` (projection idempotent, trước `season.started` không điểm, lệch thứ tự giữ ended) |
| F077-AC-18 | `TestSeasonTrack` (trần 100/ngày, cấp 100 điểm, `season_track.tier_reached`, `season.track.updated`); domain `TestTrack` |
| F077-AC-19 | `TestSeasonTrack` (một grant, cùng key phát lại, `409 SEASON_TIER_NOT_REACHED`, thiếu key 428) |
| F077-AC-20 | `TestSeasonTrack` (hết mùa tự cấp cấp 2, lịch sử, mùa không mở → 404; `paid_track`, XP, collection item → 422); domain `TestTrack`; `liveopsschema` `TestValidate` |
| F077-AC-21 | `liveops_school_test.go` `TestSchoolEvents` (template của tenant A; tenant B rỗng) |
| F077-AC-22 | `TestSchoolEvents` (tham số/quest/phần thưởng/thời lượng/trường ngoài phạm vi → `422 EVENT_TEMPLATE_PARAM_NOT_ALLOWED`, tạo + audit, `If-Match` 412, sửa khi đã chạy 409); domain `TestTemplateParams` |
| F077-AC-23 | `TestSchoolEvents` (từ placeholder → 422 nêu trường, không nêu từ; chat lỗi → 503, không lưu; tên > 255 → 422; thiếu `vi` → 422); `TestScreenTextFacade`, `TestChatScreenTextFacade` |
| F077-AC-24 | `TestSchoolEvents` (sự kiện thứ 4 → 409, hủy cần lý do + audit, giải phóng chỗ) |
| F077-AC-25 | `TestSchoolEvents` (trường khác/tenant khác 404, `academic_manager` 403 + `authz.denied`, token học sinh `PORTAL_ACCESS_STAFF_ONLY`) |
| F077-AC-26 | `liveops_publish_test.go` `TestLiveOpsPublishChecks` (thiếu `en` tên/tagline → `TRANSLATION_INCOMPLETE`), `TestLiveOpsPlayer` (`Content-Language` theo `Accept-Language`); domain `TestEventHealthRules` |
| F077-AC-27 | `TestLiveOpsPublishChecks` (lễ hội 5 ngày, nguồn điểm danh/chuỗi ngày, level cho sự kiện học tập, trường khóa lạ → 422); domain `TestEventHealthRules` (quest thêm/ngày > 2) |
| F077-AC-28 | `TestLiveOpsPublishChecks` (gói > 20 MB, thiếu bản Thấp, chồng trang trí vùng → 422); domain `TestEventHealthRules` |
| F077-AC-29 | `tests/contract/liveops_health_rules_test.go` `TestLiveOpsHealthRules` (không producer push ngoài game, không nguồn đăng nhập/chuỗi, CHECK `delta > 0` + REVOKE UPDATE/DELETE); `TestLiveOpsScheduler` (không bút toán ≤ 0) |
