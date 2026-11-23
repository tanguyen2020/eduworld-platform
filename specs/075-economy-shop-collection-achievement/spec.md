# Feature Specification: F075 — Kinh tế Xu, Shop cosmetic, inventory, collection, achievement, badge

**Feature Branch**: `075-economy-shop-collection-achievement` · **Created**: 2026-11-16 · **Status**: Done · **Module**: module `game` (+ Godot) · **Mốc**: M2 · **Phụ thuộc**: F074
**Input**: Tiền trong game "Xu" với ví không âm và sổ cái append-only, cap Xu/ngày; Shop chỉ cosmetic giá Xu, giới hạn mua, vật phẩm xác định; inventory và item ledger; quy đổi vật phẩm trùng; collection và knowledge card gắn mastery; achievement/badge có trigger, tier, ẩn, version; bút toán bù four-eyes. Không tiền thật, không loot box, không trading. FR nguồn: FR-ECO-001…005, FR-RWD-001, FR-RWD-002.

## Clarifications

### Session 2026-11-16

- Q: Định danh vật phẩm là uuid hay mã chuỗi? → A: Đổi ở Session 2026-11-17: `item_id` là mã `varchar(64)`.
- Q: Phần UI Godot ("+ Godot") thuộc feature này tới đâu? → A: F075 giao contract API/realtime cho client; màn Shop/Kho đồ/Bộ sưu tập/Thành tựu trong Godot do nhóm F làm (chưa có mã trong danh mục).
- Q: Ai kiểm tra trang bị vật phẩm? → A: Handler `updateAvatar` thuộc F070; F075 cung cấp truy vấn sở hữu và slot (`InventoryOwnershipChecker`) và test F075-AC-16 chạy qua `updateAvatar`.
- Q: Theo dõi cap Xu ở đâu? → A: Dùng chung bảng cap ngày của F074 với bucket `currency:xu`.

### Session 2026-11-17

- Q: Định danh vật phẩm? → A: `item_id` là mã ổn định `varchar(64)` (không phải UUID) ở mọi bảng (`item_definitions`, `inventory_items`, `item_ledger`, `shop_item_versions`, `collection_items`, `collection_progress`, `economy_adjustments`) và mọi contract; OpenAPI `InventoryItem`, `ShopItem`, `EconomyAdjustmentRequest` đổi theo (DATABASE_DESIGN §2.8, RB-308).

### Session 2026-11-21 (owner bỏ MFA — [Confirmed])

- Q: Owner bỏ MFA/TOTP cho đăng nhập Admin Portal với mọi tài khoản, kể cả role platform (2026-11-21, [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md)) — phần nào của feature này đổi? → A: F075-AC-27: bỏ điều kiện "thiếu MFA" và mã `403 MFA_REQUIRED`; thiếu `game.economy.adjust` hoặc token không phải `eduworld-portal` vẫn `403 PERMISSION_DENIED` + `authz.denied`. T037, plan, quickstart, contracts nhắc MFA là lịch sử. Thay thế chung: route platform (`/admin/*`, `/game-admin/*`) chỉ còn gateway nội bộ + `azp = eduworld-portal` + `user_type = staff` + role platform (guard `platform_role`, thiếu → `403 PERMISSION_DENIED` + `authz.denied`); `CrossTenantScope` giữ `platform.cross_tenant.read` + client portal + `reason` + audit; phiên `platform_staff` bị từ chối `refreshPortalToken` sau 8 giờ từ lúc nhập mật khẩu (`EW_OIDC_PLATFORM_MAX_AUTH_AGE`, `401 REFRESH_TOKEN_INVALID`).

### Session 2026-11-22 (ADR-0094 — [Confirmed])

- Q: Văn bản còn mô tả MFA/`MFA_REQUIRED`/step-up như hành vi hiện hành? → A: [Confirmed] Viết lại theo [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md), giữ nguyên mã AC: F075-AC-27, Constitution, plan. Route platform chỉ còn gateway nội bộ + `azp = eduworld-portal` + `user_type = staff` + role platform; thiếu → `403 PERMISSION_DENIED` + `authz.denied`; không còn `403 MFA_REQUIRED` hay hộp đăng nhập lại (step-up). Câu trả lời Clarifications cũ nhắc MFA là lịch sử.

### Session 2026-11-23 (ghi chú triển khai F075 — owner chốt theo lệnh "chốt hết" — [Confirmed] 2026-11-23)

