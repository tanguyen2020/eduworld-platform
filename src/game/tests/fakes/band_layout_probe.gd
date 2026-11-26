class_name BandLayoutProbe
extends RefCounted
## TEST-ONLY (AGENTS R17): layout rules of one grade band on a built screen (F098-AC-09; STUDENT_GAME_UX §4.3,
## DESIGN_SYSTEM §5.2, LEARNING_EXPERIENCE §8, NFR-A11Y-001). For the visible controls of `root` drawn in viewport `vp`:
## every button at least the band's touch target (`density.<band>.button.min_height`), every text at least the band's
## smallest font (`density.<band>.font.small`), every answer tile at least `density.<band>.answer.min_height` with the
## stem font size, no text or control outside the view (content of a ScrollContainer is checked through its scroll
## box), no wrapped text taller than its button/label (it would spill over the next control), and no trimmed
## single-line text, no two buttons of the same parent overlapping. Returns one line per problem.

## Reference windows → logical size with `stretch canvas_items` + `aspect expand` on the 1280×720 base.
const SIZES: Dictionary = {
	"1280x720": Vector2i(1280, 720), "1024x600": Vector2i(1280, 750), "720x1280": Vector2i(1280, 2276),
}
const TOLERANCE_PX: float = 1.0

## Controls checked by the last `check` (buttons, texts) — a screen that checks nothing is a broken probe.
static var last_buttons: int = 0
static var last_texts: int = 0
static var last_tiles: int = 0


static func check(root: Node, vp: Viewport, band: String, label: String) -> PackedStringArray:
	var out: PackedStringArray = []
	last_buttons = 0
	last_texts = 0
	last_tiles = 0
	var view: Rect2 = Rect2(Vector2.ZERO, Vector2(vp.size if vp is SubViewport else vp.get_visible_rect().size)).grow(
			TOLERANCE_PX)
	var button_min: float = float(UiTokens.DENSITY_BUTTON_MIN_HEIGHT[band])
	var font_min: int = int(UiTokens.DENSITY_FONT_SMALL[band])
	var tile_min: float = float(UiTokens.DENSITY_ANSWER_MIN_HEIGHT[band])
	var stem: int = int(UiTokens.DENSITY_FONT_STEM[band])
	for node: Node in root.find_children("*", "Control", true, false):
		var c: Control = node as Control
		if c.get_viewport() != vp or not c.is_visible_in_tree() or c.size.x <= 0.0 or c.size.y <= 0.0:
			continue
		var where: String = "%s %s %s" % [label, band, _short(root, c)]
		if c is BaseButton:
			last_buttons += 1
			var height: float = maxf(c.get_combined_minimum_size().y, c.size.y)
			if height + 0.5 < button_min:
				out.append("%s: button %.0f px < %.0f" % [where, height, button_min])
			if String(c.theme_type_variation).begins_with("AnswerTile"):
				last_tiles += 1
				if height + 0.5 < tile_min:
					out.append("%s: answer tile %.0f px < %.0f" % [where, height, tile_min])
				if c.get_theme_font_size(&"font_size") != stem:
					out.append("%s: answer tile font %d != %d" % [where, c.get_theme_font_size(&"font_size"), stem])
		var size: int = _font_size(c)
		if size > 0:
			last_texts += 1
		if size > 0 and size < font_min:
			out.append("%s: font %d px < %d" % [where, size, font_min])
		if _checked_for_overflow(c) and not _in_scroll(c, root) and not view.encloses(c.get_global_rect()):
			out.append("%s: outside the view %s (view %s)" % [where, c.get_global_rect(), view.size])
		if (c is Button or c is Label) and c.get_combined_minimum_size().y > c.size.y + TOLERANCE_PX:
			out.append("%s: text taller than its box (%.0f > %.0f px)" % [where, c.get_combined_minimum_size().y, c.size.y])
		var spilled: float = _clipped_wrap_overflow(c)
		if spilled > TOLERANCE_PX:
			out.append("%s: wrapped text cut at the bottom (%.0f px more)" % [where, spilled])
		var broken: String = _broken_word(c)
		if not broken.is_empty():
			out.append("%s: word cut across lines \"%s\"" % [where, broken])
		var trimmed: String = _trimmed(c)
		if not trimmed.is_empty():
			out.append("%s: trimmed text \"%s\"" % [where, trimmed])
	for line: String in overlapping_siblings(root, vp):
		out.append("%s %s: %s" % [label, band, line])
	return out


