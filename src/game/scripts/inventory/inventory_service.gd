class_name InventoryService
extends RefCounted
## Character catalog cache and the ownership view of the wardrobe (F070, CHARACTER_SYSTEM §5.0–§6). Reads
## `getCharacterCatalog` through ApiClient with `If-None-Match` (304 keeps the cached version, ADR-0043) and keeps it
## in RAM for the app run (platform content, no student data). Ownership in F070 = the catalog's free default items
## and emotes; the full inventory (`listInventory`, rewards, Shop) arrives with F075. The server stays authoritative
## for ownership and slots (`ITEM_NOT_OWNED`, `EQUIP_SLOT_INVALID`): this view only decides what to display.
## Grouping by wardrobe slot uses the display resources (`ItemVisual`), never a list of codes in code (R2).

signal catalog_loaded(version: int)

const PATH_CATALOG: String = "/game/character/catalog"
const ACCESSORY_SLOTS: PackedStringArray = ["head", "face", "back", "hand"]

var api: RestApiClient = null
var catalog: CharacterCatalogModel = null
var etag: String = ""
var _loading: bool = false


func _init(rest: RestApiClient = null) -> void:
	api = rest


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


## Item codes the student may equip as far as this client knows (F070: the catalog defaults).
func owned_item_ids() -> PackedStringArray:
	var out: PackedStringArray = []
	if catalog != null:
		out.append_array(catalog.default_item_ids)
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


## Emotes the student may put on the wheel (F070: the catalog default emotes).
func emote_ids() -> PackedStringArray:
	return catalog.default_emote_ids.duplicate() if catalog != null else PackedStringArray()


## Default (free) item shown for `slot` when an equipped item has no visual (F070-AC-18).
func default_item_for_slot(slot: String) -> String:
	return ItemVisual.first_for_slot(slot, catalog.default_item_ids) if catalog != null else ""
