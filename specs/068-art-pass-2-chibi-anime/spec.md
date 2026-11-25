# Feature Specification: F068 — Art pass 2: Chibi anime cartoon 3D cho Main Hub và nhân vật (rig, cel shading, biểu cảm, Cây Tia Sáng theo cấp)

**Feature Branch**: `068-art-pass-2-chibi-anime` · **Created**: 2026-11-25 · **Status**: Done · **Module**: `src/game/` (+ `design/game-tokens`) · **Mốc**: M2 · **Phụ thuộc**: F067, F070
**Input**: Owner 2026-11-25 hỏi áp dụng chung "Stylized 3D Cartoon, 3D Animated Cartoon, Stylized Anime 3D, 3D Chibi"; orchestrator đề xuất gộp, owner xác nhận **"Chibi anime cartoon 3D"** ([ADR-0103](../../docs/adr/ADR-0103-art-direction-chibi-anime-cartoon-3d-thay-stylized-low-poly.md)). Lượt này làm lại Main Hub và nhân vật theo hướng mới trên pipeline F067, khép hai quyết định mở của F067: F067-3 (blob shadow trên Web) và F067-4 (Cây Tia Sáng lớn theo mastery). License asset giữ [ADR-0102](../../docs/adr/ADR-0102-nguon-asset-do-hoa-theo-license-mit-hoac-de-dai-hon.md). FR nguồn: FR-GAME-001, FR-GAME-002, FR-GAME-003, FR-GAME-004, FR-GAME-009, FR-GAME-015; NFR-COMP-001, NFR-COMP-004, NFR-MAINT-007.

## Clarifications

### Session 2026-11-25

