# ADR-0098 — Tailwind CSS làm lớp tiện ích bố cục cho Admin Portal (bổ sung ADR-0047)

> **Trạng thái:** Accepted (owner yêu cầu 2026-11-22) · bổ sung [ADR-0047](ADR-0047-hai-he-thong-giao-dien-game-godot-3d-va-admin-ops-cms-lms.md) (không thay thế) · **Ngày:** 2026-11-22 · **OQ:** — · **Tài liệu:** [DESIGN_SYSTEM](../05-ux/DESIGN_SYSTEM.md) · [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

ADR-0047 chốt Admin/Ops UI là Ant Design 5 + Pro với theme sinh từ `design/admin-tokens/`, cấm hardcode màu/font/cỡ chữ/khoảng cách (AGENTS R32). Bố cục shell (topbar, sidebar, vùng nội dung) đang viết bằng `style={{…}}` đọc `theme.useToken()` nên khó đọc và khó giữ nhất quán khi thêm hành vi bố cục (cố định header/sidebar, chỉ vùng nội dung cuộn, responsive).

Owner yêu cầu (2026-11-22): sidebar tông màu riêng, header và sidebar cố định khi cuộn, nút nhanh ngôn ngữ và sáng/tối trên header, và **dùng thêm Tailwind CSS** cho bố cục/tiện ích của shell — với điều kiện không vi phạm R32/ADR-0047.

## Quyết định

1. **Tailwind CSS v3 (ghim phiên bản chính xác, hiện `3.4.19`) + PostCSS (`8.5.33`)** chỉ trong `src/cms/frontend/apps/portal` (devDependency). `packages/ops-ui` không dùng Tailwind. Plugin PostCSS cấu hình inline trong `apps/portal/vite.config.ts`; cấu hình theme ở `apps/portal/tailwind.config.ts`; điểm vào `src/app/tailwind.css` (chỉ `@tailwind base; @tailwind components; @tailwind utilities;`) được import trong `main.tsx`.
2. **Ant Design vẫn là thư viện component** và nguồn style của mọi component. Tailwind chỉ dùng cho bố cục/tiện ích (flex, overflow, chiều cao, z-index, khoảng cách) trên phần tử bao hoặc trên thuộc tính Ant Design không tự đặt; không viết lại style component Ant Design bằng class Tailwind.
3. **Mọi class mang tiền tố `tw-`** (`prefix: 'tw-'`, ví dụ `tw-h-dvh tw-overflow-hidden`): không có tiền tố, các từ trong mã nguồn (`hidden`, `table`, `fixed`, `transition`) bị quét thành tiện ích toàn cục và đè lên DOM của thư viện khác (MathLive, MathJax) dùng cùng tên class. **Tắt Preflight** (`corePlugins.preflight: false`) vì xung đột với reset của Ant Design; `@tailwind base` khi đó chỉ sinh biến `--tw-*` của tiện ích đang dùng. Tắt các plugin có giá trị mặc định là màu/độ dài thô (`boxShadow`, `ring*`, `dropShadow`, `blur`, `filter`, `container`…).
4. **Theme thay hẳn mặc định của Tailwind** và chỉ trỏ tới biến CSS sinh từ admin token (`var(--ew-*)` trong `packages/ops-ui/src/theme/tokens.css`): `colors` (`primary`, `layout`, `container`, `fg`, `line`, `sider`…), `spacing` (thang `ops.size.space.{4…48}` mới thêm vào `design/admin-tokens/seed.json`), `fontSize`, `fontFamily`, `borderRadius`, `borderWidth`. Không có palette hex/px nào của Tailwind dùng được qua class. Ngoại lệ duy nhất: `screens` (`md` 768 px = Ant Design `screenMD`, `xl` 1280 px = desktop ADMIN_PORTAL §1.5) vì media query không đọc được biến CSS.
5. **Chặn giá trị tùy ý:** ESLint `ew/no-hardcoded-style` báo mọi arbitrary value trong chuỗi class (`bg-[#…]`, `p-[13px]`, `[color:red]`, kể cả sau variant) trừ khi giá trị là đúng một biến token `var(--ew-*)`; stylelint chỉ cho `@tailwind`, cấm `@apply`, `@layer`, `@screen`… và hàm `theme()`/`screen()` trong CSS. Test `apps/portal/src/app/tailwind.test.ts` kiểm cấu hình (Preflight tắt, tiền tố `tw-`, mọi giá trị theme là token, biến tồn tại trong `tokens.css`) và CSS Tailwind sinh cho mã nguồn portal (không có màu thô, không có `px` ngoài media query, không có tiện ích thiếu tiền tố).
6. Token vẫn chỉ sửa ở `design/` rồi `make design-tokens` (R32, R33); Tailwind không phải nguồn giá trị thiết kế.

## Hệ quả

(+) Bố cục shell (header/sidebar cố định, chỉ vùng nội dung cuộn, responsive) đọc được và kiểm được bằng class; token vẫn là nguồn duy nhất, có kiểm tự động.
(+) Không đụng Ant Design reset; Storybook `ops-ui` không đổi.
(−) Thêm một phụ thuộc build (Tailwind, PostCSS) cho portal; sau khi pull cần `pnpm install` và khởi động lại Vite dev server.
(−) Thứ tự cascade giữa CSS-in-JS của Ant Design và tiện ích Tailwind không bảo đảm khi cùng thuộc tính: quy tắc mục 2 (không dùng Tailwind cho thuộc tính Ant Design đã đặt; khi cần thì dùng prop/`style` với token) phải giữ trong review.

## Phương án đã loại

- **Tailwind v4:** cấu hình CSS-first, Preflight gắn với `@import "tailwindcss"` và chưa có cách giới hạn theme dễ kiểm như v3 trong pipeline hiện tại; cân nhắc lại bằng ADR mới khi cần.
- **`StyleProvider layer` của Ant Design + `@layer`:** đổi thứ tự cascade của toàn bộ style Ant Design trong portal, rủi ro hồi quy ở mọi màn — không cần cho phạm vi bố cục shell.
- **Chỉ dùng `style={{…}}` với token (giữ nguyên):** owner yêu cầu Tailwind.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-22 | Tạo ADR (Accepted theo yêu cầu owner) | Owner |
