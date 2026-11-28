class_name EventCenterScreen
extends Control
## Event center S14 (F111 US3, US4; STUDENT_GAME_UX_SOCIAL §9; EVENT_SEASON_SYSTEM §9): opened by the calendar button of
## the HUD, the "Thêm" menu, the event board of Quảng trường Tia Sáng or an `open_event` notification
## (`ui_requested("event_center", {tab, id})`). Tabs "Hôm nay", "Tuần này", "Sự kiện", "Mùa" — lớp 1–3 only "Hôm nay" and
## "Sự kiện" with big "Đi" buttons (F111-AC-18). "Sự kiện": EventCards → EventDetailPanel (join, quests, milestones,
## community goal). Opening the screen turns the static HUD dot off (F111-AC-29). No countdown, no streak, no "chỉ còn…",
## no pushing to play more (AGENTS R19). Only server values; RAM only. UI only: LiveEventService / SeasonTrackService /
## QuestService do the work (AGENTS R1).

signal closed()

const TAB_TODAY: String = "today"
const TAB_WEEK: String = "week"
const TAB_EVENTS: String = "events"
const TAB_SEASON: String = "season"
## [id, label key, icon, icon on the selected (blue) tab].
const TABS: Array = [
	[TAB_TODAY, "ui.events.tab.today", "emoji_sun", "emoji_sun"],
	[TAB_WEEK, "ui.events.tab.week", "journal", "journal"],
	[TAB_EVENTS, "ui.events.tab.events", "star", "star"],
	[TAB_SEASON, "ui.events.tab.season", "medal", "medal"],
]
## Lớp 1–3: only these tabs (STUDENT_GAME_UX_SOCIAL §9).
const G1_3_TABS: PackedStringArray = [TAB_TODAY, TAB_EVENTS]
const NOTICE_SEC: float = 4.0
## F111-AC-32: the 3D world behind is suspended while the Event center is open (a still, blurred snapshot — QualityManager).
const SUSPEND_REASON: StringName = &"event_center"

var services: ServiceLocator = null
var live: LiveEventService = null
var tab: String = TAB_EVENTS
var detail_id: String = ""
var _ui: Dictionary = {}
var _tabs: Dictionary = {}
var _pages: Dictionary = {}
var _events_list: VBoxContainer = null
var _events_empty: Control = null
var _detail: EventDetailPanel = null
var _events_scroll: ScrollContainer = null
var _notice: PanelContainer = null
var _notice_label: Label = null
var _notice_left: float = 0.0
var _cards: Dictionary = {}
var _today: QuestDayTab = null
var _week: QuestDayTab = null
var _season: SeasonTab = null
var season_service: SeasonTrackService = null
var _render_queued: bool = false


func _ready() -> void:
	visible = false
	set_anchors_preset(Control.PRESET_FULL_RECT)
	mouse_filter = Control.MOUSE_FILTER_STOP
	_build()
	EventBus.locale_changed.connect(_on_locale_changed)


func _exit_tree() -> void:
	_unbind()
	QualityManager.resume_world(SUSPEND_REASON)


## WorldRoot: `ui_requested("event_center", {tab, id})`.
func open(args: Dictionary = {}) -> void:
	if services == null and AppState.services != null:
		services = AppState.get_services()
	_bind()
	show()
	QualityManager.suspend_world(SUSPEND_REASON)
	UiMotion.pop_in(_ui["window"] as Control, Settings.reduce_motion)
	var wanted: String = String(args.get("tab", ""))
	var id: String = String(args.get("id", ""))
	if not id.is_empty():
		wanted = TAB_EVENTS
	select_tab(wanted if not wanted.is_empty() else (TAB_TODAY if allowed_tabs().has(TAB_TODAY) else TAB_EVENTS))
	(_ui["close"] as Button).grab_focus.call_deferred(true)
	await refresh()
	if Uuid.is_valid(id) and visible:
		await open_detail(id)


func close() -> void:
	hide()
	_unbind()
	QualityManager.resume_world(SUSPEND_REASON)
	closed.emit()


