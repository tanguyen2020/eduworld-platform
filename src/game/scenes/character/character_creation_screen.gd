extends Control
## S02 "Tạo nhân vật của em" (STUDENT_GAME_UX_CORE S02, CHARACTER_SYSTEM §5.0; F070-AC-05, F070-AC-06, F070-AC-09).
## Renders CharacterCreationModel: StepIndicator, AvatarPreview3D (SubViewport, drag/buttons to rotate, wave on the
## preview step and when done), PresetGrid, SwatchPicker (skin tone, hair colour), OptionCarousel (hair, outfit),
## NicknameChoiceChips + "Gợi ý khác" (no text input anywhere, R16), the g1_3 voice hint (VoiceOverPlayer; a missing
## clip shows the caption only) and loading/error/"Thử lại" states. Every choice redraws the preview locally; the only
## requests are the model's (catalog, profile, suggestions, confirm). UI only: no rules, no network (AGENTS R1).
## Style from the generated Theme; touch targets from the UiTokens density of the grade band (g1_3 ≥ 64 dp).
## F097 (F096-1): a board over the sky; the preview is a large stage (sky behind the character) that turns slowly on
## every step like a turntable (paused a moment after the student turns it, off with reduced motion); presets are
## picture tiles (AvatarThumb in the preset colours), skin/hair colours are round swatches with a tick when chosen,
## other options are chips — big enough for young grades.

## Recorded voice hints (`<lang>/<key>.ogg`); none ship yet — the caption is shown instead.
const VOICE_DIR: String = "res://resources/audio/voice"
const STEP_KEYS: Dictionary = {
	&"preset": "ui.avatar.step_preset", &"customize": "ui.avatar.step_customize",
	&"hair_color": "ui.avatar.step_hair_color", &"name": "ui.avatar.step_name", &"preview": "ui.avatar.step_done",
}
const STATUS_LOADING: String = "ui.avatar.loading"
const STATUS_SAVING: String = "ui.avatar.saving"
## Preview turn speed on the preview step (rad/s) and per button press (rad); off with reduced motion.
const AUTO_TURN_SPEED: float = 0.6
## F097: slower turntable on the other steps, paused this long after the student turns the preview.
const IDLE_TURN_SPEED: float = 0.3
const MANUAL_PAUSE_MSEC: int = 3000
## F097: preset tile width in band touch targets (picture tile; its height comes from the Theme `PictureTile`).
const PRESET_TILE_WIDTH: float = 2.3
const TURN_STEP: float = PI / 6.0
const DRAG_TURN_FACTOR: float = 0.01

var model: CharacterCreationModel = null
var band: String = ""
var notice_key: String = ""
var _target: float = 0.0
var _built: bool = false
var _profile: ProfileService = null
var _manual_until_msec: int = 0

@onready var _steps: HBoxContainer = %StepIndicator
@onready var _preview: SubViewportContainer = %Preview
@onready var _rig: AvatarBuilder = %AvatarRig
@onready var _anim: AnimDriver = %AnimDriver
@onready var _preset_section: Control = %PresetSection
@onready var _customize_section: Control = %CustomizeSection
@onready var _hair_color_section: Control = %HairColorSection
@onready var _name_section: Control = %NameSection
@onready var _preview_section: Control = %PreviewSection
@onready var _preset_grid: GridContainer = %PresetGrid
@onready var _body_types: HFlowContainer = %BodyTypes
@onready var _skin_tones: HFlowContainer = %SkinTones
@onready var _hair_colors: HFlowContainer = %HairColors
@onready var _hair_name: Label = %HairName
@onready var _outfit_name: Label = %OutfitName
@onready var _chips: HFlowContainer = %NicknameChoiceChips
@onready var _voice: PanelContainer = %VoiceOver
@onready var _status: Label = %StatusLabel
@onready var _retry: Button = %Retry
@onready var _back: Button = %Back
@onready var _skip: Button = %SkipCustomize
@onready var _next: Button = %Next
@onready var _confirm: Button = %Confirm


