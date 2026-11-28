class_name F110Matrix
extends RefCounted
## TEST-ONLY (AGENTS R17): builds an F110 view (F110Screens) for every grade band × reference landscape window inside
## a SubViewport with the band Theme (F125: the phones 844×390 / 740×360 in compact mode, physical floors 12 / 40 CSS
## px) and runs BandLayoutProbe on it: touch targets, smallest font, nothing outside the view or cut, no overlapping
## buttons / message cards; the result window also checks that its button row never covers the outcome
## (`footer_cuts`). Returns every problem found; `seen` collects the evidence lines.

const SETTLE_FRAMES: int = 6

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
