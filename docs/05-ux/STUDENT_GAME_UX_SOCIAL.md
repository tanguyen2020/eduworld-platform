# Student Game UX — Kinh tế, xã hội, thi đấu, sự kiện, cài đặt

> Trạng thái: Draft · Phiên bản: 0.3 · Owner: UX/UI Designer + Game Director · Cập nhật: 2026-11-28 (F111: §6 lát cắt client "Tham gia bạn", nhóm "Vừa chơi cùng", tab Club; §9 Event center — Hôm nay, Tuần này, Sự kiện, Mùa; §11 endpoint đề xuất đã có) · 2026-11-28 (F137 spec Ready: §10 tab Âm thanh thêm công tắc "Tiếng bước chân" và tiếng mẫu khi kéo thanh — STUDENT_GAME_UX §8a) · 2026-11-28 (F112 spec Ready: §6 công cụ chủ club trong game, §7 khung chat HUD cho channel bền vững, DM, câu từ chối trung tính) · 2026-11-27 (F109 lượt delta server: §8 hộp lời thách đấu đến, gợi ý trả lời nhanh theo `speed_bonus_enabled`, dấu sao lượt chính, lời động viên sau chuỗi thua, câu lỗi giải theo mã) · 2026-11-27 (F109: §8 lát cắt client Đấu trường — thẻ chế độ, thanh hàng chờ, ready check, màn trận, kết quả, bảng xếp hạng, lịch sử, giải đấu) · 2026-11-27 (F119 sửa bố cục: §7 quy tắc "≥ 3 tin" cho danh sách tin của panel chat; §5 chat cột phải cao hết cửa sổ trên PC, panel điện thoại từ đỉnh thanh tiêu đề) · 2026-11-27 (F119: §7 lát cắt client chat Godot) · 2026-11-27 (F102: lát cắt client §5 Party lobby — màn Nhóm, thẻ thành viên, mời bạn, lời mời, chọn hoạt động, đếm ngược, HUD Nhóm) · 2026-11-27 (F123: §10 tab Thông báo — công tắc toast; cột tab giãn `FieldBox` để 6 tab + Đăng xuất vừa khung ở mọi khối) · 2026-11-26 (F069: lát cắt client §3 Tủ đồ · Cửa hàng · Lịch sử, §4 Thành tựu · Bộ sưu tập · Trưng bày) · 2026-11-27 (F114: §10 Cài đặt dựng lại — cửa sổ có thanh tiêu đề, 5 tab, âm lượng −/+ bước 10%, chip đang chọn vàng + ✓, hộp xác nhận Đăng xuất) · 2026-11-26 (F096: nhóm Thông tin trong Cài đặt — Giấy phép có thông báo MIT của Godot, Xem lại hướng dẫn của Mây; lựa chọn đang chọn là chip có dấu ✓) · 2026-11-21 (ADR-0095 — WP-S: bỏ lớp: không nhóm "Bạn cùng lớp", không tab chat Lớp, hồ sơ hiện khối, mục tiêu khối thay mục tiêu lớp) · 2026-11-15
> Liên quan: [STUDENT_GAME_UX](STUDENT_GAME_UX.md) (quy ước chung, màn lõi) · [DESIGN_SYSTEM](DESIGN_SYSTEM.md) · [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) · [ADR-0056 song ngữ](../adr/ADR-0056-game-song-ngu-viet-anh.md) · [ADR-0057 máy tầm trung](../adr/ADR-0057-chay-tren-may-tam-trung-hang-thiet-bi-va-ngan-sach-hieu-nang.md) · [ADR-0047](../adr/ADR-0047-hai-he-thong-giao-dien-game-godot-3d-va-admin-ops-cms-lms.md)

> [!IMPORTANT]
> **Bỏ khái niệm lớp — [ADR-0095](../adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md) (owner chốt 2026-11-21).** Đã viết lại ở WP-S (2026-11-21): học sinh thuộc một trường với một khối; người chơi khác chỉ thấy nickname + avatar (không ngoại lệ "bạn cùng lớp"); Tìm bạn không có nhóm "Bạn cùng lớp"; chat không có tab/kênh lớp, nhóm học tập hay khối; hồ sơ hiện khối; lớp 1–3 thấy mục tiêu tập thể của khối (`grade_goal`) thay thứ hạng. "Lớp 1–3", "lớp 4+"… là cách gọi **khối**.

**Mục lục:** 1. Mục đích và phạm vi · 2. Quy ước áp dụng · 3. Inventory / Shop · 4. Achievement / Collection · 5. Party lobby · 6. Bạn bè & Club · 7. Chat · 8. Competition Arena · 9. Event center · 10. Cài đặt · 11. Khóa dịch và endpoint đề xuất · 12. Acceptance criteria

## 1. Mục đích và phạm vi

Đặc tả UX **có thể triển khai** cho 8 màn học sinh trong game (Godot 4, Theme game — [DESIGN_SYSTEM §5](DESIGN_SYSTEM.md)): Inventory/Shop, Achievement/Collection, Party lobby, Bạn bè & Club, Chat, Competition Arena, Event center, Cài đặt. Mỗi màn có bố cục **Web/PC** và **Mobile** (điện thoại ngang, ghi chú tablet).

Tài liệu **không** định nghĩa lại nghiệp vụ; luật, giới hạn và tham số thuộc tài liệu sở hữu được link ở từng màn (quy tắc "một nguồn sở hữu", [CONFIG_DEFAULTS](../00-overview/CONFIG_DEFAULTS.md)). Các màn lõi (đăng nhập, tạo avatar, HUD, world map, quest journal, challenge/boss, level-up) và màn học tập nằm ở [STUDENT_GAME_UX](STUDENT_GAME_UX.md) và tài liệu học tập tương ứng.

## 2. Quy ước áp dụng

Quy ước chung (nền tảng, khung màn hình, vùng HUD, vùng chạm, phím tắt, trạng thái mẫu) do [STUDENT_GAME_UX](STUDENT_GAME_UX.md) sở hữu. Tóm tắt để đọc tài liệu này:

| Mục | Web/PC | Mobile |
|---|---|---|
| Hướng màn hình | Ngang | Ngang (khóa xoay) |
| Kích thước tham chiếu | ≥ 1280×720, thiết kế ở 1920×1080 | Điện thoại ≥ 6", 2400×1080; tablet 2160×1620 |
| Nhập liệu | Chuột, bàn phím, phím tắt | Cảm ứng; joystick ảo trái, nút hành động phải; tôn trọng safe area |
| Màn phụ (overlay) | Cửa sổ nổi giữa màn, thế giới 3D phía sau làm mờ | Toàn màn, nút Đóng/Quay lại góc trên trái; vuốt từ cạnh trái để quay lại |
| Vùng chạm | — | ≥ 48 dp; lớp 1–3 ≥ 64 dp |
| Game trên trình duyệt di động | — | Không hỗ trợ trong Release 1.0 |

- **Thế giới 3D khi mở màn phụ:** tạm dừng render độ phân giải đầy đủ — giảm render scale và tần số cập nhật thế giới nền (ví dụ 15 FPS) để dành tài nguyên cho UI ([PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md)). Nhân vật vẫn đứng yên an toàn; hub sync vẫn chạy.
- **Song ngữ:** mọi chuỗi qua khóa `ui.<screen>.<element>` (gettext `.po`, ADR-0056); dữ liệu từ backend dùng `i18nText`. Bố cục chịu được chuỗi dài hơn 35%: nút co giãn theo chữ, tối đa 2 dòng rồi rút gọn có tooltip/nhấn giữ để xem đủ.
- **Grade band:** cỡ chữ/nút theo `density/{g1_3,g4_5,g6_7,g8_9}` ([DESIGN_SYSTEM §5](DESIGN_SYSTEM.md)); lớp 1–3 ưu tiên icon kèm chữ ngắn và giọng đọc tùy chọn.
- **Riêng tư:** người chơi khác chỉ thấy nickname, avatar, level, danh hiệu, huy hiệu công khai; không bao giờ thấy tên thật, trường, khối, điểm học tập, mastery ([SOCIAL_SYSTEM §4.1](../03-social-multiplayer/SOCIAL_SYSTEM.md), không có ngoại lệ — ADR-0095).
- **Trạng thái mẫu dùng chung:** Đang tải (skeleton, không spinner toàn màn quá 300 ms), Rỗng (minh họa + lời gợi ý hành động), Lỗi (thông điệp thân thiện + Thử lại), Mất mạng (băng vàng trên cùng "Đang kết nối lại…", thao tác ghi bị khóa, dữ liệu cache vẫn xem được), Khóa (ổ khóa + "Cách mở khóa"), Thành công (toast 2 giây + âm thanh ngắn).

## 3. Inventory / Shop

**Mục đích.** Xem và trang bị vật phẩm trang trí, xem ví Xu, mua vật phẩm trong Shop. Nghiệp vụ: [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md), [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md).

**Người dùng & khối.** Mọi học sinh. Không có giao dịch tiền thật; Xu chỉ kiếm trong game.

**Điểm vào/ra.** HUD trên-phải (biểu tượng ba lô; số Xu) · phím `I` (Tủ đồ), `B` (Shop) · NPC Thợ may Bông ở Thị trấn Trung Tâm (mở thẳng tab Shop) · thông báo "Nhận vật phẩm mới". Ra: Đóng/`Esc`/vuốt lại; quay về vị trí cũ trong thế giới.

**Bố cục Web/PC**

```text
┌──────────────────────────────── Tủ đồ · Wardrobe ───────────────────── 🪙 1.250 Xu  [X] ┐
│ [Tủ đồ] [Shop] [Lịch sử]                                                              │
├────────────────────────────┬──────────────────────────────────────────────────────────┤
│                            │ Lọc: [Tất cả ▼] [Trang phục] [Tóc] [Phụ kiện] [Emote]     │
│   ┌──────────────────┐     │       [Thú cưng] [Danh hiệu] [Khung]   Sắp xếp: [Mới ▼]  │
│   │  Xem trước 3D    │     │ ┌────┐┌────┐┌────┐┌────┐┌────┐┌────┐                    │
│   │  (xoay bằng kéo) │     │ │ ▣  ││ ▣ ✓││ ▣  ││ ▣  ││ ▣  ││ ▣  │  lưới thumbnail    │
│   │                  │     │ └────┘└────┘└────┘└────┘└────┘└────┘  (ảo hóa)          │
│   └──────────────────┘     │ ┌────┐┌────┐┌────┐ …                                     │
│ Slot: [Tóc][Áo][Mũ][Mặt]   ├──────────────────────────────────────────────────────────┤
│       [Lưng][Tay][Thú][Khung]│ Chi tiết: Mũ Nhà Thám Hiểm · Hiếm   Nguồn: Quest "…"     │
│ Emote: [1][2][3][4][5][6]  │ [Trang bị] [Xem trước]                                    │
│ Bộ phối: [A][B][C] [Lưu]   │                                                          │
└────────────────────────────┴──────────────────────────────────────────────────────────┘
```

Tab **Shop** thay cột phải bằng lưới vật phẩm bán (giá Xu, giới hạn mua, "Đã sở hữu", thời gian bán); chọn vật phẩm → **Xem trước** trên nhân vật → **Mua** → hộp xác nhận "Mua Mũ Nhà Thám Hiểm với 120 Xu? Số dư sau mua: 1.130 Xu" → [Hủy] [Mua].

**Bố cục Mobile (điện thoại ngang)**

```text
┌ ←  Tủ đồ            [Tủ đồ|Shop|Lịch sử]           🪙 1.250 ┐
├───────────────┬──────────────────────────────────────────────┤
│  Xem trước 3D │ [Tất cả][Áo][Tóc][Phụ kiện][Emote][Thú] ⇄    │ ← cuộn ngang
│  (1/3 rộng)   │ ┌───┐┌───┐┌───┐┌───┐                         │
│               │ │ ▣ ││ ▣✓││ ▣ ││ ▣ │  4 cột, ô ≥ 72 dp       │
│ [Slot ▾]      │ └───┘└───┘└───┘└───┘                         │
│               │ ┌───┐┌───┐ …                                 │
├───────────────┴──────────────────────────────────────────────┤
│ Mũ Nhà Thám Hiểm · Hiếm                     [Trang bị]        │ ← thanh chi tiết dưới
└──────────────────────────────────────────────────────────────┘
```

Tablet: 6 cột, danh sách slot hiển thị đầy đủ như PC thay cho nút `[Slot ▾]`.

**Thành phần.** `WalletBadge`, `TabBar`, `FilterChips`, `VirtualItemGrid` (thumbnail), `ItemDetailBar`, `AvatarPreview3D` (một viewport duy nhất), `SlotPanel`, `EmoteWheelEditor`, `OutfitPresetBar`, `PurchaseConfirmDialog`, `LedgerList` (tab Lịch sử: nhận/mua/quy đổi trùng lặp).

**Dữ liệu & API.** `listInventory`, `listInventoryHistory`, `getWallet`, `listWalletLedger`, `listShopItems`, `purchaseShopItem` (bắt buộc `Idempotency-Key`), `updateAvatar` (trang bị), `listAvatarPresets`, `saveAvatarPreset`. Ngoại hình mới phát cho người khác qua `hub.player.appearance_changed`.

**Trạng thái.**

| Trạng thái | Hiển thị |
|---|---|
| Đang tải | Skeleton lưới 12 ô; ví hiện số cũ (cache) mờ |
| Rỗng (một bộ lọc) | "Chưa có món nào ở đây. Hoàn thành nhiệm vụ để nhận nhé!" + nút "Xem nhiệm vụ" |
| Không đủ Xu | Nút Mua xám + "Cần thêm 40 Xu"; mã lỗi `INSUFFICIENT_BALANCE` nếu server từ chối |
| Đạt giới hạn mua / đã sở hữu | Nhãn "Đã sở hữu"; `PURCHASE_LIMIT_REACHED`, `ITEM_ALREADY_OWNED` |
| Hết thời gian bán | "Sẽ quay lại mùa sau" (không gây áp lực) — `SHOP_ITEM_UNAVAILABLE` |
| Slot không hợp lệ | `EQUIP_SLOT_INVALID` → toast "Món này không đeo ở vị trí đó" |
| Mất mạng | Xem được tủ đồ cache; Mua/Trang bị khóa |
| Thành công mua | Hiệu ứng mở hộp quà ngắn (không ngẫu nhiên), số Xu cập nhật từ response |

**Tương tác & phản hồi.** Chạm/nhấp ô → chi tiết; nhấp đúp (PC) hoặc nút Trang bị → trang bị ngay trên xem trước. Kéo trên xem trước để xoay. PC: phím số `1`–`6` gán emote khi đang mở EmoteWheelEditor. Mua luôn có bước xác nhận; không có nút "Mua nhanh". Giá và Xu là số nguyên định dạng theo locale (`1.250` vi / `1,250` en).

