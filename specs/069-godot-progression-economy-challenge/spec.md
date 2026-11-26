# Feature Specification: F069 — Godot — tiến trình, kho đồ, shop, bộ sưu tập, thành tựu, challenge solo

**Feature Branch**: `069-godot-progression-economy-challenge` · **Created**: 2026-11-16 · **Status**: Done · **Module**: `src/game/` · **Mốc**: M3 · **Phụ thuộc**: F062, F074, F075, F076 (và F063 activity player, F070 `ProfileService`/`CustomizationStudio`, F092 `RealtimeClient`)
**Input**: Học sinh thấy level, danh hiệu, thanh XP, ví Xu trên HUD; xem hiệu ứng lên cấp và "Hành trình của tôi"; mở Tủ đồ/Shop/Lịch sử, mua cosmetic bằng Xu có xác nhận; xem thành tựu, bộ sưu tập, chọn huy hiệu trưng bày; chơi challenge một người (7 loại, 5 mức khó) với đồng hồ server. Màn S03 (phần tiến trình), S06 (phần challenge solo), S07, S08, S09 của [STUDENT_GAME_UX](../../docs/05-ux/STUDENT_GAME_UX.md). FR nguồn: FR-GAM-001, FR-GAM-006, FR-GAM-007, FR-GAM-008, FR-ECO-001…005, FR-RWD-001…003, FR-CHL-001, FR-CHL-003, FR-GAME-004.

## Clarifications

### Session 2026-11-16

- Q: Tủ đồ của F069 và `CustomizationStudio` của F070 trùng nhau? → A: F070 sở hữu `ProfileService.update_avatar`, `AvatarBuilder`, `AvatarPreview3D`, `CustomizationStudio` (trang bị, preset, emote wheel). F069 bọc nó thành tab "Tủ đồ" của màn S08 `InventoryShopScreen`, thêm bộ lọc `kind`, lưới ảo hóa, tab Shop và Lịch sử, ví Xu; `InventoryService` (F070 tạo) được mở rộng, không viết lại.
- Q: Boss (solo và party) thuộc F069 hay F110? → A: F110 làm toàn bộ màn boss S06 (solo + party) vì backend boss là F103; F069 chỉ làm challenge solo 7 loại của F076 (`time`, `perfect`, `survival`, `puzzle`, `knowledge`, `weekly`, `seasonal`).
- Q: Phím mở Thành tựu: STUDENT_GAME_UX_SOCIAL §4 ghi `J` nhưng `J` là Nhật ký nhiệm vụ ([STUDENT_GAME_UX §4.1](../../docs/05-ux/STUDENT_GAME_UX.md), tài liệu sở hữu)? → A: Theo §4.1: không gán phím; mở từ menu ☰ và hồ sơ. Phím `B` (Shop) theo SOCIAL §3, bổ sung vào §4.1 ở task Polish.
- Q: Ví Xu cập nhật thế nào khi chưa có message realtime riêng cho số dư? → A: Lấy `balance_after` từ `PurchaseResult`; gọi lại `getWallet` khi mở HUD/panel, sau `progression.level.reached`, `challenge.attempt.updated` kết thúc, và `notification.item.created` loại `reward`/`achievement` (F123). Không polling.
- Q: Độ khó bị khóa hiển thị điều kiện mở (AC-UX-0601) khi contract chỉ có `available_difficulties`? → A: Client chỉ hiển thị khóa khi server trả lý do (delta `GameChallengeDetail.difficulties[]`, [contracts](contracts/README.md) §6); không tự suy luật khối ở client (R1).

### Session 2026-11-26

Owner có lệnh thường trực "thiết kế game thật dễ dùng và thu hút học sinh" và "chốt hết": điểm thiếu được chốt bằng mặc định an toàn, thân thiện với trẻ nhất (theo F096/F097, ADR-0103) và ghi [Confirmed].

