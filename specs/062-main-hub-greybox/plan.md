# Implementation Plan: F062 — Main Hub greybox, character controller, `QualityManager`, tab "Nhà trường giao"

**Branch**: `062-main-hub-greybox` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Dựng zone `main_hub` dạng greybox (khối CSG/mesh đơn giản, đúng cây node chuẩn của zone), `Player` với `CharacterBody3D` + camera rig + `AnimationTree` state machine, lớp `Interactable` + `ContextActionButton`. `QualityManager` (autoload #12) hiện thực benchmark lần đầu, preset, device profile, độ phân giải động, renderer fallback và `suspend_world/resume_world` cho màn học tập. `AssignmentService` (`RefCounted`) gọi `listMyAssignments` qua `ApiClient`, cập nhật `AppState`; UI chỉ đọc state: `MainHud` tối thiểu và `QuestJournal` với tab "Nhà trường giao". `PlayTimeService` hiển thị nhắc nghỉ theo policy bootstrap. Phát triển trên Prism mock (profile `mock`, nhãn MOCK) tới khi F041 Done.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | GDScript (Godot 4.x ghim trong `godot/.godot-version`), static typing bắt buộc (ADR-0014) |
| Primary Dependencies | GdUnit4; autoload khung của F060 (`Config`, `Log`, `EventBus`, `AppState`, `SceneRouter`, `Settings`); `ApiClient`, `AuthService`, `Clock` của F061 |
| Storage | Không có dữ liệu nghiệp vụ bền; `user://settings.cfg` (preset, renderer, render scale) |
| Testing | GdUnit4 unit + scene runner headless; benchmark headless `RenderingServer`; thiết bị lab |
| Target Platform | Windows 10+, Android 9+ (Release 1.0 cũng Web/iOS — ADR-0013) |
| Project Type | game client |
| Performance Goals | [PERFORMANCE_BUDGET §3, §5–§7, §9, §11](../../docs/08-game-client/PERFORMANCE_BUDGET.md) · NFR-COMP-001 · NFR-PERF-005 |
| Constraints | AGENTS R1, R2, R3, R17, R32 · ADR-0047, ADR-0056, ADR-0057, ADR-0065 |
| Scale/Scope | 1 zone greybox, 1 nhân vật cục bộ (chưa người chơi khác) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | spec Ready; delta `MyAssignment` ở Phase 2 (T004); không chạm nội dung deferred |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Client chỉ gọi API public qua `ApiClient`; tách Service ← Gameplay ← UI (G-4) |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Mọi request có `X-Tenant-Id` của phiên (ADR-0065); không lưu dữ liệu tenant xuống đĩa; test F062-AC-12 |
| IV Server-authoritative | ✅ | Trạng thái bài giao, quyền mở bài, giờ truy cập do server; client chỉ hiển thị |
| V Xác thực, phân quyền | ✅ | Token RAM của F061; lỗi `GAME_ACCESS_STUDENT_ONLY`/`TENANT_MISMATCH` về Login; không URL/secret hardcode (config env) |
| VI An toàn trẻ em | ✅ | Không hiện dữ liệu học sinh khác (F062-AC-11); nhắc nghỉ; log không PII |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số theo tên CONFIG_DEFAULTS; ngưỡng nhắc nghỉ từ bootstrap; chuỗi dữ liệu không đổi |
| VIII UI tách logic, token | ✅ | UI chỉ gọi method service/đọc `AppState`; Theme sinh từ `design/game-tokens`, lint override |
| IX Test có bằng chứng | ✅ | Mỗi F062-AC có task test; mock gắn nhãn MOCK, không vào build phát hành |

## Project Structure

### Documentation (feature này)

```text
specs/062-main-hub-greybox/
├── spec.md
├── plan.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
godot/
├── project.godot                         # input map: move_*, jump, interact (E), journal (J), camera_*
├── scenes/
│   ├── world/zones/main_hub/Zone.tscn    # greybox: Environment, Terrain, Static, Navigation, Occluders, SpawnPoints, Interactables, Players
│   ├── character/Player.tscn             # CharacterBody3D + AvatarRig placeholder + CameraRig + AnimationTree
│   ├── bootstrap/QualityBenchmark.tscn   # cảnh benchmark ≤ 5 s
│   ├── quest/QuestJournal.tscn           # + quest_journal.gd (chỉ tab "Nhà trường giao" ở F062)
│   ├── quest/AssignmentRow.tscn
│   └── ui/hud/MainHud.tscn               # + main_hud.gd; ContextActionButton.tscn; Joystick.tscn; BreakReminderDialog.tscn
├── scripts/
│   ├── core/quality_manager.gd           # autoload QualityManager
│   ├── core/play_time_service.gd         # RefCounted
│   ├── learning/assignment_service.gd    # RefCounted, nhận ApiClient qua constructor
│   ├── learning/models/my_assignment.gd  # typed model parse từ JSON
│   ├── player/{player_controller.gd,camera_rig.gd,anim_driver.gd,tap_to_move.gd}
│   └── world/{zone_root.gd,spawn_point.gd,interactable.gd}
├── locale/{vi.po,en.po}                  # ui.hub.*, ui.quest.*, ui.quality.*, ui.common.break_reminder
└── tests/
    ├── unit/{test_assignment_service.gd,test_quality_manager.gd,test_play_time_service.gd,test_player_controller.gd}
    ├── scene/{test_main_hud.gd,test_quest_journal_assigned.gd,test_main_hub_zone.gd}
    ├── perf/MainHubBenchmark.tscn        # TC-PERF-018
    └── fixtures/learning/my_assignments_*.json   # sinh từ example OpenAPI
```

**Structure Decision**: Theo [GODOT_ARCHITECTURE §3](../../docs/08-game-client/GODOT_ARCHITECTURE.md) và cây zone chuẩn [WORLD_RENDERING §2](../../docs/08-game-client/WORLD_RENDERING.md). Logic thuần (`AssignmentService`, `PlayTimeService`, bộ điều khiển render scale trong `QualityManager` tách hàm thuần) để unit test bằng `FakeApiClient`/đồng hồ giả.

## Phase 0: Research

Không có — quyết định đã có trong PERFORMANCE_BUDGET, GODOT_ARCHITECTURE, STUDENT_GAME_UX_CORE. Ghi nhận: nhắc nghỉ dùng ngưỡng từ bootstrap thay vì "server gửi qua notification" ([GODOT_ARCHITECTURE §10](../../docs/08-game-client/GODOT_ARCHITECTURE.md)) vì M-C2 chưa có kênh notification; báo cáo lại để chỉnh tài liệu.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| Service (`RefCounted`/autoload) | `AssignmentService.refresh() -> Result`, `start(assignment)`; `PlayTimeService.tick(delta)`; `QualityManager.run_benchmark()`, `apply_preset()`, `suspend_world(reason)`, `resume_world(reason)` | Phát signal `assignments_updated`, `break_reminder_due`, `quality_changed` |
| Gameplay | `PlayerController`, `CameraRig`, `AnimDriver`, `TapToMove` (g1_3), `Interactable`, `ZoneRoot`, `SpawnPoint` | Không `preload` UI; mở UI qua `EventBus.ui_requested` |
| UI | `MainHud`, `QuestJournal` (tab `assigned`), `AssignmentRow`, `ContextActionButton`, `Joystick`, `BreakReminderDialog` | Chỉ đọc `AppState`, gọi method public service |
| Mạng | `ApiClient` (F061) | Header chuẩn [GAME_NETWORKING §2.1](../../docs/08-game-client/GAME_NETWORKING.md) |

### Tích hợp và phụ thuộc

- F061: đăng nhập, bootstrap (`grade_band`, `policies.play_time`), heartbeat game session, `Clock`. F060: Theme density, `.po`, Prism mock profile.
- API: `listMyAssignments` (learning, F041), `getClientConfig` (`device_profiles`, F014), `postTelemetry` (fps_p10, renderer, preset, F014).
- Intent cho feature sau: `ui_requested("activity_host", …)` (F063), `ui_requested("exam_lobby", …)` (F064).

### Bảo mật, tenant, idempotency, audit

- Route người chơi qua middleware `game_player` ở backend; client gắn `X-Tenant-Id` từ bootstrap, không suy tenant từ dữ liệu khác.
- F062 không có thao tác ghi nghiệp vụ (chỉ GET + telemetry) → không cần Idempotency-Key/audit phía client.

### Quan sát

- Telemetry `/game/telemetry` (≤ 1 lần/5 phút): `fps_p10`, renderer, preset, render scale, model thiết bị (không PII); sự kiện `client_unsupported_assignment_kind`, `renderer_fallback`; `break_reminder_shown` mỗi lần hộp nhắc nghỉ hiện (GAME_ANALYTICS §3.2, không phụ thuộc cờ telemetry sản phẩm).
- `Log` không ghi tên bài/nickname ở mức INFO (R14).

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
| — | — | — |
