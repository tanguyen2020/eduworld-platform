# Quickstart: F109 — Giao diện Arena, trận đấu, bảng xếp hạng, giải đấu trong Godot

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F061, F063, F092, F102, F105, F106, F107 (trước đó profile `mock` Prism + fixture AsyncAPI, nhãn MOCK; chỉ nghiệm thu `Done` với backend thật).
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)); chạy hai instance Godot `-- --env=local --device-id=<khác nhau>`.
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a` (khối 3, `g1_3`, TH Demo A), `hs.chi.3b` (khối 3, bị loại trừ khỏi giải), `hs.em.4a` (khối 4, `g4_5`), `hs.dung.7a` (`g6_7`), `ad.truong.a` (policy trường), `qlcm.truong.a`.
- Dữ liệu: season đang mở; giải khối 3 TH Demo A (`grade_challenge`, loại trừ `hs.chi.3b`) ở `registration_open` (tạo bằng `ad.truong.a` qua portal từ template đã gán); "Mục tiêu của khối" (`grade_goal`), "Đấu trường trường", "Xếp hạng rating" có dữ liệu (ADR-0095).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.an.3a` nói chuyện NPC Trọng tài Sấm / nhấn `K` | Arena mở; thẻ chế độ theo server; TierBadge chỉ tên tier | F109-AC-01, F109-AC-15 |
| 2 | Chọn 1v1 Toán → Tìm trận; đi lại trong Hub; bấm Hủy rồi tìm lại | Thanh hàng chờ nổi, hủy biến mất | F109-AC-02 |
| 3 | `hs.binh.3a` tìm cùng chế độ; cả hai bấm Vào trận; thử lần hai một bên từ chối | Ready check đếm theo server; người đã nhận quay lại hàng chờ | F109-AC-03 |
| 4 | Gọi tìm trận khi đã có ticket; chờ tới timeout | Mở thanh hiện có; gợi ý chế độ khác/Robot | F109-AC-04 |
| 5 | Trong trận: trả lời bằng chạm và phím `1`–`4` | Khóa ngay "Đã ghi nhận"; đúng/sai chỉ sau reveal, có ✓/✗ | F109-AC-05, F109-AC-06, F109-AC-08 |
| 6 | Gửi lại câu đã khóa (công cụ debug) | `ANSWER_ALREADY_SUBMITTED` → thông điệp, không gửi lại | F109-AC-07 |
| 7 | Quan sát trận lớp 3 | Không số giây, không "thưởng tốc độ", không gợi ý trả lời nhanh; vùng chạm lớn | F109-AC-09 |
| 8 | Trận đội (fixture/2v2 với `hs.dung.7a` + bot) | Điểm theo đội, đánh dấu lượt chính | F109-AC-10 |
| 9 | Ngắt mạng `hs.binh.3a` 10 giây giữa câu 3 | Bên kia thấy "Đối thủ đang kết nối lại…"; bên rớt dựng lại đúng câu từ snapshot | F109-AC-11 |
| 10 | Đo FPS BattleArena trên DEV-AND-MID1 | Không render thế giới 3D; FPS đạt preset | F109-AC-12 |
| 11 | Mở/đóng câu hỏi khi có tin chat trận; trên Android | Strip thu nhỏ, không che đáp án, không có nút tắt chat | F109-AC-13 |
| 12 | Kết thúc trận; mô phỏng trận `cancelled` | Màn kết quả động viên, points, tier; "Trận đã hủy" | F109-AC-14 |
| 13 | Báo cáo đối thủ với ghi chú dài; chặn đối thủ | Ô ghi chú giới hạn + bộ đếm; gửi một lần | F109-AC-16 |
| 14 | Lịch sử trận → chi tiết | Danh sách cursor, đối thủ chỉ nickname/tier | F109-AC-17 |
| 15 | `hs.an.3a`, `hs.em.4a`, `hs.dung.7a` mở Bảng xếp hạng | Khối 3: `GradeGoalBar` (tiến độ khối + phần em góp, không tên, không hạng); khối 4: top + "nhóm X%"; khối 7: top + quanh mình; chỉ nickname | F109-AC-18, F109-AC-19, F109-AC-20 |
| 16 | `ad.truong.a` tắt "Mục tiêu của khối" của trường; `hs.an.3a` tải lại; mở `leaderboard_id` của trường B | Bảng biến mất; 404 → "Bảng này hiện không mở" | F109-AC-21, F109-AC-18 |
| 17 | Xem log thông báo sau khi bị bạn vượt điểm | Không có thông báo "bị vượt qua"; bảng không tự làm mới | F109-AC-22 |
| 18 | `hs.an.3a` mở Giải đấu → đăng ký → check-in khi mở cửa sổ → xem bracket | Trạng thái cập nhật; bracket "con đường"; `hs.chi.3b` không thấy giải | F109-AC-23, F109-AC-24, F109-AC-25 |
| 19 | Tới giờ trận giải | Toast thông báo → ready check | F109-AC-26 |
| 20 | `hs.an.3a` thách `hs.binh.3a`; lần hai để hết hạn | Vào trận unranked; "Lời thách đấu đã hết hạn" | F109-AC-27 |
| 21 | Xem network log; dùng `match_id` của `hs.dung.7a` | `X-Tenant-Id` mọi request, `Idempotency-Key` ghi; 404 "Không tìm thấy" | F109-AC-28, F109-AC-29 |
| 22 | Đổi `en` và pseudo-locale | Mọi màn không tràn chữ, không chuỗi hardcode | F109-AC-30 |

