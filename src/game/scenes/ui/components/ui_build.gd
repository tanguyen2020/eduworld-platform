class_name UiBuild
extends RefCounted
## Small builders for the F069 screens (S06/S07/S08/S09, "Hành trình"): plain Controls styled only by the generated
## Theme through type variations (DESIGN_SYSTEM §5, AGENTS R32 — no theme override), texts as translation keys
## (auto-translated, so a locale switch re-renders them), every button with an icon beside its label (F096-AC-09) and a
## touch target from the grade-band density (`UiTokens.DENSITY_BUTTON_MIN_HEIGHT`: g1_3 ≥ 64 dp, NFR-A11Y-001).

const ICONS: String = "res://resources/ui/icons/%s.svg"


static func band() -> String:
	return AppState.grade_band if ThemeService.is_valid_band(AppState.grade_band) else ThemeService.default_band()


## Touch target height of the current grade band.
static func target() -> float:
	return float(UiTokens.DENSITY_BUTTON_MIN_HEIGHT[band()])


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


## The standard F069 window: a centred PanelContainer (like the quest journal) with a header (icon + title + Close)
## and a body box. Returns `{window, body, title, close}`.
static func window(root: Control, title_key: String, title_icon: String) -> Dictionary:
	var win: PanelContainer = panel()
	win.name = "Window"
	win.set_anchors_preset(Control.PRESET_FULL_RECT)
	win.anchor_left = 0.05
	win.anchor_top = 0.06
	win.anchor_right = 0.95
	win.anchor_bottom = 0.95
	win.offset_left = 0.0
	win.offset_top = 0.0
	win.offset_right = 0.0
	win.offset_bottom = 0.0
	root.add_child(win)
	var box: VBoxContainer = vbox()
	box.name = "Box"
	win.add_child(box)
	var header: HBoxContainer = hbox()
	header.name = "Header"
	box.add_child(header)
	header.add_child(icon_rect(title_icon, 1.4))
	var title: Label = label(title_key, &"HeadingLabel", false)
	title.name = "Title"
	title.text_overrun_behavior = TextServer.OVERRUN_TRIM_ELLIPSIS
	expand(title)
	header.add_child(title)
	var close: Button = button("ui.common.close", "exit", &"SecondaryButton")
	close.name = "Close"
	header.add_child(close)
	return {"window": win, "body": box, "title": title, "close": close, "header": header}


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