- Sổ cái Xu/vật phẩm (`currency_ledger`, `item_ledger`) **chưa partition tháng**, giống `xp_ledger` của F074 (partition khi > 50M dòng); vì vậy `idempotency_key` UNIQUE riêng (không kèm `created_at`). `shop_purchases` là bảng TA (append-only) thay cho T, đếm `purchase_limit` theo `shop_item_id` (tổng `quantity` mọi version).
- `student_achievements`: **một dòng mỗi (student, achievement)** giữ `progress`, `progress_state`, `tier` (bậc cao nhất đã mở) và `achievement_version`; bản ghi mở khóa từng bậc là `reward_grants` (`trigger_kind = achievement`, `idempotency_key` = SHA-256 của (achievement, tier, student)) — nên "đúng một lần mỗi `(student_id, achievement_id, tier)`" do unique của grant bảo đảm (lệch PK `(student_id, achievement_id, tier)` của data-model). Bảng **Mới** `gamification.achievement_progress_events` (TA, unique `(student_id, achievement_id, event_id)`) để event giao lại không cộng tiến độ lần hai.
- Lỗi idempotency theo quy ước toàn nền tảng [API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md): thiếu key → `428 IDEMPOTENCY_KEY_REQUIRED`; cùng key khác body → **`409 IDEMPOTENCY_KEY_REUSED`** (AC-08 ghi 422 — middleware `pkg/httpx` dùng chung, không đổi riêng cho game). Token không phải client portal bị guard `portal_staff` trả `403 PORTAL_ACCESS_STAFF_ONLY`; thiếu role platform hoặc `game.economy.adjust` → `403 PERMISSION_DENIED`; mọi trường hợp đều ghi `authz.denied` (AC-27).
- Phần thưởng của bậc thành tựu và hoàn thành collection **không nhận `xp`** (422 `VALIDATION_FAILED`): XP chỉ đến từ reward rule (nguồn `achievement.unlocked` của catalog XP) [Confirmed]. `badge_id`, `collection_item_id` là **mã vật phẩm** (huy hiệu = vật phẩm `kind = badge`) — [ADR-0100](../../docs/adr/ADR-0100-item-id-la-ma-chuoi-trong-moi-hop-dong-vat-pham.md) đổi mọi `item_id` của OpenAPI từ `Uuid` sang `CatalogCode` (48 thay đổi oasdiff được chấp nhận trong `specs/breaking-allowlist.yaml`).
- `xu_daily_cap` đọc từ `gamification.currencies.daily_cap` (dòng `xu` do migration `f075_economy` tạo, 300); `duplicate_item_xu` và `shop_purchase_limit_default` đọc env `EW_GAME_DUPLICATE_ITEM_XU` (`common=5,…,legendary=100`), `EW_GAME_SHOP_PURCHASE_LIMIT_DEFAULT` (1) tới khi có khóa platform settings (cùng cách F074). Xu của grant `currency` nhân hệ số khối × giảm dần khi lặp **của chính award XP** (Reward Engine), làm tròn xuống, rồi mới chịu cap; Xu quy đổi vật phẩm trùng và bút toán bù **không** tính vào cap.
- Mặt hàng gắn `event_id`/`season_id`: chưa có event run (F077) nên `LiveOps` trả "không chạy" (fail closed — `economysql.NoLiveOps`, không phải mock): không liệt kê, mua → `409 SHOP_ITEM_UNAVAILABLE`. F077 thay adapter.
- Thành tựu `hidden_until_unlocked` chưa mở **không xuất hiện** trong `listAchievements` (không tên, điều kiện, tiến độ); sau khi mở hiện đủ. Ngưỡng `distinct_count` ≤ 300, `distinct_days_in_week` ≤ 7 (giá trị đã đếm nằm trong `progress_state` ≤ 4096 byte) [Confirmed].
- Tiến độ collection cập nhật khi vật phẩm **vào inventory sau khi** collection được publish (không quét lại inventory cũ) [Confirmed]; thu hồi vật phẩm bằng bút toán bù không trừ tiến độ collection.
- Vật phẩm publish qua lõi phát hành F071 (item type `item`, `item.schema.json`, bảng F070 thêm cột phát hành, `stackable`, `max_stack`); vật phẩm mặc định của F070 vẫn qua `CatalogPublish`. Reward rule và level reward đặt vật phẩm/huy hiệu phải trỏ vật phẩm đã publish (`422 ITEM_REF_INVALID`); level reward `currency` chỉ `xu` và `amount ≥ 1`.
- Consumer achievement-evaluator: ba durable theo stream — `game.achievement_evaluator` (game), `game.achievement_evaluator_learning`, `game.achievement_evaluator_competition`; học sinh của event lấy từ `student_id`, `member_ids[]` hoặc `results[].student_id`. Knowledge card: consumer `game.collection_on_mastery` (`learning_objective.progressed`), grant `knowledge_card` duy nhất mỗi (student, objective).
- `item.granted` mang `item_id`, `item_kind`, `quantity`, `grant_id`, `rule_version_id` cho consumer mở khóa sticker của F118 (`ref.sticker_unlocks`, phía chat không thuộc F075). `reward.granted` (owner F074) nhận delta tương thích: `trigger_kind` thêm `achievement`, `collection`, `knowledge_card`; mục `items[]` thêm `duplicate_xu` (thông báo quy đổi trùng, AC-14).
- Seed `seed/economy`: 10 vật phẩm, 3 mặt hàng (một chỉ gán `th-demo-a`), collection knowledge card ẩn (LO là placeholder DEV), 2 thành tựu (một ẩn). Seed reward rule của F074 **không** thêm Xu (đổi rule seed làm đổi XP history các test F074); Xu từ quest đến từ rule do Game Ops tạo (F078/F079) — test tích hợp publish rule có grant `currency`.
- `listCollections` thêm `Limit`/`Cursor` + `page` (keyset theo `code` trong SQL, quy tắc phân trang của owner); `listEconomyAdjustments` (System Admin, `tenant_ids` hoặc mọi tenant qua bypass có audit) và `rejectEconomyAdjustment` (lý do `ReasonRequest`) thêm vào hợp đồng.
- Không push realtime riêng của kinh tế (thông báo trong game do F123 dựng từ event). Màn Godot ngoài phạm vi (nhóm F). Tài khoản platform thứ hai cho four-eyes local (T040): chưa sửa realm — dùng hai tài khoản `system_admin` [TBD, Câu hỏi mở F075-1].

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Ví Xu và sổ cái (Priority: P1)

