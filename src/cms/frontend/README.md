# src/cms/frontend — Admin Portal

Một React SPA (TypeScript `strict` + Vite) với 5 workspace (ADR-0027, ADR-0047): `apps/portal`, `packages/ops-ui`. Gọi API public của `core-api` qua APISIX; không chứa logic nghiệp vụ (AGENTS R1). Khung pnpm workspace, theme Ant Design từ `design/admin-tokens/` và lane CI `web.yml` do **F050** tạo; shell portal (đăng nhập, phiên, chọn tenant, menu theo quyền, bộ lọc tenant) do **F051**.

| Đường dẫn | Nội dung |
|---|---|
| `apps/portal/src/workspaces/{school-lms,content-studio,game-ops,live-ops,platform-admin}` | Mỗi workspace một chunk lazy, route theo `path` của danh mục (`/school-lms`, `/content-studio`, `/game-ops`, `/live-ops`, `/platform-admin`); chỉ gắn component màn hình vào khóa menu (`screens`) |
| `apps/portal/src/generated/menu-catalog.ts` | Workspace → menu → action sinh từ `specs/portal/menu-catalog.yaml` (`pnpm gen:catalog` = `make portal-catalog`, ADR-0090; không sửa tay) |
| `apps/portal/src/auth/` | Trang `/login`, thiết lập tài khoản, quên/đặt lại mật khẩu, `authApi` (`/auth/portal/*`), token trong bộ nhớ, làm mới single-flight + CSRF, `IdleGuard`, hộp đăng nhập lại khi `MFA_REQUIRED` (ADR-0091) |
| `apps/portal/src/session/` | Máy trạng thái phiên, định tuyến sau đăng nhập (`listMyTenants`), chọn/đổi đơn vị làm việc (`X-Tenant-Id`) |
| `apps/portal/src/permissions/` | `menuRegistry` (lọc theo `getMyPermissions`, `disabled_menus`/`disabled_actions`), cổng route workspace/menu, `ActionGate` |
| `apps/portal/src/shell/` | `PortalLayout` (topbar, sidebar, drawer responsive, banner slot), bộ chọn/bộ lọc tenant, trang 403/tắt tính năng/VPN, `ReasonDialog` |
| `apps/portal/src/shared/api/` | `apiClient` (`openapi-fetch` + middleware header/gateway/retry/Problem), hook `useMyPermissions`/`useMe`/`useTenantOptions`, `withTenantScope`, `useIdempotentMutation`; `schema.gen.d.ts` + `operation-meta.gen.ts` sinh bằng `pnpm gen:api` (không sửa tay) |
| `apps/portal/src/i18n/{vi,en}.json`, `catalog.{vi,en}.json` | Chuỗi UI (`vi-VN` mặc định) và nhãn workspace/menu của danh mục |
| `apps/portal/src/test/` | MOCK backend hợp đồng phiên portal cho test (không phải backend thật) |
| `packages/ops-ui/src/theme/` | `OpsThemeProvider`; `antd-theme.ts`, `tokens.css` sinh từ `design/` (`make design-tokens`, không sửa tay) |
| `packages/ops-ui/src/components`, `src/forms` | `DomainTag`, `openapiRules` + `CharCounter` |
| `eslint-rules/`, `tooling/` | Rule `ew/no-hardcoded-style`, `ew/no-game-imports`, `ew/no-env-url-literal`; plugin Vite chặn import game |
| `tests/visual/` | Playwright visual regression trên Storybook tĩnh (ops-ui + component khung portal) |

```bash
pnpm install && pnpm dev        # http://localhost:5173
pnpm gen:api | pnpm gen:check   # type + metadata từ specs/openapi/eduworld-api.v1.yaml, danh mục menu từ specs/portal/menu-catalog.yaml
pnpm lint && pnpm typecheck && pnpm test
pnpm build                      # + kiểm 5 chunk workspace
pnpm storybook                  # http://localhost:6006
pnpm storybook:build && node tooling/storybook-a11y.mjs && pnpm test:visual
```

Tương đương `make web-lint`, `make web-test`, `make web-build`, `make storybook`, `make web-storybook-check`, `make web-visual`, `make portal-catalog` (LOCAL_DEVELOPMENT §7).
