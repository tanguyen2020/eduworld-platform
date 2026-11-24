# Tasks: F067 — Art pass 1: Main Hub low-poly stylized và nhân vật chibi (asset MIT tự làm)

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md) · **Prerequisites**: F062 Done, F070 Done · ADR-0102

## Phase 1: Setup

- [x] T001 ADR-0102 nguồn asset MIT/CC0 + chỉ mục ADR; `src/game/assets/LICENSES.md` (`docs/adr/ADR-0102-*.md`)
- [x] T002 Ảnh "trước" của greybox bằng công cụ chụp mới `src/game/tools/capture_hub.gd` (5 góc) → `screenshots/hub_before_*.png`

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Token: `palette.art.*`, `art.palette.*` (slot `eduworld.index`), `light.*`, `number.light.*`; format preset `mat_toon.tres` / `mat_toon_no_outline.tres` (mảng màu), Environment (ambient phẳng, tonemap, sương), `Art3dTokens.ART_PALETTE(_COLORS)`, `MATERIAL_PATHS`; `tokens@0.8.0` (`design/`)
- [x] T004 Shader toon chung + viền inverted hull, uniform global `outline_fade_distance` (`src/game/resources/art/shaders/`, `project.godot [shader_globals]`)
- [x] T005 Builder low-poly (mặt phẳng, slot màu theo tên, pháp tuyến mượt trong COLOR, transform gương) `src/game/tools/art/lowpoly_mesh.gd`
- [x] T006 [P] Test luật license + literal màu trước, rồi `tools/lib/art_rules.gd` + CLI `tools/lint_art.gd` — F067-AC-03, F067-AC-04 (`tests/unit/art/test_art_rules.gd`)

## Phase 3: User Story 1 — Quảng trường Tia Sáng đẹp và đúng mô tả (Priority: P1) 🎯 MVP

**Goal**: Hub low-poly có đủ điểm nhấn WORLD_DESIGN §4.1, §11. **Independent Test**: `test_hub_art.gd` + ảnh `hub_after_*`.

### Tests cho User Story 1

- [x] T007 [P] [US1] Test điểm nhấn Hub (cây đa, mái ngói, ruộng bậc thang, thuyền thúng, đèn lồng, bảng tin) và biểu tượng cổng gió theo vùng — F067-AC-10 (`tests/scene/world/test_hub_art.gd`)
- [x] T008 [P] [US1] Test mọi mesh Hub dùng preset toon sinh, vật thể đặc có pass viền; preset mang đúng token — F067-AC-01, F067-AC-05 (`tests/scene/world/test_hub_art.gd`, `tests/unit/art/test_baked_art.gd`)
- [x] T009 [P] [US1] Test giữ spawn/interactable/portal/collision; đình sau hàng cổng gió — F067-AC-11 (`tests/scene/world/test_hub_art.gd`; F062/F071 test giữ xanh)

### Implementation cho User Story 1

- [x] T010 [US1] Công thức Hub: địa hình, quảng trường, ruộng bậc thang, hồ, đồi, núi; đình, Thư viện Gió, nhà mái ngói, cổng làng, hàng rào; hồ sen + thuyền thúng, đèn, ghế, biển chỉ đường có biểu tượng; cây đa Tia Sáng; cây, cau, tre, bụi, cỏ, hoa, lúa (`src/game/tools/art/hub_art.gd`)
- [x] T011 [US1] Cổng gió theo vùng đích + bảng tin + marker tia sáng; `AreaPortal.gate_mesh` (id hợp lệ) (`scenes/world/AreaPortal.tscn`, `WorldAnchor.tscn`, `scripts/world/area_portal.gd`)
- [x] T012 [US1] `ScatterSet` / `ScatterInstance` (MultiMesh lúc chạy), catalog + bake `--check` (`scripts/world/scatter_*.gd`, `tools/art/art_catalog.gd`, `tools/art/bake_art.gd`)
- [x] T013 [US1] Scene Hub: mesh bake, collision mới (cây đa, nhà, hàng rào), đình lùi sau hàng cổng gió; khối anchor F071 giữ nguyên (`scenes/world/zones/main_hub/Zone.tscn`)

## Phase 4: User Story 2 — Nhân vật chibi dễ thương, tùy biến vẫn chạy (Priority: P1)

**Goal**: thân chibi + mặt, tóc/đồ/phụ kiện bake, NPC 3 dáng. **Independent Test**: test avatar F070 + `test_baked_art.gd`.

