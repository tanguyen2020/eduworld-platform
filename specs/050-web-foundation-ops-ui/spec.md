# Feature Specification: F050 — Khung web: pnpm workspace, `ops-ui`, theme từ token, `apiClient`, Storybook

**Feature Branch**: `050-web-foundation-ops-ui` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `src/cms/frontend/` (`apps/portal`, `packages/ops-ui`) · **Mốc**: M-C0 · **Phụ thuộc**: F005
**Input**: Khung một Admin Portal React + TypeScript với 5 workspace lazy-load (chưa có nội dung nghiệp vụ), thư viện `ops-ui` theo Admin/Ops design system (Ant Design 5 + Pro, theme sinh từ `design/admin-tokens/`), lớp gọi API sinh từ OpenAPI, i18n vi/en, lint chặn hardcode giao diện và Storybook có kiểm trợ năng — FR-CMS-009, NFR-MAINT-007, NFR-A11Y-003 (ADR-0022, ADR-0027, ADR-0047).

## Clarifications

### Session 2026-11-16

- Q: `apiClient` đặt ở package riêng hay trong portal? → A: Trong portal `src/cms/frontend/apps/portal/src/shared/api/` (chỉ portal gọi API; `ops-ui` không gọi API, ADMIN_PORTAL §1.1) [Proposed].
- Q: Gọi API platform qua gateway nào? → A: Theo prefix của API_DESIGN §10: `/admin/*`, `/sis/*`, `/game-admin/*`, `/game-media*` đi base URL gateway nội bộ; còn lại gateway public (không dựa vào cờ `x-internal` vì spec còn thiếu ở 20 operation — F004 sửa).
- Q: Đăng nhập OIDC và chọn tenant có trong F050? → A: Không; F050 định nghĩa interface `TokenProvider` và `TenantContext`, F051 hiện thực bằng form `/login` của portal gọi `/api/v1/auth/portal/*` (client confidential `eduworld-portal`, ADR-0091 — thay PKCE) và màn chọn tenant.
- Q: Stack phụ trợ? → A: Vite, React Router, TanStack Query, `openapi-typescript` + `openapi-fetch`, react-i18next, Vitest + Testing Library, Playwright, Storybook 8 + test-runner axe [Proposed — ADR-0022 chi tiết stack Proposed].

### Session 2026-11-17

- Q: Tên thư mục 5 workspace? → A: `game-ops` (A), `content-studio` (B), `school-lms` (C), `platform-admin` (D), `live-ops` (E) theo [ADMIN_PORTAL §1.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) (RB-603, RB-619).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Khung Admin Portal một ứng dụng, 5 workspace (Priority: P1)

Web dev chạy `pnpm install && pnpm dev` có portal rỗng với 5 workspace tách chunk, cấu hình môi trường qua biến, chuỗi UI qua file dịch.

**Why this priority**: FR-CMS-009, ADR-0027 mục 7; mọi feature web (F051–F057, F079) dựng trên khung này.