## Font size of a control that shows text (0 when it shows none).
static func _font_size(c: Control) -> int:
	if c is Label:
		return c.get_theme_font_size(&"font_size") if not (c as Label).text.strip_edges().is_empty() else 0
	if c is Button:
		return c.get_theme_font_size(&"font_size") if not (c as Button).text.strip_edges().is_empty() else 0
	if c is LineEdit:
		return c.get_theme_font_size(&"font_size")
	if c is RichTextLabel:
		return c.get_theme_font_size(&"normal_font_size") if not (c as RichTextLabel).text.is_empty() else 0
	return 0


## Text, buttons, fields and bars must be inside the view; decoration (backdrops, art) may bleed off it.
static func _checked_for_overflow(c: Control) -> bool:
	return c is BaseButton or c is Label or c is RichTextLabel or c is LineEdit or c is Range or c is ScrollContainer


## Inside a ScrollContainer (reachable by scrolling; the scroll box itself is checked).
static func _in_scroll(c: Control, root: Node) -> bool:
	var p: Node = c.get_parent()
	while p != null and p != root.get_parent():
		if p is ScrollContainer:
			return true
		p = p.get_parent()
	return false


## The text of a single-line label/button that is cut by its width (clip or ellipsis), else "".
static func _trimmed(c: Control) -> String:
	var text: String = ""
	var font: Font = null
	var font_size: int = 0
	var available: float = c.size.x
	if c is Label:
		var l: Label = c as Label
		if l.autowrap_mode != TextServer.AUTOWRAP_OFF or (not l.clip_text
				and l.text_overrun_behavior == TextServer.OVERRUN_NO_TRIMMING):
			return ""
		text = l.tr(l.text) if l.can_auto_translate() else l.text
		font = l.get_theme_font(&"font")
		font_size = l.get_theme_font_size(&"font_size")
		var box: StyleBox = l.get_theme_stylebox(&"normal")
		if box != null:
			available -= box.get_margin(SIDE_LEFT) + box.get_margin(SIDE_RIGHT)
	elif c is Button:
		var b: Button = c as Button
		if b.autowrap_mode != TextServer.AUTOWRAP_OFF or (not b.clip_text
				and b.text_overrun_behavior == TextServer.OVERRUN_NO_TRIMMING):
			return ""
		text = b.tr(b.text) if b.can_auto_translate() else b.text
		font = b.get_theme_font(&"font")
		font_size = b.get_theme_font_size(&"font_size")
		var box: StyleBox = b.get_theme_stylebox(&"normal")
		if box != null:
			available -= box.get_margin(SIDE_LEFT) + box.get_margin(SIDE_RIGHT)
		if b.icon != null:
			available -= minf(float(b.get_theme_constant(&"icon_max_width")), float(b.icon.get_width())) \
					+ float(b.get_theme_constant(&"h_separation"))
	else:
		return ""
	if text.strip_edges().is_empty() or font == null:
		return ""
	var needed: float = font.get_string_size(text, HORIZONTAL_ALIGNMENT_LEFT, -1.0, font_size).x
	return text if needed > available + TOLERANCE_PX else ""


