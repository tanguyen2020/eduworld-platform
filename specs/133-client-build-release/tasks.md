# Tasks: F133 — Build và phát hành client Web, Android, iOS

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Image CI Godot ghim phiên bản + export templates (Linux) và cấu hình runner macOS (GitHub-hosted) cho iOS — file: `.github/images/godot/Dockerfile`
- [x] T002 [P] 3 preset export (Android arm64-v8a AAB/APK không intent filter OAuth — đã có từ F060, iOS arm64, Web Compatibility single-threaded), không mật khẩu, không preset Windows (ADR-0101; preset "Windows Desktop" đã gỡ 2026-11-24); thêm preset Web/iOS cần owner duyệt tải export templates — file: `src/game/export_presets.cfg` — **xong 2026-11-25:**
  - **Preset:** Android (preset.0, thêm loại `web/*`), iOS (preset.1: arm64, iOS 15.0, `vn.eduworld.game`, App Store; team id/ký để trống), Web (preset.2: đơn luồng, không GDExtension, ETC2/ASTC, shell `res://web/shell.html`). Cùng `eduworld_production`, chỉ `production.json`, cùng danh sách loại trừ.
  - **Test và export:**
    - `tests/unit/core/test_export_presets.gd`: 6 test pass; `test_config_mock_label.gd` nay đòi đúng 3 preset.
    - Export local Godot 4.7.2: Web `index.wasm` 39 514 754 B, `index.pck` 2 240 404 B; PCK Android 2 240 404 B.
    - `scan_artifact.sh` OK (4 artifact, gồm `index.html`); `check_web_export.sh` OK.
    - APK/AAB không build được trên máy này (không có Android SDK/JDK/Gradle); CI job `export` build APK.
    - iOS: Godot sinh được project Xcode (deployment target 15.0, entitlements rỗng) khi đặt team id tạm (không commit). Export dừng ở "Invalid icon" vì project chưa có icon ứng dụng, xem Clarifications 2026-11-25. Không build được IPA trên Windows.
- [ ] T003 [P] `gen-config.mjs` sinh `src/game/config/<env>.json` từ biến/secret của GitHub Environment (URL, khóa công khai manifest) — file: `scripts/godot/gen-config.mjs`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta contract: `TelemetryEvent` thêm `content_download` (`source`, `bytes`, `bundle_id` maxLength 128) rồi chạy `split-public.mjs`; báo F080 — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint` — ✅ đã có trên main từ F131 (`df48cb4c`, server F080 ingest + metric `eduworld_client_content_download_bytes_total{source}`) với `source` + `bytes`, **không có `bundle_id`** (Clarifications 2026-11-28)
- [ ] T005 [P] Schema `client-release-manifest.schema.json`, `bundle-index.schema.json` (maxLength/pattern) + ví dụ — file: `specs/schemas/client-release-manifest.schema.json` — kiểm: `make specs-lint`
- [ ] T006 Workflow khung `godot.yml` theo path filter (DEPLOYMENT §10.2, ADR-0088), job export/scan/size/bundles/smoke/sign/publish, action ghim SHA, `permissions: contents: read` mặc định — file: `.github/workflows/godot.yml`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Export 3 nền tảng từ tag, có cổng dung lượng và quét bảo mật (Priority: P1) 🎯 MVP

**Goal**: Artifact 3 nền tảng (Web, Android, iOS) tái lập, đã quét, đúng dung lượng, smoke xanh.

**Independent Test**: Như spec.md User Story 1.

### Tests cho User Story 1

- [ ] T007 [P] [US1] Test F133-AC-01, F133-AC-02: `Config` đọc `client_version`/build từ file sinh; bản production bỏ qua override `user://` — file: `src/game/tests/unit/test_config_release.gd` — phải FAIL trước khi code
- [ ] T008 [P] [US1] Test F133-AC-03 (TC-AUTH-010): `scan-artifact.sh` trên fixture chứa secret/URL token/intent filter OAuth/`answer_key` → exit ≠ 0 — file: `scripts/godot/tests/scan-artifact.bats`
- [ ] T009 [P] [US1] Test F133-AC-04: `check-build-size.mjs` với fixture vượt `build_size` từng nền tảng/gói → fail; trong ngưỡng → báo cáo — file: `scripts/godot/check-build-size.test.mjs`
- [ ] T010 [P] [US1] Test F133-AC-05 (TC-CNT-009): `build-bundles.sh` sinh chỉ mục hợp lệ theo schema, khóa theo hash, upload lại là no-op — file: `scripts/godot/tests/build-bundles.bats`

