# Quickstart: F119 — Giao diện chat trong Godot: panel, quick message, tab, tắt tiếng, báo cáo

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F092, F102, F109, F115–F118; F121 cho báo cáo thật (trước đó Prism mock gắn nhãn MOCK, chỉ build debug).
- Lệnh: `make up`, `make seed`, `make up-app`; mở `src/game/` bằng Godot 4.x, chạy `res://scenes/bootstrap/bootstrap.tscn` hai instance với `--device-id` khác nhau ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a`, `hs.chi.3b` (TH Demo A), `hs.dung.7a`; `sm.antoan` để xem báo cáo trong hàng đợi (F121).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | An tạo party, mời Bình; mở lobby | Panel chat có ngay; thu gọn → tin mới hiện badge; không có nút tắt chat | F119-AC-01 |
| 2 | Team battle 2v2 | Tab "Đội" và "Cả trận", badge riêng | F119-AC-02 |
| 3 | Câu hỏi hiển thị trong trận (mobile) | Panel thu thành dải 2 dòng, không che đáp án | F119-AC-03 |
| 4 | Ngắt mạng An, gửi tin, nối lại | Đồng hồ → "Đang kết nối lại chat…" → đã gửi đúng một lần | F119-AC-04, F119-AC-05 |
| 5 | Kết thúc trận, gửi tin; mở lịch sử trận | Ô nhập khóa; "Tin nhắn trong trận được giữ N ngày" (N từ server) | F119-AC-06, F119-AC-07 |
| 6 | An (vi) chạm "Chúc may mắn!", Bình đặt `en-US` | Bình thấy bản tiếng Anh | F119-AC-08 |
| 7 | An gửi tin có SĐT; gửi dồn | Thông điệp `ui.chat.feedback.pii_blocked`, không lộ chuỗi khớp; "Bạn gửi hơi nhanh…" | F119-AC-09, F119-AC-10 |
| 8 | Bình bị mute (auto-mute F116) | Ô nhập khóa kèm giờ mở lại, vẫn đọc tin | F119-AC-11 |
| 9 | Bình nhấn giữ tin của An → Báo cáo → `rude_language` | ≤ 2 thao tác tới hộp báo cáo; `createReport` có `match_id`; `sm.antoan` thấy case | F119-AC-12, F119-AC-13 |
| 10 | Màn kết quả trận → "Báo cáo người chơi", "Chặn" | Gọi API, tin của người bị chặn ẩn ngay | F119-AC-14, F119-AC-15 |
| 11 | Mở picker: pack khóa, GIF theo chủ đề | Ổ khóa + gợi ý; không có ô tìm Internet; ≤ 3 ảnh động phát | F119-AC-16, F119-AC-17 |
| 12 | Thả cảm xúc, trả lời, sao chép tin đã che, thu hồi tin của mình | Số đếm cập nhật; clipboard chỉ bản đã che; thu hồi hỏi xác nhận; nhãn/placeholder đúng | F119-AC-18, F119-AC-19, F119-AC-20 |
| 13 | Gửi 500 tin vào channel trên thiết bị tầm trung | FPS trong ngân sách; ≤ `chat_recent_buffer` tin trong RAM | F119-AC-21 |
| 14 | Chạy pseudo-locale `xx-PS`; `make godot-test` lint theme | Không tràn chữ, không chuỗi/màu hardcode | F119-AC-22 |
| 15 | Quét mã `src/game/` | Không kết nối NATS/Valkey, không logic kiểm duyệt | F119-AC-23 |

## 3. Lệnh test tự động

```bash
make godot-test            # GdUnit4 headless: tests/unit/chat, tests/scene/chat
make godot-export-check
```

## 4. Bằng chứng cần nộp

- Kết quả GdUnit4, ảnh chụp panel ở `vi`/`en`/`xx-PS` (PC + mobile), số liệu FPS profiler thiết bị lab, video thao tác báo cáo ≤ 2 chạm; ghi rõ kịch bản nào chạy trên Prism mock.

## 5. Bằng chứng (2026-11-27, phần Godot client)

Chạy trên worktree `feat/f119`, Godot 4.7.2 headless, fake + fixture sinh từ contract (`ChatFixtures`, `ChatSceneHarness`, `ChatScreens`, `RealtimeHarness` — TEST-ONLY, AGENTS R17); không backend. `createReport` không cần Prism mock (F121 Done).

| Lệnh | Kết quả |
|---|---|
| GdUnit4 `res://tests/unit/chat` | `test_chat_store.gd` 13/13, `test_chat_service.gd` 12/12, `test_chat_service_retry.gd` 7/7, `test_feedback_mapper.gd` 5/5, `test_quick_message_catalog.gd` 4/4, `test_report_service.gd` 4/4, `test_block_mute_hide.gd` 3/3, `test_chat_assets_models.gd` 7/7, `test_chat_boundaries.gd` 6/6 |
| GdUnit4 `res://tests/scene/chat` + lobby | `test_chat_dock.gd` 7/7, `test_chat_composer.gd` 9/9, `test_chat_action_menu.gd` 9/9, `test_chat_bubbles_picker.gd` 9/9; `test_party_lobby.gd::test_lobby_shows_the_party_chat_under_the_activity` pass |
| GdUnit4 `res://tests/scene/test_grade_band_matrix.gd` (phần chat) | `test_chat_dock_fits_every_band` (10 trạng thái), `test_chat_dock_fits_every_band_with_long_text` (pseudo-locale +35%: đội, menu, báo cáo, bị tắt tiếng), `test_party_lobby_with_chat_open_fits_every_band` — 4 khối × 1280×720 / 1024×600 / 844×390: pass; các test party F102 vẫn pass với panel chat trong màn Nhóm |
| GdUnit4 `res://tests/perf/test_chat_list_perf.gd` | 2/2 — 500 tin vào một channel: giữ 200 (`chat_recent_buffer`), pool bubble < 30, tin mới ≤ ngân sách cập nhật UI; typing gộp ≤ 1 lần/giây |
| GdUnit4 `res://tests/integration/test_chat_flow_headless.gd` | bỏ qua (cần backend compose + `EW_REALTIME_PUBLIC_URL`, `EDUWORLD_IT_API_BASE_URL`) — như các test live khác |
| GdUnit4 toàn bộ `res://tests` | 1314 test case — 1300 pass, 0 lỗi, 0 fail, 14 bỏ qua (test live cần backend, gồm `test_chat_flow_headless.gd`) |
| Lint Godot | `check_typed` OK (635 file), `lint_theme_overrides` OK (512), `lint_layers` OK (296), `lint_signals` OK (376), `lint_art` OK (443), `bake_art --check` 0 stale, `brand_assets --check` OK, `extract_pot --check` OK (462), `pseudo_locale --check` OK, `gen_fixtures --check` OK (154 file), `check_no_admin_ui` OK, `scan_artifact_test` 22/22 |
| Token | `make design-tokens-check`: 57/57 test, tương phản AA đạt (cặp chữ/nền chat mới), generated khớp, semver OK (`tokens@0.20.0`) |
| Web | export Web (preset "Web") + `check_web_export` OK (wasm 39,5 MB, pck 8,3 MB) + `scan_artifact` OK; `node --test src/game/web/tests/` 17 pass, 4 bỏ qua (Playwright chưa cài trình duyệt), 0 fail |
| Specs/docs | `check-features` 0 lỗi, `check-cross-features` 0 lỗi, `check-maxlength` 0, `check-enums` 0, `validate-schemas` 0, `split-public --check` khớp, `check-links` 0 hỏng, `check-mermaid` 0 lỗi, `build-rtm --check` khớp |

