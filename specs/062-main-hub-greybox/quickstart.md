# Quickstart: F062 — Main Hub greybox, character controller, `QualityManager`, tab "Nhà trường giao"

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F060, F061 (bắt buộc); F041 (cho nghiệm thu Done — trước đó dùng profile `mock` của F060, nhãn MOCK).
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md) §6); Godot chạy `-- --env=local` (backend thật) hoặc `-- --env=mock` (Prism).
- Tài khoản demo (mật khẩu `Demo@12345`): `hs.an.3a`, `hs.binh.3a` (th-demo-a/3A), `hs.dung.7a` (pgd-demo), `qlcm.truong.a`, `ad.truong.a`.
- Dữ liệu: `ad.truong.a` đã giao 1 bài activity và 1 đề cho lớp 3A (F055/F041 hoặc seed module learning).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Xóa `user://settings.cfg`, đăng nhập `hs.an.3a` | Benchmark ≤ 5 s, preset được chọn, vào Hub tại `main_hub.default` | F062-AC-14, F062-AC-01 |
| 2 | Đi bằng WASD/joystick, nhảy, xoay camera | Animation idle/walk/run/jump đúng; không giật | F062-AC-02, F062-AC-03 |
| 3 | Đến bảng `board.school_assignments`, nhấn E/chạm | Nút hành động hiện; mở tab "Nhà trường giao" | F062-AC-04, F062-AC-07 |
| 4 | Xem danh sách | 2 bài của 3A, hạn theo locale, trạng thái icon + chữ; không thấy bài của 7A | F062-AC-08, F062-AC-11 |
| 5 | Bấm "Bắt đầu" bài activity rồi bài đề (trước F063/F064) | Thông báo cần cập nhật, không crash; telemetry ghi | F062-AC-09, F062-AC-10 |
| 6 | `ad.truong.a` giao thêm bài; học sinh rời panel và quay lại Hub | Bài mới xuất hiện không cần khởi động lại | F062-AC-07 |
| 7 | Đăng nhập `hs.dung.7a` | Chỉ thấy bài của 7A pgd-demo | F062-AC-11, F062-AC-12 |
| 8 | Đổi ngôn ngữ vi ↔ en trong Cài đặt | HUD/tab đổi ngay | F062-AC-06 |
| 9 | Đăng nhập tài khoản có `grade_band = g1_3` | Theme density lớn, bấm-để-đi bật, ẩn minimap | F062-AC-05, F062-AC-02 |
| 10 | Bootstrap mock `break_reminder_minutes = 1` | Hộp nhắc nghỉ sau 1 phút, đóng được | F062-AC-21 |
| 10b | Tiếp bước 10 với backend thật, tắt telemetry sản phẩm của trường; chờ lần nhắc thứ 2 | `postTelemetry` chứa 2 sự kiện `break_reminder_shown` (`reminder_no` 1, 2), không trường định danh; `analytics.events_raw` có 2 dòng `break_reminder_shown` | F062-AC-24 |
| 11 | Chạy profile `mock` | Nhãn "MOCK" hiện trên HUD | F062-AC-13 |
| 12 | Trên DEV-AND-MID1 đo khởi động lạnh → Hub và FPS | ≤ `load_time`; FPS p10 ≥ 27 | F062-AC-20, F062-AC-16 |

## 3. Lệnh test tự động

```bash
make godot-test
make design-tokens-check
godot --headless --path src/game -s addons/gdUnit4/bin/GdUnitCmdTool.gd -a res://tests/perf
```

## 4. Bằng chứng cần nộp

- Kết quả GdUnit4 (pass/fail), báo cáo benchmark `main_hub` (draw call, tam giác, frame time CPU), ảnh HUD ở `g1_3`/`g6_7` và `vi`/`en`/`xx-PS`, bảng FPS trên thiết bị lab, trace_id của lời gọi `listMyAssignments`.

## 5. Bằng chứng (2026-11-20, local Windows, Godot 4.7.2 console, M-C2 với mock có nhãn)

Lệnh (Windows không có `make`; gọi trực tiếp lệnh của target `godot-*` với `GODOT=<Godot_v4.7.2-stable_win64_console.exe>`):

| Lệnh | Kết quả |
|---|---|
| GdUnit4 `-a res://tests` (= `make godot-test`) | **313 test: 308 pass, 0 fail, 5 skip** (4 integration F061 + 1 integration F062 tự skip khi thiếu `EDUWORLD_IT_API_BASE_URL`); 81 test mới của F062 |
| `tools/tests/scan_artifact_test.sh` | 20/20 |
| `--export-pack "Windows Desktop"` + `scan_artifact.sh` (= `make godot-export-check`) | `scan_artifact: OK (1 artifact(s), 13 patterns)` — không `mock.json`, không `tests/`, không `tools/` |
| Boot headless `--env=mock` (= `make godot-smoke`) | `splash_shown` |
| `check_typed`, `lint_theme_overrides`, `lint_layers`, `lint_signals`, `extract_pot --check`, `pseudo_locale --check`, `gen_fixtures --check`, `check_no_admin_ui` | OK (179 / 136 / 71 / 98 / 120 file; 195 khóa vi/en) |
| `design`: `pnpm test`, `pnpm check` (= `make design-tokens-check`) | 56/56; `design-tokens-check: OK` (188/188 cặp tương phản AA) |
| Spec: `check-maxlength`, `check-enums`, `validate-schemas`, `check-features`, `check-cross-features`, `check-portal-catalog`, `check-problem-codes`, `split-public --check` | 0 lỗi |
| `make specs-breaking` | Không đánh giá được: oasdiff không parse được spec gốc của `origin/main` (`items: false`, lỗi có sẵn, mọi file đều báo BREAKING). Delta F062 chỉ thêm 3 trường tùy chọn, 1 `example`, 1 response `403` |

