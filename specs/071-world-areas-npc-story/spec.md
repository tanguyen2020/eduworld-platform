# Feature Specification: F071 — Thế giới: 8 vùng đất, zone, world map, cổng, mở khóa, mạch truyện, NPC và hội thoại

**Feature Branch**: `071-world-areas-npc-story` · **Created**: 2026-11-16 · **Status**: In progress · **Module**: module `game` (+ `src/game/`) · **Mốc**: M2 · **Phụ thuộc**: F026, F070, F091
**Input**: Thế giới "Quần đảo Tri Thức" 8 vùng bao trọn 7 khu chức năng; học sinh xem bản đồ, đi qua cổng gió, vào khu theo quyền do server quyết; vùng phiêu lưu mở theo quest/level/LO, khu học tập luôn mở; mạch truyện 7 chương; khám phá và % hoàn thành bản đồ; NPC hội thoại data-driven có version. FR nguồn: FR-GAME-001, FR-GAME-002, FR-GAME-003, FR-GAME-006, FR-GAME-016, FR-GAME-017, FR-GAME-018, FR-GAM-009.

## Clarifications

### Session 2026-11-16

- Q: Ai dựng nền phát hành nội dung game (con trỏ version, snapshot, content manifest tenant) khi Game Operations Console (F078) ở M3? → A: F071 dựng lõi dùng chung: bảng định danh + version theo mẫu DATABASE_DESIGN §3.13, command nội bộ `PublishGameContent` (dùng bởi lệnh seed), snapshot, cập nhật `game.content_manifests`, event `content.published`. F078 thêm workflow duyệt, API `/game-admin/*`, rollback/unpublish trên lõi này.
- Q: `enterWorldArea` gán instance thế nào khi hub realtime (F091) chưa có? → A: Port `InstanceAllocator`; bản v0 trả một room logic theo `(tenant, school, area, grade_band)` — đúng AC-WLD-003 (không trộn trường), chưa cân tải `hub_instance_capacity`. Gắn nhãn rõ trong code; F091 thay adapter theo presence.
- Q: Server kiểm "học sinh đang ở area" (khám phá, bước quest) bằng gì trước F091? → A: Presence tạm trong Valkey ghi khi `enterWorldArea` thành công (ADR-0004: dữ liệu tạm), port `PresenceReader`; F091 cập nhật presence từ realtime.
- Q: Điều kiện `objective_progress` đọc mastery từ đâu? → A: Projection `ref.mastery_summary` (mới) từ event `learning_objective.progressed` của module learning; không gọi đồng bộ (R29).
- Q: Tên vùng/chương lưu `display_name varchar(128)` (DATABASE_DESIGN §2.8a) hay song ngữ? → A: Song ngữ `{vi, en}` bắt buộc (ADR-0056, NFR-I18N-002); cột `name_i18n jsonb`, cập nhật DATABASE_DESIGN ở Polish.

### Session 2026-11-17

- Q: REST `joinFriendInWorld` và config type `levels`? → A: Gỡ `joinFriendInWorld` khỏi contract — "Tham gia bạn" chỉ qua WS `hub.instance.join {follow_user_id}` (API_DESIGN, RB-625). Enum `levels` đổi thành `progression-levels` (bảng level của F074, RB-308); config type cho gói level `game.level_definitions` chưa có trong API_DESIGN — owner chốt 2026-11-20: `level-packages` là tên chính thức.

### Session 2026-11-20 (owner chốt)

- Config type gói level → **Chốt:** `level-packages` là tên chính thức (API_DESIGN ghi theo — agent docs cập nhật).
- OQ-16 → đã trả lời (owner 2026-11-20): stylized low-poly; zone greybox vẫn được chấp nhận ở M2 nếu đạt lint và ngân sách.

### Session 2026-11-20 (owner chốt — vị trí hub)

- Facade vị trí hub (SECURITY_ARCHITECTURE: "competition: vị trí hub — operation khai báo ở F071, dữ liệu từ F091") → **Chốt:** F071 khai báo operation trong spec internal của module competition (bên gọi `game`, kiểm khoảng cách tương tác có giá trị) và hiện thực bên nhận (module competition) cùng adapter phía game; dữ liệu vị trí do room host của F091 cung cấp → F071 phụ thuộc F091.