## Visible buttons of the same parent that overlap (a cell or tile grown by its text over its neighbour), one line each.
static func overlapping_siblings(root: Node, vp: Viewport) -> PackedStringArray:
	var out: PackedStringArray = []
	var by_parent: Dictionary = {}
	for node: Node in root.find_children("*", "BaseButton", true, false):
		var b: Control = node as Control
		if b.get_viewport() != vp or not b.is_visible_in_tree() or b.size.x <= 0.0 or b.size.y <= 0.0:
			continue
		var key: int = b.get_parent().get_instance_id()
		if not by_parent.has(key):
			by_parent[key] = []
		(by_parent[key] as Array).append(b)
	for group: Variant in by_parent.values():
		var list: Array = group as Array
		for i: int in list.size():
			for j: int in range(i + 1, list.size()):
				var a: Rect2 = (list[i] as Control).get_global_rect().grow(-TOLERANCE_PX)
				var c: Rect2 = (list[j] as Control).get_global_rect().grow(-TOLERANCE_PX)
				if a.intersects(c):
					out.append("%s and %s overlap" % [_short(root, list[i]), _short(root, list[j])])
	return out


## A clipping, wrapping button (e.g. a picture tile): how much taller its wrapped text is than the room left under
## its top margin (0 when it fits or the button does not clip) — clipped text would lose its last lines.
static func _clipped_wrap_overflow(c: Control) -> float:
	if not c is Button:
		return 0.0
	var b: Button = c as Button
	if not b.clip_text or b.autowrap_mode == TextServer.AUTOWRAP_OFF or b.text.strip_edges().is_empty():
		return 0.0
	var box: StyleBox = b.get_theme_stylebox(&"normal")
	var margins: Vector2 = Vector2.ZERO
	var top_bottom: float = 0.0
	if box != null:
		margins.x = box.get_margin(SIDE_LEFT) + box.get_margin(SIDE_RIGHT)
		top_bottom = box.get_margin(SIDE_TOP) + box.get_margin(SIDE_BOTTOM)
	var font: Font = b.get_theme_font(&"font")
	if font == null:
		return 0.0
	var text: String = b.tr(b.text) if b.can_auto_translate() else b.text
	var needed: float = font.get_multiline_string_size(text, HORIZONTAL_ALIGNMENT_LEFT, b.size.x - margins.x,
			b.get_theme_font_size(&"font_size"), -1, TextServer.BREAK_MANDATORY | TextServer.BREAK_WORD_BOUND).y
	return needed - (b.size.y - top_bottom)


## A word wider than the text width of a wrapping label/button (autowrap then cuts it inside the word), else "".
static func _broken_word(c: Control) -> String:
	var text: String = ""
	var available: float = c.size.x
	var box: StyleBox = null
	if c is Label:
		var l: Label = c as Label
		if l.autowrap_mode == TextServer.AUTOWRAP_OFF:
			return ""
		text = l.tr(l.text) if l.can_auto_translate() else l.text
		box = l.get_theme_stylebox(&"normal")
	elif c is Button:
		var b: Button = c as Button
		if b.autowrap_mode == TextServer.AUTOWRAP_OFF:
			return ""
		text = b.tr(b.text) if b.can_auto_translate() else b.text
		box = b.get_theme_stylebox(&"normal")
		if b.icon != null and b.icon_alignment != HORIZONTAL_ALIGNMENT_CENTER:
			available -= minf(float(b.get_theme_constant(&"icon_max_width")), float(b.icon.get_width())) \
					+ float(b.get_theme_constant(&"h_separation"))
	else:
		return ""
	if box != null:
		available -= box.get_margin(SIDE_LEFT) + box.get_margin(SIDE_RIGHT)
	var font: Font = c.get_theme_font(&"font")
	var font_size: int = c.get_theme_font_size(&"font_size")
	if font == null or text.strip_edges().is_empty():
		return ""
	for word: String in text.replace("\n", " ").split(" ", false):
		if font.get_string_size(word, HORIZONTAL_ALIGNMENT_LEFT, -1.0, font_size).x > available + TOLERANCE_PX:
			return word
	return ""


static func _short(root: Node, c: Node) -> String:
	return String(root.get_path_to(c))