- Q: Tab "Tủ đồ" của S08 so với `CustomizationStudio` (F070) có cơ chế nháp → Lưu? → A: [Confirmed] Tab Tủ đồ là lưới kho `listInventory` (`VirtualItemGrid`, lọc `kind`) + thanh chi tiết "Trang bị" gọi ngay `updateAvatar` qua `ProfileService`, xem trước chỉ đổi theo response (F069-AC-08); nút "Phối đồ" mở `CustomizationStudio` F070 (ngoại hình, bộ phối, vòng biểu cảm). Màn S08 có screen id `inventory` (`{tab: wardrobe|shop|history, item_id?}`); `wardrobe` vẫn là studio F070. Mỗi màn chỉ một `AvatarPreview3D` (component mới, SubViewport ≤ 512 px, render một lần mỗi lần đổi đồ/kéo xoay).
- Q: Mở Thành tựu, Núi Thử Thách, Hành trình ở đâu khi HUD chưa có menu ☰ (STUDENT_GAME_UX §3)? → A: [Confirmed] Cột phải của HUD (đối xứng thẻ "Việc tiếp theo") có chip Xu (mở tab Cửa hàng) và nút "Thêm" (icon lưới + chữ) mở `game_menu`: bốn ô lớn Cửa hàng, Thành tựu & Bộ sưu tập, Núi Thử Thách, Hành trình của tôi (≤ 2 chạm, AC-UX-0301). Không sửa vùng 3D lượt này (cổng/NPC Núi Thử Thách và NPC Thợ may Bông là follow-up khi có seed NPC).
- Q: Đặt cấp/danh hiệu/thanh XP thế nào khi `PlayerBadge` đã chật ở `g1_3`? → A: [Confirmed] Bảng `XpPanel` ngay dưới `PlayerBadge` (cấp, danh hiệu một dòng có "…", thanh + "40/120 XP", mặt trăng khi chạm trần); thẻ "Việc tiếp theo" dời xuống dưới bảng (bố cục theo signal, không `_process`).
- Q: S07 có xem trước 3D nhân vật? → A: [Confirmed] Không — Mây ăn mừng (vui) trong bong bóng "Lên cấp 4!" để giữ một viewport trên máy tầm trung (NFR-COMP-001); phần thưởng là chip icon + chữ, Xu/XP cùng loại được cộng gộp ("100 Xu", không hai chip "50 Xu"); một cấp mới đến khi modal đang mở thì modal đổi ngay thành "Lên 2 cấp!" (không hai modal liên tiếp).
- Q: Hoãn S07 tới lúc nào? → A: [Confirmed] `AppState.blocking_activity` (`begin_blocking`/`end_blocking`, nhiều nguồn) = `challenge` từ khi bắt đầu tới khi đóng màn kết quả (data-model §2 Starting…Result), `exam` trong `ExamRunner` (F064); modal hiện ngay khi về rỗng.
- Q: Số ô huy hiệu trưng bày khi danh mục không có trường `badges_displayed_max`? → A: [Confirmed] Lấy `maxItems` của `AvatarConfig.badge_showcase` (3 = CONFIG_DEFAULTS `badges_displayed_max`); server vẫn kiểm. `g1_3` không có tab Huy hiệu trưng bày (STUDENT_GAME_UX_SOCIAL §4: chỉ sổ dán + thành tựu rút gọn).
- Q: Nhận biết thành tựu ẩn khi contract chưa có `hidden`? → A: [Confirmed] `hidden = true` (delta) hoặc `category = hidden`, và `unlocked = false` → model bỏ tên, mô tả, tiến độ, phần thưởng; màn chỉ hiện "?" + gợi ý.
- Q: Độ khó `available = false` mà server không gửi lý do? → A: [Confirmed] Không hiện (chỉ hiện mức khóa có `locked_reason_code` thuộc danh mục delta); không có `difficulties[]` thì hiện đúng `available_difficulties`.
- Q: Tiếp tục challenge sau mở lại app khi contract chưa có `active_attempt_id`? → A: [Confirmed] Client đọc `active_attempt_id` khi có (đề nghị "Tiếp tục" trên bảng); khi chưa có, chỉ tiếp tục được trong phiên đang chạy (RAM) và `409 CONFLICT` hiện "Em đang có một thử thách chưa xong…"; phụ thuộc delta F076 (T004) — ghi ở "Câu hỏi mở F069-1".
- Q: HUD challenge đặt ở đâu khi màn làm bài phủ toàn màn? → A: [Confirmed] Dải ngang trên cùng (`ChallengeHud`: tên, thanh thời gian — `g1_3` không số giây, lớp 4–9 thêm mm:ss — tim sức bền, "Đã làm n · Điểm", "Tiếp tục", "Dừng lại"); trong lúc chạy HUD chính lùi (ẩn thanh trên, cột phải, thẻ tiếp theo) và `ActivityHost.set_top_inset` để màn làm bài nằm dưới dải. Dừng lại luôn qua hộp xác nhận có Mây ("Các bài đã làm vẫn được ghi lại").
- Q: Đồng hồ server cho challenge khi REST không có `server_time`? → A: [Confirmed] Lấy độ lệch đồng hồ realtime (pong) nếu đã đồng bộ, nếu không thì header `Date` của response, cuối cùng giờ thiết bị tại lúc đồng bộ; sau đó chỉ cộng thời gian đơn điệu (`ClientClock.now_msec`) — đổi giờ máy không ảnh hưởng (SC-005). Thời gian chờ cooldown hiện "Nghỉ một chút nhé, thử lại sau mm:ss" theo cùng đồng hồ.
- Q: Khóa Idempotency của một ý định mua khi học sinh đóng hộp xác nhận sau lỗi mạng? → A: [Confirmed] Khóa giữ tới khi server trả lời dứt khoát (2xx/4xx); bấm Mua lại cùng món dùng lại khóa (không bị trừ hai lần); hộp báo "Mạng đang chập chờn… em sẽ không bị trừ Xu hai lần".
- Q: Độ hiếm "màu + biểu tượng" vẽ thế nào? → A: [Confirmed] Component `RarityMark`: màu `rarity.*` + hình khác nhau (thường = chấm tròn, ít gặp = thoi, hiếm = sao, sử thi = sao đôi, huyền thoại = vương miện); chữ độ hiếm ở thanh chi tiết cho lớp 4–9, tooltip mọi lớp.
- Q: Ảnh vật phẩm khi chưa có atlas thumbnail render sẵn (`thumbnail_url` chưa dùng)? → A: [Confirmed] Component `ItemThumb` vẽ vector hình loại vật phẩm (áo, tóc, mũ, mặt cười, thú, ruy băng, huy chương, khung, thẻ, lấp lánh) theo màu art palette của `ItemVisual` hoặc màu độ hiếm, viền mực ADR-0103 — không texture riêng mỗi món; món chưa có dùng cùng hình qua shader `ui_greyscale.gdshader` + chữ "Chưa có". Atlas thumbnail từ `tools/thumbnail_renderer` là follow-up.
- Q: Hạn bán (`available_to`) có thành áp lực? → A: [Confirmed] Chỉ hiện ngày yên lặng "Có bán đến 30/11" ở thanh chi tiết, không đếm ngược, không "chỉ hôm nay" (R19); hết hạn → `SHOP_ITEM_UNAVAILABLE` "Món này sẽ quay lại mùa sau."

