# Contracts: F113 — Student UX pass 5: game luôn chơi ngang (khóa ngang Android/iOS, màn "Xoay ngang" trên Web)

> Feature chỉ đổi cấu hình hiển thị và giao diện phía client; không đổi hợp đồng giữa client và server.

## 1. API (OpenAPI)

Không có operation mới hay thay đổi. Trong lúc màn "Xoay ngang" hiện, request đang bay và heartbeat (`POST /game/sessions/{id}:heartbeat`, WebSocket ping/pong) vẫn chạy như cũ.

## 2. Internal API

Không có.

## 3. Realtime (AsyncAPI)

Không có.

## 4. Event (NATS)

Không có.

## 5. JSON Schema nội dung

Không có. Hợp đồng nội bộ client: token/icon ↔ `resources/ui/icons` (`make design-tokens-check`); khóa `.po` ↔ `messages.pot` (`extract_pot --check`); biến CSS shell ↔ `shell_tokens.css` (test shell R32); cấu hình hướng ↔ `project.godot`/`export_presets.cfg` (`test_orientation_config.gd`).

## 6. Delta cần thực hiện

- Không có delta contract API. Delta design: icon `rotate_phone` (không đổi token, giữ `tokens@0.16.0`).
