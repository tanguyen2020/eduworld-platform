# Feature Specification: F062 — Main Hub greybox, character controller, `QualityManager`, tab "Nhà trường giao"

**Feature Branch**: `062-main-hub-greybox` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `src/game/` · **Mốc**: M-C2 · **Phụ thuộc**: F041, F061 (F041 dùng Prism mock có nhãn cho tới khi Done — [CODE_PLAN §5 (\*)](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md))
**Input**: Học sinh đã đăng nhập (F061) vào Main Hub dạng greybox, điều khiển nhân vật, chạy mượt trên máy tầm trung, thấy và mở bài nhà trường giao. FR nguồn: FR-GAME-003, FR-GAME-009, FR-GAME-010, FR-GAME-012, FR-LMS-006; NFR-COMP-001, NFR-PERF-005.

## Clarifications

### Session 2026-11-16

- Q: Ở M-C2 chưa có realtime, quest, XP, ví Xu (F070–F075, F092). HUD hiển thị gì? → A: HUD chỉ gồm thành phần có dữ liệu thật: `PlayerBadge` (nickname, grade band từ bootstrap), chip "Nhà trường giao (n)", nút Cài đặt, nút Về Quảng trường, banner hệ thống. Slot `XpBar`, `CurrencyChip`, `QuestTracker`, `Minimap`, `ChatDock`, `PartyChip` có sẵn trong layout nhưng ẩn tới feature sở hữu ([STUDENT_GAME_UX_CORE S03](../../docs/05-ux/STUDENT_GAME_UX_CORE.md)).
- Q: Danh sách bài giao làm mới khi nào khi chưa có realtime? → A: Khi vào/quay lại Hub, khi mở Nhật ký nhiệm vụ; không polling định kỳ (không có tham số đăng ký). Thông báo bài mới thuộc F064.
- Q: Nhắc nghỉ do ai đếm giờ? → A: Ngưỡng lấy từ `PlayerBootstrap.policies.play_time.break_reminder_minutes` (server); client chỉ đếm thời gian phiên liên tục để hiển thị lời nhắc, không chặn chơi. Chặn ngoài giờ truy cập do server (`403 ACCESS_OUTSIDE_ALLOWED_HOURS`).
- Q: Đo lời nhắc nghỉ thế nào khi server không biết lúc nhắc? → A: Không có domain event (Technical Architect 2026-11-17). Mỗi lần hiện hộp nhắc, client gửi telemetry `break_reminder_shown` (`reminder_no`, `minutes_played`) trong lô `postTelemetry` kế tiếp ([GAME_ANALYTICS §3.2](../../docs/11-operations/GAME_ANALYTICS.md)); F080 tổng hợp chỉ số chơi lành mạnh.
- Q: Tab "Nhà trường giao" mở bài thế nào khi activity player (F063) và Exam (F064) chưa xong? → A: F062 phát intent `EventBus.ui_requested("activity_host" | "exam_lobby", args)`; khi chưa có màn đăng ký, hiển thị `ui.activity_player.update_required` (không crash).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Vào Main Hub và điều khiển nhân vật (Priority: P1)

Học sinh vào Quảng trường Tia Sáng (greybox), đi lại bằng bàn phím/chuột hoặc joystick cảm ứng, camera theo sau, đến gần bảng/NPC thì hiện nút hành động (US-GAME-002).

**Why this priority**: Là khung thế giới mà mọi tính năng game sau gắn vào; không có thì không có điểm vào học tập trong game.

