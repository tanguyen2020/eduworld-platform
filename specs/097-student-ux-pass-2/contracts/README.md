# Contracts: F097 — Student UX pass 2: màn làm bài như một trò chơi, tạo nhân vật bằng hình, marker không che nhân vật

> Feature chỉ đổi giao diện phía client và token/icon; không đổi hợp đồng giữa client và server.

## 1. API (OpenAPI)

Không có operation mới hay thay đổi. Client dùng như cũ: `startActivityAttempt`, `submitActivityResponse` (kết quả chấm, `feedback`, `explanation`), `requestNextHint`, `getActivityAttemptReview`; S02 dùng catalog nhân vật (`presets[].avatar_config`, `skin_tones`, `hair_colors`) như F070. Payload hiển thị câu hỏi không chứa đáp án hay phản hồi theo phương án (R6) — giữ nguyên.

## 2. Internal API

Không có.

## 3. Realtime (AsyncAPI)

Không có.

## 4. Event (NATS)

Không có.

## 5. JSON Schema nội dung

Không có. Hợp đồng nội bộ client: token `density.*`/`component.*` ↔ Theme Godot + `UiTokens` (`make design-tokens-check`); icon `design/icons/game/*.svg` ↔ `resources/ui/icons/*.svg`; khóa `.po` ↔ `messages.pot` (`extract_pot --check`); tín hiệu renderer `read_aloud_requested` (QuestionRendererBase → ActivityRendererBase → ActivityHost).

## 6. Delta cần thực hiện

- Không có delta contract API. Delta token: `tokens@0.14.0` (`design/CHANGELOG.md`). Delta cấu hình client: `eduworld/world/marker_near_fade_m` (CONFIG_DEFAULTS).
