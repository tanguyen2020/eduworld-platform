# Tasks: F005 — Design tokens game/admin/shared và pipeline sinh theme

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Khởi tạo `design/package.json` (style-dictionary@4, fontkit, script `build`, `check`, `test`), `design/README.md` quy trình đổi token, `design/CHANGELOG.md` `tokens@0.1.0` — file: `design/package.json` — **xong 2026-11-19 — `design/package.json` (`@eduworld/design-tokens@0.1.0`, style-dictionary 4.4.0, fontkit 2.0.4, pnpm 10.34.9, script `build`/`check`/`test`), `design/README.md`, `design/CHANGELOG.md` mục `tokens@0.1.0`**
- [x] T002 [P] Target `make design-tokens` (build web + JSON Godot + Godot headless) và `make design-tokens-check` (build vào thư mục tạm rồi so) — file: `Makefile` — **xong 2026-11-19 — `make design-tokens` (`cd design && pnpm install --frozen-lockfile && pnpm build`), `make design-tokens-check` (`pnpm test` + `pnpm check`: sinh trong bộ nhớ rồi so byte với file commit, `BASE_REF` cho semver); Godot không cần (Clarification 2026-11-19)**
- [x] T003 [P] Ghim phiên bản Godot nếu chưa có — file: `src/game/.godot-version` — **xong 2026-11-19 — `src/game/.godot-version` = `4.5.1` [Confirmed, F060 xác nhận]**

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Token nguồn shared (`brand`, `domain` có biến thể light/dark/game) — file: `design/shared-tokens/domain.json` — **xong 2026-11-19 — `shared-tokens/brand.json`, `domain.json`: 17 màu nghiệp vụ, mỗi màu `variants` light/dark/game**
- [x] T005 [P] Token game: primitive, semantic (có `contrastWith`), component, density 4 band theo DESIGN_SYSTEM §5.2, art3d — file: `design/game-tokens/semantic.json` — **xong 2026-11-19 — `game-tokens/{primitive,semantic,component,art3d,high-contrast}.json`, `density/{g1_3,g4_5,g6_7,g8_9}.json` (bảng §5.2), `contrastWith` trên mọi token chữ**
- [x] T006 [P] Token admin: seed (DESIGN_SYSTEM §4.1), semantic (status, chart 8 màu, banner, `math.font-scale`, `contrastWith`), modes light/dark/compact — file: `design/admin-tokens/seed.json` — **xong 2026-11-19 — `admin-tokens/seed.json` (seed Ant Design §4.1 + palette `ops.*`), `semantic.json` (status, chart 8 màu, banner, `math.font-scale`, `contrastWith`), `modes/{light,dark,compact}.json`**
- [x] T007 [P] Font OFL + LICENSE, icon nguồn tách `game/`, `admin/`, `shared/` — file: `design/fonts/BeVietnamPro/LICENSE` — **xong 2026-11-19 — `fonts/BeVietnamPro/` (400/500/600/700) + `fonts/Baloo2/` (700) + `LICENSE` OFL 1.1; `icons/{game,admin,shared}/`**
- [x] T008 Cấu hình Style Dictionary (platform `web-antd`, `web-css`, `godot-json`, `gdscript`) — file: `design/style-dictionary.config.mjs` — **xong 2026-11-19 — `style-dictionary.config.mjs` (5 context `web-light`, `web-dark`, `web-compact`, `game`, `game-high-contrast`; file primitive; đầu ra) + `sd/transforms.mjs` (`ew/name/css`, `ew/name/gd`, `ew/color/hex-upper`)**

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Một nguồn token sinh theme cho game và web (Priority: P1) 🎯 MVP

**Goal**: Pipeline sinh đủ đầu ra, check file sinh, tách hai design system.

**Independent Test**: Như spec.md User Story 1.

### Tests cho User Story 1

