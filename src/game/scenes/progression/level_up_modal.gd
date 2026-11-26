extends Control
## S07 — Level-up (F069-AC-03, AC-04; STUDENT_GAME_UX_CORE S07): a celebration card over the dimmed world with Mây,
## "LÊN CẤP {level}!" (or "Lên {count} cấp!" for a merged batch), the new title and the rewards ALREADY granted by the
## server — no "Nhận" button anywhere (AC-UX-0702). One tap outside the buttons skips the effect; it closes by itself
## after AUTO_CLOSE_SEC; focus starts on "Tuyệt!". Grade bands: `g1_3` only "Tuyệt!" with big reward pictures; `g4_5`
## adds "Trang bị ngay" (S08 on the item); `g6_9` adds the XP left and "Hành trình của tôi". Reduced motion → a static
## card. Hosted by WorldRoot above the panels (a level-up can come while S08 is open). UI only (AGENTS R1).

signal closed()
signal equip_requested(item_id: String)
signal journey_requested()

## S07 §9: closes itself after this many seconds without input.
const AUTO_CLOSE_SEC: float = 10.0
## S07 §9: the effect lasts at most this long.
const EFFECT_SEC: float = 2.5
const BANDS_EQUIP: PackedStringArray = ["g4_5", "g6_7", "g8_9"]
const BANDS_JOURNEY: PackedStringArray = ["g6_7", "g8_9"]
## Share of the screen width taken by the card (mobile portrait keeps a margin).
const CARD_WIDTH_SHARE: float = 0.62

var batch: LevelUpModel = null
var progression: ProgressionModel = null
var band: String = ""
var effect_running: bool = false
var elapsed: float = 0.0
var _tween: Tween = null
var _heading: Label = null
var _title: Label = null
var _rewards: RewardRow = null
var _xp_left: Label = null
var _ok: Button = null
var _equip: Button = null
var _journey: Button = null
var _card: PanelContainer = null
var _rewards_label: Label = null


func _ready() -> void:
	set_anchors_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	var dim: ColorRect = ColorRect.new()
	dim.name = "Dim"
	dim.color = Color(UiTokens.COLOR_TEXT_PRIMARY, 0.4)
	dim.set_anchors_preset(Control.PRESET_FULL_RECT)
	dim.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(dim)
	var center: CenterContainer = CenterContainer.new()
	center.set_anchors_preset(Control.PRESET_FULL_RECT)
	center.mouse_filter = Control.MOUSE_FILTER_IGNORE
	add_child(center)
	_card = UiBuild.panel(&"FormCard")
	_card.name = "Card"
	center.add_child(_card)
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	_card.add_child(box)
	var coach: Dictionary = UiBuild.coach(CloudyMascot.Mood.HAPPY)
	box.add_child(coach["row"] as Control)
	var bubble: VBoxContainer = coach["bubble_box"] as VBoxContainer
	_heading = UiBuild.data_label("", &"StemLabel")
	_heading.name = "Heading"
	_heading.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	bubble.add_child(_heading)
	_title = UiBuild.data_label("", &"SpeechBubbleLabel")
	_title.name = "NewTitle"
	_title.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	bubble.add_child(_title)
	_rewards_label = UiBuild.label("ui.levelup.rewards", &"MutedLabel")
	box.add_child(_rewards_label)
	_rewards = RewardRow.new()
	_rewards.name = "Rewards"
	box.add_child(_rewards)
	_xp_left = UiBuild.data_label("", &"MutedLabel")
	_xp_left.name = "XpLeft"
	box.add_child(_xp_left)
	var actions: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	actions.alignment = BoxContainer.ALIGNMENT_END
	box.add_child(actions)
	_journey = UiBuild.button("ui.levelup.journey", "flag", &"SecondaryButton")
	_journey.name = "Journey"
	_journey.pressed.connect(_on_journey)
	actions.add_child(_journey)
	_equip = UiBuild.button("ui.levelup.equip_now", "wardrobe", &"SecondaryButton")
	_equip.name = "EquipNow"
	_equip.pressed.connect(_on_equip)
	actions.add_child(_equip)
	_ok = UiBuild.button("ui.levelup.ok", "check")
	_ok.name = "Ok"
	_ok.pressed.connect(close)
	actions.add_child(_ok)
	EventBus.locale_changed.connect(_on_locale_changed)
	hide()


func _exit_tree() -> void:
	if EventBus.locale_changed.is_connected(_on_locale_changed):
		EventBus.locale_changed.disconnect(_on_locale_changed)


