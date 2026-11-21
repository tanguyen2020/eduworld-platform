# Data Model: F050 — Khung web: pnpm workspace, `ops-ui`, theme từ token, `apiClient`, Storybook

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — F050 không có dữ liệu bền vững phía server; portal chỉ giữ preference per-viewer (chế độ sáng/tối, mật độ, ngôn ngữ) trong `localStorage` bọc try/catch, không token, không dữ liệu nghiệp vụ.

Cấu trúc client để tham chiếu khi code:

| Cấu trúc | Trường | Nguồn |
|---|---|---|
| `OperationMeta` (sinh) | `operationId`, `method`, `path`, `service` (`x-service`), `gateway: 'public' \| 'internal'` (theo prefix API_DESIGN §10), `tenantHeader: 'required' \| 'optional' \| 'none'` (từ tham số `TenantIdHeader`/`TenantIdHeaderOptional`), `idempotency: boolean` (tham số `IdempotencyKey`), `ifMatch: boolean` (tham số `IfMatch`) | `specs/openapi/eduworld-api.v1.yaml` |
| `RuntimeConfig` | `apiBaseUrl`, `apiInternalBaseUrl`, `oidcIssuer`, `oidcClientId`, `env` | `/config.json` |
| `ApiError` | `status`, `code` (ProblemCode), `title`, `detail`, `fieldErrors: Record<string, {code, message}>`, `traceId` | schema `Problem` |
| `UserPreference` (localStorage `ew.portal.prefs`) | `theme: light \| dark`, `density: default \| compact` (mặc định `default`; `compact` chỉ khi người dùng tự bật trong menu tài khoản — `densityChosen: true`, DESIGN_SYSTEM §4.1, owner 2026-11-21), `locale: vi-VN \| en-US`, `lastWorkspace` | per-viewer |
