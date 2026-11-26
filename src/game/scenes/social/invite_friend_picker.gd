class_name InviteFriendPicker
extends Control
## "Mời bạn vào nhóm" (F102-AC-02; PARTY_SYSTEM §5, STUDENT_GAME_UX_SOCIAL §5): a window over the Party screen with the
## friends of `listFriends` as big picture tiles (2D portrait + nickname + presence in words). One tap on a tile invites
## that friend — no text field to type a name in, for every grade band (lớp 1–3 "mời bằng chạm avatar"). The list is the
## one the server sent: no group or filter by class, grade or school (ADR-0095); whether the friend may join is checked
## by the server, which answers with a friendly sentence when not (F102-AC-05). A friend already invited shows the
## chosen tile with a tick and "Đã mời", a friend already in the party "Đã ở trong nhóm". Only nicknames (AGENTS R16).
## UI only: PartyService does the work (AGENTS R1).

signal closed()

var service: PartyService = null
var _window: PanelContainer = null
var _close: Button = null
var _state: Label = null
var _retry: Button = null
var _grid: GridContainer = null
var _empty: Control = null
var _tiles: Dictionary = {}


func _init() -> void:
	name = "InviteFriendPicker"
	set_anchors_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	visible = false
	_build()


func _exit_tree() -> void:
	_unbind()


## Opens over the lobby and loads the friends (the list of the previous opening shows while it loads).
func open(party_service: PartyService) -> void:
	_unbind()
	service = party_service
	if service != null:
		service.friends_changed.connect(_render)
	show()
	UiMotion.pop_in(_window, Settings.reduce_motion)
	_close.grab_focus.call_deferred(true)
	_render()
	refresh()


func close() -> void:
	hide()
	_unbind()
	closed.emit()


## Coroutine: `listFriends` (every page).
func refresh() -> void:
	if service == null:
		return
	_set_state("ui.party.invite.loading", false)
	var result: ApiResult = await service.load_friends()
	if not visible:
		return
	_set_state("" if result.ok else "ui.party.invite.load_failed", not result.ok)
	_render()


## Number of friend tiles (tests).
func tile_count() -> int:
	return _grid.get_child_count()


func tile(student_id: String) -> Button:
	return _tiles.get(student_id) as Button


## Texts of the tiles (tests: nicknames and presence words only, no class/grade/school group).
func tile_texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for b: Node in _grid.get_children():
		out.append((b as Button).text)
	return out


## True when the picker has no field to type a name in (lớp 1–3 invite by avatar, F102-AC-02).
func has_text_field() -> bool:
	return not find_children("*", "LineEdit", true, false).is_empty() or not find_children("*", "TextEdit", true, false).is_empty()


func _unhandled_input(event: InputEvent) -> void:
	if visible and event.is_action_pressed("ui_cancel"):
		get_viewport().set_input_as_handled()
		close()


func _unbind() -> void:
	if service != null and service.friends_changed.is_connected(_render):
		service.friends_changed.disconnect(_render)


func _set_state(key: String, error: bool) -> void:
	_state.text = key
	_state.visible = not key.is_empty()
	_retry.visible = error


func _render() -> void:
	UiBuild.clear(_grid)
	_tiles.clear()
	var list: Array[PartyMemberModel] = service.friends if service != null else [] as Array[PartyMemberModel]
	var party: PartyModel = service.store.party if service != null else null
	_grid.columns = 4 if size.x <= 0.0 or size.x / maxf(size.y, 1.0) < 1.9 else 5
	for f: PartyMemberModel in list:
		var in_party: bool = party != null and party.member(f.student_id) != null
		var invited: bool = service.invited.has(f.student_id)
		var b: Button = UiBuild.button("", "", &"PictureTile")
		b.name = "Friend_%s" % f.student_id.right(12)
		b.toggle_mode = true
		b.button_pressed = in_party or invited
		b.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
		var status_key: String = "ui.party.invite.in_party" if in_party else ("ui.party.invite.invited" if invited
				else String(PartyMemberCard.PRESENCE_KEYS.get(f.presence, "ui.party.presence.offline")))
		b.text = "%s\n%s" % [f.nickname, tr(status_key)]
		b.tooltip_text = b.text
		b.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		b.custom_minimum_size = Vector2(UiBuild.target() * 2.6, 0.0)
		b.size_flags_horizontal = Control.SIZE_EXPAND_FILL
		var thumb: AvatarThumb = AvatarThumb.new()
		b.add_child(thumb)
		var look: AvatarConfigModel = f.avatar()
		thumb.set_look(look if look != null else AvatarConfigModel.from_dict(PartyMemberCard.DEFAULT_AVATAR))
		b.pressed.connect(_on_tile_pressed.bind(f.student_id))
		_grid.add_child(b)
		_tiles[f.student_id] = b
	_empty.visible = list.is_empty() and service != null and service.friends_loaded and not _retry.visible


func _on_tile_pressed(student_id: String) -> void:
	var b: Button = _tiles.get(student_id) as Button
	var party: PartyModel = service.store.party if service != null else null
	if service == null or party == null or service.invited.has(student_id) or party.member(student_id) != null:
		if b != null:
			b.set_pressed_no_signal(true)
		return
	if b != null:
		b.set_pressed_no_signal(false)
		b.disabled = true
	var result: ApiResult = await service.invite(student_id)
	if b != null and is_instance_valid(b):
		b.disabled = false
	if not result.ok:
		_render()


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
	_window.anchor_left = 0.08
	_window.anchor_top = 0.06
	_window.anchor_right = 0.92
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
	var title: Label = UiBuild.label("ui.party.invite.title", &"WindowTitleLabel", false)
	title.name = "Title"
	title.uppercase = true
	title.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	UiBuild.expand(title)
	row.add_child(title)
	_close = UiBuild.button("ui.common.close", "cross", &"DangerButton")
	_close.name = "Close"
	_close.pressed.connect(close)
	row.add_child(_close)
	var hint: Label = UiBuild.label("ui.party.invite.hint", &"MutedLabel")
	hint.name = "Hint"
	box.add_child(hint)
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
	_grid = GridContainer.new()
	_grid.name = "Friends"
	_grid.columns = 4
	UiBuild.expand(_grid)
	list.add_child(_grid)
	var coach: Dictionary = UiBuild.coach(CloudyMascot.Mood.HAPPY)
	_empty = coach["row"] as Control
	_empty.name = "Empty"
	_empty.visible = false
	(coach["bubble_box"] as VBoxContainer).add_child(UiBuild.label("ui.party.invite.empty", &"SpeechBubbleLabel"))
	(_empty.get_node("Bubble") as Control).size_flags_vertical = Control.SIZE_SHRINK_CENTER
	list.add_child(_empty)