**Independent Test**: Đăng nhập `hs.an.3a` (hoặc profile mock), vào Hub, đi tới bảng "Nhà trường giao", thấy nút `[E]` / nút chạm và mở được panel.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F062-AC-01 | KHI bootstrap thành công, game client PHẢI nạp scene `main_hub` và đặt nhân vật tại spawn point do server trả, NẾU không tồn tại thì tại `main_hub.default` | FR-GAME-003 · [WORLD_RENDERING §4](../../docs/08-game-client/WORLD_RENDERING.md) | — |
| F062-AC-02 | TRONG KHI ở Hub, game client PHẢI cho di chuyển bằng WASD/phím mũi tên + chuột xoay camera trên PC và joystick ảo + vuốt camera trên mobile; `g1_3` PHẢI có bấm-để-đi | FR-GAME-003, FR-GAME-010 · [STUDENT_GAME_UX §4](../../docs/05-ux/STUDENT_GAME_UX.md) | — |
| F062-AC-03 | game client PHẢI chuyển animation idle/walk/run/jump theo vận tốc nhân vật qua một animation state machine duy nhất | FR-GAME-003 | — |
| F062-AC-04 | KHI nhân vật vào vùng của một `Interactable`, game client PHẢI hiện `ContextActionButton` với nhãn từ `prompt_key` và chỉ phát `interacted(interactable_id)`, không chứa logic nghiệp vụ trong scene | FR-GAME-003 · AGENTS R1 · [WORLD_RENDERING §5](../../docs/08-game-client/WORLD_RENDERING.md) | — |
| F062-AC-05 | game client PHẢI chọn biến thể Theme và bố cục HUD theo `bootstrap.grade_band` (`g1_3` vùng chạm ≥ 64 dp, chữ cơ sở theo density token; ẩn minimap) và không có màu/font/cỡ chữ hardcode trong `.tscn` | FR-GAME-010 · NFR-A11Y-001 · ADR-0047 | TC-UI-001, TC-UI-003 |
| F062-AC-06 | Mọi chuỗi UI của Hub/HUD PHẢI đi qua khóa `ui.hub.*` trong `vi.po`/`en.po`; đổi ngôn ngữ PHẢI cập nhật ngay không khởi động lại | NFR-I18N-001 · ADR-0056 | TC-I18N-004, TC-I18N-005 |

---

### User Story 2 — Xem và mở bài nhà trường giao (Priority: P1)

Học sinh thấy chip "Nhà trường giao (n)" trên HUD, mở tab "Nhà trường giao" trong Nhật ký nhiệm vụ (phím `J`, nút 📜, hoặc bảng trong Hub), xem tên, hạn, trạng thái và bấm "Bắt đầu" (FR-LMS-006).

**Why this priority**: Là đường vào vòng School Loop (M-C3) và là điểm demo M-C2 cho trường.

**Independent Test**: Với Prism mock (M-C2) hoặc backend F041 (M-C3), `hs.an.3a` thấy đúng danh sách bài của mình; bấm "Bắt đầu" phát intent mở activity host/exam lobby với đúng `assignment_id`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F062-AC-07 | KHI vào/quay lại Hub hoặc mở Nhật ký nhiệm vụ, `AssignmentService` PHẢI gọi `listMyAssignments` (kèm `X-Tenant-Id` của phiên) và cập nhật `AppState.assignments` + chip HUD | FR-LMS-006 · [STUDENT_GAME_UX_CORE S03, S05](../../docs/05-ux/STUDENT_GAME_UX_CORE.md) · ADR-0065 | TC-LMS-002 |
| F062-AC-08 | Tab "Nhà trường giao" PHẢI hiện mỗi bài: tên (nguyên văn), môn, hạn theo `LocaleFormat`, trạng thái (`not_started`, `in_progress`, `submitted`, `graded`, `overdue`, `excused`) bằng icon + chữ; danh sách ảo hóa | FR-LMS-006 · [STUDENT_GAME_UX_CORE S05](../../docs/05-ux/STUDENT_GAME_UX_CORE.md) | TC-I18N-007 |
| F062-AC-09 | KHI học sinh bấm "Bắt đầu" bài `content_kind = activity`, game client PHẢI phát `ui_requested("activity_host", {activity_id: content_ref, mode: "assignment", context_type: "assignment", context_id: assignment_id})`; bài `assessment` PHẢI phát `ui_requested("exam_lobby", {exam_session_id, assignment_id})` | FR-LMS-006 · [ACTIVITY_ENGINE §9](../../docs/02-learning/ACTIVITY_ENGINE.md) | TC-LMS-002 |
| F062-AC-10 | NẾU `content_kind` không có màn đích đăng ký (vd. `quest`, `lesson` trước F072/F040), THÌ game client PHẢI hiện `ui.activity_player.update_required`, gửi telemetry `client_unsupported_assignment_kind`, không crash | FR-LMS-006 · [ACTIVITY_ENGINE §12](../../docs/02-learning/ACTIVITY_ENGINE.md) | TC-ACT-004 |
| F062-AC-11 | Tab "Nhà trường giao" PHẢI không hiển thị điểm, tên hay trạng thái của học sinh khác và không có nhận xét giáo viên | AGENTS R16 · ADR-0072 | — |
| F062-AC-12 | NẾU `listMyAssignments` trả `401`, THÌ game client PHẢI refresh token một lần rồi gọi lại; NẾU `403 TENANT_MISMATCH`/`GAME_ACCESS_STUDENT_ONLY`, THÌ PHẢI xóa token và về Login theo F061; NẾU lỗi mạng/5xx, THÌ PHẢI hiện trạng thái lỗi có nút Thử lại và giữ danh sách cũ trong bộ nhớ | ADR-0065 · [GAME_NETWORKING §2.3](../../docs/08-game-client/GAME_NETWORKING.md) | TC-AUTH-014 |
| F062-AC-13 | Ở NƠI build dùng profile `mock` (Prism), game client PHẢI hiện nhãn "MOCK" thường trực trên HUD và build phát hành PHẢI không chứa profile `mock` | AGENTS R17 · [CODE_PLAN §2](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) | — |

