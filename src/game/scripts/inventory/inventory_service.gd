class_name InventoryService
extends GameApiService
## Character catalog cache and the ownership view of the wardrobe (F070, CHARACTER_SYSTEM §5.0–§6). Reads
## `getCharacterCatalog` through ApiClient with `If-None-Match` (304 keeps the cached version, ADR-0043) and keeps it
## in RAM for the app run (platform content, no student data). The server stays authoritative for ownership and slots
## (`ITEM_NOT_OWNED`, `EQUIP_SLOT_INVALID`): this view only decides what to display. Grouping by wardrobe slot uses the
## display resources (`ItemVisual`), never a list of codes in code (R2).
##
## F069 (S08, INVENTORY_ECONOMY §2–§5): the full inventory by `kind` (`listInventory`), the Xu wallet (`getWallet`,
## WalletStore), the shop (`listShopItems`), purchases (`purchaseShopItem` with one Idempotency-Key per purchase intent
## kept on every retry until the server answers — never charged twice, F069-AC-10, SC-002), the read-only ledgers
## (`listWalletLedger`, `listInventoryHistory`, F069-AC-12) and equipping through ProfileService `updateAvatar`
## (preview follows the response, F069-AC-08). The balance only comes from the server (`balance_after`).
## Prices are Xu only; nothing here knows real money, random rewards, trading or gifts (AGENTS R19).

signal catalog_loaded(version: int)
signal inventory_changed(count: int)
signal shop_changed(count: int)
## `{shop_item_id, item_id, balance_after}` of a purchase the server accepted.
signal purchase_completed(result: Dictionary)
signal purchase_failed(code: String)

const PATH_CATALOG: String = "/game/character/catalog"
const PATH_INVENTORY: String = "/game/inventory"
const PATH_INVENTORY_HISTORY: String = "/game/inventory/history"
const PATH_WALLET: String = "/game/wallet"
const PATH_WALLET_LEDGER: String = "/game/wallet/ledger"
const PATH_SHOP: String = "/game/shop/items"
const PATH_PURCHASE: String = "/game/shop/items/%s:purchase"
const ACCESSORY_SLOTS: PackedStringArray = ["head", "face", "back", "hand"]
## Ledger page size of the "Lịch sử" tab.
const LEDGER_PAGE: int = 20
## Purchase refusals after which the wallet and the shop list are read again (F069-AC-11).
const REFRESH_CODES: PackedStringArray = [
	"INSUFFICIENT_BALANCE", "PURCHASE_LIMIT_REACHED", "ITEM_ALREADY_OWNED", "SHOP_ITEM_UNAVAILABLE", "CONFLICT",
	"NOT_FOUND",
]
## Notification categories after which the balance may have changed (Clarifications 2026-11-16, F123 messages).
const WALLET_NOTIFICATION_CATEGORIES: PackedStringArray = ["reward", "achievement"]
const MSG_NOTIFICATION_CREATED: String = "notification.item.created"

var catalog: CharacterCatalogModel = null
var etag: String = ""
var wallet_store: WalletStore = WalletStore.new()
## Inventory of the student for `items_kind` ("" = every kind), RAM only.
var items: Array[InventoryItemModel] = []
var items_kind: String = ""
var shop: Array[ShopItemModel] = []
var shop_kind: String = ""
## F070 ProfileService for `updateAvatar` (set by the ServiceLocator; null in catalog-only tests).
var profile: ProfileService = null
var realtime: GameRealtimeClient = null
## shop_item_id → Idempotency-Key of the purchase intent still waiting for a definitive answer.
var _purchase_keys: Dictionary = {}
var _loading: bool = false


func _init(rest: RestApiClient = null, realtime_client: GameRealtimeClient = null,
		telemetry_service: TelemetryService = null) -> void:
	super(rest, telemetry_service)
	realtime = realtime_client
	if realtime != null:
		realtime.register_handler("notification", handle_message)


## Coroutine: loads the catalog (revalidated with the cached ETag). A 304 counts as success with the cached copy.
func load_catalog() -> ApiResult:
	if api == null:
		return ApiResult.failure(ApiResult.CODE_NETWORK_ERROR)
	var req: ApiRequest = ApiRequest.make_get(PATH_CATALOG)
	if catalog != null and not etag.is_empty():
		req.headers["If-None-Match"] = etag
	_loading = true
	var result: ApiResult = await api.send(req)
	_loading = false
	if result.status == 304 and catalog != null:
		result.ok = true
		result.code = ""
		return result
	if not result.ok:
		return result
	var parsed: CharacterCatalogModel = CharacterCatalogModel.from_dict(result.data)
	if parsed == null:
		return ApiResult.failure(ApiResult.CODE_INVALID_RESPONSE)
	catalog = parsed
	etag = String(result.headers.get("etag", ""))
	catalog_loaded.emit(catalog.version)
	return result