## F099-AC-07: empty horizontal bands of the activity board. Every control showing something (text, button, bar,
## picture, Mây) of the board in viewport `vp` is projected on the vertical axis (content of a ScrollContainer clipped
## to its scroll box); no gap between two neighbouring contents — or from the last one to the bottom of the board — may
## be taller than `max_ratio` × the view height. Returns one line per band too tall.
static func empty_bands(root: Node, vp: Viewport, label: String, max_ratio: float = 0.25) -> PackedStringArray:
	var out: PackedStringArray = []
	var board: Control = _board(root, vp)
	if board == null:
		return PackedStringArray(["%s: no activity board" % label])
	var view_h: float = float(vp.size.y if vp is SubViewport else vp.get_visible_rect().size.y)
	var inner: Rect2 = board.get_global_rect()
	var box: StyleBox = board.get_theme_stylebox(&"panel")
	if box != null:
		inner = Rect2(inner.position + Vector2(box.get_margin(SIDE_LEFT), box.get_margin(SIDE_TOP)),
				inner.size - Vector2(box.get_margin(SIDE_LEFT) + box.get_margin(SIDE_RIGHT),
				box.get_margin(SIDE_TOP) + box.get_margin(SIDE_BOTTOM)))
	var spans: Array[Vector2] = []
	for node: Node in board.find_children("*", "Control", true, false):
		var c: Control = node as Control
		if c.get_viewport() != vp or not c.is_visible_in_tree() or c.size.x <= 0.0 or c.size.y <= 0.0 \
				or not _shows_content(c):
			continue
		var rect: Rect2 = c.get_global_rect()
		var scroll: ScrollContainer = _scroll_of(c, board)
		if scroll != null:
			rect = rect.intersection(scroll.get_global_rect())
		rect = rect.intersection(inner)
		if rect.size.y > 0.0:
			spans.append(Vector2(rect.position.y, rect.end.y))
	if spans.is_empty():
		return PackedStringArray(["%s: nothing shown on the board" % label])
	spans.sort_custom(func(a: Vector2, b: Vector2) -> bool: return a.x < b.x)
	var limit: float = max_ratio * view_h
	var reach: float = spans[0].y
	for i: int in range(1, spans.size()):
		var gap: float = spans[i].x - reach
		if gap > limit:
			out.append("%s: empty band %.0f px (%.0f%% of the view) at y=%.0f" % [label, gap, gap / view_h * 100.0, reach])
		reach = maxf(reach, spans[i].y)
	var tail: float = inner.end.y - reach
	if tail > limit:
		out.append("%s: empty band %.0f px (%.0f%% of the view) under the content" % [label, tail, tail / view_h * 100.0])
	last_largest_band = 0.0
	reach = spans[0].y
	for i: int in range(1, spans.size()):
		last_largest_band = maxf(last_largest_band, spans[i].x - reach)
		reach = maxf(reach, spans[i].y)
	last_largest_band = maxf(last_largest_band, inner.end.y - reach) / view_h
	return out


## Largest empty band of the last `empty_bands` call, as a share of the view height (for the evidence table).
static var last_largest_band: float = 0.0


static func _board(root: Node, vp: Viewport) -> Control:
	var nodes: Array[Node] = [root]
	nodes.append_array(root.find_children("*", "Control", true, false))
	for node: Node in nodes:
		var c: Control = node as Control
		if c != null and c.get_viewport() == vp and c.is_visible_in_tree() and c.theme_type_variation == &"ActivityBoard":
			return c
	return null


static func _shows_content(c: Control) -> bool:
	if c is Label:
		return not (c as Label).text.strip_edges().is_empty()
	if c is RichTextLabel:
		return not (c as RichTextLabel).text.is_empty()
	if c is TextureRect:
		return (c as TextureRect).texture != null
	return c is BaseButton or c is Range or c is LineEdit or c is CloudyMascot


static func _scroll_of(c: Control, stop: Node) -> ScrollContainer:
	var p: Node = c.get_parent()
	while p != null and p != stop:
		if p is ScrollContainer:
			return p as ScrollContainer
		p = p.get_parent()
	return null
