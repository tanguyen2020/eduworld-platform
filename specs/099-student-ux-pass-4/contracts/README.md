# Contracts: F099 — Student UX pass 4: màu trạng thái rõ ràng, bố cục cân, trình phát media thân thiện

> Feature chỉ đổi giao diện phía client; không đổi hợp đồng giữa client và server.

## 1. API (OpenAPI)

Không có operation mới hay thay đổi. Video/bài nghe/mô phỏng vẫn gọi đúng `reportActivityProgress` (heartbeat, `play_started`), `submitActivityResponse` (`cue_answer`, `question`, `simulation_question`, `branch_choice`) và `submitSimulationRun` như F065/F066; "Lùi 10 giây" và tua bằng thanh đi qua `request_seek`/`_seek_to` có sẵn (luật `allow_seek`). Thẻ "Đáp án đúng" vẫn chỉ đọc `ActivityResponseResult.correct_response` (F098).

## 2. Internal API

Không có.

## 3. Realtime (AsyncAPI)

Không có.

## 4. Event (NATS)

Không có.

## 5. JSON Schema nội dung

Không có. Client đọc thêm thông tin có sẵn của payload hiển thị: `chapters[].start_ms` (video), `segments[].start_ms` (bài nghe), `cues[].at_ms`/`questions[].at_ms` để vẽ vạch và dấu trên thanh tiến độ. Hợp đồng nội bộ client: token ↔ Theme Godot + `UiTokens` (`make design-tokens-check`); khóa `.po` ↔ `messages.pot` (`extract_pot --check`).

## 6. Delta cần thực hiện

- Không có delta contract API. Delta token: `tokens@0.16.0` (`design/CHANGELOG.md`).