---

### User Story 3 — Chạy mượt trên máy tầm trung (Priority: P1)

Lần đầu vào game, `QualityManager` chạy benchmark ngắn và chọn preset; trong khi chơi tự giảm/tăng độ phân giải 3D; khi mở màn học tập, thế giới 3D tạm dừng (NFR-COMP-001).

**Why this priority**: "Chạy được trên máy tầm trung" là yêu cầu bắt buộc (ADR-0057); hub là cảnh đo chuẩn.

**Independent Test**: Chạy trên DEV-PC-MID và DEV-AND-MID1: preset tự chọn Trung bình, FPS đạt `target_fps`; benchmark headless CI của `main_hub` không vượt `scene_budget_hub`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F062-AC-14 | KHI chạy lần đầu trên thiết bị (chưa có `quality.preset` trong `user://settings.cfg`), `QualityManager` PHẢI chạy benchmark ≤ 5 giây trước khi vào Hub, chọn preset theo `quality_presets` và cho phép bỏ qua để chọn tay | FR-GAME-009 · [PERFORMANCE_BUDGET §6](../../docs/08-game-client/PERFORMANCE_BUDGET.md) | TC-PERF-022 |
| F062-AC-15 | KHI `client-config.device_profiles` khớp thiết bị, `QualityManager` PHẢI ép renderer/preset theo profile đó thay cho kết quả benchmark | FR-GAME-009 · [PERFORMANCE_BUDGET §6](../../docs/08-game-client/PERFORMANCE_BUDGET.md) | TC-PERF-022 |
| F062-AC-16 | TRONG KHI ở Hub, `QualityManager` PHẢI điều chỉnh render scale 3D theo `dynamic_resolution` trong giới hạn preset, không scale UI, và không tự đổi preset (chỉ gợi ý) | FR-GAME-009 · NFR-COMP-001 | TC-PERF-009 |
| F062-AC-17 | NẾU renderer Mobile khởi tạo lỗi ở lần chạy trước, THÌ lần chạy sau game client PHẢI dùng Compatibility và gửi telemetry | FR-GAME-009 · [PERFORMANCE_BUDGET §4](../../docs/08-game-client/PERFORMANCE_BUDGET.md) | TC-PERF-022 |
| F062-AC-18 | KHI một màn học tập toàn màn yêu cầu `QualityManager.suspend_world(reason)`, game client PHẢI dừng vật lý/animation thế giới và thay render 3D bằng ảnh chụp mờ cho tới `resume_world(reason)`; TRONG KHI màn đứng yên > 10 s, PHẢI giảm render 3D còn 15 FPS | NFR-COMP-001 · [STUDENT_GAME_UX_LEARNING §2.1](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md) · `battery_target` | TC-PERF-020 |
| F062-AC-19 | Cảnh `main_hub` PHẢI qua benchmark headless CI trong `scene_budget_hub` (sai lệch ≤ `perf_budget_tolerance_ci`) và HUD ≤ `ui_cpu_budget` | NFR-COMP-001 · ADR-0057 | TC-PERF-018 |
| F062-AC-20 | Thời gian từ khởi động lạnh (cache ấm) đến Hub PHẢI ≤ `load_time` trên DEV-AND-MID1 | NFR-PERF-005 | TC-PERF-020 |

---

### User Story 4 — Nhắc nghỉ sau thời gian chơi liên tục (Priority: P2)

