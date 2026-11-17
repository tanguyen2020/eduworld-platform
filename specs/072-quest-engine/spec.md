# Feature Specification: F072 — Quest Engine: 12 loại quest, bước, tiến độ qua event, lọc theo khối

**Feature Branch**: `072-quest-engine` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `game` (+ module `learning` consumer, `godot/`) · **Mốc**: M2 · **Phụ thuộc**: F030, F071
**Input**: Quest là dữ liệu có version nối thế giới với Learning Engine: học sinh nhận quest đúng khối, làm bước tương tác do server xác minh và bước học tập tạo attempt riêng; tiến độ lưu server, cập nhật qua domain event; daily/weekly không phạt; phần thưởng chỉ là ý định gửi Reward Engine. FR nguồn: FR-QST-001…006, FR-QST-008.

## Clarifications

### Session 2026-11-16

- Q: Tenant thấy quest platform nào? → A: Chỉ quest có dòng gán active trong `infra.tenant_resource_assignments` của `eduworld_game` (ADR-0029, DATABASE_DESIGN §1 mục 8); seed gán cho 3 tenant demo. Quest không gán hoặc sai khối trả `404` (ẩn), không lộ tồn tại.
- Q: `quest_progress` dùng PK `(student_id, quest_version_id)` (DATABASE_DESIGN §2.8) hay `quest_progress_id` (QUEST_SYSTEM §2.2)? → A: `quest_progress_id` + unique partial "một active/quest/học sinh" + unique `(student_id, quest_id, period_key)`; PK cũ không cho quest `repeatable` lặp lại. Cập nhật DATABASE_DESIGN ở Polish.
- Q: Enum `QuestType`/`QuestProgressStatus` trong OpenAPI (7 loại cũ) lệch `quest.schema.json` v2? → A: Theo schema v2 và QUEST_SYSTEM §2.3, §5 (tài liệu sở hữu); sửa OpenAPI ở Phase 2.
- Q: Kiểm "đang ở area" cho `go_to`/`collect`/`talk_npc` trước hub realtime? → A: Dùng presence do `enterWorldArea` ghi (F071, port `PresenceReader`); F091 cập nhật nguồn presence, F072 không đổi.
- Q: Bước `activity` tham chiếu nội dung nào? → A: `content_ref.kind` `question_set`, `assessment` (có sẵn) và `activity` (mới, `activity_id` + version theo FR-CNT-013/ADR-0048); `lesson` bị validator từ chối cho tới khi internal `createAttempt` hỗ trợ.
- Q: Bước `minigame`, `party_activity`, `interdependent`, `challenge`, `boss`? → A: Engine có registry bộ xử lý bước; F072 đăng ký `talk_npc`, `go_to`, `collect`, `discover`, `activity`. Quest chứa bước chưa có bộ xử lý không publish được. `minigame` → F073; `challenge` → F076; `party_activity`, `interdependent`, `boss` → F103.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Thấy và nhận quest đúng khối (Priority: P1)

Học sinh lớp 3 gặp Ông Số, nhận nhiệm vụ "Phiên chợ của bác Gấu Nâu"; học sinh lớp 4 không thấy quest chỉ dành cho lớp 3 (US-QST-001).

**Why this priority**: Không có danh sách và nhận quest thì không có vòng lặp gameplay–học tập.

