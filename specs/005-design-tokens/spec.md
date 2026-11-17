# Feature Specification: F005 — Design tokens game/admin/shared và pipeline sinh theme

**Feature Branch**: `005-design-tokens` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `design/` · **Mốc**: M-C0 · **Phụ thuộc**: F001
**Input**: Nguồn token giao diện duy nhất cho hai design system (game Godot và Admin/Ops web) cùng mã màu nghiệp vụ dùng chung, pipeline sinh Theme Godot, `UiTokens`, resource 3D, theme Ant Design và CSS variables, kiểm tương phản WCAG và kiểm file sinh — NFR-MAINT-007, NFR-A11Y-001, NFR-A11Y-003 (ADR-0047).

## Clarifications

### Session 2026-11-16

- Q: Giá trị màu/font cụ thể (`#2F6BFF`, "Be Vietnam Pro", "Baloo 2")? → A: Dùng giá trị đề xuất trong [DESIGN_SYSTEM §4.1, §5.5, §6](../../docs/05-ux/DESIGN_SYSTEM.md) làm giá trị khởi tạo [Proposed]; đổi sau qua PR token, không chặn feature.
- Q: Godot Theme `.tres` sinh bằng gì? → A: Style Dictionary xuất JSON trung gian, script Godot headless `tools/godot/build_theme.gd` dựng `Theme`/`ShaderMaterial`/`Environment`/`Gradient` (DESIGN_SYSTEM §3) [Proposed].
- Q: Cặp chữ/nền nào phải kiểm tương phản? → A: Mọi cặp khai báo tường minh trong `contrast-pairs` của `admin-tokens/semantic.json` (sáng, tối, compact) và `game-tokens/semantic.json` (mọi density, chế độ tương phản cao); thiếu cặp cho token chữ mới → build fail.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Một nguồn token sinh theme cho game và web (Priority: P1)

UX/Game designer sửa token trong `design/`, chạy một lệnh, Theme Godot và theme Ant Design cập nhật; không ai sửa tay file sinh.

**Why this priority**: R32, NFR-MAINT-007; F050 và F060 cần đầu ra ngay.

**Independent Test**: Đổi `brand.primary` trong shared-tokens, chạy `make design-tokens`, so giá trị trong `antd-theme.ts`, `tokens.css`, `eduworld_theme.tres`, `ui_tokens.gd`; sửa tay một file sinh rồi chạy `make design-tokens-check`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F005-AC-01 | `design/` PHẢI có cấu trúc `shared-tokens/`, `game-tokens/` (gồm `density/{g1_3,g4_5,g6_7,g8_9}.json`, `art3d.json`), `admin-tokens/` (gồm `modes/{light,dark,compact}.json`), `icons/`, `fonts/`, `style-dictionary.config.mjs`, `README.md` theo [DESIGN_SYSTEM §2](../../docs/05-ux/DESIGN_SYSTEM.md); token PHẢI theo định dạng W3C Design Tokens (`$value`, `$type`, `$description`) | NFR-MAINT-007 · ADR-0047 | — |
| F005-AC-02 | KHI chạy `make design-tokens`, pipeline PHẢI sinh `godot/resources/ui/theme/eduworld_theme.tres` + biến thể density, `godot/scripts/core/ui_tokens.gd` (`class_name UiTokens`), `godot/resources/art/generated/*`, `web/packages/ops-ui/src/theme/antd-theme.ts` (light, dark, compact) và `web/packages/ops-ui/src/theme/tokens.css` (`--ew-*`), mỗi file có dòng đầu đánh dấu file sinh | NFR-MAINT-007 · DESIGN_SYSTEM §3 | TC-UI-001 |
| F005-AC-03 | NẾU file sinh khác kết quả chạy lại pipeline hoặc bị sửa tay, THÌ `make design-tokens-check` PHẢI fail và liệt kê file lệch | NFR-MAINT-007 · AGENTS R32 | TC-UI-001 |
| F005-AC-04 | NẾU alias trỏ tới token không tồn tại, tạo vòng lặp, hoặc token semantic/component dùng giá trị literal thay vì alias tới primitive, THÌ pipeline PHẢI fail kèm đường dẫn token | NFR-MAINT-007 · DESIGN_SYSTEM §2 | — |
| F005-AC-05 | Mã màu nghiệp vụ (môn học/khu vực, độ hiếm, trạng thái nội dung, brand) PHẢI chỉ được khai báo trong `design/shared-tokens/`; theme Godot và theme web sinh ra PHẢI có cùng tên token và cùng giá trị cho biến thể tương ứng | AGENTS R32 · DESIGN_SYSTEM §6 | TC-UI-008 |
| F005-AC-06 | Mỗi màu nghiệp vụ PHẢI có biến thể nền sáng, nền tối (admin) và biến thể game (3D/UI) cùng tên; NẾU thiếu biến thể, THÌ pipeline PHẢI fail | DESIGN_SYSTEM §6 · ADR-0047 | TC-UI-008 |
| F005-AC-07 | Đầu ra Godot PHẢI không chứa thành phần của Admin UI và đầu ra web PHẢI không chứa token `game-tokens` | AGENTS R32 · ADR-0047 | TC-UI-010 |