Sau khoảng thời gian chơi liên tục do trường/platform cấu hình, Mây nhắc học sinh nghỉ mắt bằng hộp thoại nhẹ (FR-GAME-012).

**Why this priority**: An toàn trẻ em; không chặn luồng học chính nên P2.

**Independent Test**: Mock bootstrap `break_reminder_minutes = 1`, chơi 1 phút → hộp nhắc nghỉ xuất hiện một lần, đóng được.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F062-AC-21 | KHI thời gian phiên liên tục đạt `policies.play_time.break_reminder_minutes`, `PlayTimeService` PHẢI hiện hộp nhắc nghỉ `ui.common.break_reminder` (không chặn, đóng được) và lặp lại theo `break_reminder` | FR-GAME-012 · [CHILD_SAFETY](../../docs/10-security/CHILD_SAFETY.md) | — |
| F062-AC-22 | TRONG KHI đang làm bài kiểm tra có giờ hoặc có hộp thoại bắt buộc, `PlayTimeService` PHẢI hoãn lời nhắc tới khi rời màn | FR-GAME-012 · AGENTS R19 | — |
| F062-AC-23 | NẾU API trả `403 ACCESS_OUTSIDE_ALLOWED_HOURS`, THÌ game client PHẢI hiện `ui.common.outside_access_window` kèm `access_window` của bootstrap và về Login | FR-GAME-012 · [GODOT_ARCHITECTURE §9](../../docs/08-game-client/GODOT_ARCHITECTURE.md) | — |
| F062-AC-24 | KHI hộp nhắc nghỉ thật sự hiện (không tính lời nhắc đang hoãn theo F062-AC-22), game client PHẢI xếp một telemetry `break_reminder_shown` (`client_event_id` mới, `occurred_at`, `reminder_no` tăng từ 1 trong phiên, `minutes_played`) vào lô `postTelemetry` kế tiếp, giữ trong hàng đợi khi mất mạng và gửi lại cùng `client_event_id`; PHẢI vẫn gửi khi `product_telemetry_enabled = false` và không kèm tên, username, `student_id` hay vị trí | FR-GAME-012 · FR-ANL-010 · [GAME_ANALYTICS §3.2](../../docs/11-operations/GAME_ANALYTICS.md) · AGENTS R14 | — |

### Edge Cases

- NẾU spawn point server trả không có trong scene, THÌ dùng `main_hub.default` và log cảnh báo (F062-AC-01).
- NẾU danh sách bài giao rỗng, THÌ tab hiện `ui.quest.empty` và chip HUD ẩn số.
- NẾU bài có `due_at` đã qua, THÌ nhãn trung tính `ui.quest.overdue` (không màu đỏ); bài vẫn mở được nếu server cho phép (server quyết định, lỗi `ASSIGNMENT_NOT_OPEN` hiển thị qua `ui.error.ASSIGNMENT_NOT_OPEN`).
- NẾU nhiệt độ Android ≥ "moderate", THÌ hạ trần FPS/scale một mức (F062-AC-16).
- NẾU app xuống nền (mobile)/tab ẩn (web), THÌ dừng render và giảm mạng về heartbeat game session (F061).
- NẾU locale đổi khi tab đang mở, THÌ tên bài giữ nguyên văn, chuỗi UI đổi ngay (F062-AC-06).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-GAME-003 | Character controller, camera rig, animation state machine, `Interactable` + nút hành động (NPC hội thoại đầy đủ: F071) | [GODOT_ARCHITECTURE §3, §6](../../docs/08-game-client/GODOT_ARCHITECTURE.md), [WORLD_RENDERING §2–§5](../../docs/08-game-client/WORLD_RENDERING.md) |
| FR-GAME-009 | `QualityManager`: benchmark, preset, device profile, độ phân giải động, renderer fallback, suspend world | [PERFORMANCE_BUDGET §4–§6, §12](../../docs/08-game-client/PERFORMANCE_BUDGET.md) |
| FR-GAME-010 | Theme density + bố cục HUD theo grade band | [STUDENT_GAME_UX_CORE S03](../../docs/05-ux/STUDENT_GAME_UX_CORE.md), [DESIGN_SYSTEM §5](../../docs/05-ux/DESIGN_SYSTEM.md) |
| FR-GAME-012 | Nhắc nghỉ theo policy bootstrap, gửi telemetry `break_reminder_shown` (GAME_ANALYTICS §3.2); hiển thị lỗi ngoài giờ truy cập | [CHILD_SAFETY](../../docs/10-security/CHILD_SAFETY.md) |
| FR-LMS-006 | Chip HUD + tab "Nhà trường giao" + bảng trong Hub, mở bài qua intent | [SCHOOL_LMS](../../docs/04-admin-portal/SCHOOL_LMS.md), [STUDENT_GAME_UX_CORE S05](../../docs/05-ux/STUDENT_GAME_UX_CORE.md) |