**Câu hỏi mở**

- F069-1: `active_attempt_id` (`GameChallengeSummary`) và `difficulties[]` (`GameChallengeDetail`) của delta F076 ([contracts §6](contracts/README.md)) chưa có trong OpenAPI — client đã đọc khi có; tới khi có, tiếp tục challenge sau khi mở lại app/đổi thiết bị chỉ qua thông báo `409 CONFLICT` và mức khóa có lý do không hiện [Proposed: owner F076 bổ sung delta T004; không cần đổi client].
- F069-2: Cổng/NPC "Núi Thử Thách" trong vùng `challenge_mountain` và NPC Thợ may Bông ở Thị trấn Trung Tâm mở thẳng S06/S08 [Proposed: khi có seed NPC/interactable (WORLD_DESIGN), gắn intent `challenge_board` / `inventory {tab: shop}` — nay mở qua HUD "Thêm" và chip Xu].
- F069-3: Ảnh thumbnail vật phẩm render sẵn trong atlas (`thumbnail_url`, `tools/thumbnail_renderer`) [Proposed: `ItemThumb` vector theo loại + màu art palette tới khi pipeline xuất atlas cho client].

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Tiến trình trên HUD, lên cấp, "Hành trình của tôi" (Priority: P1)

Học sinh làm xong bài thì thấy số XP bay lên thanh tiến độ, lên cấp có màn ăn mừng với danh hiệu và phần thưởng đã vào túi; xem lại các lần nhận XP (US-GAM-001).

**Why this priority**: Không thấy tiến độ thì XP không tạo động lực; HUD là nơi mọi feature khác đổ phần thưởng về.

**Independent Test**: Chạy với Prism mock/F074: phát `progression.xp.updated` (có `capped`) và hai `progression.level.reached` liên tiếp → thanh XP đổi ≤ 1 s, một modal "Lên 2 cấp!"; trong lúc challenge `in_progress` modal chờ tới khi kết thúc.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F069-AC-01 | KHI học sinh vào thế giới, game client PHẢI gọi `getProgression` và `getWallet`, hiển thị `PlayerBadge` (level + `title.label` theo ngôn ngữ), `XpBar` (`xp_in_level` trên `xp_in_level + xp_to_next`) và `CurrencyChip`; client PHẢI không chứa bảng ngưỡng level, đường cong XP hay danh sách danh hiệu | FR-GAM-007 · FR-GAM-008 · [PROGRESSION_SYSTEM §4.6](../../docs/01-game-design/PROGRESSION_SYSTEM.md) | — |
| F069-AC-02 | KHI nhận `progression.xp.updated`, game client PHẢI cập nhật `XpBar` trong ≤ 1 giây không polling (số bay ≤ 0,8 s, tắt khi bật giảm chuyển động); NẾU `capped = true`, THÌ PHẢI hiện biểu tượng mặt trăng và tooltip `ui.hub.daily_cap_reached`; NẾU `xp_to_next = null`, THÌ thanh đầy kèm nhãn cấp tối đa | FR-GAM-008 · AC-GAM-004 · [STUDENT_GAME_UX_CORE S03](../../docs/05-ux/STUDENT_GAME_UX_CORE.md) | — |
| F069-AC-03 | KHI nhận `progression.level.reached`, game client PHẢI mở `LevelUpModal` với `to_level`, `title`, `rewards` từ payload (không có nút "Nhận"); nhiều message chưa hiển thị PHẢI gộp một modal "Lên {count} cấp!" ở level cao nhất; TRONG KHI đang có hoạt động có giờ (challenge `in_progress`, bài kiểm tra, phase boss), modal PHẢI xếp hàng và hiện ngay sau khi hoạt động kết thúc | FR-GAM-008 · AC-GAM-005 · [STUDENT_GAME_UX_CORE S07](../../docs/05-ux/STUDENT_GAME_UX_CORE.md) (AC-UX-0701, 0702, 0704) | TC-GAM-005 |
| F069-AC-04 | `LevelUpModal` PHẢI bỏ qua hiệu ứng bằng một thao tác, tự đóng sau 10 giây, focus mặc định nút "Tuyệt!"; `g1_3` chỉ có "Tuyệt!", `g4_5` thêm "Trang bị ngay" (mở S08 đúng vật phẩm), `g6_9` thêm XP còn lại và "Hành trình của tôi" | FR-GAM-008 · AC-UX-0703 | — |
| F069-AC-05 | KHI học sinh mở "Hành trình của tôi", game client PHẢI hiển thị `listXpHistory` (cursor, 20 dòng/trang) với nhãn theo khóa `ui.journey.source.<source_code>` (thiếu khóa thì dùng `source_label`), mốc level và danh hiệu từ `listProgressionLevels`; PHẢI không hiển thị ID kỹ thuật và không trình bày XP/level như điểm học tập | FR-GAM-008 · FR-GAM-006 · [PROGRESSION_SYSTEM §2](../../docs/01-game-design/PROGRESSION_SYSTEM.md) | — |
| F069-AC-06 | HỆ THỐNG PHẢI không có request nào từ game client chứa `xp`, `level`, `score`, `amount`, `lives_left` hay kết quả hoàn thành; mọi giá trị tiến trình hiển thị chỉ lấy từ response/message của server | FR-RWD-003 · ADR-0016 · AGENTS R5 | — |