---

### User Story 2 — Token bảo đảm trợ năng (Priority: P1)

Học sinh lớp 1–3 đọc được chữ đủ lớn; nhân sự vận hành dùng portal đạt WCAG 2.1 AA; vi phạm bị chặn ngay khi build token.

**Why this priority**: NFR-A11Y-001, NFR-A11Y-003; sửa sau tốn kém trên mọi màn.

**Independent Test**: Hạ cỡ chữ `g1_3` dưới ngưỡng hoặc đổi màu chữ admin thành tương phản thấp, chạy pipeline.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F005-AC-08 | KHI build token, script tương phản PHẢI kiểm mọi cặp chữ/nền khai báo của admin-tokens (light, dark, compact) và game-tokens UI (mọi density, tương phản cao); NẾU cặp nào dưới WCAG 2.1 AA (4.5:1 chữ thường, 3:1 chữ lớn và thành phần đồ họa), THÌ build PHẢI fail và xuất báo cáo | NFR-A11Y-003 · NFR-A11Y-001 · [DESIGN_SYSTEM §4.5](../../docs/05-ux/DESIGN_SYSTEM.md) | TC-UI-002 |
| F005-AC-09 | Density `g1_3` PHẢI có cỡ chữ cơ sở và chiều cao nút tối thiểu theo [DESIGN_SYSTEM §5.2](../../docs/05-ux/DESIGN_SYSTEM.md); NẾU cỡ chữ cơ sở `g1_3` nhỏ hơn mức NFR-A11Y-001 hoặc bất kỳ density nào nhỏ hơn bảng §5.2, THÌ build PHẢI fail | NFR-A11Y-001 | — |
| F005-AC-10 | Mode compact của admin PHẢI không có cỡ chữ dưới mức tối thiểu của DESIGN_SYSTEM §4.5; NẾU vi phạm, THÌ build PHẢI fail | NFR-A11Y-003 | TC-UI-002 |
| F005-AC-11 | Pipeline PHẢI xuất token `text.expansion_allowance` theo tham số `text_expansion_allowance` và token `math.font-scale` vào `UiTokens` và CSS variables | NFR-A11Y-001 · DESIGN_SYSTEM §4.6, §5.5 | — |
| F005-AC-12 | Font trong `design/fonts/` PHẢI có file LICENSE (OFL) và phủ đủ ký tự tiếng Việt có dấu + Latin mở rộng; NẾU thiếu license hoặc thiếu glyph trong bộ ký tự kiểm, THÌ pipeline PHẢI fail | NFR-A11Y-001 · DESIGN_SYSTEM §5.5 | — |

---

### User Story 3 — Thay đổi token có phiên bản và kích hoạt client (Priority: P2)

Owner token duyệt thay đổi có semver, token bị bỏ được đánh dấu trước khi xóa, CI kích hoạt kiểm tra game và web khi token đổi.

**Why this priority**: Governance DESIGN_SYSTEM §8; tránh vỡ client âm thầm.