| Tiêu chí | Test (GdUnit4) | Kết quả |
|---|---|---|
| F062-AC-01 | `tests/scene/world/test_main_hub_zone.gd` (spawn theo id, fallback `main_hub.default` + log `spawn_point_missing`, cây zone chuẩn) | 6/6 |
| F062-AC-02, AC-03 | `tests/unit/player/test_player_controller.gd` (vận tốc theo camera, phím/joystick, tap-to-move chỉ `g1_3`, một AnimationTree state machine) | 6/6 |
| F062-AC-04 | `tests/scene/world/test_interactable.gd` | 3/3 |
| F062-AC-05, AC-06 | `tests/scene/hud/test_main_hud.gd` (vùng chạm ≥ 64 dp ở `g1_3`, ẩn minimap, đổi locale tức thì, không theme override/chuỗi cứng trên 13 scene) + `tests/scene/test_pseudo_locale.gd` (HUD ở `xx-PS`) | 5/5; 2/2 |
| F062-AC-07, AC-12 | `tests/unit/learning/test_assignment_service.gd` + `test_main_hub_zone.gd::test_entering_and_returning_to_the_hub_refreshes_assignments` | 10/10 |
| F062-AC-08…AC-11 | `tests/scene/quest/test_quest_journal_assigned.gd` | 7/7 |
| F062-AC-09/10 (host) | `tests/scene/world/test_world_root.gd` (intent không có màn → `update_required`; journal/settings mở thành panel, đóng phát `ui_closed`) | 8/8 |
| F062-AC-13 | `tests/unit/core/test_config_mock_label.gd` + export check | 2/2 |
| F062-AC-14…AC-17 | `tests/unit/core/test_quality_manager.gd` + `test_world_root.gd` (benchmark lần đầu, bỏ qua/chọn tay, device profile) | 14/14 |
| F062-AC-18 | `tests/scene/world/test_suspend_world.gd` | 2/2 |
| F062-AC-19, AC-20 (CI) | `tests/perf/test_main_hub_benchmark.gd`: 8 496 tam giác, 8 draw call (proxy), 1 vật thể động; HUD 24 Control, 0,03 ms/cập nhật; nạp Hub 27 ms | 3/3 |
| F062-AC-21, AC-22 | `tests/unit/core/test_play_time_service.gd` + `test_world_root.gd::test_break_reminder_dialog_is_gentle` | 7/7 |
| F062-AC-23 | `test_assignment_service.gd::test_outside_access_window_returns_to_login_with_the_window` | 1/1 |
| F062-AC-24 | `tests/unit/networking/test_break_reminder_telemetry.gd` | 7/7 |

Đường HTTP thật với Prism MOCK có nhãn (`stoplight/prism:4 --dynamic=false` trên spec gộp, :4011; container đã dừng sau khi chạy): `HttpRequestPool` + `ApiClient` + `AssignmentService` → `GET /learning/my-assignments` 200, 2 bài từ `example` mới (activity → `activity_host`, assessment có `exam_session_id` → `exam_lobby`); `POST /game/telemetry` qua kiểm tra request của Prism (header `X-Tenant-Id`, Bearer, body `TelemetryBatch`), Prism trả `406` vì response `202` không có media type — client giữ sự kiện trong hàng đợi đúng như khi lỗi.

Ảnh (T031): `tests/visual/hub_screens.gd` với renderer `gl_compatibility` → 12 ảnh HUD + tab "Nhà trường giao" (`g1_3`/`g6_7` × vi/en/xx-PS) trong `tests/scene/screenshots/windows/` (artefact review, không commit).

**Chưa có bằng chứng (ghi rõ):**
- Kịch bản §2 với backend thật (bước 1–12) và `tests/integration/learning/test_my_assignments_isolation.gd` (T018, T033): chờ F041 `Done` (endpoint `listMyAssignments` chưa hiện thực) — F062 giữ `In progress` theo CODE_PLAN §5 (*).
- F062-AC-20, SC-002, SC-004: đo FPS p10/khởi động lạnh trên DEV-PC-MID, DEV-AND-MID1 và lab thiết bị (chạy `res://tests/perf/MainHubBenchmark.tscn -- --seconds=20` với renderer) — chưa có thiết bị/lane F133.
- F062-AC-16 phần nhiệt Android: logic hạ trần có test, nguồn trạng thái nhiệt (plugin `PowerManager`) chờ F133. F062-AC-17 trên thiết bị: cơ chế ghi `user://renderer_override.cfg` (`application/config/project_settings_override`) có test, chưa thử với driver lỗi thật.
- `gdlint` chỉ chạy ở CI (`godot.yml`); mã viết theo cấu hình mặc định gdtoolkit.
