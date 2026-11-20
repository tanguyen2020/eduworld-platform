# Implementation Plan: F050 — Khung web: pnpm workspace, `ops-ui`, theme từ token, `apiClient`, Storybook

**Branch**: `050-web-foundation-ops-ui` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Tạo `src/cms/frontend/` pnpm workspace với `apps/portal` (Vite + React + TS strict, React Router, TanStack Query, react-i18next) có 5 route workspace lazy-load rỗng, và `packages/ops-ui` (Ant Design 5 + Pro, `OpsThemeProvider` dùng theme sinh bởi F005, `DomainTag`, helper rule form từ OpenAPI). `apiClient` dựa trên `openapi-fetch` với type từ `openapi-typescript` và bảng metadata sinh từ spec gộp; middleware gắn header, ánh xạ Problem, refresh, retry. Lint ESLint + stylelint chặn hardcode và import game; Storybook 8 + test-runner axe + Playwright visual.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | TypeScript 5 (`strict`), React 18, Node.js LTS, pnpm |
| Primary Dependencies | Vite, React Router, TanStack Query, `antd@5`, `@ant-design/pro-components`, `dayjs`, `openapi-typescript`, `openapi-fetch`, `react-i18next`, ESLint (`eslint-plugin-i18next`, rule tùy biến), stylelint, Storybook 8, `@storybook/test-runner` + `axe-playwright`, Playwright, Vitest, Testing Library, MSW (chỉ test) |
| Storage | Không (preference sáng/tối/ngôn ngữ trong `localStorage` per-viewer, không dữ liệu nghiệp vụ, không token) |
| Testing | Vitest + Testing Library, MSW, Storybook test-runner (axe), Playwright |
| Target Platform | Trình duyệt desktop/tablet/mobile (Chrome, Edge, Firefox, Safari bản hiện hành) |
| Project Type | Web app + thư viện component |
| Performance Goals | Chunk khởi đầu chỉ shell; workspace tải lười |
| Constraints | AGENTS R1, R3, R14, R32 · ADR-0022, ADR-0027, ADR-0047, ADR-0065 |
| Scale/Scope | 432 operation trong spec gộp; 5 workspace |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Client sinh từ spec gộp; không contract mới |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Portal gọi API qua gateway theo prefix; không truy cập service trực tiếp |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `X-Tenant-Id` theo metadata; backend quyết định (F002) |
| IV Server-authoritative | ✅ | Không tính điểm/kết quả ở client |
| V Xác thực, phân quyền | ✅ | Token chỉ trong bộ nhớ; menu ẩn chỉ là tiện ích; API platform qua gateway nội bộ |
| VI An toàn trẻ em | ✅ | Không log token/PII trong console |
| VII Một nguồn sự thật, cấu hình | ✅ | URL qua runtime config; rule form từ `maxLength` spec |
| VIII UI tách logic, token | ✅ | Theme từ token F005; lint chặn hardcode; component không chứa logic nghiệp vụ |
| IX Test có bằng chứng | ✅ | Vitest, axe, Playwright; MSW gắn nhãn chỉ trong test |

## Project Structure

### Documentation (feature này)

```text
specs/050-web-foundation-ops-ui/
├── spec.md
├── plan.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
src/cms/frontend/
├── package.json  pnpm-workspace.yaml  tsconfig.base.json  .eslintrc.cjs  .stylelintrc.cjs
├── eslint-rules/{no-hardcoded-style.js,no-game-imports.js}
├── apps/portal/
│   ├── index.html  vite.config.ts  public/config.example.json
│   ├── scripts/gen-op-meta.mjs                 # spec gộp → operation-meta.gen.ts
│   └── src/
│       ├── main.tsx  app/{App.tsx,router.tsx,runtime-config.ts}
│       ├── workspaces/{game-ops,content-studio,school-lms,platform-admin,live-ops}/index.tsx   # route lazy, trang trống
│       ├── shared/api/{schema.gen.d.ts,operation-meta.gen.ts,client.ts,middleware/{auth,tenant,idempotency,ifmatch,language,retry,problem}.ts,events.ts,types.ts}
│       ├── shared/auth/{TokenProvider.ts,TenantContext.tsx}                    # interface + context (hiện thực F051)
│       └── i18n/{index.ts,vi.json,en.json}
└── packages/ops-ui/
    ├── package.json  .storybook/{main.ts,preview.tsx,test-runner.ts}
    └── src/{index.ts,theme/{OpsThemeProvider.tsx,antd-theme.ts(sinh F005),tokens.css(sinh F005)},
             components/DomainTag/{DomainTag.tsx,DomainTag.stories.tsx},
             forms/{openapiRules.ts,CharCounter.tsx}}
src/cms/frontend/tests/visual/storybook.spec.ts  playwright.config.ts
.github/workflows/web.yml
```

**Structure Decision**: Theo SYSTEM_ARCHITECTURE §8 và ADMIN_PORTAL §1.2 (`src/cms/frontend/apps/portal/src/workspaces/<workspace>`); `apiClient` trong portal vì chỉ portal gọi API.

## Phase 0: Research

Không có. Stack theo ADR-0022/ADR-0047 [Confirmed] và DESIGN_SYSTEM §4; lựa chọn chi tiết ghi ở Clarifications.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| UI | `workspaces/*` (trống), `ops-ui` component | Không logic nghiệp vụ (R1) |
| State/server-state | TanStack Query (`queryClient` mặc định retry tắt — retry ở apiClient) | |
| API | `createApiClient({config, tokenProvider, tenantContext, i18n})` → `openapi-fetch` + middleware | Metadata tra theo `method + path template` |
| Config | `runtime-config.ts` đọc `/config.json` (`apiBaseUrl`, `apiInternalBaseUrl`, `oidcIssuer`, `oidcClientId`) | Fallback `import.meta.env.VITE_*` khi dev |

**Thứ tự middleware apiClient**: `language` → `auth` (Bearer từ `TokenProvider.getAccessToken()`) → `tenant` (theo `tenantHeader`) → `idempotency` (UUID v7 theo `crypto.randomUUID` fallback; key gắn với "intent id" để retry giữ key) → `ifMatch` (cache ETag theo URL tài nguyên trong bộ nhớ) → `gateway` (chọn base URL theo prefix) → `retry` (GET/có key; 502/503/504/lỗi mạng; tối đa 2 lần, backoff 100 ms × 2ⁿ + jitter — đối chiếu `internal_api_resilience`) → `problem` (parse `application/problem+json`, `ApiError{status, code, fieldErrors, traceId}`, phát sự kiện).

**Sự kiện** (`shared/api/events.ts`, EventTarget): `session_expired`, `tenant_mismatch`, `mfa_required`, `vpn_required`.

### Tích hợp và phụ thuộc

- Feature phụ thuộc: F005 (theme) · F004 (gateway nội bộ local) cho thử thật · Tiêu thụ: F051 (hiện thực `TokenProvider`, `TenantContext`, shell), F052–F057, F079, F080, F122.

### Bảo mật, tenant, idempotency, audit

- Token chỉ trong bộ nhớ (interface; F051 hiện thực) · `X-Tenant-Id` theo metadata · `Idempotency-Key` cho operation bắt buộc · không log header/token · CSP do hosting (F130).

### Quan sát

- Gửi `traceparent` (W3C, sinh ở client) để nối trace với APISIX; hiển thị `trace_id` trong lỗi; không gửi telemetry người dùng ở F050.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