func _ready() -> void:
	var services: ServiceLocator = AppState.get_services()
	_profile = services.profile
	band = AppState.grade_band
	_target = UiBuild.target_for(band if ThemeService.is_valid_band(band) else ThemeService.default_band())
	model = CharacterCreationModel.new(services.profile, services.inventory, band)
	model.draft_changed.connect(_on_draft_changed)
	model.step_changed.connect(_on_step_changed)
	model.state_changed.connect(_on_state_changed)
	model.nickname_selected.connect(_on_nickname_selected)
	model.notice_shown.connect(_on_notice_shown)
	model.creation_completed.connect(_on_completed)
	_profile.suggestions_updated.connect(_on_suggestions_updated)
	EventBus.locale_changed.connect(_on_locale_changed)
	_back.pressed.connect(_on_back)
	_next.pressed.connect(_on_next)
	_skip.pressed.connect(_on_skip)
	_confirm.pressed.connect(_on_confirm)
	_retry.pressed.connect(_on_retry)
	(%MoreNames as Button).pressed.connect(_on_more_names)
	(%HairPrev as Button).pressed.connect(func() -> void: model.cycle_hair(-1))
	(%HairNext as Button).pressed.connect(func() -> void: model.cycle_hair(1))
	(%OutfitPrev as Button).pressed.connect(func() -> void: model.cycle_outfit(-1))
	(%OutfitNext as Button).pressed.connect(func() -> void: model.cycle_outfit(1))
	(%RotateLeft as Button).pressed.connect(func() -> void: _turn(-TURN_STEP))
	(%RotateRight as Button).pressed.connect(func() -> void: _turn(TURN_STEP))
	var inset: float = float(UiTokens.COMPONENT_ACTIVITY_BOARD_INSET)
	var window: Control = %Window as Control
	window.offset_left = inset
	window.offset_top = inset
	window.offset_right = -inset
	window.offset_bottom = -inset
	_preview.gui_input.connect(_on_preview_input)
	_size_buttons()
	_render()
	model.start()


func _exit_tree() -> void:
	if EventBus.locale_changed.is_connected(_on_locale_changed):
		EventBus.locale_changed.disconnect(_on_locale_changed)
	if _profile != null and _profile.suggestions_updated.is_connected(_on_suggestions_updated):
		_profile.suggestions_updated.disconnect(_on_suggestions_updated)


func _process(delta: float) -> void:
	if model == null or model.state != CharacterCreationModel.State.READY or bool(Settings.reduce_motion):
		return
	if Time.get_ticks_msec() < _manual_until_msec:
		return
	var speed: float = AUTO_TURN_SPEED if model.current_step() == CharacterCreationModel.STEP_PREVIEW else IDLE_TURN_SPEED
	_rig.rotation.y += speed * delta


## Turns the preview by hand (buttons, drag) and pauses the turntable for a moment.
func _turn(radians: float) -> void:
	_rig.rotation.y += radians
	_manual_until_msec = Time.get_ticks_msec() + MANUAL_PAUSE_MSEC


# ---------- model → view ----------

func _on_state_changed(_state: int) -> void:
	if model.state == CharacterCreationModel.State.READY and not _built:
		_build_options()
	if model.state != CharacterCreationModel.State.READY:
		notice_key = ""
	_render()


func _on_step_changed(step: StringName) -> void:
	notice_key = ""
	_render()
	_present_voice()
	if step == CharacterCreationModel.STEP_PREVIEW:
		_wave()


func _on_draft_changed(config: AvatarConfigModel) -> void:
	_rig.apply(config, model.inventory.catalog)
	_mark_selection()


func _on_nickname_selected(_option_id: String) -> void:
	_mark_chips()
	_render()


func _on_suggestions_updated(_count: int) -> void:
	_build_chips()
	_render()


func _on_notice_shown(key: String) -> void:
	notice_key = key
	_render()


func _on_completed() -> void:
	_render()
	if Settings.reduce_motion or not _wave():
		_enter_world()
		return
	_anim.emote_finished.connect(func(_id: String) -> void: _enter_world(), CONNECT_ONE_SHOT)


func _enter_world() -> void:
	AuthService.complete_character_creation()


func _on_locale_changed(_locale: String) -> void:
	_relabel()
	_render()
	_present_voice()


# ---------- view → model ----------

func _on_back() -> void:
	model.back()


func _on_next() -> void:
	model.next()


func _on_skip() -> void:
	model.skip_customize()


func _on_confirm() -> void:
	model.confirm()


func _on_more_names() -> void:
	model.refresh_suggestions()


func _on_retry() -> void:
	if model.draft == null:
		model.start()
	else:
		model.dismiss_error()


func _on_preview_input(event: InputEvent) -> void:
	if event is InputEventScreenDrag:
		_turn((event as InputEventScreenDrag).relative.x * DRAG_TURN_FACTOR)
	elif event is InputEventMouseMotion and (event as InputEventMouseMotion).button_mask != 0:
		_turn((event as InputEventMouseMotion).relative.x * DRAG_TURN_FACTOR)


