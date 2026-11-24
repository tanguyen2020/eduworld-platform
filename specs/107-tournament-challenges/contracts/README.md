# Contracts: F107 — Tournament: 6 format, bracket, lịch, thử thách khối/giải trường từ template

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/modules/competition.v1.yaml` (route `/lms/challenges*` hiện ở `learning.v1.yaml`, cần chuyển). Bundle `specs/openapi/eduworld-api.v1.yaml`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `createChallenge` | `POST /api/v1/lms/challenges` | `learning.v1.yaml` (`ChallengeCreate`, `Challenge`) | `lms.challenge.write` · `Idempotency-Key` | Cần sửa (`x-service: competition`; đối tượng khối ± học sinh đã áp 2026-11-22, §6) |
| `listChallenges` | `GET /api/v1/lms/challenges?grade_level=&status=` | `learning.v1.yaml` | `lms.challenge.write` (scope trường) | Đã áp 2026-11-24 (F107: `x-service: competition`) |
| `listChallengeTemplates` | `GET /api/v1/lms/catalog/challenge-templates` | `learning.v1.yaml` (`ChallengeTemplate`) | `lms.challenge.write` | Đã áp 2026-11-24 (F107: `x-service: competition`) |
| `getChallenge` | `GET /api/v1/lms/challenges/{challenge_id}` | — | `lms.challenge.write` | Đã áp 2026-11-24 (F107) |
| `cancelChallenge` | `POST /api/v1/lms/challenges/{challenge_id}:cancel` | — | `lms.challenge.write` · `Idempotency-Key` | Đã áp 2026-11-24 (F107) |
| `listMyChallenges` | `GET /api/v1/competition/challenges` | — | học sinh (`game_player`) | Đã áp 2026-11-24 (F107) |
| `joinChallenge` | `POST /api/v1/competition/challenges/{challenge_id}:join` | — | học sinh (đối tượng) · `Idempotency-Key` | Đã áp 2026-11-24 (F107) |
| `listTournaments` | `GET /api/v1/competition/tournaments?scope=&status=` | `competition.v1.yaml` (`Tournament`) | học sinh (phạm vi) / `lms.challenge.write` | Đã áp 2026-11-24 (F107: `scope`, `status`, `school_id`, schema 11 trạng thái) |
| `getTournament` | `GET /api/v1/competition/tournaments/{tournament_id}` | — | học sinh (phạm vi) / `lms.challenge.write` | Đã áp 2026-11-24 (F107) |
| `publishTournament` | `POST /api/v1/competition/tournaments/{tournament_id}:publish` | — | `lms.challenge.write` · `Idempotency-Key` | Đã áp 2026-11-24 (F107) |
| `cancelTournament` | `POST /api/v1/competition/tournaments/{tournament_id}:cancel` | — | `lms.challenge.write` · `Idempotency-Key` | Đã áp 2026-11-24 (F107) |
| `registerTournamentEntry` | `POST /api/v1/competition/tournaments/{tournament_id}/entries` | — | học sinh/leader đội; người tổ chức (đăng ký hộ) · `Idempotency-Key` | Đã áp 2026-11-24 (F107) |
| `withdrawTournamentEntry` | `DELETE /api/v1/competition/tournaments/{tournament_id}/entries/{entry_id}` | — | chủ entry / người tổ chức | Đã áp 2026-11-24 (F107) |
| `confirmTournamentEntries` | `POST /api/v1/competition/tournaments/{tournament_id}/entries:confirm` | — | `lms.challenge.write` · `Idempotency-Key` | Đã áp 2026-11-24 (F107) |
| `checkInTournamentEntry` | `POST /api/v1/competition/tournaments/{tournament_id}/entries/{entry_id}:check-in` | — | chủ entry | Đã áp 2026-11-24 (F107) |
| `generateTournamentBracket` | `POST /api/v1/competition/tournaments/{tournament_id}:generate-bracket` | — | `lms.challenge.write` · `Idempotency-Key` | Đã áp 2026-11-24 (F107) |
| `getTournamentBracket` | `GET /api/v1/competition/tournaments/{tournament_id}/bracket` | — | học sinh (phạm vi) / `lms.challenge.write` | Đã áp 2026-11-24 (F107) |
| `confirmBracketNode` | `POST /api/v1/competition/tournaments/{tournament_id}/nodes/{node_id}:confirm` | — | `lms.challenge.write` | Đã áp 2026-11-24 (F107) |
| `overrideBracketNode` | `POST /api/v1/competition/tournaments/{tournament_id}/nodes/{node_id}:override` | — | `lms.challenge.write` · `Idempotency-Key` · lý do | Đã áp 2026-11-24 (F107) |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `assignTenantResources` | identity → competition | `common/internal-common.v1.yaml` | `[identity]` | Đã áp 2026-11-24 (F107 bên nhận `applyTenantAssignments`) |
| `unassignTenantResources` | identity → competition | `common/internal-common.v1.yaml` | `[identity]` | Đã áp 2026-11-24 (F107 bên nhận) |
| `listTenantResourceAssignments` | identity → competition | `common/internal-common.v1.yaml` | `[identity]` | Đã áp 2026-11-24 (F107 bên nhận) |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `tournament.match.scheduled` (`tournament_id`, `node_id`, `scheduled_at`, `opponent` PublicPlayer) | server → client | Đã áp 2026-11-24 (F107) |
| `tournament.bracket.updated` (`tournament_id`, `node_id`, `status`) | server → client | Đã áp 2026-11-24 (F107) |
| `matchmaking.proposal.created` / `matchmaking.match.assigned` (trận giải) | server → client | Có sẵn |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `tournament.match_scheduled`, `tournament.completed` | Phát | `specs/events/payloads/competition/tournament.*.v1.schema.json` | Đã áp 2026-11-24 (F107) |
| `tournament.rewarded` | Phát (module game cấp thưởng) | `specs/events/payloads/competition/tournament.rewarded.v1.schema.json` | Đã áp 2026-11-24 (F107, EVENT_DESIGN §4.8) |
| `match.completed` | Tiêu thụ (F105) | `.../match.completed.v1.schema.json` | Có sẵn (F105) |
| `audit.recorded` | Phát | envelope | Có sẵn |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/tournament-template.schema.json`, `challenge-template.schema.json` | Payload template (format, mode, `allowed_params`, `reward_rule_refs`, khung giờ) | F108 (F107 dùng seed hợp lệ `seed/tournament_templates.json`, quy ước payload ở spec Clarifications 2026-11-24) |