Học sinh nhận Xu từ quest, challenge, achievement, lên cấp; thấy số dư và lịch sử; kiếm quá cap ngày thì không cộng thêm.

**Why this priority**: Nền cho Shop và mọi phần thưởng có giá trị; phải kiểm toán được (ADR-0049).

**Independent Test**: Áp grant `currency` 3 lần (một lần lặp event) → 2 bút toán, số dư = Σ ledger; job đối soát 0 lệch.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F075-AC-01 | KHI Reward Engine áp grant `currency`, module game PHẢI ghi bút toán Xu dương (`reason_code = reward`, `balance_after`), cập nhật số dư ví trong cùng transaction với RewardGrant và phát `currency.credited` | FR-ECO-002 · ADR-0049 | TC-ECO-002 |
| F075-AC-02 | KHI Xu kiếm trong ngày chạm `xu_daily_cap`, module game PHẢI chỉ cộng phần còn lại và áp giảm dần khi lặp nội dung như XP | FR-ECO-002 · [INVENTORY_ECONOMY §4.3](../../docs/01-game-design/INVENTORY_ECONOMY.md) | — |
| F075-AC-03 | HỆ THỐNG PHẢI không để số dư âm; job đối soát hằng đêm PHẢI phát cảnh báo khi số dư khác tổng sổ cái và không tự sửa | FR-ECO-002 · AC-ECO-003 | TC-ECO-002 |
| F075-AC-04 | KHI học sinh gọi `getWallet`/`listWalletLedger`, module game PHẢI trả số dư, Xu còn kiếm được hôm nay và lịch sử cursor có `source_label` thân thiện, không lộ id kỹ thuật, chỉ của chính học sinh | FR-ECO-002 | — |
| F075-AC-05 | HỆ THỐNG PHẢI giữ sổ cái Xu và sổ cái vật phẩm append-only: role `game_app` thực thi `UPDATE`/`DELETE` PHẢI bị từ chối | FR-ECO-002 · AGENTS R21 | TC-ECO-008 |

---

### User Story 2 — Shop cosmetic (Priority: P1)

Học sinh xem mặt hàng đang bán, thấy giá trước, xác nhận rồi mua bằng Xu; mua lặp do mạng chập chờn không bị trừ hai lần.

