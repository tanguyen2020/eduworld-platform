class_name CharacterCreationModel
extends RefCounted
## S02 "Tạo nhân vật" flow (STUDENT_GAME_UX_CORE S02, CHARACTER_SYSTEM §5.0; F070-AC-05, F070-AC-06, F070-AC-09).
## Pure flow state the CharacterCreation scene renders (AGENTS R1): steps by grade band, the draft AvatarConfig, the
## chosen nickname option and confirm.
## - `g1_3`: preset → hair colour → name, then confirm (≤ 3 steps, choices only, voice hint per step);
##   other bands: preset → customise (body type, skin tone, hair, hair colour, outfit) → name → preview, then confirm,
##   with "Bỏ qua tùy chỉnh" (preset → name).
## - Every choice edits the draft and emits `draft_changed` — no request (F070-AC-06); options come from the catalog
##   (InventoryService), never from lists in code (R2).
## - The name step uses `suggestNicknames` (+ "Gợi ý khác"); there is no free-text nickname (R16).
## - Confirm: `updateAvatar` (skipped when this draft is already saved) then `updateGameProfile` with the option id.
##   `409 NICKNAME_UNAVAILABLE` → ProfileService already fetched new names; back to the name step with a notice
##   (F070-AC-09). `422 VALIDATION_FAILED` (catalog changed meanwhile) → reload the catalog. Choices are kept on error.

signal draft_changed(config: AvatarConfigModel)
signal step_changed(step: StringName)
signal state_changed(state: int)
signal nickname_selected(option_id: String)
signal notice_shown(key: String)
signal creation_completed()

enum State { LOADING, READY, SAVING, ERROR, DONE }

const STEP_PRESET: StringName = &"preset"
const STEP_CUSTOMIZE: StringName = &"customize"
const STEP_HAIR_COLOR: StringName = &"hair_color"
const STEP_NAME: StringName = &"name"
const STEP_PREVIEW: StringName = &"preview"
## Grade bands with the short flow (S02 §10, CHARACTER_SYSTEM §5.0 "Lớp 1–3").
const SHORT_FLOW_BANDS: PackedStringArray = ["g1_3"]
## Voice hint key per step (literal keys for tools/extract_pot.gd).
const VOICE_KEYS: Dictionary = {
	&"preset": "ui.avatar.voice.preset", &"customize": "ui.avatar.voice.customize",
	&"hair_color": "ui.avatar.voice.hair_color", &"name": "ui.avatar.voice.name",
	&"preview": "ui.avatar.voice.preview",
}
const NOTICE_NICKNAME_TAKEN: String = "ui.avatar.nickname_taken"
const CODE_CATALOG_CHANGED: String = "VALIDATION_FAILED"

var profiles: ProfileService = null
var inventory: InventoryService = null
var band: String = ""
var state: State = State.LOADING
var steps: Array[StringName] = []
var step_index: int = 0
var draft: AvatarConfigModel = null
var preset_code: String = ""
var selected_option_id: String = ""
## Problem/transport code of the last failure (UI shows `ui.error.<CODE>`).
var error_code: String = ""
## False when the profile already has a nickname (only the avatar is missing): no name step, no PATCH.
var needs_nickname: bool = true
var _saved: AvatarConfigModel = null


func _init(profile_service: ProfileService, inventory_service: InventoryService, grade_band: String) -> void:
	profiles = profile_service
	inventory = inventory_service
	band = grade_band


func is_short_flow() -> bool:
	return band in SHORT_FLOW_BANDS


## Coroutine: catalog (cached/ETag), profile (if not read yet) and the first nickname suggestions. False on error.
func start() -> bool:
	_set_state(State.LOADING)
	var catalog_result: ApiResult = await inventory.load_catalog()
	if not catalog_result.ok:
		return _fail(catalog_result.code)
	if profiles.profile == null:
		var profile_result: ApiResult = await profiles.fetch_profile()
		if not profile_result.ok:
			return _fail(profile_result.code)
	if not profiles.needs_setup():
		_set_state(State.DONE)
		creation_completed.emit()
		return true
	needs_nickname = profiles.profile.nickname.is_empty()
	steps = _steps_for()
	step_index = 0
	var current: AvatarConfigModel = profiles.avatar_config()
	if current != null:
		draft = current.copy()
		_saved = current.copy()
	elif not inventory.catalog.presets.is_empty():
		_use_preset(inventory.catalog.presets[0])
	else:
		return _fail(ApiResult.CODE_INVALID_RESPONSE)
	_fill_emotes()
	if needs_nickname:
		var names: ApiResult = await profiles.suggest_nicknames()
		if not names.ok:
			return _fail(names.code)
	_set_state(State.READY)
	draft_changed.emit(draft)
	step_changed.emit(current_step())
	return true


func current_step() -> StringName:
	return steps[step_index] if step_index < steps.size() else STEP_PRESET


func is_last_step() -> bool:
	return step_index >= steps.size() - 1


func can_skip_customize() -> bool:
	return STEP_CUSTOMIZE in steps and current_step() == STEP_PRESET


## Name step needs a chosen suggestion before moving on / confirming.
func can_advance() -> bool:
	return state == State.READY and (current_step() != STEP_NAME or not selected_option_id.is_empty())


func can_confirm() -> bool:
	return is_last_step() and can_advance() and (not needs_nickname or not selected_option_id.is_empty())


func next() -> bool:
	if is_last_step() or not can_advance():
		return false
	_go(step_index + 1)
	return true


func back() -> bool:
	if step_index == 0:
		return false
	_go(step_index - 1)
	return true