---

### User Story 2 — Tủ đồ, Shop, Lịch sử (Priority: P1)

Học sinh mở Tủ đồ (phím `I`), lọc theo loại, trang bị món mình có; sang Shop (phím `B` hoặc NPC Thợ may Bông) xem giá Xu, xem trước, xác nhận rồi mua; mạng chập chờn không bị trừ hai lần.

**Why this priority**: Đích dùng Xu và rủi ro dark pattern/tài chính ảo cao nhất (R19).

**Independent Test**: Với `FakeApiClient`: bấm Mua, mạng lỗi, thử lại → hai request cùng `Idempotency-Key`; số dư lấy từ `balance_after`; `409 INSUFFICIENT_BALANCE` hiện "Cần thêm {n} Xu"; lưới 500 vật phẩm cuộn ≥ 30 FPS trên Android tầm trung.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F069-AC-07 | KHI học sinh mở S08 (phím `I` → tab Tủ đồ, `B` → tab Shop, NPC Thợ may → Shop, "Trang bị ngay" từ S07), game client PHẢI tải `listInventory` theo `kind` đang lọc và `getWallet`; `VirtualItemGrid` PHẢI chỉ tạo Control cho ô đang hiển thị + 1 hàng đệm, dùng thumbnail 2D trong atlas và đúng một `AvatarPreview3D` (F070) chỉ cập nhật khi đổi đồ hoặc kéo xoay | FR-ECO-001 · FR-GAME-004 · [STUDENT_GAME_UX_SOCIAL §3](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md) (AC-UX-103) | — |
| F069-AC-08 | KHI học sinh trang bị, game client PHẢI gọi `updateAvatar` qua `ProfileService` (F070) và chỉ đổi xem trước theo response; NẾU nhận `422 ITEM_NOT_OWNED` hoặc `422 EQUIP_SLOT_INVALID`, THÌ PHẢI trả xem trước về trạng thái cũ và hiện thông điệp `ui.error.<CODE>` | FR-ECO-001 · AC-ECO-008 | TC-ECO-007 |
| F069-AC-09 | KHI mở tab Shop, game client PHẢI hiển thị `listShopItems` với giá Xu, `purchase_limit`/`purchased_count`, nhãn "Đã sở hữu", `available_to`; NẾU số dư hiển thị nhỏ hơn giá, THÌ nút Mua PHẢI xám kèm "Cần thêm {n} Xu" (chỉ hiển thị — server quyết định) | FR-ECO-003 · [INVENTORY_ECONOMY §5](../../docs/01-game-design/INVENTORY_ECONOMY.md) | — |
| F069-AC-10 | KHI học sinh bấm Mua, game client PHẢI luôn hiện `PurchaseConfirmDialog` với giá và số dư sau mua (không có "Mua nhanh"), sau xác nhận gọi `purchaseShopItem` với một `Idempotency-Key` cho mỗi ý định và giữ nguyên key khi thử lại do lỗi mạng/5xx; số dư PHẢI lấy từ `balance_after`, vật phẩm mới từ `item` của `PurchaseResult` | FR-ECO-003 · AC-ECO-001 · AC-UX-102 | TC-ECO-001 |
| F069-AC-11 | NẾU `purchaseShopItem` trả `409 INSUFFICIENT_BALANCE`, `PURCHASE_LIMIT_REACHED`, `ITEM_ALREADY_OWNED` hoặc `SHOP_ITEM_UNAVAILABLE` ("Sẽ quay lại mùa sau"), THÌ game client PHẢI hiện thông điệp thân thiện theo khóa `ui.error.<CODE>`, làm mới ví/danh sách và không hiển thị mã thô; `428 PRECONDITION_REQUIRED`/`422 IDEMPOTENCY_KEY_REUSED` PHẢI ghi telemetry `client_bug` và hiện thông điệp chung | FR-ECO-002 · FR-ECO-003 · AC-ECO-002 | TC-ECO-001 |
| F069-AC-12 | KHI mở tab Lịch sử, game client PHẢI hiển thị `listWalletLedger` và `listInventoryHistory` (cursor) với nhãn `ui.inventory.reason.<reason_code>` (quy đổi trùng: "Đã quy đổi thành {amount} Xu"); `g1_3` PHẢI ẩn tab Lịch sử | FR-ECO-002 · FR-ECO-004 · AC-ECO-005 | TC-ECO-004 |
| F069-AC-13 | HỆ THỐNG PHẢI không có trong game client màn, nút hay SDK mua bằng tiền thật (IAP), hộp/phần thưởng ngẫu nhiên, trao đổi hay tặng vật phẩm, đếm ngược khuyến mãi; test quét scene và dependency build PHẢI thất bại nếu xuất hiện | FR-RWD-002 · FR-ECO-003 · AGENTS R19 | TC-ECO-003 |

