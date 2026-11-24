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
| `--export-pack "Windows Desktop"` + `scan_artifact.sh` (= `make godot-export-check`; từ 2026-11-24 preset "Android" — ADR-0101) | `scan_artifact: OK (1 artifact(s), 13 patterns)` — không `mock.json`, không `tests/`, không `tools/` |
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
- F062-AC-20, SC-002, SC-004: đo FPS p10/khởi động lạnh trên DEV-WEB-MID (PC chạy Web — ADR-0101), DEV-AND-MID1 và lab thiết bị (chạy `res://tests/perf/MainHubBenchmark.tscn -- --seconds=20` với renderer) — chưa có thiết bị/lane F133.
- F062-AC-16 phần nhiệt Android: logic hạ trần có test, nguồn trạng thái nhiệt (plugin `PowerManager`) chờ F133. F062-AC-17 trên thiết bị: cơ chế ghi `user://renderer_override.cfg` (`application/config/project_settings_override`) có test, chưa thử với driver lỗi thật.
- `gdlint` chỉ chạy ở CI (`godot.yml`); mã viết theo cấu hình mặc định gdtoolkit.

## 6. Bằng chứng (2026-11-24, T036–T038 + chạy với backend thật, Godot 4.7.2 console)

**Tự động (local Windows, máy đang tải 100 % CPU bởi agent khác):**

| Lệnh | Kết quả |
|---|---|
| GdUnit4 `-a res://tests` (= `make godot-test`) | **778 test: 767 pass, 1 fail, 10 skip** (integration tự skip khi thiếu `EDUWORLD_IT_API_BASE_URL`). Fail duy nhất: `test_character_creation_scene.gd::test_every_choice_redraws_the_preview_without_a_request` (F070, timing) — xem dưới |
| Suite F062 sửa/thêm: `test_quest_journal_assigned.gd`, `test_assignment_service.gd`, `test_activity_host.gd` | 9/9, 11/11, 11/11 (mới: `test_question_set_opens_the_activity_host_with_the_set_id`, `test_row_shows_the_subject_name_only_when_the_server_sends_it`, `test_model_reads_subject_name_and_ignores_a_malformed_one`) |
| `check_typed`, `lint_theme_overrides`, `lint_layers`, `lint_signals`, `extract_pot --check`, `pseudo_locale --check` | OK (413 / 376 / 214 / 241 / 334 file; pseudo 1 file) |
| `node src/game/tools/gen_fixtures.mjs --check`, `check_no_admin_ui.sh` | `gen_fixtures: OK (62 files)`, OK |
| `scan_artifact_test.sh`, `--export-pack "Windows Desktop"` + `scan_artifact.sh` (bằng chứng 2026-11-20; nay `make godot-export-check` export preset "Android" — ADR-0101) | 20/20; `scan_artifact: OK (1 artifact(s), 13 patterns)` |
| `node scripts/specs/check-features.mjs` | 92 feature, 0 lỗi |

Test timing F070: chạy riêng 3 lần đều fail (183 ms > 100 ms) khi máy 100 % CPU. Đã đổi sang best of 3 lượt + dung sai CI 5 % (cách của `minigame_benchmark_test.gd`, không đổi ngưỡng 100 ms): lượt chậm nhất mỗi lượt đo được `[137208, 123551, 101884]` µs (pass) và `[122516, 119652, 113895]` µs (fail) — mọi nút chậm đều (~50 ms/nút, mũi tên tóc/trang phục ~100 ms), tức tải máy kéo dài chứ không phải một lần giật; cần chạy lại khi máy rảnh.

**Backend thật** (gateway `http://localhost:9080/api/v1`, stack compose local của owner; không khởi động lại/đổi cấu hình container; mật khẩu chỉ qua biến môi trường trong lệnh; mỗi học sinh đăng nhập một lần mỗi lượt, không thử lại):

```bash
EW_E2E_PASSWORD=*** bash -c 'EDUWORLD_IT_STUDENT_PASSWORD="$EW_E2E_PASSWORD" EDUWORLD_IT_CLIENT_VERSION=0.1.0 \
  EDUWORLD_IT_API_BASE_URL=http://localhost:9080/api/v1 godot --headless --path src/game \
  -s res://addons/gdUnit4/bin/GdUnitCmdTool.gd --ignoreHeadlessMode -c \
  -a res://tests/integration/learning/test_my_assignments_isolation.gd'
```

| Lượt (giờ VN) | Kết quả |
|---|---|
| 05:40 | Cả hai `POST /auth/game/login` → `426 CLIENT_VERSION_UNSUPPORTED` (client `0.0.0-dev` < `EW_GAME_MIN_CLIENT_VERSION`; server kiểm trước mật khẩu) → thêm `EDUWORLD_IT_CLIENT_VERSION` (Clarifications 2026-11-24) |
| 05:43 | `hs.an.3a` → `503 UNAVAILABLE` sau 3,4 s (trace `eac424c65e938df2c8480df4f8386eb7`, nghi Keycloak quá `keycloak_token_call_timeout` 3 s); `hs.dung.7a` → `403 CURFEW_ACTIVE` (trace `fa15541f3614faaaa9955192c2e17e80`, giờ giới nghiêm 21:30–06:30). Không thử lại |
| 06:32 | Cả hai: `login` 200 → `POST /game/sessions` 201 → `GET /game/profile` 200 (`setup_required` → S02, state `CHARACTER_CREATION`) → `GET /learning/my-assignments` **200** có `X-Tenant-Id` đúng tenant của phiên → `:end` 204 → `logout` 204. `hs.an.3a` (tenant `01a1fc6d-7d54-…21f0`, trace `ac50c8ab4c797ed0b2e86328562b4895`): **0 bài**; `hs.dung.7a` (tenant `01a1fc6d-7d92-…9397`, trace `2bbc5c2c453657053d89534dbdb4452d`): **0 bài**. Test **FAIL** ở `assert_array(an).is_not_empty()` — stack local chưa có bài giao nào cho DEMOA1 khối 3 |

