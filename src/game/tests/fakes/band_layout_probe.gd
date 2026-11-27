class_name BandLayoutProbe
extends RefCounted
## TEST-ONLY (AGENTS R17): layout rules of one grade band on a built screen (F098-AC-09; STUDENT_GAME_UX §4.3,
## DESIGN_SYSTEM §5.2, LEARNING_EXPERIENCE §8, NFR-A11Y-001). For the visible controls of `root` drawn in viewport `vp`:
## every button at least the band's touch target (`density.<band>.button.min_height`), every text at least the band's
## smallest font (`density.<band>.font.small`), every answer tile at least `density.<band>.answer.min_height` with the
## stem font size, no text or control outside the view (content of a ScrollContainer is checked through its scroll
## box), no wrapped text taller than its button/label (it would spill over the next control), and no trimmed
## single-line text, no two buttons of the same parent overlapping. Returns one line per problem.
## F125 (STUDENT_GAME_UX §2, DESIGN_SYSTEM §5.2, §9): the phone windows are in compact mode — logical canvas 540 tall,
## touch target and smallest font floored by `density.compact.*` (CompactPolicy) — and the physical probe converts the
## logical sizes to CSS px by the window scale (stretch × content scale): every text ≥ 12 CSS px and every pressable
## control ≥ 40 CSS px on screen.

## Reference windows → logical size with `stretch canvas_items` + `aspect expand` on the 1280×720 base. Landscape only
## (F113, STUDENT_GAME_UX §2): 1280×720, 1024×600 and two phones held sideways in compact mode (F125: 844×390 →
## 1168×540, 740×360 → 1110×540; `test_compact_policy` checks these against CompactPolicy).
const SIZES: Dictionary = {
	"1280x720": Vector2i(1280, 720), "1024x600": Vector2i(1280, 750), "844x390": Vector2i(1168, 540),
	"740x360": Vector2i(1110, 540),
}
## The window (CSS px, density 1) of every reference size above.
const WINDOWS: Dictionary = {
	"1280x720": Vector2(1280, 720), "1024x600": Vector2(1024, 600), "844x390": Vector2(844, 390),
	"740x360": Vector2(740, 360),
}
## F125-AC-04 negative check: the phone 844×390 before F125 (factor 1: logical 1558×720, 0.54 CSS px per logical px).
const LEGACY_PHONE: Dictionary = {"844x390-1x": Vector2i(1558, 720)}
## F125: physical floors on a compact phone (DESIGN_SYSTEM §9, test only).
const MIN_TEXT_CSS_PX: float = 12.0
const MIN_TOUCH_CSS_PX: float = 40.0
## F113-AC-05: windows taller than wide (720×1280, a phone held upright 390×844) — only the rotate prompt is checked.
const PORTRAIT_SIZES: Dictionary = {"720x1280": Vector2i(1280, 2276), "390x844": Vector2i(1280, 2770)}
const TOLERANCE_PX: float = 1.0

## Controls checked by the last `check` (buttons, texts) — a screen that checks nothing is a broken probe.
static var last_buttons: int = 0
static var last_texts: int = 0
static var last_tiles: int = 0
## Smallest text and pressable control of the last `check` in CSS px (0 = not measured), for the evidence table.
static var last_min_text_px: float = 0.0
static var last_min_touch_px: float = 0.0


## F125: the reference window `name` is in compact mode (CompactPolicy at density 1).
static func compact_of(name: String) -> bool:
	return WINDOWS.has(name) and CompactPolicy.is_compact(WINDOWS[name] as Vector2)


## F125: CSS px per logical px on the reference window `name` (its own content scale); 0 for an unknown name.
static func css_scale_of(name: String) -> float:
	if LEGACY_PHONE.has(name):
		return 844.0 / float((LEGACY_PHONE[name] as Vector2i).x)
	if not WINDOWS.has(name):
		return 0.0
	var window: Vector2 = WINDOWS[name] as Vector2
	var base: Vector2 = OrientationPolicy.base_size()
	return CompactPolicy.physical_scale(window, base, CompactPolicy.content_scale(window, base, compact_of(name)))


