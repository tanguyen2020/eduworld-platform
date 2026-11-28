class_name F112Matrix
extends RefCounted
## TEST-ONLY (AGENTS R17): builds an F112 view (F112Screens) for every grade band × reference landscape window inside a
## SubViewport with the band Theme (F125: the phones 844×390 / 740×360 in compact mode, physical floors 12 / 40 CSS px)
## and runs BandLayoutProbe (touch targets, smallest font, nothing outside the view or cut, no overlapping buttons /
## message cards), the "≥ 3 tin" rule on every open chat tab (ChatSpaceProbe), and the F112 compact rules
## (F112-AC-27): on a phone the HUD chat, the search and "Quản lý club" are overlays the full height of the screen, and
## the action row of a club form never scrolls away (`footer_cuts`). Returns every problem; `seen` keeps the evidence.

const SETTLE_FRAMES: int = 6
## Compact overlay: the window spans at least this share of the screen height.
const COMPACT_MIN_HEIGHT_SHARE: float = 0.9

static var seen: PackedStringArray = []


static func run(suite: Node, s: F112SceneHarness, view: String, sizes: Dictionary = BandLayoutProbe.SIZES,
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
			root.name = "F112Root"
			root.theme = AppState.theme_service.current_theme
			vp.add_child(root)
			root.position = Vector2.ZERO
			root.size = Vector2(vp.size)
			var screen: Control = await F112Screens.build(view, root, s.services, s.h.http)
			for i: int in SETTLE_FRAMES:
				await suite.get_tree().process_frame
			var label: String = "%s %s %s %s" % [view, window, band, String(Settings.locale)]
			failures.append_array(BandLayoutProbe.check(root, vp, band, label, compact,
					BandLayoutProbe.css_scale_of(window) if compact else 0.0))
			if BandLayoutProbe.last_texts == 0:
				failures.append("%s: no visible text checked" % label)
			failures.append_array(BandLayoutProbe.overlapping_messages(root, vp))
			if view.begins_with("hub_") and view != "hub_friends":
				failures.append_array(await ChatSpaceProbe.check(root, vp, label))
			if compact:
				failures.append_array(_compact_cuts(view, screen, vp, label))
			var note: String = ""
			if compact:
				note = ", smallest text %.1f CSS px, smallest touch target %.1f CSS px" % [BandLayoutProbe.last_min_text_px,
						BandLayoutProbe.last_min_touch_px]
			seen.append("%s: %d buttons, %d texts%s" % [label, BandLayoutProbe.last_buttons, BandLayoutProbe.last_texts, note])
			if screen != null and screen.has_method("close") and screen.is_visible_in_tree():
				screen.call("close")
			vp.queue_free()
			await suite.get_tree().process_frame
	AppState.set_compact(false)
	return failures


## F112-AC-27 on a phone: the window of the view is an overlay at least COMPACT_MIN_HEIGHT_SHARE of the screen tall;
## the action row of a club form stays under its scroll box (the main field and the button seen at rest).
static func _compact_cuts(view: String, screen: Control, vp: Viewport, label: String) -> PackedStringArray:
	var out: PackedStringArray = []
	var win: Control = null
	if screen is ChatHub:
		win = screen.find_child("Window", false, false) as Control
	elif screen is FriendsScreen:
		win = (screen as FriendsScreen).club_manage().find_child("Window", false, false) as Control
	if win == null:
		return PackedStringArray(["%s: window missing" % label])
	var share: float = win.get_global_rect().size.y / float((vp as SubViewport).size.y)
	if share + 0.001 < COMPACT_MIN_HEIGHT_SHARE:
		out.append("%s: window %.0f%% of the screen height < %.0f%%" % [label, share * 100.0, COMPACT_MIN_HEIGHT_SHARE * 100.0])
	if view.begins_with("manage") and view != "manage_officers":
		var manage: ClubManage = (screen as FriendsScreen).club_manage()
		var scroll: ScrollContainer = manage.find_child("Scroll", true, false) as ScrollContainer
		var footer: Control = manage.find_child("Footer", true, false) as Control
		out.append_array(BandLayoutProbe.footer_cuts(scroll, footer, [] as Array[Control], vp, label))
		var submit: Button = manage.submit_button()
		var view_rect: Rect2 = Rect2(Vector2.ZERO, Vector2((vp as SubViewport).size)).grow(BandLayoutProbe.TOLERANCE_PX)
		if not submit.is_visible_in_tree() or not view_rect.encloses(submit.get_global_rect()):
			out.append("%s: the form button is not on screen" % label)
	return out