- Q: Bốn phong cách áp dụng ra sao để không mâu thuẫn? → A: [Confirmed — owner 2026-11-25, ADR-0103] Mỗi phong cách một khía cạnh: Stylized 3D Cartoon = hình khối thế giới/đạo cụ (tròn, mềm, vát cạnh, màu tươi không chói); 3D Chibi = tỉ lệ nhân vật (đầu 1/2–1/3, tay chân ngắn, thân tròn); Stylized Anime 3D = shading và mặt (cel shading dải rõ, viền mực, mắt to có điểm sáng, tóc lọn, má hồng); 3D Animated Cartoon = chuyển động (thở, đi, chạy nhún, nhảy, vẫy, ăn mừng, squash & stretch, biểu cảm). Cấm tỉ lệ anime người thật.
- Q: Có tải pack ngoài không? → A: [Confirmed] Không (phân quyền chặn tải). Toàn bộ asset tự làm bằng code trong `src/game/tools/art/`, MIT; URL pack CC0 đề xuất ghi ở quickstart §6 để owner tự tải sau.
- Q: Tròn hơn thì có vỡ ngân sách tam giác (PERFORMANCE_BUDGET §7, §8) không? → A: [Confirmed] Giữ nguyên ngân sách. Độ tròn chủ yếu từ pháp tuyến mượt trong góc 62° (không thêm tam giác); chỉ vát cạnh khối đủ dày (≥ 0,035 m cho nhân vật, ≥ 0,15 m cho Hub); vật thể lặp, xa và NPC giữ bậc chia gốc; mặt đất và bụi cây không đổ bóng. Benchmark có renderer trên Mobile và Compatibility là cổng (F068-AC-10).
- Q: Rig và animation làm thế nào khi không có Blender/pack? → A: [Confirmed] Skeleton `kid_base` 8 xương định nghĩa trong code (`KidSkeleton`), mesh thân và trang phục bake kèm BONES/WEIGHTS, clip bake bằng công thức keyframe (`character_anim.gd`) ra `.res`; tóc/phụ kiện gắn `BoneAttachment3D`. Clip nhún giữ chỗ (R17) của F062/F070 được thay cho rig này; thân không có rig vẫn dùng clip giữ chỗ.
- Q: Biểu cảm đổi bằng gì để không có logic trong scene (R1)? → A: [Confirmed] Mỗi biểu cảm (`neutral`, `happy`, `surprised`, `trying`) là một mesh mặt bake; `AvatarBuilder.set_expression()` đổi mesh; `AnimDriver` chọn biểu cảm theo trạng thái chuyển động đã có (bảng ở CHARACTER_SYSTEM §4.1), emote theo `ItemVisual.expression`. Ánh xạ trạng thái → biểu cảm và nhịp chớp mắt là mặc định [Confirmed] (F068-2, 2026-11-25).
- Q: Viền nhân vật có xương bị lệch khi tay giơ lên (pháp tuyến mượt trong COLOR không đi theo xương)? → A: [Confirmed] Thêm preset sinh từ token `mat_toon_skinned.tres`: viền đẩy theo NORMAL đã skin; mesh tĩnh giữ COLOR.
- Q: Blob shadow hiện khi nào (F067-3)? → A: [Confirmed — F067-3 đã chốt] Khi mặt trời không có bóng thời gian thực: renderer Compatibility (Web) hoặc bóng nắng tắt (preset Thấp, hoặc học sinh tắt bóng). Màu/độ mờ là token `light.blob_shadow_*`.
- Q: Cây Tia Sáng lấy dữ liệu mastery ở đâu (F067-4: "dữ liệu mastery có sẵn qua API game")? → A: [Confirmed 2026-11-25 — xem F068-1 bên dưới] WORLD_DESIGN §4.1 nói cây lớn theo **tổng mastery của khối trong trường**; hiện **không có** API nào trả tổng này cho client (game client chưa gọi `getMyMastery`/`getMyProgress`; `getWorldExploration` không có mastery). Lượt này làm đủ phần hình: 5 cấp mesh, ánh xạ `MasteryLevel` → cấp (`SparkTreeModel`, trung bình làm tròn) và `SparkTree.show_mastery()`; Hub hiện cấp mặc định 3 (`spark_tree_default_stage`) cho tới khi có nguồn dữ liệu. Không thêm logic backend trong feature này.
- Q: Các vùng khác (Vương quốc Số, Rừng Ngôn Ngữ, Đại dương, Núi, Đấu trường…)? → A: [Confirmed] Quá lớn cho một lượt; tách sang **F093** (art pass 3: các vùng còn lại theo ADR-0103), CODE_PLAN. Lượt này làm đầy đủ Hub + nhân vật; các vùng khác hưởng nhân vật, NPC, cổng gió, shader và blob shadow mới.
- Q: Linh vật Mây ở màn đăng nhập S01 có phải sửa không? → A: [Confirmed] Không: Mây (vector 2D, viền đậm, má hồng, mắt tròn) không mâu thuẫn hướng mới; chỉ sửa khi lệch (nhiệm vụ owner).

**Câu hỏi mở — chốt 2026-11-25 (owner "chốt hết") [Confirmed]**

- F068-1: Theo đúng WORLD_DESIGN §4.1 — **operation đọc mới ở module `learning`** trả mức mastery tổng của (trường, khối) của học sinh đang chơi: chỉ một mức tổng hợp, không tên, không xếp hạng, không số liệu từng học sinh; ô dưới `analytics_min_cell_size` (5 học sinh) trả "chưa đủ dữ liệu" và cây giữ cấp mặc định 3. Không dùng mastery riêng của học sinh (sẽ lộ tiến độ cá nhân trên cây dùng chung của Hub). Việc backend + nối `SparkTree.show_mastery()` là **task F068 T040** (follow-up, cần contract mới + test cô lập tenant/trường); tới lúc đó cây hiện cấp 3.
- F068-2: Ánh xạ trạng thái → biểu cảm, độ dài clip, nhịp chớp mắt (3,4 s / 0,12 s) và cấp cây mặc định 3 là mặc định Release 1 [Confirmed]; Game Director chỉnh qua công thức clip/token (`CHARACTER_SYSTEM §4.1`), không sửa scene.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Thế giới tròn mềm, cel shading và viền mực (Priority: P1)

Học sinh vào Quảng trường Tia Sáng thấy khối nhà, đạo cụ, cây và đồi tròn mềm, vát cạnh, đổ bóng thành các dải rõ kiểu anime, viền mực ấm; Web và Mobile giống nhau.

