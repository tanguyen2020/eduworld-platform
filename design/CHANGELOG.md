# Changelog — design tokens

Phiên bản token theo semver (`design/package.json`, DESIGN_SYSTEM §3, §8). Đổi tên hoặc xóa token cần tăng MAJOR; token bị xóa phải mang `$deprecated` ít nhất một phiên bản trước (kiểm bởi `design/checks/semver.mjs`).

## tokens@0.10.0

- 2026-11-25 — Art pass 2 "chibi anime cartoon 3D" (F068, ADR-0103, DESIGN_SYSTEM §5.3): primitive `palette.art.{eye-iris,eye-white,ash,ash-dark}`, `number.toon.band-softness`, `number.light.blob-opacity`; art3d `art.palette.{eye_iris,eye_white,ash,ash_dark}` (slot 48–51), `toon.band_softness`, `light.{blob_shadow_color,blob_shadow_opacity}`. Đổi giá trị: `number.toon.steps` 3 → 2 (dải cel rõ: tối / giữa / sáng), `palette.zone.outline` `#1E2A44` → `#3A2C3F` (viền mực tím ấm). Sinh thêm preset `resources/art/generated/mat_toon_skinned.tres` (viền theo NORMAL đã skin cho nhân vật có xương) và uniform `band_softness` trong mọi preset toon. MINOR vì chỉ thêm token và đổi giá trị.

## tokens@0.9.0

- 2026-11-25 — Output mới `src/game/web/shell_tokens.css` (F133 T028): biến CSS `--ew-*` cho Web shell của game (trang trước khi engine chạy), lấy từ token game: `color.{surface,text,accent,feedback}.*`, `color.focus`, `size.{space,radius,border,layout}.*`, `font.family.*` và density cơ sở `g6_7` dưới dạng `--ew-density-*`. Check separation coi file này là output game (không token admin). Không thêm/đổi token nên giữ phiên bản.

- 2026-11-24 — Màn đăng nhập game S01 làm lại cho học sinh (owner: "giao diện đăng nhập của game bị vỡ và không phù hợp cho học sinh"; STUDENT_GAME_UX_CORE S01, DESIGN_SYSTEM §5.1–§5.2): primitive `palette.{green.300,green.400,red.50,pink.200,sand.200,sand.400,shadow.soft}`, `size.font.48`, `size.border.8`, `size.icon.{32,40}`, `size.layout.{200,280,660}`; component `component.button.secondary.{bg_hover,bg_pressed}`, `component.icon.*` (cỡ icon nút, ô checkbox, màu icon), `component.chip.*`, `component.text_button.text`, `component.inline_error.*`, `component.info_banner.*`, `component.form_card.*` (rộng thẻ 660, cột hai bên 280, padding 20, bóng mềm), `component.speech_bubble.*`, `component.logo.*` (wordmark Baloo 2 48 px viền trắng, Tia Sáng), `component.backdrop.*` (trời ấm, đồi, đảo nổi), `component.mascot.*` (Mây), mọi cặp chữ/icon có `contrastWith`. Theme sinh thêm biến thể `IconButton` (vuông, chạm ≥ chiều cao tối thiểu của khối), `ChoiceChip`, `TextButton`, `InlineErrorPanel`/`Label`, `InfoBannerPanel`/`Label`, `FormCard`, `SpeechBubble`/`Label`, `LogoLabel`, `FormBox`/`FieldBox`/`ToolbarBox`, `ScreenMargin`; `CheckBox` có ô 40 px và vùng chạm theo khối; `LineEdit` cao bằng chiều cao tối thiểu của khối (trước đây thấp hơn), có kiểu `read_only` và màu nút xóa; `Button/styles/focus` (mọi nút và biến thể) thành vòng viền `component.focus_ring.*` vẽ ngoài control (trước đây phủ nền xanh, nút phụ trông như nút chính); `SecondaryButton` giữ màu chữ khi focus. Padding dọc của nút/ô nhập tính từ chiều cao dòng thật của font (fontkit) thay cho hệ số 1,3. Bộ icon game SVG đầu tiên trong `design/icons/game/` (globe, eye, eye_off, help, info, alert, check, checkbox_on/off; màu là placeholder `{token}` do pipeline thay) sinh vào `src/game/resources/ui/icons/`. MINOR vì chỉ thêm token và đổi giá trị.

## tokens@0.8.0

- 2026-11-24 — Art pass 1 (F067, ADR-0102, DESIGN_SYSTEM §5.3): primitive `palette.art.*` (43 màu mỹ thuật low-poly: cỏ, gạch, ngói, gỗ, lá, nước, lúa, đèn lồng, mặt nhân vật…) và `number.light.{sun,ambient,fog,exposure}`; art3d `art.palette.<tên>` (48 slot cố định qua `$extensions.eduworld.index`, gồm `subject_*` alias màu môn học và `skin_npc`), `light.{sun_energy,ambient_energy,ambient_color,fog_density,exposure}`. Sinh mới `resources/art/generated/mat_toon.tres` (toon + viền) và `mat_toon_no_outline.tres`, `Art3dTokens.ART_PALETTE`, `ART_PALETTE_COLORS`, `MATERIAL_PATHS`, `LIGHT_*`; Environment mọi zone thêm ambient phẳng màu ấm, tonemap linear, `fog_density`. MINOR vì chỉ thêm token.

## tokens@0.7.0