## 6. Delta cần thực hiện

> **Đã áp toàn bộ 2026-11-24 (F107)** — tên operation internal thật là `applyTenantAssignments`/`revokeTenantAssignments`/`listResourceAssignments` (§2), `screenText` của module chat thêm `competition` vào `x-allowed-callers`; `params.maxProperties 20`; `TournamentEntryResult {entries[], skipped[]}`, `BracketGenerate {entry_order[]}`, `CancelReason`, `ChallengeJoinResult {challenge_id, match_id, room_id, starts_at}`; ngoại lệ oasdiff ở `specs/breaking-allowlist.yaml` (ADR-0074 — ba op `/lms/challenges*`, `/lms/catalog/challenge-templates` rời `learning.v1.yaml` sang `competition.v1.yaml`; các thay đổi enum response của `Tournament` không phải breaking).

- Chuyển `/lms/challenges`, `/lms/catalog/challenge-templates` sang `competition.v1.yaml` với `x-service: competition` (xóa khỏi `learning.v1.yaml`), thêm route ngoại lệ APISIX; cập nhật API_DESIGN §9.14 (ghi "Route ngoại lệ → module competition").
- **Delta ADR-0095 cho op learning — Đã áp 2026-11-22 (WP-R) trong bundle và `learning.v1.yaml`, ngoại lệ breaking ở `specs/breaking-allowlist.yaml` (F108 link tới mục này)** trên `learning.v1.yaml` (và bundle) trước khi chuyển sang `competition.v1.yaml`:
  - `listChallenges`: query `class_id` → `grade_level` (`GradeLevel`, 1–9); giữ `status`.
  - `ChallengeTemplate.kind` enum [`class_challenge`, `school_tournament`] → [`grade_challenge`, `school_tournament`].
  - `ChallengeCreate`: thêm `school_id` (Uuid, **bắt buộc**); `targets[]` (minItems 1, maxItems 2) item `{target_type: enum [grade, students], grade_level? (GradeLevel; bắt buộc khi `grade`), student_ids? (Uuid[], maxItems 500; bắt buộc khi `students`)}` thay `{target_type: class|group|grade_level|school, target_id}`; thêm `exclude_student_ids` (Uuid[], maxItems 500); mô tả: exclude thắng include, học sinh không thuộc `school_id` → `422 VALIDATION_FAILED`.
  - `ChallengeCreate.participation`: bỏ `study_group` (nhóm học tập LMS đã bỏ) → enum [`individual`] [Proposed — mở câu hỏi nếu cần thử thách theo đội].
- `ChallengeCreate`/`Challenge` (phần còn lại, chưa áp): `params.maxProperties 20`, mỗi giá trị chuỗi maxLength 255; thêm `kind` vào `Challenge` (`grade_challenge`, `school_tournament`), `school_id`, `targets[]`, `exclude_student_ids[]`, `match_id`, `cancel_reason` (maxLength 1000). *(Bản trước ADR-0095 ghi `target_type` `study_group` và `kind` `class_challenge`/`class_tournament` — đã bỏ.)*
- `TournamentCreate.scope_type` bỏ `class` (giải khối của trường = `scope_type = school` + `grade_level`) — **Đã áp 2026-11-21 (WP-S)** bởi lead trong `competition.v1.yaml` và bundle.
- `Tournament`/`TournamentCreate`: `status` enum theo vòng đời 11 trạng thái; `format` thêm `groups_then_single_elim`, bỏ `team` (thay bằng `participant_type` enum `individual`, `team`, `team_size`); `scope_type` đổi `organization` → `tenant`; thêm `grade_levels[]`, `seeding_policy`, `check_in_opens_at`, `my_entry`; thêm schema `TournamentEntry`, `BracketNode`, `Bracket {nodes[], groups[]}` (chuỗi có `maxLength`), `TournamentEntryCreate {student_ids[] maxItems 64, party_id}`, `BracketNodeOverride {winner_entry_id, reason maxLength 1000}`.
- Thêm `CompetitionChallenge` (view học sinh: `challenge_id`, `name`, `kind`, `starts_at`, `join_window_opens_at`, `status`) và response `joinChallenge` `{match_id, room_id}`.
- AsyncAPI: thêm `tournament.match.scheduled`, `tournament.bracket.updated`.
- Event payload schema mới (§4); bổ sung `tournament.rewarded` vào EVENT_DESIGN §4.8 (domain event là `tournament.match_scheduled` theo COMPETITION_SYSTEM §10.6 và EVENT_DESIGN §4.8; message WS 3 phần `tournament.match.scheduled` theo quy ước tên EVENT_DESIGN, RB-412).
- Tài liệu: DATABASE_DESIGN §2.10 cột tournament/entry/bracket; thêm `challenges`, `tournament_groups`, `tournament_results`, `tournament_rewards`.
