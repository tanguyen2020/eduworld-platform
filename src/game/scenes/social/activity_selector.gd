class_name ActivitySelector
extends Control
## "Chọn hoạt động" for the leader (F102-AC-10; PARTY_SYSTEM §5, STUDENT_GAME_UX_SOCIAL §5): the activities of
## `listPartyEligibleActivities`. An activity every member is eligible for can be chosen; one that only some members are
## eligible for shows a lock and the warning "{nickname} chưa đủ điều kiện: {reason}" and cannot be chosen (the reason is
## the activity requirement the server sent, never the grade or school of the friend — R16). Grade bands `g1_3`/`g4_5`
## pick from at most 4 big picture cards; `g6_7`/`g8_9` open a dropdown list of every activity. The choice is sent by
## PartyService (`expected_version`, server checks again); the window closes once the server took it. UI only (R1).

signal closed()

## Big picture cards of the young bands (STUDENT_GAME_UX_SOCIAL §5: 3–4 choices).
const CARD_BANDS: PackedStringArray = ["g1_3", "g4_5"]
const MAX_CARDS: int = 4

var service: PartyService = null
var band: String = ""
## Activity chosen on screen (not sent yet in the dropdown layout).
var selected: EligibleActivityModel = null
var _window: PanelContainer = null
var _close: Button = null
var _state: Label = null
var _retry: Button = null
var _cards: GridContainer = null
var _dropdown: Button = null
var _list: VBoxContainer = null
var _warnings: VBoxContainer = null
var _confirm: Button = null
var _options: Dictionary = {}


func _init() -> void:
	name = "ActivitySelector"
	set_anchors_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	visible = false
	_build()


func open(party_service: PartyService, grade_band: String) -> void:
	service = party_service
	band = grade_band
	selected = null
	show()
	UiMotion.pop_in(_window, Settings.reduce_motion)
	_close.grab_focus.call_deferred(true)
	_render()
	refresh()


func close() -> void:
	hide()
	closed.emit()


## Coroutine: `listPartyEligibleActivities`.
func refresh() -> void:
	if service == null:
		return
	_set_state("ui.party.activity.loading", false)
	var result: ApiResult = await service.load_eligible()
	if not visible:
		return
	_set_state("" if result.ok else "ui.party.activity.load_failed", not result.ok)
	_render()


func uses_cards() -> bool:
	return band in CARD_BANDS


## Option buttons on screen (cards or dropdown rows), keyed by activity ref (tests).
func option(activity_ref: String) -> Button:
	return _options.get(activity_ref) as Button


func option_count() -> int:
	return _options.size()


func warning_texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for row: Node in _warnings.get_children():
		out.append((row.find_child("Text", true, false) as Label).text)
	return out


func dropdown_button() -> Button:
	return _dropdown


func confirm_button() -> Button:
	return _confirm


## Coroutine: sends `activity` when every member is eligible; returns true once the server took it.
func choose(activity: EligibleActivityModel) -> bool:
	if activity == null or not activity.all_eligible or service == null:
		return false
	_confirm.disabled = true
	var ok: bool = await service.select_activity(activity)
	_confirm.disabled = false
	if ok and visible:
		close()
	return ok


func _unhandled_input(event: InputEvent) -> void:
	if visible and event.is_action_pressed("ui_cancel"):
		get_viewport().set_input_as_handled()
		close()


func _set_state(key: String, error: bool) -> void:
	_state.text = key
	_state.visible = not key.is_empty()
	_retry.visible = error


func _activities() -> Array[EligibleActivityModel]:
	var all: Array[EligibleActivityModel] = service.eligible if service != null else [] as Array[EligibleActivityModel]
	if not uses_cards():
		return all
	# Young bands: the eligible activities first, at most 4 cards (the server order otherwise).
	var out: Array[EligibleActivityModel] = []
	for a: EligibleActivityModel in all:
		if a.all_eligible and out.size() < MAX_CARDS:
			out.append(a)
	for a: EligibleActivityModel in all:
		if not a.all_eligible and out.size() < MAX_CARDS:
			out.append(a)
	return out