## 3. Lệnh test tự động

```bash
make godot-test
godot --headless --path src/game -s addons/gdUnit4/bin/GdUnitCmdTool.gd -a res://tests/unit/competition -a res://tests/scene/competition -a res://tests/contract/competition
make design-tokens-check
```

## 4. Bằng chứng cần nộp

- Kết quả GdUnit4 (pass/fail), video trận 1v1 hai client + reconnect, ảnh màn kết quả/bảng xếp hạng ba khối (vi/en), trace_id ticket/trận/đăng ký giải, báo cáo FPS BattleArena trên DEV-AND-MID1.

## 5. Bằng chứng 2026-11-27 (phần Godot client)

Client chạy với fake (`FakeHttp`, `RealtimeHarness`) + fixture sinh từ contract (`node src/game/tools/gen_fixtures.mjs`, 211 file, `--check` sạch). Phần chạy thật hai client và đo FPS lab **chưa làm** — F109 giữ `In progress`.

### 5.1 Test tự động

| Lệnh | Kết quả |
|---|---|
| GdUnit4 `res://tests/contract/competition` | 7/7 pass |
| GdUnit4 `res://tests/unit/competition` (`test_matchmaking_service` 11, `test_battle_client` 9, `test_competition_services` 11) | 31/31 pass |
| GdUnit4 `res://tests/scene/competition` (`test_arena_screen` 6, `test_arena_tabs` 9, `test_battle_arena` 11) | 26/26 pass |
| GdUnit4 `tests/perf/test_battle_arena_budget.gd` | 1/1 pass (0 node 3D/SubViewport, thế giới 3D tạm dừng, không gọi REST trong 120 khung hình, cập nhật trả lời 1,6–2,2 ms ≤ 3,15 ms) |
| GdUnit4 `tests/scene/test_grade_band_matrix.gd` (`test_arena_screens_fit_every_band*`, `test_chat_list_shows_three_messages_in_every_band` có vị trí `space_battle_*` thật) | pass — 4 khối × 1280×720 / 1024×600 / 844×390, vi/en/pseudo +35 % |
| GdUnit4 toàn bộ `res://tests` (sau rebase lên main có fixture sinh lại 1c244843) | 1387 test: 1370 pass, 0 lỗi, 15 bỏ qua (live/backend), 2 test fail ngân sách thời gian (3 assert: `test_area_transition`, `test_party_lobby_budget`) khi máy đang chạy test Go của agent khác; chạy lại `res://tests/perf` khi rảnh: chuyển vùng pass, `test_party_lobby_budget` (F102, không thuộc F109) dao động 2,8–3,9 ms quanh ngưỡng 3,15 ms |
| `tests/integration/competition/test_arena_flow_headless.gd` | bỏ qua — cần backend + `EW_REALTIME_PUBLIC_URL` (**chờ chạy thật**) |
| Lint Godot: `check_typed`, `lint_theme_overrides`, `lint_layers`, `lint_signals`, `lint_art`, `extract_pot --check`, `pseudo_locale --check`, `gen_fixtures --check`, `brand_assets --check`, `bake_art --check`, `check_no_admin_ui` | OK |
| `make design-tokens-check` (icon mới, token giữ `0.20.0`) | 57/57, 832/832 cặp tương phản AA |
| Web export `--export-release "Web"` + `tools/ci/check_web_export.sh` + `scan_artifact.sh` | OK (index.wasm 39 514 754 B, index.pck 8 611 828 B); scan OK (2 artifact, 13 mẫu) |
| `tools/tests/scan_artifact_test.sh` | 22 pass, 0 fail |
| `node --test src/game/web/tests/` | 17 pass, 0 fail, 4 bỏ qua |
| Spec/doc: `check-features`, `check-cross-features`, `check-maxlength`, `check-enums`, `validate-schemas`, `split-public --check`, `check-portal-catalog`, `check-problem-codes`, `check-client-authoritative`, `check-links`, `check-mermaid`, `build-rtm --check` | 0 lỗi (1 cảnh báo có sẵn của F096) |