**Why this priority**: Đích dùng Xu; rủi ro tài chính ảo và dark pattern cao nhất.

**Independent Test**: Hai request mua cùng `Idempotency-Key` → một bút toán; hai request khác key khi chỉ đủ Xu cho một → một `200`, một `409 INSUFFICIENT_BALANCE`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F075-AC-06 | KHI học sinh gọi `listShopItems`, module game PHẢI chỉ trả mặt hàng đã publish, đang trong thời gian bán (mặt hàng gắn sự kiện/mùa chỉ khi sự kiện/mùa đó đang chạy), được gán cho tenant và hợp khối, kèm giá Xu, `purchase_limit`, `purchased_count`, `owned` | FR-ECO-003 · [INVENTORY_ECONOMY §5.1](../../docs/01-game-design/INVENTORY_ECONOMY.md) | — |
| F075-AC-07 | KHI học sinh gọi `purchaseShopItem` hợp lệ, module game PHẢI trong một transaction khóa ví, ghi bút toán Xu âm (`shop_purchase`), bút toán vật phẩm, dòng inventory, `shop_purchases`, outbox `shop.item_purchased`, `item.granted`, `currency.debited` và trả `PurchaseResult` | FR-ECO-003 | TC-ECO-001 |
| F075-AC-08 | KHI gửi lại cùng `Idempotency-Key`, module game PHẢI trả cùng response và chỉ trừ Xu một lần; NẾU thiếu key, THÌ PHẢI trả `428 PRECONDITION_REQUIRED`; cùng key khác body PHẢI trả `422 IDEMPOTENCY_KEY_REUSED`; mỗi giao dịch mua PHẢI ghi `audit.recorded` | FR-ECO-003 · AC-ECO-001 · NFR-SEC-004 | TC-ECO-001 |
| F075-AC-09 | NẾU số dư không đủ, THÌ module game PHẢI trả `409 INSUFFICIENT_BALANCE`, không đổi số dư, không ghi bút toán; hai request đồng thời chỉ đủ cho một PHẢI cho đúng một thành công | FR-ECO-002 · AC-ECO-002 | TC-ECO-001 |
| F075-AC-10 | NẾU vượt `purchase_limit` THÌ PHẢI trả `409 PURCHASE_LIMIT_REACHED`; vật phẩm không xếp chồng đã sở hữu PHẢI trả `409 ITEM_ALREADY_OWNED`; ngoài thời gian bán, không gán tenant hoặc sai khối PHẢI trả `409 SHOP_ITEM_UNAVAILABLE` | FR-ECO-003 · [INVENTORY_ECONOMY §11](../../docs/01-game-design/INVENTORY_ECONOMY.md) | — |
| F075-AC-11 | HỆ THỐNG PHẢI không có endpoint, giá hay mặt hàng mua bằng tiền thật, không hộp/phần thưởng ngẫu nhiên, không vật phẩm tăng điểm/XP/lợi thế; version `shop-items`/`items` vi phạm (tiền tệ khác `xu`, giá ≤ 0, `item_kind` ngoài danh mục, `tradable = true`) PHẢI bị từ chối `422 VALIDATION_FAILED` | FR-RWD-002 · AC-ECO-004 | TC-ECO-003 |
| F075-AC-12 | HỆ THỐNG PHẢI cho mọi route `/game/wallet*`, `/game/shop/*`, `/game/inventory*`, `/game/collections*`, `/game/achievements` chỉ qua `game_player`: token nhân sự PHẢI nhận `403 GAME_ACCESS_STUDENT_ONLY`; `X-Tenant-Id` khác tenant phiên PHẢI nhận `403 TENANT_MISMATCH`; mặt hàng không gán cho tenant PHẢI không lộ (`409 SHOP_ITEM_UNAVAILABLE`) | AGENTS R23 · NFR-SEC-006 | TC-AUTH-014 |

---

### User Story 3 — Inventory, vật phẩm trùng, trang bị (Priority: P1)

Học sinh nhận trang phục, emote, pet, khung avatar, huy hiệu vào kho; nhận lại món đã có thì được Xu; chỉ trang bị được món mình sở hữu.

**Why this priority**: FR-RWD-001; vật phẩm là phần thưởng chính của quest/sự kiện.