func _render() -> void:
	UiBuild.clear(_cards)
	UiBuild.clear(_list)
	UiBuild.clear(_warnings)
	_options.clear()
	var list: Array[EligibleActivityModel] = _activities()
	var cards: bool = uses_cards()
	# The dropdown starts on the activity the party plays now (when every member can still join it).
	var party: PartyModel = service.store.party if service != null else null
	if selected == null and party != null:
		for a: EligibleActivityModel in list:
			if a.activity_ref == party.activity_id and a.all_eligible:
				selected = a
	_cards.visible = cards
	_dropdown.visible = not cards and not list.is_empty()
	_list.visible = false
	_confirm.visible = not cards
	for a: EligibleActivityModel in list:
		var b: Button = _card(a) if cards else _row(a)
		b.pressed.connect(_on_option_pressed.bind(a))
		_options[a.activity_ref] = b
		if cards:
			_cards.add_child(b)
		else:
			_list.add_child(b)
		for e: Dictionary in a.ineligible():
			_warnings.add_child(_warning(a, e))
	_update_dropdown()
	UiBuild.size_buttons(self)


func _card(a: EligibleActivityModel) -> Button:
	var b: Button = UiBuild.button("", "", &"PictureTile")
	b.name = "Card_%s" % a.activity_ref.replace(".", "_")
	b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
	b.text = "%s\n%s" % [a.display_title(), _members_text(a)]
	b.tooltip_text = b.text
	b.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	b.custom_minimum_size = Vector2(UiBuild.target() * 3.0, 0.0)
	b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	# The activity the party plays now is the chosen card (yellow + tick shape, F114).
	var party: PartyModel = service.store.party if service != null else null
	b.toggle_mode = true
	b.button_pressed = party != null and party.activity_id == a.activity_ref
	var picture: TileIcon = TileIcon.new()
	picture.texture = UiBuild.icon(a.icon_name() if a.all_eligible else "lock")
	b.add_child(picture)
	return b


func _row(a: EligibleActivityModel) -> Button:
	var b: Button = UiBuild.button("", a.icon_name() if a.all_eligible else "lock", &"ChoiceChip")
	b.name = "Row_%s" % a.activity_ref.replace(".", "_")
	b.toggle_mode = true
	b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
	b.text = "%s · %s" % [a.display_title(), _members_text(a)]
	b.alignment = HORIZONTAL_ALIGNMENT_LEFT
	b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	return b


func _warning(a: EligibleActivityModel, e: Dictionary) -> Control:
	var panel: PanelContainer = UiBuild.panel(&"InlineErrorPanel")
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	panel.add_child(row)
	row.add_child(UiBuild.icon_rect("alert"))
	var nick: String = ""
	var party: PartyModel = service.store.party if service != null else null
	var m: PartyMemberModel = party.member(String(e["student_id"])) if party != null else null
	nick = m.nickname if m != null else tr("ui.party.someone")
	var text: Label = UiBuild.data_label("%s: %s" % [a.display_title(), tr("ui.party.not_eligible").format({"nickname": nick,
		"reason": EligibleActivityModel.reason_text(String(e["reason_code"]), String(e["requirement"]))})],
		&"InlineErrorLabel")
	text.name = "Text"
	UiBuild.expand(text)
	row.add_child(text)
	return panel


func _members_text(a: EligibleActivityModel) -> String:
	if a.min_members == a.max_members:
		return tr("ui.party.activity.members_exact").format({"count": a.min_members})
	return tr("ui.party.activity.members_range").format({"min": a.min_members, "max": a.max_members})


func _on_option_pressed(a: EligibleActivityModel) -> void:
	if not a.all_eligible:
		# Not every member can join: the warning below says why; nothing is chosen.
		var b: Button = _options.get(a.activity_ref) as Button
		if b != null:
			b.set_pressed_no_signal(false)
		return
	selected = a
	if uses_cards():
		choose(a)
		return
	for ref: Variant in _options.keys():
		(_options[ref] as Button).set_pressed_no_signal(String(ref) == a.activity_ref)
	UiBuild.mark_chips(_options.values())
	_list.visible = false
	_update_dropdown()


func _update_dropdown() -> void:
	_dropdown.text = selected.display_title() if selected != null else tr("ui.party.activity.pick")
	_confirm.disabled = selected == null
	# The white tick is for the blue face; the disabled (light) button shows the blue one.
	_confirm.icon = UiBuild.icon("tick" if _confirm.disabled else "check")
	for ref: Variant in _options.keys():
		if not uses_cards():
			(_options[ref] as Button).set_pressed_no_signal(selected != null and String(ref) == selected.activity_ref)
	if not uses_cards():
		UiBuild.mark_chips(_options.values())


func _on_dropdown_pressed() -> void:
	_list.visible = not _list.visible


