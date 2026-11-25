# Tasks: F068 — Art pass 2: Chibi anime cartoon 3D cho Main Hub và nhân vật

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md) · **Prerequisites**: F067 Done, F070 Done · ADR-0102, ADR-0103

## Phase 1: Setup

- [x] T001 ADR-0103 (owner 2026-11-25 "Chibi anime cartoon 3D") + chỉ mục ADR, ADR-0102 một phần superseded (`docs/adr/`)
- [x] T002 Ảnh "trước" bằng `tools/capture_hub.gd` (5 góc) → `screenshots/hub_before_*.png`

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Token `tokens@0.10.0`: `toon.band_softness`, `toon.steps` 2, `outline.color` mực ấm, `light.blob_shadow_*`, slot 48–51; preset `mat_toon_skinned.tres` (`design/`, `make design-tokens`)
- [x] T004 Shader cel (`band_softness`) + shader blob shadow (`src/game/resources/art/shaders/`)
- [x] T005 Builder: pháp tuyến mượt theo góc (trọng số diện tích), hộp vát cạnh, trụ bo vành, `min_sides`, `blob_detail`, `set_flat()`, skinning (`src/game/tools/art/lowpoly_mesh.gd`)
- [x] T006 `KidSkeleton` (xương, đầu, anchor, biểu cảm, đường dẫn) (`src/game/scripts/player/kid_skeleton.gd`)

## Phase 3: User Story 1 — Thế giới tròn mềm, cel shading, viền mực (P1) 🎯 MVP

### Tests

- [x] T007 [P] [US1] Test builder + độ mượt mesh Hub — F068-AC-01 (`tests/unit/art/test_art_pass_2.gd`)
- [x] T008 [P] [US1] Test preset cel/viền lấy từ token, cùng shader cho mọi preset — F068-AC-02 (`tests/unit/art/test_art_pass_2.gd`, `tests/unit/art/test_baked_art.gd`)

### Implementation

- [x] T009 [US1] Công thức Hub theo builder mới; vật lặp/xa giữ bậc chia gốc, vát ≥ 0,15 m; cây/bụi/đồi mềm (`src/game/tools/art/hub_art.gd`)
- [x] T010 [US1] Mặt đất và bụi không đổ bóng (`scenes/world/zones/main_hub/Zone.tscn`)

## Phase 4: User Story 2 — Nhân vật chibi anime có biểu cảm (P1)

### Tests

- [x] T011 [P] [US2] Test tỉ lệ chibi, mặt anime theo biểu cảm, skinning thân/trang phục — F068-AC-03, F068-AC-04, F068-AC-05 (`tests/unit/art/test_art_pass_2.gd`, `tests/unit/art/test_baked_art.gd`)
- [x] T012 [P] [US2] Test rig khớp `KidSkeleton`, anchor theo xương, `set_expression`, chớp mắt, trang phục gắn skeleton — F068-AC-04, F068-AC-05 (`tests/scene/character/test_kid_rig.gd`); test F070 giữ xanh

### Implementation

- [x] T013 [US2] Công thức thân chibi skinned, mặt 4 biểu cảm + chớp mắt (decal chiếu lên đầu), 6 tóc lọn khối, 6 trang phục skinned, 4 phụ kiện, 3 NPC (`src/game/tools/art/character_art.gd`)
- [x] T014 [US2] `AvatarRig.tscn` (Skeleton3D `kid_base`, BoneAttachment3D, FaceMesh, BlobShadow); `AvatarBuilder` (anchor mới, trang phục skinned + preset `toon_skinned`, `set_expression`, chớp mắt) (`scenes/character/AvatarRig.tscn`, `scripts/player/avatar_builder.gd`)

## Phase 5: User Story 3 — Chuyển động hoạt hình (P1)

### Tests

- [x] T015 [P] [US3] Test 10 clip key mọi xương, squash giữ thể tích, chạy co/giãn, emote trỏ clip — F068-AC-06 (`tests/unit/art/test_art_pass_2.gd`)
- [x] T016 [P] [US3] Test trạng thái → clip trên xương + biểu cảm, emote → clip + biểu cảm — F068-AC-07 (`tests/scene/character/test_kid_rig.gd`); `test_player_controller.gd`, `test_emote_wheel.gd`, `test_remote_players.gd` giữ xanh

### Implementation

- [x] T017 [US3] Công thức clip `idle/walk/run/jump/wave/celebrate/clap/dance/thank/apologize` (`src/game/tools/art/character_anim.gd`), bake + `--check` clip (`art_catalog.gd`, `bake_art.gd`)
- [x] T018 [US3] `AnimDriver`: thư viện clip rig, biểu cảm theo trạng thái/emote; `ItemVisual.expression`; emote `.tres` trỏ clip; NPC thở lệch pha (`scripts/player/anim_driver.gd`, `item_visual.gd`, `resources/items/emote_*.tres`, `scenes/npc/NPC.tscn`, `scripts/world/npc_anchor.gd`)