## "Bỏ qua tùy chỉnh": keeps the preset and jumps to the step after customising.
func skip_customize() -> void:
	if not can_skip_customize():
		return
	_go(steps.find(STEP_CUSTOMIZE) + 1)


func voice_key() -> String:
	return String(VOICE_KEYS.get(current_step(), ""))


# ---------- choices (local only, F070-AC-06) ----------

func select_preset(code: String) -> void:
	var entry: Dictionary = inventory.catalog.preset(code) if inventory.catalog != null else {}
	if entry.is_empty():
		return
	_use_preset(entry)
	_fill_emotes()
	draft_changed.emit(draft)


func set_body_type(code: String) -> void:
	if _in_list(inventory.catalog.body_types, code):
		draft.body_type = code
		draft_changed.emit(draft)


func set_skin_tone(code: String) -> void:
	if _in_list(inventory.catalog.skin_tones, code):
		draft.skin_tone = code
		draft_changed.emit(draft)


func set_hair_color(code: String) -> void:
	if _in_list(inventory.catalog.hair_colors, code):
		draft.hair_color = code
		draft_changed.emit(draft)


func set_hair(item_id: String) -> void:
	if item_id in inventory.items_for_slot(ItemVisual.SLOT_HAIR):
		draft.hair_item_id = item_id
		draft_changed.emit(draft)


func set_outfit(item_id: String) -> void:
	if item_id in inventory.items_for_slot(ItemVisual.SLOT_OUTFIT):
		draft.outfit_item_id = item_id
		draft_changed.emit(draft)


## OptionCarousel ◀ ▶ over the hair items the student may use.
func cycle_hair(direction: int) -> void:
	set_hair(_cycled(inventory.items_for_slot(ItemVisual.SLOT_HAIR), draft.hair_item_id, direction))


func cycle_outfit(direction: int) -> void:
	set_outfit(_cycled(inventory.items_for_slot(ItemVisual.SLOT_OUTFIT), draft.outfit_item_id, direction))


func choose_nickname(option_id: String) -> void:
	for option: NicknameOption in profiles.suggestions:
		if option.option_id == option_id:
			selected_option_id = option_id
			nickname_selected.emit(option_id)
			return


## Coroutine: "Gợi ý khác" — new server suggestions; the previous choice is dropped.
func refresh_suggestions() -> bool:
	var result: ApiResult = await profiles.suggest_nicknames()
	if result.ok:
		selected_option_id = ""
		nickname_selected.emit("")
	return result.ok


# ---------- confirm ----------

## Coroutine: saves the avatar, then the nickname; true when the profile is complete.
func confirm() -> bool:
	if not can_confirm():
		return false
	_set_state(State.SAVING)
	if _saved == null or not _saved.equals(draft):
		var avatar: ApiResult = await profiles.save_avatar(draft)
		if not avatar.ok:
			if avatar.code == CODE_CATALOG_CHANGED:
				await inventory.load_catalog()
			return _fail(avatar.code)
		_saved = draft.copy()
	if needs_nickname:
		var named: ApiResult = await profiles.choose_nickname(selected_option_id)
		if not named.ok:
			if named.code == ProfileService.CODE_NICKNAME_UNAVAILABLE:
				selected_option_id = ""
				nickname_selected.emit("")
				_set_state(State.READY)
				_go(steps.find(STEP_NAME))
				notice_shown.emit(NOTICE_NICKNAME_TAKEN)
				return false
			return _fail(named.code)
	_set_state(State.DONE)
	creation_completed.emit()
	return true


## "Thử lại" after an error: back to READY with every choice kept.
func dismiss_error() -> void:
	if state == State.ERROR and draft != null:
		_set_state(State.READY)


# ---------- helpers ----------

func _steps_for() -> Array[StringName]:
	var out: Array[StringName] = []
	if is_short_flow():
		out = [STEP_PRESET, STEP_HAIR_COLOR]
		if needs_nickname:
			out.append(STEP_NAME)
		return out
	out = [STEP_PRESET, STEP_CUSTOMIZE]
	if needs_nickname:
		out.append(STEP_NAME)
	out.append(STEP_PREVIEW)
	return out


func _go(index: int) -> void:
	step_index = clampi(index, 0, steps.size() - 1)
	step_changed.emit(current_step())


func _use_preset(entry: Dictionary) -> void:
	preset_code = String(entry["preset_code"])
	draft = (entry["avatar_config"] as AvatarConfigModel).copy()


## The server fills the default emotes on creation (F070-AC-19); a preset without emotes sends the defaults too.
func _fill_emotes() -> void:
	if draft != null and draft.emote_slots.is_empty() and inventory.catalog != null:
		for code: String in inventory.catalog.default_emote_ids:
			if draft.emote_slots.size() < AvatarConfigModel.MAX_EMOTE_SLOTS:
				draft.emote_slots.append(code)


func _fail(code: String) -> bool:
	error_code = code if not code.is_empty() else ApiResult.CODE_NETWORK_ERROR
	_set_state(State.ERROR)
	return false


func _set_state(next_state: State) -> void:
	state = next_state
	if next_state != State.ERROR:
		error_code = ""
	state_changed.emit(next_state)


static func _in_list(list: Array[Dictionary], code: String) -> bool:
	return code in CharacterCatalogModel.codes_of(list)


static func _cycled(options: PackedStringArray, current: String, direction: int) -> String:
	if options.is_empty():
		return current
	var index: int = options.find(current)
	if index < 0:
		return options[0]
	return options[posmod(index + direction, options.size())]
