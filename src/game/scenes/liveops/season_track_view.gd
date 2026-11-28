class_name SeasonTrackView
extends ScrollContainer
## The free season strip (F111-AC-16, AC-17, AC-32; EVENT_SEASON_SYSTEM §7.2, NFR-COMP-001): one tile per tier in a
## horizontal scroller, VIRTUALISED — only the tiles in view plus a small margin exist as nodes (a pool re-bound while
## scrolling), so 30–100 tiers cost the same as a few. A tile: "Cấp n", its rewards in words, "Nhận" on a reached tier
## not yet claimed, "Đã nhận" / a lock otherwise (icon AND words); milestone tiers (10/20/30) on the highlighted card
## with "Cấp mốc". No price, no premium lane, no "buy a tier" (ADR-0049). UI only: `claim_requested` (AGENTS R1).

signal claim_requested(tier: int)

## Tile width in touch-target units and the extra tiles kept on each side of the view.
const TILE_WIDTH: float = 2.8
const MARGIN_TILES: int = 1

var _strip: Control = null
var _tiers: Array[SeasonTierModel] = []
var _items: Dictionary = {}
var _pool: Array[PanelContainer] = []
var _busy: bool = false


func _init() -> void:
	name = "SeasonTrack"
	vertical_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_AUTO
	_strip = Control.new()
	_strip.name = "Strip"
	_strip.mouse_filter = Control.MOUSE_FILTER_PASS
	add_child(_strip)
	get_h_scroll_bar().value_changed.connect(func(_v: float) -> void: _layout())
	resized.connect(_layout)


func show_tiers(tiers: Array[SeasonTierModel]) -> void:
	_tiers = tiers
	var tile: Vector2 = _tile_size()
	_strip.custom_minimum_size = Vector2(tile.x * float(_tiers.size()), tile.y)
	custom_minimum_size.y = tile.y + float(UiTokens.COMPONENT_ICON_SIZE) * 0.5
	_layout()


func set_busy(busy: bool) -> void:
	_busy = busy
	for item: PanelContainer in _items.values():
		var b: Button = item.find_child("Claim", true, false) as Button
		if b != null:
			b.disabled = busy


func tier_count() -> int:
	return _tiers.size()


func built_items() -> int:
	return _items.size() + _pool.size()


func item(tier: int) -> PanelContainer:
	return _items.get(tier) as PanelContainer


## Scrolls so `tier` is in view (the current tier when the tab opens).
func scroll_to(tier: int) -> void:
	var index: int = clampi(tier - 1, 0, maxi(0, _tiers.size() - 1))
	scroll_horizontal = int(maxf(0.0, float(index) * _tile_size().x - size.x * 0.4))
	_layout()


func texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for item_node: PanelContainer in _items.values():
		for node: Node in item_node.find_children("*", "Label", true, false):
			out.append(tr((node as Label).text))
		for node: Node in item_node.find_children("*", "Button", true, false):
			out.append(tr((node as Button).text))
	return out


func _tile_size() -> Vector2:
	var t: float = UiBuild.target()
	return Vector2(t * TILE_WIDTH, t * 3.0)


## Binds the tiles of the visible range, returning the others to the pool.
func _layout() -> void:
	if _tiers.is_empty():
		return
	var tile: Vector2 = _tile_size()
	var view_w: float = size.x if size.x > 0.0 else tile.x * 4.0
	var first: int = maxi(0, int(floor(float(scroll_horizontal) / tile.x)) - MARGIN_TILES)
	var last: int = mini(_tiers.size() - 1, int(ceil((float(scroll_horizontal) + view_w) / tile.x)) + MARGIN_TILES)
	var wanted: Dictionary = {}
	for i: int in range(first, last + 1):
		wanted[_tiers[i].tier] = i
	for t: int in _items.keys():
		if not wanted.has(t):
			_release(t)
	for t: int in wanted.keys():
		var i: int = int(wanted[t])
		var node: PanelContainer = _items.get(t) as PanelContainer
		if node == null:
			node = _pool.pop_back() if not _pool.is_empty() else PanelContainer.new()
			if node.get_parent() == null:
				_strip.add_child(node)
			node.visible = true
			_bind(node, _tiers[i])
			_items[t] = node
		elif String(node.get_meta("state", "")) != _state_of(_tiers[i]):
			_bind(node, _tiers[i])
		node.position = Vector2(tile.x * float(i), 0.0)
		node.size = Vector2(tile.x - float(UiTokens.COMPONENT_ICON_SIZE) * 0.25, tile.y)


func _release(t: int) -> void:
	var node: PanelContainer = _items.get(t) as PanelContainer
	if node == null:
		return
	_items.erase(t)
	node.visible = false
	_pool.append(node)


## What a tile shows (re-bound only when it changed).
static func _state_of(t: SeasonTierModel) -> String:
	return "%d|%s|%s|%d" % [t.tier, t.reached, t.claimed, t.rewards.size()]


func _bind(node: PanelContainer, t: SeasonTierModel) -> void:
	UiBuild.clear(node)
	node.set_meta("state", _state_of(t))
	node.name = "Tier%d" % t.tier
	node.theme_type_variation = &"QuestCard" if t.milestone else &"PartyMemberCard"
	var box: VBoxContainer = UiBuild.vbox()
	box.alignment = BoxContainer.ALIGNMENT_CENTER
	node.add_child(box)
	var head: Label = UiBuild.data_label(tr("ui.season.tier_label").format({"tier": LocaleFormat.number(t.tier)}),
			&"HeadingLabel", false)
	head.name = "TierLabel"
	head.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	box.add_child(head)
	if t.milestone:
		var tag: Label = UiBuild.label("ui.season.milestone", &"SectionTag", false)
		tag.name = "Milestone"
		tag.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
		box.add_child(tag)
	var words: PackedStringArray = []
	for r: RewardModel in t.rewards:
		words.append(r.display_text())
	var rewards: Label = UiBuild.data_label(", ".join(words), &"MutedLabel")
	rewards.name = "Rewards"
	rewards.horizontal_alignment = HORIZONTAL_ALIGNMENT_CENTER
	box.add_child(rewards)
	if t.claimed:
		box.add_child(FriendsUi.status("ui.events.claimed", "verdict_correct"))
	elif t.reached:
		var claim: Button = UiBuild.button("ui.events.claim", "gift")
		claim.name = "Claim"
		claim.disabled = _busy
		claim.size_flags_horizontal = Control.SIZE_SHRINK_CENTER
		claim.pressed.connect(func() -> void: claim_requested.emit(t.tier))
		box.add_child(claim)
	else:
		box.add_child(FriendsUi.status("ui.events.not_reached", "lock"))
