extends Control
## Tủ đồ — CustomizationStudio (CHARACTER_SYSTEM §6, F070 US3, T030), opened by WorldRoot as the `wardrobe` screen
## (`EventBus.ui_requested`). Renders WardrobeModel: tabs (`g1_3`: Ngoại hình · Trang phục · Emote), the items the
## student may wear, the rotating 3D preview (AvatarBuilder, local redraw), saved outfits in slots 1…3 and the
## save/revert actions. Server refusals (`ITEM_NOT_OWNED`, `EQUIP_SLOT_INVALID`, …) are shown as `ui.error.<CODE>`.
## Item names are not in the F070 catalog: items are labelled by their order in the tab (open question for F075).
## UI only: no rules, no network (AGENTS R1); generated Theme; touch targets from the density tokens.

signal closed()

const TAB_KEYS: Dictionary = {
	&"look": "ui.wardrobe.tab.look", &"hair": "ui.wardrobe.tab.hair", &"outfit": "ui.wardrobe.tab.outfit",
	&"accessory": "ui.wardrobe.tab.accessory", &"clothes": "ui.wardrobe.tab.clothes", &"emote": "ui.wardrobe.tab.emote",
}
const TURN_STEP: float = PI / 6.0
const DRAG_TURN_FACTOR: float = 0.01

var model: WardrobeModel = null
var tab: StringName = &""
var notice_key: String = ""
var _target: float = 0.0

@onready var _tabs: HBoxContainer = %Tabs
@onready var _items: HFlowContainer = %Items
@onready var _presets: GridContainer = %PresetSlots
@onready var _preview: SubViewportContainer = %Preview
@onready var _rig: AvatarBuilder = %AvatarRig
@onready var _status: Label = %StatusLabel
@onready var _retry: Button = %Retry
@onready var _save: Button = %Save
@onready var _revert: Button = %Revert
@onready var _close: Button = %Close


func _ready() -> void:
	visible = false
	UiBuild.fit_icon(%TitleIcon as TextureRect)
	_close.pressed.connect(close)
	_save.pressed.connect(func() -> void: model.save())
	_revert.pressed.connect(func() -> void: model.revert())
	_retry.pressed.connect(_on_retry)
	(%RotateLeft as Button).pressed.connect(func() -> void: _rig.rotation.y -= TURN_STEP)
	(%RotateRight as Button).pressed.connect(func() -> void: _rig.rotation.y += TURN_STEP)
	_preview.gui_input.connect(_on_preview_input)
	EventBus.locale_changed.connect(_on_locale_changed)


func _exit_tree() -> void:
	if EventBus.locale_changed.is_connected(_on_locale_changed):
		EventBus.locale_changed.disconnect(_on_locale_changed)


## WorldRoot: `ui_requested("wardrobe", {})`. Loads the catalog, profile and saved outfits.
func open(_args: Dictionary = {}) -> void:
	var services: ServiceLocator = AppState.get_services()
	var band: String = AppState.grade_band if ThemeService.is_valid_band(AppState.grade_band) \
			else ThemeService.default_band()
	_target = UiBuild.target_for(band)
	model = WardrobeModel.new(services.profile, services.inventory, band)
	model.draft_changed.connect(_on_draft_changed)
	model.state_changed.connect(_on_state_changed)
	model.notice_shown.connect(_on_notice_shown)
	tab = &""
	notice_key = ""
	# Buttons of a previous opening are bound to the previous model: rebuild them for this one.
	for container: Node in [_tabs, _items, _presets]:
		_clear(container)
	show()
	_size_buttons(self)
	_render()
	model.start()


func close() -> void:
	if model != null:
		model.revert()
	hide()
	closed.emit()


func _unhandled_input(event: InputEvent) -> void:
	if visible and (event.is_action_pressed("ui_cancel") or event.is_action_pressed("open_wardrobe")):
		close()
		get_viewport().set_input_as_handled()


# ---------- model → view ----------

func _on_state_changed(state: int) -> void:
	if state == WardrobeModel.State.READY and _tabs.get_child_count() == 0:
		_build_tabs()
		_build_presets()
		_select_tab(model.tabs()[0])
	if state != WardrobeModel.State.READY:
		notice_key = ""
	_refresh_presets()
	_render()


func _on_draft_changed(config: AvatarConfigModel) -> void:
	_rig.apply(config, model.inventory.catalog)
	_mark_items()
	_render()


func _on_notice_shown(key: String) -> void:
	notice_key = key
	_render()


func _on_locale_changed(_locale: String) -> void:
	_relabel()
	_render()


func _on_retry() -> void:
	if model.draft == null:
		model.start()
	else:
		model.dismiss_error()


func _on_preview_input(event: InputEvent) -> void:
	if event is InputEventScreenDrag:
		_rig.rotation.y += (event as InputEventScreenDrag).relative.x * DRAG_TURN_FACTOR
	elif event is InputEventMouseMotion and (event as InputEventMouseMotion).button_mask != 0:
		_rig.rotation.y += (event as InputEventMouseMotion).relative.x * DRAG_TURN_FACTOR


# ---------- building ----------

func _build_tabs() -> void:
	_clear(_tabs)
	var group: ButtonGroup = ButtonGroup.new()
	for t: StringName in model.tabs():
		var b: Button = _button(_tabs, "Tab_%s" % t, String(t))
		b.button_group = group
		b.pressed.connect(_select_tab.bind(t))
	_relabel()