### 5.2 Trạng thái tiêu chí

| Tiêu chí | Fake + fixture | Còn chờ |
|---|---|---|
| AC-01, AC-04, AC-07…AC-10, AC-13…AC-15, AC-17…AC-25, AC-27, AC-30 | pass (tasks.md ghi test từng tiêu chí) | — (AC-01 "khóa kèm lý do", AC-16 `note`, AC-09/10 trường chưa có trong AsyncAPI: xem Clarifications 2026-11-27) |
| AC-02, AC-03, AC-05, AC-06, AC-11, AC-26, AC-28, AC-29 | pass | chạy thật hai client + server (`test_arena_flow_headless.gd`, kịch bản 2, 3, 5, 9, 19, 21) |
| AC-12 | pass phần headless | FPS BattleArena trên DEV-AND-MID1 (kịch bản 10) |

### 5.3 Ảnh chụp và bảng khối × màn

198 ảnh trong [screenshots/](screenshots/) (renderer Compatibility; `tools/capture_screens.gd --screens=arena*`): 22 màn × 4 khối × 1280×720 và 844×390 (`vi`) + `en` `g4_5` 1280×720. Tên tệp `<màn>-after-<cỡ>-<vi|en>-<khối>.png`. Đã xem từng ảnh (bảng ghép 2×2 theo màn); những gì đã sửa sau khi xem: header trận đội gọn, người mất kết nối bằng icon + băng (không thêm dòng chữ), dải chat cạnh header / dưới câu hỏi, dải kết quả câu gộp "Đúng rồi! +150 điểm Thưởng tốc độ +50" một dòng (844×390 không phải cuộn), giải đang diễn ra mở thẳng ở sơ đồ, chữ `en` "classmates" → "friends in your grade" (ADR-0095).