**Independent Test**: Grant cùng item không xếp chồng 2 lần (2 event) → 1 dòng inventory + 1 bút toán Xu quy đổi; `updateAvatar` với item chưa có → `ITEM_NOT_OWNED`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F075-AC-13 | KHI Reward Engine áp grant `item` hoặc `badge`, module game PHẢI ghi bút toán vật phẩm và inventory (tăng `quantity` với vật phẩm xếp chồng) trong cùng transaction và phát `item.granted` | FR-RWD-001 · FR-ECO-001 | — |
| F075-AC-14 | KHI học sinh nhận lại vật phẩm không xếp chồng đã sở hữu, module game PHẢI quy đổi Xu theo `duplicate_item_xu` theo độ hiếm (`reason_code = duplicate_conversion`) và thông báo trong phần thưởng | FR-ECO-004 · AC-ECO-005 | TC-ECO-004 |
| F075-AC-15 | KHI học sinh gọi `listInventory` (lọc `kind`) hoặc `listInventoryHistory`, module game PHẢI trả vật phẩm và lịch sử của chính học sinh | FR-ECO-001 | — |
| F075-AC-16 | NẾU học sinh trang bị (`updateAvatar`) vật phẩm không sở hữu THÌ PHẢI nhận `422 ITEM_NOT_OWNED`, sai slot PHẢI nhận `422 EQUIP_SLOT_INVALID`; HỆ THỐNG PHẢI không có API nào cho client thêm vật phẩm vào inventory | FR-ECO-001 · AC-ECO-008 | TC-ECO-007 |
| F075-AC-17 | HỆ THỐNG PHẢI không có API trao đổi vật phẩm giữa học sinh khi `trading_enabled` tắt; mọi vật phẩm `tradable = false` | FR-ECO-004 · [INVENTORY_ECONOMY §6.4](../../docs/01-game-design/INVENTORY_ECONOMY.md) | — |

---

### User Story 4 — Collection và knowledge card (Priority: P2)

Học sinh sưu tầm pet, thẻ kiến thức, bộ trang phục, vật sưu tầm; thấy cách nhận từng món; thẻ kiến thức nhận khi đạt mastery và không bị thu hồi.

**Why this priority**: Động lực dài hạn; knowledge card nối game với học tập.

**Independent Test**: Phát `learning_objective.progressed` lên `proficient` → card cấp; phát giảm mức → card giữ; đủ món collection → `collection.completed` + thưởng một lần.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F075-AC-18 | KHI vật phẩm thuộc collection vào inventory, module game PHẢI cập nhật tiến độ collection, phát `collection.item_added`; khi đủ món PHẢI phát `collection.completed` và cấp `completion_rewards` đúng một lần | FR-ECO-004 | — |
| F075-AC-19 | KHI nhận `learning_objective.progressed` đạt mức `proficient` trở lên, module game PHẢI cấp knowledge card của LO đó một lần; mastery giảm sau đó PHẢI không thu hồi card | FR-ECO-004 · AC-ECO-007 | TC-ECO-006 |
| F075-AC-20 | KHI học sinh gọi `listCollections`/`getCollection`, module game PHẢI ẩn collection `hidden_until_first` tới khi có món đầu tiên và trả "cách nhận" công khai cho từng món | FR-ECO-004 · [INVENTORY_ECONOMY §6.2](../../docs/01-game-design/INVENTORY_ECONOMY.md) | — |

---

### User Story 5 — Achievement và badge (Priority: P2)

Học sinh mở khóa thành tựu theo hoàn thành và mastery, có tier đồng/bạc/vàng; thành tựu ẩn không lộ điều kiện trước khi mở.

**Why this priority**: FR-ECO-005; cần trigger từ event của F071–F074.

**Independent Test**: Achievement "hoàn thành 10 quest Toán" + event lặp → mở khóa một lần; thành tựu ẩn không có `conditions` trong `listAchievements` trước khi mở.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F075-AC-21 | KHI event thuộc `trigger.event_types` của achievement đã publish đến, consumer `achievement-evaluator` PHẢI cập nhật tiến độ theo `filters`/`progress_metric`, phát `achievement.progressed`, và mở khóa đúng một lần mỗi `(student_id, achievement_id, tier)` (phát `achievement.unlocked`, cấp `rewards` qua Reward Engine) kể cả event lặp | FR-ECO-005 · AC-ECO-006 | TC-ECO-005 |
| F075-AC-22 | KHI học sinh gọi `listAchievements`, module game PHẢI không trả điều kiện, tiến độ hay tên ẩn của achievement `hidden_until_unlocked` chưa mở khóa | FR-ECO-005 | TC-ECO-005 |
| F075-AC-23 | NẾU version `achievements` có trigger ngoài danh mục hoặc điều kiện dựa trên thời gian chơi, số lần đăng nhập, số tin nhắn hay điểm học tập chính thức, THÌ module game PHẢI từ chối `422 VALIDATION_FAILED` | FR-ECO-005 · NFR-SAFE-003 | — |
| F075-AC-24 | KHI achievement đổi version, module game PHẢI không giảm tiến độ đã đạt và giữ trạng thái mở khóa của học sinh | FR-ECO-005 | — |

