class_name UiBuild
extends RefCounted
## Small builders for the F069 screens (S06/S07/S08/S09, "Hành trình"): plain Controls styled only by the generated
## Theme through type variations (DESIGN_SYSTEM §5, AGENTS R32 — no theme override), texts as translation keys
## (auto-translated, so a locale switch re-renders them), every button with an icon beside its label (F096-AC-09) and a
## touch target from the grade-band density (`UiTokens.DENSITY_BUTTON_MIN_HEIGHT`: g1_3 ≥ 64 dp, NFR-A11Y-001).

const ICONS: String = "res://resources/ui/icons/%s.svg"
## Anchors [left, top, right, bottom] of a standard window, and on a compact phone (F111 compact polish: the full height
## but a thin edge, so a window with a header, tabs and a sub-page header still shows its content).
const WINDOW_EDGES: Array = [0.05, 0.06, 0.95, 0.95]
const WINDOW_EDGES_COMPACT: Array = [0.03, 0.02, 0.97, 0.98]


static func band() -> String:
	return AppState.grade_band if ThemeService.is_valid_band(AppState.grade_band) else ThemeService.default_band()


## Touch target height of the current grade band (F125: at least `density.compact.button.min_height` on a phone).
static func target() -> float:
	return target_for(band())


## Touch target height of `band_name` in the current mode (compact or not, AppState.compact).
static func target_for(band_name: String) -> float:
	return CompactPolicy.button_target(band_name, AppState.compact)


static func icon(name: String) -> Texture2D:
	return load(ICONS % name) as Texture2D


static func label(key: String, variation: StringName = &"", wrap: bool = true) -> Label:
	var l: Label = Label.new()
	l.text = key
	if variation != &"":
		l.theme_type_variation = variation
	if wrap:
		l.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	return l


## A label showing data text (no translation lookup of the value itself).
static func data_label(text: String, variation: StringName = &"", wrap: bool = true) -> Label:
	var l: Label = label(text, variation, wrap)
	l.auto_translate_mode = Node.AUTO_TRANSLATE_MODE_DISABLED
	return l


static func button(key: String, icon_name: String = "", variation: StringName = &"") -> Button:
	var b: Button = Button.new()
	b.text = key
	if not icon_name.is_empty():
		b.icon = icon(icon_name)
	if variation != &"":
		b.theme_type_variation = variation
	b.custom_minimum_size = Vector2(target(), target())
	return b


## A choice chip (toggle); the chosen one shows a tick icon, not only a colour (F097, NFR-A11Y-001).
static func chip(key: String, group: ButtonGroup = null) -> Button:
	var b: Button = button(key, "", &"ChoiceChip")
	b.toggle_mode = true
	b.button_group = group
	return b


## Updates the tick of every chip of `chips` to its pressed state (keeps `idle_icon` of unpressed chips).
static func mark_chips(chips: Array, idle_icons: Dictionary = {}) -> void:
	for node: Variant in chips:
		var b: Button = node as Button
		if b == null:
			continue
		b.icon = icon("check") if b.button_pressed else idle_icons.get(b, null) as Texture2D


static func icon_rect(icon_name: String, scale: float = 1.0) -> TextureRect:
	var t: TextureRect = TextureRect.new()
	t.texture = icon(icon_name)
	t.expand_mode = TextureRect.EXPAND_IGNORE_SIZE
	t.stretch_mode = TextureRect.STRETCH_KEEP_ASPECT_CENTERED
	var side: float = float(UiTokens.COMPONENT_ICON_SIZE) * scale
	t.custom_minimum_size = Vector2(side, side)
	t.mouse_filter = Control.MOUSE_FILTER_IGNORE
	return t


## Sizes the title icon of a window built in a scene (.tscn) like `icon_rect` (`component.icon.size`) and puts it on
## its white tile (`icon_badge`).
static func fit_icon(t: TextureRect, scale: float = 1.0) -> void:
	var side: float = float(UiTokens.COMPONENT_ICON_SIZE) * scale
	t.custom_minimum_size = Vector2(side, side)
	if t.get_parent() is HBoxContainer:
		var row: Node = t.get_parent()
		var at: int = t.get_index()
		row.remove_child(t)
		var badge: PanelContainer = icon_badge(t)
		row.add_child(badge)
		row.move_child(badge, at)


