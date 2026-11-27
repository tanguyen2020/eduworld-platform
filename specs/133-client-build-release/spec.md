# Feature Specification: F133 — Build và phát hành client Web, Android, iOS

**Feature Branch**: `133-client-build-release` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `src/game/` + hạ tầng CI (`.github/workflows/godot.yml`, `scripts/godot`, `deploy/installer/school-kit` (gói APK trường), CDN `play.<domain>`) · **Mốc**: M6 · **Phụ thuộc**: F062, F130
**Input**: Pipeline export, ký số, phát hành và chứng nhận hiệu năng client Godot trên 3 nền tảng Release 1.0 — Web, Android, iOS (ADR-0013, ADR-0101, ADR-0057), giới hạn dung lượng, tương thích phiên bản N-1, và ngân sách phòng máy (gói APK trường kèm gói nội dung, cache LAN, tải trước ngoài giờ; PC phòng máy chơi bản Web) — FR-GAME-009, NFR-COMP-001, NFR-COMP-002, NFR-COMP-003, NFR-COMP-004, NFR-PERF-006, NFR-PERF-019.

## Clarifications

### Session 2026-11-16

- Q: ADR-0079 đề xuất bản đầu chỉ Windows + Android — có thu hẹp phạm vi? → A: Không. ADR-0079 bị Rejected (2026-11-17); giữ 4 nền tảng theo ADR-0013 (sau đó ADR-0101, 2026-11-24, bỏ Windows: còn Web, Android, iOS).
- Q: GODOT_ARCHITECTURE §11 còn intent filter `oauth2/callback` (Android) và "redirect OIDC cùng tab" (Web)? → A: Đã gỡ theo ADR-0068 (CODE_PLAN D16): build không có intent filter OAuth, không redirect OIDC; task tài liệu sửa §11.
- Q: Windows phân phối qua Microsoft Store hay installer? → A: ~~Installer ký Authenticode tải qua CDN + kênh phần mềm quản lý phòng máy; Microsoft Store không làm trong Release 1.0~~ — không còn hiệu lực: ADR-0101 (2026-11-24) bỏ hẳn bản Windows.
- Q: Tải trước ngoài giờ không có đăng nhập học sinh thì biết tải gì? → A: Từ chỉ mục gói công khai có chữ ký trên CDN theo `client_version` (vùng thế giới, không theo tenant) + media công khai đã có trong manifest nội dung tenant được cache ở phiên trước (URL theo hash). Không gọi API cần token [Confirmed].
- Q: Firefox/Safari? → A: Không hỗ trợ trong Release 1.0 (NFR-COMP-004; OQ-04 owner chốt 2026-11-20: Chrome/Edge bắt buộc, Firefox/Safari ngoài ma trận); trình duyệt không có WebGL 2 nhận trang thông báo.

### Session 2026-11-16 (ADR-0086)

- Q: ADR-0086 lưu refresh token trong kho bảo mật của hệ điều hành — build/phát hành cần gì? → A: (1) adapter `SecureStorage` iOS Keychain (`kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly`, không đồng bộ iCloud) làm ở F133 cùng job iOS của `godot.yml` (runner macOS) (Android Keystore, `MemoryOnly` đã có ở F060); (2) Android: Keystore không cần permission runtime; file phiên nằm ngoài Auto Backup/device-transfer (`android:allowBackup="false"` hoặc `dataExtractionRules` loại thư mục phiên); (3) iOS: không bật iCloud Keychain sharing, không thêm `keychain-access-groups` ngoài mặc định của app; (4) ~~Windows: DPAPI scope CurrentUser~~ — bỏ theo ADR-0101; (5) Web: chỉ `MemoryOnly`, không localStorage/IndexedDB; (6) CI kiểm mỗi target có đúng adapter, thiếu thì fail. macOS/Linux client không làm trong Release 1.0.

### Session 2026-11-17

- Q: Bộ cài Windows chọn "Máy dùng chung"/"Máy cá nhân" (ADR-0078) chưa có tiêu chí — owner nào? → A: (Không còn hiệu lực từ 2026-11-24, ADR-0101 — xem F133-AC-27.) Bổ sung theo [CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý) (lát cắt M-P1); tham số theo tên CONFIG_DEFAULTS, mã lỗi theo [API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md).

### Session 2026-11-20 (owner chốt — OQ-04, pinning)

- Nền tảng và trình duyệt (OQ-04) → **Chốt:** OS tối thiểu Android 9, iOS 15 (khớp F133-AC-01; Windows 10 bỏ theo ADR-0101); Web hỗ trợ chính thức Chrome và Edge bản mới; Firefox/Safari không được hỗ trợ trong Release 1.0 (không kiểm thử) nhưng **không bị chặn**; thiếu WebGL 2 thì hiện trang thông báo. F133-AC-16 sửa theo (bỏ chặn trình duyệt không phải Chrome/Edge).
- Certificate pinning (GODOT_ARCHITECTURE §14) → **Chốt:** pin theo **CA bundle**: `TLSOptions.client(trusted_chain)` với `trusted_chain` chỉ gồm CA chính + một CA dự phòng phát hành chứng chỉ của tên miền API/realtime, đóng gói trong build (Android/iOS — ADR-0101; Web dùng TLS của trình duyệt); xoay chứng chỉ lá không cần cập nhật client. F133 đóng gói bundle theo môi trường và bật cờ `certificate_pinning` (T029); `ApiClient` (F061) và `RealtimeClient` (F092) dùng bundle — task ở hai feature đó.
- Nhà cung cấp CDN (OQ-10) → chọn khi dựng môi trường, cấu hình header độc lập nhà cung cấp (không phải câu hỏi mở).

