# ADR-0101 — Bỏ Windows khỏi nền tảng client Release 1.0 — client chỉ Web, Android, iOS

> **Trạng thái:** Accepted · một phần supersedes ADR-0013 (Windows trong 4 nền tảng, thứ tự phát triển Windows), ADR-0057 (hạng thiết bị "PC Windows", cột ngân sách PC), ADR-0073 (Windows trong nền tảng pilot), ADR-0078 mục 5 (Windows mặc định máy dùng chung qua bộ cài), ADR-0086 mục 1 (Windows DPAPI), ADR-0088 (export Windows trong `godot.yml`) · **Ngày:** 2026-11-24 · **OQ:** OQ-04 · **Tài liệu:** [GODOT_ARCHITECTURE](../08-game-client/GODOT_ARCHITECTURE.md) · [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) · [DEPLOYMENT](../11-operations/DEPLOYMENT.md) · [PRODUCT_REQUIREMENTS](../00-overview/PRODUCT_REQUIREMENTS.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

[ADR-0013](ADR-0013-nen-tang-client-release-1-0-gom-windows-android-ios-web.md) đặt Release 1.0 trên 4 nền tảng: Windows, Android, iOS, Web. Bản Windows kéo theo bộ cài cho phòng máy (chọn "máy dùng chung"/"máy cá nhân", ADR-0078 mục 5), ký Authenticode, tự cập nhật, GDExtension DPAPI để lưu refresh token (ADR-0086 mục 1), job CI build trên runner Windows và hạng thiết bị "PC Windows" riêng trong ngân sách hiệu năng (ADR-0057).

Ngày 2026-11-24 owner dự án quyết định: "bỏ hẳn xuất ở windows" và "game phải xuất chạy trên web, android và ios mới đúng rule". Client chỉ phát hành trên **Web, Android, iOS**.

## Quyết định

1. **Nền tảng phát hành Release 1.0:** Web (WebGL 2, renderer Compatibility, export single-threaded — như ADR-0013), Android 9+, iOS 15+. Một codebase GDScript (ADR-0014). Không có bản Windows (cũng như macOS/Linux) ở bất kỳ giai đoạn nào của Release 1.0, kể cả pilot (ADR-0073).
2. **Bỏ toàn bộ chuỗi Windows:** không có export preset Windows, không có bộ cài, không ký Authenticode, không tự cập nhật (auto-update) trên Windows, không có trang tải bản Windows, không có GDExtension DPAPI. CI `godot.yml` bỏ job build plugin Windows và bước export Windows; kiểm tra artifact chạy trên các preset còn lại.
3. **PC phòng máy của trường chơi bản Web** trong trình duyệt được hỗ trợ (danh sách trình duyệt do [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) sở hữu). Hạng thiết bị "PC Windows" của ADR-0057 không còn là một nền tảng riêng: cấu hình phần cứng PC tham chiếu chỉ còn dùng để mô tả máy chạy trình duyệt, và PC dùng **ngân sách, preset và mục tiêu khung hình của Web**.
4. **Phiên trên PC phòng máy:** Web chỉ giữ refresh token trong RAM (ADR-0086 mục 1, [Confirmed]) nên mỗi lần mở lại game trên PC phòng máy, học sinh đăng nhập lại; đóng tab/trình duyệt là kết thúc phiên trên máy. Hành vi này thay cho "Windows mặc định máy dùng chung" của ADR-0078 mục 5; màn "Tiếp tục là <nickname>?" khi tự vào lại bằng refresh token vẫn áp dụng trên Android, iOS.
5. **Lưu phiên và bảo vệ kênh truyền theo nền tảng:**
   - `SecureStorage`: Android Keystore, iOS Keychain; Web, `shared_device = true` và mọi hệ điều hành không phát hành (Windows, macOS, Linux khi chạy từ Godot editor/bản debug của dev) dùng `MemoryOnly`.
   - Certificate pinning chỉ còn cho Android và iOS; Web dựa vào TLS/HSTS của trình duyệt.
6. **Giá trị `windows` trong contract** (`X-Client-Platform`, `GameLoginRequest.platform`, `GameSessionCreate.client_platform`, `platform` của `getClientConfig`, lịch sử đăng nhập, event game session) **giữ nguyên** cùng nhóm với `macos`, `linux`: chỉ mô tả hệ điều hành của tiến trình khi dev chạy game từ Godot editor hoặc bản debug; không phải nền tảng phát hành. Lý do: dev chạy client từ editor trên Windows gọi backend thật (ví dụ F062 T018); bỏ giá trị này sẽ làm hỏng vòng phát triển và là breaking change của contract mà không mang lại lợi ích phát hành. Dữ liệu lịch sử có `windows` giữ nguyên, không có migration xóa hay đổi dữ liệu (R12).
7. **Gói nội dung cài sẵn cho trường** chỉ còn dạng Android (MDM/USB). Bản Web tải nội dung theo vùng từ CDN như mọi trình duyệt.

## Hệ quả

(+) Bớt một nền tảng phải build, ký, test và hỗ trợ; bỏ runner Windows, toolchain C++/scons và GDExtension DPAPI; PC phòng máy không cần cài đặt hay quyền quản trị.
(−) PC phòng máy phải đăng nhập lại mỗi lần mở game (Web không lưu refresh token bền vững); ngân sách hiệu năng Web (Compatibility, đơn luồng) áp cho cả PC phòng máy, nên PC tầm trung không còn mục tiêu 60 FPS và preset Cao của bản native; nhà trường không có bộ cài Windows để phân phối qua phần mềm quản lý phòng máy.
(−) Phải sửa PRD, GODOT_ARCHITECTURE, PERFORMANCE_BUDGET, GAME_NETWORKING, DEPLOYMENT, CONFIG_DEFAULTS, TEST_STRATEGY, ROADMAP, CODE_PLAN và feature F060, F061, F062, F133, F134; xóa export preset Windows, adapter DPAPI, job CI Windows.

## Phương án đã loại

- Giữ Windows như nền tảng phụ (chỉ phát hành nội bộ cho phòng máy): trái quyết định của owner, vẫn phải duy trì toolchain và ký số.
- Xóa giá trị `windows` khỏi enum nền tảng trong contract: làm hỏng chạy game từ editor trên máy dev Windows và tạo breaking change cho dữ liệu lịch sử; không cần để đạt mục tiêu "không xuất Windows".

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-24 | Tạo ADR (Accepted) theo quyết định của owner: bỏ hẳn bản Windows, client chỉ Web, Android, iOS | Owner dự án |
