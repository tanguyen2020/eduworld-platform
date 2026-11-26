# Tasks: F094 — Art pass 4: sáu vùng còn lại theo "Chibi anime cartoon 3D" và các điểm mở của F093

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md) · **Prerequisites**: F093 Done, F071 Done, F072 Done · ADR-0102, ADR-0103

## Phase 1: Setup

- [x] T001 Ảnh "trước" (greybox) sáu vùng → `screenshots/<vùng>_before_{overview,player}.jpg`; góc chụp theo bố cục từng vùng trong `tools/capture_zone.gd` — F094-AC-09

## Phase 2: Foundational (chặn mọi user story)

- [x] T002 Token `tokens@0.12.0`: `color.subject.history`, slot `art.palette.subject_history` (60), `env.{history,ocean,mountain,exam,arena}`; `make design-tokens`; nhãn portal `domain.subject.history`; `design/CHANGELOG.md` — F094-AC-11
- [x] T003 `zone_kit.gd`: tách `lawn()`/`plaza()`/`pad()` (đầu ra F093 không đổi), `clear_of()`/`scatter_in()` theo bố cục, đồ dùng chung (`pennant`, `bunting`, `balloons`, `cairn`, `rock`, `pine`)
- [x] T004 `ArtCatalog.ZONES` thêm sáu vùng; `gen_greybox_zones.mjs` map `history → env_history`

## Phase 3: User Story 4 — Đóng điểm mở F093 (P1)

- [x] T005 [US4] F093-1: sinh lại khối neo Hub; `test_interactable.gd` đổi điểm rời — F094-AC-10
- [x] T006 [US4] F093-2: Vương quốc Lịch Sử dùng `env_history`, cổng gió/biển hiệu Lịch sử dùng `subject_history` — F094-AC-11
- [x] T007 [US4] F093-3: lá cau, cánh sen, thuyền thúng khối kín; test mặt hai mặt dày 0; cảnh xa Hub tách `env_hub_far_a` để giữ `scene_budget_hub` — F094-AC-12
- [x] T008 [US4] Ghi [Confirmed] F093-1..3 + diễn giải set piece Việt Nam vào F093 Clarifications

## Phase 4: User Story 1 — Mỗi vùng có hình riêng (P1) 🎯 MVP

### Tests

- [x] T009 [P] [US1] Set piece cạnh điểm neo + slot màu, landmark, terrain, cổng gió ngôi nhà, bảng màu dịu/lễ hội — F094-AC-01, F094-AC-03 (`tests/scene/world/test_zone_art_pass_4.gd`)
- [x] T010 [P] [US1] Mọi mesh dùng preset token, viền trừ decor/cảnh xa, mesh mượt — F094-AC-02 (`test_zone_art_pass_4.gd`, `lint_art.gd`)
- [x] T026 [P] [US1] Biển chỉ đường biểu tượng hình, vòng sáng bệ điểm neo, marker Tia Sáng lớn (owner 2026-11-26) — F094-AC-13 (`test_zone_art_pass_4.gd`)

### Implementation

- [x] T011 [US1] Thư viện Gió (`zone_practice_art.gd`, `practice_zone/Zone.tscn`)
- [x] T012 [US1] Vườn Bạn Bè (`zone_social_art.gd`, `social_zone/Zone.tscn`)
- [x] T013 [US1] Đại dương Khám Phá (`zone_ocean_art.gd`, `quest_zone_discovery_ocean/Zone.tscn`)
- [x] T014 [US1] Núi Thử Thách (`zone_mountain_art.gd`, `challenge_mountain/Zone.tscn`)
- [x] T015 [US1] Tháp Thử Thách (`zone_exam_art.gd`, `exam_zone/Zone.tscn`)
- [x] T016 [US1] Đấu trường Nhà Vô Địch (`zone_arena_art.gd`, `arena/Zone.tscn`)

## Phase 5: User Story 2 — Nhanh trên máy tầm trung và Web (P1)

- [x] T017 [US2] `test_zone_benchmark.gd` mười một vùng (proxy tĩnh) — F094-AC-04
- [x] T018 [US2] Benchmark có renderer sáu vùng × Mobile + Compatibility, giảm tới khi đạt — F094-AC-04 (quickstart §5)
- [x] T019 [US2] Ngân sách asset + `bake_art --check` — F094-AC-05
- [x] T020 [US2] Export Web preset F133 — F094-AC-06

## Phase 6: User Story 3 — Hợp đồng thế giới, license (P1)

- [x] T021 [US3] Điểm neo = seed, collision mặt đất giữ, collision mới không chắn spawn/điểm neo/lối đi — F094-AC-07 (`test_zone_art_pass_4.gd`, `zone_lint`, `gen_greybox_zones.mjs --check`)
- [x] T022 [US3] Dòng `LICENSES.md` + `lint_art` — F094-AC-08
- [x] T023 [US3] Ảnh sau Mobile (5 góc) + Web (3 góc) sáu vùng — F094-AC-09 (`screenshots/`)

## Phase 7: Polish & Cross-Cutting

- [x] T024 Tài liệu sở hữu: WORLD_DESIGN §4/§11, WORLD_RENDERING §2/§8.1/§10, PERFORMANCE_BUDGET §7, DESIGN_SYSTEM §5.3/§6, CONFIG_DEFAULTS, CODE_PLAN (F094), RTM, CHANGELOG, `design/CHANGELOG.md`, `assets/LICENSES.md`
- [x] T025 Gate: GdUnit4 toàn bộ, `check_typed`, `lint_theme_overrides`, `lint_layers`, `lint_signals`, `lint_art`, `zone_lint`, `extract_pot --check`, `pseudo_locale --check`, `gen_fixtures --check`, `check_no_admin_ui`, `bake_art --check`, `gen_greybox_zones.mjs --check`, `make design-tokens-check`, portal `pnpm lint` + typecheck + Vitest liên quan, check-features, check-cross-features, check-links, check-mermaid, build-rtm `--check`

## Dependencies & Execution Order

- Phase 1 → Phase 2 (token, prefab, catalog) → US4 (điểm mở F093) → US1 (từng vùng độc lập) → US2 (đo, giảm tới khi đạt) → US3 → Polish.

## Definition of Done

- Mọi F094-AC-* có test hoặc bằng chứng công cụ pass (quickstart §5); ảnh trước/sau trong `screenshots/`.
- Gate T025 pass; không literal màu; mọi asset có dòng license được phép.
- Tài liệu sở hữu, CODE_PLAN, RTM, CHANGELOG cập nhật trong cùng push.
