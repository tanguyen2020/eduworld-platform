class_name F111Matrix
extends RefCounted
## TEST-ONLY (AGENTS R17): builds an F111 view (F111Screens) for every grade band × reference landscape window inside a
## SubViewport with the band Theme (F125: the phones 844×390 / 740×360 in compact mode, physical floors 12 / 40 CSS px)
## and runs BandLayoutProbe on it: touch targets, smallest font, nothing outside the view or cut, no overlapping buttons /
## message cards. F125 compact rules of F111: the club form's "Tạo club" and the invite picker's "Gửi lời mời" never
## scroll away (their action row sits under the scroll box, `footer_cuts`), the first event card / quest row / the season
## header shows above the fold. Returns every problem found; `seen` collects the evidence lines.

const SETTLE_FRAMES: int = 6

static var seen: PackedStringArray = []


static func run(suite: Node, s: F111SceneHarness, view: String, sizes: Dictionary = BandLayoutProbe.SIZES,
		bands: PackedStringArray = UiTokens.DENSITY_BANDS) -> PackedStringArray:
	var failures: PackedStringArray = []
	var policy: SocialPolicyModel = AppState.social_policy
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
			root.name = "F111Root"
			root.theme = AppState.theme_service.current_theme
			vp.add_child(root)
			root.position = Vector2.ZERO
			root.size = Vector2(vp.size)
			var screen: Control = await F111Screens.build(view, root, s.services, s.h.http)
			for i: int in SETTLE_FRAMES:
				await suite.get_tree().process_frame
			var label: String = "%s %s %s" % [view, window, band]
			failures.append_array(BandLayoutProbe.check(root, vp, band, label, compact,
					BandLayoutProbe.css_scale_of(window) if compact else 0.0))
			if BandLayoutProbe.last_texts == 0:
				failures.append("%s: no visible text checked" % label)
			failures.append_array(BandLayoutProbe.overlapping_messages(root, vp))
			# The no-scroll rule of the main action holds for the real languages; the pseudo-locale (+35–40 %) only has to fit
			# without cuts (as F110Matrix).
			if String(Settings.locale) != L10n.pseudo_locale():
				failures.append_array(_primary_cuts(view, screen, vp, label))
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
	AppState.social_policy = policy
	return failures


## The main action / first content of `view` is on screen at rest (inside the view and every scroll box around it).
static func _primary_cuts(view: String, screen: Control, vp: Viewport, label: String) -> PackedStringArray:
	var target: Control = null
	match view:
		"club_create":
			target = (screen as FriendsScreen).club_form().submit_button()
		"club_invite":
			target = (screen as FriendsScreen).invite_picker().send_button()
		"events_list":
			target = (screen as EventCenterScreen).card(F111Fixtures.EVENT_FESTIVAL)
		"events_today":
			var go: Array[Button] = (screen as EventCenterScreen).today_view().go_buttons()
			target = go[0] if not go.is_empty() else null
		"friends_join":
			target = (screen as FriendsScreen).profile().button(FriendProfilePanel.ACT_JOIN)
	if target == null:
		return PackedStringArray() if not view in ["club_create", "club_invite", "events_list", "events_today",
				"friends_join"] else PackedStringArray(["%s: primary control missing" % label])
	var view_rect: Rect2 = Rect2(Vector2.ZERO, Vector2(vp.size if vp is SubViewport else vp.get_visible_rect().size)).grow(
			BandLayoutProbe.TOLERANCE_PX)
	var seen_rect: Rect2 = view_rect
	var p: Node = target.get_parent()
	while p != null:
		if p is ScrollContainer:
			seen_rect = seen_rect.intersection((p as Control).get_global_rect().grow(BandLayoutProbe.TOLERANCE_PX))
		p = p.get_parent()
	if not target.is_visible_in_tree() or not seen_rect.encloses(target.get_global_rect()):
		return PackedStringArray(["%s: %s not wholly on screen %s (seen %s)" % [label, target.name, target.get_global_rect(),
			seen_rect]])
	return PackedStringArray()
