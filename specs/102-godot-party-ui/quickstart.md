# Quickstart: F102 — Giao diện party trong Godot: panel, lời mời, lobby, ready

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F092, F100, F101 (cho nghiệm thu Done; trước đó dùng profile `mock` của F060, nhãn MOCK), F095 (bạn bè).
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md) §6); hai instance Godot `-- --env=local --device-id=<khác nhau>`.
- Tài khoản demo (mật khẩu seed): `hs.an.3a` (A), `hs.binh.3a` (B) là bạn bè; `hs.chi.3b` (C, cùng khối 3 cùng trường nhưng không phải bạn bè), `hs.em.4a` (D, khối 4 — khác khối). Party lớp 1–3 theo PARTY_SYSTEM (ADR-0095): chỉ `friend`/`quest`, cùng khối với leader.
- Dữ liệu: seed module game có 1 quest party khối 3 (dữ liệu mẫu).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | A đăng nhập, bấm `P` | Màn "Mời bạn bè để cùng chơi"; bấm tạo nhóm → lobby với A có vương miện | F102-AC-01 |
| 2 | A mở `InviteFriendPicker` | Thấy B (avatar + nickname), không có nhóm "bạn cùng lớp"; khối 3 không có ô gõ tên | F102-AC-02 |
| 3 | A mời B | B thấy `InvitationToast` có đếm ngược, tự ẩn sau 10 s, mở lại từ chuông | F102-AC-03 |
| 4 | A (party `friend`) mời C (không phải bạn bè, gọi qua fixture vì picker không liệt kê C); A mời D (khác khối) | Toast lỗi thân thiện theo `ui.party.error.<CODE>` mà server trả (`PARTY_INVITE_NOT_ALLOWED`/`PARTY_POLICY_VIOLATION`), không mã thô | F102-AC-05 |
| 5 | B tạo nhóm riêng rồi chấp nhận lời mời của A | Hộp "Rời nhóm hiện tại và tham gia?"; xác nhận → vào lobby A | F102-AC-04 |
| 6 | B bấm Sẵn sàng | Thẻ B trên màn A đổi ≤ 1 s, biểu tượng + chữ | F102-AC-06, F102-AC-07, F102-AC-08 |
| 7 | B mở menu thẻ A | Không có điều khiển leader ở B; ở A nút Bắt đầu xám "Còn 1 bạn chưa sẵn sàng" khi A chưa Ready | F102-AC-09 |
| 8 | A mở `ActivitySelector` | Thẻ hình lớn (khối 3); boss khối 4 có cảnh báo "{nickname} chưa đủ điều kiện" và không chọn được | F102-AC-10 |
| 9 | A chọn quest, cả hai Ready, bấm Bắt đầu | Đếm ngược 3‑2‑1, cả hai tải `scene_ref`; khi phiên kết thúc quay lại lobby, không thấy điểm của bạn | F102-AC-11 |
| 10 | A thu nhỏ lobby; B rời nhóm (xác nhận) | `PartyHud` hiện ở A; B biến khỏi HUD | F102-AC-12 |
| 11 | Ngắt mạng B trong phiên | Màn A: khung B mờ + nhãn; A vẫn chơi | F102-AC-13 |
| 12 | Ngắt mạng A (leader) ở lobby quá grace | Màn B: băng "Trưởng nhóm đang kết nối lại…", sau đó toast trưởng nhóm mới, điều khiển leader xuất hiện | F102-AC-14 |
| 13 | Bật lại mạng A | Băng vàng biến mất, lobby đồng bộ, A là thành viên thường | F102-AC-15 |
| 14 | Đổi ngôn ngữ `vi` ↔ `en` khi đang mở lobby; chạy `lint_theme_overrides.gd` | Chuỗi đổi ngay, không cắt chữ; 0 override | F102-AC-16, F102-AC-17 |
| 15 | Chạy trên Android 2400×1080 | Thẻ ngang, nút lớn dưới, chat trượt 45% (slot) | F102-AC-18 |
| 16 | Đăng nhập A ở thiết bị thứ ba (takeover) | Thiết bị cũ xóa `PartyStore`, về màn đăng nhập | F102-AC-19 |

