# Student Game UX — Kinh tế, xã hội, thi đấu, sự kiện, cài đặt

> Trạng thái: Draft · Phiên bản: 0.2 · Owner: UX/UI Designer + Game Director · Cập nhật: 2026-11-26 (F096: nhóm Thông tin trong Cài đặt — Giấy phép có thông báo MIT của Godot, Xem lại hướng dẫn của Mây; lựa chọn đang chọn là chip có dấu ✓) · 2026-11-21 (ADR-0095 — WP-S: bỏ lớp: không nhóm "Bạn cùng lớp", không tab chat Lớp, hồ sơ hiện khối, mục tiêu khối thay mục tiêu lớp) · 2026-11-15
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

## 5. Party lobby

**Mục đích.** Tập hợp nhóm chơi, chọn hoạt động, sẵn sàng và bắt đầu phiên chơi chung. Nghiệp vụ: [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) (sở hữu mọi luật, giới hạn, grace).

**Điểm vào/ra.** Nút Party trên HUD (phím `P`) · "Mời vào party" từ Bạn bè/Club · chấp nhận lời mời (popup nhỏ góc phải, tự ẩn sau 10 giây, mở lại từ chuông). Ra: thu nhỏ lobby (party vẫn tồn tại, HUD hiện khung thành viên) hoặc Rời party (xác nhận).

**Bố cục Web/PC**

