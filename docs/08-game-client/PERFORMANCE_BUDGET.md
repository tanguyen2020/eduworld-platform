# Performance Budget — Ngân sách hiệu năng client (máy tầm trung)

> Trạng thái: Draft · Phiên bản: 0.3 · Owner: Senior Godot Developer + Technical Artist · Cập nhật: 2026-11-25 (ADR-0103/F068: ngân sách giữ nguyên với art direction mới, blob shadow trên Web) · 2026-11-24 (ADR-0101: bỏ Windows, client chỉ Web/Android/iOS) · 2026-11-15
> Liên quan: [ADR-0057](../adr/ADR-0057-chay-tren-may-tam-trung-hang-thiet-bi-va-ngan-sach-hieu-nang.md) · [ADR-0013](../adr/ADR-0013-nen-tang-client-release-1-0-gom-windows-android-ios-web.md) · [ADR-0101](../adr/ADR-0101-bo-windows-khoi-nen-tang-client-release-1-0.md) · [ADR-0047](../adr/ADR-0047-hai-he-thong-giao-dien-game-godot-3d-va-admin-ops-cms-lms.md) · [ADR-0055](../adr/ADR-0055-cong-thuc-toan-vat-ly-hoa-hoc-mathjax-4-hien-thi-mathlive-nh.md) · [GODOT_ARCHITECTURE](GODOT_ARCHITECTURE.md) · [WORLD_RENDERING](WORLD_RENDERING.md) · [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) · [DESIGN_SYSTEM](../05-ux/DESIGN_SYSTEM.md) · [TEST_STRATEGY](../12-project/TEST_STRATEGY.md)

**Mục lục:** 1. Mục đích và phạm vi · 2. Hạng thiết bị tham chiếu · 3. Mục tiêu khung hình · 4. Renderer theo nền tảng · 5. Preset chất lượng · 6. Tự phát hiện và độ phân giải động · 7. Ngân sách theo cảnh · 8. Ngân sách asset · 9. Ngân sách UI 2D · 10. Ngân sách mạng (10.1 phòng máy) · 11. Thời gian tải và dung lượng · 12. Pin và nhiệt · 13. Kỹ thuật tối ưu bắt buộc · 14. Web export · 15. Quy trình kiểm soát · 16. Tiêu chí nghiệm thu

## 1. Mục đích và phạm vi

Stakeholder yêu cầu game **đặc biệt phải chạy được trên máy tầm trung** (ADR-0057, [Confirmed]). Tài liệu này là **nguồn duy nhất** cho mọi con số hiệu năng phía client Godot: hạng thiết bị, FPS, renderer, preset, ngân sách cảnh/asset/UI/mạng, thời gian tải, dung lượng và quy trình kiểm soát. Tài liệu khác (WORLD_RENDERING, CHARACTER_SYSTEM, DESIGN_SYSTEM, DEPLOYMENT, PRD) chỉ **link** tới đây, không lặp số.

Ngoài phạm vi: hiệu năng server/API (NFR-PERF trong [PRODUCT_REQUIREMENTS](../00-overview/PRODUCT_REQUIREMENTS.md), [MONITORING](../11-operations/MONITORING.md)) và năng lực realtime ([REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md)).

Quy tắc: tính năng nào vượt ngân sách ở hạng **Tầm trung** phải có phương án giảm chất lượng; **không** được loại thiết bị tầm trung để giữ tính năng.

## 2. Hạng thiết bị tham chiếu

Theo ADR-0057 [Confirmed, chốt sau khảo sát thiết bị trường — OQ-04] và [ADR-0101](../adr/ADR-0101-bo-windows-khoi-nen-tang-client-release-1-0.md): không có bản Windows; **PC (gồm PC phòng máy) chơi bản Web** trong trình duyệt được hỗ trợ (§14), nên cột Web mô tả cả cấu hình PC chạy trình duyệt và PC dùng ngân sách, preset, mục tiêu khung hình của Web.