### Session 2026-11-24 (ADR-0101 — bỏ bản Windows)

- Q: Owner "bỏ hẳn xuất ở windows" — phạm vi F133 đổi thế nào? → A: [Confirmed] Release 1.0 chỉ export, ký và phát hành **Web, Android, iOS** ([ADR-0101](../../docs/adr/ADR-0101-bo-windows-khoi-nen-tang-client-release-1-0.md)). Bỏ preset/export Windows, installer Inno Setup, ký Authenticode, tự cập nhật Windows (F133-AC-10), bộ cài Windows chọn máy dùng chung (F133-AC-27), adapter DPAPI. Mã AC giữ để truy vết, không tái sử dụng.
- Q: PC phòng máy của trường chơi bằng gì? → A: [Confirmed] Bản Web trong Chrome/Edge (ADR-0101 mục 3): dùng ngân sách, preset và mục tiêu khung hình của Web; Web chỉ giữ refresh token trong RAM nên mỗi lần mở lại phải đăng nhập (ADR-0086 mục 1). Chứng nhận device matrix đo PC phòng máy trên bản Web.
- Q: "Bộ cài trường" còn gì? → A: [Confirmed] Chỉ gói APK Android kèm gói vùng theo khối, phân phối qua MDM/USB (ADR-0101 mục 7).
- Q: PC phòng máy chạy Web không đọc được `device.cfg` (không có `lan_cache_url`, không tải trước ngoài giờ, không gói cài sẵn) — US4/F133-AC-22 (40 máy vào Hub ≤ 2 phút) đạt cho PC phòng máy bằng cách nào? → A: [TBD] Câu hỏi mở cho owner; tạm thời F133-AC-17…22 áp cho thiết bị Android dùng chung, PC phòng máy dựa vào cache HTTP của trình duyệt và CDN (chưa có tiêu chí riêng).

### Session 2026-11-25 (T002, T028 — chọn mặc định an toàn khi triển khai)