**Independent Test**: `hs.an.3a` `listQuests` thấy quest Toán lớp 3, `acceptQuest` → `QuestProgress` ghim version; `hs.em.4a` `getQuest` cùng quest → `404`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F072-AC-01 | KHI học sinh gọi `listQuests`, module game PHẢI chỉ trả quest published được gán cho tenant, có `grade_levels` chứa khối học sinh, trong `availability`, thuộc môn trường bật, lọc được theo `status`, `quest_type` (12 loại) và `area_id` | FR-QST-006 · FR-QST-002 · AC-QST-001 | TC-GAM-012 |
| F072-AC-02 | NẾU quest không gán cho tenant hoặc không có khối học sinh, THÌ `getQuest`, `acceptQuest` PHẢI trả `404 NOT_FOUND` | FR-QST-006 · AC-QST-001 · ADR-0029 | TC-TEN-019 |
| F072-AC-03 | KHI học sinh gọi `getQuest`, module game PHẢI trả bản delivery của version đã ghim (nếu đang làm) hoặc version hiện hành: tên, mô tả, narrative, NPC giao, khu, bước (tiêu đề, gợi ý, `optional`, `depends_on`), phần thưởng dự kiến, tiến độ — văn bản theo `Accept-Language`, đủ dữ liệu `vi`/`en` | FR-QST-001 · FR-QST-008 · NFR-I18N-002 | TC-GAM-012 |
| F072-AC-04 | KHI học sinh gọi `acceptQuest` (`Idempotency-Key`) cho quest khả dụng, module game PHẢI tạo `QuestProgress` `active` ghim `quest_version_id` theo con trỏ hiện hành, mở bước đầu theo `depends_on`, phát `quest.accepted`; gửi lại cùng key PHẢI trả cùng kết quả | FR-QST-001 · ADR-0043 · NFR-SEC-004 | TC-GAM-012 |
| F072-AC-05 | NẾU chưa thỏa `prerequisites` (quest trước, khu đã mở, `min_level`) hoặc `learning_requirements` (mức mastery LO), THÌ module game PHẢI trả `403 QUEST_PREREQUISITE_NOT_MET` kèm điều kiện còn thiếu thân thiện; quest học tập bắt buộc PHẢI không bị chặn bởi `min_level` | FR-QST-008 · AC-QST-011 | TC-GAM-012 |
| F072-AC-06 | NẾU học sinh đã có `max_active_quests` quest active, THÌ `acceptQuest` PHẢI trả `409 QUEST_ACTIVE_LIMIT`; NẾU quest này đang active, THÌ PHẢI trả progress hiện có | FR-QST-001 · [QUEST_SYSTEM §2.2](../../docs/01-game-design/QUEST_SYSTEM.md#22-questprogress) | — |
| F072-AC-07 | KHI học sinh gọi `abandonQuest` (`Idempotency-Key`), module game PHẢI chuyển progress sang `abandoned`, giữ nguyên attempt/learning record, phát `quest.abandoned` (`reason = player`); nhận lại PHẢI tạo progress mới theo version hiện hành | FR-QST-008 · [QUEST_SYSTEM §5](../../docs/01-game-design/QUEST_SYSTEM.md#5-state-machine) | — |
| F072-AC-08 | HỆ THỐNG PHẢI trả `403 GAME_ACCESS_STUDENT_ONLY` cho mọi route người chơi của feature với token nhân sự, và chỉ đọc/ghi `QuestProgress` của chính học sinh trong tenant phiên (`TenantScope`); `X-Tenant-Id` khác tenant PHẢI trả `403 TENANT_MISMATCH` | AGENTS R8, R23 · NFR-SEC-006 | TC-AUTH-014, TC-TEN-001 |

---

### User Story 2 — Bước tương tác do server xác minh (Priority: P1)

Học sinh nói chuyện NPC, đến sạp trái cây, nhặt vật thể; client chỉ báo, server kiểm vị trí và ID rồi ghi nhận.

**Why this priority**: Khung trải nghiệm của mọi quest; chống gian lận tiến độ (ADR-0016).

**Independent Test**: `reportQuestStep` `go_to` khi đang ở `learning_zone.math` → bước `completed`, bước kế `active`, WS `quest.progress.updated`; báo từ `main_hub` → `422 STEP_PRECONDITION_FAILED`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F072-AC-09 | KHI học sinh gọi `reportQuestStep` (`Idempotency-Key`) cho bước `talk_npc`/`go_to`/`collect` đang `active`, module game PHẢI kiểm `game_session_id` active, presence ở instance của `area_id` của bước, `npc_id`/`interactable_id` thuộc bước, rồi ghi nhận; hoàn thành bước PHẢI mở bước kế theo `depends_on`, phát `quest.step_completed` và đẩy `quest.progress.updated` | FR-QST-004 · FR-QST-005 · ADR-0016 | TC-WLD-006 |
| F072-AC-10 | NẾU không ở đúng area/instance hoặc ID không thuộc bước, THÌ module game PHẢI trả `422 STEP_PRECONDITION_FAILED` và không đổi tiến độ; NẾU bước đang `locked` hoặc đã `completed`, THÌ PHẢI trả `409 INVALID_STATE_TRANSITION` | FR-QST-004 · AC-QST-002 | TC-GAM-010 |
| F072-AC-11 | KHI báo lặp cùng `Idempotency-Key` hoặc cùng `interactable_id` của bước `collect`, module game PHẢI tính đúng một lần | FR-QST-005 · AC-QST-003 | — |
| F072-AC-12 | KHI nhận `world.discovery_found` khớp bước `discover` đang active, module game PHẢI tăng tiến độ bước đó; bước `discover` PHẢI không hoàn thành qua `reportQuestStep` | FR-QST-005 · [QUEST_SYSTEM §3](../../docs/01-game-design/QUEST_SYSTEM.md#3-step-types) | TC-WLD-006 |
| F072-AC-13 | HỆ THỐNG PHẢI không phát learning evidence, điểm hay mastery từ bước `talk_npc`/`go_to`/`collect`/`discover` | FR-QST-003 · [QUEST_SYSTEM §3](../../docs/01-game-design/QUEST_SYSTEM.md#3-step-types) | — |

---

### User Story 3 — Bước học tập tạo attempt riêng (Priority: P1)

Bước "Giải 8 bài toán có lời văn" mở activity player; server chấm; đạt `pass_rule` thì bước xong, không đạt thì được gợi ý ôn và thử lại với câu mới.

**Why this priority**: Giá trị học tập của quest nằm ở bước này (Q-3, Q-5).

**Independent Test**: `startQuestActivity` → attempt `mode = quest`; nộp đạt → nhận `activity.graded` → bước `completed`; nộp không đạt → bước vẫn `active`; gọi `reportQuestStep` cho bước học tập → `422`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F072-AC-14 | KHI học sinh gọi `startQuestActivity` (`Idempotency-Key`) cho bước `activity` active, module game PHẢI gọi `createAttempt` (mode `quest`, context `quest_step` = `quest_progress_id`, `quest_version_id`, `step_id`, `game_session_id`) với nội dung đã khóa trong quest version, lưu liên kết attempt–bước và trả `201 Attempt` | FR-QST-003 · ADR-0040 · ADR-0059 | TC-LRN-002 |
| F072-AC-15 | KHI nhận `activity.graded` của attempt đã liên kết với bước (tra `attempt_id` trong liên kết attempt–bước), module game PHẢI đánh giá `pass_rule` (`min_correct_ratio`, `min_questions`); đạt → bước `completed`; không đạt → bước vẫn `active`, đẩy `quest.progress.updated` kèm gợi ý ôn; event lặp PHẢI xử lý một lần | FR-QST-003 · FR-QST-005 · AC-QST-004 | TC-WLD-006, TC-SVC-003 |
| F072-AC-16 | NẾU client gọi `reportQuestStep` cho bước `activity`, THÌ module game PHẢI trả `422 STEP_PRECONDITION_FAILED`; bước học tập chỉ hoàn thành từ kết quả server chấm | FR-QST-003 · AC-QST-004 · ADR-0016 | TC-LRN-007 |
| F072-AC-17 | NẾU số lần bắt đầu bước trong ngày vượt `pass_rule.max_attempts_per_day`, THÌ module game PHẢI trả `409 QUEST_STEP_ATTEMPT_LIMIT` với thông điệp "hãy quay lại ngày mai" | FR-QST-008 · [QUEST_SYSTEM §3.1](../../docs/01-game-design/QUEST_SYSTEM.md#31-pass_rule-cho-bước-học-tập) | — |
| F072-AC-18 | NẾU module learning lỗi/timeout khi `createAttempt`, THÌ module game PHẢI trả `503 UNAVAILABLE` có `Retry-After`, không đổi tiến độ quest | FR-QST-003 · AGENTS R29 | TC-SVC-004, TC-SVC-010 |

---

### User Story 4 — Hoàn thành quest và tín hiệu phần thưởng (Priority: P1)

Hoàn thành đủ bước bắt buộc, quest chuyển "Đã xong", hệ thống phát tín hiệu cho Reward Engine và ghi learning event; học sinh thấy thông báo ngay.

**Why this priority**: Kết thúc vòng lặp; nối XP (F074), mở vùng/chương (F071), evidence học tập.

**Independent Test**: Hoàn thành bước cuối → `quest.completed` đúng 1 lần trong outbox, module learning ghi learning event `quest.completed`, WS `quest.progress.updated` `completed`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F072-AC-19 | KHI `completion_rule` (`all_required_steps` hoặc `any_n_of`) thỏa, module game PHẢI chuyển progress `completed`, ghi `completed_at` giờ server, phát `quest.completed` (`quest_version_id`, `quest_type`, `objective_ids`, `quest_progress_id`) đúng một lần trong cùng transaction; bước `optional` PHẢI không chặn hoàn thành | FR-QST-001 · FR-QST-005 · AC-QST-009 | TC-GAM-004 |
| F072-AC-20 | HỆ THỐNG PHẢI coi `rewards[]` của quest chỉ là ý định thưởng: quest engine không tự cộng XP/Xu/item; Reward Engine (F074) cấp theo reward rule khi tiêu thụ `quest.completed` | FR-QST-001 · [QUEST_SYSTEM §7](../../docs/01-game-design/QUEST_SYSTEM.md#7-phần-thưởng) | — |
| F072-AC-21 | KHI nhận `quest.completed` có `objective_ids`, module learning PHẢI ghi learning event `quest.completed` một lần cho mỗi `quest_progress_id` (mode theo ngữ cảnh) | FR-QST-003 · [EVENT_DESIGN §5.2](../../docs/07-backend/EVENT_DESIGN.md#52-catalog-learning-event) | TC-SVC-003 |

---

### User Story 5 — Daily và weekly quest không áp lực (Priority: P2)

Mỗi ngày học sinh có vài nhiệm vụ ngắn hợp lớp mình; bỏ lỡ không bị phạt (US-QST-002).

**Why this priority**: Daily/weekly loop (GAME_DESIGN §7.3), sau khi engine cơ bản chạy.

**Independent Test**: Vào game lần đầu trong ngày → nhận `daily_quests_per_day` daily quest đúng khối; vào lại cùng ngày → không nhận thêm; qua mốc `quest_reset_time` → quest cũ `expired`, kỳ mới có `period_key` mới.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F072-AC-22 | KHI học sinh bắt đầu game session đầu tiên trong một kỳ, module game PHẢI cấp `daily_quests_per_day` quest `auto_daily` và (đầu tuần) `weekly_quests_per_week` quest `auto_weekly` chọn theo khối, môn được bật và LO yếu, mỗi quest có `period_key`; cấp lặp trong cùng kỳ PHẢI không tạo thêm | FR-QST-006 · FR-QST-002 · [QUEST_SYSTEM §2.4](../../docs/01-game-design/QUEST_SYSTEM.md#24-daily-và-weekly-quest) | — |
| F072-AC-23 | KHI qua mốc `quest_reset_time`, module game PHẢI chuyển progress daily/weekly của kỳ cũ chưa xong sang `expired` (phát `quest.abandoned` `reason = expired`), không trừ XP/Xu, không có streak bị mất | FR-QST-008 · AC-QST-006 · NFR-SAFE-003 | — |

---

### User Story 6 — Version, lặp lại, thất bại và kiểm định khi phát hành (Priority: P2)

Quest đổi version không ảnh hưởng người đang làm; quest lặp lại theo cooldown; hết giờ chỉ đặt lại bước; quest sai cấu trúc không được phát hành.

**Why this priority**: Bảo toàn tiến độ và chất lượng nội dung (ADR-0043, FR-QST-008).

**Independent Test**: Publish v2 khi A đang làm v1 → A giữ v1, B nhận v2; quest `once` đã xong → `409 QUEST_ALREADY_COMPLETED`; publish quest thiếu `en` → `422`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F072-AC-24 | KHI con trỏ version quest đổi (publish/rollback) trong lúc học sinh đang làm, module game PHẢI giữ `quest_version_id` đã ghim tới khi hoàn thành/bỏ; học sinh nhận mới PHẢI nhận version theo con trỏ | FR-QST-001 · AC-QST-007 · AC-QST-008 | TC-CNT-001 |
| F072-AC-25 | NẾU nhận lại quest `once` đã hoàn thành, THÌ module game PHẢI trả `409 QUEST_ALREADY_COMPLETED`; NẾU quest `repeatable` còn trong `cooldown_hours`, THÌ PHẢI trả `409 QUEST_COOLDOWN` kèm thời điểm nhận lại; vượt `max_completions` PHẢI trả `409 QUEST_ALREADY_COMPLETED` | FR-QST-008 · AC-QST-010 | — |
| F072-AC-26 | KHI điều kiện thất bại xảy ra (`time_limit_exceeded`, `availability_ended`), module game PHẢI áp `on_fail`: `reset_step` (mặc định) chỉ đặt lại bước liên quan, giữ bước khác; `fail_quest` chuyển `failed`, cho nhận lại ngay nếu còn `availability`; không phạt XP/Xu | FR-QST-008 · AC-QST-013 | — |
| F072-AC-27 | HỆ THỐNG PHẢI từ chối publish quest (`422`) khi: sai `quest.schema.json` v2; thiếu `vi`/`en` ở văn bản người chơi thấy (`TRANSLATION_INCOMPLETE`); tham chiếu `npc_id`/`area_id`/`interactable_id`/nội dung không tồn tại hoặc chưa publish (`ITEM_REF_INVALID`); thiếu `objective_ids` cho loại bắt buộc; chuỗi câu hỏi liên tục vượt `learning_block_max_items` theo grade band; có bước chưa có bộ xử lý (`VALIDATION_FAILED`) | FR-QST-002 · FR-QST-008 · AC-GDD-001…005 | — |
| F072-AC-28 | NẾU chuỗi trong body vượt giới hạn (`npc_id`, `area_id`, `interactable_id` > 64 ký tự), THÌ module game PHẢI trả `422 VALIDATION_FAILED` | FR-QST-004 · ADR-0041 | — |

### Edge Cases

- NẾU attempt bị `expired`/`abandoned` (learning), THÌ bước vẫn `active`, không tính lần thử vào thất bại quest (Q-5).
- NẾU `activity.graded` tới cho progress đã `abandoned`/`expired`, THÌ ghi nhận đã xử lý, không đổi trạng thái.
- NẾU học sinh lên lớp giữa chừng, THÌ quest đang làm giữ nguyên tới khi xong/bỏ; danh sách mới lọc theo khối mới.
- NẾU quest bị gỡ gán khỏi tenant, THÌ không nhận mới được (404); progress đang chạy hoàn thành được trên version ghim.
- NẾU presence hết hạn giữa chừng, THÌ `reportQuestStep` trả `422` (F072-AC-10); client gọi lại `enterWorldArea`.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-QST-001 | QuestDefinition/Version đủ trường §2.1, QuestProgress, accept/abandon, completion, rewards ý định | [QUEST_SYSTEM §2, §5, §7](../../docs/01-game-design/QUEST_SYSTEM.md) |
| FR-QST-002 | 12 `quest_type` (schema v2) được lưu, lọc, kiểm định; hành vi riêng của `party`/`challenge`/`boss`/`seasonal`/`special_event` ở F076/F077/F103 | [QUEST_SYSTEM §2.3](../../docs/01-game-design/QUEST_SYSTEM.md#23-mười-hai-loại-quest) |
| FR-QST-003 | Bước `activity` tạo attempt `mode = quest` qua `createAttempt`; learning event `quest.completed` | [QUEST_SYSTEM §3, §8](../../docs/01-game-design/QUEST_SYSTEM.md), [ACTIVITY_ENGINE §9](../../docs/02-learning/ACTIVITY_ENGINE.md#9-api) |
| FR-QST-004 | Bộ xử lý `talk_npc`, `go_to`, `collect`, `discover`, `activity`; `minigame` (F073), `party_activity` (F103) qua registry | [QUEST_SYSTEM §3](../../docs/01-game-design/QUEST_SYSTEM.md#3-step-types) |
| FR-QST-005 | Tiến độ lưu server, hoàn thành bước qua domain event (outbox → NATS) | ADR-0019 · ADR-0042 |
| FR-QST-006 | Lọc theo `grade_level`, gán tenant, môn bật; daily/weekly chọn theo khối | [QUEST_SYSTEM §4](../../docs/01-game-design/QUEST_SYSTEM.md#4-start-triggers) |
| FR-QST-008 | Narrative, NPC, level/learning requirement, repeatability, cooldown, failure (`reset_step` mặc định) | [QUEST_SYSTEM §2.1, §2.5](../../docs/01-game-design/QUEST_SYSTEM.md) |

### Key Entities

- **QuestDefinition / QuestVersion**: quest platform có version bất biến khi publish (schema v2), gán tenant.
- **QuestProgress**: tiến độ của học sinh trên một version ghim: trạng thái, trạng thái từng bước, kỳ (`period_key`), lần hoàn thành.
- **QuestStepAttempt**: liên kết attempt học tập ↔ bước, ngày bắt đầu, kết quả `pass_rule`.
- **PeriodicQuestGrant**: bản ghi cấp daily/weekly theo kỳ.
- **StepHandler**: bộ xử lý theo loại bước (registry).

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `max_active_quests` | Giới hạn quest active đồng thời |
| `daily_quests_per_day` / `weekly_quests_per_week` | Số quest cấp mỗi kỳ |
| `quest_reset_time` | Mốc reset daily/weekly (ICT) |
| `quest_pass_rule` | Giá trị mặc định khi quest không khai báo `pass_rule` |
| `learning_block_max_items` | Kiểm chuỗi câu hỏi liên tục khi publish |
| `attempt_resume_window` | Hiển thị resume bước học tập |
| `idempotency_record_ttl` | `acceptQuest`, `abandonQuest`, `reportQuestStep`, `startQuestActivity` |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Tiến độ quyết ở server; client chỉ báo sự kiện | ADR-0016 · AGENTS R5 |
| Gọi learning chỉ qua facade `learning` (`pkg/modcall`); phản ứng qua event | AGENTS R29 · ADR-0074 · ADR-0042 |
| Thao tác ghi quan trọng có `Idempotency-Key` + audit | AGENTS R10 · NFR-SEC-004 |
| Không streak phạt, thất bại không mất tiến độ | NFR-SAFE-003 · AGENTS R19 |
| Văn bản quest đủ vi/en trước publish | NFR-I18N-002 · ADR-0056 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 0 trường hợp học sinh thấy/nhận quest sai khối hoặc chưa gán tenant trong test duyệt toàn bộ khối 1–9.
- **SC-002**: 0 bước học tập hoàn thành khi attempt không đạt `pass_rule` (test giả mạo client).
- **SC-003**: `quest.completed` phát đúng 1 lần/progress dưới 1.000 lần replay event.
- **SC-004**: Học sinh thấy tiến độ cập nhật trong ≤ 2 giây sau khi nộp bài (p95, môi trường staging).

## Ngoài phạm vi

- Bước `minigame` → F073; `challenge` → F076; `party_activity`, `interdependent`, `boss`, shared progress, `party_required` → F101/F103; `special_event`/`seasonal` gắn EventRun → F077.
- Cấp XP/item/badge theo `rewards[]`, cap, diminishing → F074/F075; quest do nhà trường giao (`start_trigger = assignment`) → F041/F055 + catalog F078.
- Game Studio soạn/duyệt/rollback quest, API `/game-admin/quests` → F078/F079; gợi ý ôn LO yếu (nội dung) → F032.

## Assumptions

- F030 có `createAttempt` internal và event `activity.graded` (`attempt_id`, `correct_count`, `item_count`, `passed`); F071 có lõi phát hành, presence, `ref.mastery_summary`.
- Chọn daily/weekly "theo LO yếu" dùng `ref.mastery_summary` mức `beginning`/`developing` [Proposed]; chủ đề tuần do nhà trường chọn → F055.
- Seed 3 quest chỉ dùng bước của F072 (chuỗi main quest chương 0, một quest Toán lớp 3, một daily) và gán 3 tenant demo; quest mẫu "Phiên chợ của bác Gấu Nâu" có bước `minigame` do F073 seed.
