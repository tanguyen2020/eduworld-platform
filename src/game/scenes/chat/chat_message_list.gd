class_name ChatMessageList
extends ScrollContainer
## Virtualised chat list (F119-AC-21, TC-PERF-009; STUDENT_GAME_UX_SOCIAL §7 "MessageList ảo hóa"): bubbles exist only
## for the messages inside the visible part (+ one above and below), taken from a small pool and re-bound while
## scrolling — 500 messages never make 500 Controls. Heights are measured once a bubble is laid out at the list width
## and cached by message key (an estimate before that); the list stays pinned to the newest message unless the student
## scrolled up. A new message, an edit or a reaction redraws only what changed (no full rebuild). UI only.

signal menu_requested(message: ChatMessageModel)
## The student reached the top: older history may be loaded.
signal top_reached()

## Bubbles kept beyond the visible ones at each end.
const OVERSCAN: int = 1
## Distance from the bottom (px) still counted as "at the newest message".
const STICK_PX: float = 24.0

var entries: Array[ChatMessageModel] = []
## Bubble context: {self_id, catalog, assets, budget}.
var context: Dictionary = {}
var _content: Control = null
var _heights: Dictionary = {}
var _offsets: PackedFloat32Array = PackedFloat32Array()
var _total: float = 0.0
var _active: Dictionary = {}
var _pool: Array[ChatBubble] = []
var _width: float = 0.0
var _stick: bool = true
var _measure: bool = false
var _gap: float = 0.0
## A scroll set by the list itself (pinning to the newest message) — not the student scrolling away.
var _auto: bool = false


func _ready() -> void:
	horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	UiBuild.expand(self, true, true)
	_content = Control.new()
	_content.name = "Content"
	_content.mouse_filter = Control.MOUSE_FILTER_PASS
	add_child(_content)
	_gap = float(get_theme_constant(&"separation", &"VBoxContainer"))
	get_v_scroll_bar().value_changed.connect(_on_scrolled)
	resized.connect(relayout)


## Replaces the entries (order = display order). Keeps the view on the newest message when it was there.
func set_entries(list: Array[ChatMessageModel]) -> void:
	var was_bottom: bool = _stick
	entries = list
	# Content of a message on screen may have changed (recall, removal, a reply losing its original): redraw those.
	for bubble: Variant in _active.values():
		(bubble as ChatBubble).refresh()
	_measure = true
	relayout()
	if was_bottom:
		scroll_to_bottom()


## Redraws the bubble of `key` if it is on screen (edit, reaction, receipt state).
func refresh_key(key: String) -> void:
	for i: Variant in _active.keys():
		var b: ChatBubble = _active[i] as ChatBubble
		if b.message != null and b.message.key() == key:
			b.refresh()
			_measure = true


## Texts changed for every message (locale, density, quick catalog): heights are measured again, bubbles redrawn.
func invalidate() -> void:
	_heights.clear()
	for b: Variant in _active.values():
		(b as ChatBubble).refresh()
	_measure = true
	relayout()


func bubble_count() -> int:
	return _active.size()


func bubbles() -> Array[ChatBubble]:
	var out: Array[ChatBubble] = []
	for b: Variant in _active.values():
		out.append(b as ChatBubble)
	return out


func bubble_for(key: String) -> ChatBubble:
	for b: Variant in _active.values():
		if (b as ChatBubble).message != null and (b as ChatBubble).message.key() == key:
			return b as ChatBubble
	return null


func pool_size() -> int:
	return _active.size() + _pool.size()


func is_at_bottom() -> bool:
	return _stick


func scroll_to_bottom() -> void:
	_stick = true
	_apply_bottom.call_deferred()


## Brings the message `key` into view (a pinned message tapped).
func scroll_to_key(key: String) -> void:
	for i: int in entries.size():
		if entries[i].key() == key:
			_stick = false
			scroll_vertical = int(maxf(0.0, _offsets[i] - size.y * 0.3))
			_layout_visible()
			return


func relayout() -> void:
	if _content == null:
		return
	var width: float = maxf(size.x - float(get_v_scroll_bar().size.x if get_v_scroll_bar().visible else 0.0), 0.0)
	# F125 polish: a new width (column resized, title dropped, band / compact change) wraps every card again — measure
	# the cards on screen at that width instead of keeping the estimate.
	if not is_equal_approx(width, _width):
		_measure = true
	_width = width
	_offsets.resize(entries.size())
	var y: float = 0.0
	for i: int in entries.size():
		_offsets[i] = y
		y += _height_of(entries[i]) + _gap
	_total = maxf(y - _gap, 0.0)
	# Height only: the scroll box gives the content its own width (a width here would stop the list from shrinking).
	_content.custom_minimum_size = Vector2(0.0, _total)
	_layout_visible()


