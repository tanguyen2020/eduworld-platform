# ADR-0022 — Admin Portal (CMS/LMS) là React + TypeScript SPA

> **Trạng thái:** Accepted (React) · chi tiết stack Proposed · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md), [DESIGN_SYSTEM](../05-ux/DESIGN_SYSTEM.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Stakeholder xác nhận frontend quản trị dùng **React** và chọn **một Admin Portal duy nhất** (ADR-0027).

## Quyết định

- **[Confirmed]** Frontend quản trị là SPA **React**: một ứng dụng `web/apps/portal` với 5 workspace (ADR-0027), component đặc thù trong `web/packages/ops-ui`.
- **[Proposed]** Stack: TypeScript, Vite, React Router, TanStack Query, API client sinh từ OpenAPI bằng `openapi-typescript`, React Hook Form + Zod, `oidc-client-ts`; test Vitest + Testing Library, E2E Playwright.
- **Giao diện Admin/Ops** chuyên cho vận hành game: Ant Design 5 + Ant Design Pro với theme EduWorld [Proposed], không theo phong cách game (ADR-0047). Công thức: MathJax 4 + MathLive (ADR-0055).
- Đăng nhập OIDC Authorization Code + PKCE qua client `eduworld-portal`; MFA bắt buộc với role platform. Token giữ trong bộ nhớ, không lưu `localStorage`.
- Ẩn/hiện UI theo permission chỉ là tiện ích; backend luôn quyết định quyền.
- Preview nội dung game bằng Godot Web export nhúng trong khung preview.

## Hệ quả

(+) Một codebase, một quy trình phát hành cho mọi người dùng quản trị. (−) Workspace platform cần mạng nội bộ/VPN (ADR-0027).

## Phương án đã loại

Vue, Angular (stakeholder chọn React). Next.js/SSR: ứng dụng nội bộ sau đăng nhập, không cần SEO/SSR.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR (Proposed) | Phase 0 |
| 2026-11-15 | Accepted: React | Stakeholder |
| 2026-11-15 | UI Admin/Ops bằng Ant Design 5 + Pro | ADR-0047 |
| 2026-11-15 | Một portal, client `eduworld-portal` (thay `eduworld-cms`/`eduworld-admin`, 2 bundle) | ADR-0027 |