**Khác biệt grade band.** `g1_3`: ẩn tab Lịch sử, chỉ 4 bộ lọc bằng icon, xác nhận mua có hình đồng Xu trực quan, độ hiếm thể hiện bằng **màu khung + biểu tượng** (không cần đọc chữ; trợ năng vẫn có tên độ hiếm cho giọng đọc). `g6_9`: hiện đủ bộ lọc, sắp xếp, lịch sử sổ cái.

**Song ngữ.** `ui.inventory.title` ("Tủ đồ" / "Wardrobe"), `ui.shop.title` ("Cửa hàng" / "Shop"), `ui.shop.confirm_purchase` ("Mua {item} với {price} Xu? Số dư sau mua: {balance} Xu" / "Buy {item} for {price} Coins? Balance after: {balance} Coins"), `ui.inventory.empty_hint`. Tên/mô tả vật phẩm là `i18nText` từ backend. Chuỗi dài nhất cần kiểm: `ui.shop.confirm_purchase` với tên vật phẩm 40 ký tự.

**Trợ năng.** Độ hiếm có cả màu và biểu tượng/chữ; focus bàn phím đi theo lưới (mũi tên); đọc tên + giá khi focus (lớp 1–3 bật giọng đọc).

**Hiệu năng máy tầm trung.** Chỉ **một** `AvatarPreview3D` (SubViewport ≤ 512×512, cập nhật khi đổi đồ hoặc kéo xoay, không render liên tục). Lưới dùng **thumbnail 2D** render sẵn từ asset 3D ([WORLD_RENDERING](../08-game-client/WORLD_RENDERING.md), ADR-0047), đóng gói **atlas**, tải lười theo trang; `VirtualItemGrid` chỉ tạo Control cho ô hiển thị + 1 hàng đệm. Không tải mô hình 3D của vật phẩm cho đến khi xem trước.

**An toàn/riêng tư.** Không có trading, tặng quà hay xem tủ đồ người khác ngoài phần trang bị đang mặc. Không có quảng cáo hay gợi ý mua bằng tiền thật.

**Lát cắt F069 [Confirmed — F069 Clarifications 2026-11-26].** Một cửa sổ S08 ba tab chip (`g1_3`: Tủ đồ, Cửa hàng; lớp 4–9 thêm Lịch sử), ví "🪙 150 Xu" ở tiêu đề (icon đồng Xu, không emoji). Cột trái: **một** xem trước 3D (render một lần mỗi lần đổi đồ/kéo xoay) + Xoay trái/phải + "Phối đồ" (mở CustomizationStudio F070: ngoại hình, bộ phối, vòng biểu cảm). Cột phải: chip lọc loại (`g1_3` 4 chip icon + chữ: Tất cả, Trang phục, Tóc, Phụ kiện) → lưới ô hình (`PictureTile`: hình loại vật phẩm, dấu độ hiếm màu + hình, tên, dòng phụ giá Xu / "Đã sở hữu", ✓ xanh khi đang mặc) → thanh chi tiết (dấu độ hiếm, tên, chữ độ hiếm lớp 4–9, mô tả, giá, "Đã mua 1/2", "Có bán đến 30/11"; "Trang bị" hoặc "Xem trước" + "Mua"; thiếu Xu: "Mua" xám + "Cần thêm 40 Xu"). Mua → hộp xác nhận (đồng Xu lớn, "Mua Cờ nhỏ với 120 Xu?", "Số dư sau khi mua: 30 Xu", Hủy / Mua); mạng lỗi → hộp ở lại "Mạng đang chập chờn… em sẽ không bị trừ Xu hai lần". Trang bị gọi `updateAvatar` ngay, xem trước chỉ đổi theo phản hồi; mất mạng: xem được, Mua/Trang bị khóa với lời giải thích.

## 4. Achievement / Collection

**Mục đích.** Xem thành tựu, huy hiệu, danh hiệu, bộ sưu tập (pet, thẻ kiến thức, vật khám phá, sự kiện) và cách nhận. Nghiệp vụ: [INVENTORY_ECONOMY §6–§7](../01-game-design/INVENTORY_ECONOMY.md).

