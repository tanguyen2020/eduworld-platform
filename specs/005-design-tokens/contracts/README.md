# Contracts: F005 — Design tokens game/admin/shared và pipeline sinh theme

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| — | Không có API | — | — | Không áp dụng |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| — | Không dùng | — | — |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Token theo chuẩn W3C Design Tokens; không thêm schema vào `specs/` (token không phải contract giữa module/client) | Không áp dụng |

Hợp đồng đầu ra (giữa F005 và F050/F060), không thuộc `specs/`:

| Đầu ra | Bên dùng | Giao diện |
|---|---|---|
| `web/packages/ops-ui/src/theme/antd-theme.ts` | F050 | `export const lightTheme, darkTheme, compactTheme: ThemeConfig`; `export const domainColors: Record<string, {light: string; dark: string}>` |
| `web/packages/ops-ui/src/theme/tokens.css` | F050 | `:root` và `[data-theme="dark"]` biến `--ew-*` |
| `godot/resources/ui/theme/eduworld_theme*.tres` | F060 | `Theme` gốc + một file mỗi density |
| `godot/scripts/core/ui_tokens.gd` | F060 | `class_name UiTokens` hằng `const` (spacing, duration, `TEXT_EXPANSION_ALLOWANCE`, màu nghiệp vụ) |

## 6. Delta cần thực hiện

- Không có delta contract.