## F125: the icon of a window title bar on a small white tile (`TitleIconBadge`) — every icon colour reads on the blue
## bar (a blue line icon such as the journal or the map would vanish on it).
static func icon_badge(t: TextureRect) -> PanelContainer:
	var badge: PanelContainer = panel(&"TitleIconBadge")
	badge.name = "TitleIconBadge"
	badge.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	badge.mouse_filter = Control.MOUSE_FILTER_IGNORE
	badge.add_child(t)
	return badge


## Columns of a choice grid (F097/F099): one column on a portrait board or past 4 choices, else 2; F125: on the compact
## phone canvas (540 tall, wide) up to 4 choices sit on one row so every answer shows without scrolling.
static func answer_columns(count: int, tall: bool) -> int:
	if tall or count > 4:
		return 1
	return clampi(count, 2, 4) if AppState.compact else 2


static func vbox(variation: StringName = &"") -> VBoxContainer:
	var v: VBoxContainer = VBoxContainer.new()
	if variation != &"":
		v.theme_type_variation = variation
	return v


static func hbox(variation: StringName = &"") -> HBoxContainer:
	var h: HBoxContainer = HBoxContainer.new()
	if variation != &"":
		h.theme_type_variation = variation
	return h


static func panel(variation: StringName = &"") -> PanelContainer:
	var p: PanelContainer = PanelContainer.new()
	if variation != &"":
		p.theme_type_variation = variation
	return p


static func expand(control: Control, horizontal: bool = true, vertical: bool = false) -> Control:
	if horizontal:
		control.size_flags_horizontal = Control.SIZE_EXPAND_FILL
	if vertical:
		control.size_flags_vertical = Control.SIZE_EXPAND_FILL
	return control


## F114-AC-07: the scrim behind a modal window (ink 50%) or, `nested`, behind a dialog opened over a window (ink 35%
## more, ≈ 70% in total). Visual only: the screen root already takes the clicks.
static func scrim(nested: bool = false) -> Panel:
	var p: Panel = Panel.new()
	p.name = "ScrimNested" if nested else "Scrim"
	p.theme_type_variation = &"ScrimNested" if nested else &"Scrim"
	p.set_anchors_preset(Control.PRESET_FULL_RECT)
	p.mouse_filter = Control.MOUSE_FILTER_IGNORE
	return p


## The standard F069 window: a centred PanelContainer (like the quest journal) with the shared title bar (icon + title +
## the red Close, F125) and a body box, over the modal scrim (F114). Returns `{window, body, title, close, header}`
## (`header` = the row of the title bar; extra buttons go before `close`).
static func window(root: Control, title_key: String, title_icon: String) -> Dictionary:
	root.add_child(scrim())
	var win: PanelContainer = panel()
	win.name = "Window"
	win.set_anchors_preset(Control.PRESET_FULL_RECT)
	win.anchor_left = float(WINDOW_EDGES[0])
	win.anchor_top = float(WINDOW_EDGES[1])
	win.anchor_right = float(WINDOW_EDGES[2])
	win.anchor_bottom = float(WINDOW_EDGES[3])
	win.offset_left = 0.0
	win.offset_top = 0.0
	win.offset_right = 0.0
	win.offset_bottom = 0.0
	root.add_child(win)
	var box: VBoxContainer = vbox()
	box.name = "Box"
	win.add_child(box)
	var bar: Dictionary = title_bar(title_key, title_icon)
	box.add_child(bar["bar"] as Control)
	return {"window": win, "body": box, "title": bar["title"], "close": bar["close"], "header": bar["row"],
		"icon": bar["icon"]}