**Independent Test**: Build production, kiểm có 5 chunk workspace; mở `/` không tải chunk workspace nào; đổi ngôn ngữ vi/en.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F050-AC-01 | `src/cms/frontend/` PHẢI là pnpm workspace gồm `apps/portal` (React, TypeScript `strict`, Vite) và `packages/ops-ui`; KHI chạy `pnpm install && pnpm dev`, portal PHẢI chạy ở cổng của [LOCAL_DEVELOPMENT §3.2](../../docs/11-operations/LOCAL_DEVELOPMENT.md) | FR-CMS-009 · ADR-0022 | — |
| F050-AC-02 | Portal PHẢI có 5 workspace `src/workspaces/{game-ops,content-studio,school-lms,platform-admin,live-ops}` mỗi workspace một chunk lazy-load; KHI chưa điều hướng vào workspace, chunk của nó PHẢI KHÔNG được tải | FR-CMS-009 · ADR-0027 · [ADMIN_PORTAL §1.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F050-AC-03 | URL gateway public, gateway nội bộ, issuer OIDC, client id PHẢI đến từ cấu hình runtime (`/config.json` sinh lúc deploy, fallback biến `VITE_*` khi dev); NẾU mã nguồn chứa URL môi trường literal, THÌ lint PHẢI fail | FR-CMS-009 · AGENTS R3 | — |
| F050-AC-04 | Chuỗi UI PHẢI qua file dịch `vi` (mặc định) và `en`; NẾU JSX chứa chuỗi hiển thị literal ngoài allowlist hoặc khóa có ở `vi` mà thiếu ở `en`, THÌ lint/test PHẢI fail | FR-CMS-009 · AGENTS §7 | — |

---

### User Story 2 — `ops-ui` theo Admin/Ops design system, không hardcode giao diện (Priority: P1)

Web dev dùng `OpsThemeProvider` và component `ops-ui`; theme lấy từ token; lint chặn màu/font/kích thước literal và chặn mọi thứ thuộc game.

**Why this priority**: R32, NFR-MAINT-007, ADR-0047.

**Independent Test**: Render Storybook ở light/dark/compact; thêm `color: #fff` vào một file `.tsx` → lint đỏ; import file từ `src/game/` → build đỏ.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F050-AC-05 | `ops-ui` PHẢI cung cấp `OpsThemeProvider` bọc `ConfigProvider` của Ant Design với theme sinh từ token (light, dark, compact), locale Ant Design `vi_VN`/`en_US` và locale dayjs tương ứng, PHẢI KHÔNG override CSS nội bộ của component Ant Design | NFR-MAINT-007 · [DESIGN_SYSTEM §4.1](../../docs/05-ux/DESIGN_SYSTEM.md) | TC-UI-005 |
| F050-AC-06 | NẾU CSS/TSX trong `src/cms/frontend/` (trừ file sinh từ token) chứa màu, font, cỡ chữ hoặc khoảng cách literal ngoài allowlist, THÌ `make web-lint` PHẢI fail | NFR-MAINT-007 · AGENTS R32 | TC-UI-003 |
| F050-AC-07 | NẾU `src/cms/frontend/` import từ `src/game/`, `design/game-tokens/` hoặc thư viện/Theme game, THÌ build PHẢI fail | AGENTS R32 · ADR-0047 | TC-UI-010 |
| F050-AC-08 | `ops-ui` PHẢI có `DomainTag` hiển thị môn học, độ hiếm, trạng thái nội dung bằng màu nghiệp vụ từ shared-tokens kèm nhãn chữ (màu không là kênh duy nhất) | DESIGN_SYSTEM §4.4, §6 | TC-UI-008 |
| F050-AC-09 | `ops-ui` PHẢI có helper sinh rule form từ schema OpenAPI (`required`, `maxLength`, `pattern`) kèm bộ đếm ký tự; KHI người dùng nhập vượt `maxLength`, form PHẢI chặn và báo lỗi tại trường | ADR-0041 · DESIGN_SYSTEM §4.3 | TC-TEN-024 |

---

### User Story 3 — `apiClient` sinh từ OpenAPI, xử lý header và lỗi chuẩn (Priority: P1)

Mọi màn portal gọi API qua một client có type, tự gắn header bắt buộc, ánh xạ Problem Details theo `code`, xử lý hết hạn token, sai tenant, thiếu MFA.

**Why this priority**: R3, R5, R10 phía client; ADR-0065 (`X-Tenant-Id`), ADR-0010.

**Independent Test**: Unit test client với MSW (mock có nhãn trong test) cho từng loại operation và từng mã lỗi.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F050-AC-10 | `pnpm gen:api` PHẢI sinh type TypeScript từ `specs/openapi/eduworld-api.v1.yaml` và bảng metadata mỗi operation (`idempotency` bắt buộc, `ifMatch`, `tenantHeader` required/optional/none, gateway public/nội bộ); NẾU file sinh lệch spec, THÌ CI PHẢI fail | FR-CMS-009 · ADR-0010 · NFR-MAINT-002 | — |
| F050-AC-11 | apiClient PHẢI gắn `Authorization` từ `TokenProvider`, `X-Tenant-Id` từ `TenantContext` cho operation `required` (và `optional` khi có tenant làm việc), PHẢI KHÔNG gửi `X-Tenant-Id` cho operation `none`, PHẢI gắn `Accept-Language` theo ngôn ngữ giao diện | ADR-0065 · [API_DESIGN §2](../../docs/07-backend/API_DESIGN.md) | TC-TEN-011a |
| F050-AC-12 | KHI gọi operation có idempotency bắt buộc, apiClient PHẢI sinh `Idempotency-Key` dạng UUIDv7 (ADR-0089 §4) và giữ nguyên key khi retry cùng thao tác; KHI gọi operation cần `If-Match`, apiClient PHẢI gửi ETag đã nhận từ lần `GET` | AGENTS R10 · API_DESIGN §6, §7 | — |
| F050-AC-13 | Operation có prefix `/admin/`, `/sis/`, `/game-admin/`, `/game-media` PHẢI gọi base URL gateway nội bộ; các operation khác PHẢI gọi gateway public; NẾU gateway nội bộ không kết nối được, THÌ portal PHẢI báo "Cần kết nối mạng quản trị (VPN)" | FR-SIS-007 · ADR-0027 · ADMIN_PORTAL §1.2 | TC-CMS-WS-03 |
| F050-AC-14 | KHI nhận Problem Details, apiClient PHẢI ánh xạ theo `code` (không theo `title`) sang khóa dịch `error.<CODE>`, `errors[]` của `VALIDATION_FAILED` sang lỗi từng trường, mã chưa có khóa sang `error.generic`, và hiển thị `trace_id` trong chi tiết lỗi | ADR-0010 · API_DESIGN §4 | — |
| F050-AC-15 | KHI nhận `401 TOKEN_EXPIRED`, apiClient PHẢI gọi `TokenProvider.refresh()` đúng một lần rồi lặp lại request; refresh thất bại PHẢI phát sự kiện `session_expired`; `403 TENANT_MISMATCH` PHẢI phát `tenant_mismatch`; `403 MFA_REQUIRED` PHẢI phát `mfa_required`; `429` PHẢI tôn trọng `Retry-After` | API_DESIGN §4.1 · ADR-0065 · KEYCLOAK_SSO §3.3 | TC-CMS-WS-02 |
| F050-AC-16 | apiClient PHẢI chỉ tự retry GET hoặc request có `Idempotency-Key` khi lỗi mạng hoặc 502/503/504, với số lần và backoff giới hạn; PHẢI KHÔNG retry 4xx (trừ 429) | API_DESIGN §12.3 · ADR-0010 | — |
| F050-AC-17 | apiClient PHẢI KHÔNG lưu token vào `localStorage`/`sessionStorage`/cookie và PHẢI KHÔNG ghi header `Authorization` hay body chứa mật khẩu vào log/console | AGENTS R14 · KEYCLOAK_SSO §5 | — |

---

### User Story 4 — Storybook, trợ năng và visual regression (Priority: P2)

UX và QA xem mọi component `ops-ui` ở sáng/tối/compact, có kiểm axe tự động và baseline ảnh.

**Why this priority**: NFR-A11Y-003; nền cho TC-UI-004/006 của các màn sau.

**Independent Test**: `pnpm storybook:build` + test-runner axe + Playwright snapshot trên Storybook.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F050-AC-18 | Storybook 8 của `ops-ui` PHẢI build thành công và mọi component PHẢI có story ở light, dark và compact | NFR-A11Y-003 · DESIGN_SYSTEM §9 | TC-UI-005 |
| F050-AC-19 | KHI chạy test-runner, axe PHẢI kiểm mọi story; NẾU có vi phạm mức serious/critical, THÌ CI PHẢI fail | NFR-A11Y-003 · ADR-0047 | TC-UI-006 |
| F050-AC-20 | Playwright PHẢI chụp snapshot các story ở light/dark/compact và zoom 200%; sai khác vượt ngưỡng PHẢI yêu cầu duyệt baseline | NFR-A11Y-003 · DESIGN_SYSTEM §4.5 | TC-UI-004 |
| F050-AC-21 | KHI PR thay đổi `src/cms/frontend/**`, `specs/openapi/**`, `design/admin-tokens/**` hoặc `design/shared-tokens/**`, GitHub Actions PHẢI chạy workflow `.github/workflows/web.yml` theo path filter (lint, test, build portal, build Storybook, axe, visual) | [DEPLOYMENT §10.2](../../docs/11-operations/DEPLOYMENT.md) · ADR-0088 | — |

### Edge Cases

- NẾU spec gộp thêm `ProblemCode` mới chưa có khóa dịch, THÌ hiển thị `error.generic` + `code`, và test i18n cảnh báo khóa thiếu (F050-AC-14).
- NẾU người dùng thao tác lại sau lỗi mạng, THÌ request ghi dùng lại cùng `Idempotency-Key` (F050-AC-12, F050-AC-16).
- NẾU System Admin không có tenant làm việc và gọi operation `optional`, THÌ không gửi header (đọc liên tenant dùng `tenant_ids`) (F050-AC-11).
- Authorization và tenant isolation thực thi ở backend (F002); portal chỉ gắn header đúng và ẩn menu là tiện ích (ADMIN_PORTAL §1.2).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CMS-009 | Khung một Admin Portal React TS, 5 workspace lazy-load, Admin/Ops UI (Ant Design 5 + Pro) | ADMIN_PORTAL §1, DESIGN_SYSTEM §4 |
| NFR-MAINT-007 | Theme từ token, lint chặn hardcode, chặn import game | DESIGN_SYSTEM §3, §9 |
| NFR-A11Y-003 | Storybook + axe + visual regression nền | DESIGN_SYSTEM §4.5, §9 |

### Key Entities

- **Workspace**: vùng chức năng của portal (School & Learning, Content Studio, Game Operations Console, Game Live Operations, Platform Administration) — một chunk mã.
- **Operation metadata**: thuộc tính mỗi operation API cần cho client (idempotency, If-Match, header tenant, gateway).
- **TokenProvider / TenantContext**: nguồn token và tenant làm việc (hiện thực ở F051).
- **Problem**: lỗi chuẩn có `code` ổn định, ánh xạ khóa dịch.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `tenant_header_required` | Đối chiếu metadata `tenantHeader` sinh từ spec |
| `idempotency_record_ttl` | Giới hạn thời gian giữ `Idempotency-Key` khi retry phía client |
| `portal_session_sso` | Hành vi khi refresh thất bại (F051 dùng) |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không business logic trong component | AGENTS R1 |
| Không hardcode URL/credential | AGENTS R3 |
| Không mang phong cách/Theme game vào web | AGENTS R32 · ADR-0047 |
| WCAG 2.1 AA, zoom 200% | NFR-A11Y-003 · DESIGN_SYSTEM §4.5 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Web dev mới chạy được portal khung trong ≤ 10 phút sau khi có stack F001.
- **SC-002**: 0 vi phạm hardcode màu/font/khoảng cách và 0 import game trong `src/cms/frontend/` (CI).
- **SC-003**: 0 lỗi axe mức serious/critical trên mọi story `ops-ui`.
- **SC-004**: 100% operation spec có metadata đúng; 100% mã `ProblemCode` hiển thị được thông điệp (riêng hoặc chung).

## Ngoài phạm vi

- Đăng nhập OIDC `eduworld-portal`, chọn tenant sau đăng nhập, menu theo `GET /me/permissions`, bộ lọc tenant System Admin → F051.
- Nội dung workspace: C quản trị trường → F052, F055, F056; B nội dung → F053, F054; D platform → F057; A Game Ops → F079; E live-ops/analytics → F080, F122.
- Component đặc thù (trình soạn câu hỏi, `MathText`/`MathInput`, graph editor, `GamePreviewFrame`, `ResponsiveTable`) → feature màn tương ứng.

## Assumptions

- Đầu ra token của F005 (`antd-theme.ts`, `tokens.css`) có sẵn; F050 không sửa file sinh.
- Gateway nội bộ chỉ truy cập được qua VPN ở dev/staging/prod; local dùng `apisix-internal` (F004).
- MSW chỉ dùng trong test và Storybook, gắn nhãn MOCK, không vào bundle production (R17).