---

### User Story 3 — Challenge một người theo mức khó (Priority: P1)

Học sinh mở Núi Thử Thách, chọn challenge hợp khối và mức khó, chơi các hoạt động học tập thật với đồng hồ của server; kết thúc thấy tóm tắt và phần thưởng, không bị chê "thất bại".

**Why this priority**: Vòng chơi chính của M3; mỗi hoạt động là attempt học tập riêng (FR-CHL-003).

**Independent Test**: `hs.an.3a` (khối 3) thấy mức `easy`–`hard`, `hs.dung.7a` thấy thêm `expert`/`legendary`; đổi giờ máy khi chơi `time` → đồng hồ vẫn khớp `deadline_at`; tắt app giữa chừng, mở lại → tiếp tục đúng lượt.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F069-AC-14 | KHI học sinh mở `ChallengeBoard`, game client PHẢI hiển thị `listGameChallenges` (lọc `type`, `area_id`) gồm tên, loại, `attempts_left` và chỉ các mức trong `available_difficulties`; mức bị khóa PHẢI chỉ hiện khi server trả lý do khóa; challenge `party_required` PHẢI không có nút chơi một mình | FR-CHL-001 · [STUDENT_GAME_UX_CORE S06](../../docs/05-ux/STUDENT_GAME_UX_CORE.md) (AC-UX-0601) | TC-CHL-002 |
| F069-AC-15 | KHI học sinh chọn một challenge, game client PHẢI hiển thị `getGameChallenge` (mô tả, luật, thời gian, sức bền, thưởng lần đầu, lượt còn lại); NẾU `attempts_left = 0`, THÌ PHẢI hiện "Em đã dùng hết lượt hôm nay…" kèm gợi ý luyện tập; NẾU có `cooldown_until`, THÌ PHẢI hiện đếm ngược nhẹ theo đồng hồ server và khóa nút Vào | FR-CHL-001 · AC-CHL-008 | TC-CHL-008 |
| F069-AC-16 | KHI học sinh bấm Vào với một mức khó, `ChallengeClient` PHẢI gọi `startGameChallenge` (`difficulty`, `Idempotency-Key` giữ nguyên khi thử lại); NẾU nhận `403 CHALLENGE_NOT_ELIGIBLE` hoặc `409 CHALLENGE_ATTEMPT_LIMIT`, THÌ PHẢI hiện thông điệp thân thiện kèm thời điểm thử lại; NẾU nhận `409 CONFLICT`, THÌ PHẢI đề nghị tiếp tục challenge đang chạy | FR-CHL-001 · NFR-SAFE-003 | TC-CHL-008 |
| F069-AC-17 | TRONG KHI challenge `in_progress`, `ChallengeClient` PHẢI lấy hoạt động bằng `nextChallengeActivity` (`Idempotency-Key` theo lượt) và mở `ActivityHost` (F063) với `attempt` server trả, không tự chọn câu; KHI attempt học tập nộp xong, PHẢI gọi hoạt động kế; NẾU nhận `503 UNAVAILABLE`, THÌ PHẢI thử lại cùng key theo backoff và không đổi trạng thái hiển thị | FR-CHL-003 · ADR-0016 · ADR-0059 | TC-CHL-003 |
| F069-AC-18 | `ChallengeHud` PHẢI tính thời gian còn lại từ `deadline_at` bằng `Clock` (offset giờ server, lệch hiển thị ≤ 1 s, đổi giờ máy không ảnh hưởng) — `g1_3` dạng thanh màu không số giây; điểm, sức bền, tiến độ PHẢI chỉ cập nhật từ `challenge.attempt.updated` hoặc `getGameChallengeAttempt` | FR-CHL-001 · AC-CHL-001 · AC-UX-0602 | TC-CHL-001 |
| F069-AC-19 | KHI challenge chuyển `succeeded`/`failed`/`abandoned` (`done = true` hoặc message), game client PHẢI mở `ChallengeResult` với phần thưởng từ server; `failed` PHẢI hiện "Em đã hoàn thành {n} hoạt động" và gợi ý ôn, không có chữ "Thất bại" đỏ; bỏ cuộc PHẢI qua hộp xác nhận rồi gọi `abandonGameChallengeAttempt` (`Idempotency-Key`) | FR-CHL-001 · AGENTS R19 · [CHALLENGE_BOSS_SYSTEM §3.5](../../docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md) | TC-CHL-006 |
| F069-AC-20 | KHI kết nối lại, mở lại game hoặc takeover sang thiết bị khác, game client PHẢI phát hiện challenge đang chạy (`active_attempt_id`), gọi `getGameChallengeAttempt` và cho tiếp tục tới `deadline_at`; NẾU nhận `422 INVALID_STATE_TRANSITION`, THÌ PHẢI tải lại trạng thái và hiện kết quả của server | FR-CHL-001 · ADR-0031 | — |

