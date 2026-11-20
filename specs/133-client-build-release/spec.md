# Feature Specification: F133 — Build và phát hành client Windows, Android, iOS, Web

**Feature Branch**: `133-client-build-release` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `src/game/` + hạ tầng CI (`.github/workflows/godot.yml`, `scripts/godot`, `deploy/installer`, CDN `play.<domain>`) · **Mốc**: M6 · **Phụ thuộc**: F062, F130
**Input**: Pipeline export, ký số, phát hành và chứng nhận hiệu năng client Godot trên 4 nền tảng Release 1.0 (ADR-0013, ADR-0057), giới hạn dung lượng, tương thích phiên bản N-1, và ngân sách phòng máy (bộ cài kèm gói nội dung, cache LAN, tải trước ngoài giờ) — FR-GAME-009, NFR-COMP-001, NFR-COMP-002, NFR-COMP-003, NFR-COMP-004, NFR-PERF-006, NFR-PERF-019.

## Clarifications

### Session 2026-11-16

- Q: ADR-0079 đề xuất bản đầu chỉ Windows + Android — có thu hẹp phạm vi? → A: Không. ADR-0079 bị Rejected (2026-11-17); giữ 4 nền tảng theo ADR-0013.
- Q: GODOT_ARCHITECTURE §11 còn intent filter `oauth2/callback` (Android) và "redirect OIDC cùng tab" (Web)? → A: Đã gỡ theo ADR-0068 (CODE_PLAN D16): build không có intent filter OAuth, không redirect OIDC; task tài liệu sửa §11.
- Q: Windows phân phối qua Microsoft Store hay installer? → A: Installer ký Authenticode tải qua CDN + kênh phần mềm quản lý phòng máy ([DEPLOYMENT §9](../../docs/11-operations/DEPLOYMENT.md)); Microsoft Store không làm trong Release 1.0 [Confirmed].
- Q: Tải trước ngoài giờ không có đăng nhập học sinh thì biết tải gì? → A: Từ chỉ mục gói công khai có chữ ký trên CDN theo `client_version` (vùng thế giới, không theo tenant) + media công khai đã có trong manifest nội dung tenant được cache ở phiên trước (URL theo hash). Không gọi API cần token [Confirmed].
- Q: Firefox/Safari? → A: Không hỗ trợ trong Release 1.0 ([TBD] NFR-COMP-004); trình duyệt không có WebGL 2 nhận trang thông báo.

### Session 2026-11-16 (ADR-0086)

- Q: ADR-0086 lưu refresh token trong kho bảo mật của hệ điều hành — build/phát hành cần gì? → A: (1) adapter `SecureStorage` iOS Keychain (`kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly`, không đồng bộ iCloud) làm ở F133 cùng job iOS của `godot.yml` (runner macOS) (Windows DPAPI, Android Keystore, `MemoryOnly` đã có ở F060); (2) Android: Keystore không cần permission runtime; file phiên nằm ngoài Auto Backup/device-transfer (`android:allowBackup="false"` hoặc `dataExtractionRules` loại thư mục phiên); (3) iOS: không bật iCloud Keychain sharing, không thêm `keychain-access-groups` ngoài mặc định của app; (4) Windows: DPAPI scope CurrentUser, installer không cần quyền admin cho phần này; (5) Web: chỉ `MemoryOnly`, không localStorage/IndexedDB; (6) CI kiểm mỗi target có đúng adapter, thiếu thì fail. macOS/Linux client không làm trong Release 1.0.

### Session 2026-11-17

