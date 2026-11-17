# Contracts: F103 — Boss battle nhiều phase và quest co-op (shared progress, bước `interdependent`)

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec nguồn: `specs/openapi/eduworld-api.v1.yaml` → `specs/openapi/modules/game.v1.yaml`. Mọi operation: `game_player`, `X-Tenant-Id`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listBosses` | `GET /api/v1/game/bosses?area_id=` | game.v1.yaml | `game_player` | Có sẵn |
| `createBossEncounter` | `POST /api/v1/game/bosses/{boss_id}/encounters` (`Idempotency-Key`) | game.v1.yaml | `game_player` | Cần sửa (chỉ `mode = solo`, bỏ `party_id`; thêm `422`) |
| `getBossEncounter` | `GET /api/v1/game/boss-encounters/{encounter_id}` | game.v1.yaml | thành viên encounter | Cần sửa (`BossEncounter` thêm `state_version`, `deadline_at`, `phase_retries_left`, `outcome`, `my_questions` khi resync) |
| `leaveBossEncounter` | `POST /api/v1/game/boss-encounters/{encounter_id}:leave` | game.v1.yaml | thành viên encounter | Có sẵn |
| `getQuest` / `startQuestActivity` | `/api/v1/game/quests/{quest_id}…` | game.v1.yaml | `game_player` | Cần sửa (`QuestDetail.steps.step_type` thêm `discover`, `interdependent`, `challenge`, `boss`; `QuestProgress` thêm `party_progress`) |
| `getQuestStepClues` | `GET /api/v1/game/quests/{quest_id}/steps/{step_id}/clues` | game.v1.yaml | thành viên phiên | Cần thêm |
| `shareQuestStepClue` | `POST /api/v1/game/quests/{quest_id}/steps/{step_id}/clues/{role}:share` (`Idempotency-Key`) | game.v1.yaml | người giữ role | Cần thêm |
| `assignQuestStepRoles` | `PUT /api/v1/game/quests/{quest_id}/steps/{step_id}/roles` (`Idempotency-Key`; body `{assignments: [{role ≤ 32, student_id}] ≤ 4}`) | game.v1.yaml | leader party | Cần thêm |

`ProblemCode` dùng: `CHALLENGE_NOT_ELIGIBLE`, `CHALLENGE_ATTEMPT_LIMIT`, `BOSS_ENCOUNTER_FULL`, `BOSS_PHASE_CLOSED`, `ROOM_NOT_MEMBER`, `PARTY_NOT_LEADER`, `PERMISSION_DENIED`, `NOT_FOUND`, `VALIDATION_FAILED`, `PRECONDITION_REQUIRED`, `GAME_ACCESS_STUDENT_ONLY`, `CALLER_NOT_ALLOWED`.

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `listPartyActivityCandidates` | competition → game (facade; body `{party_id, area_id? ≤ 64, members: [{student_id}] ≤ 8}` → `{data: [{activity_type: quest\|boss, activity_ref, title, min_members, max_members, all_eligible, members: [{student_id, eligible, reason_code}]}] ≤ 50}`) | game.internal.v1.yaml | `competition` | Cần thêm |
| `checkPartyActivityEligibility` | competition → game (facade; body `{party_id, activity_type, activity_ref, members}` → `{eligible, min_members, max_members, allow_continue_on_disconnect, members[]}`) | game.internal.v1.yaml | `competition` | Cần thêm |
| `startPartyActivity` | competition → game (facade; `Idempotency-Key = party_activity_id`; body `{party_activity_id, party_id, leader_id, activity_type, activity_ref, members: [{student_id, game_session_id}]}` → `201 {external_ref, room_type: boss\|private_instance, area_id ≤ 64, scene_ref ≤ 255}`) | game.internal.v1.yaml | `competition` | Cần thêm |
| `createQuestionPlan`, `scorePlanAnswer` | game → learning | learning.internal.v1.yaml | thêm `game` | Cần sửa (research R1) |
| `getLearningMaterialRenders` | game → learning (facade; batch `{refs: [{id, version}] ≤ 4}`) | learning.internal.v1.yaml | `game` | Cần thêm |

`reason_code` (enum chung): `grade_not_allowed`, `grade_band_mismatch`, `prerequisite_missing`, `area_locked`, `attempt_limit`, `party_size_out_of_range`, `sanctioned`.

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| `boss.encounter.ready`, `boss.answer.submit` | client → game (`ew.cmd.game.boss.*`) | Có sẵn |
| `boss.encounter.updated`, `boss.phase.started`, `boss.phase.completed`, `boss.encounter.ended` | game → thành viên (`ew:rt:room:<encounter_id>:out`) | Có sẵn |
| `boss.question.assigned` | game → một thành viên (`ew:rt:user:<id>`: `encounter_id`, `phase_id`, `requirement_id`, `questions[QuestionRender]`, `deadline_at`) | Cần thêm |
| `boss.answer.evaluated` | game → người trả lời (`question_instance_id`, `is_correct`, `explanation`) | Cần thêm |
| `quest.progress.updated` | game → thành viên | Cần sửa (thêm `party_progress {step_id, collective_progress 0–1, my_contribution_met}`) |
| `quest.roles.assigned`, `quest.clue.shared` | game → thành viên phiên | Cần thêm |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `boss.encounter_started`, `boss.phase_completed` | game → RT, LA, ANA | `specs/events/payloads/game/boss.encounter_started.v1.schema.json`, `boss.phase_completed.v1.schema.json` | Cần thêm |
| `boss.encounter_completed` | game → competition (F101), reward, achievement, QST, LA | `boss.encounter_completed.v1.schema.json` (+ `party_activity_id`) | Cần thêm |
| `quest.party_completed` | game → competition (F101), ANA | `quest.party_completed.v1.schema.json` | Cần thêm (event mới, EVENT_DESIGN §4) |
| `party.member_left`, `party.member_kicked`, `party.disbanded`, `party.leader_transferred` | competition → game | `specs/events/payloads/competition/` | Cần thêm (F100) |
| `activity.graded` | learning → game | `specs/events/payloads/learning/activity.graded.v1.schema.json` | Có sẵn |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/boss.schema.json` | Phase, requirement, `per_grade`, `completion_rule`, `reconnect_grace_seconds`, `max_phase_retries`, rewards | Có sẵn |
| `specs/schemas/quest.schema.json` | `party_mode`, `party_size`, `shared_progress_policy`, `assignee`, `collective_target`, `roles`, `role_assignment`, `allow_multi_role`, `share_mode`, `completion` | Có sẵn |

