# Feature Specification: F094 — Art pass 4: sáu vùng còn lại theo "Chibi anime cartoon 3D" và các điểm mở của F093

**Feature Branch**: `094-art-pass-4-remaining-zones` · **Created**: 2026-11-25 · **Status**: Done · **Module**: `src/game/` (+ `design/game-tokens`, `design/shared-tokens`) · **Mốc**: M3 · **Phụ thuộc**: F093, F071, F072
**Input**: F093 làm năm vùng môn học và để lại sáu vùng còn là greybox F071 (mặt phẳng 50 × 50 m xám, điểm neo trôi nổi) cùng ba câu hỏi mở F093-1..3 ([F093 spec](../093-art-pass-3-subject-zones/spec.md#clarifications)). Lượt này làm **Thư viện Gió** (`practice_zone`), **Vườn Bạn Bè** (`social_zone`), **Đại dương Khám Phá** (`quest_zone.discovery_ocean`), **Núi Thử Thách** (`challenge_mountain`), **Tháp Thử Thách** (`exam_zone`) và **Đấu trường Nhà Vô Địch** (`arena`) theo hướng art đã chốt ([ADR-0103](../../docs/adr/ADR-0103-art-direction-chibi-anime-cartoon-3d-thay-stylized-low-poly.md)) trên pipeline F093, và đóng F093-1 (vật thể quest F072 của Hub), F093-2 (màu môn Lịch sử dùng chung) và F093-3 (viền đen của mặt phẳng hai mặt ở Hub). License asset giữ [ADR-0102](../../docs/adr/ADR-0102-nguon-asset-do-hoa-theo-license-mit-hoac-de-dai-hon.md). FR nguồn: FR-GAME-001, FR-GAME-002, FR-GAME-009, FR-GAME-016, FR-GAME-017; NFR-COMP-001, NFR-COMP-004, NFR-MAINT-007.

## Clarifications

### Session 2026-11-25

- Q: Phạm vi lượt này? → A: [Confirmed — giao việc của orchestrator 2026-11-25: "sáu vùng còn lại + các điểm mở F093; nếu quá lớn, làm trọn số vùng vừa sức, phần còn lại ghi follow-up"] Sáu vùng + F093-1..3. Nếu vùng nào không đạt chất lượng đầy đủ thì chuyển sang feature follow-up ghi ở CODE_PLAN, không giao bản dở.
- Q: Sáu vùng có bố cục điểm neo khác nhau (không phải bố cục vùng môn học), dựng thế nào? → A: [Confirmed — WORLD_DESIGN §4.2 "prefab vùng chung + cấu hình", F093] Dùng lại `zone_kit.gd` (nền cỏ, quảng trường, bệ lát, rào biên, cảnh xa, nhà, đèn, bồn hoa) cộng một **bố cục tùy biến** cho mỗi vùng (tâm quảng trường, các lối đi, các bệ dưới điểm neo, vùng cấm rải cây) — điểm neo vẫn lấy nguyên từ seed F071.
- Q: Mỗi vùng mang không khí gì? → A: [Confirmed — WORLD_DESIGN §2.1 "không vũ khí", §4.1, §4.3–§4.5; EXAM_ENGINE; COMPETITION_SYSTEM; CHALLENGE_BOSS_SYSTEM "boss mất năng lượng, không máu"; SOCIAL_SYSTEM] Thư viện Gió yên tĩnh, ấm (học cá nhân + góc học cùng nhau); Vườn Bạn Bè vui, nhiều hoa, hợp bạn bè và club; Đại dương Khám Phá phiêu lưu biển (đảo, hang, xác tàu cổ, không đại bác); Núi Thử Thách cao, mát, tinh thể; Tháp Thử Thách **yên tĩnh, tập trung** (bảng màu dịu, nước tĩnh, ít trang trí rực); Đấu trường **lễ hội, thân thiện, không bạo lực** (cờ, bóng bay, bục vinh danh, không vũ khí, không hình đối đầu).
- Q: Set piece cụ thể? → A: [Proposed — diễn giải mô tả WORLD_DESIGN; Game Director/owner chỉnh qua công thức `tools/art/zone_<vùng>_art.gd` rồi bake lại, không sửa scene] Bảng ở F094-AC-01.
- Q: Bảng màu có đủ không? → A: [Confirmed] Chỉ thêm **một** slot `art.palette.subject_history` (slot 60, = `color.subject.history` biến thể game); các vùng mới dùng slot sẵn có (`eye_light` cho tuyết, `tile_blue_dark` cho biển sâu, `blossom_violet` + `wind` cho tinh thể). Mảng palette của shader có 64 slot; sau lượt này còn 3 slot trống (61–63) — ghi chú ở DESIGN_SYSTEM §5.3. `tokens@0.12.0` (MINOR).
- Q: Ngân sách cảnh? → A: [Confirmed — PERFORMANCE_BUDGET §7 dòng "Vùng môn học / khám phá"] Cả sáu vùng là `hub_instance` ≤ 40 người → `scene_budget_zone` (≤ 120k primitive, ≤ 90 draw call, ≤ 50 vật thể động) trên Mobile và Compatibility.
- Q (F093-1): Hub thiếu `quest.hub.lantern_east`, `quest.hub.lantern_west`, `quest.hub.welcome_sign` — xử lý thế nào? → A: [Confirmed — owner "chốt hết" 2026-11-25] Sinh lại khối neo của Hub từ seed (`gen_greybox_zones.mjs --check` sạch cho mọi vùng); **không** dời vật thể seed (seed/spec không nói vị trí là tùy ý). `test_interactable.gd` đưa người chơi rời bảng tin tới một điểm trống khác thay cho (0, 0, 12) (nơi nay có `welcome_sign`).
- Q (F093-2): Màu môn Lịch sử? → A: [Confirmed — WORLD_DESIGN §11 đã chốt "Lịch sử nâu đỏ"; owner "chốt hết" 2026-11-25] Thêm `color.subject.history` (nâu đỏ) vào `design/shared-tokens/domain.json` (game + portal), `env_history` cho Vương quốc Lịch Sử, nhãn `domain.subject.history` cho portal; cổng gió/biển hiệu Lịch sử dùng `subject_history`. WCAG AA giữ trong `design-tokens-check`.
- Q (F093-3): Cây cau và mặt phẳng hai mặt của Hub viền đen khi nhìn từ phía sau? → A: [Confirmed — owner "chốt hết" 2026-11-25, cách F093 đã kiểm] Mọi tấm mỏng vẽ bằng preset có viền là **khối kín** (lá cau, cánh sen thành khối dẹt); test chặn tam giác trùng ngược chiều (mặt hai mặt dày 0) trong mesh có viền.
- Q: Owner 2026-11-26: "thiết kế game thật dễ dùng và thu hút học sinh" — áp vào sáu vùng thế nào? → A: [Confirmed — owner 2026-11-26, qua orchestrator] Mỗi vùng có **biển chỉ đường** gần điểm spawn với mũi tên có **biểu tượng hình** tới từng điểm neo và cổng gió về (ngôi nhà); bệ điểm neo có **vòng sáng** ấm; marker `WorldAnchor` dùng chung là viên đá Tia Sáng lớn có vòng hào quang (đọc được từ xa, mời học sinh lại gần); cảnh ấm, sinh động cho khối 1–9, riêng Tháp Thử Thách giữ yên tĩnh (F094-AC-13).
- Q: Ba vật thể quest F072 thêm vào Hub làm Hub vượt `scene_budget_hub` (Mobile 151k > 150k)? → A: [Confirmed — PERFORMANCE_BUDGET §7, cách F093] Tách cảnh xa của Hub (đất xa, đồi, Núi Thử Thách chân trời) thành mesh `env_hub_far_a` vẽ bằng preset không viền, không đổ bóng; marker không đổ bóng; Hub còn 148,6k / 66 draw call (Mobile).
- Q: Diễn giải set piece Việt Nam của F093 (Văn Miếu, Ngọ Môn, Chùa Một Cột, trống đồng, cọn nước, bằng lăng)? → A: [Confirmed — owner "chốt hết" 2026-11-25] Là mặc định Release 1; Game Director chỉnh công thức rồi bake lại (ghi ở F093 Clarifications).

**Câu hỏi mở**

- F094-1: Mảng palette của toon shader còn 3 slot (61–63). Lượt art sau cần nhiều màu hơn thì phải nâng `palette[64]` → `palette[128]` (đổi shader + preset sinh, kiểm WebGL 2 uniform limit) [Proposed: quyết định khi cần, ADR mới nếu đổi giới hạn].
- F094-2: Set piece mới của sáu vùng là diễn giải [Proposed] (bảng F094-AC-01); Game Director/owner duyệt qua ảnh `screenshots/` và chỉnh công thức.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Mỗi vùng còn lại có hình riêng, đúng không khí (Priority: P1)

Học sinh đi qua cổng gió từ Quảng trường Tia Sáng vào Thư viện Gió, Vườn Bạn Bè, Đại dương Khám Phá, Núi Thử Thách, Tháp Thử Thách hay Đấu trường và thấy một nơi tròn mềm, cùng phong cách Hub, có landmark riêng và set piece cạnh từng điểm neo thay cho mặt phẳng xám.

**Why this priority**: Sáu vùng là nơi học sinh luyện tập, gặp bạn, thi thử và thi đấu; greybox làm vỡ hướng art đã chốt.

**Independent Test**: `test_zone_art_pass_4.gd`; ảnh `screenshots/<vùng>_after_*` so với `<vùng>_before_*`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F094-AC-01 | HỆ THỐNG PHẢI dựng mỗi vùng từ prefab vùng chung + bố cục riêng và bộ set piece riêng: **Thư viện Gió** — thư viện gió tường vàng hai tầng mái ngói có chong chóng gió (landmark), trạm luyện tập (bảng đá trên giá, bục sách mở, chồng sách) cạnh `station.practice.board`, góc học cùng nhau (bàn tròn, ghế đệm, ô che), vòm sách trên lối chính, bàn học cá nhân, giá sách, chuông gió, chong chóng; **Vườn Bạn Bè** — nhà tổ ong của Chú Ong Kết Nối (vòm tổ ong vàng mật, landmark), chậu hạt giống có hoa hướng dương non + bình tưới cạnh `collectible.social.flower_seed`, vòm hoa hình trái tim trên lối chính, sân chơi (xích đu, bập bênh), chỗ picnic, góc club (ba mái che có cờ đuôi nheo), bồn hoa trái tim, dây cờ; **Đại dương Khám Phá** — đảo cát giữa biển, ngọn hải đăng trên đảo đá (landmark), xác tàu gỗ cổ nghiêng trong cát có rương (không đại bác) cạnh `discovery.ocean.shipwreck`, hang san hô cạnh `discovery.ocean.coral_cave`, vòm vỏ sò trên lối chính ở `collectible.ocean.rare_shell`, tượng Bạch Tuộc Câu Đố hiền, thuyền thúng, cầu tàu, phao, dừa; **Núi Thử Thách** — đỉnh núi có Tinh Thể Tri Thức lơ lửng và vòng sáng (landmark), núi tuyết xa, đài ngắm cảnh có ống nhòm cạnh `discovery.mountain.summit_view`, cụm tinh thể trên đá cạnh `collectible.mountain.crystal_shard`, vòng thử thách bằng đá có trụ tinh thể, cây thông, ụ đá xếp, cờ; **Tháp Thử Thách** — tháp nhiều tầng mái ngói men xanh dịu (landmark), giá bảng kỳ thi có mặt đồng hồ và lưới lịch trống cạnh `board.exam_sessions`, hồ Gương Phản Chiếu (nước tĩnh, khung đá tròn), vườn sỏi cào, đình học yên tĩnh, đèn đá, bảng màu dịu; **Đấu trường Nhà Vô Địch** — sân vận động tròn có mái bạt sọc màu, cờ, ngôi sao (landmark), bốn cổng xếp hàng có biểu tượng hình (1v1, đội, tính giờ, co-op) cạnh `npc.referee_thunder`, bảng lịch giải đấu cạnh `npc.madam_calendar`, bục vinh danh 1-2-3 với cúp ngôi sao và bảng chỉ có ô trống (không tên), tượng cúp, quầy lễ hội, bóng bay, dây cờ. Mỗi set piece là mesh bake tự làm đặt cạnh điểm neo của nó; cổng gió về Thị trấn hiện biểu tượng ngôi nhà; không vũ khí, không hình bạo lực, không chữ/nội dung học tập trong scene | FR-GAME-001 · FR-GAME-002 · FR-GAME-016 · FR-GAME-017 · ADR-0103 · `WORLD_DESIGN` §2.1, §4.1, §4.3–§4.5, §11 · AGENTS R2 | — |
| F094-AC-02 | HỆ THỐNG PHẢI tô mọi mesh của sáu vùng bằng preset sinh từ token (`mat_toon*`), mỗi mặt chỉ mang slot `art.palette.*`; mesh đặc có pass viền mực, chỉ trang trí rải (`decor`) và cảnh xa (`Far`, không đổ bóng) được bỏ viền; mesh thế giới đổ bóng mượt (≥ 30 % đỉnh); NẾU nguồn mỹ thuật chứa literal màu, THÌ `lint_art.gd` PHẢI thất bại | FR-GAME-001 · NFR-MAINT-007 · `DESIGN_SYSTEM` §5.3 · AGENTS R32 | TC-UI-001 · TC-UI-003 |
| F094-AC-13 | Mỗi vùng trong sáu vùng PHẢI có biển chỉ đường gần điểm spawn với mũi tên mang biểu tượng hình (không chỉ màu) chỉ tới các điểm neo chính và cổng gió về Thị trấn (ngôi nhà), và bệ điểm neo PHẢI có vòng sáng; marker `WorldAnchor` PHẢI là viên đá Tia Sáng cao trên 1,5 m có vòng hào quang | `WORLD_DESIGN` §6, §11 (trợ năng: không chỉ dùng màu) · `DESIGN_SYSTEM` §5.3 · owner 2026-11-26 | TC-UI-001 |
| F094-AC-03 | Tháp Thử Thách PHẢI dùng bảng màu dịu (không slot đỏ/cam rực `lantern`, `cloth_red`, `subject_vietnamese` trên landmark và set piece) và Đấu trường PHẢI dùng ít nhất năm màu lễ hội (`cloth_red`, `cloth_blue`, `cloth_green`, `cloth_violet`, `lantern_glow`); NẾU node hay id của vùng gọi tên vũ khí/bạo lực, THÌ `zone_lint` PHẢI thất bại | `WORLD_DESIGN` §2.1, §4.4, §4.5 · `COMPETITION_SYSTEM` · `EXAM_ENGINE` · `CHILD_SAFETY` | TC-UI-001 |

---

### User Story 2 — Vẫn nhanh trên máy tầm trung và trên Web (Priority: P1)

**Why this priority**: NFR-COMP-001 bắt buộc; owner xem bản Web (Compatibility).

**Independent Test**: `test_zone_benchmark.gd` (mười một vùng), `ZoneBenchmark.tscn --area=…` trên Mobile + Compatibility, `test_zone_art_pass_4.gd` (ngân sách asset), export Web.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F094-AC-04 | HỆ THỐNG PHẢI giữ mỗi vùng trong sáu vùng ở preset Trung bình trong `scene_budget_zone` (≤ 120k primitive, ≤ 90 draw call, ≤ 50 vật thể động) đo bằng `RenderingServer` trên **cả** renderer Mobile và Compatibility (`ZoneBenchmark.tscn --area=…`), và proxy tĩnh headless trong CI không vượt ngân sách quá `perf_budget_tolerance_ci` | NFR-COMP-001 · FR-GAME-009 · `PERFORMANCE_BUDGET` §7, §15 | TC-PERF-018 |
| F094-AC-05 | HỆ THỐNG PHẢI giữ từng asset vùng trong PERFORMANCE_BUDGET §8 (mặt đất + cảnh xa ≤ 40k mỗi vùng, tòa nhà/bộ landmark ≤ 10k, set piece ≤ 3k, cây lặp ≤ 2k) và tái tạo được mọi mesh/`ScatterSet` từ công thức; NẾU resource commit lệch công thức, THÌ `bake_art.gd --check` và test PHẢI thất bại | NFR-COMP-001 · ADR-0102 · `PERFORMANCE_BUDGET` §8 · `WORLD_RENDERING` §8.1 | — |
| F094-AC-06 | Bản Web (preset `Web` của F133) PHẢI export được và gói PCK chứa scene, mesh và `ScatterSet` của sáu vùng | NFR-COMP-004 · `PERFORMANCE_BUDGET` §14 | — |

---

### User Story 3 — Đúng hợp đồng thế giới, đúng license (Priority: P1)

**Independent Test**: `test_zone_art_pass_4.gd`, `zone_lint.gd`, `gen_greybox_zones.mjs --check`, `lint_art.gd`, `capture_zone.gd`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F094-AC-07 | KHI thay đồ họa một vùng, HỆ THỐNG PHẢI giữ điểm neo bằng đúng seed thế giới (id, loại, vị trí của spawn point, interactable, cổng và vùng đích), giữ collision mặt đất 50 × 50 m của F071, và collision mới KHÔNG ĐƯỢC chắn spawn, điểm neo và các lối đi của bố cục vùng | FR-GAME-017 · F071 · AGENTS R1, R2 · `WORLD_RENDERING` §2, §10 | — |
| F094-AC-08 | HỆ THỐNG PHẢI có dòng license được phép (MIT) trong `res://assets/LICENSES.md` cho mọi file asset mới; NẾU thiếu dòng hoặc license ngoài danh sách, THÌ `lint_art.gd` PHẢI thất bại | ADR-0102 · `WORLD_RENDERING` §8.1 | TC-UI-003 |
| F094-AC-09 | HỆ THỐNG PHẢI có ảnh duyệt trước/sau của sáu vùng (`capture_zone.gd --area=…`, góc chụp theo bố cục từng vùng) trên cả renderer Mobile và Compatibility | `DESIGN_SYSTEM` §8 · NFR-COMP-004 | TC-UI-007 |

---

### User Story 4 — Đóng các điểm mở của F093 (Priority: P1)

**Why this priority**: Owner đã chốt; seed và scene Hub phải khớp (server quyết định quest theo id vật thể), màu môn học phải đủ năm môn ở game và portal, Hub không còn viền đen sai.

**Independent Test**: `gen_greybox_zones.mjs --check`, `test_interactable.gd`, `test_zone_art_pass_4.gd`, `make design-tokens-check`, test portal liên quan màu nghiệp vụ.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F094-AC-10 | HỆ THỐNG PHẢI có ở Main Hub đủ vật thể quest F072 của seed (`quest.hub.lantern_east`, `quest.hub.lantern_west`, `quest.hub.welcome_sign`) ở đúng vị trí seed, và `gen_greybox_zones.mjs --check` PHẢI sạch cho **mọi** vùng; test tương tác của Hub PHẢI rời bảng tin tới một điểm không có điểm neo nào trong tầm với | FR-GAME-017 · F072 · F071 · `WORLD_RENDERING` §10 | — |
| F094-AC-11 | HỆ THỐNG PHẢI khai báo `color.subject.history` (nâu đỏ, biến thể light/dark/game) trong `design/shared-tokens/domain.json` và sinh ra cho game (slot `art.palette.subject_history`, `env_history`) và portal (theme, nhãn `domain.subject.history`); `make design-tokens-check` PHẢI pass (WCAG AA) và test/snapshot portal liên quan màu nghiệp vụ PHẢI xanh | `WORLD_DESIGN` §11 · `DESIGN_SYSTEM` §6 · ADR-0047 · AGENTS R32 | TC-UI-001 |
| F094-AC-12 | NẾU một mesh thế giới được vẽ bằng preset có viền mực, THÌ nó PHẢI là khối kín, không chứa mặt phẳng hai mặt dày 0 (cặp tam giác trùng đỉnh, ngược chiều); cây cau (`veg_palm_cau_a`) và cánh sen của Hub PHẢI là khối kín | `WORLD_RENDERING` §8.1 · ADR-0103 | — |

### Edge Cases

- NẾU một điểm neo nằm sát cổng gió (Núi Thử Thách: `discovery.mountain.summit_view` cách cổng 2 m), THÌ set piece đặt lệch sang bên, không chắn lối tới cổng (F094-AC-07).
- NẾU renderer là Compatibility (Web, không bóng nắng thời gian thực), THÌ nhân vật dùng blob shadow F068; cảnh xa không đổ bóng ở mọi renderer (F094-AC-02, F094-AC-04).
- NẾU preset Thấp, THÌ `ZoneArtQuality` giảm mật độ trang trí và khoảng nhìn như ở Hub (F094-AC-04).
- NẾU portal hiển thị môn `history` (DomainTag, biểu đồ), THÌ có chấm màu `color.subject.history` và nhãn chữ (không chỉ màu) (F094-AC-11).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-GAME-001 | Sáu vùng theo "Chibi anime cartoon 3D": khối tròn, cel shading, viền mực | `WORLD_DESIGN` §11 · `DESIGN_SYSTEM` §5.3 |
| FR-GAME-002 | Đồ họa khu A/C/D/E/F/G còn lại (Thư viện Gió, Vườn Bạn Bè, Tháp Thử Thách, Đấu trường, Đại dương Khám Phá, Núi Thử Thách) | `WORLD_DESIGN` §3, §4 |
| FR-GAME-016 | Landmark gắn chương truyện (Bạch Tuộc Câu Đố, Tinh Thể Tri Thức) | `WORLD_DESIGN` §2.2 |
| FR-GAME-017 | Điểm khám phá, vật sưu tầm, trạm luyện tập, bảng kỳ thi, cổng gió về Thị trấn có hình đọc được; vật thể quest F072 của Hub có mặt | `WORLD_DESIGN` §6, §8 |
| FR-GAME-009 | Ngân sách vùng trên Mobile/Compatibility, preset chất lượng | `PERFORMANCE_BUDGET` §5, §7, §8 · `WORLD_RENDERING` §6.1 |

### Key Entities

- **Prefab vùng chung** (`tools/art/zone_kit.gd`, F093) + **bố cục tùy biến** (mới): `lawn()`, `plaza()`, `pad()`, `path_strip()`, `clear_of()`/`scatter_in()` theo bố cục của vùng, đồ dùng chung (cờ đuôi nheo, dây cờ, bóng bay, ụ đá, thông).
- **Công thức vùng** (`tools/art/zone_<vùng>_art.gd`): `meshes()`, `SCATTER`, `placements()`; đăng ký ở `ArtCatalog.ZONES`.
- **Asset vùng** (`assets/models/{env,bld,prop,veg}/…_<vùng>_…_a.res`, `env_<vùng>_scatter_<loại>_a.res`).
- **Token** `color.subject.history`, slot `art.palette.subject_history` (60), `env.{history,ocean,mountain,exam,arena}`.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `scene_budget_zone` [Confirmed] | Ngân sách cảnh của sáu vùng (F094-AC-04) |
| `perf_budget_tolerance_ci`, `load_time` | Dung sai CI, thời gian nạp vùng (F094-AC-04) |
| `quality_presets`, `renderer_default` | Mật độ trang trí, khoảng nhìn, bóng (F094-AC-04) |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Máy tầm trung, Web WebGL 2, renderer parity | NFR-COMP-001 · NFR-COMP-004 · ADR-0057 · ADR-0101 |
| Không hardcode màu; file sinh không sửa tay; màu nghiệp vụ chỉ ở shared-tokens | NFR-MAINT-007 · AGENTS R32 · ADR-0047 |
| License asset MIT/CC0, asset tự làm MIT | ADR-0102 |
| Không logic nghiệp vụ, không nội dung học tập trong scene; không vũ khí/bạo lực | AGENTS R1, R2 · `WORLD_DESIGN` §2.1 · `CHILD_SAFETY` |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Ảnh sau art pass của sáu vùng (Mobile 5 góc + Web 3 góc mỗi vùng) đọc rõ là "chibi anime cartoon", cùng phong cách Hub và F093; Tháp Thử Thách đọc là yên tĩnh, Đấu trường đọc là lễ hội thân thiện.
- **SC-002**: Benchmark có renderer mỗi vùng ≤ 120k primitive và ≤ 90 draw call trên cả Mobile và Compatibility.
- **SC-003**: GdUnit4 toàn bộ xanh; mọi lint xanh; `gen_greybox_zones.mjs --check` sạch cho 12 vùng; export Web thành công; portal lint/typecheck/test liên quan xanh.

## Ngoài phạm vi

- Phòng thi 2D, phòng "Gương Phản Chiếu" phân tích câu sai (UI của EXAM_ENGINE), sân `battle` room của trận đấu, private instance của boss — chỉ sảnh `hub_instance` của mỗi vùng.
- Bảng xếp hạng 3D có dữ liệu thật (nickname): set piece chỉ là khung với ô trống; nối dữ liệu là feature thi đấu.
- Trạm theo Topic hiển thị mastery bằng màu, "đất xám → có màu" theo mastery.
- Bundle PCK theo vùng, lightmap/occluder bake, NavMesh, âm thanh vùng.
- Nâng giới hạn palette 64 slot (Câu hỏi mở F094-1).

## Assumptions

- Set piece mới là diễn giải [Proposed] của WORLD_DESIGN; Game Director chỉnh qua công thức rồi bake, không sửa scene.
- Không tải asset ngoài (phân quyền phiên làm việc chặn); toàn bộ tự làm bằng code, MIT.
