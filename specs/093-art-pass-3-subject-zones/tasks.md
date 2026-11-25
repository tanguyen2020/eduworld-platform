# Tasks: F093 — Art pass 3: năm vùng môn học theo "Chibi anime cartoon 3D"

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md) · **Prerequisites**: F068 Done, F071 Done · ADR-0102, ADR-0103

## Phase 1: Setup

- [x] T001 Công cụ chụp vùng `tools/capture_zone.gd` (5 góc, `--area`, JPEG/PNG, cả hai renderer) — F093-AC-08
- [x] T002 Ảnh "trước" (greybox) năm vùng → `screenshots/<môn>_before_{overview,player}.jpg`

## Phase 2: Foundational (chặn mọi user story)

- [x] T003 Token `tokens@0.11.0`: 8 slot `art.palette.*` (52–59), `make design-tokens`, `design/CHANGELOG.md`
- [x] T004 Prefab vùng chung `tools/art/zone_kit.gd` (terrain gần/xa, rào biên, nhà, đèn, bồn hoa, luật rải cây)
- [x] T005 `ArtCatalog.ZONES`, bake + `drift()` cho mesh và `ScatterSet` vùng; icon ngôi nhà cho cổng gió về Thị trấn (`hub_art.gd`)
- [x] T006 `gen_greybox_zones.mjs`: giữ phần tay của mọi scene có marker, chỉ sinh lại khối neo

## Phase 3: User Story 1 — Mỗi vùng môn học có hình riêng (P1) 🎯 MVP

### Tests

- [x] T007 [P] [US1] Set piece cạnh điểm neo + slot màu, landmark, terrain, cổng gió ngôi nhà — F093-AC-01 (`tests/scene/world/test_subject_zone_art.gd`)
- [x] T008 [P] [US1] Mọi mesh dùng preset token, viền trừ decor/cảnh xa, mesh mượt — F093-AC-02 (`test_subject_zone_art.gd`, `lint_art.gd`)

### Implementation

- [x] T009 [US1] Vương quốc Số (`zone_math_art.gd`, `learning_zone_math/Zone.tscn`)
- [x] T010 [US1] Rừng Ngôn Ngữ — Tiếng Việt (`zone_vietnamese_art.gd`, `learning_zone_vietnamese/Zone.tscn`)
- [x] T011 [US1] Rừng Ngôn Ngữ — Tiếng Anh (`zone_english_art.gd`, `learning_zone_english/Zone.tscn`)
- [x] T012 [US1] Đảo Khoa Học (`zone_science_art.gd`, `learning_zone_science/Zone.tscn`)
- [x] T013 [US1] Vương quốc Lịch Sử (`zone_history_art.gd`, `learning_zone_history/Zone.tscn`)

## Phase 4: User Story 2 — Nhanh trên máy tầm trung và Web (P1)

- [x] T014 [US2] `ZoneBenchmark.tscn` (`--area`), `test_zone_benchmark.gd` (proxy tĩnh) — F093-AC-03
- [x] T015 [US2] Benchmark có renderer năm vùng × Mobile + Compatibility; cảnh xa không viền/bóng, giảm rải cây tới khi đạt — F093-AC-03 (quickstart §5)
- [x] T016 [US2] Ngân sách asset vùng + `bake_art --check` — F093-AC-04 (`test_subject_zone_art.gd`, `test_baked_art.gd`)
- [x] T017 [US2] Export Web preset F133 — F093-AC-07

## Phase 5: User Story 3 — Hợp đồng thế giới, license (P1)

- [x] T018 [US3] Điểm neo = seed, collision mặt đất giữ, collision mới không chắn spawn/điểm neo/lối đi — F093-AC-05 (`test_subject_zone_art.gd`, `zone_lint`, `gen_greybox_zones.mjs --check`)
- [x] T019 [US3] Dòng `LICENSES.md` + `lint_art` — F093-AC-06
- [x] T020 [US3] Ảnh sau Mobile (5 góc) + Web (3 góc) năm vùng — F093-AC-08 (`screenshots/`)

## Phase 6: Polish & Cross-Cutting

- [x] T021 Tài liệu sở hữu: WORLD_DESIGN §4.2/§11, WORLD_RENDERING §2/§8.1/§10, PERFORMANCE_BUDGET §7, CONFIG_DEFAULTS (`scene_budget_zone`), DESIGN_SYSTEM §5.3, CODE_PLAN (F093 Done, F094), RTM, CHANGELOG, `design/CHANGELOG.md`, `assets/LICENSES.md`
- [x] T022 Gate: GdUnit4 toàn bộ, `check_typed`, `lint_theme_overrides`, `lint_layers`, `lint_signals`, `lint_art`, `zone_lint`, `extract_pot --check`, `pseudo_locale --check`, `gen_fixtures --check`, `check_no_admin_ui`, `bake_art --check`, `make design-tokens-check`, check-features, check-cross-features, check-links, check-mermaid, build-rtm `--check`

## Phase 7: Follow-up

- [ ] T030 F094 — art pass 4: `practice_zone`, `social_zone`, `quest_zone.discovery_ocean`, `challenge_mountain`, `exam_zone`, `arena` trên prefab/pipeline này; kèm Câu hỏi mở F093-3 (cây cau, mặt phẳng hai mặt của Hub)
- [ ] T031 Câu hỏi mở F093-1 (vật thể quest F072 của Hub) và F093-2 (màu môn Lịch sử dùng chung, `env_history`)

## Dependencies & Execution Order

- Phase 1 → Phase 2 (token, prefab, catalog, generator) → US1 (từng vùng độc lập, song song được) → US2 (đo, giảm tới khi đạt) → US3 → Polish.

## Definition of Done

- Mọi F093-AC-* có test hoặc bằng chứng công cụ pass (quickstart §5); ảnh trước/sau trong `screenshots/`.
- Gate T022 pass; không literal màu; mọi asset có dòng license được phép.
- Tài liệu sở hữu, CODE_PLAN, RTM, CHANGELOG cập nhật trong cùng push.
