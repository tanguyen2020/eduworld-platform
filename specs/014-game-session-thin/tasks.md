# Tasks: F014 — Game session lát mỏng: một thiết bị, login events, client-config, manifest tối thiểu

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Skeleton module game (`cmd/api`, `cmd/worker`, `cmd/migrate`, cấu hình `pkg/config`, health, outbox relay, `.golangci.yml` depguard) — file: `modules/game/`
- [ ] T002 [P] Adapter facade `identity` qua `pkg/modcall` (deadline từ context, Principal gốc, ánh xạ lỗi; cache Principal 60 s; không token, không retry/circuit breaker — ADR-0074) — file: `modules/game/internal/infrastructure/modules/identity.go`
- [ ] T003 [P] Cấu hình `EW_GAME_MIN_CLIENT_VERSION`, `EW_GAME_LATEST_CLIENT_VERSION`, ngưỡng timeout heartbeat, cờ bảo trì, khóa HMAC IP (secret file) — file: `modules/game/internal/config/session.go`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Thêm payload schema `student.game_logged_in`, `student.game_logged_out`, `game_session.started`, `game_session.ended`, `game_session.replaced` (có `old_keycloak_sid`, `old_device_id` — ADR-0086) — file: `specs/events/payloads/game/` — kiểm: `make specs-lint`
- [ ] T005 [P] Bổ sung response `426`, `503` cho `createGameSession`, `403` cho `endGameSession`/`heartbeatGameSession` ở spec gộp rồi chạy `node scripts/openapi/split-public.mjs` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=game`
- [ ] T006 ent schema `GameSession`, `LoginEvent` (AppendOnly), `ContentManifest`, `RefTenant`, `RefStudent` với `MaxLen` — file: `modules/game/ent/schema/` — kiểm: `make migrate-diff MOD=game name=f014_session`, `make migrate-lint MOD=game`
- [ ] T007 Migration: unique partial một session active, index heartbeat, partition tháng `login_events`, RLS, `REVOKE DELETE, TRUNCATE`, `infra.*` — file: `modules/game/migrations/`
- [ ] T008 [P] Consumer projection `ref.tenants`, `ref.students` (`tenant.*`, `student.synced`, `user.status_changed`, `student.cleaned_up`, `student.restored`; `source_version`) + test bắt kịp (TC-SVC-008) — file: `modules/game/internal/presentation/consumer/projection_identity.go`
- [ ] T009 [P] Test RLS bảng T/TA của module game (tenant A không đọc B; không set → 0 dòng; DELETE bị từ chối) — file: `modules/game/tests/integration/rls_test.go`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Tạo game session, một thiết bị mỗi tài khoản (Priority: P1) 🎯 MVP

**Goal**: Một session active mỗi học sinh, takeover có xác nhận.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [ ] T010 [P] [US1] Test F014-AC-01, F014-AC-02, F014-AC-08: tạo session đủ trường; nhân sự/học sinh không hoạt động → `GAME_ACCESS_STUDENT_ONLY` + audit, không login event; header tenant khác → `TENANT_MISMATCH` — file: `modules/game/tests/integration/start_session_test.go` — phải FAIL trước khi code
- [ ] T011 [P] [US1] Test F014-AC-03, F014-AC-04, F014-AC-06: `409` + `other_device` không `device_id`; takeover (key mới, không mật khẩu) kết thúc session cũ, phát `game_session.replaced`, publish Valkey; timeout cũ → không hỏi; cùng thiết bị → `logout` — file: `modules/game/tests/integration/takeover_test.go`
- [ ] T012 [P] [US1] Test F014-AC-05: 100 lượt hai thiết bị đồng thời → đúng một thành công — file: `modules/game/tests/integration/concurrent_start_test.go`
- [ ] T013 [P] [US1] Test F014-AC-07: thiếu `Idempotency-Key` → 428; cùng key + body → cùng kết quả; cùng key + `takeover: true` → 409 `IDEMPOTENCY_KEY_REUSED` — file: `modules/game/tests/integration/start_session_idempotency_test.go`
- [ ] T014 [P] [US1] Test identity consumer F014-AC-04: `game_session.replaced` → Admin API xóa session `old_keycloak_sid` và `GameAuthSessionRevoker` (F013) đánh dấu phiên game của `old_device_id` `revoked_at` (`takeover`); refresh/resume cũ → `401 REFRESH_TOKEN_INVALID` (Keycloak testcontainer, TC-AUTH-087) — file: `modules/identity/tests/integration/revoke_on_session_replaced_test.go`
- [ ] T040 [P] [US1] Test F014-AC-25 (Clock giả): `createGameSession` và `takeover` trong giới nghiêm → `403 CURFEW_ACTIVE` + `reopen_at`, không session/login event thành công — file: `modules/game/tests/integration/start_session_curfew_test.go` — phải FAIL trước khi code

### Implementation cho User Story 1

- [ ] T015 [P] [US1] Aggregate `GameSession`, `TakeoverPolicy` — file: `modules/game/internal/domain/gamesession/`
- [ ] T016 [US1] Command `StartGameSession` (advisory lock, idempotency, outbox) — file: `modules/game/internal/application/command/start_game_session.go`
- [ ] T017 [US1] `RealtimePublisher` Valkey `ew:rt:user:<id>` sau commit — file: `modules/game/internal/infrastructure/realtime/valkey_publisher.go`
- [ ] T018 [US1] Handler `createGameSession` (middleware `game_player`, `tenant`) — file: `modules/game/internal/presentation/http/public/sessions.go`
- [ ] T019 [US1] Consumer `identity.revoke_on_session_replaced` — file: `modules/identity/internal/presentation/consumer/revoke_on_session_replaced.go`
- [ ] T041 [US1] Projection khung giới nghiêm (`platform_settings.changed`, `school.updated` → `ref.tenants`/`ref.schools`) + kiểm giới nghiêm trong `StartGameSession` — file: `modules/game/internal/domain/gamesession/curfew_policy.go`, `modules/game/internal/application/command/start_game_session.go`

## Phase 4: User Story 2 — Lịch sử đăng nhập game và lần vào game gần nhất (Priority: P1)

### Tests cho User Story 2

- [ ] T020 [P] [US2] Test F014-AC-09, F014-AC-11: đúng một login event có `ip_hash`, không IP thô (quét DB + log), outbox `student.game_logged_in` cùng transaction — file: `modules/game/tests/integration/login_events_test.go` — phải FAIL trước khi code
- [ ] T021 [P] [US2] Test F014-AC-10: consumer cập nhật `last_login_at` idempotent, không lùi thời gian — file: `modules/identity/tests/integration/last_login_consumer_test.go`
- [ ] T022 [P] [US2] Test tenant isolation + authorization F014-AC-12, F014-AC-13: `listTenantStudentLoginEvents` scope trường/môn, tenant khác 404, `safety_moderator` 403; `listLoginEventsCrossTenant` chỉ System Admin + audit; response không IP/`device_id` — file: `modules/game/tests/integration/login_events_api_test.go`

### Implementation cho User Story 2

- [ ] T023 [P] [US2] `IPHasher` HMAC khóa xoay vòng — file: `modules/game/internal/infrastructure/security/ip_hasher.go`
- [ ] T024 [US2] Query `ListTenantStudentLoginEvents`, `ListLoginEventsCrossTenant` + handler — file: `modules/game/internal/application/query/login_events.go`, `modules/game/internal/presentation/http/public/login_events.go`
- [ ] T025 [US2] Consumer `identity.last_login_on_game_logged_in` — file: `modules/identity/internal/presentation/consumer/last_login_on_game_logged_in.go`

## Phase 5: User Story 3 — Kết thúc phiên: đăng xuất, đóng game, mất heartbeat (Priority: P1)

### Tests cho User Story 3

- [ ] T026 [P] [US3] Test F014-AC-14, F014-AC-15: end → `logout`, login event `logout_at` một lần, event; heartbeat cập nhật; session đã thay → 409 — file: `modules/game/tests/integration/end_heartbeat_test.go` — phải FAIL trước khi code
- [ ] T027 [P] [US3] Test F014-AC-16: job timeout với Clock giả, gồm trường hợp client chỉ gọi `gameLogout` không gọi `endGameSession` (ADR-0083) — file: `modules/game/tests/integration/session_timeout_job_test.go`
- [ ] T028 [P] [US3] Test authorization F014-AC-17, F014-AC-18: session người khác → 404; mọi route người chơi × {school_admin, academic_manager, tenant_admin, safety_moderator, system_admin} → 403 (sinh từ OpenAPI) — file: `modules/game/tests/integration/player_routes_authz_test.go`
- [ ] T038 [P] [US3] Test F014-AC-24 bằng event fixture (F013 T045): `user.access_revoked` → session `account_locked`; `student.cleaned_up` → `cleanup`; login event có `logout_at`/`end_reason`; `game_session.ended`; event lặp không xử lý hai lần — file: `modules/game/tests/integration/session_end_on_revoke_test.go` — phải FAIL trước khi code
- [ ] T042 [P] [US3] Test F014-AC-26 (Clock giả 21:29→21:30): job kết thúc session active `end_reason = curfew` ≤ 1 phút, login event, `game_session.ended`, publish Valkey; chạy hai instance không kết thúc trùng — file: `modules/game/tests/integration/curfew_session_closer_test.go` — phải FAIL trước khi code

### Implementation cho User Story 3

- [ ] T039 [US3] Command `EndSessionsForUser` + consumer `user.access_revoked`/`user.status_changed`/`student.cleaned_up` (inbox idempotent; chuyển từ F016 T013) — file: `modules/game/internal/application/command/end_sessions_for_user.go`, `modules/game/internal/presentation/consumer/access_revoked.go`
- [ ] T029 [US3] Command `EndGameSession`, `HeartbeatGameSession`, `TimeoutStaleSessions` + handler + job `session-timeout` — file: `modules/game/internal/application/command/end_session.go`, `modules/game/internal/presentation/job/session_timeout.go`
- [ ] T043 [US3] Command `CloseSessionsAtCurfew` + job `curfew-session-closer` (`worker-jobs`, mỗi phút, `FOR UPDATE SKIP LOCKED`); thêm `curfew` vào CHECK `end_reason` — file: `modules/game/internal/application/command/close_sessions_at_curfew.go`, `modules/game/internal/presentation/job/curfew_session_closer.go`

## Phase 6: User Story 4 — Client-config và buộc cập nhật (Priority: P1)

### Tests cho User Story 4

- [ ] T030 [P] [US4] Test F014-AC-19, F014-AC-20, F014-AC-21: client-config không cần token/header, không secret; version thấp → 426; bảo trì → `maintenance.active` và `503 MAINTENANCE` ở tạo session — file: `modules/game/tests/integration/client_config_test.go` — phải FAIL trước khi code

### Implementation cho User Story 4

- [ ] T031 [US4] Query `GetClientConfig` + handler `getClientConfig`; kiểm version/bảo trì trong `StartGameSession` — file: `modules/game/internal/application/query/client_config.go`

## Phase 7: User Story 5 — Content manifest tối thiểu theo tenant (Priority: P2)

### Tests cho User Story 5

- [ ] T032 [P] [US5] Test F014-AC-22, F014-AC-23: manifest rỗng `manifest_version = 0`; `If-None-Match` → 304; không trả manifest tenant khác — file: `modules/game/tests/integration/content_manifest_test.go` — phải FAIL trước khi code

### Implementation cho User Story 5

- [ ] T033 [US5] Query `GetContentManifest` + handler `getContentManifest` (ETag) — file: `modules/game/internal/application/query/content_manifest.go`

## Phase 8: Polish & Cross-Cutting

- [ ] T034 [P] Đăng ký `min_client_version`, `game_session_heartbeat_timeout` trong CONFIG_DEFAULTS; sửa IDENTITY_MAPPING §3.3 (cập nhật `last_login_at` bất đồng bộ); EVENT_DESIGN §4.5 (`old_keycloak_sid`); CHANGELOG — file: `docs/00-overview/CONFIG_DEFAULTS.md`, `docs/09-identity/IDENTITY_MAPPING.md`, `docs/07-backend/EVENT_DESIGN.md`
- [ ] T035 [P] Metric `eduworld_game_session_*`, `eduworld_projection_lag_seconds` — file: `modules/game/internal/infrastructure/observability/metrics.go`
- [ ] T036 [P] E2E takeover hai client headless (TC-AUTH-015) — file: `tests/e2e/game_session_takeover_test.go`
- [ ] T037 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- T038, T039 chỉ cần fixture (F013 T045); contract test với schema owner F011/F016 khi có, không cần F016; kiểm end-to-end ở F016-AC-24 (CODE_PLAN §4.1).

- Setup → Foundational (T008 projection trước mọi story) → US1 → US2, US3, US4 song song → US5 → Polish.
- Phụ thuộc identity: `resolvePrincipal` (F012), token game có `sid` (F013).

## Definition of Done

- [ ] Mọi tiêu chí F014-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh cho module `game` và phần module `identity` liên quan; `make test-e2e` takeover xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
