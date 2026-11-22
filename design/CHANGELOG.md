# Changelog — design tokens

Phiên bản token theo semver (`design/package.json`, DESIGN_SYSTEM §3, §8). Đổi tên hoặc xóa token cần tăng MAJOR; token bị xóa phải mang `$deprecated` ít nhất một phiên bản trước (kiểm bởi `design/checks/semver.mjs`).

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