## Shows `level_up` (merged batch) with the current progression (XP left, g6_9).
func present(level_up: LevelUpModel, current: ProgressionModel = null, grade_band: String = "") -> void:
	batch = level_up
	progression = current
	band = grade_band if ThemeService.is_valid_band(grade_band) else UiBuild.band()
	elapsed = 0.0
	show()
	_fit_card()
	UiBuild.size_buttons(self)
	_render()
	_ok.grab_focus()
	_play_effect()
	EventBus.level_up_shown.emit(batch.to_level if batch != null else 0)


func close() -> void:
	skip_effect()
	hide()
	closed.emit()


## One tap anywhere outside the buttons ends the effect at once (AC-UX-0703).
func skip_effect() -> void:
	if _tween != null:
		_tween.kill()
		_tween = null
	effect_running = false
	if _card != null:
		_card.scale = Vector2.ONE
		_heading.modulate.a = 1.0


## Auto close (tests drive it with explicit deltas).
func tick(delta: float) -> void:
	if not visible:
		return
	elapsed += delta
	if elapsed >= AUTO_CLOSE_SEC:
		close()


func buttons_shown() -> PackedStringArray:
	var out: PackedStringArray = []
	for b: Button in [_journey, _equip, _ok]:
		if b.visible:
			out.append(String(b.name))
	return out


func heading_text() -> String:
	return _heading.text


func _process(delta: float) -> void:
	tick(delta)


func _gui_input(event: InputEvent) -> void:
	var tap: bool = (event is InputEventMouseButton and (event as InputEventMouseButton).pressed) \
			or (event is InputEventScreenTouch and (event as InputEventScreenTouch).pressed)
	if tap:
		elapsed = 0.0
		skip_effect()
		accept_event()


func _unhandled_input(event: InputEvent) -> void:
	if visible and event.is_action_pressed("ui_cancel"):
		close()
		get_viewport().set_input_as_handled()


func _render() -> void:
	if batch == null:
		return
	_heading.text = tr("ui.levelup.multi").format({"count": LocaleFormat.number(batch.levels_gained)}) \
			if batch.is_multi() else tr("ui.levelup.title").format({"level": LocaleFormat.number(batch.to_level)})
	var title: String = batch.title_text()
	_title.text = tr("ui.levelup.new_title").format({"title": title}) if not title.is_empty() else ""
	_title.visible = not title.is_empty()
	_rewards.set_rewards(batch.rewards, band == "g1_3")
	_rewards_label.visible = not batch.rewards.is_empty()
	_equip.visible = band in BANDS_EQUIP and not batch.first_item_id().is_empty()
	_journey.visible = band in BANDS_JOURNEY
	var show_xp: bool = band in BANDS_JOURNEY and progression != null and not progression.is_max_level() \
			and progression.level >= batch.to_level
	_xp_left.visible = show_xp
	if show_xp:
		_xp_left.text = tr("ui.levelup.xp_to_next").format({"xp": LocaleFormat.number(progression.xp_to_next),
			"next": LocaleFormat.number(progression.level + 1)})


## The card is wide enough for the heading on one line, never wider than the layout token (S07 §4–§5).
func _fit_card() -> void:
	var width: float = get_viewport_rect().size.x * CARD_WIDTH_SHARE
	_card.custom_minimum_size.x = clampf(width, float(UiTokens.SIZE_LAYOUT_280) * 1.5, float(UiTokens.SIZE_LAYOUT_660))


func _play_effect() -> void:
	skip_effect()
	if bool(Settings.reduce_motion) or not is_inside_tree():
		return
	effect_running = true
	_card.pivot_offset = _card.size / 2.0
	_card.scale = Vector2.ONE * 0.85
	_heading.modulate.a = 0.0
	_tween = create_tween().set_parallel(true)
	_tween.tween_property(_card, "scale", Vector2.ONE, 0.4).set_trans(Tween.TRANS_BACK).set_ease(Tween.EASE_OUT)
	_tween.tween_property(_heading, "modulate:a", 1.0, 0.6)
	_tween.chain().tween_interval(EFFECT_SEC - 0.6)
	_tween.chain().tween_callback(func() -> void: effect_running = false)


func _on_equip() -> void:
	var item_id: String = batch.first_item_id() if batch != null else ""
	close()
	equip_requested.emit(item_id)


func _on_journey() -> void:
	close()
	journey_requested.emit()


func _on_locale_changed(_locale: String) -> void:
	if visible:
		_render()


## The progression read again after the level-up (`getProgression`): the XP left to the next level appears (g6_9).
func update_progression(current: ProgressionModel) -> void:
	progression = current
	if visible:
		_render()