| Hạng | Android | iOS | Web (PC chạy trình duyệt) | Preset mặc định |
|---|---|---|---|---|
| **Tối thiểu** | RAM 3 GB, Android 9, GPU OpenGL ES 3.0 | — | WebGL 2 trên PC CPU 4 nhân ~2 GHz, RAM 4 GB, GPU tích hợp hỗ trợ OpenGL 3.3 | Thấp |
| **Tầm trung (mục tiêu chính)** | RAM 4 GB, SoC tầm Snapdragon 680 / Helio G85, GPU Mali-G52 / Adreno 610 | iPhone 11 / iPad thế hệ 8, iOS 15 | Chrome/Edge mới trên PC Intel Core i3 thế hệ 10 / Ryzen 3 3200G, RAM 8 GB, iGPU Intel UHD 630 / Radeon Vega 8 | Trung bình |
| **Cao** | RAM ≥ 6 GB, SoC cận cao cấp | iPhone 13 trở lên | — (Web tối đa preset Trung bình, §14) | Cao |

### 2.1. Thiết bị kiểm thử tham chiếu (lab) [Confirmed]

| Mã | Thiết bị | Hạng | Renderer kiểm thử |
|---|---|---|---|
| DEV-PC-MIN | PC văn phòng i3 thế hệ 6–8, 4 GB RAM, Intel HD 520/620 + Chrome/Edge bản mới (bản Web) | Tối thiểu | Compatibility (WebGL 2) |
| DEV-PC-MID | Mini PC/laptop i3-10100 (UHD 630), 8 GB RAM (chạy bản Web qua DEV-WEB-MID) | Tầm trung | Compatibility (WebGL 2) |
| DEV-PC-AMD | Ryzen 3 3200G (Vega 8), 8 GB RAM + Chrome/Edge bản mới (bản Web) | Tầm trung | Compatibility (WebGL 2) |
| DEV-AND-MIN | Android 3 GB RAM, Helio G35/G37 (PowerVR) | Tối thiểu | Compatibility |
| DEV-AND-MID1 | Android 4 GB RAM, Snapdragon 680 (Adreno 610) | Tầm trung | Mobile (Vulkan) |
| DEV-AND-MID2 | Android 4 GB RAM, Helio G85 (Mali-G52) | Tầm trung | Mobile (Vulkan) |
| DEV-TAB-MID | Tablet Android 10" 4 GB RAM phổ biến ở trường | Tầm trung | Mobile |
| DEV-IOS-MID | iPhone 11 hoặc iPad thế hệ 8 | Tầm trung | Mobile (Metal) |
| DEV-WEB-MID | DEV-PC-MID + Chrome và Edge bản mới | Tầm trung | Compatibility (WebGL 2) |

Danh sách cập nhật theo kết quả khảo sát thiết bị thực tế tại các trường pilot.

## 3. Mục tiêu khung hình

Cột Mobile / Web áp dụng cho Android, iOS và Web, gồm PC phòng máy chạy bản Web (không còn cột PC native — [ADR-0101](../adr/ADR-0101-bo-windows-khoi-nen-tang-client-release-1-0.md)).

| Hạng | Mobile / Web | Đo theo |
|---|---|---|
| Tối thiểu (Thấp) | ≥ 30 FPS (frame time ≤ 33,3 ms) | FPS p10 (10% khung chậm nhất; cùng định nghĩa `fps_p10` của GAME_ANALYTICS) |
| **Tầm trung (Trung bình)** | **ổn định 30 FPS** (khóa 30), p10 ≥ 27 FPS | FPS p10, khung giật > 100 ms ≤ 1/phút |
| Cao (chỉ Android, iOS) | 30 FPS khóa (60 tùy chọn) | FPS p10 |

- **Ngân sách thời gian một khung** ở hạng Tầm trung mobile (33,3 ms): game logic + script ≤ 6 ms, vật lý ≤ 3 ms, animation ≤ 4 ms, render CPU ≤ 10 ms, GPU ≤ 28 ms, còn lại dự phòng [Confirmed].
- Màn học tập (activity player) không được giật khi học sinh thao tác: phản hồi chạm/nhấn ≤ 100 ms.

