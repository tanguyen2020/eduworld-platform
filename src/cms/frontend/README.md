# src/cms/frontend — Admin Portal

Một React SPA (TypeScript `strict` + Vite) với 5 workspace (ADR-0027, ADR-0047): `apps/portal`, `packages/ops-ui`. Gọi API public của `core-api` qua APISIX; không chứa logic nghiệp vụ (AGENTS R1). Khung pnpm workspace, theme Ant Design từ `design/admin-tokens/` và lane CI `web.yml` do **F050** tạo; shell portal do **F051**.

| Đường dẫn | Nội dung |
|---|---|
| `apps/portal/src/workspaces/{school-lms,content-studio,game-ops,live-ops,platform-admin}` | Mỗi workspace một chunk lazy (route `/school`, `/content`, `/game-ops`, `/live-ops`, `/platform`) |
| `apps/portal/src/shared/api/` | `apiClient` (`openapi-fetch` + middleware header/gateway/retry/Problem), `schema.gen.d.ts` + `operation-meta.gen.ts` sinh bằng `pnpm gen:api` (không sửa tay) |
| `apps/portal/src/shared/auth/` | `TokenProvider`, `TenantContext` (F051 hiện thực) |
| `apps/portal/src/i18n/{vi,en}.json` | Chuỗi UI (`vi-VN` mặc định) |
| `packages/ops-ui/src/theme/` | `OpsThemeProvider`; `antd-theme.ts`, `tokens.css` sinh từ `design/` (`make design-tokens`, không sửa tay) |
| `packages/ops-ui/src/components`, `src/forms` | `DomainTag`, `openapiRules` + `CharCounter` |
| `eslint-rules/`, `tooling/` | Rule `ew/no-hardcoded-style`, `ew/no-game-imports`, `ew/no-env-url-literal`; plugin Vite chặn import game |
| `tests/visual/` | Playwright visual regression trên Storybook tĩnh |

```bash
pnpm install && pnpm dev        # http://localhost:5173
pnpm gen:api | pnpm gen:check   # type + metadata từ specs/openapi/eduworld-api.v1.yaml
pnpm lint && pnpm typecheck && pnpm test
pnpm build                      # + kiểm 5 chunk workspace
pnpm storybook                  # http://localhost:6006
pnpm storybook:build && node tooling/storybook-a11y.mjs && pnpm test:visual
```

Tương đương `make web-lint`, `make web-test`, `make web-build`, `make storybook`, `make web-storybook-check`, `make web-visual` (LOCAL_DEVELOPMENT §7).