- Q: Bản Web ở origin khác API (`play.<domain>` so với `api.<domain>`), đăng nhập bị CORS chặn — phục vụ cùng origin hay mở CORS? → A: [Confirmed] **Mở CORS theo allowlist, chỉ cho route của game.** Lý do: DEPLOYMENT §9 đã đặt bản Web trên CDN theo phiên bản, còn API ở `api.<domain>` (`api_base_url` của `config/<env>.json`); game dùng Bearer trong RAM, không cookie. SECURITY_ARCHITECTURE chỉ đòi cùng origin cho route cookie của portal. Cách làm:
  - Route `cors: game` trong `deploy/apisix/routes/routes.yaml` (`/auth/game/*`, `/game/client-config`, `player`, `/realtime/v1`) nhận thêm origin bản Web từ `EW_CORS_GAME_ORIGINS` (local `http://localhost:8060`, staging `https://play.staging.eduworld.example`).
  - Route portal, nhất là `/auth/portal/*`, không bao giờ nhận origin này. Không `*`, không đổi `allow_credential`.
  - Thêm `X-Client-Version`, `X-Client-Platform` vào `allow_headers` (game gửi ở mọi request).
  - `realtime` của compose đặt `EW_CORS_ALLOWED_ORIGINS=http://localhost:5173,http://localhost:8060`.
  - CORS của CDN media/gói cho origin bản Web thuộc T022 (cần F130).
  - Chi tiết: [DEPLOYMENT §9](../../docs/11-operations/DEPLOYMENT.md#9-cdn-và-phân-phối-godot-client).
- Q: T026 ghi "Firefox → trang thông báo", trái F133-AC-16 sau ngày 2026-11-20 — theo bên nào? → A: [Confirmed] Theo F133-AC-16 và OQ-04: chỉ thiếu WebGL 2 (hoặc thiếu WebAssembly/fetch/secure context) mới hiện trang thông báo và không tải engine. Trình duyệt khác Chrome/Edge có WebGL 2 (Firefox, Safari, Chrome iOS, Opera) chỉ có banner "không hỗ trợ chính thức" đóng được. T026 sửa theo.
- Q: Heap Web "khởi tạo 512 MB, tối đa 1 GB" (PERFORMANCE_BUDGET §14) đặt ở đâu khi template Godot 4.7.2 cố định bộ nhớ lúc build (wasm tự export memory, tăng tới 2 GiB)? → A: [Confirmed] Shell đặt chính sách trên `WebAssembly.Memory.prototype.grow`:
  - lần tăng đầu dành sẵn 512 MiB (thiết bị không đủ thì tăng đúng mức xin);
  - không bao giờ vượt 1 GiB (vượt thì ném `RangeError`, engine báo hết bộ nhớ, shell hiện thông báo lỗi).
  Không build template riêng.
- Q: "Âm thanh bật sau tương tác đầu" làm thế nào? → A: [Confirmed] Shell tải engine + PCK ngay. Engine chỉ `start()` (lúc đó mới tạo `AudioContext`) khi học sinh bấm "Chơi", nên không có âm thanh hay `AudioContext` nào trước cử chỉ người dùng. PWA/service worker tắt (`progressive_web_app/enabled=false`); cache theo hash CDN (§9, T022).
- Q: Chuỗi và giao diện của shell (chạy trước engine, chưa có file dịch hay Theme)? → A: [Confirmed] Chuỗi vi (mặc định)/en nằm trong shell, chọn theo `navigator.languages`. Màu, cỡ chữ, khoảng cách lấy từ `src/game/web/shell_tokens.css`, sinh bởi `make design-tokens` từ `design/game-tokens` (output game, không token admin — R32). File này được `make godot-export-web` chép cạnh `index.html`. Shell không dùng localStorage/sessionStorage/IndexedDB/cookie (ADR-0086).
  - Bổ sung 2026-11-26 ([F113](../113-student-ux-pass-5/spec.md), STUDENT_GAME_UX §2): khi cửa sổ dọc ở trạng thái `checking`/`loading`/`ready`/`starting`, shell hiện lời nhắc "Xoay ngang điện thoại để chơi nhé!" (Mây, icon xoay, vi/en, biến `shell_tokens.css`; không che trang thông báo/lỗi) và "Chơi" trên thiết bị cảm ứng thử toàn màn hình + `screen.orientation.lock('landscape')` (bỏ qua im lặng khi bị từ chối). Mọi tiêu chí F133-AC-16 giữ nguyên.
- Q: Preset iOS — bản tối thiểu, ký, icon? → A: [Confirmed] Thông số preset:
  - arm64 (template 4.7.2 chỉ có arm64), `min_ios_version = 15.0`, bundle `vn.eduworld.game` (khớp Android), xuất App Store (release)/Development (debug);
  - team id, identity, provisioning profile để trống: job ký trên runner macOS ghi vào (T019);
  - không entitlement thêm, không Game Center, không mở thư mục app cho ứng dụng Tệp/iTunes.
  Trên Windows Godot 4.7.2 sinh được project Xcode khi có team id, nhưng không build/ký được IPA (cần Xcode). Project chưa có icon ứng dụng (`application/config/icon`), nên export iOS báo "Invalid icon". **Không** dùng icon giả: icon thương hiệu 1024×1024 là điều kiện trước T019 (việc của thiết kế).
- Q: Chạy bản Web với stack local khi preset phát hành chỉ đóng gói `production.json`? → A: [Confirmed] Không thêm preset debug/local và không sửa `local.json` hay preset rồi commit. Bản Web chạy với stack local dùng `config` sinh bởi `gen-config.mjs` (T003, chưa làm). `make godot-web-preview` phục vụ ở `http://localhost:8060`, origin đã có trong CORS local.

### Session 2026-11-28 (telemetry `content_download` phía client — T033, một phần T037)

- Q: Contract `TelemetryEvent` trên main (F131/F080, `df48cb4c`) chỉ có `source` + `bytes`, không có `bundle_id` như F133-AC-21 và T004 ghi; client gửi gì? → A: [Proposed] Chỉ `source` (`preinstalled`/`lan_cache`/`cdn`) và `bytes` (0–10 GiB) — `TelemetryEvent` có `additionalProperties: false` nên gửi `bundle_id` sẽ bị `422`, và metric `eduworld_client_content_download_bytes_total{source}` không dùng nó. Thêm `bundle_id` cần delta contract mới (không làm ở đây); đến lúc đó F133-AC-21 được kiểm với `source` + `bytes`.
- Q: Gửi một event cho mỗi lần tải? → A: [Proposed] `TelemetryService.queue_content_download(source, bytes)` cộng dồn byte cùng `source` vào event còn chờ lô kế tiếp (tối đa một event mỗi nguồn mỗi lần flush, chạm 10 GiB thì mở event mới; event đã nằm trong lô đang gửi không bao giờ bị sửa). Tổng counter phía server không đổi; hàng đợi RAM không bị đầy vì nhiều lần tải nhỏ. Không lọc bởi cờ telemetry sản phẩm (metric vận hành), giữ trong hàng đợi khi offline như mọi event.
- Q: Đường tải nào gọi nó khi `ContentDownloader` (T037) chưa có? → A: [Proposed] Hiện chỉ `VideoFileCache` (video bài học, media công khai từ CDN) tải nội dung lớn: mỗi lần tải xong (không tính khi đã có trong cache) gửi `source = cdn`, `bytes` = kích thước file. `ContentDownloader` của T037 phải gọi cùng hàm với nguồn thật (gói cài sẵn/LAN/CDN). SVG công thức, nhãn dán/GIF chat (vài KB) chưa tính — câu hỏi mở cho T037.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Export 3 nền tảng từ tag, có cổng dung lượng và quét bảo mật (Priority: P1)

Release Manager tạo tag `client/v1.0.0`; CI export Web, Android, iOS cùng gói nội dung vùng, đo dung lượng, quét artifact, chạy smoke rồi mới cho phát hành.

**Why this priority**: Không có build tái lập thì không có gì để chứng nhận hay phát hành.

**Independent Test**: Chạy workflow `godot.yml` trên tag thử; 3 artifact + báo cáo dung lượng + kết quả quét + smoke xanh; cố ý thêm asset lớn → job dung lượng đỏ.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F133-AC-01 | KHI có tag `client/vX.Y.Z`, CI PHẢI export Android 9+ arm64-v8a (AAB + APK), iOS 15+ arm64 (runner macOS của GitHub Actions) và Web (runner Linux) trong image Godot ghim phiên bản + export templates; `client_version` = `X.Y.Z` và build number = `github.run_number` PHẢI được nhúng vào `Config`; PHẢI KHÔNG có target Windows/macOS/Linux | NFR-COMP-001 · NFR-COMP-003 · [GODOT_ARCHITECTURE §11](../../docs/08-game-client/GODOT_ARCHITECTURE.md) · ADR-0013 · ADR-0101 · ADR-0088 | — |
| F133-AC-02 | Cấu hình môi trường (`api_base_url`, `realtime_url`, `cdn_base_url`, khóa công khai kiểm chữ ký manifest) PHẢI sinh từ biến/secret của GitHub Environment vào `res://config/<env>.json`; build production PHẢI KHÔNG đọc override `user://`; `export_presets.cfg` PHẢI KHÔNG chứa mật khẩu keystore/chứng chỉ | AGENTS R3, R4 · [GODOT_ARCHITECTURE §4, §11](../../docs/08-game-client/GODOT_ARCHITECTURE.md) | — |
| F133-AC-03 | KHI export xong, job `godot / scan-artifact` PHẢI giải nén artifact và fail nếu tìm thấy secret client `eduworld-game-backend`, URL `/protocol/openid-connect/token`, intent filter/redirect OAuth, khóa riêng ký, hay câu hỏi đề/answer key trong PCK | ADR-0068 · AGENTS R6, R31 | TC-AUTH-010 |
| F133-AC-04 | NẾU kích thước bản cài Android/iOS, tải đầu Web hoặc một gói vùng vượt `build_size`, THÌ job `godot / size-check` PHẢI fail; mỗi release PHẢI đính báo cáo dung lượng theo nền tảng và theo gói | NFR-PERF-006 · [PERFORMANCE_BUDGET §11](../../docs/08-game-client/PERFORMANCE_BUDGET.md) · [DEPLOYMENT §9](../../docs/11-operations/DEPLOYMENT.md) | — |
| F133-AC-05 | CI PHẢI build gói PCK theo vùng + chỉ mục gói (`bundle_id`, `version`, `url`, `sha256`, `size`) ký RSA-SHA256, tải lên `ew-game-bundles` theo khóa hash, không ghi đè object đã có; nội dung ngoài lõi PHẢI KHÔNG đóng vào bản cài | NFR-PERF-006 · ADR-0044 · [DEPLOYMENT §9](../../docs/11-operations/DEPLOYMENT.md) | TC-CNT-009 |
| F133-AC-06 | KHI artifact sẵn sàng, smoke headless PHẢI chạy boot → `getClientConfig` → đăng nhập học sinh QA staging qua form (`gameLogin`) → `createGameSession` → vào Hub; NẾU thất bại, THÌ job phát hành PHẢI bị chặn | [GODOT_ARCHITECTURE §11](../../docs/08-game-client/GODOT_ARCHITECTURE.md) · ADR-0083 | TC-AUTH-001 |

---

### User Story 2 — Ký số và phát hành theo kênh (Priority: P1)

Release Manager đẩy bản đã ký lên Google Play, App Store và CDN Web; dừng rollout khi crash tăng; client cũ được nhắc cập nhật đúng chính sách.

**Why this priority**: Exit criteria M6 yêu cầu ứng dụng ký số và phát hành trên mọi kênh.

**Independent Test**: Phát hành `client/v0.9.0-rc1` lên kênh nội bộ (Play internal, TestFlight, `play-staging`); cài từ từng kênh và vào Hub.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F133-AC-07 | Job ký PHẢI chỉ chạy trên tag `client/v*` được bảo vệ bằng tag ruleset, nhận keystore Android/chứng chỉ Apple từ secret chỉ cấp cho GitHub Environment `staging`/`production` và PHẢI KHÔNG lưu khóa vào artifact hay log; NẾU người duyệt không thuộc nhóm Release Manager (required reviewers của environment `production`), THÌ job phát hành production PHẢI bị từ chối | FR-OPS-004 · [DEPLOYMENT §13](../../docs/11-operations/DEPLOYMENT.md) · AGENTS R4 · ADR-0088 | — |
| F133-AC-08 | KHI phát hành, CI PHẢI đẩy AAB lên Google Play track `internal`, IPA lên TestFlight, Web lên `play-staging`; chuyển `closed`/`production`/App Store/`play.<domain>` PHẢI là bước thủ công có approval; APK ký PHẢI có sẵn cho MDM trường | [DEPLOYMENT §9, §11.2](../../docs/11-operations/DEPLOYMENT.md) | — |
| F133-AC-09 | Web PHẢI được tải lên thư mục theo phiên bản trên CDN, `.wasm`/`.pck` nén Brotli với `Cache-Control: immutable` theo hash, `index.html` (cache ngắn) đổi cuối cùng; rollback Web PHẢI là trỏ `index.html` về phiên bản trước | NFR-COMP-004 · [DEPLOYMENT §9, §11.2](../../docs/11-operations/DEPLOYMENT.md) | — |
| F133-AC-10 | ~~KHI client Windows khởi động và manifest phát hành có bản mới, client PHẢI tải installer, kiểm chữ ký manifest, SHA-256 và Authenticode trước khi chạy; NẾU bất kỳ kiểm tra nào lỗi, THÌ client PHẢI giữ bản hiện tại và ghi `client_error` telemetry~~ — **không làm** (ADR-0101, 2026-11-24: bỏ bản Windows); mã giữ để truy vết, không tái sử dụng | [GODOT_ARCHITECTURE §11](../../docs/08-game-client/GODOT_ARCHITECTURE.md) · NFR-SEC-005 | — |
| F133-AC-11 | NẾU alert `ClientCrashRateHigh` bắn cho phiên bản đang rollout, THÌ runbook PHẢI dừng rollout store (halt staged rollout) và giữ Web ở bản trước | [MONITORING §5](../../docs/11-operations/MONITORING.md) | — |
| F133-AC-12 | Trước mỗi release backend, CI PHẢI chạy smoke của bản client N-1 đang phát hành với staging mới; `min_client_version` PHẢI chỉ được nâng khi bản cũ hơn đã hết thời hạn hỗ trợ (≥ 1 học kỳ); KHI server trả `426 CLIENT_VERSION_UNSUPPORTED` hoặc `getClientConfig.update_required = true`, client PHẢI hiện màn "Cần cập nhật" có link kênh cài đúng nền tảng, không crash | NFR-COMP-002 · [DEPLOYMENT §9](../../docs/11-operations/DEPLOYMENT.md) | TC-AUTH-066 |

---

### User Story 3 — Chứng nhận trên device matrix 3 nền tảng (Priority: P1)

QA chạy bản release candidate trên thiết bị lab mọi hạng; báo cáo đạt AC-PERF-001…011, đăng nhập form chạy trên cả 3 nền tảng; PC phòng máy được đo trên bản Web (ADR-0101).

**Why this priority**: FR-GAME-009, NFR-COMP-001 là yêu cầu bắt buộc (ADR-0057); exit criteria M6.

**Independent Test**: Chạy bộ TC-PERF client trên lab §2.1; báo cáo `perf-reports/client-<ngày>.md` bảng thiết bị × cảnh × chỉ số.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F133-AC-13 | KHI có release candidate, QA PHẢI chạy bộ kiểm hiệu năng client trên toàn bộ thiết bị hạng Tầm trung và ít nhất 1 thiết bị Tối thiểu mỗi nền tảng, PC phòng máy đo trên bản Web trong Chrome/Edge ([TEST_STRATEGY §4.1](../../docs/12-project/TEST_STRATEGY.md)); mọi AC-PERF-001…011 PHẢI đạt; NẾU không đạt, THÌ release PHẢI bị chặn hoặc có phương án giảm chất lượng được duyệt (không loại thiết bị tầm trung) | FR-GAME-009 · NFR-COMP-001 · ADR-0057 · ADR-0101 | TC-PERF-009 · TC-PERF-019 · TC-PERF-020 · TC-PERF-021 · TC-PERF-022 · TC-PERF-023 · TC-PERF-024 |
| F133-AC-14 | Bản build release của mỗi nền tảng PHẢI qua benchmark headless từng zone; NẾU vượt ngân sách quá `perf_budget_tolerance_ci`, THÌ CI PHẢI chặn | FR-GAME-009 · [PERFORMANCE_BUDGET §15](../../docs/08-game-client/PERFORMANCE_BUDGET.md) | TC-PERF-018 |
| F133-AC-15 | Trên cả 3 nền tảng, học sinh PHẢI đăng nhập bằng form trong game (bàn phím ảo mobile, bàn phím vật lý trên Web), không mở trình duyệt/WebView; trên Android/iOS, mở lại game khi phiên còn hạn PHẢI tự vào Hub không hiện form, trên Web PHẢI luôn hiện form; Ở NƠI nền tảng là Android/iOS, client PHẢI bật `certificate_pinning`; Web dựa vào HSTS | ADR-0068 · ADR-0086 · ADR-0101 | TC-AUTH-001 · TC-AUTH-082 · TC-AUTH-084 |
| F133-AC-16 | Bản Web PHẢI dùng renderer Compatibility (WebGL 2), export single-threaded (không cần COOP/COEP), preset tối đa Trung bình, heap theo [PERFORMANCE_BUDGET §14](../../docs/08-game-client/PERFORMANCE_BUDGET.md), âm thanh bật sau tương tác đầu; NẾU trình duyệt không có WebGL 2, THÌ trang PHẢI hiện thông báo thân thiện thay vì tải engine; trình duyệt khác Chrome/Edge có WebGL 2 PHẢI không bị chặn (không hỗ trợ chính thức — OQ-04) | NFR-COMP-004 · ADR-0057 | TC-SIM-005 |

---

### User Story 4 — Phòng máy: gói APK trường kèm nội dung, cache LAN, tải trước ngoài giờ (Priority: P2)

School Admin cài gói APK trường (kèm gói vùng theo khối) lên tablet Android qua MDM/USB, có thể đặt cache LAN; cả lớp 40 máy vào Hub đầu tiết mà không nghẽn đường truyền. PC phòng máy chơi bản Web (ADR-0101) — cách đạt ngân sách phòng máy trên Web là câu hỏi mở (Clarifications 2026-11-24).

**Why this priority**: NFR-PERF-019; quan trọng cho trường nhưng không chặn phát hành cá nhân.

**Independent Test**: Phòng giả lập 40 client trên đường truyền `classroom_reference_bandwidth` với cache LAN; đo thời gian cả lớp vào Hub.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F133-AC-17 | CI PHẢI build gói APK trường kèm gói vùng công khai theo khối, cập nhật delta theo hash; gói PHẢI KHÔNG chứa dữ liệu tenant, secret, câu hỏi đề hay media private, và nằm trong `build_size` | NFR-PERF-019 · [PERFORMANCE_BUDGET §10.1](../../docs/08-game-client/PERFORMANCE_BUDGET.md) · AGENTS R31 · ADR-0101 | — |
| F133-AC-18 | Ở NƠI `device.cfg` có `lan_cache_url`, client PHẢI tải PCK/media công khai qua LAN trước, kiểm SHA-256 mọi gói và về CDN khi lỗi/lệch hash; PHẢI KHÔNG gửi request API, câu hỏi đề hay media private qua LAN; NẾU `lan_cache_url` không phải URL `http(s)` hợp lệ hoặc dài hơn 2048 ký tự, THÌ client PHẢI bỏ qua và ghi cảnh báo | NFR-PERF-019 · ADR-0044 · [DEPLOYMENT §9](../../docs/11-operations/DEPLOYMENT.md) | — |
| F133-AC-19 | TRONG KHI thiết bị có `shared_device: true` và nằm trong `offhours_prefetch_window`, client PHẢI tải trước gói vùng từ chỉ mục gói có chữ ký và media công khai đã biết, không đăng nhập học sinh, không gọi API cần token; client PHẢI dùng gói cài sẵn khi có thay vì tải lại | NFR-PERF-019 · [PERFORMANCE_BUDGET §10.1](../../docs/08-game-client/PERFORMANCE_BUDGET.md) | — |
| F133-AC-20 | Ở NƠI chế độ dữ liệu thấp bật (trường bật hoặc đo băng thông thấp), client PHẢI tắt tải trước vùng kế tiếp và giảm chất lượng media | NFR-PERF-019 · [PERFORMANCE_BUDGET §10.1](../../docs/08-game-client/PERFORMANCE_BUDGET.md) | — |
| F133-AC-21 | KHI tải gói/media, client PHẢI gửi telemetry `content_download` (`source` = `preinstalled`/`lan_cache`/`cdn`, `bytes`, `bundle_id`) qua `postTelemetry`, không PII (`bundle_id` chưa có trong contract — Clarifications 2026-11-28) | NFR-PERF-019 · NFR-OBS-003 · [MONITORING §3](../../docs/11-operations/MONITORING.md) | TC-ANL-003 |
| F133-AC-22 | KHI mô phỏng 40 client trên `classroom_reference_bandwidth` có gói cài sẵn hoặc cache LAN, cả lớp PHẢI vào Hub ≤ 2 phút (AC-PERF-012) | NFR-PERF-019 · [PERFORMANCE_BUDGET §16](../../docs/08-game-client/PERFORMANCE_BUDGET.md) | — |
| F133-AC-27 | ~~KHI cài đặt trên Windows, bộ cài PHẢI có lựa chọn "Máy dùng chung" (mặc định, ghi `shared_device: true` vào `device.cfg`) và "Máy cá nhân" (ghi `shared_device: false`), kể cả tham số cài im lặng cho phòng máy; với "Máy dùng chung" đăng nhập PHẢI không ghi refresh token vào DPAPI và thoát game = đăng xuất~~ — **không làm** (ADR-0101, 2026-11-24: bỏ bản Windows; PC phòng máy chơi Web, không lưu refresh token); mã giữ để truy vết, không tái sử dụng | AC-AUTH-030 · ADR-0078 · ADR-0086 | TC-AUTH-090 |

---

### User Story 5 — Kho bảo mật giữ phiên trong bản phát hành (Priority: P1)

Học sinh cài bản chính thức trên điện thoại/tablet Android hoặc iPhone/iPad: phiên đăng nhập được giữ an toàn trong kho bảo mật của hệ điều hành, không bị sao lưu ra đám mây hay chuyển sang máy khác; bản Web không lưu gì (ADR-0086).

**Why this priority**: ADR-0086; thiếu adapter hoặc cấu hình backup sai làm lộ refresh token hoặc làm tính năng tự vào lại không chạy trên bản phát hành.

**Independent Test**: Cài artifact release trên thiết bị lab mỗi nền tảng: đăng nhập, thoát, mở lại → vào Hub không form; kiểm manifest/entitlement; chạy `adb backup`/`bmgr` và kiểm backup không chứa file phiên; Web: tải lại tab → form.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F133-AC-23 | Bản iOS PHẢI có adapter `SecureStorage` iOS Keychain (`kSecClassGenericPassword`, `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly`, `kSecAttrSynchronizable = false`) hiện thực interface của F060, và PHẢI KHÔNG bật iCloud Keychain sharing hay thêm `keychain-access-groups` ngoài mặc định của app | ADR-0086 · NFR-COMP-003 · [GODOT_ARCHITECTURE §5.2](../../docs/08-game-client/GODOT_ARCHITECTURE.md#52-lưu-phiên-bằng-securestorage-adr-0086) | TC-AUTH-082 |
| F133-AC-24 | Manifest Android release PHẢI loại file phiên khỏi Auto Backup và device-to-device transfer (`android:allowBackup="false"` hoặc `dataExtractionRules`/`fullBackupContent` loại thư mục phiên), PHẢI đóng gói plugin Keystore của F060 và PHẢI KHÔNG khai báo permission runtime mới cho việc này; NẾU manifest thiếu cấu hình loại trừ, THÌ job `godot / scan-artifact` PHẢI fail | ADR-0086 · AGENTS R4 | TC-AUTH-084 |
| F133-AC-25 | KHI export mỗi target, CI PHẢI kiểm `SecureStorageFactory` chọn đúng adapter (Android → Keystore, iOS → Keychain, Web → `MemoryOnly`) và thư viện native tương ứng có trong artifact; NẾU thiếu, hoặc bản Web có mã ghi refresh token vào localStorage/IndexedDB, THÌ job export PHẢI fail | ADR-0086 · ADR-0101 · [GODOT_ARCHITECTURE §11](../../docs/08-game-client/GODOT_ARCHITECTURE.md) | TC-AUTH-084 |
| F133-AC-26 | KHI chứng nhận release candidate, QA PHẢI kiểm trên mỗi nền tảng cá nhân (Android, iOS): đăng nhập → thoát → mở lại vào Hub không form; Đăng xuất → mở lại hiện form; sao chép dữ liệu app sang máy khác → không tự vào được | ADR-0086 · ADR-0101 | TC-AUTH-082 · TC-AUTH-088 |

### Edge Cases

- NẾU chỉ mục gói hoặc manifest phát hành có chữ ký sai, THÌ client PHẢI bỏ qua toàn bộ chỉ mục đó, dùng gói đã cache và báo `client_error` (không tải gói không kiểm được).
- NẾU bộ nhớ cache vượt trần `build_size` (cache thiết bị), THÌ client xóa LRU vùng ít dùng, không xóa gói cài sẵn của gói APK trường.
- NẾU store từ chối bản (review App Store), THÌ backend vẫn giữ `min_client_version` cũ; các kênh khác phát hành độc lập.
- NẾU Godot export template đổi phiên bản, THÌ phải chạy lại toàn bộ chứng nhận US3 và F133-AC-26.
- NẾU người dùng khôi phục iPhone từ bản sao lưu hoặc chuyển sang máy mới, THÌ mục Keychain `ThisDeviceOnly` không đi theo → game hiện form (F133-AC-23).
- Authorization: phát hành chỉ Release Manager (F133-AC-07). Tenant isolation: artifact và gói APK trường không chứa dữ liệu tenant (F133-AC-17); client gửi `X-Tenant-Id` theo F061. Idempotency: upload theo khóa hash không ghi đè (F133-AC-05). Lỗi chính: `426 CLIENT_VERSION_UNSUPPORTED` (F133-AC-12). Giới hạn độ dài: `lan_cache_url` (F133-AC-18).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-GAME-009 | Chứng nhận preset/renderer/LOD… trên bản release mọi nền tảng (hiện thực `QualityManager` ở F062) | PERFORMANCE_BUDGET §4–§16 |
| NFR-COMP-001 | Device matrix, AC-PERF-001…011 cho release candidate | PERFORMANCE_BUDGET, TEST_STRATEGY §4.1 |
| NFR-COMP-002 | Smoke client N-1, chính sách nâng `min_client_version`, màn cập nhật | DEPLOYMENT §9, §11 |
| NFR-COMP-003 | Pipeline iOS (ký, TestFlight, App Store), thiết bị iOS 15+, adapter `SecureStorage` iOS Keychain (ADR-0086) | GODOT_ARCHITECTURE §11, §5.2 |
| NFR-COMP-004 | Web export Compatibility, single-thread, CDN, trình duyệt hỗ trợ | PERFORMANCE_BUDGET §14 |
| NFR-PERF-006 | Cổng dung lượng, gói vùng tải theo nhu cầu | PERFORMANCE_BUDGET §11 |
| NFR-PERF-019 | Gói APK trường, cache LAN, tải trước ngoài giờ, chế độ dữ liệu thấp, telemetry nguồn tải | PERFORMANCE_BUDGET §10.1 |

### Key Entities

- **Bản phát hành client**: nền tảng, `client_version`, build number, artifact, chữ ký, kênh.
- **Manifest phát hành**: phiên bản mới nhất theo nền tảng/kênh, URL, SHA-256, kích thước, chữ ký.
- **Chỉ mục gói**: danh sách gói vùng theo `client_version` (id, version, URL hash, SHA-256), có chữ ký.
- **Gói APK trường**: APK + gói vùng theo khối (không còn installer Windows — ADR-0101).
- **Cấu hình thiết bị (`device.cfg`)**: `shared_device`, `lan_cache_url`, chế độ dữ liệu thấp.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `build_size` | Cổng dung lượng bản cài, tải đầu Web, gói vùng, cache thiết bị |
| `perf_budget_tolerance_ci` | Ngưỡng chặn benchmark CI |
| `target_fps`, `renderer_default`, `quality_presets`, `load_time`, `battery_target`, `hub_sync_bandwidth` | Tiêu chí chứng nhận device matrix |
| `device_tier_reference` | Hạng thiết bị lab |
| `certificate_pinning` | Bật pinning theo nền tảng |
| `classroom_reference_bandwidth`, `offhours_prefetch_window`, `lan_cache_url` | Ngân sách phòng máy |
| `game_remember_username` | Tắt nhớ username trên `shared_device` |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| 3 nền tảng (Web, Android, iOS), một codebase GDScript | ADR-0013 · ADR-0014 · ADR-0101 |
| Không secret/endpoint token Keycloak trong build | ADR-0068 · AGENTS R4 |
| Refresh token chỉ trong kho bảo mật của hệ điều hành, không sao lưu ra đám mây; Web chỉ RAM | ADR-0086 |
| Không câu hỏi đề/answer key/media private trên CDN hay cache LAN | AGENTS R31 · ADR-0044 |
| Không hardcode URL môi trường | AGENTS R3 |
| Không loại thiết bị tầm trung để giữ tính năng | ADR-0057 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Từ một tag, cả 3 nền tảng có artifact đã ký, qua quét và smoke trong một lần chạy workflow.
- **SC-002**: 100% thiết bị hạng Tầm trung trong lab đạt AC-PERF-001…011 ở release candidate cuối.
- **SC-003**: 0 artifact phát hành vượt `build_size` hoặc chứa secret.
- **SC-004**: Bản N-1 vào Hub được với backend mới trong 100% lần release backend.
- **SC-005**: Phòng 40 máy có gói cài sẵn/cache LAN vào Hub ≤ 2 phút.
- **SC-006**: 100% artifact release có đúng adapter `SecureStorage` theo nền tảng; 0 file phiên trong bản sao lưu Android/iCloud.

## Ngoài phạm vi

- `QualityManager`, tự phát hiện preset, renderer fallback → F062; form đăng nhập, hàng chờ, tự vào lại/đăng xuất, xử lý 426 ở `ApiClient` → F061; interface `SecureStorage` + adapter Android/`MemoryOnly` → F060; `client-config`, `min_client_version` phía server → F014.
- Ingest telemetry và metric `eduworld_client_content_download_bytes_total` ở module game → F080.
- Trang tải gói cài trên portal cho School Admin → F134; Release 1.0 phát gói APK trường qua link CDN/MDM.
- Bản Windows (export, installer, Authenticode, tự cập nhật, DPAPI — ADR-0101), Microsoft Store, Firefox/Safari, macOS/Linux client: không làm.
- Stack CDN/object storage → F130; alert crash → F131.

## Assumptions

- Tài khoản Google Play Console, Apple Developer (tổ chức) được mua trước M6 (quy trình tổ chức).
- Nhà cung cấp CDN chọn khi dựng môi trường (OQ-10 đã trả lời 2026-11-20, không phải câu hỏi mở của feature); cấu hình header viết dạng độc lập nhà cung cấp.
- Thiết bị lab §2.1 có sẵn; device matrix chốt sau khảo sát OQ-04 [Confirmed].
- ADR-0079 Rejected; ADR-0101 (2026-11-24) bỏ Windows: còn 3 nền tảng Web, Android, iOS. ADR-0075 Accepted không đổi quy trình build (client dùng URL CDN; upload gói qua API S3 chuẩn).
- CI/CD là GitHub Actions (ADR-0088): `godot.yml` export Android/Web trên runner Linux, export + ký iOS trên runner macOS (tốn phút tính phí — chỉ chạy job iOS trên tag và nhánh `main`); action ghim SHA commit, Dependabot cập nhật action.
