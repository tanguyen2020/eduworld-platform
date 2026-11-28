class_name F110Matrix
extends RefCounted
## TEST-ONLY (AGENTS R17): builds an F110 view (F110Screens) for every grade band × reference landscape window inside
## a SubViewport with the band Theme (F125: the phones 844×390 / 740×360 in compact mode, physical floors 12 / 40 CSS
## px) and runs BandLayoutProbe on it: touch targets, smallest font, nothing outside the view or cut, no overlapping
## buttons / message cards; the result window also checks that its button row never covers the outcome
## (`footer_cuts`). F110 compact polish: every answer tile of a boss question is wholly on screen at rest — inside the
## view and every scroll box around it, never below the fold (`answers_cut`) — and on a compact phone the co-op step
## shows the student's role card whole above its action row (`footer_cuts`). Returns every problem found; `seen`
## collects the evidence lines.

const SETTLE_FRAMES: int = 6
## Boss views that show a question (their answer tiles must be on screen without scrolling).
const QUESTION_VIEWS: PackedStringArray = ["boss_question", "boss_correct", "boss_wrong", "boss_retry", "boss_reconnect",
	"boss_solo"]

static var seen: PackedStringArray = []


static func run(suite: Node, s: F110SceneHarness, view: String, sizes: Dictionary = BandLayoutProbe.SIZES,
		bands: PackedStringArray = UiTokens.DENSITY_BANDS) -> PackedStringArray:
	var failures: PackedStringArray = []
	for band: String in bands:
		for window: String in sizes.keys():
			AppState.set_grade_band(band)
			var compact: bool = BandLayoutProbe.compact_of(window)
			AppState.set_compact(compact)
			var vp: SubViewport = SubViewport.new()
			vp.size = sizes[window] as Vector2i
			vp.render_target_update_mode = SubViewport.UPDATE_DISABLED
			suite.add_child(vp)
			var root: Control = Control.new()
			root.name = "F110Root"
			root.theme = AppState.theme_service.current_theme
			vp.add_child(root)
			root.position = Vector2.ZERO
			root.size = Vector2(vp.size)
			var screen: Control = await F110Screens.build(view, root, s.services, s.h.http)
			for i: int in SETTLE_FRAMES:
				await suite.get_tree().process_frame
			var label: String = "%s %s" % [view, window]
			failures.append_array(BandLayoutProbe.check(root, vp, band, label, compact,
					BandLayoutProbe.css_scale_of(window) if compact else 0.0))
			if BandLayoutProbe.last_texts == 0:
				failures.append("%s %s: no visible text checked" % [label, band])
			failures.append_array(BandLayoutProbe.overlapping_messages(root, vp))
			if view.begins_with("boss_result"):
				failures.append_array(_result_cuts(screen, vp, "%s %s" % [label, band]))
			# The no-scroll rules hold for the real languages (vi, en); the pseudo-locale (+35–40 %) only has to fit
			# without cuts (a taller HUD may then scroll the board, as any long translation would).
			var real: bool = String(Settings.locale) != L10n.pseudo_locale()
			if view in QUESTION_VIEWS and real:
				failures.append_array(answers_cut(screen, vp, "%s %s" % [label, band]))
			if view == "coop_step" and compact and real:
				failures.append_array(_role_card_cuts(screen, vp, "%s %s" % [label, band]))
			var note: String = ""
			if compact:
				note = ", smallest text %.1f CSS px, smallest touch target %.1f CSS px" % [BandLayoutProbe.last_min_text_px,
						BandLayoutProbe.last_min_touch_px]
			seen.append("%s %s: %d buttons, %d texts%s" % [label, band, BandLayoutProbe.last_buttons,
					BandLayoutProbe.last_texts, note])
			if screen != null and screen.has_method("close") and screen.is_visible_in_tree():
				screen.call("close")
			vp.queue_free()
			await suite.get_tree().process_frame
	AppState.set_compact(false)
	return failures


static func _result_cuts(screen: Control, vp: Viewport, label: String) -> PackedStringArray:
	var arena: BossArena = screen as BossArena
	if arena == null:
		return PackedStringArray(["%s: not a boss arena" % label])
	var r: BossResult = arena.result_view()
	var scroll: ScrollContainer = r.find_child("Scroll", true, false) as ScrollContainer
	var outcome: Control = r.find_child("Outcome", true, false) as Control
	var keep: Array[Control] = [outcome]
	return BandLayoutProbe.footer_cuts(scroll, r.find_child("Buttons", true, false) as Control, keep, vp, label)


## The answer tiles of the boss question of `screen` (a BossArena) that are not wholly seen at rest: outside the view or
## cut by a scroll box around them (one line each; none at all is a problem too).
static func answers_cut(screen: Control, vp: Viewport, label: String) -> PackedStringArray:
	var out: PackedStringArray = []
	var arena: BossArena = screen as BossArena
	if arena == null or arena.question_panel().renderer == null:
		return PackedStringArray(["%s: no boss question on screen" % label])
	var view: Rect2 = Rect2(Vector2.ZERO, Vector2(vp.size if vp is SubViewport else vp.get_visible_rect().size)).grow(
			BandLayoutProbe.TOLERANCE_PX)
	var tiles: int = 0
	for node: Node in arena.question_panel().find_children("*", "BaseButton", true, false):
		var b: Control = node as Control
		if not b.is_visible_in_tree() or not String(b.theme_type_variation).begins_with("AnswerTile"):
			continue
		tiles += 1
		var seen: Rect2 = view
		var p: Node = b.get_parent()
		while p != null:
			if p is ScrollContainer:
				seen = seen.intersection((p as Control).get_global_rect().grow(BandLayoutProbe.TOLERANCE_PX))
			p = p.get_parent()
		if not seen.encloses(b.get_global_rect()):
			out.append("%s: answer tile %s not wholly on screen %s (seen %s)" % [label, b.name, b.get_global_rect(), seen])
	if tiles == 0:
		out.append("%s: no answer tile on screen" % label)
	return out


static func _role_card_cuts(screen: Control, vp: Viewport, label: String) -> PackedStringArray:
	var panel: InterdependentStepPanel = screen as InterdependentStepPanel
	if panel == null or panel.my_card(F110Fixtures.ROLE_MEASURE) == null:
		return PackedStringArray(["%s: no role card of the student" % label])
	var keep: Array[Control] = [panel.my_card(F110Fixtures.ROLE_MEASURE)]
	var out: PackedStringArray = BandLayoutProbe.footer_cuts(panel.scroll_box(), panel.actions_row(), keep, vp, label)
	if not panel.final_button().is_visible_in_tree():
		out.append("%s: no final answer button" % label)
	return out