### Session 2026-11-21 (triển khai — [Confirmed], owner chốt 2026-11-20)

Quyết định khi triển khai (owner đã duyệt trước các mặc định đề xuất; phương án bảo thủ nhất):

- **`enterWorldArea` và hub F091** → `room_id` là phòng phân vùng v0 = UUIDv5 của `(tenant, school, area, grade_band)` (ALLOCATOR TẠM, gắn nhãn trong code); instance hub thật do room host F091 gán qua `hub.instance.join` với cùng phân vùng. Không trộn trường (F071-AC-09) được bảo đảm ở cả hai nơi.
- **Sự kiện điều kiện mở khóa** → bảng mới `game.world_unlock_facts` (T) ghi một lần các sự kiện tiến trình mà module game chưa có bảng sở hữu (quest đã xong — tới F072, huy hiệu — F075, tham gia sự kiện — F077, level cao nhất — F074). Một consumer `game.unlock_on_events` (thay vì hai consumer `unlock_on_events` + `story_on_quest_completed` của plan) để chương truyện và mở khóa do chương đó gây ra được áp trong cùng một transaction.
- **Payload tiêu thụ chưa có schema** → consumer đọc tối thiểu: `quest.completed` cần `student_id`, `quest_id` (F072 phải có `quest_id` trong payload — Câu hỏi mở F071-2); `level.reached` `to_level`; `achievement.unlocked` `achievement_id`; `event.participation_started` `event_id` hoặc `event_run_id`. Không tạo schema thay feature sở hữu.
- **Môn bị trường tắt** → đọc khóa `enabled_subject_codes` (mảng mã môn; vắng = mọi môn bật) trong `ref.schools.settings_subset` (đã được `game.projection_identity` chiếu từ `school.updated`). Module identity chưa phát khóa này — Câu hỏi mở F071-1 (F015 thêm cài đặt bật/tắt môn của trường).
- **Tham chiếu của dialogue** → NPC hợp lệ = NPC đặt trong layout của một area đã publish (hoặc cùng lô); quest hợp lệ (tới khi F072 có bảng quest) = main quest của các chương đã publish. Mỗi NPC chỉ một dialogue publish cho một khối (trùng khối → `422 VALIDATION_FAILED`).
- **Item khóa chuỗi trong manifest / pin level** → `ContentManifestItem.item_id` là UUID nên area và chương dùng UUIDv5 của `<item_type>:<mã>`, thêm trường tùy chọn `item_code` (mã ổn định). Manifest tenant dựng lại qua consumer `game.manifest_on_game_content` (mọi tenant active), ETag gồm `manifest_version` nên đổi ở mỗi lần publish.
- **`GameConfigType`** (`progression-levels`, `level-packages`) → không đổi enum ở F071 vì kéo theo danh mục action portal (`x-portal-action.map`) của API `/game-admin/*`; F078 đổi cùng lúc hiện thực API cấu hình.
- **Bundle zone** → seed dùng URL placeholder `https://cdn.eduworld.example/...`, `size_bytes = 0`, `placeholder: true` (R17, OQ-16); client coi bundle "sẵn sàng" khi scene greybox có trong bản build; tải PCK theo `url`/`sha256` làm cùng asset thật. Snapshot nội dung game chỉ cache Valkey `ew:game:cv:*` (PostgreSQL là nguồn); không upload object storage vì client đọc qua API.
- **Schema** → `world-area.schema.json` bắt buộc `max_speed_mps`, `bounds` (room host F091 cần), thêm kind `exam_board` (bảng thi F064); `npc-dialogue.schema.json` thêm `npc_name {vi,en}` tùy chọn (tên NPC là dữ liệu).
- **Khoảng cách tương tác** → `interact_radius_default` 3 m (khi anchor không có `interact_radius_m`), `interact_distance_tolerance` 1,5 m; chỉ áp khi facade vị trí hub (T049) trả vị trí trong cùng area, không có thì chỉ kiểm presence.
- **Level session** → `startLevel` mới khi còn level session active: phiên cũ chuyển `expired` (một active/học sinh). Cổng gió tới khu bị ẩn (môn tắt) không trả về trong `portals`.
- **HTTP status của ** → 409 theo bảng mã lỗi sở hữu ở API_DESIGN §4.1 (registry `pkg/problem`); "422" trong F071-AC-25 và WORLD_DESIGN §12 đọc là mã `STEP_PRECONDITION_FAILED` với status của registry (WORLD_DESIGN §12 đã sửa theo).
- **Câu hỏi mở F071-1** → module identity (F015) chưa có cài đặt bật/tắt môn của trường; F071 đọc khóa `enabled_subject_codes` trong `settings_subset` của `school.updated` khi có (vắng = mọi môn bật). Cần F015/F010 thêm khóa vào cài đặt trường và schema `school.updated`.
- **Câu hỏi mở F071-2** → payload `quest.completed` (F072) phải mang `quest_id` (EVENT_DESIGN hiện chỉ ghi `quest_version_id`); consumer `game.unlock_on_events` từ chối event thiếu `quest_id` (DLQ).
- **Test** → test tích hợp nằm phẳng trong `src/backend/modules/game/tests/integration/world_*_test.go` (cùng môi trường testcontainers của module) thay cho thư mục con `world/`, `content/` của tasks.md.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Danh sách vùng đất và quyền vào theo học sinh (Priority: P1)