---

### User Story 6 — Bút toán bù có người duyệt thứ hai (Priority: P2)

System Admin sửa số dư/vật phẩm/XP sai do lỗi hoặc gian lận bằng bút toán bù có lý do; người thứ hai duyệt mới ghi sổ.

**Why this priority**: Cách duy nhất sửa sổ cái append-only (ADR-0049, ANTI_CHEAT §7).

**Independent Test**: `sa.admin` tạo điều chỉnh −50 Xu → `pending_approval`, ví chưa đổi; người tạo tự duyệt → 403; người thứ hai duyệt → bút toán `adjustment`, `economy.adjusted`, audit.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F075-AC-25 | KHI người có `game.economy.adjust` gọi `createEconomyAdjustment` với `X-Tenant-Id` = tenant đích, lý do 10–1000 ký tự và `Idempotency-Key`, module game PHẢI tạo điều chỉnh `pending_approval`, chưa ghi sổ, và ghi audit | FR-ECO-002 · ADR-0049 | — |
| F075-AC-26 | KHI người thứ hai có `game.economy.adjust` gọi `approveEconomyAdjustment`, module game PHẢI ghi bút toán bù (`reason_code = adjustment`) vào sổ Xu/vật phẩm/XP trong một transaction, phát `economy.adjusted`, ghi audit; NẾU người duyệt là người tạo THÌ PHẢI trả `403 PERMISSION_DENIED`; làm số dư âm PHẢI trả `409 INSUFFICIENT_BALANCE`; đã xử lý PHẢI trả `422 INVALID_STATE_TRANSITION` | FR-ECO-002 · [ANTI_CHEAT §7](../../docs/10-security/ANTI_CHEAT.md) | — |
| F075-AC-27 | NẾU người gọi không có `game.economy.adjust`, không có role platform hoặc token không phải `eduworld-portal`, THÌ module game PHẢI trả `403 PERMISSION_DENIED` (không MFA — ADR-0094) và ghi `authz.denied`; `reason` vượt 1000 ký tự PHẢI trả `422 VALIDATION_FAILED` | AGENTS R20 · ADR-0027 · ADR-0041 | TC-TEN-004 |

### Edge Cases

- NẾU giao dịch mua lỗi sau khi trừ Xu (không thể với một transaction), THÌ hoàn Xu bằng bút toán `refund`; không có trạng thái "trừ rồi chưa giao hàng".
- NẾU mặt hàng sự kiện hết hạn giữa lúc học sinh đang xác nhận, THÌ trả `409 SHOP_ITEM_UNAVAILABLE`, không trừ Xu.
- NẾU học sinh bị dọn (`student.cleaned_up`), THÌ ví/inventory giữ nguyên (soft-delete theo F016), khôi phục khi `student.restored`.
- NẾU hai grant cùng item đến đồng thời, THÌ đúng một dòng inventory, grant còn lại thành quy đổi Xu (khóa theo `(student_id, item_id)`).
- NẾU achievement trigger `level.reached` lặp do `curve-rollout`, THÌ không mở khóa lần hai (F075-AC-21).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-ECO-001 | Item catalog, slot trang bị, inventory, item history | [INVENTORY_ECONOMY §2–3](../../docs/01-game-design/INVENTORY_ECONOMY.md) |
| FR-ECO-002 | Xu, sổ cái append-only, ví không âm, đối soát, bút toán bù | INVENTORY_ECONOMY §4 · [ADR-0049](../../docs/adr/ADR-0049-so-cai-phan-thuong-va-tien-te-trong-game-reward-currency-led.md) |
| FR-ECO-003 | Shop cosmetic giá Xu, giới hạn mua, vật phẩm xác định | INVENTORY_ECONOMY §5 |
| FR-ECO-004 | Collection, knowledge card, quy đổi trùng, trading tắt | INVENTORY_ECONOMY §6 |
| FR-ECO-005 | Achievement: định nghĩa, trigger, tiến độ, ẩn, tier, version | INVENTORY_ECONOMY §7 |
| FR-RWD-001 | Cosmetic mở khóa bằng bài tập, quest, sự kiện, thành tựu, thi đấu (qua grant) | INVENTORY_ECONOMY §5.2 |
| FR-RWD-002 | Không trả tiền để học tốt hơn, không loot box | [GAME_DESIGN §13](../../docs/01-game-design/GAME_DESIGN.md) |

