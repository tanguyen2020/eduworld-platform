# Contracts: F114 — Student UX pass 6: phong cách viền hoạt hình phân tầng và màn Cài đặt mới

> Feature chỉ đổi token, Theme và giao diện phía client; không đổi hợp đồng giữa client và server.

## 1. API (OpenAPI)

Không có operation mới hay thay đổi. Đăng xuất vẫn qua luồng hiện có (`endGameSession` → `gameLogout`, `AuthService.logout()`); cài đặt không gửi server.

## 2. Internal API

Không có.

## 3. Realtime (AsyncAPI)

Không có.

## 4. Event (NATS)

Không có.

## 5. JSON Schema nội dung

Không có. Hợp đồng nội bộ client: token ↔ Theme/`UiTokens`/`shell_tokens.css`/icon (`make design-tokens-check`: generated, contrast AA, semver); khóa `.po` ↔ `messages.pot` (`extract_pot --check`); tầng 3 ↔ fixture `tokens@0.16.0` (`test_outline_style.gd`).

## 6. Delta cần thực hiện

- Không có delta contract API. Delta design: `tokens@0.17.0` (MINOR — chỉ thêm token và đổi giá trị, không xóa/đổi tên), 17 icon mới trong `design/icons/game/`.
