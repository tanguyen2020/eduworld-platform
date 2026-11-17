# Tasks: F005 — Design tokens game/admin/shared và pipeline sinh theme

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Khởi tạo `design/package.json` (style-dictionary@4, fontkit, script `build`, `check`, `test`), `design/README.md` quy trình đổi token, `design/CHANGELOG.md` `tokens@0.1.0` — file: `design/package.json`
- [ ] T002 [P] Target `make design-tokens` (build web + JSON Godot + Godot headless) và `make design-tokens-check` (build vào thư mục tạm rồi so) — file: `Makefile`
- [ ] T003 [P] Ghim phiên bản Godot nếu chưa có — file: `godot/.godot-version`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Token nguồn shared (`brand`, `domain` có biến thể light/dark/game) — file: `design/shared-tokens/domain.json`
- [ ] T005 [P] Token game: primitive, semantic (có `contrastWith`), component, density 4 band theo DESIGN_SYSTEM §5.2, art3d — file: `design/game-tokens/semantic.json`
- [ ] T006 [P] Token admin: seed (DESIGN_SYSTEM §4.1), semantic (status, chart 8 màu, banner, `math.font-scale`, `contrastWith`), modes light/dark/compact — file: `design/admin-tokens/seed.json`
- [ ] T007 [P] Font OFL + LICENSE, icon nguồn tách `game/`, `admin/`, `shared/` — file: `design/fonts/BeVietnamPro/LICENSE`
- [ ] T008 Cấu hình Style Dictionary (platform `web-antd`, `web-css`, `godot-json`, `gdscript`) — file: `design/style-dictionary.config.mjs`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Một nguồn token sinh theme cho game và web (Priority: P1) 🎯 MVP

**Goal**: Pipeline sinh đủ đầu ra, check file sinh, tách hai design system.

**Independent Test**: Như spec.md User Story 1.

### Tests cho User Story 1

- [ ] T009 [P] [US1] Test F005-AC-01: validator cấu trúc thư mục + thuộc tính W3C bắt buộc — file: `design/checks/structure.test.mjs` — phải FAIL trước khi code
- [ ] T010 [P] [US1] Test F005-AC-02: snapshot đầu ra 4 format từ fixture token, dòng đầu GENERATED (TC-UI-001) — file: `design/sd/formats/formats.test.mjs`
- [ ] T011 [P] [US1] Test F005-AC-03: sửa tay file sinh trong thư mục tạm → `check` fail (TC-UI-001) — file: `design/checks/generated.test.mjs`
- [ ] T012 [P] [US1] Test F005-AC-04: alias hỏng, vòng lặp, literal ở semantic → fail — file: `design/checks/aliases.test.mjs`
- [ ] T013 [P] [US1] Test F005-AC-05, F005-AC-06: màu nghiệp vụ khai báo ngoài shared → fail; thiếu biến thể → fail; giá trị trùng giữa đầu ra game và web (TC-UI-008) — file: `design/checks/domain-variants.test.mjs`
- [ ] T014 [P] [US1] Test F005-AC-07: đầu ra Godot không có token admin, đầu ra web không có token game (TC-UI-010) — file: `design/checks/separation.test.mjs`
- [ ] T015 [P] [US1] Test GdUnit4: `.tres` sinh nạp được, có biến thể density, `UiTokens` compile typed — file: `godot/tests/unit/theme/test_generated_theme.gd`

### Implementation cho User Story 1

- [ ] T016 [P] [US1] Transform + format `antd-theme.mjs`, `css-vars.mjs` — file: `design/sd/formats/antd-theme.mjs`
- [ ] T017 [P] [US1] Format `godot-json.mjs`, `gdscript-tokens.mjs` — file: `design/sd/formats/gdscript-tokens.mjs`
- [ ] T018 [US1] Script Godot headless dựng `Theme`, biến thể density, `ShaderMaterial`/`Environment`/`Gradient` từ JSON trung gian — file: `tools/godot/build_theme.gd`
- [ ] T019 [US1] Bộ kiểm `aliases.mjs`, `domain-variants.mjs`, `separation.mjs`, `generated.mjs` gọi từ `pnpm --dir design check` — file: `design/checks/aliases.mjs`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Token bảo đảm trợ năng (Priority: P1)