**Điểm vào/ra.** Menu chính → "Thành tựu" (không có phím tắt riêng; `J` là Nhật ký nhiệm vụ theo [STUDENT_GAME_UX §4.1](STUDENT_GAME_UX.md#41-webpc--phím-mặc-định-cho-phép-đổi-phím-trong-cài-đặt-confirmed)) · thông báo mở khóa · hồ sơ của mình. Ra: Đóng/`Esc`.

**Bố cục Web/PC**

```text
┌──────────────── Thành tựu & Bộ sưu tập · Achievements & Collections ───────────── [X] ┐
│ [Thành tựu] [Bộ sưu tập] [Huy hiệu trưng bày]                                         │
├─────────────────────┬────────────────────────────────────────────────────────────────┤
│ Nhóm                │ ★ Nhà thám hiểm Vương quốc Số          ███████░░  7/10 khu vực  │
│ ▸ Lần đầu           │   Thưởng: Huy hiệu + 50 Xu                       [Chi tiết]     │
│ ▸ Học tập           │ ★ Bạn đồng hành                        ██████████ Hoàn thành ✓  │
│ ▸ Khám phá          │ ? Thành tựu ẩn                          "Tiếp tục khám phá…"    │
│ ▸ Xã hội · Đội      │ …                                                                │
│ ▸ Thi đấu · Mùa     │                                                                  │
└─────────────────────┴────────────────────────────────────────────────────────────────┘
```

Tab **Bộ sưu tập**: chọn bộ (ví dụ "Thú cưng các vùng") → lưới món (đã có: thumbnail màu; chưa có: bóng xám + "Cách nhận: Hoàn thành chuỗi quest Rừng Ngôn Ngữ"), thanh tiến độ bộ và phần thưởng hoàn thành. Tab **Huy hiệu trưng bày**: 3 ô kéo thả huy hiệu (slot `badge_showcase_1..3`).

**Bố cục Mobile**

```text
┌ ←  Thành tựu      [Thành tựu|Bộ sưu tập|Trưng bày]          ┐
│ [Lần đầu][Học tập][Khám phá][Xã hội][Thi đấu][Mùa] ⇄          │
│ ★ Nhà thám hiểm Vương quốc Số   ███████░░ 7/10      ›         │
│ ★ Bạn đồng hành                 ██████████ ✓        ›         │
│ ? Thành tựu ẩn                                      ›         │
└──────────────────────────────────────────────────────────────┘
```

Chạm một dòng mở bảng chi tiết trượt từ dưới lên (điều kiện, tiến độ, phần thưởng, ngày đạt). Tablet: hai cột như PC.

**Thành phần.** `AchievementList` (ảo hóa), `ProgressBar`, `RewardChips`, `CollectionGrid`, `HowToObtainCard`, `BadgeShowcaseEditor`, `TierBadge` (đồng/bạc/vàng nếu có `tiers[]`).

**Dữ liệu & API.** `listAchievements`, `listCollections`, `getCollection`, `updateAvatar` (huy hiệu trưng bày là slot). Mở khóa realtime: `notification.item.created`; tiến độ cập nhật khi mở màn (pull), không cần push liên tục.

**Trạng thái.** Đang tải (skeleton danh sách) · Rỗng ("Hành trình mới bắt đầu! Thành tựu đầu tiên đang chờ em ở Thị trấn Trung Tâm") · Ẩn (`hidden_until_unlocked`: chỉ hiện "?" và gợi ý mơ hồ) · Hoàn thành (dấu ✓, ngày đạt) · Mất mạng (dữ liệu cache, ghi chú "Cập nhật lần cuối …").

**Tương tác.** Không có nút "nhận thưởng" thủ công — phần thưởng tự vào ví/tủ đồ (server-authoritative), màn chỉ báo "Đã nhận". Món trùng lặp hiển thị "Đã quy đổi thành 25 Xu".

**Khác biệt grade band.** `g1_3`: chỉ tab Bộ sưu tập dạng sổ dán hình lớn và tab Thành tựu rút gọn (icon + thanh tiến độ, không chữ dài). `g6_9`: đủ nhóm, hiển thị tier.

**Song ngữ.** `ui.achievement.title`, `ui.achievement.hidden_hint` ("Tiếp tục khám phá để mở khóa" / "Keep exploring to unlock"), `ui.collection.how_to_obtain` ("Cách nhận" / "How to get"), `ui.collection.duplicate_converted` ("Đã quy đổi thành {amount} Xu" / "Converted to {amount} Coins"). Tên/điều kiện từ backend là `i18nText`.

**Trợ năng.** Tiến độ đọc thành "7 trên 10"; món chưa có có chữ "Chưa có" (không chỉ dựa vào màu xám).

**Hiệu năng.** Thumbnail atlas, không mô hình 3D; danh sách ảo hóa; ảnh pet chưa sở hữu dùng cùng texture với hiệu ứng shader xám (không tạo texture riêng).

**An toàn/riêng tư.** Thành tựu học tập dựa trên mastery/hoàn thành, không hiển thị điểm số; người khác chỉ thấy huy hiệu trưng bày và danh hiệu.

**Lát cắt F069 [Confirmed — F069 Clarifications 2026-11-26].** Mở từ HUD "Thêm" → "Thành tựu & Bộ sưu tập". Tab Thành tựu: dòng thẻ (icon huy chương/sao, tên, mô tả lớp 4–9, thanh "7/10", bậc lớp 4–9, ✓ "Đã nhận" + chip phần thưởng); thành tựu ẩn chỉ "?" + "Tiếp tục khám phá để mở khóa". Tab Bộ sưu tập: danh sách bộ "Thú cưng các vùng · 3/10 món" → trang bộ: thanh tiến độ, "Thưởng khi đủ bộ", lưới ô (`g1_3` ô lớn kiểu sổ dán): đã có = hình màu + ✓ "Đã có"; chưa có = cùng hình xám + khóa "Chưa có" + "Cách nhận: …". Tab Huy hiệu trưng bày (lớp 4–9): 3 ô + huy hiệu đang sở hữu, chạm để thêm/bỏ, "Lưu trưng bày".

## 5. Party lobby

**Mục đích.** Tập hợp nhóm chơi, chọn hoạt động, sẵn sàng và bắt đầu phiên chơi chung. Nghiệp vụ: [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) (sở hữu mọi luật, giới hạn, grace).

**Điểm vào/ra.** Nút Party trên HUD (phím `P`) · "Mời vào party" từ Bạn bè/Club · chấp nhận lời mời (popup nhỏ góc phải, tự ẩn sau 10 giây, mở lại từ chuông). Ra: nút đỏ "✕ Đóng" ở cuối thanh tiêu đề như mọi cửa sổ (F125, [DESIGN_SYSTEM §5.1](DESIGN_SYSTEM.md#51-token-ui); thay nút phụ "Thu nhỏ": đóng cửa sổ, party vẫn tồn tại, HUD hiện khung thành viên) hoặc Rời party (xác nhận).

**Bố cục Web/PC**

```text
┌────────────── Nhóm: "Nhóm Sao Băng" · Quest Party ─────────── [Rời] [✕ Đóng] ┐
├───────────────────────────────────────────┬──────────────────────────────────────┤
│ 👑 MèoXanh42 (em)  Lv 12  ● Online  ✔     │ Chat nhóm                     [‒]    │
│    CáHeoVui07      Lv 10  ● Online  ✔     │ CáHeoVui07: Sẵn sàng rồi!            │
│    GấuNâu15        Lv 11  ● Online  …     │ GấuNâu15: chờ tớ chút                │
│    [+ Mời bạn]                             │                                      │
├───────────────────────────────────────────┤ [Chúc may mắn!][👍][Đi thôi!]       │
│ Hoạt động: [Boss nhóm ▼] Rồng Phân Số      │ [Nhập tin nhắn…              ] [Gửi] │
│ Độ khó: [Thường ▼]  Cần: 2–4 người        │                                      │
│ ⚠ GấuNâu15 chưa đủ điều kiện: cần lớp 4+  │                                      │
│ [Sẵn sàng ✔]               [Bắt đầu ▶]     │                                      │
└───────────────────────────────────────────┴──────────────────────────────────────┘
```

**Bố cục Mobile**

```text
┌ ←  Nhóm Sao Băng                    [💬 2]   [Rời] ┐
│ 👑 MèoXanh42 ✔   CáHeoVui07 ✔   GấuNâu15 …   [+]   │ ← thẻ thành viên ngang
│ ─────────────────────────────────────────────────── │
│ Hoạt động: Boss nhóm · Rồng Phân Số        [Đổi ›] │
│ Độ khó: Thường                                       │
│                                                      │
│ [   Sẵn sàng ✔   ]              [   Bắt đầu ▶   ]   │ ← nút lớn dưới
└─────────────────────────────────────────────────────┘
```

Chat nhóm mở bằng nút `💬` (trượt từ phải, chiếm 45% chiều rộng, từ đỉnh thanh tiêu đề xuống tới thanh Sẵn sàng/Bắt đầu — vẫn thấy nút Sẵn sàng; quy tắc chỗ cho tin ở §7). Tablet: bố cục hai cột như PC.

**Thành phần.** `PartyMemberCard` (avatar 2D, level, presence, ready, vương miện leader, menu leader: Chuyển trưởng nhóm/Mời rời nhóm), `InviteFriendPicker`, `ActivitySelector` (chỉ liệt kê hoạt động mọi thành viên đủ điều kiện; mục chỉ một số đủ điều kiện hiện cảnh báo), `DifficultySelector`, `ReadyToggle`, `StartButton`, `EmbeddedChat` (channel `party`, always-on).

**Dữ liệu & API.** REST: `createParty`, `getMyParty`, `getParty`, `updatePartySettings`, `inviteToParty`, `acceptPartyInvitation`, `rejectPartyInvitation`, `cancelPartyInvitation`, `joinParty`, `kickPartyMember`, `transferPartyLeader`, `leaveParty`, `disbandParty`, `setPartyReady`, `selectPartyActivity`, `startPartySession`. WS: `party.state.snapshot`, `party.member.joined`, `party.member.left`, `party.member.ready_changed`, `party.member.status_changed`, `party.activity.selected`, `party.leader.transferred`, `party.session.started`, `party.party.disbanded`, `party.invitation.received`.

**Trạng thái.**

| Trạng thái | Hiển thị |
|---|---|
| Đang tải | Thẻ thành viên skeleton; nút Bắt đầu ẩn |
| Một mình | "Mời bạn bè để cùng chơi" + nút Mời lớn |
| Chưa đủ điều kiện bắt đầu | Nút Bắt đầu xám + lý do ("Còn 1 bạn chưa sẵn sàng") — không phải mã lỗi |
| Trưởng nhóm mất kết nối | Băng "Trưởng nhóm đang kết nối lại…"; nút leader khóa trong grace ([PARTY_SYSTEM §9](../03-social-multiplayer/PARTY_SYSTEM.md)) |
| Đầy nhóm | "Nhóm đã đủ người" — `PARTY_FULL` |
| Lời mời hết hạn | `PARTY_INVITATION_EXPIRED` → "Lời mời đã hết hạn" |
| Mất mạng | Băng vàng; ready của mình reset theo luật party |
| Bắt đầu thành công | Đếm ngược 3‑2‑1 rồi chuyển cảnh vào instance |

**Tương tác.** Leader nhấp chuột phải/nhấn giữ thẻ thành viên để mở menu leader. Đổi hoạt động reset ready của mọi người (hiện thông báo nhỏ). Nút Bắt đầu chỉ hoạt động khi đủ điều kiện, do server kiểm tra lại.

**Khác biệt grade band.** `g1_3`: chọn hoạt động bằng thẻ hình lớn (3–4 lựa chọn), không có ô độ khó (dùng mặc định), mời bạn bằng chạm avatar. `g6_9`: dropdown đầy đủ mode/độ khó.

**Song ngữ.** `ui.party.title` ("Nhóm: {name}" / "Party: {name}"), `ui.party.ready` ("Sẵn sàng" / "Ready"), `ui.party.start` ("Bắt đầu" / "Start"), `ui.party.not_eligible` ("{nickname} chưa đủ điều kiện: {reason}" / "{nickname} isn't eligible yet: {reason}") — chuỗi dài nhất. Tên hoạt động là `i18nText`.

**Trợ năng.** Trạng thái sẵn sàng có biểu tượng + chữ; thông báo thành viên vào/ra được đọc khi bật giọng đọc.

**Hiệu năng.** Avatar thành viên là ảnh 2D render sẵn (thumbnail theo cấu hình avatar, cache), **không** render 4 nhân vật 3D trong lobby. Snapshot WS áp vào state store, chỉ vẽ lại thẻ thay đổi.

**An toàn/riêng tư.** Mời chỉ từ danh sách được phép (bạn bè, club, người vừa chơi cùng — trong phạm vi ADR-0054 và điều kiện loại party/khối của [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md)). Mỗi tin chat nhóm có Tắt tiếng/Chặn/Báo cáo. Kết quả học tập trong hoạt động chung luôn riêng từng người (không hiển thị điểm của bạn khác).

**Lát cắt client F102 [Confirmed — F102 spec, 2026-11-27].** Màn Nhóm là cửa sổ tầng 1 như Cài đặt (cửa sổ gần kín màn — lề 1,5% hai bên, 2% trên dưới; thanh tiêu đề xanh "NHÓM: {tên}" — các nút giữ một dòng, tên dài xuống dòng trước (biểu tượng nhóm nhường chỗ khi tên sẽ phải xuống dòng), các nút chỉ trôi xuống dòng thứ hai khi vượt 70% thanh; nhãn loại nhóm cạnh "THÀNH VIÊN (3/4)"; nội dung cuộn khi không vừa, thanh Sẵn sàng/Bắt đầu luôn ở đáy; nút đỏ chỉ cho Rời nhóm/Đóng/Giải tán/Mời rời nhóm — F114). *Một mình:* Mây "Mời bạn bè để cùng chơi nhé!" + một nút lớn "Tạo nhóm và mời bạn" (tạo party `friend` tên "Nhóm của {nickname}", mở ngay bảng mời — HUD "Nhóm" → Tạo → chạm bạn = 3 thao tác, SC-001) và mục "Lời mời cho em" (chân dung, "{nickname} mời em vào nhóm!", "Lời mời còn m:ss" theo `expires_at` của server, "Lần sau nhé" / "Vào nhóm"). *Trong nhóm:* PC hai cột (thẻ thành viên + "Mời bạn" | hoạt động: thẻ hình + tên, "Chọn/Đổi hoạt động" cho trưởng nhóm, cảnh báo chưa đủ điều kiện); khi có chat nhóm (F119) thì cột trái là thành viên + hoạt động + thanh Sẵn sàng/Bắt đầu (lý do của nút Bắt đầu nằm dòng riêng phía trên các nút; bản dịch quá dài thì "Bắt đầu" xuống dòng), cột phải là chat cao hết cửa sổ — thẻ thành viên luôn thấy ít nhất một thẻ, nội dung cột trái cuộn khi không vừa; điện thoại ngang (tỉ lệ > 1,9) một hàng thẻ, "Đổi hoạt động" nằm trong thẻ hoạt động, nút lớn ở đáy. Thẻ thành viên: chân dung 2D (`AvatarThumb`, không 3D), nickname ("(em)" cho chính mình), vương miện + "TRƯỞNG NHÓM", trạng thái icon + chữ ("Đang trực tuyến", "Đang kết nối lại…", "Không trực tuyến"), nhãn "Sẵn sàng"/"Chưa sẵn sàng"; bạn mất kết nối = thẻ xám `PartyMemberCardAway` + biểu tượng wifi gạch. Thanh đáy: "Em sẵn sàng!" (nút chính) → "Em đã sẵn sàng" (chip vàng + ✓, bấm lại để hủy); "Bắt đầu" chỉ trưởng nhóm, xám kèm lý do bằng lời ("Còn 1 bạn chưa sẵn sàng", "Hãy chọn một hoạt động trước nhé"); thành viên thấy "Trưởng nhóm sẽ bấm Bắt đầu khi cả nhóm sẵn sàng". Menu thẻ (nút icon menu trên thẻ bạn khác, nhấn giữ/chuột phải) → "Chuyển trưởng nhóm" / "Mời rời nhóm"; mọi rời/giải tán/mời rời/chuyển/đổi nhóm qua hộp xác nhận vàng (focus "Hủy"). Cài đặt nhóm (bánh răng, trưởng nhóm): lớp 6–9 thêm chip "Ai được vào nhóm" (Chỉ bạn được mời / Bạn bè của trưởng nhóm) và "Ai được mời bạn" (Chỉ trưởng nhóm / Mọi thành viên); mọi khối có "Giải tán nhóm". Bảng mời: lưới ô hình bạn bè (`PictureTile` chân dung + nickname + trạng thái), một chạm là mời, không ô gõ tên, không nhóm theo lớp/khối/trường; đã mời = ô vàng + ✓ "Đã mời". Chọn hoạt động: `g1_3`/`g4_5` tối đa 4 thẻ hình lớn (hoạt động đang chơi = thẻ vàng), `g6_7`/`g8_9` danh sách thả xuống + "Chọn hoạt động này"; hoạt động chưa đủ điều kiện có ổ khóa và dòng "{nickname} chưa đủ điều kiện: hoạt động dành cho khối 4" (yêu cầu của hoạt động, không phải khối của bạn). Băng vàng `WarningBannerPanel`: "Đang kết nối lại… Em vẫn xem được nhóm" (thao tác ghi khóa) và "Trưởng nhóm đang kết nối lại…". `party.session.started` → thẻ đếm ngược 3‑2‑1 (`CountdownLabel`) rồi vào phòng phiên qua `SceneRouter`; nếu đang mở màn khác thì HUD báo "Nhóm của em đã bắt đầu!" (không đóng màn học tập). Hết phiên → mở lại Nhóm với tóm tắt chung ("Cả nhóm đã hoàn thành hoạt động! Làm tốt lắm."), không có điểm của bạn khác. *HUD:* nút "Nhóm" (phím `P`) ở cột phải; trong nhóm thêm dải chân dung nhỏ (vương miện, huy hiệu ✓ vàng khi sẵn sàng, khung xám + wifi gạch khi mất kết nối) và băng trưởng nhóm đang kết nối lại. *Lời mời:* popup góc phải (`InvitationToast`: chân dung, "{nickname} mời em vào nhóm!", "Nhóm có 3 bạn", "Lời mời còn 4:59", "Lần sau nhé" / "Vào nhóm"), tự ẩn sau **10 giây** (giá trị sở hữu ở đây), nằm giữa cột trái HUD và cột phải (thanh bên, nút "Biểu cảm") — không bao giờ nằm dưới các nút đó (F125 polish), không bật khi đang làm bài kiểm tra/thử thách/màn học tập hoặc khi tắt popup trong Cài đặt — khi đó chỉ số trên chuông; thông báo `party_invitation` trong hộp thông báo có "Xem" mở màn Nhóm ở mục lời mời (lời mời đã hết hạn → "Lời mời đã hết hạn"). Đếm ngược bắt đầu **3 giây** (giá trị sở hữu ở đây).

## 6. Bạn bè & Club

**Mục đích.** Quản lý bạn bè, lời mời, mã bạn bè; tham gia bạn, mời party, thách đấu; xem và tham gia club. Nghiệp vụ: [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md).

**Điểm vào/ra.** HUD (biểu tượng hai người, phím `F`) · NPC Chú Ong Kết Nối (Vườn Bạn Bè) · thẻ người chơi khi nhấp vào nhân vật gần đó. Ra: Đóng.

**Bố cục Web/PC**

```text
┌────────────────────────────── Bạn bè · Friends ────────────────────────────── [X] ┐
│ [Bạn bè 12] [Lời mời 2] [Tìm bạn] [Club] [Đã chặn]                                │
├──────────────────────────────────────┬────────────────────────────────────────────┤
│ ● Online (4)                          │ CáHeoVui07  Lv 10  🏷 Nhà thám hiểm         │
│   CáHeoVui07   Vương quốc Số  [›]     │ Đang ở: Vương quốc Số                       │
│   GấuNâu15     Trong trận     [›]     │ Huy hiệu: ★ ★ ★                             │
│ ○ Offline (8)                         │ [Tham gia bạn] [Mời party] [Thách đấu]     │
│   ThỏTrắng03   2 giờ trước    [›]     │ [Nhắn tin]                                  │
│                                       │ [⋯ Hủy kết bạn · Tắt tiếng · Chặn · Báo cáo]│
└──────────────────────────────────────┴────────────────────────────────────────────┘
```

Tab **Tìm bạn** không có ô tìm tự do: hai nhóm "Người chơi gần đây", "Vừa chơi cùng" (không có nhóm "Bạn cùng lớp" — ADR-0095) + mục **Mã bạn bè** ([Tạo mã] hiển thị mã 6 ký tự đếm ngược; [Nhập mã]). Tab **Club**: club của em (tối đa theo SOCIAL_SYSTEM), club đề xuất trong trường, [Tạo club] (cần nhà trường duyệt); trang club gồm thành viên + vai trò, mục tiêu tuần (thanh tiến độ tập thể), sự kiện club, nút chat club.

**Bố cục Mobile**

```text
┌ ←  Bạn bè   [Bạn bè|Lời mời 2|Tìm bạn|Club|Đã chặn] ⇄     ┐
│ ● CáHeoVui07   Vương quốc Số               [Mời] [›]      │
│ ● GấuNâu15     Trong trận                        [›]      │
│ ○ ThỏTrắng03   2 giờ trước                       [›]      │
└───────────────────────────────────────────────────────────┘
```

Chạm `›` mở trang hồ sơ bạn toàn màn với các nút hành động lớn. Tablet: hai cột như PC.

**Thành phần.** `FriendList` (ảo hóa, nhóm theo presence), `FriendProfilePanel`, `RequestList` (chấp nhận/từ chối), `SuggestionGroups`, `FriendCodeWidget`, `ClubList`, `ClubPage` (`ClubGoalProgress`, `ClubEventList`, `ClubMemberList`), `BlockList`.

**Dữ liệu & API.** `listFriends`, `listFriendRequests`, `sendFriendRequest`, `acceptFriendRequest`, `rejectFriendRequest`, `removeFriend`, `createFriendCode`, `redeemFriendCode`, `blockUser`, `unblockUser`, `listBlocks`, `muteUser`, `unmuteUser`, `listMutes`, `createReport`, `inviteToParty`, `createDuel`, club: `listClubs`, `createClub`, `getClub`, `leaveClub`, `listClubMembers`, `requestJoinClub`, `acceptClubInvitation`, `declineClubInvitation`, `listClubGoals`, `listClubEvents`. Presence realtime: `presence.friend.updated`. "Tham gia bạn": lệnh WS `hub.instance.join` có `follow_user_id` (module competition gán instance của bạn nếu còn chỗ và cùng partition — [MULTIPLAYER_GAMEPLAY §6](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md#6-gán-instance)); không có REST riêng.

**Trạng thái.**

| Trạng thái | Hiển thị |
|---|---|
| Rỗng | "Em chưa có bạn nào. Hãy chào một bạn đang ở gần nhé!" + nhóm gợi ý |
| Lời mời đã gửi | "Đang chờ" (không bao giờ hiện "bị từ chối" — từ chối lặng lẽ) |
| Ngoài phạm vi | `SOCIAL_SCOPE_VIOLATION` → "Em chỉ kết bạn được với bạn cùng trường" |
| Đạt giới hạn | `FRIEND_LIMIT_REACHED`; mã sai/hết hạn `FRIEND_CODE_INVALID` |
| Không tham gia được | Bạn đang trong trận/bài kiểm tra → nút "Tham gia bạn" xám + lý do |
| Club chờ duyệt | `CLUB_NOT_APPROVED` → "Club đang chờ nhà trường duyệt"; đầy `CLUB_FULL`; đạt giới hạn `CLUB_LIMIT_REACHED` |
| Mất mạng | Danh sách cache, presence hiện "—" |

**Tương tác.** Chặn và Báo cáo luôn có xác nhận một bước, có lựa chọn lý do (icon cho lớp 1–3). Sau khi chặn: bạn đó biến mất khỏi danh sách, không thấy nhau trong chat/party/ghép trận.

**Khác biệt grade band.** Quyền xã hội như nhau mọi khối (ADR-0020); khác biệt chỉ ở trình bày. `g1_3`: tab Club rút gọn (thẻ hình lớn của club đã tham gia, xin vào/nhận lời mời bằng nút lớn; nút "Tạo club" chỉ hiện khi policy trường và điều kiện tạo club cho phép — [SOCIAL_SYSTEM §6.1](../03-social-multiplayer/SOCIAL_SYSTEM.md#61-loại-club); bị từ chối thì cùng mã `403 PERMISSION_DENIED` như mọi khối), Mã bạn bè hiển thị to kèm hình, nút "Tham gia bạn" và "Mời party" là icon lớn. `g6_9`: đủ tab.

**Song ngữ.** `ui.friends.title`, `ui.friends.join_friend` ("Tham gia bạn" / "Join friend"), `ui.friends.request_pending` ("Đang chờ" / "Pending"), `ui.friends.scope_violation`, `ui.friendcode.instructions` ("Đọc mã này cho bạn ngồi cạnh, mã hết hạn sau {minutes} phút" / "Read this code to the friend next to you; it expires in {minutes} minutes") — chuỗi dài nhất. Tên vùng hiển thị theo ngôn ngữ đã chọn.

**Trợ năng.** Presence có chữ ("Đang trực tuyến") ngoài chấm màu; mã bạn bè có nút đọc to từng ký tự.

**Hiệu năng.** Danh sách ảo hóa; ảnh đại diện 2D 64×64 cache; presence cập nhật theo lô (gộp sự kiện tối đa 1 lần/giây cho danh sách).

**An toàn/riêng tư.** Không tìm kiếm theo tên/ID; hồ sơ bạn không hiện điểm học tập, mastery, kết quả kiểm tra; không hiện tên thật, trường, khối với bất kỳ ai (R16, ADR-0095). Báo cáo gắn ngữ cảnh (người, tin, trận).

**Bạn bè trong client (F110, [Confirmed — F110 Clarifications 2026-11-28]).** Cửa sổ "Bạn bè" (phím `F`, ô "Bạn bè" trong menu "Thêm", NPC `open_ui` `friends`, thông báo lời mời kết bạn → tab Lời mời) có 4 tab: Bạn bè (nút "Ẩn trạng thái" + trạng thái của em; danh sách ảo hóa nhóm "Đang trực tuyến (n)"/"Không trực tuyến (n)", mỗi dòng: chân dung, nickname, "Cấp n", biểu tượng + chữ trạng thái, chip "Xem"; hồ sơ bên phải: chân dung, nickname, cấp, danh hiệu nếu có, trạng thái, nút "Mời vào nhóm", "Tắt tiếng", "Chặn", "Báo cáo", "Hủy kết bạn"), Lời mời (Đồng ý/Từ chối; lời mời em gửi luôn "Đang chờ"), Tìm bạn ("Ở gần đây" với "Kết bạn" + Mã bạn bè: "Tạo mã" hiện từng ký tự lớn, đếm ngược, "Đọc to"; ô nhập mã tự viết hoa), Đã chặn (Bỏ chặn/Bỏ tắt tiếng). Chặn/Tắt tiếng: một hộp xác nhận vàng có 3 chip lý do bằng hình — chạm chip là xác nhận (≤ 2 chạm). Chưa có tab Club, "Vừa chơi cùng", "Tham gia bạn" (F111/F112). Thẻ người trong Lời mời/Tìm bạn/Đã chặn là một dòng: chân dung, nickname, nút/trạng thái ở đầu phải (như chip "Xem"). Điện thoại (compact F125): hồ sơ bạn chiếm nửa trang, chân dung nhỏ, nickname + "Cấp n" chung một dòng, nên "Mời vào nhóm", "Tắt tiếng", "Chặn", "Báo cáo" thấy ngay, "Hủy kết bạn" cuộn tới [Confirmed — F110 Clarifications 2026-11-28 (polish)]. Bước `interdependent` ("Cùng nhóm giải đố") trên điện thoại có hai tab "Vai của em" (bước + cả nhóm bên trái, thẻ vai của em trọn vẹn bên phải) và "Bảng chung của nhóm", đặt ở hàng nút cạnh "Trả lời câu cuối"; màn lớn hiện tất cả cùng lúc.

**Tham gia bạn, "Vừa chơi cùng", tab Club trong client (F111, [Confirmed — F111 Clarifications 2026-11-28]).** Hồ sơ bạn có nút "Tham gia bạn" đầu hàng: xám kèm lý do bằng chữ khi bạn offline/ẩn trạng thái, đang trong trận, đang làm bài kiểm tra, đang học, chưa ở khu nào, hoặc chính em đang mất mạng/đang làm dở hoạt động có giờ; bấm → cùng khu: `hub.instance.join` có `follow_user_id`; khác khu: vào khu trước (`enterWorldArea`) rồi join có `follow_user_id`; đến nơi → thông báo "Em đã đến chỗ {nickname}"; đầy → vào instance thường + "Khu vực của bạn đã đầy"; từ chối/không thấy → "Chưa thể tham gia bạn lúc này" (không nêu khu, phòng, lý do). Tab Tìm bạn có đúng ba nhóm: "Ở gần đây", "Vừa chơi cùng" (`listFriendSuggestions` `recently_played`, ngữ cảnh party/trận/club/co-op bằng biểu tượng + chữ; lớp 1–3 là ô chân dung lớn, chạm để kết bạn; rỗng → gợi ý Mã bạn bè; lỗi → ẩn nhóm) và Mã bạn bè. Tab **Club**: "Tạo club" (chỉ khi `policies.social` cho phép và học sinh đủ điều kiện; trường từ chối → "Trường em chưa mở tạo club"), lời mời vào club (Đồng ý/Từ chối), "Club của em", "Club đề xuất" (không có ở lớp 1–3); thẻ club có tên, biểu tượng, loại, số thành viên, trạng thái bằng chữ; trang club: Chat club (ChatDock, channel `team` của club), Mời bạn (chủ/phó, chỉ chọn từ danh sách bạn, ≤ 50, "Chưa mời được" không nêu lý do), Xin vào, Rời club (một bước xác nhận), mục Thành viên (nickname, chân dung, vai trò; 20 người một lần, "Xem thêm thành viên"), Mục tiêu (thanh tập thể + "Em đã góp {n}"), Lịch, Yêu cầu (chủ/phó: Chấp nhận/Từ chối); club khóa/lưu trữ chỉ xem. Form tạo club: loại Nhóm sở thích/Đội thi đấu, tên 2–64 ký tự, giới thiệu ≤ 500 (bộ đếm), biểu tượng thư viện; bị kiểm duyệt từ chối → câu chung, không nhắc lại từ, giữ form. Thông báo club (`open_club`) mở thẳng trang club. Điện thoại (`compact`, F125 — polish 2026-11-28): cửa sổ Bạn bè, form tạo club và bảng mời cao gần hết màn (lề 2% trên dưới, 3% hai bên); trang club có **đầu trang một dòng** — "Quay lại", "Chat club", "Mời bạn", "Rời club" thành nút chỉ có biểu tượng (chữ thành tooltip + tên trợ năng), tên club chung dòng với nhãn "{n}/{max} thành viên" và vai trò, ẩn dòng loại club — và hàng tab Bạn bè · Lời mời · … ẩn khi đang ở trang club ("Quay lại" trả lại danh sách và hàng tab); thành viên xếp hai thẻ một hàng; nên mục đang chọn thấy ngay ít nhất 3 thành viên / thẻ mục tiêu / yêu cầu đầu tiên không cần cuộn. Tìm bạn: "Chưa có bạn nào ở gần em." và "Chạm vào bạn để kết bạn" nằm cùng dòng tiêu đề nhóm, ô "Vừa chơi cùng" lớp 1–3 thành hàng ngang (chân dung bên trái, nickname + "Cấp n" một dòng, ngữ cảnh dưới) nên thấy trọn ngay khi mở tab. Kiểm: `F111Matrix.content_cuts` trong ma trận khối (vùng nội dung chính thấy ≥ 3 vùng chạm `compact` hoặc trọn vẹn).

**Công cụ chủ club trong game (F112, [spec](../../specs/112-godot-persistent-chat-club-tools/spec.md) Ready) [Confirmed — F112 Clarifications 2026-11-28].** Chỉ học sinh `owner` của club `active` thấy nút "Quản lý club" trên trang club: Sửa thông tin (tên, mô tả, biểu tượng — qua kiểm duyệt, khóa lạc quan theo `version`), Phó nhóm (chọn/thôi `officer`, xác nhận một bước), Mục tiêu tuần (chỉ số bằng hình + chữ, mục tiêu −/+), Sự kiện club (luyện tập chung, quest co-op, boss nhóm, khác; ngày + giờ, không đếm ngược). `officer` chỉ duyệt yêu cầu và mời ([SOCIAL_SYSTEM §6.2](../03-social-multiplayer/SOCIAL_SYSTEM.md#62-thành-viên--vai-trò)); học sinh không gỡ thành viên, không giải tán/lưu trữ club trong game — việc đó của nhân sự trường ở portal. Từ chối tên/mô tả → thông điệp kiểm duyệt chung, giữ form.

## 7. Chat

**Mục đích.** Giao tiếp an toàn trong các kênh: World, Area, Match, Party, Team, Club (team cố định), Direct — không có kênh lớp, nhóm học tập hay khối (ADR-0095). Nghiệp vụ và mọi giới hạn: [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) (§3.1 bảng kênh, §4.2–§4.3 trải nghiệm, §7 tính năng); kiểm duyệt: [MODERATION](../03-social-multiplayer/MODERATION.md).

**Điểm vào/ra.** PC: khung chat dưới-trái của HUD (thu gọn/mở rộng bằng `Enter` hoặc `T`); mobile: nút chat dưới-trái có badge. Trong party lobby, trận, boss: panel nhúng luôn có (always-on, thu gọn được).

**Bố cục Web/PC (khung nổi, có thể kéo giãn trong giới hạn)**

```text
┌ Chat ───────────────────────────────────────────── [🔍] [‒] ┐
│ [Khu vực] [Nhóm 2] [Club] [@1 Bạn bè ▾]                      │ ← tab kênh, badge chưa đọc
│ 📌 CáHeoVui07: Hẹn 19h vào boss Rừng Phiêu Lưu nhé!           │ ← tin ghim của thành viên (≤ 3)
│ CáHeoVui07: Ai đi boss không?                    10:21       │
│   ↳ trả lời MèoXanh42: "Đi thôi!"                             │
│ MèoXanh42 (em): Tớ đi!                     ✓✓ 10:22 (đã xem) │
│ GấuNâu15 đang soạn tin…                                       │
│ [Chúc may mắn!] [Hay quá!] [Đợi tớ!] [👍]                     │ ← quick message theo ngữ cảnh
│ [😊] [Nhập tin nhắn… (tối đa 200)                ] [Gửi ↵]   │
└──────────────────────────────────────────────────────────────┘
```

Nhấp chuột phải hoặc nút `⋯` trên tin mở menu thao tác (Trả lời, Trích dẫn, Chuyển tiếp, Sao chép, Ghim, Sửa, Thu hồi, Thả cảm xúc, Tắt tiếng, Chặn, Báo cáo — hiển thị theo quyền, [CHAT_SYSTEM §4.3](../03-social-multiplayer/CHAT_SYSTEM.md)).

**Bố cục Mobile (toàn màn trượt từ trái)**

```text
┌ ←  Chat · Nhóm Sao Băng                        [🔍] [⋯] ┐
│ [Khu vực][Nhóm 2][Club][Bạn bè] ⇄                        │
│ CáHeoVui07: Ai đi boss không?                            │
│ MèoXanh42 (em): Tớ đi!                          ✓✓       │
│ ─────────────────────────────────────────────────────── │
│ [Chúc may mắn!][Hay quá!][Đợi tớ!][👍] ⇄                 │
│ [😊][ Nhập tin nhắn…                       ][ ➤ ]        │ ← đẩy lên khi bàn phím mở
└──────────────────────────────────────────────────────────┘
```

Nhấn giữ tin để mở menu thao tác dạng bảng dưới. Trong **trận/boss trên mobile**: chat là dải mỏng 2 dòng dưới cùng bên trái + hàng quick message; chạm để mở rộng nửa màn; tự thu nhỏ khi câu hỏi hiển thị. Tablet: khung nổi như PC, chiếm ≤ 40% chiều rộng.

**Thành phần.** `ChannelTabs` (badge chưa đọc, badge `@`), `PinnedBar`, `MessageList` (ảo hóa, nhóm theo người gửi), `MessageBubble` (trạng thái ✓/✓✓, nhãn "đã chỉnh sửa", "Đã chuyển tiếp từ…", tin đã thu hồi), `ReplyQuoteBar`, `TypingIndicator`, `QuickMessageBar`, `Composer` (đếm ký tự theo `max_text_length` của kênh), `EmojiStickerGifPicker` (3 tab; GIF chỉ thư viện nội bộ theo chủ đề), `MentionPicker`, `MessageActionMenu`, `ReportDialog`, `SearchPanel`.

**Dữ liệu & API.** REST: `listMyChannels`, `listChannelMessages` (`before_seq`/`after_seq`), `searchMessages`, `listChannelPins`, `getMessageReceipts`, `listStickerPacks`, `listGifs`, `createReport`. WS lệnh: `chat.message.send`, `chat.message.edit`, `chat.message.recall`, `chat.message.forward`, `chat.reaction.toggle`, `chat.pin.add`/`remove`, `chat.typing.start`/`stop`, `chat.receipt.delivered`/`read`, `chat.history.fetch`. WS sự kiện: `chat.message.acked`, `chat.message.created`, `chat.message.edited`, `chat.message.recalled`, `chat.message.held`, `chat.message.removed`, `chat.reaction.updated`, `chat.pin.updated`, `chat.typing.updated`, `chat.receipt.updated`, `chat.unread.updated`, `chat.mention.created`, `chat.sanction.applied`.

**Trạng thái.** Phản hồi khi gửi bị che/chặn/giữ theo đúng bảng [CHAT_SYSTEM §4.3](../03-social-multiplayer/CHAT_SYSTEM.md) (không lặp lại thông điệp ở đây; khóa dịch `ui.chat.feedback.*`). Thêm:

| Trạng thái | Hiển thị |
|---|---|
| Đang tải lịch sử | 3 dòng skeleton ở đầu danh sách khi kéo lên |
| Kênh rỗng | "Hãy chào mọi người bằng một tin nhắn nhanh!" + quick message |
| Đang gửi / chưa gửi được | Biểu tượng đồng hồ; tự gửi lại khi có mạng |
| Khoảng trống seq | Tự tải bù, không hiển thị gì nếu thành công trong 2 giây; nếu không, "Đang đồng bộ tin nhắn…" |
| Bị tắt tiếng bởi đội an toàn | Ô nhập khóa + "Em tạm thời không gửi được tin đến {time}" (`chat.sanction.applied`) |
| Ngoài giờ chat của trường | Ô nhập khóa + giờ mở lại |
| Kênh trận đã đóng | `CHAT_CHANNEL_CLOSED` → "Trận đã kết thúc, tin nhắn chỉ xem được trong {N} ngày" |
| Mất mạng | "Đang kết nối lại chat…" |

**Tương tác & phản hồi.** `Enter` gửi, `Shift+Enter` xuống dòng (PC); mũi tên lên sửa tin vừa gửi (trong thời hạn). Gõ `@` mở danh sách nickname thành viên kênh. Thu hồi luôn hỏi lại. Báo cáo một bước: chọn lý do → gửi → "Cảm ơn em đã báo, đội an toàn EduWorld sẽ xem xét".

**Khác biệt grade band.** Quy tắc chat như nhau mọi khối (ADR-0020). Khác biệt chỉ ở trình bày: `g1_3` quick message to, nằm trên ô nhập và luôn hiện; bảng emoji/sticker mở mặc định thay vì bàn phím; cỡ chữ 24 px.

**Chỗ cho tin nhắn — quy tắc "≥ 3 tin" [Confirmed — F119 Clarifications 2026-11-27].** Tài liệu này sở hữu quy tắc bố cục chat (giá trị đăng ký ở [CONFIG_DEFAULTS](../00-overview/CONFIG_DEFAULTS.md) `chat_list_min_visible`). Khi panel chat mở với ô soạn ở trạng thái mặc định (`g1_3`: bảng mặt cười mở sẵn — **trừ chế độ `compact` trên điện thoại** (F125): mọi khối mở với hàng tin nhanh + ô nhập, nút 😊 mở bảng bằng một chạm, nút tin nhanh `g1_3` cao bằng vùng chạm `compact` thay vì 1,2×), phần nhìn thấy của danh sách tin phải cao ít nhất bằng **3 tin chữ điển hình** (tin một dòng của bạn khác: tên + giờ, câu nói, khoảng cách giữa các tin) **hoặc 45%** chiều cao nội dung panel — lấy giá trị lớn hơn — ở mọi khối × cửa sổ 1280×720 / 1024×600 / 844×390 / 740×360 (hai điện thoại ở chế độ `compact`, F125) × `vi` / `en` / pseudo +35%, ở mọi chỗ đặt panel (cột chat màn Nhóm PC, panel trượt điện thoại — trong `compact` là tấm phủ cao hết màn hình bên phải, "Thu gọn" trả lại thanh Sẵn sàng / Bắt đầu —, panel trận/trận đội). Không áp cho panel thu gọn và dải 2 dòng khi câu hỏi hiển thị. Cách giữ chỗ (không đổi gửi tin hay kiểm duyệt): tab bảng Cảm xúc / Nhãn dán / Ảnh động là chip **một dòng** (icon + chữ, không xuống dòng — biến thể `ChatTabButton`), bản dịch quá dài thì cuộn ngang bằng mũi tên; panel đủ rộng thì "Gõ chữ" · tab · hình trên **một hàng**, hẹp thì tab một hàng riêng trên "Gõ chữ" · hình; gợi ý mở bộ nhãn dán bị khóa chạy dưới cả bảng; bong bóng của bạn khác để **giờ cạnh tên** (tin chữ điển hình = 2 dòng; dòng dưới chỉ còn "đã chỉnh sửa" / "Nhắc tới em" khi có); trận đội đặt tab "Đội" / "Cả trận" **lên dòng tiêu đề** thay biểu tượng + chữ "CHAT TRẬN" (thu gọn thì hiện lại tiêu đề); màn Nhóm PC: chat là **cột phải cao hết cửa sổ** (§5); điện thoại: panel trượt phải 45% từ đỉnh thanh tiêu đề tới thanh Sẵn sàng/Bắt đầu (nút thu gọn của panel trả lại các nút của thanh tiêu đề). Kiểm tự động: `ChatSpaceProbe` trong `test_grade_band_matrix.gd` (đo bong bóng thật của khối; có test âm trên bố cục F119 cũ).

**Song ngữ.** Khóa giao diện `ui.chat.*` (ví dụ `ui.chat.typing` "{nickname} đang soạn tin…" / "{nickname} is typing…", `ui.chat.recalled` "Tin nhắn đã được thu hồi" / "This message was unsent", `ui.chat.feedback.pii_blocked` — chuỗi dài nhất). Quick message là nội dung cấu hình song ngữ: người gửi chọn câu theo ngôn ngữ của mình, **người nhận thấy câu đó theo ngôn ngữ của người nhận** (quick message gửi bằng `quick_message_id`). Tin nhập tự do **không dịch** (ADR-0056).

**Trợ năng.** Tin mới được đọc khi bật giọng đọc (tùy chọn, mặc định tắt); độ tương phản bong bóng tin đạt AA; có thể tăng cỡ chữ chat riêng trong Cài đặt.

**Hiệu năng máy tầm trung.** `MessageList` ảo hóa, giữ tối đa 200 tin trong bộ nhớ mỗi kênh (khớp bộ đệm server); sticker/GIF là ảnh động **giới hạn khung hình và kích thước** (GIF chuyển sang sprite sheet/WebP động từ thư viện nội bộ), chỉ phát khi trong khung nhìn, tối đa 3 ảnh động phát cùng lúc; typing/receipt gộp cập nhật 1 lần/giây; không render lại toàn danh sách khi có tin mới. Thẻ tin **không bao giờ chồng lên nhau** ở mọi khối × cửa sổ: chiều cao mỗi thẻ được đo lại khi bề rộng cột chat đổi (bỏ tiêu đề, đổi khối, canvas `compact`) hoặc khi nội dung thẻ cao lên (trả lời/trích dẫn, "đã chỉnh sửa", ảnh tới muộn) — thẻ sau dời xuống, không thẻ nào vẽ đè dòng tên/giờ của thẻ khác (F125 polish 2026-11-28; kiểm: `BandLayoutProbe.overlapping_messages` trong ma trận khối).

**An toàn/riêng tư.** Không bao giờ cho người gửi biết từ bị khớp. Tắt tiếng/Chặn/Báo cáo có trên mọi tin của người khác. Không có kênh với người lạ ngoài phạm vi ADR-0054.

**Lát cắt client F119 [Confirmed — F119 spec, 2026-11-27].** Chat gameplay (party, trận, boss) là `ChatDock` khung tầng 2 nền xanh nhạt (`ChatPanel`): tiêu đề "CHAT NHÓM"/"CHAT TRẬN" + nút "Người chơi cùng em" (danh sách Báo cáo người chơi / Chặn) + nút thu gọn; trận đội thêm tab "Đội"/"Cả trận" trên dòng tiêu đề thay chữ "CHAT TRẬN" (tab kiểu thẻ Cài đặt `ChatTabButton`, badge số chưa đọc riêng). Bong bóng tầng 2 viền mực mảnh: của bạn khác trắng bên trái (nickname xanh đậm + giờ trên cùng một dòng), của em xanh nhạt bên phải, tin nhắc tới em vàng kèm chữ "Nhắc tới em"; khung trả lời/trích dẫn vạch xanh bên trái ("Tin nhắn gốc không còn" khi tin gốc bị thu hồi/gỡ), "Đã chuyển tiếp từ {nickname}", "đã chỉnh sửa", giờ, và trạng thái tin của em luôn có biểu tượng **và** chữ: Đang gửi (đồng hồ) → Đã gửi (✓ xám) → Đã nhận (✓✓ xám) → Đã xem (✓✓ xanh); số cảm xúc là viên nhỏ (của em vàng + dấu tick). Nhấn giữ/chuột phải/nút ⋯ (hiện khi rê chuột) mở bảng thao tác: hàng 6 cảm xúc nhanh, Trả lời · Trích dẫn · Chuyển tiếp · Sao chép · Ghim · Sửa · Thu hồi · Ai đã xem theo quyền, rồi mục "GIỮ AN TOÀN" Tắt tiếng · Chặn · Báo cáo (chỉ tin của bạn khác — Báo cáo là thao tác thứ 2). Báo cáo: 7 lý do bằng hình + chữ (chip vàng + ✓ khi chọn), "Gửi báo cáo" mờ tới khi chọn; xong → Mây "Cảm ơn em đã báo…" + "Em có muốn chặn {nickname} không?" (Chặn / Xong). Thu hồi luôn hỏi lại (thanh tiêu đề vàng, "Hủy" có focus). Ô soạn: hàng quick message luôn ở trên (nút phụ có hình cảm xúc + câu theo ngôn ngữ người xem; `g1_3` cao hơn 1,2 lần), **bảng mặt cười và hàng gõ chữ thay phiên nhau** (bảng Emoji/Nhãn dán/Ảnh động — tab chip một dòng — + nút "Gõ chữ"; `g1_3` mở bảng sẵn), bộ đếm "{count}/{max}" (đỏ "dài quá rồi" khi vượt, nút Gửi mờ; cột chat hẹp đến mức gợi ý "Nhập tin nhắn…" bị cắt thì nút Gửi chỉ còn hình máy bay giấy — chữ "Gửi" thành tooltip + tên trợ năng, F125 polish). Băng vàng "Đang kết nối lại chat…", "Đang đồng bộ tin nhắn…" (chỉ khi > 2 s); phản hồi kiểm duyệt là băng vàng/xanh 5 s; thông điệp quan tâm (tự hại, dụ dỗ) có Mây và ở lại tới khi bấm "Em hiểu rồi". Bị tắt tiếng / trận đã kết thúc: ô nhập khóa với băng vàng có ổ khóa ("Em tạm thời không gửi được tin đến {time}…", "Trận đã kết thúc. Em vẫn đọc lại được tin nhắn.") + dòng "Tin nhắn trong trận được giữ {days} ngày…". Party lobby: PC — cột phải cao hết cửa sổ; điện thoại ngang — nút "Chat nhóm" (badge) ở thanh đáy mở panel trượt phải 45% cửa sổ từ đỉnh thanh tiêu đề, Sẵn sàng/Bắt đầu vẫn thấy (chỗ cho tin: quy tắc "≥ 3 tin" ở trên). Tối đa 3 ảnh động phát cùng lúc, chỉ trong khung nhìn; GIF hiện ảnh tĩnh `preview_url` (Godot không giải mã GIF — câu hỏi mở F119).

**Khung chat HUD cho channel bền vững (F112, [spec](../../specs/112-godot-persistent-chat-club-tools/spec.md) Ready) [Confirmed — F112 Clarifications 2026-11-28].** Nút chat dưới-trái của HUD mở `ChatHub` dùng lại toàn bộ thành phần F119 (bong bóng, ô soạn, menu, Tắt tiếng · Chặn · Báo cáo, quy tắc "≥ 3 tin"): tab chỉ cho channel server trả — "Khu vực" (đổi theo instance đang đứng), "Cả trường", mỗi club của em, "Bạn bè" (danh sách bạn + DM); World/Area chỉ join khi tab đang mở, không có số chưa đọc (chỉ badge `@`), không chuyển tiếp/ghim/đã xem, không danh sách thành viên. Điện thoại (`compact`): tấm phủ cao hết màn trượt từ trái, "✕ Đóng" đỏ. "Nhắn tin" chỉ với bạn bè, ẩn khi tenant tắt kết bạn/DM. Câu báo trung tính, không nói ai chặn ai hay vì sao: DM bị từ chối → "Em chưa nhắn riêng được với bạn này lúc này"; DM đóng băng → "Cuộc trò chuyện này đang tạm đóng"; ngoài giờ chat/policy tắt → "Kênh này đang nghỉ, em quay lại sau nhé"; club khóa → "Club đang tạm khóa, em vẫn đọc được tin cũ". Trong bài kiểm tra/thi nút chat HUD ẩn. Kính lúp tìm trong club + DM, đoạn trích là văn bản thuần.

## 8. Competition Arena

**Mục đích.** Chọn chế độ thi đấu, vào hàng chờ, xác nhận trận, chơi trận 1v1/đội, xem kết quả và tier. Nghiệp vụ: [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) (luật, chấm điểm, tier, leaderboard), [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md) (hàng chờ, ready check).

**Điểm vào/ra.** Đi vào Đấu trường Nhà Vô Địch (NPC Trọng tài Sấm) · nút Thi đấu trên HUD (phím `K`) · lời thách đấu của bạn. Ra: hủy hàng chờ, hoặc sau màn kết quả.

**Bố cục Web/PC — chọn chế độ & hàng chờ**

```text
┌──────────────────────── Đấu trường · Arena ─────────────── Tier: 🌱 Chồi Non  [X] ┐
│ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐                    │
│ │ 1v1 Đấu trí │ │ Đấu đội     │ │ Thử thách   │ │ Giải đấu    │                    │
│ │ 3–5 phút    │ │ 2v2–4v4     │ │ hợp tác     │ │ (lịch)      │                    │
│ └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘                    │
│ Môn: [Toán ▼]  Chủ đề: [Tất cả ▼]          [Bảng xếp hạng] [Lịch sử trận]          │
│                                                                                    │
│                       [ Tìm trận ▶ ]                                               │
└────────────────────────────────────────────────────────────────────────────────────┘
```

Khi đang chờ: thanh trạng thái nổi trên HUD "Đang tìm đối thủ… 0:24 [Hủy]" — học sinh được đi lại trong thế giới. Khi có trận: hộp **ready check** "Đã tìm thấy trận! [Vào trận] (15 giây)".

**Bố cục Web/PC — trong trận 1v1**

```text
┌ MèoXanh42  ████ 420            Câu 4/7  ⏱ ▬▬▬▬▬▬▭▭          CáHeoVui07  ███ 350 ┐
│                                                                                 │
│                  Câu hỏi (renderer hoạt động học tập, công thức SVG)            │
│                                                                                 │
│        [ A … ]          [ B … ]          [ C … ]          [ D … ]               │
│                                                                                 │
│ Chat trận (thu nhỏ): [Chúc may mắn!][Hay quá!]                                  │
└─────────────────────────────────────────────────────────────────────────────────┘
```

Màn kết quả: Thắng/Hòa/Chưa thắng (từ ngữ động viên), điểm từng câu, câu cần ôn (link ôn tập), Competition Points nhận được, tier (không có số rating), [Chơi lại] [Về Đấu trường], nút Báo cáo/Chặn đối thủ.

**Bố cục Mobile**

```text
┌ MèoXanh42 420        Câu 4/7  ▬▬▬▬▭▭        CáHeoVui07 350 ┐
│                                                             │
│             Câu hỏi (cỡ chữ theo grade band)                │
│                                                             │
│   [     A …     ]                 [     B …     ]           │ ← 2×2, ô ≥ 64 dp
│   [     C …     ]                 [     D …     ]           │
│ 💬 Hay quá!                                    [Chúc may mắn]│ ← dải chat mỏng
└─────────────────────────────────────────────────────────────┘
```

Chọn chế độ trên mobile: thẻ chế độ cuộn ngang, nút Tìm trận cố định dưới phải. Tablet: như PC.

**Thành phần.** `ModeCard`, `SubjectTopicPicker`, `QueueStatusBar`, `ReadyCheckDialog` (đếm ngược), `MatchHeader` (điểm hai bên/đội, tiến độ câu), `QuestionPanel` (renderer hoạt động — [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md)), `AnswerButtons`, `AnswerRevealOverlay`, `MatchChatStrip` (channel `match`/`team`, always-on), `ResultScreen`, `TierBadge`, `LeaderboardView` (theo scope/period; lớp 1–3 hiển thị mục tiêu tập thể của khối trong trường — `grade_goal`, không tên — thay thứ hạng), `MatchHistoryList`.

**Dữ liệu & API.** REST: `createMatchmakingTicket`, `getMatchmakingTicket`, `cancelMatchmakingTicket`, `acceptMatchProposal`, `createDuel`/`acceptDuel`/`declineDuel`, `getMatch`, `listMyMatches`, `getMyRatings` (chỉ dùng tier), `getLeaderboard`, `getMyLeaderboardPosition`, `listTournaments`. WS: `matchmaking.ticket.updated`, `matchmaking.proposal.created`/`updated`/`cancelled`, `matchmaking.proposal.accept`/`decline`, `matchmaking.match.assigned`, `battle.countdown.started`, `battle.question.shown`, `battle.answer.submit`, `battle.answer.recorded`, `battle.answer.evaluated`, `battle.question.revealed`, `battle.round.ended`, `battle.participant.status_changed`, `battle.match.paused`/`resumed`, `battle.match.ended`, `battle.state.request`/`snapshot`, `competition.duel.received` (lời thách đấu đến).

**Trạng thái.**

| Trạng thái | Hiển thị |
|---|---|
| Chờ quá lâu | Gợi ý "Thử chế độ khác" hoặc "Luyện với Robot" (bot unranked có nhãn rõ) theo MATCHMAKING |
| Đã có ticket | `MM_TICKET_ALREADY_ACTIVE` → mở thanh hàng chờ hiện có |
| Đối thủ mất kết nối | "Đối thủ đang kết nối lại…" (grace theo [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md)) |
| Em mất kết nối | Tự kết nối lại và vào lại trận (`battle.state.request`); quá grace → kết quả tính theo luật |
| Trận bị hủy do sự cố | "Trận đã hủy, không tính kết quả" |
| Thua liên tiếp | Thông điệp động viên + gợi ý ôn chủ đề ([COMPETITION_SYSTEM §12](../01-game-design/COMPETITION_SYSTEM.md)) |

**Tương tác.** Trả lời một lần, khóa ngay sau khi chọn (hiệu ứng "đã ghi nhận"); điểm và đúng/sai chỉ hiển thị khi server công bố (`battle.answer.evaluated`/`question.revealed`). PC: phím `1`–`4` chọn đáp án.

**Khác biệt grade band.** `g1_3`: chỉ 1v1 và Thử thách hợp tác; thời gian dài hơn và **không hiển thị đếm ngược số giây** (thanh màu); không có bảng xếp hạng cá nhân. `g4_5`: thêm Đấu đội; bảng top 10. `g6_9`: đủ chế độ, bảng top 50 ± 3.

**Song ngữ.** `ui.arena.title`, `ui.arena.find_match` ("Tìm trận" / "Find match"), `ui.arena.ready_check` ("Đã tìm thấy trận! Vào trận trong {seconds} giây" / "Match found! Join within {seconds} seconds"), `ui.arena.result.not_won` ("Chưa thắng lần này — em đã làm rất tốt ở {topic}!" / "Not this time — you did great on {topic}!") — chuỗi dài nhất. Tên tier là cấu hình `i18nText`. Câu hỏi hiển thị theo ngôn ngữ nội dung ([ADR-0056](../adr/ADR-0056-game-song-ngu-viet-anh.md) mục 3); hai người cùng trận có thể thấy hai ngôn ngữ giao diện khác nhau.

**Trợ năng.** Đếm thời gian có cả thanh và rung nhẹ (mobile) khi còn 5 giây (tắt được); không chỉ dùng màu đỏ/xanh cho đúng/sai (có ✓/✗).

**Hiệu năng.** Trong trận **không render thế giới 3D** phía sau (nền 2D tĩnh hoặc cảnh đấu trường đơn giản ≤ 30k tam giác, không bóng đổ); avatar hai bên là ảnh 2D + animation sprite nhẹ. Cập nhật điểm theo sự kiện, không polling.

**An toàn/riêng tư.** Đối thủ chỉ thấy nickname, avatar, tier; không hiển thị trường khi đấu liên trường. Không có thông báo "bạn bị vượt qua".

**Lát cắt client F109 [Confirmed — F109 spec, 2026-11-27].** *Đấu trường* (`ArenaScreen`; HUD "Thêm" → Đấu trường / Bảng xếp hạng, phím `K`, NPC Trọng tài Sấm `open_ui: arena`) là cửa sổ tầng 1 như màn Nhóm (lề 1,5%/2%; thanh tiêu đề xanh "ĐẤU TRƯỜNG" + chip hạng "Hạng: Bạc II" — tên tier của server, "Đang xếp hạng (2/5)" khi còn provisional, không bao giờ số rating — + "Đóng" đỏ) với 4 tab kiểu Cài đặt (icon trắng `tab_match`, `tab_podium`, `tab_history`, `tab_bracket`):
- *Thi đấu:* dòng hướng dẫn "Chọn chế độ em thích rồi bấm Tìm trận nhé!", một `ModeCard` (`PictureTile`: hình chế độ lớn, tên + "Tính hạng" / "Luyện tập, không tính hạng"; thẻ đang chọn vàng **và** nhãn "Đã chọn") cho **đúng các chế độ server trả** (`listCompetitionModes`, client không tự quyết theo khối), dòng chip "Môn học" (tên từ `listSubjects`) và "Chủ đề" (chỉ khi chế độ ghép theo chủ đề; "Tất cả chủ đề" = không chủ đề), "Thách đấu bạn" và "Tìm trận" ở đáy (canvas `compact`: thành cột bên phải các thẻ ("Thách đấu bạn" rồi "Tìm trận") — hàng nút ở đáy sẽ cắt thẻ chế độ; F125 polish 2026-11-28). Thẻ và môn đầu đã chọn sẵn: HUD → Đấu trường → Tìm trận ≤ 3 thao tác (SC-001). Không có chế độ nào → câu rỗng thân thiện.
- *Bảng xếp hạng:* chip mỗi bảng server trả. `grade_goal` → `GradeGoalBar` (cờ, "Mục tiêu của khối 3", thanh tiến độ, "Cả khối: 640/1.000", "Em đã góp 25", "38 bạn đã góp sức" chỉ khi server gửi số — ẩn khi < 5, không ô trống/0 —, nhãn "Đã đạt!", Mây "Mỗi câu đúng của em giúp cả khối tiến gần mục tiêu!"; không hạng, không nickname người khác). `top_n_percentile` → các dòng top + thẻ "Em đang ở nhóm 25%, cố lên nhé!". `ranked` → top + mục "Quanh em" (không lặp dòng). Dòng = "#hạng", chân dung 2D, nickname (một dòng, không cắt), giá trị; dòng của em nền vàng nhạt **và** nhãn "Em". "Đang cập nhật" khi `stale`; 404 → "Bảng này hiện không mở" và bỏ chip; "Làm mới" là cách duy nhất đọc lại (không hẹn giờ).
- *Lịch sử:* chip lọc chế độ, thẻ trận (icon kết cục, "Em thắng rồi! Giỏi quá! · 1v1 Đấu trí", "Cùng: CaHeoVui07", "+12 điểm thi đấu", "Xem"), "Xem thêm" (cursor) ở cuối danh sách, trong vùng cuộn (không thành hàng nút cắt thẻ cuối — F125 polish). Chi tiết: Mây với kết cục động viên, người chơi (chân dung + nickname, nhãn "Em"), "Xem lại các câu" (✓/✗ trong đĩa + chữ "Em đã trả lời đúng" / "Em ôn lại câu này nhé"), "Người chơi cùng em" (Báo cáo người chơi / Chặn — `ChatPlayerSafety` + bảng báo cáo của F119). 404 → "Không tìm thấy".
- *Giải đấu:* thẻ thử thách khối Timed Quiz ("Vào phòng" chỉ trong cửa sổ của server) và thẻ giải (trạng thái bằng chữ, giờ bắt đầu, "Em đã đăng ký"). Chi tiết: lịch (hạn đăng ký, mở check-in, bắt đầu), số đã đăng ký, phần thưởng, "Đăng ký" / "Rút tên" / "Check-in" (xám kèm "Check-in sẽ mở đúng giờ nhé" ngoài cửa sổ server); sơ đồ: lớp 6–9 cây theo vòng (người thắng có ✓ **và** chữ "Thắng"), lớp 1–5 "con đường" chỉ các trận của em ("Vòng 1 · Gặp CaHeoVui07 · Em đã thắng!"). Lỗi đăng ký/check-in là băng đỏ một câu theo mã của server: `TOURNAMENT_FULL` "Giải đã đủ bạn tham gia rồi. Em xem giải khác nhé!", `TOURNAMENT_REGISTRATION_CLOSED`, `TOURNAMENT_CHECK_IN_CLOSED`, `TOURNAMENT_NOT_ELIGIBLE` ("Lúc này em chưa tham gia được giải này." — không nêu lý do hạn chế), đã đăng ký (`409 CONFLICT`) "Em đã đăng ký giải này rồi."; sau lỗi chi tiết giải được đọc lại để nút theo đúng trạng thái server.
- *Thách đấu bạn:* hộp thoại chọn một bạn (chip nickname theo danh sách bạn bè của server, không nhóm theo lớp/khối) + môn → "Gửi lời thách đấu" → "Đã gửi! Bạn có 60 giây để trả lời."; người nhận (WS `competition.duel.received`): hộp `DuelIncomingPrompt` nổi trên thế giới, giữa phía trên — chân dung + nickname + "Cấp 7" của bạn thách (chỉ thẻ công khai, R16), "CaHeoVui07 muốn thách đấu em!", "Môn Toán · Phép nhân", nhãn "Luyện tập, không tính hạng", thanh thời gian theo `expires_at` của server (60 s; lớp 4–9 thêm "Còn 45 giây", lớp 1–3 chỉ thanh, không đếm số), "Nhận lời" (`acceptDuel` → vào trận) / "Để lần sau" (`declineDuel`); không nhấp nháy, không âm thanh; tự đóng khi hết hạn (không lưu thông báo); không hiện khi đang thi, thử thách, làm bài, trong trận hoặc tắt popup; hết hạn khi bấm → "Lời thách đấu đã hết hạn" + "Đóng"; ngoài phạm vi → một câu chung không nêu lý do.
- *Hàng chờ:* `QueueStatusBar` (`HudPanel`, đáy giữa màn — giữa joystick và nút hành động; đỉnh màn là chồng toast; canvas `compact`: các nút xuống dưới chữ và thanh nằm trong dải 30%–75% bề ngang để không đè thẻ "Việc tiếp theo"/joystick bên trái và "Về Quảng trường" bên phải — F125 polish) "Đang tìm bạn đấu… 0:24 · 1v1 Đấu trí" (đếm lên từ `wait_ms` của server, không đếm ngược) + "Hủy"; dòng phụ "Đang tìm rộng hơn một chút cho em." (`search_widened`) hoặc "Chờ hơi lâu? Em có thể luyện với Robot (không tính hạng)." + "Luyện với Robot" (`bot_offer_available`); hết giờ → "Chưa tìm được bạn đấu lần này. Em thử chế độ khác hoặc tìm lại nhé!" + "Chọn chế độ khác" / "Tìm lại". Học sinh vẫn đi lại trong thế giới.
- *Ready check:* hộp thoại vàng "Đã tìm thấy trận!" với người chơi (chân dung + nickname), thanh thời gian theo `deadline_at` (đồng hồ server), "Vào trận trong 12 giây" (lớp 1–3: "Bấm Vào trận nhé!", không số), "Để lần sau" / "Vào trận"; đã nhận → "Đang chờ các bạn khác… (1/2)".
- *Màn trận* (`BattleArena`, không thế giới 3D — nền trời vector tĩnh + bảng kem của màn học tập): `MatchHeader` khung tầng 1 (hai thẻ trắng: chân dung + nickname + "420 điểm" của server; đấu đội: "Đội của em" / "Đội bạn" + điểm đội + nickname thành viên, nhãn "Lượt của em" / "Em hỗ trợ" từ `primary_user_ids`, mỗi đội người trả lời chính câu này có ngôi sao + chữ đậm trước nickname), giữa "Câu 2/3" + thanh thời gian (lớp 4–9 thêm "12 giây"; lớp 1–3 chỉ thanh). Câu hỏi bằng renderer F063 (chạm một ô = trả lời với câu một bước, phím 1–4; loại khác có "Gửi đáp án"); ngay sau khi chạm mọi ô khóa và băng "Đã ghi nhận! Chờ kết quả nhé."; khi server chấm: ô của em ✓ "Đúng rồi!" / ✗ (đáp án đúng có nhãn), dải phản hồi `FeedbackCorrectPanel`/`FeedbackIncorrectPanel` "Đúng rồi!" / "Chưa đúng lần này, không sao!" (dải được cuộn vào tầm nhìn khi hiện) + "+150 điểm" + "Thưởng tốc độ +50" **chỉ khi** `time_bonus > 0` + giải thích. Chat trận (`ChatDock`, always-on, không có nút tắt): giữa hai câu là cột phải cao hết màn (quy tắc "≥ 3 tin" ở §7); khi câu hỏi hiện, dock thu thành dải 2 dòng **cạnh** `MatchHeader` (góc trên phải) và câu hỏi chiếm cả chiều ngang bên dưới — không bao giờ che đáp án; cửa sổ hẹp/chữ dài không đủ chỗ cạnh header thì dải nằm dưới câu hỏi và câu hỏi cuộn. Trên canvas `compact` (điện thoại, F125) khi câu hỏi hiện, chat thu về **dòng tiêu đề gọn** (biểu tượng chat — chữ "CHAT TRẬN" thành tooltip —, số tin chưa đọc, "Mở chat") thay cho dải 2 dòng: nằm cạnh `MatchHeader` khi hàng đó đủ chỗ (hàng chỉ cao bằng header, thẻ đội đủ rộng để "Đội của em" nằm một dòng), không thì dưới câu hỏi — mọi ô đáp án thấy trọn không cần cuộn ở mọi khối (F125 polish 2026-11-28). Băng vàng "CaHeoVui07 đang kết nối lại… Em chờ một chút nhé." (tới `grace_until`), "Em đang kết nối lại… Trận vẫn giữ cho em."; bạn đang kết nối lại luôn có biểu tượng wifi gạch trên thẻ của bạn đó (đấu đội: trước nickname trong thẻ đội), câu đầy đủ là tooltip + tên trợ năng — trên canvas `compact` (điện thoại, F125) **khi câu hỏi đang hiện**, băng của người chơi khác thu vào biểu tượng này để mọi đáp án thấy được không cần cuộn (băng "Em đang kết nối lại…" vẫn giữ; giữa hai câu và trên PC băng hiện như cũ — F125 polish 2026-11-28); `battle.match.paused` → lớp phủ "Trận tạm dừng một chút, em chờ nhé."; bắt đầu trận → số 3‑2‑1 (`CountdownLabel`) theo `starts_at` của server, kèm "Trả lời đúng và nhanh để được thêm điểm nhé!" **chỉ khi** server gửi `speed_bonus_enabled = true` (luật trận đã ghim; lớp dưới ngưỡng không bao giờ thấy).
- *Kết quả* (khung tầng 1 thay câu hỏi): Mây + "Em thắng rồi! Giỏi quá!" / "Hòa! Cả hai đều giỏi!" / "Chưa thắng lần này, em đã làm rất tốt ở các câu hỏi!", "Em được 420 · đúng 3/3 câu", "+12 điểm thi đấu", chip hạng (sau khi đọc lại `getMyRatings`); Timed Quiz lớp 1–3: `GradeGoalBar` của lượt, lớp 4–9: "Em xếp thứ 3 trong lượt này" + "Các bạn đứng đầu: …"; lời động viên khi server gửi `encouragement` (chỉ người vừa thua chuỗi, COMPETITION_SYSTEM §12): dòng thứ hai trong bong bóng Mây có icon sách "Em đã rất cố gắng! Mình cùng ôn lại chủ đề Phép nhân môn Toán rồi thử tiếp nhé." (khóa `ui.competition.encouragement.loss_streak_review`, trận không có chủ đề → câu chỉ nêu môn; không nói số trận thua); "Câu cần ôn" (câu sai trước); "Người chơi cùng em" (không có cho Robot); "Về Đấu trường" / "Chơi lại" (hàng nút ở đáy thẻ, vùng cuộn luôn dừng phía trên hàng nút — không nút nào che nội dung; trên canvas `compact` thẻ kết quả chiếm cả cột, `MatchHeader` ẩn — "Câu 1/3" và thanh thời gian đã cũ — để bong bóng Mây, kể cả lời động viên, và các dòng điểm đọc được ngay mà không cuộn; hạng, "Câu cần ôn", "Người chơi cùng em" cuộn tiếp — F125 polish 2026-11-28). Trận `cancelled`/`aborted` → "Trận đã hủy, không tính kết quả." (không "Chơi lại").
- *Thông báo:* `tournament_update` dùng toast F123 và mở tab Giải đấu (`ref_id` = giải); `tournament.match.scheduled` → toast "Trận giải {tên} của em bắt đầu sau 10 phút!" + "Xem". Không có thông báo "bị vượt qua", không làm mới bảng định kỳ.

## 9. Event center

**Mục đích.** Một nơi xem sự kiện đang diễn ra, nhiệm vụ ngày/tuần, mục tiêu tuần, season track miễn phí và phần thưởng. Nghiệp vụ: [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) (sự kiện, season track, mục tiêu cộng đồng), [QUEST_SYSTEM §2.4](../01-game-design/QUEST_SYSTEM.md) (daily/weekly), [GAME_DESIGN §7.3](../01-game-design/GAME_DESIGN.md).

**Điểm vào/ra.** HUD trên-phải (biểu tượng lịch, có chấm khi có nội dung mới) · bảng tin ở Quảng trường Tia Sáng · thông báo bắt đầu sự kiện (không đẩy ngoài game). Ra: Đóng.

**Bố cục Web/PC**

```text
┌──────────────────────── Sự kiện · Events ──────────────────────────────────── [X] ┐
│ [Hôm nay] [Tuần này] [Sự kiện] [Mùa]                                              │
├───────────────────────────────────────┬──────────────────────────────────────────┤
│ Nhiệm vụ hôm nay (đặt lại 04:00)       │ Mục tiêu tuần: học 4/7 ngày  ●●●○○○○      │
│ ☐ Học: Giải 5 bài phép nhân  → [Đi]    │ Thưởng: Mũ Lá Phong                       │
│ ☑ Khám phá: Tìm 1 điểm bí mật          │──────────────────────────────────────────│
│ ☐ Chơi cùng bạn (tùy chọn)  → [Đi]     │ Sự kiện: Lễ hội Trăng Rằm · còn 5 ngày    │
│ Nhà trường giao (ưu tiên)   → [Đi]     │ Mục tiêu cả trường ████████░░ 8.120/10.000│
│                                        │ [Xem chi tiết]                            │
└───────────────────────────────────────┴──────────────────────────────────────────┘
```

Tab **Mùa**: season track miễn phí dạng dải ngang có mốc (điểm mùa → phần thưởng), mốc đã nhận/đang tới; không có track trả phí.

**Bố cục Mobile**

```text
┌ ←  Sự kiện   [Hôm nay|Tuần này|Sự kiện|Mùa] ⇄                ┐
│ ☐ Học: Giải 5 bài phép nhân                      [Đi ›]      │
│ ☑ Khám phá: Tìm 1 điểm bí mật                                │
│ ☐ Chơi cùng bạn (tùy chọn)                       [Đi ›]      │
│ ─────────────────────────────────────────────────────────── │
│ Lễ hội Trăng Rằm · còn 5 ngày  ████████░░ 81%   [›]          │
└──────────────────────────────────────────────────────────────┘
```

Tablet: hai cột như PC.

**Thành phần.** `DailyQuestList` (nút Đi dẫn đường tới NPC/khu), `WeeklyGoalTracker`, `EventCard` (thời gian còn lại dạng ngày, không đếm giây), `CommunityGoalBar`, `SeasonTrack` (cuộn ngang ảo hóa), `RewardPreview`.

**Client (F111, [Confirmed — F111 Clarifications 2026-11-28]).** Mở từ nút lịch "Sự kiện" ở cột phải HUD (cùng hàng với "Thêm"; chấm tĩnh khi có sự kiện em chưa xem, tắt khi đã mở), ô "Sự kiện" trong menu "Thêm", bảng tin sự kiện ở Quảng trường Tia Sáng và thông báo `open_event`. Hôm nay/Tuần này: `listQuests` theo `quest_type`, trạng thái + "{done}/{total} bước" của server, "Làm mới lúc {time}, ngày {date}" (ICT), nút "Đi" (nhận quest nếu còn chưa nhận rồi dẫn đường); xong hết → "Tuyệt! Em đã xong nhiệm vụ hôm nay. Nghỉ ngơi chút nhé". Sự kiện: thẻ (biểu tượng loại sự kiện, tên theo ngôn ngữ — thiếu tiếng Anh dùng tiếng Việt, "Còn n ngày"/"Hôm nay là ngày cuối"/"Sắp diễn ra"/"Sự kiện tạm dừng"), chi tiết (mô tả, Tham gia, điểm sự kiện + "Hôm nay em còn nhận được n điểm", nhiệm vụ sự kiện + Đi, mốc thưởng Nhận/Đã nhận/Chưa đạt/"Mới đạt!", thanh mục tiêu cộng đồng 25/50/75/100%, "Em đã góp sức", khối góp sức chỉ khối 4–9 server trả, không xếp hạng). Mùa: tên mùa, "Cấp n/max", điểm mùa, "Miễn phí cho mọi bạn", "Phần thưởng chưa nhận sẽ tự gửi khi hết mùa", dải cấp ảo hóa, Mùa trước; 404 → "Mùa mới sắp bắt đầu". Thế giới 3D tạm dừng khi mở. Điện thoại (`compact`, F125 — polish 2026-11-28): cửa sổ cao gần hết màn; chi tiết sự kiện ẩn hàng tab ("Quay lại" chỉ còn mũi tên, chữ thành tooltip + tên trợ năng), tên sự kiện + thời gian còn lại + điểm sự kiện chung một dòng, nên điểm và nhiệm vụ sự kiện đầu tiên (sự kiện đã kết thúc: ghi chú nhận thưởng + mốc đầu tiên) thấy ngay; tab Mùa đặt dải cấp ngay dưới dòng tên mùa, khung "Miễn phí…/tự gửi…/kết thúc" sau dải; tab "Tuần này" dùng biểu tượng sách màu (biểu tượng nét xanh biến mất trên tab đang chọn màu xanh).

**Dữ liệu & API.** Quest: `listQuests` (lọc daily/weekly), `acceptQuest`. Sự kiện và season track: `listGameEvents`, `getGameEvent`, `joinGameEvent`, `claimEventMilestone`, `getCommunityGoal`, `getSeasonTrack`, `claimSeasonTier`, `listSeasonTrackHistory` (F077, có trong `game.v1`); thiết kế ban đầu (lịch sử) theo EVENT_SEASON_SYSTEM — ví dụ `GET /api/v1/game/events` (đang diễn ra cho học sinh), `GET /api/v1/game/events/{event_id}` (tiến độ cá nhân + mục tiêu cộng đồng), `GET /api/v1/game/season/track` (mốc + tiến độ); cập nhật tiến độ qua `quest.progress.updated` và sự kiện WS mới **đề xuất** `event.progress.updated` (thuộc fork EVT/spec xác nhận).

**Trạng thái.** Không có sự kiện ("Sự kiện mới sắp đến, quay lại sau nhé!") · Đã hoàn thành hết nhiệm vụ ngày ("Tuyệt! Em đã xong nhiệm vụ hôm nay. Nghỉ ngơi chút nhé" — không thúc chơi thêm) · Sự kiện kết thúc ("Đã kết thúc — phần thưởng sẽ quay lại mùa sau") · Mất mạng (cache).

**Tương tác.** Nút "Đi" mở dẫn đường (đường chỉ hướng trên mặt đất, không dịch chuyển tức thì trừ khi có cổng gió). Không có phạt mất chuỗi; mục tiêu tuần tính theo số ngày, không cần liên tiếp.

**Khác biệt grade band.** `g1_3`: chỉ tab Hôm nay và Sự kiện, mỗi nhiệm vụ có hình minh họa lớn và nút Đi lớn. `g6_9`: đủ 4 tab.

**Song ngữ.** `ui.events.title`, `ui.events.reset_time` ("Đặt lại lúc {time}" / "Resets at {time}"), `ui.events.time_left_days` ("còn {n} ngày" / "{n} days left"), `ui.events.all_done` — chuỗi dài nhất. Tên/mô tả sự kiện là `i18nText`; giờ hiển thị theo múi giờ Việt Nam ở cả hai ngôn ngữ.

**Trợ năng.** Thời gian còn lại bằng chữ; danh sách đọc được theo thứ tự.

**Hiệu năng.** Season track ảo hóa (chỉ tạo mốc trong khung nhìn); ảnh phần thưởng là thumbnail atlas.

**An toàn/riêng tư.** Mục tiêu cộng đồng chỉ hiển thị tổng, không bảng xếp hạng cá nhân trong sự kiện cho lớp 1–5.

## 10. Cài đặt

**Mục đích.** Ngôn ngữ, đồ họa/hiệu năng, âm thanh, điều khiển, trợ năng, quyền riêng tư, tài khoản. Quy định liên quan: [ADR-0056](../adr/ADR-0056-game-song-ngu-viet-anh.md), [ADR-0057](../adr/ADR-0057-chay-tren-may-tam-trung-hang-thiet-bi-va-ngan-sach-hieu-nang.md), [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md), [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) (nhắc nghỉ), [GAME_NETWORKING](../08-game-client/GAME_NETWORKING.md) (đăng xuất, thiết bị dùng chung).

**Điểm vào/ra.** HUD trên-phải biểu tượng bánh răng (phím `Esc` khi không có màn phụ → Menu → Cài đặt). Ra: Đóng (lưu tự động từng thay đổi).

**Bố cục (F114, mọi thiết bị — game chỉ chạy ngang, F113) [Confirmed — owner duyệt preview 2026-11-26]**

```text
┌──────────────────────────────────────────────────────────────────────────────────────┐
│ CÀI ĐẶT  (thanh tiêu đề màu nhấn, chữ trắng in hoa, cạnh cứng mực)         [x Đóng] │  ← DangerButton đỏ
├───────────────────┬──────────────────────────────────────────────────────────────────┤
│ [(o) Ngôn ngữ]    │ (NGÔN NGỮ / LANGUAGE)      ← SectionTag vàng nhạt                │
│  [^] Đồ họa       │ [v Tiếng Việt]  [ English ]  ← chip đang chọn vàng + ✓           │
│  <)) Âm thanh     │                                                                  │
│  (*) Trợ năng     │ Âm thanh:  (n) Nhạc nền [−] ━━━━━━●──── [+]  (70%)                 │
│  (i) Thông tin    │ Trợ năng:  Tương phản cao              Tắt (◯━━)                 │
│                   │            Giảm chuyển động            Bật (━━●)                 │
│ [->| Đăng xuất]   │                       (nội dung cuộn dọc khi dài)                │
└───────────────────┴──────────────────────────────────────────────────────────────────┘
```

Cửa sổ giữa màn trên lớp phủ mực 50%, chừa ít nhất khoảng cách khối mỗi phía (vừa 1280×720 và điện thoại ngang 844×390). Cột tab trái nền nhạt của màu nhấn (`component.settings.tab_column_bg`), viền phải mực; tab = icon + chữ; tab đang mở = nền nhấn chữ trắng + viền + cạnh dưới; tab khác trong suốt chữ mực; "Đăng xuất" đỏ ở đáy cột; cột tab cuộn được khi cỡ chữ lớn. Nội dung bên phải cuộn dọc khi dài. Quy tắc viền/màu ở [DESIGN_SYSTEM §5.1](DESIGN_SYSTEM.md#51-token-ui). (Bố cục Web/PC và Mobile trước F114 — hai cột trên PC, trang con toàn màn trên điện thoại — được thay bằng cửa sổ này.)

**Nội dung từng nhóm.**

| Nhóm | Thiết lập | Ghi chú |
|---|---|---|
| Hồ sơ của em | Chỉ xem: trường (tên song ngữ), tỉnh/thành, khối — lấy từ PlayerBootstrap (dữ liệu SIS đồng bộ khi đăng nhập, ADR-0061); nút "Thông tin chưa đúng? Báo nhà trường" (mở hướng dẫn, không sửa được trong game). **Nickname (F136, ADR-0105):** nickname hiện tại, ô gõ + chip gợi ý + "Gợi ý khác", dòng "Đừng dùng tên thật của em", số lần đổi còn trong tháng; hết lượt → ô khóa kèm ngày đổi lại được (ngày theo locale, không đếm ngược); quy tắc ở [CHARACTER_SYSTEM §5.3](../01-game-design/CHARACTER_SYSTEM.md#53-nickname). Học sinh tự đăng ký thấy trường "EduWorld chung" (ADR-0104) | Chỉ chính học sinh thấy; người khác không thấy trường/khối (ADR-0060, ADR-0095, [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md)); khóa dịch `ui.settings.profile.*` |
| Ngôn ngữ | Tiếng Việt / English; hiển thị song song tên riêng & từ vựng chủ chốt | Đổi ngay, không khởi động lại; lưu vào hồ sơ (`updateGameProfile`) để đồng bộ thiết bị; mặc định theo tenant |
| Đồ họa | Preset Thấp / Trung bình / Cao; Tự phát hiện lại (benchmark 5 giây); độ phân giải động; giới hạn FPS (30/60; mobile mặc định 30); số người chơi hiển thị (Ít/Vừa/Nhiều); hiện FPS | Preset và giá trị theo [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md); máy không đủ cho preset đã chọn → gợi ý hạ preset (không tự ép) ; lưu cục bộ theo thiết bị |
| Âm thanh | Âm lượng tổng, nhạc, hiệu ứng, giọng NPC; giọng đọc (TTS) cho câu hỏi/chat | TTS mặc định bật cho `g1_3` |
| Điều khiển | PC: gán phím, độ nhạy chuột, đảo trục; Mobile: vị trí/kích thước joystick và nút, rung | Đặt lại mặc định |
| Trợ năng | Cỡ chữ (+0/+1/+2 bậc theo density), tương phản cao, giảm chuyển động, phụ đề hội thoại NPC (luôn bật mặc định), chế độ màu cho người mù màu | Áp dụng toàn bộ UI game |
| Riêng tư | Ẩn nickname khỏi bảng xếp hạng ngoài trường (bảng toàn platform) [Proposed — ADR-0095 bỏ bảng lớp]; ai được gửi lời mời kết bạn (theo phạm vi cho phép, chỉ siết chặt hơn); hiển thị "đã xem" ở tin nhắn riêng; danh sách đã chặn | Không cho nới quá chính sách trường |
| Tài khoản | Nickname (chỉ xem), trường (chỉ xem), thời gian chơi hôm nay (thông tin, kèm nhắc nghỉ), **Đăng xuất**, Đăng xuất trên mọi thiết bị | Đăng xuất hỏi xác nhận; trên máy dùng chung luôn hiện nút Đăng xuất to |
| Thông tin (F096) | **Xem lại hướng dẫn của Mây** (mở lại hướng dẫn lần đầu của S03); **Giấy phép**: trang cuộn trong panel Cài đặt — phiên bản game, câu giới thiệu, giấy phép của EduWorld (hình ảnh/mô hình/icon MIT; font Baloo 2, Be Vietnam Pro OFL-1.1), văn bản giấy phép MIT của Godot Engine và danh sách thành phần bên thứ ba kèm văn bản giấy phép, đọc từ engine (`Engine.get_license_text()`, `get_copyright_info()`, `get_license_info()`); nút "Quay lại Cài đặt" | Văn bản pháp lý giữ nguyên tiếng Anh (không dịch), tiêu đề/nút qua `.po` (`ui.settings.about`, `ui.settings.licenses*`, `ui.settings.replay_guide`, `ui.settings.back`); có ở mọi khối; [Confirmed — F096 Clarifications Session 2026-11-26: cách nhỏ nhất để giữ thông báo MIT của Godot] |

**Dữ liệu & API.** `getGameProfile`, `updateGameProfile` (ngôn ngữ, cài đặt đồng bộ như trợ năng/riêng tư), `logout`, `endGameSession`, `postTelemetry` (kết quả tự phát hiện preset, FPS trung bình — không chứa PII). Đăng xuất mọi thiết bị: **đề xuất** `POST /api/v1/auth/sessions:revoke-all` nếu chưa có (cần fork spec xác nhận).

**Trạng thái.** Lưu thất bại (mất mạng) → giữ thay đổi cục bộ, đồng bộ lại khi có mạng; thiết lập bị trường khóa (ví dụ không nới phạm vi kết bạn) → hiển thị khóa + "Do nhà trường cài đặt".

**Bản hiện tại trong client (F114) [Confirmed].** Chỉ các thiết lập đã có trong `Settings`/`QualitySettings`, lưu cục bộ ngay khi đổi, không gửi server:

| Tab | Nội dung |
|---|---|
| Ngôn ngữ | Tiêu đề song ngữ "Ngôn ngữ / Language"; 2 chip Tiếng Việt / English (đổi ngay, vẫn ở tab Ngôn ngữ) |
| Đồ họa | 3 chip preset có hình (1/2/3 cột) Thấp / Trung bình / Cao; khối khác `g1_3` thêm công tắc Bóng đổ, Khử răng cưa và gợi ý hạ preset như F062 |
| Âm thanh | 3 hàng Nhạc nền / Hiệu ứng / Giọng nói: icon + tên, nút "−", thanh trượt, nút "+", nhãn "70%". **Bước 10%** (hằng client `Settings.VOLUME_STEP` — sở hữu ở đây); kéo thanh vẫn được; ở 0% / 100% nút − / + mờ (vẫn có viền); nhãn cho trình đọc "Giảm/Tăng âm lượng {tên}". F137: kéo/bấm −/+ phát một tiếng mẫu của bus đó (Nhạc nền: đoạn nhạc đang phát, Hiệu ứng: tiếng chạm, Giọng nói: câu mẫu của Mây); thêm công tắc "Tiếng bước chân" (`ToggleSwitch`, mặc định theo preset — [STUDENT_GAME_UX §8a](STUDENT_GAME_UX.md#8a-âm-thanh-trong-game)) |
| Trợ năng | Cỡ chữ 3 mức (chip Vừa / Lớn / Rất lớn), Tương phản cao và Giảm chuyển động (`ToggleSwitch` luôn kèm chữ Bật/Tắt) — áp ngay; Cài đặt dựng lại theo Theme mới và giữ tab đang mở |
| Thông báo (F123) | Mục "THÔNG BÁO TRONG GAME": công tắc "Hiện thông báo nhỏ khi đang chơi" (`ToggleSwitch` Bật/Tắt, **mặc định Bật**, lưu cục bộ `Settings.notification_popups`, không gửi server) + câu "Dù tắt, em vẫn xem được mọi thông báo khi bấm vào chuông." Tắt → chỉ số trên chuông và hộp thông báo, không toast ([STUDENT_GAME_UX_CORE S03 §6c](STUDENT_GAME_UX_CORE.md#s03--main-hub-hud)) |
| Thông tin | Xem lại hướng dẫn của Mây, Giấy phép (trang trong vùng nội dung có nút Quay lại, nội dung F096), phiên bản game |
| Đăng xuất (đáy cột) | Hộp xác nhận: lớp phủ lồng (thêm mực 35%), thanh tiêu đề vàng "ĐĂNG XUẤT?", "Em muốn đăng xuất khỏi EduWorld?" + "Tiến độ của em đã được lưu rồi nhé.", nút "Hủy" (phụ, icon ✕, **focus mặc định** — lựa chọn an toàn) và "Đăng xuất" (`DangerButton`, icon cửa ra); chỉ "Đăng xuất" chạy luồng đăng xuất hiện có (`endGameSession` → `gameLogout`) |

Mở lần đầu trong phiên ở tab Ngôn ngữ (tiêu đề song ngữ giúp em lỡ đổi ngôn ngữ vẫn tìm lại được); lần sau ở tab dùng gần nhất. Bàn phím/tay cầm: Lên/Xuống giữa các tab, Phải vào nội dung, Trái về cột tab; `Esc`/Back đóng hộp xác nhận trước, rồi trang Giấy phép, rồi cửa sổ. Các nhóm Hồ sơ, Điều khiển, Riêng tư, Tài khoản đầy đủ, phụ đề/TTS ở bảng trên chưa có trong client (F114-1, F114-2 [Proposed]).

**Khác biệt grade band.** `g1_3`: đủ 5 tab + Đăng xuất (các tab đều thuộc nhóm `g1_3` được thấy), tab Đồ họa chỉ có 3 nút preset có hình (ẩn bóng đổ, khử răng cưa), tab Trợ năng có cả 3 thiết lập; các mục khác do mặc định an toàn. Lựa chọn đang chọn (ngôn ngữ, preset, cỡ chữ) là `ChoiceChip` **nền vàng + chữ mực + dấu ✓ icon** (F114, thay "nền nhấn + ✓" của F096), không chỉ màu. Lớp 6–9 thấy cùng bố cục với nét viền mảnh hơn (DESIGN_SYSTEM §5.2). `g6_9`: đủ.

**Song ngữ.** `ui.settings.language` ("Ngôn ngữ / Language" — luôn song ngữ để người không đọc được ngôn ngữ hiện tại vẫn tìm thấy), `ui.settings.graphics.preset.*`, `ui.settings.graphics.autodetect` ("Tự phát hiện lại" / "Re-detect"), `ui.settings.privacy.locked_by_school` ("Do nhà trường cài đặt" / "Set by your school"), `ui.settings.account.logout_all` ("Đăng xuất trên mọi thiết bị" / "Sign out on all devices") — chuỗi dài nhất.

**Trợ năng.** Toàn bộ Cài đặt điều khiển được bằng bàn phím/gamepad; mỗi tùy chọn có mô tả ngắn.

**Hiệu năng.** Thay đổi preset áp dụng ngay (không tải lại cảnh) trừ đổi renderer (cần khởi động lại, hỏi trước). Màn Cài đặt dừng render thế giới ở render scale thấp.

**An toàn/riêng tư.** Không có mục chia sẻ thông tin liên lạc; nickname không đổi tự do (chọn từ danh sách sinh tự động theo [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md)).

## 11. Khóa dịch và endpoint đề xuất

**Tiền tố khóa dịch** (gettext `.po`, ADR-0056): `ui.inventory.*`, `ui.shop.*`, `ui.achievement.*`, `ui.collection.*`, `ui.party.*`, `ui.friends.*`, `ui.friendcode.*`, `ui.club.*`, `ui.chat.*` (gồm `ui.chat.feedback.*` ánh xạ bảng phản hồi kiểm duyệt), `ui.arena.*`, `ui.leaderboard.*`, `ui.events.*`, `ui.season.*`, `ui.settings.*`. Quy tắc: tham số dạng `{name}`; số và ngày định dạng theo locale; không ghép chuỗi bằng code.

**Endpoint/message đề xuất (chưa có trong spec, cần fork spec xác nhận):**

| Đề xuất | Màn | Lý do |
|---|---|---|
| WS `hub.instance.join {area_id, follow_user_id}` (module competition, room host hub) — **đã có trong AsyncAPI, client dùng từ F111** | Bạn bè | "Tham gia bạn" dịch chuyển tới instance của bạn; server kiểm tra phạm vi và chỗ trống, `ROOM_FULL` → vào instance thường và báo "Khu vực của bạn đã đầy". Không có REST `joinFriendInWorld` |
| `GET /api/v1/game/events`, `GET /api/v1/game/events/{event_id}`, `GET /api/v1/game/season-track` — **đã có (F077), client dùng từ F111** | Event center | Theo EVENT_SEASON_SYSTEM |
| WS `event.progress.updated` (+ `event.status.changed`, `event.community_goal.updated`, `season.track.updated`) — **đã có, client dùng từ F111** | Event center | Cập nhật mục tiêu cộng đồng/tiến độ sự kiện |
| `POST /api/v1/auth/sessions:revoke-all` | Cài đặt | Đăng xuất mọi thiết bị |

## 12. Acceptance criteria

| Mã | Tiêu chí |
|---|---|
| AC-UX-101 | Mọi màn trong tài liệu có bố cục dùng được ở 1280×720 (PC), điện thoại ngang 2400×1080 và tablet 2160×1620; không có phần tử bị cắt hoặc chồng nhau ở cả `vi` và `en` |
| AC-UX-102 | Mua vật phẩm luôn qua hộp xác nhận hiển thị giá và số dư sau mua; gửi lại cùng `Idempotency-Key` không trừ Xu hai lần |
| AC-UX-103 | Lưới tủ đồ/bộ sưu tập với 500 vật phẩm cuộn mượt ≥ 30 FPS trên thiết bị Android tầm trung tham chiếu; chỉ có một viewport 3D xem trước |
| AC-UX-104 | Party lobby cập nhật trạng thái sẵn sàng của thành viên khác trong ≤ 1 giây (mạng bình thường) mà không render nhân vật 3D trong lobby |
| AC-UX-105 | Lời mời kết bạn bị từ chối vẫn hiển thị "Đang chờ" cho người gửi đến khi hết hạn; tab Tìm bạn không có ô tìm kiếm tự do |
| AC-UX-106 | Chat: mọi tin của người khác có Tắt tiếng/Chặn/Báo cáo; phản hồi kiểm duyệt đúng bảng CHAT_SYSTEM §4.3 và không tiết lộ từ bị khớp |
| AC-UX-107 | Quick message gửi bằng `vi` hiển thị bằng `en` cho người nhận chọn tiếng Anh; tin tự do không bị dịch |
| AC-UX-108 | Trong trận, panel chat tự thu nhỏ khi câu hỏi hiển thị; trên mobile không che vùng đáp án |
| AC-UX-109 | Lớp 1–3 không thấy đếm ngược số giây trong trận và không thấy thứ hạng cá nhân |
| AC-UX-110 | Kết quả trận không hiển thị số rating thô, chỉ tier; có nút Báo cáo/Chặn đối thủ |
| AC-UX-111 | Event center không có phạt mất chuỗi; khi xong nhiệm vụ ngày hiển thị gợi ý nghỉ, không gợi ý chơi thêm |
| AC-UX-112 | Đổi ngôn ngữ trong Cài đặt áp dụng ngay cho toàn bộ UI và tên trong thế giới mà không khởi động lại; lựa chọn đồng bộ sang thiết bị khác sau khi đăng nhập |
| AC-UX-113 | Tự phát hiện đồ họa chọn preset Trung bình trên thiết bị tầm trung tham chiếu và Thấp trên thiết bị tối thiểu (ADR-0057); đổi preset áp dụng không cần tải lại cảnh |
| AC-UX-114 | Thiết lập riêng tư bị nhà trường khóa hiển thị trạng thái khóa và không gửi được giá trị nới hơn chính sách (server từ chối) |

Test case tương ứng được bổ sung vào [TEST_STRATEGY](../12-project/TEST_STRATEGY.md) bởi đợt tổng hợp (nhóm UI, kiểm thử trên thiết bị tham chiếu).
