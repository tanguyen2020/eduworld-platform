class_name WardrobeModel
extends RefCounted
## Tủ đồ / CustomizationStudio (CHARACTER_SYSTEM §5.1, §6; F070 US3, T030). Pure state the scene renders (AGENTS R1):
## tabs by grade band (`g1_3` merges outfit, hair and accessories into one "Trang phục" tab), the items the student
## may wear (InventoryService ownership view, grouped by their display slot), a draft AvatarConfig edited locally
## (body type and skin tone stay free to change), the equipped emotes (≤ `emote_wheel_slots`) and the saved outfits
## (slots 1…3). "Lưu" sends `updateAvatar`; the server decides ownership and slots and its 422 `ITEM_NOT_OWNED` /
## `EQUIP_SLOT_INVALID` are shown as `ui.error.<CODE>` with the draft kept.

signal draft_changed(config: AvatarConfigModel)
signal state_changed(state: int)
signal notice_shown(key: String)
signal wardrobe_saved(config: AvatarConfigModel)

enum State { LOADING, READY, SAVING, ERROR }

const TAB_LOOK: StringName = &"look"
const TAB_HAIR: StringName = &"hair"
const TAB_OUTFIT: StringName = &"outfit"
const TAB_ACCESSORY: StringName = &"accessory"
const TAB_CLOTHES: StringName = &"clothes"
const TAB_EMOTE: StringName = &"emote"
## Grade bands with fewer tabs (CHARACTER_SYSTEM §6: "Lớp 1–3 … gộp outfit/hair/accessory thành 'Trang phục'").
const MERGED_BANDS: PackedStringArray = ["g1_3"]
const NOTICE_EMOTES_FULL: String = "ui.wardrobe.emotes_full"
const NOTICE_SAVED: String = "ui.wardrobe.saved"
const NOTICE_PRESET_SAVED: String = "ui.wardrobe.preset_saved"

var profiles: ProfileService = null
var inventory: InventoryService = null
var band: String = ""
var state: State = State.LOADING
var draft: AvatarConfigModel = null
var error_code: String = ""
var _saved: AvatarConfigModel = null


func _init(profile_service: ProfileService, inventory_service: InventoryService, grade_band: String) -> void:
	profiles = profile_service
	inventory = inventory_service
	band = grade_band


func tabs() -> Array[StringName]:
	var out: Array[StringName] = [TAB_LOOK]
	if band in MERGED_BANDS:
		out.append(TAB_CLOTHES)
	else:
		out.append_array([TAB_HAIR, TAB_OUTFIT, TAB_ACCESSORY])
	out.append(TAB_EMOTE)
	return out


## Coroutine: catalog, profile (when not read yet) and saved outfits. False on error.
func start() -> bool:
	_set_state(State.LOADING)
	var catalog: ApiResult = await inventory.load_catalog()
	if not catalog.ok:
		return _fail(catalog.code)
	if profiles.avatar_config() == null:
		var profile: ApiResult = await profiles.fetch_profile()
		if not profile.ok:
			return _fail(profile.code)
	if profiles.avatar_config() == null:
		return _fail(ApiResult.CODE_INVALID_RESPONSE)
	var presets: ApiResult = await profiles.list_presets()
	if not presets.ok:
		return _fail(presets.code)
	draft = profiles.avatar_config().copy()
	_saved = draft.copy()
	_set_state(State.READY)
	draft_changed.emit(draft)
	return true


## Item codes listed on `tab` (catalog order); `look` lists no items (body type, skin tone, hair colour).
func items_for_tab(tab: StringName) -> PackedStringArray:
	var out: PackedStringArray = []
	match tab:
		TAB_HAIR:
			out = inventory.items_for_slot(ItemVisual.SLOT_HAIR)
		TAB_OUTFIT:
			out = inventory.items_for_slot(ItemVisual.SLOT_OUTFIT)
		TAB_ACCESSORY:
			for slot: String in AvatarConfigModel.ACCESSORY_SLOTS:
				out.append_array(inventory.items_for_slot(slot))
		TAB_CLOTHES:
			out.append_array(items_for_tab(TAB_OUTFIT))
			out.append_array(items_for_tab(TAB_HAIR))
			out.append_array(items_for_tab(TAB_ACCESSORY))
		TAB_EMOTE:
			out = inventory.emote_ids()
	return out


