# Implementation Plan: F014 — Game session lát mỏng: một thiết bị, login events, client-config, manifest tối thiểu

**Branch**: `014-game-session-thin` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Feature đầu tiên của module game: dựng skeleton module `src/backend/modules/game/` theo BACKEND_DESIGN và ADR-0074, projection `ref.tenants`/`ref.students`, aggregate `GameSession`. `StartGameSession` chạy một transaction trong `eduworld_game` với `pg_advisory_xact_lock(hashtextextended(student_id::text, 0))`: kiểm Principal (`resolvePrincipal` qua facade `identity`, cache) + `ref.students`, xử lý session cũ (409/takeover/timeout/cùng thiết bị), insert `game_sessions` + `login_events`, outbox `student.game_logged_in` (+ `game_session.replaced`). Sau commit publish Valkey `ew:rt:user:<student_id>` để gateway gửi `session.session.replaced`. Job `session-timeout`. module identity thêm hai consumer: `last_login_at` và thu hồi Keycloak `sid`.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | chi, ent, Atlas, oapi-codegen (server public + facade `identity` sinh vào `src/backend/modules/identity/api`), `src/backend/pkg/modcall`, `src/backend/pkg/outbox`, `src/backend/pkg/httpx` (`game_player`, `tenant`), Valkey client |
| Storage | PostgreSQL `eduworld_game` (`game.game_sessions` T, `game.login_events` TA P tháng, `game.content_manifests` T, `ref.tenants`, `ref.students`, `infra.*`); Valkey pub/sub `ew:rt:user:<id>` |
| Testing | go test + testcontainers (PostgreSQL, NATS, Valkey); E2E 2 client headless (TC-AUTH-015) |
| Target Platform | Linux container (module `game` api + worker) |
| Project Type | backend module |
| Performance Goals | `game/sessions` p95 theo `login_slo` ([BACKEND_DESIGN §6.1](../../docs/07-backend/BACKEND_DESIGN.md)) |
| Constraints | AGENTS R8, R10, R13, R21, R23, R29, R30; ADR-0031, ADR-0037, ADR-0040, ADR-0042, ADR-0043, ADR-0065, ADR-0074 |
| Scale/Scope | `login_capacity_target` |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Operation có sẵn; delta payload event game và field `old_keycloak_sid`, `old_device_id` (ADR-0086) ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | module game không đọc DB identity; Principal qua facade `identity` (`src/backend/pkg/modcall`); `last_login_at` và thu hồi `sid` qua event |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `game_sessions`, `content_manifests` T + RLS; `login_events` TA; test isolation mọi endpoint |
| IV Server-authoritative | ✅ | Thời điểm theo giờ server; `createGameSession` có `Idempotency-Key`; audit từ chối/takeover |
| V Xác thực, phân quyền | ✅ | `game_player` (`azp = eduworld-game-backend`, `user_type = student`); `lms.student_activity.read`; `platform.cross_tenant.read` |
| VI An toàn trẻ em | ✅ | Không IP thô, không `device_id` trong response; không log PII |
| VII Một nguồn sự thật, cấu hình | ✅ | `min_client_version`, timeout heartbeat từ cấu hình (đăng ký CONFIG_DEFAULTS) |
| VIII UI tách logic, token | ✅ | Không có UI |
| IX Test có bằng chứng | ✅ | Test mọi F014-AC-*, test đồng thời, E2E takeover |

## Project Structure

### Documentation (feature này)

```text
specs/014-game-session-thin/
├── spec.md · plan.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
src/backend/modules/game/
├── cmd/{api,worker,migrate}/main.go
├── ent/schema/                      # gamesession.go, loginevent.go, contentmanifest.go, reftenant.go, refstudent.go
├── migrations/
├── internal/domain/gamesession/     # GameSession, TakeoverPolicy, EndReason
├── internal/application/command/    # StartGameSession, EndGameSession, HeartbeatGameSession, TimeoutStaleSessions
├── internal/application/query/      # GetClientConfig, GetContentManifest, ListTenantStudentLoginEvents, ListLoginEventsCrossTenant
├── internal/application/port/       # IdentityClient, RealtimePublisher, IPHasher, Clock
├── internal/infrastructure/modules/identity.go       # adapter facade `identity` (src/backend/modules/identity/api) qua src/backend/pkg/modcall
├── internal/infrastructure/persistence/
├── internal/infrastructure/realtime/valkey_publisher.go
├── internal/presentation/http/public/{sessions.go, client_config.go, content_manifest.go, login_events.go}
├── internal/presentation/consumer/projection_identity.go   # tenant.*, student.synced, user.status_changed, student.cleaned_up/restored
├── internal/presentation/job/session_timeout.go
└── tests/{unit,integration,contract,e2e}/
src/backend/modules/identity/internal/presentation/consumer/
├── last_login_on_game_logged_in.go
└── revoke_on_session_replaced.go
```

**Structure Decision**: module game theo BACKEND_DESIGN §3 (D1: `src/backend/modules/game/`); hai consumer nhỏ ở module identity vì Keycloak và `identity.users` chỉ identity sở hữu (R13).

## Phase 0: Research

Không có — ADR-0031 và DATABASE_DESIGN §3.12 đã mô tả thuật toán tạo session.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `GameSession` (Start, End(reason), Heartbeat, Replace), `TakeoverPolicy` (409 / takeover / timeout / cùng thiết bị) | Thuần; Clock inject |
| application | `StartGameSession` (advisory lock, idempotency), `EndGameSession`, `HeartbeatGameSession`, `TimeoutStaleSessions`; query ở cây trên | Permission theo [contracts](contracts/README.md) §1 |
| infrastructure | ent repo, adapter facade `identity` (`resolvePrincipal`), Valkey publisher, HMAC IP hasher (khóa xoay vòng từ secret), projection writer | |
| presentation | `createGameSession`, `endGameSession`, `heartbeatGameSession`, `getClientConfig`, `getContentManifest`, `listTenantStudentLoginEvents`, `listLoginEventsCrossTenant` · consumer projection · job `session-timeout` | |

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F013 (token game có `sid`), F012 (`resolvePrincipal`), F010 (event tenant/học sinh cho projection), F003 (outbox, idempotency).
- Event phát: `game_session.started`, `game_session.ended`, `game_session.replaced`, `student.game_logged_in`, `student.game_logged_out`, `audit.recorded`.
- Event tiêu thụ: `tenant.created/updated`, `student.synced`, `user.status_changed`, `student.cleaned_up`, `student.restored` (projection); `user.access_revoked`, `user.status_changed`, `student.cleaned_up` (kết thúc session, F014-AC-24 — schema contract-first F013 T045, test bằng fixture); identity tiêu thụ `student.game_logged_in`, `game_session.replaced`.
- Realtime: publish Valkey `ew:rt:user:<student_id>` message `session.session.replaced` (AsyncAPI), gateway F090 giao và đóng 4409.

### Bảo mật, tenant, idempotency, audit

- `game_player` cho route người chơi; `portal_staff` + permission cho route `/tenant/*`, `/admin/*`.
- `TenantScope` từ Principal; `ListLoginEventsCrossTenant` dùng `CrossTenantScope` + audit `cross_tenant.accessed`.
- `Idempotency-Key` bắt buộc `createGameSession`; audit `game_session.replaced`, `game_access_denied_non_student`.

### Quan sát

- Metric `eduworld_game_session_start_total{result}`, `eduworld_game_session_takeover_total`, `eduworld_game_session_timeout_total`, `eduworld_game_session_start_duration_seconds`, `eduworld_projection_lag_seconds{table}`.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