## Coroutine: the events (the HUD dot goes off once the student saw them).
func refresh() -> void:
	if live == null:
		return
	await live.load_list()
	if visible and live != null:
		live.store.mark_seen()


## Tabs of the student's band (lớp 1–3: Hôm nay, Sự kiện).
func allowed_tabs() -> PackedStringArray:
	var out: PackedStringArray = []
	for spec: Array in TABS:
		var id: String = String(spec[0])
		if _pages.has(id) and (UiBuild.band() != "g1_3" or id in G1_3_TABS):
			out.append(id)
	return out


func select_tab(next: String) -> void:
	var allowed: PackedStringArray = allowed_tabs()
	tab = next if next in allowed else (allowed[0] if not allowed.is_empty() else TAB_EVENTS)
	for spec: Array in TABS:
		var id: String = String(spec[0])
		var b: Button = _tabs.get(id) as Button
		if b == null:
			continue
		b.visible = id in allowed
		b.theme_type_variation = &"TabButtonSelected" if id == tab else &"TabButton"
		b.set_pressed_no_signal(id == tab)
	for id: String in _pages.keys():
		(_pages[id] as Control).visible = id == tab
	if visible:
		_load_tab()
	render()


func render() -> void:
	if live == null:
		return
	_render_events()
	_render_quests()
	_render_season()


## Coroutine: the data of the tab on screen (quests by type through QuestService, the season track).
func _load_tab() -> void:
	match tab:
		TAB_TODAY, TAB_WEEK:
			if services != null and services.quest != null:
				await services.quest.load_type("daily" if tab == TAB_TODAY else "weekly")
				_render_quests()
		TAB_SEASON:
			if season_service != null:
				await season_service.load_track()
				if season_service != null:
					await season_service.load_history()


func _render_quests() -> void:
	if services == null or services.quest == null:
		return
	var now: int = live.now_unix() if live != null else 0
	_today.show_quests(services.quest.of_type("daily"), now)
	_week.show_quests(services.quest.of_type("weekly"), now)


func _render_season() -> void:
	if season_service != null:
		_season.show_track(season_service.store, UiBuild.band())


## Coroutine: the detail of `event_id` (the list stays under it).
func open_detail(event_id: String) -> void:
	if live == null or not Uuid.is_valid(event_id):
		return
	detail_id = event_id
	_render_events()
	await live.get_detail(event_id)
	_render_events()


func close_detail() -> void:
	detail_id = ""
	_render_events()


# ---------- test hooks ----------

func tab_button(id: String) -> Button:
	return _tabs.get(id) as Button


func card(event_id: String) -> EventCard:
	return _cards.get(event_id) as EventCard


func today_view() -> QuestDayTab:
	return _today


func week_view() -> QuestDayTab:
	return _week


func season_view() -> SeasonTab:
	return _season


func detail_view() -> EventDetailPanel:
	return _detail


func events_empty_visible() -> bool:
	return _events_empty.is_visible_in_tree()


func notice_text() -> String:
	return _notice_label.text if _notice.visible else ""


func shown_texts() -> PackedStringArray:
	var out: PackedStringArray = []
	for node: Node in find_children("*", "Label", true, false):
		if (node as Label).is_visible_in_tree():
			out.append(tr((node as Label).text))
	for node: Node in find_children("*", "Button", true, false):
		if (node as Button).is_visible_in_tree():
			out.append(tr((node as Button).text))
	return out


func show_notice(key: String, args: Dictionary = {}) -> void:
	_notice_label.text = TranslationServer.translate(key).format(args)
	_notice.visible = not key.is_empty()
	_notice_left = NOTICE_SEC


# ---------- per frame ----------

func _process(delta: float) -> void:
	if _notice_left > 0.0:
		_notice_left -= delta
		if _notice_left <= 0.0:
			_notice.visible = false


func _unhandled_input(event: InputEvent) -> void:
	if visible and event.is_action_pressed("ui_cancel"):
		get_viewport().set_input_as_handled()
		if not detail_id.is_empty():
			close_detail()
		else:
			close()