# ---------- building ----------

func _build_options() -> void:
	_built = true
	var catalog: CharacterCatalogModel = model.inventory.catalog
	_clear(_preset_grid)
	for entry: Dictionary in catalog.presets:
		var b: Button = _option_button(_preset_grid, String(entry["preset_code"]))
		b.pressed.connect(model.select_preset.bind(String(entry["preset_code"])))
		_make_picture_tile(b, entry.get("avatar_config") as AvatarConfigModel)
	_build_named(_body_types, catalog.body_types, model.set_body_type, false)
	_build_named(_skin_tones, catalog.skin_tones, model.set_skin_tone, true)
	_build_named(_hair_colors, catalog.hair_colors, model.set_hair_color, true)
	_build_chips()
	_relabel()
	_size_buttons()
	if model.draft != null:
		_rig.apply(model.draft, catalog)
	_mark_selection()


func _build_named(container: Container, list: Array[Dictionary], setter: Callable, swatch: bool) -> void:
	_clear(container)
	for entry: Dictionary in list:
		var code: String = String(entry["code"])
		var b: Button = _option_button(container, code)
		b.pressed.connect(setter.bind(code))
		if swatch:
			# F097: a round swatch (tick when chosen) instead of a square colour patch.
			b.add_child(ColorDot.new(ItemVisual.appearance_color(code)))


func _option_button(container: Container, code: String) -> Button:
	var b: Button = Button.new()
	b.name = code
	b.toggle_mode = true
	b.theme_type_variation = &"ChoiceChip"
	b.set_meta(&"code", code)
	b.custom_minimum_size = Vector2(_target, _target)
	container.add_child(b)
	return b


## A preset tile (F097, F096-1): the preset's picture in the tall top margin of `PictureTile`, its name below.
func _make_picture_tile(b: Button, config: AvatarConfigModel) -> void:
	b.theme_type_variation = &"PictureTile"
	b.custom_minimum_size = Vector2(_target * PRESET_TILE_WIDTH, _target)
	var thumb: AvatarThumb = AvatarThumb.new()
	b.add_child(thumb)
	thumb.set_look(config, AnswerBadge.ACCENTS[posmod(b.get_index(), AnswerBadge.ACCENTS.size())])


func _build_chips() -> void:
	_clear(_chips)
	for option: NicknameOption in _profile.suggestions:
		var chip: Button = Button.new()
		chip.name = "Chip%d" % _chips.get_child_count()
		chip.toggle_mode = true
		chip.theme_type_variation = &"ChoiceChip"
		chip.text = option.nickname
		chip.set_meta(&"code", option.option_id)
		chip.custom_minimum_size = Vector2(_target, _target)
		chip.pressed.connect(model.choose_nickname.bind(option.option_id))
		_chips.add_child(chip)
	_mark_chips()


## Texts that depend on the locale: catalog names (vi/en) and the carousel labels.
func _relabel() -> void:
	var catalog: CharacterCatalogModel = model.inventory.catalog if model != null else null
	if catalog == null:
		return
	for b: Node in _preset_grid.get_children():
		var entry: Dictionary = catalog.preset(String(b.get_meta(&"code")))
		(b as Button).text = L10n.pick(entry.get("name", {}) as Dictionary)
	for pair: Array in [[_body_types, catalog.body_types, false], [_skin_tones, catalog.skin_tones, true],
			[_hair_colors, catalog.hair_colors, true]]:
		for b: Node in (pair[0] as Container).get_children():
			var label: String = CharacterCatalogModel.name_in(pair[1] as Array[Dictionary], String(b.get_meta(&"code")))
			(b as Button).tooltip_text = label
			(b as Button).text = "" if bool(pair[2]) else label
	_label_carousels()


func _label_carousels() -> void:
	if model == null or model.draft == null:
		return
	var hair: PackedStringArray = model.inventory.items_for_slot(ItemVisual.SLOT_HAIR)
	var outfits: PackedStringArray = model.inventory.items_for_slot(ItemVisual.SLOT_OUTFIT)
	var catalog: CharacterCatalogModel = model.inventory.catalog
	_hair_name.text = catalog.item_label(model.draft.hair_item_id,
			tr("ui.avatar.option_n").format({"n": hair.find(model.draft.hair_item_id) + 1}))
	_outfit_name.text = catalog.item_label(model.draft.outfit_item_id,
			tr("ui.avatar.option_n").format({"n": outfits.find(model.draft.outfit_item_id) + 1}))


