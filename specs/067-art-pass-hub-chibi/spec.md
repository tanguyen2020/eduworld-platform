# Feature Specification: F067 — Art pass 1: Main Hub low-poly stylized và nhân vật chibi (asset MIT tự làm)

**Feature Branch**: `067-art-pass-hub-chibi` · **Created**: 2026-11-24 · **Status**: Done · **Module**: `src/game/` (+ `design/game-tokens`) · **Mốc**: M2 · **Phụ thuộc**: F062, F070
**Input**: Owner 2026-11-24: "đảm bảo game phải đẹp và đồ họa phải đúng với mô tả", nguồn asset "làm theo hướng MIT License" ([ADR-0102](../../docs/adr/ADR-0102-nguon-asset-do-hoa-theo-license-mit-hoac-de-dai-hon.md)). Thay greybox của Quảng trường Tia Sáng và thân nhân vật bằng đồ họa stylized low-poly, toon + viền mềm, cảm hứng Việt Nam, đúng [WORLD_DESIGN §4.1, §11](../../docs/01-game-design/WORLD_DESIGN.md#11-art-direction). FR nguồn: FR-GAME-001, FR-GAME-002, FR-GAME-003, FR-GAME-004, FR-GAME-009, FR-GAME-015; NFR-COMP-001, NFR-COMP-004, NFR-MAINT-007.

## Clarifications

### Session 2026-11-24

- Q: Owner nói "hướng MIT License" — cụ thể license nào được dùng? → A: [Confirmed — owner 2026-11-24, cách hiểu ghi ở ADR-0102] MIT, CC0-1.0, public domain; cấm share-alike, non-commercial, no-derivatives, họ GPL và pack trả phí; asset tự làm phát hành MIT. CC-BY (ghi công, không share-alike) chưa được phép — Câu hỏi mở F067-1.
- Q: Có tải pack CC0/MIT về không? → A: [Confirmed] Không tải trong phiên agent (phân quyền chặn tải); lượt này toàn bộ asset **tự làm bằng code** (mesh low-poly sinh trong `tools/art/`, bake ra `res://assets/models/**`); URL pack đề xuất ghi ở quickstart §6 để owner tự tải sau.
- Q: Màu của mesh lấy từ đâu để đúng R32? → A: [Confirmed] Bảng màu mỹ thuật là token `art.palette.<tên>` (`design/game-tokens/art3d.json`, alias `palette.art.*`), có chỉ số slot cố định (`eduworld.index`, chỉ thêm, không đánh số lại). Mesh bake chỉ lưu slot (UV.x = slot + 0,5), không lưu màu; preset vật liệu toon sinh từ token mang mảng màu. Da, màu tóc, màu đồ của avatar là `albedo` theo từng avatar từ token (`avatar.*`, `art.palette.*`).
- Q: Toon/outline cụ thể thế nào? → A: [Confirmed] Một shader toon chung (`toon_steps` dải sáng mềm về `shadow_tint`, rim nhẹ, không specular) + viền inverted hull làm `next_pass` (độ dày `outline.width`, màu `outline.color`, pháp tuyến làm mượt lưu trong COLOR để viền không nứt); trang trí dày (cỏ, hoa, lúa) dùng preset không viền. Ánh sáng: nắng ấm `light.sun_*`, ambient phẳng màu ấm `light.ambient_*`, sương nhẹ `light.fog_density`, tonemap linear — giống nhau trên Mobile và Compatibility.
- Q: Renderer Compatibility (Web) cộng ánh nắng hai lần khi DirectionalLight3D bật bóng với shader `light()` tùy biến — xử lý ra sao? → A: [Confirmed — giữ renderer parity, PERFORMANCE_BUDGET §4] Trên Compatibility tắt bóng thời gian thực của mặt trời (`QualityPolicy.shadows_supported`); Mobile giữ bóng theo preset. Blob shadow cho nhân vật trên Web là việc sau (Câu hỏi mở F067-3).
- Q: Preset ảnh hưởng gì tới đồ họa? → A: [Proposed — PERFORMANCE_BUDGET §5, CONFIG_DEFAULTS `quality_presets`] Viền toon mờ dần và tắt từ 12 m (Thấp) / 30 m (Trung bình) / không tắt (Cao); mật độ cỏ, hoa, lúa, bụi 25 % / 60 % / 100 %, trong tầm nhìn 40 / 70 / 120 m của preset. Con số 12 m của "Thấp" là đề xuất (bảng chỉ ghi "tắt ở vật thể xa").
- Q: Hall (đình) của F062 đè lên cổng gió đi Đại dương (z = -20) — giữ vị trí cũ? → A: [Confirmed] Không: đình lùi ra sau hàng cổng gió (tâm z = -26,2, mặt trước z = -22,4) để trigger cổng (bán kính 1,6 m) tới được; mọi spawn point, interactable, portal và id giữ nguyên.

**Câu hỏi mở — chốt 2026-11-24 (owner "chốt hết", orchestrator quyết theo hướng đúng chữ owner và an toàn) [Confirmed]:**
- F067-1: CC-BY-4.0 **không** được thêm (owner chọn "hướng MIT"; giữ danh sách ADR-0102 mục 2).
- F067-2: Âm thanh **theo cùng danh sách license** của ADR-0102 (MIT, CC0, public domain; âm thanh tự làm phát hành MIT).
- F067-3: Nhân vật trên Web (Compatibility) dùng **blob shadow** (mesh phẳng mờ dưới chân, màu từ token, tắt khi renderer có bóng thời gian thực) — việc của art pass kế tiếp, không để nhân vật "lơ lửng".
- F067-4: Cây Tia Sáng lớn theo mastery là việc của **art pass 2** (dùng dữ liệu mastery có sẵn qua API game, không thêm logic vào scene — R1), lên kế hoạch cùng art pass các vùng khác.
- Giá trị màu, độ sáng, sương, tonemap và khoảng cách tắt viền 12 m ở preset Thấp: [Confirmed] làm mặc định Release 1; Game Director chỉnh qua token (`design/game-tokens/`), không sửa scene.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Quảng trường Tia Sáng đẹp và đúng mô tả (Priority: P1)

Học sinh vào Main Hub thấy một quảng trường low-poly ấm áp: cây đa Tia Sáng giữa quảng trường, hồ sen có thuyền thúng, đình và nhà mái ngói, cổng làng, ruộng bậc thang hai bên, hồ phía nam, núi Thử Thách phía xa, hàng cổng gió có biểu tượng hình dạng cho từng vùng, bảng tin có mái ngói nhỏ, đèn lồng (WORLD_DESIGN §4.1, §11).

**Why this priority**: Owner yêu cầu trực tiếp; Hub là cảnh đầu tiên mọi học sinh thấy.

**Independent Test**: Chạy `tools/capture_hub.gd` (Mobile và Compatibility), so ảnh với greybox; test scene `test_hub_art.gd`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F067-AC-10 | HỆ THỐNG PHẢI dựng Main Hub với các điểm nhấn cảm hứng Việt Nam của WORLD_DESIGN: cây đa Tia Sáng, nhà/đình mái ngói, ruộng bậc thang, thuyền thúng trên hồ sen, đèn lồng, cổng gió có biểu tượng hình dạng (không chỉ màu) theo vùng đích và bảng tin, bằng mesh tự làm đã bake | FR-GAME-001 · FR-GAME-002 · `WORLD_DESIGN` §4.1, §11 | — |
| F067-AC-01 | HỆ THỐNG PHẢI tô mọi mesh mỹ thuật chỉ bằng slot của bảng màu token `art.palette.*` (hoặc `albedo` theo avatar lấy từ token), không lưu màu trong mesh, và preset vật liệu mang đúng màu của token | NFR-MAINT-007 · `DESIGN_SYSTEM` §5.3 · AGENTS R32 | TC-UI-001 |
| F067-AC-05 | HỆ THỐNG PHẢI vẽ đồ họa bằng shader toon chung (dải sáng `toon.steps`, bóng `toon.shadow_tint`, rim `toon.rim_strength`) và viền mềm inverted hull (`outline.width`, `outline.color`) cho mọi vật thể đặc; trang trí dày dùng preset không viền | FR-GAME-001 · `DESIGN_SYSTEM` §5.3 | — |
| F067-AC-11 | KHI thay greybox bằng đồ họa mới, HỆ THỐNG PHẢI giữ nguyên spawn point, interactable, portal, id và collision mà code F062/F071 dùng, không thêm logic nghiệp vụ vào scene | FR-GAME-003 · AGENTS R1, R2 · `WORLD_RENDERING` §2, §4, §5 | — |
| F067-AC-12 | HỆ THỐNG PHẢI có công cụ chụp Hub từ ít nhất 4 góc máy ra PNG để duyệt trước/sau | `DESIGN_SYSTEM` §8 | TC-UI-007 |

---

### User Story 2 — Nhân vật chibi dễ thương, tùy biến vẫn chạy (Priority: P1)

Nhân vật người chơi và NPC có tỉ lệ chibi (đầu khoảng 1/3 thân), khuôn mặt đơn giản biểu cảm (mắt có điểm sáng, lông mày, miệng cười, má hồng), không vũ khí; tóc, trang phục, phụ kiện, màu da/tóc của F070 vẫn đổi được.

**Why this priority**: Nhân vật là thứ học sinh nhìn nhiều nhất; tạo nhân vật lần đầu (F070) phải khớp đồ họa mới.

**Independent Test**: `test_avatar_builder.gd`, `test_character_creation_scene.gd`, `test_baked_art.gd` (tỉ lệ đầu, khuôn mặt); ảnh `hub_after_characters.png`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F067-AC-08 | HỆ THỐNG PHẢI vẽ nhân vật người chơi và NPC tỉ lệ chibi (chiều cao đầu 0,3–0,45 chiều cao thân, cao 1,2–1,5 m), có mắt, điểm sáng mắt, miệng, má hồng, không vũ khí; KHI học sinh đổi dáng người, màu da, kiểu và màu tóc, trang phục, phụ kiện (F070), HỆ THỐNG PHẢI vẽ lại bằng mesh mới với màu từ token | FR-GAME-004 · FR-GAME-015 · `WORLD_DESIGN` §11 · `CHARACTER_SYSTEM` §5 | — |

---

### User Story 3 — Chạy mượt, đúng license, đúng trên Web (Priority: P1)

Đồ họa mới vẫn trong ngân sách máy tầm trung, chạy trên renderer Compatibility (Web), đổi theo preset Thấp/Trung bình/Cao, và mọi asset có license được phép.

**Why this priority**: NFR-COMP-001 là yêu cầu bắt buộc; ADR-0102 là quyết định owner.

**Independent Test**: `test_main_hub_benchmark.gd`, `test_zone_art_quality.gd`, `test_art_rules.gd`, `lint_art.gd`, chạy `MainHubBenchmark.tscn` với renderer, export Web.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F067-AC-02 | HỆ THỐNG PHẢI giữ cảnh Hub trong `scene_budget_hub` (tam giác, draw call tính cả pass viền, vật thể động) với dung sai `perf_budget_tolerance_ci`, và từng asset trong ngân sách §8 (nhân vật ≤ 8k tam giác gồm tóc/đồ/phụ kiện, NPC ≤ 10k, prop lớn ≤ 3k, cây ≤ 2k, mặt đất ≤ 40k) | NFR-COMP-001 · FR-GAME-009 · `PERFORMANCE_BUDGET` §7, §8 | TC-PERF-018 |
| F067-AC-03 | HỆ THỐNG PHẢI có dòng trong `res://assets/LICENSES.md` (nguồn, tác giả, license, ngày) cho mọi file asset đồ họa; NẾU thiếu dòng hoặc license ngoài MIT/CC0-1.0/Public-Domain (OFL-1.1 chỉ cho font), THÌ `lint_art.gd` PHẢI thất bại | ADR-0102 · `WORLD_RENDERING` §8.1 | — |
| F067-AC-04 | NẾU file nguồn mỹ thuật 3D (scene thế giới, NPC, AvatarRig, vật liệu, item, asset, công thức bake) chứa literal màu, THÌ `lint_art.gd` PHẢI thất bại | NFR-MAINT-007 · AGENTS R32 | TC-UI-003 |
| F067-AC-06 | KHI preset là Thấp/Trung bình/Cao, HỆ THỐNG PHẢI tắt viền toon từ 12 m / 30 m / không tắt và vẽ 25 % / 60 % / 100 % cỏ, hoa, lúa, bụi trong tầm nhìn của preset | FR-GAME-009 · `PERFORMANCE_BUDGET` §5 | — |
| F067-AC-07 | TRONG KHI chạy renderer Compatibility (Web), HỆ THỐNG PHẢI hiển thị cùng màu và dải toon như renderer Mobile (shader chỉ dùng tính năng chung, không bóng thời gian thực của mặt trời) và bản Web PHẢI export được | NFR-COMP-004 · `PERFORMANCE_BUDGET` §4, §14 | TC-UI-007 |
| F067-AC-09 | HỆ THỐNG PHẢI tái tạo được mọi mesh đã bake từ công thức trong repo; NẾU mesh commit lệch công thức, THÌ `bake_art.gd --check` và test PHẢI thất bại | ADR-0102 · `WORLD_RENDERING` §8 | — |

### Edge Cases

- NẾU `target_area_id` của cổng gió không có mesh riêng hoặc không đúng dạng id, THÌ HỆ THỐNG PHẢI dùng cổng mặc định, không dựng đường dẫn tùy ý từ dữ liệu server (F067-AC-10).
- NẾU item của avatar chưa có trên client, THÌ hiển thị item mặc định cùng slot như F070-AC-18 (giữ nguyên).
- NẾU bảng màu token thêm slot mới, THÌ mesh đã bake không phải bake lại (slot cố định, F067-AC-01).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-GAME-001 | Thế giới 3D stylized low-poly, toon + viền mềm, thân thiện | `WORLD_DESIGN` §11 · `DESIGN_SYSTEM` §5.3 |
| FR-GAME-002 | Main Hub (Thị trấn Trung Tâm) có điểm nhấn đúng mô tả | `WORLD_DESIGN` §4.1 |
| FR-GAME-003 | Giữ character controller, interactable, NPC interaction của F062/F071 | `WORLD_RENDERING` §2–§5 |
| FR-GAME-004 | Avatar tùy biến vẽ bằng mesh chibi mới | `CHARACTER_SYSTEM` §5 |
| FR-GAME-009 | Preset Thấp/Trung bình/Cao, MultiMesh, ngân sách | `PERFORMANCE_BUDGET` §5, §7, §8, §13 |
| FR-GAME-015 | Tạo avatar lần đầu (S02) dùng thân chibi mới | `CHARACTER_SYSTEM` §5 |

### Key Entities

- **Bảng màu mỹ thuật** (`art.palette.<tên>`): danh sách màu có slot cố định; sinh vào `Art3dTokens.ART_PALETTE(_COLORS)` và preset `mat_toon*.tres`.
- **Mesh bake** (`res://assets/models/<category>/<category>_<name>_<variant>.res`): mesh low-poly tự làm, chỉ chứa vị trí, pháp tuyến phẳng, pháp tuyến mượt (COLOR, cho viền) và slot màu (UV).
- **ScatterSet** (`env_hub_scatter_<kind>_a.res`): mesh + danh sách transform của vật thể lặp, vẽ bằng MultiMesh.
- **Dòng license** (`assets/LICENSES.md`): glob đường dẫn, nguồn, tác giả, license, ngày.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `quality_presets` | Khoảng cách tắt viền, mật độ trang trí, tầm nhìn theo preset (F067-AC-06) |
| `scene_budget_hub` | Ngân sách tam giác/draw call của Hub (F067-AC-02) |
| `character_asset_budget` | Ngân sách tam giác nhân vật (F067-AC-02) |
| `perf_budget_tolerance_ci` | Dung sai benchmark CI (F067-AC-02) |
| `renderer_default` | Mobile (Android/iOS) và Compatibility (Web) phải giống nhau (F067-AC-07) |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Chạy trên máy tầm trung, Web WebGL 2 | NFR-COMP-001 · NFR-COMP-004 · ADR-0057 · ADR-0101 |
| Không hardcode màu/font/kích thước; file sinh không sửa tay | NFR-MAINT-007 · AGENTS R32 · ADR-0047 |
| License asset MIT/CC0, asset tự làm MIT | ADR-0102 |
| Không logic nghiệp vụ, không nội dung học tập trong scene | AGENTS R1, R2 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Ảnh chụp Hub sau art pass (5 góc, Mobile và Web) thể hiện đủ điểm nhấn của F067-AC-10, khác rõ greybox (quickstart §5).
- **SC-002**: Benchmark Hub ở preset Trung bình: ≤ 100 draw call và ≤ 150k tam giác đo bằng `RenderingServer` trên cả Mobile và Compatibility.
- **SC-003**: `lint_art.gd` 0 vi phạm; GdUnit4 toàn bộ xanh (trừ test timing đã biết khi máy quá tải).

## Ngoài phạm vi

- Rig/skeleton `kid_base`, animation thật, emote 3D (CHARACTER_SYSTEM §5.2) — lượt art sau.
- Đồ họa các vùng khác (môn học, Đại dương, Núi, Đấu trường, Thư viện Gió, Vườn Bạn Bè): vẫn greybox, chỉ hưởng cổng gió, NPC, marker mới.
- Cây Tia Sáng lớn theo mastery (Câu hỏi mở F067-4), lightmap bake, occluder bake, LOD tự động.
- Export preset Web/iOS (F133 T002), đo trên thiết bị lab (F133).
- Âm thanh.

## Assumptions

- Màu, độ sáng, sương, tonemap là mặc định [Confirmed] (owner "chốt hết" 2026-11-24); Game Director chỉnh qua token (DESIGN_SYSTEM §8), không sửa scene.
- Khoảng cách tắt viền 12 m ở preset Thấp là [Confirmed] (2026-11-24).
- Bóng nắng thời gian thực tắt trên Compatibility; nhân vật dùng blob shadow ở art pass kế tiếp (F067-3 đã chốt).