### Key Entities

- **Currency / Wallet / CurrencyLedgerEntry**: loại tiền (Xu), số dư cache theo học sinh, bút toán append-only.
- **ItemDefinition (version) / InventoryItem / ItemLedgerEntry**: vật phẩm platform có version, sở hữu của học sinh, lịch sử thay đổi sở hữu.
- **ShopItem (version) / ShopPurchase**: mặt hàng bán bằng Xu và lần mua.
- **Collection (version) / CollectionProgress**: bộ sưu tập và tiến độ.
- **Achievement (version) / StudentAchievement**: thành tựu, tier và tiến độ của học sinh.
- **EconomyAdjustment**: yêu cầu bút toán bù chờ người duyệt thứ hai.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `xu_daily_cap` | Cap Xu/ngày |
| `duplicate_item_xu` | Bảng quy đổi vật phẩm trùng theo độ hiếm |
| `shop_purchase_limit_default` | Giới hạn mua mặc định |
| `badges_displayed_max` | Số slot `badge_showcase` |
| `trading_enabled` | Luôn tắt — không có API trao đổi |
| `real_money_purchase` | Không có — test quét spec |
| `idempotency_record_ttl` | Lưu kết quả mua/điều chỉnh theo `Idempotency-Key` |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Mọi thay đổi Xu/vật phẩm qua sổ cái trong transaction có khóa ví | ADR-0049 · AGENTS R10 |
| Không pay-to-win, loot box, dark pattern | AGENTS R19 · NFR-SAFE-003 |
| Bút toán bù four-eyes, audit, chỉ role platform qua gateway nội bộ | AGENTS R20 · ADR-0027 |
| Bảng T có `tenant_id` + RLS; ledger chỉ INSERT | AGENTS R8, R21 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 1.000 cặp request mua đồng thời → 0 số dư âm, 0 lần trừ trùng.
- **SC-002**: 30 ngày staging, đối soát ví 0 lệch.
- **SC-003**: Quét spec và catalog: 0 mặt hàng/endpoint mua bằng tiền thật, 0 phần thưởng ngẫu nhiên.
- **SC-004**: Mua một mặt hàng phản hồi p95 ≤ 300 ms ở tải thử 500 lượt mua/phút.
- **SC-005**: 100% điều chỉnh số dư có người duyệt khác người tạo và có audit.

## Ngoài phạm vi

- Reward Engine, XP, applier `xp`/`title` → F074; trao Xu từ challenge → F076; vật phẩm/điểm sự kiện, shop sự kiện theo lịch chạy → F077.
- HTTP workflow `/game-admin/*` cho `items`, `currencies`, `shop-items`, `collections`, `achievements` → F078; màn Item/Shop/Achievement Builder → F079.
- Handler `updateAvatar`, preset trang phục → F070; vật sưu tầm khám phá (nguồn) → F071.
- Màn Shop/Kho đồ/Bộ sưu tập/Thành tựu trong Godot → nhóm F (chưa có mã trong danh mục).
- Loại tiền thứ hai → không có trong Release 1.0 (OQ-14 owner chốt 2026-11-20); trading → cần ADR mới.

## Assumptions

- Cap Xu/ngày dùng ngày ICT như cap XP (F074) [Confirmed].
- Achievement `category` thêm `event` khi F077 thêm trigger sự kiện (schema hiện chưa có `event`).
- Knowledge card cấp từ `learning_objective.progressed` (payload chưa có schema — owner F031) với `level` theo `MasteryLevel`.
- Quyền đọc trang `listEconomyAdjustments`/từ chối điều chỉnh chưa có trong spec (delta §6); approver tìm điều chỉnh chờ qua endpoint mới.