func _select_tab(next: StringName) -> void:
	tab = next
	for b: Node in _tabs.get_children():
		(b as Button).set_pressed_no_signal(String(b.get_meta(&"code")) == String(next))
	_clear(_items)
	if next == WardrobeModel.TAB_LOOK:
		var catalog: CharacterCatalogModel = model.inventory.catalog
		_build_look(catalog.body_types, model.set_body_type, false)
		_build_look(catalog.skin_tones, model.set_skin_tone, true)
		_build_look(catalog.hair_colors, model.set_hair_color, true)
	else:
		for item_id: String in model.items_for_tab(next):
			var b: Button = _button(_items, "Item_%s" % item_id, item_id)
			b.toggle_mode = true
			b.pressed.connect(model.equip.bind(item_id))
	_relabel()
	_mark_items()


func _build_look(list: Array[Dictionary], setter: Callable, swatch: bool) -> void:
	for entry: Dictionary in list:
		var code: String = String(entry["code"])
		var b: Button = _button(_items, "Look_%s" % code, code)
		b.toggle_mode = true
		b.set_meta(&"look", true)
		b.pressed.connect(setter.bind(code))
		if swatch:
			# F097: the same round swatch as S02 (tick when chosen).
			b.theme_type_variation = &"ChoiceChip"
			b.add_child(ColorDot.new(ItemVisual.appearance_color(code)))
			b.set_meta(&"swatch", true)


func _build_presets() -> void:
	_clear(_presets)
	for slot: int in range(ProfileService.PRESET_SLOT_MIN, ProfileService.PRESET_SLOT_MAX + 1):
		var wear: Button = _button(_presets, "WearPreset%d" % slot, str(slot))
		wear.pressed.connect(model.wear_preset.bind(slot))
		var store: Button = _button(_presets, "SavePreset%d" % slot, str(slot))
		store.theme_type_variation = &"SecondaryButton"
		store.pressed.connect(func() -> void: model.save_preset(slot))
	_relabel()


func _button(container: Container, node_name: String, code: String) -> Button:
	var b: Button = Button.new()
	b.name = node_name
	b.set_meta(&"code", code)
	b.custom_minimum_size = Vector2(_target, _target)
	container.add_child(b)
	return b


func _relabel() -> void:
	if model == null or model.inventory.catalog == null:
		return
	for b: Node in _tabs.get_children():
		(b as Button).text = tr(String(TAB_KEYS.get(StringName(String(b.get_meta(&"code"))), "")))
	var catalog: CharacterCatalogModel = model.inventory.catalog
	var index: int = 0
	for node: Node in _items.get_children():
		var b: Button = node as Button
		var code: String = String(b.get_meta(&"code"))
		if b.has_meta(&"look"):
			var label: String = ""
			for list: Array[Dictionary] in [catalog.body_types, catalog.skin_tones, catalog.hair_colors]:
				if label.is_empty():
					label = CharacterCatalogModel.name_in(list, code)
			b.tooltip_text = label
			b.text = "" if b.has_meta(&"swatch") else label
		else:
			index += 1
			b.text = catalog.item_label(code, tr("ui.wardrobe.item_n").format({"n": index}))
			b.tooltip_text = b.text
	for slot: int in range(ProfileService.PRESET_SLOT_MIN, ProfileService.PRESET_SLOT_MAX + 1):
		var wear: Button = _presets.get_node_or_null("WearPreset%d" % slot) as Button
		var store: Button = _presets.get_node_or_null("SavePreset%d" % slot) as Button
		if wear != null:
			wear.text = tr("ui.wardrobe.wear_preset").format({"n": slot})
		if store != null:
			store.text = tr("ui.wardrobe.save_preset").format({"n": slot})


func _mark_items() -> void:
	if model == null or model.draft == null:
		return
	for node: Node in _items.get_children():
		var b: Button = node as Button
		var code: String = String(b.get_meta(&"code"))
		var on: bool = model.is_equipped(code) if not b.has_meta(&"look") \
				else code in [model.draft.body_type, model.draft.skin_tone, model.draft.hair_color]
		b.set_pressed_no_signal(on)


func _refresh_presets() -> void:
	for slot: int in range(ProfileService.PRESET_SLOT_MIN, ProfileService.PRESET_SLOT_MAX + 1):
		var wear: Button = _presets.get_node_or_null("WearPreset%d" % slot) as Button
		if wear != null:
			wear.disabled = model.preset_in(slot) == null


func _render() -> void:
	if model == null:
		return
	var busy: bool = model.state in [WardrobeModel.State.LOADING, WardrobeModel.State.SAVING]
	_save.disabled = busy or not model.has_changes()
	_revert.disabled = busy or not model.has_changes()
	_retry.visible = model.state == WardrobeModel.State.ERROR
	match model.state:
		WardrobeModel.State.LOADING:
			_status.text = tr("ui.wardrobe.loading")
		WardrobeModel.State.SAVING:
			_status.text = tr("ui.wardrobe.saving")
		WardrobeModel.State.ERROR:
			_status.text = L10n.error_text(model.error_code)
		_:
			_status.text = tr(notice_key) if not notice_key.is_empty() else ""
	_status.visible = not _status.text.is_empty()


func _size_buttons(root: Node) -> void:
	for node: Node in root.find_children("*", "Button", true, false):
		(node as Button).custom_minimum_size = Vector2(_target, _target)


static func _clear(container: Node) -> void:
	for child: Node in container.get_children():
		container.remove_child(child)
		child.queue_free()