**Why this priority**: Owner chốt hướng mới; Hub là cảnh đầu tiên mọi học sinh thấy.

**Independent Test**: `test_art_pass_2.gd`, `test_baked_art.gd`, `test_hub_art.gd`; ảnh `hub_after_*` / `hub_web_*` so với `hub_before_*`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F068-AC-01 | HỆ THỐNG PHẢI dựng mesh thế giới và đạo cụ bằng khối tròn mềm: pháp tuyến làm mượt giữa các mặt lệch nhau dưới góc gấp, khối đủ dày được vát cạnh, trụ/tiện tối thiểu 8 cạnh; mesh Hub (nhà, đạo cụ, cây, cổng gió, Cây Tia Sáng) có ít nhất 30 % đỉnh đổ bóng mượt | FR-GAME-001 · ADR-0103 · `WORLD_RENDERING` §8.1 | — |
| F068-AC-02 | HỆ THỐNG PHẢI tô bằng một shader cel chung có dải sáng phân bậc (`toon.steps`) với cạnh dải hẹp (`toon.band_softness` ≤ 0,15 một bậc) và viền mực tối (`outline.color`), mọi tham số lấy từ preset sinh từ token, cùng shader trên renderer Mobile và Compatibility | FR-GAME-001 · NFR-COMP-004 · `DESIGN_SYSTEM` §5.3 · AGENTS R32 | TC-UI-001 |

---

### User Story 2 — Nhân vật chibi anime có biểu cảm (Priority: P1)

Nhân vật có đầu to (khoảng một nửa chiều cao), tay chân ngắn, thân tròn, mắt to có điểm sáng, tóc lọn, má hồng; đổi được biểu cảm vui, ngạc nhiên, cố gắng; mọi lựa chọn F070 (dáng, da, tóc, màu tóc, trang phục, phụ kiện) vẫn vẽ đúng.

**Why this priority**: Nhân vật là thứ học sinh nhìn nhiều nhất; S02 và Tủ đồ dùng cùng rig.

**Independent Test**: `test_art_pass_2.gd`, `test_kid_rig.gd`, test F070 (`test_avatar_builder.gd`, `test_character_creation_scene.gd`, `test_customization_studio.gd`); ảnh `avatars_after_*`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F068-AC-03 | HỆ THỐNG PHẢI vẽ nhân vật tỉ lệ chibi: chiều cao đầu bằng 1/3–1/2 chiều cao thân, cao 1,1–1,4 m, tay và chân ngắn hơn chiều cao đầu; không tỉ lệ anime người thật | FR-GAME-004 · ADR-0103 · `CHARACTER_SYSTEM` §4.1 | — |
| F068-AC-04 | HỆ THỐNG PHẢI vẽ khuôn mặt anime theo từng biểu cảm `neutral`, `happy`, `surprised`, `trying` (mắt, miệng, má hồng; mặt `neutral` có lòng trắng, tròng hai tông và điểm sáng); KHI code hiển thị gọi `AvatarBuilder.set_expression(<biểu cảm>)`, HỆ THỐNG PHẢI đổi mesh mặt tương ứng; NẾU giá trị không thuộc danh sách, THÌ HỆ THỐNG PHẢI giữ nguyên mặt và trả về false; TRONG KHI mặt `neutral`, HỆ THỐNG PHẢI chớp mắt theo `avatar_blink` | FR-GAME-004 · `CHARACTER_SYSTEM` §4.1 · AGENTS R1 | — |
| F068-AC-05 | HỆ THỐNG PHẢI dựng avatar trên skeleton `kid_base` khớp `KidSkeleton` (tên, cha, tư thế nghỉ); thân và trang phục skinned theo skeleton đó (tổng trọng số mỗi đỉnh = 1) và vẽ bằng preset `toon_skinned`; tóc, mũ, kính, ba lô, cờ gắn `BoneAttachment3D` của xương đầu/ngực/tay | FR-GAME-003 · FR-GAME-004 · `CHARACTER_SYSTEM` §4.1, §5.2 | — |