```text
┌────────────── Nhóm: "Nhóm Sao Băng" · Quest Party ─────────────── [Thu nhỏ] [Rời] ┐
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

Chat nhóm mở bằng nút `💬` (trượt từ phải, chiếm 45% chiều rộng, vẫn thấy nút Sẵn sàng). Tablet: bố cục hai cột như PC.

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

**Song ngữ.** Khóa giao diện `ui.chat.*` (ví dụ `ui.chat.typing` "{nickname} đang soạn tin…" / "{nickname} is typing…", `ui.chat.recalled` "Tin nhắn đã được thu hồi" / "This message was unsent", `ui.chat.feedback.pii_blocked` — chuỗi dài nhất). Quick message là nội dung cấu hình song ngữ: người gửi chọn câu theo ngôn ngữ của mình, **người nhận thấy câu đó theo ngôn ngữ của người nhận** (quick message gửi bằng `quick_message_id`). Tin nhập tự do **không dịch** (ADR-0056).

**Trợ năng.** Tin mới được đọc khi bật giọng đọc (tùy chọn, mặc định tắt); độ tương phản bong bóng tin đạt AA; có thể tăng cỡ chữ chat riêng trong Cài đặt.

**Hiệu năng máy tầm trung.** `MessageList` ảo hóa, giữ tối đa 200 tin trong bộ nhớ mỗi kênh (khớp bộ đệm server); sticker/GIF là ảnh động **giới hạn khung hình và kích thước** (GIF chuyển sang sprite sheet/WebP động từ thư viện nội bộ), chỉ phát khi trong khung nhìn, tối đa 3 ảnh động phát cùng lúc; typing/receipt gộp cập nhật 1 lần/giây; không render lại toàn danh sách khi có tin mới.

**An toàn/riêng tư.** Không bao giờ cho người gửi biết từ bị khớp. Tắt tiếng/Chặn/Báo cáo có trên mọi tin của người khác. Không có kênh với người lạ ngoài phạm vi ADR-0054.

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

**Dữ liệu & API.** REST: `createMatchmakingTicket`, `getMatchmakingTicket`, `cancelMatchmakingTicket`, `acceptMatchProposal`, `createDuel`/`acceptDuel`/`declineDuel`, `getMatch`, `listMyMatches`, `getMyRatings` (chỉ dùng tier), `getLeaderboard`, `getMyLeaderboardPosition`, `listTournaments`. WS: `matchmaking.ticket.updated`, `matchmaking.proposal.created`/`updated`/`cancelled`, `matchmaking.proposal.accept`/`decline`, `matchmaking.match.assigned`, `battle.countdown.started`, `battle.question.shown`, `battle.answer.submit`, `battle.answer.recorded`, `battle.answer.evaluated`, `battle.question.revealed`, `battle.round.ended`, `battle.participant.status_changed`, `battle.match.paused`/`resumed`, `battle.match.ended`, `battle.state.request`/`snapshot`.

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

**Dữ liệu & API.** Quest: `listQuests` (lọc daily/weekly), `acceptQuest`. Sự kiện và season track: **đề xuất** theo EVENT_SEASON_SYSTEM — ví dụ `GET /api/v1/game/events` (đang diễn ra cho học sinh), `GET /api/v1/game/events/{event_id}` (tiến độ cá nhân + mục tiêu cộng đồng), `GET /api/v1/game/season/track` (mốc + tiến độ); cập nhật tiến độ qua `quest.progress.updated` và sự kiện WS mới **đề xuất** `event.progress.updated` (thuộc fork EVT/spec xác nhận).

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

**Bố cục Web/PC**

```text
┌──────────────────────────────── Cài đặt · Settings ───────────────────────────── [X] ┐
│ ▸ Ngôn ngữ      │ Ngôn ngữ / Language:   (•) Tiếng Việt   ( ) English                  │
│ ▸ Đồ họa        │ Hiển thị song song tên riêng (VD "Vương quốc Số · Math Kingdom")  [◯] │
│ ▸ Âm thanh      │────────────────────────────────────────────────────────────────────│
│ ▸ Điều khiển    │ Đồ họa:  ( ) Thấp  (•) Trung bình  ( ) Cao     [Tự phát hiện lại]   │
│ ▸ Trợ năng      │ Độ phân giải động (giữ khung hình ổn định)                     [●] │
│ ▸ Riêng tư      │ Giới hạn khung hình: [30 ▼]   Hiển thị FPS                     [◯] │
│ ▸ Tài khoản     │ Số người chơi hiển thị quanh em: [Vừa ▼]                           │
│                 │ Đang chạy: Trung bình · renderer Mobile · 58 FPS trung bình         │
└─────────────────┴────────────────────────────────────────────────────────────────────┘
```

**Bố cục Mobile**

```text
┌ ←  Cài đặt                                                    ┐
│ Ngôn ngữ        Tiếng Việt                                ›   │
│ Đồ họa          Trung bình (tự động)                      ›   │
│ Âm thanh                                                  ›   │
│ Điều khiển      Joystick trái · Nút phải                  ›   │
│ Trợ năng                                                  ›   │
│ Riêng tư                                                  ›   │
│ Tài khoản       MèoXanh42                                 ›   │
└───────────────────────────────────────────────────────────────┘
```

Mỗi mục mở trang con toàn màn. Tablet: hai cột như PC.

**Nội dung từng nhóm.**

| Nhóm | Thiết lập | Ghi chú |
|---|---|---|
| Hồ sơ của em | Chỉ xem: nickname, trường (tên song ngữ), tỉnh/thành, khối — lấy từ PlayerBootstrap (dữ liệu SIS đồng bộ khi đăng nhập, ADR-0061); nút "Thông tin chưa đúng? Báo nhà trường" (mở hướng dẫn, không sửa được trong game) | Chỉ chính học sinh thấy; người khác không thấy trường/khối (ADR-0060, ADR-0095, [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md)); khóa dịch `ui.settings.profile.*` |
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

**Khác biệt grade band.** `g1_3`: chỉ hiện Ngôn ngữ, Âm thanh, Đồ họa (3 nút hình), Trợ năng cơ bản, Đăng xuất, Thông tin; các mục khác do mặc định an toàn. Lựa chọn đang chọn (ngôn ngữ, preset đồ họa) là `ChoiceChip` nền nhấn + dấu ✓ icon, không chỉ màu (F096). `g6_9`: đủ.

**Song ngữ.** `ui.settings.language` ("Ngôn ngữ / Language" — luôn song ngữ để người không đọc được ngôn ngữ hiện tại vẫn tìm thấy), `ui.settings.graphics.preset.*`, `ui.settings.graphics.autodetect` ("Tự phát hiện lại" / "Re-detect"), `ui.settings.privacy.locked_by_school` ("Do nhà trường cài đặt" / "Set by your school"), `ui.settings.account.logout_all` ("Đăng xuất trên mọi thiết bị" / "Sign out on all devices") — chuỗi dài nhất.

**Trợ năng.** Toàn bộ Cài đặt điều khiển được bằng bàn phím/gamepad; mỗi tùy chọn có mô tả ngắn.

**Hiệu năng.** Thay đổi preset áp dụng ngay (không tải lại cảnh) trừ đổi renderer (cần khởi động lại, hỏi trước). Màn Cài đặt dừng render thế giới ở render scale thấp.

**An toàn/riêng tư.** Không có mục chia sẻ thông tin liên lạc; nickname không đổi tự do (chọn từ danh sách sinh tự động theo [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md)).

## 11. Khóa dịch và endpoint đề xuất

**Tiền tố khóa dịch** (gettext `.po`, ADR-0056): `ui.inventory.*`, `ui.shop.*`, `ui.achievement.*`, `ui.collection.*`, `ui.party.*`, `ui.friends.*`, `ui.friendcode.*`, `ui.club.*`, `ui.chat.*` (gồm `ui.chat.feedback.*` ánh xạ bảng phản hồi kiểm duyệt), `ui.arena.*`, `ui.leaderboard.*`, `ui.events.*`, `ui.season.*`, `ui.settings.*`. Quy tắc: tham số dạng `{name}`; số và ngày định dạng theo locale; không ghép chuỗi bằng code.

**Endpoint/message đề xuất (chưa có trong spec, cần fork spec xác nhận):**

| Đề xuất | Màn | Lý do |
|---|---|---|
| WS `hub.instance.join {area_id, follow_user_id}` (module competition, room host hub) | Bạn bè | "Tham gia bạn" dịch chuyển tới instance của bạn; server kiểm tra phạm vi và chỗ trống, `ROOM_FULL` → vào instance thường và báo "Khu vực của bạn đã đầy". Không có REST `joinFriendInWorld` |
| `GET /api/v1/game/events`, `GET /api/v1/game/events/{event_id}`, `GET /api/v1/game/season/track` | Event center | Theo EVENT_SEASON_SYSTEM |
| WS `event.progress.updated` | Event center | Cập nhật mục tiêu cộng đồng/tiến độ sự kiện |
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