## 3. Lệnh test tự động

```bash
make godot-test          # GdUnit4 unit + scene + integration headless
make godot-export-check
```

## 4. Bằng chứng cần nộp

- Kết quả `make godot-test` (pass/fail), ảnh lobby PC/mobile ở `vi` và `en`, video kịch bản 9 và 12, đo thời gian kịch bản 6 trên thiết bị tầm trung.

## 5. Bằng chứng (2026-11-27, phần Godot client)

Chạy trên worktree `feat/f102`, Godot 4.7.2 headless, fake + fixture sinh từ contract (`PartyFixtures`, `PartySceneHarness`, `RealtimeHarness` — TEST-ONLY, AGENTS R17); không backend.

| Lệnh | Kết quả |
|---|---|
| GdUnit4 `res://tests/unit/multiplayer` (phần party) | `test_party_models.gd` 8/8, `test_party_service.gd` 12/12, `test_party_store.gd` 10/10, `test_party_reconnect.gd` 4/4, `test_party_privacy_arch.gd` 5/5, `test_party_session_router.gd` 3/3 |
| GdUnit4 `res://tests/scene/social` | `test_party_lobby.gd` 15/15, `test_party_invites.gd` 9/9, `test_activity_selector.gd` 4/4, `test_party_hud.gd` 2/2 |
| GdUnit4 `res://tests/scene/test_grade_band_matrix.gd` (phần party) | `test_party_lobby_fits_every_band` (lobby, một mình + lời mời, bảng mời, chọn hoạt động đóng/mở, hộp rời nhóm, đếm ngược), `test_party_lobby_fits_every_band_with_long_text` (pseudo-locale +35%), `test_hub_with_party_fits_every_band` — 4 khối × 1280×720 / 1024×600 / 844×390: pass |
| GdUnit4 `res://tests/perf/test_party_lobby_budget.gd` | 1/1 — lobby 4 thành viên: 0 node 3D/SubViewport, 84 Control hiện (≤ 150), cập nhật Ready của một bạn ≈ 1 ms (≤ 3 ms) |
| GdUnit4 `res://tests/integration/test_party_flow_headless.gd` | bỏ qua (cần backend compose + `EW_REALTIME_PUBLIC_URL`, `EDUWORLD_IT_API_BASE_URL`) — như các test live khác |
| GdUnit4 toàn bộ `res://tests` | 1212 test case — 1199 pass, 0 lỗi, 0 fail, 13 bỏ qua (test live cần backend, gồm `test_party_flow_headless.gd`); `scan_artifact_test` 22/22 |
| Lint Godot | `check_typed` OK (597 file), `lint_theme_overrides` OK, `lint_layers` OK, `lint_signals` OK, `lint_art` OK, `bake_art --check` 0 stale, `brand_assets --check` OK, `extract_pot --check` OK, `pseudo_locale --check` OK, `gen_fixtures --check` OK (109 file), `check_no_admin_ui` OK |
| Token | `make design-tokens-check`: 57/57 test, 736/736 cặp tương phản AA, generated 123 file khớp, semver OK (`tokens@0.19.0`) |
| Web | export Web (preset "Web") + `check_web_export` OK (wasm 39,5 MB, pck 8,1 MB) + `scan_artifact` OK; `node --test src/game/web/tests/` 17 pass, 4 bỏ qua (Playwright chưa cài trình duyệt), 0 fail |
| Specs/docs | `check-features` 0 lỗi, `check-cross-features` 0 lỗi, `check-maxlength` 0, `check-enums` 0, `validate-schemas` 0, `split-public --check`, `gen-routes --check` 0 lệch, `check-links` 0 hỏng, `check-mermaid` 0 lỗi, `build-rtm --check` khớp |

### 5.1. Trạng thái tiêu chí

