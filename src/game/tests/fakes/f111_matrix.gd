class_name F111Matrix
extends RefCounted
## TEST-ONLY (AGENTS R17): builds an F111 view (F111Screens) for every grade band × reference landscape window inside a
## SubViewport with the band Theme (F125: the phones 844×390 / 740×360 in compact mode, physical floors 12 / 40 CSS px)
## and runs BandLayoutProbe on it: touch targets, smallest font, nothing outside the view or cut, no overlapping buttons /
## message cards. F125 compact rules of F111: the club form's "Tạo club" and the invite picker's "Gửi lời mời" never
## scroll away (their action row sits under the scroll box, `footer_cuts`), the first event card / quest row / the season
## header shows above the fold. Returns every problem found; `seen` collects the evidence lines.
## F111 compact polish: on a compact phone the main content of every view is seen at rest (`content_cuts`) — its first
## rows / cards / the season strip wholly inside the view and every scroll box around them, and the content area seen at
## least `MIN_CONTENT_ROWS` compact touch targets tall (all of it when shorter) — a header never eats the screen.

const SETTLE_FRAMES: int = 6
## F111 compact polish: the content area of a view shows at least this many compact touch targets of height at rest.
const MIN_CONTENT_ROWS: float = 3.0

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
				if compact:
					failures.append_array(content_cuts(view, screen, vp, label))
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


## F111 compact polish: `{area, whole}` of `view` — the main content area and the controls that must be wholly seen.
static func main_content(view: String, screen: Control) -> Dictionary:
	var area: Control = null
	var whole: Array[Control] = []
	if screen is FriendsScreen:
		var fs: FriendsScreen = screen as FriendsScreen
		match view:
			"friends_join":
				area = fs.profile()
				whole.append(fs.profile().button(FriendProfilePanel.ACT_JOIN))
			"friends_suggest":
				var list: Node = fs.find_view().suggestion_group().find_child("List", false, false)
				area = list as Control
				if list != null and list.get_child_count() > 0:
					whole.append(list.get_child(0) as Control)
			"clubs":
				area = fs.club_view().find_child("Lists", false, false) as Control
				for row_name: String in ["Invitation", "ClubCard"]:
					var row: Control = fs.club_view().find_child(row_name, true, false) as Control
					if row != null and row.is_visible_in_tree():
						whole.append(row)
						break
			"club_page", "club_goals", "club_requests", "club_readonly":
				var page: ClubPage = fs.club_view().page_view()
				area = page.find_child("Content", false, false) as Control
				# The first three members (one card of another section); a grid of member cards counts card by card.
				var rows: int = 3 if view == "club_page" else 1
				var cards: Array[Node] = area.find_children("Member", "", true, false) if view in ["club_page",
						"club_readonly"] else area.get_children()
				for child: Node in cards:
					if whole.size() >= rows:
						break
					whole.append(child as Control)
				if view == "club_readonly":
					whole.append(page.find_child("ReadOnly", false, false) as Control)
				if view == "club_requests":
					whole.append(page.request_button(F111Fixtures.REQUEST_ID, true))
	elif screen is EventCenterScreen:
		var ec: EventCenterScreen = screen as EventCenterScreen
		match ec.tab:
			EventCenterScreen.TAB_TODAY, EventCenterScreen.TAB_WEEK:
				var day: QuestDayTab = ec.today_view() if ec.tab == EventCenterScreen.TAB_TODAY else ec.week_view()
				area = day
				var go: Array[Button] = day.go_buttons()
				if not go.is_empty():
					whole.append(go[0])
				elif day.all_done_visible():
					whole.append(day.find_child("AllDone", false, false) as Control)
			EventCenterScreen.TAB_SEASON:
				var season: SeasonTab = ec.season_view()
				area = season
				if season.track_view() != null:
					whole.append(season.track_view())
				else:
					whole.append(season.find_child("NoSeason", false, false) as Control)
			EventCenterScreen.TAB_EVENTS:
				if ec.detail_view().visible:
					var d: EventDetailPanel = ec.detail_view()
					area = d
					var section: String = "Milestones" if view == "event_ended" else "Quests"
					whole.append(d.find_child("EndedNote" if view == "event_ended" else "Points", true, false) as Control)
					var box: Node = d.find_child(section, false, false)
					var rows: Node = box.find_child("Rows", false, false) if box != null else null
					if rows != null and rows.get_child_count() > 0:
						whole.append(rows.get_child(0) as Control)
				else:
					area = ec.find_child("EventList", true, false) as Control
					var first: EventCard = ec.card(F111Fixtures.EVENT_FESTIVAL)
					if first != null:
						whole.append(first)
	return {"area": area, "whole": whole}


## F111 compact polish: the main content of `view` is seen at rest on a compact phone — every control of
## `main_content().whole` wholly inside the view and the scroll boxes around it, and the content area seen at least
## `MIN_CONTENT_ROWS` compact touch targets tall (all of it when it is shorter). One line per problem.
static func content_cuts(view: String, screen: Control, vp: Viewport, label: String) -> PackedStringArray:
	if not view in F111Screens.VIEWS or view in ["club_create", "club_invite"]:
		return PackedStringArray()
	var out: PackedStringArray = []
	var main: Dictionary = main_content(view, screen)
	var area: Control = main["area"] as Control
	if area == null or not area.is_visible_in_tree():
		return PackedStringArray(["%s: main content area missing" % label])
	var whole: Array = main["whole"] as Array
	if whole.is_empty():
		out.append("%s: no main content row to check" % label)
	for c: Variant in whole:
		var ctl: Control = c as Control
		if ctl == null or not ctl.is_visible_in_tree():
			out.append("%s: a main content control is missing or hidden" % label)
			continue
		var at_rest: Rect2 = seen_rect(ctl, vp)
		if not at_rest.grow(BandLayoutProbe.TOLERANCE_PX).encloses(ctl.get_global_rect()):
			out.append("%s: %s not wholly seen at rest %s (seen %s)" % [label, ctl.name, ctl.get_global_rect(), at_rest])
	var need: float = minf(area.size.y, MIN_CONTENT_ROWS * float(UiTokens.DENSITY_COMPACT_BUTTON_MIN_HEIGHT))
	var shown: Rect2 = seen_rect(area, vp).intersection(area.get_global_rect())
	if shown.size.y + BandLayoutProbe.TOLERANCE_PX < need:
		out.append("%s: content %s seen %.0f px tall < %.0f" % [label, area.name, shown.size.y, need])
	seen.append("%s: content %s seen %.0f of %.0f px" % [label, area.name, shown.size.y, area.size.y])
	return out


## The part of the view where `c` can be seen at rest: the view clipped by every scroll box around `c`.
static func seen_rect(c: Control, vp: Viewport) -> Rect2:
	var r: Rect2 = Rect2(Vector2.ZERO, Vector2(vp.size if vp is SubViewport else vp.get_visible_rect().size))
	var p: Node = c.get_parent()
	while p != null:
		if p is ScrollContainer:
			r = r.intersection((p as Control).get_global_rect())
		p = p.get_parent()
	return r
