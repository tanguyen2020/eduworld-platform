# Contracts: F096 — Student UX pass 1: nhận diện EduWorld và làm mượt hành trình học sinh

> Feature chỉ đổi giao diện phía client, Web shell và token/icon; không đổi hợp đồng giữa client và server.

## 1. API (OpenAPI)

Không có operation mới hay thay đổi. Client dùng như cũ: `gameLogin` (S01; "Hủy" chỉ bỏ phản hồi phía client), `listMyAssignments` (đếm bài cho thẻ Việc tiếp theo), `listWorldAreas`/`getWorldExploration` (World Map, trường `icon_key` đã có), `getActivityAttemptReview` (màn kết quả). Mã lỗi `UNAVAILABLE` (Problem `ServiceUnavailable`, 503) đã có trong spec.

## 2. Internal API

Không có.

## 3. Realtime (AsyncAPI)

Không có.

## 4. Event (NATS)

Không có.

## 5. JSON Schema nội dung

Không có. Hợp đồng nội bộ client: token `component.*` ↔ `shell_tokens.css` ↔ `shell.html`; icon `design/icons/game/*.svg` ↔ `resources/ui/icons/*.svg`; `tools/brand_assets.gd` ↔ `project.godot`/`export_presets.cfg` (`--check`); khóa `.po` ↔ `messages.pot` (`extract_pot --check`).

## 6. Delta cần thực hiện

- Không có delta contract API. Delta token: `tokens@0.13.0` (`design/CHANGELOG.md`): biến shell CSS `component.{logo,backdrop,mascot,progress}.*`, icon game mới.