---

### User Story 3 — Chuyển động hoạt hình (Priority: P1)

Nhân vật thở khi đứng, đi, chạy nhún, nhảy; emote vẫy tay, ăn mừng, vỗ tay, nhảy múa, cảm ơn, xin lỗi; co giãn kiểu hoạt hình; mặt đổi theo trạng thái.

**Why this priority**: Thay clip nhún giữ chỗ (R17) bằng animation thật; owner yêu cầu "3D Animated Cartoon".

**Independent Test**: `test_art_pass_2.gd` (clip), `test_kid_rig.gd` (driver), `test_player_controller.gd`, `test_emote_wheel.gd`, `test_remote_players.gd`; ảnh `avatars_after_poses_*`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F068-AC-06 | HỆ THỐNG PHẢI có clip `idle`, `walk`, `run` (lặp) và `jump`, `wave`, `celebrate`, `clap`, `dance`, `thank`, `apologize` (một lần) cùng key mọi xương `kid_base`; squash & stretch là scale xương gốc giữ thể tích (x·y·z = 1); `run` co khi chạm đất và giãn khi bay; mỗi emote mặc định trỏ clip đã bake (không còn nhãn giữ chỗ) | FR-GAME-003 · FR-GAME-004 · `CHARACTER_SYSTEM` §4.1, §7 | — |
| F068-AC-07 | KHI trạng thái chuyển động (`AnimDriver`, từ vận tốc và chạm đất — không đọc input) đổi, HỆ THỐNG PHẢI chạy clip tương ứng trên xương và đặt biểu cảm theo `CHARACTER_SYSTEM` §4.1 (`run` → `trying`, `jump` → `surprised`, `idle`/`walk` → `neutral`); KHI phát emote, HỆ THỐNG PHẢI chạy clip của emote với biểu cảm của emote rồi trở về locomotion | FR-GAME-003 · AGENTS R1 · F062-AC-03 · F070-AC-20 | — |

---

### User Story 4 — Bóng nhân vật trên Web và Cây Tia Sáng theo cấp (Priority: P2)

Trên Web nhân vật có bóng mờ dưới chân (không "lơ lửng"); Cây Tia Sáng có 5 cấp hình từ cây non xám đến cây đa nở hoa.

**Why this priority**: Khép hai quyết định mở F067-3, F067-4.

**Independent Test**: `test_blob_shadow.gd`, `test_spark_tree.gd`; ảnh `hub_web_*`, `avatars_after_spark_stages.png`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F068-AC-08 | TRONG KHI mặt trời không có bóng thời gian thực (renderer Compatibility, hoặc bóng nắng tắt theo preset/cài đặt), HỆ THỐNG PHẢI vẽ blob shadow dưới mỗi nhân vật (người chơi, NPC, avatar xem trước) với màu/độ mờ từ token `light.blob_shadow_*`, và PHẢI ẩn nó khi có bóng thời gian thực; với người chơi cục bộ, vệt bóng PHẢI nằm trên mặt đất khi nhảy và nhỏ lại theo độ cao | FR-GAME-009 · NFR-COMP-004 · F067-3 · `WORLD_RENDERING` §7 | — |
| F068-AC-09 | HỆ THỐNG PHẢI có 5 cấp hình Cây Tia Sáng (0 cây non xám, 1 cây non xanh, 2 cây đang lớn có tia sáng, 3 cây đa đầy đủ, 4 cây đa nở hoa); KHI nhận danh sách mức `MasteryLevel`, HỆ THỐNG PHẢI hiện cấp = trung bình các mức làm tròn (bỏ giá trị lạ); NẾU không có dữ liệu, THÌ HỆ THỐNG PHẢI hiện `spark_tree_default_stage`; việc đổi cấp chỉ đổi mesh, không đổi collision hay hợp đồng scene | FR-GAME-002 · F067-4 · `WORLD_DESIGN` §4.1, §11 · AGENTS R1 | — |

---