### Implementation cho User Story 1

- [ ] T011 [US1] `export.sh` (3 target, nhúng version/build) + job `godot / export-{android,ios,web}` (iOS trên runner macOS; không Windows — ADR-0101) — file: `scripts/godot/export.sh`
- [ ] T012 [US1] `scan-artifact.sh`, `check-build-size.mjs` (ngưỡng đọc PERFORMANCE_BUDGET §11) + job — file: `scripts/godot/scan-artifact.sh`
- [ ] T013 [US1] `build-bundles.sh` + `sign-manifest.mjs` (RSA-SHA256) + upload `ew-game-bundles` — file: `scripts/godot/build-bundles.sh`
- [ ] T014 [US1] `smoke.sh` F133-AC-06 (TC-AUTH-001): headless boot → `getClientConfig` → `gameLogin` QA → `createGameSession` → Hub; chặn publish khi đỏ — file: `scripts/godot/smoke.sh`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Ký số và phát hành theo kênh (Priority: P1)

**Goal**: Ký, đẩy kênh, promote thủ công, N-1 (không có tự cập nhật Windows — ADR-0101).

**Independent Test**: Như spec.md User Story 2.

### Tests cho User Story 2

- [ ] T015 [P] [US2] ~~Test F133-AC-10~~ — F133-AC-10 không làm (ADR-0101); giữ test `ManifestVerifier` (chữ ký đúng/sai, SHA-256) vì chỉ mục gói và manifest phát hành vẫn ký (F133-AC-05, Edge Cases) — file: `src/game/tests/unit/test_manifest_verifier.gd` — phải FAIL trước khi code
- [ ] T016 [P] [US2] Test F133-AC-12 (TC-AUTH-066): `UpdateService` với `426 CLIENT_VERSION_UNSUPPORTED` hoặc `update_required = true` → signal `update_required` + link kênh đúng nền tảng — file: `src/game/tests/unit/test_update_service.gd`
- [ ] T017 [P] [US2] Kiểm job F133-AC-07, F133-AC-08: job ký/publish chỉ chạy trên tag `client/v*` (ruleset) + GitHub Environment có required reviewers; log không in khóa (kiểm bằng `actionlint` + parser điều kiện `if`/`environment` của workflow + grep log mẫu) — file: `scripts/godot/tests/release-rules.sh`
- [ ] T018 [P] [US2] Test F133-AC-09: header CDN (`immutable` cho thư mục version, `index.html` max-age ngắn, Brotli) qua `curl` trên `play-staging` — file: `scripts/godot/tests/web-headers.sh`

### Implementation cho User Story 2

- [ ] T019 [US2] Job `godot / sign-*` (apksigner, codesign/fastlane match trên runner macOS) từ secret của GitHub Environment — file: `.github/workflows/godot.yml`
- [ ] T020 [P] [US2] fastlane Android (`supply` track internal, promote thủ công) và iOS (`pilot`, `deliver`) — file: `src/game/platform/android/fastlane/Fastfile`
- [ ] T021 [P] [US2] ~~Inno Setup (cài im lặng, tham số `/SHARED`, `/LANCACHE`, ghi `device.cfg`) + ký — file: `deploy/installer/windows/eduworld.iss`~~ — **không làm** (ADR-0101, 2026-11-24: bỏ bản Windows); mã giữ để truy vết
- [ ] T022 [P] [US2] Publish Web versioned + header CDN + rollback `index.html` — file: `deploy/cdn/play/headers.yaml`
- [ ] T023 [US2] `ManifestVerifier`, `UpdateService` (mobile mở store/APK MDM, Web reload; không tự cập nhật Windows — ADR-0101) + `UpdateRequired.tscn` (chuỗi qua file dịch, style theo theme) — file: `src/game/scripts/core/update_service.gd`
- [ ] T024 [US2] Job `godot / n-1-smoke` F133-AC-12 chạy bản đang phát hành với staging trước release backend; runbook halt rollout F133-AC-11 — file: `docs/11-operations/runbooks/ClientCrashRateHigh.md`

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — Chứng nhận trên device matrix 3 nền tảng (Priority: P1)