**Goal**: Kiểm tương phản, cỡ chữ tối thiểu, font.

**Independent Test**: Như spec.md User Story 2.

### Tests cho User Story 2

- [ ] T020 [P] [US2] Test F005-AC-08: cặp dưới 4.5:1 / 3:1 fail, báo cáo JSON; token chữ thiếu `contrastWith` fail (TC-UI-002) — file: `design/checks/contrast.test.mjs` — phải FAIL trước khi code
- [ ] T021 [P] [US2] Test F005-AC-09, F005-AC-10: `g1_3` và mọi density dưới bảng DESIGN_SYSTEM §5.2 fail; compact dưới DESIGN_SYSTEM §4.5 fail — file: `design/checks/min-sizes.test.mjs`
- [ ] T022 [P] [US2] Test F005-AC-11: `TEXT_EXPANSION_ALLOWANCE` và `--ew-math-font-scale` có trong đầu ra — file: `design/sd/formats/a11y-tokens.test.mjs`
- [ ] T023 [P] [US2] Test F005-AC-12: font thiếu LICENSE hoặc thiếu glyph trong bộ ký tự tiếng Việt kiểm → fail — file: `design/checks/fonts.test.mjs`

### Implementation cho User Story 2

- [ ] T024 [US2] `contrast.mjs` (WCAG 2.1 relative luminance, mức normal/large, mọi mode/density + tương phản cao) — file: `design/checks/contrast.mjs`
- [ ] T025 [P] [US2] `min-sizes.mjs` (ngưỡng đọc từ bảng cấu hình trích DESIGN_SYSTEM §4.5, §5.2) — file: `design/checks/min-sizes.mjs`
- [ ] T026 [P] [US2] `fonts.mjs` (fontkit, bộ ký tự tiếng Việt đầy đủ + Latin Extended) — file: `design/checks/fonts.mjs`

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — Thay đổi token có phiên bản và kích hoạt client (Priority: P2)

**Goal**: Semver, deprecated, CI kích hoạt.

**Independent Test**: Như spec.md User Story 3.

### Tests cho User Story 3

- [ ] T027 [P] [US3] Test F005-AC-13, F005-AC-14: so token với tag phiên bản trước (git), đổi tên không tăng MAJOR → fail; xóa token chưa deprecated → fail — file: `design/checks/semver.test.mjs` — phải FAIL trước khi code
- [ ] T028 [P] [US3] Test F005-AC-15: `paths` filter — thay đổi `design/**` kích hoạt `design.yml` và lượt gọi `godot.yml`, `web.yml` — file: `scripts/ci/test-design-rules.sh`

### Implementation cho User Story 3

- [ ] T029 [US3] `semver.mjs` (đọc `design/package.json` version + snapshot tên token tag trước) — file: `design/checks/semver.mjs`
- [ ] T030 [US3] Workflow `design.yml` (build, check, artifact báo cáo; action ghim SHA) + `workflow_call` tới `godot.yml`, `web.yml` — file: `.github/workflows/design.yml`

**Checkpoint**: User Story 3 chạy và kiểm độc lập được.

## Phase N: Polish & Cross-Cutting

- [ ] T031 [P] Cập nhật DESIGN_SYSTEM §3 (bỏ [Proposed] cho `build_theme.gd` khi đã chạy), ghi phiên bản token đầu tiên — file: `docs/05-ux/DESIGN_SYSTEM.md`
- [ ] T032 [P] CHANGELOG mục F005 — file: `CHANGELOG.md`
- [ ] T033 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/005-design-tokens/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 (cần đầu ra US1) → US3 → Polish.
- T008 trước T016–T018; T018 cần Godot headless trong runner CI.
- F050 và F060 dùng đầu ra sau US1.

## Definition of Done

- [ ] Mọi tiêu chí F005-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