func _mark_selection() -> void:
	if model == null or model.draft == null:
		return
	_mark(_preset_grid, model.preset_code)
	_mark(_body_types, model.draft.body_type)
	_mark(_skin_tones, model.draft.skin_tone)
	_mark(_hair_colors, model.draft.hair_color)
	_label_carousels()


func _mark_chips() -> void:
	_mark(_chips, model.selected_option_id if model != null else "")


static func _mark(container: Container, code: String) -> void:
	for b: Node in container.get_children():
		(b as Button).set_pressed_no_signal(String(b.get_meta(&"code", "")) == code)


func _render() -> void:
	if model == null:
		return
	var step: StringName = model.current_step()
	var ready_state: bool = model.state == CharacterCreationModel.State.READY
	_render_steps()
	_preset_section.visible = ready_state and step == CharacterCreationModel.STEP_PRESET
	_customize_section.visible = ready_state and step == CharacterCreationModel.STEP_CUSTOMIZE
	_hair_color_section.visible = ready_state and step in [CharacterCreationModel.STEP_CUSTOMIZE,
			CharacterCreationModel.STEP_HAIR_COLOR]
	_name_section.visible = ready_state and step == CharacterCreationModel.STEP_NAME
	_preview_section.visible = ready_state and step == CharacterCreationModel.STEP_PREVIEW
	var has_steps: bool = not model.steps.is_empty()
	_back.visible = has_steps and model.step_index > 0 and model.state != CharacterCreationModel.State.DONE
	_skip.visible = ready_state and model.can_skip_customize()
	_next.visible = has_steps and not model.is_last_step()
	_next.disabled = not model.can_advance()
	_confirm.visible = has_steps and model.is_last_step()
	_confirm.disabled = not model.can_confirm()
	_retry.visible = model.state == CharacterCreationModel.State.ERROR
	match model.state:
		CharacterCreationModel.State.LOADING:
			_status.text = tr(STATUS_LOADING)
		CharacterCreationModel.State.SAVING:
			_status.text = tr(STATUS_SAVING)
		CharacterCreationModel.State.ERROR:
			_status.text = L10n.error_text(model.error_code)
		_:
			_status.text = tr(notice_key) if not notice_key.is_empty() else ""
	_status.visible = not _status.text.is_empty()


func _render_steps() -> void:
	if _steps.get_child_count() != model.steps.size():
		_clear(_steps)
		for s: StringName in model.steps:
			var label: Label = Label.new()
			label.name = String(s)
			_steps.add_child(label)
	for i: int in model.steps.size():
		var label: Label = _steps.get_child(i) as Label
		label.text = tr(String(STEP_KEYS.get(model.steps[i], "")))
		label.theme_type_variation = &"HeadingLabel" if i == model.step_index else &"MutedLabel"


## g1_3: reads the instruction of the step aloud (caption always shown; a missing clip never fails).
func _present_voice() -> void:
	if not model.is_short_flow() or model.voice_key().is_empty():
		return
	var key: String = model.voice_key()
	_voice.set("always_caption", true)
	_voice.call("present", tr(key), _voice_stream(key), true)


func _voice_stream(key: String) -> AudioStream:
	var path: String = "%s/%s/%s.ogg" % [VOICE_DIR, L10n.data_language(TranslationServer.get_locale()), key]
	return ResourceLoader.load(path) as AudioStream if ResourceLoader.exists(path) else null


## Waves with the first default emote of the catalog (the "wave" of S02); false when there is none.
func _wave() -> bool:
	var catalog: CharacterCatalogModel = model.inventory.catalog
	if catalog == null or catalog.default_emote_ids.is_empty():
		return false
	var emote: String = catalog.default_emote_ids[0]
	var visual: ItemVisual = ItemVisual.load_item(emote)
	return _anim.play_emote(emote, visual.emote_clip if visual != null else null)


func _size_buttons() -> void:
	for node: Node in find_children("*", "Button", true, false):
		var b: Button = node as Button
		# Never smaller than the band target; picture tiles keep their larger size (F097).
		b.custom_minimum_size = Vector2(maxf(b.custom_minimum_size.x, _target), maxf(b.custom_minimum_size.y, _target))


static func _clear(container: Node) -> void:
	for child: Node in container.get_children():
		container.remove_child(child)
		child.queue_free()