Học sinh mở game thấy 8 vùng đất với tên song ngữ; khu học tập theo khối/môn của mình luôn mở; môn bị trường tắt không hiện (US-GAME-002).

**Why this priority**: Mọi di chuyển, quest, khám phá dựa trên danh sách area và trạng thái mở khóa do server quyết.

**Independent Test**: `hs.an.3a` level 1, ví 0 → `listWorldAreas` có mọi `area_id` chuẩn, `learning_zone.math` `accessible = true`, `quest_zone.discovery_ocean` khóa kèm điều kiện.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F071-AC-01 | KHI học sinh gọi `listWorldAreas`, module game PHẢI trả các area của 8 vùng (WORLD_DESIGN §3) với `area_id` chuẩn, `region_code`, `name {vi,en}`, `accessible`, `lock_reason`, bao trọn 7 khu chức năng A–G | FR-GAME-002 · AC-WLD-001…004 | TC-WLD-001 |
| F071-AC-02 | HỆ THỐNG PHẢI cho khu học tập theo khối và môn của học sinh `accessible = true` bất kể level, Xu, chương truyện; không có điều kiện thanh toán nào trong `unlock_rule` | FR-GAM-009 · FR-GAME-017 · AC-GAM-009 | TC-GAM-009, TC-WLD-005 |
| F071-AC-03 | TRONG KHI một môn bị trường tắt trong cài đặt trường, module game PHẢI ẩn area học tập của môn đó khỏi `listWorldAreas`, `getWorldManifest`, `getWorldExploration` | FR-GAME-002 · [WORLD_DESIGN §7.1](../../docs/01-game-design/WORLD_DESIGN.md#71-nguyên-tắc) | — |
| F071-AC-04 | KHI học sinh gọi `getWorldManifest`, module game PHẢI trả area, scene, bundle PCK (`url`, `sha256`, `size_bytes`) theo version hiện hành; đổi tên/vị trí vùng qua dữ liệu PHẢI hiện ra mà không build lại client | FR-GAME-006 · [WORLD_DESIGN §3](../../docs/01-game-design/WORLD_DESIGN.md#3-bản-đồ-vùng-đất) | TC-WLD-001 |
| F071-AC-05 | KHI nội dung thế giới (area, chương, NPC dialogue, level) được publish version mới, module game PHẢI tăng `manifest_version` của mọi tenant, đổi ETag và phát `content.published`; học sinh đang trong level session PHẢI tiếp tục dùng version đã ghim | FR-GAME-006 · ADR-0043 | TC-CNT-001, TC-CNT-005 |
| F071-AC-06 | HỆ THỐNG PHẢI trả `403 GAME_ACCESS_STUDENT_ONLY` cho mọi route người chơi của feature khi token là nhân sự, và chỉ đọc/ghi unlock, khám phá, chương, level session trong tenant của phiên (`TenantScope`) | AGENTS R8, R23 · NFR-SEC-006 | TC-AUTH-014, TC-TEN-001 |

---

### User Story 2 — Đi qua cổng gió và vào khu (Priority: P1)

Học sinh chạm cổng gió hoặc chọn vùng trên bản đồ; server cho vào khu đã mở, gán instance cùng trường, ghim version nội dung của màn; khu chưa mở trả điều kiện thân thiện.

**Why this priority**: Cửa vào mọi hoạt động trong thế giới; là điều kiện kiểm "đang ở area" cho khám phá và quest.

**Independent Test**: `enterWorldArea("learning_zone.math")` → `200` room + spawn; `startLevel` → version ghim; `enterWorldArea("challenge_mountain")` khi chưa đủ chương → `403 AREA_LOCKED` + điều kiện.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F071-AC-07 | KHI học sinh gọi `enterWorldArea` cho area đã mở với `game_session_id` active của mình, module game PHẢI trả `room_id`, `room_type`, `spawn_point_id`, `level_id`, ghi presence tạm và `last_area_id` (chỉ khu công khai) | FR-GAME-017 · ADR-0012 | TC-WLD-002 |
| F071-AC-08 | NẾU area chưa mở, THÌ module game PHẢI trả `403 AREA_LOCKED` kèm danh sách điều kiện còn thiếu (`kind`, `ref`, `label {vi,en}`, `met`) và không ghi presence | FR-GAME-017 · AC-GAM-008 | TC-GAM-008, TC-WLD-002 |
| F071-AC-09 | HỆ THỐNG PHẢI không gán học sinh vào hub instance chứa học sinh trường khác (mặc định) | FR-GAME-017 · AC-WLD-003 · ADR-0054 | — |
| F071-AC-10 | NẾU `game_session_id` không active hoặc không thuộc học sinh, THÌ module game PHẢI trả `409 CONFLICT` cho `enterWorldArea`/`startLevel` | FR-GAME-017 · ADR-0031 | — |
| F071-AC-11 | KHI học sinh gọi `startLevel` (`Idempotency-Key`), module game PHẢI tạo level session ghim `level_version_id`, `manifest_version`, `content_versions` theo con trỏ hiện hành (một level session active/học sinh); `endLevelSession` PHẢI bỏ ghim | FR-GAME-006 · ADR-0043 | TC-CNT-001 |
| F071-AC-12 | KHI chuyển vùng qua cổng gió trên thiết bị hạng Tầm trung, game client PHẢI hoàn tất (gồm tải bundle đã cache, đặt spawn) trong `area_transition_max`; spawn không tồn tại trong scene PHẢI rơi về `default` và log cảnh báo | FR-GAME-017 · NFR-PERF-005 · AC-WLD-002 | TC-PERF-020 |
| F071-AC-13 | HỆ THỐNG PHẢI chỉ chấp nhận zone scene qua lint CI (ID `SpawnPoint`/`Interactable`/`AreaPortal` duy nhất, tham chiếu `area_id` hợp lệ, material từ preset token) và checklist an toàn nội dung (không vũ khí, bạo lực, biểu tượng không phù hợp) | FR-GAME-001 · ADR-0047 · [WORLD_RENDERING §10](../../docs/08-game-client/WORLD_RENDERING.md#10-kiểm-thử--nghiệm-thu-một-zone) | TC-PERF-018 |

---

### User Story 3 — Mở khóa vùng theo tiến trình (Priority: P1)

Hoàn thành main quest chương 0, đủ chương môn học, LO đạt mức, level, huy hiệu… thì vùng mới mở, có hiệu ứng; không bao giờ mở bằng tiền.

**Why this priority**: World progression là phần thưởng chính của tiến bộ (FR-GAM-009).

**Independent Test**: Phát event `quest.completed` của main quest chương 0 → `world_area_unlocks` có dòng, `world.area_unlocked` một lần, WS `world.area.unlocked`; phát lại event → không thêm.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F071-AC-14 | KHI nhận event làm thay đổi điều kiện (`quest.completed`, `story.chapter_completed`, `level.reached`, `learning_objective.progressed`, `achievement.unlocked`, `event.participation_started`), module game PHẢI đánh giá `unlock_rule` (AND/OR của `quest_completed`, `chapters_completed`, `objective_progress`, `min_level`, `achievement_unlocked`, `event_participation`) cho học sinh đó | FR-GAM-009 · [WORLD_DESIGN §7.2](../../docs/01-game-design/WORLD_DESIGN.md#72-điều-kiện-mở-khóa-worldareaunlock_rule) | TC-GAM-008 |
| F071-AC-15 | KHI `unlock_rule` thỏa lần đầu, module game PHẢI ghi một dòng mở khóa (kèm `rule_version`), phát `world.area_unlocked` và đẩy `world.area.unlocked` tới học sinh; event giao lặp PHẢI không tạo dòng/event thứ hai | FR-GAM-009 · AC-GAM-008 | TC-WLD-002, TC-SVC-003 |
| F071-AC-16 | HỆ THỐNG PHẢI từ chối publish `unlock_rule` có `min_level` hoặc `event_participation` trên area học tập, và PHẢI không có loại điều kiện thanh toán | FR-GAM-009 · [WORLD_DESIGN §7](../../docs/01-game-design/WORLD_DESIGN.md#7-mở-khóa-và-gating) | — |

---

### User Story 4 — NPC và hội thoại (Priority: P2)

Học sinh nói chuyện với Mây, Ông Số, Bà Chữ…; hội thoại song ngữ theo version, có lựa chọn; hành động nghiệp vụ (nhận quest, mở hoạt động) đi qua server; học xong quay lại đúng chỗ trong thế giới.

**Why this priority**: NPC là cầu nối kể chuyện và dẫn vào học tập (FR-GAME-003, FR-GAME-018); quest (F072) dùng `talk_npc` và `give_quest`.

**Independent Test**: `getNpcDialogue("npc.mr_numbers")` → graph version publish; client chạy hội thoại, action `open_minigame` mở activity, đóng màn tổng kết quay về vị trí NPC.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F071-AC-17 | KHI học sinh gọi `getNpcDialogue`, module game PHẢI trả dialogue version đang publish theo `npc-dialogue.schema.json`, lọc `grade_levels` theo khối học sinh, `ETag` theo version; NPC không tồn tại hoặc không có version cho khối PHẢI trả `404` | FR-GAME-006 · FR-GAME-003 · [WORLD_DESIGN §2.3](../../docs/01-game-design/WORLD_DESIGN.md#23-npc-chính-confirmed--người-duyệt-2026-11-15-adr-0056) | — |
| F071-AC-18 | HỆ THỐNG PHẢI từ chối publish dialogue thiếu bản `vi` hoặc `en` ở bất kỳ node/choice nào (`422 TRANSLATION_INCOMPLETE`) hoặc tham chiếu `npc_id`/`quest_id` không tồn tại (`422 ITEM_REF_INVALID`) | NFR-I18N-002 · ADR-0056 | — |
| F071-AC-19 | KHI node hội thoại có action `give_quest`, `complete_step`, `open_activity`, `open_minigame`, game client PHẢI gọi API server tương ứng (quest: F072; activity: F063/F073) và chỉ hiển thị kết quả server trả; điều kiện node chỉ dùng để hiển thị | FR-GAME-003 · ADR-0016 | TC-GAM-012 |
| F071-AC-20 | KHI một hoạt động học tập mở từ NPC/trạm trong thế giới kết thúc, game client PHẢI hiện màn tổng kết rồi trả học sinh về đúng vị trí trong zone, không tự nối sang hoạt động khác | FR-GAME-018 · [GAME_DESIGN §7.2](../../docs/01-game-design/GAME_DESIGN.md#72-core-loop--một-phiên-1530-phút) | — |

---

### User Story 5 — Mạch truyện "Thắp lại Tia Sáng" (Priority: P2)

Chương 0 mở lần đầu vào game; chương môn học 1–4 mở song song sau chương 0; chương 5 khi xong ≥ 1 chương môn học, chương 6 khi xong ≥ 2.

**Why this priority**: Cho học sinh mục tiêu dài hạn mà không khóa học tập (FR-GAME-016).

**Independent Test**: Hoàn thành main quest chương 0 → chương 1–4 `available`; xong chuỗi main quest chương 1 → `story.chapter_completed` một lần, chương 5 mở.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F071-AC-21 | KHI học sinh hoàn thành main quest đầu tiên của chương, module game PHẢI chuyển chương sang `in_progress`; KHI hoàn thành mọi main quest của chương, PHẢI chuyển `completed` và phát `story.chapter_completed` đúng một lần (event lặp không phát lại) | FR-GAME-016 · [WORLD_DESIGN §2.2](../../docs/01-game-design/WORLD_DESIGN.md#22-mạch-truyện-chính-confirmed--người-duyệt-2026-11-15-adr-0056) | TC-WLD-004 |
| F071-AC-22 | HỆ THỐNG PHẢI mở song song các chương môn học sau chương 0 (chương của môn bị trường tắt không hiện); chưa xong chương Toán vẫn vào được chương Tiếng Việt | FR-GAME-016 · FR-GAM-009 | TC-WLD-005 |
| F071-AC-23 | KHI học sinh gọi `getWorldExploration`, module game PHẢI trả `chapters[]` (`chapter_code`, `region_code`, `title {vi,en}`, `order`, `status` locked/available/in_progress/completed, `main_quests_completed/total`) | FR-GAME-016 | TC-WLD-004 |

---

### User Story 6 — Khám phá và bản đồ thế giới (Priority: P2)

Học sinh tìm điểm khám phá, vật sưu tầm; bản đồ thế giới hiện % khám phá từng vùng, điều kiện mở vùng; khối 1–3 dịch chuyển bằng bảng chọn hình 1 chạm.

**Why this priority**: Pillar "thế giới đáng khám phá" (FR-GAME-017); phụ thuộc presence (US2).

**Independent Test**: Trong `learning_zone.math`, `reportDiscovery("discovery.math.old_well")` → `completion_pct` tăng; gọi lại → `409 DISCOVERY_ALREADY_FOUND`; gọi từ area khác → `422 STEP_PRECONDITION_FAILED`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F071-AC-24 | KHI học sinh gọi `reportDiscovery` (`Idempotency-Key`) khi presence đang ở instance của `area_id` và `interactable_id` là `discovery`/`collectible` hợp lệ trong level version đã ghim, module game PHẢI ghi một lần, cập nhật `completion_pct`, phát `world.discovery_found` (và `world.area_completed` khi đạt 100%) | FR-GAME-017 · AC-GAM-010 | TC-GAM-010, TC-WLD-003 |
| F071-AC-25 | NẾU không ở đúng area/instance hoặc ID không hợp lệ, THÌ module game PHẢI trả `422 STEP_PRECONDITION_FAILED`; NẾU điểm đã tìm, THÌ PHẢI trả `409 DISCOVERY_ALREADY_FOUND` không cộng thưởng; NẾU vượt `discovery_report_rate`, THÌ PHẢI trả `429 RATE_LIMITED`; cùng `Idempotency-Key` PHẢI trả cùng kết quả | FR-GAME-017 · AC-GAM-010 · NFR-SEC-004 | TC-GAM-010, TC-WLD-003 |
| F071-AC-26 | KHI học sinh mở World Map (S04), game client PHẢI hiển thị vùng mở/khóa kèm điều kiện bằng lời, % khám phá từ `getWorldExploration` (bằng `exploration_progress`), nút dịch chuyển tới cổng đã kích hoạt; không có lựa chọn mở bằng Xu | FR-GAME-017 · AC-WLD-004 · [STUDENT_GAME_UX_CORE S04](../../docs/05-ux/STUDENT_GAME_UX_CORE.md#s04--world-map) | TC-WLD-001 |
| F071-AC-27 | Ở NƠI học sinh thuộc grade band `g1_3`, game client PHẢI hiển thị bản đồ dạng bảng chọn hình (không %), dịch chuyển tới vùng đã mở bằng 1 chạm | FR-GAME-017 · FR-GAME-010 · [WORLD_DESIGN §6](../../docs/01-game-design/WORLD_DESIGN.md#6-di-chuyển-cổng-spawn) | — |

### Edge Cases

- NẾU Valkey mất presence, THÌ `reportDiscovery` trả `422 STEP_PRECONDITION_FAILED`; client gọi lại `enterWorldArea` khi vào lại zone (F071-AC-25).
- NẾU học sinh lên khối (năm học mới, `student.synced` đổi `grade_level`), THÌ area học tập theo khối mới mở ngay; unlock vùng phiêu lưu giữ nguyên.
- NẾU publish version area làm mất một discovery point đã tìm, THÌ `exploration_progress` giữ ID cũ nhưng % tính theo tổng của version hiện hành, không vượt 100%.
- NẾU `ref.mastery_summary` chưa bắt kịp, THÌ `objective_progress` được đánh giá lại khi event tới; không khóa khu học tập (F071-AC-02).
- NẾU NPC dialogue tham chiếu quest chưa gán tenant, THÌ action `give_quest` trả `404` từ F072; client hiện thoại dự phòng.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-GAME-001 | Zone stylized low-poly, không bạo lực: lint zone + checklist an toàn nội dung, material từ token | [WORLD_DESIGN §11](../../docs/01-game-design/WORLD_DESIGN.md#11-art-direction), [WORLD_RENDERING](../../docs/08-game-client/WORLD_RENDERING.md) |
| FR-GAME-002 | 8 vùng, 12 `area_id` bao 7 khu chức năng; seed dữ liệu thế giới | [WORLD_DESIGN §3–§5](../../docs/01-game-design/WORLD_DESIGN.md) |
| FR-GAME-003 | NPC interaction: `NPC.tscn`, `DialogueRunner`, camera hội thoại (controller/camera cơ bản đã có ở F062) | [CHARACTER_SYSTEM §3](../../docs/01-game-design/CHARACTER_SYSTEM.md#3-camera-controller), [WORLD_RENDERING §9](../../docs/08-game-client/WORLD_RENDERING.md#9-npc) |
| FR-GAME-006 | Lõi phát hành nội dung game (version, snapshot, manifest tenant), level session ghim version; NPC dialogue, area config tải từ backend | [GAME_DESIGN §14](../../docs/01-game-design/GAME_DESIGN.md#14-data-driven-content), ADR-0043 |
| FR-GAME-016 | 7 chương, mở song song, `story_progress`, `story.chapter_completed` | [WORLD_DESIGN §2.2](../../docs/01-game-design/WORLD_DESIGN.md#22-mạch-truyện-chính-confirmed--người-duyệt-2026-11-15-adr-0056) |
| FR-GAME-017 | World map, cổng gió, vào khu, khám phá, % hoàn thành, mở khóa không theo thanh toán | [WORLD_DESIGN §6–§8](../../docs/01-game-design/WORLD_DESIGN.md) |
| FR-GAME-018 | Học tập mở từ NPC/trạm trong ngữ cảnh thế giới, trả về thế giới sau tổng kết (giới hạn khối học tập trong quest: F072) | [GAME_DESIGN §6](../../docs/01-game-design/GAME_DESIGN.md#6-học-tập-lồng-trong-gameplay) |
| FR-GAM-009 | Unlock theo level/quest/LO/achievement, khu học tập luôn mở, khám phá, hoàn thành bản đồ | [WORLD_DESIGN §7–§8](../../docs/01-game-design/WORLD_DESIGN.md) |

### Key Entities

- **WorldArea (+ version)**: khu tải trọn gói: `area_id`, vùng, loại khu (học tập/phiêu lưu/…), môn, scene, bundle, sức chứa, `access_policy`, `unlock_rule`, danh sách điểm neo (spawn, cổng, discovery, collectible).
- **StoryChapter (+ version)**: chương truyện gắn vùng, thứ tự, chuỗi main quest, boss, điều kiện mở.
- **NpcDialogue (+ version)**: đồ thị hội thoại của một `npc_id`, song ngữ, theo khối.
- **LevelPackage (+ version) / LevelSession**: gói zone (bundle) có version; lượt vào màn ghim version.
- **AreaUnlock, ExplorationProgress, StoryProgress**: tiến trình thế giới của học sinh.
- **GameContentRelease / ContentManifest**: lịch sử con trỏ version và manifest tenant.
- **MasterySummary (projection)**: mức mastery theo LO của học sinh từ module learning.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `hub_instance_capacity` | Sức chứa khai báo của area hub (allocator v0 chưa cân tải; F091) |
| `area_transition_max` | Ngân sách chuyển vùng |
| `discovery_report_rate` | Rate limit `reportDiscovery` |
| `bilingual_names_display` | Hiển thị tên song ngữ trên bản đồ/NPC |
| `idempotency_record_ttl` | `startLevel`, `endLevelSession`, `reportDiscovery` |
| `api_rate_default` | Rate limit route đọc |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Mở khóa/khám phá do server quyết; client chỉ ẩn cổng | ADR-0016 · AGENTS R5 |
| Phản ứng mở khóa qua event, không gọi đồng bộ module khác | AGENTS R29 · ADR-0042 |
| Zone trong ngân sách cảnh, chuyển vùng theo hạng Tầm trung | NFR-COMP-001 · NFR-PERF-005 · NFR-PERF-006 · ADR-0057 |
| Văn bản thế giới đủ vi/en trước publish | NFR-I18N-002 · ADR-0056 |
| Nội dung thế giới là dữ liệu version, không hardcode trong scene | AGENTS R2, R31 · ADR-0043 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% khu học tập theo khối/môn vào được với học sinh level 1, ví 0 (test tự động mọi khối 1–9).
- **SC-002**: Chuyển vùng qua cổng gió đạt ngân sách `area_transition_max` ở ≥ 95% lượt trên thiết bị hạng Tầm trung.
- **SC-003**: Mỗi mở khóa vùng / hoàn thành chương / điểm khám phá được ghi đúng 1 lần dưới event lặp (1.000 lần replay).
- **SC-004**: Đổi tên vùng/hội thoại NPC hiển thị trên client ở lần vào màn kế tiếp, không phát hành client mới.

## Ngoài phạm vi

- Hub realtime: cân tải instance theo `hub_instance_capacity`, presence, đồng bộ vị trí, ưu tiên bạn bè → F091/F092; "Tham gia bạn" là lệnh WS `hub.instance.join {area_id, follow_user_id}` của F091 (không có REST `joinFriendInWorld`, RB-625).
- Workflow duyệt/rollback/unpublish, API `/game-admin/world-areas|story-chapters|npc-dialogues` và cấu hình gói level, thumbnail 3D → F078/F079.
- Quest (`give_quest`, `talk_npc`, `go_to`, phần quest trong %) → F072; XP khám phá, level → F074; collection/achievement khu vực → F075; world event/trang trí lễ hội → F077; boss chương → F103.
- Tháp Thử Thách luồng thi → F064; party `private_instance` → F101.

## Assumptions

- F026 cung cấp mẫu phát hành và event `content.published` của learning; F071 áp cùng mẫu cho `eduworld_game`.
- Asset 3D 12 zone theo pipeline WORLD_RENDERING; nguồn asset OQ-16 (đã trả lời 2026-11-20: stylized low-poly, asset pack có license + tự làm nhân vật chính) — zone greybox được chấp nhận ở M2 nếu đạt lint và ngân sách.
- Cài đặt môn bật/tắt của trường đến qua projection `ref.schools.settings_subset` (event `school.updated` của identity) [Confirmed].