### User Story 5 — Vẫn nhanh, đúng license, đúng hợp đồng (Priority: P1)

**Why this priority**: NFR-COMP-001 bắt buộc; ADR-0102 là quyết định owner; F062/F070/F071 không được vỡ.

**Independent Test**: `MainHubBenchmark.tscn` (Mobile + Compatibility), `test_main_hub_benchmark.gd`, `test_baked_art.gd`, `lint_art.gd`, `bake_art.gd --check`, export Web.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F068-AC-10 | HỆ THỐNG PHẢI giữ Hub ở preset Trung bình trong `scene_budget_hub` (≤ 150k primitive, ≤ 100 draw call đo bằng `RenderingServer` trên cả Mobile và Compatibility) và từng asset trong `character_asset_budget`/PERFORMANCE_BUDGET §8 (nhân vật ≤ 8k gồm mặt, tóc, đồ, phụ kiện; NPC ≤ 10k; prop lớn và mọi cấp Cây Tia Sáng ≤ 3k; cây lặp ≤ 2k; mặt đất ≤ 40k) | NFR-COMP-001 · FR-GAME-009 · `PERFORMANCE_BUDGET` §7, §8 | TC-PERF-018 |
| F068-AC-11 | HỆ THỐNG PHẢI có dòng license được phép trong `res://assets/LICENSES.md` cho mọi file asset mới (mesh, mặt, clip, shader blob) và không có literal màu trong nguồn mỹ thuật; NẾU vi phạm, THÌ `lint_art.gd` PHẢI thất bại | ADR-0102 · NFR-MAINT-007 · AGENTS R32 | TC-UI-003 |
| F068-AC-12 | KHI thay đồ họa, HỆ THỐNG PHẢI giữ nguyên spawn point, interactable, portal, id, collision của F062/F071 và luồng tùy biến F070 (slot, fallback item, preview S02/Tủ đồ) | FR-GAME-003 · FR-GAME-015 · AGENTS R1, R2 | — |
| F068-AC-13 | HỆ THỐNG PHẢI tái tạo được mọi mesh và clip đã bake từ công thức; NẾU resource commit lệch công thức (vị trí, UV, xương, trọng số, key clip), THÌ `bake_art.gd --check` và test PHẢI thất bại | ADR-0102 · `WORLD_RENDERING` §8.1 | — |
| F068-AC-14 | Bản Web (preset `Web` của F133) PHẢI export được và gói PCK chứa mesh, clip và shader mới | NFR-COMP-004 · `PERFORMANCE_BUDGET` §14 | — |
| F068-AC-15 | HỆ THỐNG PHẢI có công cụ chụp ảnh duyệt Hub (`capture_hub.gd`) và nhân vật (`capture_avatars.gd`: mọi kiểu tóc/trang phục/màu da, phụ kiện, dáng người, biểu cảm, tư thế clip, 5 cấp Cây Tia Sáng) trên cả hai renderer | `DESIGN_SYSTEM` §8 | TC-UI-007 |

### Edge Cases

- NẾU file mặt hoặc clip chưa có trên client, THÌ `AvatarBuilder` giữ mặt hiện tại và `AnimDriver` dùng clip giữ chỗ (không crash) (F068-AC-04, F068-AC-07).
- NẾU item trang phục ngoài (cosmetic bundle sau này) không có trọng số xương, THÌ HỆ THỐNG PHẢI vẽ nó như vật cứng gắn slot (preset `toon`), không gắn skeleton (F068-AC-05).
- NẾU thân không có Skeleton `kid_base` (rig cũ/thử nghiệm), THÌ `AnimDriver` dùng thư viện clip giữ chỗ (F068-AC-07).
- NẾU danh sách mastery rỗng hoặc toàn giá trị lạ, THÌ cây hiện cấp mặc định (F068-AC-09).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-GAME-001 | Thế giới 3D cartoon/chibi anime, khối tròn, cel shading, viền mực | `WORLD_DESIGN` §11 · `DESIGN_SYSTEM` §5.3 |
| FR-GAME-002 | Main Hub; Cây Tia Sáng 5 cấp theo mastery | `WORLD_DESIGN` §4.1, §11 |
| FR-GAME-003 | Animation state machine chạy clip thật trên rig `kid_base` | `CHARACTER_SYSTEM` §4, §4.1 |
| FR-GAME-004 | Avatar tùy biến + emote trên rig mới, biểu cảm | `CHARACTER_SYSTEM` §4.1, §5, §7 |
| FR-GAME-009 | Ngân sách, preset, blob shadow trên Web/preset Thấp | `PERFORMANCE_BUDGET` §4, §5, §7, §8 · `WORLD_RENDERING` §7 |
| FR-GAME-015 | Tạo avatar lần đầu (S02) dùng rig mới | `CHARACTER_SYSTEM` §5 |