**Goal**: Release candidate đạt AC-PERF trên lab (PC phòng máy đo trên bản Web), đăng nhập form 3 nền tảng.

**Independent Test**: Như spec.md User Story 3.

### Tests cho User Story 3

- [ ] T025 [P] [US3] Job F133-AC-14 (TC-PERF-018): benchmark headless chạy trên bản build release, chặn khi vượt `perf_budget_tolerance_ci` — file: `.github/workflows/godot.yml` — phải FAIL trước khi code
- [x] T026 [P] [US3] Playwright F133-AC-16 (TC-SIM-005 tham chiếu): `shell.html` khi không có WebGL 2 hiện thông báo; Firefox/Safari có WebGL 2 chỉ có banner, không bị chặn (OQ-04, Clarifications 2026-11-25); Chrome tải engine single-threaded — file: `src/game/web/tests/shell.browser.test.mjs` (+ `shell.test.mjs` jsdom; `.mjs` thay `.ts` vì game không có toolchain TypeScript) — **xong 2026-11-25:**
  - jsdom `shell.test.mjs` 9/9 pass: không WebGL 2/không secure context → thông báo và không tải engine; Firefox/Safari/Chrome iOS/Opera → banner và engine vẫn tải; Chrome/Edge/Chrome Android → không banner, engine chỉ `start` sau khi bấm; heap 512 MiB/1 GiB; không dùng storage; CSS chỉ dùng token.
  - Playwright trên bản export: Chromium (headless shell) 2/2 pass. Có tải, không `AudioContext` trước khi bấm, Godot 4.7 khởi động sau "Chơi", không lỗi trang; khi tắt WebGL thì có thông báo và không request `.wasm`/`.pck`.
  - WebKit bỏ qua: bản WebKit mà Playwright 1.64.1 cần chưa cài, tải bị chặn. Firefox không có trong bộ trình duyệt của repo; Gecko được kiểm bằng test UA của jsdom.
  - CI `godot.yml` job `export` chạy `make godot-export-web` + `node --test src/game/web/tests/` (phần trình duyệt tự bỏ qua khi thiếu Playwright).
- [ ] T027 [P] [US3] Checklist lab F133-AC-13 (TC-PERF-009, TC-PERF-019…024) và F133-AC-15 (TC-AUTH-001) + mẫu báo cáo — file: `docs/12-project/perf-reports/client-template.md`

### Implementation cho User Story 3

- [x] T028 [US3] `src/game/web/shell.html` (kiểm WebGL 2/UA, âm thanh sau tương tác) và cấu hình heap Web theo PERFORMANCE_BUDGET §14 — file: `src/game/web/shell.html` — **xong 2026-11-25:**
  - **Shell:** kiểm WebGL 2/WebAssembly/fetch/secure context trước khi chèn `index.js`; banner cho trình duyệt khác Chrome/Edge; heap qua `WebAssembly.Memory.prototype.grow` (dành 512 MiB lần đầu, chặn trên 1 GiB); tải engine + PCK ngay nhưng chỉ `engine.start()` khi bấm "Chơi"; chuỗi vi/en.
  - **Giao diện:** `shell_tokens.css` sinh từ `design/game-tokens` (output mới `gameShellCss` của `make design-tokens`, check separation coi là output game).
  - **Công cụ:** `make godot-export-web`, `godot-web-preview` (`tools/web/serve_web.mjs`, cổng 8060), `godot-web-test`, `godot-export-ios`; `tools/ci/check_web_export.sh` (shell đúng, placeholder đã thay, đơn luồng, không GDExtension, không dùng storage, có `shell_tokens.css`); `scan_artifact.sh` quét cả `.html`.
  - **CORS bản Web khác origin** (Clarifications 2026-11-25): APISIX `cors: game` + `EW_CORS_GAME_ORIGINS`, `allow_headers` thêm `X-Client-Version`/`X-Client-Platform`, Origin của realtime trong compose. Test `deploy/apisix/scripts/render.test.mjs` (2 test mới pass).
  - Bằng chứng test: xem T026.