- [x] T009 [P] [US1] Test F005-AC-01: validator cấu trúc thư mục + thuộc tính W3C bắt buộc — file: `design/checks/structure.test.mjs` — phải FAIL trước khi code — **xong 2026-11-19 — `checks/structure.test.mjs` 4/4 pass**
- [x] T010 [P] [US1] Test F005-AC-02: snapshot đầu ra 4 format từ fixture token, dòng đầu GENERATED (TC-UI-001) — file: `design/sd/formats/formats.test.mjs` — **xong 2026-11-19 — `sd/formats/formats.test.mjs` 7/7 pass (snapshot CSS/antd từ fixture, hằng GDScript, JSON, kiểm cấu trúc mọi `.tres`)**
- [x] T011 [P] [US1] Test F005-AC-03: sửa tay file sinh trong thư mục tạm → `check` fail (TC-UI-001) — file: `design/checks/generated.test.mjs` — **xong 2026-11-19 — `checks/generated.test.mjs` 4/4 pass (sửa tay `tokens.css` → `generated-drift`; thiếu/thừa file; bỏ qua `.import`/`.uid`)**
- [x] T012 [P] [US1] Test F005-AC-04: alias hỏng, vòng lặp, literal ở semantic → fail — file: `design/checks/aliases.test.mjs` — **xong 2026-11-19 — `checks/aliases.test.mjs` 7/7 pass**
- [x] T013 [P] [US1] Test F005-AC-05, F005-AC-06: màu nghiệp vụ khai báo ngoài shared → fail; thiếu biến thể → fail; giá trị trùng giữa đầu ra game và web (TC-UI-008) — file: `design/checks/domain-variants.test.mjs` — **xong 2026-11-19 — `checks/domain-variants.test.mjs` 5/5 pass**
- [x] T014 [P] [US1] Test F005-AC-07: đầu ra Godot không có token admin, đầu ra web không có token game (TC-UI-010) — file: `design/checks/separation.test.mjs` — **xong 2026-11-19 — `checks/separation.test.mjs` 4/4 pass**
- [x] T015 [P] [US1] Test GdUnit4: `.tres` sinh nạp được, có biến thể density, `UiTokens` compile typed — file: `src/game/tests/unit/theme/test_generated_theme.gd` — **xong 2026-11-19 — viết `src/game/tests/unit/theme/test_generated_theme.gd` (GdUnit4, typed); **chưa chạy** vì môi trường không có Godot — chạy trong lane `godot.yml` khi F060 thêm project + GdUnit4; cấu trúc `.tres` đã kiểm bằng `formats.test.mjs`**

### Implementation cho User Story 1

- [x] T016 [P] [US1] Transform + format `antd-theme.mjs`, `css-vars.mjs` — file: `design/sd/formats/antd-theme.mjs` — **xong 2026-11-19 — `sd/formats/antd-theme.mjs` (`lightTheme`/`darkTheme`/`compactTheme`, `compactOverrides`, `domainColors`), `sd/formats/css-vars.mjs` (`:root`, `[data-theme="dark"]`, `[data-density="compact"]`)**
- [x] T017 [P] [US1] Format `godot-json.mjs`, `gdscript-tokens.mjs` — file: `design/sd/formats/gdscript-tokens.mjs` — **xong 2026-11-19 — `sd/formats/godot-json.mjs`, `sd/formats/gdscript-tokens.mjs` (`UiTokens`, `Art3dTokens`)**
- [x] T018 [US1] Script Godot headless dựng `Theme`, biến thể density, `ShaderMaterial`/`Environment`/`Gradient` từ JSON trung gian — file: `src/game/tools/build_theme.gd` — **xong 2026-11-19 — thay bằng `sd/formats/godot-tres.mjs` ghi trực tiếp `Theme` (gốc + 4 density + tương phản cao), `Environment` 5 zone, `Gradient` 3 VFX (Clarification 2026-11-19); không tạo `build_theme.gd`**
- [x] T019 [US1] Bộ kiểm `aliases.mjs`, `domain-variants.mjs`, `separation.mjs`, `generated.mjs` gọi từ `pnpm --dir design check` — file: `design/checks/aliases.mjs` — **xong 2026-11-19 — `checks/{structure,aliases,domain-variants,separation,generated,contrast,min-sizes,fonts,semver}.mjs`, `checks/index.mjs`, `checks/run-all.mjs` (`pnpm check`)**

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Token bảo đảm trợ năng (Priority: P1)

**Goal**: Kiểm tương phản, cỡ chữ tối thiểu, font.

**Independent Test**: Như spec.md User Story 2.

### Tests cho User Story 2

- [x] T020 [P] [US2] Test F005-AC-08: cặp dưới 4.5:1 / 3:1 fail, báo cáo JSON; token chữ thiếu `contrastWith` fail (TC-UI-002) — file: `design/checks/contrast.test.mjs` — phải FAIL trước khi code — **xong 2026-11-19 — `checks/contrast.test.mjs` 6/6 pass; báo cáo `design/build/reports/contrast.json` 188/188 cặp đạt**
- [x] T021 [P] [US2] Test F005-AC-09, F005-AC-10: `g1_3` và mọi density dưới bảng DESIGN_SYSTEM §5.2 fail; compact dưới DESIGN_SYSTEM §4.5 fail — file: `design/checks/min-sizes.test.mjs` — **xong 2026-11-19 — `checks/min-sizes.test.mjs` 5/5 pass (ngưỡng đọc từ DESIGN_SYSTEM §5.2, §4.5)**
- [x] T022 [P] [US2] Test F005-AC-11: `TEXT_EXPANSION_ALLOWANCE` và `--ew-math-font-scale` có trong đầu ra — file: `design/sd/formats/a11y-tokens.test.mjs` — **xong 2026-11-19 — `sd/formats/a11y-tokens.test.mjs` 1/1 pass**
- [x] T023 [P] [US2] Test F005-AC-12: font thiếu LICENSE hoặc thiếu glyph trong bộ ký tự tiếng Việt kiểm → fail — file: `design/checks/fonts.test.mjs` — **xong 2026-11-19 — `checks/fonts.test.mjs` 5/5 pass**

