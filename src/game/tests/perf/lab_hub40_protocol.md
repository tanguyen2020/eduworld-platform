# Quy trình đo Hub 40 người trên thiết bị lab — F092 (T025)

**Tiêu chí:** F092-AC-20 (FPS, TC-PERF-009), F092-AC-22 (băng thông, TC-PERF-023), F092-AC-23 (rò rỉ bộ nhớ 60 phút, TC-PERF-021).
**Ngưỡng:** [PERFORMANCE_BUDGET §16](../../../../docs/08-game-client/PERFORMANCE_BUDGET.md) (AC-PERF-001/002/003/006/009), `quality_presets`, `hub_sync_bandwidth` ([CONFIG_DEFAULTS](../../../../docs/00-overview/CONFIG_DEFAULTS.md)) — tài liệu này chỉ mô tả cách đo, không đặt ngưỡng.

Phần chạy được trong CI (headless, không renderer) nằm ở `test_hub40_headless.gd`: số nhân vật đầy đủ theo preset, LOD animation > 30 m, thời gian script mỗi khung với 40 người, băng thông lên, tái dùng pool `RemotePlayer`. FPS, băng thông xuống và soak bộ nhớ cần renderer và thiết bị thật nên đo theo quy trình dưới.

## 1. Chuẩn bị

- Backend compose: `make up`, `make seed`, `make up-app` ([LOCAL_DEVELOPMENT](../../../../docs/11-operations/LOCAL_DEVELOPMENT.md)); bot hub `tests/load/hubbot` (F091) cấu hình 39 bot vào `main_hub`, di chuyển ngẫu nhiên, emote 1 lần/30 s.
- Thiết bị: DEV-AND-MID1, DEV-PC-MID, DEV-WEB-MID ([PERFORMANCE_BUDGET §2](../../../../docs/08-game-client/PERFORMANCE_BUDGET.md)); preset Trung bình; build release Android / Web (`make godot-export-android`, export Web của F133; DEV-PC-MID chạy bản Web — ADR-0101) trỏ `realtime_url` tới compose qua `wss` (chứng chỉ dev trong CA bundle của build lab).
- Tài khoản: `hs.an.3a` (client thật).

## 2. Đo

| # | Bước | Ghi lại |
|---|---|---|
| 1 | Vào Hub, chờ 39 bot hiện đủ, đứng ở quảng trường trung tâm 10 phút, xoay camera theo vòng | FPS p10 (Profiler Godot / `fps_p10` telemetry), frame time p95 |
| 2 | Cùng phiên: đếm byte WebSocket xuống/lên (`AppState.net_stats.bytes_in/bytes_out`, đối chiếu proxy) | KB/s trung bình mỗi chiều trong 10 phút |
| 3 | Soak 60 phút: bot vào/ra liên tục (mỗi 20 s 1 bot rời, 1 bot vào) | RSS tiến trình đầu/cuối, số node `RemotePlayer` (`RemotePlayerSync.created`) |

## 3. Kết quả

| Thiết bị | Ngày | FPS p10 | Xuống (KB/s) | Lên (KB/s) | RAM đầu → cuối (MB) | Đạt |
|---|---|---|---|---|---|---|
| DEV-AND-MID1 | — | — | — | — | — | Chưa đo |
| DEV-PC-MID | — | — | — | — | — | Chưa đo |
| DEV-WEB-MID | — | — | — | — | — | Chưa đo |

Chưa có thiết bị lab và backend compose trong lần triển khai 2026-11-21 → F092-AC-20, AC-22 (xuống), AC-23 (60 phút) còn chờ đo.
