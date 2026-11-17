# ADR-0047 — Hai hệ thống giao diện: Game (Godot 3D) và Admin/Ops (Admin Portal)

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [DESIGN_SYSTEM](../05-ux/DESIGN_SYSTEM.md), [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md), [GODOT_ARCHITECTURE](../08-game-client/GODOT_ARCHITECTURE.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Stakeholder làm rõ: CMS/LMS **không** theo giao diện game mà dùng UI/CSS phù hợp **vận hành game**; game dùng Godot + 3D.

## Quyết định

1. **Hai design system tách biệt:**

   | | Game design system | Admin/Ops design system |
   |---|---|---|
   | Người dùng | Học sinh 6–15 tuổi | Vận hành tenant (nhà trường), System Admin, game designer/live-ops |
   | Nền tảng | Godot 4, thế giới 3D + UI Control | Admin Portal React (`web/apps/portal`, ADR-0027) |
   | Phong cách | Stylized/cartoon, chữ to, nút to, nhiều màu, chuyển động | Gọn, trung tính, **dày dữ liệu**, thao tác nhanh |
   | Nguồn token | `design/game-tokens/` | `design/admin-tokens/` |

2. **Admin/Ops UI:** **Ant Design 5** + Ant Design Pro (ProLayout, ProTable, ProForm) [Proposed]; theme EduWorld qua `ConfigProvider` từ `design/admin-tokens/` (màu thương hiệu, màu trạng thái, font hỗ trợ tiếng Việt, mật độ compact, sáng/tối); mẫu màn hình chuẩn (sidebar theo permission, bộ chọn tenant cho System Admin, danh sách = lọc + bảng + thao tác hàng loạt, chi tiết = tab + timeline audit/version, dashboard live-ops bằng ECharts/Ant Design Charts [Proposed], banner cảnh báo hệ thống); **WCAG 2.1 AA**; i18n `vi-VN`; component đặc thù trong `web/packages/ops-ui`. Phương án thay thế MUI/Mantine cần ADR mới.
3. **Game design system:** token `design/game-tokens/` sinh bằng Style Dictionary [Proposed] ra `Theme` Godot + `UiTokens.gd` và resource 3D (preset shader stylized/toon, outline, `Environment`/ánh sáng, gradient VFX, màu độ hiếm, palette theo khu vực) trong `godot/resources/art/generated/`. UI trong game: Control 2D phủ trên thế giới 3D và UI trong không gian 3D qua `SubViewport`, cùng một Theme. Font hỗ trợ đủ tiếng Việt, chọn bởi Game Director [Proposed].
4. **Dùng chung (không phải phong cách UI):** logo và màu thương hiệu; **mã màu nghiệp vụ** (môn học/khu vực, độ hiếm, trạng thái nội dung) trong `design/shared-tokens/`; **tài sản 3D hiển thị trong portal như dữ liệu** (thumbnail/turntable render headless bằng Godot, lưu object storage); khung preview Godot Web export hiển thị đúng giao diện game, phần còn lại theo Admin/Ops UI.
5. **Kiểm soát chất lượng:** Storybook cho `ops-ui` + theme; visual regression (Playwright); lint chặn màu/font hardcode trong CSS/TSX; kiểm tra scene Godot không override màu ngoài Theme; so sánh ảnh render giữa renderer Mobile/Forward+/Compatibility trong CI [Proposed].

## Hệ quả

(+) Portal tối ưu cho vận hành; game giữ phong cách riêng cho trẻ em; màu nghiệp vụ nhất quán. (−) Hai design system quản lý riêng; theme Ant Design cần tùy chỉnh; phụ thuộc vòng đời Ant Design.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Accepted: design system chung cho game và CMS | Stakeholder |
| 2026-11-15 | Bổ sung token 3D | Stakeholder (game Godot 3D) |
| 2026-11-15 | Đổi thành hai design system; CMS theo UI vận hành (Ant Design 5) | Stakeholder |
| 2026-11-15 | Nền tảng web là một Admin Portal (thay 2 bundle) | ADR-0027 |
| 2026-11-15 | Cập nhật tham chiếu theo ADR-0072 (bỏ vai trò giáo viên), không đổi ý nghĩa quyết định | Technical Architect |
