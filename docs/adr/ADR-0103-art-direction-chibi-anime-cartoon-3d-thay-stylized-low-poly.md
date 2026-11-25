# ADR-0103 — Art direction "Chibi anime cartoon 3D" thay stylized low-poly

> **Trạng thái:** Accepted · thay phần "phong cách stylized low-poly" của OQ-16, dòng "Phong cách" của [WORLD_DESIGN §11](../01-game-design/WORLD_DESIGN.md#11-art-direction) và câu "phong cách stylized low-poly giữ nguyên" ở mục 7 của [ADR-0102](ADR-0102-nguon-asset-do-hoa-theo-license-mit-hoac-de-dai-hon.md) · **Ngày:** 2026-11-25 · **OQ:** OQ-16 · **Tài liệu:** [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) · [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) · [DESIGN_SYSTEM](../05-ux/DESIGN_SYSTEM.md) · [WORLD_RENDERING](../08-game-client/WORLD_RENDERING.md) · [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

OQ-16 (owner 2026-11-20, theo Master Prompt §3.1) chốt phong cách **stylized low-poly**; art pass 1 ([F067](../../specs/067-art-pass-hub-chibi/spec.md), ADR-0102) dựng Main Hub và nhân vật chibi theo hướng đó: mặt phẳng, cạnh cứng, toon mềm 3 dải, nhân vật không có xương (animation chỉ là nhún giữ chỗ).

Ngày 2026-11-25 owner hỏi có áp dụng chung được bốn phong cách sau không: "Stylized 3D Cartoon, 3D Animated Cartoon, Stylized Anime 3D, 3D Chibi". Orchestrator đề xuất gộp thành **một** hướng nhất quán, mỗi phong cách lo một khía cạnh (bảng ở Quyết định); owner xác nhận nguyên văn: **"Chibi anime cartoon 3D"**.

## Phương án

1. **Gộp bốn phong cách thành "Chibi anime cartoon 3D"**, mỗi phong cách một khía cạnh (hình khối, tỉ lệ, shading/mặt, chuyển động).
2. Giữ stylized low-poly như F067.
3. Thế giới low-poly, chỉ nhân vật theo cartoon/anime.
4. Anime với tỉ lệ người thật (chân dài, cao).

## Quyết định

1. **Art direction của game là "Chibi anime cartoon 3D"** [Confirmed — owner 2026-11-25]. Bốn phong cách áp dụng cùng lúc, mỗi phong cách cho một khía cạnh:

   | Phong cách | Áp dụng cho | Cách làm |
   |---|---|---|
   | Stylized 3D Cartoon | Hình khối thế giới và đạo cụ | Khối tròn, mềm, vát cạnh (bevel), đổ bóng mượt; màu tươi nhưng không chói; không còn mặt phẳng low-poly cứng |
   | 3D Chibi | Tỉ lệ nhân vật | Đầu lớn (khoảng 1/2 đến 1/3 chiều cao), tay chân ngắn, thân tròn |
   | Stylized Anime 3D | Shading và khuôn mặt | Cel shading (các dải sáng/tối rõ, phân bậc), viền mực (ink outline), mắt to có điểm sáng, tóc dạng khối/lọn, má hồng |
   | 3D Animated Cartoon | Chuyển động | Thở khi đứng, đi, chạy nhún, nhảy, vẫy tay, ăn mừng; squash & stretch kiểu hoạt hình; biểu cảm mặt (vui, ngạc nhiên, cố gắng) |

2. **Không cho phép** tỉ lệ anime người thật (chân dài, dáng cao) vì mâu thuẫn với chibi. [Confirmed — owner 2026-11-25]
3. **Quy tắc chi tiết nằm ở tài liệu sở hữu** (R33): hướng tổng và mastery visualization — [WORLD_DESIGN §11](../01-game-design/WORLD_DESIGN.md#11-art-direction); tỉ lệ, khuôn mặt, biểu cảm, rig và bộ clip của nhân vật — [CHARACTER_SYSTEM §5](../01-game-design/CHARACTER_SYSTEM.md); cel shading, viền mực, token mỹ thuật — [DESIGN_SYSTEM §5.3](../05-ux/DESIGN_SYSTEM.md); pipeline bake, rig, blob shadow — [WORLD_RENDERING](../08-game-client/WORLD_RENDERING.md); ngân sách — [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) §7, §8. ADR này chỉ ghi quyết định.
4. **License asset không đổi**: vẫn theo ADR-0102 (chỉ MIT, CC0, public domain; asset tự làm phát hành MIT; mỗi file một dòng `src/game/assets/LICENSES.md`, `lint_art.gd` kiểm). Asset của hướng mới vẫn tự làm bằng code (công thức `src/game/tools/art/`); pack ngoài (nếu cần) chỉ liệt kê URL để owner tự tải.
5. **Lượt dựng hình đầu theo hướng mới** là [F068](../../specs/068-art-pass-2-chibi-anime/spec.md) (art pass 2: Main Hub + nhân vật đầy đủ, Cây Tia Sáng theo cấp, blob shadow); các vùng khác làm ở feature tiếp theo (F093, CODE_PLAN).

## Hệ quả

- (+) Một hướng thống nhất, dễ nhận diện, hợp học sinh lớp 1–9 (dễ thương, ấm, đọc rõ), áp dụng cho mọi vùng về sau.
- (+) Cel shading và viền mực chạy bằng **cùng shader** trên renderer Mobile và Compatibility (Web) — chỉ dùng tính năng chung, giữ renderer parity (PERFORMANCE_BUDGET §4); tham số dải sáng, độ mềm cạnh dải, màu viền là design token (R32).
- (−) **Ngân sách tam giác chặt hơn**: khối tròn và vát cạnh tốn tam giác hơn low-poly. Giữ ngân sách cảnh/asset hiện có (PERFORMANCE_BUDGET §7, §8) bằng cách làm mượt bằng pháp tuyến (không tăng tam giác), chỉ vát cạnh khối đủ dày, giảm chi tiết của vật thể lặp, xa và của NPC, và tắt đổ bóng của mặt đất/vật trang trí nhỏ; benchmark có renderer trên cả hai renderer là cổng.
- (−) Nhân vật cần rig xương và clip thật (thay clip nhún giữ chỗ), mặt theo biểu cảm, hướng dẫn và test mới; Compatibility không có bóng nắng thời gian thực nên nhân vật dùng blob shadow (F067-3).
- (−) Việc phải làm cùng MR: cập nhật WORLD_DESIGN §11, CHARACTER_SYSTEM §5, DESIGN_SYSTEM §1 và §5.3, WORLD_RENDERING, PERFORMANCE_BUDGET, OPEN_QUESTIONS OQ-16, ghi Lịch sử thay đổi ADR-0102; F067 giữ `Done` (lượt trước), ảnh trước/sau ở F068.

## Phương án đã loại

- **Giữ stylized low-poly**: owner muốn phong cách cartoon/anime/chibi; low-poly mặt phẳng cứng không thể hiện được.
- **Thế giới low-poly, chỉ nhân vật cartoon/anime**: hai ngôn ngữ hình khối lệch nhau trong cùng khung hình; mâu thuẫn với "Stylized 3D Cartoon" cho thế giới.
- **Tỉ lệ anime người thật (chân dài, cao)**: mâu thuẫn với 3D Chibi; owner loại.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-25 | Tạo ADR (Accepted) | Owner 2026-11-25: "Chibi anime cartoon 3D" |