- 2026-11-22 — Sidebar Admin Portal tông xanh thương hiệu đậm (owner yêu cầu, ADR-0098, DESIGN_SYSTEM §4.1): primitive `ops.palette.navy.{100,150,700,800,850,900}`; mode `ops.mode.sider.{bg,hover-bg,selected-bg,text,selected-text}` (light: nền `#163A7A`, chữ `#DCE6FA`, mục chọn là ô trắng chữ xanh đậm; dark: nền `#0E1C3A`, chữ `#C9D6F2`, mục chọn nền brand primary chữ trắng); semantic `ops.color.sider.{bg,hover-bg,selected-bg,text.default,text.selected}` có cặp tương phản (chữ ≥ 4,5:1, ô chọn ≥ 3:1 so với nền sidebar); component token `antd.components.Layout.siderBg` đổi sang `ops.color.sider.bg` và `antd.components.Menu.dark{ItemBg,SubMenuItemBg,PopupBg,ItemColor,ItemHoverBg,ItemHoverColor,ItemSelectedBg,ItemSelectedColor}` (menu sidebar dùng `theme="dark"`). Thang khoảng cách `ops.size.space.{4,8,12,16,24,32,48}` (sinh `--ew-ops-size-space-*`) cho lớp tiện ích Tailwind (ADR-0098). MINOR vì chỉ thêm token và đổi giá trị.

## tokens@0.6.0

- 2026-11-21 — Giao diện Admin hướng B "Sổ tay giáo viên" và màn đăng nhập hướng A "Sân trường" (owner chốt, DESIGN_SYSTEM §4.1, ADMIN_PORTAL §1.3b): primitive `ops.palette.slate.{50,200,600,900}`, `ops.palette.neutral.800`, `ops.size.font.{15,18,20,24,28,30}`, `ops.size.radius.{8,12}`, `ops.size.control.{36,40,48}`, `ops.size.border.4`; semantic `ops.color.border-subtle`, `dashboard.accent.width`; seed `antd.colorBgLayout`, `colorBorderSecondary`, `fontSizeHeading1…5`, `borderRadiusLG`; component token `antd.components.{Layout,Card,Menu,Statistic}.*` (sinh vào `ThemeConfig.components`); theme đăng nhập `auth.antd.*` (sinh `authTheme`). Đổi giá trị: `fontSize` 14 → 15, `borderRadius` 6 → 8, `controlHeight` 32 → 36, nền trang sáng `#F5F5F5` → `#F7F8FA`, chữ sáng `#1F1F1F`/`#595959` → `#1F2430`/`#4B5263`; compact giữ 13/28. MINOR vì chỉ thêm token và đổi giá trị.

## tokens@0.5.0

- 2026-11-21 — Thêm `component.exam_clock.warning_text` (alias `color.feedback.incorrect`) và biến thể Theme `ExamClockWarningLabel` của game: đồng hồ bài kiểm tra đổi sang một màu cảnh báo nhẹ, cố định khi còn 5 phút và 1 phút (F064-AC-04, STUDENT_GAME_UX_LEARNING §9.4). MINOR vì chỉ thêm token.

## tokens@0.4.0

- 2026-11-21 — Thêm bảng màu avatar (F070 T041, OQ F070-2): `palette.avatar.*` (6 màu da, 8 màu tóc) trong `game-tokens/primitive.json` và `avatar.skin_tone.<mã>`, `avatar.hair_color.<mã>` trong `game-tokens/art3d.json` — khóa là mã `skin_tones`/`hair_colors` của danh mục nhân vật; sinh `Art3dTokens.AVATAR_SKIN_TONES`/`AVATAR_HAIR_COLORS`, thay resource placeholder `src/game/resources/avatar/{tone_*,hair_col_*}.tres`. Giá trị giữ nguyên màu placeholder [Proposed] đến khi có art thật (OQ-16). MINOR vì chỉ thêm token.

## tokens@0.3.0

- 2026-11-19 — Thêm seed Ant Design `antd.colorTextPlaceholder` (alias `ops.color.text.secondary`): mặc định `#bfbfbf` chỉ đạt 1,83:1, axe chặn placeholder "Tất cả tenant" của bộ lọc tenant (F051-AC-18, F051-AC-22). MINOR vì chỉ thêm token.

## tokens@0.2.0

- 2026-11-19 — Thêm seed Ant Design `antd.colorText`, `colorTextSecondary`, `colorTextTertiary`, `colorTextDescription` (alias `ops.color.text.*`): giá trị mặc định của Ant Design (độ mờ 0,45) chỉ đạt 3,36:1, bị axe chặn trong Storybook (F050-AC-19). MINOR vì chỉ thêm token.

## tokens@0.1.0

- 2026-11-19 — Phiên bản đầu (F005): `shared-tokens` (brand, môn học/khu vực, độ hiếm, trạng thái nội dung — mỗi màu có biến thể `light`/`dark`/`game`), `game-tokens` (primitive, semantic, component, art3d, density `g1_3`…`g8_9`, chế độ tương phản cao), `admin-tokens` (seed Ant Design, semantic, mode `light`/`dark`/`compact`); font Be Vietnam Pro + Baloo 2 (OFL). Giá trị khởi tạo [Proposed] theo DESIGN_SYSTEM §4.1, §5.2, §5.5, §6; `color.brand.primary` (light) đổi `#2F6BFF` → `#2A62F0` để chữ trắng đạt 4.5:1.
