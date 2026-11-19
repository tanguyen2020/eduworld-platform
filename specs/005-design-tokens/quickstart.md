# Quickstart: F005 — Design tokens game/admin/shared và pipeline sinh theme

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F001
- Công cụ: Node.js LTS + pnpm, Godot headless đúng `src/game/.godot-version` ([LOCAL_DEVELOPMENT §1](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Lệnh: `make design-tokens`, `make design-tokens-check`.
- Tài khoản demo: không cần.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `tree design/`; mở một token bất kỳ | Cấu trúc DESIGN_SYSTEM §2; có `$value`, `$type`, `$description` | F005-AC-01 |
| 2 | `make design-tokens`; liệt kê file sinh | 5 nhóm đầu ra, dòng đầu "GENERATED" | F005-AC-02 |
| 3 | Sửa tay một màu trong `tokens.css`; `make design-tokens-check` | Fail, liệt kê `tokens.css` | F005-AC-03 |
| 4 | Thêm alias `{color.khong.ton.tai}`; thêm literal ở semantic | Build fail, chỉ rõ đường dẫn | F005-AC-04 |
| 5 | Thêm `color.subject.math` vào `admin-tokens/semantic.json` | Build fail | F005-AC-05 |
| 6 | So `color.subject.math` trong `antd-theme.ts` (`domainColors`) và `ui_tokens.gd` | Cùng tên, giá trị theo biến thể | F005-AC-05 |
| 7 | Xóa biến thể `dark` của `color.rarity.epic` | Build fail | F005-AC-06 |
| 8 | Grep `antd`/`ConfigProvider` trong đầu ra Godot; grep `game-tokens` trong đầu ra web | 0 kết quả | F005-AC-07 |
| 9 | Đổi `color.text.secondary` (light) thành màu nhạt; build | Fail, `reports/contrast.json` chỉ cặp lỗi | F005-AC-08 |
| 10 | Hạ `density.g1_3.font.base` dưới ngưỡng | Build fail | F005-AC-09 |
| 11 | Hạ cỡ chữ compact dưới ngưỡng | Build fail | F005-AC-10 |
| 12 | Xem `UiTokens.TEXT_EXPANSION_ALLOWANCE`, `--ew-math-font-scale` | Có, giá trị theo nguồn | F005-AC-11 |
| 13 | Xóa `LICENSE` của font; dùng font thiếu glyph "ỹ" | Build fail | F005-AC-12 |
| 14 | PR đổi tên token không tăng MAJOR; xóa token chưa `$deprecated` | CI fail | F005-AC-13, F005-AC-14 |
| 15 | PR chỉ sửa `design/shared-tokens/brand.json` | Workflow `design.yml` chạy rồi gọi `godot.yml`, `web.yml` | F005-AC-15 |

## 3. Lệnh test tự động

```bash
pnpm --dir design test          # node --test cho transform + checks
make design-tokens-check
godot --headless --path src/game -s addons/gdUnit4/bin/GdUnitCmdTool.gd -a res://tests/unit/theme
```

## 4. Bằng chứng cần nộp

- Báo cáo tương phản (`design/build/reports/contrast.json`), log `design-tokens-check` xanh, ảnh lượt chạy GitHub Actions của PR kịch bản 15.

## Bằng chứng (2026-11-19, local — Windows, Node 20.19, pnpm 10.34.9; không có Godot)

| Tiêu chí | Bằng chứng tự động | Kết quả |
|---|---|---|
| F005-AC-01 | `checks/structure.test.mjs` (cấu trúc §2, `$value`/`$type`/`$description`) | 4/4 pass |
| F005-AC-02 | `sd/formats/formats.test.mjs` (4 format, dòng GENERATED, `.tres` hợp lệ cấu trúc); `make design-tokens` ghi 23 file (2 web, 6 Theme, 5 `env_*`, 3 `vfx_*`, `art3d_tokens.gd`, `ui_tokens.gd`, 3 font + 2 OFL) | 7/7 pass |
| F005-AC-03 | `checks/generated.test.mjs` (sửa tay `tokens.css` → `generated-drift` liệt kê file; thiếu/thừa file) | 4/4 pass |
| F005-AC-04 | `checks/aliases.test.mjs` (`{color.khong.ton.tai}`, vòng lặp, literal ở semantic/mode/art) | 7/7 pass |
| F005-AC-05, 06 | `checks/domain-variants.test.mjs` (TC-UI-008: khai báo lại trong admin/game, thiếu `dark` của `color.rarity.epic`, cùng tên + giá trị giữa `domainColors` và `UiTokens.DOMAIN_COLORS`) | 5/5 pass |
| F005-AC-07 | `checks/separation.test.mjs` (TC-UI-010) | 4/4 pass |
| F005-AC-08 | `checks/contrast.test.mjs` (TC-UI-002); báo cáo `design/build/reports/contrast.json` | 6/6 pass; 188/188 cặp đạt |
| F005-AC-09, 10 | `checks/min-sizes.test.mjs` | 5/5 pass |
| F005-AC-11 | `sd/formats/a11y-tokens.test.mjs` (`UiTokens.TEXT_EXPANSION_ALLOWANCE = 1.35`, `--ew-math-font-scale: 1`) | 1/1 pass |
| F005-AC-12 | `checks/fonts.test.mjs` (xóa LICENSE, LICENSE không OFL, glyph thiếu) | 5/5 pass |
| F005-AC-13, 14 | `checks/semver.test.mjs` (đổi tên không MAJOR, xóa chưa `$deprecated`, repo git tạm) | 7/7 pass |
| F005-AC-15 | `scripts/ci/test-design-rules.sh` (path filter `design/**`, `make design-tokens-check`, artifact, `uses: ./.github/workflows/{godot,web}.yml`); `path-filters.test.mjs`; `check-workflows.mjs` | 4/4, 22/22 pass; 0 lỗi |

Lệnh: `cd design && pnpm test` → 56/56 pass (≈ 11 s); `pnpm check` → `design-tokens-check: OK` (23 file khớp; semver bỏ qua vì `origin/main` chưa có `design/`). GdUnit4 `src/game/tests/unit/theme/test_generated_theme.gd` đã viết nhưng **chưa chạy** (không có Godot; chạy ở lane `godot.yml` khi F060 thêm project + GdUnit4). Lượt chạy GitHub Actions của `design.yml` diễn ra khi đẩy lên `main`.