func is_equipped(item_id: String) -> bool:
	if draft == null:
		return false
	return item_id == draft.hair_item_id or item_id == draft.outfit_item_id \
			or item_id in draft.accessories.values() or item_id in draft.emote_slots


## Wears `item_id` on its display slot (hair/outfit replace; an accessory or emote toggles). Local only.
func equip(item_id: String) -> void:
	if draft == null or not (item_id in inventory.owned_item_ids() or item_id in inventory.emote_ids()):
		return
	var visual: ItemVisual = ItemVisual.load_item(item_id)
	if visual == null:
		return
	match visual.slot:
		ItemVisual.SLOT_HAIR:
			draft.hair_item_id = item_id
		ItemVisual.SLOT_OUTFIT:
			draft.outfit_item_id = item_id
		ItemVisual.SLOT_EMOTE:
			if not _toggle_emote(item_id):
				return
		_:
			if not visual.slot in AvatarConfigModel.ACCESSORY_SLOTS:
				return
			if String(draft.accessories.get(visual.slot, "")) == item_id:
				draft.accessories.erase(visual.slot)
			else:
				draft.accessories[visual.slot] = item_id
	draft_changed.emit(draft)


func set_body_type(code: String) -> void:
	if draft != null and code in CharacterCatalogModel.codes_of(inventory.catalog.body_types):
		draft.body_type = code
		draft_changed.emit(draft)


func set_skin_tone(code: String) -> void:
	if draft != null and code in CharacterCatalogModel.codes_of(inventory.catalog.skin_tones):
		draft.skin_tone = code
		draft_changed.emit(draft)


func set_hair_color(code: String) -> void:
	if draft != null and code in CharacterCatalogModel.codes_of(inventory.catalog.hair_colors):
		draft.hair_color = code
		draft_changed.emit(draft)


## Puts a saved outfit on the draft (local; "Lưu" equips it).
func wear_preset(slot: int) -> void:
	for p: AvatarPresetModel in profiles.presets:
		if p.slot == slot:
			draft = p.avatar_config.copy()
			draft_changed.emit(draft)
			return


func preset_in(slot: int) -> AvatarPresetModel:
	for p: AvatarPresetModel in profiles.presets:
		if p.slot == slot:
			return p
	return null


func has_changes() -> bool:
	return draft != null and not draft.equals(_saved)


func revert() -> void:
	if _saved != null:
		draft = _saved.copy()
		draft_changed.emit(draft)


## Coroutine: `updateAvatar` with the draft. On 422 the draft is kept and the code shown.
func save() -> bool:
	if draft == null:
		return false
	_set_state(State.SAVING)
	var result: ApiResult = await profiles.save_avatar(draft)
	if not result.ok:
		return _fail(result.code)
	_saved = draft.copy()
	_set_state(State.READY)
	notice_shown.emit(NOTICE_SAVED)
	wardrobe_saved.emit(_saved)
	return true


## Coroutine: `saveAvatarPreset` of the draft into `slot` (1…3).
func save_preset(slot: int) -> bool:
	if draft == null:
		return false
	_set_state(State.SAVING)
	var result: ApiResult = await profiles.save_preset(slot, draft)
	if not result.ok:
		return _fail(result.code)
	_set_state(State.READY)
	notice_shown.emit(NOTICE_PRESET_SAVED)
	return true


func dismiss_error() -> void:
	if state == State.ERROR and draft != null:
		_set_state(State.READY)


func _toggle_emote(emote_id: String) -> bool:
	var index: int = draft.emote_slots.find(emote_id)
	if index >= 0:
		draft.emote_slots.remove_at(index)
		return true
	if draft.emote_slots.size() >= AvatarConfigModel.MAX_EMOTE_SLOTS:
		notice_shown.emit(NOTICE_EMOTES_FULL)
		return false
	draft.emote_slots.append(emote_id)
	return true


func _fail(code: String) -> bool:
	error_code = code if not code.is_empty() else ApiResult.CODE_NETWORK_ERROR
	_set_state(State.ERROR)
	return false


func _set_state(next_state: State) -> void:
	state = next_state
	if next_state != State.ERROR:
		error_code = ""
	state_changed.emit(next_state)