## Phase 6: User Story 4 — Blob shadow và Cây Tia Sáng (P2)

### Tests

- [x] T019 [P] [US4] Test blob shadow theo renderer/bóng nắng, vật liệu token, co theo độ cao, có ở mọi nhân vật — F068-AC-08 (`tests/unit/world/test_blob_shadow.gd`)
- [x] T020 [P] [US4] Test cấp cây từ mastery, 5 mesh từ xám tới nở hoa, Hub đổi mesh — F068-AC-09 (`tests/unit/world/test_spark_tree.gd`)

### Implementation

- [x] T021 [US4] `BlobShadow` + `QualityPolicy.blob_shadow_visible` + `ZoneArtQuality` (`scripts/world/blob_shadow.gd`, `scripts/core/quality_policy.gd`, `scripts/world/zone_art_quality.gd`, `scenes/character/Player.tscn`)
- [x] T022 [US4] Công thức `spark_tree(stage)`, `SparkTree`, `SparkTreeModel`, Hub 5 cấp (`tools/art/hub_art.gd`, `scripts/world/spark_tree.gd`, `scripts/world/models/spark_tree_model.gd`, `Zone.tscn`)

## Phase 7: User Story 5 — Nhanh, đúng license, đúng hợp đồng (P1)

- [x] T023 [US5] Benchmark có renderer Mobile + Compatibility; ngân sách asset — F068-AC-10 (`tests/perf/MainHubBenchmark.tscn`, `test_main_hub_benchmark.gd`, `test_baked_art.gd`)
- [x] T024 [US5] `lint_art` + dòng `LICENSES.md` cho asset mới — F068-AC-11
- [x] T025 [US5] Hợp đồng F062/F071/F070 giữ nguyên — F068-AC-12 (`test_hub_art.gd`, test F062/F070/F071)
- [x] T026 [US5] `bake_art --check` mesh + clip — F068-AC-13
- [x] T027 [US5] Export Web preset F133 — F068-AC-14 (quickstart §5)
- [x] T028 [US5] Công cụ `tools/capture_avatars.gd`; ảnh Hub/nhân vật/tư thế/biểu cảm/cấp cây trên Mobile + Web — F068-AC-15 (`screenshots/`)

## Phase 8: Polish & Cross-Cutting

- [x] T029 Tài liệu sở hữu: WORLD_DESIGN §11, CHARACTER_SYSTEM §4.1, DESIGN_SYSTEM §1/§5.3, WORLD_RENDERING §6.1/§7/§8.1, PERFORMANCE_BUDGET §4/§5, CONFIG_DEFAULTS (`avatar_blink`, `spark_tree_default_stage`), OPEN_QUESTIONS OQ-16, CODE_PLAN (F068, F093), RTM, CHANGELOG, `design/CHANGELOG.md`, `assets/LICENSES.md`
- [x] T030 Gate: GdUnit4 toàn bộ, `check_typed`, `lint_theme_overrides`, `lint_layers`, `lint_signals`, `lint_art`, `zone_lint`, `extract_pot --check`, `pseudo_locale --check`, `gen_fixtures --check`, `check_no_admin_ui`, `bake_art --check`, `make design-tokens-check`, check-features, check-cross-features, check-links, check-mermaid, build-rtm `--check`

## Phase 9: Follow-up chốt 2026-11-25 (Câu hỏi mở F068-1 [Confirmed])

- [ ] T040 Operation đọc mới ở module `learning`: mức mastery tổng của (trường, khối) của học sinh đang chơi — một mức tổng hợp, không tên/xếp hạng/số liệu cá nhân, ô < `analytics_min_cell_size` → "chưa đủ dữ liệu"; route `game_player`, `TenantScope`, contract + `x-portal-action` không cần (chỉ game); test cô lập tenant/trường và ô nhỏ; client gọi khi vào Hub và nối `SparkTree.show_mastery()` (thiếu dữ liệu → cấp mặc định 3)

## Dependencies & Execution Order

- Phase 1 → Phase 2 (token, shader, builder, KidSkeleton) → US1 ∥ US2 → US3 (clip cần rig) → US4 → US5 (đo, ảnh, export) → Polish.

## Definition of Done

- Mọi F068-AC-* có test hoặc bằng chứng công cụ pass (quickstart §5); ảnh trước/sau trong `screenshots/`.
- Gate T030 pass; không literal màu; mọi asset có dòng license được phép.
- Tài liệu sở hữu, CODE_PLAN, RTM, CHANGELOG cập nhật trong cùng push.