## 4. Renderer theo nền tảng

| Nền tảng | Renderer mặc định | Dự phòng tự chuyển | Ghi chú |
|---|---|---|---|
| Android | **Mobile** (Vulkan) | **Compatibility** (OpenGL ES 3.0) với GPU trong danh sách đen Vulkan hoặc khi khởi tạo lỗi | Danh sách GPU cấu hình qua `client-config` |
| iOS | **Mobile** (Metal qua MoltenVK) | — | |
| Web (gồm PC phòng máy, ADR-0101) | **Compatibility** (WebGL 2) | — | Xem §14 |
| Thumbnail/turntable (server) | Forward+ preset cố định | — | ADR-0047, không ảnh hưởng client |
| Desktop chạy từ editor/bản debug (Windows, macOS, Linux — không phát hành, ADR-0101) | Mobile; Forward+ ở preset Cao | Compatibility | Chỉ để phát triển: `QualityPolicy` coi mọi nền tảng ngoài Android/iOS/Web là nhánh desktop (giới hạn 60 FPS ở preset Trung bình/Cao); không có ngân sách, thiết bị tham chiếu hay nghiệm thu cho nhánh này |

- Lựa chọn renderer lưu ở `user://settings.cfg`; lần khởi động lỗi do renderer → tự chuyển sang Compatibility ở lần sau và gửi telemetry.
- Art direction stylized/toon phải đạt **renderer parity** giữa Mobile và Compatibility (shader toon dùng tính năng chung, không phụ thuộc SSAO/SSR/SDFGI); kiểm tra theo [WORLD_RENDERING](WORLD_RENDERING.md) và [DESIGN_SYSTEM](../05-ux/DESIGN_SYSTEM.md). Trên Compatibility (Web) mặt trời không có bóng thời gian thực ở mọi preset (F067, [WORLD_RENDERING §7](WORLD_RENDERING.md#7-lighting)); nhân vật dùng blob shadow ở đó (F068). Art direction "Chibi anime cartoon 3D" ([ADR-0103](../adr/ADR-0103-art-direction-chibi-anime-cartoon-3d-thay-stylized-low-poly.md)) giữ nguyên các ngân sách §7, §8: khối tròn làm bằng pháp tuyến mượt (không thêm tam giác), chỉ vát cạnh khối đủ dày, vật thể lặp/xa/NPC giữ bậc chia gốc, mặt đất và trang trí nhỏ không đổ bóng; đo bằng benchmark có renderer (§15) trên cả Mobile và Compatibility.

## 5. Preset chất lượng

| Thiết lập | Thấp | Trung bình (mặc định tầm trung) | Cao |
|---|---|---|---|
| Render scale 3D (UI luôn 1,0) | 0,75 (động 0,6–0,75) | 0,9 (động 0,7–1,0) | 1,0 |
| Giới hạn FPS | 30 | 30 | 60 |
| Bóng | Blob shadow cho nhân vật (F068), không bóng realtime | 1 directional, 1 cascade, shadow map 1024, khoảng cách 25 m | 2 cascade, 2048, 40 m |
| Khoảng cách nhìn (visibility range) | 40 m | 70 m | 120 m |
| LOD bias (`mesh_lod_threshold`) | 4,0 | 2,0 | 1,0 |
| Nhân vật render đầy đủ (LOD0/1) | 10 | 20 | 40 |
| Khử răng cưa | Tắt | FXAA | MSAA 2x (Forward+/Mobile) |
| Hậu kỳ | Tắt (chỉ tone mapping) | Tắt | Bloom + glow nhẹ |
| Mật độ cỏ / vật trang trí | 25% | 60% | 100% |
| Hạt (particle) tối đa cùng lúc | 300 | 800 | 2.000 |
| Độ phân giải video bài học | 480p | 720p | 720p (1080p tùy chọn) |
| Animation UI / chuyển cảnh | Rút gọn (fade) | Đầy đủ, tắt hiệu ứng nền động | Đầy đủ |
| Outline toon | Tắt dần, hết ở 12 m [Proposed] | Bật, tắt ở > 30 m | Bật |
| Âm thanh đồng thời | 16 voice | 24 voice | 32 voice |

Học sinh đổi preset và từng thiết lập trong Cài đặt; lựa chọn lưu theo thiết bị (`user://settings.cfg`), không đồng bộ lên server.

## 6. Tự phát hiện và độ phân giải động

- **Lần chạy đầu** (sau đăng nhập, trước khi vào Hub, ≤ 5 giây): chạy cảnh benchmark ngắn (camera quay quanh Hub mẫu) đo frame time p50/p10 + đọc RAM, tên GPU, renderer khả dụng → chọn preset. Có thể bỏ qua và chọn tay.
- Danh sách GPU/SoC đã biết (`client-config.device_profiles`) ép preset hoặc renderer cho thiết bị có lỗi biết trước.
- **Độ phân giải động**: bộ điều khiển (`QualityManager`) theo dõi frame time trung bình 2 giây; vượt mục tiêu 10% → giảm render scale 0,05; dưới mục tiêu 20% trong 10 giây → tăng 0,05; giới hạn theo preset (§5). UI không bị scale.
- Nếu ở render scale tối thiểu mà vẫn dưới mục tiêu trong 30 giây → gợi ý hạ preset (không tự đổi giữa trận/bài).
- Nhiệt độ/hạn chế pin (Android `PowerManager` thermal status qua plugin) ≥ "moderate" → hạ trần FPS/scale một mức.

## 7. Ngân sách theo cảnh

Áp dụng cho hạng **Tầm trung, preset Trung bình**, cho Android, iOS và Web (gồm PC phòng máy chạy bản Web; không có ngân sách PC native — ADR-0101).

| Cảnh | Tam giác hiển thị | Draw call | Texture | RAM tiến trình | Vật thể động | Hạt |
|---|---|---|---|---|---|---|
| **Hub đông** (40 người chơi trong instance) | ≤ 150k | ≤ 100 | ≤ 256 MB | ≤ 1 GB | ≤ 60 (người chơi + NPC + vật tương tác) | ≤ 800 |
| Vùng môn học / khám phá (≤ 40 người) | ≤ 120k | ≤ 90 | ≤ 256 MB | ≤ 900 MB | ≤ 50 | ≤ 600 |
| Private instance (party ≤ 8) | ≤ 100k | ≤ 80 | ≤ 200 MB | ≤ 800 MB | ≤ 30 | ≤ 500 |
| Boss co-op (≤ 8 người + boss) | ≤ 120k | ≤ 90 | ≤ 220 MB | ≤ 900 MB | ≤ 25 | ≤ 1.000 (đỉnh 3 s) |
| Trận 1v1 / đội (≤ 8 người) | ≤ 80k | ≤ 70 | ≤ 180 MB | ≤ 750 MB | ≤ 20 | ≤ 400 |
| **Activity player** (thế giới 3D tạm dừng phía sau) | ≤ 30k (nền mờ/ảnh chụp) | ≤ 40 | ≤ 128 MB thêm cho media/SVG của bài | ≤ 700 MB | Chỉ UI | ≤ 100 (hiệu ứng đúng/sai) |

- Khi mở activity player toàn màn, thế giới 3D **tạm dừng render** (thay bằng ảnh chụp mờ) hoặc giảm còn 10 FPS; mạng hub tiếp tục ở tần suất thấp.
- Ánh sáng động: 1 `DirectionalLight3D` + ≤ 4 `OmniLight3D`/`SpotLight3D` trong tầm nhìn; không GI realtime.
- Physics: chỉ người chơi cục bộ, NPC di chuyển và vật tương tác là body động; props tĩnh dùng `StaticBody3D` gộp collider.

## 8. Ngân sách asset

| Loại asset | Tam giác (LOD0 / LOD1 / LOD2) | Texture | Khác |
|---|---|---|---|
| Nhân vật người chơi (tổng body + trang phục + phụ kiện) | ≤ 8k / 3k / 1k; impostor billboard > 30 m | 1 atlas 512² (Mobile/Web) · 1024² (preset Cao) | ≤ 60 xương, ≤ 4 bone influence, ≤ 3 material |
| NPC chính | ≤ 10k / 4k / 1,5k | 1024² | ≤ 70 xương |
| Boss | ≤ 25k / 10k / 4k | 2 × 1024² | ≤ 100 xương, VFX theo ngân sách hạt |
| Prop nhỏ | ≤ 500 | Palette/atlas chung của zone | `visibility_range_end` theo preset |
| Prop lớn | ≤ 3k / 1k | Atlas zone | |
| Tòa nhà | ≤ 10k / 4k / 1k | Atlas zone 1024²–2048² | Occluder bake |
| Thực vật lặp (cây/cỏ/đá) | Cây ≤ 2k, cỏ ≤ 50/cụm | Atlas | Bắt buộc `MultiMeshInstance3D` |
| Mặt đất/địa hình mỗi zone | ≤ 40k tổng | ≤ 4 lớp texture | |

- **Nén texture theo nền tảng:** Android ETC2 + ASTC (VRAM compressed), iOS ASTC, Web: S3TC/ETC2 theo hỗ trợ WebGL 2 (xuất cả hai). Mipmap bật cho texture 3D; UI không mipmap.
- **Âm thanh:** Ogg Vorbis; nhạc nền stream (không nạp toàn bộ vào RAM), 96–128 kbps; hiệu ứng ≤ 1 giây ở dạng nén nạp sẵn; tổng âm thanh nạp sẵn mỗi zone ≤ 16 MB.
- **Video bài học:** Ogg Theora, 480p/720p theo preset (§5), bitrate ≤ 1,5 Mbps ở 720p.
- **Atlas và material:** mỗi zone ưu tiên một material toon dùng palette texture chung; số material duy nhất trong cảnh Hub ≤ 40.
- Asset vượt ngân sách bị **chặn ở bước import CI** (script kiểm tra số tam giác, kích thước texture, số xương) trừ khi có ngoại lệ ghi rõ.

## 9. Ngân sách UI 2D

| Chỉ số | Ngân sách (Tầm trung) |
|---|---|
| Thời gian CPU cho UI mỗi khung | ≤ 3 ms (mobile/Web) |
| Số `Control` hiển thị đồng thời trên HUD | ≤ 150 |
| Danh sách dài (chat, inventory, bạn bè) | Bắt buộc ảo hóa (chỉ tạo item trong tầm nhìn + đệm), tái dùng node |
| Cập nhật HUD | Theo sự kiện/signal, không cập nhật mỗi frame; số liệu dạng chữ cập nhật ≤ 4 lần/giây |
| UI trong không gian 3D (`SubViewport`) | ≤ 4 viewport hoạt động cùng lúc, cập nhật `UPDATE_WHEN_VISIBLE`/khi đổi nội dung, độ phân giải ≤ 512² |
| Font | Dynamic font có cache glyph; tối đa 2 họ font; MSDF cho chữ trong 3D |
| Công thức (SVG) | Rasterize một lần theo DPI rồi cache texture (ADR-0055); không rasterize lại mỗi frame |
| Animation UI | Tween ngắn ≤ 300 ms; preset Thấp rút gọn |

## 10. Ngân sách mạng

| Luồng | Ngân sách | Ghi chú |
|---|---|---|
| Đồng bộ Hub (40 người) | ≤ 30 KB/s xuống, ≤ 3 KB/s lên | Snapshot 10 Hz theo interest grid ([REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md)) |
| Trận/boss | ≤ 10 KB/s | Sự kiện theo câu hỏi/phase |
| Chat | ≤ 2 KB/s trung bình | |
| Mạng tối thiểu chơi được | 4G hoặc Wi-Fi 1 Mbps, RTT ≤ 200 ms | Trên 300 ms: hiển thị chỉ báo mạng yếu |
| Tải nội dung vùng (PCK) | Tải nền khi đang ở vùng trước; tiếp tục được khi rớt mạng | §11 |

### 10.1. Ngân sách phòng máy (classroom budget) [Confirmed]

Ngân sách trên tính cho **một** học sinh. Phòng máy trường học dùng chung một đường truyền: 40 máy cùng tải gói vùng 40 MB đầu tiết là khoảng 1,6 GB, vượt khả năng của đường truyền trường phổ thông. Release 1.0 thiết kế theo các quy tắc sau (nguồn: [EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) CLS-03; NFR-PERF-019 trong [PRODUCT_REQUIREMENTS](../00-overview/PRODUCT_REQUIREMENTS.md)):

| Hạng mục | Mục tiêu / quy tắc | Ghi chú |
|---|---|---|
| Đường truyền tham chiếu của phòng máy | 20 Mbps chia sẻ cho 40 máy | Giá trị thiết kế; đo thực tế ở trường pilot (OQ-04) |
| Lưu lượng nền khi cả lớp đang chơi (sau khi đã có nội dung) | ≤ 40 × (Hub + chat) theo bảng §10, tức ≤ 1,3 MB/s xuống cho cả phòng | Không tải PCK/media trong lúc cả lớp đang chơi nếu đã có gói cài sẵn |
| Tải đầu tiết khi máy chưa có nội dung | Không phụ thuộc tải 40 MB/máy cùng lúc: ưu tiên gói cài sẵn hoặc cache LAN | Không đạt được thì game vẫn vào được Hub với nội dung lõi trong bản cài |
| **Gói nội dung cài sẵn** | Bản Android cho trường (MDM/USB) kèm PCK các vùng theo khối của trường; cập nhật delta theo hash | Không có bộ cài Windows (ADR-0101); bản Web (PC phòng máy) dùng cache trình duyệt theo hash (§14) |
| **Tải trước ngoài giờ** | Thiết bị Android dùng chung (`shared_device: true`) tải PCK/media mới trong khung giờ trường cấu hình (mặc định 18:00–06:00) khi máy bật | Tôn trọng khung giờ truy cập của trường; không đăng nhập học sinh |
| **Cache LAN tùy chọn theo hash** | Trường có thể đặt một máy chủ cache HTTP trong LAN (reverse proxy) cho CDN nội dung; client Android thử `lan_cache_url` trong `device.cfg` trước, kiểm tra SHA-256 của mọi gói (khóa content-addressed, ADR-0044), lỗi thì về CDN; bản Web không đọc `device.cfg` — cách bản Web trên PC phòng máy dùng cache LAN là [TBD] (câu hỏi mở theo ADR-0101) | Chỉ nội dung công khai (PCK, media công khai); không cache API, câu hỏi đề, answer key hay media private |
| Chế độ dữ liệu thấp | Tắt tải trước vùng kế tiếp, giảm chất lượng media video/âm thanh | Bật tự động khi đo băng thông thấp hoặc do trường bật |
| Đo đạc | Telemetry không PII: dung lượng tải theo nguồn (gói cài sẵn/LAN/CDN), tỷ lệ cache LAN hit | Dashboard trong [MONITORING](../11-operations/MONITORING.md) |

Tham số `classroom_reference_bandwidth` (đường truyền tham chiếu), `offhours_prefetch_window` (khung giờ tải trước), `lan_cache_url` là [Confirmed — Technical Architect 2026-11-15], đăng ký ở [CONFIG_DEFAULTS](../00-overview/CONFIG_DEFAULTS.md). Phân phối bản Android cho trường (MDM/USB): [DEPLOYMENT](../11-operations/DEPLOYMENT.md).

## 11. Thời gian tải và dung lượng

| Chỉ số | Mục tiêu (Tầm trung) |
|---|---|
| Khởi động lạnh đến Hub (đã đăng nhập, nội dung đã cache) | ≤ 20 giây |
| Chuyển vùng (vùng đã tải) | ≤ 5 giây |
| Mở activity player | ≤ 2 giây (media đã preload) |
| Bản cài Android (APK/AAB base) | ≤ 300 MB |
| Bản cài iOS | ≤ 300 MB |
| Web tải đầu (wasm + pck lõi, đã nén) | ≤ 40 MB; phần còn lại tải theo vùng |
| Gói nội dung mỗi vùng (PCK) | ≤ 40 MB nén |
| Bộ nhớ cache nội dung trên thiết bị | ≤ 1,5 GB, xóa LRU theo vùng ít dùng |

- Nội dung vùng tải **theo nhu cầu** qua CDN với hash bất biến ([WORLD_RENDERING](WORLD_RENDERING.md), [DEPLOYMENT](../11-operations/DEPLOYMENT.md)); vùng chưa tải hiển thị trạng thái "Đang chuẩn bị vùng đất" có tiến độ.
- Shader được biên dịch trước ở màn hình tải (warm-up) để tránh giật lần đầu.

## 12. Pin và nhiệt

- Mobile khóa **30 FPS** mặc định (§3); không cho 60 FPS ở preset Trung bình.
- Ứng dụng xuống nền: dừng render, giảm mạng về heartbeat; Web tab ẩn: `requestAnimationFrame` tự dừng.
- Màn đứng yên (đọc bài, chờ trong lobby) > 10 giây: giảm render 3D xuống 15 FPS.
- Mục tiêu tiêu thụ pin trên DEV-AND-MID: ≤ 15%/giờ chơi ở preset Trung bình [Confirmed]; nhiệt độ không kích hoạt throttle mức "severe" trong phiên 30 phút.

## 13. Kỹ thuật tối ưu bắt buộc

| Kỹ thuật | Bắt buộc ở |
|---|---|
| Occlusion culling (`OccluderInstance3D` bake) | Mọi zone có tòa nhà/tường |
| LOD (mesh LOD tự động + `visibility_range`) | Mọi mesh > 1k tam giác |
| `MultiMeshInstance3D` / instancing | Mọi vật thể lặp ≥ 10 bản |
| Bake lightmap (`LightmapGI`), không GI realtime (SDFGI/VoxelGI cấm) | Mọi zone tĩnh |
| Impostor/billboard nhân vật xa | Hub, vùng đông |
| Object pooling (người chơi từ xa, hạt, UI item, projectile hiệu ứng) | Toàn client |
| Streaming vùng (scene + PCK theo vùng, giải phóng vùng cũ) | Chuyển vùng |
| Gộp collider tĩnh, không physics cho props | Mọi zone |
| Không SSAO/SSR/volumetric fog trên Mobile/Compatibility | Preset Thấp/Trung bình |
| Precompile shader tại màn tải | Mọi bản build |

## 14. Web export

- Renderer **Compatibility** (WebGL 2), export **single-threaded** (không cần COOP/COEP) [Confirmed].
- Preset tối đa: Trung bình; không Forward+.
- Giới hạn bộ nhớ heap wasm khởi tạo 512 MB, tối đa 1 GB; không preload toàn bộ vùng.
- Tải theo gói: lõi ≤ 40 MB (§11), vùng tải sau; dùng cache trình duyệt + Cache Storage theo hash.
- Âm thanh khởi động sau tương tác đầu của người dùng (chính sách trình duyệt).
- Kiểm thử trên DEV-WEB-MID với Chrome và Edge bản mới (bắt buộc). Safari/Firefox không được hỗ trợ trong Release 1.0 (không kiểm thử, lỗi riêng không chặn release); không chặn truy cập, trình duyệt thiếu WebGL 2 nhận trang thông báo (F133) [Confirmed — owner 2026-11-20, OQ-04].

## 15. Quy trình kiểm soát

1. **Ngân sách trong merge request:** asset mới ghi số tam giác/texture/xương; zone mới ghi bảng ngân sách cảnh (§7).
2. **CI — kiểm tra import:** script Godot headless đọc asset đã import, chặn khi vượt §8.
3. **CI — cảnh benchmark headless:** mỗi zone có scene benchmark (camera bay theo path, spawn số người chơi giả theo §7). Chạy headless để đo **frame time CPU** (script, physics, animation) và đếm **draw call, tam giác, bộ nhớ** qua `RenderingServer.get_rendering_info()`/`Performance`; so với ngân sách, **chặn merge** khi vượt > 5%. (Frame time GPU chỉ đo được trên thiết bị thật.)
4. **Thiết bị thật mỗi milestone:** chạy bộ kịch bản hiệu năng (TC-PERF client trong [TEST_STRATEGY](../12-project/TEST_STRATEGY.md)) trên toàn bộ lab §2.1; ghi FPS p10, frame time GPU, RAM, nhiệt, pin; dùng Godot profiler + công cụ GPU của hãng.
5. **Telemetry thực tế:** client gửi FPS p10, renderer, preset, model thiết bị (không PII) qua `/game/telemetry`; dashboard theo hạng thiết bị trong [MONITORING](../11-operations/MONITORING.md).
6. **Báo cáo:** mỗi milestone có báo cáo hiệu năng (bảng thiết bị × cảnh × chỉ số), vượt ngân sách phải có kế hoạch xử lý trước khi qua milestone.

## 16. Tiêu chí nghiệm thu

| ID | Tiêu chí |
|---|---|
| AC-PERF-001 | Trên DEV-AND-MID1 và DEV-AND-MID2, preset Trung bình, Hub với 40 người chơi: FPS p10 ≥ 27, không khung > 100 ms quá 1 lần/phút trong 10 phút |
| AC-PERF-002 | ~~Trên DEV-PC-MID (bản native), preset Trung bình: Hub ≥ 50 FPS p10; boss co-op ≥ 30 FPS p10~~ — **không làm** (ADR-0101, 2026-11-24: bỏ bản Windows; PC chạy bản Web theo AC-PERF-003); mã giữ để truy vết, không tái sử dụng |
| AC-PERF-003 | Trên DEV-WEB-MID (Chrome, Edge): Hub ≥ 27 FPS p10; tải đầu ≤ 40 MB; vào Hub ≤ 30 giây trên mạng 20 Mbps |
| AC-PERF-004 | Trên DEV-PC-MIN (bản Web, Chrome/Edge) và DEV-AND-MIN, preset Thấp: chơi hết luồng quest mẫu (đăng nhập → quest → activity → nhận thưởng) ≥ 30 FPS p10 ở Private instance và ≥ 24 FPS p10 ở Hub |
| AC-PERF-005 | Khởi động lạnh đến Hub ≤ 20 giây và chuyển vùng ≤ 5 giây trên DEV-AND-MID1 |
| AC-PERF-006 | RAM tiến trình không vượt §7 sau 60 phút chơi liên tục (không rò rỉ > 5%) |
| AC-PERF-007 | Tự phát hiện chọn đúng preset theo hạng cho toàn bộ lab §2.1 |
| AC-PERF-008 | Renderer Mobile lỗi khởi tạo → lần chạy sau tự dùng Compatibility và vẫn chơi được |
| AC-PERF-009 | Đồng bộ Hub 40 người ≤ 30 KB/s xuống trong 10 phút đo |
| AC-PERF-010 | Mọi zone trong bản build qua benchmark CI không vượt ngân sách §7 quá 5% |
| AC-PERF-011 | DEV-AND-MID1 chơi 30 phút: pin giảm ≤ 8%, không throttle mức severe |
| AC-PERF-012 | Phòng máy giả lập 40 client trên đường truyền 20 Mbps: với gói nội dung cài sẵn hoặc cache LAN, cả lớp vào Hub ≤ 2 phút; client kiểm tra SHA-256 và tự về CDN khi cache LAN lỗi [Confirmed] |