### 5.1. Trạng thái tiêu chí

| AC | Bằng chứng (fake + fixture) | Còn chờ chạy thật |
|---|---|---|
| F119-AC-01 | `test_chat_dock` (panel có ngay, thu gọn có badge, không tùy chọn tắt), `test_party_lobby` (màn Nhóm), ma trận `party_chat` | Hai client thật trong party (quickstart #1); gắn trận/boss chờ F109/F110 |
| F119-AC-02 | `test_chat_dock::test_team_battle_has_team_and_match_tabs_with_their_own_badges` | Trận đội thật (chờ F109 + `chat_channel_ids`) |
| F119-AC-03 | `test_chat_dock::test_question_shrinks…` (EventBus `question_shown/hidden`), ma trận `chat_mini` | Màn câu hỏi F109/F110 phát sự kiện |
| F119-AC-04 | `test_chat_store` (thứ tự `seq`, sent chỉ khi ack, gap thử một lần, buffer), `test_chat_service_retry` (sync, > buffer → REST) | — |
| F119-AC-05 | `test_chat_service_retry` (CHAT_TEMPORARILY_UNAVAILABLE / mất mạng → chờ + cùng `client_message_id`, REST fallback cùng `Idempotency-Key`, không trùng), `test_chat_dock` (băng) | 20 lần ngắt/nối thật (SC-003) |
| F119-AC-06 | `test_chat_service_retry` (join `NOT_FOUND` thử lại, `CHAT_CHANNEL_CLOSED` khóa) | Channel tạo trễ thật |
| F119-AC-07 | `test_chat_dock` (N từ `purge_after`, 5 và 3 ngày, không hằng số) | — |
| F119-AC-08 | `test_chat_composer` (hàng quick message trên ô nhập, một chạm gửi `quick_message_id`, vi/en theo người xem, g1_3 to + bảng mở), `test_quick_message_catalog`, ảnh `en` | Người nhận `en-US` thật (quickstart #6) |
| F119-AC-09 | `test_chat_composer` (bộ đếm theo `max_text_length`, Enter/Shift+Enter, `client_message_id` riêng) | — |
| F119-AC-10 | `test_feedback_mapper`, `test_chat_service` (pii/từ/care/held/masked/rate), `test_chat_composer::test_blocked_message…` (không lộ chữ) | Pipeline thật (quickstart #7) |
| F119-AC-11 | `test_chat_service` (sanction khóa kèm giờ, vẫn đọc; `CHAT_MUTED`), ma trận `chat_muted` | Auto-mute F116 thật (quickstart #8) |
| F119-AC-12 | `test_chat_action_menu` (chuột phải/nhấn giữ/⋯ → Báo cáo = 2 thao tác; tin của mình không có) | — |
| F119-AC-13 | `test_report_service`, `test_chat_action_menu` (lý do bằng hình, `party_id`, cảm ơn + Chặn) | Case thật ở `sm.antoan` (quickstart #9) |
| F119-AC-14 | `test_chat_action_menu::test_players_list…`, `::test_match_result_safety_row…` (`ChatPlayerSafety`, `match_id`) | Nhúng vào `BattleResult` của F109 |
| F119-AC-15 | `test_block_mute_hide`, `test_chat_action_menu::test_block_from_the_menu…` | Server lọc fan-out thật (test live) |
| F119-AC-16 | `test_chat_bubbles_picker` (3 tab, ổ khóa + gợi ý, chủ đề GIF, không ô nhập), `test_chat_assets_models` (chỉ CDN EduWorld) | Thư viện sticker/GIF thật |
| F119-AC-17 | `test_chat_assets_models` (≤ 3, chỉ khi hiện), `test_chat_bubbles_picker` (5 sticker → ≤ 3 động) | Đo lab thiết bị (TC-PERF-019); GIF động thật chờ sprite sheet (câu hỏi mở) |
| F119-AC-18 | `test_chat_service` (toggle, lỗi giới hạn, số theo `chat.reaction.updated`), `test_chat_bubbles_picker` (viên cảm xúc, của em) | — |
| F119-AC-19 | `test_chat_action_menu` (mục theo features/quyền, thu hồi hỏi lại, sao chép bản đã che), `test_chat_service` | Thời hạn sửa/thu hồi do server (câu hỏi mở) |
| F119-AC-20 | `test_chat_bubbles_picker` (đã chỉnh sửa, chuyển tiếp, thu hồi, tin gốc không còn, typing ≤ 3, ✓/✓✓/xanh có chữ) | — |
| F119-AC-21 | `test_chat_list_perf`, `test_chat_store` (buffer) | FPS thiết bị tầm trung (TC-PERF-009) |
| F119-AC-22 | `test_chat_boundaries` (khóa vi/en, không override/màu), ma trận pseudo-locale, `extract_pot`/`pseudo_locale --check` | — |
| F119-AC-23 | `test_chat_boundaries` (không NATS/Valkey/GIF ngoài, không danh mục/luật kiểm duyệt, không công tắc tắt chat) | — |

**Kết luận:** 23/23 F119-AC có test tự động pass với fake + fixture contract. F119 **In progress** cho tới khi `test_chat_flow_headless.gd` chạy được trên backend của owner (AC-01/05/06/08/10/11/13/15 phần thật), F109/F110 gắn `ChatDock` vào trận/boss/kết quả (AC-01…03, AC-14), và đo lab FPS (AC-17, AC-21).

### 5.2. Rà ảnh — khối × màn (99 ảnh trong [screenshots/](screenshots/), Compatibility; 4 khối × 1280×720 + 844×390 `vi`, `en` g4_5 1280×720)

| Màn | g1_3 | g4_5 | g6_7 | g8_9 | en (g4_5) |
|---|---|---|---|---|---|
| `party_chat` (màn Nhóm + chat) | Đạt — PC: thành viên + hoạt động trái, chat cột phải; bảng mặt cười mở sẵn; điện thoại: nút "Chat nhóm" + panel trượt phải | Đạt | Đạt | Đạt | Đạt ("Party chat", quick message tiếng Anh) |
| `chat_team` (trận đội, tab Đội/Cả trận) | Đạt — ghim đáy tin mới nhất, typing, bảng mở sẵn | Đạt — hàng gõ chữ | Đạt | Đạt | Đạt |
| `chat_menu` | Đạt — 6 cảm xúc, Trả lời/Trích dẫn/Sao chép, "GIỮ AN TOÀN" Tắt tiếng/Chặn/Báo cáo | Đạt | Đạt | Đạt | Đạt |
| `chat_report` | Đạt — 7 lý do hình + chữ, chọn = vàng + ✓, "Gửi báo cáo" sáng khi chọn | Đạt | Đạt | Đạt | Đạt |
| `chat_thanks` | Đạt — Mây + cảm ơn + gợi ý Chặn | Đạt | Đạt | Đạt | Đạt |
| `chat_picker` (Nhãn dán) | Đạt — 3 tab một hàng (chữ xuống dòng trong tab), "Gõ chữ", pack khóa có ổ khóa + gợi ý | Đạt | Đạt | Đạt | Đạt |
| `chat_blocked` (PII) | Đạt — băng vàng, không lặp chữ đã gõ | Đạt | Đạt | Đạt | Đạt |
| `chat_muted` | Đạt — ô nhập khóa + giờ mở lại, một băng (không trùng) | Đạt | Đạt | Đạt | Đạt |
| `chat_closed` | Đạt — "Chưa có tin nhắn nào trong kênh này.", "giữ 5 ngày", khóa | Đạt | Đạt | Đạt | Đạt |
| `chat_care` | Đạt — Mây + thông điệp quan tâm + 111, ô soạn ẩn tới "Em hiểu rồi" | Đạt | Đạt | Đạt | Đạt |
| `chat_mini` (dải 2 dòng) | Đạt | Đạt | Đạt | Đạt | Đạt |

Đã sửa trong lúc rà: tin mới nhất bị cắt (ghim đáy khi danh sách tự cuộn), bảng mặt cười và hàng gõ chữ cùng hiện làm panel tràn (thay phiên), tab bảng xuống 2 hàng (tab đều nhau một hàng), icon tab xanh trên nền xanh (icon trắng `tab_*`), icon trên nút chính khi mờ (`flag`/`send_blue`), ô nhập nhiều dòng nền xám mặc định (theme `TextEdit`), băng tắt tiếng lặp hai lần, lời mời "chào mọi người" ở kênh đã đóng, panel chat bị gỡ liên kết khi chuyển sang overlay điện thoại, màn Nhóm PC để chat chỉ còn một khe nhỏ (chat chiếm cột phải).