---

### User Story 4 — Thành tựu, bộ sưu tập, huy hiệu trưng bày (Priority: P2)

Học sinh xem thành tựu theo nhóm và tier, bộ sưu tập (pet, thẻ kiến thức…) với "Cách nhận" cho món chưa có, chọn huy hiệu trưng bày cho người khác thấy.

**Why this priority**: Động lực dài hạn; phụ thuộc trigger từ nhiều feature nên làm sau vòng chính.

**Independent Test**: Fixture có achievement ẩn chưa mở (response có lỡ chứa `title`) → màn chỉ hiện "?"; bộ sưu tập 3/10 hiển thị 7 bóng xám có "Chưa có" + "Cách nhận".

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F069-AC-21 | KHI học sinh mở tab Thành tựu, game client PHẢI hiển thị `listAchievements` theo `category` với thanh `progress_current`/`progress_target` và `tier`; thành tựu ẩn chưa mở khóa PHẢI chỉ hiện "?" và `ui.achievement.hidden_hint`, không hiển thị tên, mô tả hay tiến độ kể cả khi response có trường đó | FR-ECO-005 · AC-ECO-006 | TC-ECO-005 |
| F069-AC-22 | HỆ THỐNG PHẢI không có nút "Nhận thưởng" thủ công: thành tựu đã mở khóa hiển thị "Đã nhận" và phần thưởng server đã cấp | FR-RWD-003 · FR-ECO-005 | — |
| F069-AC-23 | KHI học sinh mở một bộ sưu tập, game client PHẢI hiển thị `getCollection`: món đã có dùng thumbnail màu, món chưa có dùng cùng texture với shader xám kèm chữ "Chưa có" và "Cách nhận"; `g1_3` PHẢI hiển thị dạng sổ dán hình lớn | FR-ECO-004 · [INVENTORY_ECONOMY §6](../../docs/01-game-design/INVENTORY_ECONOMY.md) | — |
| F069-AC-24 | KHI học sinh lưu huy hiệu trưng bày, game client PHẢI chỉ cho chọn huy hiệu đang sở hữu, số ô theo danh mục server (`badges_displayed_max`), và lưu bằng `updateAvatar` (`badge_showcase[]`) qua `ProfileService` | FR-ECO-001 · FR-ECO-005 | TC-ECO-007 |

---

### User Story 5 — Song ngữ, trợ năng, token, hiệu năng, an toàn (Priority: P2)

**Why this priority**: Bắt buộc theo constitution (V, VI, VIII) nhưng kiểm được sau khi luồng chính chạy.

**Independent Test**: Pseudo-locale `xx` chụp S07/S08/S09/S06; `lint_theme_overrides.gd`; benchmark lưới 500 ô; đăng xuất rồi kiểm `user://` và store.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F069-AC-25 | Mọi chuỗi UI của feature PHẢI qua khóa `ui.progression.*`, `ui.levelup.*`, `ui.journey.*`, `ui.inventory.*`, `ui.shop.*`, `ui.achievement.*`, `ui.collection.*`, `ui.challenge.*` trong `vi.po`/`en.po`; dữ liệu `LocalizedName`/`LocalizedText` PHẢI chọn theo ngôn ngữ hiện tại; số Xu/XP định dạng theo locale; bố cục PHẢI chịu chuỗi dài hơn 35% | NFR-I18N-001 · NFR-I18N-003 · ADR-0056 | TC-I18N-004, TC-I18N-007 |
| F069-AC-26 | game client PHẢI lấy màu, font, khoảng cách từ Theme sinh từ `design/game-tokens` theo density grade band (vùng chạm ≥ 48 dp, `g1_3` ≥ 64 dp); độ hiếm PHẢI dùng màu `rarity.*` kèm biểu tượng (lớp 4–9 thêm chữ), không chỉ dựa vào màu | NFR-A11Y-001 · ADR-0047 · AGENTS R32 | TC-UI-001, TC-UI-003, TC-UI-008 |
| F069-AC-27 | Mỗi panel S07/S08/S09/S06 PHẢI mở ≤ 200 ms (skeleton khi chờ dữ liệu) và lưới 500 vật phẩm PHẢI cuộn ≥ 30 FPS trên thiết bị Android tầm trung tham chiếu; HUD PHẢI cập nhật theo signal, không trong `_process` | NFR-COMP-001 · [STUDENT_GAME_UX §6](../../docs/05-ux/STUDENT_GAME_UX.md) (AC-UX-0004) | — |
| F069-AC-28 | game client PHẢI giữ dữ liệu tiến trình/ví/kho/challenge chỉ trong RAM (không ghi `user://`), xóa khi đăng xuất hoặc `session.session.replaced`; TRONG KHI mất mạng, PHẢI cho xem dữ liệu đã tải và khóa Mua, Trang bị, Vào challenge; script trong `scenes/**` PHẢI chỉ gọi method public của service | AGENTS R1 · ADR-0064 · [GODOT_ARCHITECTURE §6](../../docs/08-game-client/GODOT_ARCHITECTURE.md) | — |
| F069-AC-29 | Mọi request PHẢI đi qua `ApiClient` (gắn `X-Tenant-Id` của phiên); NẾU nhận `403 GAME_ACCESS_STUDENT_ONLY` hoặc `TENANT_MISMATCH`, THÌ PHẢI theo luồng F061 (xóa token, về đăng nhập); NẾU nhận `404 NOT_FOUND` cho challenge/collection, THÌ PHẢI làm mới danh sách và hiện "Thử thách không còn mở" | AGENTS R23 · ADR-0065 | TC-AUTH-014 |
| F069-AC-30 | game client PHẢI chỉ gửi giá trị enum/uuid/cursor do server cấp (không có ô nhập tự do trong feature); NẾU nhận `422 VALIDATION_FAILED`, THÌ PHẢI ghi telemetry `client_bug` không PII và hiện thông điệp chung | ADR-0041 · AGENTS R14 | — |