Kết luận: đường đăng nhập → phiên game → `listMyAssignments` (F041 thật, header tenant) chạy được trên backend thật; **chưa chứng minh được cô lập tenant có dữ liệu** (T018/TC-LMS-002) và các bước §2 cần bài giao (3–7) vì không có bài nào, mà phạm vi được giao chỉ cho đọc/chơi (không tạo bài bằng `ad.truong.a`). Bước §2 cần màn hình/thiết bị (2, 8, 9, 10, 12) chưa chạy (agent headless).

**Còn lại để Done:** (1) seed/giao ít nhất 1 bài `activity` (và nếu có, 1 `question_set`, 1 đề) cho DEMOA1 khối 3 bằng `ad.truong.a` (F041), rồi chạy lại đúng lệnh trên ngoài giờ giới nghiêm → T018 pass; (2) T033: chạy §2 bước 1–11 bằng client có giao diện (`--env=local` cần `client_version` ≥ `EW_GAME_MIN_CLIENT_VERSION`); (3) test F070 timing chạy lại khi máy rảnh; (4) F062-AC-20/SC-002 đo thiết bị — hoãn sang F133 (đã thống nhất).

### 6.1 Lượt 2 (2026-11-24, sau quyết định owner "chốt hết")

- **Seed bài giao DEV** (module learning, `make seed-content`): `seed/assignments/demoa1_g3.dev.json` — `ad.truong.a` tạo và publish qua API LMS 1 bài `activity` + 1 bài `question_set` cho th-demo-a/DEMOA1 khối 3 (mở từ 1 giờ trước lần chạy đầu, hạn 365 ngày), không có bài cho pgd-demo; không có đề (seed chưa có assessment blueprint). `go test ./modules/learning/internal/presentation/devseed/` ok (`TestAssignmentSeedFileValid`, `TestRunAssignmentsIsIdempotent`: lần 2 không tạo/publish gì, bản nháp bị bỏ dở được publish bằng ETag của GET); `go vet`, `golangci-lint` 0 issue.
- **Chưa nạp vào stack của owner:** lệnh `EW_SEED_PASSWORD=… EW_ENV=local EW_SEED_API_URL=http://localhost:9080/api/v1 EW_SEED_INTERNAL_API_URL=http://localhost:9180/api/v1 go run ./modules/learning/cmd/seed-content` bị hệ thống phân quyền của phiên agent chặn (ghi dữ liệu vào stack đang chạy). Không chạy cách khác. Vì không có bài, **không chạy lại T018** (tránh đăng nhập vô ích; kết quả sẽ giống lượt 06:32).
- **`config/env/local.json` `client_version = 0.1.0`:** boot headless `Boot.tscn -- --env=local` → `boot_completed` (`client_version 0.1.0`), `splash_shown`, `GET /game/client-config` 200 (trace `ce335a065f6aa1b50fde3b1dc8a1c9cd`), không vào ForceUpdate.
- **T033 headless:** bước 1 (boot → version check với backend thật) OK; bước 1 đăng nhập → phiên → `listMyAssignments` đã có ở lượt 06:32; bước 3–7 chờ dữ liệu seed; bước 2, 8, 9, 10, 12 cần giao diện/thiết bị → owner kiểm tay; F062-AC-20/SC-002 → F133.

**Để Done (theo thứ tự):** (1) owner chạy `EW_SEED_PASSWORD=<mật khẩu demo> make seed-content`; (2) chạy lại lệnh T018 ở §6 ngoài 21:30–06:30 (không cần `EDUWORLD_IT_CLIENT_VERSION` nếu dùng `--env=local`; harness vẫn cần biến này vì test nạp profile khác); (3) bước giao diện của §2 do owner kiểm tay.

### 6.2 Lượt 3 (2026-11-24 17:07–17:10 ICT, owner cho chạy seed)

- **Seed:** `EW_ENV=local EW_SEED_PASSWORD=*** … go -C src/backend run ./modules/learning/cmd/seed-content` → `learning content seed done … published_now=0`, `learning assignment seed done tenant=th-demo-a school=DEMOA1 assignments=2 created_now=2 published_now=2`.
- **T018:** lệnh ở §6 (Godot 4.7.2 console, sau `--import`) → pass 2026-11-24 17:10 ICT với backend thật sau khi nạp seed bài giao (`seed-content`: 2 bài DEMOA1 khối 3): `hs.an.3a` 2 bài (`activity` 1, `question_set` 1, đủ `subject_name`, 2 intent; trace `06eedef3b5aac210ef03da4d3f0bf21e`), `hs.dung.7a` 0 bài (trace `74aa4fe6a5e3f964457c5d5aa76d6aac`) — 1/1 test case, 0 failures. Mỗi học sinh đăng nhập một lần, phiên kết thúc + đăng xuất 204.
- **Còn lại (không chặn Done):** bước giao diện 2, 8, 9, 10 của §2 do owner kiểm tay trên cửa sổ `--env=local`; F062-AC-20/SC-002 đo thiết bị lab → F133; test timing F070 phụ thuộc tải máy.
