# Quickstart: F133 — Build và phát hành client Web, Android, iOS

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F062 (`QualityManager`, benchmark), F061 (đăng nhập form, 426), F014 (`client-config`), F130 (staging, `ew-game-bundles`, CDN origin), F131 (alert crash).
- Local: `make godot-export-check` (export PCK release preset Android, không ký), `make godot-test`; backend `make up up-app seed`.
- Tài khoản: `hs.an.3a` (local), học sinh QA `hs.qa.probe` (staging — smoke), `ad.truong.a` (phải bị `GAME_ACCESS_STUDENT_ONLY` khi thử đăng nhập game). Thiết bị lab theo [PERFORMANCE_BUDGET §2.1](../../docs/08-game-client/PERFORMANCE_BUDGET.md).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Tag `client/v0.9.0-rc1` | 3 artifact (Web, Android, iOS — không Windows); `Config.client_version = 0.9.0`, build = `github.run_number` | F133-AC-01 |
| 2 | Xem `src/game/config/production.json` trong artifact; thử `user://override.json` trên bản production | URL lấy từ CI; override bị bỏ qua; `export_presets.cfg` không có mật khẩu | F133-AC-02 |
| 3 | `scan-artifact.sh` trên artifact; nhánh thử nhúng chuỗi `/protocol/openid-connect/token` | Lần đầu sạch; nhánh thử đỏ | F133-AC-03 |
| 4 | Thêm asset 600 MB vào nhánh thử | `godot / size-check` đỏ; báo cáo dung lượng đính release | F133-AC-04 |
| 5 | `build-bundles.sh`; tải lại cùng gói | Chỉ mục có chữ ký; object theo hash không bị ghi đè | F133-AC-05 |
| 6 | Job `godot / smoke` trên staging | Boot → login `hs.qa.probe` → session → Hub | F133-AC-06 |
| 7 | Chạy `godot / publish-production` (environment `production`) bằng tài khoản không thuộc Release Manager | Bị từ chối; log không có khóa | F133-AC-07 |
| 8 | Phát hành rc lên Play internal, TestFlight, `play-staging` | Cài từ từng kênh, vào Hub; APK MDM có sẵn | F133-AC-08 |
| 9 | Mở `play-staging`; rollback về bản trước | Header Brotli + immutable; `index.html` đổi cuối; rollback tức thì | F133-AC-09 |
| 10 | ~~Windows: phát hành bản mới; lần hai sửa 1 byte installer~~ — không làm (ADR-0101) | — | F133-AC-10 |
| 11 | Giả lập `ClientCrashRateHigh` cho rc | Runbook halt rollout; Web giữ bản trước | F133-AC-11 |
| 12 | Chạy client N-1 với staging mới; đặt `EW_GAME_MIN_CLIENT_VERSION` cao hơn client | N-1 vào Hub; khi nâng min → màn "Cần cập nhật", không crash | F133-AC-12 |
| 13 | Bộ TC-PERF client trên lab (PC phòng máy chạy bản Web trong Chrome/Edge); báo cáo `perf-reports/client-<ngày>.md` | AC-PERF-001…011 đạt | F133-AC-13 |
| 14 | Benchmark headless bản release | Không zone vượt `perf_budget_tolerance_ci` | F133-AC-14 |
| 15 | Đăng nhập `hs.an.3a` trên 3 nền tảng (bàn phím ảo trên Android/iOS, Web trên PC phòng máy); thoát, mở lại | Form trong game, không mở trình duyệt; pinning bật Android/iOS; Android/iOS mở lại vào Hub không form, Web hiện form | F133-AC-15 |
| 16 | Mở Web trên Firefox và trên Chrome tắt WebGL 2 | Trang thông báo thân thiện, không tải engine | F133-AC-16 |
| 17 | Build gói APK trường; quét nội dung | Có gói vùng theo khối, không secret/dữ liệu tenant/câu hỏi đề | F133-AC-17 |
| 18 | Đặt `lan_cache_url` tới proxy LAN; sửa 1 gói trên proxy; đặt URL `ftp://x` | Gói sửa bị từ chối → tải CDN; URL sai bị bỏ qua có cảnh báo; không request API qua LAN | F133-AC-18 |
| 19 | Máy `shared_device: true` trong `offhours_prefetch_window` | Tải gói mới không đăng nhập, không gọi API cần token | F133-AC-19 |
| 20 | Bật `low_data_mode` | Không tải trước vùng kế tiếp | F133-AC-20 |
| 21 | Xem telemetry sau khi tải gói | Event `content_download` có `source`, `bytes`, không PII | F133-AC-21 |
| 22 | Phòng giả lập 40 client trên `classroom_reference_bandwidth` | Cả lớp vào Hub ≤ 2 phút | F133-AC-22 |
| 23 | iPad lab: đăng nhập, thoát, mở lại; xem entitlement của IPA; khôi phục thiết bị từ bản sao lưu iCloud | Vào Hub không form; không có Keychain Sharing/`keychain-access-groups` lạ; sau khôi phục hiện form | F133-AC-23 |
| 24 | Android: `aapt dump xmltree` manifest; `adb shell bmgr backupnow` rồi kiểm bản sao lưu; nhánh thử bỏ `dataExtractionRules` | Có quy tắc loại trừ; bản sao lưu không có file phiên; nhánh thử bị `godot / scan-artifact` chặn | F133-AC-24 |
| 25 | `node scripts/godot/check-secure-storage.mjs build/`; nhánh thử xóa plugin Keystore khỏi export Android / thêm `localStorage.setItem` cho khóa phiên ở Web shell | Lần đầu xanh; nhánh thử đỏ | F133-AC-25 |
| 26 | Chứng nhận RC trên Android/iOS: đăng nhập → thoát → mở lại; Đăng xuất → mở lại; chép dữ liệu app sang thiết bị khác | Lần lượt: vào Hub không form / form / không tự vào được (TC-AUTH-082, TC-AUTH-088) | F133-AC-26 |
| 27 | ~~Cài bản Windows với lựa chọn mặc định, đăng nhập rồi thoát; cài lại chọn "Máy cá nhân"~~ — không làm (ADR-0101) | — | F133-AC-27 |