- Q: Bộ cài Windows chọn "Máy dùng chung"/"Máy cá nhân" (ADR-0078) chưa có tiêu chí — owner nào? → A: Bổ sung theo [CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý) (lát cắt M-P1); tham số theo tên CONFIG_DEFAULTS, mã lỗi theo [API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Export 4 nền tảng từ tag, có cổng dung lượng và quét bảo mật (Priority: P1)

Release Manager tạo tag `client/v1.0.0`; CI export Windows, Android, iOS, Web cùng gói nội dung vùng, đo dung lượng, quét artifact, chạy smoke rồi mới cho phát hành.

**Why this priority**: Không có build tái lập thì không có gì để chứng nhận hay phát hành.

**Independent Test**: Chạy workflow `godot.yml` trên tag thử; 4 artifact + báo cáo dung lượng + kết quả quét + smoke xanh; cố ý thêm asset lớn → job dung lượng đỏ.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F133-AC-01 | KHI có tag `client/vX.Y.Z`, CI PHẢI export Windows 10+ x86_64, Android 9+ arm64-v8a (AAB + APK), iOS 15+ arm64 (runner macOS của GitHub Actions) và Web (runner Linux) trong image Godot ghim phiên bản + export templates; `client_version` = `X.Y.Z` và build number = `github.run_number` PHẢI được nhúng vào `Config` | NFR-COMP-001 · NFR-COMP-003 · [GODOT_ARCHITECTURE §11](../../docs/08-game-client/GODOT_ARCHITECTURE.md) · ADR-0013 · ADR-0088 | — |
| F133-AC-02 | Cấu hình môi trường (`api_base_url`, `realtime_url`, `cdn_base_url`, khóa công khai kiểm chữ ký manifest) PHẢI sinh từ biến/secret của GitHub Environment vào `res://config/<env>.json`; build production PHẢI KHÔNG đọc override `user://`; `export_presets.cfg` PHẢI KHÔNG chứa mật khẩu keystore/chứng chỉ | AGENTS R3, R4 · [GODOT_ARCHITECTURE §4, §11](../../docs/08-game-client/GODOT_ARCHITECTURE.md) | — |
| F133-AC-03 | KHI export xong, job `godot / scan-artifact` PHẢI giải nén artifact và fail nếu tìm thấy secret client `eduworld-game-backend`, URL `/protocol/openid-connect/token`, intent filter/redirect OAuth, khóa riêng ký, hay câu hỏi đề/answer key trong PCK | ADR-0068 · AGENTS R6, R31 | TC-AUTH-010 |
| F133-AC-04 | NẾU kích thước bản cài Android/iOS/Windows, tải đầu Web hoặc một gói vùng vượt `build_size`, THÌ job `godot / size-check` PHẢI fail; mỗi release PHẢI đính báo cáo dung lượng theo nền tảng và theo gói | NFR-PERF-006 · [PERFORMANCE_BUDGET §11](../../docs/08-game-client/PERFORMANCE_BUDGET.md) · [DEPLOYMENT §9](../../docs/11-operations/DEPLOYMENT.md) | — |
| F133-AC-05 | CI PHẢI build gói PCK theo vùng + chỉ mục gói (`bundle_id`, `version`, `url`, `sha256`, `size`) ký RSA-SHA256, tải lên `ew-game-bundles` theo khóa hash, không ghi đè object đã có; nội dung ngoài lõi PHẢI KHÔNG đóng vào bản cài | NFR-PERF-006 · ADR-0044 · [DEPLOYMENT §9](../../docs/11-operations/DEPLOYMENT.md) | TC-CNT-009 |
| F133-AC-06 | KHI artifact sẵn sàng, smoke headless PHẢI chạy boot → `getClientConfig` → đăng nhập học sinh QA staging qua form (`gameLogin`) → `createGameSession` → vào Hub; NẾU thất bại, THÌ job phát hành PHẢI bị chặn | [GODOT_ARCHITECTURE §11](../../docs/08-game-client/GODOT_ARCHITECTURE.md) · ADR-0083 | TC-AUTH-001 |

---

### User Story 2 — Ký số và phát hành theo kênh (Priority: P1)

Release Manager đẩy bản đã ký lên Google Play, App Store, CDN Web và installer Windows; dừng rollout khi crash tăng; client cũ được nhắc cập nhật đúng chính sách.

**Why this priority**: Exit criteria M6 yêu cầu ứng dụng ký số và phát hành trên mọi kênh.

**Independent Test**: Phát hành `client/v0.9.0-rc1` lên kênh nội bộ (Play internal, TestFlight, `play-staging`, installer staging); cài từ từng kênh và vào Hub.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F133-AC-07 | Job ký PHẢI chỉ chạy trên tag `client/v*` được bảo vệ bằng tag ruleset, nhận keystore Android/chứng chỉ Apple/Authenticode từ secret chỉ cấp cho GitHub Environment `staging`/`production` và PHẢI KHÔNG lưu khóa vào artifact hay log; NẾU người duyệt không thuộc nhóm Release Manager (required reviewers của environment `production`), THÌ job phát hành production PHẢI bị từ chối | FR-OPS-004 · [DEPLOYMENT §13](../../docs/11-operations/DEPLOYMENT.md) · AGENTS R4 · ADR-0088 | — |
| F133-AC-08 | KHI phát hành, CI PHẢI đẩy AAB lên Google Play track `internal`, IPA lên TestFlight, Web lên `play-staging` và installer Windows lên kênh staging; chuyển `closed`/`production`/App Store/`play.<domain>` PHẢI là bước thủ công có approval; APK ký PHẢI có sẵn cho MDM trường | [DEPLOYMENT §9, §11.2](../../docs/11-operations/DEPLOYMENT.md) | — |
| F133-AC-09 | Web PHẢI được tải lên thư mục theo phiên bản trên CDN, `.wasm`/`.pck` nén Brotli với `Cache-Control: immutable` theo hash, `index.html` (cache ngắn) đổi cuối cùng; rollback Web PHẢI là trỏ `index.html` về phiên bản trước | NFR-COMP-004 · [DEPLOYMENT §9, §11.2](../../docs/11-operations/DEPLOYMENT.md) | — |
| F133-AC-10 | KHI client Windows khởi động và manifest phát hành có bản mới, client PHẢI tải installer, kiểm chữ ký manifest, SHA-256 và Authenticode trước khi chạy; NẾU bất kỳ kiểm tra nào lỗi, THÌ client PHẢI giữ bản hiện tại và ghi `client_error` telemetry | [GODOT_ARCHITECTURE §11](../../docs/08-game-client/GODOT_ARCHITECTURE.md) · NFR-SEC-005 | — |
| F133-AC-11 | NẾU alert `ClientCrashRateHigh` bắn cho phiên bản đang rollout, THÌ runbook PHẢI dừng rollout store (halt staged rollout) và giữ Web/installer ở bản trước | [MONITORING §5](../../docs/11-operations/MONITORING.md) | — |
| F133-AC-12 | Trước mỗi release backend, CI PHẢI chạy smoke của bản client N-1 đang phát hành với staging mới; `min_client_version` PHẢI chỉ được nâng khi bản cũ hơn đã hết thời hạn hỗ trợ (≥ 1 học kỳ); KHI server trả `426 CLIENT_VERSION_UNSUPPORTED` hoặc `getClientConfig.update_required = true`, client PHẢI hiện màn "Cần cập nhật" có link kênh cài đúng nền tảng, không crash | NFR-COMP-002 · [DEPLOYMENT §9](../../docs/11-operations/DEPLOYMENT.md) | TC-AUTH-066 |

---

### User Story 3 — Chứng nhận trên device matrix 4 nền tảng (Priority: P1)

QA chạy bản release candidate trên thiết bị lab mọi hạng; báo cáo đạt AC-PERF-001…011, đăng nhập form chạy trên cả 4 nền tảng.

**Why this priority**: FR-GAME-009, NFR-COMP-001 là yêu cầu bắt buộc (ADR-0057); exit criteria M6.

**Independent Test**: Chạy bộ TC-PERF client trên lab §2.1; báo cáo `perf-reports/client-<ngày>.md` bảng thiết bị × cảnh × chỉ số.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F133-AC-13 | KHI có release candidate, QA PHẢI chạy bộ kiểm hiệu năng client trên toàn bộ thiết bị hạng Tầm trung và ít nhất 1 thiết bị Tối thiểu mỗi nền tảng ([TEST_STRATEGY §4.1](../../docs/12-project/TEST_STRATEGY.md)); mọi AC-PERF-001…011 PHẢI đạt; NẾU không đạt, THÌ release PHẢI bị chặn hoặc có phương án giảm chất lượng được duyệt (không loại thiết bị tầm trung) | FR-GAME-009 · NFR-COMP-001 · ADR-0057 | TC-PERF-009 · TC-PERF-019 · TC-PERF-020 · TC-PERF-021 · TC-PERF-022 · TC-PERF-023 · TC-PERF-024 |
| F133-AC-14 | Bản build release của mỗi nền tảng PHẢI qua benchmark headless từng zone; NẾU vượt ngân sách quá `perf_budget_tolerance_ci`, THÌ CI PHẢI chặn | FR-GAME-009 · [PERFORMANCE_BUDGET §15](../../docs/08-game-client/PERFORMANCE_BUDGET.md) | TC-PERF-018 |
| F133-AC-15 | Trên cả 4 nền tảng, học sinh PHẢI đăng nhập bằng form trong game (bàn phím vật lý, bàn phím ảo mobile, Web), không mở trình duyệt/WebView; trên Windows/Android/iOS, mở lại game khi phiên còn hạn PHẢI tự vào Hub không hiện form, trên Web PHẢI luôn hiện form; Ở NƠI nền tảng là desktop/mobile, client PHẢI bật `certificate_pinning`; Web dựa vào HSTS | ADR-0068 · ADR-0086 | TC-AUTH-001 · TC-AUTH-082 · TC-AUTH-084 |
| F133-AC-16 | Bản Web PHẢI dùng renderer Compatibility (WebGL 2), export single-threaded (không cần COOP/COEP), preset tối đa Trung bình, heap theo [PERFORMANCE_BUDGET §14](../../docs/08-game-client/PERFORMANCE_BUDGET.md), âm thanh bật sau tương tác đầu; NẾU trình duyệt không có WebGL 2 hoặc không phải Chrome/Edge được hỗ trợ, THÌ trang PHẢI hiện thông báo thân thiện thay vì tải engine | NFR-COMP-004 · ADR-0057 | TC-SIM-005 |

---

### User Story 4 — Phòng máy: bộ cài kèm nội dung, cache LAN, tải trước ngoài giờ (Priority: P2)

School Admin cài bộ cài trường (kèm gói vùng theo khối) qua MDM/USB, có thể đặt cache LAN; cả lớp 40 máy vào Hub đầu tiết mà không nghẽn đường truyền.

**Why this priority**: NFR-PERF-019; quan trọng cho trường nhưng không chặn phát hành cá nhân.

**Independent Test**: Phòng giả lập 40 client trên đường truyền `classroom_reference_bandwidth` với cache LAN; đo thời gian cả lớp vào Hub.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F133-AC-17 | CI PHẢI build bộ cài trường (Windows installer, APK) kèm gói vùng công khai theo khối, cập nhật delta theo hash; bộ cài PHẢI KHÔNG chứa dữ liệu tenant, secret, câu hỏi đề hay media private, và nằm trong `build_size` | NFR-PERF-019 · [PERFORMANCE_BUDGET §10.1](../../docs/08-game-client/PERFORMANCE_BUDGET.md) · AGENTS R31 | — |
| F133-AC-18 | Ở NƠI `device.cfg` có `lan_cache_url`, client PHẢI tải PCK/media công khai qua LAN trước, kiểm SHA-256 mọi gói và về CDN khi lỗi/lệch hash; PHẢI KHÔNG gửi request API, câu hỏi đề hay media private qua LAN; NẾU `lan_cache_url` không phải URL `http(s)` hợp lệ hoặc dài hơn 2048 ký tự, THÌ client PHẢI bỏ qua và ghi cảnh báo | NFR-PERF-019 · ADR-0044 · [DEPLOYMENT §9](../../docs/11-operations/DEPLOYMENT.md) | — |
| F133-AC-19 | TRONG KHI thiết bị có `shared_device: true` và nằm trong `offhours_prefetch_window`, client PHẢI tải trước gói vùng từ chỉ mục gói có chữ ký và media công khai đã biết, không đăng nhập học sinh, không gọi API cần token; client PHẢI dùng gói cài sẵn khi có thay vì tải lại | NFR-PERF-019 · [PERFORMANCE_BUDGET §10.1](../../docs/08-game-client/PERFORMANCE_BUDGET.md) | — |
| F133-AC-20 | Ở NƠI chế độ dữ liệu thấp bật (trường bật hoặc đo băng thông thấp), client PHẢI tắt tải trước vùng kế tiếp và giảm chất lượng media | NFR-PERF-019 · [PERFORMANCE_BUDGET §10.1](../../docs/08-game-client/PERFORMANCE_BUDGET.md) | — |
| F133-AC-21 | KHI tải gói/media, client PHẢI gửi telemetry `content_download` (`source` = `preinstalled`/`lan_cache`/`cdn`, `bytes`, `bundle_id`) qua `postTelemetry`, không PII | NFR-PERF-019 · NFR-OBS-003 · [MONITORING §3](../../docs/11-operations/MONITORING.md) | TC-ANL-003 |
| F133-AC-22 | KHI mô phỏng 40 client trên `classroom_reference_bandwidth` có gói cài sẵn hoặc cache LAN, cả lớp PHẢI vào Hub ≤ 2 phút (AC-PERF-012) | NFR-PERF-019 · [PERFORMANCE_BUDGET §16](../../docs/08-game-client/PERFORMANCE_BUDGET.md) | — |
| F133-AC-27 | KHI cài đặt trên Windows, bộ cài PHẢI có lựa chọn "Máy dùng chung" (mặc định, ghi `shared_device: true` vào `device.cfg`) và "Máy cá nhân" (ghi `shared_device: false`), kể cả tham số cài im lặng cho phòng máy; với "Máy dùng chung" đăng nhập PHẢI không ghi refresh token vào DPAPI và thoát game = đăng xuất | AC-AUTH-030 · ADR-0078 · ADR-0086 | TC-AUTH-090 |

---

### User Story 5 — Kho bảo mật giữ phiên trong bản phát hành (Priority: P1)

Học sinh cài bản chính thức trên laptop Windows, điện thoại Android hoặc iPad: phiên đăng nhập được giữ an toàn trong kho bảo mật của hệ điều hành, không bị sao lưu ra đám mây hay chuyển sang máy khác; bản Web không lưu gì (ADR-0086).

**Why this priority**: ADR-0086; thiếu adapter hoặc cấu hình backup sai làm lộ refresh token hoặc làm tính năng tự vào lại không chạy trên bản phát hành.

**Independent Test**: Cài artifact release trên thiết bị lab mỗi nền tảng: đăng nhập, thoát, mở lại → vào Hub không form; kiểm manifest/entitlement; chạy `adb backup`/`bmgr` và kiểm backup không chứa file phiên; Web: tải lại tab → form.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F133-AC-23 | Bản iOS PHẢI có adapter `SecureStorage` iOS Keychain (`kSecClassGenericPassword`, `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly`, `kSecAttrSynchronizable = false`) hiện thực interface của F060, và PHẢI KHÔNG bật iCloud Keychain sharing hay thêm `keychain-access-groups` ngoài mặc định của app | ADR-0086 · NFR-COMP-003 · [GODOT_ARCHITECTURE §5.2](../../docs/08-game-client/GODOT_ARCHITECTURE.md#52-lưu-phiên-bằng-securestorage-adr-0086) | TC-AUTH-082 |
| F133-AC-24 | Manifest Android release PHẢI loại file phiên khỏi Auto Backup và device-to-device transfer (`android:allowBackup="false"` hoặc `dataExtractionRules`/`fullBackupContent` loại thư mục phiên), PHẢI đóng gói plugin Keystore của F060 và PHẢI KHÔNG khai báo permission runtime mới cho việc này; NẾU manifest thiếu cấu hình loại trừ, THÌ job `godot / scan-artifact` PHẢI fail | ADR-0086 · AGENTS R4 | TC-AUTH-084 |
| F133-AC-25 | KHI export mỗi target, CI PHẢI kiểm `SecureStorageFactory` chọn đúng adapter (Windows → DPAPI, Android → Keystore, iOS → Keychain, Web → `MemoryOnly`) và thư viện native tương ứng có trong artifact; NẾU thiếu, hoặc bản Web có mã ghi refresh token vào localStorage/IndexedDB, THÌ job export PHẢI fail | ADR-0086 · [GODOT_ARCHITECTURE §11](../../docs/08-game-client/GODOT_ARCHITECTURE.md) | TC-AUTH-084 |
| F133-AC-26 | KHI chứng nhận release candidate, QA PHẢI kiểm trên mỗi nền tảng cá nhân (Windows, Android, iOS): đăng nhập → thoát → mở lại vào Hub không form; Đăng xuất → mở lại hiện form; sao chép dữ liệu app sang máy/user khác → không tự vào được | ADR-0086 | TC-AUTH-082 · TC-AUTH-088 |

### Edge Cases

- NẾU chỉ mục gói hoặc manifest phát hành có chữ ký sai, THÌ client PHẢI bỏ qua toàn bộ chỉ mục đó, dùng gói đã cache và báo `client_error` (không tải gói không kiểm được).
- NẾU bộ nhớ cache vượt trần `build_size` (cache thiết bị), THÌ client xóa LRU vùng ít dùng, không xóa gói cài sẵn của bộ cài trường.
- NẾU store từ chối bản (review App Store), THÌ backend vẫn giữ `min_client_version` cũ; các kênh khác phát hành độc lập.
- NẾU Godot export template đổi phiên bản, THÌ phải chạy lại toàn bộ chứng nhận US3 và F133-AC-26.
- NẾU người dùng khôi phục iPhone từ bản sao lưu hoặc chuyển sang máy mới, THÌ mục Keychain `ThisDeviceOnly` không đi theo → game hiện form (F133-AC-23).
- Authorization: phát hành chỉ Release Manager (F133-AC-07). Tenant isolation: artifact và bộ cài không chứa dữ liệu tenant (F133-AC-17); client gửi `X-Tenant-Id` theo F061. Idempotency: upload theo khóa hash không ghi đè (F133-AC-05). Lỗi chính: `426 CLIENT_VERSION_UNSUPPORTED` (F133-AC-12). Giới hạn độ dài: `lan_cache_url` (F133-AC-18).

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
| NFR-PERF-019 | Bộ cài trường, cache LAN, tải trước ngoài giờ, chế độ dữ liệu thấp, telemetry nguồn tải | PERFORMANCE_BUDGET §10.1 |

### Key Entities

- **Bản phát hành client**: nền tảng, `client_version`, build number, artifact, chữ ký, kênh.
- **Manifest phát hành**: phiên bản mới nhất theo nền tảng/kênh, URL, SHA-256, kích thước, chữ ký.
- **Chỉ mục gói**: danh sách gói vùng theo `client_version` (id, version, URL hash, SHA-256), có chữ ký.
- **Bộ cài trường**: installer/APK + gói vùng theo khối.
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
| 4 nền tảng, một codebase GDScript | ADR-0013 · ADR-0014 |
| Không secret/endpoint token Keycloak trong build | ADR-0068 · AGENTS R4 |
| Refresh token chỉ trong kho bảo mật của hệ điều hành, không sao lưu ra đám mây; Web chỉ RAM | ADR-0086 |
| Không câu hỏi đề/answer key/media private trên CDN hay cache LAN | AGENTS R31 · ADR-0044 |
| Không hardcode URL môi trường | AGENTS R3 |
| Không loại thiết bị tầm trung để giữ tính năng | ADR-0057 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Từ một tag, cả 4 nền tảng có artifact đã ký, qua quét và smoke trong một lần chạy workflow.
- **SC-002**: 100% thiết bị hạng Tầm trung trong lab đạt AC-PERF-001…011 ở release candidate cuối.
- **SC-003**: 0 artifact phát hành vượt `build_size` hoặc chứa secret.
- **SC-004**: Bản N-1 vào Hub được với backend mới trong 100% lần release backend.
- **SC-005**: Phòng 40 máy có gói cài sẵn/cache LAN vào Hub ≤ 2 phút.
- **SC-006**: 100% artifact release có đúng adapter `SecureStorage` theo nền tảng; 0 file phiên trong bản sao lưu Android/iCloud.

## Ngoài phạm vi

- `QualityManager`, tự phát hiện preset, renderer fallback → F062; form đăng nhập, hàng chờ, tự vào lại/đăng xuất, xử lý 426 ở `ApiClient` → F061; interface `SecureStorage` + adapter Windows/Android/`MemoryOnly` → F060; `client-config`, `min_client_version` phía server → F014.
- Ingest telemetry và metric `eduworld_client_content_download_bytes_total` ở module game → F080.
- Trang tải bộ cài trên portal cho School Admin → chưa có feature (đề xuất bổ sung CODE_PLAN); Release 1.0 phát bộ cài qua link CDN/MDM.
- Microsoft Store, Firefox/Safari, macOS/Linux client: không làm.
- Stack CDN/object storage → F130; alert crash → F131.

## Assumptions

- Tài khoản Google Play Console, Apple Developer (tổ chức) và chứng chỉ Authenticode được mua trước M6 (quy trình tổ chức).
- Nhà cung cấp CDN theo OQ-10 [TBD]; cấu hình header viết dạng độc lập nhà cung cấp.
- Thiết bị lab §2.1 có sẵn; device matrix chốt sau khảo sát OQ-04 [Confirmed].
- ADR-0079 Rejected: giữ đủ 4 nền tảng (ADR-0013). ADR-0075 Accepted không đổi quy trình build (client dùng URL CDN; upload gói qua API S3 chuẩn).
- CI/CD là GitHub Actions (ADR-0088): `godot.yml` export Windows/Android/Web trên runner Linux, export + ký iOS trên runner macOS (tốn phút tính phí — chỉ chạy job iOS trên tag và nhánh `main`); action ghim SHA commit, Dependabot cập nhật action.