| AC | Bằng chứng (fake + fixture) | Còn chờ chạy thật |
|---|---|---|
| F102-AC-01 | `test_party_service.gd` (key giữ khi thử lại, key mới cho ý định mới), `test_party_lobby.gd` (một mình, tạo → bảng mời, 3 thao tác) | — |
| F102-AC-02 | `test_party_invites.gd` (danh sách server, không nhóm/lọc lớp-khối-trường, không ô gõ tên), `test_party_service.gd` | — |
| F102-AC-03 | `test_party_invites.gd` (đếm ngược theo `expires_at`, tự ẩn 10 s, hết hạn rời danh sách, chuông → màn Nhóm, không bật khi làm bài/tắt popup) | nhận realtime giữa hai client (`test_party_flow_headless.gd`) |
| F102-AC-04 | `test_party_service.gd`, `test_party_lobby.gd`, `test_party_invites.gd` | — |
| F102-AC-05 | `test_party_service.gd` (mọi mã → câu vi/en không chứa mã) | — |
| F102-AC-06 | `test_party_lobby.gd` (thẻ 2D, chỉ thẻ đổi được vẽ lại), `test_party_lobby_budget.gd` | — |
| F102-AC-07 | `test_party_store.gd` (thứ tự `state_version`, nhảy cóc → `getParty`) | **≤ 1 s giữa hai client** |
| F102-AC-08 | `test_party_store.gd` (WS khi nhận lệnh, chỉ đổi sau ack; REST khi WS chưa sẵn sàng), `test_party_lobby.gd` | — |
| F102-AC-09 | `test_party_lobby.gd` (điều khiển chỉ ở trưởng nhóm, Bắt đầu xám + lý do) | — |
| F102-AC-10 | `test_activity_selector.gd` | — |
| F102-AC-11 | `test_party_lobby.gd` (3-2-1 → phòng phiên, tóm tắt chung không điểm), `test_party_session_router.gd` | **vào đúng instance của phiên** |
| F102-AC-12 | `test_party_lobby.gd` (xác nhận rời/kick/chuyển/giải tán, thu nhỏ giữ nhóm), `test_party_hud.gd` | — |
| F102-AC-13 | `test_party_lobby.gd`, `test_party_hud.gd` (thẻ/khung mờ + chữ) | **mất kết nối thật trong phiên** |
| F102-AC-14 | `test_party_lobby.gd` (băng trưởng nhóm, trưởng nhóm mới + điều khiển) | **grace 60 s của server** |
| F102-AC-15 | `test_party_reconnect.gd`, `test_party_lobby.gd` (băng vàng, khóa ghi) | **ngắt mạng thật, vào lại phòng** |
| F102-AC-16 | `test_party_lobby.gd` (đổi ngôn ngữ ngay), ma trận pseudo-locale | — |
| F102-AC-17 | `lint_theme_overrides`, ma trận khối (vùng chạm, cỡ chữ) | — |
| F102-AC-18 | `test_party_lobby.gd` (bố cục điện thoại), ảnh 844×390; slot chat ẩn tới F119 | — |
| F102-AC-19 | `test_party_privacy_arch.gd` | — |
| F102-AC-20 | `test_party_privacy_arch.gd` (quét script `scenes/**` party) | — |

SC-002 (Ready ≤ 1 s p95) và SC-003 (≥ 30 FPS lobby 4 người) cần thiết bị Android tầm trung tham chiếu + backend: chờ lab. SC-001 (≤ 3 thao tác) và SC-004 (0 mã thô / tên thật) có test.

### 5.2. Ảnh và kết quả xem theo khối

Renderer Compatibility (= Web), `tools/capture_screens.gd`, fixture + fake. Ảnh ở [screenshots/](screenshots/) theo mẫu `<màn>-after-<cỡ>-<ngôn ngữ>-<khối>.png`: `party` (trưởng nhóm), `party_member` (thành viên, trưởng nhóm đang kết nối lại), `party_alone` (một mình + lời mời), `party_invite` (bảng mời), `party_activity` (chọn hoạt động), `party_confirm` (rời nhóm), `party_countdown` (3‑2‑1), `hub_party` (HUD + popup lời mời) — 4 khối × 1280×720 và 844×390 `vi`, `en` cho `g4_5` 1280×720 (72 ảnh).

