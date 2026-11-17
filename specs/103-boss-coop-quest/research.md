# Research: F103 — Boss battle nhiều phase và quest co-op (shared progress, bước `interdependent`)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Chấm câu trả lời boss

- **Decision**: đầu mỗi lượt phase, với từng thành viên module game gọi `createQuestionPlan` (learning internal) với `context_type = boss_encounter`, `context_id = encounter_id`, `activity_ref` của requirement theo khối người đó (`per_grade`), `attempt_mode = challenge`, `participants = [student_id]`; lưu `plan_id` vào `game.boss_phase_plans`. `boss.answer.submit` → `scorePlanAnswer(plan_id, index, student_id, response, received_at)` (idempotent theo `(plan_id, index, student_id)`); kết quả lưu `game.boss_answer_results` (unique cùng khóa) rồi cộng đóng góp.
- **Rationale**: WS `boss.answer.submit` (AsyncAPI) cần chấm từng câu ngay để cập nhật năng lượng; luồng question plan đã có cho trận đấu và ghi `attempt_answer` đúng học sinh (REALTIME §7.1); không lộ đáp án (ADR-0016).
- **Alternatives**: (a) `createAttempt` + học sinh nộp qua API attempt public, module game chờ `activity.graded` — chỉ có kết quả khi nộp cả attempt, không cập nhật boss theo từng câu; (b) module game tự chấm — vi phạm ADR-0016.
- **Contract**: `learning.internal.v1.yaml` — `x-allowed-callers` của `createQuestionPlan`, `scorePlanAnswer` thêm `game`; `QuestionPlanRequest` thêm `context_type` (`match`/`boss_encounter`), `context_id`, `activity_ref`, `attempt_mode` (`competition`/`challenge`), `match_id` thành tùy chọn. Nhóm D (module learning) hiện thực.

## R2. Encounter party chỉ qua Party Lobby

- **Decision**: encounter `party` tạo bởi internal `startPartyActivity` (module competition gọi trong `startPartySession`, F101); public `createBossEncounter` chỉ `mode = solo` (bỏ `party_id`).
- **Rationale**: FR-PTY-007 yêu cầu luồng 9 bước (Ready, điều kiện, party `in_activity`); hai đường tạo sẽ cho hai cơ chế Ready và party không biết đang trong boss.
- **Alternatives**: giữ `mode = party` cho leader gọi thẳng — cần module game đọc membership party (projection mới) và đồng bộ ngược trạng thái party.
- **Hệ quả**: sửa CHALLENGE_BOSS_SYSTEM §7 (dòng `POST /game/bosses/{boss_id}/encounters`) và `game.v1.yaml`.

## R3. Thẻ dữ kiện `interdependent`

- **Decision**: khi bước bắt đầu, module game lấy bản render các `clue_content_ref` qua learning internal `getLearningMaterialRenders` (batch, theo version ghim; Cần thêm), cache `ew:game:clue:<quest_version_id>:<step_id>:<role>` (TTL 1 h, Rebuildable). `getQuestStepClues` trả thẻ của role người gọi + thẻ đã chia sẻ (`shared_clues`). Message `quest.roles.assigned`, `quest.clue.shared` đẩy qua `ew:rt:user:<member_id>`.
- **Rationale**: không cho client tải dữ kiện theo id qua API public (sẽ lấy được thẻ của role khác); server là nơi duy nhất quyết định ai thấy gì (QUEST_SYSTEM §3.2).
- **Alternatives**: nhúng dữ kiện vào snapshot quest version — snapshot là chung cho mọi người (lộ thẻ).

## R4. Lưu tiến độ chung

- **Decision**: bảng mới `game.party_quest_sessions` (một dòng cho một `party_activity_id`: trạng thái bước chung, phân vai, thẻ đã chia sẻ, danh sách thành viên) và `game.party_quest_contributions` (đóng góp theo `(session, student, step)`), cập nhật trong cùng transaction với `QuestProgress` của các thành viên (tuần tự hóa `SELECT … FOR UPDATE` dòng session).
- **Rationale**: PARTY_SYSTEM §6.1 bước (5) cần "shared progress record"; DATABASE_DESIGN chưa có bảng; tiến độ quan trọng không chỉ ở Valkey (R9).
- **Alternatives**: suy ra từ `quest_progress.step_states` của mọi thành viên mỗi lần đọc — race khi cập nhật song song, không có chỗ cho phân vai/thẻ.

## R5. Chạy encounter

- **Decision**: encounter dùng `pkg/roomhost` (F076): lease `ew:rt:room:<encounter_id>:lease`, checkpoint `ew:rt:room:<encounter_id>:ckpt` mỗi chuyển trạng thái, output `ew:rt:room:<encounter_id>:out`; lệnh `ew.cmd.game.boss.ready|answer` (reply `boss.encounter.ready` trả `subscribe: [ew:rt:room:<id>:out]`); câu hỏi riêng và kết quả câu gửi `ew:rt:user:<student_id>`. Timer phase/grace chạy trong owner, khôi phục từ `deadline_at` lưu PG.
- **Rationale**: CHALLENGE_BOSS_SYSTEM §4 (module game, dùng chung room host); VALKEY_DESIGN ghi `ew:rt:room:*` thuộc competition — cần thêm game là owner cho room `boss`.