- [ ] T029 [US3] Bật `certificate_pinning` cho Android/iOS trong `ApiClient` (phối hợp F061; ADR-0101) — file: `src/game/scripts/networking/api_client.gd`
- [ ] T030 [US3] Chạy chứng nhận trên lab, ghi báo cáo — file: `docs/12-project/perf-reports/client-2026-12-01.md`

**Checkpoint**: User Story 3 chạy và kiểm độc lập được.

## Phase 6: User Story 4 — Phòng máy: gói APK trường kèm nội dung, cache LAN, tải trước ngoài giờ (Priority: P2)

**Goal**: NFR-PERF-019.

**Independent Test**: Như spec.md User Story 4.

### Tests cho User Story 4

- [ ] T031 [P] [US4] Test F133-AC-18: `DeviceConfig.parse` bỏ `lan_cache_url` sai/quá dài; `ContentDownloader` thử preinstalled → LAN → CDN, từ chối hash lệch, không bao giờ gửi URL API qua LAN — file: `src/game/tests/unit/test_content_downloader.gd` — phải FAIL trước khi code
- [ ] T032 [P] [US4] Test F133-AC-19, F133-AC-20: `PrefetchScheduler` chỉ chạy khi `shared_device` + trong `offhours_prefetch_window`, không gọi endpoint cần token; `low_data_mode` tắt tải trước — file: `src/game/tests/unit/test_prefetch_scheduler.gd`
- [x] T033 [P] [US4] Test F133-AC-21 (TC-ANL-003): event `content_download` đúng schema, không có trường định danh — file: `src/game/tests/unit/test_download_telemetry.gd` — ✅ 2026-11-28 (viết trước code, FAIL rồi PASS): `tests/unit/networking/test_content_download_telemetry.gd` (mỗi nguồn một event, cộng dồn cùng nguồn tới lô kế tiếp, kẹp 0–10 GiB, nguồn lạ bỏ, `VideoFileCache` tải xong → `cdn` + kích thước file, cache hit/tải lỗi không gửi, không URL/hash/`bundle_id`/định danh) + `tests/contract/test_telemetry_fixtures.gd` (lô thật khớp fixture `TelemetryBatch` sinh từ contract và enum/khoảng của `TelemetryEvent`)
- [ ] T034 [P] [US4] Test F133-AC-17: `build-kit.sh` quét gói APK trường không có secret/dữ liệu tenant/câu hỏi đề, trong `build_size` — file: `deploy/installer/school-kit/tests/kit.bats`
- [ ] T035 [P] [US4] Kịch bản F133-AC-22: 40 client headless + giới hạn băng thông `classroom_reference_bandwidth` (tc netem) với cache LAN — file: `tests/classroom/simulate-40.sh`
- [ ] T049 [P] [US4] ~~Test F133-AC-27 trên runner Windows: cài mặc định và `/SHARED=0` (im lặng) → `device.cfg` đúng; máy dùng chung sau đăng nhập + thoát không còn bản ghi DPAPI — file: `ci/tests/windows_installer_shared_device.ps1`~~ — **không làm** (ADR-0101, 2026-11-24: bỏ bản Windows); mã giữ để truy vết

### Implementation cho User Story 4