## F111 compact polish (STUDENT_GAME_UX §2 compact): on a compact phone the window `win` takes the full height of the
## screen but a thin edge (`WINDOW_EDGES_COMPACT`), so the rows of its header leave room for its content; outside
## compact mode the usual centred frame. The screen calls it again when the mode changes (`AppState.compact_changed`).
static func fit_window(win: Control) -> void:
	var edges: Array = WINDOW_EDGES_COMPACT if AppState.compact else WINDOW_EDGES
	win.anchor_left = float(edges[0])
	win.anchor_top = float(edges[1])
	win.anchor_right = float(edges[2])
	win.anchor_bottom = float(edges[3])


## F111 compact polish: on a compact phone `b` shows its icon alone — its words (`key`) stay as tooltip and accessible
## name — so a header of actions is one line; outside compact mode icon + words (F096-AC-09).
static func icon_only(b: Button, key: String, on: bool) -> void:
	b.text = "" if on else key
	b.tooltip_text = key if on else ""
	b.accessibility_name = b.tr(key) if on else ""


## F125 (F114-3, DESIGN_SYSTEM §5.1): the title bar of every window — tier 1 blue `WindowTitleBar`, the window icon, an
## upper-case title and, at its right end, the one close button of the window (`close_button`). Returns
## `{bar, row, icon, title, close}`; a window adds its own buttons (Rời nhóm, Xu…) to `row` before `close`.
static func title_bar(title_key: String, icon_name: String) -> Dictionary:
	var bar: PanelContainer = panel(&"WindowTitleBar")
	bar.name = "TitleBar"
	var row: HBoxContainer = hbox(&"ToolbarBox")
	row.name = "Row"
	bar.add_child(row)
	var title_icon: TextureRect = icon_rect(icon_name)
	title_icon.name = "TitleIcon"
	row.add_child(icon_badge(title_icon))
	var title: Label = label(title_key, &"WindowTitleLabel", false)
	title.name = "Title"
	title.uppercase = true
	title.autowrap_mode = TextServer.AUTOWRAP_WORD_SMART
	title.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	expand(title)
	row.add_child(title)
	var close: Button = close_button()
	row.add_child(close)
	return {"bar": bar, "row": row, "icon": title_icon, "title": title, "close": close}


## F125-AC-06: the one close button of a window — red `DangerButton`, icon ✕ + "Đóng", the band touch target (compact
## floor on a phone). Red = leaving (F114); confirmation dialogs keep their neutral "Hủy" and activity/exam screens their
## guarded "Thoát" (F125-AC-07). The window wires it and `ui_cancel` (Esc/Back) to the same close.
static func close_button() -> Button:
	var close: Button = button("ui.common.close", "cross", &"DangerButton")
	close.name = "Close"
	close.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	return close


## Removes and frees every child of `node`.
static func clear(node: Node) -> void:
	for child: Node in node.get_children():
		node.remove_child(child)
		child.queue_free()


## Sizes every Button under `root` to the grade-band touch target (height; width at least the same).
static func size_buttons(root: Node) -> void:
	var t: float = target()
	for node: Node in root.find_children("*", "Button", true, false):
		var b: Button = node as Button
		b.custom_minimum_size = Vector2(maxf(b.custom_minimum_size.x, t), maxf(t, 0.0))


## F125: a wrapping label never narrower than its longest word (names, pseudo-locale words): a narrow column on the
## compact phone canvas then grows or moves it instead of cutting a word. Re-measured when the Theme changes.
static func keep_words(l: Label) -> Label:
	l.ready.connect(fit_words.bind(l))
	l.theme_changed.connect(fit_words.bind(l))
	return l