| Màn \ khối | g1_3 | g4_5 | g6_7 | g8_9 |
|---|---|---|---|---|
| Nhóm — trưởng nhóm (`party`) | Đạt — 3 thẻ vừa khung (thẻ thứ 4 cuộn), "Bắt đầu" xám + "Còn 1 bạn chưa sẵn sàng"; 844×390 một hàng thẻ | Đạt | Đạt — PC còn trống phía dưới (slot chat ẩn) | Đạt |
| Nhóm — thành viên (`party_member`) | Đạt — băng vàng trưởng nhóm, thẻ trưởng nhóm xám + "Đang kết nối lại…", không nút trưởng nhóm | Đạt | Đạt | Đạt |
| Một mình (`party_alone`) | Đạt — Mây + nút lớn "Tạo nhóm và mời bạn", dòng lời mời "Lời mời còn 5:00" | Đạt | Đạt | Đạt |
| Bảng mời (`party_invite`) | Đạt — ô hình chân dung + nickname + trạng thái; bạn đã ở trong nhóm = ô vàng ✓ | Đạt | Đạt | Đạt |
| Chọn hoạt động (`party_activity`) | Đạt — 3 thẻ hình lớn, hoạt động đang chơi vàng, Rồng Phân Số có ổ khóa + lý do | Đạt — thẻ hình | Đạt — danh sách thả xuống mở sẵn, hoạt động hiện tại đã chọn | Đạt |
| Hộp rời nhóm (`party_confirm`) | Đạt — tiêu đề vàng, "Hủy" trung tính, "Rời nhóm" đỏ | Đạt | Đạt | Đạt |
| Đếm ngược (`party_countdown`) | Đạt — số 3 lớn viền trắng + "Cả nhóm chuẩn bị nhé!" | Đạt | Đạt | Đạt |
| HUD + lời mời (`hub_party`) | Đạt — popup góc phải, dải chân dung có vương miện + huy hiệu ✓ vàng | Đạt | Đạt | Đạt |

Sửa sau khi xem ảnh: thẻ thành viên gộp nhãn trưởng nhóm, trạng thái và Sẵn sàng vào một dòng chảy (3 thẻ vừa khung ở `g1_3` 1280×720, nickname không bị cắt trên điện thoại); nút "Chọn/Đổi hoạt động" xuống dưới thẻ hoạt động (tên không còn gãy 3 dòng); icon trắng của nút chính bị khóa ("Bắt đầu", "Chọn hoạt động này") đổi sang icon màu nhấn; tên kỹ thuật của hoạt động (`math_g3_multiplication`) không bao giờ hiện — dùng chữ loại hoạt động tới khi có tên; bạn bè ngoại tuyến "Không trực tuyến" (không phải "Đang kết nối lại…"); bố cục điện thoại không còn khoảng trống giữa hàng thẻ và hoạt động; danh sách thả xuống mở với hoạt động hiện tại; nút "Tạo nhóm và mời bạn" rộng hơn; huy hiệu Sẵn sàng trên HUD to hơn (đĩa vàng, dấu mực). Sau test pseudo-locale (+35%): tên nhóm và nickname xuống dòng thay vì bị cắt, các nút của thanh tiêu đề trôi xuống dòng thứ hai, nhãn loại nhóm chuyển cạnh "THÀNH VIÊN", nội dung màn Nhóm cuộn khi không vừa (thanh Sẵn sàng/Bắt đầu luôn ở đáy). Lỗi tìm được khi viết test: lưu cài đặt nhóm đọc chip sau khi hộp thoại đã xóa nội dung — sửa (hộp thoại chỉ ẩn rồi mới báo lựa chọn), có test `test_older_bands_change_who_may_join_from_the_settings`. Ghi chú: popup lời mời có thể che toast thông báo giữa màn trong vài giây khi cả hai đến cùng lúc (popup tự ẩn sau 10 s).

**T031 (hai thiết bị với backend):** chưa chạy — cần stack local của owner với `EW_REALTIME_PUBLIC_URL` (giống F092); `test_party_flow_headless.gd` sẵn sàng chạy khi đặt `EDUWORLD_IT_API_BASE_URL`, `EDUWORLD_IT_STUDENT_PASSWORD`.