func has_catalog() -> bool:
	return catalog != null


## Item codes the student may equip as far as this client knows: the catalog defaults and the loaded inventory.
func owned_item_ids() -> PackedStringArray:
	var out: PackedStringArray = []
	if catalog != null:
		out.append_array(catalog.default_item_ids)
	for item: InventoryItemModel in items:
		if not item.item_id in out:
			out.append(item.item_id)
	return out


## Owned item codes whose display slot is `slot` (`hair`, `outfit`, `head`, `face`, `back`, `hand`), catalog order.
## Codes without a visual on this client are left out (they cannot be previewed).
func items_for_slot(slot: String) -> PackedStringArray:
	var out: PackedStringArray = []
	for item_id: String in owned_item_ids():
		var visual: ItemVisual = ItemVisual.load_item(item_id)
		if visual != null and visual.slot == slot:
			out.append(item_id)
	return out


## Emotes the student may put on the wheel (catalog default emotes and owned emotes).
func emote_ids() -> PackedStringArray:
	var out: PackedStringArray = catalog.default_emote_ids.duplicate() if catalog != null else PackedStringArray()
	for item: InventoryItemModel in items:
		if item.kind == "emote" and not item.item_id in out:
			out.append(item.item_id)
	return out


## Default (free) item shown for `slot` when an equipped item has no visual (F070-AC-18).
func default_item_for_slot(slot: String) -> String:
	return ItemVisual.first_for_slot(slot, catalog.default_item_ids) if catalog != null else ""


# ---------- F069: inventory, wallet, shop ----------

## Coroutine: `listInventory?kind=` (every page, F069-AC-07). Keeps the previous list on failure.
func load_inventory(kind: String = "") -> ApiResult:
	var query: Dictionary = {"kind": kind} if not kind.is_empty() else {}
	var page: Dictionary = await send_all_pages(PATH_INVENTORY, query, "listInventory")
	var result: ApiResult = page["result"] as ApiResult
	if bool(page["ok"]):
		items = InventoryItemModel.list_from(page["items"])
		items_kind = kind
		inventory_changed.emit(items.size())
	return result


## Coroutine: `getWallet`.
func load_wallet() -> ApiResult:
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_WALLET), "getWallet")
	if result.ok:
		var parsed: WalletModel = WalletModel.from_dict(result.data)
		if parsed == null:
			return ApiResult.failure(ApiResult.CODE_INVALID_RESPONSE)
		wallet_store.set_wallet(parsed)
	return result


## Coroutine: `listShopItems?kind=` (event items appear only while their event is live — server side, F077).
func load_shop(kind: String = "") -> ApiResult:
	var query: Dictionary = {"kind": kind} if not kind.is_empty() else {}
	var page: Dictionary = await send_all_pages(PATH_SHOP, query, "listShopItems")
	var result: ApiResult = page["result"] as ApiResult
	if bool(page["ok"]):
		shop = ShopItemModel.list_from(page["items"])
		shop_kind = kind
		shop_changed.emit(shop.size())
	return result


func shop_item(shop_item_id: String) -> ShopItemModel:
	for s: ShopItemModel in shop:
		if s.shop_item_id == shop_item_id:
			return s
	return null


## Idempotency-Key of a purchase intent still waiting for an answer ("" = none). Tests and telemetry only.
func pending_purchase_key(shop_item_id: String) -> String:
	return String(_purchase_keys.get(shop_item_id, ""))


## Coroutine (F069-AC-10, AC-11): `purchaseShopItem` after the student confirmed the dialog. One Idempotency-Key per
## intent, kept when the network or the server fails (ApiClient retries with it; pressing "Mua" again re-sends it), so
## the server charges once. The balance comes from `balance_after`, the new item from `item`. Refusals are reported as
## `purchase_failed(code)` (friendly `ui.error.<CODE>` in the UI) and the wallet/shop are read again.
func purchase(item: ShopItemModel) -> ApiResult:
	if item == null:
		return ApiResult.failure(ApiResult.CODE_INVALID_RESPONSE)
	var key: String = pending_purchase_key(item.shop_item_id)
	if key.is_empty():
		key = Uuid.v7()
		_purchase_keys[item.shop_item_id] = key
	var req: ApiRequest = ApiRequest.make_post(PATH_PURCHASE % item.shop_item_id, {"expected_price": item.price})
	req.idempotency_key = key
	var result: ApiResult = await send_request(req, "purchaseShopItem")
	if result.network_error or result.is_transient():
		purchase_failed.emit(result.code if not result.code.is_empty() else ApiResult.CODE_NETWORK_ERROR)
		return result
	_purchase_keys.erase(item.shop_item_id)
	if not result.ok:
		purchase_failed.emit(result.code)
		if result.code in REFRESH_CODES:
			load_wallet()
			load_shop(shop_kind)
		return result
	var body: Dictionary = result.body()
	wallet_store.set_balance(DataRead.int_of(body, "balance_after", wallet_store.balance()))
	var bought: InventoryItemModel = InventoryItemModel.from_dict(body.get("item"))
	if bought != null and owned_item(bought.item_id) == null:
		items.append(bought)
		inventory_changed.emit(items.size())
	item.purchased_count += 1
	item.owned = item.owned or bought != null
	shop_changed.emit(shop.size())
	purchase_completed.emit({"shop_item_id": item.shop_item_id,
		"item_id": bought.item_id if bought != null else item.item_id, "balance_after": wallet_store.balance()})
	return result


