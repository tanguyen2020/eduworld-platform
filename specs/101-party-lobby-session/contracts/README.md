# Contracts: F101 — Party Lobby, Start Session, leader/thành viên mất kết nối

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/eduworld-api.v1.yaml` → `specs/openapi/modules/competition.v1.yaml`. Mọi operation: `game_player`, `X-Tenant-Id`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `setPartyReady` | `PUT /api/v1/multiplayer/parties/{party_id}/members/me/ready` | competition.v1.yaml | thành viên | Cần sửa (thêm `409`, `429`) |
| `selectPartyActivity` | `PUT /api/v1/multiplayer/parties/{party_id}/activity` (`PartyActivitySelection` + `expected_version`) | competition.v1.yaml | leader | Cần sửa |
| `startPartySession` | `POST /api/v1/multiplayer/parties/{party_id}:start` (`Idempotency-Key`; body `{expected_version}`) → `PartySessionStart` | competition.v1.yaml | leader | Cần sửa (thêm requestBody, `409` có `ineligible_members`, `503`) |
| `listPartyEligibleActivities` | `GET /api/v1/multiplayer/parties/{party_id}/eligible-activities` | competition.v1.yaml | thành viên | Cần thêm |
| `getParty` | `GET /api/v1/multiplayer/parties/{party_id}` (snapshot lobby) | competition.v1.yaml | thành viên | Có sẵn (F100) |
| `getRoom` | `GET /api/v1/multiplayer/rooms/{room_id}` | competition.v1.yaml | thành viên session | Có sẵn |

`ProblemCode` dùng: `PARTY_NOT_LEADER`, `PARTY_ACTIVITY_NOT_ELIGIBLE`, `PARTY_NOT_FOUND`, `VERSION_MISMATCH`, `INVALID_STATE_TRANSITION`, `RATE_LIMITED`, `UNAVAILABLE`, `PRECONDITION_REQUIRED`, `GAME_ACCESS_STUDENT_ONLY`; **Cần thêm** `PARTY_NOT_READY` (đã có trong AsyncAPI `ErrorCode`, thiếu ở OpenAPI).

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `listPartyActivityCandidates` | competition → game (facade) | game.internal.v1.yaml | `competition` | Cần thêm (F103 hiện thực) |
| `checkPartyActivityEligibility` | competition → game (facade) | game.internal.v1.yaml | `competition` | Cần thêm (F103) |
| `startPartyActivity` | competition → game (facade, `Idempotency-Key = party_activity_id`) | game.internal.v1.yaml | `competition` | Cần thêm (F103) |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `party.member.ready_set` | client → server (`ew.cmd.competition.party.member.ready_set`) | Có sẵn |
| `party.activity.select` | client → server | Cần sửa (thêm `expected_version`; `activity` theo `PartyActivityType`) |
| `party.session.start` | client → server | Cần sửa (thêm `expected_version`) |
| `party.member.ready_changed`, `party.member.status_changed`, `party.activity.selected`, `party.leader.transferred`, `party.state.snapshot`, `party.session.ended` | server → thành viên (`ew:rt:user:<id>`) | Có sẵn |
| `party.session.started` | server → thành viên tham gia | Cần sửa (`room_type` thêm `boss`) |
| `ActivityRef` (schema) | — | Cần sửa (`kind` theo `PartyActivityType`, thêm `boss`) |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `party.activity_selected` | competition → AUD/ANA | `specs/events/payloads/competition/party.activity_selected.v1.schema.json` | Cần thêm |
| `party.activity_started` | competition → game (F103), chat, AUD, ANA | `party.activity_started.v1.schema.json` | Cần thêm |
| `party.activity_ended`, `party.activity_completed` | competition → game (reward co-op), LA, AUD | `party.activity_ended.v1`, `party.activity_completed.v1` | Cần thêm |
| `boss.encounter_completed` | game → competition (kết thúc activity) | `specs/events/payloads/game/boss.encounter_completed.v1.schema.json` (thêm `party_activity_id`) | Cần thêm (F103) |
| `quest.party_completed` | game → competition | `specs/events/payloads/game/quest.party_completed.v1.schema.json` | Cần thêm (owner F103) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/common/enums.schema.json#/$defs/PartyActivityType` | Một enum loại hoạt động party (research R4) | Cần thêm |

## 6. Delta cần thực hiện

- `enums.schema.json`: thêm `PartyActivityType` (`explore`, `quest`, `boss`, `learning_challenge`, `coop_challenge`, `team_battle`); OpenAPI `PartyActivitySelection.activity_type`, AsyncAPI `ActivityRef.kind` `$ref` tới đây.
- `eduworld-api.v1.yaml`: `PartyActivitySelection` thêm `expected_version` (integer); `startPartySession` thêm requestBody `PartySessionStartRequest {expected_version}`, response `409` (Problem + `ineligible_members[] {student_id, reason_code (enum: grade_not_allowed, grade_band_mismatch, prerequisite_missing, area_locked, attempt_limit, sanctioned, not_online), requirement ≤ 255}`), `503`; thêm `listPartyEligibleActivities` (response `{data: [{activity_type, activity_ref, title LocalizedName, min_members, max_members, all_eligible, members: [{student_id, eligible, reason_code}]}]}`); `ProblemCode` thêm `PARTY_NOT_READY`; `setPartyReady` thêm `409`, `429`. Sinh lại `competition.v1.yaml`.
- `eduworld-realtime.v1.yaml`: `party.activity.select`, `party.session.start` thêm `expected_version`; `PartySessionStarted.room_type` thêm `boss`.
- `game.internal.v1.yaml`: ba operation ở §2 (schema chi tiết ở [F103 contracts](../../103-boss-coop-quest/contracts/README.md)).
- Event payload schema ở §4 (thư mục `competition/`).
- Tài liệu (task Polish): API_DESIGN §5 (`PARTY_ACTIVITY_NOT_ELIGIBLE` 409), VALKEY_DESIGN §3.1 (key mới), DATABASE_DESIGN §2.9 (cột mới), CONFIG_DEFAULTS (`party_activity_min_members_timeout`, `party_lobby_cmd_rate`).
- Kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=competition`, `make test-contract`.