- [ ] T036 [US4] `DeviceConfig` (đọc `device.cfg` theo nền tảng) — file: `src/game/scripts/core/device_config.gd`
- [ ] T037 [US4] `ContentDownloader` (nguồn, SHA-256, LRU, telemetry) + `BandwidthProbe` — file: `src/game/scripts/networking/content_downloader.gd` — ⏳ phần telemetry có sẵn từ 2026-11-28: gọi `TelemetryService.queue_content_download(source, bytes)` (hằng `DOWNLOAD_SOURCE_*`) khi một gói tải xong; `VideoFileCache` đã gọi với `cdn` (Clarifications 2026-11-28)
- [ ] T038 [US4] `PrefetchScheduler` (chỉ mục gói có chữ ký + media công khai đã biết) — file: `src/game/scripts/networking/prefetch_scheduler.gd`
- [ ] T039 [US4] Gói APK trường (APK kèm gói theo khối, delta theo hash; không installer Windows — ADR-0101) — file: `deploy/installer/school-kit/build-kit.sh`
- [ ] T050 [US4] ~~Trang chọn "Máy dùng chung"/"Máy cá nhân" + tham số im lặng trong script bộ cài Windows ghi `device.cfg` — file: `src/game/export/windows/installer.iss`~~ — **không làm** (ADR-0101, 2026-11-24: bỏ bản Windows); mã giữ để truy vết

**Checkpoint**: User Story 4 chạy và kiểm độc lập được.

## Phase 7: User Story 5 — Kho bảo mật giữ phiên trong bản phát hành (Priority: P1)

**Goal**: Adapter iOS Keychain, cấu hình backup Android, kiểm adapter theo nền tảng trong CI (ADR-0086).

**Independent Test**: Như spec.md User Story 5.

### Tests cho User Story 5

- [ ] T043 [P] [US5] Test F133-AC-23: `IosKeychainSecureStorage` round-trip `save`/`load`/`erase`, thuộc tính `AfterFirstUnlockThisDeviceOnly`, `kSecAttrSynchronizable = false` (chạy trên simulator trong runner macOS) (TC-AUTH-082) — file: `src/game/tests/integration/platform/test_ios_keychain_secure_storage.gd` — phải FAIL trước khi code
- [ ] T044 [P] [US5] Test F133-AC-24, F133-AC-25: `check-secure-storage.mjs` trên fixture artifact thiếu thư viện native / manifest Android không loại backup / entitlement iOS có Keychain Sharing / Web shell ghi localStorage → exit ≠ 0 (TC-AUTH-084) — file: `scripts/godot/tests/check-secure-storage.test.mjs`
- [ ] T045 [US5] Kịch bản chứng nhận F133-AC-26 trên lab Android/iOS (TC-AUTH-082, TC-AUTH-088) — file: `docs/12-project/perf-reports/client-2026-12-01.md`

### Implementation cho User Story 5

- [ ] T046 [US5] Plugin iOS (Swift) `IosKeychainSecureStorage` + adapter GDScript, đăng ký trong `SecureStorageFactory` (F060) — file: `src/game/addons/eduworld_platform/ios/IosKeychainSecureStorage.swift`
- [ ] T047 [P] [US5] `data_extraction_rules.xml` + `fullBackupContent` loại thư mục phiên trong manifest Android release — file: `src/game/platform/android/res/xml/data_extraction_rules.xml`
- [ ] T048 [P] [US5] `check-secure-storage.mjs` và gắn vào job `godot / scan-artifact` cho mọi target — file: `scripts/godot/check-secure-storage.mjs`

**Checkpoint**: User Story 5 chạy và kiểm độc lập được.

## Phase N: Polish & Cross-Cutting

- [ ] T040 [P] GODOT_ARCHITECTURE §11: gỡ intent filter `oauth2/callback` và "redirect OIDC cùng tab" (ADR-0068, D16), thêm `device.cfg`, update service; DEPLOYMENT §9: `content_manifest_url` → `world_manifest_url` theo `ClientConfig` — file: `docs/08-game-client/GODOT_ARCHITECTURE.md`
- [ ] T041 [P] PERFORMANCE_BUDGET §10.1/§15: vị trí `device.cfg`, chỉ mục gói ký; CHANGELOG mục F133 — file: `CHANGELOG.md`
- [ ] T042 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/133-client-build-release/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → US3 (cần artifact ký từ US2) → US4 → Polish; US5 sau US1 (cần F060 US5 `SecureStorage`), T045 cùng đợt chứng nhận US3.
- T004 trước T037 (telemetry); T005 trước T013, T023; T013 trước T038, T039.
- F062, F061, F014 Done trước US3; F080 ingest `content_download` trước khi đánh giá F133-AC-21 end-to-end.

## Definition of Done

- [ ] Mọi tiêu chí F133-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