| Màn (tệp) | `g1_3` | `g4_5` | `g6_7` | `g8_9` |
|---|---|---|---|---|
| Đấu trường — chọn chế độ (`arena`) | thẻ to, chữ lớn, "Đã chọn" + viền vàng | như `g1_3`, chữ vừa | tầng chữ nhỏ (F114), cùng bố cục | như `g6_7` |
| Thách đấu bạn (`arena_duel`) | chip bạn cuộn ngang, nút ngang | như `g1_3` | như `g1_3` | 844×390: hai nút xếp dọc (flow) |
| Hàng chờ / hết giờ (`arena_queue`, `arena_queue_timeout`) | thanh dưới giữa màn, "Luyện với Robot" / "Chọn chế độ khác" | như `g1_3` | như `g1_3` | như `g1_3` |
| Ready check (`arena_ready_check`) | "Bấm Vào trận nhé!" — **không số giây**, chỉ thanh | "Vào trận trong 12 giây" | như `g4_5` | như `g4_5` |
| Câu hỏi trong trận (`arena_battle_question`) | ô đáp án to (≥ 64 dp), **không số giây** (chỉ thanh) | thanh + số giây | như `g4_5`, chữ nhỏ hơn | như `g6_7` |
| Đã ghi nhận / kết quả câu (`arena_battle_recorded`, `arena_battle_reveal`) | băng "Đã ghi nhận! Chờ kết quả nhé."; ✓ + "Đúng rồi!" + điểm | như `g1_3` | như `g1_3` | như `g1_3` |
| Đấu đội / mất kết nối (`arena_battle_team`, `arena_battle_reconnect`) | điểm theo đội, nhãn "Lượt của em"; thẻ đối thủ xám + icon wifi, băng "đang kết nối lại…" | như `g1_3` | như `g1_3` | như `g1_3` |
| Kết quả trận (`arena_result_win`, `_loss`, `_cancelled`, `_quiz_rank`) | Mây động viên, điểm, điểm thi đấu, tier bằng chữ, "Câu cần ôn", chat trận + Báo cáo/Chặn | như `g1_3` | như `g1_3` | như `g1_3` |
| Kết quả quiz khối (`arena_result_quiz_goal`) | thanh "Mục tiêu của khối", không thứ hạng cá nhân | như `g1_3` | như `g1_3` | như `g1_3` |
| Bảng xếp hạng (`arena_leaderboard_goal`, `_top`, `_ranked`) | mỗi tệp là một `visibility` server trả, giống nhau ở 4 khối: `_goal` → `GradeGoalBar` (tiến độ khối + phần em góp, không tên/hạng người khác) | `_top` → top + "Em đang ở nhóm 25%" | `_ranked` → top + "Quanh em", nhãn "Đang cập nhật" khi `stale` | chỉ nickname + avatar ở mọi dạng |
| Lịch sử / chi tiết (`arena_history`, `arena_history_detail`) | thẻ trận, lọc chế độ; chi tiết chỉ nickname + avatar, "Xem lại các câu" | như `g1_3` | như `g1_3` | như `g1_3` |
| Giải đấu (`arena_tournaments`, `arena_tournament_bracket`) | danh sách + chi tiết; sơ đồ dạng **con đường** | **con đường** | sơ đồ **cây** | **cây** |

Khối chỉ đổi những gì server/Theme quyết định (cỡ chữ tầng F114, số giây, dạng sơ đồ); bảng xếp hạng hiển thị đúng `visibility` server trả, client không tự lọc theo khối (R5, R6).

### 5.4 Còn chờ và câu hỏi mở

- Chạy thật hai client với backend (kịch bản 2, 3, 5, 9, 19, 21 — `test_arena_flow_headless.gd`), video trận + reconnect, trace_id; đo FPS BattleArena trên DEV-AND-MID1 (kịch bản 10).
- Câu hỏi mở cho backend/owner ở [spec Clarifications 2026-11-27](spec.md#session-2026-11-27-triển-khai-client--đối-chiếu-contract-trên-main): `speed_bonus_enabled` / `primary_user_ids` / `encouragement` và `chat_channel_ids` chưa có trong AsyncAPI, `ticket_id` trong 409, `option_id` / `id` của câu hỏi, nguồn giao lời thách đấu cho người nhận, mã lỗi `TOURNAMENT_*`, ô `note` của AC-16, thẻ chế độ "khóa kèm lý do" — đều [Proposed].
