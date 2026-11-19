# Quickstart: F014 — Game session lát mỏng: một thiết bị, login events, client-config, manifest tối thiểu

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F013 (và F010–F012).
- Lệnh: `make up`, `make seed`, `make up-app EXCEPT=core-api`, `make run-core-api`, `make run-worker PROFILE=events`; kiểm projection bằng `make seed-check` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `hs.an.3a`, `hs.dung.7a` (tenant `pgd-demo`), `ad.truong.a`, `ad.truong.b`, `qlcm.truong.a`, `sa.admin`.
- Token game lấy bằng `gameLogin` (F013); hai `X-Device-Id` khác nhau mô phỏng máy A, máy B.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `GET /game/client-config?platform=windows&version=0.0.1` không token | `200`, `update_required = true`, không có client id/secret | F014-AC-19 |
| 2 | `hs.an.3a` máy A `createGameSession` (`Idempotency-Key` K1) | `201`; một `game.login_events`; event `student.game_logged_in`; `identity.users.last_login_at` cập nhật | F014-AC-01, F014-AC-09, F014-AC-10 |
| 3 | Gửi lại cùng K1 | Cùng response, không session thứ hai | F014-AC-07 |
| 4 | Máy B `gameLogin` rồi `createGameSession` (`Idempotency-Key` K2) | `409 ACTIVE_SESSION_ON_OTHER_DEVICE` + `other_device` (không `device_id`) | F014-AC-03 |
| 5 | Máy B gọi lại `takeover: true` với `Idempotency-Key` K3 mới, không gửi lại mật khẩu (gọi với K2 → `409 IDEMPOTENCY_KEY_REUSED`) | `201`; A nhận `session.session.replaced` (khi có F090) hoặc `409` ở heartbeat; refresh/resume của A → `401 REFRESH_TOKEN_INVALID` (phiên game của A bị thu hồi, ADR-0086) | F014-AC-04, F014-AC-07 |
| 6 | Token `ad.truong.a` gọi `createGameSession` | `403 GAME_ACCESS_STUDENT_ONLY`, không login event | F014-AC-02, F014-AC-18 |
| 7 | Máy B ngừng heartbeat quá ngưỡng, chạy `session-timeout --once` | Session `timeout`; login event có `logout_at` | F014-AC-16 |
| 8 | `endGameSession` session của `hs.dung.7a` bằng token `hs.an.3a` | `404` | F014-AC-17 |
| 9 | `ad.truong.a` xem login events `hs.an.3a`; `ad.truong.b` xem cùng học sinh | Có lịch sử, không IP/`device_id`; `404` | F014-AC-11, F014-AC-12 |
| 10 | `sa.admin` gọi `listLoginEventsCrossTenant?tenant_ids=…` | Có cột `tenant_id`; audit `cross_tenant.accessed` | F014-AC-13 |
| 11 | `getContentManifest` rồi gửi `If-None-Match` | `200` (`manifest_version = 0`) rồi `304` | F014-AC-22, F014-AC-23 |
| 12 | `hs.an.3a` có session active; phát event fixture `user.access_revoked` (F013 T045), rồi fixture `student.cleaned_up` cho `hs.dung.7a`; phát lại cùng `event_id` | Session `account_locked`/`cleanup`, login event có `end_reason`, `game_session.ended` một lần. Khóa/dọn thật: quickstart F016 bước 13 | F014-AC-24 |
| 13 | Clock giả 21:35: `hs.an.3a` `createGameSession` (và `takeover: true`) | `403 CURFEW_ACTIVE` + `reopen_at`; không có session mới | F014-AC-25 |
| 14 | `hs.an.3a` có session active; tua 21:29 → 21:30 | Session `ended`, `end_reason = curfew` trước 21:31; `game_session.ended`; client nhận `session.session.ended` | F014-AC-26 |

## 3. Lệnh test tự động

```bash
make test MOD=game
make test-integration MOD=game
make test-integration MOD=identity   # consumer last_login_at, revoke sid
make test-e2e                                # takeover hai client headless
```

## 4. Bằng chứng cần nộp

- Kết quả test (pass/fail), trace_id kịch bản 2 và 5, truy vấn chứng minh một session active, ảnh login events không có IP.

## 5. Bằng chứng (2026-11-19, local)

Chạy bằng lệnh tương đương target `make` (Windows, chưa có `make`; giới hạn CPU `GOMAXPROCS=2`, `-p 2`), từ `src/backend`:

- `TESTCONTAINERS_RYUK_DISABLED=true go test -p 2 -tags integration ./modules/game/tests/integration/ -count=1` — PostgreSQL 17 + migration Atlas thật, NATS JetStream thật (consumer), Valkey thật (adapter realtime); HTTP đi qua chuỗi `pkg/httpx` thật (JWT, `X-Tenant-Id`, principal, guard, OpenAPI, idempotency) với issuer OIDC giả `httpxtest` (test double có nhãn); push realtime trong các test use case là test double `pushes` (có nhãn R17).
- `TESTCONTAINERS_RYUK_DISABLED=true go test -p 2 -tags integration ./modules/identity/tests/integration/ -run 'TestRevokeOnSessionReplaced|TestLastLoginOnGameLoggedIn|TestGameSessionRevokeConsumer' -count=1` — Keycloak Admin API giả như F013.
- `go test -p 2 ./modules/game/... ./modules/identity/... ./pkg/httpx/...` (unit + contract), `golangci-lint run` (cả `.golangci.arch.yml`), `go run ./tools/tenancyguard ./...`, `go run ./tools/lint/ewlint ./...`, `go run ./tools/entlint`, `go run ./tools/archgen --check`, `go run ./tools/routematrix check`.

| Kịch bản | Test tự động | Kết quả |
|---|---|---|
| 1 | `TestClientConfig` (không token/header, `update_required`, không client id/secret/OIDC; `426` khi tạo session) | PASS |
| 2 | `TestStartSessionCreates`, `TestLoginEventRecorded` (một login event, `ip_hash` HMAC, outbox `student.game_logged_in` cùng transaction); `TestLastLoginOnGameLoggedIn` (identity) | PASS |
| 3 | `TestStartSessionIdempotency` (thiếu key `428`, cùng key replay `Idempotent-Replayed`, body khác `409 IDEMPOTENCY_KEY_REUSED`) | PASS |
| 4, 5 | `TestTakeover` (`409` + `other_device` không `device_id`; takeover key mới; `replaced_by_session_id`; `game_session.replaced` có `old_device_id`/`old_keycloak_sid`; push `session.session.replaced`; heartbeat cũ `409`; payload khớp JSON Schema); `TestRevokeOnSessionReplaced` (identity: phiên game thiết bị cũ `takeover`, resume cũ `401 REFRESH_TOKEN_INVALID`); `TestConcurrentStart` (100 vòng); `TestStartAfterTimeoutAndSameDevice` | PASS |
| 6 | `TestStartSessionStudentOnly`, `TestPlayerRoutesStaffDenied` (OQ F014-4) | PASS |
| 7 | `TestSessionTimeoutJob` (gồm học sinh chỉ gọi `gameLogout`; gauge `eduworld_game_sessions_active`) | PASS |
| 8 | `TestEndHeartbeatOtherStudent` | PASS |
| 9 | `TestTenantStudentLoginEvents` (grant theo trường → `404` trường khác, tenant khác `404`, `tenant_admin`/`tenant_content_editor` `403`, không IP/`device_id`, cursor) | PASS |
| 10 | `TestLoginEventsCrossTenant` (System Admin + `cross_tenant.accessed`; role khác `403`) | PASS |
| 11 | `TestContentManifest` (rỗng `manifest_version = 0`, `304`, manifest tenant khác không trả) | PASS |
| 12 | `TestSessionEndOnRevoke` (fixture F013 T045: `account_locked`/`access_revoked`/`cleanup`, phát lại cùng `event_id`, DLQ, event platform) | PASS |
| 13 | `TestStartSessionCurfew` (Clock giả; khung trường mở rộng; takeover) | PASS |
| 14 | `TestCurfewSessionCloser` (21:29 → 21:30:05, hai instance, push `session.session.ended` có `reopen_at`); `TestValkeyPublisher` | PASS |
| Nền | `TestProjectionIdentity`, `TestRLSGameTables`, `TestMaintenance`, `TestStartSessionTenantMismatch`, `TestStartSessionNotProjected`, `TestEndAndHeartbeat`; unit `domain/gamesession`, `TestSessionFrom`, `TestIPHasher`; contract `TestOperationsMatchSpec`; golden `TestMigrationMatchesEnt` | PASS |

Truy vấn chứng minh một session active: `TestConcurrentStart` kiểm `SELECT count(*) FROM (SELECT student_id FROM game.game_sessions WHERE ended_at IS NULL GROUP BY student_id HAVING count(*) > 1) d` = 0 sau 100 vòng. Không có IP thô: `TestLoginEventRecorded` quét `game.login_events`, `game.game_sessions`, `infra.outbox_events`.

Chưa làm (cần môi trường): trace_id kịch bản 2 và 5 trên stack chạy thật, E2E hai client headless qua APISIX + realtime gateway (F090) và p95 `login_slo` ở `login_capacity_target` (SC-004) trên staging — chờ nối module vào binary (OQ F014-7, OQ F010-2).
