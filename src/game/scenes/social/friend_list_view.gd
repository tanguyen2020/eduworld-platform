class_name FriendListView
extends ScrollContainer
## The friends list of the friends screen (F110-AC-19; STUDENT_GAME_UX_SOCIAL §6 `FriendList` "ảo hóa, nhóm theo
## presence"): two groups "Đang trực tuyến (n)" / "Không trực tuyến (n)" — never by class, grade or school (ADR-0095) —
## in the server order inside each group. Virtualised like VirtualItemGrid: only the rows in view plus one buffer row are
## Controls (FriendRow pool re-bound while scrolling), whatever the list size (`friend_limits` ≤ 100). UI only.

signal friend_selected(user_id: String)

const BUFFER_ROWS: int = 1

## `{kind: "header", key, count, icon}` or `{kind: "friend", friend}` in display order.
var entries: Array[Dictionary] = []
var selected_id: String = ""
var presence_unknown: bool = false
var row_height: float = 0.0
var _content: Control = null
var _pool: Array[FriendRow] = []
var _headers: Array[HBoxContainer] = []
var _offsets: Array[float] = []


func _ready() -> void:
	horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	size_flags_vertical = Control.SIZE_EXPAND_FILL
	size_flags_horizontal = Control.SIZE_EXPAND_FILL
	_content = Control.new()
	_content.name = "Content"
	_content.mouse_filter = Control.MOUSE_FILTER_PASS
	add_child(_content)
	get_v_scroll_bar().value_changed.connect(func(_v: float) -> void: _layout())
	resized.connect(_on_resized)


func set_friends(online: Array[FriendModel], offline: Array[FriendModel], unknown: bool, selected: String) -> void:
	presence_unknown = unknown
	selected_id = selected
	entries.clear()
	if not online.is_empty():
		entries.append({"kind": "header", "key": "ui.friends.group.online", "count": online.size(), "icon": "presence_online"})
		for f: FriendModel in online:
			entries.append({"kind": "friend", "friend": f})
	if not offline.is_empty():
		entries.append({"kind": "header", "key": "ui.friends.group.offline", "count": offline.size(), "icon": "wifi_off"})
		for f: FriendModel in offline:
			entries.append({"kind": "friend", "friend": f})
	_relayout()


## Friend rows that are Controls now (tests: virtualised — fewer than the friends of a long list).
func pooled_rows() -> int:
	return _pool.size()


func visible_rows() -> Array[FriendRow]:
	var out: Array[FriendRow] = []
	for r: FriendRow in _pool:
		if r.visible:
			out.append(r)
	return out


func row_for(user_id: String) -> FriendRow:
	for r: FriendRow in _pool:
		if r.visible and r.user_id == user_id:
			return r
	return null


## Header words in order (tests: Online / Offline only, never a class group).
func header_keys() -> PackedStringArray:
	var out: PackedStringArray = []
	for e: Dictionary in entries:
		if String(e["kind"]) == "header":
			out.append(String(e["key"]))
	return out


## A new width: the rows are measured again (a narrow first layout must not keep tall rows).
func _on_resized() -> void:
	row_height = 0.0
	_relayout()


func _relayout() -> void:
	if _content == null:
		return
	if row_height <= 0.0:
		row_height = UiBuild.target() * 1.4
	_offsets.clear()
	var y: float = 0.0
	var gap: float = float(UiTokens.SIZE_SPACE_8)
	for e: Dictionary in entries:
		_offsets.append(y)
		y += (_header_height() if String(e["kind"]) == "header" else row_height) + gap
	_content.custom_minimum_size = Vector2(0.0, maxf(y - gap, 0.0))
	_layout()


func _layout() -> void:
	if _content == null:
		return
	var top: float = scroll_vertical
	var bottom: float = top + maxf(size.y, row_height) + row_height * BUFFER_ROWS
	var width: float = maxf(size.x - (get_v_scroll_bar().size.x if get_v_scroll_bar().visible else 0.0), 0.0)
	var used_rows: int = 0
	var used_headers: int = 0
	var grown: bool = false
	for i: int in entries.size():
		var e: Dictionary = entries[i]
		var h: float = _header_height() if String(e["kind"]) == "header" else row_height
		if _offsets[i] + h < top - row_height * BUFFER_ROWS or _offsets[i] > bottom:
			continue
		if String(e["kind"]) == "header":
			var head: HBoxContainer = _header(used_headers)
			used_headers += 1
			(head.get_node("Icon") as TextureRect).texture = UiBuild.icon(String(e["icon"]))
			(head.get_node("Label") as Label).text = tr(String(e["key"])).format({"count": LocaleFormat.number(int(e["count"]))})
			head.position = Vector2(0.0, _offsets[i])
			head.size = Vector2(width, h)
			head.visible = true
			continue
		var row: FriendRow = _row(used_rows)
		used_rows += 1
		var f: FriendModel = e["friend"] as FriendModel
		row.bind(f, presence_unknown, f.user_id == selected_id)
		row.custom_minimum_size = Vector2.ZERO
		row.position = Vector2(0.0, _offsets[i])
		row.size = Vector2(width, row_height)
		row.visible = true
		var need: float = row.content_height()
		if width >= UiBuild.target() * 4.0 and need > row_height + 0.5:
			row_height = ceilf(need)
			grown = true
	for j: int in range(used_rows, _pool.size()):
		_pool[j].visible = false
	for j: int in range(used_headers, _headers.size()):
		_headers[j].visible = false
	if grown:
		_relayout.call_deferred()


func _row(index: int) -> FriendRow:
	while _pool.size() <= index:
		var r: FriendRow = FriendRow.new()
		r.name = "FriendRow%d" % _pool.size()
		r.chosen.connect(func(id: String) -> void:
			selected_id = id
			friend_selected.emit(id))
		_content.add_child(r)
		_pool.append(r)
	return _pool[index]


func _header(index: int) -> HBoxContainer:
	while _headers.size() <= index:
		var h: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
		h.name = "Group%d" % _headers.size()
		h.mouse_filter = Control.MOUSE_FILTER_IGNORE
		var icon: TextureRect = UiBuild.icon_rect("presence_online", 0.8)
		icon.name = "Icon"
		icon.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		h.add_child(icon)
		var l: Label = UiBuild.data_label("", &"HeadingLabel", false)
		l.name = "Label"
		l.size_flags_vertical = Control.SIZE_SHRINK_CENTER
		h.add_child(l)
		_content.add_child(h)
		_headers.append(h)
	return _headers[index]


func _header_height() -> float:
	return maxf(float(UiTokens.COMPONENT_ICON_SIZE), float(get_theme_font_size(&"font_size", &"HeadingLabel")) * 1.6)