## Sets the minimum width of `l` to its longest word (call again after changing its text).
static func fit_words(l: Label) -> void:
	var font: Font = l.get_theme_font(&"font")
	if font == null or not l.is_inside_tree():
		return
	var px: int = l.get_theme_font_size(&"font_size")
	var text: String = l.tr(l.text) if l.can_auto_translate() else l.text
	if l.uppercase:
		text = text.to_upper()
	var widest: float = 0.0
	for word: String in text.replace("
", " ").split(" ", false):
		widest = maxf(widest, font.get_string_size(word, HORIZONTAL_ALIGNMENT_LEFT, -1.0, px).x)
	var box: StyleBox = l.get_theme_stylebox(&"normal")
	var pad: float = box.get_margin(SIDE_LEFT) + box.get_margin(SIDE_RIGHT) if box != null else 0.0
	l.custom_minimum_size.x = ceilf(widest + pad)


## Mây beside a speech bubble (F097 coach layout): `mood` happy for a celebration, encourage for a "keep going" note
## (never sad, NFR-SAFE-003). Returns `{row, bubble_box, mascot}`; the caller fills `bubble_box`.
static func coach(mood: CloudyMascot.Mood) -> Dictionary:
	var row: HBoxContainer = hbox()
	row.name = "Coach"
	var mascot: CloudyMascot = CloudyMascot.new()
	mascot.name = "Mascot"
	mascot.compact = true
	mascot.bubble_tail = false
	mascot.side_tail = true
	mascot.mood = mood
	mascot.size_flags_vertical = Control.SIZE_SHRINK_END
	row.add_child(mascot)
	var bubble: PanelContainer = panel(&"SpeechBubble")
	bubble.name = "Bubble"
	expand(bubble)
	row.add_child(bubble)
	var box: VBoxContainer = vbox()
	bubble.add_child(box)
	return {"row": row, "bubble_box": box, "mascot": mascot}


## A themed progress bar (0…1) at the progress token height (achievements, collections, journey).
static func progress_bar(value: float = 0.0) -> ProgressBar:
	var bar: ProgressBar = ProgressBar.new()
	bar.max_value = 1.0
	bar.step = 0.0
	bar.show_percentage = false
	bar.value = value
	bar.custom_minimum_size = Vector2(float(UiTokens.SIZE_LAYOUT_120), float(UiTokens.COMPONENT_PROGRESS_HEIGHT) * 0.6)
	bar.size_flags_vertical = Control.SIZE_SHRINK_CENTER
	return bar


## A one-line horizontal scroller for a long chip row (STUDENT_GAME_UX_SOCIAL §3 "cuộn ngang"): the row never wraps
## into several lines that would push the content down.
static func chip_scroller(row: Control) -> Control:
	# F099: arrows on both sides appear when the chips do not fit (the hidden chips are never a surprise).
	return ChipScroller.new(row)


## F098 (grade-band matrix): the minimum width of a wrapping button — its whole label on one line up to `cap` px, never
## narrower than its longest word, plus icon and padding, at least the band touch target. In a shrinking container
## (the HUD SideBar) an autowrapping button otherwise reports only its icon and is laid out as a tall column that cuts
## its words; a longer label (pseudo-locale, a long translation) still wraps between words past `cap`.
static func fit_wrapping(b: Button, cap: float) -> void:
	var font: Font = b.get_theme_font(&"font")
	if font == null:
		return
	var font_px: int = b.get_theme_font_size(&"font_size")
	var text: String = b.tr(b.text) if b.can_auto_translate() else b.text
	var widest: float = 0.0
	for word: String in text.replace("\n", " ").split(" ", false):
		widest = maxf(widest, font.get_string_size(word, HORIZONTAL_ALIGNMENT_LEFT, -1.0, font_px).x)
	var whole: float = font.get_string_size(text, HORIZONTAL_ALIGNMENT_LEFT, -1.0, font_px).x
	var extra: float = 0.0
	var box: StyleBox = b.get_theme_stylebox(&"normal")
	if box != null:
		extra += box.get_margin(SIDE_LEFT) + box.get_margin(SIDE_RIGHT)
	if b.icon != null:
		extra += minf(float(b.get_theme_constant(&"icon_max_width")), float(b.icon.get_width())) \
				+ float(b.get_theme_constant(&"h_separation"))
	b.custom_minimum_size.x = ceilf(maxf(target(), maxf(minf(whole, cap), widest) + extra))
