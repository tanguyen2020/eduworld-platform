# Quickstart: F133 — Build và phát hành client Windows, Android, iOS, Web

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F062 (`QualityManager`, benchmark), F061 (đăng nhập form, 426), F014 (`client-config`), F130 (staging, `ew-game-bundles`, CDN origin), F131 (alert crash).
- Local: `make godot-export-check` (export Windows/Web không ký), `make godot-test`; backend `make up up-app seed`.
- Tài khoản: `hs.an.3a` (local), học sinh QA `hs.qa.probe` (staging — smoke), `ad.truong.a` (phải bị `GAME_ACCESS_STUDENT_ONLY` khi thử đăng nhập game). Thiết bị lab theo [PERFORMANCE_BUDGET §2.1](../../docs/08-game-client/PERFORMANCE_BUDGET.md).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Tag `client/v0.9.0-rc1` | 4 artifact; `Config.client_version = 0.9.0`, build = `github.run_number` | F133-AC-01 |
| 2 | Xem `godot/config/production.json` trong artifact; thử `user://override.json` trên bản production | URL lấy từ CI; override bị bỏ qua; `export_presets.cfg` không có mật khẩu | F133-AC-02 |
| 3 | `scan-artifact.sh` trên artifact; nhánh thử nhúng chuỗi `/protocol/openid-connect/token` | Lần đầu sạch; nhánh thử đỏ | F133-AC-03 |
| 4 | Thêm asset 600 MB vào nhánh thử | `godot / size-check` đỏ; báo cáo dung lượng đính release | F133-AC-04 |
| 5 | `build-bundles.sh`; tải lại cùng gói | Chỉ mục có chữ ký; object theo hash không bị ghi đè | F133-AC-05 |
| 6 | Job `godot / smoke` trên staging | Boot → login `hs.qa.probe` → session → Hub | F133-AC-06 |
| 7 | Chạy `godot / publish-production` (environment `production`) bằng tài khoản không thuộc Release Manager | Bị từ chối; log không có khóa | F133-AC-07 |
| 8 | Phát hành rc lên Play internal, TestFlight, `play-staging`, installer staging | Cài từ từng kênh, vào Hub; APK MDM có sẵn | F133-AC-08 |
| 9 | Mở `play-staging`; rollback về bản trước | Header Brotli + immutable; `index.html` đổi cuối; rollback tức thì | F133-AC-09 |
| 10 | Windows: phát hành bản mới; lần hai sửa 1 byte installer | Bản 1 tự cập nhật; bản 2 bị từ chối, giữ bản cũ, có `client_error` | F133-AC-10 |
| 11 | Giả lập `ClientCrashRateHigh` cho rc | Runbook halt rollout; Web/installer giữ bản trước | F133-AC-11 |
| 12 | Chạy client N-1 với staging mới; đặt `EW_GAME_MIN_CLIENT_VERSION` cao hơn client | N-1 vào Hub; khi nâng min → màn "Cần cập nhật", không crash | F133-AC-12 |
| 13 | Bộ TC-PERF client trên lab; báo cáo `perf-reports/client-<ngày>.md` | AC-PERF-001…011 đạt | F133-AC-13 |
| 14 | Benchmark headless bản release | Không zone vượt `perf_budget_tolerance_ci` | F133-AC-14 |
| 15 | Đăng nhập `hs.an.3a` trên 4 nền tảng (bàn phím ảo trên Android/iOS); thoát, mở lại | Form trong game, không mở trình duyệt; pinning bật desktop/mobile; Windows/Android/iOS mở lại vào Hub không form, Web hiện form | F133-AC-15 |
| 16 | Mở Web trên Firefox và trên Chrome tắt WebGL 2 | Trang thông báo thân thiện, không tải engine | F133-AC-16 |
| 17 | Build bộ cài trường; quét nội dung | Có gói vùng theo khối, không secret/dữ liệu tenant/câu hỏi đề | F133-AC-17 |
| 18 | Đặt `lan_cache_url` tới proxy LAN; sửa 1 gói trên proxy; đặt URL `ftp://x` | Gói sửa bị từ chối → tải CDN; URL sai bị bỏ qua có cảnh báo; không request API qua LAN | F133-AC-18 |
| 19 | Máy `shared_device: true` trong `offhours_prefetch_window` | Tải gói mới không đăng nhập, không gọi API cần token | F133-AC-19 |
| 20 | Bật `low_data_mode` | Không tải trước vùng kế tiếp | F133-AC-20 |
| 21 | Xem telemetry sau khi tải gói | Event `content_download` có `source`, `bytes`, không PII | F133-AC-21 |
| 22 | Phòng giả lập 40 client trên `classroom_reference_bandwidth` | Cả lớp vào Hub ≤ 2 phút | F133-AC-22 |
| 23 | iPad lab: đăng nhập, thoát, mở lại; xem entitlement của IPA; khôi phục thiết bị từ bản sao lưu iCloud | Vào Hub không form; không có Keychain Sharing/`keychain-access-groups` lạ; sau khôi phục hiện form | F133-AC-23 |
| 24 | Android: `aapt dump xmltree` manifest; `adb shell bmgr backupnow` rồi kiểm bản sao lưu; nhánh thử bỏ `dataExtractionRules` | Có quy tắc loại trừ; bản sao lưu không có file phiên; nhánh thử bị `godot / scan-artifact` chặn | F133-AC-24 |
| 25 | `node scripts/godot/check-secure-storage.mjs build/`; nhánh thử xóa thư viện DPAPI khỏi export Windows / thêm `localStorage.setItem` cho khóa phiên ở Web shell | Lần đầu xanh; nhánh thử đỏ | F133-AC-25 |
| 26 | Chứng nhận RC trên Windows/Android/iOS: đăng nhập → thoát → mở lại; Đăng xuất → mở lại; chép dữ liệu app sang user Windows khác | Lần lượt: vào Hub không form / form / không tự vào được (TC-AUTH-082, TC-AUTH-088) | F133-AC-26 |
| 27 | Cài bản Windows với lựa chọn mặc định, đăng nhập rồi thoát; cài lại chọn "Máy cá nhân" | Mặc định: `shared_device: true`, không còn token sau thoát; "Máy cá nhân": tự vào lại qua màn xác nhận | F133-AC-27 |

## 3. Lệnh test tự động

```bash
make godot-test                 # GdUnit4: device_config, content_downloader, manifest_verifier, update_service, prefetch_scheduler
make godot-export-check
bash scripts/godot/scan-artifact.sh build/
node scripts/godot/check-build-size.mjs build/
node scripts/godot/check-secure-storage.mjs build/
```

## 4. Bằng chứng cần nộp

- Pipeline tag (4 export, scan, size, smoke), báo cáo dung lượng, link kênh phát hành, báo cáo lab `perf-reports/client-*.md`, kết quả phòng giả lập 40 client.