### Edge Cases

- NẾU `progression.level.reached` đến khi đang mở S08, THÌ modal hiện chồng; "Trang bị ngay" chọn đúng vật phẩm trong tab đang mở.
- NẾU phần thưởng chưa có visual (`ItemVisual` thiếu), THÌ hiện biểu tượng chung + tên (như F070-AC-18).
- NẾU mặt hàng hết hạn trong lúc hộp xác nhận đang mở, THÌ nhận `SHOP_ITEM_UNAVAILABLE` (F069-AC-11), không trừ Xu.
- NẾU `challenge.attempt.updated` đến khi `ActivityHost` đang mở câu cuối, THÌ đóng activity sau khi hiển thị phản hồi câu đó rồi chuyển `ChallengeResult`.
- NẾU curve được publish lại (level không đổi nhưng `xp_to_next` đổi), THÌ `XpBar` lấy lại từ `getProgression` ở lần `progression.xp.updated` kế tiếp.
- NẾU học sinh lên lớp (khối mới), THÌ danh sách challenge làm mới khi mở lại, không giữ cache khối cũ.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-GAM-001 | Hiển thị XP, level, achievement, badge, collection | [PROGRESSION_SYSTEM](../../docs/01-game-design/PROGRESSION_SYSTEM.md) · [INVENTORY_ECONOMY §6–§7](../../docs/01-game-design/INVENTORY_ECONOMY.md) |
| FR-GAM-006 | Không trình bày game level/XP như kết quả học tập | PROGRESSION_SYSTEM §2 |
| FR-GAM-007 | Không hardcode đường cong/danh hiệu trong Godot; hiển thị danh hiệu từ server | PROGRESSION_SYSTEM §4 |
| FR-GAM-008 | Thanh tiến độ, hiệu ứng lên cấp, phần thưởng lên cấp, lịch sử XP/level | PROGRESSION_SYSTEM §4.5–§4.6 · [STUDENT_GAME_UX_CORE S07](../../docs/05-ux/STUDENT_GAME_UX_CORE.md) |
| FR-ECO-001 | Tủ đồ theo loại, trang bị, huy hiệu trưng bày | [INVENTORY_ECONOMY §2–§3](../../docs/01-game-design/INVENTORY_ECONOMY.md) · [STUDENT_GAME_UX_SOCIAL §3](../../docs/05-ux/STUDENT_GAME_UX_SOCIAL.md) |
| FR-ECO-002 | Ví Xu, lịch sử sổ cái (chỉ đọc) | INVENTORY_ECONOMY §4 |
| FR-ECO-003 | Shop cosmetic giá Xu, xác nhận mua, idempotent | INVENTORY_ECONOMY §5 |
| FR-ECO-004 | Bộ sưu tập, thẻ kiến thức, hiển thị quy đổi trùng | INVENTORY_ECONOMY §6 |
| FR-ECO-005 | Thành tựu theo nhóm/tier, ẩn | INVENTORY_ECONOMY §7 |
| FR-RWD-001 | Hiển thị cosmetic nhận từ quest/challenge/thành tựu | INVENTORY_ECONOMY §5.2 |
| FR-RWD-002 | Không tiền thật, không loot box trong client | [GAME_DESIGN §13](../../docs/01-game-design/GAME_DESIGN.md) |
| FR-RWD-003 | Phần thưởng chỉ hiển thị từ server, không nút "nhận" | [ANTI_CHEAT §7](../../docs/10-security/ANTI_CHEAT.md) |
| FR-CHL-001 | Màn challenge solo 7 loại, 5 mức khó, lượt thử, cooldown, đồng hồ server | [CHALLENGE_BOSS_SYSTEM §2](../../docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md) |
| FR-CHL-003 | Mỗi hoạt động chơi qua activity player trên attempt học tập riêng | CHALLENGE_BOSS_SYSTEM §2.3 |
| FR-GAME-004 | Inventory và character level trên client | [CHARACTER_SYSTEM](../../docs/01-game-design/CHARACTER_SYSTEM.md) |

