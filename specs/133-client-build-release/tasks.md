# Tasks: F133 — Build và phát hành client Web, Android, iOS

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Image CI Godot ghim phiên bản + export templates (Linux) và cấu hình runner macOS (GitHub-hosted) cho iOS — file: `.github/images/godot/Dockerfile` — ⏳ **pending: cần CI image + runner** (Dockerfile Godot ghim + export templates cho runner self-hosted `deploy`, `setup-godot.sh macos` cho `macos-14`; hiện `setup-godot.sh` chỉ có `linux`, job `export-ios` tắt bằng `vars.IOS_BUILD_ENABLED`)
- [x] T002 [P] 3 preset export (Android arm64-v8a AAB/APK không intent filter OAuth — đã có từ F060, iOS arm64, Web Compatibility single-threaded), không mật khẩu, không preset Windows (ADR-0101; preset "Windows Desktop" đã gỡ 2026-11-24); thêm preset Web/iOS cần owner duyệt tải export templates — file: `src/game/export_presets.cfg` — **xong 2026-11-25:**
  - **Preset:** Android (preset.0, thêm loại `web/*`), iOS (preset.1: arm64, iOS 15.0, `vn.eduworld.game`, App Store; team id/ký để trống), Web (preset.2: đơn luồng, không GDExtension, ETC2/ASTC, shell `res://web/shell.html`). Cùng `eduworld_production`, chỉ `production.json`, cùng danh sách loại trừ.
  - **Test và export:**
    - `tests/unit/core/test_export_presets.gd`: 6 test pass; `test_config_mock_label.gd` nay đòi đúng 3 preset.
    - Export local Godot 4.7.2: Web `index.wasm` 39 514 754 B, `index.pck` 2 240 404 B; PCK Android 2 240 404 B.
    - `scan_artifact.sh` OK (4 artifact, gồm `index.html`); `check_web_export.sh` OK.
    - APK/AAB không build được trên máy này (không có Android SDK/JDK/Gradle); CI job `export` build APK.
    - iOS: Godot sinh được project Xcode (deployment target 15.0, entitlements rỗng) khi đặt team id tạm (không commit). Export dừng ở "Invalid icon" vì project chưa có icon ứng dụng, xem Clarifications 2026-11-25. Không build được IPA trên Windows.
