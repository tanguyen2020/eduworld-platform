extends Control
## "Thêm" menu of the HUD (STUDENT_GAME_UX §3 ☰, F069; AC-UX-0301 every function ≤ 2 taps): big picture tiles with an
## icon and a word — Cửa hàng (S08 Shop), Thành tựu & Bộ sưu tập (S09), Núi Thử Thách (S06), Hành trình của tôi; F109: Đấu trường and Bảng xếp hạng.
## Each tile opens its screen through `EventBus.ui_requested`; no rule, no network (AGENTS R1).

signal closed()

## Tiles: [screen_id, args, label key, icon].
const TILES: Array = [
	[&"inventory", {"tab": "shop"}, "ui.menu.shop", "shop"],
	[&"achievements", {}, "ui.menu.achievements", "medal"],
	[&"challenge_board", {}, "ui.menu.challenges", "area_challenge_mountain"],
	[&"journey", {}, "ui.menu.journey", "flag"],
	# F109: Đấu trường (also K) and its leaderboards.
	[&"arena", {}, "ui.menu.arena", "arena"],
	[&"arena", {"tab": "leaderboard"}, "ui.menu.leaderboard", "podium"],
]

var _grid: GridContainer = null


func _ready() -> void:
	visible = false
	var ui: Dictionary = UiBuild.window(self, "ui.menu.title", "menu")
	(ui["close"] as Button).pressed.connect(close)
	var win: PanelContainer = ui["window"] as PanelContainer
	# The window hugs its tiles, centred (no empty card on a tall or small screen).
	for side: Side in [SIDE_LEFT, SIDE_TOP, SIDE_RIGHT, SIDE_BOTTOM]:
		win.set_anchor(side, 0.5)
		win.set_offset(side, 0.0)
	win.grow_horizontal = Control.GROW_DIRECTION_BOTH
	win.grow_vertical = Control.GROW_DIRECTION_BOTH
	var center: CenterContainer = CenterContainer.new()
	UiBuild.expand(center, true, true)
	(ui["body"] as VBoxContainer).add_child(center)
	_grid = GridContainer.new()
	_grid.name = "Tiles"
	_grid.columns = 2
	center.add_child(_grid)
	for t: Array in TILES:
		var b: Button = UiBuild.button(String(t[2]), String(t[3]), &"SecondaryButton")
		b.name = "Tile_%s" % String(t[2]).get_slice(".", 2)
		b.icon_alignment = HORIZONTAL_ALIGNMENT_CENTER
		b.vertical_icon_alignment = VERTICAL_ALIGNMENT_TOP
		b.expand_icon = false
		b.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
		b.pressed.connect(_open.bind(t[0] as StringName, t[1] as Dictionary))
		_grid.add_child(b)


## WorldRoot: `ui_requested("game_menu", {})`.
func open(_args: Dictionary = {}) -> void:
	show()
	var t: float = UiBuild.target()
	for b: Node in _grid.get_children():
		# Never narrower than a long word ("Achievements") — no break inside a word.
		(b as Button).custom_minimum_size = Vector2(maxf(t * 4.0, float(UiTokens.SIZE_LAYOUT_280) * 0.9), t * 2.2)


func close() -> void:
	hide()
	closed.emit()


func tile_screens() -> Array[StringName]:
	var out: Array[StringName] = []
	for t: Array in TILES:
		out.append(t[0] as StringName)
	return out


func _open(screen: StringName, args: Dictionary) -> void:
	EventBus.ui_requested.emit(screen, args)


func _unhandled_input(event: InputEvent) -> void:
	if visible and event.is_action_pressed("ui_cancel"):
		close()
		get_viewport().set_input_as_handled()