## 3. Lệnh test tự động

```bash
make godot-test                 # GdUnit4: config_release, manifest_verifier, update_service, content_downloader, prefetch_scheduler, …
make godot-release-test         # node --test scripts/godot/ deploy/installer/school-kit/tests/ + publish_index_test.sh
bash scripts/godot/export.sh web --godot=<godot>          # đóng dấu version/build, export, check, scan (Android: android --debug)
bash scripts/godot/scan-artifact.sh --platform=web src/game/build/web
node scripts/godot/check-build-size.mjs --web=src/game/build/web --report=size-report.md
node scripts/godot/check-secure-storage.mjs --platform=android src/game/build/android/EduWorld.apk
bash scripts/godot/smoke.sh --api=http://localhost:9080/api/v1 --client-version=0.1.0   # + EW_SMOKE_USERNAME/PASSWORD
```

## 4. Bằng chứng cần nộp

- Pipeline tag (3 export, scan, size, smoke), báo cáo dung lượng, link kênh phát hành, báo cáo lab `perf-reports/client-*.md`, kết quả phòng giả lập 40 client.

## 5. Bằng chứng local 2026-11-28

Máy Windows, Godot 4.7.2 (Compatibility), Node 20.19, backend compose local đang chạy; không có Android SDK/JDK, Xcode, khóa ký, CDN thật.

| # §2 | Đã kiểm local | Kết quả | Còn |
|---|---|---|---|
| 1 | `export.sh web` với `GITHUB_RUN_NUMBER=133`; test `export.test.mjs` (tag `client/v1.2.3`, APK + AAB, iOS ngoài macOS) | `index.pck` chứa `client_version 0.0.0-ci.133`, `build_number 133`; 6/6 | Tag thật trên CI; Android/iOS export thật |
| 2 | `gen-config.test.mjs` 7/7, `test_config_release.gd` 4/4, `export_presets.cfg` không mật khẩu | Override `user://` bị bỏ ở release | — |
| 3 | `scan-artifact.test.mjs` 12/12 + bản Web thật | Nhánh thử đỏ, bản thật sạch | Artifact Android/iOS thật |
| 4 | `check-build-size.test.mjs` 3/3 + bản Web thật | Web 15,13 MB ≤ 40 MB | APK/AAB/IPA thật |
| 5 | `build-bundles.test.mjs` 5/5, `sign-manifest.test.mjs` 4/4 | Upload lại = no-op (aws giả) | `ew-game-bundles` thật (F130) |
| 6 | `smoke.sh` với backend local | `getClientConfig OK`, dừng PENDING (exit 3) | Mật khẩu học sinh QA |
| 9 | `publish-web.test.mjs` 5/5 | `index.html` sau cùng, rollback chỉ đổi `index.html` | `curl` trên `play-staging` (T018) |
| 12 | `test_update_service.gd` 7/7 + ảnh màn `UpdateRequired` | Không crash, link đúng nền tảng | N-1 với staging (T024) |
| 17 | `build-kit.test.mjs` 6/6 + `publish_index_test.sh` 8/8 | Gói đúng khối, từ chối dữ liệu tenant/secret/câu hỏi | APK ký thật (T019) |
| 18–20 | `test_content_downloader.gd` 8/8, `test_prefetch_scheduler.gd` 6/6 | LAN lệch hash → CDN, URL LAN sai bị bỏ, không API qua LAN, ngoài giờ/dữ liệu thấp | Proxy LAN + thiết bị thật |
| 21 | `test_content_downloader.gd` (cả 3 nguồn), `test_content_download_telemetry.gd` | Chỉ `source` + `bytes` | Dashboard F080 |
| 24–25 | `check-secure-storage.test.mjs` 7/7, `android-backup-rules.test.mjs` 3/3, bản Web thật `web OK` | Nhánh thử đỏ | `aapt`/`bmgr` trên APK thật, iOS (T046) |

Bước 7, 8, 10, 11, 13–16, 22, 23, 26 cần tag/CI thật, store, lab hoặc 40 client — chưa chạy.

**Cổng chung (2026-11-28):** GdUnit4 toàn bộ 1476 test, 0 lỗi, 0 fail, 16 skip (integration cần backend/mật khẩu); `check_typed`, `lint_theme_overrides`, `lint_layers`, `lint_signals`, `lint_art`, `bake_art --check`, `brand_assets --check`, `extract_pot --check`, `pseudo_locale --check`, `gen_fixtures --check`, `check_no_admin_ui` OK; `scan_artifact_test.sh` với PCK release thật 22/22; node `scripts/godot/` + `school-kit/tests/` 61/61, `publish_index_test.sh` 8/8, `scripts/ci/` 64/64; `check-workflows` 0 lỗi, actionlint 1.7.7 0 lỗi, shellcheck 0; specs (check-features, check-cross-features, check-maxlength, validate-schemas, split-public, oasdiff) 0 lỗi; gitleaks 8.28 trên file đổi: không lộ.