func _on_confirm_pressed() -> void:
	choose(selected)


func _on_locale_changed(_locale: String) -> void:
	if visible:
		_render()


func _ready() -> void:
	EventBus.locale_changed.connect(_on_locale_changed)


func _build() -> void:
	add_child(UiBuild.scrim(true))
	_window = UiBuild.panel()
	_window.name = "Window"
	_window.set_anchors_preset(Control.PRESET_FULL_RECT)
	_window.anchor_left = 0.1
	_window.anchor_top = 0.06
	_window.anchor_right = 0.9
	_window.anchor_bottom = 0.94
	_window.offset_left = 0.0
	_window.offset_top = 0.0
	_window.offset_right = 0.0
	_window.offset_bottom = 0.0
	add_child(_window)
	var box: VBoxContainer = UiBuild.vbox(&"FormBox")
	_window.add_child(box)
	var bar: PanelContainer = UiBuild.panel(&"WindowTitleBar")
	bar.name = "TitleBar"
	box.add_child(bar)
	var row: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	bar.add_child(row)
	var title: Label = UiBuild.label("ui.party.activity.title", &"WindowTitleLabel", false)
	title.name = "Title"
	title.uppercase = true
	title.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(title)
	row.add_child(title)
	_close = UiBuild.button("ui.common.close", "cross", &"DangerButton")
	_close.name = "Close"
	_close.pressed.connect(close)
	row.add_child(_close)
	var tools: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	box.add_child(tools)
	_state = UiBuild.label("", &"MutedLabel")
	_state.name = "StateLabel"
	UiBuild.expand(_state)
	tools.add_child(_state)
	_retry = UiBuild.button("ui.common.retry", "rotate_right", &"SecondaryButton")
	_retry.name = "Retry"
	_retry.visible = false
	_retry.pressed.connect(refresh)
	tools.add_child(_retry)
	var scroll: ScrollContainer = ScrollContainer.new()
	scroll.name = "Scroll"
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	UiBuild.expand(scroll, true, true)
	box.add_child(scroll)
	var list: VBoxContainer = UiBuild.vbox(&"FormBox")
	UiBuild.expand(list)
	scroll.add_child(list)
	_cards = GridContainer.new()
	_cards.name = "Cards"
	_cards.columns = MAX_CARDS
	UiBuild.expand(_cards)
	list.add_child(_cards)
	_dropdown = UiBuild.button("ui.party.activity.pick", "arrow_down", &"SecondaryButton")
	_dropdown.name = "Dropdown"
	_dropdown.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
	_dropdown.alignment = HORIZONTAL_ALIGNMENT_LEFT
	_dropdown.icon_alignment = HORIZONTAL_ALIGNMENT_RIGHT
	_dropdown.pressed.connect(_on_dropdown_pressed)
	list.add_child(_dropdown)
	_list = UiBuild.vbox(&"FieldBox")
	_list.name = "Options"
	list.add_child(_list)
	_warnings = UiBuild.vbox(&"FieldBox")
	_warnings.name = "Warnings"
	list.add_child(_warnings)
	_confirm = UiBuild.button("ui.party.activity.confirm", "check")
	_confirm.name = "Confirm"
	_confirm.size_flags_horizontal = Control.SIZE_SHRINK_END
	_confirm.pressed.connect(_on_confirm_pressed)
	box.add_child(_confirm)


## The activity picture in the top margin of a `PictureTile` (like AvatarThumb), shown at once with the title below.
class TileIcon:
	extends TextureRect

	func _init() -> void:
		name = "Picture"
		mouse_filter = Control.MOUSE_FILTER_IGNORE
		expand_mode = TextureRect.EXPAND_IGNORE_SIZE
		stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED

	func _ready() -> void:
		var parent: Control = get_parent() as Control
		if parent != null:
			parent.resized.connect(_layout)
			parent.theme_changed.connect(_layout)
		_layout()

	func _layout() -> void:
		var parent: Control = get_parent() as Control
		if parent == null:
			return
		var box: StyleBox = parent.get_theme_stylebox(&"normal")
		var pad: float = box.content_margin_left if box != null else 0.0
		var top: float = box.content_margin_top if box != null else parent.size.y / 2.0
		anchor_left = 0.0
		anchor_top = 0.0
		anchor_right = 1.0
		anchor_bottom = 0.0
		offset_left = pad
		offset_top = pad
		offset_right = -pad
		offset_bottom = top - pad * 0.5