## 6. Delta cần thực hiện

- `game.v1.yaml` (qua `eduworld-api.v1.yaml`): sửa `createBossEncounter`, `BossEncounter`, `QuestDetail.steps.step_type`, `QuestProgress.party_progress`; thêm `getQuestStepClues`, `shareQuestStepClue`, `assignQuestStepRoles` (mọi chuỗi có `maxLength`).
- `game.internal.v1.yaml`: thêm ba operation party-activity (§2) + enum `reason_code`; facade test phía bên nhận (hiện thực facade với ví dụ trong spec) cho bên gọi F101.
- `learning.internal.v1.yaml` (nhóm D sở hữu): `x-allowed-callers` thêm `game` cho `createQuestionPlan`/`scorePlanAnswer`; `QuestionPlanRequest` thêm `context_type`, `context_id`, `activity_ref`, `attempt_mode`, `match_id` tùy chọn; thêm `getLearningMaterialRenders`.
- `eduworld-realtime.v1.yaml`: `boss.question.assigned`, `boss.answer.evaluated`, `quest.roles.assigned`, `quest.clue.shared`; `quest.progress.updated.party_progress`; `BossEncounterEnded.outcome` thống nhất với event (`defeated`/`time_up`/`abandoned`).
- `specs/events/payloads/game/`: 4 payload ở §4.
- Tài liệu (task Polish): CHALLENGE_BOSS_SYSTEM §7 (encounter party qua Party Lobby), §8 (consumer competition + `party_activity_id`); EVENT_DESIGN §4 (`quest.party_completed`); REALTIME_ARCHITECTURE §4.1 (`boss.*`, `quest.*` → `ew.cmd.game.*`); VALKEY_DESIGN (room `boss` owner game, key `ew:game:clue:*`); DATABASE_DESIGN §2.8a (cột/bảng mới).
- Kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=game`, `make test-contract`.
