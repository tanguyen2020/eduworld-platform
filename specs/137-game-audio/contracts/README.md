# Contracts: F137 — Âm thanh game: nhạc theo vùng/màn, âm thanh UI và sự kiện, ambience, phù hợp giáo dục, asset CC0/MIT

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

F137 không thêm hay đổi operation nào. Client chỉ đọc trường đã có:

| operationId / schema | Trường dùng | Dùng cho | Trạng thái |
|---|---|---|---|
| `MiniGameConfig.presentation` (`specs/openapi/modules/game.v1.yaml`, `specs/schemas/minigame-config.schema.json`) | `background_music_id` (`maxLength: 64`) | Nhạc của mini-game (F137-AC-04) | Có sẵn (F073) |
| Activity/challenge render payload | `time_limit_seconds` | Xác định "có giờ" để tắt nhạc (F137-AC-08) | Có sẵn |
| `PlayerBootstrap` / Exam Zone | `AppState.blocking_activity` (client) từ luồng bài kiểm tra F064 | Tắt nhạc khi kiểm tra | Có sẵn |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| — | Không có (chỉ client) | — | — | — |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| Không thêm message. Âm thanh trận (`match_start`, `match_win`, `match_lose_gentle`) và boss (`boss_hit`, `boss_defeated`) phát theo message đã có của F105/F109/F110 | server → client | Có sẵn |

## 4. Event (NATS)

| Event | Phát / tiêu thụ | Schema | Trạng thái |
|---|---|---|---|
| — | Không áp dụng (client không dùng NATS, R25) | — | — |

## 5. JSON Schema / file dữ liệu client

| File | Schema | Kiểm bởi | Trạng thái |
|---|---|---|---|
| `src/game/resources/audio/audio_catalog.json` | `src/game/tools/audio/schemas/audio-catalog.schema.json` (mọi chuỗi có `maxLength`/`pattern`/`enum`, R28) | `lint_audio.mjs` (ajv), test GdUnit4 | Cần thêm (T004) |
| `src/game/assets/audio/audio_manifest.json` | `src/game/tools/audio/schemas/audio-manifest.schema.json` | `lint_audio.mjs` | Cần thêm (T004) |

Hai schema là công cụ asset nội bộ của client, không phải contract API, nên đặt cạnh tool (không trong `specs/schemas/`).

## 6. Delta cần thực hiện

| Delta | Nơi | Ghi chú |
|---|---|---|
| Không có delta OpenAPI/AsyncAPI/event | — | Quyết định ADR-0106 mục 8: id âm thanh là asset trình bày; dùng `background_music_id` sẵn có |
| Thêm 2 JSON Schema cho catalog + manifest | `src/game/tools/audio/schemas/` | T004 |
| Thêm dòng `assets/audio/**` vào `src/game/assets/LICENSES.md` | client | T005 |
| Câu hỏi mở F137-2 (nhạc theo sự kiện/season do Game Studio cấu hình) | Feature sau, module `game` | Nếu owner muốn: thêm `music_id` vào cấu hình sự kiện (contract `game`) — ngoài phạm vi F137 |