## `compact`: thresholds of compact mode (F125-AC-03); `css_scale` > 0: also the physical floors (12 / 40 CSS px) with
## that many CSS px per logical px.
static func check(root: Node, vp: Viewport, band: String, label: String, compact: bool = false,
		css_scale: float = 0.0) -> PackedStringArray:
	var out: PackedStringArray = []
	last_buttons = 0
	last_texts = 0
	last_tiles = 0
	last_min_text_px = 0.0
	last_min_touch_px = 0.0
	var view: Rect2 = Rect2(Vector2.ZERO, Vector2(vp.size if vp is SubViewport else vp.get_visible_rect().size)).grow(
			TOLERANCE_PX)
	var button_min: float = CompactPolicy.button_target(band, compact)
	var font_min: int = CompactPolicy.font_floor(band, compact)
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
			if css_scale > 0.0:
				last_min_touch_px = height * css_scale if last_min_touch_px == 0.0 						else minf(last_min_touch_px, height * css_scale)
				if height * css_scale + 0.05 < MIN_TOUCH_CSS_PX:
					out.append("%s: touch target %.1f CSS px < %.0f" % [where, height * css_scale, MIN_TOUCH_CSS_PX])
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
		if size > 0 and css_scale > 0.0:
			last_min_text_px = float(size) * css_scale if last_min_text_px == 0.0 					else minf(last_min_text_px, float(size) * css_scale)
			if float(size) * css_scale + 0.05 < MIN_TEXT_CSS_PX:
				out.append("%s: text %.1f CSS px < %.0f" % [where, float(size) * css_scale, MIN_TEXT_CSS_PX])
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
	for line: String in overlapping_messages(root, vp):
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


## F125 polish (STUDENT_GAME_UX_SOCIAL §7 MessageList): the message cards of every visible chat list of `root` in `vp`
## never overlap — each card (at least its own minimum height, the real text height) ends above the next one — so no
## card is drawn over another card's name row or time. One line per pair.
static func overlapping_messages(root: Node, vp: Viewport) -> PackedStringArray:
	var out: PackedStringArray = []
	var lists: Array[Node] = root.find_children("*", "ChatMessageList", true, false)
	if root is ChatMessageList:
		lists.append(root)
	for node: Node in lists:
		var list: ChatMessageList = node as ChatMessageList
		if list.get_viewport() != vp or not list.is_visible_in_tree():
			continue
		var shown: Array[ChatBubble] = []
		for b: ChatBubble in list.bubbles():
			if b.visible and b.message != null:
				shown.append(b)
		shown.sort_custom(func(a: ChatBubble, c: ChatBubble) -> bool: return a.position.y < c.position.y)
		for i: int in range(1, shown.size()):
			var above: ChatBubble = shown[i - 1]
			var below: ChatBubble = shown[i]
			var bottom: float = above.position.y + maxf(above.size.y, above.get_combined_minimum_size().y)
			if bottom > below.position.y + TOLERANCE_PX:
				out.append("%s: message \"%s\" is drawn %.0f px over \"%s\"" % [_short(root, list),
						above.shown_text().left(24), bottom - below.position.y, below.shown_text().left(24)])
	return out


## F125 polish (STUDENT_GAME_UX_SOCIAL §8 ResultScreen): the button row `footer` under the scroll box `scroll` never
## covers it, and every control of `keep` (what the student must read at once, e.g. the outcome and the score of a
## result) is wholly inside the part of `scroll` seen at rest — not cut by its bottom edge just above the buttons, nor
## hidden under them. The seen part is the scroll box clipped by the scroll boxes around it and by the view.
static func footer_cuts(scroll: ScrollContainer, footer: Control, keep: Array[Control], vp: Viewport,
		label: String) -> PackedStringArray:
	var out: PackedStringArray = []
	var seen: Rect2 = scroll.get_global_rect().intersection(
			Rect2(Vector2.ZERO, Vector2(vp.size if vp is SubViewport else vp.get_visible_rect().size)))
	var p: Node = scroll.get_parent()
	while p != null:
		if p is ScrollContainer:
			seen = seen.intersection((p as Control).get_global_rect())
		p = p.get_parent()
	if footer != null and footer.is_visible_in_tree():
		for node: Node in footer.find_children("*", "BaseButton", true, false):
			var b: Control = node as Control
			if b.is_visible_in_tree() and b.get_global_rect().grow(-TOLERANCE_PX).intersects(seen):
				out.append("%s: button %s covers the scroll box %s" % [label, b.name, seen])
	for c: Control in keep:
		if c == null or not c.is_visible_in_tree() or c.size.y <= 0.0:
			continue
		var r: Rect2 = c.get_global_rect()
		if r.end.y > seen.end.y + TOLERANCE_PX:
			out.append("%s: %s cut at the bottom of the scroll box (%.0f px hidden above the buttons)" % [label, c.name,
					r.end.y - maxf(seen.end.y, r.position.y)])
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
