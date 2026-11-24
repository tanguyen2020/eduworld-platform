# ADR-0057 — Chạy được trên máy tầm trung: hạng thiết bị và ngân sách hiệu năng

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** OQ-04 · **Tài liệu:** [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) · [GODOT_ARCHITECTURE](../08-game-client/GODOT_ARCHITECTURE.md) · [WORLD_RENDERING](../08-game-client/WORLD_RENDERING.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Stakeholder yêu cầu: game **đặc biệt phải chạy được trên máy tầm trung**, trên cả web và mobile. Học sinh dùng phòng máy trường, laptop gia đình, điện thoại/tablet Android phổ thông. Thay thế mục ngân sách hiệu năng rải rác trong ADR-0013.

## Quyết định

1. **Yêu cầu [Confirmed]:** mọi tính năng của Release 1.0 phải chơi được ở **cấu hình tham chiếu tầm trung** dưới đây. Tính năng nào không đạt ngân sách thì phải có phương án giảm chất lượng, không được loại thiết bị tầm trung.
2. **Hạng thiết bị tham chiếu [Confirmed, chốt khi khảo sát OQ-04]:**

   | Hạng | PC Windows | Android | iOS | Web |
   |---|---|---|---|---|
   | **Tối thiểu** (vẫn chơi được, preset Thấp) | CPU 4 nhân ~2 GHz, RAM 4 GB, GPU tích hợp hỗ trợ OpenGL 3.3 | RAM 3 GB, Android 9, GPU hỗ trợ OpenGL ES 3.0 | — | Trình duyệt có WebGL 2 trên PC tối thiểu |
   | **Tầm trung — mục tiêu chính** (preset Trung bình) | Intel Core i3 thế hệ 10 / Ryzen 3 3200G, RAM 8 GB, iGPU Intel UHD 630 / Radeon Vega 8 | RAM 4 GB, SoC tầm Snapdragon 680 / Helio G85, GPU Mali-G52 / Adreno 610 | iPhone 11 / iPad thế hệ 8, iOS 15 | Chrome/Edge mới trên PC tầm trung |
   | **Cao** (preset Cao) | GPU rời tầm GTX 1650 | RAM ≥ 6 GB, SoC cận cao cấp | iPhone 13 trở lên | — |

3. **Mục tiêu khung hình:** tầm trung **ổn định 30 FPS** trên mobile và web, **60 FPS** trên PC tầm trung (cho phép giảm còn 30 ở cảnh đông); tối thiểu 30 FPS ở preset Thấp. Mobile khóa 30 FPS mặc định để giảm nóng máy và hao pin.
4. **Renderer Godot:**
   - Mặc định renderer **Mobile** trên PC/Android/iOS; **Compatibility** (OpenGL/WebGL 2) cho Web và máy Tối thiểu (tự chuyển khi không có Vulkan/D3D12).
   - **Forward+ chỉ dùng ở preset Cao** và cho render thumbnail (ADR-0047). Art direction stylized phải trông tốt trên Compatibility.
5. **Preset chất lượng** Thấp / Trung bình / Cao + **tự phát hiện** khi chạy lần đầu (benchmark 5 giây) + **độ phân giải động** (render scale 0,6–1,0 giữ FPS mục tiêu). Học sinh đổi được trong Cài đặt.
6. **Ngân sách tầm trung, preset Trung bình, cảnh Hub đông nhất:**

   | Hạng mục | Mobile/Web | PC |
   |---|---|---|
   | Tam giác hiển thị | ≤ 150k | ≤ 300k |
   | Draw call | ≤ 100 | ≤ 200 |
   | Bộ nhớ texture | ≤ 256 MB | ≤ 512 MB |
   | RAM toàn tiến trình | ≤ 1 GB | ≤ 1,5 GB |
   | Nhân vật render đầy đủ | ≤ 20; còn lại LOD thấp/impostor | ≤ 30 |
   | Ánh sáng | 1 directional, shadow 1 cascade độ phân giải thấp; lightmap bake; không GI realtime | 2 cascade |
   | Hậu kỳ | Tắt (chỉ tone mapping) | Bloom nhẹ |

7. **Thời gian và dung lượng:** khởi động lạnh đến Hub ≤ 20 giây; chuyển vùng ≤ 5 giây; bản cài Android ≤ 300 MB, nội dung vùng tải theo nhu cầu (PCK); Web tải đầu ≤ 40 MB; mạng 4G/1 Mbps vẫn chơi được, đồng bộ hub ≤ 30 KB/s.
8. **UI 2D:** giới hạn số Control động, không vẽ lại toàn màn mỗi frame; công thức là SVG cache (ADR-0055).
9. **Quy trình kiểm soát:** mỗi cảnh/asset có ngân sách ghi trong PERFORMANCE_BUDGET; CI chạy cảnh benchmark headless đo frame time CPU và đếm draw call/tam giác, chặn merge khi vượt; **thiết bị tham chiếu thật** (ít nhất 1 PC iGPU, 2 Android tầm trung, 1 iPhone 11) chạy kiểm thử hiệu năng mỗi milestone.

## Hệ quả

(+) Tiếp cận được đa số học sinh; ngân sách rõ giúp art/dev tự kiểm soát. (−) Hạn chế hiệu ứng đồ họa; cần quy trình tối ưu asset, LOD, bake ánh sáng; cần mua thiết bị tham chiếu.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR; thay phần ngân sách hiệu năng của ADR-0013 | Stakeholder |
| 2026-11-20 | Owner chốt thông số (Accepted toàn bộ) | Owner |
