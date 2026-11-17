# src/cms/frontend — Admin Portal

Một React SPA (TypeScript + Vite) với 5 workspace (ADR-0027, ADR-0047): `apps/portal`, `packages/ops-ui`. Gọi API public của `core-api` qua APISIX; không chứa logic nghiệp vụ (AGENTS R1). Khung pnpm workspace, theme Ant Design từ `design/admin-tokens/` và lane CI `web.yml` do **F050** tạo; shell portal do **F051**.