# ---------- binding ----------

func _bind() -> void:
	_unbind()
	live = services.live_events if services != null else null
	season_service = services.season if services != null else null
	if live == null:
		return
	for pair: Array in _pairs():
		(pair[0] as Signal).connect(pair[1] as Callable)


func _unbind() -> void:
	if live == null:
		return
	for pair: Array in _pairs():
		if (pair[0] as Signal).is_connected(pair[1] as Callable):
			(pair[0] as Signal).disconnect(pair[1] as Callable)
	live = null
	season_service = null


func _pairs() -> Array:
	var out: Array = [[live.store.events_changed, _queue_render], [live.store.detail_changed, _on_detail_changed],
		[live.notice_requested, show_notice], [live.detail_closed, _on_detail_closed], [live.reward_claimed, _on_rewards],
		[live.busy_changed, _on_busy]]
	if season_service != null:
		out.append_array([[season_service.store.track_changed, _queue_render], [season_service.store.history_changed, _queue_render],
			[season_service.notice_requested, show_notice], [season_service.reward_claimed, _on_rewards],
			[season_service.busy_changed, _on_season_busy]])
	return out


# ---------- events tab ----------

func _render_events() -> void:
	if live == null:
		return
	var now: int = live.now_unix()
	var e: LiveEventModel = live.store.event(detail_id) if not detail_id.is_empty() else null
	_detail.visible = e != null
	_events_scroll.visible = e == null
	if e != null:
		_detail.show_event(e, now, UiBuild.band(), _quest_names())
		return
	if not detail_id.is_empty() and live.store.loaded:
		detail_id = ""
	UiBuild.clear(_events_list)
	_cards.clear()
	for ev: LiveEventModel in live.store.events:
		var c: EventCard = EventCard.new()
		c.opened.connect(open_detail)
		_events_list.add_child(c)
		c.show_event(ev, now)
		_cards[ev.event_id] = c
	_events_empty.visible = live.store.loaded and live.store.events.is_empty()
	UiBuild.size_buttons(_events_list)


func _quest_names() -> Dictionary:
	var out: Dictionary = {}
	if services != null and services.quest != null:
		for q: QuestModel in services.quest.quests:
			out[q.quest_id] = q
	return out


func _on_detail_changed(event_id: String) -> void:
	if visible and event_id == detail_id:
		_queue_render()


## F111-AC-32: store updates (realtime points, a season update…) are drawn once at the end of the frame, however many
## arrive in it — the handler of a server message stays cheap.
func _queue_render() -> void:
	if _render_queued:
		return
	_render_queued = true
	_flush_render.call_deferred()


func _flush_render() -> void:
	_render_queued = false
	if visible:
		render()


func _on_detail_closed(event_id: String, _key: String) -> void:
	if event_id == detail_id:
		close_detail()


func _on_rewards(rewards: Array) -> void:
	var words: PackedStringArray = []
	for r: Variant in rewards:
		if r is RewardModel:
			words.append((r as RewardModel).display_text())
	if not words.is_empty():
		show_notice("ui.events.reward_got", {"rewards": ", ".join(words)})


func _on_busy(busy: bool) -> void:
	_detail.set_busy(busy)


func _on_season_busy(busy: bool) -> void:
	_season.set_busy(busy)


func _on_tier_claim(tier: int) -> void:
	if season_service != null:
		await season_service.claim_tier(tier)


func _on_join(event_id: String) -> void:
	if live != null:
		await live.join(event_id)


func _on_claim(event_id: String, milestone_id: String) -> void:
	if live != null:
		await live.claim_milestone(event_id, milestone_id)


## "Đi" (F111-AC-10, AC-15): QuestService (F072) takes the quest (accept when it is still available) and the Quest
## Tracker leads the way; the Event center closes so the student sees the world.
func _on_go(quest_id: String) -> void:
	if services == null or services.quest == null:
		return
	await QuestGo.go(services.quest, quest_id)
	close()


func _on_locale_changed(_locale: String) -> void:
	if visible:
		render()


# ---------- build ----------