- [x] T003 [P] `gen-config.mjs` sinh `src/game/config/<env>.json` từ biến/secret của GitHub Environment (URL, khóa công khai manifest) — file: `scripts/godot/gen-config.mjs` — **xong 2026-11-28:** `scripts/godot/gen-config.mjs` (biến `EW_GAME_*` của Environment → `config/env/<env>.json`; `client_version` từ tag `client/vX.Y.Z`/`EW_GAME_CLIENT_VERSION`/`0.0.0-ci.<run>`, `build_number`; staging/production chỉ https/wss, không localhost/cổng 4011/query/credential, khóa công khai RSA ≥ 3072, khóa riêng bị từ chối; `--keep-urls` cho build ngoài tag). Test `scripts/godot/tests/gen-config.test.mjs` 7/7 (viết trước: FAIL vì chưa có module).

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta contract: `TelemetryEvent` thêm `content_download` (`source`, `bytes`) rồi chạy `split-public.mjs`; báo F080 — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint` — ✅ đã có trên main từ F131 (`df48cb4c`, server F080 ingest + metric `eduworld_client_content_download_bytes_total{source}`) với `source` + `bytes`; **không** thêm `bundle_id` — [Confirmed — owner 2026-11-28 "chốt hết"] (Clarifications 2026-11-28): ít dữ liệu hơn an toàn hơn, `additionalProperties: false`, không delta
- [x] T005 [P] Schema `client-release-manifest.schema.json`, `bundle-index.schema.json` (maxLength/pattern) + ví dụ — file: `specs/schemas/client-release-manifest.schema.json` — kiểm: `make specs-lint` — **xong 2026-11-28:** `client-release-manifest.schema.json`, `bundle-index.schema.json` (`schema_version: 1`, mọi chuỗi `maxLength`/`pattern`, URL gói `…/bundles/<sha256>.pck` không query) + `specs/examples/client-release-manifest.android-staging.json`, `bundle-index.0-9-0.json` trong bảng ví dụ của `validate-schemas.mjs` (271 schema, 21 example, 0 lỗi); `check-maxlength` 0 vi phạm; `split-public --check`, `specs-breaking` (oasdiff `tufin/oasdiff:v1.11.7`) 0 breaking; fixture client sinh lại (`gen_fixtures.mjs`, 223 file).
- [x] T006 Workflow khung `godot.yml` theo path filter (DEPLOYMENT §10.2, ADR-0088), job export/scan/size/bundles/smoke/sign/publish, action ghim SHA, `permissions: contents: read` mặc định — file: `.github/workflows/godot.yml` — **xong 2026-11-28 (khung):** path filter thêm `scripts/godot/**`, `deploy/installer/school-kit/**`, `deploy/cdn/play/**` + tag `client/v*`; job `release-tools`, `scan-artifact`, `size-check` (đính báo cáo), `bundles` (khóa ký dùng một lần ngoài phát hành), `export-ios` (`macos-14`, tag/main, `vars.IOS_BUILD_ENABLED`), lane phát hành `smoke-staging` → `publish-staging` → `publish-production` (Environment, `[self-hosted, deploy]`, chặn fork, `vars.CLIENT_RELEASE_ENABLED`); secret chỉ ở env của bước. Kiểm: `actionlint` 1.7.7 (docker, nhãn `deploy`) 0 lỗi, shellcheck 0, `check-workflows.mjs` 17 workflow 0 lỗi, `scripts/ci/godot-rules.test.mjs` 14/14 (7 test mới viết trước, FAIL 6 trước khi sửa), `path-filters.test.mjs` 29/29, `node --test scripts/ci/` 64/64.

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Export 3 nền tảng từ tag, có cổng dung lượng và quét bảo mật (Priority: P1) 🎯 MVP

**Goal**: Artifact 3 nền tảng (Web, Android, iOS) tái lập, đã quét, đúng dung lượng, smoke xanh.

**Independent Test**: Như spec.md User Story 1.

### Tests cho User Story 1

- [x] T007 [P] [US1] Test F133-AC-01, F133-AC-02: `Config` đọc `client_version`/build từ file sinh; bản production bỏ qua override `user://` — file: `src/game/tests/unit/test_config_release.gd` — phải FAIL trước khi code — **xong 2026-11-28** ở `tests/unit/core/test_config_release.gd` (thư mục theo nhóm của repo) 4/4: viết trước, FAIL (parse: chưa có `override_allowed`), rồi PASS — fixture `tests/fixtures/release/config_production.json` sinh bằng `gen-config.mjs` thật (`make_fixtures.mjs`); khóa mới được kiểm kiểu, khóa riêng bị từ chối.
- [x] T008 [P] [US1] Test F133-AC-03 (TC-AUTH-010): `scan-artifact.sh` trên fixture chứa secret/URL token/intent filter OAuth/`answer_key` → exit ≠ 0 — file: `scripts/godot/tests/scan-artifact.bats` — **xong 2026-11-28** theo quy ước test node của repo (như F132): `scripts/godot/tests/scan-artifact.test.mjs` 12/12 (FAIL 7 trước khi thêm mẫu: client `eduworld-game-backend`, khóa riêng PEM sinh lúc chạy, `answer_key`/`correct_option_ids`/`scoring_rule`/`accepted_answers` có giá trị, IPA); `src/game/tools/tests/scan_artifact_test.sh` cũ vẫn 13/13.
- [x] T009 [P] [US1] Test F133-AC-04: `check-build-size.mjs` với fixture vượt `build_size` từng nền tảng/gói → fail; trong ngưỡng → báo cáo — file: `scripts/godot/check-build-size.test.mjs` — **xong 2026-11-28** ở `scripts/godot/tests/check-build-size.test.mjs` 3/3 (ngưỡng đọc từ PERFORMANCE_BUDGET §11 thật: 300/300/40/40 MB, cache 1,5 GB; fixture vượt từng nền tảng/gói → exit 1; báo cáo Markdown).
- [x] T010 [P] [US1] Test F133-AC-05 (TC-CNT-009): `build-bundles.sh` sinh chỉ mục hợp lệ theo schema, khóa theo hash, upload lại là no-op — file: `scripts/godot/tests/build-bundles.bats` — **xong 2026-11-28** ở `scripts/godot/tests/build-bundles.test.mjs` 5/5 (chỉ mục hợp lệ theo schema + chữ ký kiểm được, khóa theo hash, upload lại = no-op qua `aws` giả trên PATH, chỉ mục khác cho cùng phiên bản bị từ chối, mục sai bị từ chối, danh sách rỗng hợp lệ).

### Implementation cho User Story 1

- [x] T011 [US1] `export.sh` (3 target, nhúng version/build) + job `godot / export-{android,ios,web}` (iOS trên runner macOS; không Windows — ADR-0101) — file: `scripts/godot/export.sh` — **xong 2026-11-28 (iOS pending macOS):** `scripts/godot/export.sh <android|ios|web>` (Android APK + AAB, release cần keystore env của job ký, `--debug` cho CI thường; iOS ngoài macOS → exit 3 "pending"); đóng dấu `client_version`/`build_number` qua `gen-config.mjs` rồi khôi phục file đã commit; `make godot-export-android|web` gọi nó. Test `scripts/godot/tests/export.test.mjs` 6/6 (Godot giả). **Bằng chứng thật trên máy này:** `GITHUB_RUN_NUMBER=133 bash scripts/godot/export.sh web` với Godot 4.7.2 → `index.wasm` 39 514 754 B, `index.pck` 9 494 896 B, `check_web_export` OK, `scan_artifact` OK (17 mẫu), `check-secure-storage: web OK`, `index.pck` chứa `"client_version": "0.0.0-ci.133"` + `"build_number": 133`, `production.json` đã khôi phục. ⏳ Android: máy không có Android SDK/JDK (chạy trong CI job `export`); iOS: cần Xcode trên runner macOS (T001/T019).
- [x] T012 [US1] `scan-artifact.sh`, `check-build-size.mjs` (ngưỡng đọc PERFORMANCE_BUDGET §11) + job — file: `scripts/godot/scan-artifact.sh` — **xong 2026-11-28:** `scripts/godot/scan-artifact.sh` (mẫu cấm dùng chung của `src/game/tools/scan_artifact.sh` + `--platform` gọi `check-secure-storage.mjs`), `check-build-size.mjs`; job `scan-artifact`, `size-check` trong `godot.yml`. Bản Web thật: tải đầu Brotli 15,13 MB ≤ 40 MB.
- [x] T013 [US1] `build-bundles.sh` + `sign-manifest.mjs` (RSA-SHA256) + upload `ew-game-bundles` — file: `scripts/godot/build-bundles.sh` — **xong 2026-11-28 (upload thật chờ F130):** `build-bundles.sh` + `lib/bundle-index.mjs` + `sign-manifest.mjs` (`manifest`/`sign`/`verify`; RSA ≥ 3072, PKCS#1 v1.5 SHA-256, base64 `.sig` — cùng định dạng `publish-index.sh`); test `sign-manifest.test.mjs` 4/4 (chữ ký khớp `openssl`, khóa ngắn/không RSA bị từ chối, manifest theo schema, không Windows) + T010. Chữ ký Node được Godot `Crypto` kiểm (T015).
- [x] T014 [US1] `smoke.sh` F133-AC-06 (TC-AUTH-001): headless boot → `getClientConfig` → `gameLogin` QA → `createGameSession` → Hub; chặn publish khi đỏ — file: `scripts/godot/smoke.sh` — **xong 2026-11-28 (lượt đăng nhập thật pending):** `scripts/godot/smoke.sh` (bước 1 `getClientConfig` bằng curl; bước 2 GdUnit `tests/integration/release/test_release_smoke.gd`: boot → client-config → `gameLogin` học sinh QA → `createGameSession` → IN_WORLD; thiếu `EW_SMOKE_USERNAME`/`EW_SMOKE_PASSWORD` → exit 3, làm đỏ job). Test `scripts/godot/tests/smoke.test.mjs` 4/4 (API giả, Godot giả). **Chạy thật với backend local đang chạy:** `smoke.sh --api=http://localhost:9080/api/v1 --client-version=0.1.0` → `getClientConfig OK (min_client_version 0.1.0)`, rồi exit 3 PENDING; `--client-version=0.0.1` → exit 1 "requires an update". ⏳ đăng nhập → phiên → Hub: cần mật khẩu học sinh QA (secret, không có trên máy này).

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Ký số và phát hành theo kênh (Priority: P1)

**Goal**: Ký, đẩy kênh, promote thủ công, N-1 (không có tự cập nhật Windows — ADR-0101).

**Independent Test**: Như spec.md User Story 2.

### Tests cho User Story 2

- [x] T015 [P] [US2] ~~Test F133-AC-10~~ — F133-AC-10 không làm (ADR-0101); giữ test `ManifestVerifier` (chữ ký đúng/sai, SHA-256) vì chỉ mục gói và manifest phát hành vẫn ký (F133-AC-05, Edge Cases) — file: `src/game/tests/unit/test_manifest_verifier.gd` — phải FAIL trước khi code — **xong 2026-11-28** ở `tests/unit/core/test_manifest_verifier.gd` 6/6: viết trước (FAIL: chưa có `ManifestVerifier`), chỉ mục + manifest ký bằng Node (`make_fixtures.mjs`, khóa riêng không lưu) kiểm được bằng Godot `Crypto`; đổi 1 byte/khóa khác/base64 sai/khóa hỏng/phiên bản khác/nền tảng khác/mục sai schema → từ chối; SHA-256 file đọc theo khối. **Sửa 2026-11-28:** khóa công khai fixture đổi `release_public.pem` → `release_public_key.txt` (`.gitignore` bỏ qua `*.pem` nên file cũ chưa từng được commit, suite đỏ trên mọi checkout mới); thêm test fixture có mặt + trùng khóa trong `config_production.json` → 7/7.
- [x] T016 [P] [US2] Test F133-AC-12 (TC-AUTH-066): `UpdateService` với `426 CLIENT_VERSION_UNSUPPORTED` hoặc `update_required = true` → signal `update_required` + link kênh đúng nền tảng — file: `src/game/tests/unit/test_update_service.gd` — **xong 2026-11-28** ở `tests/unit/core/test_update_service.gd` 7/7 (426 + `store_url` của server → mở store Android; `update_required` iOS → link App Store của build; dưới `min_client_version` → cập nhật; Web → tải lại; máy Android dùng chung → nhờ thầy cô; link không https không bao giờ mở).
- [ ] T017 [P] [US2] Kiểm job F133-AC-07, F133-AC-08: job ký/publish chỉ chạy trên tag `client/v*` (ruleset) + GitHub Environment có required reviewers; log không in khóa (kiểm bằng `actionlint` + parser điều kiện `if`/`environment` của workflow + grep log mẫu) — file: `scripts/godot/tests/release-rules.sh` — ⏳ **pending: cần secret ký thật, ruleset tag và Environment có required reviewers trên GitHub.** Phần kiểm tĩnh đã có trong `godot-rules.test.mjs` (job staging/production chỉ trên tag `client/v*`, Environment, runner `deploy`, chặn fork, secret chỉ ở env của bước, không `echo` secret) + `actionlint`; còn grep log mẫu của lần chạy thật.
- [ ] T018 [P] [US2] Test F133-AC-09: header CDN (`immutable` cho thư mục version, `index.html` max-age ngắn, Brotli) qua `curl` trên `play-staging` — file: `scripts/godot/tests/web-headers.sh` — ⏳ **pending: cần CDN `play-staging` (F130)**; quy tắc header đã có ở `deploy/cdn/play/headers.yaml` và được test (T022).

### Implementation cho User Story 2

- [ ] T019 [US2] Job `godot / sign-*` (apksigner, codesign/fastlane match trên runner macOS) từ secret của GitHub Environment — file: `.github/workflows/godot.yml` — ⏳ **pending: cần keystore/upload key Android, chứng chỉ + provisioning Apple, icon 1024×1024 và runner macOS**; job ký iOS gắn Environment trên `macos-*` cần ngoại lệ luật `check-workflows.mjs` theo ADR-0088 mục 3–4. Bước gói APK trường đã có sẵn sau `vars.ANDROID_SIGNING_ENABLED`.
- [ ] T020 [P] [US2] fastlane Android (`supply` track internal, promote thủ công) và iOS (`pilot`, `deliver`) — file: `src/game/platform/android/fastlane/Fastfile` — ⏳ **pending: cần tài khoản Play Console/App Store Connect và khóa API store.**
- [ ] T021 [P] [US2] ~~Inno Setup (cài im lặng, tham số `/SHARED`, `/LANCACHE`, ghi `device.cfg`) + ký — file: `deploy/installer/windows/eduworld.iss`~~ — **không làm** (ADR-0101, 2026-11-24: bỏ bản Windows); mã giữ để truy vết
- [x] T022 [P] [US2] Publish Web versioned + header CDN + rollback `index.html` — file: `deploy/cdn/play/headers.yaml` — **xong 2026-11-28 (CDN thật chờ F130):** `deploy/cdn/play/headers.yaml` (độc lập nhà cung cấp: `v/<version>/*` Brotli + `immutable`, `index.html` `max-age=60`, không COOP/COEP, CORS chỉ `EW_CORS_GAME_ORIGINS`, không `*`) + `scripts/godot/publish-web.mjs` (asset trước, `index.html` gốc sau cùng với `<base href="/v/<version>/">`, thư mục đã có bị từ chối, `--rollback` chỉ trỏ lại `index.html`). Test `scripts/godot/tests/publish-web.test.mjs` 5/5 (`aws` giả).
- [x] T023 [US2] `ManifestVerifier`, `UpdateService` (mobile mở store/APK MDM, Web reload; không tự cập nhật Windows — ADR-0101) + `UpdateRequired.tscn` (chuỗi qua file dịch, style theo theme) — file: `src/game/scripts/core/update_service.gd` — **xong 2026-11-28:** `ManifestVerifier`, `UpdateService` (`scripts/core/update_service.gd`), `scenes/bootstrap/UpdateRequired.tscn` thay `ForceUpdate` (Mây + bong bóng "Có bản game mới rồi!", **một** nút: "Cập nhật ngay" / "Tải lại trang" / "Thử lại" khi "nhờ thầy cô cập nhật", dòng nhỏ phiên bản cần có cho giáo viên; chuỗi `ui.update.*` vi/en, `messages.pot` + `xx_PS.po` sinh lại, style theo Theme/token). `AuthService.open_store()` dùng `UpdateService` (cả màn cập nhật của activity). Ảnh (Compatibility, `specs/133-client-build-release/screenshots/`): `update_required{,_web,_teacher}-final-{1280x720,844x390,740x360}-vi-g1_3.png` + `update_required-final-1280x720-en-g4_5.png` (`tools/capture_screens.gd --screens=update_required,update_required_web,update_required_teacher`), đã xem từng ảnh: chữ không tràn, nút rõ ở mọi cỡ.
- [ ] T024 [US2] Job `godot / n-1-smoke` F133-AC-12 chạy bản đang phát hành với staging trước release backend; runbook halt rollout F133-AC-11 — file: `docs/11-operations/runbooks/ClientCrashRateHigh.md` — ⏳ **pending: cần client N-1 đã phát hành + staging (F130)**; runbook halt rollout viết cùng lần phát hành đầu.

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — Chứng nhận trên device matrix 3 nền tảng (Priority: P1)

**Goal**: Release candidate đạt AC-PERF trên lab (PC phòng máy đo trên bản Web), đăng nhập form 3 nền tảng.

**Independent Test**: Như spec.md User Story 3.

### Tests cho User Story 3

- [ ] T025 [P] [US3] Job F133-AC-14 (TC-PERF-018): benchmark headless chạy trên bản build release, chặn khi vượt `perf_budget_tolerance_ci` — file: `.github/workflows/godot.yml` — phải FAIL trước khi code — ⏳ **pending: benchmark trên bản build release trong CI** (cần image Godot có renderer + export templates — T001).
- [x] T026 [P] [US3] Playwright F133-AC-16 (TC-SIM-005 tham chiếu): `shell.html` khi không có WebGL 2 hiện thông báo; Firefox/Safari có WebGL 2 chỉ có banner, không bị chặn (OQ-04, Clarifications 2026-11-25); Chrome tải engine single-threaded — file: `src/game/web/tests/shell.browser.test.mjs` (+ `shell.test.mjs` jsdom; `.mjs` thay `.ts` vì game không có toolchain TypeScript) — **xong 2026-11-25:**
  - jsdom `shell.test.mjs` 9/9 pass: không WebGL 2/không secure context → thông báo và không tải engine; Firefox/Safari/Chrome iOS/Opera → banner và engine vẫn tải; Chrome/Edge/Chrome Android → không banner, engine chỉ `start` sau khi bấm; heap 512 MiB/1 GiB; không dùng storage; CSS chỉ dùng token.
  - Playwright trên bản export: Chromium (headless shell) 2/2 pass. Có tải, không `AudioContext` trước khi bấm, Godot 4.7 khởi động sau "Chơi", không lỗi trang; khi tắt WebGL thì có thông báo và không request `.wasm`/`.pck`.
  - WebKit bỏ qua: bản WebKit mà Playwright 1.64.1 cần chưa cài, tải bị chặn. Firefox không có trong bộ trình duyệt của repo; Gecko được kiểm bằng test UA của jsdom.
  - CI `godot.yml` job `export` chạy `make godot-export-web` + `node --test src/game/web/tests/` (phần trình duyệt tự bỏ qua khi thiếu Playwright).
- [ ] T027 [P] [US3] Checklist lab F133-AC-13 (TC-PERF-009, TC-PERF-019…024) và F133-AC-15 (TC-AUTH-001) + mẫu báo cáo — file: `docs/12-project/perf-reports/client-template.md` — ⏳ **pending: lab thiết bị** (PERFORMANCE_BUDGET §2.1).

### Implementation cho User Story 3

- [x] T028 [US3] `src/game/web/shell.html` (kiểm WebGL 2/UA, âm thanh sau tương tác) và cấu hình heap Web theo PERFORMANCE_BUDGET §14 — file: `src/game/web/shell.html` — **xong 2026-11-25:**
  - **Shell:** kiểm WebGL 2/WebAssembly/fetch/secure context trước khi chèn `index.js`; banner cho trình duyệt khác Chrome/Edge; heap qua `WebAssembly.Memory.prototype.grow` (dành 512 MiB lần đầu, chặn trên 1 GiB); tải engine + PCK ngay nhưng chỉ `engine.start()` khi bấm "Chơi"; chuỗi vi/en.
  - **Giao diện:** `shell_tokens.css` sinh từ `design/game-tokens` (output mới `gameShellCss` của `make design-tokens`, check separation coi là output game).
  - **Công cụ:** `make godot-export-web`, `godot-web-preview` (`tools/web/serve_web.mjs`, cổng 8060), `godot-web-test`, `godot-export-ios`; `tools/ci/check_web_export.sh` (shell đúng, placeholder đã thay, đơn luồng, không GDExtension, không dùng storage, có `shell_tokens.css`); `scan_artifact.sh` quét cả `.html`.
  - **CORS bản Web khác origin** (Clarifications 2026-11-25): APISIX `cors: game` + `EW_CORS_GAME_ORIGINS`, `allow_headers` thêm `X-Client-Version`/`X-Client-Platform`, Origin của realtime trong compose. Test `deploy/apisix/scripts/render.test.mjs` (2 test mới pass).
  - Bằng chứng test: xem T026.
- [ ] T029 [US3] Bật `certificate_pinning` cho Android/iOS trong `ApiClient` (phối hợp F061; ADR-0101) — file: `src/game/scripts/networking/api_client.gd` — ⏳ **pending: cần CA bundle của môi trường (chọn khi dựng F130) và tùy chọn TLS cho `HttpRequestPool` của `ApiClient` (task F061)**; `RealtimeClient` đã đọc `res://config/tls/ca_bundle.crt` (F092).
- [ ] T030 [US3] Chạy chứng nhận trên lab, ghi báo cáo — file: `docs/12-project/perf-reports/client-2026-12-01.md` — ⏳ **pending: lab thiết bị.**

**Checkpoint**: User Story 3 chạy và kiểm độc lập được.

## Phase 6: User Story 4 — Phòng máy: gói APK trường kèm nội dung, cache LAN, tải trước ngoài giờ (Priority: P2)

**Goal**: NFR-PERF-019.

**Independent Test**: Như spec.md User Story 4.

### Tests cho User Story 4

- [x] T031 [P] [US4] Test F133-AC-18: `DeviceConfig.parse` bỏ `lan_cache_url` sai/quá dài; `ContentDownloader` thử preinstalled → LAN → CDN, từ chối hash lệch, không bao giờ gửi URL API qua LAN — file: `src/game/tests/unit/test_content_downloader.gd` — phải FAIL trước khi code — **xong 2026-11-28** ở `tests/unit/networking/test_content_downloader.gd` 8/8: viết trước (FAIL: chưa có `DeviceConfig`/`ContentDownloader`), rồi PASS — URL LAN sai (ftp, javascript:, credential, query, > 2048) bị bỏ + cảnh báo `lan_cache_url_invalid`; thứ tự cache → cài sẵn → LAN → CDN; gói LAN lệch hash bị từ chối và lấy CDN; CDN lệch hash = lỗi, không giữ file; chỉ khóa nội dung `/bundles/<sha>.pck`, `/media/<sha>/<file>` không ký mới qua LAN (URL API/media ký → không); LRU; chế độ dữ liệu thấp (device hoặc `BandwidthProbe`).
- [x] T032 [P] [US4] Test F133-AC-19, F133-AC-20: `PrefetchScheduler` chỉ chạy khi `shared_device` + trong `offhours_prefetch_window`, không gọi endpoint cần token; `low_data_mode` tắt tải trước — file: `src/game/tests/unit/test_prefetch_scheduler.gd` — **xong 2026-11-28** ở `tests/unit/networking/test_prefetch_scheduler.gd` 6/6: viết trước (FAIL), khung 18:00–06:00 qua nửa đêm, chỉ máy dùng chung + trong khung + không dữ liệu thấp + có CDN https; chỉ GET chỉ mục ký công khai (không `/api/`), gói đúng khối của `device.cfg`, gói cài sẵn không tải lại, chữ ký sai → không tải + `client_error` `bundle_index_invalid`. **Sửa 2026-11-28:** đọc khóa từ `release_public_key.txt` (xem T015) → 6/6; `DeviceConfig` không còn ghi `ERROR` của engine khi thiếu khóa.
- [x] T033 [P] [US4] Test F133-AC-21 (TC-ANL-003): event `content_download` đúng schema, không có trường định danh — file: `src/game/tests/unit/test_download_telemetry.gd` — ✅ 2026-11-28 (viết trước code, FAIL rồi PASS): `tests/unit/networking/test_content_download_telemetry.gd` (mỗi nguồn một event, cộng dồn cùng nguồn tới lô kế tiếp, kẹp 0–10 GiB, nguồn lạ bỏ, `VideoFileCache` tải xong → `cdn` + kích thước file, cache hit/tải lỗi không gửi, không URL/hash/`bundle_id`/định danh) + `tests/contract/test_telemetry_fixtures.gd` (lô thật khớp fixture `TelemetryBatch` sinh từ contract và enum/khoảng của `TelemetryEvent`)
- [x] T034 [P] [US4] Test F133-AC-17: `build-kit.sh` quét gói APK trường không có secret/dữ liệu tenant/câu hỏi đề, trong `build_size` — file: `deploy/installer/school-kit/tests/kit.bats` — **xong 2026-11-28** theo quy ước test node: `deploy/installer/school-kit/tests/build-kit.test.mjs` 6/6 (chỉ gói của khối, chỉ mục + chữ ký trong `assets/preinstalled/`, chữ ký v1 cũ bị bỏ; gói có `tenant_id`/khóa riêng/`answer_key` bị từ chối; chỉ mục sai chữ ký/khác phiên bản bị từ chối; bản ký → `kits.json` → `publish-index.sh` F134 ra chỉ mục hợp lệ; delta theo hash; thiếu keystore env/khối lạ → dừng) + `publish_index_test.sh` 8/8.
- [ ] T035 [P] [US4] Kịch bản F133-AC-22: 40 client headless + giới hạn băng thông `classroom_reference_bandwidth` (tc netem) với cache LAN — file: `tests/classroom/simulate-40.sh` — ⏳ **pending: cần môi trường 40 client + `tc netem`** (Linux, staging).
- [ ] T049 [P] [US4] ~~Test F133-AC-27 trên runner Windows: cài mặc định và `/SHARED=0` (im lặng) → `device.cfg` đúng; máy dùng chung sau đăng nhập + thoát không còn bản ghi DPAPI — file: `ci/tests/windows_installer_shared_device.ps1`~~ — **không làm** (ADR-0101, 2026-11-24: bỏ bản Windows); mã giữ để truy vết

### Implementation cho User Story 4

- [x] T036 [US4] `DeviceConfig` (đọc `device.cfg` theo nền tảng) — file: `src/game/scripts/core/device_config.gd` — **xong 2026-11-28:** `scripts/core/device_config.gd` (`DeviceConfig.parse/load_file`: `shared_device`, `lan_cache_url` ≤ 2048 http(s) không query/credential, `low_data_mode`, `preinstalled_bundles_dir` ≤ 512 mặc định `res://preinstalled`, `grade_bands`; Web luôn dùng chung, không LAN/cài sẵn); `Config.load_device_config` dùng nó (cảnh báo khi bỏ URL). Test T031 + `test_device_config_defaults.gd` (cũ) pass.
- [x] T037 [US4] `ContentDownloader` (nguồn, SHA-256, LRU, telemetry) + `BandwidthProbe` — file: `src/game/scripts/networking/content_downloader.gd` — ⏳ phần telemetry có sẵn từ 2026-11-28: gọi `TelemetryService.queue_content_download(source, bytes)` (hằng `DOWNLOAD_SOURCE_*`) khi một gói tải xong; `VideoFileCache` đã gọi với `cdn` (Clarifications 2026-11-28) — **xong 2026-11-28:** `ContentDownloader` (cache → gói cài sẵn dùng tại chỗ → LAN → CDN, SHA-256 mọi gói, LRU theo `eduworld/content/cache_limit_bytes` = `build_size` 1,5 GB, `queue_content_download` cho `preinstalled`/`lan_cache`/`cdn`, không tính khi trúng cache) + `BandwidthProbe` (trung bình trượt, ngưỡng `low_data_bandwidth_threshold` 1 Mbps [Proposed]); nối vào `ServiceLocator` (`content`, `prefetch`). Chọn chất lượng media thấp: `media_quality()` sẵn sàng, chưa có biến thể media để chọn (F065).
- [x] T038 [US4] `PrefetchScheduler` (chỉ mục gói có chữ ký + media công khai đã biết) — file: `src/game/scripts/networking/prefetch_scheduler.gd` — **xong 2026-11-28:** `PrefetchScheduler` (timer 15 phút chỉ trên Android dùng chung, `run()` đọc chỉ mục ký qua GET công khai). Mở: media công khai "đã biết" của phiên trước chưa tải trước vì thiết bị chưa lưu manifest nội dung (ghi trong Clarifications).
- [x] T039 [US4] Gói APK trường (APK kèm gói theo khối, delta theo hash; không installer Windows — ADR-0101) — file: `deploy/installer/school-kit/build-kit.sh` — **xong 2026-11-28 (ký thật chờ T019):** `deploy/installer/school-kit/build-kit.sh` + `scripts/godot/lib/school-kit.mjs` (zip thuần Node `lib/zip.mjs`, gói stored căn 4 byte, ký lại `zipalign -P 16` + `apksigner`, `.sig` khóa phát hành) + `lib/kits-list.mjs`; nối `publish-index.sh` (F134) vào `godot.yml` `publish-staging`/`publish-production` → F134 T033 xong.
- [ ] T050 [US4] ~~Trang chọn "Máy dùng chung"/"Máy cá nhân" + tham số im lặng trong script bộ cài Windows ghi `device.cfg` — file: `src/game/export/windows/installer.iss`~~ — **không làm** (ADR-0101, 2026-11-24: bỏ bản Windows); mã giữ để truy vết

**Checkpoint**: User Story 4 chạy và kiểm độc lập được.

## Phase 7: User Story 5 — Kho bảo mật giữ phiên trong bản phát hành (Priority: P1)

**Goal**: Adapter iOS Keychain, cấu hình backup Android, kiểm adapter theo nền tảng trong CI (ADR-0086).

**Independent Test**: Như spec.md User Story 5.

### Tests cho User Story 5

- [ ] T043 [P] [US5] Test F133-AC-23: `IosKeychainSecureStorage` round-trip `save`/`load`/`erase`, thuộc tính `AfterFirstUnlockThisDeviceOnly`, `kSecAttrSynchronizable = false` (chạy trên simulator trong runner macOS) (TC-AUTH-082) — file: `src/game/tests/integration/platform/test_ios_keychain_secure_storage.gd` — phải FAIL trước khi code — ⏳ **pending: simulator iOS trên runner macOS.**
- [x] T044 [P] [US5] Test F133-AC-24, F133-AC-25: `check-secure-storage.mjs` trên fixture artifact thiếu thư viện native / manifest Android không loại backup / entitlement iOS có Keychain Sharing / Web shell ghi localStorage → exit ≠ 0 (TC-AUTH-084) — file: `scripts/godot/tests/check-secure-storage.test.mjs` — **xong 2026-11-28** ở `scripts/godot/tests/check-secure-storage.test.mjs` 7/7 (viết trước: FAIL vì chưa có module): APK/AAB thiếu plugin Keystore, manifest thiếu `dataExtractionRules`/`fullBackupContent`, thiếu file luật; iOS có `keychain-access-groups`/iCloud hoặc thiếu adapter Keychain; Web có `localStorage`/`indexedDB` hoặc thiếu `MemoryOnly` → exit 1.
- [ ] T045 [US5] Kịch bản chứng nhận F133-AC-26 trên lab Android/iOS (TC-AUTH-082, TC-AUTH-088) — file: `docs/12-project/perf-reports/client-2026-12-01.md` — ⏳ **pending: lab Android/iOS.**

### Implementation cho User Story 5

- [ ] T046 [US5] Plugin iOS (Swift) `IosKeychainSecureStorage` + adapter GDScript, đăng ký trong `SecureStorageFactory` (F060) — file: `src/game/addons/eduworld_platform/ios/IosKeychainSecureStorage.swift` — ⏳ **pending: cần Xcode/Swift trên macOS** (khi chưa có, `check-secure-storage --platform=ios` chặn bản iOS — đúng ý F133-AC-25).
- [x] T047 [P] [US5] `data_extraction_rules.xml` + `fullBackupContent` loại thư mục phiên trong manifest Android release — file: `src/game/platform/android/res/xml/data_extraction_rules.xml` — **xong 2026-11-28** (đặt trong AAR của plugin F060 để luôn đi cùng Keystore: `addons/eduworld_platform/android/src/main/res/xml/data_extraction_rules.xml` + `backup_rules.xml`, manifest plugin khai `android:dataExtractionRules`/`android:fullBackupContent`; loại `secure/`, `cache/` khỏi cloud backup và device transfer; preset vẫn `allowBackup=false`, không permission mới). Test `scripts/godot/tests/android-backup-rules.test.mjs` 3/3; APK thật kiểm bởi `scan-artifact` trong CI (máy này không build được AAR — không có Gradle/JDK).
- [x] T048 [P] [US5] `check-secure-storage.mjs` và gắn vào job `godot / scan-artifact` cho mọi target — file: `scripts/godot/check-secure-storage.mjs` — **xong 2026-11-28:** `scripts/godot/check-secure-storage.mjs` (+ `lib/zip.mjs` đọc APK/AAB/IPA), gọi từ `scan-artifact.sh --platform=<…>` trong job `export` và `scan-artifact`; bản Web thật: `check-secure-storage: web OK`.

**Checkpoint**: User Story 5 chạy và kiểm độc lập được.

## Phase N: Polish & Cross-Cutting

- [x] T040 [P] GODOT_ARCHITECTURE §11: gỡ intent filter `oauth2/callback` và "redirect OIDC cùng tab" (ADR-0068, D16), thêm `device.cfg`, update service; DEPLOYMENT §9: `content_manifest_url` → `world_manifest_url` theo `ClientConfig` — file: `docs/08-game-client/GODOT_ARCHITECTURE.md` — **xong 2026-11-28:** GODOT_ARCHITECTURE §4 (khóa cấu hình phát hành, `DeviceConfig`), §5 (sơ đồ + 426 → `UpdateRequired`), §9, §11 (script phát hành, kiểm kho bảo mật, cập nhật client, nội dung vùng/phòng máy, `device.cfg`); intent filter/redirect OIDC đã gỡ từ trước (D16); `content_manifest_url` không còn trong DEPLOYMENT §9. DEPLOYMENT §9 (phát hành Web/gói vùng/manifest/gói APK trường), §10.2 (`godot.yml`), §13 (secret phát hành).
- [x] T041 [P] PERFORMANCE_BUDGET §10.1/§15: vị trí `device.cfg`, chỉ mục gói ký; CHANGELOG mục F133 — file: `CHANGELOG.md` — **xong 2026-11-28:** PERFORMANCE_BUDGET §10.1 (vị trí và khóa `device.cfg`, chỉ mục gói ký, ngưỡng dữ liệu thấp), §15 (bước CI dung lượng); CONFIG_DEFAULTS `low_data_bandwidth_threshold` [Proposed]; CHANGELOG mục F133.
- [ ] T042 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/133-client-build-release/quickstart.md` — ⏳ **một phần 2026-11-28:** bằng chứng local ở [quickstart §5](quickstart.md#5-bằng-chứng-local-2026-11-28); còn các bước cần tag/CI thật, ký, store, CDN, lab, 40 client.

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → US3 (cần artifact ký từ US2) → US4 → Polish; US5 sau US1 (cần F060 US5 `SecureStorage`), T045 cùng đợt chứng nhận US3.
- T004 trước T037 (telemetry); T005 trước T013, T023; T013 trước T038, T039.
- F062, F061, F014 Done trước US3; F080 ingest `content_download` trước khi đánh giá F133-AC-21 end-to-end.

## Definition of Done

- [ ] Mọi tiêu chí F133-AC-* có test pass (ghi mã test/TC) — 2026-11-28: phần local xong (xem quickstart §5: GdUnit4 1476/0 fail, node 61/61); AC cần tag/ký/store/CDN/lab còn pending
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