### Implementation cho User Story 2

- [x] T024 [US2] `contrast.mjs` (WCAG 2.1 relative luminance, mức normal/large, mọi mode/density + tương phản cao) — file: `design/checks/contrast.mjs` — **xong 2026-11-19 — `checks/contrast.mjs` (WCAG 2.1, `normal`/`large`/`graphic`, 5 context × 4 density)**
- [x] T025 [P] [US2] `min-sizes.mjs` (ngưỡng đọc từ bảng cấu hình trích DESIGN_SYSTEM §4.5, §5.2) — file: `design/checks/min-sizes.mjs` — **xong 2026-11-19 — `checks/min-sizes.mjs`**
- [x] T026 [P] [US2] `fonts.mjs` (fontkit, bộ ký tự tiếng Việt đầy đủ + Latin Extended) — file: `design/checks/fonts.mjs` — **xong 2026-11-19 — `checks/fonts.mjs` (fontkit; bộ glyph theo Clarification 2026-11-19)**

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — Thay đổi token có phiên bản và kích hoạt client (Priority: P2)

**Goal**: Semver, deprecated, CI kích hoạt.

**Independent Test**: Như spec.md User Story 3.

### Tests cho User Story 3

- [x] T027 [P] [US3] Test F005-AC-13, F005-AC-14: so token với tag phiên bản trước (git), đổi tên không tăng MAJOR → fail; xóa token chưa deprecated → fail — file: `design/checks/semver.test.mjs` — phải FAIL trước khi code — **xong 2026-11-19 — `checks/semver.test.mjs` 7/7 pass (gồm repo git tạm)**
- [x] T028 [P] [US3] Test F005-AC-15: `paths` filter — thay đổi `design/**` kích hoạt `design.yml` và lượt gọi `godot.yml`, `web.yml` — file: `scripts/ci/test-design-rules.sh` — **xong 2026-11-19 — `scripts/ci/test-design-rules.sh` → `design-rules.test.mjs` 4/4 pass; `path-filters.test.mjs` 22/22 pass (thêm case design/web/godot)**

### Implementation cho User Story 3

- [x] T029 [US3] `semver.mjs` (đọc `design/package.json` version + snapshot tên token tag trước) — file: `design/checks/semver.mjs` — **xong 2026-11-19 — `checks/semver.mjs`**
- [x] T030 [US3] Workflow `design.yml` (build, check, artifact báo cáo; action ghim SHA) + `workflow_call` tới `godot.yml`, `web.yml` — file: `.github/workflows/design.yml` — **xong 2026-11-19 — `.github/workflows/design.yml` (check + artifact báo cáo, action ghim SHA) gọi `godot.yml`, `web.yml` (lane khung `workflow_call`); `check-workflows` 0 lỗi**

**Checkpoint**: User Story 3 chạy và kiểm độc lập được.

## Phase N: Polish & Cross-Cutting

- [x] T031 [P] Cập nhật DESIGN_SYSTEM §3 (bỏ [Confirmed] cho `build_theme.gd` khi đã chạy), ghi phiên bản token đầu tiên — file: `docs/05-ux/DESIGN_SYSTEM.md` — **xong 2026-11-19 — DESIGN_SYSTEM §3 (pipeline ghi `.tres` trực tiếp, bộ kiểm, `tokens@0.1.0`), §4.1 (`colorPrimary`, compact không dùng `compactAlgorithm`)**
- [x] T032 [P] CHANGELOG mục F005 — file: `CHANGELOG.md` — **xong 2026-11-19 — CHANGELOG mục F005**
- [x] T033 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/005-design-tokens/quickstart.md` — **xong 2026-11-19 — quickstart.md mục Bằng chứng**

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Chưa làm.

- [ ] T034 **OQ chốt 2026-11-20** (OQ F051-4) Token `ops.breakpoint.desktop` = 1280 px và kích thước vùng chạm 44 px trong `design/admin-tokens` (MINOR), `make design-tokens`; portal shell dùng token thay hằng — file: `design/admin-tokens/`, `src/cms/frontend/apps/portal/src/shell/` — kiểm: `make design-tokens-check`, `make web-test` — cỡ S

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 (cần đầu ra US1) → US3 → Polish.
- T008 trước T016–T018; T018 cần Godot headless trong runner CI.
- F050 và F060 dùng đầu ra sau US1.

## Definition of Done

- [x] Mọi tiêu chí F005-AC-* có test pass (ghi mã test/TC) — **F005-AC-01…15 có test pass (xem quickstart §Bằng chứng)**
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh — **không áp dụng Go; `pnpm test` 56/56, `pnpm check` OK, `ci-local` phần design OK; `make godot-test` chờ F060**
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — **đạt; không mock**
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi — **DESIGN_SYSTEM, CHANGELOG, design/CHANGELOG**
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật — **CODE_PLAN: Done**