### Key Entities

- **KidSkeleton** (`scripts/player/kid_skeleton.gd`): 8 xương, tư thế nghỉ, tâm/bán kính đầu, anchor slot, danh sách biểu cảm, đường dẫn mặt/clip.
- **Mesh mặt** (`assets/models/chr/chr_face_<biểu cảm>_a.res`), **clip** (`anim_kid_<clip>_a.res`), **Cây Tia Sáng** (`assets/models/veg/veg_banyan_spark_s<0–4>_a.res`).
- **SparkTreeModel** / **SparkTree**: ánh xạ `MasteryLevel` → cấp; node đổi mesh.
- **BlobShadow**: vệt bóng dưới nhân vật, vật liệu từ token.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `scene_budget_hub`, `character_asset_budget`, `perf_budget_tolerance_ci` | Ngân sách (F068-AC-10) |
| `quality_presets`, `renderer_default` | Bóng nắng theo preset/renderer → blob shadow (F068-AC-08) |
| `avatar_blink` [Confirmed] | Nhịp chớp mắt (F068-AC-04) |
| `spark_tree_default_stage` [Confirmed] | Cấp cây khi chưa có dữ liệu (F068-AC-09) |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Chạy trên máy tầm trung, Web WebGL 2, renderer parity | NFR-COMP-001 · NFR-COMP-004 · ADR-0057 · ADR-0101 |
| Không hardcode màu; file sinh không sửa tay | NFR-MAINT-007 · AGENTS R32 · ADR-0047 |
| License asset MIT/CC0, asset tự làm MIT | ADR-0102 |
| Không logic nghiệp vụ trong scene; không nội dung học tập trong scene | AGENTS R1, R2 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Ảnh sau art pass (Hub 5 góc Mobile + Web; nhân vật: hàng 6 kiểu tóc/trang phục/màu da, phụ kiện, dáng người, 4 biểu cảm, tư thế 10 clip, 5 cấp cây) đọc rõ là "chibi anime cartoon": dễ thương, ấm, không chói, phù hợp lớp 1–9.
- **SC-002**: Benchmark Hub preset Trung bình ≤ 150k primitive và ≤ 100 draw call trên cả Mobile và Compatibility.
- **SC-003**: GdUnit4 toàn bộ xanh (trừ test timing đã biết khi máy quá tải, chạy lại riêng); mọi lint xanh.

## Ngoài phạm vi

- Đồ họa các vùng khác (môn học, Đại dương, Núi, Đấu trường, Thư viện Gió, Vườn Bạn Bè) → F093.
- Nguồn dữ liệu tổng mastery của khối (Câu hỏi mở F068-1) và mọi logic backend.
- LOD/impostor tự động cho nhân vật, gộp mesh slot thành một mesh skinned (CHARACTER_SYSTEM §5.2), lightmap/occluder bake.
- Âm thanh; đo trên thiết bị lab (F133).

## Assumptions

- Hướng "Chibi anime cartoon 3D" là [Confirmed] (owner 2026-11-25); giá trị màu viền, độ mềm dải, độ mờ bóng là mặc định token, Game Director chỉnh qua `design/game-tokens/`.
- Ánh xạ biểu cảm, độ dài clip, nhịp chớp mắt, cấp cây mặc định là [Confirmed] (F068-1, F068-2, chốt 2026-11-25).