### Key Entities

- **MyAssignment** (đọc): bài được giao cho chính học sinh — tên, môn, loại nội dung, tham chiếu nội dung/đề, hạn, trạng thái, số lượt còn lại.
- **QualityProfile** (thiết bị): renderer, preset, render scale, trần FPS; lưu theo thiết bị, không gửi server.
- **PlayTimePolicy** (đọc): ngưỡng nhắc nghỉ, khung giờ truy cập — từ bootstrap.
- **Interactable**: điểm neo trong scene (`interactable_id`, `kind`, `prompt_key`), không mang dữ liệu nghiệp vụ.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `quality_presets`, `renderer_default`, `target_fps`, `device_tier_reference` | Chọn renderer/preset, trần FPS |
| `dynamic_resolution` | Bộ điều khiển render scale |
| `scene_budget_hub`, `ui_cpu_budget`, `perf_budget_tolerance_ci` | Ngân sách cảnh Hub/HUD, cổng CI |
| `load_time`, `battery_target` | Thời gian vào Hub; giảm FPS khi đứng yên |
| `break_reminder` | Nhắc nghỉ (giá trị hiệu lực do server gửi trong bootstrap) |
| `supported_locales` | Ngôn ngữ UI |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Chạy được trên máy tầm trung, chơi được ở hạng Tối thiểu preset Thấp | NFR-COMP-001 · ADR-0057 |
| Khởi động lạnh → Hub theo hạng Tầm trung | NFR-PERF-005 |
| UI không chứa logic nghiệp vụ; service `RefCounted` test được | AGENTS R1 · [GODOT_ARCHITECTURE G-4](../../docs/08-game-client/GODOT_ARCHITECTURE.md) |
| GDScript static typing; theme từ token; chuỗi qua `.po` | ADR-0014 · ADR-0047 · ADR-0056 |
| Không lưu dữ liệu tenant xuống đĩa (chỉ settings thiết bị) | [GODOT_ARCHITECTURE §8.2](../../docs/08-game-client/GODOT_ARCHITECTURE.md) |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% học sinh demo vào được Hub và tới bảng "Nhà trường giao" trong ≤ 2 thao tác từ HUD (PC và mobile).
- **SC-002**: Trên DEV-AND-MID1 preset Trung bình, Hub greybox đạt FPS p10 ≥ 27; trên DEV-PC-MID ≥ 50.
- **SC-003**: Bài vừa được giao xuất hiện trong tab khi học sinh quay lại Hub, không cần khởi động lại.
- **SC-004**: Benchmark tự phát hiện chọn đúng preset trên toàn bộ lab thiết bị tham chiếu.

## Ngoài phạm vi

- Người chơi khác, presence, đồng bộ vị trí: F092. Avatar/nickname lần đầu: F070. Quest journal các tab khác, quest tracker: F072. NPC hội thoại, world map, cổng vùng: F071. XP/Xu: F074/F075.
- Activity player: F063. Exam lobby/runner, thông báo bài mới: F064.
- Art Main Hub hoàn chỉnh, PCK vùng: F071. Build phát hành đa nền tảng: F133.

## Assumptions

- F061 cung cấp `ApiClient`, `AuthService`, `Clock`, heartbeat game session, xử lý lỗi đăng nhập; F060 cung cấp autoload khung, Theme sinh từ token, `.po`, profile `mock` (Prism).
- Ở M-C2, `listMyAssignments` chạy trên Prism mock gắn nhãn; F062 chỉ `Done` khi chạy với F041 thật (CODE_PLAN §5).
- Contract `MyAssignment` cần thêm `exam_session_id`, `result_release`, `late_policy` (contracts §6) [Proposed]. Nhắc nghỉ [Proposed, OQ-17].