func owned_item(item_id: String) -> InventoryItemModel:
	for i: InventoryItemModel in items:
		if i.item_id == item_id:
			return i
	return null


## Coroutine: one page of `listWalletLedger` → `{ok, entries: Array[LedgerEntryModel], next_cursor, code}`.
func load_wallet_ledger(cursor: String = "") -> Dictionary:
	return await _ledger_page(PATH_WALLET_LEDGER, cursor, "listWalletLedger", LedgerEntryModel.LEDGER_CURRENCY)


## Coroutine: one page of `listInventoryHistory`.
func load_item_history(cursor: String = "") -> Dictionary:
	return await _ledger_page(PATH_INVENTORY_HISTORY, cursor, "listInventoryHistory", LedgerEntryModel.LEDGER_ITEM)


## Coroutine (F069-AC-08): wears `item` through ProfileService `updateAvatar`; returns the result — the preview takes
## the server's answer (`avatar_saved`), a `422 ITEM_NOT_OWNED` / `EQUIP_SLOT_INVALID` leaves the profile unchanged.
func equip(item: InventoryItemModel) -> ApiResult:
	if profile == null or profile.avatar_config() == null or item == null:
		return ApiResult.failure(ApiResult.CODE_INVALID_RESPONSE)
	return await profile.save_avatar(equip_config(profile.avatar_config(), item))


## The avatar config with `item` worn in the place of its kind/slot (display mapping; the server checks the slot).
static func equip_config(current: AvatarConfigModel, item: InventoryItemModel) -> AvatarConfigModel:
	var next: AvatarConfigModel = current.copy()
	var visual: ItemVisual = ItemVisual.load_item(item.item_id)
	var slot: String = item.equip_slot if not item.equip_slot.is_empty() else (visual.slot if visual != null else "")
	match item.kind:
		"hair":
			next.hair_item_id = item.item_id
		"outfit":
			next.outfit_item_id = item.item_id
		"emote":
			if not item.item_id in next.emote_slots and next.emote_slots.size() < AvatarConfigModel.MAX_EMOTE_SLOTS:
				next.emote_slots.append(item.item_id)
		"pet":
			next.pet = item.item_id
		"title":
			next.title = item.item_id
		"frame":
			next.frame = item.item_id
		_:
			next.accessories[slot if slot in ACCESSORY_SLOTS else "head"] = item.item_id
	return next


## True when `item_id` is worn in the current profile (tick on the grid cell).
func is_equipped(item_id: String) -> bool:
	return profile != null and profile.avatar_config() != null and item_id in profile.avatar_config().item_ids()


## Realtime dispatcher handler (`notification.*`): a reward/achievement notice may change the balance.
func handle_message(type: String, payload: Dictionary, _envelope: Variant = null) -> void:
	if type == MSG_NOTIFICATION_CREATED and DataRead.str_of(payload, "category") in WALLET_NOTIFICATION_CATEGORIES:
		load_wallet()


## Session ended: drops the student's inventory, wallet, shop view and pending intents (F069-AC-28).
func clear() -> void:
	items.clear()
	items_kind = ""
	shop.clear()
	_purchase_keys.clear()
	wallet_store.clear()


func _ledger_page(path: String, cursor: String, operation_id: String, ledger_kind: String) -> Dictionary:
	var query: Dictionary = {"limit": LEDGER_PAGE}
	if not cursor.is_empty():
		query["cursor"] = cursor
	var result: ApiResult = await send_request(ApiRequest.make_get(path, query), operation_id)
	if not result.ok:
		return {"ok": false, "entries": [] as Array[LedgerEntryModel], "next_cursor": "", "code": result.code}
	return {"ok": true, "entries": LedgerEntryModel.list_from(result.body().get("data"), ledger_kind),
		"next_cursor": next_cursor(result.body()), "code": ""}