NFR: NFR-SAFE-003 (không streak/áp lực), NFR-I18N-001, NFR-I18N-003, NFR-A11Y-001, NFR-COMP-001.

### Key Entities

- **ProgressionModel**: level, danh hiệu, XP trong cấp, XP tới cấp sau, XP còn nhận hôm nay, cờ cap.
- **WalletModel / LedgerEntryModel**: số dư Xu và dòng lịch sử Xu/vật phẩm.
- **InventoryItemModel / ShopItemModel**: vật phẩm sở hữu; mặt hàng bán kèm giá, giới hạn, đã sở hữu.
- **AchievementModel / CollectionModel**: thành tựu (tier, tiến độ, ẩn), bộ sưu tập và món.
- **ChallengeSummary / ChallengeDetail / ChallengeAttemptModel**: challenge khả dụng, chi tiết mức khó, lượt chơi (trạng thái, hạn chót, sức bền, điểm).
- **LevelUpQueue**: hàng chờ hiệu ứng lên cấp khi đang có hoạt động có giờ.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `badges_displayed_max` | Số ô huy hiệu trưng bày (giá trị đọc từ server) |
| `challenge_attempts_per_day`, `challenge_fail_cooldown` | Chỉ hiển thị (`attempts_left`, `cooldown_until` do server tính) |
| `g1_3_challenge_rules` | Chỉ hiển thị mức server trả cho khối 1–3 |
| `supported_locales` | Ngôn ngữ UI |
| `client_reconnect_backoff` | Thử lại `nextChallengeActivity` khi `503` và kết nối lại |
| `target_fps`, `ui_cpu_budget` | Ngân sách FPS/CPU UI cho lưới và HUD |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không business logic trong UI; service → state → UI | AGENTS R1 · [GODOT_ARCHITECTURE §6](../../docs/08-game-client/GODOT_ARCHITECTURE.md) |
| Không tin client: không gửi XP/điểm/kết quả | AGENTS R5 · ADR-0016 |
| Không pay-to-win, loot box, dark pattern, áp lực thời gian ngoài hoạt động có giờ | AGENTS R19 · NFR-SAFE-003 |
| Theme/token, không hardcode style; song ngữ | AGENTS R32 · ADR-0047 · ADR-0056 |
| Máy tầm trung | NFR-COMP-001 · ADR-0057 · [PERFORMANCE_BUDGET](../../docs/08-game-client/PERFORMANCE_BUDGET.md) |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 95% lần cộng XP hiển thị trên HUD trong ≤ 1 giây sau khi client nhận message.
- **SC-002**: 100 lần mua với mạng ngắt ngẫu nhiên → 0 lần bị trừ Xu hai lần cho cùng một ý định mua.
- **SC-003**: Lưới 500 vật phẩm cuộn ≥ 30 FPS trên Android tầm trung tham chiếu; mỗi panel mở ≤ 200 ms.
- **SC-004**: 0 màn trong feature hiển thị mã lỗi thô, chữ "Thất bại" đỏ, nút mua bằng tiền thật hay nút "nhận thưởng" thủ công (bộ scene test).
- **SC-005**: Đồng hồ challenge lệch ≤ 1 giây so với `deadline_at` khi giờ máy bị đổi ±10 phút.

## Ngoài phạm vi

- Backend XP/level/Reward Engine → F074; ví, shop, inventory, collection, achievement → F075; Challenge Engine → F076.
- Tạo avatar, `CustomizationStudio`, preset, emote wheel, `updateAvatar` → F070. Activity player → F063. Quest tracker/journal → F072. Nhắc nghỉ, HUD khung → F062.
- Màn boss S06 (solo và party), quest co-op, thẻ dữ kiện → F110. Challenge `coop`/`party`/`tournament` → F109/F110.
- Event center, season track, shop sự kiện theo lịch → F077 (Godot event center chưa có mã trong danh mục). Hộp thông báo/toast thưởng → F123.

## Assumptions

- Trước khi F074/F075/F076 `Done`, client chạy trên Prism mock sinh từ `specs/` (profile `mock`, nhãn MOCK — R17); feature chỉ `Done` với backend thật.
- Delta contract ở [contracts §6](contracts/README.md) (`active_attempt_id`, `difficulties[]`, `how_to_obtain`, `hidden`, nhãn song ngữ) do F075/F076 thực hiện [Confirmed]; khi chưa có, client ẩn phần tương ứng (không suy luật).
- `ProfileService`, `InventoryService` (bản đầu), `AvatarPreview3D`, `ItemVisual` có từ F070; `ActivityHost`, `LearningService.resume` từ F063; `Clock` từ F061.
- `AppState.blocking_activity` (mới) do ExamRunner (F064), boss (F110) và `ChallengeClient` đặt để hoãn level-up [Confirmed].
