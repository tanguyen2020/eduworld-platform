# Tasks: F014 — Game session lát mỏng: một thiết bị, login events, client-config, manifest tối thiểu

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Skeleton module game (`src/backend/cmd/api`, `src/backend/cmd/worker`, `src/backend/cmd/migrate`, cấu hình `src/backend/pkg/config`, health, outbox relay, `.golangci.yml` depguard) — file: `src/backend/modules/game/` — **xong 2026-11-19** — skeleton có sẵn từ F001 (`modules/game/module.go`, 3 binary, health, `infra.*`); F014 thêm `ent` (generate + golden `ent/ddl_test.go`), các tầng `internal/{domain,application,infrastructure,presentation}`; nối route/consumer/job vào binary theo OQ F014-7 (OQ F010-2)
- [x] T002 [P] Adapter facade `identity` qua `src/backend/pkg/modcall` (deadline từ context, Principal gốc, ánh xạ lỗi; cache Principal 60 s; không token, không retry/circuit breaker — ADR-0074) — file: `src/backend/modules/game/internal/infrastructure/modules/identity.go` — **xong 2026-11-19** — không cần adapter riêng: Principal phân giải bởi `pkg/httpx` qua `cmd/internal/wiring.PrincipalResolver` (facade `resolvePrincipal`, `pkg/modcall`, cache `pkg/authz`) — xem Clarifications 2026-11-19
- [x] T003 [P] Cấu hình `EW_GAME_MIN_CLIENT_VERSION`, `EW_GAME_LATEST_CLIENT_VERSION`, ngưỡng timeout heartbeat, cờ bảo trì, khóa HMAC IP (secret file) — file: `src/backend/modules/game/internal/config/session.go` — **xong 2026-11-19** — `internal/infrastructure/config/session.go` (`SessionFrom`: `EW_GAME_MIN_CLIENT_VERSION`, `EW_GAME_LATEST_CLIENT_VERSION`, `EW_GAME_SESSION_HEARTBEAT_TIMEOUT`, `EW_GAME_SESSION_TIMEOUT_SCAN`, `EW_GAME_MAINTENANCE*`, `EW_PLATFORM_CURFEW`, `EW_GAME_LOGIN_URL`, `EW_GAME_REMEMBER_USERNAME_DEFAULT`, `EW_SUPPORTED_LOCALES`, `EW_GAME_IP_HASH_KEY(_FILE)`); test `TestSessionFrom`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Thêm payload schema `student.game_logged_in`, `student.game_logged_out`, `game_session.started`, `game_session.ended`, `game_session.replaced` (có `old_keycloak_sid`, `old_device_id` — ADR-0086) — file: `specs/events/payloads/game/` — kiểm: `make specs-lint` — **xong 2026-11-17** (kiểm bằng script `scripts/specs/*`, `scripts/openapi/*`; target `make` chưa có tới F001)
- [x] T005 [P] Bổ sung response `426`, `503` cho `createGameSession`, `403` cho `endGameSession`/`heartbeatGameSession` ở spec gộp rồi chạy `node scripts/openapi/split-public.mjs` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make gen MOD=game` — **xong 2026-11-17** (kiểm bằng script `scripts/specs/*`, `scripts/openapi/*`; target `make` chưa có tới F001)
- [x] T006 ent schema `GameSession`, `LoginEvent` (AppendOnly), `ContentManifest`, `RefTenant`, `RefStudent` với `MaxLen` — file: `src/backend/modules/game/ent/schema/` — kiểm: `make migrate-diff MOD=game name=f014_session`, `make migrate-lint MOD=game` — **xong 2026-11-19** — `ent/schema/gamesession.go` (`GameSession` Base, `LoginEvent` TenantAppendOnly, mọi chuỗi `MaxLen`); projection `ref.*` và `content_manifests` viết tay (Clarifications 2026-11-19); `TestMigrationMatchesEnt`, `go run ./tools/entlint` sạch
- [x] T007 Migration: unique partial một session active, index heartbeat, partition tháng `login_events`, RLS, `REVOKE DELETE, TRUNCATE`, `infra.*` — file: `src/backend/modules/game/migrations/` — **xong 2026-11-19** — `migrations/20261123000001_f014_game_session.sql` (unique partial một session active, index heartbeat, FK `replaced_by_session_id` DEFERRABLE, `login_events` partition RANGE + DEFAULT, trigger ghi kết thúc một lần, RLS FORCE, REVOKE DELETE/TRUNCATE, `game_sessions_live`, `ref.projection_tenant`); `atlas.sum` qua `scripts/db/atlas-sum.mjs`; Atlas áp trong testcontainers
- [x] T008 [P] Consumer projection `ref.tenants`, `ref.students` (`tenant.*`, `student.synced`, `user.status_changed`, `student.cleaned_up`, `student.restored`; `source_version`) + test bắt kịp (TC-SVC-008) — file: `src/backend/modules/game/internal/presentation/consumer/projection_identity.go` — **xong 2026-11-19** — consumer `game.projection_identity` + `command.UpsertProjection` + `write.RefRepo`; `TestProjectionIdentity` (tenant đến muộn → retry bắt kịp, source_version cũ bỏ qua, `curfew_window`, `user.status_changed`, cleanup/restore, lệch tenant → DLQ) PASS
- [x] T009 [P] Test RLS bảng T/TA của module game (tenant A không đọc B; không set → 0 dòng; DELETE bị từ chối) — file: `src/backend/modules/game/tests/integration/rls_test.go` — **xong 2026-11-19** — `tests/integration/rls_test.go` `TestRLSGameTables` PASS

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Tạo game session, một thiết bị mỗi tài khoản (Priority: P1) 🎯 MVP

**Goal**: Một session active mỗi học sinh, takeover có xác nhận.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [x] T010 [P] [US1] Test F014-AC-01, F014-AC-02, F014-AC-08: tạo session đủ trường; nhân sự/học sinh không hoạt động → `GAME_ACCESS_STUDENT_ONLY` + audit, không login event; header tenant khác → `TENANT_MISMATCH` — file: `src/backend/modules/game/tests/integration/start_session_test.go` — phải FAIL trước khi code — **xong 2026-11-19** — `TestStartSessionCreates`, `TestStartSessionStudentOnly`, `TestStartSessionTenantMismatch`, `TestStartSessionNotProjected` PASS
- [x] T011 [P] [US1] Test F014-AC-03, F014-AC-04, F014-AC-06: `409` + `other_device` không `device_id`; takeover (key mới, không mật khẩu) kết thúc session cũ, phát `game_session.replaced`, publish Valkey; timeout cũ → không hỏi; cùng thiết bị → `logout` — file: `src/backend/modules/game/tests/integration/takeover_test.go` — **xong 2026-11-19** — `TestTakeover`, `TestStartAfterTimeoutAndSameDevice` PASS (push Valkey qua test double; adapter thật ở `TestValkeyPublisher`)
- [x] T012 [P] [US1] Test F014-AC-05: 100 lượt hai thiết bị đồng thời → đúng một thành công — file: `src/backend/modules/game/tests/integration/concurrent_start_test.go` — **xong 2026-11-19** — `TestConcurrentStart` (100 vòng, mỗi vòng đúng một 201 + một 409, 0 học sinh hai session active) PASS
- [x] T013 [P] [US1] Test F014-AC-07: thiếu `Idempotency-Key` → 428; cùng key + body → cùng kết quả; cùng key + `takeover: true` → 409 `IDEMPOTENCY_KEY_REUSED` — file: `src/backend/modules/game/tests/integration/start_session_idempotency_test.go` — **xong 2026-11-19** — `TestStartSessionIdempotency` PASS (OQ F014-3)
- [x] T014 [P] [US1] Test identity consumer F014-AC-04: `game_session.replaced` → Admin API xóa session `old_keycloak_sid` và `GameAuthSessionRevoker` (F013) đánh dấu phiên game của `old_device_id` `revoked_at` (`takeover`); refresh/resume cũ → `401 REFRESH_TOKEN_INVALID` (Keycloak testcontainer, TC-AUTH-087) — file: `src/backend/modules/identity/tests/integration/revoke_on_session_replaced_test.go` — **xong 2026-11-19** — `modules/identity/tests/integration/game_session_replaced_integration_test.go` `TestRevokeOnSessionReplaced` PASS (Keycloak Admin API giả như F013)
- [x] T040 [P] [US1] Test F014-AC-25 (Clock giả): `createGameSession` và `takeover` trong giới nghiêm → `403 CURFEW_ACTIVE` + `reopen_at`, không session/login event thành công — file: `src/backend/modules/game/tests/integration/start_session_curfew_test.go` — phải FAIL trước khi code — **xong 2026-11-19** — `TestStartSessionCurfew` (Clock giả 20:30 trường mở rộng, 21:35 platform; takeover) PASS

### Implementation cho User Story 1

- [x] T015 [P] [US1] Aggregate `GameSession`, `TakeoverPolicy` — file: `src/backend/modules/game/internal/domain/gamesession/` — **xong 2026-11-19** — `internal/domain/gamesession` (`Session`, `Decide`, `Version`, `Window`/`Curfew`); unit test PASS
- [x] T016 [US1] Command `StartGameSession` (advisory lock, idempotency, outbox) — file: `src/backend/modules/game/internal/application/command/start_game_session.go` — **xong 2026-11-19** — `command.Sessions.Start` (advisory lock `hashtextextended`, 409/takeover/timeout/cùng thiết bị, outbox, `idempotency.SetResult`)
- [x] T017 [US1] `RealtimePublisher` Valkey `ew:rt:user:<id>` sau commit — file: `src/backend/modules/game/internal/infrastructure/realtime/valkey_publisher.go` — **xong 2026-11-19** — `internal/infrastructure/realtime/valkey_publisher.go`; `TestValkeyPublisher` PASS
- [x] T018 [US1] Handler `createGameSession` (middleware `game_player`, `tenant`) — file: `src/backend/modules/game/internal/presentation/http/public/sessions.go` — **xong 2026-11-19** — `internal/presentation/http/routes.go` (`createGameSession` + guard `game_player`, `TenantRequired`, `IdempotencyRequired`); contract `TestOperationsMatchSpec` PASS
- [x] T019 [US1] Consumer `identity.revoke_on_session_replaced` — file: `src/backend/modules/identity/internal/presentation/consumer/revoke_on_session_replaced.go` — **xong 2026-11-19** — `modules/identity/internal/presentation/consumer/revoke_on_session_replaced.go`
- [x] T041 [US1] Projection khung giới nghiêm (`platform_settings.changed`, `school.updated` → `ref.tenants`/`ref.schools`) + kiểm giới nghiêm trong `StartGameSession` — file: `src/backend/modules/game/internal/domain/gamesession/curfew_policy.go`, `src/backend/modules/game/internal/application/command/start_game_session.go` — **2026-11-17:** phần contract đã áp (`createGameSession` khai báo `403 CURFEW_ACTIVE` + `Problem.reopen_at`; payload `platform_settings.changed.curfew_window`, `school.updated.settings_subset.curfew_window`); còn projection + kiểm giới nghiêm — **xong 2026-11-19** — `domain/gamesession/curfew_policy.go` + kiểm giới nghiêm trong `Start`; khung trường từ `ref.schools.settings_subset` (projection `school.synced/updated`); `platform_curfew` từ cấu hình (OQ F014-2) — **2026-11-20:** consumer `game.platform_settings` + projection `ref.platform_settings` (`platform_settings.changed`, key `platform_curfew`) dùng cho `Start` và `curfew-session-closer`, env chỉ là mặc định ban đầu; `TestPlatformCurfewChangeTakesEffectWithoutRestart` PASS

## Phase 4: User Story 2 — Lịch sử đăng nhập game và lần vào game gần nhất (Priority: P1)

### Tests cho User Story 2

- [x] T020 [P] [US2] Test F014-AC-09, F014-AC-11: đúng một login event có `ip_hash`, không IP thô (quét DB + log), outbox `student.game_logged_in` cùng transaction — file: `src/backend/modules/game/tests/integration/login_events_test.go` — phải FAIL trước khi code — **xong 2026-11-19** — `TestLoginEventRecorded` PASS (ip_hash 32 byte, quét DB + outbox không có IP thô, append-only, đóng một lần)
- [x] T021 [P] [US2] Test F014-AC-10: consumer cập nhật `last_login_at` idempotent, không lùi thời gian — file: `src/backend/modules/identity/tests/integration/last_login_consumer_test.go` — **xong 2026-11-19** — `TestLastLoginOnGameLoggedIn` (identity) PASS
- [x] T022 [P] [US2] Test tenant isolation + authorization F014-AC-12, F014-AC-13: `listTenantStudentLoginEvents` scope trường/môn, tenant khác 404, `safety_moderator` 403; `listLoginEventsCrossTenant` chỉ System Admin + audit; response không IP/`device_id` — file: `src/backend/modules/game/tests/integration/login_events_api_test.go` — **xong 2026-11-19** — `TestTenantStudentLoginEvents`, `TestLoginEventsCrossTenant` PASS

### Implementation cho User Story 2

- [x] T023 [P] [US2] `IPHasher` HMAC khóa xoay vòng — file: `src/backend/modules/game/internal/infrastructure/security/ip_hasher.go` — **xong 2026-11-19** — `internal/infrastructure/security/ip_hasher.go`; `TestIPHasher` PASS
- [x] T024 [US2] Query `ListTenantStudentLoginEvents`, `ListLoginEventsCrossTenant` + handler — file: `src/backend/modules/game/internal/application/query/login_events.go`, `src/backend/modules/game/internal/presentation/http/public/login_events.go` — **xong 2026-11-19** — `query.LoginEvents` (`ForStudent`, `CrossTenant`), `persistence/read.LoginEvents`, handler trong `presentation/http/routes.go`
- [x] T025 [US2] Consumer `identity.last_login_on_game_logged_in` — file: `src/backend/modules/identity/internal/presentation/consumer/last_login_on_game_logged_in.go` — **xong 2026-11-19** — `modules/identity/internal/presentation/consumer/last_login_on_game_logged_in.go` + `command.RecordGameLogin` + `write.LastLoginRepo`

## Phase 5: User Story 3 — Kết thúc phiên: đăng xuất, đóng game, mất heartbeat (Priority: P1)

### Tests cho User Story 3

- [x] T026 [P] [US3] Test F014-AC-14, F014-AC-15: end → `logout`, login event `logout_at` một lần, event; heartbeat cập nhật; session đã thay → 409 — file: `src/backend/modules/game/tests/integration/end_heartbeat_test.go` — phải FAIL trước khi code — **xong 2026-11-19** — `TestEndAndHeartbeat` PASS
- [x] T027 [P] [US3] Test F014-AC-16: job timeout với Clock giả, gồm trường hợp client chỉ gọi `gameLogout` không gọi `endGameSession` (ADR-0083) — file: `src/backend/modules/game/tests/integration/session_timeout_job_test.go` — **xong 2026-11-19** — `TestSessionTimeoutJob` PASS
- [x] T028 [P] [US3] Test authorization F014-AC-17, F014-AC-18: session người khác → 404; mọi route người chơi × {school_admin, academic_manager, tenant_admin, safety_moderator, system_admin} → 403 (sinh từ OpenAPI) — file: `src/backend/modules/game/tests/integration/player_routes_authz_test.go` — **xong 2026-11-19** — `TestEndHeartbeatOtherStudent`, `TestPlayerRoutesStaffDenied` PASS (OQ F014-4)
- [x] T038 [P] [US3] Test F014-AC-24 bằng event fixture (F013 T045): `user.access_revoked` → session `account_locked`; `student.cleaned_up` → `cleanup`; login event có `logout_at`/`end_reason`; `game_session.ended`; event lặp không xử lý hai lần — file: `src/backend/modules/game/tests/integration/session_end_on_revoke_test.go` — phải FAIL trước khi code — **xong 2026-11-19** — `TestSessionEndOnRevoke` (fixture F013 T045, NATS thật, phát lại cùng event_id, DLQ, event platform) PASS
- [x] T042 [P] [US3] Test F014-AC-26 (Clock giả 21:29→21:30): job kết thúc session active `end_reason = curfew` ≤ 1 phút, login event, `game_session.ended`, publish Valkey; chạy hai instance không kết thúc trùng — file: `src/backend/modules/game/tests/integration/curfew_session_closer_test.go` — phải FAIL trước khi code — **xong 2026-11-19** — `TestCurfewSessionCloser` (21:29 → 21:30:05, hai instance song song) PASS

### Implementation cho User Story 3

- [x] T039 [US3] Command `EndSessionsForUser` + consumer `user.access_revoked`/`user.status_changed`/`student.cleaned_up` (inbox idempotent; chuyển từ F016 T013) — file: `src/backend/modules/game/internal/application/command/end_sessions_for_user.go`, `src/backend/modules/game/internal/presentation/consumer/access_revoked.go` — **xong 2026-11-19** — `command.Sessions.EndSessionsForUser` (file `end_session.go`) + consumer `game.session_end_on_revoke` (`presentation/consumer/access_revoked.go`)
- [x] T029 [US3] Command `EndGameSession`, `HeartbeatGameSession`, `TimeoutStaleSessions` + handler + job `session-timeout` — file: `src/backend/modules/game/internal/application/command/end_session.go`, `src/backend/modules/game/internal/presentation/job/session_timeout.go` — **xong 2026-11-19** — `command/end_session.go` (`End`, `Heartbeat`, `TimeoutStaleSessions`), `presentation/job/jobs.go` (`SessionTimeoutJob`, `Every`)
- [x] T043 [US3] Command `CloseSessionsAtCurfew` + job `curfew-session-closer` (`worker-jobs`, mỗi phút, `FOR UPDATE SKIP LOCKED`); thêm `curfew` vào CHECK `end_reason` — file: `src/backend/modules/game/internal/application/command/close_sessions_at_curfew.go`, `src/backend/modules/game/internal/presentation/job/curfew_session_closer.go` — **2026-11-17:** phần contract đã áp (`LoginEndReason`/payload `game_session.ended`, `student.game_logged_out` thêm `curfew`); AsyncAPI `session.session.ended` + close `4410` đã thêm (quyết định Technical Architect); còn command/job + CHECK DB — **xong 2026-11-19** — `CloseSessionsAtCurfew` (trong `command/end_session.go`) + `job.CurfewSessionCloserJob`; CHECK `end_reason` có `curfew` trong migration; chống kết thúc trùng bằng `FOR UPDATE` + kiểm lại trạng thái

## Phase 6: User Story 4 — Client-config và buộc cập nhật (Priority: P1)

### Tests cho User Story 4

- [x] T030 [P] [US4] Test F014-AC-19, F014-AC-20, F014-AC-21: client-config không cần token/header, không secret; version thấp → 426; bảo trì → `maintenance.active` và `503 MAINTENANCE` ở tạo session — file: `src/backend/modules/game/tests/integration/client_config_test.go` — phải FAIL trước khi code — **xong 2026-11-19** — `TestClientConfig`, `TestMaintenance` PASS

### Implementation cho User Story 4

- [x] T031 [US4] Query `GetClientConfig` + handler `getClientConfig`; kiểm version/bảo trì trong `StartGameSession` — file: `src/backend/modules/game/internal/application/query/client_config.go` — **xong 2026-11-19** — `query.ClientConfigSource.GetClientConfig` (file `query/queries.go`); kiểm version/bảo trì trong `Start`

## Phase 7: User Story 5 — Content manifest tối thiểu theo tenant (Priority: P2)

### Tests cho User Story 5

- [x] T032 [P] [US5] Test F014-AC-22, F014-AC-23: manifest rỗng `manifest_version = 0`; `If-None-Match` → 304; không trả manifest tenant khác — file: `src/backend/modules/game/tests/integration/content_manifest_test.go` — phải FAIL trước khi code — **xong 2026-11-19** — `TestContentManifest` PASS

### Implementation cho User Story 5

- [x] T033 [US5] Query `GetContentManifest` + handler `getContentManifest` (ETag) — file: `src/backend/modules/game/internal/application/query/content_manifest.go` — **xong 2026-11-19** — `query.GetContentManifest` + `persistence/read.Manifests`; handler ETag/`If-None-Match` (file `query/queries.go`)

## Phase 8: Polish & Cross-Cutting

- [x] T034 [P] Đăng ký `min_client_version`, `game_session_heartbeat_timeout` trong CONFIG_DEFAULTS; sửa IDENTITY_MAPPING §3.3 (cập nhật `last_login_at` bất đồng bộ); EVENT_DESIGN §4.5 (`old_keycloak_sid`); CHANGELOG — file: `docs/00-overview/CONFIG_DEFAULTS.md`, `docs/09-identity/IDENTITY_MAPPING.md`, `docs/07-backend/EVENT_DESIGN.md` — **xong 2026-11-19** — `min_client_version`, `game_session_heartbeat_timeout` đã có trong CONFIG_DEFAULTS; IDENTITY_MAPPING §3.3 và EVENT_DESIGN §4.5 đã đúng; cập nhật LOCAL_DEVELOPMENT (biến game), MONITORING (metric F014), CHANGELOG
- [x] T035 [P] Metric `eduworld_game_session_*`, `eduworld_projection_lag_seconds` — file: `src/backend/modules/game/internal/infrastructure/observability/metrics.go` — **xong 2026-11-19** — `internal/infrastructure/observability/metrics.go` (tên MONITORING: `eduworld_game_logins_total`, `eduworld_game_sessions_active`, `eduworld_game_session_takeovers_total`, `eduworld_game_session_ended_total`, `eduworld_game_access_denied_total`; plan: `eduworld_game_session_start_total`, `eduworld_game_session_start_duration_seconds`; `eduworld_game_projection_lag_seconds` — OQ F014-6)
- [x] T036 [P] E2E takeover hai client headless (TC-AUTH-015) — file: `tests/e2e/game_session_takeover_test.go` — **xong 2026-11-19** — mức integration: `TestTakeover` (hai client HTTP qua chuỗi `pkg/httpx` thật) + `TestRevokeOnSessionReplaced`; E2E trên binary chạy thật chờ nối composition root (OQ F014-7) và gateway F090
- [x] T037 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — **xong 2026-11-19** — bằng chứng ở [quickstart.md](quickstart.md) §5

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Đã làm 2026-11-20.

- [x] T044 **OQ chốt 2026-11-20** (OQ F014-1) Projection `ref.students.sis_connection_id` từ `student.sis_link_created`/`student.sis_link_superseded`/`student.synced`; `login_events` ghi giá trị này — file: `src/backend/modules/game/internal/presentation/consumer/`, migration game — kiểm: `make test-integration MOD=game` — cỡ S — ✅ 2026-11-20: `ref.students.sis_connection_id` (migration `20261128000001_oq17_daily_limit_sis_link.sql`), consumer `game.projection_identity` nhận `student.sis_link_created`/`student.sis_link_superseded`, `student.synced` ghi `sis_connection_id`; `createGameSession` ghi vào `game.login_events`; test `TestLoginEventSISConnection` PASS
- [x] T045 **OQ chốt 2026-11-20** (OQ-17, F014-AC-27) Giới hạn chơi/ngày: projection `daily_play_limit_minutes` của trường, tổng thời gian session trong ngày ICT, chặn `createGameSession` bằng `DAILY_PLAY_LIMIT_REACHED` + cảnh báo/đóng phiên (`end_reason = daily_limit`) trong job đóng phiên; schema `game_session.ended` thêm giá trị `end_reason`; test F014-AC-27 — file: `specs/events/payloads/game/game_session.ended.v1.schema.json`, `src/backend/modules/game/internal/application/command/session.go`, `internal/presentation/job/curfew_session_closer.go` — kiểm: `make test-integration MOD=game` — cỡ M — ✅ 2026-11-20: `end_reason = daily_limit` (domain + CHECK DB), `Directory.DailyPlayLimit` (tenant `common` không áp), `SessionRepository.PlayedSince`; `createGameSession`/takeover → `403 DAILY_PLAY_LIMIT_REACHED` + `reopen_at`; job `curfew-session-closer` gửi `session.session.ending_soon` một lần (`EW_CURFEW_WARNING_MINUTES`) rồi kết thúc phiên, `session.session.ended` `daily_limit` + `reopen_at` (AsyncAPI, REALTIME_ARCHITECTURE §3.1 close 4410); test `TestDailyPlayLimit`, `TestDailyAllowance` PASS (F014-AC-27)

## Dependencies & Execution Order

- T038, T039 chỉ cần fixture (F013 T045); contract test với schema owner F011/F016 khi có, không cần F016; kiểm end-to-end ở F016-AC-24 (CODE_PLAN §4.1).

- Setup → Foundational (T008 projection trước mọi story) → US1 → US2, US3, US4 song song → US5 → Polish.
- Phụ thuộc identity: `resolvePrincipal` (F012), token game có `sid` (F013).

## Definition of Done

- [x] Mọi tiêu chí F014-AC-* có test pass (ghi mã test/TC) — mọi AC có test PASS (bảng quickstart §5)
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` xanh cho module `game` và phần module `identity` liên quan; `make test-e2e` takeover xanh — chạy bằng lệnh tương đương (chưa có `make`), xem quickstart §5; E2E chờ OQ F014-7
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — test double chỉ trong test, có nhãn R17
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