### Tests cho User Story 2

- [x] T014 [P] [US2] Test tỉ lệ đầu 0,3–0,45, chiều cao 1,2–1,5 m, mặt có mắt/điểm sáng/miệng/má — F067-AC-08 (`tests/unit/art/test_baked_art.gd`); test F070 (`test_avatar_builder.gd`, `test_character_creation_scene.gd`, `test_customization_studio.gd`) giữ xanh

### Implementation cho User Story 2

- [x] T015 [US2] Công thức thân chibi, 6 kiểu tóc, 6 trang phục, 4 phụ kiện, 3 NPC (nón lá, mũ bưu tá, khăn đóng) (`src/game/tools/art/character_art.gd`)
- [x] T016 [US2] `AvatarRig` thân bake, anchor theo đầu mới; `AvatarBuilder` dùng preset toon + `albedo`; `ItemVisual.tint_token` + cache; item trỏ mesh bake (`scenes/character/AvatarRig.tscn`, `scripts/player/*.gd`, `resources/items/*.tres`)
- [x] T017 [US2] NPC chọn dáng ổn định theo `npc_id` (`scenes/npc/NPC.tscn`, `scripts/world/npc_anchor.gd`)

## Phase 5: User Story 3 — Chạy mượt, đúng license, đúng trên Web (Priority: P1)

**Goal**: ngân sách, preset, renderer parity, license. **Independent Test**: benchmark, `test_zone_art_quality.gd`, `lint_art.gd`, export Web.

### Tests cho User Story 3

- [x] T018 [P] [US3] Benchmark Hub đếm draw call theo pass vật liệu + MultiMesh; ngân sách asset §8 — F067-AC-02 (`tests/perf/main_hub_benchmark.gd`, `tests/unit/art/test_baked_art.gd`)
- [x] T019 [P] [US3] Test preset Thấp/Trung bình/Cao (viền, mật độ, tầm nhìn) và tắt bóng nắng trên Compatibility — F067-AC-06, F067-AC-07 (`tests/unit/world/test_zone_art_quality.gd`)
- [x] T020 [P] [US3] Test shader chỉ dùng tính năng chung Mobile/Compatibility; mesh khớp công thức — F067-AC-07, F067-AC-09 (`tests/unit/art/test_baked_art.gd`)
- [x] T021 [US3] Chụp ảnh "sau" (Mobile, Web/Compatibility, preset Thấp/Cao), đo benchmark có renderer, export Web với preset tạm (không commit) — F067-AC-02, F067-AC-07, F067-AC-12 (quickstart §5)

### Implementation cho User Story 3

- [x] T022 [US3] `ZoneArtQuality` + `QualityPolicy` (`outline_distance`, `decor_density`, `shadows_supported`) gọi từ `ZoneRoot` (`scripts/world/zone_art_quality.gd`, `scripts/core/quality_policy.gd`, `scripts/world/zone_root.gd`)

## Phase 6: Polish & Cross-Cutting

- [x] T023 Tài liệu sở hữu: WORLD_DESIGN §11, WORLD_RENDERING §6.1/§7/§8, DESIGN_SYSTEM §5.3, PERFORMANCE_BUDGET §5, CONFIG_DEFAULTS `quality_presets`, OPEN_QUESTIONS OQ-16, RISKS RISK-16, CODE_PLAN, RTM, CHANGELOG, `design/CHANGELOG.md`
- [x] T024 Gate: GdUnit4 toàn bộ, `check_typed`, `lint_theme_overrides`, `lint_layers`, `lint_signals`, `lint_art`, `extract_pot --check`, `pseudo_locale --check`, `check_no_admin_ui`, `gen_fixtures --check`, `bake_art --check`, `make design-tokens-check`, check-features, check-cross-features, check-links, check-mermaid, build-rtm `--check`

## Dependencies & Execution Order

- Phase 1 → Phase 2 (token, shader, builder, lint) → US1 (Hub) ∥ US2 (nhân vật) → US3 (đo, preset, Web) → Polish.
- T003 trước T010/T015 (slot màu); T012 trước T013 (scatter); T016 trước test F070.

## Definition of Done

- Mọi F067-AC-* có test hoặc bằng chứng công cụ pass (quickstart §5); ảnh trước/sau trong `screenshots/`.
- Lint/gate ở T024 pass; không literal màu, mọi asset có dòng license được phép.
- Tài liệu sở hữu, CODE_PLAN, RTM, CHANGELOG cập nhật trong cùng push.