**Independent Test**: PR đổi tên token không tăng MAJOR; PR xóa token chưa `deprecated`; PR chạm `design/**`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F005-AC-13 | `design/CHANGELOG.md` và `design/package.json` PHẢI ghi phiên bản token semver; NẾU PR đổi tên hoặc xóa token mà không tăng MAJOR, THÌ CI PHẢI fail | DESIGN_SYSTEM §3, §8 | — |
| F005-AC-14 | NẾU token bị xóa chưa từng mang `$deprecated` ở phiên bản trước, THÌ CI PHẢI fail | DESIGN_SYSTEM §8 | — |
| F005-AC-15 | KHI PR thay đổi `design/**`, GitHub Actions PHẢI chạy workflow `design.yml` (`make design-tokens-check`, báo cáo tương phản) theo `paths` filter rồi gọi `godot.yml` và `web.yml` (`workflow_call`) | DEPLOYMENT §10.2 · ADR-0047 · ADR-0088 | TC-UI-001 |

### Edge Cases

- NẾU Godot headless không có trong môi trường dev, THÌ `make design-tokens` vẫn sinh đầu ra web + JSON trung gian và báo rõ bước Godot bị bỏ; `design-tokens-check` trong CI luôn chạy đủ (F005-AC-02, F005-AC-03).
- NẾU token chữ mới không khai báo cặp tương phản, THÌ build fail (F005-AC-08).
- NẾU màu nghiệp vụ được khai báo lại trong `game-tokens` hoặc `admin-tokens`, THÌ build fail (F005-AC-05).
- Authorization, tenant isolation, idempotency, Problem Details, giới hạn độ dài: không áp dụng — F005 là pipeline build, không có endpoint hay dữ liệu người dùng.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| NFR-MAINT-007 | Nguồn token, pipeline sinh, `design-tokens-check`, chặn khai báo màu nghiệp vụ ngoài shared | DESIGN_SYSTEM §2, §3, §6 |
| NFR-A11Y-001 | Density theo grade band, font tiếng Việt, tương phản UI game | DESIGN_SYSTEM §5.2, §5.5 |
| NFR-A11Y-003 | Báo cáo tương phản WCAG AA admin khi build token | DESIGN_SYSTEM §4.5, §9 |

### Key Entities

- **Token**: giá trị thiết kế có tên, kiểu, mô tả, alias; thuộc tầng primitive/semantic/component.
- **Bộ token**: shared (brand, mã màu nghiệp vụ), game (UI, density, 3D), admin (seed, semantic, mode).
- **Đầu ra sinh**: Theme Godot, `UiTokens`, resource 3D, theme Ant Design, CSS variables, báo cáo tương phản.
- **Cặp tương phản**: cặp token chữ/nền phải đạt ngưỡng.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `text_expansion_allowance` | Token `text.expansion_allowance` |
| `question_font_px` | Tham chiếu khi kiểm token cỡ chữ đề bài (không sinh lại giá trị — LEARNING_EXPERIENCE sở hữu) |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Hai design system tách biệt; không mang Theme game vào web và ngược lại | AGENTS R32 · ADR-0047 |
| File sinh không sửa tay | AGENTS R32 |
| Font license OFL, nhúng hợp lệ | DESIGN_SYSTEM §5.5 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Đổi một token shared → cả game và web nhận giá trị mới sau một lệnh, 0 file sửa tay.
- **SC-002**: 100% cặp chữ/nền khai báo đạt WCAG 2.1 AA trong báo cáo build.
- **SC-003**: 100% lần sửa tay file sinh bị CI phát hiện.
- **SC-004**: Mã màu nghiệp vụ trùng tên và giá trị giữa game và web ở 100% token.

## Ngoài phạm vi

- Component `ops-ui`, Storybook, lint hardcode trong `web/` → F050; áp Theme trong Godot, `lint_theme_overrides.gd`, UI Gallery → F060.
- Thumbnail/turntable render tài sản 3D → F078; nội dung art 3D theo zone → F071.

## Assumptions

- Style Dictionary v4 (ESM) chạy bằng Node.js LTS; Godot headless đúng phiên bản ghim (`godot/.godot-version`, F060 tạo — F005 tạo file nếu chưa có).
- Giá trị màu/font khởi tạo [Proposed] theo DESIGN_SYSTEM; Game Director/UX Lead duyệt qua PR token.