func _height_of(m: ChatMessageModel) -> float:
	var key: String = "%s@%d" % [m.key(), int(_width)]
	if _heights.has(key):
		return float(_heights[key])
	# Estimate: one text line per ~32 characters of the bubble width at the band font, plus the meta line.
	var lines: int = 1
	if m.kind == ChatChannelModel.KIND_TEXT:
		var per_line: float = maxf(8.0, _width * ChatBubble.WIDTH_SHARE / maxf(float(_font_px()) * 0.55, 1.0))
		lines = clampi(ceili(float(m.body.length()) / per_line), 1, 12)
	elif m.kind in [ChatChannelModel.KIND_STICKER, ChatChannelModel.KIND_GIF]:
		return UiBuild.target() * ChatBubble.PICTURE_TARGETS + float(_font_px()) * 3.0
	return float(_font_px()) * 1.45 * float(lines + 2) + UiBuild.target() * 0.25


func _font_px() -> int:
	return get_theme_font_size(&"font_size", &"Label")


func _layout_visible() -> void:
	if entries.is_empty():
		for i: Variant in _active.keys():
			_recycle(int(i))
		return
	var top: float = float(scroll_vertical)
	var bottom: float = top + size.y
	var first: int = _index_at(top)
	var last: int = _index_at(bottom)
	first = maxi(0, first - OVERSCAN)
	last = mini(entries.size() - 1, last + OVERSCAN)
	for i: Variant in _active.keys():
		var idx: int = int(i)
		if idx < first or idx > last or (_active[i] as ChatBubble).message != entries[idx]:
			_recycle(idx)
	for idx: int in range(first, last + 1):
		var b: ChatBubble = _active.get(idx) as ChatBubble
		if b == null:
			b = _take()
			b.bind(entries[idx], context)
			_active[idx] = b
			_measure = true
		_place(b, idx)
		var shown: bool = _offsets[idx] + b.size.y > top and _offsets[idx] < bottom
		b.update_animation(shown)


func _place(b: ChatBubble, idx: int) -> void:
	var m: ChatMessageModel = entries[idx]
	var w: float = _width if m.is_system() else _width * ChatBubble.WIDTH_SHARE
	var x: float = (_width - w) if m.is_mine(String(context.get("self_id", ""))) else 0.0
	b.position = Vector2(x, _offsets[idx])
	b.size = Vector2(w, _height_of(m))


func _index_at(y: float) -> int:
	var lo: int = 0
	var hi: int = entries.size() - 1
	while lo < hi:
		var mid: int = (lo + hi + 1) / 2
		if _offsets[mid] <= y:
			lo = mid
		else:
			hi = mid - 1
	return lo


func _take() -> ChatBubble:
	var b: ChatBubble = _pool.pop_back() if not _pool.is_empty() else null
	if b == null:
		b = ChatBubble.new()
		b.menu_requested.connect(func(m: ChatMessageModel) -> void: menu_requested.emit(m))
		# F125 polish: a card whose real height changed after it was laid out is measured again (no overlap).
		b.minimum_size_changed.connect(func() -> void: _measure = true)
		_content.add_child(b)
	b.visible = true
	return b


func _recycle(idx: int) -> void:
	var b: ChatBubble = _active.get(idx) as ChatBubble
	if b == null:
		return
	_active.erase(idx)
	b.release()
	b.visible = false
	_pool.append(b)


func _process(_delta: float) -> void:
	# The scroll bar learns a new range a frame after the content grew: keep the newest message in view meanwhile.
	if _stick and _content != null:
		var bottom: int = int(maxf(0.0, _total - size.y))
		if absi(scroll_vertical - bottom) > 1:
			_auto = true
			scroll_vertical = bottom
			_auto = false
	if not _measure:
		return
	_measure = false
	var changed: bool = false
	for i: Variant in _active.keys():
		var b: ChatBubble = _active[i] as ChatBubble
		if b.message == null:
			continue
		var real: float = b.get_combined_minimum_size().y
		var key: String = "%s@%d" % [b.message.key(), int(_width)]
		if absf(float(_heights.get(key, -1.0)) - real) > 0.5:
			_heights[key] = real
			changed = true
	if changed:
		var keep: bool = _stick
		relayout()
		if keep:
			_apply_bottom()
		return
	# F125 polish: a card sized while its old (taller) minimum still held keeps that height — size it again to its
	# measured height so it does not reach over the next card.
	for i: Variant in _active.keys():
		var b: ChatBubble = _active[i] as ChatBubble
		if b.message != null and absf(b.size.y - _height_of(b.message)) > 0.5:
			_place(b, int(i))


func _apply_bottom() -> void:
	if _content == null:
		return
	_auto = true
	scroll_vertical = int(maxf(0.0, _total - size.y))
	_auto = false
	_layout_visible()


func _on_scrolled(value: float) -> void:
	if not _auto:
		_stick = value + size.y >= _total - STICK_PX
	_layout_visible()
	if value <= 0.5 and not entries.is_empty() and _total > size.y:
		top_reached.emit()