func _build() -> void:
	_ui = UiBuild.window(self, "ui.events.title", "star")
	(_ui["close"] as Button).pressed.connect(close)
	var body: VBoxContainer = _ui["body"] as VBoxContainer
	var tabs: HBoxContainer = UiBuild.hbox(&"ToolbarBox")
	tabs.name = "Tabs"
	var group: ButtonGroup = ButtonGroup.new()
	body.add_child(UiBuild.chip_scroller(tabs))
	_notice = UiBuild.panel(&"InfoBannerPanel")
	_notice.name = "Notice"
	_notice.visible = false
	_notice_label = UiBuild.keep_words(UiBuild.data_label("", &"InfoBannerLabel"))
	_notice.add_child(_notice_label)
	body.add_child(_notice)
	var pages: Control = UiBuild.vbox()
	pages.name = "Pages"
	UiBuild.expand(pages, true, true)
	body.add_child(pages)
	_build_pages(pages)
	for spec: Array in TABS:
		if not _pages.has(spec[0]):
			continue
		var b: Button = UiBuild.button(String(spec[1]), String(spec[2]), &"TabButton")
		b.name = "Tab_" + String(spec[0])
		b.toggle_mode = true
		b.button_group = group
		b.pressed.connect(select_tab.bind(String(spec[0])))
		tabs.add_child(b)
		_tabs[spec[0]] = b
	UiBuild.size_buttons(self)


func _build_pages(pages: Control) -> void:
	_today = QuestDayTab.new("daily")
	_today.go_requested.connect(_on_go)
	_pages[TAB_TODAY] = _scroll_page(pages, _today, "TodayPage")
	_week = QuestDayTab.new("weekly")
	_week.go_requested.connect(_on_go)
	_pages[TAB_WEEK] = _scroll_page(pages, _week, "WeekPage")
	_build_events(pages)
	_season = SeasonTab.new()
	_season.claim_requested.connect(_on_tier_claim)
	_pages[TAB_SEASON] = _scroll_page(pages, _season, "SeasonPage")


func _scroll_page(parent: Control, content: Control, page_name: String) -> Control:
	var scroll: ScrollContainer = ScrollContainer.new()
	scroll.name = page_name
	scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	UiBuild.expand(scroll, true, true)
	UiBuild.expand(content)
	scroll.add_child(content)
	parent.add_child(scroll)
	return scroll


func _build_events(pages: Control) -> void:
	var events: VBoxContainer = UiBuild.vbox()
	events.name = "EventsPage"
	UiBuild.expand(events, true, true)
	pages.add_child(events)
	_events_scroll = ScrollContainer.new()
	_events_scroll.name = "EventsScroll"
	_events_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	UiBuild.expand(_events_scroll, true, true)
	events.add_child(_events_scroll)
	var list_box: VBoxContainer = UiBuild.vbox(&"FormBox")
	UiBuild.expand(list_box)
	_events_scroll.add_child(list_box)
	_events_list = UiBuild.vbox(&"FormBox")
	_events_list.name = "EventList"
	list_box.add_child(_events_list)
	_events_empty = FriendsUi.empty_line("ui.events.none", "EventsEmpty")
	_events_empty.visible = false
	list_box.add_child(_events_empty)
	var detail_scroll: ScrollContainer = ScrollContainer.new()
	detail_scroll.name = "DetailScroll"
	detail_scroll.horizontal_scroll_mode = ScrollContainer.SCROLL_MODE_DISABLED
	UiBuild.expand(detail_scroll, true, true)
	events.add_child(detail_scroll)
	_detail = EventDetailPanel.new()
	UiBuild.expand(_detail)
	_detail.back_requested.connect(close_detail)
	_detail.join_requested.connect(_on_join)
	_detail.claim_requested.connect(_on_claim)
	_detail.go_requested.connect(_on_go)
	detail_scroll.add_child(_detail)
	_detail.visibility_changed.connect(func() -> void: detail_scroll.visible = _detail.visible)
	_detail.visible = false
	detail_scroll.visible = false
	_pages[TAB_EVENTS] = events
