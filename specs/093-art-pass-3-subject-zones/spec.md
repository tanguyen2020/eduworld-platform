# Feature Specification: F093 — Art pass 3: năm vùng môn học theo "Chibi anime cartoon 3D" (prefab vùng chung, set piece cảm hứng Việt Nam)

**Feature Branch**: `093-art-pass-3-subject-zones` · **Created**: 2026-11-25 · **Status**: Done · **Module**: `src/game/` (+ `design/game-tokens`) · **Mốc**: M3 · **Phụ thuộc**: F068, F071
**Input**: F068 tách đồ họa các vùng còn lại sang F093 ([ADR-0103](../../docs/adr/ADR-0103-art-direction-chibi-anime-cartoon-3d-thay-stylized-low-poly.md) mục 5). Sau art pass 2 chỉ Main Hub có đồ họa; 11 vùng khác vẫn là greybox F071 (mặt phẳng 50 × 50 m xám, điểm neo trôi nổi). Lượt này làm trọn **năm vùng môn học** (khu B — Vương quốc Số, Rừng Ngôn Ngữ khu Tiếng Việt và khu Tiếng Anh, Đảo Khoa Học, Vương quốc Lịch Sử): học sinh vào đây ngay sau chương 0 vì bốn chương môn học mở song song ([WORLD_DESIGN §2.2](../../docs/01-game-design/WORLD_DESIGN.md#22-mạch-truyện-chính-confirmed--người-duyệt-2026-11-15-adr-0056)). Sáu vùng còn lại → **F094** (CODE_PLAN). License asset giữ [ADR-0102](../../docs/adr/ADR-0102-nguon-asset-do-hoa-theo-license-mit-hoac-de-dai-hon.md). FR nguồn: FR-GAME-001, FR-GAME-002, FR-GAME-009, FR-GAME-016, FR-GAME-017; NFR-COMP-001, NFR-COMP-004, NFR-MAINT-007.

## Clarifications

### Session 2026-11-25

- Q: Làm vùng nào trong lượt này? → A: [Confirmed — giao việc của orchestrator 2026-11-25: "nếu quá lớn, ưu tiên vùng học sinh tới trước, làm trọn, phần còn lại ghi follow-up"] Năm vùng môn học (khu B) làm trọn; `practice_zone`, `social_zone`, `quest_zone.discovery_ocean`, `challenge_mountain`, `exam_zone`, `arena` → **F094** (art pass 4) trên cùng pipeline.
- Q: Năm vùng có bố cục điểm neo giống nhau (seed F071), dựng thế nào để môn mới dùng lại? → A: [Confirmed — theo WORLD_DESIGN §4.2 "môn mới dùng prefab vùng chung + cấu hình"] Một **prefab vùng chung** (`tools/art/zone_kit.gd`: nền cỏ hai tông, quảng trường tròn quanh NPC, đường chính spawn → vật sưu tầm → cổng gió, đường ngang tới hai điểm khám phá, bệ lát dưới mỗi điểm neo, vòng rào biên, rải cây theo luật tránh bố cục) + **bộ set piece và bảng màu riêng của từng môn** (`zone_<môn>_art.gd`). Landmark của mỗi vùng đặt sau cổng gió, khung nhìn từ điểm spawn.
- Q: Set piece cụ thể của từng vùng? → A: [Confirmed — owner "chốt hết" 2026-11-25 là mặc định Release 1 (trước đó Proposed); diễn giải mô tả WORLD_DESIGN §2.2, §2.3, §4.2, §11 "cảm hứng Việt Nam trộn kỳ ảo"; Game Director/owner chỉnh qua công thức, không sửa scene] Bảng ở F093-AC-01.
- Q: Bảng màu có đủ không? → A: [Confirmed] Thêm 8 slot `art.palette.*` (`tile_blue`, `tile_blue_dark`, `leaf_autumn`, `bronze`, `bronze_patina`, `metal`, `metal_dark`, `blossom_violet`; slot 52–59) — `tokens@0.11.0`, MINOR; mọi màu vẫn là slot token (R32).
- Q: Ngân sách cảnh của vùng môn học? → A: [Confirmed — PERFORMANCE_BUDGET §7 dòng "Vùng môn học / khám phá"] ≤ 120k primitive, ≤ 90 draw call, ≤ 50 vật thể động trên Mobile và Compatibility, đăng ký `scene_budget_zone` ở CONFIG_DEFAULTS (tham số đã có giá trị ở tài liệu sở hữu, chỉ thêm tên). Để giữ ngân sách: cảnh xa (đồi, mặt đất xa, biển, đảo, núi lửa) bake riêng (`env_<môn>_far_a`) và vẽ bằng preset không viền, không đổ bóng; cây lặp giữ bậc chia gốc; số cây ngoài vòng rào và trang trí nhỏ giới hạn.
- Q: Cây cau của Hub (lá là hai mặt phẳng dính nhau) bị viền mực tô đen khi nhìn từ phía sau — dùng cho Đảo Khoa Học thế nào? → A: [Confirmed] Đảo dùng cây dừa riêng `veg_palm_coconut_a` (lá là khối dẹt kín, viền inverted hull đúng); sửa cây cau/mái chợ của Hub để sau (Câu hỏi mở F093-3).
- Q: Có đổi hợp đồng thế giới F071 không? → A: [Confirmed] Không. Điểm neo lấy **nguyên** từ seed (`gen_greybox_zones.mjs` chỉ sinh lại khối neo giữa hai marker cho scene đã có art); collision mặt đất 50 × 50 m giữ; collision mới (rào biên, nhà, set piece) không chắn spawn, điểm neo và hai lối đi; cầu Vương quốc Số là sàn đi được (convex). Vương quốc Số nay có đủ 4 vật thể quest F072 (`quest.math.fruit_stall`, `quest.math.apple_basket_1–3`) vốn có trong seed nhưng thiếu trong scene greybox vì generator chưa chạy lại sau F072.

**Câu hỏi mở** (đã đóng ở [F094](../094-art-pass-4-remaining-zones/spec.md))

- F093-1: Main Hub vẫn thiếu 3 vật thể quest F072 có trong seed (`quest.hub.lantern_east`, `quest.hub.lantern_west`, `quest.hub.welcome_sign`): sinh lại khối neo của Hub làm `test_interactable.gd` hỏng (người chơi được đưa tới (0, 0, 12) để rời bảng tin, trùng `welcome_sign`). → [Confirmed — owner "chốt hết" 2026-11-25, làm ở F094] Sinh lại khối neo Hub từ seed (`gen_greybox_zones.mjs --check` sạch cho mọi vùng); không dời vật thể seed (seed/spec không nói vị trí là tùy ý); test đổi sang một điểm rời trống khác.
- F093-2: Lịch sử chưa có mã màu môn học (`color.subject.history`) và environment riêng (`env_history`). → [Confirmed — WORLD_DESIGN §11 đã chốt "Lịch sử nâu đỏ", không phải quyết định mới; owner "chốt hết" 2026-11-25, làm ở F094] Thêm `color.subject.history` vào `design/shared-tokens/domain.json` (game + portal), slot `art.palette.subject_history`, `env_history`.
- F093-3: Cây cau (`veg_palm_cau_a`) và các mặt phẳng hai mặt khác của Hub viền đen khi nhìn từ phía sau (viền inverted hull trên mặt dày 0). → [Confirmed — owner "chốt hết" 2026-11-25, làm ở F094 theo cách F093 đã kiểm] Tấm mỏng vẽ bằng preset có viền là khối kín; test chặn mặt hai mặt dày 0.
- Diễn giải set piece Việt Nam (Văn Miếu/Khuê Văn Các, Ngọ Môn, Chùa Một Cột, trống đồng Đông Sơn, cọn nước, bằng lăng, gốm men lam). → [Confirmed — owner "chốt hết" 2026-11-25] Là mặc định Release 1; Game Director chỉnh công thức `tools/art/zone_<môn>_art.gd` rồi bake lại.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Mỗi vùng môn học có hình riêng, đúng mô tả (Priority: P1)

Học sinh đi qua cổng gió từ Quảng trường Tia Sáng vào Vương quốc Số, Rừng Ngôn Ngữ, Đảo Khoa Học hay Vương quốc Lịch Sử và thấy ngay một nơi tròn mềm, ấm, cùng phong cách với Hub, có landmark riêng và các điểm khám phá có hình (cầu, giếng, bia rùa, trống đồng…) thay cho mặt phẳng xám.

**Why this priority**: Bốn chương môn học mở ngay sau chương 0; greybox làm vỡ hướng art đã chốt.

**Independent Test**: `test_subject_zone_art.gd`; ảnh `screenshots/<môn>_after_*` so với `<môn>_before_*`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F093-AC-01 | HỆ THỐNG PHẢI dựng mỗi vùng môn học từ prefab vùng chung và bộ set piece riêng: **Vương quốc Số** — lâu đài Số ngói men xanh, cầu Số cong có hạt đếm (`discovery.math.number_bridge`) trên con suối, giếng làng có mái ngói (`discovery.math.old_well`), cổng Bàn Tính (`collectible.math.shell_abacus`), sạp trái cây + rổ táo (`quest.math.*`), cọn nước, khối đếm có chấm, cây cắt hình khối; **Rừng Ngôn Ngữ — Tiếng Việt** — gác Khuê Văn Các có cửa sổ mặt trời, bia đá trên lưng rùa dưới nhà bia (`discovery.vietnamese.ancient_letter`), Cây Chữ lá thu treo thẻ giấy và đèn lồng (`discovery.vietnamese.word_tree`), hai bút lông khổng lồ trong chậu men lam treo cuộn giấy (`collectible.vietnamese.page_lost`), giếng Thiên Quang, hồ sen, án thư; **Rừng Ngôn Ngữ — Tiếng Anh** — tháp đồng hồ, xưởng robot tròn có bánh răng và ống khói (`discovery.english.robot_workshop`), Hang Vọng trong gò đá có tinh thể tím (`discovery.english.echo_cave`), cổng Bánh Răng (`collectible.english.gear_word`), hòm thư, biển hình bong bóng thoại, tượng robot, cây bằng lăng; **Đảo Khoa Học** — đảo giữa biển (bãi cát, dừa, đảo nhỏ, núi lửa hiền), nhà kính mái vòm + đài thiên văn + pin mặt trời + tua-bin gió, trạm thời tiết (`discovery.science.weather_station`), hồ thủy triều có sao biển (`discovery.science.tide_pool`), cổng Hóa Thạch (`collectible.science.leaf_fossil`), hàng chậu vòng đời cây; **Vương quốc Lịch Sử** — cổng thành lầu Ngũ Phụng mái vàng, trống đồng Đông Sơn (`discovery.history.bronze_drum`), đình bản đồ cổ có quả địa cầu (`discovery.history.old_map`), cổng Đồng Xu lỗ vuông người chơi đi xuyên qua (`collectible.history.ancient_coin`), chùa Một Cột trong hồ sen, đồng hồ cát, cờ đuôi nheo. Mỗi set piece là mesh bake tự làm đặt cạnh điểm neo của nó; cổng gió về Thị trấn hiện biểu tượng ngôi nhà; không vũ khí, không chữ/nội dung học tập trong scene | FR-GAME-001 · FR-GAME-002 · FR-GAME-016 · FR-GAME-017 · ADR-0103 · `WORLD_DESIGN` §2.2, §4.2, §11 · AGENTS R2 | — |
| F093-AC-02 | HỆ THỐNG PHẢI tô mọi mesh của năm vùng bằng preset sinh từ token (`mat_toon*`), mỗi mặt chỉ mang slot `art.palette.*`; mesh đặc có pass viền mực, chỉ trang trí rải (`decor`) và cảnh xa (`Far`, không đổ bóng) được bỏ viền; mesh thế giới đổ bóng mượt (≥ 30 % đỉnh); NẾU nguồn mỹ thuật chứa literal màu, THÌ `lint_art.gd` PHẢI thất bại | FR-GAME-001 · NFR-MAINT-007 · `DESIGN_SYSTEM` §5.3 · AGENTS R32 | TC-UI-001 · TC-UI-003 |

---

### User Story 2 — Vẫn nhanh trên máy tầm trung và trên Web (Priority: P1)

**Why this priority**: NFR-COMP-001 bắt buộc; owner xem bản Web (Compatibility).

**Independent Test**: `test_zone_benchmark.gd`, `ZoneBenchmark.tscn` có renderer (Mobile + Compatibility), `test_subject_zone_art.gd` (ngân sách asset), export Web.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F093-AC-03 | HỆ THỐNG PHẢI giữ mỗi vùng môn học ở preset Trung bình trong `scene_budget_zone` (≤ 120k primitive, ≤ 90 draw call, ≤ 50 vật thể động) đo bằng `RenderingServer` trên **cả** renderer Mobile và Compatibility (benchmark có renderer `ZoneBenchmark.tscn`, mỗi vùng một lần chạy), và proxy tĩnh headless trong CI không vượt ngân sách quá `perf_budget_tolerance_ci` | NFR-COMP-001 · FR-GAME-009 · `PERFORMANCE_BUDGET` §7, §15 | TC-PERF-018 |
| F093-AC-04 | HỆ THỐNG PHẢI giữ từng asset vùng trong PERFORMANCE_BUDGET §8 (mặt đất + cảnh xa ≤ 40k mỗi vùng, tòa nhà/bộ landmark ≤ 10k, set piece ≤ 3k, cây lặp ≤ 2k) và tái tạo được mọi mesh/`ScatterSet` từ công thức; NẾU resource commit lệch công thức, THÌ `bake_art.gd --check` và test PHẢI thất bại | NFR-COMP-001 · ADR-0102 · `PERFORMANCE_BUDGET` §8 · `WORLD_RENDERING` §8.1 | — |
| F093-AC-07 | Bản Web (preset `Web` của F133) PHẢI export được và gói PCK chứa scene, mesh và `ScatterSet` của năm vùng | NFR-COMP-004 · `PERFORMANCE_BUDGET` §14 | — |

---

### User Story 3 — Đúng hợp đồng thế giới, đúng license (Priority: P1)

**Why this priority**: Server và client cùng một bố cục (F071, ADR-0016); quyết định license của owner (ADR-0102).

**Independent Test**: `test_subject_zone_art.gd`, `zone_lint.gd`, `gen_greybox_zones.mjs --check`, `lint_art.gd`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F093-AC-05 | KHI thay đồ họa một vùng, HỆ THỐNG PHẢI giữ điểm neo bằng đúng seed thế giới (id, loại, vị trí của spawn point, interactable, cổng và vùng đích), giữ collision mặt đất của F071, và collision mới KHÔNG ĐƯỢC chắn spawn, điểm neo và hai lối đi (chính, ngang); `gen_greybox_zones.mjs` PHẢI chỉ sinh lại khối neo giữa hai marker của scene đã có art | FR-GAME-017 · F071 · AGENTS R1, R2 · `WORLD_RENDERING` §2, §10 | — |
| F093-AC-06 | HỆ THỐNG PHẢI có dòng license được phép (MIT) trong `res://assets/LICENSES.md` cho mọi file asset mới của các vùng; NẾU thiếu dòng hoặc license ngoài danh sách, THÌ `lint_art.gd` PHẢI thất bại | ADR-0102 · `WORLD_RENDERING` §8.1 | TC-UI-003 |
| F093-AC-08 | HỆ THỐNG PHẢI có công cụ chụp ảnh duyệt vùng (`capture_zone.gd --area=…`: 5 góc cố định của bố cục vùng môn học) chạy trên cả renderer Mobile và Compatibility, dùng cho ảnh trước/sau | `DESIGN_SYSTEM` §8 · NFR-COMP-004 | TC-UI-007 |

### Edge Cases

- NẾU scene vùng chưa có marker khối neo (vùng greybox của F094), THÌ `gen_greybox_zones.mjs` sinh lại greybox như F071 (F093-AC-05).
- NẾU một môn mới được bật (WORLD_DESIGN §4.2), THÌ vùng của nó dùng lại prefab chung với bộ set piece/màu riêng; điểm neo vẫn sinh từ seed (F093-AC-01, F093-AC-05).
- NẾU renderer là Compatibility (Web, không có bóng nắng thời gian thực), THÌ nhân vật dùng blob shadow F068; cảnh xa không đổ bóng ở mọi renderer (F093-AC-02, F093-AC-03).
- NẾU preset Thấp, THÌ `ZoneArtQuality` giảm mật độ trang trí và khoảng nhìn như ở Hub (F093-AC-03).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-GAME-001 | Năm vùng môn học theo "Chibi anime cartoon 3D": khối tròn, cel shading, viền mực | `WORLD_DESIGN` §11 · `DESIGN_SYSTEM` §5.3 |
| FR-GAME-002 | Đồ họa các vùng đất khu B (Learning Zone theo môn) | `WORLD_DESIGN` §3, §4.2 |
| FR-GAME-016 | Landmark và set piece gắn chương truyện của từng vùng (sửa cầu, chia hàng; thư cổ; xưởng robot; thời tiết, vòng đời; bản đồ cổ, đồng hồ cát) | `WORLD_DESIGN` §2.2 |
| FR-GAME-017 | Điểm khám phá, vật sưu tầm, cổng gió về Thị trấn có hình đọc được | `WORLD_DESIGN` §6, §8 |
| FR-GAME-009 | Ngân sách vùng trên Mobile/Compatibility, preset chất lượng | `PERFORMANCE_BUDGET` §5, §7, §8 · `WORLD_RENDERING` §6.1 |

### Key Entities

- **Prefab vùng chung** (`tools/art/zone_kit.gd`): bố cục điểm neo của seed, `terrain()` (gần, có viền), `far()` (cảnh xa, không viền/bóng), `boundary()`, `house()`, `lamp()`, `scatter()` (luật tránh bố cục).
- **Công thức vùng** (`tools/art/zone_<môn>_art.gd`): `meshes()`, `SCATTER`, `placements()`; đăng ký ở `ArtCatalog.ZONES`.
- **Asset vùng** (`assets/models/{env,bld,prop,veg}/…_<môn>_…_a.res`, `env_<môn>_scatter_<loại>_a.res`).
- **Scene vùng** (`scenes/world/zones/learning_zone_<môn>/Zone.tscn`): phần tay (art, collision) trên marker + khối neo sinh từ seed.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `scene_budget_zone` [Confirmed] | Ngân sách cảnh vùng môn học / khám phá (F093-AC-03) |
| `character_asset_budget`, `perf_budget_tolerance_ci` | Ngân sách asset, dung sai CI (F093-AC-03, F093-AC-04) |
| `quality_presets`, `renderer_default` | Mật độ trang trí, khoảng nhìn, bóng (F093-AC-03) |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Máy tầm trung, Web WebGL 2, renderer parity | NFR-COMP-001 · NFR-COMP-004 · ADR-0057 · ADR-0101 |
| Không hardcode màu; file sinh không sửa tay | NFR-MAINT-007 · AGENTS R32 · ADR-0047 |
| License asset MIT/CC0, asset tự làm MIT | ADR-0102 |
| Không logic nghiệp vụ, không nội dung học tập trong scene | AGENTS R1, R2 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Ảnh sau art pass của năm vùng (5 góc Mobile + 3 góc Web mỗi vùng) đọc rõ là "chibi anime cartoon", cùng phong cách với Hub, mỗi vùng nhận ra được qua landmark và màu môn.
- **SC-002**: Benchmark có renderer mỗi vùng ≤ 120k primitive và ≤ 90 draw call trên cả Mobile và Compatibility.
- **SC-003**: GdUnit4 toàn bộ xanh; mọi lint xanh; export Web thành công.

## Ngoài phạm vi

- Đồ họa `practice_zone`, `social_zone`, `quest_zone.discovery_ocean`, `challenge_mountain`, `exam_zone`, `arena` → **F094**.
- Trạm theo Topic hiển thị mastery bằng màu (WORLD_DESIGN §4.2 — dữ liệu curriculum, chưa có ở client) và "đất xám → có màu" theo mastery của vùng.
- Bundle PCK theo vùng (WORLD_RENDERING §8.2), lightmap/occluder bake, NavMesh, âm thanh vùng.
- Sửa Hub (Câu hỏi mở F093-1, F093-3); màu môn Lịch sử dùng chung (F093-2) → đã làm ở F094.

## Assumptions

- Diễn giải set piece Việt Nam (Văn Miếu, Ngọ Môn, Chùa Một Cột, trống đồng Đông Sơn, cọn nước, bằng lăng, gốm men lam) là mặc định Release 1 [Confirmed — owner 2026-11-25]; Game Director chỉnh qua công thức `tools/art/zone_<môn>_art.gd` rồi bake, không sửa scene.
- Không tải asset ngoài (phân quyền phiên làm việc chặn); toàn bộ tự làm bằng code, MIT.
